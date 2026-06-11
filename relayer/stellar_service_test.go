package relayer

import (
	"context"
	"errors"
	"testing"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	commontypes "github.com/smartcontractkit/chainlink-common/pkg/types"
	stellartypes "github.com/smartcontractkit/chainlink-common/pkg/types/chains/stellar"

	"github.com/smartcontractkit/chainlink-stellar/internal/mocks"
	"github.com/smartcontractkit/chainlink-stellar/relayer/chain"
	"github.com/smartcontractkit/chainlink-stellar/relayer/txm"
)

type stubChain struct {
	chain.Chain
	rpc   chain.RPCClient
	txMgr *txm.StellarTxm
}

func (s *stubChain) GetClient() (chain.RPCClient, error) { return s.rpc, nil }
func (s *stubChain) TxManager() *txm.StellarTxm          { return s.txMgr }

// newTestStellarService builds a stellarService backed by the given mock RPC client.
func newTestStellarService(t *testing.T, rpc chain.RPCClient) *stellarService {
	t.Helper()
	svc := newStellarService(&stubChain{rpc: rpc})
	return &svc
}

// mockTxManager is a minimal test double for stellarTxManager.
type mockTxManager struct {
	enqueueAndWait func(ctx context.Context, req txm.TxRequest) (*txm.TxResult, error)
}

func (m *mockTxManager) EnqueueAndWait(ctx context.Context, req txm.TxRequest) (*txm.TxResult, error) {
	return m.enqueueAndWait(ctx, req)
}

// newTestStellarServiceWithTxm builds a stellarService backed by the given mock TXM.
func newTestStellarServiceWithTxm(t *testing.T, txMgr stellarTxManager) *stellarService {
	t.Helper()
	return &stellarService{txMgr: txMgr}
}

func TestStellarService_GetLedgerEntries(t *testing.T) {
	t.Parallel()

	t.Run("WithLiveUntil", func(t *testing.T) {
		ctx := t.Context()
		liveUntil := uint32(500)
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().GetLedgerEntries(ctx, protocol.GetLedgerEntriesRequest{
			Keys: []string{"a2V5WERS"},
		}).Return(protocol.GetLedgerEntriesResponse{
			LatestLedger: 50,
			Entries: []protocol.LedgerEntryResult{
				{KeyXDR: "a2V5WERS", DataXDR: "ZGF0YXJES", LastModifiedLedger: 30, LiveUntilLedgerSeq: &liveUntil},
			},
		}, nil)

		svc := newTestStellarService(t, rpc)
		resp, err := svc.GetLedgerEntries(ctx, stellartypes.GetLedgerEntriesRequest{Keys: []string{"a2V5WERS"}})
		require.NoError(t, err)
		require.Equal(t, uint32(50), resp.LatestLedger)
		require.Len(t, resp.Entries, 1)
		require.Equal(t, "a2V5WERS", resp.Entries[0].KeyXDR)
		require.Equal(t, "ZGF0YXJES", resp.Entries[0].DataXDR)
		require.Equal(t, uint32(30), resp.Entries[0].LastModifiedLedger)
		require.NotNil(t, resp.Entries[0].LiveUntilLedgerSeq)
		require.Equal(t, liveUntil, *resp.Entries[0].LiveUntilLedgerSeq)
	})

	t.Run("NoLiveUntil", func(t *testing.T) {
		ctx := t.Context()
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().GetLedgerEntries(ctx, protocol.GetLedgerEntriesRequest{
			Keys: []string{"a2V5Mg=="},
		}).Return(protocol.GetLedgerEntriesResponse{
			LatestLedger: 60,
			Entries: []protocol.LedgerEntryResult{
				{KeyXDR: "a2V5Mg==", DataXDR: "ZGF0YTI=", LastModifiedLedger: 40},
			},
		}, nil)

		svc := newTestStellarService(t, rpc)
		resp, err := svc.GetLedgerEntries(ctx, stellartypes.GetLedgerEntriesRequest{Keys: []string{"a2V5Mg=="}})
		require.NoError(t, err)
		require.Len(t, resp.Entries, 1)
		require.Nil(t, resp.Entries[0].LiveUntilLedgerSeq)
		require.Equal(t, uint32(60), resp.LatestLedger)
	})

	t.Run("MixedLiveUntil", func(t *testing.T) {
		ctx := t.Context()
		liveUntil := uint32(777)
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().GetLedgerEntries(ctx, protocol.GetLedgerEntriesRequest{
			Keys: []string{"azE=", "azI="},
		}).Return(protocol.GetLedgerEntriesResponse{
			LatestLedger: 70,
			Entries: []protocol.LedgerEntryResult{
				{KeyXDR: "azE=", DataXDR: "ZDE=", LastModifiedLedger: 10, LiveUntilLedgerSeq: &liveUntil},
				{KeyXDR: "azI=", DataXDR: "ZDI=", LastModifiedLedger: 20},
			},
		}, nil)

		svc := newTestStellarService(t, rpc)
		resp, err := svc.GetLedgerEntries(ctx, stellartypes.GetLedgerEntriesRequest{Keys: []string{"azE=", "azI="}})
		require.NoError(t, err)
		require.Len(t, resp.Entries, 2)
		require.NotNil(t, resp.Entries[0].LiveUntilLedgerSeq)
		require.Equal(t, liveUntil, *resp.Entries[0].LiveUntilLedgerSeq)
		require.Nil(t, resp.Entries[1].LiveUntilLedgerSeq)
	})

	t.Run("RPCError", func(t *testing.T) {
		ctx := t.Context()
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().GetLedgerEntries(ctx, protocol.GetLedgerEntriesRequest{
			Keys: []string{"a2V5"},
		}).Return(protocol.GetLedgerEntriesResponse{}, errors.New("ledger gone"))

		svc := newTestStellarService(t, rpc)
		_, err := svc.GetLedgerEntries(ctx, stellartypes.GetLedgerEntriesRequest{Keys: []string{"a2V5"}})
		require.ErrorContains(t, err, "ledger gone")
	})
}

