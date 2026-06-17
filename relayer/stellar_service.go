package relayer

import (
	"context"
	"fmt"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/smartcontractkit/chainlink-framework/multinode"

	relaytypes "github.com/smartcontractkit/chainlink-common/pkg/types"
	stellartypes "github.com/smartcontractkit/chainlink-common/pkg/types/chains/stellar"

	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	"github.com/smartcontractkit/chainlink-stellar/relayer/chain"
	"github.com/smartcontractkit/chainlink-stellar/relayer/txm"
)

const readContractTimeBound = 1 * time.Minute

// StellarTxManager is the subset of txm.StellarTxm used by SubmitTransaction.
type StellarTxManager interface {
	EnqueueAndWait(ctx context.Context, req txm.TxRequest) (*txm.TxResult, error)
}

type stellarService struct {
	relaytypes.UnimplementedStellarService
	chain chain.Chain
	txMgr StellarTxManager
}

var _ relaytypes.StellarService = (*stellarService)(nil)

func newStellarService(ch chain.Chain) stellarService {
	return stellarService{chain: ch, txMgr: ch.TxManager()}
}

func (s *stellarService) GetLedgerEntries(ctx context.Context, req stellartypes.GetLedgerEntriesRequest) (stellartypes.GetLedgerEntriesResponse, error) {
	rpc, err := s.chain.GetClient()
	if err != nil {
		return stellartypes.GetLedgerEntriesResponse{}, fmt.Errorf("failed to get client: %w", err)
	}

	keys := make([]string, len(req.Keys))
	for i, k := range req.Keys {
		keys[i] = k
	}

	resp, err := rpc.GetLedgerEntries(ctx, protocol.GetLedgerEntriesRequest{Keys: keys})
	if err != nil {
		return stellartypes.GetLedgerEntriesResponse{}, err
	}

	entries := make([]stellartypes.LedgerEntryResult, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		entry := stellartypes.LedgerEntryResult{
			KeyXDR:             e.KeyXDR,
			DataXDR:            e.DataXDR,
			LastModifiedLedger: e.LastModifiedLedger,
			ExtensionXDR:       e.ExtensionXDR,
		}
		if e.LiveUntilLedgerSeq != nil {
			v := *e.LiveUntilLedgerSeq
			entry.LiveUntilLedgerSeq = &v
		}
		entries = append(entries, entry)
	}

	return stellartypes.GetLedgerEntriesResponse{
		Entries:      entries,
		LatestLedger: resp.LatestLedger,
	}, nil
}

func (s *stellarService) GetLatestLedger(ctx context.Context) (stellartypes.GetLatestLedgerResponse, error) {
	rpc, err := s.chain.GetClient()
	if err != nil {
		return stellartypes.GetLatestLedgerResponse{}, fmt.Errorf("failed to get client: %w", err)
	}

	resp, err := rpc.GetLatestLedger(ctx)
	if err != nil {
		return stellartypes.GetLatestLedgerResponse{}, fmt.Errorf("%w: %w", multinode.ErrNodeError, err)
	}

	return stellartypes.GetLatestLedgerResponse{
		Hash:              resp.Hash,
		ProtocolVersion:   resp.ProtocolVersion,
		Sequence:          resp.Sequence,
		LedgerCloseTime:   resp.LedgerCloseTime,
		LedgerHeaderXDR:   resp.LedgerHeader,
		LedgerMetadataXDR: resp.LedgerMetadata,
	}, nil
}

