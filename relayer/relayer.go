package relayer

import (
	"context"
	"errors"
	"math/big"

	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/loop"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	relaytypes "github.com/smartcontractkit/chainlink-common/pkg/types"
	"github.com/smartcontractkit/chainlink-stellar/relayer/chain"
)

var _ loop.Relayer = (*Relayer)(nil)
var _ relaytypes.StellarService = (*Relayer)(nil)

// Relayer wraps the Stellar chain and exposes it as loop.Relayer.
type Relayer struct {
	relaytypes.UnimplementedRelayer

	ch      chain.Chain
	lggr    logger.Logger
	starter services.StateMachine
	stopCh  services.StopChan

	stellarService
}

// NewRelayer constructs a Relayer from an already-built chain.
func NewRelayer(lggr logger.Logger, ch chain.Chain) *Relayer {
	lggr = logger.Named(lggr, "StellarRelayer")
	return &Relayer{
		ch:             ch,
		lggr:           lggr,
		stopCh:         make(chan struct{}),
		stellarService: newStellarService(ch),
	}
}

func (r *Relayer) Name() string { return r.lggr.Name() }

func (r *Relayer) Start(ctx context.Context) error {
	return r.starter.StartOnce("StellarRelayer", func() error {
		r.lggr.Debugw("Starting")
		if r.ch == nil {
			return errors.New("stellar chain unavailable")
		}
		var ms services.MultiStart
		return ms.Start(ctx, r.ch)
	})
}

func (r *Relayer) Close() error {
	return r.starter.StopOnce("StellarRelayer", func() error {
		r.lggr.Debugw("Stopping")
		close(r.stopCh)
		return services.CloseAll(r.ch)
	})
}

func (r *Relayer) Ready() error { return r.starter.Ready() }

func (r *Relayer) Healthy() error { return r.starter.Healthy() }

func (r *Relayer) HealthReport() map[string]error {
	report := map[string]error{r.Name(): r.starter.Healthy()}
	services.CopyHealth(report, r.ch.HealthReport())
	return report
}

func (r *Relayer) GetChainInfo(ctx context.Context) (relaytypes.ChainInfo, error) {
	return r.ch.GetChainInfo(ctx)
}

func (r *Relayer) GetChainStatus(ctx context.Context) (relaytypes.ChainStatus, error) {
	return r.ch.GetChainStatus(ctx)
}

func (r *Relayer) LatestHead(ctx context.Context) (relaytypes.Head, error) {
	return r.ch.LatestHead(ctx)
}

func (r *Relayer) FinalizedHead(ctx context.Context) (relaytypes.Head, error) {
	return r.ch.FinalizedHead(ctx)
}

func (r *Relayer) ListNodeStatuses(ctx context.Context, pageSize int32, pageToken string) ([]relaytypes.NodeStatus, string, int, error) {
	return r.ch.ListNodeStatuses(ctx, pageSize, pageToken)
}

func (r *Relayer) Transact(ctx context.Context, from, to string, amount *big.Int, balanceCheck bool) error {
	return r.ch.Transact(ctx, from, to, amount, balanceCheck)
}

func (r *Relayer) Stellar() (relaytypes.StellarService, error) {
	return &r.stellarService, nil
}
