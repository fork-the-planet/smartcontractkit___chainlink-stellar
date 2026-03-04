package e2e_tests

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stretchr/testify/require"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccip/ccv/chains/evm/deployment/v1_7_0/operations/fee_quoter"
	onrampoperations "github.com/smartcontractkit/chainlink-ccip/ccv/chains/evm/deployment/v1_7_0/operations/onramp"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v1_6_0/operations/rmn_remote"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	devenvcommon "github.com/smartcontractkit/chainlink-ccv/build/devenv/common"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/tests/e2e"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-common/pkg/utils/tests"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	onrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/onramp"
	ccvsourcereader "github.com/smartcontractkit/chainlink-stellar/ccv/source_reader"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
)

const (
	// Test timeouts for Stellar to EVM flow
	stellarSentTimeout = 30 * time.Second
)

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
//	go test -v -timeout 10m ./tests/e2e/...
func TestStellarToEVMSourceReader(t *testing.T) {
	configOutputPath := "../env/env-stellar-evm-out.toml"

	stellarChainID := chainsel.STELLAR_LOCALNET.ChainID
	stellarSelector := chainsel.STELLAR_LOCALNET.Selector

	ctx := ccv.Plog.WithContext(t.Context())
	l := zerolog.Ctx(ctx)

	env := helpers.NewE2ETestEnv(t, ctx, l, configOutputPath, stellarChainID, stellarSelector)
	deployer := env.Deployer
	deployerKP := env.DeployerKP
	rpc := env.RPCClient
	stellarDetails := env.SourceChainDetails
	evmDetails := env.DestChainDetails
	destChain := env.DestChain

	// Look up the OnRamp contract address from the CCV datastore.
	// It was deployed and configured during NewE2ETestEnv → ccv.NewEnvironment()
	// via chain.go DeployContractsForSelector.
	onrampKey := datastore.NewAddressRefKey(
		stellarDetails.ChainSelector,
		datastore.ContractType(onrampoperations.ContractType),
		onrampoperations.Version,
		"",
	)
	onrampRef, err := env.DataStore.Addresses().Get(onrampKey)
	require.NoError(t, err)
	require.NotEmpty(t, onrampRef.Address)

	onrampContractID, err := hexToContractStrkey(onrampRef.Address)
	require.NoError(t, err)
	l.Info().Str("onrampContractID", onrampContractID).Msg("Found OnRamp in CCV datastore")

	rmnRemoteKey := datastore.NewAddressRefKey(
		stellarDetails.ChainSelector,
		datastore.ContractType(rmn_remote.ContractType),
		rmn_remote.Version,
		"",
	)
	rmnRemoteRef, err := env.DataStore.Addresses().Get(rmnRemoteKey)
	require.NoError(t, err)
	require.NotEmpty(t, rmnRemoteRef.Address)

	rmnRemoteAddress, err := hexToContractStrkey(rmnRemoteRef.Address)
	require.NoError(t, err)
	l.Info().Str("rmnRemoteAddress", rmnRemoteAddress).Msg("Found RMN Remote in CCV datastore")

	onRampClient := onrampbindings.NewOnRampClient(deployer, onrampContractID)

	// Create the Stellar source reader with the DEPLOYED OnRamp contract ID
	stellarSourceReader, err := ccvsourcereader.NewSourceReaderWithClient(
		rpc,
		deployer,
		onrampContractID,
		"onramp_1_7_CCIPMessageSent", // Event topic from OnRamp contract
		rmnRemoteAddress,
		l,
	)
	require.NoError(t, err)
	l.Info().Str("onrampContractID", onrampContractID).Msg("Created Stellar source reader")

	// Read fee quoter state
	feeQuoterKey := datastore.NewAddressRefKey(
		stellarDetails.ChainSelector,
		datastore.ContractType(fee_quoter.ContractType),
		fee_quoter.Version,
		"",
	)
	feeQuoterRef, err := env.DataStore.Addresses().Get(feeQuoterKey)
	require.NoError(t, err)
	require.NotEmpty(t, feeQuoterRef.Address)

	feeQuoterContractID, err := hexToContractStrkey(feeQuoterRef.Address)
	require.NoError(t, err)
	l.Info().Str("feeQuoterContractID", feeQuoterContractID).Msg("Found FeeQuoter in CCV datastore")

	t.Run("basic_stellar_to_evm_message", func(t *testing.T) {
		// Get receiver address on EVM
		evmReceiver, err := destChain.GetEOAReceiverAddress()
		require.NoError(t, err)
		l.Info().Str("evmReceiver", hex.EncodeToString(evmReceiver)).Msg("Using EVM receiver address")

		// Record the latest ledger before sending so we know where to scan for events
		latestLedger, err := rpc.GetLatestLedger(ctx)
		require.NoError(t, err)
		startLedger := latestLedger.Sequence
		l.Info().Uint32("startLedger", startLedger).Msg("Recording start ledger before sending")

		// Build the CCIP message
		mockFeeToken := helpers.GenerateMockContractID(t, deployerKP.Address(), "fee-token")
		msg := onrampbindings.StellarToAnyMessage{
			Receiver:     evmReceiver,                    // 20-byte EVM address
			Data:         []byte("hello from stellar"),   // arbitrary payload
			TokenAmounts: []onrampbindings.TokenAmount{}, // no token transfer
			FeeToken:     mockFeeToken,                   // placeholder fee token
			ExtraArgs:    []byte{},                       // no extra args
		}

		// Send the message via the OnRamp's forward_from_router.
		// The deployer is configured as the "router" in the dest chain config,
		// so its transaction signature satisfies the router auth check.
		l.Info().Str("original sender address", deployerKP.Address()).Msg("Sending CCIP message via OnRamp forward_from_router...")
		messageID, err := onRampClient.ForwardFromRouter(
			ctx,
			evmDetails.ChainSelector,
			msg,
			0,                    // fee token amount (fee calculation not yet implemented)
			deployerKP.Address(), // original sender
		)
		require.NoError(t, err)
		l.Info().
			Str("messageID", hex.EncodeToString(messageID[:])).
			Msg("CCIP message sent successfully via OnRamp")

		// Capture the CCIPMessageSent event via the SourceReader
		l.Info().Msg("Fetching CCIPMessageSent events via SourceReader...")
		var events []protocol.MessageSentEvent
		require.Eventually(t, func() bool {
			events, err = stellarSourceReader.FetchMessageSentEvents(
				ctx,
				big.NewInt(int64(startLedger)),
				nil, // toBlock: nil means latest ledger
			)
			if err != nil {
				l.Debug().Err(err).Msg("Failed to fetch events, retrying...")
				return false
			}
			return len(events) > 0
		}, stellarSentTimeout, 2*time.Second, "expected to find CCIPMessageSent event via SourceReader")

		require.Len(t, events, 1, "expected exactly one CCIPMessageSent event")
		capturedEvent := events[0]

		l.Info().
			Str("capturedMessageID", hex.EncodeToString(capturedEvent.MessageID[:])).
			Uint64("sequenceNumber", uint64(capturedEvent.Message.SequenceNumber)).
			Uint64("blockNumber", capturedEvent.BlockNumber).
			Int("receiptsCount", len(capturedEvent.Receipts)).
			Msg("Message captured via SourceReader")

		for i, r := range capturedEvent.Receipts {
			l.Info().
				Int("index", i).
				Str("issuer", r.Issuer.String()).
				Uint64("destGasLimit", r.DestGasLimit).
				Uint32("destBytesOverhead", r.DestBytesOverhead).
				Str("feeTokenAmount", r.FeeTokenAmount.String()).
				Str("extraArgs", hex.EncodeToString(r.ExtraArgs)).
				Str("blob", hex.EncodeToString(r.Blob)).
				Msg("Receipt details")
		}

		// Verify the captured event matches what we sent
		require.Equal(t, protocol.Bytes32(messageID), capturedEvent.MessageID,
			"message ID from OnRamp should match the one captured by SourceReader")

		l.Info().
			Str("messageID", hex.EncodeToString(messageID[:])).
			Msg("Successfully sent and captured CCIP message from Stellar")

		// =====================================================================
		// Verifier and Execution Assertions
		// =====================================================================

		defaultAggregatorClient := env.AggregatorClients[devenvcommon.DefaultCommitteeVerifierQualifier]
		require.NotNil(t, defaultAggregatorClient)

		// Wait for the committee verifier to sign and the aggregator to collect
		// a quorum of signatures for this message.
		testCtx := e2e.NewTestingContext(t, t.Context(), env.Chains, defaultAggregatorClient, env.IndexerMonitor)
		result, err := testCtx.AssertMessage(protocol.Bytes32(messageID), e2e.AssertMessageOptions{
			TickInterval:            1 * time.Second,
			ExpectedVerifierResults: 1, // one default committee verifier
			Timeout:                 tests.WaitTimeout(t),
			AssertVerifierLogs:      false,
			AssertExecutorLogs:      false,
		})
		require.NoError(t, err)
		require.NotNil(t, result.AggregatedResult)
		require.Len(t, result.IndexedVerifications.Results, 1)

		// Wait for the message to be executed on the EVM destination chain.
		ev, err := destChain.WaitOneExecEventBySeqNo(t.Context(), stellarDetails.ChainSelector, 1, tests.WaitTimeout(t))
		require.NoError(t, err)
		require.Equalf(
			t,
			cciptestinterfaces.ExecutionStateSuccess,
			ev.State,
			"message should have been successfully executed, return data: %x",
			ev.ReturnData,
		)

		l.Info().
			Str("messageID", hex.EncodeToString(messageID[:])).
			Msg("Message executed successfully on EVM")
	})
}

// hexToContractStrkey converts a 0x-prefixed hex address to a Stellar contract strkey (C…).
func hexToContractStrkey(hexAddr string) (string, error) {
	raw, err := hex.DecodeString(strings.TrimPrefix(hexAddr, "0x"))
	if err != nil {
		return "", fmt.Errorf("decode hex address: %w", err)
	}
	return strkey.Encode(strkey.VersionByteContract, raw)
}
