package devenv

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/testcontainers/testcontainers-go"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/committeeverifier"
	"github.com/smartcontractkit/chainlink-ccv/verifier/commit"
	"github.com/smartcontractkit/chainlink-testing-framework/framework/components/blockchain"
	"github.com/stellar/go-stellar-sdk/strkey"

	"github.com/smartcontractkit/chainlink-stellar/ccv/common"
	sourcereader "github.com/smartcontractkit/chainlink-stellar/ccv/source_reader"
)

const defaultStellarVerifierImage = "stellarcommittee-verifier:dev"

// StellarModifier is a committeeverifier.ReqModifier that configures the container
// request for the Stellar committee verifier:
//  1. Switches the image from the default EVM verifier to stellarcommittee-verifier:dev.
//  2. Builds a stellar.toml from real deployed addresses (via verifierInput.GeneratedConfig)
//     and Stellar network info (passphrase + internal RPC URL from blockchain outputs), then
//     bind-mounts it at DefaultStellarConfigPath so the binary reads it on startup.
func StellarModifier(req testcontainers.ContainerRequest, verifierInput *committeeverifier.Input, outputs []*blockchain.Output) (testcontainers.ContainerRequest, error) {
	req.Image = defaultStellarVerifierImage
	req.Name = fmt.Sprintf("stellar-%s", verifierInput.ContainerName)

	configBytes, err := buildStellarConfig(verifierInput, outputs)
	if err != nil {
		return req, fmt.Errorf("building stellar config for %s: %w", verifierInput.ContainerName, err)
	}

	configFilePath := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("stellar-%s-config-%d.toml", verifierInput.CommitteeName, verifierInput.NodeIndex+1),
	)
	if err := os.WriteFile(configFilePath, configBytes, 0o644); err != nil {
		return req, fmt.Errorf("writing stellar config for %s: %w", verifierInput.ContainerName, err)
	}

	//nolint:staticcheck
	req.Mounts = append(req.Mounts, testcontainers.BindMount(configFilePath, common.DefaultStellarConfigPath))

	return req, nil
}

// buildStellarConfig constructs a common.Config and serialises it as TOML.
//
// For each Stellar chain in outputs:
//   - Network passphrase: output.NetworkSpecificData.StellarNetwork.NetworkPassphrase
//   - Soroban RPC URL: output.Nodes[0].InternalHTTPUrl (Docker-internal, reachable from the container)
//   - OnRamp and RMN Remote addresses: verifierInput.GeneratedConfig (commit.Config),
//     which contains the real deployed addresses from the CLDF datastore, stored as
//     0x-prefixed hex; we convert them to Stellar contract strkeys (C...) here.
func buildStellarConfig(verifierInput *committeeverifier.Input, outputs []*blockchain.Output) ([]byte, error) {
	var deployedCfg commit.Config
	if verifierInput.GeneratedConfig != "" {
		if _, err := toml.Decode(verifierInput.GeneratedConfig, &deployedCfg); err != nil {
			return nil, fmt.Errorf("parse GeneratedConfig: %w", err)
		}
	}

	readerConfigs := make(map[string]sourcereader.ReaderConfig)

	for _, output := range outputs {
		if output.Family != chainsel.FamilyStellar {
			continue
		}

		details, err := chainsel.GetChainDetailsByChainIDAndFamily(output.ChainID, output.Family)
		if err != nil {
			return nil, fmt.Errorf("get chain details for Stellar chain %s: %w", output.ChainID, err)
		}
		strSelector := strconv.FormatUint(details.ChainSelector, 10)

		if output.NetworkSpecificData == nil || output.NetworkSpecificData.StellarNetwork == nil {
			return nil, fmt.Errorf("missing Stellar network info in output for chain %s", output.ChainID)
		}
		networkPassphrase := output.NetworkSpecificData.StellarNetwork.NetworkPassphrase

		if len(output.Nodes) == 0 {
			return nil, fmt.Errorf("no nodes in output for Stellar chain %s", output.ChainID)
		}
		sorobanRPCURL := output.Nodes[0].InternalHTTPUrl

		var onrampContractID string
		onrampHex, ok := deployedCfg.OnRampAddresses[strSelector]
		if !ok {
			// return nil, fmt.Errorf("no deployed OnRamp address for Stellar chain %s in GeneratedConfig", strSelector)
			// TODO: should we throw an error here?
		} else {
			onrampContractID, err = hexToContractStrkey(onrampHex)
			if err != nil {
				return nil, fmt.Errorf("convert OnRamp hex to strkey for chain %s: %w", strSelector, err)
			}
		}

		var rmnRemoteContractID string
		if rmnRemoteHex, ok := deployedCfg.RMNRemoteAddresses[strSelector]; ok {
			rmnRemoteContractID, err = hexToContractStrkey(rmnRemoteHex)
			if err != nil {
				return nil, fmt.Errorf("convert RMN Remote hex to strkey for chain %s: %w", strSelector, err)
			}
		}

		readerConfigs[strSelector] = sourcereader.ReaderConfig{
			NetworkPassphrase:   networkPassphrase,
			SorobanRPCURL:       sorobanRPCURL,
			OnRampContractID:    onrampContractID,
			RMNRemoteContractID: rmnRemoteContractID,
		}
	}

	cfg := common.Config{ReaderConfigs: readerConfigs}
	configBytes, err := toml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal stellar config: %w", err)
	}

	return configBytes, nil
}

// hexToContractStrkey converts a 0x-prefixed hex address (32 bytes) to a Stellar
// contract strkey (C...).
func hexToContractStrkey(hexAddr string) (string, error) {
	raw, err := hex.DecodeString(strings.TrimPrefix(hexAddr, "0x"))
	if err != nil {
		return "", fmt.Errorf("decode hex address %q: %w", hexAddr, err)
	}
	return strkey.Encode(strkey.VersionByteContract, raw)
}
