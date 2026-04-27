package e2e_tests

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stretchr/testify/require"

	chainsel "github.com/smartcontractkit/chain-selectors"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	devenvcommon "github.com/smartcontractkit/chainlink-ccv/build/devenv/common"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/tests/e2e"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-common/pkg/utils/tests"
	ccvchain "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
)

const (
	tokenTransferSentTimeout = 30 * time.Second
	tokenTransferAmount      = int64(1_000_000) // 0.1 token in SAC base units (7 decimals)
)

// TestStellarToEVMTokenTransfer validates the Stellar-to-EVM token transfer flow:
//
//  1. Fund sender with test SAC tokens
//  2. Send ccip_send via Router with TokenAmounts populated
//  3. OnRamp calls lock_or_burn on the lock-release pool
//  4. Verifiers + Indexer process the message
//  5. EVM Executor triggers OffRamp release/mint on the EVM side
//
// Prerequisites:
//
//	make build
//	CTF_CONFIGS=tests/env/env-stellar-evm.toml go run ./tests/testutils/cmd/devenv
//
// Run:
//
//	go test -v -timeout 10m ./tests/e2e/... -run TestStellarToEVMTokenTransfer
func TestStellarToEVMTokenTransfer(t *testing.T) {
	configOutputPath := "../env/env-stellar-evm-out.toml"

	stellarChainID := chainsel.STELLAR_LOCALNET.ChainID
	stellarSelector := chainsel.STELLAR_LOCALNET.Selector

	ctx := ccv.Plog.WithContext(t.Context())
	l := zerolog.Ctx(ctx)

	env := helpers.NewE2ETestEnv(t, ctx, l, configOutputPath, stellarChainID, stellarSelector)
	stellarDetails := env.SourceChainDetails
	evmDetails := env.DestChainDetails

	stellarChain := env.Chains[stellarDetails.ChainSelector]
	require.NotNil(t, stellarChain, "Stellar chain not found")

	evmChain := env.Chains[evmDetails.ChainSelector]
	require.NotNil(t, evmChain, "EVM chain not found")

	stellarCcvChain, ok := stellarChain.(*ccvchain.Chain)
	require.True(t, ok, "Stellar chain must be *ccvchain.Chain")

	tokenAddr, err := stellarCcvChain.GetTokenAddress()
	require.NoError(t, err, "test token must be deployed")
	l.Info().Str("tokenAddress", tokenAddr).Msg("Using test SAC token")

	tokenRaw, err := strkey.Decode(strkey.VersionByteContract, tokenAddr)
	require.NoError(t, err)

	senderAddr, err := stellarChain.GetSenderAddress()
	require.NoError(t, err)

	t.Run("stellar_to_evm_token_transfer", func(t *testing.T) {
		evmReceiver, err := evmChain.GetEOAReceiverAddress()
		require.NoError(t, err)

		balBefore, err := stellarChain.GetTokenBalance(ctx, senderAddr, protocol.UnknownAddress(tokenRaw))
		require.NoError(t, err)
		l.Info().Str("balance", balBefore.String()).Msg("Sender token balance before transfer")
		require.True(t, balBefore.Int64() >= tokenTransferAmount,
			"sender must have enough tokens; balance=%s, need=%d", balBefore, tokenTransferAmount)

		seqNo, err := stellarChain.GetExpectedNextSequenceNumber(ctx, evmDetails.ChainSelector)
		require.NoError(t, err)

		sendResult, err := stellarChain.SendMessage(ctx, evmDetails.ChainSelector,
			cciptestinterfaces.MessageFields{
				Receiver: evmReceiver,
				Data:     []byte("token-transfer-test"),
				TokenAmount: cciptestinterfaces.TokenAmount{
					Amount:       big.NewInt(tokenTransferAmount),
					TokenAddress: protocol.UnknownAddress(tokenRaw),
				},
			},
			cciptestinterfaces.MessageOptions{},
		)
		require.NoError(t, err)
		l.Info().
			Str("messageID", hex.EncodeToString(sendResult.MessageID[:])).
			Msg("Token transfer message sent from Stellar")

		sentEvent, err := stellarChain.WaitOneSentEventBySeqNo(ctx, evmDetails.ChainSelector, seqNo, tokenTransferSentTimeout)
		require.NoError(t, err)
		l.Info().
			Str("messageID", hex.EncodeToString(sentEvent.MessageID[:])).
			Msg("Sent event confirmed")

		balAfter, err := stellarChain.GetTokenBalance(ctx, senderAddr, protocol.UnknownAddress(tokenRaw))
		require.NoError(t, err)
		l.Info().Str("balance", balAfter.String()).Msg("Sender token balance after transfer")

		locked := new(big.Int).Sub(balBefore, balAfter)
		require.Equal(t, tokenTransferAmount, locked.Int64(),
			"sender balance should decrease by exactly the transfer amount")

		l.Info().
			Str("messageID", hex.EncodeToString(sentEvent.MessageID[:])).
			Int64("lockedAmount", locked.Int64()).
			Msg("Token transfer source-side flow validated: tokens locked, CCIPMessageSent emitted")

		// TODO(NONEVM-3946): The aggregator currently rejects CCV data for messages
		// that contain token transfers (InvalidArgument: validation failed). Once the
		// aggregator supports the token-transfer message format, uncomment to verify
		// the full pipeline:
		//
		// defaultAggregatorClient := env.AggregatorClients[devenvcommon.DefaultCommitteeVerifierQualifier]
		// require.NotNil(t, defaultAggregatorClient)
		// testCtx := e2e.NewTestingContext(t, t.Context(), env.Chains, defaultAggregatorClient, env.IndexerMonitor)
		// result, err := testCtx.AssertMessage(protocol.Bytes32(sentEvent.MessageID), e2e.AssertMessageOptions{
		// 	TickInterval:            1 * time.Second,
		// 	ExpectedVerifierResults: 1,
		// 	Timeout:                 tests.WaitTimeout(t),
		// 	AssertVerifierLogs:      false,
		// 	AssertExecutorLogs:      false,
		// })
		// require.NoError(t, err)
		// require.NotNil(t, result.AggregatedResult)
		//
		// execEvent, err := evmChain.WaitOneExecEventBySeqNo(ctx, stellarDetails.ChainSelector, seqNo, 5*time.Minute)
		// require.NoError(t, err)
		// require.Equal(t, cciptestinterfaces.ExecutionStateSuccess, execEvent.State)
	})
}

