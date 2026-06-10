package relayer

import (
	"context"
	"fmt"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"

	relaytypes "github.com/smartcontractkit/chainlink-common/pkg/types"
	stellartypes "github.com/smartcontractkit/chainlink-common/pkg/types/chains/stellar"

	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	"github.com/smartcontractkit/chainlink-stellar/relayer/chain"
)

const readContractTimeBound = 1 * time.Minute

type stellarService struct {
	relaytypes.UnimplementedStellarService
	chain chain.Chain
}

var _ relaytypes.StellarService = (*stellarService)(nil)

func newStellarService(ch chain.Chain) stellarService {
	return stellarService{chain: ch}
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
		return stellartypes.GetLatestLedgerResponse{}, err
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

	args := make([]xdr.ScVal, len(req.Args))
	for i, a := range req.Args {
		xv, convErr := scValToXDR(a)
		if convErr != nil {
			return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to convert arg %d (type %d): %w", i, a.Type, convErr)
		}
		args[i] = xv
	}

	hostFn := xdr.HostFunction{
		Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
		InvokeContract: &xdr.InvokeContractArgs{
			ContractAddress: *contractAddr,
			FunctionName:    xdr.ScSymbol(req.Function),
			Args:            args,
		},
	}

	srcAddr, err := strkey.Encode(strkey.VersionByteAccountID, make([]byte, 32))
	if err != nil {
		return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to build placeholder source account: %w", err)
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
		return stellartypes.ReadContractResponse{}, fmt.Errorf("failed to read contract: %w", err)
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
