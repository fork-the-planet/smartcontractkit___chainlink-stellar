package adapters

import (
	"testing"

	"github.com/smartcontractkit/chainlink-ccip/deployment/fastcurse"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"

	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
)

func TestStellarCurseAdapter_InterfaceCompliance(t *testing.T) {
	a := NewStellarCurseAdapter()
	var _ fastcurse.CurseAdapter = a
	var _ fastcurse.CurseSubjectAdapter = a
}

func TestStellarCurseAdapter_SelectorToSubject(t *testing.T) {
	a := NewStellarCurseAdapter()
	sel := uint64(12345)
	subject := a.SelectorToSubject(sel)
	got := fastcurse.GenericSelectorToSubject(sel)
	require.Equal(t, got, subject)
}

func TestStellarCurseAdapter_SubjectToSelector(t *testing.T) {
	a := NewStellarCurseAdapter()

	t.Run("normal", func(t *testing.T) {
		sel := uint64(99)
		subject := fastcurse.GenericSelectorToSubject(sel)
		got, err := a.SubjectToSelector(subject)
		require.NoError(t, err)
		require.Equal(t, sel, got)
	})

	t.Run("global_returns_zero", func(t *testing.T) {
		got, err := a.SubjectToSelector(fastcurse.GlobalCurseSubject())
		require.NoError(t, err)
		require.Equal(t, uint64(0), got)
	})
}

func TestStellarCurseAdapter_DeriveCurseAdapterVersion(t *testing.T) {
	a := NewStellarCurseAdapter()
	env := envWithDatastore(newSealedDatastore())
	v, err := a.DeriveCurseAdapterVersion(env, 0)
	require.NoError(t, err)
	require.True(t, v.Equal(stellarops.ContractDeploymentVersion))
}

func TestStellarCurseAdapter_IsCurseEnabled_beforeInit(t *testing.T) {
	a := NewStellarCurseAdapter()
	env := envWithDatastore(newSealedDatastore())
	ok, err := a.IsCurseEnabledForChain(env, 42)
	require.NoError(t, err)
	require.False(t, ok, "should be false before Initialize")
}

func TestStellarCurseAdapter_CurseUncurse_sequenceNonNil(t *testing.T) {
	a := NewStellarCurseAdapter()
	require.NotNil(t, a.Curse())
	require.NotNil(t, a.Uncurse())
}

func TestStellarCurseAdapter_SubjectRoundTrip(t *testing.T) {
	a := NewStellarCurseAdapter()
	for _, sel := range []uint64{0, 1, 100, 1<<63 - 1} {
		subject := a.SelectorToSubject(sel)
		got, err := a.SubjectToSelector(subject)
		require.NoError(t, err)
		require.Equal(t, sel, got, "roundtrip failed for selector %d", sel)
	}
}

func TestStellarCurseAdapter_Registration(t *testing.T) {
	reg := fastcurse.GetCurseRegistry()
	v := stellarops.ContractDeploymentVersion

	_, ok := reg.GetCurseAdapter("stellar", v)
	require.True(t, ok, "StellarCurseAdapter should be registered from init()")

	_, ok = reg.GetCurseSubjectAdapter("stellar")
	require.True(t, ok, "StellarCurseSubjectAdapter should be registered from init()")
}

func TestStellarContractIDOnChain_emptyDatastore(t *testing.T) {
	ds := newSealedDatastore()
	env := envWithDatastore(ds)
	_, err := stellarContractIDOnChain(env, 42, stellarccip.RouterDatastoreRef())
	require.Error(t, err)
}

func TestStellarContractIDOnChain_routerResolvesToStrkey(t *testing.T) {
	ds := datastore.NewMemoryDataStore()
	sel := uint64(7)
	routerStrkey := stellarutil.MustGenerateMockContractID("deployer", "router-curse-adapter-test")
	require.NoError(t, stellarccip.RecordRouter(ds, sel, routerStrkey))
	env := envWithDatastore(ds.Seal())
	got, err := stellarContractIDOnChain(env, sel, stellarccip.RouterDatastoreRef())
	require.NoError(t, err)
	require.Equal(t, routerStrkey, got)
}
