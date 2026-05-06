package devenv

import (
	"context"

	ccvdeployment "github.com/smartcontractkit/chainlink-ccv/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	stellardeploy "github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellardeploy"
)

// StellarCCIPDeploySession is an alias for [stellardeploy.StellarCCIPDeploySession].
type StellarCCIPDeploySession = stellardeploy.StellarCCIPDeploySession

// NewStellarCCIPDeploySession forwards to [stellardeploy.NewStellarCCIPDeploySession].
func NewStellarCCIPDeploySession(
	ctx context.Context,
	opBundle cldfops.Bundle,
	host Host,
	allSelectors []uint64,
	selector uint64,
	topology *ccvdeployment.EnvironmentTopology,
	existingAddresses []datastore.AddressRef,
) (*StellarCCIPDeploySession, error) {
	return stellardeploy.NewStellarCCIPDeploySession(ctx, opBundle, host, allSelectors, selector, topology, existingAddresses)
}
