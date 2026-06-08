package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const maxRetryBackoff = 5 * time.Second

type Store interface {
	StartDue(ctx context.Context, limit int) (int, error)
	ListDueEndIDs(ctx context.Context, limit int) ([]int64, error)
	EndOne(ctx context.Context, auctionID int64) (EndResult, error)
}

type Runner struct {
	store    Store
	logger   *slog.Logger
	interval time.Duration
	batch    int
}

func NewRunner(
	store Store,
	logger *slog.Logger,
	interval time.Duration,
	batch int,
) *Runner {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	if batch <= 0 {
		batch = 100
	}
	return &Runner{
		store:    store,
		logger:   logger,
		interval: interval,
		batch:    batch,
	}
}

func (r *Runner) Run(ctx context.Context) {
	wait := time.Duration(0)
	backoff := r.interval

	for {
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}

		result, err := r.ProcessOnce(ctx)
		if err != nil {
			r.logger.Error(
				"lifecycle cycle failed",
				"error", err,
				"retry_in_ms", backoff.Milliseconds(),
			)
			wait = backoff
			backoff = minDuration(backoff*2, maxRetryBackoff)
			continue
		}

		if result.Started > 0 || result.Ended > 0 {
			r.logger.Info(
				"lifecycle cycle completed",
				"started", result.Started,
				"ended", result.Ended,
				"orders", result.Orders,
			)
		}
		backoff = r.interval
		wait = r.interval
	}
}

func (r *Runner) ProcessOnce(ctx context.Context) (CycleResult, error) {
	var cycle CycleResult
	started, err := r.store.StartDue(ctx, r.batch)
	if err != nil {
		return cycle, fmt.Errorf("start due auctions: %w", err)
	}
	cycle.Started = started

	ids, err := r.store.ListDueEndIDs(ctx, r.batch)
	if err != nil {
		return cycle, fmt.Errorf("list due auctions: %w", err)
	}

	var endErrors []error
	for _, id := range ids {
		result, err := r.store.EndOne(ctx, id)
		if err != nil {
			endErrors = append(endErrors, fmt.Errorf("end auction %d: %w", id, err))
			continue
		}
		if result.Ended {
			cycle.Ended++
		}
		if result.OrderID != nil {
			cycle.Orders++
		}
	}
	return cycle, errors.Join(endErrors...)
}

func minDuration(left time.Duration, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}
