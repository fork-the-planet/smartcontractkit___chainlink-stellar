package rmn_remote

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	rmnremotebindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_remote"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels RMN Remote.
const ContractType = "RmnRemote"

// Deploy uploads rmn_remote.wasm.
var Deploy = stellarops.NewDeployOperation("rmn-remote:deploy", "Deploys the RMN Remote Soroban contract from WASM")

// InitializeInput configures RMN Remote owner and local chain selector.
type InitializeInput struct {
	ContractID    string `json:"contract_id"`
	Owner         string `json:"owner"`
	ChainSelector uint64 `json:"chain_selector"`
}

// Initialize calls RMN Remote `initialize`.
var Initialize = cldfops.NewOperation(
	"rmn-remote:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes RMN Remote with owner and chain selector",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := rmnremotebindings.NewRmnRemoteClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner, in.ChainSelector); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
