package sequences

import (
	"fmt"

	seq_core "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// DeployStellarCCIPInnerInput is the input for the inner full-stack deploy sequence.
// AllSelectors must list every chain selector in the environment (from the outer BlockChains map);
// it is stored as a plain slice so CLDF can persist sequence inputs (BlockChains is not serializable).
type DeployStellarCCIPInnerInput struct {
	ChainSelector     uint64
	AllSelectors      []uint64
	ExistingAddresses []datastore.AddressRef
}

// DeployStellarCCIPInner runs the full Stellar CCIP devenv deploy in one inner sequence
// with DEP=stellardeps.StellarDeps (EVM-style: one inner sequence, same role as evm.Chain).
var DeployStellarCCIPInner = cldf_ops.NewSequence(
	"deploy-stellar-ccip-inner",
	SequenceVersion,
	"Deploys the full Stellar CCIP Soroban stack (inner sequence; DEP=stellardeps.StellarDeps).",
	func(b cldf_ops.Bundle, deps stellardeps.StellarDeps, in DeployStellarCCIPInnerInput) (seq_core.OnChainOutput, error) {
		if deps.Deploy == nil || deps.Invoker == nil {
			return seq_core.OnChainOutput{}, fmt.Errorf("deploy-stellar-ccip-inner: incomplete StellarDeps (nil deploy or invoker)")
		}
		runner, topology, err := takeStellarDeployChainContext(in.ChainSelector)
		if err != nil {
			return seq_core.OnChainOutput{}, err
		}
		return runner.DeployStellarCCIPContracts(b.GetContext(), b, in.AllSelectors, in.ChainSelector, topology, in.ExistingAddresses)
	},
)

// StellarDeployChainContracts is the outer adapter-facing sequence: DEP is
// cldf_chain.BlockChains. It validates the Stellar chain entry, resolves
// StellarDeps from the registered runner, and ExecuteSequence on the inner deploy.
var StellarDeployChainContracts = cldf_ops.NewSequence(
	"stellar-deploy-chain-contracts",
	SequenceVersion,
	"Deploys Stellar Soroban CCIP contracts for devenv (outer sequence; validates BlockChains, runs inner sequence).",
	func(b cldf_ops.Bundle, chains cldf_chain.BlockChains, input ccvadapters.DeployChainContractsInput) (seq_core.OnChainOutput, error) {
		if _, ok := chains.StellarChains()[input.ChainSelector]; !ok {
			return seq_core.OnChainOutput{}, fmt.Errorf("stellar chain not found for selector %d", input.ChainSelector)
		}
		runner, _, err := takeStellarDeployChainContext(input.ChainSelector)
		if err != nil {
			return seq_core.OnChainOutput{}, err
		}
		deps := runner.StellarDepsForDeploy()
		report, err := cldf_ops.ExecuteSequence(b, DeployChainContractsInner, deps, DeployChainContractsInnerInput{
			ChainSelector:     input.ChainSelector,
			AllSelectors:      allSelectorsFromBlockChains(chains),
			ExistingAddresses: input.ExistingAddresses,
		})
		if err != nil {
			return seq_core.OnChainOutput{}, fmt.Errorf("stellar deploy inner sequence: %w", err)
		}
		return report.Output, nil
	},
)