func TestStellarService_GetLatestLedger(t *testing.T) {
	t.Parallel()

	t.Run("OK", func(t *testing.T) {
		ctx := t.Context()
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().GetLatestLedger(ctx).Return(protocol.GetLatestLedgerResponse{
			Hash:            "ledgerhash",
			ProtocolVersion: 21,
			Sequence:        1234,
			LedgerCloseTime: 9876543210,
		}, nil)

		svc := newTestStellarService(t, rpc)
		resp, err := svc.GetLatestLedger(ctx)
		require.NoError(t, err)
		require.Equal(t, "ledgerhash", resp.Hash)
		require.Equal(t, uint32(21), resp.ProtocolVersion)
		require.Equal(t, uint32(1234), resp.Sequence)
		require.Equal(t, int64(9876543210), resp.LedgerCloseTime)
	})

	t.Run("RPCError", func(t *testing.T) {
		ctx := t.Context()
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().GetLatestLedger(ctx).Return(protocol.GetLatestLedgerResponse{}, errors.New("connection refused"))

		svc := newTestStellarService(t, rpc)
		_, err := svc.GetLatestLedger(ctx)
		require.ErrorContains(t, err, "connection refused")
	})
}

// testContractID returns a valid C… StrKey contract address (all-zero contract).
func testContractID(t *testing.T) string {
	t.Helper()
	id, err := strkey.Encode(strkey.VersionByteContract, make([]byte, 32))
	require.NoError(t, err)
	return id
}

