package ccvchain

import (
	"sync"

	"github.com/Masterminds/semver/v3"
	chainsel "github.com/smartcontractkit/chain-selectors"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	devenvccipevm "github.com/smartcontractkit/chainlink-ccv/build/devenv/evm"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/registry"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/chainconfig"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/committeeverifier"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/executor"
	modifier "github.com/smartcontractkit/chainlink-stellar/ccv/chain/modifier"
	ccvadapters "github.com/smartcontractkit/chainlink-stellar/deployment/adapters"
)

var registerOnce sync.Once

// RegisterStellarDevenvComponents registers Stellar-specific chainlink-ccv **devenv**
// infrastructure (modifiers, ImplFactory, CLDF providers, extra-args serializer).
// CCIP 2.0 deployment adapters are registered via blank-import of
// github.com/smartcontractkit/chainlink-stellar/deployment/adapters (see deployment/adapters/init.go).
func RegisterStellarDevenvComponents() {
	registerOnce.Do(func() {
		chainconfig.RegisterChainConfigLoader(chainsel.FamilyStellar, StellarChainConfigLoader)
		committeeverifier.RegisterModifier(chainsel.FamilyStellar, modifier.StellarVerifierModifier)
		executor.RegisterModifier(chainsel.FamilyStellar, modifier.StellarExecutorModifier)
		ccv.RegisterImplFactory(chainsel.FamilyStellar, NewImplFactory())
		registry.RegisterCLDFProviderFactory(chainsel.FamilyStellar, NewCLDFProviderFactory())

		ccvadapters.GetCommitteeVerifierContractRegistry().Register(chainsel.FamilyStellar, &adapter.StellarCommitteeVerifierContractAdapter{})
		stellarChainFamily := &adapter.StellarChainFamilyAdapter{}

		ccvadapters.GetChainFamilyRegistry().RegisterChainFamily(chainsel.FamilyStellar, stellarChainFamily)
		ccvadapters.GetAggregatorConfigRegistry().Register(chainsel.FamilyStellar, &adapter.StellarAggregatorConfigAdapter{})
		ccvadapters.GetIndexerConfigRegistry().Register(chainsel.FamilyStellar, &adapter.StellarIndexerConfigAdapter{})
		ccvadapters.GetVerifierJobConfigRegistry().Register(chainsel.FamilyStellar, &adapter.StellarVerifierConfigAdapter{})
		ccvadapters.GetExecutorConfigRegistry().Register(chainsel.FamilyStellar, &adapter.StellarExecutorConfigAdapter{})
		ccvadapters.GetTokenVerifierConfigRegistry().Register(chainsel.FamilyStellar, &adapter.StellarTokenVerifierConfigAdapter{})
		ccvadapters.GetDeployChainContractsRegistry().Register(chainsel.FamilyStellar, &StellarDeployChainContractsAdapter{})

		// Register every CCIP message version we expect EVM-as-source to send to
		// a Stellar destination. The EVM OnRamp serialises EVM ABI extra args
		// regardless of destination family; chainlink-ccv's evm package now
		// keys serializers by (family, version) so we mirror its FamilyEVM
		// registrations for FamilyStellar destinations.
		cciptestinterfaces.RegisterExtraArgsSerializer(cciptestinterfaces.ExtraArgsSerializerEntry{Family: chainsel.FamilyStellar, Version: 3}, devenvccipevm.SerializeMessageV3ExtraArgs)
		cciptestinterfaces.RegisterExtraArgsSerializer(cciptestinterfaces.ExtraArgsSerializerEntry{Family: chainsel.FamilyStellar, Version: 2}, devenvccipevm.BuildEVMExtraArgsV2)
		cciptestinterfaces.RegisterExtraArgsSerializer(cciptestinterfaces.ExtraArgsSerializerEntry{Family: chainsel.FamilyStellar, Version: 1}, devenvccipevm.BuildEVMExtraArgsV1)

		// Register Stellar with chainlink-ccv/deployment/adapters so devenv changesets
		// (GenerateAggregatorConfig, ApplyExecutorConfig, etc.) can resolve Stellar chains.
		ccvdeploymentadapters.GetRegistry().Register(chainsel.FamilyStellar, ccvdeploymentadapters.ChainAdapters{
			Aggregator:               &adapter.StellarCCVDeploymentAggregatorConfigAdapter{},
			CommitteeVerifierOnchain: &adapter.StellarCCVCommitteeVerifierOnchainAdapter{},
			Executor:                 &adapter.StellarCCVDeploymentExecutorConfigAdapter{},
			Verifier:                 &adapter.StellarCCVDeploymentVerifierConfigAdapter{},
			Indexer:                  &adapter.StellarCCVDeploymentIndexerConfigAdapter{},
			TokenVerifier:            &adapter.StellarCCVDeploymentTokenVerifierConfigAdapter{},
		})

		tokenAdapterRegistry := tokenscore.GetTokenAdapterRegistry()
		tokenAdapterRegistry.RegisterTokenAdapter(chainsel.FamilyStellar, semver.MustParse("1.0.0"), &adapter.StellarTokenAdapter{})
	})
}

// RegisterStellarComponents is an alias for RegisterStellarDevenvComponents.
func RegisterStellarComponents() {
	RegisterStellarDevenvComponents()
}
