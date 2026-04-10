//go:build integration

package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stellar/go-stellar-sdk/keypair"

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

	ccvResolverID = helpers.GenerateMockContractID(t, deployerAddr, "ccv-resolver")
	ccvVerifierID = helpers.GenerateMockContractID(t, deployerAddr, "ccv-verifier")

	return rmnRemoteID, rmnProxyID, ccvResolverID, ccvVerifierID
}

func TestOffRamp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectRoot, deployerKP, deployer, _, networkPassphrase, _ := GetSharedTestEnv(ctx, t)

	rmnRemoteID, rmnProxyID, _, _ := deployOffRampDependencies(ctx, t, projectRoot, deployer, deployerKP.Address())

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

	_ = networkPassphrase
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
		// Full source chain config application requires the complete contract stack
		// (Router + VVR + CommitteeVerifier). The basic TestOffRamp deploys only
		// OffRamp + RMN. See TestOffRampExecute for full-stack source chain config.
		t.Skip("Source chain config requires full contract stack; covered in TestOffRampExecute")
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
	})

	t.Run("execute with CCV length mismatch", func(t *testing.T) {
		t.Skip("Placeholder: ContractTransmitter.ConvertAndWriteMessageToChain should fail on CCV length mismatch")
	})

	t.Run("execute with gas limit override too low", func(t *testing.T) {
		t.Skip("Placeholder: ContractTransmitter.ConvertAndWriteMessageToChain should fail when gas limit override is too low")
	})

	t.Run("execute with invalid destination chain", func(t *testing.T) {
		t.Skip("Placeholder: ContractTransmitter.ConvertAndWriteMessageToChain should fail on destination chain mismatch")
	})

	t.Run("execute with invalid onramp address", func(t *testing.T) {
		t.Skip("Placeholder: ContractTransmitter.ConvertAndWriteMessageToChain should fail on invalid onramp")
	})

	t.Run("execute with invalid offramp address in message", func(t *testing.T) {
		t.Skip("Placeholder: ContractTransmitter.ConvertAndWriteMessageToChain should fail when offramp address doesn't match")
	})

	t.Run("execute valid message and watch ExecutionStateChanged event", func(t *testing.T) {
		t.Skip("Placeholder: full execute path with ExecutionStateChanged event verification")
	})

	t.Run("execute already-executed message fails", func(t *testing.T) {
		t.Skip("Placeholder: re-execution of a successful message should be rejected")
	})

	t.Run("execute on cursed offramp fails", func(t *testing.T) {
		t.Skip("Placeholder: execute should fail when RMN has a global curse active")
	})

	t.Run("execute with invalid CCV resolver address", func(t *testing.T) {
		t.Skip("Placeholder: execution should fail on invalid CCV resolver address")
	})

	t.Run("execute with empty CCV verifier address set to ensure default CCV is used", func(t *testing.T) {
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
		t.Skip("Placeholder: verify execution state is Success after a valid execute")
	})

	// ========================================
	// Ownership
	// ========================================

	t.Run("transfer ownership", func(t *testing.T) {
		newOwnerKP, err := keypair.Random()
		if err != nil {
			t.Fatalf("Failed to generate new owner keypair: %v", err)
		}

		err = offrampClient.TransferOwnership(ctx, newOwnerKP.Address())
		if err != nil {
			t.Fatalf("TransferOwnership failed: %v", err)
		}

		pending, err := offrampClient.GetPendingOwner(ctx)
		if err != nil {
			t.Fatalf("GetPendingOwner failed: %v", err)
		}
		if pending == nil || *pending != newOwnerKP.Address() {
			t.Fatalf("PendingOwner mismatch: expected %s, got %v", newOwnerKP.Address(), pending)
		}
		t.Logf("Ownership transfer initiated to %s", newOwnerKP.Address())

		err = offrampClient.CancelOwnershipTransfer(ctx)
		if err != nil {
			t.Fatalf("CancelOwnershipTransfer failed: %v", err)
		}

		owner, err := offrampClient.Owner(ctx)
		if err != nil {
			t.Fatalf("Owner check after cancel failed: %v", err)
		}
		if owner == nil || *owner != deployerKP.Address() {
			t.Fatalf("Owner should still be original deployer after cancel, got %v", owner)
		}
		t.Log("Ownership transfer cancelled; original owner retained")
	})
}

