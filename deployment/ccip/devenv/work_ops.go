package devenv

import (
	"context"

	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

func (w *work) initOperationsBundle() {
	w.opBundle = cldfops.NewBundle(
		func() context.Context { return w.ctx },
		cldflogger.Nop(),
		cldfops.NewMemoryReporter(),
	)
}

func (w *work) stellarDeps() stellardeps.StellarDeps {
	return stellardeps.FromDeployer(w.host.Deployer())
}

func execStellarOp[IN, OUT any](w *work, op *cldfops.Operation[IN, OUT, stellardeps.StellarDeps], in IN) (OUT, error) {
	report, err := cldfops.ExecuteOperation(w.opBundle, op, w.stellarDeps(), in)
	if err != nil {
		var z OUT
		return z, err
	}
	return report.Output, nil
}
