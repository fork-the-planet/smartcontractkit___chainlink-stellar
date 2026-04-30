package rmn_proxy

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	rmnproxybindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_proxy"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels RMN Proxy.
const ContractType = "RmnProxy"

// Deploy uploads rmn_proxy.wasm.
var Deploy = stellarops.NewDeployOperation("rmn-proxy:deploy", "Deploys the RMN Proxy Soroban contract from WASM")

// InitializeInput wires RMN Proxy to RMN Remote.
type InitializeInput struct {
	ContractID string `json:"contract_id"`
	Owner      string `json:"owner"`
	RmnRemote  string `json:"rmn_remote"`
}

// Initialize calls RMN Proxy `initialize`.
var Initialize = cldfops.NewOperation(
	"rmn-proxy:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes RMN Proxy with owner and RMN Remote address",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := rmnproxybindings.NewRmnProxyClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner, in.RmnRemote); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
