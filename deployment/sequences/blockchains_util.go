package sequences

import (
	"sort"

	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
)

// allSelectorsFromBlockChains returns sorted selectors from [cldf_chain.BlockChains.All]
// (same ordering contract as ccv/chain.selectorsFromBlockChains).
func allSelectorsFromBlockChains(chains cldf_chain.BlockChains) []uint64 {
	selectors := make([]uint64, 0)
	for sel := range chains.All() {
		selectors = append(selectors, sel)
	}
	sort.Slice(selectors, func(i, j int) bool {
		return selectors[i] < selectors[j]
	})
	return selectors
}
