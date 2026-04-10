package stellarutil

import (
	"sort"
)

// FilterRemoteSelectors returns selectors other than 0 and localSelector, deduplicated and sorted.
func FilterRemoteSelectors(selectors []uint64, localSelector uint64) []uint64 {
	remote := make([]uint64, 0, len(selectors))
	seen := make(map[uint64]struct{}, len(selectors))
	for _, selector := range selectors {
		if selector == 0 || selector == localSelector {
			continue
		}
		if _, ok := seen[selector]; ok {
			continue
		}
		seen[selector] = struct{}{}
		remote = append(remote, selector)
	}
	sort.Slice(remote, func(i, j int) bool {
		return remote[i] < remote[j]
	})
	return remote
}
