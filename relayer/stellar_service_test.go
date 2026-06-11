package relayer

import (
	"errors"
	"testing"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-framework/multinode"

	stellartypes "github.com/smartcontractkit/chainlink-common/pkg/types/chains/stellar"

	"github.com/smartcontractkit/chainlink-stellar/internal/mocks"
	"github.com/smartcontractkit/chainlink-stellar/relayer/chain"
)

type stubChain struct {
	chain.Chain
	rpc chain.RPCClient
}

func (s *stubChain) GetClient() (chain.RPCClient, error) { return s.rpc, nil }

// newTestStellarService builds a stellarService backed by the given mock RPC client.
func newTestStellarService(t *testing.T, rpc chain.RPCClient) *stellarService {
	t.Helper()
	svc := newStellarService(&stubChain{rpc: rpc})
	return &svc
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
		require.ErrorIs(t, err, multinode.ErrNodeError)
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
	// Input-validation failures are returned plainly (not wrapped with ErrNodeError) so the
	// chain capability defaults them to user errors.
	t.Run("EmptyContractID", func(t *testing.T) {
		svc := newTestStellarService(t, mocks.NewMockRPCClient(t))
		_, err := svc.ReadContract(t.Context(), stellartypes.ReadContractRequest{Function: "get"})
		require.ErrorContains(t, err, "contract id is required")
		require.NotErrorIs(t, err, multinode.ErrNodeError)
	})

	t.Run("EmptyFunction", func(t *testing.T) {
		svc := newTestStellarService(t, mocks.NewMockRPCClient(t))
		_, err := svc.ReadContract(t.Context(), stellartypes.ReadContractRequest{ContractID: testContractID(t)})
		require.ErrorContains(t, err, "function is required")
		require.NotErrorIs(t, err, multinode.ErrNodeError)
	})

	t.Run("InvalidContractID", func(t *testing.T) {
		svc := newTestStellarService(t, mocks.NewMockRPCClient(t))
		_, err := svc.ReadContract(t.Context(), stellartypes.ReadContractRequest{ContractID: "not-a-contract", Function: "get"})
		require.ErrorContains(t, err, "failed to decode contract id")
		require.NotErrorIs(t, err, multinode.ErrNodeError)
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
		require.NotErrorIs(t, err, multinode.ErrNodeError)
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
		// Transport failures are node-availability errors: they carry ErrNodeError so the
		// capability classifies them as infra, and the underlying cause is preserved.
		require.ErrorIs(t, err, multinode.ErrNodeError)
	})

	t.Run("InvalidSourceAccount", func(t *testing.T) {
		// Never reaches the RPC client; a mock with no expectations rejects any call.
		svc := newTestStellarService(t, mocks.NewMockRPCClient(t))
		_, err := svc.ReadContract(t.Context(), stellartypes.ReadContractRequest{
			ContractID:    testContractID(t),
			Function:      "get",
			SourceAccount: "not-an-account",
		})
		require.ErrorContains(t, err, "failed to decode source account")
		require.NotErrorIs(t, err, multinode.ErrNodeError)
	})

	t.Run("CustomSourceAccount", func(t *testing.T) {
		ctx := t.Context()
		raw := make([]byte, 32)
		for i := range raw {
			raw[i] = 0x11
		}
		source, err := strkey.Encode(strkey.VersionByteAccountID, raw)
		require.NoError(t, err)

		returnXDR := "AAAAAwAAAAE="
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().SimulateTransaction(mock.Anything, mock.MatchedBy(func(req protocol.SimulateTransactionRequest) bool {
			return simulatedTxSource(t, req) == source
		})).Return(protocol.SimulateTransactionResponse{
			LatestLedger: 99,
			Results:      []protocol.SimulateHostFunctionResult{{ReturnValueXDR: &returnXDR}},
		}, nil)

		svc := newTestStellarService(t, rpc)
		resp, err := svc.ReadContract(ctx, stellartypes.ReadContractRequest{
			ContractID:    testContractID(t),
			Function:      "get",
			SourceAccount: source,
		})
		require.NoError(t, err)
		require.Equal(t, returnXDR, resp.Result)
	})

	t.Run("DefaultPlaceholderSourceAccount", func(t *testing.T) {
		ctx := t.Context()
		placeholder, err := strkey.Encode(strkey.VersionByteAccountID, make([]byte, 32))
		require.NoError(t, err)

		returnXDR := "AAAAAwAAAAE="
		rpc := mocks.NewMockRPCClient(t)
		rpc.EXPECT().SimulateTransaction(mock.Anything, mock.MatchedBy(func(req protocol.SimulateTransactionRequest) bool {
			return simulatedTxSource(t, req) == placeholder
		})).Return(protocol.SimulateTransactionResponse{
			LatestLedger: 99,
			Results:      []protocol.SimulateHostFunctionResult{{ReturnValueXDR: &returnXDR}},
		}, nil)

		svc := newTestStellarService(t, rpc)
		resp, err := svc.ReadContract(ctx, stellartypes.ReadContractRequest{ContractID: testContractID(t), Function: "get"})
		require.NoError(t, err)
		require.Equal(t, returnXDR, resp.Result)
	})
}

// simulatedTxSource decodes the simulated transaction XDR and returns its source account address.
func simulatedTxSource(t *testing.T, req protocol.SimulateTransactionRequest) string {
	t.Helper()
	gtx, err := txnbuild.TransactionFromXDR(req.Transaction)
	require.NoError(t, err)
	tx, ok := gtx.Transaction()
	require.True(t, ok)
	return tx.SourceAccount().AccountID
}
