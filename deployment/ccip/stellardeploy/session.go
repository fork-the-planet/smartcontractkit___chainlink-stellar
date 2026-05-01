package stellardeploy

import (
	"context"
	"fmt"

	ccvdeployment "github.com/smartcontractkit/chainlink-ccv/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// StellarCCIPDeploySession holds in-memory deploy state across phased steps.
// Create with [NewStellarCCIPDeploySession], run phases in order, then [StellarCCIPDeploySession.SealDataStore]
// or call [StellarCCIPDeploySession.RunAllPhasesAndSeal] for the full stack.
type StellarCCIPDeploySession struct {
	w *deployRun
}

// NewStellarCCIPDeploySession merges existing address refs, finds the repo root, and prepares phased deploy.
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
	w := &deployRun{
		host:              host,
		ctx:               ctx,
		opBundle:          opBundle,
		allSelectors:      allSelectors,
		selector:          selector,
		topology:          topology,
		existingAddresses: existingAddresses,
	}
	if err := w.setup(); err != nil {
		return nil, err
	}
	return &StellarCCIPDeploySession{w: w}, nil
}

// RunAllPhasesAndSeal runs all devenv phases in order and returns a sealed datastore view.
func (s *StellarCCIPDeploySession) RunAllPhasesAndSeal() (datastore.DataStore, error) {
	if err := s.DeployFoundationPhase(); err != nil {
		return nil, err
	}
	if err := s.DeployVerificationAndFeesPhase(); err != nil {
		return nil, err
	}
	if err := s.DeployRampsAndLanesPhase(); err != nil {
		return nil, err
	}
	if err := s.DeployReceiverAndDatastorePhase(); err != nil {
		return nil, err
	}
	return s.SealDataStore(), nil
}

// DeployFoundationPhase runs foundation contract deploys and datastore recording.
func (s *StellarCCIPDeploySession) DeployFoundationPhase() error {
	return s.w.deployFoundationContracts()
}

// DeployVerificationAndFeesPhase configures committee verification and fee quoter.
func (s *StellarCCIPDeploySession) DeployVerificationAndFeesPhase() error {
	return s.w.configureVerificationAndFeeQuoter()
}

// DeployRampsAndLanesPhase deploys ramps, router, registry, and provisional lanes.
func (s *StellarCCIPDeploySession) DeployRampsAndLanesPhase() error {
	return s.w.deployRampsAndProvisionalLanes()
}

// DeployReceiverAndDatastorePhase deploys the receiver and final datastore writes.
func (s *StellarCCIPDeploySession) DeployReceiverAndDatastorePhase() error {
	return s.w.deployReceiverAndWriteDatastore()
}

// SealDataStore returns an immutable view of the in-memory datastore (call once after all phases).
func (s *StellarCCIPDeploySession) SealDataStore() datastore.DataStore {
	return s.w.ds.Seal()
}
