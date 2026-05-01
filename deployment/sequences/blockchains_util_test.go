package sequences

import (
	"testing"

	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_stellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
	"github.com/stretchr/testify/require"
)

func TestAllSelectorsFromBlockChains_sorted(t *testing.T) {
	t.Parallel()
	selA := uint64(10)
	selB := uint64(5)
	chains := cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		selA: cldf_stellar.Chain{ChainMetadata: cldf_stellar.ChainMetadata{Selector: selA}},
		selB: cldf_stellar.Chain{ChainMetadata: cldf_stellar.ChainMetadata{Selector: selB}},
	})
	got := allSelectorsFromBlockChains(chains)
	require.Equal(t, []uint64{selB, selA}, got)
}