// TestStellarToEVMTokenTransferFees validates fee collection during a
// Stellar-to-EVM token transfer. Fees are paid in the fee token (a separate
// SAC from the transferred token), so this test tracks sender balances for both:
//
//  1. Transferred token: sender loses exactly tokenTransferAmount
//  2. Fee token: sender's fee-token balance decreases (fee charged by Router/OnRamp)
//
// Note: pool and OnRamp contract balances are not checked because
// GetTokenBalance encodes all addresses as classic accounts (G-prefix),
// which fails for contract addresses that hold SAC balances via contract
// storage rather than trustlines.
func TestStellarToEVMTokenTransferFees(t *testing.T) {
	configOutputPath := "../env/env-stellar-evm-out.toml"

	stellarChainID := chainsel.STELLAR_LOCALNET.ChainID
	stellarSelector := chainsel.STELLAR_LOCALNET.Selector

	ctx := ccv.Plog.WithContext(t.Context())
	l := zerolog.Ctx(ctx)

	env := helpers.NewE2ETestEnv(t, ctx, l, configOutputPath, stellarChainID, stellarSelector)
	stellarDetails := env.SourceChainDetails
	evmDetails := env.DestChainDetails

	stellarChain := env.Chains[stellarDetails.ChainSelector]
	require.NotNil(t, stellarChain, "Stellar chain not found")

	evmChain := env.Chains[evmDetails.ChainSelector]
	require.NotNil(t, evmChain, "EVM chain not found")

	stellarCcvChain, ok := stellarChain.(*ccvchain.Chain)
	require.True(t, ok, "Stellar chain must be *ccvchain.Chain")

	tokenAddr, err := stellarCcvChain.GetTokenAddress()
	require.NoError(t, err)
	tokenRaw, err := strkey.Decode(strkey.VersionByteContract, tokenAddr)
	require.NoError(t, err)

	feeTokenAddr, err := stellarCcvChain.GetFeeTokenAddress()
	require.NoError(t, err)
	feeTokenRaw, err := strkey.Decode(strkey.VersionByteContract, feeTokenAddr)
	require.NoError(t, err)

	senderAddr, err := stellarChain.GetSenderAddress()
	require.NoError(t, err)

	t.Run("fee_collection_on_token_transfer", func(t *testing.T) {
		evmReceiver, err := evmChain.GetEOAReceiverAddress()
		require.NoError(t, err)

		senderTokenBefore, err := stellarChain.GetTokenBalance(ctx, senderAddr, protocol.UnknownAddress(tokenRaw))
		require.NoError(t, err)

		senderFeeBefore, err := stellarChain.GetTokenBalance(ctx, senderAddr, protocol.UnknownAddress(feeTokenRaw))
		require.NoError(t, err)

		l.Info().
			Str("sender_token", senderTokenBefore.String()).
			Str("sender_fee_token", senderFeeBefore.String()).
			Msg("Sender balances before transfer")

		seqNo, err := stellarChain.GetExpectedNextSequenceNumber(ctx, evmDetails.ChainSelector)
		require.NoError(t, err)

		_, err = stellarChain.SendMessage(ctx, evmDetails.ChainSelector,
			cciptestinterfaces.MessageFields{
				Receiver: evmReceiver,
				Data:     []byte("fee-test"),
				TokenAmount: cciptestinterfaces.TokenAmount{
					Amount:       big.NewInt(tokenTransferAmount),
					TokenAddress: protocol.UnknownAddress(tokenRaw),
				},
			},
			cciptestinterfaces.MessageOptions{},
		)
		require.NoError(t, err)

		_, err = stellarChain.WaitOneSentEventBySeqNo(ctx, evmDetails.ChainSelector, seqNo, tokenTransferSentTimeout)
		require.NoError(t, err)

		senderTokenAfter, err := stellarChain.GetTokenBalance(ctx, senderAddr, protocol.UnknownAddress(tokenRaw))
		require.NoError(t, err)

		senderFeeAfter, err := stellarChain.GetTokenBalance(ctx, senderAddr, protocol.UnknownAddress(feeTokenRaw))
		require.NoError(t, err)

		tokenDelta := new(big.Int).Sub(senderTokenBefore, senderTokenAfter)
		feeDelta := new(big.Int).Sub(senderFeeBefore, senderFeeAfter)

		l.Info().
			Str("sender_token_delta", tokenDelta.String()).
			Str("sender_fee_delta", feeDelta.String()).
			Msg("Sender balance deltas after token transfer")

		require.Equal(t, tokenTransferAmount, tokenDelta.Int64(),
			"sender transferred-token balance should decrease by exactly the transfer amount")

		require.True(t, feeDelta.Sign() > 0,
			"sender fee-token balance should decrease (fee charged); got delta=%s", feeDelta)

		l.Info().Str("fee_paid", feeDelta.String()).Msg("Fee collection validated")
	})
}

