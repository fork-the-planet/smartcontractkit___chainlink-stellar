package sequences

import (
	"testing"

	chainsel "github.com/smartcontractkit/chain-selectors"
	api "github.com/smartcontractkit/chainlink-ccip/deployment/fastcurse"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/stretchr/testify/require"
)

func TestStellarCurse_sequenceMetadata(t *testing.T) {
	t.Parallel()
	require.Equal(t, "stellar-curse-rmn-remote", StellarCurse.ID())
	require.Equal(t, stellarops.ContractDeploymentVersion.String(), StellarCurse.Version())
}

func TestStellarUncurse_sequenceMetadata(t *testing.T) {
	t.Parallel()
	require.Equal(t, "stellar-uncurse-rmn-remote", StellarUncurse.ID())
	require.Equal(t, stellarops.ContractDeploymentVersion.String(), StellarUncurse.Version())
}

func TestStellarCurse_RejectsMissingStellarChain(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	sel := chainsel.STELLAR_LOCALNET.Selector
	chains := cldf_chain.NewBlockChains(nil)
	in := StellarCurseInput{
		CurseInput: api.CurseInput{
			ChainSelector: sel,
			Subjects:      nil,
		},
		RMNContractID: "CCONTRACTTESTCURSE0000000000000000000000",
	}
	_, err := cldf_ops.ExecuteSequence(b, StellarCurse, chains, in)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestStellarUncurse_RejectsMissingStellarChain(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	sel := chainsel.STELLAR_LOCALNET.Selector
	chains := cldf_chain.NewBlockChains(nil)
	in := StellarCurseInput{
		CurseInput: api.CurseInput{
			ChainSelector: sel,
			Subjects:      nil,
		},
		RMNContractID: "CCONTRACTTESTUNCURSE00000000000000000000",
	}
	_, err := cldf_ops.ExecuteSequence(b, StellarUncurse, chains, in)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}
