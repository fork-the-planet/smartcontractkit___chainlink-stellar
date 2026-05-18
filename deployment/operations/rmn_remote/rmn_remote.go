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
	ContractID    string   `json:"contract_id"`
	Owner         string   `json:"owner"`
	ChainSelector uint64   `json:"chain_selector"`
	CurseAdmins   []string `json:"curse_admins,omitempty"`
}

// Initialize calls RMN Remote `initialize`.
var Initialize = cldfops.NewOperation(
	"rmn-remote:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes RMN Remote with owner and chain selector",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := rmnremotebindings.NewRmnRemoteClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner, in.ChainSelector, in.CurseAdmins); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// TransferOwnershipInput starts two-step ownership transfer.
type TransferOwnershipInput struct {
	ContractID string `json:"contract_id"`
	NewOwner   string `json:"new_owner"`
}

// TransferOwnership calls `transfer_ownership` on RMN Remote.
var TransferOwnership = cldfops.NewOperation(
	"rmn-remote:transfer-ownership",
	stellarops.ContractDeploymentVersion,
	"Transfers RMN Remote ownership to a pending new owner",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in TransferOwnershipInput) (stellarops.Void, error) {
		c := rmnremotebindings.NewRmnRemoteClient(d.Invoker, in.ContractID)
		if err := c.TransferOwnership(b.GetContext(), in.NewOwner); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// AcceptOwnershipInput completes two-step ownership transfer for the caller.
type AcceptOwnershipInput struct {
	ContractID string `json:"contract_id"`
}

// AcceptOwnership calls `accept_ownership` on RMN Remote.
var AcceptOwnership = cldfops.NewOperation(
	"rmn-remote:accept-ownership",
	stellarops.ContractDeploymentVersion,
	"Accepts RMN Remote ownership after transfer_ownership",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in AcceptOwnershipInput) (stellarops.Void, error) {
		c := rmnremotebindings.NewRmnRemoteClient(d.Invoker, in.ContractID)
		if err := c.AcceptOwnership(b.GetContext()); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// CurseInput holds the contract ID, caller, and subjects to curse/uncurse.
type CurseInput struct {
	ContractID string     `json:"contract_id"`
	Caller     string     `json:"caller"`
	Subjects   [][16]byte `json:"subjects"`
}

// Curse calls `curse` on RMN Remote with the given subjects.
var Curse = cldfops.NewOperation(
	"rmn-remote:curse",
	stellarops.ContractDeploymentVersion,
	"Curses subjects on Stellar RMN Remote",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in CurseInput) (stellarops.Void, error) {
		c := rmnremotebindings.NewRmnRemoteClient(d.Invoker, in.ContractID)
		if err := c.Curse(b.GetContext(), in.Caller, in.Subjects); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// Uncurse calls `uncurse` on RMN Remote with the given subjects.
var Uncurse = cldfops.NewOperation(
	"rmn-remote:uncurse",
	stellarops.ContractDeploymentVersion,
	"Uncurses subjects on Stellar RMN Remote",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in CurseInput) (stellarops.Void, error) {
		c := rmnremotebindings.NewRmnRemoteClient(d.Invoker, in.ContractID)
		if err := c.Uncurse(b.GetContext(), in.Subjects); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
