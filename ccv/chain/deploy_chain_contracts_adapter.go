package ccvchain

import (
	"fmt"
	"sync"

	"github.com/Masterminds/semver/v3"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	seq_core "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v1_7_0/adapters"
	ccipOffchain "github.com/smartcontractkit/chainlink-ccip/deployment/v1_7_0/offchain"
)

// StellarDeployChainContractsAdapter connects Stellar to the shared
// deployment/v1_7_0 DeployChainContracts changeset.
type StellarDeployChainContractsAdapter struct{}

var _ ccvadapters.DeployChainContractsAdapter = (*StellarDeployChainContractsAdapter)(nil)

type stellarDeployChangesetCtx struct {
	chain    *Chain
	topology *ccipOffchain.EnvironmentTopology
}

var stellarDeployCtxBySelector sync.Map // uint64 -> *stellarDeployChangesetCtx

func registerStellarDeployChangesetCtx(selector uint64, c *Chain, topology *ccipOffchain.EnvironmentTopology) {
	stellarDeployCtxBySelector.Store(selector, &stellarDeployChangesetCtx{chain: c, topology: topology})
}

func clearStellarDeployChangesetCtx(selector uint64) {
	stellarDeployCtxBySelector.Delete(selector)
}

var stellarImportConfigForDeployContracts = cldf_ops.NewSequence(
	"stellar-import-config-for-deploy-chain-contracts",
	semver.MustParse("2.0.0"),
	"Stellar devenv: pass through user deploy config (no v1.6 import path)",
	func(b cldf_ops.Bundle, chains cldf_chain.BlockChains, input ccvadapters.DeployChainConfigCreatorInput) (ccvadapters.DeployContractParams, error) {
		_ = b
		_ = chains
		return input.UserProvidedConfig, nil
	},
)

var stellarDeployChainContractsSeq = cldf_ops.NewSequence(
	"stellar-deploy-chain-contracts",
	semver.MustParse("2.0.0"),
	"Deploys Stellar Soroban CCIP contracts for devenv",
	func(b cldf_ops.Bundle, chains cldf_chain.BlockChains, input ccvadapters.DeployChainContractsInput) (seq_core.OnChainOutput, error) {
		v, ok := stellarDeployCtxBySelector.Load(input.ChainSelector)
		if !ok {
			return seq_core.OnChainOutput{}, fmt.Errorf("stellar deploy context missing for selector %d", input.ChainSelector)
		}
		ctx := v.(*stellarDeployChangesetCtx)
		return ctx.chain.DeployStellarCCIPContracts(b.GetContext(), chains, input.ChainSelector, ctx.topology)
	},
)

// SetContractParamsFromImportedConfig implements ccvadapters.DeployChainContractsAdapter.
func (a *StellarDeployChainContractsAdapter) SetContractParamsFromImportedConfig() *cldf_ops.Sequence[ccvadapters.DeployChainConfigCreatorInput, ccvadapters.DeployContractParams, cldf_chain.BlockChains] {
	return stellarImportConfigForDeployContracts
}

// DeployChainContracts implements ccvadapters.DeployChainContractsAdapter.
func (a *StellarDeployChainContractsAdapter) DeployChainContracts() *cldf_ops.Sequence[ccvadapters.DeployChainContractsInput, seq_core.OnChainOutput, cldf_chain.BlockChains] {
	return stellarDeployChainContractsSeq
}
