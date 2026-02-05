// tags: integration
package integration

import (
	"context"
	"crypto/sha256"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	stellar_devenv "github.com/smartcontractkit/chainlink-ccv/devenv/stellar"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-testing-framework/framework/components/blockchain"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/strkey"
)

const STELLAR_LOCALNET_CHAIN_ID = "baefd734b8d3e48472cff83912375fedbc7573701912fe308af730180f97d74a"

func TestOnRampDeployAndInitialize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Deploy local Stellar network using devenv
	chain := stellar_devenv.New(zerolog.New(os.Stdout))
	input := &blockchain.Input{
		Type:                     "stellar",
		ChainID:                  STELLAR_LOCALNET_CHAIN_ID,
		ContainerName:            "blockchain-stellar",
		Port:                     "8010",
		DockerCmdParamsOverrides: []string{},
	}

	output, err := chain.DeployLocalNetwork(ctx, input)
	if err != nil {
		t.Fatalf("Failed to deploy local network: %v", err)
	}
	t.Logf("Local network deployed at: %s", output.ContainerName)

	rpcURL := output.Nodes[0].ExternalHTTPUrl
	networkPassphrase := chain.NetworkPassphrase()

	// Create RPC client
	rpcClient := rpcclient.NewClient(rpcURL, &http.Client{Timeout: 60 * time.Second})

	deployerKP, err := keypair.Random()
	if err != nil {
		t.Fatalf("Failed to generate deployer keypair: %v", err)
	}

	deployerAddressBytes, err := strkey.Decode(strkey.VersionByteAccountID, deployerKP.Address())
	if err != nil {
		t.Fatalf("Failed to decode deployer address: %v", err)
	}

	err = chain.FundAddresses(ctx, input, []protocol.UnknownAddress{deployerAddressBytes}, nil)
	if err != nil {
		t.Fatalf("Failed to fund deployer account: %v", err)
	}

	deployer := stellar_devenv.NewDeployer(rpcClient, networkPassphrase, deployerKP)

	// Find the project root (where Cargo.toml is)
	projectRoot := findProjectRoot(t)

	// Build the OnRamp contract
	t.Log("Building OnRamp contract...")

	// Deploy the OnRamp contract
	t.Log("Deploying OnRamp contract...")
	onrampSalt := stellar_devenv.GenerateDeterministicSalt(deployerKP.Address(), "onramp")
	onrampWasmPath := filepath.Join(projectRoot, "target", "wasm32-unknown-unknown", "release", "onramp.wasm")
	contractID, err := deployer.DeployContract(ctx, onrampWasmPath, onrampSalt)
	if err != nil {
		t.Fatalf("Failed to deploy OnRamp: %v", err)
	}

	// Generate mock addresses for the configuration
	mockFeeQuoter := generateMockContractID(t, deployerKP.Address(), "fee-quoter")
	mockFeeAggregator := generateMockContractID(t, deployerKP.Address(), "fee-aggregator")
	mockRMNRemote := generateMockContractID(t, deployerKP.Address(), "rmn-remote")
	mockTokenAdminRegistry := generateMockContractID(t, deployerKP.Address(), "token-admin-registry")

	t.Logf("Mock contracts - FeeQuoter: %s, FeeAggregator: %s, RMNRemote: %s, TokenAdminRegistry: %s",
		mockFeeQuoter, mockFeeAggregator, mockRMNRemote, mockTokenAdminRegistry)

	// Create OnRamp client using devenv
	onRampClient := stellar_devenv.NewOnRampClient(rpcClient, networkPassphrase, deployerKP, contractID)

	// Initialize the OnRamp contract using devenv types
	t.Log("Initializing OnRamp contract...")

	staticConfig := stellar_devenv.OnRampStaticConfig{
		ChainSelector:         12345, // Test chain selector
		TokenAdminRegistry:    mockTokenAdminRegistry,
		RMNRemote:             mockRMNRemote,
		MaxUsdCentsPerMessage: 10000, // $100
	}

	dynamicConfig := stellar_devenv.OnRampDynamicConfig{
		FeeQuoter:     mockFeeQuoter,
		FeeAggregator: mockFeeAggregator,
	}

	// Call initialize using the OnRampClient
	err = onRampClient.Initialize(ctx, deployerKP.Address(), staticConfig, dynamicConfig)
	if err != nil {
		t.Fatalf("Failed to initialize OnRamp: %v", err)
	}
	t.Log("OnRamp initialized successfully")

	// Verify static config using OnRampClient
	t.Log("Verifying static config...")
	parsedStaticConfig, err := onRampClient.GetStaticConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to call get_static_config: %v", err)
	}
	if parsedStaticConfig == nil {
		t.Fatal("get_static_config() returned nil")
	}

	if parsedStaticConfig.ChainSelector != staticConfig.ChainSelector {
		t.Errorf("ChainSelector mismatch: expected %d, got %d", staticConfig.ChainSelector, parsedStaticConfig.ChainSelector)
	}
	if parsedStaticConfig.MaxUsdCentsPerMessage != staticConfig.MaxUsdCentsPerMessage {
		t.Errorf("MaxUsdCentsPerMessage mismatch: expected %d, got %d", staticConfig.MaxUsdCentsPerMessage, parsedStaticConfig.MaxUsdCentsPerMessage)
	}
	t.Logf("Static config verified: ChainSelector=%d, MaxUsdCentsPerMessage=%d",
		parsedStaticConfig.ChainSelector, parsedStaticConfig.MaxUsdCentsPerMessage)

	// Verify dynamic config using OnRampClient
	t.Log("Verifying dynamic config...")
	parsedDynamicConfig, err := onRampClient.GetDynamicConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to call get_dynamic_config: %v", err)
	}
	if parsedDynamicConfig == nil {
		t.Fatal("get_dynamic_config() returned nil")
	}

	if parsedDynamicConfig.FeeQuoter != dynamicConfig.FeeQuoter {
		t.Errorf("FeeQuoter mismatch: expected %s, got %s", dynamicConfig.FeeQuoter, parsedDynamicConfig.FeeQuoter)
	}
	if parsedDynamicConfig.FeeAggregator != dynamicConfig.FeeAggregator {
		t.Errorf("FeeAggregator mismatch: expected %s, got %s", dynamicConfig.FeeAggregator, parsedDynamicConfig.FeeAggregator)
	}
	t.Logf("Dynamic config verified: FeeQuoter=%s, FeeAggregator=%s",
		parsedDynamicConfig.FeeQuoter, parsedDynamicConfig.FeeAggregator)

	t.Log("✅ OnRamp deployment and initialization test passed!")
}

