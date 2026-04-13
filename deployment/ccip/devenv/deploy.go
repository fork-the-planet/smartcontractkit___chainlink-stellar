package devenv

import (
	"context"
	"fmt"

	ccipOffchain "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/offchain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
)

// DeployStellarCCIPContracts deploys the full Stellar CCIP stack for devenv.
// allSelectors must list every chain selector in the environment.
func DeployStellarCCIPContracts(ctx context.Context, host Host, allSelectors []uint64, selector uint64, topology *ccipOffchain.EnvironmentTopology) (datastore.DataStore, error) {
	if host == nil {
		return nil, fmt.Errorf("stellar CCIP deploy host is nil")
	}
	w := &work{
		host:         host,
		ctx:          ctx,
		allSelectors: allSelectors,
		selector:     selector,
		topology:     topology,
	}
	if err := w.setup(); err != nil {
		return nil, err
	}
	if err := w.deployFoundationContracts(); err != nil {
		return nil, err
	}
	if err := w.configureVerificationAndFeeQuoter(); err != nil {
		return nil, err
	}
	if err := w.deployRampsAndProvisionalLanes(); err != nil {
		return nil, err
	}
	if err := w.deployReceiverAndWriteDatastore(); err != nil {
		return nil, err
	}
	return w.ds.Seal(), nil
}
