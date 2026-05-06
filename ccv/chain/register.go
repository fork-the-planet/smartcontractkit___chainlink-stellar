package ccvchain

import (
	"sync"

	"github.com/Masterminds/semver/v3"
	chainsel "github.com/smartcontractkit/chain-selectors"
	tokenscore "github.com/smartcontractkit/chainlink-ccip/deployment/tokens"
	ccipadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	devenvccipevm "github.com/smartcontractkit/chainlink-ccv/build/devenv/evm"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/registry"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/chainconfig"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/committeeverifier"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/executor"

	ccvdeploymentadapters "github.com/smartcontractkit/chainlink-ccv/deployment/adapters"

	modifier "github.com/smartcontractkit/chainlink-stellar/ccv/chain/modifier"
	"github.com/smartcontractkit/chainlink-stellar/deployment/adapters"
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

		ccipadapters.GetCommitteeVerifierContractRegistry().Register(chainsel.FamilyStellar, &adapters.StellarCommitteeVerifierContractAdapter{})
		stellarChainFamily := &adapters.StellarChainFamilyAdapter{}

		ccipadapters.GetChainFamilyRegistry().RegisterChainFamily(chainsel.FamilyStellar, stellarChainFamily)
		ccipadapters.GetAggregatorConfigRegistry().Register(chainsel.FamilyStellar, &adapters.StellarAggregatorConfigAdapter{})
		ccipadapters.GetIndexerConfigRegistry().Register(chainsel.FamilyStellar, &adapters.StellarIndexerConfigAdapter{})
		ccipadapters.GetVerifierJobConfigRegistry().Register(chainsel.FamilyStellar, &adapters.StellarVerifierConfigAdapter{})
		ccipadapters.GetExecutorConfigRegistry().Register(chainsel.FamilyStellar, &adapters.StellarExecutorConfigAdapter{})
		ccipadapters.GetTokenVerifierConfigRegistry().Register(chainsel.FamilyStellar, &adapters.StellarTokenVerifierConfigAdapter{})
		ccipadapters.GetDeployChainContractsRegistry().Register(chainsel.FamilyStellar, &adapters.StellarDeployChainContractsAdapter{})

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
			Aggregator:               &adapters.StellarCCVDeploymentAggregatorConfigAdapter{},
			CommitteeVerifierOnchain: &adapters.StellarCCVCommitteeVerifierOnchainAdapter{},
			Executor:                 &adapters.StellarCCVDeploymentExecutorConfigAdapter{},
			Verifier:                 &adapters.StellarCCVDeploymentVerifierConfigAdapter{},
			Indexer:                  &adapters.StellarCCVDeploymentIndexerConfigAdapter{},
			TokenVerifier:            &adapters.StellarCCVDeploymentTokenVerifierConfigAdapter{},
		})

		tokenAdapterRegistry := tokenscore.GetTokenAdapterRegistry()
		tokenAdapterRegistry.RegisterTokenAdapter(chainsel.FamilyStellar, semver.MustParse("1.0.0"), &adapters.StellarTokenAdapter{})
	})
}

// RegisterStellarComponents is an alias for RegisterStellarDevenvComponents.
func RegisterStellarComponents() {
	RegisterStellarDevenvComponents()
}
