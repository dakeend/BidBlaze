package bid

import (
	"errors"
	"testing"
	"time"

	httpx "auction-system/server-go/internal/http"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMinAcceptableAmount(t *testing.T) {
	tests := []struct {
		name     string
		snapshot AuctionSnapshot
		want     int64
	}{
		{
			name: "first bid uses positive start price",
			snapshot: AuctionSnapshot{
				StartPrice: 1000,
				PriceStep:  100,
			},
			want: 1000,
		},
		{
			name: "zero price auction starts at one step",
			snapshot: AuctionSnapshot{
				StartPrice: 0,
				PriceStep:  100,
			},
			want: 100,
		},
		{
			name: "later bid adds one step",
			snapshot: AuctionSnapshot{
				StartPrice:   1000,
				PriceStep:    100,
				CurrentPrice: 1500,
				BidCount:     4,
			},
			want: 1600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, minAcceptableAmount(tt.snapshot))
		})
	}
}

func TestValidateAuction(t *testing.T) {
	now := time.Date(2026, 6, 8, 4, 0, 0, 0, time.UTC)
	ceiling := int64(2000)
	base := AuctionSnapshot{
		Status:        "active",
		StartPrice:    1000,
		PriceStep:     100,
		CeilingPrice:  &ceiling,
		CurrentPrice:  1000,
		BidCount:      1,
		EndTime:       now.Add(time.Minute),
		ExtendSeconds: 30,
	}

	tests := []struct {
		name   string
		mutate func(*AuctionSnapshot)
		amount int64
		code   httpx.Code
	}{
		{
			name:   "exact minimum is accepted",
			amount: 1100,
		},
		{
			name:   "exact ceiling is accepted",
			amount: 2000,
		},
		{
			name: "pending auction rejects bids",
			mutate: func(snapshot *AuctionSnapshot) {
				snapshot.Status = "pending"
			},
			amount: 1100,
			code:   httpx.CodeAuctionPending,
		},
		{
			name: "ended auction rejects bids",
			mutate: func(snapshot *AuctionSnapshot) {
				snapshot.Status = "ended"
			},
			amount: 1100,
			code:   httpx.CodeAuctionEnded,
		},
		{
			name: "cancelled auction rejects bids",
			mutate: func(snapshot *AuctionSnapshot) {
				snapshot.Status = "cancelled"
			},
			amount: 1100,
			code:   httpx.CodeAuctionCancel,
		},
		{
			name: "expired active auction rejects bids",
			mutate: func(snapshot *AuctionSnapshot) {
				snapshot.EndTime = now
			},
			amount: 1100,
			code:   httpx.CodeAuctionEnded,
		},
		{
			name:   "amount below current plus step is rejected",
			amount: 1099,
			code:   httpx.CodeBidTooLow,
		},
		{
			name:   "amount above ceiling is rejected",
			amount: 2001,
			code:   httpx.CodeBidOverCeiling,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := base
			if tt.mutate != nil {
				tt.mutate(&snapshot)
			}

			err := validateAuction(snapshot, tt.amount, now)
			if tt.code == httpx.CodeOK {
				require.NoError(t, err)
				return
			}
			require.Equal(t, tt.code, appErrorCode(t, err))
		})
	}
}

func TestShouldExtend(t *testing.T) {
	now := time.Date(2026, 6, 8, 4, 0, 0, 0, time.UTC)
	snapshot := AuctionSnapshot{ExtendThreshold: 30}

	snapshot.EndTime = now.Add(31 * time.Second)
	assert.False(t, shouldExtend(snapshot, now))

	snapshot.EndTime = now.Add(30 * time.Second)
	assert.True(t, shouldExtend(snapshot, now))

	snapshot.EndTime = now.Add(time.Second)
	assert.True(t, shouldExtend(snapshot, now))
}

func TestBuildEventsUsesContiguousAuctionSequence(t *testing.T) {
	snapshot := AuctionSnapshot{
		ID:            7,
		Version:       10,
		ExtendSeconds: 30,
	}
	result := Result{
		Bid: Bid{
			ID:        20,
			AuctionID: 7,
			Amount:    1500,
			Status:    "accepted",
		},
		AuctionVersion: 12,
		CurrentPrice:   1500,
		ServerTime:     "2026-06-08T12:00:00+08:00",
		Extended:       true,
		NewEndTime:     "2026-06-08T12:01:00+08:00",
	}

	events, err := buildEvents(snapshot, result)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "BidAccepted", events[0].EventType)
	assert.Equal(t, int64(11), events[0].EventSeq)
	assert.Equal(t, "AuctionExtended", events[1].EventType)
	assert.Equal(t, int64(12), events[1].EventSeq)
}

func appErrorCode(t *testing.T, err error) httpx.Code {
	t.Helper()
	require.Error(t, err)

	var appErr *httpx.AppError
	require.True(t, errors.As(err, &appErr))
	return appErr.Code
}
