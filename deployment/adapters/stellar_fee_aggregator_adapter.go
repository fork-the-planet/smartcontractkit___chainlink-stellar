package adapters

import (
	"fmt"

	"github.com/smartcontractkit/chainlink-ccip/deployment/fees"
	seqcore "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	stellarsequences "github.com/smartcontractkit/chainlink-stellar/deployment/sequences"
)

var _ fees.FeeAggregatorAdapter = (*StellarFeeAggregatorAdapter)(nil)

// StellarFeeAggregatorAdapter implements fees.FeeAggregatorAdapter.
// On Stellar, the fee aggregator address is configured during contract deployment
// (via FeeQuoter initialize or post-deploy), so SetFeeAggregator delegates to
// a no-op sequence and GetFeeAggregator is not yet implemented.
type StellarFeeAggregatorAdapter struct{}

func (a *StellarFeeAggregatorAdapter) SetFeeAggregator(_ cldf.Environment) *cldf_ops.Sequence[fees.FeeAggregatorForChain, seqcore.OnChainOutput, cldf_chain.BlockChains] {
	return cldf_ops.NewSequence(
		stellarsequences.StellarSetFeeAggregator.ID(),
		stellarops.ContractDeploymentVersion,
		stellarsequences.StellarSetFeeAggregator.Description(),
		func(b cldf_ops.Bundle, chains cldf_chain.BlockChains, in fees.FeeAggregatorForChain) (seqcore.OnChainOutput, error) {
			report, err := cldf_ops.ExecuteSequence(b, stellarsequences.StellarSetFeeAggregator, chains, stellarsequences.StellarSetFeeAggregatorInput{
				FeeAggregatorForChain: in,
			})
			if err != nil {
				return seqcore.OnChainOutput{}, err
			}
			return report.Output, nil
		},
	)
}

func (a *StellarFeeAggregatorAdapter) GetFeeAggregator(_ cldf.Environment, chainSelector uint64) (string, error) {
	return "", fmt.Errorf("stellar fee aggregator read not implemented for chain %d (configured during deploy)", chainSelector)
}
