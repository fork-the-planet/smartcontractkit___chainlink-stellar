package token_pool

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	tpoolbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/token_pool"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType is used for generic token pool deployments (e.g. lock-release WASM behind this client).
const ContractType = "TokenPool"

// Deploy uploads pool WASM (e.g. pools_lock_release_pool.wasm).
var Deploy = stellarops.NewDeployOperation("token-pool:deploy", "Deploys a Soroban token pool contract from WASM")

// InitializeInput configures pool owner, token, decimals, router, and ramp registry.
type InitializeInput struct {
	ContractID    string `json:"contract_id"`
	Owner         string `json:"owner"`
	Token         string `json:"token"`
	TokenDecimals uint32 `json:"token_decimals"`
	Router        string `json:"router"`
	RampRegistry  string `json:"ramp_registry"`
}

// Initialize calls token pool `initialize`.
var Initialize = cldfops.NewOperation(
	"token-pool:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes token pool with owner, token, router, and ramp registry",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := tpoolbindings.NewTokenPoolClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner, in.Token, in.TokenDecimals, in.Router, in.RampRegistry); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
