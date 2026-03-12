package devenv

import (
	chainsel "github.com/smartcontractkit/chain-selectors"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/registry"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/chainconfig"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/committeeverifier"
	executorsvc "github.com/smartcontractkit/chainlink-ccv/build/devenv/services/executor"

	ccvchain "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
	"github.com/smartcontractkit/chainlink-stellar/ccv/devenv/modifier"
)

// RegisterStellarComponents registers all Stellar-specific devenv components with
// the global registries. Call this in init() of any entry point (CLI command or E2E test)
// that needs to operate on Stellar chains.
//
// This registers:
//   - CommitteeVerifierModifier: customises the verifier Docker container for Stellar.
//   - ExecutorModifier:          customises the executor Docker container for Stellar.
//   - ChainConfigLoader:         provides placeholder blockchain info for Stellar chains.
//   - ChainFamilyAdapter:        adapter wrapping the EVM adapter for Stellar chains.
//   - ImplFactory:               factory for creating Stellar CCIP17 chain implementations.
//   - CLDFProviderFactory:       factory for creating Stellar CLDF BlockChain providers.
func RegisterStellarComponents() {
	// The EVM adapter is registered by the ccv init() function. Retrieve it as the
	// base for the Stellar adapter (Stellar reuses EVM-compatible chain infrastructure
	// for offchain components while having its own onchain logic).
	evmAdapter, ok := registry.GetGlobalChainFamilyAdapterRegistry().GetChainFamily(chainsel.FamilyEVM)
	if !ok {
		// EVM adapter is always registered by the ccv init(); if it's missing we panic
		// early rather than produce confusing downstream errors.
		panic("EVM chain family adapter not registered; ensure chainlink-ccv/build/devenv/registry is initialised before calling RegisterStellarComponents")
	}

	chainconfig.RegisterChainConfigLoader(chainsel.FamilyStellar, StellarChainConfigLoader)
	committeeverifier.RegisterModifier(chainsel.FamilyStellar, modifier.StellarVerifierModifier)
	executorsvc.RegisterModifier(chainsel.FamilyStellar, modifier.StellarExecutorModifier)
	ccv.RegisterImplFactory(chainsel.FamilyStellar, ccvchain.NewImplFactory())
	registry.RegisterChainFamilyAdapter(chainsel.FamilyStellar, ccvchain.NewChainFamilyAdapter(evmAdapter))
	registry.RegisterCLDFProviderFactory(chainsel.FamilyStellar, ccvchain.NewCLDFProviderFactory())
}
