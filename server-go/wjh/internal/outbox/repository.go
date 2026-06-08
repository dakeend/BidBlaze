package outbox

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type Store interface {
	ProcessNext(
		ctx context.Context,
		publish func(context.Context, Event) error,
	) (ProcessResult, bool, error)
	PendingCount(ctx context.Context) (int64, error)
}

type Repository struct {
	db         *sql.DB
	maxRetries int
	retryBase  time.Duration
	retryMax   time.Duration
}

func NewRepository(
	db *sql.DB,
	maxRetries int,
	retryBase time.Duration,
	retryMax time.Duration,
) *Repository {
	if maxRetries <= 0 {
		maxRetries = 10
	}
	if retryBase <= 0 {
		retryBase = 200 * time.Millisecond
	}
	if retryMax < retryBase {
		retryMax = 5 * time.Second
	}
	return &Repository{
		db:         db,
		maxRetries: maxRetries,
		retryBase:  retryBase,
		retryMax:   retryMax,
	}
}

func (r *Repository) ProcessNext(
	ctx context.Context,
	publish func(context.Context, Event) error,
) (ProcessResult, bool, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return ProcessResult{}, false, err
	}
	defer tx.Rollback()

	event, found, err := r.lockNext(ctx, tx)
	if err != nil || !found {
		return ProcessResult{}, found, err
	}

	startedAt := time.Now()
	publishErr := publish(ctx, event)
	latency := time.Since(startedAt)

	result := ProcessResult{
		Event:          event,
		PublishLatency: latency,
		PublishError:   publishErr,
	}
	if publishErr == nil {
		if err := markPublished(ctx, tx, event.ID); err != nil {
			return ProcessResult{}, true, err
		}
		result.Outcome = OutcomePublished
	} else {
		outcome, err := r.markFailure(ctx, tx, event, publishErr)
		if err != nil {
			return ProcessResult{}, true, err
		}
		result.Outcome = outcome
	}

	if err := tx.Commit(); err != nil {
		return ProcessResult{}, true, err
	}
	return result, true, nil
}

func (r *Repository) lockNext(
	ctx context.Context,
	tx *sql.Tx,
) (Event, bool, error) {
	const query = `
SELECT o.id, o.aggregate_type, o.aggregate_id, o.event_type,
       o.event_seq, CAST(o.payload AS CHAR), o.retry_count, o.created_at
 FROM event_outbox o
 WHERE o.status = 'pending'
   AND (
     o.retry_count = 0
     OR o.updated_at <= TIMESTAMPADD(
          MICROSECOND,
          -LEAST(?, ? * POW(2, o.retry_count - 1)),
          CURRENT_TIMESTAMP(3)
        )
   )
   AND NOT EXISTS (
     SELECT 1
       FROM event_outbox previous
      WHERE previous.aggregate_type = o.aggregate_type
        AND previous.aggregate_id = o.aggregate_id
        AND previous.event_seq < o.event_seq
        AND previous.status = 'pending'
   )
 ORDER BY o.created_at, o.id
 LIMIT 1
 FOR UPDATE SKIP LOCKED`
	var event Event
	var payload []byte
	err := tx.QueryRowContext(
		ctx,
		query,
		r.retryMax.Microseconds(),
		r.retryBase.Microseconds(),
	).Scan(
		&event.ID,
		&event.AggregateType,
		&event.AggregateID,
		&event.EventType,
		&event.EventSeq,
		&payload,
		&event.RetryCount,
		&event.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, err
	}
	event.Payload = append(event.Payload[:0], payload...)
	return event, true, nil
}

func markPublished(ctx context.Context, tx *sql.Tx, eventID int64) error {
	const query = `
UPDATE event_outbox
   SET status = 'published',
       published_at = CURRENT_TIMESTAMP(3),
       last_error = NULL,
       updated_at = CURRENT_TIMESTAMP(3)
 WHERE id = ?
   AND status = 'pending'`
	result, err := tx.ExecContext(ctx, query, eventID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrStateConflict
	}
	return nil
}

func (r *Repository) markFailure(
	ctx context.Context,
	tx *sql.Tx,
	event Event,
	publishErr error,
) (Outcome, error) {
	nextRetry := event.RetryCount + 1
	status := "pending"
	outcome := OutcomeRetry
	if nextRetry >= r.maxRetries {
		status = "failed"
		outcome = OutcomeDeadLetter
	}

	const query = `
UPDATE event_outbox
   SET status = ?,
       retry_count = ?,
       last_error = ?,
       updated_at = CURRENT_TIMESTAMP(3)
 WHERE id = ?
   AND status = 'pending'`
	result, err := tx.ExecContext(
		ctx,
		query,
		status,
		nextRetry,
		truncateError(publishErr),
		event.ID,
	)
	if err != nil {
		return "", err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if affected != 1 {
		return "", ErrStateConflict
	}
	return outcome, nil
}

func (r *Repository) PendingCount(ctx context.Context) (int64, error) {
	const query = `
SELECT COUNT(*)
  FROM event_outbox
 WHERE status = 'pending'`
	var count int64
	if err := r.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func truncateError(err error) string {
	message := strings.TrimSpace(err.Error())
	if len(message) <= 512 {
		return message
	}
	return message[:512]
}

var ErrStateConflict = errors.New("outbox state changed while publishing")
