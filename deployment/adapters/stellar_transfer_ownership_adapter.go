package adapters

import (
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
	cldfchain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	evmcontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations/contract"
	cldfstellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccip/deployment/deploy"
	"github.com/smartcontractkit/chainlink-ccip/deployment/utils/changesets"
	"github.com/smartcontractkit/chainlink-ccip/deployment/utils/mcms"
	seqcore "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/smartcontractkit/chainlink-stellar/deployment/mcmsutil"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ownership"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// StellarTransferOwnershipAdapter builds MCMS batch operations (or executes directly when the
// deployer account is still owner), mirroring the EVM timelock vs deployer split.
type StellarTransferOwnershipAdapter struct {
	governanceAddr map[uint64]string
	mcmsAddr       map[uint64]string
}

var _ deploy.TransferOwnershipAdapter = (*StellarTransferOwnershipAdapter)(nil)

func stellarMCMSTxAdditionalFields() json.RawMessage {
	return json.RawMessage(`{"version":1,"family":"stellar"}`)
}

func (a *StellarTransferOwnershipAdapter) stellarDepsFromChain(ch cldfstellar.Chain) (stellardeps.StellarDeps, string, error) {
	hexKey, err := mcmsutil.StellarDeployerPrivateKeyHex()
	if err != nil {
		return stellardeps.StellarDeps{}, "", err
	}
	kp, err := cldfstellar.KeypairFromHex(hexKey)
	if err != nil {
		return stellardeps.StellarDeps{}, "", fmt.Errorf("STELLAR_DEPLOYER_PRIVATE_KEY: %w", err)
	}
	dep := stellardeployment.NewDeployer(ch.Client, ch.NetworkPassphrase, kp)
	return stellardeps.FromDeployer(dep), kp.Address(), nil
}

// InitializeTimelockAddress caches timelock (or legacy MCMS) and MCMS addresses per chain.
func (a *StellarTransferOwnershipAdapter) InitializeTimelockAddress(e cldf.Environment, input mcms.Input) error {
	reader, ok := changesets.GetRegistry().GetMCMSReader(chainsel.FamilyStellar)
	if !ok {
		return fmt.Errorf("no MCMS reader registered for %s", chainsel.FamilyStellar)
	}
	if a.governanceAddr == nil {
		a.governanceAddr = make(map[uint64]string)
	}
	if a.mcmsAddr == nil {
		a.mcmsAddr = make(map[uint64]string)
	}
	for sel := range e.BlockChains.StellarChains() {
		mcmsRef, err := reader.GetMCMSRef(e, sel, input)
		if err != nil {
			return err
		}
		if mcmsRef.Address == "" {
			return fmt.Errorf("empty MCMS address for stellar chain %d", sel)
		}
		a.mcmsAddr[sel] = mcmsRef.Address

		tlRef, err := reader.GetTimelockRef(e, sel, input)
		if err != nil {
			return err
		}
		if tlRef.Address == "" {
			return fmt.Errorf("empty timelock governance address for stellar chain %d", sel)
		}
		a.governanceAddr[sel] = tlRef.Address
	}
	return nil
}

func (a *StellarTransferOwnershipAdapter) SequenceTransferOwnershipViaMCMS() *cldfops.Sequence[deploy.TransferOwnershipPerChainInput, seqcore.OnChainOutput, cldfchain.BlockChains] {
	return cldfops.NewSequence(
		"stellar-seq-transfer-ownership-via-mcms",
		semver.MustParse("1.0.0"),
		"Transfers Soroban contract ownership via MCMS or deployer",
		func(b cldfops.Bundle, chains cldfchain.BlockChains, in deploy.TransferOwnershipPerChainInput) (output seqcore.OnChainOutput, err error) {
			ch, ok := chains.StellarChains()[in.ChainSelector]
			if !ok {
				return output, fmt.Errorf("stellar chain %d not found in environment", in.ChainSelector)
			}
			deps, deployerAddr, err := a.stellarDepsFromChain(ch)
			if err != nil {
				return output, err
			}
			gov, ok := a.governanceAddr[in.ChainSelector]
			if !ok {
				return output, fmt.Errorf("governance address not initialized for chain %d", in.ChainSelector)
			}
			mcmsContract, ok := a.mcmsAddr[in.ChainSelector]
			if !ok {
				return output, fmt.Errorf("MCMS address not initialized for chain %d", in.ChainSelector)
			}
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
				case owner == gov:
					if owner != mcmsContract {
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
						ref.Address, owner, deployerAddr, gov,
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
}

func (a *StellarTransferOwnershipAdapter) ShouldAcceptOwnershipWithTransferOwnership(e cldf.Environment, in deploy.TransferOwnershipPerChainInput) (bool, error) {
	gov, ok := a.governanceAddr[in.ChainSelector]
	if !ok {
		return false, fmt.Errorf("governance address not initialized for chain %d", in.ChainSelector)
	}
	ch, ok := e.BlockChains.StellarChains()[in.ChainSelector]
	if !ok {
		return false, fmt.Errorf("stellar chain %d not found in environment", in.ChainSelector)
	}
	_, deployerAddr, err := a.stellarDepsFromChain(ch)
	if err != nil {
		return false, err
	}
	if in.ProposedOwner != gov && in.ProposedOwner != deployerAddr {
		return false, nil
	}
	return true, nil
}

func (a *StellarTransferOwnershipAdapter) SequenceAcceptOwnership() *cldfops.Sequence[deploy.TransferOwnershipPerChainInput, seqcore.OnChainOutput, cldfchain.BlockChains] {
	return cldfops.NewSequence(
		"stellar-seq-accept-ownership",
		semver.MustParse("1.0.0"),
		"Accepts Soroban contract ownership via MCMS or deployer",
		func(b cldfops.Bundle, chains cldfchain.BlockChains, in deploy.TransferOwnershipPerChainInput) (output seqcore.OnChainOutput, err error) {
			ch, ok := chains.StellarChains()[in.ChainSelector]
			if !ok {
				return output, fmt.Errorf("stellar chain %d not found in environment", in.ChainSelector)
			}
			deps, deployerAddr, err := a.stellarDepsFromChain(ch)
			if err != nil {
				return output, err
			}
			gov, ok := a.governanceAddr[in.ChainSelector]
			if !ok {
				return output, fmt.Errorf("governance address not initialized for chain %d", in.ChainSelector)
			}
			mcmsContract, ok := a.mcmsAddr[in.ChainSelector]
			if !ok {
				return output, fmt.Errorf("MCMS address not initialized for chain %d", in.ChainSelector)
			}
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
				case in.ProposedOwner == gov:
					if gov != mcmsContract {
						return output, fmt.Errorf(
							"accept via RBACTimelock %q is not implemented for %s (legacy MCMS-as-governance only)",
							gov, ref.Address,
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
						in.ProposedOwner, deployerAddr, gov, ref.Address,
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
}
