package auction

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, params CreateParams) (int64, error) {
	imagesJSON, err := json.Marshal(params.Images)
	if err != nil {
		return 0, err
	}
	const query = `
INSERT INTO auctions (
  title, description, cover_url, images, stream_url,
  start_price, price_step, ceiling_price, current_price,
  start_time, end_time, original_end_time,
  extend_seconds, extend_threshold, seller_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		params.Title, params.Description, params.CoverURL, string(imagesJSON), params.StreamURL,
		params.StartPrice, params.PriceStep, params.CeilingPrice, params.StartPrice,
		params.StartAt, params.EndAt, params.OriginalEndAt,
		params.ExtendSec, params.ExtendThreshold, params.SellerID,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *Repository) FindByID(ctx context.Context, id int64) (auctionRow, bool, error) {
	const query = baseSelect + `
 WHERE a.id = ?
 LIMIT 1`
	return r.scanOne(ctx, query, id)
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]auctionRow, int64, error) {
	where, args := listWhere(query)
	countSQL := `SELECT COUNT(*) FROM auctions a` + where
	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.Size
	args = append(args, query.Size, offset)
	rows, err := r.db.QueryContext(ctx, baseSelect+where+`
 ORDER BY a.start_time DESC, a.id DESC
 LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]auctionRow, 0)
	for rows.Next() {
		row, err := scanAuction(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, row)
	}
	return result, total, rows.Err()
}

func (r *Repository) Update(ctx context.Context, params UpdateParams) error {
	sets := make([]string, 0, 12)
	args := make([]any, 0, 16)
	add := func(expr string, value any) {
		sets = append(sets, expr)
		args = append(args, value)
	}

	if params.Title != nil {
		add("title = ?", *params.Title)
	}
	if params.Description != nil {
		add("description = ?", *params.Description)
	}
	if params.CoverURL != nil {
		add("cover_url = ?", *params.CoverURL)
	}
	if params.Images != nil {
		imagesJSON, err := json.Marshal(params.Images)
		if err != nil {
			return err
		}
		add("images = ?", string(imagesJSON))
	}
	if params.StreamURL != nil {
		add("stream_url = ?", *params.StreamURL)
	}
	if params.StartPrice != nil {
		add("start_price = ?", *params.StartPrice)
		add("current_price = ?", *params.StartPrice)
	}
	if params.PriceStep != nil {
		add("price_step = ?", *params.PriceStep)
	}
	if params.CeilingPrice != nil {
		add("ceiling_price = ?", *params.CeilingPrice)
	}
	if params.StartAt != nil {
		add("start_time = ?", *params.StartAt)
	}
	if params.EndAt != nil {
		add("end_time = ?", *params.EndAt)
	}
	if params.OriginalEndAt != nil {
		add("original_end_time = ?", *params.OriginalEndAt)
	}
	if params.ExtendSec != nil {
		add("extend_seconds = ?", *params.ExtendSec)
	}
	if params.ExtendThreshold != nil {
		add("extend_threshold = ?", *params.ExtendThreshold)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = CURRENT_TIMESTAMP(3)")
	args = append(args, params.AuctionID, params.SellerID)

	sqlText := `
UPDATE auctions
   SET ` + strings.Join(sets, ", ") + `
 WHERE id = ?
   AND seller_id = ?
   AND status = 'pending'
   AND start_time > CURRENT_TIMESTAMP(3)`
	result, err := r.db.ExecContext(ctx, sqlText, args...)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errNoRowsAffected
	}
	return nil
}