// TestOffRampExecute exercises the full execute path using a complete contract stack
// deployed via deployFullStack. These tests are separated from the basic TestOffRamp
// suite because they require the full contract dependency chain.
func TestOffRampExecute(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectRoot, deployerKP, deployer, _, _, _ := GetSharedTestEnv(ctx, t)

	stack := deployFullStack(ctx, t, projectRoot, deployer, deployerKP.Address(), localChainSelector, "offramp-exec", false)

	t.Run("execute valid message succeeds", func(t *testing.T) {
		encoded, msgID, verifierBlob := stack.buildValidMessage(t, localChainSelector, 1, []byte("offramp-execute-test"))

		err := stack.OfframpClient.Execute(ctx, encoded, []string{stack.VvrID}, [][]byte{verifierBlob}, 0)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		state, err := stack.OfframpClient.GetExecutionState(ctx, msgID)
		if err != nil {
			t.Fatalf("GetExecutionState: %v", err)
		}
		if state != offrampbindings.MessageExecutionStateSuccess {
			t.Fatalf("execution state = %d, want Success (%d)", state, offrampbindings.MessageExecutionStateSuccess)
		}
		t.Logf("Execute succeeded; message ID %x has state Success", msgID[:8])
	})

	t.Run("execute already-executed message fails", func(t *testing.T) {
		encoded, _, verifierBlob := stack.buildValidMessage(t, localChainSelector, 1, []byte("offramp-execute-test"))

		err := stack.OfframpClient.Execute(ctx, encoded, []string{stack.VvrID}, [][]byte{verifierBlob}, 0)
		if err == nil {
			t.Fatal("Expected error on duplicate execute, got nil")
		}
		if !strings.Contains(err.Error(), "Error(Contract") {
			t.Fatalf("Expected contract error, got: %v", err)
		}
		t.Logf("Duplicate execution correctly rejected: %v", err)
	})

	t.Run("get execution state after successful execute", func(t *testing.T) {
		_, msgID, _ := stack.buildValidMessage(t, localChainSelector, 1, []byte("offramp-execute-test"))
		state, err := stack.OfframpClient.GetExecutionState(ctx, msgID)
		if err != nil {
			t.Fatalf("GetExecutionState: %v", err)
		}
		if state != offrampbindings.MessageExecutionStateSuccess {
			t.Errorf("Expected Success state, got %d", state)
		}
		t.Logf("Execution state confirmed: %d (Success)", state)
	})

	t.Run("execute on cursed offramp fails", func(t *testing.T) {
		var globalCurseSubject [16]byte
		copy(globalCurseSubject[:], []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01})

		err := stack.RmnRemoteClient.Curse(ctx, [][16]byte{globalCurseSubject})
		if err != nil {
			t.Fatalf("Failed to curse: %v", err)
		}

		encoded, _, verifierBlob := stack.buildValidMessage(t, localChainSelector, 2, []byte("cursed-test"))
		err = stack.OfframpClient.Execute(ctx, encoded, []string{stack.VvrID}, [][]byte{verifierBlob}, 0)
		if err == nil {
			t.Fatal("Expected error executing on cursed offramp, got nil")
		}
		if !strings.Contains(err.Error(), "Error(Contract") {
			t.Fatalf("Expected contract error for curse, got: %v", err)
		}
		t.Logf("Execute on cursed offramp correctly rejected: %v", err)

		err = stack.RmnRemoteClient.Uncurse(ctx, [][16]byte{globalCurseSubject})
		if err != nil {
			t.Fatalf("Failed to uncurse: %v", err)
		}
		t.Log("Uncursed successfully; offramp restored")
	})

	t.Run("execute with wrong destination chain fails", func(t *testing.T) {
		wrongDest := uint64(99998)
		encoded, _, verifierBlob := stack.buildValidMessage(t, wrongDest, 3, []byte("wrong-dest"))

		err := stack.OfframpClient.Execute(ctx, encoded, []string{stack.VvrID}, [][]byte{verifierBlob}, 0)
		if err == nil {
			t.Fatal("Expected error for wrong destination chain, got nil")
		}
		if !strings.Contains(err.Error(), "Error(Contract") {
			t.Fatalf("Expected contract error, got: %v", err)
		}
		t.Logf("Wrong destination chain correctly rejected: %v", err)
	})

	t.Run("execute with invalid onramp address fails", func(t *testing.T) {
		sender := bytes.Repeat([]byte{0xcd}, 20)
		var ccvHashZero [32]byte
		badOnRamp := bytes.Repeat([]byte{0xFF}, 32)
		encoded, err := encodeCcipMessageV1(ccipV1Wire{
			SourceChainSelector: remoteSourceChain,
			DestChainSelector:   localChainSelector,
			SequenceNumber:      4,
			ExecutionGasLimit:   500_000,
			CcipReceiveGasLimit: 200_000,
			Finality:            0,
			CcvExecutorHash:     ccvHashZero,
			OnRampAddress:       badOnRamp,
			OffRampAddress:      stack.OffRampSuffix,
			Sender:              sender,
			Receiver:            stack.ReceiverRaw,
			DestBlob:            nil,
			TokenTransfer:       nil,
			Data:                []byte("bad-onramp"),
		})
		if err != nil {
			t.Fatalf("encodeCcipMessageV1: %v", err)
		}

		msgID := keccak256MessageID(encoded)
		verifierBlob := stack.signVerifierBlob(t, msgID)
		err = stack.OfframpClient.Execute(ctx, encoded, []string{stack.VvrID}, [][]byte{verifierBlob}, 0)
		if err == nil {
			t.Fatal("Expected error for invalid onramp, got nil")
		}
		if !strings.Contains(err.Error(), "Error(Contract") {
			t.Fatalf("Expected contract error, got: %v", err)
		}
		t.Logf("Invalid onramp address correctly rejected: %v", err)
	})

	t.Run("execute with second valid message succeeds", func(t *testing.T) {
		encoded, msgID, verifierBlob := stack.buildValidMessage(t, localChainSelector, 5, []byte("second-message"))

		err := stack.OfframpClient.Execute(ctx, encoded, []string{stack.VvrID}, [][]byte{verifierBlob}, 0)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		state, err := stack.OfframpClient.GetExecutionState(ctx, msgID)
		if err != nil {
			t.Fatalf("GetExecutionState: %v", err)
		}
		if state != offrampbindings.MessageExecutionStateSuccess {
			t.Fatalf("execution state = %d, want Success", state)
		}
		t.Logf("Second message executed successfully; ID %x", msgID[:8])
	})
}
