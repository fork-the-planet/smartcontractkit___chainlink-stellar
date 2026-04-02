package deployment

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	protocolrpc "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRPC satisfies stellarRPCClient and delegates every call to a
// configurable function field. Tests set only the fields they need.
type mockRPC struct {
	SimulateTransactionFn func(ctx context.Context, req protocolrpc.SimulateTransactionRequest) (protocolrpc.SimulateTransactionResponse, error)
	SendTransactionFn     func(ctx context.Context, req protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error)
	GetTransactionFn      func(ctx context.Context, req protocolrpc.GetTransactionRequest) (protocolrpc.GetTransactionResponse, error)
	GetLedgerEntriesFn    func(ctx context.Context, req protocolrpc.GetLedgerEntriesRequest) (protocolrpc.GetLedgerEntriesResponse, error)
	GetEventsFn           func(ctx context.Context, req protocolrpc.GetEventsRequest) (protocolrpc.GetEventsResponse, error)
}

func (m *mockRPC) SimulateTransaction(ctx context.Context, req protocolrpc.SimulateTransactionRequest) (protocolrpc.SimulateTransactionResponse, error) {
	if m.SimulateTransactionFn != nil {
		return m.SimulateTransactionFn(ctx, req)
	}
	return protocolrpc.SimulateTransactionResponse{}, fmt.Errorf("SimulateTransaction not mocked")
}

func (m *mockRPC) SendTransaction(ctx context.Context, req protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error) {
	if m.SendTransactionFn != nil {
		return m.SendTransactionFn(ctx, req)
	}
	return protocolrpc.SendTransactionResponse{}, fmt.Errorf("SendTransaction not mocked")
}

func (m *mockRPC) GetTransaction(ctx context.Context, req protocolrpc.GetTransactionRequest) (protocolrpc.GetTransactionResponse, error) {
	if m.GetTransactionFn != nil {
		return m.GetTransactionFn(ctx, req)
	}
	return protocolrpc.GetTransactionResponse{}, fmt.Errorf("GetTransaction not mocked")
}

func (m *mockRPC) GetLedgerEntries(ctx context.Context, req protocolrpc.GetLedgerEntriesRequest) (protocolrpc.GetLedgerEntriesResponse, error) {
	if m.GetLedgerEntriesFn != nil {
		return m.GetLedgerEntriesFn(ctx, req)
	}
	return protocolrpc.GetLedgerEntriesResponse{}, fmt.Errorf("GetLedgerEntries not mocked")
}

func (m *mockRPC) GetEvents(ctx context.Context, req protocolrpc.GetEventsRequest) (protocolrpc.GetEventsResponse, error) {
	if m.GetEventsFn != nil {
		return m.GetEventsFn(ctx, req)
	}
	return protocolrpc.GetEventsResponse{}, fmt.Errorf("GetEvents not mocked")
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func testSorobanDataB64(t *testing.T) string {
	t.Helper()
	data := xdr.SorobanTransactionData{
		Resources: xdr.SorobanResources{
			Footprint: xdr.LedgerFootprint{},
		},
	}
	b64, err := xdr.MarshalBase64(data)
	require.NoError(t, err)
	return b64
}

func testTransactionMetaB64(t *testing.T) string {
	t.Helper()
	meta, err := xdr.NewTransactionMeta(3, xdr.TransactionMetaV3{
		SorobanMeta: &xdr.SorobanTransactionMeta{
			ReturnValue: xdr.ScVal{Type: xdr.ScValTypeScvVoid},
		},
	})
	require.NoError(t, err)
	b64, err := xdr.MarshalBase64(meta)
	require.NoError(t, err)
	return b64
}

func successGetTxResponse(t *testing.T) protocolrpc.GetTransactionResponse {
	t.Helper()
	return protocolrpc.GetTransactionResponse{
		TransactionDetails: protocolrpc.TransactionDetails{
			Status:        "SUCCESS",
			ResultMetaXDR: testTransactionMetaB64(t),
		},
	}
}

func newTestDeployer(t *testing.T, mock *mockRPC) *Deployer {
	t.Helper()
	return &Deployer{
		rpcClient:         mock,
		networkPassphrase: "Test SDF Network ; September 2015",
		signer:            keypair.MustRandom(),
		accountSequence:   100,
		autoRestore:       true,
	}
}

func testInvokeOp(signerAddr string) txnbuild.Operation {
	var contractId xdr.ContractId
	return &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: xdr.ScAddress{
					Type:       xdr.ScAddressTypeScAddressTypeContract,
					ContractId: &contractId,
				},
				FunctionName: xdr.ScSymbol("noop"),
			},
		},
		SourceAccount: signerAddr,
	}
}

