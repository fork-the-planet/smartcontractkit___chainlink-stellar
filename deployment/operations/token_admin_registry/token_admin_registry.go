package token_admin_registry

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	tarbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/token_admin_registry"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels TokenAdminRegistry.
const ContractType = "TokenAdminRegistry"

// Deploy uploads token_admin_registry.wasm.
var Deploy = stellarops.NewDeployOperation("token-admin-registry:deploy", "Deploys the TokenAdminRegistry Soroban contract from WASM")

// InitializeInput sets registry owner.
type InitializeInput struct {
	ContractID string `json:"contract_id"`
	Owner      string `json:"owner"`
}

// Initialize calls TokenAdminRegistry `initialize`.
var Initialize = cldfops.NewOperation(
	"token-admin-registry:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes TokenAdminRegistry with owner",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := tarbindings.NewTokenAdminRegistryClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// ProposeAdministratorInput proposes a token administrator.
type ProposeAdministratorInput struct {
	ContractID    string `json:"contract_id"`
	Caller        string `json:"caller"`
	LocalToken    string `json:"local_token"`
	Administrator string `json:"administrator"`
}

// ProposeAdministrator calls `propose_administrator`.
var ProposeAdministrator = cldfops.NewOperation(
	"token-admin-registry:propose-administrator",
	stellarops.ContractDeploymentVersion,
	"Proposes a token administrator in TokenAdminRegistry",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in ProposeAdministratorInput) (stellarops.Void, error) {
		c := tarbindings.NewTokenAdminRegistryClient(d.Invoker, in.ContractID)
		if err := c.ProposeAdministrator(b.GetContext(), in.Caller, in.LocalToken, in.Administrator); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// AcceptAdminRoleInput accepts the proposed admin role for a token.
type AcceptAdminRoleInput struct {
	ContractID string `json:"contract_id"`
	LocalToken string `json:"local_token"`
}

// AcceptAdminRole calls `accept_admin_role`.
var AcceptAdminRole = cldfops.NewOperation(
	"token-admin-registry:accept-admin-role",
	stellarops.ContractDeploymentVersion,
	"Accepts administrator role for a token in TokenAdminRegistry",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in AcceptAdminRoleInput) (stellarops.Void, error) {
		c := tarbindings.NewTokenAdminRegistryClient(d.Invoker, in.ContractID)
		if err := c.AcceptAdminRole(b.GetContext(), in.LocalToken); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// SetPoolInput registers the default pool for a token.
type SetPoolInput struct {
	ContractID string  `json:"contract_id"`
	LocalToken string  `json:"local_token"`
	Pool       *string `json:"pool,omitempty"`
}

// SetPool calls `set_pool`.
var SetPool = cldfops.NewOperation(
	"token-admin-registry:set-pool",
	stellarops.ContractDeploymentVersion,
	"Sets the default pool for a token in TokenAdminRegistry",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in SetPoolInput) (stellarops.Void, error) {
		c := tarbindings.NewTokenAdminRegistryClient(d.Invoker, in.ContractID)
		if err := c.SetPool(b.GetContext(), in.LocalToken, in.Pool); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
