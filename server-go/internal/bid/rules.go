package bid

import (
	"net/http"
	"time"

	httpx "auction-system/server-go/internal/http"
)

func validateAuction(snapshot AuctionSnapshot, amount int64, now time.Time) error {
	switch snapshot.Status {
	case "pending":
		return httpx.AuctionNotPending()
	case "ended":
		return httpx.AuctionEnded()
	case "cancelled":
		return httpx.AuctionCancelled()
	case "active":
	default:
		return httpx.BidConflict()
	}
	if !snapshot.EndTime.After(now) {
		return httpx.AuctionEnded()
	}

	minimum := minAcceptableAmount(snapshot)
	if amount < minimum {
		return httpx.NewErrorData(
			httpx.CodeBidTooLow,
			http.StatusOK,
			"出价低于当前价+加价幅度",
			map[string]any{
				"min_acceptable_amount": minimum,
				"current_price":         snapshot.CurrentPrice,
				"price_step":            snapshot.PriceStep,
				"server_time":           now.Format(time.RFC3339Nano),
			},
		)
	}
	if snapshot.CeilingPrice != nil && amount > *snapshot.CeilingPrice {
		return httpx.NewErrorData(
			httpx.CodeBidOverCeiling,
			http.StatusOK,
			"出价超过封顶价",
			map[string]any{
				"ceiling_price": *snapshot.CeilingPrice,
				"server_time":   now.Format(time.RFC3339Nano),
			},
		)
	}
	return nil
}

func minAcceptableAmount(snapshot AuctionSnapshot) int64 {
	if snapshot.BidCount == 0 {
		if snapshot.StartPrice > 0 {
			return snapshot.StartPrice
		}
		return snapshot.PriceStep
	}
	return snapshot.CurrentPrice + snapshot.PriceStep
}

func shouldExtend(snapshot AuctionSnapshot, now time.Time) bool {
	threshold := time.Duration(snapshot.ExtendThreshold) * time.Second
	return snapshot.EndTime.Sub(now) <= threshold
}
