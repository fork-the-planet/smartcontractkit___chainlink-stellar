package contracttransmitter

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-ccv/executor"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
)

// mockInvoker is a test double for the Invoker interface.
type mockInvoker struct {
	invokeFunc func(ctx context.Context, contractID, functionName string, args []xdr.ScVal) (*xdr.ScVal, error)
}

func (m *mockInvoker) InvokeContract(ctx context.Context, contractID, functionName string, args []xdr.ScVal) (*xdr.ScVal, error) {
	if m.invokeFunc != nil {
		return m.invokeFunc(ctx, contractID, functionName, args)
	}
	return nil, nil
}

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
// Constructor tests
// ---------------------------------------------------------------------------

func TestNewContractTransmitter(t *testing.T) {
	lggr := testLogger()

	t.Run("returns error when invoker is nil", func(t *testing.T) {
		ct, err := NewContractTransmitter(nil, "COFFRAMP", lggr)
		require.Error(t, err)
		require.Nil(t, ct)
		assert.Contains(t, err.Error(), "invoker is required")
	})

	t.Run("returns error when offRampContractID is empty", func(t *testing.T) {
		ct, err := NewContractTransmitter(&mockInvoker{}, "", lggr)
		require.Error(t, err)
		require.Nil(t, ct)
		assert.Contains(t, err.Error(), "offramp contract ID is required")
	})

	t.Run("returns error when logger is nil", func(t *testing.T) {
		ct, err := NewContractTransmitter(&mockInvoker{}, "COFFRAMP", nil)
		require.Error(t, err)
		require.Nil(t, ct)
		assert.Contains(t, err.Error(), "logger is required")
	})

	t.Run("succeeds with valid inputs", func(t *testing.T) {
		ct, err := NewContractTransmitter(&mockInvoker{}, "COFFRAMP", lggr)
		require.NoError(t, err)
		require.NotNil(t, ct)
		assert.Equal(t, "COFFRAMP", ct.offRampContractID)
	})
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
		var capturedContractID, capturedFuncName string
		var capturedArgs []xdr.ScVal

		inv := &mockInvoker{
			invokeFunc: func(_ context.Context, contractID, functionName string, args []xdr.ScVal) (*xdr.ScVal, error) {
				capturedContractID = contractID
				capturedFuncName = functionName
				capturedArgs = args
				return nil, nil
			},
		}

		ct, err := NewContractTransmitter(inv, offRampID, lggr)
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

		expectedAddr1, _ := strkey.Encode(strkey.VersionByteContract, ccv1)
		expectedAddr2, _ := strkey.Encode(strkey.VersionByteContract, ccv2)
		assert.Equal(t, scval.AddressToScVal(expectedAddr1), (*ccvsVec)[0])
		assert.Equal(t, scval.AddressToScVal(expectedAddr2), (*ccvsVec)[1])

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
		var capturedArgs []xdr.ScVal
		inv := &mockInvoker{
			invokeFunc: func(_ context.Context, _, _ string, args []xdr.ScVal) (*xdr.ScVal, error) {
				capturedArgs = args
				return nil, nil
			},
		}

		ct, err := NewContractTransmitter(inv, offRampID, lggr)
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
		inv := &mockInvoker{}
		ct, err := NewContractTransmitter(inv, offRampID, lggr)
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

	t.Run("CCV address with wrong length returns error", func(t *testing.T) {
		inv := &mockInvoker{}
		ct, err := NewContractTransmitter(inv, offRampID, lggr)
		require.NoError(t, err)

		report := protocol.AbstractAggregatedReport{
			CCVS:    []protocol.UnknownAddress{[]byte{0x01, 0x02, 0x03}}, // only 3 bytes
			CCVData: [][]byte{{0xAA}},
			Message: mustCreateMessage(t, 1, 2, 102, 300000),
		}

		err = ct.ConvertAndWriteMessageToChain(context.Background(), report)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid length")
	})

	t.Run("invoker error is propagated", func(t *testing.T) {
		invokeErr := errors.New("soroban rpc unavailable")
		inv := &mockInvoker{
			invokeFunc: func(_ context.Context, _, _ string, _ []xdr.ScVal) (*xdr.ScVal, error) {
				return nil, invokeErr
			},
		}

		ct, err := NewContractTransmitter(inv, offRampID, lggr)
		require.NoError(t, err)

		report := protocol.AbstractAggregatedReport{
			CCVS:    []protocol.UnknownAddress{ccv1},
			CCVData: [][]byte{{0x01}},
			Message: mustCreateMessage(t, 1, 2, 103, 300000),
		}

		err = ct.ConvertAndWriteMessageToChain(context.Background(), report)
		require.Error(t, err)
		assert.ErrorIs(t, err, invokeErr)
		assert.Contains(t, err.Error(), "failed to invoke offramp execute")
	})

	t.Run("invoker receives the correct contract ID", func(t *testing.T) {
		customOffRampID := "CDXHXQJHQVFBPHKBYB73MA5KZAWWHBXWZRKQJDTY4ZYR3GYQHXADOKEP"
		var capturedID string
		inv := &mockInvoker{
			invokeFunc: func(_ context.Context, contractID, _ string, _ []xdr.ScVal) (*xdr.ScVal, error) {
				capturedID = contractID
				return nil, nil
			},
		}

		ct, err := NewContractTransmitter(inv, customOffRampID, lggr)
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

// ---------------------------------------------------------------------------
// unknownAddressesToStellarAddressScVals tests
// ---------------------------------------------------------------------------

func TestUnknownAddressesToStellarAddressScVals(t *testing.T) {
	t.Run("converts valid 32-byte addresses", func(t *testing.T) {
		addr := testAddress(0xFF)
		vals, err := unknownAddressesToStellarAddressScVals([]protocol.UnknownAddress{addr})
		require.NoError(t, err)
		require.Len(t, vals, 1)

		expectedStrkey, _ := strkey.Encode(strkey.VersionByteContract, addr)
		assert.Equal(t, scval.AddressToScVal(expectedStrkey), vals[0])
	})

	t.Run("returns error for invalid length", func(t *testing.T) {
		_, err := unknownAddressesToStellarAddressScVals([]protocol.UnknownAddress{[]byte{0x01}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "index 0")
		assert.Contains(t, err.Error(), "invalid length")
	})

	t.Run("empty slice returns empty result", func(t *testing.T) {
		vals, err := unknownAddressesToStellarAddressScVals(nil)
		require.NoError(t, err)
		assert.Len(t, vals, 0)
	})
}
