//go:build integration

package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	onrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/onramp"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
	tokenpoolbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/token_pool"
	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
)

func TestTokenPool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectRoot, deployerKP, deployer, _, _ := GetSharedTestEnv(ctx, t)
	deployerAddr := deployerKP.Address()

	t.Run("deploy and initialize lock-release pool", func(t *testing.T) {
		wasmPath := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "pools_lock_release_pool.wasm")
		salt := deployment.GenerateDeterministicSalt(deployerAddr, "test-lock-release-pool")
		contractID, err := deployer.DeployContract(ctx, wasmPath, salt)
		if err != nil {
			t.Fatalf("Deploy LockRelease pool: %v", err)
		}
		t.Logf("Pool deployed at: %s", contractID)

		mockToken := helpers.GenerateMockContractID(t, deployerAddr, "pool-test-token")
		client := tokenpoolbindings.NewTokenPoolClient(deployer, contractID)

		if err := client.Initialize(ctx, deployerAddr, mockToken); err != nil {
			t.Fatalf("Initialize pool: %v", err)
		}

		gotToken, err := client.GetToken(ctx)
		if err != nil {
			t.Fatalf("GetToken: %v", err)
		}
		if gotToken != mockToken {
			t.Fatalf("token mismatch: want %s, got %s", mockToken, gotToken)
		}

		supported, err := client.IsSupportedToken(ctx, mockToken)
		if err != nil {
			t.Fatalf("IsSupportedToken: %v", err)
		}
		if !supported {
			t.Fatal("expected token to be supported")
		}
		t.Log("Pool deployed, initialized, and token verified")
	})

	t.Run("apply chain updates", func(t *testing.T) {
		wasmPath := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "pools_lock_release_pool.wasm")
		salt := deployment.GenerateDeterministicSalt(deployerAddr, "test-pool-chain-updates")
		contractID, err := deployer.DeployContract(ctx, wasmPath, salt)
		if err != nil {
			t.Fatalf("Deploy pool: %v", err)
		}

		mockToken := helpers.GenerateMockContractID(t, deployerAddr, "pool-chain-test-token")
		client := tokenpoolbindings.NewTokenPoolClient(deployer, contractID)
		if err := client.Initialize(ctx, deployerAddr, mockToken); err != nil {
			t.Fatalf("Initialize pool: %v", err)
		}

		remoteChain := uint64(54321)
		remotePool := make([]byte, 20)
		remoteToken := make([]byte, 20)
		err = client.ApplyChainUpdates(ctx, []tokenpoolbindings.ChainUpdate{
			{
				RemoteChainSelector: remoteChain,
				RemotePoolAddresses: remotePool,
				RemoteTokenAddress:  remoteToken,
			},
		}, nil)
		if err != nil {
			t.Fatalf("ApplyChainUpdates: %v", err)
		}

		supported, err := client.IsSupportedChain(ctx, remoteChain)
		if err != nil {
			t.Fatalf("IsSupportedChain: %v", err)
		}
		if !supported {
			t.Fatal("expected remote chain to be supported after ApplyChainUpdates")
		}
		t.Logf("Chain %d supported after update", remoteChain)
	})

	t.Run("deploy full stack with token pool", func(t *testing.T) {
		const destChain = uint64(11111)
		stack := deployFullStack(ctx, t, projectRoot, deployer, deployerAddr, destChain, "token-pool-stack")

		mockToken := helpers.GenerateMockContractID(t, deployerAddr, "stack-test-token")
		stack.deployTokenPool(ctx, t, projectRoot, deployer, deployerAddr, "token-pool-stack", mockToken)

		if stack.TokenAdminRegistryID == "" {
			t.Fatal("TokenAdminRegistryID not set after deployTokenPool")
		}
		if stack.TokenPoolID == "" {
			t.Fatal("TokenPoolID not set after deployTokenPool")
		}

		pool, err := stack.TokenAdminRegistryClient.GetPool(ctx, mockToken)
		if err != nil {
			t.Fatalf("GetPool: %v", err)
		}
		if pool == nil || *pool != stack.TokenPoolID {
			t.Fatalf("pool mismatch: want %s, got %v", stack.TokenPoolID, pool)
		}
		t.Log("Full stack with token pool: TokenAdminRegistry correctly maps token to pool")
	})

	t.Run("router ccip_send with lock-release pool token amount", func(t *testing.T) {
		const localChain = uint64(11111)
		const remoteDestChain = uint64(22222)

		stack := deployFullStack(ctx, t, projectRoot, deployer, deployerAddr, localChain, "ccip-token-send")

		mockToken := helpers.GenerateMockContractID(t, deployerAddr, "ccip-send-mock-token")
		mockFeeToken := helpers.GenerateMockContractID(t, deployerAddr, "ccip-send-fee-token")

		stack.deployTokenPool(ctx, t, projectRoot, deployer, deployerAddr, "ccip-token-send", mockToken)

		remotePool := make([]byte, 20)
		remoteToken := make([]byte, 20)
		for i := range remotePool {
			remotePool[i] = 0x11
			remoteToken[i] = 0x22
		}
		if err := stack.TokenPoolClient.ApplyChainUpdates(ctx, []tokenpoolbindings.ChainUpdate{{
			RemoteChainSelector: remoteDestChain,
			RemotePoolAddresses: remotePool,
			RemoteTokenAddress:  remoteToken,
		}}, nil); err != nil {
			t.Fatalf("TokenPool ApplyChainUpdates: %v", err)
		}

		_ = deployOutboundSendWire(ctx, t, projectRoot, deployer, deployerAddr, "ccip-token-send", stack,
			localChain, remoteDestChain, mockFeeToken, []string{mockToken})

		defaultExecutor := helpers.GenerateMockContractID(t, deployerAddr, "ccip-token-send-executor")
		extraArgs, err := encodeOnrampExtraArgsV3(onrampbindings.GenericExtraArgsV3{
			Ccvs:               []string{stack.VvrID},
			CcvArgs:            [][]byte{{}},
			Executor:           defaultExecutor,
			ExecutorArgs:       []byte{},
			GasLimit:           0,
			BlockConfirmations: 0,
			TokenReceiver:      []byte{},
			TokenArgs:          []byte{},
		})
		if err != nil {
			t.Fatalf("encode extra args: %v", err)
		}

		evmReceiver := make([]byte, 20)
		for i := range evmReceiver {
			evmReceiver[i] = 0x33
		}

		msg := routerbindings.StellarToAnyMessage{
			Receiver:     evmReceiver,
			Data:         []byte("integration token ccip_send"),
			FeeToken:     mockFeeToken,
			ExtraArgs:    extraArgs,
			TokenAmounts: []routerbindings.TokenAmount{{Token: mockToken, Amount: 1_000_000}},
		}

		requiredFee, err := stack.RouterClient.GetFee(ctx, remoteDestChain, msg)
		if err != nil {
			t.Fatalf("Router GetFee: %v", err)
		}
		if requiredFee <= 0 {
			t.Fatalf("expected positive fee for token message, got %d", requiredFee)
		}
		t.Logf("quoted fee (fee token base units): %d", requiredFee)

		// mockToken is a deterministic address, not a real Stellar asset contract: lock_or_burn's
		// token transfer typically fails once OnRamp reaches the pool. We still exercise deploy wiring,
		// GetFee, and the Router → OnRamp entrypoint.
		_, err = stack.RouterClient.CcipSend(ctx, deployerAddr, remoteDestChain, msg, requiredFee)
		if err == nil {
			t.Fatal("expected CcipSend to fail: mock token cannot authorize transfer for lock_or_burn")
		}
		t.Logf("CcipSend failed as expected without a spendable SAC: %v", err)
	})
}
