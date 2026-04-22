package devenv

import (
	"sync"

	chainsel "github.com/smartcontractkit/chain-selectors"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/registry"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/chainconfig"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/committeeverifier"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/services/executor"

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
// We intentionally do NOT call cciptestinterfaces.RegisterExtraArgsSerializer here.
// Previously this file registered FamilyStellar with the same SerializeEVMExtraArgs used
// for EVM (ABI-encoded GenericExtraArgsV1/V2/V3 with function selectors). That was misleading
// and unsafe:
//   - Stellar OnRamp parses user extra_args as Soroban XDR of GenericExtraArgsV3
//     (see ccvchain.EncodeExtraArgsV3 / EncodeStellarSourceExtraArgsForOnRamp), not EVM ABI.
//     Registering EVM bytes under FamilyStellar implied “destination Stellar” messages should
//     carry EVM-style extraArgs, which the Stellar contracts do not accept.
//   - Composable Stellar→EVM builds extra_args using GetExtraArgsSerializer(destFamily); for
//     an EVM destination that is FamilyEVM’s serializer anyway, so the FamilyStellar entry did
//     not fix Stellar-as-source—it only risked confusing any code path that looks up
//     FamilyStellar expecting Soroban-shaped bytes.
// Stellar-source encoding stays explicit on ccvchain.Chain via EncodeStellarSourceExtraArgsForOnRamp.
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
	})
}
