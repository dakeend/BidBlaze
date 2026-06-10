package outbox

import (
	"sync/atomic"
	"time"
)

type Metrics struct {
	pending          atomic.Int64
	published        atomic.Int64
	failed           atomic.Int64
	deadLetter       atomic.Int64
	latencyCount     atomic.Int64
	latencyTotalNano atomic.Int64
	latencyMaxNano   atomic.Int64
}

type MetricsSnapshot struct {
	OutboxPendingTotal int64
	PublishedTotal     int64
	PublishFailedTotal int64
	DeadLetterTotal    int64
	PublishLatencyAvg  time.Duration
	PublishLatencyMax  time.Duration
}

func (m *Metrics) SetPending(value int64) {
	m.pending.Store(value)
}

func (m *Metrics) Observe(result ProcessResult) {
	if result.PublishLatency > 0 {
		m.latencyCount.Add(1)
		m.latencyTotalNano.Add(result.PublishLatency.Nanoseconds())
		updateMax(&m.latencyMaxNano, result.PublishLatency.Nanoseconds())
	}
	switch result.Outcome {
	case OutcomePublished:
		m.published.Add(1)
	case OutcomeRetry:
		m.failed.Add(1)
	case OutcomeDeadLetter:
		m.failed.Add(1)
		m.deadLetter.Add(1)
	}
}

func (m *Metrics) Snapshot() MetricsSnapshot {
	count := m.latencyCount.Load()
	var average time.Duration
	if count > 0 {
		average = time.Duration(m.latencyTotalNano.Load() / count)
	}
	return MetricsSnapshot{
		OutboxPendingTotal: m.pending.Load(),
		PublishedTotal:     m.published.Load(),
		PublishFailedTotal: m.failed.Load(),
		DeadLetterTotal:    m.deadLetter.Load(),
		PublishLatencyAvg:  average,
		PublishLatencyMax:  time.Duration(m.latencyMaxNano.Load()),
	}
}

func updateMax(target *atomic.Int64, value int64) {
	for {
		current := target.Load()
		if value <= current || target.CompareAndSwap(current, value) {
			return
		}
	}
}