// // TestOnRampApplyDestChainConfig tests applying destination chain configuration.
// func TestOnRampApplyDestChainConfig(t *testing.T) {
// 	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
// 		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=1 to run.")
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
// 	defer cancel()

// 	// Deploy local Stellar network using devenv
// 	chain := stellar_devenv.New(zerolog.New(os.Stdout))
// 	output, err := chain.DeployLocalNetwork(ctx, &blockchain.Input{
// 		Type:                     "stellar",
// 		ChainID:                  STELLAR_LOCALNET_CHAIN_ID,
// 		ContainerName:            "blockchain-stellar-destchain",
// 		Port:                     "8011",
// 		DockerCmdParamsOverrides: []string{},
// 	})
// 	if err != nil {
// 		t.Fatalf("Failed to deploy local network: %v", err)
// 	}

// 	rpcURL := output.Nodes[0].ExternalHTTPUrl
// 	friendbotURL := output.Nodes[0].ExternalHTTPUrl
// 	networkPassphrase := chain.NetworkPassphrase()

// 	rpcClient := rpcclient.NewClient(rpcURL, &http.Client{Timeout: 60 * time.Second})
// 	testClient := helpers.NewStellarTestClient(rpcURL, networkPassphrase, friendbotURL)

// 	// Wait for RPC
// 	if err := testClient.WaitForRPC(ctx, 2*time.Minute); err != nil {
// 		t.Fatalf("RPC not ready: %v", err)
// 	}

// 	// Generate and fund deployer
// 	deployerKP, err := helpers.GenerateDeterministicKeypair("test-deployer-destchain")
// 	if err != nil {
// 		t.Fatalf("Failed to generate deployer keypair: %v", err)
// 	}
// 	if err := testClient.FundAccount(ctx, deployerKP.Address()); err != nil {
// 		t.Fatalf("Failed to fund deployer: %v", err)
// 	}

// 	// Build and deploy
// 	projectRoot := findProjectRoot(t)
// 	wasmPath, err := helpers.BuildOnRampWASM(projectRoot)
// 	if err != nil {
// 		t.Fatalf("Failed to build OnRamp WASM: %v", err)
// 	}

// 	deployer := stellar_devenv.NewDeployer(rpcClient, networkPassphrase, deployerKP)
// 	salt := stellar_devenv.GenerateDeterministicSalt(deployerKP.Address(), "onramp-destchain-test")
// 	contractID, err := deployer.DeployContract(ctx, wasmPath, salt)
// 	if err != nil {
// 		t.Fatalf("Failed to deploy OnRamp: %v", err)
// 	}
// 	t.Logf("OnRamp deployed at: %s", contractID)

