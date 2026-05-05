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

// ApplyOnrampUpdatesInput updates the RampRegistry on-ramp map.
type ApplyOnrampUpdatesInput struct {
	ContractID string                    `json:"contract_id"`
	Updates    []rrbindings.OnRampUpdate `json:"updates"`
}

// ApplyOnrampUpdates calls RampRegistry `apply_onramp_updates`.
var ApplyOnrampUpdates = cldfops.NewOperation(
	"ramp-registry:apply-onramp-updates",
	stellarops.ContractDeploymentVersion,
	"Applies RampRegistry on-ramp map updates",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in ApplyOnrampUpdatesInput) (stellarops.Void, error) {
		c := rrbindings.NewRampRegistryClient(d.Invoker, in.ContractID)
		if err := c.ApplyOnrampUpdates(b.GetContext(), in.Updates); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// ApplyOfframpUpdatesInput updates the RampRegistry off-ramp map.
type ApplyOfframpUpdatesInput struct {
	ContractID string                     `json:"contract_id"`
	Updates    []rrbindings.OffRampUpdate `json:"updates"`
}

// ApplyOfframpUpdates calls RampRegistry `apply_offramp_updates`.
var ApplyOfframpUpdates = cldfops.NewOperation(
	"ramp-registry:apply-offramp-updates",
	stellarops.ContractDeploymentVersion,
	"Applies RampRegistry off-ramp map updates",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in ApplyOfframpUpdatesInput) (stellarops.Void, error) {
		c := rrbindings.NewRampRegistryClient(d.Invoker, in.ContractID)
		if err := c.ApplyOfframpUpdates(b.GetContext(), in.Updates); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
