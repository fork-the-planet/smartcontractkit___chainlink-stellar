package adapters

import (
	"context"
	"testing"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/Masterminds/semver/v3"
	"github.com/smartcontractkit/chainlink-ccip/deployment/deploy"
	tokens "github.com/smartcontractkit/chainlink-ccip/deployment/tokens"
	"github.com/smartcontractkit/chainlink-ccip/deployment/utils/changesets"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	ccvdeploymentadapters "github.com/smartcontractkit/chainlink-ccv/deployment/adapters"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/stretchr/testify/require"

	stellarsequences "github.com/smartcontractkit/chainlink-stellar/deployment/sequences"
)

func TestCCIPAdapterRegistrations_stellar(t *testing.T) {
	t.Parallel()
	v2 := semver.MustParse("2.0.0")

	_, ok := deploy.GetRegistry().GetDeployer(chainsel.FamilyStellar, v2)
	require.True(t, ok, "Deployer for stellar 2.0.0")

	_, ok = ccvadapters.GetDeployChainContractsRegistry().Get(chainsel.FamilyStellar)
	require.True(t, ok, "DeployChainContractsAdapter")

	_, ok = ccvadapters.GetChainFamilyRegistry().GetChainFamily(chainsel.FamilyStellar)
	require.True(t, ok, "ChainFamily")

	_, ok = ccvadapters.GetAggregatorConfigRegistry().Get(chainsel.FamilyStellar)
	require.True(t, ok, "AggregatorConfig")
	_, ok = ccvadapters.GetIndexerConfigRegistry().Get(chainsel.FamilyStellar)
	require.True(t, ok, "IndexerConfig")
	_, ok = ccvadapters.GetVerifierJobConfigRegistry().Get(chainsel.FamilyStellar)
	require.True(t, ok, "VerifierConfig")
	_, ok = ccvadapters.GetExecutorConfigRegistry().Get(chainsel.FamilyStellar)
	require.True(t, ok, "ExecutorConfig")
	_, ok = ccvadapters.GetTokenVerifierConfigRegistry().Get(chainsel.FamilyStellar)
	require.True(t, ok, "TokenVerifierConfig")
	_, ok = ccvadapters.GetCommitteeVerifierContractRegistry().Get(chainsel.FamilyStellar)
	require.True(t, ok, "CommitteeVerifierContract")

	_, ok = changesets.GetRegistry().GetMCMSReader(chainsel.FamilyStellar)
	require.True(t, ok, "MCMSReader")

	_, ok = tokens.GetTokenAdapterRegistry().GetTokenAdapter(chainsel.FamilyStellar, semver.MustParse("2.0.0"))
	require.True(t, ok, "TokenAdapter 2.0.0")
}

func TestCCVDeploymentAdapterRegistry_stellar(t *testing.T) {
	t.Parallel()
	a, ok := ccvdeploymentadapters.GetRegistry().Get(chainsel.FamilyStellar)
	require.True(t, ok)
	require.NotNil(t, a.Aggregator)
	require.NotNil(t, a.Executor)
	require.NotNil(t, a.Verifier)
	require.NotNil(t, a.Indexer)
	require.NotNil(t, a.TokenVerifier)
	require.NotNil(t, a.CommitteeVerifierOnchain)
}

func TestStellarDeployChainContractsAdapter_smoke(t *testing.T) {
	t.Parallel()
	var _ ccvadapters.DeployChainContractsAdapter = (*StellarDeployChainContractsAdapter)(nil)
	a := &StellarDeployChainContractsAdapter{}
	seqImp := a.SetContractParamsFromImportedConfig()
	require.NotNil(t, seqImp)
	require.Equal(t, stellarsequences.StellarImportConfigForDeployContracts.ID(), seqImp.ID())
	require.Equal(t, stellarsequences.SequenceVersion.String(), seqImp.Version())
	seqDep := a.DeployChainContracts()
	require.NotNil(t, seqDep)
	require.Equal(t, stellarsequences.StellarDeployChainContracts.ID(), seqDep.ID())
}

func TestStellarTokenAdapter_smoke(t *testing.T) {
	t.Parallel()
	var _ tokens.TokenAdapter = (*StellarTokenAdapter)(nil)
	a := &StellarTokenAdapter{}
	require.NotNil(t, a.ConfigureTokenForTransfersSequence())
	_, err := a.AddressRefToBytes(datastore.AddressRef{Address: "not-hex"})
	require.Error(t, err)
}

func TestStellarTransferOwnershipAdapter_smoke(t *testing.T) {
	t.Parallel()
	var _ deploy.TransferOwnershipAdapter = (*StellarTransferOwnershipAdapter)(nil)
	a := &StellarTransferOwnershipAdapter{}
	require.NotNil(t, a.SequenceTransferOwnershipViaMCMS())
	require.NotNil(t, a.SequenceAcceptOwnership())
}

func TestStellarVerifierConfigAdapter_smoke(t *testing.T) {
	t.Parallel()
	var _ ccvadapters.VerifierConfigAdapter = (*StellarVerifierConfigAdapter)(nil)
	ds := datastore.NewMemoryDataStore().Seal()
	_, err := (&StellarVerifierConfigAdapter{}).ResolveVerifierContractAddresses(ds, 1, "q", "q")
	require.Error(t, err)
}

func TestStellarTokenVerifierConfigAdapter_smoke(t *testing.T) {
	t.Parallel()
	var _ ccvadapters.TokenVerifierConfigAdapter = (*StellarTokenVerifierConfigAdapter)(nil)
	ds := datastore.NewMemoryDataStore().Seal()
	_, err := (&StellarTokenVerifierConfigAdapter{}).ResolveTokenVerifierAddresses(ds, 1, "", "")
	require.Error(t, err)
}

