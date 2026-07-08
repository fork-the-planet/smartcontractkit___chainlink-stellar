package txm

import (
	"context"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/smartcontractkit/chainlink-common/pkg/beholder"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/metrics"
)

var (
	promStellarTxmBroadcastedTxs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "stellar_txm_tx_broadcasted",
		Help: "Number of transactions accepted by SendTransaction",
	}, []string{"chainID"})

	promStellarTxmSuccessTxs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "stellar_txm_tx_success",
		Help: "Number of transactions confirmed with GetTransaction SUCCESS",
	}, []string{"chainID"})

	promStellarTxmPendingTxs = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "stellar_txm_tx_pending",
		Help: "Current in-flight unconfirmed transactions",
	}, []string{"chainID"})

	promStellarTxmErrorTxs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "stellar_txm_tx_error",
		Help: "Transaction errors by reason",
	}, []string{"chainID", "reason"})

	promStellarTxmRetryTxs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "stellar_txm_tx_retry",
		Help: "Transaction retries by reason",
	}, []string{"chainID", "reason"})

	promStellarTxmDroppedTxs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "stellar_txm_tx_dropped",
		Help: "Transactions dropped due to backpressure (oldest-evicted or new-rejected)",
	}, []string{"chainID", "reason"})

	promStellarTxmTimeUntilTxConfirmed = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "txm_time_until_tx_confirmed",
		Help: "The amount of time elapsed from a transaction being broadcast to being included in a ledger.",
	}, []string{"chainID"})

	// Stellar-specific metrics

	promStellarTxmRestoreTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "stellar_txm_restore_total",
		Help: "RestoreFootprint transactions submitted",
	}, []string{"chainID"})

	promStellarTxmRestoreSuccess = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "stellar_txm_restore_success",
		Help: "RestoreFootprint transactions that succeeded",
	}, []string{"chainID"})

	promStellarTxmRestoreFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "stellar_txm_restore_failed",
		Help: "RestoreFootprint transactions that failed",
	}, []string{"chainID"})

	promStellarTxmSimDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "stellar_txm_simulation_duration_seconds",
		Help:    "Time spent in SimulateTransaction calls",
		Buckets: prometheus.DefBuckets,
	}, []string{"chainID"})

	promStellarTxmFeeInclusion = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "stellar_txm_fee_inclusion_stroops",
		Help:    "Inclusion fee paid (in stroops)",
		Buckets: []float64{100, 200, 500, 1000, 5000, 10000, 50000, 100000},
	}, []string{"chainID"})

	promStellarTxmFeeResource = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "stellar_txm_fee_resource_stroops",
		Help:    "Resource fee charged (in stroops)",
		Buckets: []float64{10000, 50000, 100000, 500000, 1000000, 5000000},
	}, []string{"chainID"})
)

// Metrics is the metrics contract for the Stellar TXM transaction lifecycle.
type Metrics interface {
	IncrementBroadcastedTxs(context.Context)
	IncrementSuccessTxs(context.Context)
	SetPendingTxs(context.Context, int)
	IncrementErrorTxs(context.Context, string)
	IncrementRetryTxs(context.Context, string)
	IncrementDroppedTxs(context.Context, string)
	IncrementRestoreTotal(context.Context)
	IncrementRestoreSuccess(context.Context)
	IncrementRestoreFailed(context.Context)
	ObserveSimulationDuration(context.Context, float64)
	ObserveInclusionFee(context.Context, int64)
	ObserveResourceFee(context.Context, int64)
	RecordTimeUntilTxConfirmed(context.Context, float64)
}

type stellarTxmMetrics struct {
	metrics.Labeler
	chainID string

	// Transaction lifecycle metrics
	broadcastedTxs       metric.Int64Counter
	successTxs           metric.Int64Counter
	pendingTxs           metric.Int64Gauge
	errorTxs             metric.Int64Counter
	retryTxs             metric.Int64Counter
	droppedTxs           metric.Int64Counter
	timeUntilTxConfirmed metric.Float64Histogram

	// Stellar-specific metrics
	restoreTotal   metric.Int64Counter
	restoreSuccess metric.Int64Counter
	restoreFailed  metric.Int64Counter
	simDuration    metric.Float64Histogram
	feeInclusion   metric.Int64Histogram
	feeResource    metric.Int64Histogram
}

