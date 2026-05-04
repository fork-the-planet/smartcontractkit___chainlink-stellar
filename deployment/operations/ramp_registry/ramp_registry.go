package ramp_registry

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	rrbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/ramp_registry"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels RampRegistry.
const ContractType = "RampRegistry"

// Deploy uploads ccip_ramp_registry.wasm.
var Deploy = stellarops.NewDeployOperation("ramp-registry:deploy", "Deploys the RampRegistry Soroban contract from WASM")

// InitializeInput sets RampRegistry owner.
type InitializeInput struct {
	ContractID string `json:"contract_id"`
	Owner      string `json:"owner"`
}

// Initialize calls RampRegistry `initialize`.
var Initialize = cldfops.NewOperation(
	"ramp-registry:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes RampRegistry with owner",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := rrbindings.NewRampRegistryClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// ApplyRampUpdatesInput mirrors Router ramp map updates on the registry.
type ApplyRampUpdatesInput struct {
	ContractID     string                    `json:"contract_id"`
	OnRampUpdates  []rrbindings.OnRampEntry  `json:"on_ramp_updates"`
	OffRampRemoves []rrbindings.OffRampEntry `json:"off_ramp_removes"`
	OffRampAdds    []rrbindings.OffRampEntry `json:"off_ramp_adds"`
}

// ApplyRampUpdates calls RampRegistry `apply_ramp_updates`.
var ApplyRampUpdates = cldfops.NewOperation(
	"ramp-registry:apply-ramp-updates",
	stellarops.ContractDeploymentVersion,
	"Applies RampRegistry on-ramp and off-ramp map updates",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in ApplyRampUpdatesInput) (stellarops.Void, error) {
		c := rrbindings.NewRampRegistryClient(d.Invoker, in.ContractID)
		if err := c.ApplyRampUpdates(b.GetContext(), in.OnRampUpdates, in.OffRampRemoves, in.OffRampAdds); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
