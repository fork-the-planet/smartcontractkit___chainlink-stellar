package accessors

import (
	"fmt"
	"maps"
	"os"

	"github.com/BurntSushi/toml"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccv/pkg/chainaccess"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	"github.com/smartcontractkit/chainlink-stellar/ccv/common"
	sourcereader "github.com/smartcontractkit/chainlink-stellar/ccv/source_reader"
)

func init() {
	chainaccess.Register(chainsel.FamilyStellar, CreateStellarAccessorFactory)
}

var _ chainaccess.AccessorFactoryConstructor = CreateStellarAccessorFactory

// StellarConfigPathEnv is the env var for the bind-mounted Stellar TOML (RPC, etc.).
const StellarConfigPathEnv = "STELLAR_CONFIG_PATH"

func loadStellarFileConfig(path string) (*common.Config, error) {
	var cfg common.Config
	if md, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", path, err)
	} else if len(md.Undecoded()) > 0 {
		return nil, fmt.Errorf("unknown fields in config: %v", md.Undecoded())
	}
	return &cfg, nil
}

func mergeReaderConfig(base, overlay sourcereader.ReaderConfig) sourcereader.ReaderConfig {
	out := base
	if overlay.NetworkPassphrase != "" {
		out.NetworkPassphrase = overlay.NetworkPassphrase
	}
	if overlay.OnRampContractID != "" {
		out.OnRampContractID = overlay.OnRampContractID
	}
	if overlay.RMNRemoteContractID != "" {
		out.RMNRemoteContractID = overlay.RMNRemoteContractID
	}
	if overlay.SorobanRPCURL != "" {
		out.SorobanRPCURL = overlay.SorobanRPCURL
	}
	return out
}

// CreateStellarAccessorFactory is registered with chainaccess.Register for the Stellar family.
// It merges per-chain reader settings from the Stellar TOML file, Stellar sections under
// blockchain_infos in the job spec, and on-ramp / RMN remote hex addresses from GenericConfig
// (same behavior as the legacy committee-verifier bootstrap callback).
func CreateStellarAccessorFactory(lggr logger.Logger, genericConfig chainaccess.GenericConfig) (chainaccess.AccessorFactory, error) {
	_ = lggr

	configPath, ok := os.LookupEnv(StellarConfigPathEnv)
	if !ok {
		configPath = common.DefaultStellarConfigPath
	}

	stellarFileCfg, err := loadStellarFileConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load stellar config: %w", err)
	}

	readerConfigs := make(map[string]sourcereader.ReaderConfig)
	if stellarFileCfg.ReaderConfigs != nil {
		maps.Copy(readerConfigs, stellarFileCfg.ReaderConfigs)
	}

	var jobInfos chainaccess.Infos[sourcereader.ReaderConfig]
	if err := genericConfig.GetAllConcreteConfig(chainsel.FamilyStellar, &jobInfos); err != nil {
		return nil, fmt.Errorf("get stellar blockchain_infos: %w", err)
	}
	for sel, jobCfg := range jobInfos {
		readerConfigs[sel] = mergeReaderConfig(readerConfigs[sel], jobCfg)
	}

	// The bind-mounted config file may be created before contracts are deployed, so
	// OnRamp and RMN Remote addresses may be missing. Fill from GenericConfig (job spec),
	// which is generated after contract deployment.
	for sel, rc := range readerConfigs {
		if rc.OnRampContractID == "" {
			if onrampHex, ok := genericConfig.OnRampAddresses[sel]; ok && onrampHex != "" {
				addr, err := scval.HexToContractStrkey(onrampHex)
				if err != nil {
					return nil, fmt.Errorf("convert OnRamp hex to strkey for chain %s: %w", sel, err)
				}
				rc.OnRampContractID = addr
			}
		}
		if rc.RMNRemoteContractID == "" {
			if rmnHex, ok := genericConfig.RMNRemoteAddresses[sel]; ok && rmnHex != "" {
				addr, err := scval.HexToContractStrkey(rmnHex)
				if err != nil {
					return nil, fmt.Errorf("convert RMN Remote hex to strkey for chain %s: %w", sel, err)
				}
				rc.RMNRemoteContractID = addr
			}
		}
		readerConfigs[sel] = rc
	}

	return NewFactory(lggr, readerConfigs), nil
}
