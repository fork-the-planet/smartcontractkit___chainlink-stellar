package txm

import (
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-common/pkg/beholder/beholdertest"
	"github.com/smartcontractkit/chainlink-common/pkg/config"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	commontypes "github.com/smartcontractkit/chainlink-common/pkg/types"
	protocolrpc "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/protocols/stellarcore"
	svrv1 "github.com/smartcontractkit/chainlink-protos/svr/v1"
)

const testChainID = "stellar:testnet"

func TestEmitTxMessage(t *testing.T) {
	t.Run("populates Soroban contract ID as ToAddress", func(t *testing.T) {
		ctx := t.Context()
		beholderTester := beholdertest.NewObserver(t)

		contractID, err := strkey.Encode(strkey.VersionByteContract, make([]byte, 32))
		require.NoError(t, err)
		fromAddress := "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF"
		expectedHash := "abc123hash"
		expectedSequence := int64(42)

		op, err := BuildInvokeContractOperation(contractID, "hello", nil, fromAddress)
		require.NoError(t, err)

		txmMetrics := NewStellarTxmMetrics(logger.Test(t), testChainID)
		tx := &StellarTx{
			FromAddress: fromAddress,
			Operations:  []txnbuild.Operation{op},
		}

		err = txmMetrics.EmitTxMessage(ctx, expectedHash, fromAddress, expectedSequence, tx)
		require.NoError(t, err)

		messages := beholderTester.Messages(t)
		require.Len(t, messages, 1)

		var actualMessage svrv1.TxMessage
		err = proto.Unmarshal(messages[0].Body, &actualMessage)
		require.NoError(t, err)

		assert.Equal(t, expectedHash, actualMessage.Hash)
		assert.Equal(t, fromAddress, actualMessage.FromAddress)
		assert.Equal(t, contractID, actualMessage.ToAddress)
		assert.Equal(t, strconv.FormatInt(expectedSequence, 10), actualMessage.Nonce)
		assert.Equal(t, testChainID, actualMessage.ChainId)
		assert.Equal(t, "", actualMessage.FeedAddress)
	})

	t.Run("leaves ToAddress empty when no invoke operation", func(t *testing.T) {
		ctx := t.Context()
		beholderTester := beholdertest.NewObserver(t)

		fromAddress := "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF"
		txmMetrics := NewStellarTxmMetrics(logger.Test(t), testChainID)

		err := txmMetrics.EmitTxMessage(ctx, "hash", fromAddress, 1, &StellarTx{FromAddress: fromAddress})
		require.NoError(t, err)

		messages := beholderTester.Messages(t)
		require.Len(t, messages, 1)

		var actualMessage svrv1.TxMessage
		err = proto.Unmarshal(messages[0].Body, &actualMessage)
		require.NoError(t, err)

		assert.Equal(t, "", actualMessage.ToAddress)
	})
}

func TestReachedMaxAttempts(t *testing.T) {
	ctx := t.Context()
	txmMetrics := NewStellarTxmMetrics(logger.Test(t), testChainID)

	txmMetrics.ReachedMaxAttempts(ctx, true)
	value := testutil.ToFloat64(promStellarTxmReachedMaxAttempts.WithLabelValues(testChainID))
	require.InEpsilon(t, float64(1), value, 0.00001)

	txmMetrics.ReachedMaxAttempts(ctx, false)
	value = testutil.ToFloat64(promStellarTxmReachedMaxAttempts.WithLabelValues(testChainID))
	require.InDelta(t, float64(0), value, 0.00001)
}

func TestNoopStellarTxmMetrics(t *testing.T) {
	ctx := t.Context()
	m := NewNoopStellarTxmMetrics()

	assert.NotPanics(t, func() {
		m.IncrementLifecycleFailure(ctx, StageBroadcast)
		m.IncrementBroadcastedTxs(ctx)
		m.IncrementSuccessTxs(ctx)
		m.IncrementFinalizedTxs(ctx)
		m.ReachedMaxAttempts(ctx, true)
		m.RecordTimeUntilTxConfirmed(ctx, 1)
	})
	assert.NoError(t, m.EmitTxMessage(ctx, "hash", "from", 1, nil))
}

