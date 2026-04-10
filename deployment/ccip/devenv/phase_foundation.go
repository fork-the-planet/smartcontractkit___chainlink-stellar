package devenv

import (
	"fmt"
	"os"
	"path/filepath"

	fqbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/fee_quoter"
	onrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/onramp"
	rmnproxybindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_proxy"
	rmnremotebindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_remote"
	tarbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/token_admin_registry"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
)

func (w *work) deployFoundationContracts() error {
	h := w.host
	ctx := w.ctx
	stellarRoot := w.stellarRoot

	onrampWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "onramp.wasm")
	if _, statErr := os.Stat(onrampWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("OnRamp WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", onrampWasmPath)
	}

	h.Logger().Info().Str("wasmPath", onrampWasmPath).Msg("Deploying OnRamp contract...")

	onrampSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "onramp")
	onrampContractID, err := h.Deployer().DeployContract(ctx, onrampWasmPath, onrampSalt)
	if err != nil {
		return fmt.Errorf("failed to deploy OnRamp contract: %w", err)
	}
	w.onrampContractID = onrampContractID
	h.Logger().Info().Str("contractID", onrampContractID).Msg("OnRamp contract deployed")

	onRampClient := onrampbindings.NewOnRampClient(h.Deployer(), onrampContractID)
	h.SetOnRamp(onrampContractID, onRampClient)

	rmnRemoteWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "rmn_remote.wasm")
	if _, statErr := os.Stat(rmnRemoteWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("RMN Remote WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", rmnRemoteWasmPath)
	}

	h.Logger().Info().Str("wasmPath", rmnRemoteWasmPath).Msg("Deploying RMN Remote contract...")
	rmnRemoteSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "rmn-remote")
	rmnRemoteContractID, err := h.Deployer().DeployContract(ctx, rmnRemoteWasmPath, rmnRemoteSalt)
	if err != nil {
		return fmt.Errorf("failed to deploy RMN Remote contract: %w", err)
	}
	w.rmnRemoteContractID = rmnRemoteContractID
	h.Logger().Info().Str("contractID", rmnRemoteContractID).Msg("RMN Remote contract deployed")

	rmnRemoteClient := rmnremotebindings.NewRmnRemoteClient(h.Deployer(), rmnRemoteContractID)
	err = rmnRemoteClient.Initialize(ctx, h.DeployerKeypair().Address(), w.selector)
	if err != nil {
		return fmt.Errorf("failed to initialize RMN Remote: %w", err)
	}
	h.Logger().Info().Str("rmnRemoteContractID", rmnRemoteContractID).Msg("RMN Remote initialized")

	rmnProxyWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "rmn_proxy.wasm")
	if _, statErr := os.Stat(rmnProxyWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("RMN Proxy WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", rmnProxyWasmPath)
	}

	h.Logger().Info().Str("wasmPath", rmnProxyWasmPath).Msg("Deploying RMN Proxy contract...")
	rmnProxySalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "rmn-proxy")
	rmnProxyContractID, err := h.Deployer().DeployContract(ctx, rmnProxyWasmPath, rmnProxySalt)
	if err != nil {
		return fmt.Errorf("failed to deploy RMN Proxy contract: %w", err)
	}
	w.rmnProxyContractID = rmnProxyContractID
	h.Logger().Info().Str("contractID", rmnProxyContractID).Msg("RMN Proxy contract deployed")

	rmnProxyClient := rmnproxybindings.NewRmnProxyClient(h.Deployer(), rmnProxyContractID)
	err = rmnProxyClient.Initialize(ctx, h.DeployerKeypair().Address(), rmnRemoteContractID)
	if err != nil {
		return fmt.Errorf("failed to initialize RMN Proxy: %w", err)
	}
	h.Logger().Info().Str("rmnProxyContractID", rmnProxyContractID).Msg("RMN Proxy initialized")

	feeQuoterWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "fee_quoter.wasm")
	if _, statErr := os.Stat(feeQuoterWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("FeeQuoter WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", feeQuoterWasmPath)
	}

	h.Logger().Info().Str("wasmPath", feeQuoterWasmPath).Msg("Deploying FeeQuoter contract...")
	feeQuoterSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "fee-quoter")
	feeQuoterContractID, err := h.Deployer().DeployContract(ctx, feeQuoterWasmPath, feeQuoterSalt)
	if err != nil {
		return fmt.Errorf("failed to deploy FeeQuoter contract: %w", err)
	}
	w.feeQuoterContractID = feeQuoterContractID
	h.Logger().Info().Str("contractID", feeQuoterContractID).Msg("FeeQuoter contract deployed")

	w.mockLinkToken = stellarutil.MustGenerateMockContractID(h.DeployerKeypair().Address(), "link-token")
	feeQuoterClient := fqbindings.NewFeeQuoterClient(h.Deployer(), feeQuoterContractID)
	h.SetFeeQuoter(feeQuoterClient)
	err = feeQuoterClient.Initialize(ctx, h.DeployerKeypair().Address(), fqbindings.StaticConfig{
		LinkToken:         w.mockLinkToken,
		MaxFeeJuelsPerMsg: 1_000_000_000_000_000_000, // 1e18
	}, []string{h.DeployerKeypair().Address()})
	if err != nil {
		return fmt.Errorf("failed to initialize FeeQuoter: %w", err)
	}
	h.Logger().Info().Str("feeQuoterContractID", feeQuoterContractID).Msg("FeeQuoter initialized")

	tarWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "token_admin_registry.wasm")
	if _, statErr := os.Stat(tarWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("TokenAdminRegistry WASM not found at %s. Run 'make build'.", tarWasmPath)
	}
	h.Logger().Info().Str("wasmPath", tarWasmPath).Msg("Deploying TokenAdminRegistry contract...")
	tarSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "token-admin-registry")
	tarContractID, err := h.Deployer().DeployContract(ctx, tarWasmPath, tarSalt)
	if err != nil {
		return fmt.Errorf("failed to deploy TokenAdminRegistry: %w", err)
	}
	w.tarContractID = tarContractID
	tarClient := tarbindings.NewTokenAdminRegistryClient(h.Deployer(), tarContractID)
	h.SetTokenAdminRegistry(tarContractID, tarClient)
	if err := tarClient.Initialize(ctx, h.DeployerKeypair().Address()); err != nil {
		return fmt.Errorf("failed to initialize TokenAdminRegistry: %w", err)
	}
	h.Logger().Info().Str("contractID", tarContractID).Msg("TokenAdminRegistry deployed and initialized")

	mockFeeAggregator := stellarutil.MustGenerateMockContractID(h.DeployerKeypair().Address(), "fee-aggregator")

	err = onRampClient.Initialize(ctx, h.DeployerKeypair().Address(), onrampbindings.StaticConfig{
		ChainSelector:         w.selector,
		TokenAdminRegistry:    tarContractID,
		RmnProxy:              rmnProxyContractID,
		MaxUsdCentsPerMessage: 10000, // $100
	}, onrampbindings.DynamicConfig{
		FeeQuoter:     feeQuoterContractID,
		FeeAggregator: mockFeeAggregator,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize OnRamp: %w", err)
	}

	h.Logger().Info().
		Str("onRampContractID", onrampContractID).
		Msg("OnRamp client initialized")

	return nil
}
