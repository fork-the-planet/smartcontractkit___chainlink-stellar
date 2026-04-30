package mcms

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmsbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/mcms"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels MCMS.
const ContractType = "MCMS"

// Deploy uploads mcms.wasm.
var Deploy = stellarops.NewDeployOperation("mcms:deploy", "Deploys the MCMS Soroban contract from WASM")

// InitializeInput configures MCMS owner and chain network id.
type InitializeInput struct {
	ContractID     string   `json:"contract_id"`
	Owner          string   `json:"owner"`
	ChainNetworkID [32]byte `json:"chain_network_id"`
}

// Initialize calls MCMS `initialize`.
var Initialize = cldfops.NewOperation(
	"mcms:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes MCMS with owner and chain network id",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := mcmsbindings.NewMcmsClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner, in.ChainNetworkID); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
