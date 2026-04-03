//go:build integration

package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

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
}