func NewStellarTxmMetrics(lggr logger.Logger, chainID string) Metrics {
	var initErr error
	meter := beholder.GetMeter()

	broadcastedTxs, err := meter.Int64Counter("stellar_txm_tx_broadcasted")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_tx_broadcasted: %w", err))
	}

	successTxs, err := meter.Int64Counter("stellar_txm_tx_success")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_tx_success: %w", err))
	}

	pendingTxs, err := meter.Int64Gauge("stellar_txm_tx_pending")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_tx_pending: %w", err))
	}

	errorTxs, err := meter.Int64Counter("stellar_txm_tx_error")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_tx_error: %w", err))
	}

	retryTxs, err := meter.Int64Counter("stellar_txm_tx_retry")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_tx_retry: %w", err))
	}

	droppedTxs, err := meter.Int64Counter("stellar_txm_tx_dropped")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_tx_dropped: %w", err))
	}

	timeUntilTxConfirmed, err := meter.Float64Histogram("txm_time_until_tx_confirmed")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("txm_time_until_tx_confirmed: %w", err))
	}

	restoreTotal, err := meter.Int64Counter("stellar_txm_restore_total")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_restore_total: %w", err))
	}

	restoreSuccess, err := meter.Int64Counter("stellar_txm_restore_success")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_restore_success: %w", err))
	}

	restoreFailed, err := meter.Int64Counter("stellar_txm_restore_failed")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_restore_failed: %w", err))
	}

	simDuration, err := meter.Float64Histogram("stellar_txm_simulation_duration_seconds")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_simulation_duration_seconds: %w", err))
	}

	feeInclusion, err := meter.Int64Histogram("stellar_txm_fee_inclusion_stroops")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_fee_inclusion_stroops: %w", err))
	}

	feeResource, err := meter.Int64Histogram("stellar_txm_fee_resource_stroops")
	if err != nil {
		initErr = errors.Join(initErr, fmt.Errorf("stellar_txm_fee_resource_stroops: %w", err))
	}

	if initErr != nil {
		lggr.Errorw("Failed to initialize Stellar TXM metrics; using noop metrics", "err", initErr)
		return NewNoopStellarTxmMetrics()
	}

	return &stellarTxmMetrics{
		chainID: chainID,
		Labeler: metrics.NewLabeler().With("chainID", chainID),

		broadcastedTxs:       broadcastedTxs,
		successTxs:           successTxs,
		pendingTxs:           pendingTxs,
		errorTxs:             errorTxs,
		retryTxs:             retryTxs,
		droppedTxs:           droppedTxs,
		timeUntilTxConfirmed: timeUntilTxConfirmed,

		restoreTotal:   restoreTotal,
		restoreSuccess: restoreSuccess,
		restoreFailed:  restoreFailed,
		simDuration:    simDuration,
		feeInclusion:   feeInclusion,
		feeResource:    feeResource,
	}
}

func NewNoopStellarTxmMetrics() Metrics {
	return noopStellarTxmMetrics{}
}

func (m *stellarTxmMetrics) getOtelAttributes() []attribute.KeyValue {
	return beholder.OtelAttributes(m.Labels).AsStringAttributes()
}

// --- Transaction lifecycle metrics ---

func (m *stellarTxmMetrics) IncrementBroadcastedTxs(ctx context.Context) {
	promStellarTxmBroadcastedTxs.WithLabelValues(m.chainID).Add(1)
	m.broadcastedTxs.Add(ctx, 1, metric.WithAttributes(m.getOtelAttributes()...))
}

func (m *stellarTxmMetrics) IncrementSuccessTxs(ctx context.Context) {
	promStellarTxmSuccessTxs.WithLabelValues(m.chainID).Add(1)
	m.successTxs.Add(ctx, 1, metric.WithAttributes(m.getOtelAttributes()...))
}

func (m *stellarTxmMetrics) SetPendingTxs(ctx context.Context, count int) {
	promStellarTxmPendingTxs.WithLabelValues(m.chainID).Set(float64(count))
	m.pendingTxs.Record(ctx, int64(count), metric.WithAttributes(m.getOtelAttributes()...))
}

func (m *stellarTxmMetrics) IncrementErrorTxs(ctx context.Context, reason string) {
	promStellarTxmErrorTxs.WithLabelValues(m.chainID, reason).Add(1)
	otelAttrs := append(m.getOtelAttributes(), attribute.String("reason", reason))
	m.errorTxs.Add(ctx, 1, metric.WithAttributes(otelAttrs...))
}

