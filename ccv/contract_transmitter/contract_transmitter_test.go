package contracttransmitter

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-ccv/executor"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-stellar/internal/mocks"
)

func testLogger() *zerolog.Logger {
	lggr := zerolog.New(os.Stdout).With().Timestamp().Logger().Level(zerolog.DebugLevel)
	return &lggr
}

func testAddress(index byte) []byte {
	addr := make([]byte, 32)
	addr[0] = index
	return addr
}

func mustCreateMessage(t *testing.T, sourceChain, destChain uint64, seqNum uint64, gasLimit uint32) protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(
		protocol.ChainSelector(sourceChain),
		protocol.ChainSelector(destChain),
		protocol.SequenceNumber(seqNum),
		testAddress(0x01), // onRampAddress
		testAddress(0x02), // offRampAddress
		1,                 // finality
		gasLimit,
		gasLimit,           // ccipReceiveGasLimit
		protocol.Bytes32{}, // ccvAndExecutorHash
		testAddress(0x03),  // sender
		testAddress(0x04),  // receiver
		[]byte{},           // destBlob
		[]byte("test"),     // data
		nil,                // tokenTransfer
	)
	require.NoError(t, err)
	return *msg
}

// ---------------------------------------------------------------------------
// ConvertAndWriteMessageToChain tests
// ---------------------------------------------------------------------------

