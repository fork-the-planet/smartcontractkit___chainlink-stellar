package e2e_tests

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	ccv "github.com/smartcontractkit/chainlink-ccv/devenv"
	"github.com/smartcontractkit/chainlink-ccv/devenv/cciptestinterfaces"
	devenvcommon "github.com/smartcontractkit/chainlink-ccv/devenv/common"
	registry "github.com/smartcontractkit/chainlink-ccv/devenv/registry"
	e2e "github.com/smartcontractkit/chainlink-ccv/devenv/tests/e2e"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	tests "github.com/smartcontractkit/chainlink-common/pkg/utils/tests"
	ccvchain "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
	stellar "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
	"github.com/smartcontractkit/chainlink-testing-framework/framework"
	"github.com/smartcontractkit/chainlink-testing-framework/framework/components/blockchain"
)

const (
	// Test timeouts for Stellar to EVM flow
	stellarSentTimeout = 30 * time.Second
)

// Start the environment required for this test using:
// CTF_CONFIGS=env-stellar-evm.toml go run ./cmd/ccv
// from the build/devenv directory.
func TestStellarToEVMSourceReader(t *testing.T) {
	ccvDevenvDir, err := filepath.Abs("../../../chainlink-ccv/build/devenv")
	require.NoError(t, err)

	// We change the working dir to allow chainlink-ccv command to find the fake
	// services with relative paths
	origDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(ccvDevenvDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.Chdir(origDir)
		framework.RemoveTestContainers()
	})

	// CTF_CONFIGS must be a relative path because ccv.Load joins it with "."
	// via filepath.Join, which strips leading "/" from absolute paths.
	// This path is relative to ccvDevenvDir (the CWD after Chdir).
	configRelPath, err := filepath.Rel(ccvDevenvDir, filepath.Join(origDir, "../env/env-stellar-evm.toml"))
	require.NoError(t, err)

	configOutputPath, err := filepath.Rel(ccvDevenvDir, filepath.Join(origDir, "../env/env-stellar-evm-out.toml"))
	require.NoError(t, err)

	os.Setenv("CTF_CONFIGS", configRelPath)
	os.Setenv("CTF_CONFIG_OUTPUT", configOutputPath)
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "false")
	os.Setenv("STELLAR_DEPLOYER_PRIVATE_KEY", "c3636a3c2491503668222f58e783d956703fdcfbaea7e5ac7a384e7f2378969b")

	stellarChainID := "baefd734b8d3e48472cff83912375fedbc7573701912fe308af730180f97d74a"

	ctx := ccv.Plog.WithContext(t.Context())
	l := zerolog.Ctx(ctx)

	// Register the Stellar chain adapter by using the EVM adapter as a base
	global_family_registry := registry.GetGlobalChainFamilyAdapterRegistry()
	evm_adapter, ok := global_family_registry.GetChainFamily(chain_selectors.FamilyEVM)
	require.True(t, ok)
	require.NotNil(t, evm_adapter)

	stellar_adapter := ccvchain.NewChainFamilyAdapter(evm_adapter)
	global_family_registry.RegisterChainFamily(chain_selectors.FamilyStellar, stellar_adapter)

	// Register the Stellar chain implementation
	registry.GetGlobalChainImplRegistry().
		Register(stellarChainID, chain_selectors.FamilyStellar, stellar.New(zerolog.New(os.Stdout)))

	in, err := ccv.NewEnvironment()
	require.NoError(t, err)

	// Load EVM chain for destination interactions
	lib, err := ccv.NewLib(l, configOutputPath, chain_selectors.FamilyEVM)
	require.NoError(t, err)
	chains, err := lib.ChainsMap(ctx)
	require.NoError(t, err)
	require.NotNil(t, chains)

	// t.Cleanup(func() {
	// 	_, err := framework.SaveContainerLogs(fmt.Sprintf("%s-%s", framework.DefaultCTFLogsDir, t.Name()))
	// 	require.NoError(t, err)
	// })

	// Set up aggregator client
	var indexerMonitor *ccv.IndexerMonitor
	indexerClient, err := lib.Indexer()
	require.NoError(t, err)
	indexerMonitor, err = ccv.NewIndexerMonitor(
		zerolog.Ctx(ctx).With().Str("component", "indexer-client").Logger(),
		indexerClient)
	require.NoError(t, err)
	require.NotNil(t, indexerMonitor)

	aggregatorClients := make(map[string]*ccv.AggregatorClient)
	for qualifier := range in.AggregatorEndpoints {
		client, err := in.NewAggregatorClientForCommittee(
			zerolog.Ctx(ctx).With().Str("component", fmt.Sprintf("aggregator-client-%s", qualifier)).Logger(),
			qualifier)
		require.NoError(t, err)
		require.NotNil(t, client)
		aggregatorClients[qualifier] = client
		t.Cleanup(func() {
			client.Close()
		})
	}
	defaultAggregatorClient := aggregatorClients[devenvcommon.DefaultCommitteeVerifierQualifier]
	require.NotNil(t, defaultAggregatorClient)

	configsOutput, err := ccv.LoadOutput[ccv.Cfg](configOutputPath)
	require.NoError(t, err)
	require.NotNil(t, configsOutput)

	// Find Stellar chain
	var stellarChain *blockchain.Input
	for _, bc := range configsOutput.Blockchains {
		if bc.Type == blockchain.TypeStellar {
			stellarChain = bc
			break
		}
	}
	require.NotNil(t, stellarChain, "need at least one stellar chain for this test")

	// Find EVM chain
	var evmChain *blockchain.Input
	for _, bc := range configsOutput.Blockchains {
		if bc.Type == blockchain.TypeAnvil {
			evmChain = bc
			break
		}
	}
	require.NotNil(t, evmChain, "need at least one evm chain for this test")

	stellarDetails, err := chain_selectors.GetChainDetailsByChainIDAndFamily(stellarChain.ChainID, chain_selectors.FamilyStellar)
	require.NoError(t, err)
	require.NotNil(t, stellarDetails)

	evmDetails, err := chain_selectors.GetChainDetailsByChainIDAndFamily(evmChain.ChainID, chain_selectors.FamilyEVM)
	require.NoError(t, err)
	require.NotNil(t, evmDetails)

	destChain := chains[evmDetails.ChainSelector]
	require.NotNil(t, destChain)

	t.Run("basic_stellar_to_evm_message", func(t *testing.T) {
		// Get receiver address on EVM
		evmReceiver, err := destChain.GetEOAReceiverAddress()
		require.NoError(t, err)
		l.Info().Str("evmReceiver", evmReceiver.String()).Msg("Using EVM receiver address")

		// TODO: Once Stellar impl is fully integrated, use the Stellar chain from lib
		// For now, we'll construct a test message manually similar to Canton test

		// Create a test message from Stellar to EVM
		seqNr := int64(1)
		msg := newStellarToEVMMessage(
			t,
			protocol.ChainSelector(stellarDetails.ChainSelector),
			protocol.ChainSelector(evmDetails.ChainSelector),
			seqNr,
			evmReceiver,
		)

		messageID, err := msg.MessageID()
		require.NoError(t, err)

		// l.Info().
		// 	Str("messageID", hex.EncodeToString(messageID[:])).
		// 	Int64("sequenceNumber", seqNr).
		// 	Msg("Created test message from Stellar to EVM")

		// Wait for verification through the aggregator
		testCtx := e2e.NewTestingContext(t, t.Context(), chains, defaultAggregatorClient, indexerMonitor)
		result, err := testCtx.AssertMessage(msg.MustMessageID(), e2e.AssertMessageOptions{
			TickInterval:            1 * time.Second,
			ExpectedVerifierResults: 1, // just committee verifier
			Timeout:                 tests.WaitTimeout(t),
			AssertVerifierLogs:      false,
			AssertExecutorLogs:      false,
		})
		require.NoError(t, err)
		require.NotNil(t, result.AggregatedResult)
		require.Len(t, result.IndexedVerifications.Results, 1)

		// Wait for execution on EVM
		ev, err := destChain.WaitOneExecEventBySeqNo(t.Context(), stellarDetails.ChainSelector, uint64(seqNr), tests.WaitTimeout(t))
		require.NoError(t, err)
		require.Equalf(
			t,
			cciptestinterfaces.ExecutionStateSuccess,
			ev.State,
			"message %d should have been successfully executed, return data: %x",
			seqNr,
			ev.ReturnData,
		)

		l.Info().
			Str("messageID", hex.EncodeToString(messageID[:])).
			Msg("Message executed successfully on EVM")
	})
}

