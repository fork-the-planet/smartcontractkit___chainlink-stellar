package adapters

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-ccip/deployment/fees"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
)

func TestStellarFeeAdapter_InterfaceCompliance(t *testing.T) {
	var _ fees.FeeAdapter = (*StellarFeeAdapter)(nil)
}

func TestStellarFeeAdapter_GetDefaultTokenTransferFeeConfig(t *testing.T) {
	a := &StellarFeeAdapter{}
	cfg := a.GetDefaultTokenTransferFeeConfig(0, 0)
	require.True(t, cfg.IsEnabled)
	require.Greater(t, cfg.MinFeeUSDCents, uint32(0))
	require.GreaterOrEqual(t, cfg.MaxFeeUSDCents, cfg.MinFeeUSDCents)
}

func TestStellarFeeAdapter_GetDefaultDestChainConfig(t *testing.T) {
	a := &StellarFeeAdapter{}
	cfg := a.GetDefaultDestChainConfig(0, 0)
	require.True(t, cfg.IsEnabled)
	require.Greater(t, cfg.MaxDataBytes, uint32(0))
	require.Greater(t, cfg.MaxPerMsgGasLimit, uint32(0))
}

func TestStellarFeeAdapter_GetFeeContractRef_emptyDatastore(t *testing.T) {
	a := &StellarFeeAdapter{}
	env := envWithDatastore(newSealedDatastore())
	_, err := a.GetFeeContractRef(env, 42, 0)
	require.Error(t, err)
}

func TestStellarFeeAdapter_GetFeeContractRef_found(t *testing.T) {
	a := &StellarFeeAdapter{}
	ds := datastore.NewMemoryDataStore()
	ref := datastore.AddressRef{
		ChainSelector: 42,
		Type:          "FeeQuoter",
		Version:       semver.MustParse("2.0.0"),
		Address:       "CFQADDR",
	}
	require.NoError(t, ds.Addresses().Upsert(ref))
	env := envWithDatastore(ds.Seal())
	got, err := a.GetFeeContractRef(env, 42, 0)
	require.NoError(t, err)
	require.Equal(t, "CFQADDR", got.Address)
}

func TestStellarFeeAdapter_SetTokenTransferFee_nonNil(t *testing.T) {
	a := &StellarFeeAdapter{}
	env := envWithDatastore(newSealedDatastore())
	seq := a.SetTokenTransferFee(env)
	require.NotNil(t, seq)
}

func TestStellarFeeAdapter_ApplyDestChainConfigUpdates_nonNil(t *testing.T) {
	a := &StellarFeeAdapter{}
	env := envWithDatastore(newSealedDatastore())
	seq := a.ApplyDestChainConfigUpdates(env)
	require.NotNil(t, seq)
}

func TestStellarFeeAdapter_Registration(t *testing.T) {
	reg := fees.GetRegistry()
	_, ok := reg.GetFeeAdapter("stellar", stellarops.ContractDeploymentVersion)
	if !ok {
		_, ok = reg.GetFeeAdapter("stellar", semver.MustParse("2.0.0"))
	}
	require.True(t, ok, "StellarFeeAdapter should be registered from init()")
}

func TestStellarFeeAggregatorAdapter_InterfaceCompliance(t *testing.T) {
	var _ fees.FeeAggregatorAdapter = (*StellarFeeAggregatorAdapter)(nil)
}

func TestStellarFeeAggregatorAdapter_SetFeeAggregator_nonNil(t *testing.T) {
	a := &StellarFeeAggregatorAdapter{}
	env := envWithDatastore(newSealedDatastore())
	seq := a.SetFeeAggregator(env)
	require.NotNil(t, seq)
}

func TestStellarFeeAggregatorAdapter_GetFeeAggregator_notImplemented(t *testing.T) {
	a := &StellarFeeAggregatorAdapter{}
	env := envWithDatastore(newSealedDatastore())
	_, err := a.GetFeeAggregator(env, 42)
	require.Error(t, err)
}

func TestStellarFeeAggregatorAdapter_Registration(t *testing.T) {
	reg := fees.GetFeeAggregatorRegistry()
	_, ok := reg.GetFeeAggregatorAdapter("stellar", semver.MustParse("2.0.0"))
	require.True(t, ok, "StellarFeeAggregatorAdapter should be registered from init()")
}