func TestConvertAndWriteMessageToChain(t *testing.T) {
	lggr := testLogger()
	offRampID := "COFFRAMP"

	ccv1 := testAddress(0xAA)
	ccv2 := testAddress(0xBB)

	t.Run("successful transmission", func(t *testing.T) {
		mockInvoker := mocks.NewMockInvoker(t)

		var capturedContractID, capturedFuncName string
		var capturedArgs []xdr.ScVal

		mockInvoker.On("InvokeContract", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
			Run(func(args mock.Arguments) {
				capturedContractID = args.Get(1).(string)
				capturedFuncName = args.Get(2).(string)
				capturedArgs = args.Get(3).([]xdr.ScVal)
			}).
			Return((*xdr.ScVal)(nil), nil).
			Once()

		ct, err := NewContractTransmitterWithClient(mockInvoker, offRampID, "statechanged", "RMNREMOTE", lggr)
		require.NoError(t, err)

		msg := mustCreateMessage(t, 1, 2, 100, 500000)
		report := protocol.AbstractAggregatedReport{
			CCVS:    []protocol.UnknownAddress{ccv1, ccv2},
			CCVData: [][]byte{{0x01, 0x02}, {0x03, 0x04}},
			Message: msg,
		}

		err = ct.ConvertAndWriteMessageToChain(context.Background(), report)
		require.NoError(t, err)

		assert.Equal(t, offRampID, capturedContractID)
		assert.Equal(t, "execute", capturedFuncName)
		require.Len(t, capturedArgs, 4, "execute expects 4 args: encoded_message, ccvs, verifier_results, gas_limit_override")

		// arg[0]: encoded_message (Bytes)
		encodedMsg, ok := capturedArgs[0].GetBytes()
		assert.True(t, ok, "first arg should be bytes")
		expectedEncodedMsg, _ := msg.Encode()
		assert.Equal(t, xdr.ScBytes(expectedEncodedMsg), encodedMsg)

		// arg[1]: ccvs (Vec<Address>)
		ccvsVec, ok := capturedArgs[1].GetVec()
		require.True(t, ok, "second arg should be a vec")
		require.NotNil(t, ccvsVec)
		assert.Len(t, *ccvsVec, 2)

		// arg[2]: verifier_results (Vec<Bytes>)
		resultsVec, ok := capturedArgs[2].GetVec()
		require.True(t, ok, "third arg should be a vec")
		require.NotNil(t, resultsVec)
		assert.Len(t, *resultsVec, 2)

		// arg[3]: gas_limit_override (u32)
		gasOverride, ok := capturedArgs[3].GetU32()
		assert.True(t, ok, "fourth arg should be u32")
		assert.Equal(t, xdr.Uint32(DefaultGasLimitOverride), gasOverride)
	})

	t.Run("empty CCVs and CCVData", func(t *testing.T) {
		mockInvoker := mocks.NewMockInvoker(t)

		var capturedArgs []xdr.ScVal
		mockInvoker.On("InvokeContract", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
			Run(func(args mock.Arguments) {
				capturedArgs = args.Get(3).([]xdr.ScVal)
			}).
			Return((*xdr.ScVal)(nil), nil).
			Once()

		ct, err := NewContractTransmitterWithClient(mockInvoker, offRampID, "statechanged", "RMNREMOTE", lggr)
		require.NoError(t, err)

		report := protocol.AbstractAggregatedReport{
			CCVS:    []protocol.UnknownAddress{},
			CCVData: [][]byte{},
			Message: mustCreateMessage(t, 1, 2, 101, 300000),
		}

		err = ct.ConvertAndWriteMessageToChain(context.Background(), report)
		require.NoError(t, err)

		// ccvs vec should be empty
		ccvsVec, ok := capturedArgs[1].GetVec()
		require.True(t, ok)
		assert.Len(t, *ccvsVec, 0)

		// verifier_results vec should be empty
		resultsVec, ok := capturedArgs[2].GetVec()
		require.True(t, ok)
		assert.Len(t, *resultsVec, 0)
	})

	t.Run("message encoding error returns ErrMessageEncoding", func(t *testing.T) {
		mockInvoker := mocks.NewMockInvoker(t)

		ct, err := NewContractTransmitterWithClient(mockInvoker, offRampID, "statechanged", "RMNREMOTE", lggr)
		require.NoError(t, err)

		report := protocol.AbstractAggregatedReport{
			Message: protocol.Message{
				SenderLength: 99, // mismatch triggers validation error
				Sender:       []byte{0x01},
			},
		}

		err = ct.ConvertAndWriteMessageToChain(context.Background(), report)
		require.Error(t, err)
		assert.True(t, errors.Is(err, executor.ErrMessageEncoding),
			"should wrap executor.ErrMessageEncoding so the executor skips retries")
	})

	t.Run("invoker error is propagated", func(t *testing.T) {
		mockInvoker := mocks.NewMockInvoker(t)

		invokeErr := errors.New("soroban rpc unavailable")
		mockInvoker.On("InvokeContract", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
			Return((*xdr.ScVal)(nil), invokeErr).
			Once()

		ct, err := NewContractTransmitterWithClient(mockInvoker, offRampID, "statechanged", "RMNREMOTE", lggr)
		require.NoError(t, err)

		report := protocol.AbstractAggregatedReport{
			CCVS:    []protocol.UnknownAddress{ccv1},
			CCVData: [][]byte{{0x01}},
			Message: mustCreateMessage(t, 1, 2, 103, 300000),
		}

		err = ct.ConvertAndWriteMessageToChain(context.Background(), report)
		require.Error(t, err)
		assert.ErrorIs(t, err, invokeErr)
		assert.Contains(t, err.Error(), "failed to call execute")
	})

	t.Run("invoker receives the correct contract ID", func(t *testing.T) {
		mockInvoker := mocks.NewMockInvoker(t)

		customOffRampID := "CDXHXQJHQVFBPHKBYB73MA5KZAWWHBXWZRKQJDTY4ZYR3GYQHXADOKEP"
		var capturedID string
		mockInvoker.On("InvokeContract", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
			Run(func(args mock.Arguments) {
				capturedID = args.Get(1).(string)
			}).
			Return((*xdr.ScVal)(nil), nil).
			Once()

		ct, err := NewContractTransmitterWithClient(mockInvoker, customOffRampID, "statechanged", "RMNREMOTE", lggr)
		require.NoError(t, err)

		report := protocol.AbstractAggregatedReport{
			CCVS:    []protocol.UnknownAddress{},
			CCVData: [][]byte{},
			Message: mustCreateMessage(t, 1, 2, 104, 300000),
		}

		err = ct.ConvertAndWriteMessageToChain(context.Background(), report)
		require.NoError(t, err)
		assert.Equal(t, customOffRampID, capturedID)
	})
}
