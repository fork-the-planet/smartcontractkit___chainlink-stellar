package sequences

import (
	"fmt"

	cldfchain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/chainlink-ccip/deployment/fees"
	seqcore "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"

	fqbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/fee_quoter"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	fqops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/fee_quoter"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// StellarSetTokenTransferFeeInput is the sequence input for applying per-token transfer fees.
type StellarSetTokenTransferFeeInput struct {
	fees.SetTokenTransferFeeSequenceInput
	FQContractID string
}

// StellarSetTokenTransferFee applies per-token transfer fee configs on the Stellar FeeQuoter.
var StellarSetTokenTransferFee = cldfops.NewSequence(
	"stellar-set-token-transfer-fee",
	stellarops.ContractDeploymentVersion,
	"Set per-token transfer fees on Stellar FeeQuoter",
	func(b cldfops.Bundle, chains cldfchain.BlockChains, in StellarSetTokenTransferFeeInput) (seqcore.OnChainOutput, error) {
		ch, ok := chains.StellarChains()[in.Selector]
		if !ok {
			return seqcore.OnChainOutput{}, fmt.Errorf("stellar chain %d not found", in.Selector)
		}
		dep, err := stellardeployment.NewDeployerFromChain(ch)
		if err != nil {
			return seqcore.OnChainOutput{}, err
		}
		deps := stellardeps.FromDeployer(dep)

		var cfgArgs []fqbindings.TokenFeeConfigArgs
		for destSel, tokenFees := range in.Settings {
			for token, feeArgs := range tokenFees {
				if feeArgs == nil {
					continue
				}
				cfgArgs = append(cfgArgs, fqbindings.TokenFeeConfigArgs{
					DestChainSelector: destSel,
					Token:             token,
					Config: fqbindings.TokenTransferFeeConfig{
						DestBytesOverhead: feeArgs.DestBytesOverhead,
						DestGasOverhead:   feeArgs.DestGasOverhead,
						FeeUsdCents:       feeArgs.MinFeeUSDCents,
						IsEnabled:         feeArgs.IsEnabled,
					},
				})
			}
		}
		if len(cfgArgs) == 0 {
			return seqcore.OnChainOutput{}, nil
		}
		_, err = cldfops.ExecuteOperation(b, fqops.ApplyTokenFeeConfigs, deps, fqops.ApplyTokenFeeConfigsInput{
			ContractID: in.FQContractID,
			AddConfigs: cfgArgs,
		})
		if err != nil {
			return seqcore.OnChainOutput{}, fmt.Errorf("apply token fee configs on chain %d: %w", in.Selector, err)
		}
		return seqcore.OnChainOutput{}, nil
	},
)

// StellarApplyDestChainConfigInput is the sequence input for applying dest chain configs.
type StellarApplyDestChainConfigInput struct {
	fees.ApplyDestChainConfigSequenceInput
	FQContractID string
}

// StellarApplyDestChainConfig applies destination chain configs on the Stellar FeeQuoter.
var StellarApplyDestChainConfig = cldfops.NewSequence(
	"stellar-apply-dest-chain-config",
	stellarops.ContractDeploymentVersion,
	"Apply dest chain config updates on Stellar FeeQuoter",
	func(b cldfops.Bundle, chains cldfchain.BlockChains, in StellarApplyDestChainConfigInput) (seqcore.OnChainOutput, error) {
		ch, ok := chains.StellarChains()[in.Selector]
		if !ok {
			return seqcore.OnChainOutput{}, fmt.Errorf("stellar chain %d not found", in.Selector)
		}
		dep, err := stellardeployment.NewDeployerFromChain(ch)
		if err != nil {
			return seqcore.OnChainOutput{}, err
		}
		deps := stellardeps.FromDeployer(dep)

		var cfgArgs []fqbindings.DestChainConfigArgs
		for destSel, cfg := range in.Settings {
			cfgArgs = append(cfgArgs, fqbindings.DestChainConfigArgs{
				DestChainSelector: destSel,
				Config: fqbindings.DestChainConfig{
					IsEnabled:             cfg.IsEnabled,
					MaxDataBytes:          cfg.MaxDataBytes,
					MaxPerMsgGasLimit:     cfg.MaxPerMsgGasLimit,
					DestGasOverhead:       cfg.DestGasOverhead,
					DestGasPerPayloadByte: uint32(cfg.DestGasPerPayloadByteBase),
					DefaultTokenFeeUsd:    uint32(cfg.DefaultTokenFeeUSDCents),
					DefaultTokenDestGas:   cfg.DefaultTokenDestGasOverhead,
					DefaultTxGasLimit:     cfg.DefaultTxGasLimit,
					NetworkFeeUsdCents:    uint32(cfg.NetworkFeeUSDCents),
				},
			})
		}
		if len(cfgArgs) == 0 {
			return seqcore.OnChainOutput{}, nil
		}
		_, err = cldfops.ExecuteOperation(b, fqops.ApplyDestChainConfigs, deps, fqops.ApplyDestChainConfigsInput{
			ContractID: in.FQContractID,
			Configs:    cfgArgs,
		})
		if err != nil {
			return seqcore.OnChainOutput{}, fmt.Errorf("apply dest chain configs on chain %d: %w", in.Selector, err)
		}
		return seqcore.OnChainOutput{}, nil
	},
)

// StellarSetFeeAggregatorInput is the sequence input for setting the fee aggregator.
type StellarSetFeeAggregatorInput struct {
	fees.FeeAggregatorForChain
}

// StellarSetFeeAggregator is a no-op placeholder.
// Stellar fee aggregator is configured during contract deployment (PostDeployContractsForSelector).
var StellarSetFeeAggregator = cldfops.NewSequence(
	"stellar-set-fee-aggregator",
	stellarops.ContractDeploymentVersion,
	"No-op: Stellar fee aggregator is configured during contract deployment",
	func(_ cldfops.Bundle, _ cldfchain.BlockChains, _ StellarSetFeeAggregatorInput) (seqcore.OnChainOutput, error) {
		return seqcore.OnChainOutput{}, nil
	},
)