func mustMarshalScValBase64(t *testing.T, v xdr.ScVal) string {
	t.Helper()
	b64, err := xdr.MarshalBase64(v)
	require.NoError(t, err)
	return b64
}

func ptrU32(v uint32) *xdr.Uint32 {
	x := xdr.Uint32(v)
	return &x
}

func randomContractID(t *testing.T) string {
	t.Helper()
	var b [32]byte
	_, err := rand.Read(b[:])
	require.NoError(t, err)
	id, err := strkey.Encode(strkey.VersionByteContract, b[:])
	require.NoError(t, err)
	return id
}

// ---------------------------------------------------------------------------
// restoreFootprint
// ---------------------------------------------------------------------------

func TestRestoreFootprint_Success(t *testing.T) {
	sorobanB64 := testSorobanDataB64(t)

	mock := &mockRPC{
		SendTransactionFn: func(_ context.Context, _ protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error) {
			return protocolrpc.SendTransactionResponse{
				Status: "PENDING",
				Hash:   "abc123",
			}, nil
		},
		GetTransactionFn: func(_ context.Context, _ protocolrpc.GetTransactionRequest) (protocolrpc.GetTransactionResponse, error) {
			return successGetTxResponse(t), nil
		},
	}

	d := newTestDeployer(t, mock)
	seqBefore := d.accountSequence

	err := d.restoreFootprint(context.Background(), protocolrpc.RestorePreamble{
		TransactionDataXDR: sorobanB64,
		MinResourceFee:     50000,
	})
	require.NoError(t, err)
	assert.Equal(t, seqBefore+1, d.accountSequence, "account sequence should increment after restore")
}

