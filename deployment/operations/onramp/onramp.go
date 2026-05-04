package onramp

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	onrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/onramp"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels OnRamp.
const ContractType = "OnRamp"

// Deploy uploads onramp.wasm.
var Deploy = stellarops.NewDeployOperation("onramp:deploy", "Deploys the OnRamp Soroban contract from WASM")

// InitializeInput configures static and dynamic OnRamp settings.
type InitializeInput struct {
	ContractID    string                       `json:"contract_id"`
	Owner         string                       `json:"owner"`
	StaticConfig  onrampbindings.StaticConfig  `json:"static_config"`
	DynamicConfig onrampbindings.DynamicConfig `json:"dynamic_config"`
}

// Initialize calls OnRamp `initialize`.
var Initialize = cldfops.NewOperation(
	"onramp:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes OnRamp with owner, static, and dynamic configuration",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := onrampbindings.NewOnRampClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner, in.StaticConfig, in.DynamicConfig); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// ApplyDestChainConfigUpdatesInput applies destination chain configs on OnRamp.
type ApplyDestChainConfigUpdatesInput struct {
	ContractID string                               `json:"contract_id"`
	Updates    []onrampbindings.DestChainConfigArgs `json:"updates"`
}

// ApplyDestChainConfigUpdates calls OnRamp `apply_dest_chain_config_updates`.
var ApplyDestChainConfigUpdates = cldfops.NewOperation(
	"onramp:apply-dest-chain-config-updates",
	stellarops.ContractDeploymentVersion,
	"Applies OnRamp destination chain configuration updates",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in ApplyDestChainConfigUpdatesInput) (stellarops.Void, error) {
		c := onrampbindings.NewOnRampClient(d.Invoker, in.ContractID)
		if err := c.ApplyDestChainConfigUpdates(b.GetContext(), in.Updates); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
