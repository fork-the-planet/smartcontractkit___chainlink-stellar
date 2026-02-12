package bindings

import (
	"context"

	protocolrpc "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Invoker provides methods to invoke and simulate Soroban contracts.
// This is a common interface that can be implemented by any Stellar
// contract invocation mechanism (e.g., Deployer, RPC client wrapper).
// It is shared across all generated contract bindings.
type Invoker interface {
	InvokeContract(ctx context.Context, contractID string, functionName string, args []xdr.ScVal) (*xdr.ScVal, error)
	SimulateContract(ctx context.Context, contractID string, functionName string, args []xdr.ScVal) (*xdr.ScVal, error)
	GetEvents(ctx context.Context, contractID string, startLedger uint32, topics []string) ([]protocolrpc.EventInfo, error)
}
