package token_lock_box

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	tlbbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/token_lock_box"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels token lock box contracts.
const ContractType = "TokenLockBox"

// Deploy uploads pools_token_lock_box.wasm.
var Deploy = stellarops.NewDeployOperation("token-lock-box:deploy", "Deploys the token lock box Soroban contract from WASM")

// InitializeInput configures lock box owner and token.
type InitializeInput struct {
	ContractID string `json:"contract_id"`
	Owner      string `json:"owner"`
	Token      string `json:"token"`
}

// Initialize calls token lock box `initialize`.
var Initialize = cldfops.NewOperation(
	"token-lock-box:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes token lock box with owner and token",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := tlbbindings.NewTokenLockBoxClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner, in.Token); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
