package stellardeploy

import (
	"context"

	ccvdeployment "github.com/smartcontractkit/chainlink-ccv/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// DeployStellarCCIPContracts deploys the full Stellar CCIP stack for devenv.
// allSelectors must list every chain selector in the environment.
// opBundle must be the CLDF bundle for this run (parent changeset bundle from sequences, or [cldfops.NewBundle] for tests).
func DeployStellarCCIPContracts(
	ctx context.Context,
	opBundle cldfops.Bundle,
	host Host,
	allSelectors []uint64,
	selector uint64,
	topology *ccvdeployment.EnvironmentTopology,
	existingAddresses []datastore.AddressRef,
) (datastore.DataStore, error) {
	s, err := NewStellarCCIPDeploySession(ctx, opBundle, host, allSelectors, selector, topology, existingAddresses)
	if err != nil {
		return nil, err
	}
	return s.RunAllPhasesAndSeal()
}
