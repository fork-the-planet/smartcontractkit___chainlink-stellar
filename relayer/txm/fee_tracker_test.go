package txm

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	protocolrpc "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingFeeStatsClient struct {
	calls atomic.Int32
	resp  protocolrpc.GetFeeStatsResponse
	err   error
}

func (c *countingFeeStatsClient) GetFeeStats(_ context.Context) (protocolrpc.GetFeeStatsResponse, error) {
	c.calls.Add(1)
	return c.resp, c.err
}

func TestFeeTracker_UsesTTLBetweenCalls(t *testing.T) {
	t.Parallel()
	client := &countingFeeStatsClient{
		resp: protocolrpc.GetFeeStatsResponse{
			SorobanInclusionFee: protocolrpc.FeeDistribution{
				P50: 100,
				P90: 200,
			},
		},
	}
	tr := newFeeTracker(5 * time.Second)
	ctx := context.Background()

	p50, p90, ok, err := tr.sorobanInclusionPercentiles(ctx, client)
	require.True(t, ok)
	require.NoError(t, err)
	assert.Equal(t, uint64(100), p50)
	assert.Equal(t, uint64(200), p90)
	assert.Equal(t, int32(1), client.calls.Load())

	p50, p90, ok, err = tr.sorobanInclusionPercentiles(ctx, client)
	require.True(t, ok)
	require.NoError(t, err)
	assert.Equal(t, uint64(100), p50)
	assert.Equal(t, uint64(200), p90)
	assert.Equal(t, int32(1), client.calls.Load(), "second call within TTL must not hit RPC")
}

func TestFeeTracker_ZeroPollIntervalAlwaysFetches(t *testing.T) {
	t.Parallel()
	client := &countingFeeStatsClient{
		resp: protocolrpc.GetFeeStatsResponse{
			SorobanInclusionFee: protocolrpc.FeeDistribution{P50: 1, P90: 2},
		},
	}
	tr := newFeeTracker(0)
	ctx := context.Background()

	_, _, ok, err := tr.sorobanInclusionPercentiles(ctx, client)
	require.True(t, ok)
	require.NoError(t, err)
	_, _, ok, err = tr.sorobanInclusionPercentiles(ctx, client)
	require.True(t, ok)
	require.NoError(t, err)
	assert.Equal(t, int32(2), client.calls.Load())
}

func TestFeeTracker_StaleWhileRevalidate(t *testing.T) {
	t.Parallel()
	client := &countingFeeStatsClient{
		resp: protocolrpc.GetFeeStatsResponse{
			SorobanInclusionFee: protocolrpc.FeeDistribution{P50: 10, P90: 20},
		},
	}
	tr := newFeeTracker(10 * time.Millisecond)
	ctx := context.Background()

	_, _, ok, err := tr.sorobanInclusionPercentiles(ctx, client)
	require.True(t, ok)
	require.NoError(t, err)
	require.Equal(t, int32(1), client.calls.Load())

	time.Sleep(15 * time.Millisecond)
	client.err = assert.AnError
	client.resp = protocolrpc.GetFeeStatsResponse{}

	p50, p90, ok, err := tr.sorobanInclusionPercentiles(ctx, client)
	require.True(t, ok, "should serve stale data when refresh fails")
	require.NoError(t, err)
	assert.Equal(t, uint64(10), p50)
	assert.Equal(t, uint64(20), p90)
	assert.Equal(t, int32(2), client.calls.Load())
}

func TestFeeTracker_FirstFetchErrorNoData(t *testing.T) {
	t.Parallel()
	client := &countingFeeStatsClient{err: assert.AnError}
	tr := newFeeTracker(time.Hour)
	ctx := context.Background()

	_, _, ok, err := tr.sorobanInclusionPercentiles(ctx, client)
	assert.False(t, ok)
	assert.Error(t, err)
	assert.Equal(t, int32(1), client.calls.Load())
}