func TestStellarService_ReadContract(t *testing.T) {
	t.Parallel()

	// These never reach the RPC client; a mock with no expectations rejects any call.
	t.Run("EmptyContractID", func(t *testing.T) {
		svc := newTestStellarService(t, mocks.NewMockRPCClient(t))
		_, err := svc.ReadContract(t.Context(), stellartypes.ReadContractRequest{Function: "get"})
		require.ErrorContains(t, err, "contract id is required")
	})

	t.Run("EmptyFunction", func(t *testing.T) {
		svc := newTestStellarService(t, mocks.NewMockRPCClient(t))
		_, err := svc.ReadContract(t.Context(), stellartypes.ReadContractRequest{ContractID: testContractID(t)})
		require.ErrorContains(t, err, "function is required")
	})

	t.Run("InvalidContractID", func(t *testing.T) {
		svc := newTestStellarService(t, mocks.NewMockRPCClient(t))
		_, err := svc.ReadContract(t.Context(), stellartypes.ReadContractRequest{ContractID: "not-a-contract", Function: "get"})
		require.ErrorContains(t, err, "failed to decode contract id")
	})

	t.Run("ArgConversionFailure", func(t *testing.T) {
		svc := newTestStellarService(t, mocks.NewMockRPCClient(t))
		_, err := svc.ReadContract(t.Context(), stellartypes.ReadContractRequest{
			ContractID: testContractID(t),
			Function:   "get",
			// Declares Bool but leaves the pointer nil, so scValToXDR rejects it.
			Args: []stellartypes.ScVal{{Type: stellartypes.ScValTypeBool}},
		})
		require.ErrorContains(t, err, "failed to convert arg 0")
	})

	t.Run("SimulationError", func(t *testing.T) {
		ctx := t.Context()
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
			Error:        "HostError: contract trapped",
			LatestLedger: 42,
		}, nil)

		svc := newTestStellarService(t, rpc)
		resp, err := svc.ReadContract(ctx, stellartypes.ReadContractRequest{ContractID: testContractID(t), Function: "get"})
		require.NoError(t, err)
		require.Equal(t, "HostError: contract trapped", resp.Error)
		require.Equal(t, uint32(42), resp.LedgerSequence)
		require.Empty(t, resp.Result)
	})

	t.Run("Success", func(t *testing.T) {
		ctx := t.Context()
		returnXDR := "AAAAAwAAAAE=" // arbitrary base64 XDR ScVal
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
			LatestLedger: 99,
			Results:      []protocol.SimulateHostFunctionResult{{ReturnValueXDR: &returnXDR}},
		}, nil)

		svc := newTestStellarService(t, rpc)
		resp, err := svc.ReadContract(ctx, stellartypes.ReadContractRequest{ContractID: testContractID(t), Function: "get"})
		require.NoError(t, err)
		require.Equal(t, returnXDR, resp.Result)
		require.Equal(t, uint32(99), resp.LedgerSequence)
		require.Empty(t, resp.Error)
	})

	t.Run("ZeroResultsIsError", func(t *testing.T) {
		ctx := t.Context()
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
			LatestLedger: 5,
			Results:      nil,
		}, nil)

		svc := newTestStellarService(t, rpc)
		_, err := svc.ReadContract(ctx, stellartypes.ReadContractRequest{ContractID: testContractID(t), Function: "get"})
		require.ErrorContains(t, err, "unexpected simulation result count")
	})

	t.Run("EmptyReturnValueIsError", func(t *testing.T) {
		ctx := t.Context()
		empty := ""
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
			LatestLedger: 5,
			Results:      []protocol.SimulateHostFunctionResult{{ReturnValueXDR: &empty}},
		}, nil)

		svc := newTestStellarService(t, rpc)
		_, err := svc.ReadContract(ctx, stellartypes.ReadContractRequest{ContractID: testContractID(t), Function: "get"})
		require.ErrorContains(t, err, "return value XDR was empty")
	})

	t.Run("RPCError", func(t *testing.T) {
		ctx := t.Context()
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{}, errors.New("rpc down"))

		svc := newTestStellarService(t, rpc)
		_, err := svc.ReadContract(ctx, stellartypes.ReadContractRequest{ContractID: testContractID(t), Function: "get"})
		require.ErrorContains(t, err, "rpc down")
	})
}

