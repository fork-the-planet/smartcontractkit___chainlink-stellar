package stellardeploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	tokenpoolbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/token_pool"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
	tarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/token_admin_registry"
	poolops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/token_pool"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// See [github.com/smartcontractkit/chainlink-stellar/deployment/ccip.DevenvTestTokenPoolQualifier].

// initialPoolLiquidity is the amount of SAC base units seeded into the
// lock-release pool so that inbound release_or_mint calls have liquidity.
// 100 tokens at 7 decimals = 1_000_000_000 base units.
const initialPoolLiquidity int64 = 1_000_000_000

// DeployLockReleaseTestTokenPool deploys the lock-release pool and, when Friendbot is
// available, creates the test SAC token, initializes the pool, and registers token+pool
// in TokenAdminRegistry. Intended to run from PostDeployContractsForSelector after core CCIP deploy.
// opBundle must be the parent CLDF operations bundle for the deploy run (e.g. env.OperationsBundle).
func DeployLockReleaseTestTokenPool(ctx context.Context, opBundle cldfops.Bundle, host Host) error {
	h := host
	stellarRoot, err := stellarutil.FindStellarRoot()
	if err != nil {
		return fmt.Errorf("find stellar root: %w", err)
	}

	poolWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "pools_lock_release_pool.wasm")
	if _, statErr := os.Stat(poolWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("LockReleasePool WASM not found at %s. Run 'make build'.", poolWasmPath)
	}
	h.Logger().Info().Str("wasmPath", poolWasmPath).Msg("Deploying LockRelease pool contract (post-deploy)...")
	poolSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "lock-release-pool")
	deps := stellardeps.FromDeployer(h.Deployer())
	poolRep, err := cldfops.ExecuteOperation(opBundle, poolops.Deploy, deps, stellarops.DeployInput{WasmPath: poolWasmPath, Salt: poolSalt})
	if err != nil {
		return fmt.Errorf("failed to deploy LockRelease pool: %w", err)
	}
	poolContractID := poolRep.Output.ContractID
	poolClient := tokenpoolbindings.NewTokenPoolClient(h.Deployer(), poolContractID)
	h.SetTokenPool(poolContractID, poolClient)
	h.Logger().Info().Str("contractID", poolContractID).Msg("LockRelease pool deployed")

	tarClient := h.TokenAdminRegistryClient()
	if tarClient == nil {
		return fmt.Errorf("token admin registry client is nil before pool token setup")
	}

	if h.FriendbotURL() == "" {
		h.Logger().Warn().Msg("Friendbot URL not available; skipping test token deployment. Token transfer E2E tests will not work.")
		return nil
	}

	tokenContractID, tokenErr := h.CreateTestToken(ctx, h.FriendbotURL())
	if tokenErr != nil {
		return fmt.Errorf("failed to create test token: %w", tokenErr)
	}
	h.SetTestToken(tokenContractID)

	// Match typical Stellar SAC / pool configuration (EVM `uint8` token decimals on the pool).
	const testTokenPoolDecimals uint32 = 7
	routerContractID := h.RouterContractID()
	if routerContractID == "" {
		return fmt.Errorf("router contract ID is empty; token pool initialize requires router (deploy core CCIP first)")
	}
	rampRegistryContractID := h.RampRegistryContractID()
	if rampRegistryContractID == "" {
		return fmt.Errorf("ramp registry contract ID is empty; token pool initialize requires ramp registry (deploy core CCIP first)")
	}
	if _, err := cldfops.ExecuteOperation(opBundle, poolops.Initialize, deps, poolops.InitializeInput{
		ContractID:    poolContractID,
		Owner:         h.DeployerKeypair().Address(),
		Token:         tokenContractID,
		TokenDecimals: testTokenPoolDecimals,
		Router:        routerContractID,
		RampRegistry:  rampRegistryContractID,
	}); err != nil {
		return fmt.Errorf("failed to initialize pool with token: %w", err)
	}

	// Fund the pool with SAC liquidity so inbound release_or_mint calls succeed.
	deployerAddr := h.DeployerKeypair().Address()
	if err := fundPoolWithSAC(ctx, h.Deployer(), tokenContractID, deployerAddr, poolContractID, initialPoolLiquidity); err != nil {
		return fmt.Errorf("fund pool with SAC liquidity: %w", err)
	}
	h.Logger().Info().
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
	h.Logger().Info().
		Str("token", tokenContractID).
		Str("pool", poolContractID).
		Msg("Token pool registered in TokenAdminRegistry")
	return nil
}

// fundPoolWithSAC transfers SAC tokens from the deployer to the pool contract,
// providing liquidity for inbound release_or_mint operations.
func fundPoolWithSAC(ctx context.Context, deployer *stellardeployment.Deployer, sacContractID, fromStrkey, poolStrkey string, amount int64) error {
	args := []xdr.ScVal{
		scval.AddressToScVal(fromStrkey),
		scval.AddressToScVal(poolStrkey),
		scval.I128ToScVal(amount),
	}
	_, err := deployer.InvokeContract(ctx, sacContractID, "transfer", args)
	if err != nil {
		return fmt.Errorf("SAC transfer %s -> %s amount=%d: %w", fromStrkey, poolStrkey, amount, err)
	}
	return nil
}

// LockReleasePoolAddressRefDataStore forwards to [github.com/smartcontractkit/chainlink-stellar/deployment/ccip.LockReleasePoolAddressRefDataStore].
func LockReleasePoolAddressRefDataStore(chainSelector uint64, poolContractID, tokenContractID string) (datastore.DataStore, error) {
	return stellarccip.LockReleasePoolAddressRefDataStore(chainSelector, poolContractID, tokenContractID)
}
