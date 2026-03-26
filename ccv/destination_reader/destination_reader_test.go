package destinationreader

import (
	"context"
	"io"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	offrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/offramp"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	"github.com/smartcontractkit/chainlink-stellar/internal/mocks"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testOffRampContractID  = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	testRMNRemoteContractID = "CDXHXQJHQVFBPHKBYB73MA5KZAWWHBXWZRKQJDTY4ZYR3GYQHXADOKEP"
)

func testLogger(t *testing.T) *zerolog.Logger {
	t.Helper()
	z := zerolog.New(io.Discard).Level(zerolog.Disabled)
	return &z
}

func testRPCClient(t *testing.T) *rpcclient.Client {
	t.Helper()
	// Non-nil client; no network calls until Start/poll paths run.
	return rpcclient.NewClient("http://127.0.0.1:9", &http.Client{Timeout: time.Millisecond})
}

func mustTestMessage(t *testing.T) protocol.Message {
	t.Helper()
	onRamp, err := protocol.RandomAddress()
	require.NoError(t, err)
	offRamp, err := protocol.RandomAddress()
	require.NoError(t, err)
	sender, err := protocol.RandomAddress()
	require.NoError(t, err)
	receiver, err := protocol.RandomAddress()
	require.NoError(t, err)
	msg, err := protocol.NewMessage(
		protocol.ChainSelector(1337),
		protocol.ChainSelector(2337),
		protocol.SequenceNumber(1),
		onRamp,
		offRamp,
		protocol.Finality(0),
		200_000,
		100_000,
		protocol.Bytes32{},
		sender,
		receiver,
		nil,
		[]byte("data"),
		nil,
	)
	require.NoError(t, err)
	return *msg
}

