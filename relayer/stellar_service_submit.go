package relayer

import (
	"context"
	"fmt"

	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"

	commontypes "github.com/smartcontractkit/chainlink-common/pkg/types"
	stellartypes "github.com/smartcontractkit/chainlink-common/pkg/types/chains/stellar"

	"github.com/smartcontractkit/chainlink-stellar/relayer/txm"
)

// stellarTxManager is the subset of txm.StellarTxm used by SubmitTransaction.
type stellarTxManager interface {
	EnqueueAndWait(ctx context.Context, req txm.TxRequest) (*txm.TxResult, error)
}

// SubmitTransaction invokes a Soroban contract via the TXM pipeline.
// It converts the high-level domain request into an InvokeHostFunction operation,
// enqueues it through the TXM, and maps the terminal status into a response.
func (s *stellarService) SubmitTransaction(ctx context.Context, req stellartypes.SubmitTransactionRequest) (*stellartypes.SubmitTransactionResponse, error) {
	if s.txMgr == nil {
		return nil, fmt.Errorf("SubmitTransaction: txm is not configured")
	}
	if req.ContractID == "" {
		return nil, fmt.Errorf("SubmitTransaction: contract_id is required")
	}
	if req.Function == "" {
		return nil, fmt.Errorf("SubmitTransaction: function is required")
	}

	xdrArgs, err := domainScValsToXDR(req.Args)
	if err != nil {
		return nil, fmt.Errorf("SubmitTransaction: convert args: %w", err)
	}

	op, err := txm.BuildInvokeContractOperation(req.ContractID, req.Function, xdrArgs, req.FromAddress)
	if err != nil {
		return nil, fmt.Errorf("SubmitTransaction: build operation: %w", err)
	}

	result, err := s.txMgr.EnqueueAndWait(ctx, txm.TxRequest{
		ID:                 req.IdempotencyKey,
		FromAddress:        req.FromAddress,
		Operations:         []txnbuild.Operation{op},
		LedgerBoundsOffset: req.LedgerBoundsOffset,
	})
	if err != nil {
		return nil, fmt.Errorf("SubmitTransaction: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("SubmitTransaction: nil result returned by TXM")
	}

	reply := &stellartypes.SubmitTransactionResponse{
		TxHash:           result.Hash,
		TxIdempotencyKey: result.ID,
		ResultXDR:        result.ResultXDR,
		ResultMetaXDR:    result.ResultMetaXDR,
	}

	switch result.Status {
	case commontypes.Finalized:
		reply.TxStatus = stellartypes.TxSuccess
	case commontypes.Failed:
		reply.TxStatus = stellartypes.TxFailed
		if result.Error != nil {
			return reply, result.Error
		}
	default:
		reply.TxStatus = stellartypes.TxFatal
		if result.Error != nil {
			return reply, result.Error
		}
		return reply, fmt.Errorf("SubmitTransaction: unexpected terminal status %v", result.Status)
	}

	return reply, nil
}

func domainScValsToXDR(vals []stellartypes.ScVal) ([]xdr.ScVal, error) {
	if len(vals) == 0 {
		return nil, nil
	}
	out := make([]xdr.ScVal, len(vals))
	for i, v := range vals {
		x, err := scValToXDR(v)
		if err != nil {
			return nil, fmt.Errorf("arg[%d]: %w", i, err)
		}
		out[i] = x
	}
	return out, nil
}
