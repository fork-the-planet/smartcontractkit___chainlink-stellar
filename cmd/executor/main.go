package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"go.uber.org/zap/zapcore"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccv/executor"
	x "github.com/smartcontractkit/chainlink-ccv/executor/pkg/adapter"
	ex "github.com/smartcontractkit/chainlink-ccv/executor/pkg/executor"
	"github.com/smartcontractkit/chainlink-ccv/executor/pkg/leaderelector"
	"github.com/smartcontractkit/chainlink-ccv/executor/pkg/monitoring"
	"github.com/smartcontractkit/chainlink-ccv/integration/pkg/backofftimeprovider"
	"github.com/smartcontractkit/chainlink-ccv/integration/pkg/ccvstreamer"
	"github.com/smartcontractkit/chainlink-ccv/integration/pkg/cursechecker"
	"github.com/smartcontractkit/chainlink-ccv/pkg/chainaccess"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-ccv/protocol/common/logging"
	"github.com/smartcontractkit/chainlink-common/pkg/beholder"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	"github.com/smartcontractkit/chainlink-stellar/ccv/common"
	contracttransmitter "github.com/smartcontractkit/chainlink-stellar/ccv/contract_transmitter"
	destinationreader "github.com/smartcontractkit/chainlink-stellar/ccv/destination_reader"
	"github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/keypair"
)

const (
	StellarConfigPathEnv            = "STELLAR_CONFIG_PATH"
	indexerPollingInterval           = 1 * time.Second
	indexerGarbageCollectionInterval = 1 * time.Hour
	messageContextWindow            = 9 * time.Hour
)

func loadConfig(path string) (*common.Config, error) {
	var cfg common.Config
	if md, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", path, err)
	} else if len(md.Undecoded()) > 0 {
		return nil, fmt.Errorf("unknown fields in config: %v", md.Undecoded())
	}

	return &cfg, nil
}

func loadExecutorConfig(filepath string) (*executor.Configuration, error) {
	var config executor.ConfigWithBlockchainInfo
	if _, err := toml.DecodeFile(filepath, &config); err != nil {
		return nil, err
	}

	return config.GetNormalizedConfig()
}

