package devenv

import (
	"sync"

	"github.com/Masterminds/semver/v3"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccip/deployment/lanes"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v1_7_0/adapters"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/registry"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/chainconfig"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/committeeverifier"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/executor"

	ccvchain "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
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
func RegisterStellarComponents() {
	registerOnce.Do(func() {
		chainconfig.RegisterChainConfigLoader(chainsel.FamilyStellar, StellarChainConfigLoader)
		committeeverifier.RegisterModifier(chainsel.FamilyStellar, modifier.StellarVerifierModifier)
		executor.RegisterModifier(chainsel.FamilyStellar, modifier.StellarExecutorModifier)
		ccv.RegisterImplFactory(chainsel.FamilyStellar, ccvchain.NewImplFactory())
		registry.RegisterCLDFProviderFactory(chainsel.FamilyStellar, ccvchain.NewCLDFProviderFactory())
		ccvadapters.GetCommitteeVerifierContractRegistry().Register(chainsel.FamilyStellar, &StellarCommitteeVerifierContractAdapter{})
		stellarAdapter := &StellarLaneAdapter{}
		lanes.GetLaneAdapterRegistry().RegisterLaneAdapter(chainsel.FamilyStellar, semver.MustParse("2.0.0"), stellarAdapter)
		ccvadapters.GetChainFamilyRegistry().RegisterChainFamily(chainsel.FamilyStellar, stellarAdapter)
		ccvadapters.GetAggregatorConfigRegistry().Register(chainsel.FamilyStellar, &StellarAggregatorConfigAdapter{})
		ccvadapters.GetIndexerConfigRegistry().Register(chainsel.FamilyStellar, &StellarIndexerConfigAdapter{})
		ccvadapters.GetVerifierJobConfigRegistry().Register(chainsel.FamilyStellar, &StellarVerifierConfigAdapter{})
		ccvadapters.GetExecutorConfigRegistry().Register(chainsel.FamilyStellar, &StellarExecutorConfigAdapter{})
		ccvadapters.GetTokenVerifierConfigRegistry().Register(chainsel.FamilyStellar, &StellarTokenVerifierConfigAdapter{})
	})
}
