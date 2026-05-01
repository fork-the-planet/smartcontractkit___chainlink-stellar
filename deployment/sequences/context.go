package sequences

import (
	"context"
	"fmt"
	"sync"

	seq_core "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"
	ccvdeployment "github.com/smartcontractkit/chainlink-ccv/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// StellarDeployRunner is implemented by *ccvchain.Chain. It lives in this
// package so deployment/sequences does not import ccv/chain (avoids cycles).
type StellarDeployRunner interface {
	DeployStellarCCIPContracts(
		ctx context.Context,
		opBundle cldf_ops.Bundle,
		allSelectors []uint64,
		selector uint64,
		topology *ccvdeployment.EnvironmentTopology,
		existingAddresses []datastore.AddressRef,
	) (seq_core.OnChainOutput, error)
	StellarDepsForDeploy() stellardeps.StellarDeps
}

type stellarDeployChainContext struct {
	runner   StellarDeployRunner
	topology *ccvdeployment.EnvironmentTopology
}

var stellarDeployChainCtxBySelector sync.Map // uint64 -> *stellarDeployChainContext

// RegisterStellarDeployChainContext records the runner and topology for a chain
// selector before the shared CCIP DeployChainContracts changeset runs.
func RegisterStellarDeployChainContext(selector uint64, runner StellarDeployRunner, topology *ccvdeployment.EnvironmentTopology) {
	stellarDeployChainCtxBySelector.Store(selector, &stellarDeployChainContext{runner: runner, topology: topology})
}

// ClearStellarDeployChainContext removes the context after post-deploy work.
func ClearStellarDeployChainContext(selector uint64) {
	stellarDeployChainCtxBySelector.Delete(selector)
}

func takeStellarDeployChainContext(selector uint64) (StellarDeployRunner, *ccvdeployment.EnvironmentTopology, error) {
	v, ok := stellarDeployChainCtxBySelector.Load(selector)
	if !ok {
		return nil, nil, fmt.Errorf("stellar deploy context missing for selector %d", selector)
	}
	ctx := v.(*stellarDeployChainContext)
	return ctx.runner, ctx.topology, nil
}
