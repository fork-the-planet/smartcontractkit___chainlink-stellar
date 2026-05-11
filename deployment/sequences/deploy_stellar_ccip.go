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
// It builds StellarDeps and a [stellarccip.CCIPDevenvHost] from the CLDF Stellar chain entry.
// CCV stashes offchain topology in [RegisterStellarDeployOffchainTopologyForSelector] during
// PreDeployContractsForSelector; this sequence [TakeStellarDeployOffchainTopologyForSelector] before
// [RunStellarCCIPFullDeploy]. A missing stash entry fails fast: signature quorum config requires NOP topology.
var StellarDeployChainContracts = cldf_ops.NewSequence(
	"stellar-deploy-chain-contracts",
	SequenceVersion,
	"Deploys Stellar Soroban CCIP contracts via CLDF Stellar chain (adapter path); uses pre-stashed offchain topology from CCV pre-deploy when present.",
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
		offTopo, _ := TakeStellarDeployOffchainTopologyForSelector(input.ChainSelector)
		if offTopo == nil {
			return seq_core.OnChainOutput{}, fmt.Errorf("stellar deploy chain contracts: offchain topology required for selector %d (CCV PreDeployContractsForSelector must call RegisterStellarDeployOffchainTopologyForSelector before DeployChainContracts so committee verifier signature config can be applied)", input.ChainSelector)
		}
		return RunStellarCCIPFullDeploy(b.GetContext(), b, deps, host, offTopo, DeployStellarCCIPInnerInput{
			ChainSelector:     input.ChainSelector,
			AllSelectors:      allSelectorsFromBlockChains(chains),
			ExistingAddresses: input.ExistingAddresses,
		})
	},
)