func TestStellarExecutorConfigAdapter_smoke(t *testing.T) {
	t.Parallel()
	var _ ccvadapters.ExecutorConfigAdapter = (*StellarExecutorConfigAdapter)(nil)
	a := &StellarExecutorConfigAdapter{}
	require.Nil(t, a.GetDeployedChains(nil, "q"))
	ds := datastore.NewMemoryDataStore().Seal()
	require.Empty(t, a.GetDeployedChains(ds, "q"))
	_, err := a.BuildChainConfig(ds, 1, "q")
	require.Error(t, err)
}

func TestStellarIndexerConfigAdapter_smoke(t *testing.T) {
	t.Parallel()
	var _ ccvadapters.IndexerConfigAdapter = (*StellarIndexerConfigAdapter)(nil)
	ds := datastore.NewMemoryDataStore().Seal()
	ad := &StellarIndexerConfigAdapter{}
	_, err := ad.ResolveVerifierAddresses(ds, 1, "q", ccvadapters.CCTPVerifierKind)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not support")

	var miss *ccvadapters.MissingIndexerVerifierAddressesError
	_, err = ad.ResolveVerifierAddresses(ds, 1, "q", ccvadapters.CommitteeVerifierKind)
	require.Error(t, err)
	require.ErrorAs(t, err, &miss)
}

func TestStellarCommitteeVerifierContractAdapter_smoke(t *testing.T) {
	t.Parallel()
	var _ ccvadapters.CommitteeVerifierContractAdapter = (*StellarCommitteeVerifierContractAdapter)(nil)
	ds := datastore.NewMemoryDataStore().Seal()
	_, err := (&StellarCommitteeVerifierContractAdapter{}).ResolveCommitteeVerifierContracts(ds, 1, "q")
	require.Error(t, err)
}

func TestStellarChainFamilyAdapter_smoke(t *testing.T) {
	t.Parallel()
	var _ ccvadapters.ChainFamily = (*StellarChainFamilyAdapter)(nil)
	a := &StellarChainFamilyAdapter{}
	require.True(t, a.GetFeeQuoterDestChainConfig().IsEnabled)
	require.NotNil(t, a.ConfigureChainForLanes())
	ds := datastore.NewMemoryDataStore().Seal()
	_, err := a.GetOnRampAddress(ds, 1)
	require.Error(t, err)
}

func TestStellarAggregatorConfigAdapter_smoke(t *testing.T) {
	t.Parallel()
	var _ ccvadapters.AggregatorConfigAdapter = (*StellarAggregatorConfigAdapter)(nil)
	ctx := context.Background()
	env := cldf.Environment{
		DataStore:   datastore.NewMemoryDataStore().Seal(),
		BlockChains: cldf_chain.NewBlockChains(nil),
	}
	states, err := (&StellarAggregatorConfigAdapter{}).ScanCommitteeStates(ctx, env, 1)
	require.NoError(t, err)
	require.Empty(t, states)
}

func TestStellarMCMSDeployer_smoke(t *testing.T) {
	t.Parallel()
	var _ deploy.Deployer = (*StellarMCMSDeployer)(nil)
	var _ changesets.MCMSReader = (*StellarMCMSReader)(nil)
	d := StellarMCMSDeployer{}
	require.Nil(t, d.DeployChainContracts())
	require.NotNil(t, d.DeployMCMS())
	require.NotNil(t, d.FinalizeDeployMCMS())
	require.NotNil(t, d.GrantAdminRoleToTimelock())
	require.NotNil(t, d.UpdateMCMSConfig())
}

func TestStellarCCVDeploymentAdapters_smoke(t *testing.T) {
	t.Parallel()
	var _ ccvdeploymentadapters.AggregatorConfigAdapter = (*StellarCCVDeploymentAggregatorConfigAdapter)(nil)
	var _ ccvdeploymentadapters.ExecutorConfigAdapter = (*StellarCCVDeploymentExecutorConfigAdapter)(nil)
	var _ ccvdeploymentadapters.VerifierConfigAdapter = (*StellarCCVDeploymentVerifierConfigAdapter)(nil)
	var _ ccvdeploymentadapters.IndexerConfigAdapter = (*StellarCCVDeploymentIndexerConfigAdapter)(nil)
	var _ ccvdeploymentadapters.TokenVerifierConfigAdapter = (*StellarCCVDeploymentTokenVerifierConfigAdapter)(nil)

	ds := datastore.NewMemoryDataStore().Seal()
	_, err := (&StellarCCVDeploymentAggregatorConfigAdapter{}).ResolveVerifierAddress(ds, 1, "q")
	require.Error(t, err)

	require.Empty(t, (&StellarCCVDeploymentExecutorConfigAdapter{}).GetDeployedChains(ds, "q"))
	_, err = (&StellarCCVDeploymentExecutorConfigAdapter{}).BuildChainConfig(ds, 1, "q")
	require.Error(t, err)

	require.Equal(t, chainsel.FamilyStellar, (&StellarCCVDeploymentVerifierConfigAdapter{}).GetSignerAddressFamily())
	_, err = (&StellarCCVDeploymentVerifierConfigAdapter{}).ResolveVerifierContractAddresses(ds, 1, "a", "b")
	require.Error(t, err)

	_, err = (&StellarCCVDeploymentIndexerConfigAdapter{}).ResolveVerifierAddresses(ds, 1, "q", ccvdeploymentadapters.CommitteeVerifierKind)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no committee verifier addresses")

	_, err = (&StellarCCVDeploymentTokenVerifierConfigAdapter{}).ResolveTokenVerifierAddresses(ds, 1, "", "")
	require.Error(t, err)
}
