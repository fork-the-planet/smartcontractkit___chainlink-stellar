package txm

import (
	"context"
	"fmt"
	"sync"

	protocolrpc "github.com/stellar/go-stellar-sdk/protocols/rpc"

	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/loop"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	commonutils "github.com/smartcontractkit/chainlink-common/pkg/utils"
)

// RPCClient is the subset of the Stellar Soroban JSON-RPC client used by the TXM.
// Any value satisfying chain.RPCClient (which is a superset) also satisfies this.
type RPCClient interface {
	SimulateTransaction(ctx context.Context, req protocolrpc.SimulateTransactionRequest) (protocolrpc.SimulateTransactionResponse, error)
	SendTransaction(ctx context.Context, req protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error)
	GetTransaction(ctx context.Context, req protocolrpc.GetTransactionRequest) (protocolrpc.GetTransactionResponse, error)
	GetLedgerEntries(ctx context.Context, req protocolrpc.GetLedgerEntriesRequest) (protocolrpc.GetLedgerEntriesResponse, error)
	GetLatestLedger(ctx context.Context) (protocolrpc.GetLatestLedgerResponse, error)
	GetFeeStats(ctx context.Context) (protocolrpc.GetFeeStatsResponse, error)
}

var _ services.Service = (*StellarTxm)(nil)

// StellarTxm orchestrates the lifecycle of Stellar/Soroban transactions:
// enqueue → simulate → (restore) → assemble → sign → send → confirm.
//
// This is the foundation skeleton: data structures, fee strategy, metrics,
// and account store are wired here. The broadcast/confirm pipeline lands in
// a follow-up PR.
type StellarTxm struct {
	baseLogger logger.Logger
	keystore   loop.Keystore
	config     Config
	chainID    string
	metrics    *stellarTxmMetrics
	feeStrat   FeeStrategy

	transactions              map[string]*StellarTx
	transactionsLock          sync.RWMutex
	transactionsLastPruneTime uint64

	broadcastChan chan string
	accountStore  *AccountStore
	starter       commonutils.StartStopOnce
	done          sync.WaitGroup
	stop          chan struct{}

	getClient         func() (RPCClient, error)
	networkPassphrase string
}

// New creates a StellarTxm. The getClient callback should be obtained from
// chain.Chain.GetClient — the chain package constructs the TXM and passes its
// own GetClient method.
func New(
	lgr logger.Logger,
	keystore loop.Keystore,
	cfg Config,
	getClient func() (RPCClient, error),
	chainID string,
	networkPassphrase string,
) (*StellarTxm, error) {
	cfg.Resolve()

	metrics, err := newStellarTxmMetrics(chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	return &StellarTxm{
		baseLogger: logger.Named(lgr, "StellarTxm"),
		keystore:   keystore,
		config:     cfg,
		chainID:    chainID,
		metrics:    metrics,
		feeStrat:   NewFeeStrategyFromConfig(cfg),

		transactions:              make(map[string]*StellarTx),
		transactionsLastPruneTime: getTimestampSecs(),

		broadcastChan:     make(chan string, *cfg.BroadcastChanSize),
		accountStore:      NewAccountStore(),
		stop:              make(chan struct{}),
		getClient:         getClient,
		networkPassphrase: networkPassphrase,
	}, nil
}

// --- services.Service ---

func (s *StellarTxm) Name() string {
	return s.baseLogger.Name()
}

func (s *StellarTxm) Ready() error {
	return s.starter.Ready()
}

func (s *StellarTxm) HealthReport() map[string]error {
	return map[string]error{s.Name(): s.starter.Healthy()}
}

// Start transitions the service to running. The broadcast/confirm goroutines
// land in the follow-up PR; this PR only establishes the lifecycle hooks.
func (s *StellarTxm) Start(_ context.Context) error {
	return s.starter.StartOnce(s.Name(), func() error {
		return nil
	})
}

func (s *StellarTxm) Close() error {
	return s.starter.StopOnce(s.Name(), func() error {
		close(s.stop)
		return nil
	})
}
