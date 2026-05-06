package mcmsutil

import (
	"testing"

	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/chainlink-ccip/deployment/utils"
	mcmsutils "github.com/smartcontractkit/chainlink-ccip/deployment/utils/mcms"
)

func TestMCMSRefLookupOrder(t *testing.T) {
	t.Run("schedule", func(t *testing.T) {
		order, err := MCMSRefLookupOrder(mcmstypes.TimelockActionSchedule)
		require.NoError(t, err)
		require.Equal(t, cldf.ContractType(utils.ProposerManyChainMultisig), order[0])
	})

	t.Run("unsupported", func(t *testing.T) {
		_, err := MCMSRefLookupOrder(mcmstypes.TimelockAction("nope"))
		require.Error(t, err)
	})
}

func TestFindStellarTimelockAddressRef(t *testing.T) {
	chainSel := uint64(42)
	qual := "q1"
	mcmsAddr := "CMCMSAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAY"
	tlAddr := "CTLOCKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAY"

	input := mcmsutils.Input{
		Qualifier:      qual,
		TimelockAction: mcmstypes.TimelockActionSchedule,
	}

	t.Run("prefers_RBACTimelock_when_present", func(t *testing.T) {
		ds := datastore.NewMemoryDataStore()
		for _, r := range StellarMCMSDatastoreRefs(chainSel, qual, mcmsAddr) {
			require.NoError(t, ds.Addresses().Upsert(r))
		}
		require.NoError(t, ds.Addresses().Upsert(StellarTimelockDatastoreRef(chainSel, qual, tlAddr)))

		env := cldf.Environment{DataStore: ds.Seal()}
		ref, err := FindStellarTimelockAddressRef(env, chainSel, input)
		require.NoError(t, err)
		require.Equal(t, tlAddr, ref.Address)
		require.Equal(t, datastore.ContractType(utils.RBACTimelock), ref.Type)
	})

	t.Run("falls_back_to_MCMS_when_no_timelock_row", func(t *testing.T) {
		ds := datastore.NewMemoryDataStore()
		for _, r := range StellarMCMSDatastoreRefs(chainSel, qual, mcmsAddr) {
			require.NoError(t, ds.Addresses().Upsert(r))
		}

		env := cldf.Environment{DataStore: ds.Seal()}
		ref, err := FindStellarTimelockAddressRef(env, chainSel, input)
		require.NoError(t, err)
		require.Equal(t, mcmsAddr, ref.Address)
	})

	t.Run("FindStellarMCMSAddressRef_errors_when_empty", func(t *testing.T) {
		ds := datastore.NewMemoryDataStore()
		env := cldf.Environment{DataStore: ds.Seal()}
		_, err := FindStellarMCMSAddressRef(env, chainSel, input)
		require.Error(t, err)
	})
}
