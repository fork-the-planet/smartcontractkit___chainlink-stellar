package ccip

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
)

func TestMergeExistingAddressRefs(t *testing.T) {
	ds := datastore.NewMemoryDataStore()
	ref := datastore.AddressRef{
		Address:       "0x01",
		ChainSelector: 42,
		Type:          datastore.ContractType("Foo"),
		Version:       semver.MustParse("1.0.0"),
	}
	require.NoError(t, MergeExistingAddressRefs(ds, []datastore.AddressRef{ref}))
	got, err := ds.Addresses().Get(datastore.NewAddressRefKey(42, datastore.ContractType("Foo"), semver.MustParse("1.0.0"), ""))
	require.NoError(t, err)
	require.Equal(t, ref.Address, got.Address)
}
