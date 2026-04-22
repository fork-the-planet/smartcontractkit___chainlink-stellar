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

// stellarConfigPath returns STELLAR_CONFIG_PATH or the default bind-mount path.
func stellarConfigPath() string {
	if p, ok := os.LookupEnv(StellarConfigPathEnv); ok {
		return p
	}
	return common.DefaultStellarConfigPath
}

// mergeFileAndJobReaderConfigs starts from file-backed reader_configs and merges
// each Stellar chain's blockchain_infos entry from the job spec (non-empty job fields win).
func mergeFileAndJobReaderConfigs(
	file map[string]sourcereader.ReaderConfig,
	job chainaccess.Infos[sourcereader.ReaderConfig],
) map[string]sourcereader.ReaderConfig {
	out := make(map[string]sourcereader.ReaderConfig)
	if file != nil {
		maps.Copy(out, file)
	}
	for sel, jobCfg := range job {
		out[sel] = mergeReaderConfig(out[sel], jobCfg)
	}
	return out
}

// applyOnRampRMNHexOverrides fills empty OnRampContractID / RMNRemoteContractID from
// committee-style hex maps (job spec), converting to Stellar contract strkeys.
func applyOnRampRMNHexOverrides(
	readerConfigs map[string]sourcereader.ReaderConfig,
	onRampHexBySelector map[string]string,
	rmnRemoteHexBySelector map[string]string,
) error {
	for sel, rc := range readerConfigs {
		if rc.OnRampContractID == "" {
			if onrampHex, ok := onRampHexBySelector[sel]; ok && onrampHex != "" {
				addr, err := scval.HexToContractStrkey(onrampHex)
				if err != nil {
					return fmt.Errorf("convert OnRamp hex to strkey for chain %s: %w", sel, err)
				}
				rc.OnRampContractID = addr
			}
		}
		if rc.RMNRemoteContractID == "" {
			if rmnHex, ok := rmnRemoteHexBySelector[sel]; ok && rmnHex != "" {
				addr, err := scval.HexToContractStrkey(rmnHex)
				if err != nil {
					return fmt.Errorf("convert RMN Remote hex to strkey for chain %s: %w", sel, err)
				}
				rc.RMNRemoteContractID = addr
			}
		}
		readerConfigs[sel] = rc
	}
	return nil
}

func loadStellarJobReaderInfos(genericConfig chainaccess.GenericConfig) (chainaccess.Infos[sourcereader.ReaderConfig], error) {
	var jobInfos chainaccess.Infos[sourcereader.ReaderConfig]
	if err := genericConfig.GetAllConcreteConfig(chainsel.FamilyStellar, &jobInfos); err != nil {
		return nil, fmt.Errorf("get stellar blockchain_infos: %w", err)
	}
	return jobInfos, nil
}

// buildStellarReaderConfigs loads the Stellar file, merges job blockchain_infos for
// FamilyStellar, then applies on-ramp / RMN remote hex overrides from genericConfig.
func buildStellarReaderConfigs(configPath string, genericConfig chainaccess.GenericConfig) (map[string]sourcereader.ReaderConfig, error) {
	stellarFileCfg, err := loadStellarFileConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load stellar config: %w", err)
	}

	jobInfos, err := loadStellarJobReaderInfos(genericConfig)
	if err != nil {
		return nil, err
	}

	readerConfigs := mergeFileAndJobReaderConfigs(stellarFileCfg.ReaderConfigs, jobInfos)
	if err := applyOnRampRMNHexOverrides(readerConfigs, genericConfig.OnRampAddresses, genericConfig.RMNRemoteAddresses); err != nil {
		return nil, err
	}
	return readerConfigs, nil
}

// CreateStellarAccessorFactory is registered with chainaccess.Register for the Stellar family.
// It merges per-chain reader settings from the Stellar TOML file, Stellar sections under
// blockchain_infos in the job spec, and on-ramp / RMN remote hex addresses from GenericConfig
// (same behavior as the legacy committee-verifier bootstrap callback).
func CreateStellarAccessorFactory(lggr logger.Logger, genericConfig chainaccess.GenericConfig) (chainaccess.AccessorFactory, error) {
	readerConfigs, err := buildStellarReaderConfigs(stellarConfigPath(), genericConfig)
	if err != nil {
		return nil, err
	}
	return NewFactory(lggr, readerConfigs), nil
}
