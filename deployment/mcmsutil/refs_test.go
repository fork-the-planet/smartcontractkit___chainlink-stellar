package mcmsutil

import (
	"bytes"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"

	"github.com/smartcontractkit/chainlink-ccip/deployment/deploy"
	"github.com/smartcontractkit/chainlink-ccip/deployment/utils"
)

func TestTimelockDeploySalt_deterministic(t *testing.T) {
	s1 := TimelockDeploySalt(7, "q")
	s2 := TimelockDeploySalt(7, "q")
	require.True(t, bytes.Equal(s1[:], s2[:]))
}

func TestTimelockDeploySalt_differsFromMCMSDeploySalt(t *testing.T) {
	a := TimelockDeploySalt(1, "x")
	b := MCMSDeploySalt(1, "x")
	require.False(t, bytes.Equal(a[:], b[:]), "timelock and MCMS salts must not collide")
}

func TestFindExistingStellarTimelock(t *testing.T) {
	chainSel := uint64(999)
	qual := "qual-a"
	tlAddr := "CTIMELOCKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	t.Run("match", func(t *testing.T) {
		refs := []datastore.AddressRef{
			StellarTimelockDatastoreRef(chainSel, qual, tlAddr),
		}
		got, ok := FindExistingStellarTimelock(refs, chainSel, qual)
		require.True(t, ok)
		require.Equal(t, tlAddr, got)
	})

	t.Run("wrong_chain", func(t *testing.T) {
		refs := []datastore.AddressRef{StellarTimelockDatastoreRef(chainSel, qual, tlAddr)}
		_, ok := FindExistingStellarTimelock(refs, chainSel+1, qual)
		require.False(t, ok)
	})

	t.Run("wrong_qualifier", func(t *testing.T) {
		refs := []datastore.AddressRef{StellarTimelockDatastoreRef(chainSel, qual, tlAddr)}
		_, ok := FindExistingStellarTimelock(refs, chainSel, "other")
		require.False(t, ok)
	})

	t.Run("wrong_version", func(t *testing.T) {
		ref := StellarTimelockDatastoreRef(chainSel, qual, tlAddr)
		ref.Version = semver.MustParse("0.9.0")
		_, ok := FindExistingStellarTimelock([]datastore.AddressRef{ref}, chainSel, qual)
		require.False(t, ok)
	})

	t.Run("wrong_type", func(t *testing.T) {
		ref := StellarTimelockDatastoreRef(chainSel, qual, tlAddr)
		ref.Type = datastore.ContractType(utils.ProposerManyChainMultisig)
		_, ok := FindExistingStellarTimelock([]datastore.AddressRef{ref}, chainSel, qual)
		require.False(t, ok)
	})
}

func TestStellarTimelockDatastoreRef(t *testing.T) {
	chainSel := uint64(3)
	qual := "q"
	addr := "CADDR"
	ref := StellarTimelockDatastoreRef(chainSel, qual, addr)
	require.Equal(t, chainSel, ref.ChainSelector)
	require.Equal(t, qual, ref.Qualifier)
	require.Equal(t, addr, ref.Address)
	require.Equal(t, datastore.ContractType(utils.RBACTimelock), ref.Type)
	require.True(t, ref.Version.Equal(deploy.MCMSVersion))
}
