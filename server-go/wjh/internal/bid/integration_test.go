//go:build integration

package bid

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	httpx "auction-system/server-go/internal/http"

	_ "github.com/go-sql-driver/mysql"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

type bidIntegrationEnv struct {
	db      *sql.DB
	redis   *goredis.Client
	service *Service
}

func TestBidIntegration(t *testing.T) {
	env := newBidIntegrationEnv(t)

	t.Run("accepted bid commits auction bid idempotency and outbox", func(t *testing.T) {
		env.reset(t)
		env.insertUsers(t, 2)
		env.insertAuction(t, auctionFixture{
			StartPrice:   1000,
			PriceStep:    100,
			CurrentPrice: 1000,
			EndTime:      time.Now().UTC().Add(10 * time.Minute),
		})

		result, err := env.service.Place(
			t.Context(),
			1,
			Bidder{ID: 2, Nickname: "buyer-002"},
			1000,
			"normal-bid",
			"normal-request",
		)
		require.NoError(t, err)
		assert.Equal(t, int64(1000), result.CurrentPrice)
		assert.True(t, strings.HasSuffix(result.ServerTime, "+08:00"))

		var currentPrice, bidCount, version int64
		var leaderID int64
		require.NoError(t, env.db.QueryRowContext(t.Context(), `
SELECT current_price, current_leader_id, bid_count, version
  FROM auctions
 WHERE id = 1`).Scan(&currentPrice, &leaderID, &bidCount, &version))
		assert.Equal(t, int64(1000), currentPrice)
		assert.Equal(t, int64(2), leaderID)
		assert.Equal(t, int64(1), bidCount)
		assert.Equal(t, int64(1), version)
		assert.Equal(t, 1, env.scalarInt(t, "SELECT COUNT(*) FROM bids"))
		assert.Equal(t, 1, env.scalarInt(t, "SELECT COUNT(*) FROM event_outbox"))
		assert.Equal(t, "succeeded", env.scalarString(t, "SELECT status FROM idempotency_keys"))
	})

	t.Run("same idempotency key returns stored result and rejects changed amount", func(t *testing.T) {
		env.reset(t)
		env.insertUsers(t, 2)
		env.insertAuction(t, auctionFixture{
			StartPrice:   1000,
			PriceStep:    100,
			CurrentPrice: 1000,
			EndTime:      time.Now().UTC().Add(10 * time.Minute),
		})
		bidder := Bidder{ID: 2, Nickname: "buyer-002"}

		first, err := env.service.Place(t.Context(), 1, bidder, 1000, "same-key", "request-1")
		require.NoError(t, err)
		second, err := env.service.Place(t.Context(), 1, bidder, 1000, "same-key", "request-2")
		require.NoError(t, err)
		assert.Equal(t, first, second)

		_, err = env.service.Place(t.Context(), 1, bidder, 1100, "same-key", "request-3")
		assert.Equal(t, httpx.CodeIdempotent, integrationErrorCode(t, err))
		assert.Equal(t, 1, env.scalarInt(t, "SELECT COUNT(*) FROM bids"))
		assert.Equal(t, 1, env.scalarInt(t, "SELECT COUNT(*) FROM event_outbox"))
	})

	t.Run("ceiling bid ends auction and creates exactly one order", func(t *testing.T) {
		env.reset(t)
		env.insertUsers(t, 2)
		ceiling := int64(2000)
		env.insertAuction(t, auctionFixture{
			StartPrice:   1000,
			PriceStep:    100,
			CeilingPrice: &ceiling,
			CurrentPrice: 1000,
			BidCount:     1,
			Version:      7,
			EndTime:      time.Now().UTC().Add(10 * time.Minute),
		})

		result, err := env.service.Place(
			t.Context(),
			1,
			Bidder{ID: 2, Nickname: "buyer-002"},
			2000,
			"ceiling-key",
			"ceiling-request",
		)
		require.NoError(t, err)
		require.NotNil(t, result.OrderID)
		assert.True(t, result.CeilingHit)
		assert.False(t, result.Extended)
		assert.Equal(t, int64(9), result.AuctionVersion)

		assert.Equal(t, "ended", env.scalarString(t, "SELECT status FROM auctions WHERE id = 1"))
		assert.Equal(t, 1, env.scalarInt(t, "SELECT COUNT(*) FROM orders WHERE auction_id = 1"))

		rows, err := env.db.QueryContext(t.Context(), `
SELECT event_type, event_seq
  FROM event_outbox
 WHERE aggregate_id = 1
 ORDER BY event_seq`)
		require.NoError(t, err)
		defer rows.Close()

		var eventTypes []string
		var eventSeqs []int64
		for rows.Next() {
			var eventType string
			var eventSeq int64
			require.NoError(t, rows.Scan(&eventType, &eventSeq))
			eventTypes = append(eventTypes, eventType)
			eventSeqs = append(eventSeqs, eventSeq)
		}
		require.NoError(t, rows.Err())
		assert.Equal(t, []string{"BidAccepted", "AuctionEnded"}, eventTypes)
		assert.Equal(t, []int64{8, 9}, eventSeqs)
	})

	t.Run("one hundred concurrent bidders keep current price monotonic", func(t *testing.T) {
		env.reset(t)
		env.insertUsers(t, 101)
		env.insertAuction(t, auctionFixture{
			StartPrice:   0,
			PriceStep:    100,
			CurrentPrice: 0,
			EndTime:      time.Now().UTC().Add(10 * time.Minute),
		})

		const bidderCount = 100
		start := make(chan struct{})
		stopMonitor := make(chan struct{})
		samples := make(chan []int64, 1)
		monitorErrors := make(chan error, 1)
		go env.monitorCurrentPrice(t.Context(), stopMonitor, samples, monitorErrors)

		var wg sync.WaitGroup
		unexpected := make(chan error, bidderCount)
		for i := 0; i < bidderCount; i++ {
			index := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start

				// Staggering preserves heavy overlap while avoiding a test that only
				// proves the non-blocking Redis lock rejected 99 callers.
				time.Sleep(time.Duration(index) * 3 * time.Millisecond)
				amount := int64(index+1) * 100
				for attempt := 0; attempt < 200; attempt++ {
					_, err := env.service.Place(
						context.Background(),
						1,
						Bidder{
							ID:       int64(index + 2),
							Nickname: fmt.Sprintf("buyer-%03d", index+2),
						},
						amount,
						fmt.Sprintf("concurrent-%03d", index),
						fmt.Sprintf("request-%03d-%03d", index, attempt),
					)
					if err == nil {
						return
					}
					switch integrationErrorCodeNoFail(err) {
					case httpx.CodeBidConflict:
						time.Sleep(2 * time.Millisecond)
					case httpx.CodeBidTooLow:
						return
					default:
						unexpected <- fmt.Errorf("bidder %d amount %d: %w", index, amount, err)
						return
					}
				}
				unexpected <- fmt.Errorf("bidder %d exhausted lock retries", index)
			}()
		}

		close(start)
		wg.Wait()
		close(stopMonitor)
		priceSamples := <-samples
		select {
		case err := <-monitorErrors:
			require.NoError(t, err)
		default:
		}
		close(unexpected)
		for err := range unexpected {
			require.NoError(t, err)
		}

		accepted := env.acceptedAmounts(t)
		require.Greater(t, len(accepted), 1)
		for i := 1; i < len(accepted); i++ {
			assert.Greater(t, accepted[i], accepted[i-1])
		}
		for i := 1; i < len(priceSamples); i++ {
			assert.GreaterOrEqual(t, priceSamples[i], priceSamples[i-1])
		}

		var currentPrice, bidCount, version int64
		require.NoError(t, env.db.QueryRowContext(t.Context(), `
SELECT current_price, bid_count, version
  FROM auctions
 WHERE id = 1`).Scan(&currentPrice, &bidCount, &version))
		assert.Equal(t, accepted[len(accepted)-1], currentPrice)
		assert.Equal(t, int64(10000), currentPrice)
		assert.Equal(t, int64(len(accepted)), bidCount)
		assert.Equal(t, bidCount, version)
		assert.Equal(t, 0, env.scalarInt(t, "SELECT COUNT(*) FROM orders"))

		var eventCount, distinctSeq int
		var minSeq, maxSeq int64
		require.NoError(t, env.db.QueryRowContext(t.Context(), `
SELECT COUNT(*), COUNT(DISTINCT event_seq), MIN(event_seq), MAX(event_seq)
  FROM event_outbox
 WHERE aggregate_type = 'auction'
   AND aggregate_id = 1`).Scan(&eventCount, &distinctSeq, &minSeq, &maxSeq))
		assert.Equal(t, len(accepted), eventCount)
		assert.Equal(t, eventCount, distinctSeq)
		assert.Equal(t, int64(1), minSeq)
		assert.Equal(t, int64(eventCount), maxSeq)
	})
}

