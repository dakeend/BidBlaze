package bid

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	httpx "auction-system/server-go/internal/http"
)

type Service struct {
	store    Store
	locker   Locker
	location *time.Location
	now      func() time.Time
}

func NewService(store Store, locker Locker, location *time.Location) *Service {
	return &Service{
		store:    store,
		locker:   locker,
		location: location,
		now:      time.Now,
	}
}

func (s *Service) Place(
	ctx context.Context,
	auctionID int64,
	bidder Bidder,
	amount int64,
	idempotencyKey string,
	requestID string,
) (Result, error) {
	if auctionID <= 0 || amount <= 0 || idempotencyKey == "" || len(idempotencyKey) > 128 {
		detail := "出价参数或 Idempotency-Key 非法"
		if auctionID <= 0 {
			detail = "拍卖 ID 非法"
		} else if amount <= 0 {
			detail = "出价金额必须大于 0"
		} else if idempotencyKey == "" {
			detail = "缺少 Idempotency-Key 请求头"
		} else if len(idempotencyKey) > 128 {
			detail = "Idempotency-Key 过长"
		}
		return Result{}, httpx.InvalidParam(detail)
	}

	scope := fmt.Sprintf("bid:%d", auctionID)
	requestHash := hashRequest(auctionID, amount)
	if result, handled, err := s.readIdempotency(ctx, bidder.ID, scope, idempotencyKey, requestHash); handled || err != nil {
		return result, err
	}

	snapshot, ok, err := s.store.GetAuction(ctx, auctionID)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		return Result{}, httpx.AuctionNotFound()
	}
	now := s.now().In(s.location)
	if err := validateAuction(snapshot, amount, now); err != nil {
		return Result{}, err
	}

	lockKey := fmt.Sprintf("bid_lock:%d", auctionID)
	locked, err := s.locker.Acquire(ctx, lockKey, requestID, 3*time.Second)
	if err != nil {
		return Result{}, httpx.SystemProtect(err)
	}
	if !locked {
		return Result{}, httpx.BidConflict()
	}
	defer func() {
		_ = s.locker.Release(context.WithoutCancel(ctx), lockKey, requestID)
	}()

	var result Result
	err = s.store.WithTx(ctx, func(tx TxStore) error {
		record, err := tx.ClaimIdempotency(ctx, bidder.ID, scope, idempotencyKey, requestHash)
		if err != nil {
			return err
		}
		if record.RequestHash != requestHash {
			return businessAbort{err: httpx.IdempotencyConflict()}
		}
		if record.Status == "succeeded" && len(record.ResponseJSON) > 0 {
			if err := json.Unmarshal(record.ResponseJSON, &result); err != nil {
				return err
			}
			return nil
		}

		current, ok, err := tx.LockAuction(ctx, auctionID)
		if err != nil {
			return err
		}
		if !ok {
			return businessAbort{err: httpx.AuctionNotFound()}
		}

		txNow := s.now().In(s.location)
		if err := validateAuction(current, amount, txNow); err != nil {
			return businessAbort{err: err}
		}

		ceilingHit := current.CeilingPrice != nil && amount == *current.CeilingPrice
		extended := !ceilingHit && shouldExtend(current, txNow)
		newEndTime := current.EndTime
		eventCount := int64(1)
		if ceilingHit {
			newEndTime = txNow
			eventCount = 2
		} else if extended {
			newEndTime = current.EndTime.Add(time.Duration(current.ExtendSeconds) * time.Second)
			eventCount = 2
		}

		affected, err := tx.ConditionalAccept(ctx, AcceptParams{
			AuctionID:  auctionID,
			UserID:     bidder.ID,
			Amount:     amount,
			NewEndTime: newEndTime,
			CeilingHit: ceilingHit,
			EventCount: eventCount,
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			return businessAbort{err: httpx.BidConflict()}
		}

		bidID, createdAt, err := tx.InsertAcceptedBid(ctx, auctionID, bidder.ID, amount, idempotencyKey)
		if err != nil {
			return err
		}

		var orderID *int64
		if ceilingHit {
			id, err := tx.InsertOrder(ctx, auctionID, bidder.ID, current.SellerID, amount)
			if err != nil {
				return err
			}
			orderID = &id
		}

		version := current.Version + eventCount
		result = Result{
			Bid: Bid{
				ID:        bidID,
				AuctionID: auctionID,
				User:      bidder,
				Amount:    amount,
				Status:    "accepted",
				CreatedAt: formatTime(createdAt, s.location),
			},
			AuctionVersion: version,
			CurrentPrice:   amount,
			CurrentLeader:  bidder,
			Extended:       extended,
			NewEndTime:     formatTime(newEndTime, s.location),
			ServerTime:     formatTime(txNow, s.location),
			CeilingHit:     ceilingHit,
			OrderID:        orderID,
		}

		responseJSON, err := encodeResult(result)
		if err != nil {
			return err
		}
		if err := tx.FinalizeIdempotency(ctx, bidder.ID, scope, idempotencyKey, responseJSON); err != nil {
			return err
		}

		events, err := buildEvents(current, result)
		if err != nil {
			return err
		}
		for _, event := range events {
			if err := tx.InsertOutbox(ctx, auctionID, event); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		var abort businessAbort
		if errors.As(err, &abort) {
			return Result{}, abort.err
		}
		return Result{}, err
	}
	return result, nil
}

func (s *Service) readIdempotency(
	ctx context.Context,
	userID int64,
	scope string,
	key string,
	requestHash string,
) (Result, bool, error) {
	record, ok, err := s.store.FindIdempotency(ctx, userID, scope, key)
	if err != nil || !ok {
		return Result{}, false, err
	}
	if record.RequestHash != requestHash {
		return Result{}, true, httpx.IdempotencyConflict()
	}
	if record.Status != "succeeded" || len(record.ResponseJSON) == 0 {
		return Result{}, false, nil
	}
	var result Result
	if err := json.Unmarshal(record.ResponseJSON, &result); err != nil {
		return Result{}, true, err
	}
	return result, true, nil
}

func buildEvents(snapshot AuctionSnapshot, result Result) ([]StoredEvent, error) {
	firstSeq := snapshot.Version + 1
	bidPayload, err := json.Marshal(map[string]any{
		"type":        "bid_update",
		"event_id":    eventID(snapshot.ID, firstSeq),
		"auction_id":  snapshot.ID,
		"seq":         firstSeq,
		"server_time": result.ServerTime,
		"data": map[string]any{
			"auction_version": result.AuctionVersion,
			"current_price":   result.CurrentPrice,
			"current_leader":  result.CurrentLeader,
			"latest_bid":      result.Bid,
		},
	})
	if err != nil {
		return nil, err
	}
	events := []StoredEvent{{
		EventType: "BidAccepted",
		EventSeq:  firstSeq,
		Payload:   bidPayload,
	}}

	if result.Extended {
		extendedPayload, err := json.Marshal(map[string]any{
			"type":        "auction_extended",
			"event_id":    eventID(snapshot.ID, firstSeq+1),
			"auction_id":  snapshot.ID,
			"seq":         firstSeq + 1,
			"server_time": result.ServerTime,
			"data": map[string]any{
				"new_end_time":     result.NewEndTime,
				"extended_seconds": snapshot.ExtendSeconds,
			},
		})
		if err != nil {
			return nil, err
		}
		events = append(events, StoredEvent{
			EventType: "AuctionExtended",
			EventSeq:  firstSeq + 1,
			Payload:   extendedPayload,
		})
	}

	if result.CeilingHit {
		endedPayload, err := json.Marshal(map[string]any{
			"type":        "auction_ended",
			"event_id":    eventID(snapshot.ID, firstSeq+1),
			"auction_id":  snapshot.ID,
			"seq":         firstSeq + 1,
			"server_time": result.ServerTime,
			"data": map[string]any{
				"winner":      result.CurrentLeader,
				"final_price": result.CurrentPrice,
				"order_id":    result.OrderID,
			},
		})
		if err != nil {
			return nil, err
		}
		events = append(events, StoredEvent{
			EventType: "AuctionEnded",
			EventSeq:  firstSeq + 1,
			Payload:   endedPayload,
		})
	}
	return events, nil
}

func hashRequest(auctionID int64, amount int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", auctionID, amount)))
	return hex.EncodeToString(sum[:])
}

func eventID(auctionID int64, seq int64) string {
	return fmt.Sprintf("evt_%d_%d", auctionID, seq)
}

func formatTime(value time.Time, location *time.Location) string {
	return value.In(location).Format(time.RFC3339Nano)
}

type businessAbort struct {
	err error
}

func (e businessAbort) Error() string {
	return e.err.Error()
}
