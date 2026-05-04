package ccip_receiver

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	recvbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/ccip_receiver"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels the example CCIP receiver contract.
const ContractType = "CCIPReceiver"

// Deploy uploads ccip_receiver_example.wasm.
var Deploy = stellarops.NewDeployOperation("ccip-receiver:deploy", "Deploys the example CCIP receiver Soroban contract from WASM")

// InitializeInput wires owner and router on the receiver.
type InitializeInput struct {
	ContractID string `json:"contract_id"`
	Owner      string `json:"owner"`
	Router     string `json:"router"`
}

// Initialize calls example receiver `initialize`.
var Initialize = cldfops.NewOperation(
	"ccip-receiver:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes CCIP example receiver with owner and router",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := recvbindings.NewExampleCcipReceiverClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner, in.Router); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// EnableRemoteChainInput enables inbound CCIP receive from a remote chain selector.
type EnableRemoteChainInput struct {
	ContractID            string `json:"contract_id"`
	Caller                string `json:"caller"`
	RemoteChainSelector   uint64 `json:"remote_chain_selector"`
	ExtraArgs             []byte `json:"extra_args"`
	AllowedFinalityConfig uint32 `json:"allowed_finality_config"`
}

// EnableRemoteChain calls `enable_remote_chain`.
var EnableRemoteChain = cldfops.NewOperation(
	"ccip-receiver:enable-remote-chain",
	stellarops.ContractDeploymentVersion,
	"Enables CCIP receiver processing for a remote chain selector",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in EnableRemoteChainInput) (stellarops.Void, error) {
		c := recvbindings.NewExampleCcipReceiverClient(d.Invoker, in.ContractID)
		if err := c.EnableRemoteChain(b.GetContext(), in.Caller, in.RemoteChainSelector, in.ExtraArgs, in.AllowedFinalityConfig); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
