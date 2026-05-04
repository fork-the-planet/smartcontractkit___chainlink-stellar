package timelock

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	tlbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/timelock"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels Timelock.
const ContractType = "Timelock"

// Deploy uploads timelock.wasm.
var Deploy = stellarops.NewDeployOperation("timelock:deploy", "Deploys the Timelock Soroban contract from WASM")

// InitializeInput configures timelock roles and minimum delay.
type InitializeInput struct {
	ContractID string   `json:"contract_id"`
	MinDelay   uint64   `json:"min_delay"`
	Admin      string   `json:"admin"`
	Proposers  []string `json:"proposers"`
	Executors  []string `json:"executors"`
	Cancellers []string `json:"cancellers"`
	Bypassers  []string `json:"bypassers"`
}

// Initialize calls Timelock `initialize`.
var Initialize = cldfops.NewOperation(
	"timelock:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes Timelock with delay, admin, and role holders",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := tlbindings.NewTimelockClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.MinDelay, in.Admin, in.Proposers, in.Executors, in.Cancellers, in.Bypassers); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
