package realtime

import "context"

const defaultReplayLimit = 100

type ReplayResult struct {
	Events           []EventEnvelope
	SnapshotRequired bool
}

type SnapshotProvider interface {
	Snapshot(ctx context.Context, auctionID int64) (EventEnvelope, error)
}

type ReplayProvider interface {
	EventsAfter(ctx context.Context, auctionID int64, afterSeq int64, limit int) (ReplayResult, error)
}

type Provider interface {
	SnapshotProvider
	ReplayProvider
}

// StaticProvider keeps the WS gateway usable before Task E is wired to MySQL.
// Replace this with a provider backed by /status and event_outbox queries.
type StaticProvider struct{}

func (StaticProvider) Snapshot(_ context.Context, auctionID int64) (EventEnvelope, error) {
	return newSnapshotEvent(auctionID, 0), nil
}

func (StaticProvider) EventsAfter(_ context.Context, _ int64, _ int64, _ int) (ReplayResult, error) {
	return ReplayResult{Events: nil, SnapshotRequired: true}, nil
}
