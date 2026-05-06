package sequences

import (
	"context"
	"testing"

	chainsel "github.com/smartcontractkit/chain-selectors"
	seq_core "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	ccvdeployment "github.com/smartcontractkit/chainlink-ccv/deployment"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_stellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
	"github.com/stretchr/testify/require"

	protocolrpc "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestDeployStellarCCIPInnerSequenceID(t *testing.T) {
	t.Parallel()
	require.Equal(t, "deploy-stellar-ccip-inner", DeployStellarCCIPInner.ID())
}

func newTestBundle(t *testing.T) cldf_ops.Bundle {
	t.Helper()
	return cldf_ops.NewBundle(
		func() context.Context { return t.Context() },
		cldflogger.Nop(),
		cldf_ops.NewMemoryReporter(),
	)
}

type stubDeployer struct{}

func (stubDeployer) DeployContract(ctx context.Context, wasmPath string, salt [32]byte) (string, error) {
	_ = ctx
	_ = wasmPath
	_ = salt
	return "", nil
}

type stubInvoker struct{}

func (stubInvoker) InvokeContract(ctx context.Context, contractID string, functionName string, args []xdr.ScVal) (*xdr.ScVal, error) {
	_ = ctx
	_ = contractID
	_ = functionName
	_ = args
	return nil, nil
}

func (stubInvoker) SimulateContract(ctx context.Context, contractID string, functionName string, args []xdr.ScVal) (*xdr.ScVal, error) {
	_ = ctx
	_ = contractID
	_ = functionName
	_ = args
	return nil, nil
}

func (stubInvoker) GetEvents(ctx context.Context, contractID string, startLedger uint32, topics []string) ([]protocolrpc.EventInfo, error) {
	_ = ctx
	_ = contractID
	_ = startLedger
	_ = topics
	return nil, nil
}

type stubStellarDeployRunner struct {
	lastOpBundle cldf_ops.Bundle
}

func (s *stubStellarDeployRunner) DeployStellarCCIPContracts(
	ctx context.Context,
	opBundle cldf_ops.Bundle,
	allSelectors []uint64,
	selector uint64,
	topology *ccvdeployment.EnvironmentTopology,
	existingAddresses []datastore.AddressRef,
) (seq_core.OnChainOutput, error) {
	_ = ctx
	_ = allSelectors
	_ = topology
	_ = existingAddresses
	_ = selector
	s.lastOpBundle = opBundle
	return seq_core.OnChainOutput{}, nil
}

func (s *stubStellarDeployRunner) StellarDepsForDeploy() stellardeps.StellarDeps {
	return stellardeps.StellarDeps{
		Deploy:  stubDeployer{},
		Invoker: stubInvoker{},
	}
}

func TestStellarDeployChainContracts_RejectsMissingStellarChain(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	sel := chainsel.STELLAR_LOCALNET.Selector
	chains := cldf_chain.NewBlockChains(nil)
	_, err := cldf_ops.ExecuteSequence(b, StellarDeployChainContracts, chains, ccvadapters.DeployChainContractsInput{
		ChainSelector: sel,
	})
	require.Error(t, err)
}

func TestStellarDeployChainContracts_RejectsMissingDeployContext(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	// Unique selector so parallel tests that register STELLAR_LOCALNET deploy context do not mask this case.
	sel := uint64(424242420001)
	st := cldf_stellar.Chain{ChainMetadata: cldf_stellar.ChainMetadata{Selector: sel}}
	chains := cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{sel: st})

	_, err := cldf_ops.ExecuteSequence(b, StellarDeployChainContracts, chains, ccvadapters.DeployChainContractsInput{
		ChainSelector: sel,
	})
	require.Error(t, err)
}

func TestDeployStellarCCIPInner_ForwardsParentOperationsBundle(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	sel := chainsel.STELLAR_LOCALNET.Selector
	runner := &stubStellarDeployRunner{}
	RegisterStellarDeployChainContext(sel, runner, nil)
	t.Cleanup(func() { ClearStellarDeployChainContext(sel) })

	_, err := cldf_ops.ExecuteSequence(b, DeployStellarCCIPInner, runner.StellarDepsForDeploy(), DeployStellarCCIPInnerInput{
		ChainSelector: sel,
		AllSelectors:  []uint64{sel, 123},
	})
	require.NoError(t, err)
	require.NotNil(t, runner.lastOpBundle.GetContext, "inner sequence should pass the parent CLDF bundle into DeployStellarCCIPContracts (EVM-style)")
}
