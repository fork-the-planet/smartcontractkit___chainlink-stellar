package sac_token

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TransferInput is the input for a Soroban Asset Contract (SAC) `transfer` call.
type TransferInput struct {
	ContractID string `json:"contract_id"`
	From       string `json:"from"`
	To         string `json:"to"`
	Amount     int64  `json:"amount"`
}

// Transfer calls `transfer` on a SAC token contract (e.g. for funding lock-release pools).
var Transfer = cldfops.NewOperation(
	"sac-token:transfer",
	stellarops.ContractDeploymentVersion,
	"Transfers SAC tokens between Stellar accounts/contracts",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in TransferInput) (stellarops.Void, error) {
		args := []xdr.ScVal{
			scval.AddressToScVal(in.From),
			scval.AddressToScVal(in.To),
			scval.I128ToScVal(in.Amount),
		}
		_, err := d.Invoker.InvokeContract(b.GetContext(), in.ContractID, "transfer", args)
		if err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
