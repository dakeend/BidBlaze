package auction

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	httpx "auction-system/server-go/internal/http"
)

type Service struct {
	repo     *Repository
	location *time.Location
}

func NewService(repo *Repository, location *time.Location) *Service {
	return &Service{repo: repo, location: location}
}

func (s *Service) Create(ctx context.Context, sellerID int64, req CreateRequest) (Auction, error) {
	params, err := s.validateCreate(sellerID, req)
	if err != nil {
		return Auction{}, err
	}
	id, err := s.repo.Create(ctx, params)
	if err != nil {
		return Auction{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, auctionID int64, sellerID int64, req UpdateRequest) (Auction, error) {
	existing, ok, err := s.repo.FindByID(ctx, auctionID)
	if err != nil {
		return Auction{}, err
	}
	if !ok {
		return Auction{}, httpx.AuctionNotFound()
	}
	if existing.SellerID != sellerID {
		return Auction{}, httpx.Forbidden()
	}
	if existing.Status != "pending" || !existing.StartTime.After(s.now()) {
		return Auction{}, httpx.AuctionNotPending()
	}

	params, err := s.validateUpdate(auctionID, sellerID, existing, req)
	if err != nil {
		return Auction{}, err
	}
	if err := s.repo.Update(ctx, params); err != nil {
		if errors.Is(err, errNoRowsAffected) {
			return Auction{}, httpx.AuctionNotPending()
		}
		return Auction{}, err
	}
	return s.Get(ctx, auctionID)
}

func (s *Service) List(ctx context.Context, query ListQuery) ([]Auction, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Size <= 0 {
		query.Size = 20
	}
	if query.Size > 100 {
		query.Size = 100
	}
	if query.Status != "" && !validStatus(query.Status) {
		return nil, 0, httpx.InvalidParam("拍卖状态非法")
	}

	rows, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	result := make([]Auction, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapAuction(row, s.location))
	}
	return result, total, nil
}

func (s *Service) Status(ctx context.Context, id int64) (StatusResponse, error) {
	row, ok, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return StatusResponse{}, err
	}
	if !ok {
		return StatusResponse{}, httpx.AuctionNotFound()
	}

	bids, err := s.repo.GetTopBids(ctx, id, 10)
	if err != nil {
		return StatusResponse{}, err
	}
	seq, err := s.repo.GetLastEventSeq(ctx, id)
	if err != nil {
		return StatusResponse{}, err
	}

	return StatusResponse{
		Auction:      mapAuction(row, s.location),
		TopBids:      bids,
		LastEventSeq: seq,
		ServerTime:   s.now().Format(time.RFC3339Nano),
	}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (Auction, error) {
	row, ok, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return Auction{}, err
	}
	if !ok {
		return Auction{}, httpx.AuctionNotFound()
	}
	return mapAuction(row, s.location), nil
}

func (s *Service) Cancel(ctx context.Context, auctionID int64, sellerID int64) (Auction, error) {
	if err := s.repo.Cancel(ctx, auctionID, sellerID); err != nil {
		switch {
		case errors.Is(err, errAuctionNotFound):
			return Auction{}, httpx.AuctionNotFound()
		case errors.Is(err, errForbidden):
			return Auction{}, httpx.Forbidden()
		case errors.Is(err, errEnded):
			return Auction{}, httpx.AuctionEnded()
		case errors.Is(err, errCancelled):
			return Auction{}, httpx.AuctionCancelled()
		case errors.Is(err, errInvalidState):
			return Auction{}, httpx.AuctionNotPending()
		default:
			return Auction{}, err
		}
	}
	return s.Get(ctx, auctionID)
}

func (s *Service) validateCreate(sellerID int64, req CreateRequest) (CreateParams, error) {
	if err := validateTitle(req.Title); err != nil {
		return CreateParams{}, err
	}
	if err := validateOptionalText(req.Description, 2000, "描述过长"); err != nil {
		return CreateParams{}, err
	}
	if err := validateOptionalURL(req.CoverURL); err != nil {
		return CreateParams{}, err
	}
	if err := validateImages(req.Images); err != nil {
		return CreateParams{}, err
	}
	if err := validateOptionalURL(req.StreamURL); err != nil {
		return CreateParams{}, err
	}
	if err := validatePrices(req.StartPrice, req.PriceStep, req.CeilingPrice); err != nil {
		return CreateParams{}, err
	}
	if req.DurationSeconds < 30 || req.DurationSeconds > 86400 {
		return CreateParams{}, httpx.InvalidParam("拍卖时长非法")
	}
	startAt, err := s.parseFuture(req.StartTime)
	if err != nil {
		return CreateParams{}, err
	}
	extendSeconds := valueOr(req.ExtendSeconds, 30)
	if extendSeconds < 10 || extendSeconds > 30 {
		return CreateParams{}, httpx.InvalidParam("延时时长非法")
	}
	extendThreshold := valueOr(req.ExtendThreshold, 30)
	if extendThreshold < 1 || extendThreshold > 300 {
		return CreateParams{}, httpx.InvalidParam("延时阈值非法")
	}
	endAt := startAt.Add(time.Duration(req.DurationSeconds) * time.Second)
	return CreateParams{
		CreateRequest:   req,
		SellerID:        sellerID,
		StartAt:         startAt,
		EndAt:           endAt,
		OriginalEndAt:   endAt,
		ExtendSec:       extendSeconds,
		ExtendThreshold: extendThreshold,
	}, nil
}

