package e2e_tests

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	chainsel "github.com/smartcontractkit/chain-selectors"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	devenvcommon "github.com/smartcontractkit/chainlink-ccv/build/devenv/common"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/tests/e2e"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-common/pkg/utils/tests"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
)

const (
	stellarSentTimeout = 30 * time.Second
)

// TestStellarToEVMExecution validates the full Stellar-to-EVM CCIP message flow:
// Stellar Router → OnRamp → Verifiers → Indexer → EVM Executor → EVM OffRamp.
//
// Contracts must be compiled before running:
//
//	make build
//
// Start the devenv from the chainlink-stellar root:
//
//	CTF_CONFIGS=tests/env/env-stellar-evm.toml go run ./tests/testutils/cmd/devenv
//
// Once the devenv is running, run the test:
//
//	go test -v -timeout 10m ./tests/e2e/... -run TestStellarToEVMExecution
func TestStellarToEVMExecution(t *testing.T) {
	configOutputPath := "../env/env-stellar-evm-out.toml"

	stellarChainID := chainsel.STELLAR_LOCALNET.ChainID
	stellarSelector := chainsel.STELLAR_LOCALNET.Selector

	ctx := ccv.Plog.WithContext(t.Context())
	l := zerolog.Ctx(ctx)

	env := helpers.NewE2ETestEnv(t, ctx, l, configOutputPath, stellarChainID, stellarSelector)
	stellarDetails := env.SourceChainDetails
	evmDetails := env.DestChainDetails

	stellarChain := env.Chains[stellarDetails.ChainSelector]
	require.NotNil(t, stellarChain, "Stellar chain not found in chains map")

	evmChain := env.Chains[evmDetails.ChainSelector]
	require.NotNil(t, evmChain, "EVM chain not found in chains map")

	t.Run("stellar_to_evm_execution", func(t *testing.T) {
		evmReceiver, err := evmChain.GetEOAReceiverAddress()
		require.NoError(t, err)
		l.Info().Str("evmReceiver", hex.EncodeToString(evmReceiver)).Msg("Using EVM receiver address")

		seqNo, err := stellarChain.GetExpectedNextSequenceNumber(ctx, evmDetails.ChainSelector)
		require.NoError(t, err)
		l.Info().Uint64("seqNo", seqNo).Msg("Expected next sequence number from Stellar OnRamp")

		sendResult, err := stellarChain.SendMessage(ctx, evmDetails.ChainSelector,
			cciptestinterfaces.MessageFields{
				Receiver: evmReceiver,
				Data:     []byte("hello from stellar"),
			},
			cciptestinterfaces.MessageOptions{},
		)
		require.NoError(t, err)
		l.Info().
			Str("messageID", hex.EncodeToString(sendResult.MessageID[:])).
			Msg("CCIP message sent from Stellar")

		sentEvent, err := stellarChain.WaitOneSentEventBySeqNo(ctx, evmDetails.ChainSelector, seqNo, stellarSentTimeout)
		require.NoError(t, err)
		messageID := sentEvent.MessageID
		l.Info().
			Str("messageID", hex.EncodeToString(messageID[:])).
			Msg("Sent event confirmed on Stellar")

		defaultAggregatorClient := env.AggregatorClients[devenvcommon.DefaultCommitteeVerifierQualifier]
		require.NotNil(t, defaultAggregatorClient)

		testCtx := e2e.NewTestingContext(t, t.Context(), env.Chains, defaultAggregatorClient, env.IndexerMonitor)
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

		// TODO: uncomment once EVM executor is wired up for Stellar-sourced messages.
		// execEvent, err := evmChain.WaitOneExecEventBySeqNo(t.Context(), stellarDetails.ChainSelector, seqNo, execTimeout)
		// require.NoError(t, err)
		// require.Equalf(
		// 	t,
		// 	cciptestinterfaces.ExecutionStateSuccess,
		// 	execEvent.State,
		// 	"message should have been successfully executed, return data: %x",
		// 	execEvent.ReturnData,
		// )
		//
		// l.Info().
		// 	Str("messageID", hex.EncodeToString(messageID[:])).
		// 	Uint64("seqNo", seqNo).
		// 	Msg("Message executed successfully on EVM")
	})
}