func TestNewDestinationReader_validation(t *testing.T) {
	lggr := testLogger(t)
	rpc := testRPCClient(t)
	inv := mocks.NewMockInvoker(t)

	tests := []struct {
		name    string
		invoker *mocks.MockInvoker
		rpc     *rpcclient.Client
		off     string
		rmn     string
		lg      *zerolog.Logger
		wantErr string
	}{
		{name: "nil invoker", invoker: nil, rpc: rpc, off: testOffRampContractID, rmn: testRMNRemoteContractID, lg: lggr, wantErr: "invoker is required"},
		{name: "nil rpc", invoker: inv, rpc: nil, off: testOffRampContractID, rmn: testRMNRemoteContractID, lg: lggr, wantErr: "rpc client is required"},
		{name: "empty offramp", invoker: inv, rpc: rpc, off: "", rmn: testRMNRemoteContractID, lg: lggr, wantErr: "offramp contract ID is required"},
		{name: "empty rmn", invoker: inv, rpc: rpc, off: testOffRampContractID, rmn: "", lg: lggr, wantErr: "rmn remote contract ID is required"},
		{name: "nil logger", invoker: inv, rpc: rpc, off: testOffRampContractID, rmn: testRMNRemoteContractID, lg: nil, wantErr: "logger is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := New(tt.invoker, tt.rpc, tt.off, tt.rmn, tt.lg, time.Minute)
			require.Error(t, err)
			require.Nil(t, d)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNewDestinationReader_success(t *testing.T) {
	inv := mocks.NewMockInvoker(t)
	d, err := New(inv, testRPCClient(t), testOffRampContractID, testRMNRemoteContractID, testLogger(t), time.Minute)
	require.NoError(t, err)
	require.NotNil(t, d)
	err = d.Close()
	require.NoError(t, err)
}

func TestNewStellarExecutionAttemptPoller_validation(t *testing.T) {
	rpc := testRPCClient(t)
	lg := testLogger(t)

	t.Run("nil rpc", func(t *testing.T) {
		p, err := NewStellarExecutionAttemptPoller(nil, testOffRampContractID, lg, time.Minute)
		require.Error(t, err)
		require.Nil(t, p)
		assert.Contains(t, err.Error(), "rpc client cannot be nil")
	})

	t.Run("nil logger", func(t *testing.T) {
		p, err := NewStellarExecutionAttemptPoller(rpc, testOffRampContractID, nil, time.Minute)
		require.Error(t, err)
		require.Nil(t, p)
		assert.Contains(t, err.Error(), "logger cannot be nil")
	})

	t.Run("empty contract id", func(t *testing.T) {
		p, err := NewStellarExecutionAttemptPoller(rpc, "", lg, time.Minute)
		require.Error(t, err)
		require.Nil(t, p)
		assert.Contains(t, err.Error(), "offramp contract ID cannot be empty")
	})
}

func TestDestinationReader_GetMessageSuccess(t *testing.T) {
	ctx := context.Background()
	msg := mustTestMessage(t)
	msgID, err := msg.MessageID()
	require.NoError(t, err)

	t.Run("returns true when execution state is success", func(t *testing.T) {
		inv := mocks.NewMockInvoker(t)
		stateScVal, err := offrampbindings.MessageExecutionStateSuccess.ToScVal()
		require.NoError(t, err)
		inv.On("SimulateContract", mock.Anything, testOffRampContractID, "get_execution_state", mock.MatchedBy(func(args []xdr.ScVal) bool {
			require.Len(t, args, 1)
			id, err := scval.Bytes32FromScVal(args[0])
			require.NoError(t, err)
			return id == msgID
		}))).Return(&stateScVal, nil).Once()

		d, err := New(inv, testRPCClient(t), testOffRampContractID, testRMNRemoteContractID, testLogger(t), time.Minute)
		require.NoError(t, err)
		t.Cleanup(func() { _ = d.Close() })

		ok, err := d.GetMessageSuccess(ctx, msg)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("returns false when execution state is not success", func(t *testing.T) {
		inv := mocks.NewMockInvoker(t)
		stateScVal, err := offrampbindings.MessageExecutionStateUntouched.ToScVal()
		require.NoError(t, err)
		inv.On("SimulateContract", mock.Anything, testOffRampContractID, "get_execution_state", mock.Anything).
			Return(&stateScVal, nil).Once()

		d, err := New(inv, testRPCClient(t), testOffRampContractID, testRMNRemoteContractID, testLogger(t), time.Minute)
		require.NoError(t, err)
		t.Cleanup(func() { _ = d.Close() })

		ok, err := d.GetMessageSuccess(ctx, msg)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("wraps simulate error", func(t *testing.T) {
		inv := mocks.NewMockInvoker(t)
		inv.On("SimulateContract", mock.Anything, testOffRampContractID, "get_execution_state", mock.Anything).
			Return((*xdr.ScVal)(nil), assert.AnError).Once()

		d, err := New(inv, testRPCClient(t), testOffRampContractID, testRMNRemoteContractID, testLogger(t), time.Minute)
		require.NoError(t, err)
		t.Cleanup(func() { _ = d.Close() })

		_, err = d.GetMessageSuccess(ctx, msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get execution state")
	})
}

func TestDestinationReader_GetCCVSForMessage(t *testing.T) {
	ctx := context.Background()
	msg := mustTestMessage(t)

	t.Run("maps lane-mandated and default CCVs and sets optional threshold", func(t *testing.T) {
		inv := mocks.NewMockInvoker(t)
		cfg := offrampbindings.SourceChainConfig{
			DefaultCcvs:      []string{testOffRampContractID},
			IsEnabled:        true,
			LaneMandatedCcvs: []string{testRMNRemoteContractID},
			OnRamps:          [][]byte{{0x01}},
			Router:           testOffRampContractID,
		}
		cfgScVal, err := cfg.ToScVal()
		require.NoError(t, err)
		inv.On("SimulateContract", mock.Anything, testOffRampContractID, "get_source_chain_config", mock.Anything).
			Return(&cfgScVal, nil).Once()

		d, err := New(inv, testRPCClient(t), testOffRampContractID, testRMNRemoteContractID, testLogger(t), time.Minute)
		require.NoError(t, err)
		t.Cleanup(func() { _ = d.Close() })

		info, err := d.GetCCVSForMessage(ctx, msg)
		require.NoError(t, err)
		require.Len(t, info.RequiredCCVs, 1)
		require.Len(t, info.OptionalCCVs, 1)
		assert.Equal(t, uint8(1), info.OptionalThreshold)

		reqParsed := scval.ParseAddress(testRMNRemoteContractID)
		require.NotNil(t, reqParsed)
		assert.Equal(t, protocol.UnknownAddress((*reqParsed.ContractId)[:]), info.RequiredCCVs[0])

		optParsed := scval.ParseAddress(testOffRampContractID)
		require.NotNil(t, optParsed)
		assert.Equal(t, protocol.UnknownAddress((*optParsed.ContractId)[:]), info.OptionalCCVs[0])
	})

	t.Run("optional threshold zero when no default CCVs", func(t *testing.T) {
		inv := mocks.NewMockInvoker(t)
		cfg := offrampbindings.SourceChainConfig{
			DefaultCcvs:      nil,
			IsEnabled:        true,
			LaneMandatedCcvs: []string{testRMNRemoteContractID},
			OnRamps:          nil,
			Router:           testOffRampContractID,
		}
		cfgScVal, err := cfg.ToScVal()
		require.NoError(t, err)
		inv.On("SimulateContract", mock.Anything, testOffRampContractID, "get_source_chain_config", mock.Anything).
			Return(&cfgScVal, nil).Once()

		d, err := New(inv, testRPCClient(t), testOffRampContractID, testRMNRemoteContractID, testLogger(t), time.Minute)
		require.NoError(t, err)
		t.Cleanup(func() { _ = d.Close() })

		info, err := d.GetCCVSForMessage(ctx, msg)
		require.NoError(t, err)
		require.Len(t, info.RequiredCCVs, 1)
		require.Empty(t, info.OptionalCCVs)
		assert.Equal(t, uint8(0), info.OptionalThreshold)
	})

	t.Run("fails when mandated address cannot be parsed", func(t *testing.T) {
		inv := mocks.NewMockInvoker(t)
		cfg := offrampbindings.SourceChainConfig{
			LaneMandatedCcvs: []string{"not-a-stellar-address"},
			Router:           testOffRampContractID,
		}
		cfgScVal, err := cfg.ToScVal()
		require.NoError(t, err)
		inv.On("SimulateContract", mock.Anything, testOffRampContractID, "get_source_chain_config", mock.Anything).
			Return(&cfgScVal, nil).Once()

		d, err := New(inv, testRPCClient(t), testOffRampContractID, testRMNRemoteContractID, testLogger(t), time.Minute)
		require.NoError(t, err)
		t.Cleanup(func() { _ = d.Close() })

		_, err = d.GetCCVSForMessage(ctx, msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse address")
	})
}

func TestDestinationReader_GetRMNCursedSubjects(t *testing.T) {
	ctx := context.Background()

	t.Run("returns decoded subjects", func(t *testing.T) {
		inv := mocks.NewMockInvoker(t)
		s1 := [16]byte{1}
		s2 := [16]byte{2}
		vecScVal := scval.Bytes16SliceToScVal([][16]byte{s1, s2})
		inv.On("SimulateContract", mock.Anything, testRMNRemoteContractID, "get_cursed_subjects", mock.Anything).
			Return(&vecScVal, nil).Once()

		d, err := New(inv, testRPCClient(t), testOffRampContractID, testRMNRemoteContractID, testLogger(t), time.Minute)
		require.NoError(t, err)
		t.Cleanup(func() { _ = d.Close() })

		out, err := d.GetRMNCursedSubjects(ctx)
		require.NoError(t, err)
		require.Len(t, out, 2)
		assert.Equal(t, protocol.Bytes16(s1), out[0])
		assert.Equal(t, protocol.Bytes16(s2), out[1])
	})

	t.Run("wraps simulate error", func(t *testing.T) {
		inv := mocks.NewMockInvoker(t)
		inv.On("SimulateContract", mock.Anything, testRMNRemoteContractID, "get_cursed_subjects", mock.Anything).
			Return((*xdr.ScVal)(nil), assert.AnError).Once()

		d, err := New(inv, testRPCClient(t), testOffRampContractID, testRMNRemoteContractID, testLogger(t), time.Minute)
		require.NoError(t, err)
		t.Cleanup(func() { _ = d.Close() })

		_, err = d.GetRMNCursedSubjects(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get cursed subjects")
	})
}

func TestDestinationReader_serviceSurface(t *testing.T) {
	inv := mocks.NewMockInvoker(t)
	d, err := New(inv, testRPCClient(t), testOffRampContractID, testRMNRemoteContractID, testLogger(t), time.Minute)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	assert.Equal(t, "StellarDestinationReader", d.Name())
	require.NoError(t, d.Ready())

	report := d.HealthReport()
	assert.Contains(t, report, "StellarDestinationReader")
	assert.Nil(t, report["StellarDestinationReader"])
	assert.Contains(t, report, stellarPollerServiceName)
}

func TestStellarExecutionAttemptPoller_GetExecutionAttempts_cacheMiss(t *testing.T) {
	p, err := NewStellarExecutionAttemptPoller(testRPCClient(t), testOffRampContractID, testLogger(t), time.Minute)
	require.NoError(t, err)

	msg := mustTestMessage(t)
	attempts, err := p.GetExecutionAttempts(context.Background(), msg)
	require.NoError(t, err)
	assert.Nil(t, attempts)
}

func TestDecodeExecuteArgsToAttempt(t *testing.T) {
	t.Run("wrong arg count", func(t *testing.T) {
		_, err := decodeExecuteArgsToAttempt([]xdr.ScVal{{}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected 4 args")
	})

	t.Run("success round trip", func(t *testing.T) {
		msg := mustTestMessage(t)
		encoded, err := msg.Encode()
		require.NoError(t, err)

		ccvAddr := testOffRampContractID
		args := []xdr.ScVal{
			scval.BytesToScVal(encoded),
			scval.AddressSliceToScVal([]string{ccvAddr}),
			scval.BytesSliceToScVal([][]byte{{0xab, 0xcd}}),
			scval.Uint32ToScVal(4242),
		}

		attempt, err := decodeExecuteArgsToAttempt(args)
		require.NoError(t, err)
		require.NotNil(t, attempt)
		assert.Equal(t, big.NewInt(4242), attempt.TransactionGasLimit)
		idWant, err := msg.MessageID()
		require.NoError(t, err)
		idGot := attempt.Report.Message.MustMessageID()
		assert.Equal(t, idWant, idGot)
		require.Len(t, attempt.Report.CCVData, 1)
		assert.Equal(t, []byte{0xab, 0xcd}, []byte(attempt.Report.CCVData[0]))
	})
}

func TestExtractInvokeContractArgs_invalidEnvelope(t *testing.T) {
	_, err := extractInvokeContractArgs("not-valid-base64-!!!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal transaction envelope")
}
