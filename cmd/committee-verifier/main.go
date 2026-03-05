package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	_ "github.com/lib/pq"
	"github.com/stellar/go-stellar-sdk/strkey"
	"go.uber.org/zap/zapcore"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccv/bootstrap"
	cmd "github.com/smartcontractkit/chainlink-ccv/cmd/verifier"
	"github.com/smartcontractkit/chainlink-ccv/pkg/chainaccess"
	"github.com/smartcontractkit/chainlink-ccv/verifier/commit"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-stellar/ccv/accessors"
	"github.com/smartcontractkit/chainlink-stellar/ccv/common"
	sourcereader "github.com/smartcontractkit/chainlink-stellar/ccv/source_reader"
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

// hexToContractStrkey converts a 0x-prefixed hex address (32 bytes) to a
// Stellar contract strkey (C...).
func hexToContractStrkey(hexAddr string) (string, error) {
	raw, err := hex.DecodeString(strings.TrimPrefix(hexAddr, "0x"))
	if err != nil {
		return "", fmt.Errorf("decode hex address %q: %w", hexAddr, err)
	}
	return strkey.Encode(strkey.VersionByteContract, raw)
}

func main() {
	if err := bootstrap.Run(
		"StellarCommitteeVerifier",
		cmd.NewServiceFactory(
			chainsel.FamilyStellar,
			func(
				ctx context.Context,
				lggr logger.Logger,
				infos map[string]*sourcereader.ReaderConfig,
				cfg commit.Config,
			) (chainaccess.AccessorFactory, error) {
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
				// deployed, so OnRamp and RMN Remote addresses may be missing.
				// Fill them in from the commit.Config in the job spec, which is
				// generated after contract deployment.
				for sel, rc := range stellarConfig.ReaderConfigs {
					if rc.OnRampContractID == "" {
						if onrampHex, ok := cfg.OnRampAddresses[sel]; ok && onrampHex != "" {
							addr, err := hexToContractStrkey(onrampHex)
							if err != nil {
								return nil, fmt.Errorf("convert OnRamp hex to strkey for chain %s: %w", sel, err)
							}
							rc.OnRampContractID = addr
						}
					}
					if rc.RMNRemoteContractID == "" {
						if rmnHex, ok := cfg.RMNRemoteAddresses[sel]; ok && rmnHex != "" {
							addr, err := hexToContractStrkey(rmnHex)
							if err != nil {
								return nil, fmt.Errorf("convert RMN Remote hex to strkey for chain %s: %w", sel, err)
							}
							rc.RMNRemoteContractID = addr
						}
					}
					stellarConfig.ReaderConfigs[sel] = rc
				}

				return accessors.NewFactory(lggr, stellarConfig.ReaderConfigs), nil
			}),
		bootstrap.WithLogLevel[commit.JobSpec](zapcore.InfoLevel),
	); err != nil {
		panic(fmt.Sprintf("failed to run Stellar committee verifier: %s", err.Error()))
	}
}