func TestStellarTxm_Metrics_BroadcastEmitsTxMessage(t *testing.T) {
	observer := beholdertest.NewObserver(t)
	chainID := chainsel.STELLAR_TESTNET.ChainID
	broadcastBefore := testutil.ToFloat64(promStellarTxmBroadcastedTxs.WithLabelValues(chainID))

	accountXDR := buildAccountEntryXDR(t, testAddress, 100)
	mock := &mockRPCClient{
		getLedgerEntriesResp: protocolrpc.GetLedgerEntriesResponse{
			Entries: []protocolrpc.LedgerEntryResult{{DataXDR: accountXDR}},
		},
		getLatestLedgerResp: protocolrpc.GetLatestLedgerResponse{Sequence: 1000},
		simulateResp:        protocolrpc.SimulateTransactionResponse{MinResourceFee: 10000},
		sendTransactionResp: protocolrpc.SendTransactionResponse{
			Status: stellarcore.TXStatusPending,
			Hash:   "test-hash",
		},
	}

	txm, err := New(logger.Test(t), &mockKeystore{}, Config{}, newTestGetClient(mock), chainID)
	require.NoError(t, err)

	require.NoError(t, txm.Start(t.Context()))
	defer txm.Close()

	txID, err := txm.Enqueue(t.Context(), TxRequest{
		FromAddress: testAddress,
		Operations:  []txnbuild.Operation{testInvokeNoopOp()},
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		status, err := txm.GetStatus(txID)
		return err == nil && status == commontypes.Unconfirmed
	}, 5*time.Second, 50*time.Millisecond, "tx should reach Unconfirmed after broadcast")

	broadcastAfter := testutil.ToFloat64(promStellarTxmBroadcastedTxs.WithLabelValues(chainID))
	assert.Equal(t, float64(1), broadcastAfter-broadcastBefore, "broadcast counter should increment once")

	messages := observer.Messages(t)
	require.Len(t, messages, 1, "expected one Beholder TxMessage emit on broadcast accept")

	var txMsg svrv1.TxMessage
	require.NoError(t, proto.Unmarshal(messages[0].Body, &txMsg))
	assert.Equal(t, "test-hash", txMsg.Hash)
	assert.Equal(t, testAddress, txMsg.FromAddress)
	assert.Equal(t, chainID, txMsg.ChainId)
	assert.Equal(t, strconv.FormatInt(int64(101), 10), txMsg.Nonce, "first tx uses account seq 100 + 1")
	assert.NotEmpty(t, txMsg.ToAddress, "Soroban invoke should populate contract ToAddress")
}

func TestStellarTxm_Metrics_ConfirmIncrementsSuccessAndFinalized(t *testing.T) {
	chainID := chainsel.STELLAR_TESTNET.ChainID
	successBefore := testutil.ToFloat64(promStellarTxmSuccessTxs.WithLabelValues(chainID))
	finalizedBefore := testutil.ToFloat64(promStellarTxmFinalizedTxs.WithLabelValues(chainID))

	accountXDR := buildAccountEntryXDR(t, testAddress, 100)
	mock := &mockRPCClient{
		getLedgerEntriesResp: protocolrpc.GetLedgerEntriesResponse{
			Entries: []protocolrpc.LedgerEntryResult{{DataXDR: accountXDR}},
		},
		getLatestLedgerResp: protocolrpc.GetLatestLedgerResponse{Sequence: 1000},
		simulateResp:        protocolrpc.SimulateTransactionResponse{MinResourceFee: 10000},
		sendTransactionResp: protocolrpc.SendTransactionResponse{
			Status: stellarcore.TXStatusPending,
			Hash:   "test-hash",
		},
		getTransactionResp: protocolrpc.GetTransactionResponse{
			TransactionDetails: protocolrpc.TransactionDetails{
				Status: protocolrpc.TransactionStatusSuccess,
			},
		},
	}

	cfg := Config{ConfirmPollInterval: config.MustNewDuration(100 * time.Millisecond)}
	txm, err := New(logger.Test(t), &mockKeystore{}, cfg, newTestGetClient(mock), chainID)
	require.NoError(t, err)

	require.NoError(t, txm.Start(t.Context()))
	defer txm.Close()

	txID, err := txm.Enqueue(t.Context(), TxRequest{
		FromAddress: testAddress,
		Operations:  []txnbuild.Operation{testInvokeNoopOp()},
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		status, err := txm.GetStatus(txID)
		return err == nil && status == commontypes.Finalized
	}, 5*time.Second, 50*time.Millisecond, "tx should reach Finalized after confirm loop")

	successAfter := testutil.ToFloat64(promStellarTxmSuccessTxs.WithLabelValues(chainID))
	finalizedAfter := testutil.ToFloat64(promStellarTxmFinalizedTxs.WithLabelValues(chainID))

	assert.Equal(t, float64(1), successAfter-successBefore, "success counter should increment once")
	assert.Equal(t, float64(1), finalizedAfter-finalizedBefore, "finalized counter should increment once")
}
