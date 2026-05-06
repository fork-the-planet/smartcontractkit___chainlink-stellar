package sequences

import (
	"encoding/json"
	"fmt"

	cldfchain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	evmcontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations/contract"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/chainlink-ccip/deployment/deploy"
	seqcore "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"

	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/smartcontractkit/chainlink-stellar/deployment/mcmsutil"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ownership"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// StellarTransferOwnershipInput extends the shared input with pre-resolved addresses.
type StellarTransferOwnershipInput struct {
	deploy.TransferOwnershipPerChainInput
	GovernanceAddr string
	MCMSAddr       string
}

func stellarMCMSTxAdditionalFields() json.RawMessage {
	return json.RawMessage(`{"version":1,"family":"stellar"}`)
}

// StellarTransferOwnershipViaMCMS transfers contract ownership via deployer direct call
// or builds an MCMS batch operation when the current owner is the MCMS contract.
var StellarTransferOwnershipViaMCMS = cldfops.NewSequence(
	"stellar-seq-transfer-ownership-via-mcms",
	deploy.MCMSVersion,
	"Transfers Soroban contract ownership via MCMS or deployer",
	func(b cldfops.Bundle, chains cldfchain.BlockChains, in StellarTransferOwnershipInput) (output seqcore.OnChainOutput, err error) {
		ch, ok := chains.StellarChains()[in.ChainSelector]
		if !ok {
			return output, fmt.Errorf("stellar chain %d not found in environment", in.ChainSelector)
		}
		dep, err := stellardeployment.NewDeployerFromChain(ch)
		if err != nil {
			return output, err
		}
		deps := stellardeps.FromDeployer(dep)
		deployerAddr := dep.SignerAddress()
		ctx := b.GetContext()

		for _, ref := range in.ContractRef {
			owner, err := ownership.ContractOwner(ctx, deps, ref)
			if err != nil {
				return output, fmt.Errorf("read owner %s: %w", ref.Address, err)
			}
			if owner == in.ProposedOwner {
				continue
			}
			var wo evmcontract.WriteOutput
			switch {
			case owner == deployerAddr:
				if err := ownership.ExecuteTransferOwnership(b, deps, ref, in.ProposedOwner); err != nil {
					return output, fmt.Errorf("transfer ownership %s: %w", ref.Address, err)
				}
				wo = evmcontract.WriteOutput{
					ChainSelector: in.ChainSelector,
					ExecInfo:      &evmcontract.ExecInfo{Hash: "stellar-direct-transfer"},
				}
			case owner == in.GovernanceAddr:
				if owner != in.MCMSAddr {
					return output, fmt.Errorf(
						"contract %s is owned by RBACTimelock %q; Soroban timelock-scheduled ownership changes are not implemented (use deployer or MCMS-as-owner legacy layout)",
						ref.Address, owner,
					)
				}
				data, err := mcmsutil.EncodeSorobanMCMSInvokePayload("transfer_ownership", []xdr.ScVal{scval.AddressToScVal(in.ProposedOwner)})
				if err != nil {
					return output, err
				}
				wo = evmcontract.WriteOutput{
					ChainSelector: in.ChainSelector,
					Tx: mcmstypes.Transaction{
						OperationMetadata: mcmstypes.OperationMetadata{
							ContractType: string(ref.Type),
						},
						To:               ref.Address,
						Data:             data,
						AdditionalFields: stellarMCMSTxAdditionalFields(),
					},
				}
			default:
				return output, fmt.Errorf(
					"contract %s owner %q is neither deployer %q nor governance %q",
					ref.Address, owner, deployerAddr, in.GovernanceAddr,
				)
			}
			batchOp, err := evmcontract.NewBatchOperationFromWrites([]evmcontract.WriteOutput{wo})
			if err != nil {
				return output, err
			}
			output.BatchOps = append(output.BatchOps, batchOp)
		}
		return output, nil
	},
)

// StellarAcceptOwnership accepts contract ownership via deployer direct call
// or builds an MCMS batch operation when the proposed owner is the MCMS contract.
var StellarAcceptOwnership = cldfops.NewSequence(
	"stellar-seq-accept-ownership",
	deploy.MCMSVersion,
	"Accepts Soroban contract ownership via MCMS or deployer",
	func(b cldfops.Bundle, chains cldfchain.BlockChains, in StellarTransferOwnershipInput) (output seqcore.OnChainOutput, err error) {
		ch, ok := chains.StellarChains()[in.ChainSelector]
		if !ok {
			return output, fmt.Errorf("stellar chain %d not found in environment", in.ChainSelector)
		}
		dep, err := stellardeployment.NewDeployerFromChain(ch)
		if err != nil {
			return output, err
		}
		deps := stellardeps.FromDeployer(dep)
		deployerAddr := dep.SignerAddress()
		ctx := b.GetContext()

		for _, ref := range in.ContractRef {
			owner, err := ownership.ContractOwner(ctx, deps, ref)
			if err != nil {
				return output, fmt.Errorf("read owner %s: %w", ref.Address, err)
			}
			if owner == in.ProposedOwner {
				continue
			}
			var wo evmcontract.WriteOutput
			switch {
			case in.ProposedOwner == deployerAddr:
				if err := ownership.ExecuteAcceptOwnership(b, deps, ref); err != nil {
					return output, fmt.Errorf("accept ownership %s: %w", ref.Address, err)
				}
				wo = evmcontract.WriteOutput{
					ChainSelector: in.ChainSelector,
					ExecInfo:      &evmcontract.ExecInfo{Hash: "stellar-direct-accept"},
				}
			case in.ProposedOwner == in.GovernanceAddr:
				if in.GovernanceAddr != in.MCMSAddr {
					return output, fmt.Errorf(
						"accept via RBACTimelock %q is not implemented for %s (legacy MCMS-as-governance only)",
						in.GovernanceAddr, ref.Address,
					)
				}
				data, err := mcmsutil.EncodeSorobanMCMSInvokePayload("accept_ownership", nil)
				if err != nil {
					return output, err
				}
				wo = evmcontract.WriteOutput{
					ChainSelector: in.ChainSelector,
					Tx: mcmstypes.Transaction{
						OperationMetadata: mcmstypes.OperationMetadata{
							ContractType: string(ref.Type),
						},
						To:               ref.Address,
						Data:             data,
						AdditionalFields: stellarMCMSTxAdditionalFields(),
					},
				}
			default:
				return output, fmt.Errorf(
					"accept routing: proposed owner %q must be deployer %q or governance %q for %s",
					in.ProposedOwner, deployerAddr, in.GovernanceAddr, ref.Address,
				)
			}
			batchOp, err := evmcontract.NewBatchOperationFromWrites([]evmcontract.WriteOutput{wo})
			if err != nil {
				return output, err
			}
			output.BatchOps = append(output.BatchOps, batchOp)
		}
		return output, nil
	},
)
