package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"go.uber.org/zap/zapcore"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccv/bootstrap"
	cmd "github.com/smartcontractkit/chainlink-ccv/cmd/executor"
	"github.com/smartcontractkit/chainlink-ccv/executor"
	"github.com/smartcontractkit/chainlink-ccv/pkg/chainaccess"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	"github.com/smartcontractkit/chainlink-stellar/ccv/common"
	contracttransmitter "github.com/smartcontractkit/chainlink-stellar/ccv/contract_transmitter"
	destinationreader "github.com/smartcontractkit/chainlink-stellar/ccv/destination_reader"
	"github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/keypair"
)

const StellarConfigPathEnv = "STELLAR_CONFIG_PATH"

func loadConfig(path string) (*common.Config, error) {
	var cfg common.Config
	if md, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", path, err)
	} else if len(md.Undecoded()) > 0 {
		return nil, fmt.Errorf("unknown fields in config: %v", md.Undecoded())
	}

	return &cfg, nil
}

func main() {
	if err := bootstrap.Run(
		"StellarExecutor",
		cmd.NewServiceFactory[any](
			chainsel.FamilyStellar,
			func(
				ctx context.Context,
				lggr logger.Logger,
				_ map[string]*any,
				cfg executor.Configuration,
			) (*cmd.ServiceComponents, error) {
				configPath, ok := os.LookupEnv(StellarConfigPathEnv)
				if !ok {
					configPath = common.DefaultStellarConfigPath
				}

				stellarConfig, err := loadConfig(configPath)
				if err != nil {
					return nil, fmt.Errorf("failed to load config: %w", err)
				}

				// TODO: this may be removed once we have a way to get the contract IDs from the modifier.
				// The bind-mounted config file is created before contracts are
				// deployed, so OffRamp and RMN Remote addresses may be missing.
				// Fill them in from the executor Configuration, which is
				// generated after contract deployment.
				for sel, tc := range stellarConfig.TransmitterConfigs {
					if tc.OffRampContractID == "" {
						if chainCfg, ok := cfg.ChainConfiguration[sel]; ok && chainCfg.OffRampAddress != "" {
							addr, err := scval.HexToContractStrkey(chainCfg.OffRampAddress)
							if err != nil {
								return nil, fmt.Errorf("convert OffRamp hex to strkey for chain %s: %w", sel, err)
							}
							tc.OffRampContractID = addr
						}
					}
					if tc.RMNRemoteAddress == "" {
						if chainCfg, ok := cfg.ChainConfiguration[sel]; ok && chainCfg.RmnAddress != "" {
							addr, err := scval.HexToContractStrkey(chainCfg.RmnAddress)
							if err != nil {
								return nil, fmt.Errorf("convert RMN Remote hex to strkey for chain %s: %w", sel, err)
							}
							tc.RMNRemoteAddress = addr
						}
					}
					stellarConfig.TransmitterConfigs[sel] = tc
				}

				contractTransmitters := make(map[protocol.ChainSelector]chainaccess.ContractTransmitter)
				destReaders := make(map[protocol.ChainSelector]chainaccess.DestinationReader)
				rmnReaders := make(map[protocol.ChainSelector]chainaccess.RMNCurseReader)

				for strSel, tc := range stellarConfig.TransmitterConfigs {
					selector, err := strconv.ParseUint(strSel, 10, 64)
					if err != nil {
						return nil, fmt.Errorf("invalid chain selector %s: %w", strSel, err)
					}

					family, err := chainsel.GetSelectorFamily(selector)
					if err != nil || family != chainsel.FamilyStellar {
						continue
					}

					// TODO: get deployer keypair from env instead of generating a random one
					deployerSeed := sha256.Sum256(fmt.Appendf(nil, "deployer-%s", tc.NetworkPassphrase))
					deployerKeypair, err := keypair.FromRawSeed(deployerSeed)
					if err != nil {
						return nil, fmt.Errorf("failed to create deployer keypair for chain %s: %w", strSel, err)
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
						return nil, fmt.Errorf("failed to create contract transmitter for chain %s: %w", strSel, err)
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

					dr, err := destinationreader.New(invoker, rpcClient, offRampID, rmnRemoteID, &zerologLogger, cfg.MaxRetryDuration)
					if err != nil {
						return nil, fmt.Errorf("failed to create destination reader for chain %s: %w", strSel, err)
					}
					destReaders[protocol.ChainSelector(selector)] = dr
					rmnReaders[protocol.ChainSelector(selector)] = dr
				}

				return &cmd.ServiceComponents{
					ContractTransmitters: contractTransmitters,
					DestinationReaders:   destReaders,
					RMNCurseReaders:      rmnReaders,
				}, nil
			}),
		bootstrap.WithLogLevel[executor.JobSpec](zapcore.InfoLevel),
	); err != nil {
		panic(fmt.Sprintf("failed to run Stellar executor: %s", err.Error()))
	}
}
