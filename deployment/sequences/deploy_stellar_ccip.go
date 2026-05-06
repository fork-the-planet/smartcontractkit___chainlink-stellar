package sequences

import (
	"fmt"

	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	seq_core "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// DeployStellarCCIPInnerInput carries per-chain inputs for the Stellar full-stack deploy sequence.
// AllSelectors must list every chain selector in the environment (from BlockChains).
type DeployStellarCCIPInnerInput struct {
	ChainSelector     uint64
	AllSelectors      []uint64
	ExistingAddresses []datastore.AddressRef
}

// StellarDeployChainContracts is the adapter sequence: DEP is BlockChains.
// It builds StellarDeps and a [stellarccip.CCIPDevenvHost] from the CLDF Stellar chain entry — no sync.Map / PreDeploy runner stash.
var StellarDeployChainContracts = cldf_ops.NewSequence(
	"stellar-deploy-chain-contracts",
	SequenceVersion,
	"Deploys Stellar Soroban CCIP contracts via CLDF Stellar chain (adapter path). Topology is nil: committee signer config uses placeholders unless deploy is run via ccv ([RunStellarCCIPFullDeployForCCV]).",
	func(b cldf_ops.Bundle, chains cldf_chain.BlockChains, input ccvadapters.DeployChainContractsInput) (seq_core.OnChainOutput, error) {
		ch, ok := chains.StellarChains()[input.ChainSelector]
		if !ok {
			return seq_core.OnChainOutput{}, fmt.Errorf("stellar chain not found for selector %d", input.ChainSelector)
		}
		dep, err := stellardeployment.NewDeployerFromChain(ch)
		if err != nil {
			return seq_core.OnChainOutput{}, fmt.Errorf("stellar deployer from chain: %w", err)
		}
		deps := stellardeps.FromDeployer(dep)
		lg := stellarccip.DefaultStellarDeployZerolog()
		host, err := stellarccip.NewCLDFStellarCCIPDevenvHost(ch, lg, dep)
		if err != nil {
			return seq_core.OnChainOutput{}, fmt.Errorf("stellar devenv host: %w", err)
		}
		// DeployChainContractsInput has no topology field on this chainlink-ccip/deployment pin; ccv uses [RunStellarCCIPFullDeployForCCV].
		return RunStellarCCIPFullDeploy(b.GetContext(), b, deps, host, nil, DeployStellarCCIPInnerInput{
			ChainSelector:     input.ChainSelector,
			AllSelectors:      allSelectorsFromBlockChains(chains),
			ExistingAddresses: input.ExistingAddresses,
		})
	},
)
