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
	"github.com/smartcontractkit/chainlink-ccv/protocol"
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

// TestEVMToStellarTokenTransfer validates the EVM-to-Stellar token transfer flow:
//
//  1. EVM OnRamp lock/burn
//  2. Verifiers + Indexer
//  3. Stellar Executor → Stellar OffRamp release/mint
//
// This test exercises the reverse direction from TestStellarToEVMTokenTransfer.
//
// TODO(NONEVM-3946): Implement once the Stellar executor and OffRamp
// release_or_mint_single_token are wired for inbound token transfers.
func TestEVMToStellarTokenTransfer(t *testing.T) {
	t.Skip("EVM-to-Stellar token transfer not yet supported; Stellar executor needs token release/mint wiring")

	configOutputPath := "../env/env-stellar-evm-out.toml"

	stellarChainID := chainsel.STELLAR_LOCALNET.ChainID
	stellarSelector := chainsel.STELLAR_LOCALNET.Selector

	ctx := ccv.Plog.WithContext(t.Context())
	l := zerolog.Ctx(ctx)

	env := helpers.NewE2ETestEnv(t, ctx, l, configOutputPath, stellarChainID, stellarSelector)
	stellarDetails := env.SourceChainDetails
	evmDetails := env.DestChainDetails

	stellarChain := env.Chains[stellarDetails.ChainSelector]
	require.NotNil(t, stellarChain)

	evmChain := env.Chains[evmDetails.ChainSelector]
	require.NotNil(t, evmChain)

	t.Run("evm_to_stellar_token_transfer", func(t *testing.T) {
		stellarReceiver, err := stellarChain.GetEOAReceiverAddress()
		require.NoError(t, err)

		seqNo, err := evmChain.GetExpectedNextSequenceNumber(ctx, stellarDetails.ChainSelector)
		require.NoError(t, err)

		_ = seqNo
		_ = stellarReceiver

		// TODO(NONEVM-3946): Populate EVM token amount and send via EVM Router,
		// then wait for Stellar OffRamp execution event.
		t.Log("EVM-to-Stellar token transfer placeholder")
	})
}
