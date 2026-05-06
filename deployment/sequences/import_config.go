package sequences

import (
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
)

// StellarImportConfigForDeployContracts passes through user deploy config (no
// v1.6 import path) for Stellar devenv.
var StellarImportConfigForDeployContracts = cldf_ops.NewSequence(
	"stellar-import-config-for-deploy-chain-contracts",
	SequenceVersion,
	"Stellar devenv: pass through user deploy config (no v1.6 import path)",
	func(b cldf_ops.Bundle, chains cldf_chain.BlockChains, input ccvadapters.DeployChainConfigCreatorInput) (ccvadapters.DeployContractParams, error) {
		_ = b
		_ = chains
		return input.UserProvidedConfig, nil
	},
)
