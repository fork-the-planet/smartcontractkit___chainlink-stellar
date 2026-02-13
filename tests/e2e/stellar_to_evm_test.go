package e2e_tests

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stretchr/testify/require"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	ccv "github.com/smartcontractkit/chainlink-ccv/devenv"
	devenvcommon "github.com/smartcontractkit/chainlink-ccv/devenv/common"
	registry "github.com/smartcontractkit/chainlink-ccv/devenv/registry"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	onrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/onramp"
	ccvchain "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
	stellar "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
	ccvsourcereader "github.com/smartcontractkit/chainlink-stellar/ccv/source_reader"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
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
//
// Contracts must be compiled before running:
// make build
// from the chainlink-stellar root directory.
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

	// =========================================================================
	// Deploy and initialize a real OnRamp contract on the Stellar local network
	// =========================================================================

	// Get Stellar network configuration from the environment output
	require.NotEmpty(t, stellarChain.Out.Nodes, "stellar chain output must have nodes")
	stellarRPCURL := stellarChain.Out.Nodes[0].ExternalHTTPUrl
	require.NotEmpty(t, stellarRPCURL, "stellar RPC URL is required")
	networkPassphrase := stellarChain.Out.NetworkSpecificData.StellarNetwork.NetworkPassphrase
	require.NotEmpty(t, networkPassphrase, "network passphrase is required")
	friendbotURL := stellarChain.Out.NetworkSpecificData.StellarNetwork.FriendbotURL
	require.NotEmpty(t, friendbotURL, "friendbot URL is required")

	l.Info().
		Str("stellarRPCURL", stellarRPCURL).
		Str("networkPassphrase", networkPassphrase).
		Str("friendbotURL", friendbotURL).
		Msg("Stellar network configuration")

	// Create deployer keypair from the environment variable
	deployerPrivKeyHex := os.Getenv("STELLAR_DEPLOYER_PRIVATE_KEY")
	require.NotEmpty(t, deployerPrivKeyHex, "STELLAR_DEPLOYER_PRIVATE_KEY env var is required")
	seedBytes, err := hex.DecodeString(deployerPrivKeyHex)
	require.NoError(t, err)
	var seed [32]byte
	copy(seed[:], seedBytes)
	deployerKP, err := keypair.FromRawSeed(seed)
	require.NoError(t, err)
	l.Info().Str("deployerAddress", deployerKP.Address()).Msg("Created deployer keypair")

	// Create Soroban RPC client
	rpc := rpcclient.NewClient(stellarRPCURL, &http.Client{Timeout: 60 * time.Second})
	t.Cleanup(func() { rpc.Close() })

	// Fund deployer account via Friendbot
	l.Info().Msg("Funding deployer via Friendbot...")
	faucetURL := fmt.Sprintf("%s?addr=%s", friendbotURL, deployerKP.Address())
	require.Eventually(t, func() bool {
		resp, httpErr := http.Get(faucetURL)
		if httpErr != nil {
			l.Debug().Err(httpErr).Msg("Friendbot request failed, retrying...")
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			l.Debug().Int("status", resp.StatusCode).Msg("Friendbot not ready, retrying...")
			return false
		}
		return true
	}, 3*time.Minute, 5*time.Second, "failed to fund deployer via Friendbot")
	l.Info().Msg("Deployer funded successfully")

	// Create the Deployer (implements bindings.Invoker) for contract interactions
	deployer := stellardeployment.NewDeployer(rpc, networkPassphrase, deployerKP)

	// Locate the compiled OnRamp WASM
	stellarRoot, err := filepath.Abs(filepath.Join(origDir, "../.."))
	require.NoError(t, err)
	onrampWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "onramp.wasm")
	if _, statErr := os.Stat(onrampWasmPath); os.IsNotExist(statErr) {
		t.Skipf("OnRamp WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", onrampWasmPath)
	}

	// Deploy the OnRamp contract
	l.Info().Str("wasmPath", onrampWasmPath).Msg("Deploying OnRamp contract...")
	onrampSalt := stellardeployment.GenerateDeterministicSalt(deployerKP.Address(), "onramp")
	onrampContractID, err := deployer.DeployContract(ctx, onrampWasmPath, onrampSalt)
	require.NoError(t, err)
	l.Info().Str("contractID", onrampContractID).Msg("OnRamp contract deployed")

	// Create the OnRamp client
	onRampClient := onrampbindings.NewOnRampClient(deployer, onrampContractID)

	// Initialize the OnRamp with mock dependency contracts
	mockFeeQuoter := helpers.GenerateMockContractID(t, deployerKP.Address(), "fee-quoter")
	mockFeeAggregator := helpers.GenerateMockContractID(t, deployerKP.Address(), "fee-aggregator")
	mockRMNRemote := helpers.GenerateMockContractID(t, deployerKP.Address(), "rmn-remote")
	mockTokenAdminRegistry := helpers.GenerateMockContractID(t, deployerKP.Address(), "token-admin-registry")

	err = onRampClient.Initialize(ctx, deployerKP.Address(), onrampbindings.StaticConfig{
		ChainSelector:         stellarDetails.ChainSelector,
		TokenAdminRegistry:    mockTokenAdminRegistry,
		RmnRemote:             mockRMNRemote,
		MaxUsdCentsPerMessage: 10000, // $100
	}, onrampbindings.DynamicConfig{
		FeeQuoter:     mockFeeQuoter,
		FeeAggregator: mockFeeAggregator,
	})
	require.NoError(t, err)
	l.Info().Msg("OnRamp initialized")

	// Configure the destination chain (EVM) on the OnRamp
	mockCCV := helpers.GenerateMockContractID(t, deployerKP.Address(), "ccv-default")
	mockExecutor := helpers.GenerateMockContractID(t, deployerKP.Address(), "executor-default")

	err = onRampClient.ApplyDestChainConfigUpdates(ctx, []onrampbindings.DestChainConfigArgs{
		{
			DestChainSelector:         evmDetails.ChainSelector,
			Router:                    deployerKP.Address(), // deployer acts as router
			AddressBytesLength:        20,                   // EVM addresses are 20 bytes
			DefaultCcvs:               []string{mockCCV},
			DefaultExecutor:           mockExecutor,
			LaneMandatedCcvs:          []string{},
			OffRamp:                   make([]byte, 20), // placeholder
			BaseExecutionGasCost:      200_000,
			MessageNetworkFeeUsdCents: 0,
			TokenNetworkFeeUsdCents:   0,
			TokenReceiverAllowed:      false,
		},
	})
	require.NoError(t, err)
	l.Info().Uint64("destChainSelector", evmDetails.ChainSelector).Msg("Dest chain config applied")

	// Create the Stellar source reader with the DEPLOYED OnRamp contract ID
	stellarSourceReader, err := ccvsourcereader.NewSourceReaderWithClient(
		rpc,
		onrampContractID,
		"onramp_1_7_CCIPMessageSent", // Event topic from OnRamp contract
		l,
	)
	require.NoError(t, err)
	l.Info().Str("onrampContractID", onrampContractID).Msg("Created Stellar source reader")

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
		l.Info().Msg("Sending CCIP message via OnRamp forward_from_router...")
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
			Msg("Message captured via SourceReader")

		// Verify the captured event matches what we sent
		require.Equal(t, protocol.Bytes32(messageID), capturedEvent.MessageID,
			"message ID from OnRamp should match the one captured by SourceReader")

		l.Info().
			Str("messageID", hex.EncodeToString(messageID[:])).
			Msg("Successfully sent and captured CCIP message from Stellar")

		// =====================================================================
		// Verifier and Execution Assertions
		// =====================================================================

		// NOTE: These assertions require the verifier to be configured with the
		// actual deployed OnRamp contract ID. Currently, the environment setup
		// generates deterministic placeholder addresses in DeployContractsForSelector
		// (chain.go) and the verifier config uses those placeholders.
		//
		// TODO: Update the environment setup to:
		// 1. Deploy real Stellar contracts (router, onramp, etc.)
		// 2. Pass the deployed OnRamp contract ID to the verifier config
		// Once that is done, these assertions should pass end-to-end.

		// Wait for verification through the aggregator
		// testCtx := e2e.NewTestingContext(t, t.Context(), chains, defaultAggregatorClient, indexerMonitor)
		// result, err := testCtx.AssertMessage(protocol.Bytes32(messageID), e2e.AssertMessageOptions{
		// 	TickInterval:            1 * time.Second,
		// 	ExpectedVerifierResults: 1, // just committee verifier
		// 	Timeout:                 tests.WaitTimeout(t),
		// 	AssertVerifierLogs:      false,
		// 	AssertExecutorLogs:      false,
		// })
		// require.NoError(t, err)
		// require.NotNil(t, result.AggregatedResult)
		// require.Len(t, result.IndexedVerifications.Results, 1)

		// // Wait for execution on EVM
		// ev, err := destChain.WaitOneExecEventBySeqNo(t.Context(), stellarDetails.ChainSelector, 1, tests.WaitTimeout(t))
		// require.NoError(t, err)
		// require.Equalf(
		// 	t,
		// 	cciptestinterfaces.ExecutionStateSuccess,
		// 	ev.State,
		// 	"message should have been successfully executed, return data: %x",
		// 	ev.ReturnData,
		// )

		// l.Info().
		// 	Str("messageID", hex.EncodeToString(messageID[:])).
		// 	Msg("Message executed successfully on EVM")
	})
}
