package e2e_tests

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	chainsel "github.com/smartcontractkit/chain-selectors"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	devenvcommon "github.com/smartcontractkit/chainlink-ccv/build/devenv/common"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/tests/e2e"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-common/pkg/utils/tests"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	fqbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/fee_quoter"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
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

		sendResult, seqNo, err := sendAndVerifyMessage(t, ctx, l, stellarChain, env, evmDetails, evmReceiver,
			"hello from stellar", "Message verified and aggregated successfully")

		require.NoError(t, err)

		// TODO: uncomment once EVM executor is wired up for Stellar-sourced messages.
		execEvent, err := evmChain.ConfirmExecOnDest(t.Context(), evmDetails.ChainSelector, cciptestinterfaces.MessageEventKey{
			SeqNum:    seqNo,
			MessageID: sendResult.MessageID,
		}, execTimeout)
		require.NoError(t, err)
		require.Equalf(
			t,
			cciptestinterfaces.ExecutionStateSuccess,
			execEvent.State,
			"message should have been successfully executed, return data: %x",
			execEvent.ReturnData,
		)
	})

	t.Run("stellar_to_evm_execution_cursed_destination", func(t *testing.T) {
		evmReceiver, err := evmChain.GetEOAReceiverAddress()
		require.NoError(t, err)
		l.Info().Str("evmReceiver", hex.EncodeToString(evmReceiver)).Msg("Using EVM receiver address")

		// Curse the EVM destination chain and ensure it gets uncursed even if test fails
		l.Info().Uint64("chainSelector", evmDetails.ChainSelector).Msg("Cursing EVM destination chain")
		err = stellarChain.Curse(ctx, [][16]byte{chainSelectorToSubject(evmDetails.ChainSelector)})
		require.NoError(t, err)
		l.Info().Msg("✅ EVM destination chain cursed successfully")

		t.Cleanup(func() {
			l.Info().Msg("🔓 Cleaning up: uncursing EVM destination chain")
			_ = stellarChain.Uncurse(ctx, [][16]byte{chainSelectorToSubject(evmDetails.ChainSelector)})
		})

		// Try to send a message from Stellar to the cursed EVM chain
		// This should fail with an error because the Router will reject it due to RMN curse
		l.Info().Msg("📨 Attempting to send message to cursed EVM destination")
		_, sendErr := stellarChain.SendMessage(ctx, evmDetails.ChainSelector,
			cciptestinterfaces.MessageFields{
				Receiver: evmReceiver,
				Data:     []byte("should fail - chain is cursed"),
			},
			cciptestinterfaces.MessageOptions{},
		)
		require.Error(t, sendErr, "sending message to cursed chain should fail")
		l.Info().Err(sendErr).Msg("✅ Message send failed as expected due to curse on destination chain")

		// Uncurse the EVM destination chain
		l.Info().Msg("🔓 Uncursing EVM destination chain")
		err = stellarChain.Uncurse(ctx, [][16]byte{chainSelectorToSubject(evmDetails.ChainSelector)})
		require.NoError(t, err)
		l.Info().Msg("✅ EVM destination chain uncursed successfully")

		// Now sending a message should work
		sendAndVerifyMessage(t, ctx, l, stellarChain, env, evmDetails, evmReceiver,
			"hello from stellar after uncurse", "Message verified and aggregated successfully after uncurse")
	})
}

// chainSelectorToSubject converts a chain selector to a bytes16 curse subject.
func chainSelectorToSubject(chainSel uint64) [16]byte {
	var result [16]byte
	// Convert the uint64 to bytes and place it in the last 8 bytes of the array
	binary.BigEndian.PutUint64(result[8:], chainSel)
	return result
}

