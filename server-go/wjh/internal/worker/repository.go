package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Repository struct {
	db       *sql.DB
	location *time.Location
}

func NewRepository(db *sql.DB, location *time.Location) *Repository {
	return &Repository{db: db, location: location}
}

func (r *Repository) StartDue(ctx context.Context, limit int) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	const selectSQL = `
SELECT id, title, seller_id, status, current_price,
       current_leader_id, end_time, version
  FROM auctions
 WHERE status = 'pending'
   AND start_time <= CURRENT_TIMESTAMP(3)
 ORDER BY start_time, id
 LIMIT ?
 FOR UPDATE SKIP LOCKED`
	rows, err := tx.QueryContext(ctx, selectSQL, limit)
	if err != nil {
		return 0, err
	}

	candidates := make([]lifecycleAuction, 0, limit)
	for rows.Next() {
		var item lifecycleAuction
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.SellerID,
			&item.Status,
			&item.CurrentPrice,
			&item.CurrentLeaderID,
			&item.EndTime,
			&item.Version,
		); err != nil {
			rows.Close()
			return 0, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	const updateSQL = `
UPDATE auctions
   SET status = 'active',
       version = version + 1,
       updated_at = CURRENT_TIMESTAMP(3)
 WHERE id = ?
   AND status = 'pending'
   AND start_time <= CURRENT_TIMESTAMP(3)`

	started := 0
	for _, item := range candidates {
		result, err := tx.ExecContext(ctx, updateSQL, item.ID)
		if err != nil {
			return 0, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if affected == 0 {
			continue
		}

		eventSeq := item.Version + 1
		payload, err := r.startedPayload(item, eventSeq)
		if err != nil {
			return 0, err
		}
		if err := insertOutbox(ctx, tx, item.ID, "AuctionStarted", eventSeq, payload); err != nil {
			return 0, err
		}
		started++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return started, nil
}

func (r *Repository) ListDueEndIDs(ctx context.Context, limit int) ([]int64, error) {
	const query = `
SELECT id
  FROM auctions
 WHERE status = 'active'
   AND end_time <= CURRENT_TIMESTAMP(3)
 ORDER BY end_time, id
 LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) EndOne(ctx context.Context, auctionID int64) (EndResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return EndResult{}, err
	}
	defer tx.Rollback()

	const lockSQL = `
SELECT a.id, a.title, a.seller_id, a.status, a.current_price,
       a.current_leader_id, leader.nickname, leader.avatar,
       a.end_time, a.version
  FROM auctions a
  LEFT JOIN users leader ON leader.id = a.current_leader_id
 WHERE a.id = ?
 FOR UPDATE`
	var item lifecycleAuction
	err = tx.QueryRowContext(ctx, lockSQL, auctionID).Scan(
		&item.ID,
		&item.Title,
		&item.SellerID,
		&item.Status,
		&item.CurrentPrice,
		&item.CurrentLeaderID,
		&item.CurrentLeaderNickname,
		&item.CurrentLeaderAvatar,
		&item.EndTime,
		&item.Version,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return EndResult{}, nil
	}
	if err != nil {
		return EndResult{}, err
	}
	if item.Status != "active" {
		return EndResult{}, nil
	}

	const updateSQL = `
UPDATE auctions
   SET status = 'ended',
       version = version + 1,
       updated_at = CURRENT_TIMESTAMP(3)
 WHERE id = ?
   AND status = 'active'
   AND end_time <= CURRENT_TIMESTAMP(3)`
	result, err := tx.ExecContext(ctx, updateSQL, auctionID)
	if err != nil {
		return EndResult{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return EndResult{}, err
	}
	if affected == 0 {
		return EndResult{}, nil
	}

	var orderID *int64
	if item.CurrentLeaderID != nil {
		id, err := ensureOrder(
			ctx,
			tx,
			item.ID,
			*item.CurrentLeaderID,
			item.SellerID,
			item.CurrentPrice,
		)
		if err != nil {
			return EndResult{}, err
		}
		orderID = &id
	}

	eventSeq := item.Version + 1
	payload, err := r.endedPayload(item, eventSeq, orderID)
	if err != nil {
		return EndResult{}, err
	}
	if err := insertOutbox(ctx, tx, item.ID, "AuctionEnded", eventSeq, payload); err != nil {
		return EndResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return EndResult{}, err
	}
	return EndResult{Ended: true, OrderID: orderID}, nil
}

func ensureOrder(
	ctx context.Context,
	tx *sql.Tx,
	auctionID int64,
	winnerID int64,
	sellerID int64,
	finalPrice int64,
) (int64, error) {
	const insertSQL = `
INSERT INTO orders (
  auction_id, winner_id, seller_id, final_price, status
) VALUES (?, ?, ?, ?, 'pending_pay')
ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)`
	result, err := tx.ExecContext(ctx, insertSQL, auctionID, winnerID, sellerID, finalPrice)
	if err != nil {
		return 0, err
	}
	orderID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	const verifySQL = `
SELECT winner_id, seller_id, final_price
  FROM orders
 WHERE id = ?`
	var storedWinnerID int64
	var storedSellerID int64
	var storedFinalPrice int64
	if err := tx.QueryRowContext(ctx, verifySQL, orderID).Scan(
		&storedWinnerID,
		&storedSellerID,
		&storedFinalPrice,
	); err != nil {
		return 0, err
	}
	if storedWinnerID != winnerID ||
		storedSellerID != sellerID ||
		storedFinalPrice != finalPrice {
		return 0, fmt.Errorf(
			"%w: auction=%d existing winner=%d seller=%d price=%d",
			ErrOrderConflict,
			auctionID,
			storedWinnerID,
			storedSellerID,
			storedFinalPrice,
		)
	}
	return orderID, nil
}

func insertOutbox(
	ctx context.Context,
	tx *sql.Tx,
	auctionID int64,
	eventType string,
	eventSeq int64,
	payload []byte,
) error {
	const query = `
INSERT INTO event_outbox (
  aggregate_type, aggregate_id, event_type, event_seq, payload, status
) VALUES ('auction', ?, ?, ?, ?, 'pending')`
	_, err := tx.ExecContext(ctx, query, auctionID, eventType, eventSeq, string(payload))
	return err
}

func (r *Repository) startedPayload(item lifecycleAuction, eventSeq int64) ([]byte, error) {
	now := time.Now().In(r.location).Format(time.RFC3339Nano)
	return json.Marshal(map[string]any{
		"type":        "auction_started",
		"event_id":    eventID(item.ID, eventSeq),
		"auction_id":  item.ID,
		"seq":         eventSeq,
		"server_time": now,
		"data": map[string]any{
			"auction": map[string]any{
				"id":            item.ID,
				"title":         item.Title,
				"status":        "active",
				"current_price": item.CurrentPrice,
				"end_time":      item.EndTime.In(r.location).Format(time.RFC3339Nano),
				"version":       eventSeq,
			},
		},
	})
}

func (r *Repository) endedPayload(
	item lifecycleAuction,
	eventSeq int64,
	orderID *int64,
) ([]byte, error) {
	now := time.Now().In(r.location).Format(time.RFC3339Nano)
	var winner any
	var finalPrice any
	if item.CurrentLeaderID != nil {
		winner = map[string]any{
			"id":       *item.CurrentLeaderID,
			"nickname": stringValue(item.CurrentLeaderNickname),
			"avatar":   item.CurrentLeaderAvatar,
		}
		finalPrice = item.CurrentPrice
	}
	return json.Marshal(map[string]any{
		"type":        "auction_ended",
		"event_id":    eventID(item.ID, eventSeq),
		"auction_id":  item.ID,
		"seq":         eventSeq,
		"server_time": now,
		"data": map[string]any{
			"auction": map[string]any{
				"id":            item.ID,
				"title":         item.Title,
				"status":        "ended",
				"current_price": item.CurrentPrice,
				"end_time":      item.EndTime.In(r.location).Format(time.RFC3339Nano),
				"version":       eventSeq,
			},
			"winner":      winner,
			"final_price": finalPrice,
			"order_id":    orderID,
		},
	})
}

func eventID(auctionID int64, eventSeq int64) string {
	return fmt.Sprintf("evt_%d_%d", auctionID, eventSeq)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var ErrOrderConflict = errors.New("existing order does not match auction result")
