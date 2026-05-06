package devenv

import (
	"context"

	ccvdeployment "github.com/smartcontractkit/chainlink-ccv/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	stellardeploy "github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellardeploy"
)

// DeployStellarCCIPContracts forwards to [stellardeploy.DeployStellarCCIPContracts].
func DeployStellarCCIPContracts(
	ctx context.Context,
	opBundle cldfops.Bundle,
	host Host,
	allSelectors []uint64,
	selector uint64,
	topology *ccvdeployment.EnvironmentTopology,
	existingAddresses []datastore.AddressRef,
) (datastore.DataStore, error) {
	return stellardeploy.DeployStellarCCIPContracts(ctx, opBundle, host, allSelectors, selector, topology, existingAddresses)
}