// 	// Generate mock addresses
// 	mockFeeQuoter := generateMockContractID(t, deployerKP.Address(), "fee-quoter-dc")
// 	mockFeeAggregator := generateMockContractID(t, deployerKP.Address(), "fee-aggregator-dc")
// 	mockRMNRemote := generateMockContractID(t, deployerKP.Address(), "rmn-remote-dc")
// 	mockTokenAdminRegistry := generateMockContractID(t, deployerKP.Address(), "token-admin-registry-dc")
// 	mockRouter := generateMockContractID(t, deployerKP.Address(), "router-dc")
// 	mockExecutor := generateMockContractID(t, deployerKP.Address(), "executor-dc")
// 	mockCCV := generateMockContractID(t, deployerKP.Address(), "ccv-dc")

// 	// Create OnRamp client
// 	onRampClient := stellar_devenv.NewOnRampClient(rpcClient, networkPassphrase, deployerKP, contractID)

// 	// Initialize using devenv types
// 	staticConfig := stellar_devenv.OnRampStaticConfig{
// 		ChainSelector:         12345,
// 		TokenAdminRegistry:    mockTokenAdminRegistry,
// 		RMNRemote:             mockRMNRemote,
// 		MaxUsdCentsPerMessage: 10000,
// 	}
// 	dynamicConfig := stellar_devenv.OnRampDynamicConfig{
// 		FeeQuoter:     mockFeeQuoter,
// 		FeeAggregator: mockFeeAggregator,
// 	}

// 	err = onRampClient.Initialize(ctx, deployerKP.Address(), staticConfig, dynamicConfig)
// 	if err != nil {
// 		t.Fatalf("Failed to initialize OnRamp: %v", err)
// 	}

// 	// Apply destination chain config using devenv types
// 	t.Log("Applying destination chain config...")

// 	destChainSelector := uint64(54321) // EVM destination
// 	destChainConfigs := []stellar_devenv.DestChainConfigArgs{
// 		{
// 			DestChainSelector:    destChainSelector,
// 			Router:               mockRouter,
// 			AddressBytesLength:   20, // EVM address length
// 			TokenReceiverAllowed: true,
// 			MessageNetworkFeeUsd: 10,
// 			TokenNetworkFeeUsd:   5,
// 			BaseExecutionGasCost: 21000,
// 			DefaultExecutor:      mockExecutor,
// 			LaneMandatedCCVs:     []string{},
// 			DefaultCCVs:          []string{mockCCV},
// 			OffRamp:              make([]byte, 20), // 20-byte EVM address
// 		},
// 	}

// 	err = onRampClient.ApplyDestChainConfigUpdates(ctx, destChainConfigs)
// 	if err != nil {
// 		t.Fatalf("Failed to apply dest chain config: %v", err)
// 	}
// 	t.Log("Destination chain config applied")

// 	// Verify by calling get_expected_next_message_number
// 	t.Log("Verifying dest chain config by calling get_expected_next_message_number...")
// 	nextMsgNum, err := onRampClient.GetExpectedNextMessageNumber(ctx, destChainSelector)
// 	if err != nil {
// 		t.Fatalf("Failed to call get_expected_next_message_number: %v", err)
// 	}

// 	// First message number should be 1 (message_number starts at 0, next is 1)
// 	if nextMsgNum != 1 {
// 		t.Errorf("Expected next message number to be 1, got %d", nextMsgNum)
// 	}
// 	t.Logf("Next message number for dest chain %d: %d", destChainSelector, nextMsgNum)

// 	t.Log("✅ OnRamp destination chain config test passed!")
// }

// findProjectRoot finds the root of the chainlink-stellar project.
func findProjectRoot(t *testing.T) string {
	// Start from the current working directory
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Walk up until we find Cargo.toml
	for {
		cargoPath := filepath.Join(dir, "Cargo.toml")
		if _, err := os.Stat(cargoPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding Cargo.toml
			t.Fatal("Could not find project root (Cargo.toml)")
		}
		dir = parent
	}
}

// generateMockContractID generates a deterministic mock contract ID for testing.
func generateMockContractID(t *testing.T, deployerAddress, contractName string) string {
	// Generate a deterministic salt
	salt := stellar_devenv.GenerateDeterministicSalt(deployerAddress, contractName)

	// Encode as a Stellar contract address
	encoded, err := strkey.Encode(strkey.VersionByteContract, salt[:])
	if err != nil {
		t.Fatalf("Failed to encode mock contract ID: %v", err)
	}
	return encoded
}

// generateDeterministicKeypair generates a keypair from a seed string.
// This is used for generating mock addresses.
func generateDeterministicKeypair(seed string) (*keypair.Full, error) {
	hash := sha256.Sum256([]byte(seed))
	return keypair.FromRawSeed(hash)
}
