package siloed_lock_release_pool

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	slrbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/siloed_lock_release_pool"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels siloed lock-release pool contracts.
const ContractType = "SiloedLockReleasePool"

// Deploy uploads pools_siloed_lock_release_pool.wasm.
var Deploy = stellarops.NewDeployOperation("siloed-lock-release-pool:deploy", "Deploys the siloed lock-release pool Soroban contract from WASM")

// InitializeInput matches siloed lock-release pool `initialize`.
type InitializeInput struct {
	ContractID    string `json:"contract_id"`
	Owner         string `json:"owner"`
	Token         string `json:"token"`
	TokenDecimals uint32 `json:"token_decimals"`
	Router        string `json:"router"`
	RampRegistry  string `json:"ramp_registry"`
}

// Initialize calls siloed lock-release pool `initialize`.
var Initialize = cldfops.NewOperation(
	"siloed-lock-release-pool:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes siloed lock-release pool with owner, token, router, and ramp registry",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := slrbindings.NewSiloedLockReleasePoolClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner, in.Token, in.TokenDecimals, in.Router, in.RampRegistry); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