func (s *Service) validateUpdate(auctionID int64, sellerID int64, existing auctionRow, req UpdateRequest) (UpdateParams, error) {
	if req.Title != nil {
		if err := validateTitle(*req.Title); err != nil {
			return UpdateParams{}, err
		}
	}
	if err := validateOptionalText(req.Description, 2000, "描述过长"); err != nil {
		return UpdateParams{}, err
	}
	if err := validateOptionalURL(req.CoverURL); err != nil {
		return UpdateParams{}, err
	}
	if req.Images != nil {
		if err := validateImages(req.Images); err != nil {
			return UpdateParams{}, err
		}
	}
	if err := validateOptionalURL(req.StreamURL); err != nil {
		return UpdateParams{}, err
	}

	startPrice := existing.StartPrice
	if req.StartPrice != nil {
		startPrice = *req.StartPrice
	}
	priceStep := existing.PriceStep
	if req.PriceStep != nil {
		priceStep = *req.PriceStep
	}
	ceilingPrice := existing.CeilingPrice
	if req.CeilingPrice != nil {
		ceilingPrice = req.CeilingPrice
	}
	if err := validatePrices(startPrice, priceStep, ceilingPrice); err != nil {
		return UpdateParams{}, err
	}

	var startAt *time.Time
	effectiveStart := existing.StartTime
	if req.StartTime != nil {
		parsed, err := s.parseFuture(*req.StartTime)
		if err != nil {
			return UpdateParams{}, err
		}
		startAt = &parsed
		effectiveStart = parsed
	}

	var endAt *time.Time
	var originalEndAt *time.Time
	if req.StartTime != nil || req.DurationSeconds != nil {
		duration := int(existing.EndTime.Sub(existing.StartTime).Seconds())
		if req.DurationSeconds != nil {
			if *req.DurationSeconds < 30 || *req.DurationSeconds > 86400 {
				return UpdateParams{}, httpx.InvalidParam("拍卖时长非法")
			}
			duration = *req.DurationSeconds
		}
		calculated := effectiveStart.Add(time.Duration(duration) * time.Second)
		endAt = &calculated
		originalEndAt = &calculated
	}

	var extendSeconds *int
	if req.ExtendSeconds != nil {
		if *req.ExtendSeconds < 10 || *req.ExtendSeconds > 30 {
			return UpdateParams{}, httpx.InvalidParam("延时时长非法")
		}
		extendSeconds = req.ExtendSeconds
	}
	var extendThreshold *int
	if req.ExtendThreshold != nil {
		if *req.ExtendThreshold < 1 || *req.ExtendThreshold > 300 {
			return UpdateParams{}, httpx.InvalidParam("延时阈值非法")
		}
		extendThreshold = req.ExtendThreshold
	}

	return UpdateParams{
		UpdateRequest:   req,
		AuctionID:       auctionID,
		SellerID:        sellerID,
		StartAt:         startAt,
		EndAt:           endAt,
		OriginalEndAt:   originalEndAt,
		ExtendSec:       extendSeconds,
		ExtendThreshold: extendThreshold,
	}, nil
}

func (s *Service) parseFuture(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, httpx.InvalidParam("开始时间格式非法")
	}
	parsed = parsed.In(s.location)
	if !parsed.After(s.now()) {
		return time.Time{}, httpx.InvalidParam("开始时间必须晚于当前时间")
	}
	return parsed, nil
}

func (s *Service) now() time.Time {
	return time.Now().In(s.location)
}

func validateTitle(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > 128 {
		return httpx.InvalidParam("标题为空或过长")
	}
	return nil
}

func validateOptionalText(value *string, max int, msg string) error {
	if value != nil && utf8.RuneCountInString(*value) > max {
		return httpx.InvalidParam(msg)
	}
	return nil
}

func validateOptionalURL(value *string) error {
	if value == nil || *value == "" {
		return nil
	}
	parsed, err := url.ParseRequestURI(*value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return httpx.InvalidParam("URL 非法")
	}
	return nil
}

func validateImages(images []string) error {
	if len(images) > 9 {
		return httpx.InvalidParam("图片最多 9 张")
	}
	for _, imageURL := range images {
		if err := validateOptionalURL(&imageURL); err != nil {
			return err
		}
	}
	return nil
}

func validatePrices(startPrice int64, priceStep int64, ceilingPrice *int64) error {
	if startPrice < 0 {
		return httpx.InvalidParam("起拍价非法")
	}
	if priceStep <= 0 {
		return httpx.InvalidParam("加价幅度非法")
	}
	if ceilingPrice != nil && (*ceilingPrice <= 0 || *ceilingPrice < startPrice) {
		return httpx.InvalidParam("封顶价非法")
	}
	return nil
}

func validStatus(status string) bool {
	switch status {
	case "pending", "active", "ended", "cancelled":
		return true
	default:
		return false
	}
}

func valueOr(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

func parseInt64Param(value string) (*int64, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return nil, httpx.InvalidParam("ID 参数非法")
	}
	return &parsed, nil
}
