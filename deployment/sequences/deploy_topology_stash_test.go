package sequences

import (
	"testing"

	"github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/offchain"
	"github.com/stretchr/testify/require"
)

func TestRegisterStellarDeployOffchainTopologyForSelector_nilNoOp(t *testing.T) {
	t.Parallel()
	sel := uint64(881001)
	t.Cleanup(func() { ClearStellarDeployOffchainTopologyForSelector(sel) })

	RegisterStellarDeployOffchainTopologyForSelector(sel, nil)
	got, ok := TakeStellarDeployOffchainTopologyForSelector(sel)
	require.False(t, ok)
	require.Nil(t, got)
}

func TestStellarDeployOffchainTopology_RegisterTakeClear(t *testing.T) {
	t.Parallel()
	sel := uint64(881002)
	t.Cleanup(func() { ClearStellarDeployOffchainTopologyForSelector(sel) })

	topo := &offchain.EnvironmentTopology{PyroscopeURL: "http://stash-test"}
	RegisterStellarDeployOffchainTopologyForSelector(sel, topo)

	got, ok := TakeStellarDeployOffchainTopologyForSelector(sel)
	require.True(t, ok)
	require.NotNil(t, got)
	require.Equal(t, "http://stash-test", got.PyroscopeURL)

	_, ok = TakeStellarDeployOffchainTopologyForSelector(sel)
	require.False(t, ok)

	RegisterStellarDeployOffchainTopologyForSelector(sel, topo)
	ClearStellarDeployOffchainTopologyForSelector(sel)
	_, ok = TakeStellarDeployOffchainTopologyForSelector(sel)
	require.False(t, ok)
}

func TestStellarDeployOffchainTopology_overwritesPrevious(t *testing.T) {
	t.Parallel()
	sel := uint64(881003)
	t.Cleanup(func() { ClearStellarDeployOffchainTopologyForSelector(sel) })

	RegisterStellarDeployOffchainTopologyForSelector(sel, &offchain.EnvironmentTopology{PyroscopeURL: "first"})
	RegisterStellarDeployOffchainTopologyForSelector(sel, &offchain.EnvironmentTopology{PyroscopeURL: "second"})
	got, ok := TakeStellarDeployOffchainTopologyForSelector(sel)
	require.True(t, ok)
	require.Equal(t, "second", got.PyroscopeURL)
}