// ReadContract performs a read-only Soroban contract call by building an
// InvokeHostFunction operation and simulating it against the network (the transaction is never submitted or signed).
//
// A successful simulation that reports a contract/host-function failure is returned in ReadContractResponse.Error with a nil Go error.
func (s *stellarService) ReadContract(ctx context.Context, req stellartypes.ReadContractRequest) (stellartypes.ReadContractResponse, error) {
	if req.ContractID == "" {
		return stellartypes.ReadContractResponse{}, fmt.Errorf("contract id is required")
	}
	if req.Function == "" {
		return stellartypes.ReadContractResponse{}, fmt.Errorf("function is required")
	}

	rpc, err := s.chain.GetClient()
	if err != nil {
		return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to get chain client: %w", err)
	}

	contractBytes, err := strkey.Decode(strkey.VersionByteContract, req.ContractID)
	if err != nil {
		return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to decode contract id %q: %w", req.ContractID, err)
	}

	contractAddr := scval.BuildContractScAddress(contractBytes)
	if contractAddr == nil {
		return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to build contract address from contractID: %s", req.ContractID)
	}

	args, err := convertDomainScVals(req.Args)
	if err != nil {
		return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to convert args: %w", err)
	}

	hostFn := xdr.HostFunction{
		Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
		InvokeContract: &xdr.InvokeContractArgs{
			ContractAddress: *contractAddr,
			FunctionName:    xdr.ScSymbol(req.Function),
			Args:            args,
		},
	}

	srcAddr := req.SourceAccount
	if srcAddr == "" {
		srcAddr, err = strkey.Encode(strkey.VersionByteAccountID, make([]byte, 32))
		if err != nil {
			return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to build placeholder source account: %w", err)
		}
	} else if _, err = strkey.Decode(strkey.VersionByteAccountID, srcAddr); err != nil {
		return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to decode source account %q: %w", srcAddr, err)
	}

	sourceAccount := &txnbuild.SimpleAccount{AccountID: srcAddr, Sequence: 0}

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        sourceAccount,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{
			&txnbuild.InvokeHostFunction{
				HostFunction:  hostFn,
				SourceAccount: srcAddr,
			},
		},
		BaseFee:       txnbuild.MinBaseFee,
		Preconditions: txnbuild.Preconditions{TimeBounds: txnbuild.NewTimebounds(0, time.Now().Add(readContractTimeBound).Unix())},
	})
	if err != nil {
		return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to build transaction: %w", err)
	}

	txXDR, err := tx.Base64()
	if err != nil {
		return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to encode transaction: %w", err)
	}

	simResult, err := rpc.SimulateTransaction(ctx, protocol.SimulateTransactionRequest{Transaction: txXDR})
	if err != nil {
		return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to read contract: %w: %w", multinode.ErrNodeError, err)
	}
	resp := stellartypes.ReadContractResponse{LedgerSequence: simResult.LatestLedger}

	if simResult.Error != "" {
		// Contract-level failure: report via the response, not a transport error.
		resp.Error = simResult.Error
		return resp, nil
	}

	// A successful InvokeHostFunction simulation always yields exactly one result
	// whose return value is a (possibly void) XDR ScVal. Anything else indicates
	// an unexpected RPC/SDK response shape rather than a valid empty result.
	if len(simResult.Results) != 1 {
		return resp, fmt.Errorf("unexpected simulation result count: got %d, want 1", len(simResult.Results))
	}
	rv := simResult.Results[0].ReturnValueXDR
	if rv == nil || *rv == "" {
		return resp, fmt.Errorf("simulation succeeded but return value XDR was empty")
	}

	resp.Result = *rv
	return resp, nil
}

// SubmitTransaction invokes a Soroban contract via the TXM pipeline.
// It converts the high-level domain request into an InvokeHostFunction operation,
// enqueues it through the TXM, and maps the terminal status into a response.
func (s *stellarService) SubmitTransaction(ctx context.Context, req stellartypes.SubmitTransactionRequest) (*stellartypes.SubmitTransactionResponse, error) {
	if s.txMgr == nil {
		return nil, fmt.Errorf("submit transaction: txm is not configured")
	}
	if req.ContractID == "" {
		return nil, fmt.Errorf("submit transaction: contractID is required")
	}
	if req.Function == "" {
		return nil, fmt.Errorf("submit transaction: function is required")
	}

	xdrArgs, err := convertDomainScVals(req.Args)
	if err != nil {
		return nil, fmt.Errorf("submit transaction: convert args: %w", err)
	}

	op, err := txm.BuildInvokeContractOperation(req.ContractID, req.Function, xdrArgs, req.FromAddress)
	if err != nil {
		return nil, fmt.Errorf("submit transaction: build operation: %w", err)
	}

	result, err := s.txMgr.EnqueueAndWait(ctx, txm.TxRequest{
		ID:                 req.IdempotencyKey,
		FromAddress:        req.FromAddress,
		Operations:         []txnbuild.Operation{op},
		LedgerBoundsOffset: req.LedgerBoundsOffset,
	})
	if err != nil {
		return nil, fmt.Errorf("submit transaction: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("submit transaction: nil result returned by TXM")
	}

	reply := &stellartypes.SubmitTransactionResponse{
		TxHash:           result.Hash,
		TxIdempotencyKey: result.ID,
		ResultXDR:        result.ResultXDR,
		ResultMetaXDR:    result.ResultMetaXDR,
	}
	if result.Fee != nil && result.Fee.IsUint64() {
		fee := result.Fee.Uint64()
		reply.TransactionFee = &fee
	}

	switch result.Status {
	case relaytypes.Finalized:
		reply.TxStatus = stellartypes.TxSuccess
	case relaytypes.Failed:
		reply.TxStatus = stellartypes.TxFailed
		if result.Error != nil {
			reply.Error = result.Error.Error()
		}
	default:
		reply.TxStatus = stellartypes.TxFatal
		if result.Error != nil {
			return reply, result.Error
		}
		return reply, fmt.Errorf("submit transaction: unexpected terminal status %v", result.Status)
	}

	return reply, nil
}