func main() {
	lggr, err := logger.NewWith(logging.DevelopmentConfig(zapcore.InfoLevel))
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}
	lggr = logger.Sugared(logger.Named(lggr, "StellarExecutor"))

	executorConfigPath := executor.DefaultConfigFile
	if len(os.Args) > 1 {
		executorConfigPath = os.Args[1]
	}
	if envConfig := os.Getenv("EXECUTOR_CONFIG_PATH"); envConfig != "" {
		executorConfigPath = envConfig
	}

	executorConfig, err := loadExecutorConfig(executorConfigPath)
	if err != nil {
		lggr.Errorw("Failed to load executor configuration", "path", executorConfigPath, "error", err)
		os.Exit(1)
	}

	protocol.InitChainSelectorCache()

	configPath, ok := os.LookupEnv(StellarConfigPathEnv)
	if !ok {
		configPath = common.DefaultStellarConfigPath
	}

	stellarConfig, err := loadConfig(configPath)
	if err != nil {
		lggr.Errorw("Failed to load stellar config", "path", configPath, "error", err)
		os.Exit(1)
	}

	for sel, tc := range stellarConfig.TransmitterConfigs {
		if tc.OffRampContractID == "" {
			if chainCfg, ok := executorConfig.ChainConfiguration[sel]; ok && chainCfg.OffRampAddress != "" {
				addr, err := scval.HexToContractStrkey(chainCfg.OffRampAddress)
				if err != nil {
					lggr.Errorw("Convert OffRamp hex to strkey failed", "chain", sel, "error", err)
					os.Exit(1)
				}
				tc.OffRampContractID = addr
			}
		}
		if tc.RMNRemoteAddress == "" {
			if chainCfg, ok := executorConfig.ChainConfiguration[sel]; ok && chainCfg.RmnAddress != "" {
				addr, err := scval.HexToContractStrkey(chainCfg.RmnAddress)
				if err != nil {
					lggr.Errorw("Convert RMN Remote hex to strkey failed", "chain", sel, "error", err)
					os.Exit(1)
				}
				tc.RMNRemoteAddress = addr
			}
		}
		stellarConfig.TransmitterConfigs[sel] = tc
	}

	// Setup monitoring
	var executorMonitoring executor.Monitoring
	if executorConfig.Monitoring.Enabled && executorConfig.Monitoring.Type == "beholder" {
		beholderConfig := beholder.Config{
			InsecureConnection:       executorConfig.Monitoring.Beholder.InsecureConnection,
			CACertFile:               executorConfig.Monitoring.Beholder.CACertFile,
			OtelExporterHTTPEndpoint: executorConfig.Monitoring.Beholder.OtelExporterHTTPEndpoint,
			OtelExporterGRPCEndpoint: executorConfig.Monitoring.Beholder.OtelExporterGRPCEndpoint,
			LogStreamingEnabled:      executorConfig.Monitoring.Beholder.LogStreamingEnabled,
			MetricReaderInterval:     time.Second * time.Duration(executorConfig.Monitoring.Beholder.MetricReaderInterval),
			TraceSampleRatio:         executorConfig.Monitoring.Beholder.TraceSampleRatio,
			TraceBatchTimeout:        time.Second * time.Duration(executorConfig.Monitoring.Beholder.TraceBatchTimeout),
			MetricViews:              monitoring.MetricViews(),
		}
		beholderClient, err := beholder.NewClient(beholderConfig)
		if err != nil {
			lggr.Fatalf("Failed to create beholder client: %v", err)
		}
		beholder.SetClient(beholderClient)
		beholder.SetGlobalOtelProviders()

		executorMonitoring, err = monitoring.InitMonitoring()
		if err != nil {
			lggr.Fatalf("Failed to initialize monitoring: %v", err)
		}
	} else {
		executorMonitoring = monitoring.NewNoopExecutorMonitoring()
	}

	contractTransmitters := make(map[protocol.ChainSelector]chainaccess.ContractTransmitter)
	destReaders := make(map[protocol.ChainSelector]chainaccess.DestinationReader)
	rmnReaders := make(map[protocol.ChainSelector]chainaccess.RMNCurseReader)
	enabledDestChains := make([]protocol.ChainSelector, 0)

	for strSel, tc := range stellarConfig.TransmitterConfigs {
		selector, err := strconv.ParseUint(strSel, 10, 64)
		if err != nil {
			lggr.Errorw("Invalid chain selector", "error", err, "chainSelector", strSel)
			continue
		}

		family, err := chainsel.GetSelectorFamily(selector)
		if err != nil || family != chainsel.FamilyStellar {
			lggr.Warnw("Skipping non-Stellar chain", "chainSelector", strSel)
			continue
		}

		// TODO: get deployer keypair from env instead of generating a random one
		deployerSeed := sha256.Sum256(fmt.Appendf(nil, "deployer-%s", tc.NetworkPassphrase))
		deployerKeypair, err := keypair.FromRawSeed(deployerSeed)
		if err != nil {
			lggr.Errorw("Failed to create deployer keypair", "chain", strSel, "error", err)
			os.Exit(1)
		}

		zerologLogger := zerolog.New(os.Stdout).With().
			Str("chain_selector", strSel).
			Str("component", "executor").
			Logger().Level(zerolog.InfoLevel)

		rpcClient := rpcclient.NewClient(tc.NetworkPassphrase, &http.Client{Timeout: 60 * time.Second})

		if rc, ok := stellarConfig.ReaderConfigs[strSel]; ok && rc.SorobanRPCURL != "" {
			rpcClient = rpcclient.NewClient(rc.SorobanRPCURL, &http.Client{Timeout: 60 * time.Second})
		}

		invoker := deployment.NewDeployer(
			rpcClient,
			tc.NetworkPassphrase,
			deployerKeypair,
		)

		ct, err := contracttransmitter.NewContractTransmitterWithClient(
			invoker,
			tc.OffRampContractID,
			tc.CCIPStateChangedTopic,
			tc.RMNRemoteAddress,
			&zerologLogger,
		)
		if err != nil {
			lggr.Errorw("Failed to create contract transmitter", "chain", strSel, "error", err)
			os.Exit(1)
		}
		contractTransmitters[protocol.ChainSelector(selector)] = ct

		drCfg, hasDRCfg := stellarConfig.DestinationReaderConfigs[strSel]
		offRampID := tc.OffRampContractID
		rmnRemoteID := tc.RMNRemoteAddress
		if hasDRCfg {
			if drCfg.OffRampContractID != "" {
				offRampID = drCfg.OffRampContractID
			}
			if drCfg.RMNRemoteContractID != "" {
				rmnRemoteID = drCfg.RMNRemoteContractID
			}
		}

		dr, err := destinationreader.New(invoker, rpcClient, offRampID, rmnRemoteID, &zerologLogger, executorConfig.MaxRetryDuration)
		if err != nil {
			lggr.Errorw("Failed to create destination reader", "chain", strSel, "error", err)
			os.Exit(1)
		}
		destReaders[protocol.ChainSelector(selector)] = dr
		rmnReaders[protocol.ChainSelector(selector)] = dr
		enabledDestChains = append(enabledDestChains, protocol.ChainSelector(selector))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	curseChecker := cursechecker.NewCachedCurseChecker(cursechecker.Params{
		Lggr:        lggr,
		Metrics:     executorMonitoring.Metrics(),
		RmnReaders:  rmnReaders,
		CacheExpiry: executorConfig.ReaderCacheExpiry,
	})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	verifierResultReader, err := x.NewIndexerReaderAdapter(
		executorConfig.IndexerAddress,
		httpClient,
		executorMonitoring,
		lggr,
	)
	if err != nil {
		lggr.Errorw("Failed to create indexer adapter", "error", err)
		os.Exit(1)
	}

	execPool := make(map[protocol.ChainSelector][]string)
	execIntervals := make(map[protocol.ChainSelector]time.Duration)
	defaultExecutorAddresses := make(map[protocol.ChainSelector]protocol.UnknownAddress)

	for strSel, chainConfig := range executorConfig.ChainConfiguration {
		selector, err := strconv.ParseUint(strSel, 10, 64)
		if err != nil {
			lggr.Errorw("Invalid chain selector in configuration", "error", err, "chainSelector", strSel)
			continue
		}
		execPool[protocol.ChainSelector(selector)] = chainConfig.ExecutorPool
		execIntervals[protocol.ChainSelector(selector)] = chainConfig.ExecutionInterval
		defaultExecutorAddresses[protocol.ChainSelector(selector)], err = protocol.NewUnknownAddressFromHex(chainConfig.DefaultExecutorAddress)
		if err != nil {
			lggr.Errorw("Invalid default executor address", "error", err, "chainSelector", strSel)
			continue
		}
	}

	chainlinkExecutor := ex.NewChainlinkExecutor(lggr, contractTransmitters, destReaders, curseChecker, verifierResultReader, executorMonitoring, defaultExecutorAddresses)
	if err := chainlinkExecutor.Validate(); err != nil {
		lggr.Errorw("Failed to validate executor", "error", err)
		os.Exit(1)
	}

	le, err := leaderelector.NewHashBasedLeaderElector(
		lggr,
		execPool,
		executorConfig.ExecutorID,
		execIntervals,
	)
	if err != nil {
		lggr.Errorw("Failed to create leader elector", "error", err)
		os.Exit(1)
	}

	timeProvider := backofftimeprovider.NewBackoffNTPProvider(lggr, executorConfig.BackoffDuration, executorConfig.NtpServer)

	indexerStream := ccvstreamer.NewIndexerStorageStreamer(
		lggr,
		ccvstreamer.IndexerStorageConfig{
			IndexerClient:     verifierResultReader,
			InitialQueryTime:  time.Now().Add(-1 * executorConfig.LookbackWindow),
			PollingInterval:   indexerPollingInterval,
			Backoff:           executorConfig.BackoffDuration,
			QueryLimit:        executorConfig.IndexerQueryLimit,
			ExpiryDuration:    messageContextWindow,
			CleanInterval:     indexerGarbageCollectionInterval,
			TimeProvider:      timeProvider,
			EnabledDestChains: enabledDestChains,
		})

	coordinator, err := executor.NewCoordinator(
		lggr,
		chainlinkExecutor,
		indexerStream,
		le,
		executorMonitoring,
		executorConfig.MaxRetryDuration,
		timeProvider,
		executorConfig.WorkerCount,
	)
	if err != nil {
		lggr.Errorw("Failed to create execution coordinator", "error", err)
		os.Exit(1)
	}

	if err := coordinator.Start(ctx); err != nil {
		lggr.Errorw("Failed to start execution coordinator", "error", err)
		os.Exit(1)
	}

	lggr.Infow("Stellar executor started successfully")

	<-sigCh
	lggr.Infow("Shutdown signal received, stopping executor...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	done := make(chan error, 1)
	go func() {
		done <- coordinator.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			lggr.Errorw("Execution coordinator stop error", "error", err)
		}
	case <-shutdownCtx.Done():
		lggr.Errorw("Execution coordinator shutdown timed out")
	}

	lggr.Infow("Stellar executor stopped")
}
