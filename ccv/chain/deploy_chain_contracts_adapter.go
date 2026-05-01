package ccvchain

import (
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	seq_core "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	stellarsequences "github.com/smartcontractkit/chainlink-stellar/deployment/sequences"
)

// StellarDeployChainContractsAdapter connects Stellar to the shared
// deployment/v2_0_0 DeployChainContracts changeset.
type StellarDeployChainContractsAdapter struct{}

var _ ccvadapters.DeployChainContractsAdapter = (*StellarDeployChainContractsAdapter)(nil)

// SetContractParamsFromImportedConfig implements ccvadapters.DeployChainContractsAdapter.
func (a *StellarDeployChainContractsAdapter) SetContractParamsFromImportedConfig() *cldf_ops.Sequence[ccvadapters.DeployChainConfigCreatorInput, ccvadapters.DeployContractParams, cldf_chain.BlockChains] {
	return stellarsequences.StellarImportConfigForDeployContracts
}

// DeployChainContracts implements ccvadapters.DeployChainContractsAdapter.
func (a *StellarDeployChainContractsAdapter) DeployChainContracts() *cldf_ops.Sequence[ccvadapters.DeployChainContractsInput, seq_core.OnChainOutput, cldf_chain.BlockChains] {
	return stellarsequences.StellarDeployChainContracts
}