func newBidIntegrationEnv(t *testing.T) *bidIntegrationEnv {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()

	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	schemaPath := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "docs", "schema.sql"))

	var mysqlContainer *tcmysql.MySQLContainer
	var redisContainer *tcredis.RedisContainer
	var mysqlErr, redisErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		mysqlContainer, mysqlErr = tcmysql.Run(
			ctx,
			"mysql:8.0.36",
			tcmysql.WithDatabase("auction"),
			tcmysql.WithUsername("root"),
			tcmysql.WithPassword("auction_root"),
			tcmysql.WithScripts(schemaPath),
			testcontainers.WithCmdArgs("--default-time-zone=+00:00"),
		)
	}()
	go func() {
		defer wg.Done()
		redisContainer, redisErr = tcredis.Run(ctx, "redis:7-alpine")
	}()
	wg.Wait()

	if mysqlContainer != nil {
		t.Cleanup(func() {
			require.NoError(t, testcontainers.TerminateContainer(mysqlContainer))
		})
	}
	if redisContainer != nil {
		t.Cleanup(func() {
			require.NoError(t, testcontainers.TerminateContainer(redisContainer))
		})
	}
	require.NoError(t, mysqlErr)
	require.NoError(t, redisErr)

	dsn, err := mysqlContainer.ConnectionString(ctx, "parseTime=true", "loc=UTC", "charset=utf8mb4")
	require.NoError(t, err)
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(120)
	db.SetMaxIdleConns(20)
	require.NoError(t, db.PingContext(ctx))
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	redisURL, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err)
	redisOptions, err := goredis.ParseURL(redisURL)
	require.NoError(t, err)
	redisClient := goredis.NewClient(redisOptions)
	require.NoError(t, redisClient.Ping(ctx).Err())
	t.Cleanup(func() {
		require.NoError(t, redisClient.Close())
	})

	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	repository := NewRepository(db)
	service := NewService(repository, NewRedisLocker(redisClient), location)

	return &bidIntegrationEnv{
		db:      db,
		redis:   redisClient,
		service: service,
	}
}

