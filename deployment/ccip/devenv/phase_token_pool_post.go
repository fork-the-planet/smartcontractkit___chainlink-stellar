package devenv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	tokenpoolbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/token_pool"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// DevenvTestTokenPoolQualifier is the datastore qualifier for the lock-release test pool.
const DevenvTestTokenPoolQualifier = "TEST"

// initialPoolLiquidity is the amount of SAC base units seeded into the
// lock-release pool so that inbound release_or_mint calls have liquidity.
// 100 tokens at 7 decimals = 1_000_000_000 base units.
const initialPoolLiquidity int64 = 1_000_000_000

// DeployLockReleaseTestTokenPool deploys the lock-release pool and, when Friendbot is
// available, creates the test SAC token, initializes the pool, and registers token+pool
// in TokenAdminRegistry. Intended to run from PostDeployContractsForSelector after core CCIP deploy.
func DeployLockReleaseTestTokenPool(ctx context.Context, host Host) error {
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
	poolContractID, err := h.Deployer().DeployContract(ctx, poolWasmPath, poolSalt)
	if err != nil {
		return fmt.Errorf("failed to deploy LockRelease pool: %w", err)
	}
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
	if err := poolClient.Initialize(ctx, h.DeployerKeypair().Address(), tokenContractID, testTokenPoolDecimals, routerContractID, rampRegistryContractID); err != nil {
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

	if err := tarClient.ProposeAdministrator(ctx, deployerAddr, tokenContractID, deployerAddr); err != nil {
		return fmt.Errorf("failed to propose administrator in TokenAdminRegistry: %w", err)
	}
	if err := tarClient.AcceptAdminRole(ctx, tokenContractID); err != nil {
		return fmt.Errorf("failed to accept admin role in TokenAdminRegistry: %w", err)
	}
	if err := tarClient.SetPool(ctx, tokenContractID, &poolContractID); err != nil {
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

// LockReleasePoolAddressRefDataStore returns a sealed datastore containing the
// lock-release pool AddressRef and, when tokenContractID is non-empty, the
// test token AddressRef for devenv (qualifier DevenvTestTokenPoolQualifier).
func LockReleasePoolAddressRefDataStore(chainSelector uint64, poolContractID, tokenContractID string) (datastore.DataStore, error) {
	ds := datastore.NewMemoryDataStore()
	poolHex, err := stellarutil.StrkeyToHex(poolContractID)
	if err != nil {
		return nil, fmt.Errorf("convert pool address: %w", err)
	}
	if err := ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       poolHex,
		ChainSelector: chainSelector,
		Type:          datastore.ContractType(LockReleaseTokenPoolContractType),
		Version:       semver.MustParse("1.0.0"),
		Qualifier:     DevenvTestTokenPoolQualifier,
	}); err != nil {
		return nil, fmt.Errorf("add pool address ref: %w", err)
	}
	if tokenContractID != "" {
		tokenHex, err := stellarutil.StrkeyToHex(tokenContractID)
		if err != nil {
			return nil, fmt.Errorf("convert token address: %w", err)
		}
		if err := ds.AddressRefStore.Add(datastore.AddressRef{
			Address:       tokenHex,
			ChainSelector: chainSelector,
			Type:          datastore.ContractType(TestTokenContractType),
			Version:       semver.MustParse("1.0.0"),
			Qualifier:     DevenvTestTokenPoolQualifier,
		}); err != nil {
			return nil, fmt.Errorf("add token address ref: %w", err)
		}
	}
	return ds.Seal(), nil
}
