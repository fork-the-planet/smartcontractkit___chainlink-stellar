package devenv

import (
	"context"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
	stellardeploy "github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellardeploy"
)

// DeployLockReleaseTestTokenPool forwards to [stellardeploy.DeployLockReleaseTestTokenPool].
func DeployLockReleaseTestTokenPool(ctx context.Context, opBundle cldfops.Bundle, host Host) error {
	return stellardeploy.DeployLockReleaseTestTokenPool(ctx, opBundle, host)
}

// LockReleasePoolAddressRefDataStore forwards to [stellarccip.LockReleasePoolAddressRefDataStore].
func LockReleasePoolAddressRefDataStore(chainSelector uint64, poolContractID, tokenContractID string) (datastore.DataStore, error) {
	return stellarccip.LockReleasePoolAddressRefDataStore(chainSelector, poolContractID, tokenContractID)
}
