//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"

	offrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/offramp"
	rmnproxybindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_proxy"
	rmnremotebindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_remote"
	contracttransmitter "github.com/smartcontractkit/chainlink-stellar/ccv/contract_transmitter"
	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
)

const localChainSelector = uint64(12345)

// deployOffRampDependencies deploys and initializes the RMN Remote and RMN Proxy
// contracts that the OffRamp depends on for curse-checking.
func deployOffRampDependencies(
	ctx context.Context,
	t *testing.T,
	projectRoot string,
	deployer *deployment.Deployer,
	deployerAddr string,
) (rmnRemoteID, rmnProxyID, ccvResolverID, ccvVerifierID string) {
	t.Helper()

	// Deploy RMN Remote
	salt := deployment.GenerateDeterministicSalt(deployerAddr, "rmn-remote-offramp")
	wasmPath := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "rmn_remote.wasm")
	rmnRemoteID, err := deployer.DeployContract(ctx, wasmPath, salt)
	if err != nil {
		t.Fatalf("Failed to deploy RMN Remote: %v", err)
	}
	t.Logf("RMN Remote deployed at: %s", rmnRemoteID)

	rmnRemoteClient := rmnremotebindings.NewRmnRemoteClient(deployer, rmnRemoteID)
	if err := rmnRemoteClient.Initialize(ctx, deployerAddr, localChainSelector); err != nil {
		t.Fatalf("Failed to initialize RMN Remote: %v", err)
	}

	// Deploy RMN Proxy
	salt = deployment.GenerateDeterministicSalt(deployerAddr, "rmn-proxy-offramp")
	wasmPath = filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "rmn_proxy.wasm")
	rmnProxyID, err = deployer.DeployContract(ctx, wasmPath, salt)
	if err != nil {
		t.Fatalf("Failed to deploy RMN Proxy: %v", err)
	}
	t.Logf("RMN Proxy deployed at: %s", rmnProxyID)

	rmnProxyClient := rmnproxybindings.NewRmnProxyClient(deployer, rmnProxyID)
	if err := rmnProxyClient.Initialize(ctx, deployerAddr, rmnRemoteID); err != nil {
		t.Fatalf("Failed to initialize RMN Proxy: %v", err)
	}

	// TODO: Deploy CCV Resolver + Committee Verifier
	ccvResolverID = helpers.GenerateMockContractID(t, deployerAddr, "ccv-resolver")
	ccvVerifierID = helpers.GenerateMockContractID(t, deployerAddr, "ccv-verifier")

	return rmnRemoteID, rmnProxyID, ccvResolverID, ccvVerifierID
}

