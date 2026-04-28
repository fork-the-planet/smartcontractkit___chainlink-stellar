package devenv

import (
	"sync"

	"github.com/Masterminds/semver/v3"
	chainsel "github.com/smartcontractkit/chain-selectors"
	tokenscore "github.com/smartcontractkit/chainlink-ccip/deployment/tokens"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	devenvccipevm "github.com/smartcontractkit/chainlink-ccv/build/devenv/evm"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/registry"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/chainconfig"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/committeeverifier"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/executor"

	ccvdeploymentadapters "github.com/smartcontractkit/chainlink-ccv/deployment/adapters"

	ccvchain "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
	"github.com/smartcontractkit/chainlink-stellar/ccv/devenv/adapter"
	"github.com/smartcontractkit/chainlink-stellar/ccv/devenv/modifier"
)

var registerOnce sync.Once

// RegisterStellarComponents registers all Stellar-specific devenv components with
// the global registries. Safe to call multiple times (idempotent via sync.Once).
//
// This registers:
//   - CommitteeVerifierModifier: customises the verifier Docker container for Stellar.
//   - ExecutorModifier:          customises the executor Docker container for Stellar.
//   - ChainConfigLoader:         provides placeholder blockchain info for Stellar chains.
//   - ImplFactory:               factory for creating Stellar CCIP17 chain implementations.
//   - CLDFProviderFactory:       factory for creating Stellar CLDF BlockChain providers.
//
// Stellar does not register deployment/lanes.LaneAdapter (legacy 1.6 ConnectChains path);
// use ConfigureChainsForLanesFromTopology with ChainFamilyRegistry only.
//
// cciptestinterfaces.RegisterExtraArgsSerializer(FamilyStellar, devenvccipevm.SerializeEVMExtraArgs)
// is only for EVM-as-source: CCIP17EVM.BuildChainMessage looks up the serializer by destination
// family; the EVM OnRamp still emits EVM ABI GenericExtraArgs for a Stellar destination selector.
// That is unrelated to Stellar-as-source Soroban/XDR extra_args, which ccvchain.Chain builds via
// EncodeStellarSourceExtraArgsForOnRamp and never uses this registry entry.
func RegisterStellarComponents() {
	registerOnce.Do(func() {
		chainconfig.RegisterChainConfigLoader(chainsel.FamilyStellar, StellarChainConfigLoader)
		committeeverifier.RegisterModifier(chainsel.FamilyStellar, modifier.StellarVerifierModifier)
		executor.RegisterModifier(chainsel.FamilyStellar, modifier.StellarExecutorModifier)
		ccv.RegisterImplFactory(chainsel.FamilyStellar, ccvchain.NewImplFactory())
		registry.RegisterCLDFProviderFactory(chainsel.FamilyStellar, ccvchain.NewCLDFProviderFactory())

		ccvadapters.GetCommitteeVerifierContractRegistry().Register(chainsel.FamilyStellar, &adapter.StellarCommitteeVerifierContractAdapter{})
		stellarChainFamily := &adapter.StellarChainFamilyAdapter{}

		ccvadapters.GetChainFamilyRegistry().RegisterChainFamily(chainsel.FamilyStellar, stellarChainFamily)
		ccvadapters.GetAggregatorConfigRegistry().Register(chainsel.FamilyStellar, &adapter.StellarAggregatorConfigAdapter{})
		ccvadapters.GetIndexerConfigRegistry().Register(chainsel.FamilyStellar, &adapter.StellarIndexerConfigAdapter{})
		ccvadapters.GetVerifierJobConfigRegistry().Register(chainsel.FamilyStellar, &adapter.StellarVerifierConfigAdapter{})
		ccvadapters.GetExecutorConfigRegistry().Register(chainsel.FamilyStellar, &adapter.StellarExecutorConfigAdapter{})
		ccvadapters.GetTokenVerifierConfigRegistry().Register(chainsel.FamilyStellar, &adapter.StellarTokenVerifierConfigAdapter{})
		ccvadapters.GetDeployChainContractsRegistry().Register(chainsel.FamilyStellar, &ccvchain.StellarDeployChainContractsAdapter{})

		cciptestinterfaces.RegisterExtraArgsSerializer(chainsel.FamilyStellar, devenvccipevm.SerializeEVMExtraArgs)

		// Register Stellar with chainlink-ccv/deployment/adapters so devenv changesets
		// (GenerateAggregatorConfig, ApplyExecutorConfig, etc.) can resolve Stellar chains.
		ccvdeploymentadapters.GetRegistry().Register(chainsel.FamilyStellar, ccvdeploymentadapters.ChainAdapters{
			Aggregator:    &adapter.StellarCCVDeploymentAggregatorConfigAdapter{},
			Executor:      &adapter.StellarCCVDeploymentExecutorConfigAdapter{},
			Verifier:      &adapter.StellarCCVDeploymentVerifierConfigAdapter{},
			Indexer:       &adapter.StellarCCVDeploymentIndexerConfigAdapter{},
			TokenVerifier: &adapter.StellarCCVDeploymentTokenVerifierConfigAdapter{},
		})
    
		tokenAdapterRegistry := tokenscore.GetTokenAdapterRegistry()
		tokenAdapterRegistry.RegisterTokenAdapter(chainsel.FamilyStellar, semver.MustParse("1.0.0"), &adapter.StellarTokenAdapter{})
	})
}
