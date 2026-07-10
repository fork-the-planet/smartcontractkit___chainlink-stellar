package txm

import (
	"math/big"
	"sync"
	"time"

	"github.com/stellar/go-stellar-sdk/txnbuild"

	commontypes "github.com/smartcontractkit/chainlink-common/pkg/types"
)

// StellarTx represents a single transaction tracked by the TXM from enqueue to confirmation.
type StellarTx struct {
	ID          string
	Metadata    *commontypes.TxMeta
	Timestamp   uint64
	FromAddress string // G... strkey: source account and signer for this TXM

	Operations         []txnbuild.Operation
	LedgerBoundsOffset uint32 // per-tx override (0 = use config default)

	Attempt         uint64
	Status          commontypes.TransactionStatus
	BroadcastAt     time.Time // set when SendTransaction accepts the tx
	TxHash          string
	Fee             *big.Int // total fee in stroops; updated to actual FeeCharged on confirmation
	LedgerCloseTime int64    // unix seconds when tx was included in a ledger; from GetTransaction
	ResultXDR       string   // XDR-encoded transaction result from GetTransaction
	ResultCode      string   // result code from GetTransaction (for diagnostics)
	ResultMetaXDR   string   // XDR-encoded result meta from GetTransaction SUCCESS
	MaxLedger       uint32   // ledger bounds set during broadcast
	MinResourceFee  int64    // from simulation result

	// Done is closed when the transaction reaches a terminal state.
	Done     chan struct{}
	doneOnce sync.Once
}

// TxRequest is the input accepted by Enqueue / EnqueueAndWait.
type TxRequest struct {
	ID                 string               // idempotency key (auto-generated if empty)
	FromAddress        string               // optional; defaults to TXM's signer address
	Operations         []txnbuild.Operation // the Stellar operations to execute
	LedgerBoundsOffset uint32               // per-tx override (0 = use config default)
	Metadata           *commontypes.TxMeta  // optional; carries WorkflowExecutionID and other node-level context
}

// TxResult is returned by EnqueueAndWait and Simulate with the outcome of a transaction.
type TxResult struct {
	ID              string
	Hash            string
	Status          commontypes.TransactionStatus
	Fee             *big.Int // total fee charged in stroops
	LedgerCloseTime int64    // unix seconds when tx was included in a ledger; 0 if unknown
	ResultXDR       string   // XDR-encoded transaction result from GetTransaction
	ResultMetaXDR   string   // XDR-encoded result meta from GetTransaction
	Error           error
}

// ErrorReason is a bounded label classifying broadcast and confirmation failures.
type ErrorReason string

const (
	ErrorReasonSequenceNumber    ErrorReason = "sequence_number"
	ErrorReasonStoreCreate       ErrorReason = "store_create"
	ErrorReasonSimulation        ErrorReason = "simulation"
	ErrorReasonAssembly          ErrorReason = "assembly"
	ErrorReasonSigning           ErrorReason = "signing"
	ErrorReasonNoHash            ErrorReason = "no_hash"
	ErrorReasonStoreAdd          ErrorReason = "store_add"
	ErrorReasonUnknownSubmit     ErrorReason = "unknown_submit"
	ErrorReasonMaxRetries        ErrorReason = "max_retries"
	ErrorReasonRevert            ErrorReason = "revert"
	ErrorReasonRevertDecode      ErrorReason = ErrorReasonRevert + "_decode_error"
	ErrorReasonTimedOut          ErrorReason = "timed_out"
	ErrorReasonBadSeq            ErrorReason = "bad_seq"
	ErrorReasonRestoreFailed     ErrorReason = "restore_failed"
	ErrorReasonTryAgainLater     ErrorReason = "try_again_later"
	ErrorReasonClientUnavailable ErrorReason = "client_unavailable"
	ErrorReasonInsufficientFee   ErrorReason = "insufficient_fee"
	ErrorReasonInternalError     ErrorReason = "internal_error"
	ErrorReasonNilTx             ErrorReason = "nil_tx"
	ErrorReasonNilTxStore        ErrorReason = "nil_tx_store"
	// ErrorReasonSubmitErrorUndecoded means the node returned TXStatusError but
	// ErrorResultXDR was empty or not valid transaction-result XDR.
	ErrorReasonSubmitErrorUndecoded ErrorReason = "submit_error_undecoded"
)

// DropReason is a bounded label classifying why a transaction was dropped from
// the broadcast queue.
type DropReason string

const (
	// DropReasonChannelFullOldestEvicted: the oldest queued tx was evicted to make
	// room for a newer one. The oldest has the stalest simulation data and the
	// nearest LedgerBounds expiry, so the new tx's intent takes priority.
	DropReasonChannelFullOldestEvicted DropReason = "channel_full_oldest_evicted"

	// DropReasonChannelFullNewRejected: the incoming tx was rejected because the
	// channel was still full after an attempted oldest-evict (concurrent enqueue race).
	DropReasonChannelFullNewRejected DropReason = "channel_full_new_rejected"
)

// RetryReason classifies why a transaction is being retried (Layer 3 lifecycle retries).
type RetryReason string

const (
	RetryReasonResourceExhaustion RetryReason = "resource_exhaustion"
	RetryReasonTimedOut           RetryReason = "timed_out"
	RetryReasonBadSeq             RetryReason = "bad_seq"
	RetryReasonTryAgainLater      RetryReason = "try_again_later"
	RetryReasonClientUnavailable  RetryReason = "client_unavailable"
)
