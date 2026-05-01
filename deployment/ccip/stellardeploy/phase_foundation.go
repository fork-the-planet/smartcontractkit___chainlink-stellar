package stellardeploy

import (
	"fmt"
	"os"
	"path/filepath"

	fqbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/fee_quoter"
	onrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/onramp"
	tarbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/token_admin_registry"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	fqops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/fee_quoter"
	onrampops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/onramp"
	rmnproxyops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/rmn_proxy"
	rmnremoteops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/rmn_remote"
	tarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/token_admin_registry"
)

func (w *deployRun) deployFoundationContracts() error {
	h := w.host
	ctx := w.ctx
	stellarRoot := w.stellarRoot

	onrampWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "onramp.wasm")
	if _, statErr := os.Stat(onrampWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("OnRamp WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", onrampWasmPath)
	}

	h.Logger().Info().Str("wasmPath", onrampWasmPath).Msg("Deploying OnRamp contract...")

	onrampSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "onramp")
	onrampOut, err := execStellarOp(w, onrampops.Deploy, stellarops.DeployInput{WasmPath: onrampWasmPath, Salt: onrampSalt})
	if err != nil {
		return fmt.Errorf("failed to deploy OnRamp contract: %w", err)
	}
	onrampContractID := onrampOut.ContractID
	w.onrampContractID = onrampContractID
	if err := stellarccip.RecordOnRamp(w.ds, w.selector, onrampContractID); err != nil {
		return fmt.Errorf("record OnRamp in datastore: %w", err)
	}
	h.Logger().Info().Str("contractID", onrampContractID).Msg("OnRamp contract deployed")

	onRampClient := onrampbindings.NewOnRampClient(h.Deployer(), onrampContractID)
	h.SetOnRamp(onrampContractID, onRampClient)

	rmnRemoteWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "rmn_remote.wasm")
	if _, statErr := os.Stat(rmnRemoteWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("RMN Remote WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", rmnRemoteWasmPath)
	}

	h.Logger().Info().Str("wasmPath", rmnRemoteWasmPath).Msg("Deploying RMN Remote contract...")
	rmnRemoteSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "rmn-remote")
	rmnRemoteOut, err := execStellarOp(w, rmnremoteops.Deploy, stellarops.DeployInput{WasmPath: rmnRemoteWasmPath, Salt: rmnRemoteSalt})
	if err != nil {
		return fmt.Errorf("failed to deploy RMN Remote contract: %w", err)
	}
	rmnRemoteContractID := rmnRemoteOut.ContractID
	w.rmnRemoteContractID = rmnRemoteContractID
	if err := stellarccip.RecordRMNRemote(w.ds, w.selector, rmnRemoteContractID); err != nil {
		return fmt.Errorf("record RMN Remote in datastore: %w", err)
	}
	h.Logger().Info().Str("contractID", rmnRemoteContractID).Msg("RMN Remote contract deployed")

	if _, err := execStellarOp(w, rmnremoteops.Initialize, rmnremoteops.InitializeInput{
		ContractID:    rmnRemoteContractID,
		Owner:         h.DeployerKeypair().Address(),
		ChainSelector: w.selector,
	}); err != nil {
		return fmt.Errorf("failed to initialize RMN Remote: %w", err)
	}
	h.Logger().Info().Str("rmnRemoteContractID", rmnRemoteContractID).Msg("RMN Remote initialized")

	rmnProxyWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "rmn_proxy.wasm")
	if _, statErr := os.Stat(rmnProxyWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("RMN Proxy WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", rmnProxyWasmPath)
	}

	h.Logger().Info().Str("wasmPath", rmnProxyWasmPath).Msg("Deploying RMN Proxy contract...")
	rmnProxySalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "rmn-proxy")
	rmnProxyOut, err := execStellarOp(w, rmnproxyops.Deploy, stellarops.DeployInput{WasmPath: rmnProxyWasmPath, Salt: rmnProxySalt})
	if err != nil {
		return fmt.Errorf("failed to deploy RMN Proxy contract: %w", err)
	}
	rmnProxyContractID := rmnProxyOut.ContractID
	w.rmnProxyContractID = rmnProxyContractID
	h.Logger().Info().Str("contractID", rmnProxyContractID).Msg("RMN Proxy contract deployed")

	if _, err := execStellarOp(w, rmnproxyops.Initialize, rmnproxyops.InitializeInput{
		ContractID: rmnProxyContractID,
		Owner:      h.DeployerKeypair().Address(),
		RmnRemote:  rmnRemoteContractID,
	}); err != nil {
		return fmt.Errorf("failed to initialize RMN Proxy: %w", err)
	}
	h.Logger().Info().Str("rmnProxyContractID", rmnProxyContractID).Msg("RMN Proxy initialized")

	feeQuoterWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "fee_quoter.wasm")
	if _, statErr := os.Stat(feeQuoterWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("FeeQuoter WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", feeQuoterWasmPath)
	}

	h.Logger().Info().Str("wasmPath", feeQuoterWasmPath).Msg("Deploying FeeQuoter contract...")
	feeQuoterSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "fee-quoter")
	feeQuoterOut, err := execStellarOp(w, fqops.Deploy, stellarops.DeployInput{WasmPath: feeQuoterWasmPath, Salt: feeQuoterSalt})
	if err != nil {
		return fmt.Errorf("failed to deploy FeeQuoter contract: %w", err)
	}
	feeQuoterContractID := feeQuoterOut.ContractID
	w.feeQuoterContractID = feeQuoterContractID
	if err := stellarccip.RecordFeeQuoter(w.ds, w.selector, feeQuoterContractID); err != nil {
		return fmt.Errorf("record FeeQuoter in datastore: %w", err)
	}
	h.Logger().Info().Str("contractID", feeQuoterContractID).Msg("FeeQuoter contract deployed")

	if h.FriendbotURL() != "" {
		feeTokenID, feeTokenErr := h.CreateFeeToken(ctx, h.FriendbotURL())
		if feeTokenErr != nil {
			return fmt.Errorf("failed to create fee token: %w", feeTokenErr)
		}
		h.SetFeeToken(feeTokenID)
		w.feeTokenContractID = feeTokenID
		h.Logger().Info().Str("contractID", feeTokenID).Msg("Fee token SAC deployed for CCIP fee payments")
	} else {
		h.Logger().Warn().Msg("Friendbot URL not available; using mock fee token ID (fee transfers will not work)")
		w.feeTokenContractID = stellarutil.MustGenerateMockContractID(h.DeployerKeypair().Address(), "fee-token")
	}

	feeQuoterClient := fqbindings.NewFeeQuoterClient(h.Deployer(), feeQuoterContractID)
	h.SetFeeQuoter(feeQuoterClient)
	if _, err := execStellarOp(w, fqops.Initialize, fqops.InitializeInput{
		ContractID: feeQuoterContractID,
		Owner:      h.DeployerKeypair().Address(),
		StaticConfig: fqbindings.StaticConfig{
			LinkToken:         w.feeTokenContractID,
			MaxFeeJuelsPerMsg: 1_000_000_000_000_000_000, // 1e18
		},
		AuthorizedCallers: []string{h.DeployerKeypair().Address()},
	}); err != nil {
		return fmt.Errorf("failed to initialize FeeQuoter: %w", err)
	}
	h.Logger().Info().Str("feeQuoterContractID", feeQuoterContractID).Msg("FeeQuoter initialized")

	tarWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "token_admin_registry.wasm")
	if _, statErr := os.Stat(tarWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("TokenAdminRegistry WASM not found at %s. Run 'make build'.", tarWasmPath)
	}
	h.Logger().Info().Str("wasmPath", tarWasmPath).Msg("Deploying TokenAdminRegistry contract...")
	tarSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "token-admin-registry")
	tarOut, err := execStellarOp(w, tarops.Deploy, stellarops.DeployInput{WasmPath: tarWasmPath, Salt: tarSalt})
	if err != nil {
		return fmt.Errorf("failed to deploy TokenAdminRegistry: %w", err)
	}
	tarContractID := tarOut.ContractID
	w.tarContractID = tarContractID
	if err := stellarccip.RecordTokenAdminRegistry(w.ds, w.selector, tarContractID); err != nil {
		return fmt.Errorf("record TokenAdminRegistry in datastore: %w", err)
	}
	tarClient := tarbindings.NewTokenAdminRegistryClient(h.Deployer(), tarContractID)
	h.SetTokenAdminRegistry(tarContractID, tarClient)
	if _, err := execStellarOp(w, tarops.Initialize, tarops.InitializeInput{
		ContractID: tarContractID,
		Owner:      h.DeployerKeypair().Address(),
	}); err != nil {
		return fmt.Errorf("failed to initialize TokenAdminRegistry: %w", err)
	}
	h.Logger().Info().Str("contractID", tarContractID).Msg("TokenAdminRegistry deployed and initialized")

	mockFeeAggregator := stellarutil.MustGenerateMockContractID(h.DeployerKeypair().Address(), "fee-aggregator")

	if _, err := execStellarOp(w, onrampops.Initialize, onrampops.InitializeInput{
		ContractID: onrampContractID,
		Owner:      h.DeployerKeypair().Address(),
		StaticConfig: onrampbindings.StaticConfig{
			ChainSelector:         w.selector,
			TokenAdminRegistry:    tarContractID,
			RmnProxy:              rmnProxyContractID,
			MaxUsdCentsPerMessage: 10000, // $100
		},
		DynamicConfig: onrampbindings.DynamicConfig{
			FeeQuoter:     feeQuoterContractID,
			FeeAggregator: mockFeeAggregator,
		},
	}); err != nil {
		return fmt.Errorf("failed to initialize OnRamp: %w", err)
	}

	h.Logger().Info().
		Str("onRampContractID", onrampContractID).
		Msg("OnRamp client initialized")

	return nil
}