// TestStellarToEVMFeeQuoterDestChainDisabled validates that disabling a
// destination chain on the Stellar FeeQuoter prevents message sending, and
// that re-enabling it restores normal operation.
//
// The Stellar send path is: Router.ccip_send → OnRamp.get_fee →
// FeeQuoter.get_message_fee, which returns DestinationChainNotEnabled when
// the dest chain config has IsEnabled = false.
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
//	go test -v -timeout 10m ./tests/e2e/... -run TestStellarToEVMFeeQuoterDestChainDisabled
func TestStellarToEVMFeeQuoterDestChainDisabled(t *testing.T) {
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

	// Resolve the Stellar FeeQuoter contract from the datastore.
	fqKey := datastore.NewAddressRefKey(
		stellarDetails.ChainSelector,
		"FeeQuoter",
		semver.MustParse("2.0.0"),
		"",
	)
	fqRef, err := env.DataStore.Addresses().Get(fqKey)
	require.NoError(t, err)
	require.NotEmpty(t, fqRef.Address)
	fqContractID, err := scval.HexToContractStrkey(fqRef.Address)
	require.NoError(t, err)
	l.Info().Str("feeQuoterContractID", fqContractID).Msg("Found FeeQuoter in datastore")

	fqClient := fqbindings.NewFeeQuoterClient(env.Deployer, fqContractID)

	t.Run("stellar_to_evm_fee_quoter_dest_disabled", func(t *testing.T) {
		evmReceiver, err := evmChain.GetEOAReceiverAddress()
		require.NoError(t, err)
		l.Info().Str("evmReceiver", hex.EncodeToString(evmReceiver)).Msg("Using EVM receiver address")

		// Read current config, then disable the EVM destination chain.
		cfg, err := fqClient.GetDestChainConfig(ctx, evmDetails.ChainSelector)
		require.NoError(t, err)
		require.True(t, cfg.IsEnabled, "dest chain should start enabled")

		cfg.IsEnabled = false
		l.Info().Uint64("destChainSelector", evmDetails.ChainSelector).Msg("Disabling EVM dest chain on FeeQuoter")
		err = fqClient.ApplyDestChainConfigs(ctx, []fqbindings.DestChainConfigArgs{
			{DestChainSelector: evmDetails.ChainSelector, Config: *cfg},
		})
		require.NoError(t, err)
		l.Info().Msg("EVM dest chain disabled on FeeQuoter")

		t.Cleanup(func() {
			l.Info().Msg("Cleaning up: re-enabling EVM dest chain on FeeQuoter")
			cfg.IsEnabled = true
			_ = fqClient.ApplyDestChainConfigs(ctx, []fqbindings.DestChainConfigArgs{
				{DestChainSelector: evmDetails.ChainSelector, Config: *cfg},
			})
		})

		// Attempt to send — should fail because FeeQuoter rejects disabled dest chains.
		l.Info().Msg("Attempting to send message to disabled EVM destination")
		_, sendErr := stellarChain.SendMessage(ctx, evmDetails.ChainSelector,
			cciptestinterfaces.MessageFields{
				Receiver: evmReceiver,
				Data:     []byte("should fail - dest chain disabled"),
			},
			cciptestinterfaces.MessageOptions{},
		)
		require.Error(t, sendErr, "sending message to disabled dest chain should fail")
		l.Info().Err(sendErr).Msg("Message send failed as expected (dest chain disabled on FeeQuoter)")

		// Re-enable the EVM destination chain.
		l.Info().Msg("Re-enabling EVM dest chain on FeeQuoter")
		cfg.IsEnabled = true
		err = fqClient.ApplyDestChainConfigs(ctx, []fqbindings.DestChainConfigArgs{
			{DestChainSelector: evmDetails.ChainSelector, Config: *cfg},
		})
		require.NoError(t, err)
		l.Info().Msg("EVM dest chain re-enabled on FeeQuoter")

		// Now sending a message should work.
		sendAndVerifyMessage(t, ctx, l, stellarChain, env, evmDetails, evmReceiver,
			"hello from stellar after re-enable", "Message verified and aggregated successfully after re-enable")
	})
}

// ---------------------------
// Helper functions
// ---------------------------

// sendAndVerifyMessage sends a CCIP message from Stellar to EVM and verifies it was processed.
// It handles the complete flow: get sequence number, send message, wait for sent event,
// and verify the message was verified and aggregated.
func sendAndVerifyMessage(
	t *testing.T,
	ctx context.Context,
	l *zerolog.Logger,
	stellarChain cciptestinterfaces.CCIP17,
	env *helpers.E2ETestEnv,
	evmDetails *chainsel.ChainDetails,
	receiver []byte,
	messageData string,
	successMsg string,
) (cciptestinterfaces.MessageSentEvent, uint64, error) {
	seqNo, err := stellarChain.GetExpectedNextSequenceNumber(ctx, evmDetails.ChainSelector)
	require.NoError(t, err)
	l.Info().Uint64("seqNo", seqNo).Msg("Expected next sequence number from Stellar OnRamp")

	sendResult, err := stellarChain.SendMessage(ctx, evmDetails.ChainSelector,
		cciptestinterfaces.MessageFields{
			Receiver: receiver,
			Data:     []byte(messageData),
		},
		cciptestinterfaces.MessageOptions{},
	)
	require.NoError(t, err)
	l.Info().
		Str("messageID", hex.EncodeToString(sendResult.MessageID[:])).
		Msg("CCIP message sent from Stellar")

	sentEvent, err := stellarChain.ConfirmSendOnSource(ctx, evmDetails.ChainSelector, cciptestinterfaces.MessageEventKey{SeqNum: seqNo}, stellarSentTimeout)
	require.NoError(t, err)
	messageID := sentEvent.MessageID
	l.Info().
		Str("messageID", hex.EncodeToString(messageID[:])).
		Msg("Sent event confirmed on Stellar")

	// Verify the message was processed
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
		Msg(successMsg)

	return sendResult, seqNo, nil
}