func TestRestoreFootprint_SubmitError(t *testing.T) {
	sorobanB64 := testSorobanDataB64(t)

	mock := &mockRPC{
		SendTransactionFn: func(_ context.Context, _ protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error) {
			return protocolrpc.SendTransactionResponse{
				Status:         "ERROR",
				ErrorResultXDR: "AAAA",
			}, nil
		},
	}

	d := newTestDeployer(t, mock)
	err := d.restoreFootprint(context.Background(), protocolrpc.RestorePreamble{
		TransactionDataXDR: sorobanB64,
		MinResourceFee:     50000,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restore transaction rejected")
}

func TestRestoreFootprint_TransactionFailed(t *testing.T) {
	sorobanB64 := testSorobanDataB64(t)

	mock := &mockRPC{
		SendTransactionFn: func(_ context.Context, _ protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error) {
			return protocolrpc.SendTransactionResponse{
				Status: "PENDING",
				Hash:   "abc123",
			}, nil
		},
		GetTransactionFn: func(_ context.Context, _ protocolrpc.GetTransactionRequest) (protocolrpc.GetTransactionResponse, error) {
			return protocolrpc.GetTransactionResponse{
				TransactionDetails: protocolrpc.TransactionDetails{
					Status: "FAILED",
				},
			}, nil
		},
	}

	d := newTestDeployer(t, mock)
	err := d.restoreFootprint(context.Background(), protocolrpc.RestorePreamble{
		TransactionDataXDR: sorobanB64,
		MinResourceFee:     50000,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restore transaction failed")
}

func TestRestoreFootprint_InvalidPreambleXDR(t *testing.T) {
	mock := &mockRPC{}
	d := newTestDeployer(t, mock)

	err := d.restoreFootprint(context.Background(), protocolrpc.RestorePreamble{
		TransactionDataXDR: "not-valid-base64-xdr!!",
		MinResourceFee:     50000,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode restore preamble soroban data")
}

// ---------------------------------------------------------------------------
// buildAndSubmitTransaction — restore flow
// ---------------------------------------------------------------------------

func TestBuildAndSubmitTransaction_WithRestore(t *testing.T) {
	sorobanB64 := testSorobanDataB64(t)

	var simCount atomic.Int32
	var sendCount atomic.Int32
	var getTxCount atomic.Int32

	mock := &mockRPC{
		SimulateTransactionFn: func(_ context.Context, _ protocolrpc.SimulateTransactionRequest) (protocolrpc.SimulateTransactionResponse, error) {
			call := simCount.Add(1)
			if call == 1 {
				return protocolrpc.SimulateTransactionResponse{
					RestorePreamble: &protocolrpc.RestorePreamble{
						TransactionDataXDR: sorobanB64,
						MinResourceFee:     50000,
					},
					TransactionDataXDR: sorobanB64,
					MinResourceFee:     100000,
				}, nil
			}
			return protocolrpc.SimulateTransactionResponse{
				TransactionDataXDR: sorobanB64,
				MinResourceFee:     100000,
			}, nil
		},
		SendTransactionFn: func(_ context.Context, _ protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error) {
			call := sendCount.Add(1)
			return protocolrpc.SendTransactionResponse{
				Status: "PENDING",
				Hash:   fmt.Sprintf("hash-%d", call),
			}, nil
		},
		GetTransactionFn: func(_ context.Context, _ protocolrpc.GetTransactionRequest) (protocolrpc.GetTransactionResponse, error) {
			getTxCount.Add(1)
			return successGetTxResponse(t), nil
		},
	}

	d := newTestDeployer(t, mock)
	seqBefore := d.accountSequence

	op := testInvokeOp(d.signer.Address())
	src := &txnbuild.SimpleAccount{AccountID: d.signer.Address(), Sequence: d.accountSequence}

	meta, err := d.buildAndSubmitTransaction(context.Background(), src, op)
	require.NoError(t, err)
	require.NotNil(t, meta)

	assert.EqualValues(t, 2, simCount.Load(), "should simulate twice (before and after restore)")
	assert.EqualValues(t, 2, sendCount.Load(), "should send two transactions (restore + invoke)")
	assert.EqualValues(t, 2, getTxCount.Load(), "should poll two transactions")
	assert.Equal(t, seqBefore+2, d.accountSequence, "sequence should increment for both restore and invoke")
}

func TestBuildAndSubmitTransaction_NoRestore(t *testing.T) {
	sorobanB64 := testSorobanDataB64(t)

	var simCount atomic.Int32

	mock := &mockRPC{
		SimulateTransactionFn: func(_ context.Context, _ protocolrpc.SimulateTransactionRequest) (protocolrpc.SimulateTransactionResponse, error) {
			simCount.Add(1)
			return protocolrpc.SimulateTransactionResponse{
				TransactionDataXDR: sorobanB64,
				MinResourceFee:     100000,
			}, nil
		},
		SendTransactionFn: func(_ context.Context, _ protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error) {
			return protocolrpc.SendTransactionResponse{
				Status: "PENDING",
				Hash:   "hash-1",
			}, nil
		},
		GetTransactionFn: func(_ context.Context, _ protocolrpc.GetTransactionRequest) (protocolrpc.GetTransactionResponse, error) {
			return successGetTxResponse(t), nil
		},
	}

	d := newTestDeployer(t, mock)

	op := testInvokeOp(d.signer.Address())
	src := &txnbuild.SimpleAccount{AccountID: d.signer.Address(), Sequence: d.accountSequence}

	meta, err := d.buildAndSubmitTransaction(context.Background(), src, op)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.EqualValues(t, 1, simCount.Load(), "should simulate exactly once when no restore needed")
}

func TestBuildAndSubmitTransaction_RestoreFails(t *testing.T) {
	sorobanB64 := testSorobanDataB64(t)

	mock := &mockRPC{
		SimulateTransactionFn: func(_ context.Context, _ protocolrpc.SimulateTransactionRequest) (protocolrpc.SimulateTransactionResponse, error) {
			return protocolrpc.SimulateTransactionResponse{
				RestorePreamble: &protocolrpc.RestorePreamble{
					TransactionDataXDR: sorobanB64,
					MinResourceFee:     50000,
				},
				TransactionDataXDR: sorobanB64,
				MinResourceFee:     100000,
			}, nil
		},
		SendTransactionFn: func(_ context.Context, _ protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error) {
			return protocolrpc.SendTransactionResponse{}, fmt.Errorf("network error")
		},
	}

	d := newTestDeployer(t, mock)

	op := testInvokeOp(d.signer.Address())
	src := &txnbuild.SimpleAccount{AccountID: d.signer.Address(), Sequence: d.accountSequence}

	_, err := d.buildAndSubmitTransaction(context.Background(), src, op)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to restore expired ledger entries")
}

// ---------------------------------------------------------------------------
// SimulateContract — restore flow
// ---------------------------------------------------------------------------

func TestSimulateContract_WithRestore(t *testing.T) {
	sorobanB64 := testSorobanDataB64(t)
	returnValueXDR := mustMarshalScValBase64(t, xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: ptrU32(42)})

	var simCount atomic.Int32

	mock := &mockRPC{
		SimulateTransactionFn: func(_ context.Context, _ protocolrpc.SimulateTransactionRequest) (protocolrpc.SimulateTransactionResponse, error) {
			call := simCount.Add(1)
			if call == 1 {
				return protocolrpc.SimulateTransactionResponse{
					RestorePreamble: &protocolrpc.RestorePreamble{
						TransactionDataXDR: sorobanB64,
						MinResourceFee:     50000,
					},
				}, nil
			}
			return protocolrpc.SimulateTransactionResponse{
				Results: []protocolrpc.SimulateHostFunctionResult{
					{ReturnValueXDR: &returnValueXDR},
				},
			}, nil
		},
		SendTransactionFn: func(_ context.Context, _ protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error) {
			return protocolrpc.SendTransactionResponse{
				Status: "PENDING",
				Hash:   "restore-hash",
			}, nil
		},
		GetTransactionFn: func(_ context.Context, _ protocolrpc.GetTransactionRequest) (protocolrpc.GetTransactionResponse, error) {
			return successGetTxResponse(t), nil
		},
	}

	d := newTestDeployer(t, mock)

	result, err := d.SimulateContract(context.Background(), randomContractID(t), "get_value", nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	v, ok := result.GetU32()
	require.True(t, ok)
	assert.EqualValues(t, 42, v)
	assert.EqualValues(t, 2, simCount.Load(), "should simulate twice (before and after restore)")
}

func TestSimulateContract_NoRestore(t *testing.T) {
	returnValueXDR := mustMarshalScValBase64(t, xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: ptrU32(7)})

	mock := &mockRPC{
		SimulateTransactionFn: func(_ context.Context, _ protocolrpc.SimulateTransactionRequest) (protocolrpc.SimulateTransactionResponse, error) {
			return protocolrpc.SimulateTransactionResponse{
				Results: []protocolrpc.SimulateHostFunctionResult{
					{ReturnValueXDR: &returnValueXDR},
				},
			}, nil
		},
	}

	d := newTestDeployer(t, mock)

	result, err := d.SimulateContract(context.Background(), randomContractID(t), "get_value", nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	v, ok := result.GetU32()
	require.True(t, ok)
	assert.EqualValues(t, 7, v)
}

// ---------------------------------------------------------------------------
// WithAutoRestore toggle
// ---------------------------------------------------------------------------

func TestWithAutoRestore_Option(t *testing.T) {
	kp := keypair.MustRandom()
	d := NewDeployer(nil, "Test", kp)
	assert.True(t, d.autoRestore, "auto-restore should be enabled by default")

	d2 := NewDeployer(nil, "Test", kp, WithAutoRestore(false))
	assert.False(t, d2.autoRestore, "auto-restore should be disabled when opted out")

	d3 := NewDeployer(nil, "Test", kp, WithAutoRestore(true))
	assert.True(t, d3.autoRestore)
}

func TestBuildAndSubmitTransaction_AutoRestoreDisabled(t *testing.T) {
	sorobanB64 := testSorobanDataB64(t)

	var simCount atomic.Int32

	mock := &mockRPC{
		SimulateTransactionFn: func(_ context.Context, _ protocolrpc.SimulateTransactionRequest) (protocolrpc.SimulateTransactionResponse, error) {
			simCount.Add(1)
			// Always return a RestorePreamble — but the deployer should
			// ignore it because autoRestore is off.
			return protocolrpc.SimulateTransactionResponse{
				RestorePreamble: &protocolrpc.RestorePreamble{
					TransactionDataXDR: sorobanB64,
					MinResourceFee:     50000,
				},
				TransactionDataXDR: sorobanB64,
				MinResourceFee:     100000,
			}, nil
		},
		SendTransactionFn: func(_ context.Context, _ protocolrpc.SendTransactionRequest) (protocolrpc.SendTransactionResponse, error) {
			return protocolrpc.SendTransactionResponse{
				Status: "PENDING",
				Hash:   "hash-1",
			}, nil
		},
		GetTransactionFn: func(_ context.Context, _ protocolrpc.GetTransactionRequest) (protocolrpc.GetTransactionResponse, error) {
			return successGetTxResponse(t), nil
		},
	}

	d := newTestDeployer(t, mock)
	d.autoRestore = false

	op := testInvokeOp(d.signer.Address())
	src := &txnbuild.SimpleAccount{AccountID: d.signer.Address(), Sequence: d.accountSequence}

	meta, err := d.buildAndSubmitTransaction(context.Background(), src, op)
	require.NoError(t, err)
	require.NotNil(t, meta)

	assert.EqualValues(t, 1, simCount.Load(), "should simulate only once — restore path must be skipped")
}

func TestSimulateContract_AutoRestoreDisabled(t *testing.T) {
	sorobanB64 := testSorobanDataB64(t)
	returnValueXDR := mustMarshalScValBase64(t, xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: ptrU32(99)})

	var simCount atomic.Int32

	mock := &mockRPC{
		SimulateTransactionFn: func(_ context.Context, _ protocolrpc.SimulateTransactionRequest) (protocolrpc.SimulateTransactionResponse, error) {
			simCount.Add(1)
			return protocolrpc.SimulateTransactionResponse{
				RestorePreamble: &protocolrpc.RestorePreamble{
					TransactionDataXDR: sorobanB64,
					MinResourceFee:     50000,
				},
				Results: []protocolrpc.SimulateHostFunctionResult{
					{ReturnValueXDR: &returnValueXDR},
				},
			}, nil
		},
	}

	d := newTestDeployer(t, mock)
	d.autoRestore = false

	result, err := d.SimulateContract(context.Background(), randomContractID(t), "get_value", nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	v, ok := result.GetU32()
	require.True(t, ok)
	assert.EqualValues(t, 99, v, "should return simulation result without restoring")
	assert.EqualValues(t, 1, simCount.Load(), "should simulate only once — restore path must be skipped")
}