func (env *bidIntegrationEnv) reset(t *testing.T) {
	t.Helper()
	statements := []string{
		"SET FOREIGN_KEY_CHECKS = 0",
		"TRUNCATE TABLE event_outbox",
		"TRUNCATE TABLE idempotency_keys",
		"TRUNCATE TABLE orders",
		"TRUNCATE TABLE bids",
		"TRUNCATE TABLE auctions",
		"TRUNCATE TABLE users",
		"SET FOREIGN_KEY_CHECKS = 1",
	}
	for _, statement := range statements {
		_, err := env.db.ExecContext(t.Context(), statement)
		require.NoError(t, err)
	}
	require.NoError(t, env.redis.FlushDB(t.Context()).Err())
}

func (env *bidIntegrationEnv) insertUsers(t *testing.T, highestID int) {
	t.Helper()
	statement, err := env.db.PrepareContext(t.Context(), `
INSERT INTO users (id, nickname, avatar, token)
VALUES (?, ?, NULL, ?)`)
	require.NoError(t, err)
	defer statement.Close()

	for id := 1; id <= highestID; id++ {
		nickname := fmt.Sprintf("buyer-%03d", id)
		token := fmt.Sprintf("mock-token-user-%03d", id)
		if id == 1 {
			nickname = "seller-001"
			token = "mock-token-seller-001"
		}
		_, err := statement.ExecContext(t.Context(), id, nickname, token)
		require.NoError(t, err)
	}
}

