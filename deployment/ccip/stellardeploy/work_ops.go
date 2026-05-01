package stellardeploy

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

func (w *deployRun) stellarDeps() stellardeps.StellarDeps {
	return stellardeps.FromDeployer(w.host.Deployer())
}

func execStellarOp[IN, OUT any](w *deployRun, op *cldfops.Operation[IN, OUT, stellardeps.StellarDeps], in IN) (OUT, error) {
	report, err := cldfops.ExecuteOperation(w.opBundle, op, w.stellarDeps(), in)
	if err != nil {
		var z OUT
		return z, err
	}
	return report.Output, nil
}
