package bid

import (
	"encoding/json"
	"time"
)

type Bidder struct {
	ID       int64   `json:"id"`
	Nickname string  `json:"nickname"`
	Avatar   *string `json:"avatar"`
}

type Request struct {
	Amount int64 `json:"amount"`
}

type Bid struct {
	ID        int64  `json:"id"`
	AuctionID int64  `json:"auction_id"`
	User      Bidder `json:"user"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type Result struct {
	Bid            Bid    `json:"bid"`
	AuctionVersion int64  `json:"auction_version"`
	CurrentPrice   int64  `json:"current_price"`
	CurrentLeader  Bidder `json:"current_leader"`
	Extended       bool   `json:"extended"`
	NewEndTime     string `json:"new_end_time"`
	ServerTime     string `json:"server_time"`
	CeilingHit     bool   `json:"ceiling_hit,omitempty"`
	OrderID        *int64 `json:"order_id,omitempty"`
}

type AuctionSnapshot struct {
	ID              int64
	SellerID        int64
	Status          string
	StartPrice      int64
	PriceStep       int64
	CeilingPrice    *int64
	CurrentPrice    int64
	CurrentLeaderID *int64
	BidCount        int64
	EndTime         time.Time
	ExtendSeconds   int
	ExtendThreshold int
	Version         int64
}

type IdempotencyRecord struct {
	RequestHash  string
	ResponseJSON json.RawMessage
	Status       string
}

type AcceptParams struct {
	AuctionID  int64
	UserID     int64
	Amount     int64
	NewEndTime time.Time
	CeilingHit bool
	EventCount int64
}

type StoredEvent struct {
	EventType string
	EventSeq  int64
	Payload   json.RawMessage
}
