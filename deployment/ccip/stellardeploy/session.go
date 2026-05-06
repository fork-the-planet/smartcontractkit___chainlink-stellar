package stellardeploy

import (
	"context"
	"fmt"

	ccvdeployment "github.com/smartcontractkit/chainlink-ccv/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// StellarCCIPDeploySession runs the full Stellar CCIP Soroban deploy via
// [DeployStellarCCIPContracts] / [RunStellarCCIPFullDeploy] in deployment/sequences.
type StellarCCIPDeploySession struct {
	ctx               context.Context
	opBundle          cldfops.Bundle
	host              Host
	allSelectors      []uint64
	selector          uint64
	topology          *ccvdeployment.EnvironmentTopology
	existingAddresses []datastore.AddressRef
}

// NewStellarCCIPDeploySession validates inputs for a Stellar CCIP deploy run.
// opBundle must be a valid CLDF bundle (e.g. the parent changeset bundle from chainlink-ccv DeployContractsForSelector,
// or [cldfops.NewBundle] for standalone tests); all Soroban operations run on this bundle.
func NewStellarCCIPDeploySession(
	ctx context.Context,
	opBundle cldfops.Bundle,
	host Host,
	allSelectors []uint64,
	selector uint64,
	topology *ccvdeployment.EnvironmentTopology,
	existingAddresses []datastore.AddressRef,
) (*StellarCCIPDeploySession, error) {
	if host == nil {
		return nil, fmt.Errorf("stellar CCIP deploy host is nil")
	}
	if opBundle.GetContext == nil {
		return nil, fmt.Errorf("stellar CCIP deploy: operations bundle must provide GetContext")
	}
	return &StellarCCIPDeploySession{
		ctx:               ctx,
		opBundle:          opBundle,
		host:              host,
		allSelectors:      allSelectors,
		selector:          selector,
		topology:          topology,
		existingAddresses: existingAddresses,
	}, nil
}

// RunAllPhasesAndSeal deploys and configures the full stack and returns a sealed datastore view.
func (s *StellarCCIPDeploySession) RunAllPhasesAndSeal() (datastore.DataStore, error) {
	return DeployStellarCCIPContracts(s.ctx, s.opBundle, s.host, s.allSelectors, s.selector, s.topology, s.existingAddresses)
}
