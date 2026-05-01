package adapters

import (
	"github.com/Masterminds/semver/v3"
	chainsel "github.com/smartcontractkit/chain-selectors"
	tokenscore "github.com/smartcontractkit/chainlink-ccip/deployment/tokens"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	ccvdeploymentadapters "github.com/smartcontractkit/chainlink-ccv/deployment/adapters"
)

func init() {
	ccvadapters.GetCommitteeVerifierContractRegistry().Register(chainsel.FamilyStellar, &StellarCommitteeVerifierContractAdapter{})
	ccvadapters.GetChainFamilyRegistry().RegisterChainFamily(chainsel.FamilyStellar, &StellarChainFamilyAdapter{})
	ccvadapters.GetAggregatorConfigRegistry().Register(chainsel.FamilyStellar, &StellarAggregatorConfigAdapter{})
	ccvadapters.GetIndexerConfigRegistry().Register(chainsel.FamilyStellar, &StellarIndexerConfigAdapter{})
	ccvadapters.GetVerifierJobConfigRegistry().Register(chainsel.FamilyStellar, &StellarVerifierConfigAdapter{})
	ccvadapters.GetExecutorConfigRegistry().Register(chainsel.FamilyStellar, &StellarExecutorConfigAdapter{})
	ccvadapters.GetTokenVerifierConfigRegistry().Register(chainsel.FamilyStellar, &StellarTokenVerifierConfigAdapter{})
	ccvadapters.GetDeployChainContractsRegistry().Register(chainsel.FamilyStellar, &StellarDeployChainContractsAdapter{})

	ccvdeploymentadapters.GetRegistry().Register(chainsel.FamilyStellar, ccvdeploymentadapters.ChainAdapters{
		Aggregator:    &StellarCCVDeploymentAggregatorConfigAdapter{},
		Executor:      &StellarCCVDeploymentExecutorConfigAdapter{},
		Verifier:      &StellarCCVDeploymentVerifierConfigAdapter{},
		Indexer:       &StellarCCVDeploymentIndexerConfigAdapter{},
		TokenVerifier: &StellarCCVDeploymentTokenVerifierConfigAdapter{},
	})

	tokenscore.GetTokenAdapterRegistry().RegisterTokenAdapter(
		chainsel.FamilyStellar, semver.MustParse("1.0.0"), &StellarTokenAdapter{},
	)
}
