package adapters

import (
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

func newSealedDatastore() datastore.DataStore {
	return datastore.NewMemoryDataStore().Seal()
}

func envWithDatastore(ds datastore.DataStore) cldf.Environment {
	return cldf.Environment{DataStore: ds}
}