func TestStellarService_SubmitTransaction(t *testing.T) {
	t.Parallel()

	sym := "transfer"
	baseReq := stellartypes.SubmitTransactionRequest{
		IdempotencyKey:     "idem-1",
		FromAddress:        testStellarAccount(t),
		ContractID:         testContractID(t),
		Function:           "transfer",
		Args:               []stellartypes.ScVal{{Type: stellartypes.ScValTypeSymbol, Symbol: &sym}},
		LedgerBoundsOffset: 2,
	}

	t.Run("Success_Finalized", func(t *testing.T) {
		ctx := t.Context()
		txMgr := &mockTxManager{
			enqueueAndWait: func(_ context.Context, req txm.TxRequest) (*txm.TxResult, error) {
				require.Equal(t, "idem-1", req.ID)
				require.Equal(t, baseReq.FromAddress, req.FromAddress)
				require.Len(t, req.Operations, 1)
				require.Equal(t, uint32(2), req.LedgerBoundsOffset)
				return &txm.TxResult{
					ID:            "idem-1",
					Hash:          "txhash123",
					Status:        commontypes.Finalized,
					ResultXDR:     "resultXDR",
					ResultMetaXDR: "metaXDR",
				}, nil
			},
		}

		svc := newTestStellarServiceWithTxm(t, txMgr)
		reply, err := svc.SubmitTransaction(ctx, baseReq)
		require.NoError(t, err)
		require.Equal(t, stellartypes.TxSuccess, reply.TxStatus)
		require.Equal(t, "txhash123", reply.TxHash)
		require.Equal(t, "idem-1", reply.TxIdempotencyKey)
		require.Equal(t, "resultXDR", reply.ResultXDR)
		require.Equal(t, "metaXDR", reply.ResultMetaXDR)
	})

	t.Run("Failed_OnChain", func(t *testing.T) {
		ctx := t.Context()
		txMgr := &mockTxManager{
			enqueueAndWait: func(_ context.Context, _ txm.TxRequest) (*txm.TxResult, error) {
				return &txm.TxResult{
					ID:        "idem-1",
					Hash:      "failhash",
					Status:    commontypes.Failed,
					ResultXDR: "failedResultXDR",
					Error:     errors.New("tx revert: contract error"),
				}, nil
			},
		}

		svc := newTestStellarServiceWithTxm(t, txMgr)
		reply, err := svc.SubmitTransaction(ctx, baseReq)
		require.Error(t, err)
		require.ErrorContains(t, err, "contract error")
		require.Equal(t, stellartypes.TxFailed, reply.TxStatus)
		require.Equal(t, "failhash", reply.TxHash)
	})

	t.Run("Fatal_TxmError", func(t *testing.T) {
		ctx := t.Context()
		txMgr := &mockTxManager{
			enqueueAndWait: func(_ context.Context, _ txm.TxRequest) (*txm.TxResult, error) {
				return nil, errors.New("simulation failed: insufficient fee")
			},
		}

		svc := newTestStellarServiceWithTxm(t, txMgr)
		_, err := svc.SubmitTransaction(ctx, baseReq)
		require.Error(t, err)
		require.ErrorContains(t, err, "simulation failed")
	})

	t.Run("MissingContractID", func(t *testing.T) {
		ctx := t.Context()
		svc := newTestStellarServiceWithTxm(t, &mockTxManager{})
		_, err := svc.SubmitTransaction(ctx, stellartypes.SubmitTransactionRequest{Function: "fn"})
		require.ErrorContains(t, err, "contractID is required")
	})

	t.Run("MissingFunction", func(t *testing.T) {
		ctx := t.Context()
		svc := newTestStellarServiceWithTxm(t, &mockTxManager{})
		_, err := svc.SubmitTransaction(ctx, stellartypes.SubmitTransactionRequest{ContractID: testContractID(t)})
		require.ErrorContains(t, err, "function is required")
	})

	t.Run("BadArg_NilBool", func(t *testing.T) {
		ctx := t.Context()
		svc := newTestStellarServiceWithTxm(t, &mockTxManager{})
		_, err := svc.SubmitTransaction(ctx, stellartypes.SubmitTransactionRequest{
			ContractID: testContractID(t),
			Function:   "fn",
			Args:       []stellartypes.ScVal{{Type: stellartypes.ScValTypeBool}},
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "convert args")
	})

	t.Run("ContextCancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		txMgr := &mockTxManager{
			enqueueAndWait: func(ctx context.Context, _ txm.TxRequest) (*txm.TxResult, error) {
				return nil, ctx.Err()
			},
		}

		svc := newTestStellarServiceWithTxm(t, txMgr)
		_, err := svc.SubmitTransaction(ctx, baseReq)
		require.Error(t, err)
	})
}

func testStellarAccount(t *testing.T) string {
	t.Helper()
	accountID := make([]byte, 32)
	addr, err := strkey.Encode(strkey.VersionByteAccountID, accountID)
	require.NoError(t, err)
	return addr
}