// TestEVMToStellarTokenTransfer validates the full EVM-to-Stellar token transfer flow:
//
//  1. EVM Router ccip_send with TokenAmount → OnRamp lock/burn on BurnMintTokenPool
//  2. Verifiers + Indexer process the message
//  3. Stellar Executor → OffRamp release_or_mint_single_token on LockReleasePool
//
// The EVM test token uses 18 decimals; the Stellar SAC uses 7 decimals.
// The pool's calculate_local_amount scales the amount down by 10^(18-7) = 10^11.
// We send evmTokenTransferAmount (= tokenTransferAmount * 10^11) on EVM so that
// the Stellar receiver gets exactly tokenTransferAmount base units.
//
// Prerequisites:
//
//	make build
//	CTF_CONFIGS=tests/env/env-stellar-evm.toml go run ./tests/testutils/cmd/devenv
//
// Run:
//
//	go test -v -timeout 15m ./tests/e2e/... -run TestEVMToStellarTokenTransfer
func TestEVMToStellarTokenTransfer(t *testing.T) {
	configOutputPath := "../env/env-stellar-evm-out.toml"

	stellarChainID := chainsel.STELLAR_LOCALNET.ChainID
	stellarSelector := chainsel.STELLAR_LOCALNET.Selector

	ctx := ccv.Plog.WithContext(t.Context())
	l := zerolog.Ctx(ctx)

	env := helpers.NewE2ETestEnv(t, ctx, l, configOutputPath, stellarChainID, stellarSelector)
	stellarDetails := env.SourceChainDetails
	evmDetails := env.DestChainDetails

	stellarChain := env.Chains[stellarDetails.ChainSelector]
	require.NotNil(t, stellarChain, "Stellar chain not found")

	evmChain := env.Chains[evmDetails.ChainSelector]
	require.NotNil(t, evmChain, "EVM chain not found")

	stellarCcvChain, ok := stellarChain.(*ccvchain.Chain)
	require.True(t, ok, "Stellar chain must be *ccvchain.Chain")

	evmToken, err := helpers.ResolveEVMTestToken(env.DataStore, evmDetails.ChainSelector)
	require.NoError(t, err, "EVM test token must be in datastore")
	l.Info().Str("evmToken", hex.EncodeToString(evmToken)).Msg("Using EVM test token")

	stellarTokenAddr, err := stellarCcvChain.GetTokenAddress()
	require.NoError(t, err, "Stellar test token must be deployed")
	stellarTokenRaw, err := strkey.Decode(strkey.VersionByteContract, stellarTokenAddr)
	require.NoError(t, err)

	// Scale tokenTransferAmount from 7-decimal Stellar units to 18-decimal EVM units.
	decimalScale := new(big.Int).Exp(big.NewInt(10), big.NewInt(11), nil) // 10^(18-7)
	evmAmount := new(big.Int).Mul(big.NewInt(tokenTransferAmount), decimalScale)

	t.Run("evm_to_stellar_token_transfer", func(t *testing.T) {
		stellarReceiver, err := stellarChain.GetEOAReceiverAddress()
		require.NoError(t, err)
		l.Info().Str("stellarReceiver", hex.EncodeToString(stellarReceiver)).Msg("Using Stellar receiver")

		evmSender, err := evmChain.GetSenderAddress()
		require.NoError(t, err)

		evmBalBefore, err := evmChain.GetTokenBalance(ctx, evmSender, evmToken)
		require.NoError(t, err)
		l.Info().Str("evmBalance", evmBalBefore.String()).Msg("EVM sender token balance before transfer")
		require.True(t, evmBalBefore.Cmp(evmAmount) >= 0,
			"EVM sender must have enough tokens; balance=%s, need=%s", evmBalBefore, evmAmount)

		stellarReceiverBalBefore, err := stellarChain.GetTokenBalance(ctx, stellarReceiver, protocol.UnknownAddress(stellarTokenRaw))
		require.NoError(t, err)
		l.Info().Str("stellarReceiverBalance", stellarReceiverBalBefore.String()).Msg("Stellar receiver balance before transfer")

		seqNo, err := evmChain.GetExpectedNextSequenceNumber(ctx, stellarDetails.ChainSelector)
		require.NoError(t, err)
		l.Info().Uint64("seqNo", seqNo).Msg("Expected next sequence number from EVM OnRamp")

		sendResult, err := evmChain.SendMessage(ctx, stellarDetails.ChainSelector,
			cciptestinterfaces.MessageFields{
				Receiver: stellarReceiver,
				Data:     []byte("evm-to-stellar-token-transfer"),
				TokenAmount: cciptestinterfaces.TokenAmount{
					Amount:       evmAmount,
					TokenAddress: evmToken,
				},
			},
			cciptestinterfaces.MessageOptions{
				Version:           3,
				ExecutionGasLimit: 200_000,
			},
		)
		require.NoError(t, err)
		l.Info().
			Str("messageID", hex.EncodeToString(sendResult.MessageID[:])).
			Msg("Token transfer message sent from EVM")

		sentEvent, err := evmChain.WaitOneSentEventBySeqNo(ctx, stellarDetails.ChainSelector, seqNo, tokenTransferSentTimeout)
		require.NoError(t, err)
		messageID := sentEvent.MessageID
		l.Info().
			Str("messageID", hex.EncodeToString(messageID[:])).
			Msg("Sent event confirmed on EVM")

		evmBalAfter, err := evmChain.GetTokenBalance(ctx, evmSender, evmToken)
		require.NoError(t, err)
		evmDelta := new(big.Int).Sub(evmBalBefore, evmBalAfter)
		l.Info().
			Str("evmBalanceDelta", evmDelta.String()).
			Msg("EVM sender token balance delta")
		require.True(t, evmDelta.Cmp(evmAmount) == 0,
			"EVM sender token balance should decrease by exactly the transfer amount; delta=%s, expected=%s", evmDelta, evmAmount)

		defaultAggregatorClient := env.AggregatorClients[devenvcommon.DefaultCommitteeVerifierQualifier]
		require.NotNil(t, defaultAggregatorClient)

		testCtx := e2e.NewTestingContext(t, ctx, env.Chains, defaultAggregatorClient, env.IndexerMonitor)
		result, err := testCtx.AssertMessage(protocol.Bytes32(messageID), e2e.AssertMessageOptions{
			TickInterval:            1 * time.Second,
			ExpectedVerifierResults: 1,
			Timeout:                 tests.WaitTimeout(t),
			AssertVerifierLogs:      false,
			AssertExecutorLogs:      false,
		})
		require.NoError(t, err)
		require.NotNil(t, result.AggregatedResult)
		require.Len(t, result.IndexedVerifications.Results, 1)
		l.Info().
			Str("messageID", hex.EncodeToString(messageID[:])).
			Msg("Message verified and aggregated successfully")

		execEvent, err := stellarChain.WaitOneExecEventBySeqNo(ctx, evmDetails.ChainSelector, seqNo, execTimeout)
		require.NoError(t, err)
		require.Equalf(t, cciptestinterfaces.ExecutionStateSuccess, execEvent.State,
			"message should have been successfully executed, return data: %x", execEvent.ReturnData)

		l.Info().
			Str("messageID", hex.EncodeToString(messageID[:])).
			Uint64("seqNo", seqNo).
			Msg("Token transfer executed successfully on Stellar OffRamp")

		stellarReceiverBalAfter, err := stellarChain.GetTokenBalance(ctx, stellarReceiver, protocol.UnknownAddress(stellarTokenRaw))
		require.NoError(t, err)
		stellarDelta := new(big.Int).Sub(stellarReceiverBalAfter, stellarReceiverBalBefore)
		l.Info().
			Str("stellarReceiverDelta", stellarDelta.String()).
			Int64("expectedStellarAmount", tokenTransferAmount).
			Msg("Stellar receiver balance delta")
		require.Equal(t, tokenTransferAmount, stellarDelta.Int64(),
			"Stellar receiver should gain exactly the scaled transfer amount")
	})
}

