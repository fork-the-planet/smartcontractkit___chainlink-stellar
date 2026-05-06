package stellardeploy

import (
	"context"
	"fmt"

	ccvdeployment "github.com/smartcontractkit/chainlink-ccv/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
	"github.com/smartcontractkit/chainlink-stellar/deployment/sequences"
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
	if host == nil {
		return nil, fmt.Errorf("stellar CCIP deploy host is nil")
	}
	if opBundle.GetContext == nil {
		return nil, fmt.Errorf("stellar CCIP deploy: operations bundle must provide GetContext")
	}
	offTopo, err := stellarccip.CCVEnvironmentTopologyToOffchain(topology)
	if err != nil {
		return nil, err
	}
	deps := stellardeps.FromDeployer(host.Deployer())
	out, err := sequences.RunStellarCCIPFullDeploy(ctx, opBundle, deps, host, offTopo, sequences.DeployStellarCCIPInnerInput{
		ChainSelector:     selector,
		AllSelectors:      allSelectors,
		ExistingAddresses: existingAddresses,
	})
	if err != nil {
		return nil, err
	}
	mem := datastore.NewMemoryDataStore()
	for _, ref := range out.Addresses {
		if err := mem.AddressRefStore.Upsert(ref); err != nil {
			return nil, err
		}
	}
	return mem.Seal(), nil
}
