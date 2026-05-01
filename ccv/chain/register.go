package ccvchain

import (
	"sync"

	chainsel "github.com/smartcontractkit/chain-selectors"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	devenvccipevm "github.com/smartcontractkit/chainlink-ccv/build/devenv/evm"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/registry"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/chainconfig"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/committeeverifier"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/executor"

	// Registers CCIP 2.0 adapters with chainlink-ccip registries (see deployment/adapters/init.go).
	_ "github.com/smartcontractkit/chainlink-stellar/deployment/adapters"

	modifier "github.com/smartcontractkit/chainlink-stellar/ccv/chain/modifier"
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

		cciptestinterfaces.RegisterExtraArgsSerializer(chainsel.FamilyStellar, devenvccipevm.SerializeEVMExtraArgs)
	})
}

// RegisterStellarComponents is an alias for RegisterStellarDevenvComponents.
func RegisterStellarComponents() {
	RegisterStellarDevenvComponents()
}
