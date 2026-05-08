// Package ccvchain implements the Stellar chain for chainlink-ccv devenv (runtime wiring lives
// in other files in this package).
//
// Registration relationship with deployment/adapters/init.go:
//
//   - That package’s init registers Stellar with chainlink-ccip CCIP 2.0 tooling, shared
//     registries (MCMS, fees, tokens, curse), and chainlink-ccv/deployment/adapters.
//
//   - This file registers chainlink-ccv/build/devenv-only hooks via RegisterStellarDevenvComponents
//     (modifiers, ImplFactory, CLDF provider, extra-args serializers, chain config loader).
//
// This package blank-imports deployment/adapters so its init runs as soon as ccvchain loads.
// Loading ccvchain does not call RegisterStellarDevenvComponents; devenv mains and tests must
// invoke RegisterStellarDevenvComponents or RegisterStellarComponents for those hooks.
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

	modifier "github.com/smartcontractkit/chainlink-stellar/ccv/chain/modifier"

	// Triggers deployment/adapters init; see package doc.
	_ "github.com/smartcontractkit/chainlink-stellar/deployment/adapters"
)

var registerOnce sync.Once

// RegisterStellarDevenvComponents registers Stellar with chainlink-ccv **build/devenv**
// (chain config loader, modifiers, ImplFactory, CLDF provider, extra-args serializers).
// It does not replace deployment/adapters init; see package doc.
func RegisterStellarDevenvComponents() {
	registerOnce.Do(func() {
		chainconfig.RegisterChainConfigLoader(chainsel.FamilyStellar, StellarChainConfigLoader)
		committeeverifier.RegisterModifier(chainsel.FamilyStellar, modifier.StellarVerifierModifier)
		executor.RegisterModifier(chainsel.FamilyStellar, modifier.StellarExecutorModifier)
		ccv.RegisterImplFactory(chainsel.FamilyStellar, NewImplFactory())
		registry.RegisterCLDFProviderFactory(chainsel.FamilyStellar, NewCLDFProviderFactory())

		// Register every CCIP message version we expect EVM-as-source to send to
		// a Stellar destination. The EVM OnRamp serialises EVM ABI extra args
		// regardless of destination family; chainlink-ccv's evm package now
		// keys serializers by (family, version) so we mirror its FamilyEVM
		// registrations for FamilyStellar destinations.
		cciptestinterfaces.RegisterExtraArgsSerializer(cciptestinterfaces.ExtraArgsSerializerEntry{Family: chainsel.FamilyStellar, Version: 3}, devenvccipevm.SerializeMessageV3ExtraArgs)
		cciptestinterfaces.RegisterExtraArgsSerializer(cciptestinterfaces.ExtraArgsSerializerEntry{Family: chainsel.FamilyStellar, Version: 2}, devenvccipevm.BuildEVMExtraArgsV2)
		cciptestinterfaces.RegisterExtraArgsSerializer(cciptestinterfaces.ExtraArgsSerializerEntry{Family: chainsel.FamilyStellar, Version: 1}, devenvccipevm.BuildEVMExtraArgsV1)
	})
}

// RegisterStellarComponents is an alias for [RegisterStellarDevenvComponents].
func RegisterStellarComponents() {
	RegisterStellarDevenvComponents()
}
