package order

import (
	"context"
	"database/sql"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListBySeller(ctx context.Context, sellerID int64, status string, page int, size int) ([]orderRow, int64, error) {
	return r.list(ctx, "seller_id", sellerID, status, page, size)
}

func (r *Repository) ListByWinner(ctx context.Context, winnerID int64, status string, page int, size int) ([]orderRow, int64, error) {
	return r.list(ctx, "winner_id", winnerID, status, page, size)
}

func (r *Repository) list(ctx context.Context, roleColumn string, userID int64, status string, page int, size int) ([]orderRow, int64, error) {
	whereClause := " WHERE o." + roleColumn + " = ?"
	args := []any{userID}
	if status != "" {
		whereClause += " AND o.status = ?"
		args = append(args, status)
	}

	var total int64
	countSQL := "SELECT COUNT(*) FROM orders o" + whereClause
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	args = append(args, size, offset)
	query := baseSelect + whereClause + ` ORDER BY o.created_at DESC LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]orderRow, 0)
	for rows.Next() {
		row, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, row)
	}
	return result, total, rows.Err()
}

func (r *Repository) FindByID(ctx context.Context, orderID int64) (orderRow, bool, error) {
	row := r.db.QueryRowContext(ctx, baseSelect+" WHERE o.id = ?", orderID)
	result, err := scanOrder(row)
	if err != nil {
		return orderRow{}, false, nil
	}
	return result, true, nil
}

func (r *Repository) Pay(ctx context.Context, orderID int64, winnerID int64) error {
	const query = `
UPDATE orders
   SET status = 'paid',
       paid_at = CURRENT_TIMESTAMP(3),
       updated_at = CURRENT_TIMESTAMP(3)
 WHERE id = ?
   AND winner_id = ?
   AND status = 'pending_pay'`
	result, err := r.db.ExecContext(ctx, query, orderID, winnerID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errNotPayable
	}
	return nil
}

type orderRow struct {
	ID           int64
	AuctionID    int64
	WinnerID     int64
	WinnerNick   string
	WinnerAvatar *string
	SellerID     int64
	SellerNick   string
	SellerAvatar *string
	FinalPrice   int64
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	PaidAt       *time.Time
}

type scanner interface {
	Scan(dest ...any) error
}

func scanOrder(row scanner) (orderRow, error) {
	var r orderRow
	var winnerAvatar, sellerAvatar sql.NullString
	var paidAt sql.NullTime
	err := row.Scan(
		&r.ID, &r.AuctionID,
		&r.WinnerID, &r.WinnerNick, &winnerAvatar,
		&r.SellerID, &r.SellerNick, &sellerAvatar,
		&r.FinalPrice, &r.Status,
		&r.CreatedAt, &r.UpdatedAt, &paidAt,
	)
	if winnerAvatar.Valid {
		r.WinnerAvatar = &winnerAvatar.String
	}
	if sellerAvatar.Valid {
		r.SellerAvatar = &sellerAvatar.String
	}
	if paidAt.Valid {
		r.PaidAt = &paidAt.Time
	}
	return r, err
}

const baseSelect = `
SELECT o.id, o.auction_id,
       w.id, w.nickname, w.avatar,
       s.id, s.nickname, s.avatar,
       o.final_price, o.status,
       o.created_at, o.updated_at, o.paid_at
  FROM orders o
  JOIN users w ON w.id = o.winner_id
  JOIN users s ON s.id = o.seller_id`

var errNotPayable = sql.ErrNoRows
