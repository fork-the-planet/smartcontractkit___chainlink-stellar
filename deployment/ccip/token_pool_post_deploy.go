package ccip

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	tokenpoolbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/token_pool"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	sacops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/sac_token"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
	tarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/token_admin_registry"
	poolops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/token_pool"
)

// See [DevenvTestTokenPoolQualifier].

// initialPoolLiquidity is the amount of SAC base units seeded into the
// lock-release pool so that inbound release_or_mint calls have liquidity.
// 100 tokens at 7 decimals = 1_000_000_000 base units.
const initialPoolLiquidity int64 = 1_000_000_000

// DeployLockReleaseTestTokenPool deploys the lock-release pool and, when Friendbot is
// available, creates the test SAC token, initializes the pool, and registers token+pool
// in TokenAdminRegistry. Intended to run from PostDeployContractsForSelector after core CCIP deploy.
// opBundle must be the parent CLDF operations bundle for the deploy run (e.g. env.OperationsBundle).
func DeployLockReleaseTestTokenPool(ctx context.Context, opBundle cldfops.Bundle, host CCIPDevenvHost) error {
	stellarRoot, err := stellarutil.FindStellarRoot()
	if err != nil {
		return fmt.Errorf("find stellar root: %w", err)
	}

	poolWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "pools_lock_release_pool.wasm")
	if _, statErr := os.Stat(poolWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("LockReleasePool WASM not found at %s. Run 'make build'.", poolWasmPath)
	}
	host.Logger().Info().Str("wasmPath", poolWasmPath).Msg("Deploying LockRelease pool contract (post-deploy)...")
	poolSalt := stellardeployment.GenerateDeterministicSalt(host.DeployerKeypair().Address(), "lock-release-pool")
	deps := stellardeps.FromDeployer(host.Deployer())
	poolRep, err := cldfops.ExecuteOperation(opBundle, poolops.Deploy, deps, stellarops.DeployInput{WasmPath: poolWasmPath, Salt: poolSalt})
	if err != nil {
		return fmt.Errorf("failed to deploy LockRelease pool: %w", err)
	}
	poolContractID := poolRep.Output.ContractID
	poolClient := tokenpoolbindings.NewTokenPoolClient(host.Deployer(), poolContractID)
	host.SetTokenPool(poolContractID, poolClient)
	host.Logger().Info().Str("contractID", poolContractID).Msg("LockRelease pool deployed")

	tarClient := host.TokenAdminRegistryClient()
	if tarClient == nil {
		return fmt.Errorf("token admin registry client is nil before pool token setup")
	}

	if host.FriendbotURL() == "" {
		host.Logger().Warn().Msg("Friendbot URL not available; skipping test token deployment. Token transfer E2E tests will not work.")
		return nil
	}

	tokenContractID, tokenErr := host.CreateTestToken(ctx, host.FriendbotURL())
	if tokenErr != nil {
		return fmt.Errorf("failed to create test token: %w", tokenErr)
	}
	host.SetTestToken(tokenContractID)

	// Match typical Stellar SAC / pool configuration (EVM `uint8` token decimals on the pool).
	const testTokenPoolDecimals uint32 = 7
	routerContractID := host.RouterContractID()
	if routerContractID == "" {
		return fmt.Errorf("router contract ID is empty; token pool initialize requires router (deploy core CCIP first)")
	}
	rampRegistryContractID := host.RampRegistryContractID()
	if rampRegistryContractID == "" {
		return fmt.Errorf("ramp registry contract ID is empty; token pool initialize requires ramp registry (deploy core CCIP first)")
	}
	if _, err := cldfops.ExecuteOperation(opBundle, poolops.Initialize, deps, poolops.InitializeInput{
		ContractID:    poolContractID,
		Owner:         host.DeployerKeypair().Address(),
		Token:         tokenContractID,
		TokenDecimals: testTokenPoolDecimals,
		Router:        routerContractID,
		RampRegistry:  rampRegistryContractID,
	}); err != nil {
		return fmt.Errorf("failed to initialize pool with token: %w", err)
	}

	// Fund the pool with SAC liquidity so inbound release_or_mint calls succeed.
	deployerAddr := host.DeployerKeypair().Address()
	if _, err := cldfops.ExecuteOperation(opBundle, sacops.Transfer, deps, sacops.TransferInput{
		ContractID: tokenContractID,
		From:       deployerAddr,
		To:         poolContractID,
		Amount:     initialPoolLiquidity,
	}); err != nil {
		return fmt.Errorf("fund pool with SAC liquidity: %w", err)
	}
	host.Logger().Info().
		Str("pool", poolContractID).
		Int64("amount", initialPoolLiquidity).
		Msg("Funded lock-release pool with SAC liquidity")

	tarID := tarClient.ContractID()
	if _, err := cldfops.ExecuteOperation(opBundle, tarops.ProposeAdministrator, deps, tarops.ProposeAdministratorInput{
		ContractID:    tarID,
		Caller:        deployerAddr,
		LocalToken:    tokenContractID,
		Administrator: deployerAddr,
	}); err != nil {
		return fmt.Errorf("failed to propose administrator in TokenAdminRegistry: %w", err)
	}
	if _, err := cldfops.ExecuteOperation(opBundle, tarops.AcceptAdminRole, deps, tarops.AcceptAdminRoleInput{
		ContractID: tarID,
		LocalToken: tokenContractID,
	}); err != nil {
		return fmt.Errorf("failed to accept admin role in TokenAdminRegistry: %w", err)
	}
	poolID := poolContractID
	if _, err := cldfops.ExecuteOperation(opBundle, tarops.SetPool, deps, tarops.SetPoolInput{
		ContractID: tarID,
		LocalToken: tokenContractID,
		Pool:       &poolID,
	}); err != nil {
		return fmt.Errorf("failed to register pool in TokenAdminRegistry: %w", err)
	}
	host.Logger().Info().
		Str("token", tokenContractID).
		Str("pool", poolContractID).
		Msg("Token pool registered in TokenAdminRegistry")
	return nil
}
