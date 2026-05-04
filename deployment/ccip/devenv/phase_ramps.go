package devenv

import (
	"fmt"
	"os"
	"path/filepath"

	offrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/offramp"
	rampregistrybindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/ramp_registry"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	offrampops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/offramp"
	onrampops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/onramp"
	rrops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/ramp_registry"
	routerops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/router"
)

func (w *work) deployRampsAndProvisionalLanes() error {
	h := w.host
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
	offRampOut, err := execStellarOp(w, offrampops.Deploy, stellarops.DeployInput{WasmPath: offRampWasmPath, Salt: offRampSalt})
	if err != nil {
		return fmt.Errorf("failed to deploy OffRamp contract: %w", err)
	}
	offRampContractID := offRampOut.ContractID
	w.offRampContractID = offRampContractID
	h.Logger().Info().Str("contractID", offRampContractID).Msg("OffRamp contract deployed")

	offRampClient := offrampbindings.NewOffRampClient(h.Deployer(), offRampContractID)
	h.SetOffRamp(offRampContractID, offRampClient)

	if _, err := execStellarOp(w, offrampops.Initialize, offrampops.InitializeInput{
		ContractID: offRampContractID,
		Owner:      h.DeployerKeypair().Address(),
		Config: offrampbindings.StaticConfig{
			ChainSelector:      w.selector,
			RmnProxy:           rmnProxyContractID,
			TokenAdminRegistry: tarContractID,
		},
	}); err != nil {
		return fmt.Errorf("failed to initialize OffRamp: %w", err)
	}
	h.Logger().Info().Str("offRampContractID", offRampContractID).Msg("OffRamp initialized")

	routerWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "router.wasm")
	if _, statErr := os.Stat(routerWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("Router WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", routerWasmPath)
	}

	h.Logger().Info().Str("wasmPath", routerWasmPath).Msg("Deploying Router contract...")
	routerSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "router")
	routerOut, err := execStellarOp(w, routerops.Deploy, stellarops.DeployInput{WasmPath: routerWasmPath, Salt: routerSalt})
	if err != nil {
		return fmt.Errorf("failed to deploy Router contract: %w", err)
	}
	routerContractID := routerOut.ContractID
	w.routerContractID = routerContractID
	h.Logger().Info().Str("contractID", routerContractID).Msg("Router contract deployed")

	routerClient := routerbindings.NewRouterClient(h.Deployer(), routerContractID)
	if _, err := execStellarOp(w, routerops.Initialize, routerops.InitializeInput{
		ContractID: routerContractID,
		Owner:      h.DeployerKeypair().Address(),
		RmnProxy:   rmnProxyContractID,
	}); err != nil {
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
	if _, err := execStellarOp(w, onrampops.ApplyDestChainConfigUpdates, onrampops.ApplyDestChainConfigUpdatesInput{
		ContractID: onrampContractID,
		Updates:    onRampDestConfigs,
	}); err != nil {
		return fmt.Errorf("failed to apply dest chain config updates on OnRamp: %w", err)
	}
	h.Logger().Info().Int("count", len(onRampDestConfigs)).Msg("OnRamp dest chain configs applied")

	offRampSourceConfigs, err := h.BuildOffRampSourceConfigs(nil, remoteSelectors, false)
	if err != nil {
		return fmt.Errorf("build provisional offramp source configs: %w", err)
	}
	if _, err := execStellarOp(w, offrampops.ApplySourceChainCfgUpdates, offrampops.ApplySourceChainCfgUpdatesInput{
		ContractID: offRampContractID,
		Updates:    offRampSourceConfigs,
	}); err != nil {
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
	if _, err := execStellarOp(w, routerops.ApplyRampUpdates, routerops.ApplyRampUpdatesInput{
		ContractID:     routerContractID,
		OnRampUpdates:  onRampEntries,
		OffRampRemoves: []routerbindings.OffRampEntry{},
		OffRampAdds:    offRampEntries,
	}); err != nil {
		return fmt.Errorf("failed to apply ramp updates on Router: %w", err)
	}
	h.Logger().Info().
		Int("onRampEntries", len(onRampEntries)).
		Int("offRampEntries", len(offRampEntries)).
		Msg("Router ramp updates applied")

	rampRegistryWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "ccip_ramp_registry.wasm")
	if _, statErr := os.Stat(rampRegistryWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("RampRegistry WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", rampRegistryWasmPath)
	}
	h.Logger().Info().Str("wasmPath", rampRegistryWasmPath).Msg("Deploying RampRegistry contract...")
	rampRegistrySalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "ramp-registry")
	rrOut, err := execStellarOp(w, rrops.Deploy, stellarops.DeployInput{WasmPath: rampRegistryWasmPath, Salt: rampRegistrySalt})
	if err != nil {
		return fmt.Errorf("failed to deploy RampRegistry contract: %w", err)
	}
	rampRegistryContractID := rrOut.ContractID
	if _, err := execStellarOp(w, rrops.Initialize, rrops.InitializeInput{
		ContractID: rampRegistryContractID,
		Owner:      h.DeployerKeypair().Address(),
	}); err != nil {
		return fmt.Errorf("failed to initialize RampRegistry: %w", err)
	}
	rrOnRamp := make([]rampregistrybindings.OnRampUpdate, len(onRampEntries))
	for i, e := range onRampEntries {
		onramp := e.Onramp
		rrOnRamp[i] = rampregistrybindings.OnRampUpdate{
			DestChainSelector: e.DestChainSelector,
			Onramp:            &onramp,
		}
	}
	if _, err := execStellarOp(w, rrops.ApplyOnrampUpdates, rrops.ApplyOnrampUpdatesInput{
		ContractID: rampRegistryContractID,
		Updates:    rrOnRamp,
	}); err != nil {
		return fmt.Errorf("failed to apply onramp updates on RampRegistry: %w", err)
	}
	rrOffRamp := make([]rampregistrybindings.OffRampUpdate, len(offRampEntries))
	for i, e := range offRampEntries {
		rrOffRamp[i] = rampregistrybindings.OffRampUpdate{
			SourceChainSelector: e.SourceChainSelector,
			Offramp:             e.Offramp,
			Enabled:             true,
		}
	}
	if _, err := execStellarOp(w, rrops.ApplyOfframpUpdates, rrops.ApplyOfframpUpdatesInput{
		ContractID: rampRegistryContractID,
		Updates:    rrOffRamp,
	}); err != nil {
		return fmt.Errorf("failed to apply offramp updates on RampRegistry: %w", err)
	}
	h.SetRampRegistry(rampRegistryContractID)
	h.Logger().Info().Str("contractID", rampRegistryContractID).Msg("RampRegistry deployed and ramp maps synced with Router")

	return nil
}