func (m *stellarTxmMetrics) IncrementRetryTxs(ctx context.Context, reason string) {
	promStellarTxmRetryTxs.WithLabelValues(m.chainID, reason).Add(1)
	otelAttrs := append(m.getOtelAttributes(), attribute.String("reason", reason))
	m.retryTxs.Add(ctx, 1, metric.WithAttributes(otelAttrs...))
}

func (m *stellarTxmMetrics) IncrementDroppedTxs(ctx context.Context, reason string) {
	promStellarTxmDroppedTxs.WithLabelValues(m.chainID, reason).Add(1)
	otelAttrs := append(m.getOtelAttributes(), attribute.String("reason", reason))
	m.droppedTxs.Add(ctx, 1, metric.WithAttributes(otelAttrs...))
}

func (m *stellarTxmMetrics) RecordTimeUntilTxConfirmed(ctx context.Context, duration float64) {
	promStellarTxmTimeUntilTxConfirmed.WithLabelValues(m.chainID).Observe(duration)
	m.timeUntilTxConfirmed.Record(ctx, duration, metric.WithAttributes(m.getOtelAttributes()...))
}

// --- Stellar-specific metrics ---

func (m *stellarTxmMetrics) IncrementRestoreTotal(ctx context.Context) {
	promStellarTxmRestoreTotal.WithLabelValues(m.chainID).Add(1)
	m.restoreTotal.Add(ctx, 1, metric.WithAttributes(m.getOtelAttributes()...))
}

func (m *stellarTxmMetrics) IncrementRestoreSuccess(ctx context.Context) {
	promStellarTxmRestoreSuccess.WithLabelValues(m.chainID).Add(1)
	m.restoreSuccess.Add(ctx, 1, metric.WithAttributes(m.getOtelAttributes()...))
}

func (m *stellarTxmMetrics) IncrementRestoreFailed(ctx context.Context) {
	promStellarTxmRestoreFailed.WithLabelValues(m.chainID).Add(1)
	m.restoreFailed.Add(ctx, 1, metric.WithAttributes(m.getOtelAttributes()...))
}

func (m *stellarTxmMetrics) ObserveSimulationDuration(ctx context.Context, seconds float64) {
	promStellarTxmSimDuration.WithLabelValues(m.chainID).Observe(seconds)
	m.simDuration.Record(ctx, seconds, metric.WithAttributes(m.getOtelAttributes()...))
}

func (m *stellarTxmMetrics) ObserveInclusionFee(ctx context.Context, stroops int64) {
	promStellarTxmFeeInclusion.WithLabelValues(m.chainID).Observe(float64(stroops))
	m.feeInclusion.Record(ctx, stroops, metric.WithAttributes(m.getOtelAttributes()...))
}

func (m *stellarTxmMetrics) ObserveResourceFee(ctx context.Context, stroops int64) {
	promStellarTxmFeeResource.WithLabelValues(m.chainID).Observe(float64(stroops))
	m.feeResource.Record(ctx, stroops, metric.WithAttributes(m.getOtelAttributes()...))
}

type noopStellarTxmMetrics struct{}

func (noopStellarTxmMetrics) IncrementBroadcastedTxs(context.Context) {}

func (noopStellarTxmMetrics) IncrementSuccessTxs(context.Context) {}

func (noopStellarTxmMetrics) SetPendingTxs(context.Context, int) {}

func (noopStellarTxmMetrics) IncrementErrorTxs(context.Context, string) {}

func (noopStellarTxmMetrics) IncrementRetryTxs(context.Context, string) {}

func (noopStellarTxmMetrics) IncrementDroppedTxs(context.Context, string) {}

func (noopStellarTxmMetrics) IncrementRestoreTotal(context.Context) {}

func (noopStellarTxmMetrics) IncrementRestoreSuccess(context.Context) {}

func (noopStellarTxmMetrics) IncrementRestoreFailed(context.Context) {}

func (noopStellarTxmMetrics) ObserveSimulationDuration(context.Context, float64) {}

func (noopStellarTxmMetrics) ObserveInclusionFee(context.Context, int64) {}

func (noopStellarTxmMetrics) ObserveResourceFee(context.Context, int64) {}

func (noopStellarTxmMetrics) RecordTimeUntilTxConfirmed(context.Context, float64) {}