// TestEVMToStellarTokenTransferFees validates the EVM sender's token balance
// delta during an EVM-to-Stellar token transfer. The test-token (BurnMintERC20)
// should decrease by exactly the transfer amount; EVM CCIP fees are paid
// separately in native ETH and are not asserted here because the test interface
// does not expose a GetNativeBalance API.
//
// Prerequisites:
//
//	make build
//	CTF_CONFIGS=tests/env/env-stellar-evm.toml go run ./tests/testutils/cmd/devenv
//
// Run:
//
//	go test -v -timeout 15m ./tests/e2e/... -run TestEVMToStellarTokenTransferFees
func TestEVMToStellarTokenTransferFees(t *testing.T) {
	configOutputPath := "../env/env-stellar-evm-out.toml"

	stellarChainID := chainsel.STELLAR_LOCALNET.ChainID
	stellarSelector := chainsel.STELLAR_LOCALNET.Selector

	ctx := ccv.Plog.WithContext(t.Context())
	l := zerolog.Ctx(ctx)

	env := helpers.NewE2ETestEnv(t, ctx, l, configOutputPath, stellarChainID, stellarSelector)
	stellarDetails := env.SourceChainDetails
	evmDetails := env.DestChainDetails

	stellarChain := env.Chains[stellarDetails.ChainSelector]
	require.NotNil(t, stellarChain, "Stellar chain not found")

	evmChain := env.Chains[evmDetails.ChainSelector]
	require.NotNil(t, evmChain, "EVM chain not found")

	evmToken, err := helpers.ResolveEVMTestToken(env.DataStore, evmDetails.ChainSelector)
	require.NoError(t, err, "EVM test token must be in datastore")

	decimalScale := new(big.Int).Exp(big.NewInt(10), big.NewInt(11), nil)
	evmAmount := new(big.Int).Mul(big.NewInt(tokenTransferAmount), decimalScale)

	t.Run("evm_to_stellar_fee_collection", func(t *testing.T) {
		stellarReceiver, err := stellarChain.GetEOAReceiverAddress()
		require.NoError(t, err)

		evmSender, err := evmChain.GetSenderAddress()
		require.NoError(t, err)

		senderTokenBefore, err := evmChain.GetTokenBalance(ctx, evmSender, evmToken)
		require.NoError(t, err)

		l.Info().
			Str("sender_token", senderTokenBefore.String()).
			Msg("EVM sender balance before transfer")

		seqNo, err := evmChain.GetExpectedNextSequenceNumber(ctx, stellarDetails.ChainSelector)
		require.NoError(t, err)

		_, err = evmChain.SendMessage(ctx, stellarDetails.ChainSelector,
			cciptestinterfaces.MessageFields{
				Receiver: stellarReceiver,
				Data:     []byte("fee-test-evm-to-stellar"),
				TokenAmount: cciptestinterfaces.TokenAmount{
					Amount:       evmAmount,
					TokenAddress: evmToken,
				},
			},
			cciptestinterfaces.MessageOptions{
				Version:           3,
				ExecutionGasLimit: 200_000,
			},
		)
		require.NoError(t, err)

		_, err = evmChain.WaitOneSentEventBySeqNo(ctx, stellarDetails.ChainSelector, seqNo, tokenTransferSentTimeout)
		require.NoError(t, err)

		senderTokenAfter, err := evmChain.GetTokenBalance(ctx, evmSender, evmToken)
		require.NoError(t, err)

		tokenDelta := new(big.Int).Sub(senderTokenBefore, senderTokenAfter)

		l.Info().
			Str("sender_token_delta", tokenDelta.String()).
			Str("expected_transfer", evmAmount.String()).
			Msg("EVM sender balance delta after token transfer")

		require.True(t, tokenDelta.Cmp(evmAmount) == 0,
			"sender token balance should decrease by exactly the transfer amount; delta=%s, expected=%s",
			tokenDelta, evmAmount)

		l.Info().
			Str("token_spent", tokenDelta.String()).
			Msg("EVM sender token delta validated (fees paid in native ETH, not tracked here)")
	})
}
