// Package adapters registers Stellar with chainlink-ccip CCIP 2.0 tooling (v2_0_0 adapter
// registries), chainlink-ccv/deployment/adapters for service-config changesets, and shared
// infrastructure (MCMS, transfer ownership, token pools, fees, RMN curse).
//
// Devenv-only hooks under chainlink-ccv/build/devenv (chain config loader, verifier/executor
// modifiers, ImplFactory, CLDF provider, extra-args serializers) are not registered here; they
// live in ccv/chain/register.go and run when RegisterStellarDevenvComponents is called;
// see the package doc there for the full split.
// The ccvchain package blank-imports this package so its init runs as soon as ccvchain loads.
package adapters

import (
	"github.com/Masterminds/semver/v3"
	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccip/deployment/deploy"
	"github.com/smartcontractkit/chainlink-ccip/deployment/fastcurse"
	"github.com/smartcontractkit/chainlink-ccip/deployment/fees"
	tokenscore "github.com/smartcontractkit/chainlink-ccip/deployment/tokens"
	"github.com/smartcontractkit/chainlink-ccip/deployment/utils/changesets"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	ccvdeploymentadapters "github.com/smartcontractkit/chainlink-ccv/deployment/adapters"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
)

// init performs CCIP-platform tooling API registration for FamilyStellar. It does not call
// into chainlink-ccv build/devenv registries; those are populated from ccv/chain/register.go.
func init() {
	v2 := semver.MustParse("2.0.0")
	deploy.GetRegistry().RegisterDeployer(chainsel.FamilyStellar, v2, &StellarMCMSDeployer{})
	changesets.GetRegistry().RegisterMCMSReader(chainsel.FamilyStellar, &StellarMCMSReader{})
	deploy.GetTransferOwnershipRegistry().RegisterAdapter(chainsel.FamilyStellar, deploy.MCMSVersion, &StellarTransferOwnershipAdapter{})
	deploy.GetTransferOwnershipRegistry().RegisterAdapter(chainsel.FamilyStellar, v2, &StellarTransferOwnershipAdapter{})

	ccvadapters.GetCommitteeVerifierContractRegistry().Register(chainsel.FamilyStellar, &StellarCommitteeVerifierContractAdapter{})
	ccvadapters.GetChainFamilyRegistry().RegisterChainFamily(chainsel.FamilyStellar, &StellarChainFamilyAdapter{})
	ccvadapters.GetAggregatorConfigRegistry().Register(chainsel.FamilyStellar, &StellarAggregatorConfigAdapter{})
	ccvadapters.GetIndexerConfigRegistry().Register(chainsel.FamilyStellar, &StellarIndexerConfigAdapter{})
	ccvadapters.GetVerifierJobConfigRegistry().Register(chainsel.FamilyStellar, &StellarVerifierConfigAdapter{})
	ccvadapters.GetExecutorConfigRegistry().Register(chainsel.FamilyStellar, &StellarExecutorConfigAdapter{})
	ccvadapters.GetTokenVerifierConfigRegistry().Register(chainsel.FamilyStellar, &StellarTokenVerifierConfigAdapter{})
	ccvadapters.GetDeployChainContractsRegistry().Register(chainsel.FamilyStellar, &StellarDeployChainContractsAdapter{})

	// chainlink-ccv/deployment/adapters: service-config changesets need CommitteeVerifierOnchain
	// for on-chain committee scans plus the StellarCCVDeployment* adapters below.
	ccvdeploymentadapters.GetRegistry().Register(chainsel.FamilyStellar, ccvdeploymentadapters.ChainAdapters{
		Aggregator:               &StellarCCVDeploymentAggregatorConfigAdapter{},
		CommitteeVerifierOnchain: &StellarCCVCommitteeVerifierOnchainAdapter{},
		Executor:                 &StellarCCVDeploymentExecutorConfigAdapter{},
		Verifier:                 &StellarCCVDeploymentVerifierConfigAdapter{},
		Indexer:                  &StellarCCVDeploymentIndexerConfigAdapter{},
		TokenVerifier:            &StellarCCVDeploymentTokenVerifierConfigAdapter{},
	})

	tokenscore.GetTokenAdapterRegistry().RegisterTokenAdapter(
		chainsel.FamilyStellar, semver.MustParse("1.0.0"), &StellarTokenAdapter{},
	)

	fees.GetRegistry().RegisterFeeAdapter(chainsel.FamilyStellar, v2, &StellarFeeAdapter{})
	fees.GetFeeAggregatorRegistry().RegisterFeeAggregatorAdapter(chainsel.FamilyStellar, v2, &StellarFeeAggregatorAdapter{})

	curseAdapter := NewStellarCurseAdapter()
	fastcurse.GetCurseRegistry().RegisterNewCurse(fastcurse.CurseRegistryInput{
		CursingFamily:       chainsel.FamilyStellar,
		CursingVersion:      stellarops.ContractDeploymentVersion,
		CurseAdapter:        curseAdapter,
		CurseSubjectAdapter: curseAdapter,
	})
}
