package bid

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Store interface {
	FindIdempotency(ctx context.Context, userID int64, scope string, key string) (IdempotencyRecord, bool, error)
	GetAuction(ctx context.Context, auctionID int64) (AuctionSnapshot, bool, error)
	WithTx(ctx context.Context, fn func(TxStore) error) error
}

type TxStore interface {
	ClaimIdempotency(ctx context.Context, userID int64, scope string, key string, requestHash string) (IdempotencyRecord, error)
	LockAuction(ctx context.Context, auctionID int64) (AuctionSnapshot, bool, error)
	ConditionalAccept(ctx context.Context, params AcceptParams) (int64, error)
	InsertAcceptedBid(ctx context.Context, auctionID int64, userID int64, amount int64, key string) (int64, time.Time, error)
	InsertOrder(ctx context.Context, auctionID int64, winnerID int64, sellerID int64, finalPrice int64) (int64, error)
	InsertOutbox(ctx context.Context, auctionID int64, event StoredEvent) error
	FinalizeIdempotency(ctx context.Context, userID int64, scope string, key string, responseJSON []byte) error
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindIdempotency(ctx context.Context, userID int64, scope string, key string) (IdempotencyRecord, bool, error) {
	const query = `
SELECT request_hash, response_json, status
  FROM idempotency_keys
 WHERE user_id = ?
   AND scope = ?
   AND idempotency_key = ?
 LIMIT 1`
	var record IdempotencyRecord
	var response []byte
	err := r.db.QueryRowContext(ctx, query, userID, scope, key).Scan(
		&record.RequestHash, &response, &record.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return IdempotencyRecord{}, false, nil
	}
	if err != nil {
		return IdempotencyRecord{}, false, err
	}
	record.ResponseJSON = response
	return record, true, nil
}

func (r *Repository) GetAuction(ctx context.Context, auctionID int64) (AuctionSnapshot, bool, error) {
	const query = `
SELECT id, seller_id, status, start_price, price_step, ceiling_price,
       current_price, current_leader_id, bid_count, end_time,
       extend_seconds, extend_threshold, version
  FROM auctions
 WHERE id = ?
 LIMIT 1`
	return scanAuction(r.db.QueryRowContext(ctx, query, auctionID))
}

func (r *Repository) WithTx(ctx context.Context, fn func(TxStore) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(&txRepository{tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

type txRepository struct {
	tx *sql.Tx
}

func (r *txRepository) ClaimIdempotency(
	ctx context.Context,
	userID int64,
	scope string,
	key string,
	requestHash string,
) (IdempotencyRecord, error) {
	const insertSQL = `
INSERT INTO idempotency_keys (
  user_id, scope, idempotency_key, request_hash, response_json, status
) VALUES (?, ?, ?, ?, NULL, 'processing')
ON DUPLICATE KEY UPDATE id = id`
	if _, err := r.tx.ExecContext(ctx, insertSQL, userID, scope, key, requestHash); err != nil {
		return IdempotencyRecord{}, err
	}

	const lockSQL = `
SELECT request_hash, response_json, status
  FROM idempotency_keys
 WHERE user_id = ?
   AND scope = ?
   AND idempotency_key = ?
 FOR UPDATE`
	var record IdempotencyRecord
	var response []byte
	if err := r.tx.QueryRowContext(ctx, lockSQL, userID, scope, key).Scan(
		&record.RequestHash, &response, &record.Status,
	); err != nil {
		return IdempotencyRecord{}, err
	}
	record.ResponseJSON = response
	return record, nil
}

func (r *txRepository) LockAuction(ctx context.Context, auctionID int64) (AuctionSnapshot, bool, error) {
	const query = `
SELECT id, seller_id, status, start_price, price_step, ceiling_price,
       current_price, current_leader_id, bid_count, end_time,
       extend_seconds, extend_threshold, version
  FROM auctions
 WHERE id = ?
 FOR UPDATE`
	return scanAuction(r.tx.QueryRowContext(ctx, query, auctionID))
}

func (r *txRepository) ConditionalAccept(ctx context.Context, params AcceptParams) (int64, error) {
	const query = `
UPDATE auctions
   SET current_price = ?,
       current_leader_id = ?,
       end_time = ?,
       status = CASE WHEN ? THEN 'ended' ELSE status END,
       version = version + ?,
       bid_count = bid_count + 1,
       updated_at = CURRENT_TIMESTAMP(3)
 WHERE id = ?
   AND status = 'active'
   AND end_time > CURRENT_TIMESTAMP(3)
   AND (
     (bid_count = 0 AND start_price > 0 AND ? >= start_price)
     OR (bid_count = 0 AND start_price = 0 AND ? >= price_step)
     OR (bid_count > 0 AND current_price + price_step <= ?)
   )
   AND (ceiling_price IS NULL OR ? <= ceiling_price)`
	result, err := r.tx.ExecContext(ctx, query,
		params.Amount,
		params.UserID,
		params.NewEndTime,
		params.CeilingHit,
		params.EventCount,
		params.AuctionID,
		params.Amount,
		params.Amount,
		params.Amount,
		params.Amount,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *txRepository) InsertAcceptedBid(
	ctx context.Context,
	auctionID int64,
	userID int64,
	amount int64,
	key string,
) (int64, time.Time, error) {
	const query = `
INSERT INTO bids (
  auction_id, user_id, amount, status, reject_reason, idempotency_key
) VALUES (?, ?, ?, 'accepted', NULL, ?)`
	result, err := r.tx.ExecContext(ctx, query, auctionID, userID, amount, key)
	if err != nil {
		return 0, time.Time{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, time.Time{}, err
	}

	const createdSQL = `SELECT created_at FROM bids WHERE id = ?`
	var createdAt time.Time
	if err := r.tx.QueryRowContext(ctx, createdSQL, id).Scan(&createdAt); err != nil {
		return 0, time.Time{}, err
	}
	return id, createdAt, nil
}

func (r *txRepository) InsertOrder(
	ctx context.Context,
	auctionID int64,
	winnerID int64,
	sellerID int64,
	finalPrice int64,
) (int64, error) {
	const query = `
INSERT INTO orders (
  auction_id, winner_id, seller_id, final_price, status
) VALUES (?, ?, ?, ?, 'pending_pay')`
	result, err := r.tx.ExecContext(ctx, query, auctionID, winnerID, sellerID, finalPrice)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *txRepository) InsertOutbox(ctx context.Context, auctionID int64, event StoredEvent) error {
	const query = `
INSERT INTO event_outbox (
  aggregate_type, aggregate_id, event_type, event_seq, payload, status
) VALUES ('auction', ?, ?, ?, ?, 'pending')`
	_, err := r.tx.ExecContext(ctx, query, auctionID, event.EventType, event.EventSeq, string(event.Payload))
	return err
}

func (r *txRepository) FinalizeIdempotency(
	ctx context.Context,
	userID int64,
	scope string,
	key string,
	responseJSON []byte,
) error {
	const query = `
UPDATE idempotency_keys
   SET response_json = ?,
       status = 'succeeded',
       updated_at = CURRENT_TIMESTAMP(3)
 WHERE user_id = ?
   AND scope = ?
   AND idempotency_key = ?`
	_, err := r.tx.ExecContext(ctx, query, string(responseJSON), userID, scope, key)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAuction(row rowScanner) (AuctionSnapshot, bool, error) {
	var snapshot AuctionSnapshot
	err := row.Scan(
		&snapshot.ID,
		&snapshot.SellerID,
		&snapshot.Status,
		&snapshot.StartPrice,
		&snapshot.PriceStep,
		&snapshot.CeilingPrice,
		&snapshot.CurrentPrice,
		&snapshot.CurrentLeaderID,
		&snapshot.BidCount,
		&snapshot.EndTime,
		&snapshot.ExtendSeconds,
		&snapshot.ExtendThreshold,
		&snapshot.Version,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return AuctionSnapshot{}, false, nil
	}
	if err != nil {
		return AuctionSnapshot{}, false, err
	}
	return snapshot, true, nil
}

func encodeResult(result Result) ([]byte, error) {
	return json.Marshal(result)
}