func (r *Repository) Cancel(ctx context.Context, auctionID int64, sellerID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const lockSQL = `
SELECT seller_id, status, version
  FROM auctions
 WHERE id = ?
 FOR UPDATE`
	var ownerID int64
	var status string
	var version int64
	err = tx.QueryRowContext(ctx, lockSQL, auctionID).Scan(&ownerID, &status, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return errAuctionNotFound
	}
	if err != nil {
		return err
	}
	if ownerID != sellerID {
		return errForbidden
	}
	if status == "ended" {
		return errEnded
	}
	if status == "cancelled" {
		return errCancelled
	}
	if status != "pending" && status != "active" {
		return errInvalidState
	}

	eventSeq := version + 1
	payload, err := cancelPayload(auctionID, eventSeq)
	if err != nil {
		return err
	}
	const updateSQL = `
UPDATE auctions
   SET status = 'cancelled',
       version = version + 1,
       updated_at = CURRENT_TIMESTAMP(3)
 WHERE id = ?
   AND status IN ('pending', 'active')`
	if _, err := tx.ExecContext(ctx, updateSQL, auctionID); err != nil {
		return err
	}

	const outboxSQL = `
INSERT INTO event_outbox (
  aggregate_type, aggregate_id, event_type, event_seq, payload, status
) VALUES ('auction', ?, 'AuctionCancelled', ?, ?, 'pending')`
	if _, err := tx.ExecContext(ctx, outboxSQL, auctionID, eventSeq, string(payload)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) scanOne(ctx context.Context, query string, args ...any) (auctionRow, bool, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	result, err := scanAuction(row)
	if errors.Is(err, sql.ErrNoRows) {
		return auctionRow{}, false, nil
	}
	if err != nil {
		return auctionRow{}, false, err
	}
	return result, true, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAuction(row scanner) (auctionRow, error) {
	var result auctionRow
	err := row.Scan(
		&result.ID,
		&result.Title,
		&result.Description,
		&result.CoverURL,
		&result.ImagesJSON,
		&result.StreamURL,
		&result.StartPrice,
		&result.PriceStep,
		&result.CeilingPrice,
		&result.CurrentPrice,
		&result.CurrentLeaderID,
		&result.CurrentLeaderNickname,
		&result.CurrentLeaderAvatar,
		&result.StartTime,
		&result.EndTime,
		&result.OriginalEndTime,
		&result.ExtendSeconds,
		&result.ExtendThreshold,
		&result.Status,
		&result.Version,
		&result.ViewerCount,
		&result.BidCount,
		&result.SellerID,
		&result.SellerNickname,
		&result.SellerAvatar,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	return result, err
}

func listWhere(query ListQuery) (string, []any) {
	conditions := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if query.Status != "" {
		conditions = append(conditions, "a.status = ?")
		args = append(args, query.Status)
	}
	if query.SellerID != nil {
		conditions = append(conditions, "a.seller_id = ?")
		args = append(args, *query.SellerID)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

const baseSelect = `
SELECT
  a.id,
  a.title,
  a.description,
  a.cover_url,
  CAST(a.images AS CHAR),
  a.stream_url,
  a.start_price,
  a.price_step,
  a.ceiling_price,
  a.current_price,
  leader.id,
  leader.nickname,
  leader.avatar,
  a.start_time,
  a.end_time,
  a.original_end_time,
  a.extend_seconds,
  a.extend_threshold,
  a.status,
  a.version,
  a.viewer_count,
  a.bid_count,
  seller.id,
  seller.nickname,
  seller.avatar,
  a.created_at,
  a.updated_at
FROM auctions a
JOIN users seller ON seller.id = a.seller_id
LEFT JOIN users leader ON leader.id = a.current_leader_id`

var (
	errAuctionNotFound = errors.New("auction not found")
	errForbidden       = errors.New("forbidden")
	errEnded           = errors.New("auction ended")
	errCancelled       = errors.New("auction cancelled")
	errInvalidState    = errors.New("invalid auction state")
	errNoRowsAffected  = errors.New("no rows affected")
)

// GetTopBids 返回指定拍卖的前 N 条 accepted 出价（含用户信息）。
func (r *Repository) GetTopBids(ctx context.Context, auctionID int64, limit int) ([]BidInfo, error) {
	const query = `
SELECT b.id, b.auction_id, u.id, u.nickname, u.avatar,
       b.amount, b.status, b.reject_reason, b.idempotency_key, b.created_at
  FROM bids b
  JOIN users u ON u.id = b.user_id
 WHERE b.auction_id = ?
   AND b.status = 'accepted'
 ORDER BY b.created_at DESC
 LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, auctionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]BidInfo, 0, limit)
	for rows.Next() {
		var bid BidInfo
		var avatar sql.NullString
		var rejectReason, idempotencyKey sql.NullString
		var createdAt time.Time
		if err := rows.Scan(
			&bid.ID, &bid.AuctionID,
			&bid.User.ID, &bid.User.Nickname, &avatar,
			&bid.Amount, &bid.Status, &rejectReason, &idempotencyKey, &createdAt,
		); err != nil {
			return nil, err
		}
		if avatar.Valid {
			bid.User.Avatar = &avatar.String
		}
		if rejectReason.Valid {
			bid.RejectReason = &rejectReason.String
		}
		if idempotencyKey.Valid {
			bid.IdempotencyKey = &idempotencyKey.String
		}
		bid.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		result = append(result, bid)
	}
	return result, rows.Err()
}

// GetLastEventSeq 返回指定拍卖的最后事件序号。
func (r *Repository) GetLastEventSeq(ctx context.Context, auctionID int64) (int64, error) {
	const query = `
SELECT COALESCE(MAX(event_seq), 0)
  FROM event_outbox
 WHERE aggregate_type = 'auction'
   AND aggregate_id = ?`
	var seq int64
	err := r.db.QueryRowContext(ctx, query, auctionID).Scan(&seq)
	return seq, err
}

func cancelPayload(auctionID int64, eventSeq int64) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":       "auction_cancelled",
		"event_id":   fmt.Sprintf("evt_%d", eventSeq),
		"auction_id": auctionID,
		"seq":        eventSeq,
		"data": map[string]any{
			"reason": "seller_cancelled",
		},
	})
}
