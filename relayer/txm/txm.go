package txm

import (
	"context"

	protocolrpc "github.com/stellar/go-stellar-sdk/protocols/rpc"

	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
)

// RPCClient is the subset of the Stellar Soroban JSON-RPC client used by the TXM.
// Any value satisfying chain.RPCClient (a superset) automatically satisfies this.
type RPCClient interface {
	SimulateTransaction(ctx context.Context, req protocolrpc.SimulateTransactionRequest) (protocolrpc.SimulateTransactionResponse, error)
	SendTransaction(ctx context.Context, req protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error)
	GetTransaction(ctx context.Context, req protocolrpc.GetTransactionRequest) (protocolrpc.GetTransactionResponse, error)
	GetLedgerEntries(ctx context.Context, req protocolrpc.GetLedgerEntriesRequest) (protocolrpc.GetLedgerEntriesResponse, error)
	GetLatestLedger(ctx context.Context) (protocolrpc.GetLatestLedgerResponse, error)
	GetFeeStats(ctx context.Context) (protocolrpc.GetFeeStatsResponse, error)
}

// StellarTxm is a placeholder for the Stellar transaction manager. The full
// implementation (enqueue, broadcast, confirm, retry) lands in the follow-up
// PR. This stub only carries enough state to let the chain wire it through
// the standard services.Service lifecycle.
type StellarTxm struct {
	services.StateMachine

	lggr      logger.Logger
	keystore  core.Keystore
	chainID   string
	getClient func() (RPCClient, error)
}

var _ services.Service = (*StellarTxm)(nil)

// New constructs a StellarTxm. The getClient callback is supplied by the
// chain package and resolves to a healthy RPC client at call time.
func New(
	lggr logger.Logger,
	keystore core.Keystore,
	getClient func() (RPCClient, error),
	chainID string,
) (*StellarTxm, error) {
	return &StellarTxm{
		lggr:      logger.Named(lggr, "StellarTxm"),
		keystore:  keystore,
		chainID:   chainID,
		getClient: getClient,
	}, nil
}

func (t *StellarTxm) Name() string { return t.lggr.Name() }

func (t *StellarTxm) Start(_ context.Context) error {
	return t.StartOnce("StellarTxm", func() error {
		t.lggr.Debugw("Starting")
		return nil
	})
}

func (t *StellarTxm) Close() error {
	return t.StopOnce("StellarTxm", func() error {
		t.lggr.Debugw("Stopping")
		return nil
	})
}

func (t *StellarTxm) Ready() error { return t.StateMachine.Ready() }

func (t *StellarTxm) HealthReport() map[string]error {
	return map[string]error{t.Name(): t.StateMachine.Healthy()}
}
