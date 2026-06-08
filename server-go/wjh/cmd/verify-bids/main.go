package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type bidRow struct {
	ID     int64
	Amount int64
}

func main() {
	var dsn string
	var auctionID int64
	var timeout time.Duration

	flag.StringVar(
		&dsn,
		"dsn",
		env("MYSQL_DSN", "root:auction_root@tcp(127.0.0.1:3306)/auction?parseTime=true&loc=UTC&charset=utf8mb4"),
		"MySQL DSN",
	)
	flag.Int64Var(&auctionID, "auction-id", 900001, "auction ID to verify")
	flag.DurationVar(&timeout, "timeout", 15*time.Second, "verification timeout")
	flag.Parse()

	if err := verify(dsn, auctionID, timeout); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func verify(dsn string, auctionID int64, timeout time.Duration) error {
	if auctionID <= 0 {
		return errors.New("auction-id must be positive")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open MySQL: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping MySQL: %w", err)
	}

	bids, err := loadAcceptedBids(ctx, db, auctionID)
	if err != nil {
		return err
	}
	if len(bids) == 0 {
		return errors.New("no accepted bids found")
	}

	for index := 1; index < len(bids); index++ {
		previous := bids[index-1]
		current := bids[index]
		if current.Amount <= previous.Amount {
			return fmt.Errorf(
				"non-monotonic accepted bids: bid %d amount=%d follows bid %d amount=%d",
				current.ID,
				current.Amount,
				previous.ID,
				previous.Amount,
			)
		}
	}

	var currentPrice, bidCount int64
	if err := db.QueryRowContext(ctx, `
SELECT current_price, bid_count
  FROM auctions
 WHERE id = ?`,
		auctionID,
	).Scan(&currentPrice, &bidCount); err != nil {
		return fmt.Errorf("load auction state: %w", err)
	}

	lastAmount := bids[len(bids)-1].Amount
	if currentPrice != lastAmount {
		return fmt.Errorf(
			"auction current_price=%d does not match last accepted amount=%d",
			currentPrice,
			lastAmount,
		)
	}
	if bidCount != int64(len(bids)) {
		return fmt.Errorf(
			"auction bid_count=%d does not match accepted rows=%d",
			bidCount,
			len(bids),
		)
	}

	var eventCount, distinctSeq int64
	var minSeq, maxSeq sql.NullInt64
	if err := db.QueryRowContext(ctx, `
SELECT
  COUNT(*),
  COUNT(DISTINCT event_seq),
  MIN(event_seq),
  MAX(event_seq)
FROM event_outbox
WHERE aggregate_type = 'auction'
  AND aggregate_id = ?`,
		auctionID,
	).Scan(&eventCount, &distinctSeq, &minSeq, &maxSeq); err != nil {
		return fmt.Errorf("load outbox sequence: %w", err)
	}

	if eventCount != bidCount || distinctSeq != eventCount {
		return fmt.Errorf(
			"outbox sequence mismatch: events=%d distinct_seq=%d bids=%d",
			eventCount,
			distinctSeq,
			bidCount,
		)
	}
	if !minSeq.Valid || !maxSeq.Valid || minSeq.Int64 != 1 || maxSeq.Int64 != eventCount {
		return fmt.Errorf(
			"outbox sequence is not contiguous: min=%v max=%v events=%d",
			minSeq,
			maxSeq,
			eventCount,
		)
	}

	fmt.Printf(
		"PASS auction_id=%d accepted_bids=%d first_price=%d final_price=%d outbox_seq=1..%d\n",
		auctionID,
		len(bids),
		bids[0].Amount,
		lastAmount,
		eventCount,
	)
	return nil
}

func loadAcceptedBids(ctx context.Context, db *sql.DB, auctionID int64) ([]bidRow, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, amount
  FROM bids
 WHERE auction_id = ?
   AND status = 'accepted'
 ORDER BY id`,
		auctionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query accepted bids: %w", err)
	}
	defer rows.Close()

	result := make([]bidRow, 0, 1024)
	for rows.Next() {
		var bid bidRow
		if err := rows.Scan(&bid.ID, &bid.Amount); err != nil {
			return nil, fmt.Errorf("scan accepted bid: %w", err)
		}
		result = append(result, bid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accepted bids: %w", err)
	}
	return result, nil
}

func env(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
