package txm

import (
	"context"
	"sync"
	"time"

	protocolrpc "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

// feeStatsRPC is the subset of RPCClient needed to refresh Soroban fee stats.
type feeStatsRPC interface {
	GetFeeStats(ctx context.Context) (protocolrpc.GetFeeStatsResponse, error)
}

// feeTracker holds P50/P90 Soroban inclusion fees from GetFeeStats and refreshes them
// at most once per pollInterval to reduce RPC load when many txs broadcast back-to-back.
// A non-positive pollInterval disables caching (every call fetches). On refresh error,
// the last successful snapshot is returned if any (stale-while-revalidate).
type feeTracker struct {
	mu           sync.Mutex
	pollInterval time.Duration

	haveData  bool
	fetchedAt time.Time
	p50, p90  uint64
}

func newFeeTracker(pollInterval time.Duration) *feeTracker {
	return &feeTracker{pollInterval: pollInterval}
}

// sorobanInclusionPercentiles returns Soroban inclusion fee P50 and P90 (stroops).
// On error it returns (0, 0, err) when there is no usable cached data yet; otherwise
// it returns the last successful snapshot with err == nil (stale-while-revalidate).
func (t *feeTracker) sorobanInclusionPercentiles(ctx context.Context, client feeStatsRPC) (p50, p90 uint64, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if t.pollInterval > 0 && t.haveData && now.Sub(t.fetchedAt) < t.pollInterval {
		return t.p50, t.p90, nil
	}

	resp, fetchErr := client.GetFeeStats(ctx)
	if fetchErr != nil {
		if t.haveData {
			return t.p50, t.p90, nil
		}
		return 0, 0, fetchErr
	}

	t.p50 = resp.SorobanInclusionFee.P50
	t.p90 = resp.SorobanInclusionFee.P90
	t.fetchedAt = now
	t.haveData = true
	return t.p50, t.p90, nil
}