// newStellarToEVMMessage creates a test CCIP message from Stellar to EVM.
func newStellarToEVMMessage(
	t *testing.T,
	sourceSelector,
	destSelector protocol.ChainSelector,
	seqNr int64,
	evmReceiver protocol.UnknownAddress,
) protocol.Message {
	// For testing, we use placeholder addresses
	// In production, these would come from deployed contracts
	stellarOnRamp := protocol.UnknownAddress(make([]byte, 32)) // Stellar addresses are 32 bytes
	evmOffRamp := protocol.UnknownAddress(make([]byte, 20))    // EVM addresses are 20 bytes
	stellarSender := protocol.UnknownAddress(make([]byte, 32))

	// Placeholder CCV and executor addresses
	ccvAddresses := []protocol.UnknownAddress{
		protocol.UnknownAddress(make([]byte, 32)), // Stellar CCV address
	}
	executorAddress := protocol.UnknownAddress(make([]byte, 32))

	// Compute the CCV and executor hash for validation
	ccvAndExecutorHash, err := protocol.ComputeCCVAndExecutorHash(ccvAddresses, executorAddress)
	require.NoError(t, err)

	msg, err := protocol.NewMessage(
		sourceSelector,
		destSelector,
		protocol.SequenceNumber(seqNr),
		stellarOnRamp,
		evmOffRamp,
		1,                  // finality
		200_000,            // execution gas limit
		100_000,            // ccip receive gas limit
		ccvAndExecutorHash, // ccv and executor hash
		stellarSender,
		evmReceiver,
		[]byte{},                       // dest blob, not required for EVM
		[]byte("message from stellar"), // message data
		nil,                            // token transfer
	)
	require.NoError(t, err)

	return *msg
}
