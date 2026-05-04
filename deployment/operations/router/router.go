package router

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels Router in datastore-style tooling.
const ContractType = "Router"

// Deploy uploads router.wasm.
var Deploy = stellarops.NewDeployOperation("router:deploy", "Deploys the Router Soroban contract from WASM")

// InitializeInput wires owner and RMN proxy to Router.
type InitializeInput struct {
	ContractID string `json:"contract_id"`
	Owner      string `json:"owner"`
	RmnProxy   string `json:"rmn_proxy"`
}

// Initialize calls Router `initialize`.
var Initialize = cldfops.NewOperation(
	"router:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes Router with owner and RMN proxy",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := routerbindings.NewRouterClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner, in.RmnProxy); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// ApplyRampUpdatesInput updates on-ramp and off-ramp routing tables.
type ApplyRampUpdatesInput struct {
	ContractID     string                        `json:"contract_id"`
	OnRampUpdates  []routerbindings.OnRampEntry  `json:"on_ramp_updates"`
	OffRampRemoves []routerbindings.OffRampEntry `json:"off_ramp_removes"`
	OffRampAdds    []routerbindings.OffRampEntry `json:"off_ramp_adds"`
}

// ApplyRampUpdates calls Router `apply_ramp_updates`.
var ApplyRampUpdates = cldfops.NewOperation(
	"router:apply-ramp-updates",
	stellarops.ContractDeploymentVersion,
	"Applies Router on-ramp and off-ramp map updates",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in ApplyRampUpdatesInput) (stellarops.Void, error) {
		c := routerbindings.NewRouterClient(d.Invoker, in.ContractID)
		if err := c.ApplyRampUpdates(b.GetContext(), in.OnRampUpdates, in.OffRampRemoves, in.OffRampAdds); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
