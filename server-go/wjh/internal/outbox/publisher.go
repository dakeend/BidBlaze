package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Publisher struct {
	store          Store
	sink           Sink
	metrics        *Metrics
	logger         *slog.Logger
	pollInterval   time.Duration
	batchSize      int
	publishTimeout time.Duration
}

func NewPublisher(
	store Store,
	sink Sink,
	metrics *Metrics,
	logger *slog.Logger,
	pollInterval time.Duration,
	batchSize int,
	publishTimeout time.Duration,
) *Publisher {
	if pollInterval <= 0 {
		pollInterval = 200 * time.Millisecond
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if publishTimeout <= 0 {
		publishTimeout = time.Second
	}
	if metrics == nil {
		metrics = &Metrics{}
	}
	return &Publisher{
		store:          store,
		sink:           sink,
		metrics:        metrics,
		logger:         logger,
		pollInterval:   pollInterval,
		batchSize:      batchSize,
		publishTimeout: publishTimeout,
	}
}

func (p *Publisher) Run(ctx context.Context) {
	for {
		result, err := p.ProcessOnce(ctx)
		if err != nil {
			p.logger.Error("outbox publish cycle failed", "error", err)
		} else if result.Published > 0 || result.Retried > 0 || result.DeadLetter > 0 {
			snapshot := p.metrics.Snapshot()
			p.logger.Info(
				"outbox publish cycle completed",
				"published", result.Published,
				"retried", result.Retried,
				"dead_letter", result.DeadLetter,
				"outbox_pending_total", snapshot.OutboxPendingTotal,
				"publish_latency_ms", snapshot.PublishLatencyAvg.Milliseconds(),
				"publish_latency_max_ms", snapshot.PublishLatencyMax.Milliseconds(),
			)
		}

		timer := time.NewTimer(p.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (p *Publisher) ProcessOnce(ctx context.Context) (BatchResult, error) {
	var batch BatchResult
	for index := 0; index < p.batchSize; index++ {
		result, found, err := p.store.ProcessNext(ctx, p.publish)
		if err != nil {
			return batch, fmt.Errorf("process outbox event: %w", err)
		}
		if !found {
			break
		}

		p.metrics.Observe(result)
		switch result.Outcome {
		case OutcomePublished:
			batch.Published++
		case OutcomeRetry:
			batch.Retried++
			p.logPublishFailure(result, false)
		case OutcomeDeadLetter:
			batch.DeadLetter++
			p.logPublishFailure(result, true)
		}
	}

	pending, err := p.store.PendingCount(ctx)
	if err != nil {
		return batch, fmt.Errorf("count pending outbox events: %w", err)
	}
	p.metrics.SetPending(pending)
	return batch, nil
}

func (p *Publisher) Metrics() MetricsSnapshot {
	return p.metrics.Snapshot()
}

func (p *Publisher) publish(ctx context.Context, event Event) error {
	publishCtx, cancel := context.WithTimeout(ctx, p.publishTimeout)
	defer cancel()
	return p.sink.Publish(publishCtx, event)
}

func (p *Publisher) logPublishFailure(result ProcessResult, deadLetter bool) {
	p.logger.Warn(
		"outbox event publish failed",
		"event_id", result.Event.ID,
		"auction_id", result.Event.AggregateID,
		"event_seq", result.Event.EventSeq,
		"retry_count", result.Event.RetryCount+1,
		"dead_letter", deadLetter,
		"error", result.PublishError,
	)
}
