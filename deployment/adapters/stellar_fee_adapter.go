package adapters

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	fqopstype "github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/fee_quoter"
	"github.com/smartcontractkit/chainlink-ccip/deployment/fees"
	"github.com/smartcontractkit/chainlink-ccip/deployment/lanes"
	datastore_utils "github.com/smartcontractkit/chainlink-ccip/deployment/utils/datastore"
	seqcore "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	stellarsequences "github.com/smartcontractkit/chainlink-stellar/deployment/sequences"
)

var _ fees.FeeAdapter = (*StellarFeeAdapter)(nil)

// StellarFeeAdapter implements fees.FeeAdapter for the Stellar FeeQuoter.
type StellarFeeAdapter struct{}

func (a *StellarFeeAdapter) GetFeeContractRef(e cldf.Environment, src uint64, _ uint64) (datastore.AddressRef, error) {
	toRef := func(ref datastore.AddressRef) (datastore.AddressRef, error) { return ref, nil }
	return datastore_utils.FindAndFormatRef(e.DataStore, datastore.AddressRef{
		Type:    datastore.ContractType(fqopstype.ContractType),
		Version: semver.MustParse(fqopstype.Deploy.Version()),
	}, src, toRef)
}

func (a *StellarFeeAdapter) SetTokenTransferFee(e cldf.Environment) *cldf_ops.Sequence[fees.SetTokenTransferFeeSequenceInput, seqcore.OnChainOutput, cldf_chain.BlockChains] {
	return cldf_ops.NewSequence(
		stellarsequences.StellarSetTokenTransferFee.ID(),
		stellarops.ContractDeploymentVersion,
		stellarsequences.StellarSetTokenTransferFee.Description(),
		func(b cldf_ops.Bundle, chains cldf_chain.BlockChains, in fees.SetTokenTransferFeeSequenceInput) (seqcore.OnChainOutput, error) {
			fqRef, err := a.GetFeeContractRef(e, in.Selector, 0)
			if err != nil {
				return seqcore.OnChainOutput{}, fmt.Errorf("resolve FeeQuoter for chain %d: %w", in.Selector, err)
			}
			report, err := cldf_ops.ExecuteSequence(b, stellarsequences.StellarSetTokenTransferFee, chains, stellarsequences.StellarSetTokenTransferFeeInput{
				SetTokenTransferFeeSequenceInput: in,
				FQContractID:                     fqRef.Address,
			})
			if err != nil {
				return seqcore.OnChainOutput{}, err
			}
			return report.Output, nil
		},
	)
}

func (a *StellarFeeAdapter) GetOnchainTokenTransferFeeConfig(_ cldf.Environment, _ uint64, _ uint64, _ string) (fees.TokenTransferFeeArgs, error) {
	return fees.TokenTransferFeeArgs{}, fmt.Errorf("stellar GetOnchainTokenTransferFeeConfig: not yet implemented (requires live chain query)")
}

func (a *StellarFeeAdapter) GetDefaultTokenTransferFeeConfig(_, _ uint64) fees.TokenTransferFeeArgs {
	return fees.TokenTransferFeeArgs{
		MinFeeUSDCents:    50,
		MaxFeeUSDCents:    500,
		DestBytesOverhead: 0,
		DestGasOverhead:   50_000,
		DeciBps:           0,
		IsEnabled:         true,
	}
}

func (a *StellarFeeAdapter) ApplyDestChainConfigUpdates(e cldf.Environment) *cldf_ops.Sequence[fees.ApplyDestChainConfigSequenceInput, seqcore.OnChainOutput, cldf_chain.BlockChains] {
	return cldf_ops.NewSequence(
		stellarsequences.StellarApplyDestChainConfig.ID(),
		stellarops.ContractDeploymentVersion,
		stellarsequences.StellarApplyDestChainConfig.Description(),
		func(b cldf_ops.Bundle, chains cldf_chain.BlockChains, in fees.ApplyDestChainConfigSequenceInput) (seqcore.OnChainOutput, error) {
			fqRef, err := a.GetFeeContractRef(e, in.Selector, 0)
			if err != nil {
				return seqcore.OnChainOutput{}, fmt.Errorf("resolve FeeQuoter for chain %d: %w", in.Selector, err)
			}
			report, err := cldf_ops.ExecuteSequence(b, stellarsequences.StellarApplyDestChainConfig, chains, stellarsequences.StellarApplyDestChainConfigInput{
				ApplyDestChainConfigSequenceInput: in,
				FQContractID:                      fqRef.Address,
			})
			if err != nil {
				return seqcore.OnChainOutput{}, err
			}
			return report.Output, nil
		},
	)
}

func (a *StellarFeeAdapter) GetOnchainDestChainConfig(_ cldf.Environment, _ uint64, _ uint64) (lanes.FeeQuoterDestChainConfig, error) {
	return lanes.FeeQuoterDestChainConfig{}, fmt.Errorf("stellar GetOnchainDestChainConfig: not yet implemented (requires live chain query)")
}

func (a *StellarFeeAdapter) GetDefaultDestChainConfig(_, _ uint64) lanes.FeeQuoterDestChainConfig {
	return (&StellarChainFamilyAdapter{}).GetFeeQuoterDestChainConfig()
}
