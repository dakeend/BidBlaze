package auction

import "time"

type UserLite struct {
	ID       int64   `json:"id"`
	Nickname string  `json:"nickname"`
	Avatar   *string `json:"avatar"`
}

type Auction struct {
	ID              int64     `json:"id"`
	Title           string    `json:"title"`
	Description     *string   `json:"description"`
	CoverURL        *string   `json:"cover_url"`
	Images          []string  `json:"images"`
	StreamURL       *string   `json:"stream_url"`
	StartPrice      int64     `json:"start_price"`
	PriceStep       int64     `json:"price_step"`
	CeilingPrice    *int64    `json:"ceiling_price"`
	CurrentPrice    int64     `json:"current_price"`
	CurrentLeader   *UserLite `json:"current_leader"`
	StartTime       string    `json:"start_time"`
	EndTime         string    `json:"end_time"`
	OriginalEndTime string    `json:"original_end_time"`
	ExtendSeconds   int       `json:"extend_seconds"`
	ExtendThreshold int       `json:"extend_threshold"`
	Status          string    `json:"status"`
	Version         int64     `json:"version"`
	ViewerCount     int       `json:"viewer_count"`
	BidCount        int64     `json:"bid_count"`
	Seller          UserLite  `json:"seller"`
	CreatedAt       string    `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
}

type auctionRow struct {
	ID                    int64
	Title                 string
	Description           *string
	CoverURL              *string
	ImagesJSON            *string
	StreamURL             *string
	StartPrice            int64
	PriceStep             int64
	CeilingPrice          *int64
	CurrentPrice          int64
	CurrentLeaderID       *int64
	CurrentLeaderNickname *string
	CurrentLeaderAvatar   *string
	StartTime             time.Time
	EndTime               time.Time
	OriginalEndTime       time.Time
	ExtendSeconds         int
	ExtendThreshold       int
	Status                string
	Version               int64
	ViewerCount           int
	BidCount              int64
	SellerID              int64
	SellerNickname        string
	SellerAvatar          *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type CreateRequest struct {
	Title           string   `json:"title"`
	Description     *string  `json:"description"`
	CoverURL        *string  `json:"cover_url"`
	Images          []string `json:"images"`
	StreamURL       *string  `json:"stream_url"`
	StartPrice      int64    `json:"start_price"`
	PriceStep       int64    `json:"price_step"`
	CeilingPrice    *int64   `json:"ceiling_price"`
	StartTime       string   `json:"start_time"`
	DurationSeconds int      `json:"duration_seconds"`
	ExtendSeconds   *int     `json:"extend_seconds"`
	ExtendThreshold *int     `json:"extend_threshold"`
}

type UpdateRequest struct {
	Title           *string  `json:"title"`
	Description     *string  `json:"description"`
	CoverURL        *string  `json:"cover_url"`
	Images          []string `json:"images"`
	StreamURL       *string  `json:"stream_url"`
	StartPrice      *int64   `json:"start_price"`
	PriceStep       *int64   `json:"price_step"`
	CeilingPrice    *int64   `json:"ceiling_price"`
	StartTime       *string  `json:"start_time"`
	DurationSeconds *int     `json:"duration_seconds"`
	ExtendSeconds   *int     `json:"extend_seconds"`
	ExtendThreshold *int     `json:"extend_threshold"`
}

type ListQuery struct {
	Status   string
	SellerID *int64
	Page     int
	Size     int
}

type CreateParams struct {
	CreateRequest
	SellerID        int64
	StartAt         time.Time
	EndAt           time.Time
	OriginalEndAt   time.Time
	ExtendSec       int
	ExtendThreshold int
}

type UpdateParams struct {
	UpdateRequest
	AuctionID       int64
	SellerID        int64
	StartAt         *time.Time
	EndAt           *time.Time
	OriginalEndAt   *time.Time
	ExtendSec       *int
	ExtendThreshold *int
}