type auctionFixture struct {
	StartPrice   int64
	PriceStep    int64
	CeilingPrice *int64
	CurrentPrice int64
	BidCount     int64
	Version      int64
	EndTime      time.Time
}

func (env *bidIntegrationEnv) insertAuction(t *testing.T, fixture auctionFixture) {
	t.Helper()
	_, err := env.db.ExecContext(t.Context(), `
INSERT INTO auctions (
  id, title, description, cover_url, images, stream_url,
  start_price, price_step, ceiling_price, current_price,
  current_leader_id, start_time, end_time, original_end_time,
  extend_seconds, extend_threshold, status, version,
  viewer_count, bid_count, seller_id
) VALUES (
  1, 'integration auction', NULL, NULL, NULL, NULL,
  ?, ?, ?, ?,
  NULL, ?, ?, ?,
  30, 30, 'active', ?,
  0, ?, 1
)`,
		fixture.StartPrice,
		fixture.PriceStep,
		fixture.CeilingPrice,
		fixture.CurrentPrice,
		time.Now().UTC().Add(-time.Minute),
		fixture.EndTime.UTC(),
		fixture.EndTime.UTC(),
		fixture.Version,
		fixture.BidCount,
	)
	require.NoError(t, err)
}

func (env *bidIntegrationEnv) monitorCurrentPrice(
	ctx context.Context,
	stop <-chan struct{},
	samples chan<- []int64,
	errs chan<- error,
) {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	values := make([]int64, 0, 512)
	read := func() bool {
		var value int64
		if err := env.db.QueryRowContext(ctx, "SELECT current_price FROM auctions WHERE id = 1").Scan(&value); err != nil {
			errs <- err
			return false
		}
		values = append(values, value)
		return true
	}
	if !read() {
		samples <- values
		return
	}

	for {
		select {
		case <-stop:
			read()
			samples <- values
			return
		case <-ticker.C:
			if !read() {
				samples <- values
				return
			}
		}
	}
}

func (env *bidIntegrationEnv) acceptedAmounts(t *testing.T) []int64 {
	t.Helper()
	rows, err := env.db.QueryContext(t.Context(), `
SELECT amount
  FROM bids
 WHERE auction_id = 1
   AND status = 'accepted'
 ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()

	var amounts []int64
	for rows.Next() {
		var amount int64
		require.NoError(t, rows.Scan(&amount))
		amounts = append(amounts, amount)
	}
	require.NoError(t, rows.Err())
	return amounts
}

func (env *bidIntegrationEnv) scalarInt(t *testing.T, query string) int {
	t.Helper()
	var value int
	require.NoError(t, env.db.QueryRowContext(t.Context(), query).Scan(&value))
	return value
}

func (env *bidIntegrationEnv) scalarString(t *testing.T, query string) string {
	t.Helper()
	var value string
	require.NoError(t, env.db.QueryRowContext(t.Context(), query).Scan(&value))
	return value
}

func integrationErrorCode(t *testing.T, err error) httpx.Code {
	t.Helper()
	require.Error(t, err)
	code := integrationErrorCodeNoFail(err)
	require.NotEqual(t, httpx.CodeInternal, code)
	return code
}

func integrationErrorCodeNoFail(err error) httpx.Code {
	var appErr *httpx.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return httpx.CodeInternal
}