func TestOffRamp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectRoot, deployerKP, deployer, _, networkPassphrase := GetSharedTestEnv(ctx, t)

	rmnRemoteID, rmnProxyID, _, _ := deployOffRampDependencies(ctx, t, projectRoot, deployer, deployerKP.Address())

	// Deploy OffRamp contract
	t.Log("Deploying OffRamp contract...")
	salt := deployment.GenerateDeterministicSalt(deployerKP.Address(), "offramp")
	wasmPath := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "offramp.wasm")
	offrampContractID, err := deployer.DeployContract(ctx, wasmPath, salt)
	if err != nil {
		t.Fatalf("Failed to deploy OffRamp: %v", err)
	}
	t.Logf("OffRamp deployed at: %s", offrampContractID)

	offrampClient := offrampbindings.NewOffRampClient(deployer, offrampContractID)

	mockTokenAdminRegistry := helpers.GenerateMockContractID(t, deployerKP.Address(), "token-admin-registry")

	lggr := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Used across subtests that exercise the ContractTransmitter path.
	_ = networkPassphrase
	_ = rmnRemoteID
	_ = rmnProxyID
	_ = &lggr

	// ========================================
	// Initialization
	// ========================================

	t.Run("initialize offramp", func(t *testing.T) {
		staticCfg := offrampbindings.StaticConfig{
			ChainSelector:      localChainSelector,
			RmnProxy:           rmnProxyID,
			TokenAdminRegistry: mockTokenAdminRegistry,
		}
		err := offrampClient.Initialize(ctx, deployerKP.Address(), staticCfg)
		if err != nil {
			t.Fatalf("Failed to initialize OffRamp: %v", err)
		}
		t.Log("OffRamp initialized successfully")
	})

	t.Run("double initialize fails", func(t *testing.T) {
		staticCfg := offrampbindings.StaticConfig{
			ChainSelector:      localChainSelector,
			RmnProxy:           rmnProxyID,
			TokenAdminRegistry: mockTokenAdminRegistry,
		}
		err := offrampClient.Initialize(ctx, deployerKP.Address(), staticCfg)
		if err == nil {
			t.Fatal("Expected error on double initialize, got nil")
		}
		t.Logf("Double initialize correctly rejected: %v", err)
	})

	t.Run("watch for StaticConfigSet event after initialization", func(t *testing.T) {
		t.Skip("Placeholder: watch for StaticConfigSet event after initialization")
		// TODO: query events from a known start ledger and verify StaticConfigSetEvent is emitted
	})

	// ========================================
	// Query Functions
	// ========================================

	t.Run("verify owner", func(t *testing.T) {
		owner, err := offrampClient.Owner(ctx)
		if err != nil {
			t.Fatalf("Failed to get owner: %v", err)
		}
		if owner == nil || *owner != deployerKP.Address() {
			t.Errorf("Owner mismatch: expected %s, got %v", deployerKP.Address(), owner)
		}
		t.Logf("Owner verified: %s", *owner)
	})

	t.Run("verify static config", func(t *testing.T) {
		cfg, err := offrampClient.GetStaticConfig(ctx)
		if err != nil {
			t.Fatalf("Failed to get static config: %v", err)
		}
		if cfg.ChainSelector != localChainSelector {
			t.Errorf("ChainSelector mismatch: expected %d, got %d", localChainSelector, cfg.ChainSelector)
		}
		if cfg.RmnProxy != rmnProxyID {
			t.Errorf("RmnProxy mismatch: expected %s, got %s", rmnProxyID, cfg.RmnProxy)
		}
		t.Logf("Static config verified: chain_selector=%d", cfg.ChainSelector)
	})

	t.Run("is not cursed initially", func(t *testing.T) {
		cursed, err := offrampClient.IsCursed(ctx)
		if err != nil {
			t.Fatalf("Failed to check IsCursed: %v", err)
		}
		if cursed {
			t.Fatal("OffRamp should not be cursed initially")
		}
		t.Log("OffRamp is not cursed (as expected)")
	})

	// ========================================
	// Source Chain Configuration
	// ========================================

	t.Run("apply source chain config", func(t *testing.T) {
		// TODO: populate OnRamps, DefaultCcvs, and Router with real deployed addresses
		// once those contracts are available for integration testing
		t.Skip("Placeholder: apply source chain config with a valid source chain selector and verify persistence")
	})

	t.Run("watch for SourceChainConfigSet event", func(t *testing.T) {
		// TODO: after applying source chain config, use offrampClient.WaitForSourceChainConfigSetEvent
		// to verify the event is emitted with the correct selector and config
		t.Skip("Placeholder: verify SourceChainConfigSet event after apply_source_chain_cfg_updates")
	})

	t.Run("get all source chain configs initially empty", func(t *testing.T) {
		selectors, configs, err := offrampClient.GetAllSourceChainConfigs(ctx)
		if err != nil {
			t.Fatalf("Failed to get all source chain configs: %v", err)
		}
		if len(selectors) != 0 || len(configs) != 0 {
			t.Errorf("Expected empty source chain configs, got %d selectors and %d configs", len(selectors), len(configs))
		}
		t.Log("Source chain configs are empty initially (as expected)")
	})

	// ========================================
	// ContractTransmitter — Execute via ConvertAndWriteMessageToChain
	// ========================================

	t.Run("create contract transmitter", func(t *testing.T) {
		_, err := contracttransmitter.NewContractTransmitterWithClient(
			deployer,
			offrampContractID,
			offrampbindings.ExecutionStateChangedEventTopic,
			rmnRemoteID,
			&lggr,
		)
		if err != nil {
			t.Fatalf("Failed to create ContractTransmitter: %v", err)
		}
		t.Log("ContractTransmitter created successfully")
	})

	t.Run("execute with source chain not enabled", func(t *testing.T) {
		t.Skip("Placeholder: execute with source chain not enabled")
		// TODO: construct a protocol.AbstractAggregatedReport with a source chain selector
		// that has no config applied — expect the offramp to reject with SourceChainNotEnabled
	})

	t.Run("execute with CCV length mismatch", func(t *testing.T) {
		// TODO: construct a report where len(CCVS) != len(CCVData) and invoke via
		// ContractTransmitter — expect CCVLengthMismatch error
		t.Skip("Placeholder: ContractTransmitter.ConvertAndWriteMessageToChain should fail on CCV length mismatch")
	})

	t.Run("execute with gas limit override too low", func(t *testing.T) {
		// TODO: construct a report with gas_limit_override > 0 but less than the
		// message's ccip_receive_gas_limit — expect GasLimitOverrideTooLow error
		t.Skip("Placeholder: ContractTransmitter.ConvertAndWriteMessageToChain should fail when gas limit override is too low")
	})

	t.Run("execute with invalid destination chain", func(t *testing.T) {
		// TODO: construct a message where dest_chain_selector does not match the
		// offramp's local chain selector — expect InvalidMessageDestination error
		t.Skip("Placeholder: ContractTransmitter.ConvertAndWriteMessageToChain should fail on destination chain mismatch")
	})

	t.Run("execute with invalid onramp address", func(t *testing.T) {
		// TODO: construct a message with an onramp_address not in the source chain's
		// allowed on_ramps — expect InvalidOnRampAddress error
		t.Skip("Placeholder: ContractTransmitter.ConvertAndWriteMessageToChain should fail on invalid onramp")
	})

	t.Run("execute with invalid offramp address in message", func(t *testing.T) {
		// TODO: construct a message where offramp_address does not match the contract's
		// own address — expect InvalidOffRampAddress error
		t.Skip("Placeholder: ContractTransmitter.ConvertAndWriteMessageToChain should fail when offramp address doesn't match")
	})

	t.Run("execute valid message and watch ExecutionStateChanged event", func(t *testing.T) {
		// TODO: requires full dependency stack (CCV resolver, verifier, router) to be deployed.
		// 1. Deploy and configure CCV resolver + verifier contracts
		// 2. Deploy and configure a Router contract
		// 3. Apply source chain config with valid onramps, ccvs, and router
		// 4. Build a valid protocol.AbstractAggregatedReport
		// 5. Call ContractTransmitter.ConvertAndWriteMessageToChain
		// 6. Use offrampClient.WaitForExecutionStateChangedEvent to assert the event
		// 7. Verify message_id, source_chain_selector, sequence_number, and state fields
		t.Skip("Placeholder: full execute path with ExecutionStateChanged event verification")
	})

	t.Run("execute already-executed message fails", func(t *testing.T) {
		// TODO: after a successful execute, submit the same message again — expect
		// MessageAlreadyExecuted error
		t.Skip("Placeholder: re-execution of a successful message should be rejected")
	})

	t.Run("execute on cursed offramp fails", func(t *testing.T) {
		// TODO: curse the RMN Remote, then attempt execute via ContractTransmitter
		// — expect CursedByRMN error; uncurse afterwards to not affect subsequent tests
		t.Skip("Placeholder: execute should fail when RMN has a global curse active")
	})

	t.Run("execute with invalid CCV resolver address", func(t *testing.T) {
		// TODO: construct a message with an invalid CCV resolver address
		// — expect InvalidCCVResolverAddress error
		t.Skip("Placeholder: execution should fail on invalid CCV resolver address")
	})

	t.Run("execute with empty CCV verifier address set to ensure default CCV is used", func(t *testing.T) {
		// TODO: construct a message with an invalid CCV verifier address
		// — expect InvalidCCVVerifierAddress error
		t.Skip("Placeholder: execution should succeed using the default CCV set for the source chain")
	})

	// ========================================
	// Execution State Queries
	// ========================================

	t.Run("get execution state for unknown message returns Untouched", func(t *testing.T) {
		unknownID := [32]byte{0xFF, 0xFE, 0xFD}
		state, err := offrampClient.GetExecutionState(ctx, unknownID)
		if err != nil {
			t.Fatalf("Failed to get execution state: %v", err)
		}
		if state != offrampbindings.MessageExecutionStateUntouched {
			t.Errorf("Expected Untouched state, got %d", state)
		}
		t.Log("Unknown message correctly returns Untouched state")
	})

	t.Run("get execution state after successful execute", func(t *testing.T) {
		// TODO: after a successful execute, query get_execution_state with the message_id
		// and verify it returns MessageExecutionStateSuccess
		t.Skip("Placeholder: verify execution state is Success after a valid execute")
	})

	// ========================================
	// Ownership
	// ========================================

	t.Run("transfer ownership", func(t *testing.T) {
		// TODO: use offrampClient.TransferOwnership to start a 2-step transfer, then
		// verify GetPendingOwner, AcceptOwnership, and the OwnershipTransferStarted event
		t.Skip("Placeholder: 2-step ownership transfer flow")
	})
}
