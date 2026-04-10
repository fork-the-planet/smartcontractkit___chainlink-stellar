package devenv

import (
	"fmt"
	"os"
	"path/filepath"

	offrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/offramp"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
)

func (w *work) deployRampsAndProvisionalLanes() error {
	h := w.host
	ctx := w.ctx
	stellarRoot := w.stellarRoot
	remoteSelectors := w.remoteSelectors

	onrampContractID := w.onrampContractID
	rmnProxyContractID := w.rmnProxyContractID
	tarContractID := w.tarContractID

	offRampWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "offramp.wasm")
	if _, statErr := os.Stat(offRampWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("OffRamp WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", offRampWasmPath)
	}

	h.Logger().Info().Str("wasmPath", offRampWasmPath).Msg("Deploying OffRamp contract...")
	offRampSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "offramp")
	offRampContractID, err := h.Deployer().DeployContract(ctx, offRampWasmPath, offRampSalt)
	if err != nil {
		return fmt.Errorf("failed to deploy OffRamp contract: %w", err)
	}
	w.offRampContractID = offRampContractID
	h.Logger().Info().Str("contractID", offRampContractID).Msg("OffRamp contract deployed")

	offRampClient := offrampbindings.NewOffRampClient(h.Deployer(), offRampContractID)
	h.SetOffRamp(offRampContractID, offRampClient)

	err = offRampClient.Initialize(ctx, h.DeployerKeypair().Address(), offrampbindings.StaticConfig{
		ChainSelector:      w.selector,
		RmnProxy:           rmnProxyContractID,
		TokenAdminRegistry: tarContractID,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize OffRamp: %w", err)
	}
	h.Logger().Info().Str("offRampContractID", offRampContractID).Msg("OffRamp initialized")

	routerWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "router.wasm")
	if _, statErr := os.Stat(routerWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("Router WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", routerWasmPath)
	}

	h.Logger().Info().Str("wasmPath", routerWasmPath).Msg("Deploying Router contract...")
	routerSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "router")
	routerContractID, err := h.Deployer().DeployContract(ctx, routerWasmPath, routerSalt)
	if err != nil {
		return fmt.Errorf("failed to deploy Router contract: %w", err)
	}
	w.routerContractID = routerContractID
	h.Logger().Info().Str("contractID", routerContractID).Msg("Router contract deployed")

	routerClient := routerbindings.NewRouterClient(h.Deployer(), routerContractID)
	err = routerClient.Initialize(ctx, h.DeployerKeypair().Address(), rmnProxyContractID)
	if err != nil {
		return fmt.Errorf("failed to initialize Router: %w", err)
	}
	h.SetRouter(routerContractID, routerClient)
	w.routerClient = routerClient
	h.Logger().Info().Str("routerContractID", routerContractID).Msg("Router initialized")

	executorProxyHex := w.contractHexAddr("stellar-executor-proxy")
	executorContractID, err := scval.HexToContractStrkey(executorProxyHex)
	if err != nil {
		return fmt.Errorf("failed to convert executor proxy placeholder address: %w", err)
	}
	onRampDestConfigs, err := h.BuildOnRampDestConfigs(nil, remoteSelectors, executorContractID, false)
	if err != nil {
		return fmt.Errorf("build provisional onramp dest configs: %w", err)
	}
	if err := h.OnRampClient().ApplyDestChainConfigUpdates(ctx, onRampDestConfigs); err != nil {
		return fmt.Errorf("failed to apply dest chain config updates on OnRamp: %w", err)
	}
	h.Logger().Info().Int("count", len(onRampDestConfigs)).Msg("OnRamp dest chain configs applied")

	offRampSourceConfigs, err := h.BuildOffRampSourceConfigs(nil, remoteSelectors, false)
	if err != nil {
		return fmt.Errorf("build provisional offramp source configs: %w", err)
	}
	if err := h.OffRampClient().ApplySourceChainCfgUpdates(ctx, offRampSourceConfigs); err != nil {
		return fmt.Errorf("failed to apply source chain config updates on OffRamp: %w", err)
	}
	h.Logger().Info().Int("count", len(offRampSourceConfigs)).Msg("OffRamp source chain configs applied")

	onRampEntries := make([]routerbindings.OnRampEntry, 0, len(remoteSelectors))
	offRampEntries := make([]routerbindings.OffRampEntry, 0, len(remoteSelectors))
	for _, rs := range remoteSelectors {
		onRampEntries = append(onRampEntries, routerbindings.OnRampEntry{
			DestChainSelector: rs,
			Onramp:            onrampContractID,
		})
		offRampEntries = append(offRampEntries, routerbindings.OffRampEntry{
			SourceChainSelector: rs,
			Offramp:             offRampContractID,
		})
	}
	err = routerClient.ApplyRampUpdates(ctx, onRampEntries, []routerbindings.OffRampEntry{}, offRampEntries)
	if err != nil {
		return fmt.Errorf("failed to apply ramp updates on Router: %w", err)
	}
	h.Logger().Info().
		Int("onRampEntries", len(onRampEntries)).
		Int("offRampEntries", len(offRampEntries)).
		Msg("Router ramp updates applied")

	return nil
}
