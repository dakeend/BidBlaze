package realtime

import (
	"context"
	"database/sql"
	"encoding/json"
)

type OutboxProvider struct {
	db                     *sql.DB
	snapshotRequiredWindow int64
}

func NewOutboxProvider(db *sql.DB) *OutboxProvider {
	return &OutboxProvider{
		db:                     db,
		snapshotRequiredWindow: defaultSnapshotRequiredWindow,
	}
}

func (p *OutboxProvider) Snapshot(ctx context.Context, auctionID int64) (EventEnvelope, error) {
	maxSeq, err := p.maxEventSeq(ctx, auctionID)
	if err != nil {
		return EventEnvelope{}, err
	}
	return newSnapshotEvent(auctionID, maxSeq), nil
}

func (p *OutboxProvider) EventsAfter(ctx context.Context, auctionID int64, afterSeq int64, limit int) (ReplayResult, error) {
	limit = normalizeReplayLimit(limit)

	minSeq, maxSeq, hasEvents, err := p.eventSeqRange(ctx, auctionID)
	if err != nil {
		return ReplayResult{}, err
	}
	if p.snapshotRequired(afterSeq, minSeq, maxSeq, hasEvents) {
		return ReplayResult{SnapshotRequired: true}, nil
	}

	rows, err := p.db.QueryContext(ctx, `
SELECT payload
  FROM event_outbox
 WHERE aggregate_type = 'auction'
   AND aggregate_id = ?
   AND event_seq > ?
   AND event_type NOT IN ('ViewerCount', 'viewer_count')
 ORDER BY event_seq ASC
 LIMIT ?`, auctionID, afterSeq, limit+1)
	if err != nil {
		return ReplayResult{}, err
	}
	defer rows.Close()

	events := make([]EventEnvelope, 0, limit)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return ReplayResult{}, err
		}

		var event EventEnvelope
		if err := json.Unmarshal(payload, &event); err != nil {
			return ReplayResult{}, err
		}
		if event.Type == EventViewerCount {
			continue
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return ReplayResult{}, err
	}

	result := ReplayResult{Events: events}
	if len(result.Events) > limit {
		result.Events = result.Events[:limit]
		result.HasMore = true
	}
	return result, nil
}

func (p *OutboxProvider) eventSeqRange(ctx context.Context, auctionID int64) (int64, int64, bool, error) {
	var minSeq, maxSeq sql.NullInt64
	err := p.db.QueryRowContext(ctx, `
SELECT MIN(event_seq), MAX(event_seq)
  FROM event_outbox
 WHERE aggregate_type = 'auction'
   AND aggregate_id = ?
   AND event_type NOT IN ('ViewerCount', 'viewer_count')`, auctionID).Scan(&minSeq, &maxSeq)
	if err != nil {
		return 0, 0, false, err
	}
	if !minSeq.Valid || !maxSeq.Valid {
		return 0, 0, false, nil
	}
	return minSeq.Int64, maxSeq.Int64, true, nil
}

func (p *OutboxProvider) maxEventSeq(ctx context.Context, auctionID int64) (int64, error) {
	var maxSeq sql.NullInt64
	err := p.db.QueryRowContext(ctx, `
SELECT MAX(event_seq)
  FROM event_outbox
 WHERE aggregate_type = 'auction'
   AND aggregate_id = ?
   AND event_type NOT IN ('ViewerCount', 'viewer_count')`, auctionID).Scan(&maxSeq)
	if err != nil {
		return 0, err
	}
	if !maxSeq.Valid {
		return 0, nil
	}
	return maxSeq.Int64, nil
}

func (p *OutboxProvider) snapshotRequired(afterSeq int64, minSeq int64, maxSeq int64, hasEvents bool) bool {
	if !hasEvents {
		return false
	}
	if maxSeq-afterSeq > p.snapshotRequiredWindow {
		return true
	}
	return afterSeq < minSeq-1
}

func normalizeReplayLimit(limit int) int {
	if limit <= 0 {
		return defaultReplayLimit
	}
	if limit > maxReplayLimit {
		return maxReplayLimit
	}
	return limit
}
