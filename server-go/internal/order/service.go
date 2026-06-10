package order

import (
	"context"
	"errors"
	"time"

	httpx "auction-system/server-go/internal/http"
)

type Service struct {
	repo *Repository
	loc  *time.Location
}

func NewService(repo *Repository, loc *time.Location) *Service {
	return &Service{repo: repo, loc: loc}
}

func (s *Service) ListBySeller(ctx context.Context, sellerID int64, status string, page int, size int) (ListData, error) {
	rows, total, err := s.repo.ListBySeller(ctx, sellerID, status, page, size)
	if err != nil {
		return ListData{}, err
	}
	return s.buildList(rows, total, page, size), nil
}

func (s *Service) ListByWinner(ctx context.Context, winnerID int64, status string, page int, size int) (ListData, error) {
	rows, total, err := s.repo.ListByWinner(ctx, winnerID, status, page, size)
	if err != nil {
		return ListData{}, err
	}
	return s.buildList(rows, total, page, size), nil
}

func (s *Service) Get(ctx context.Context, orderID int64, userID int64, role string) (Order, error) {
	row, ok, err := s.repo.FindByID(ctx, orderID)
	if err != nil {
		return Order{}, err
	}
	if !ok {
		return Order{}, httpx.InvalidParam("订单不存在")
	}
	if (role == "seller" && row.SellerID != userID) || (role == "buyer" && row.WinnerID != userID) {
		return Order{}, httpx.Forbidden()
	}
	return mapOrder(row, s.loc), nil
}

func (s *Service) Pay(ctx context.Context, orderID int64, winnerID int64) (Order, error) {
	if err := s.repo.Pay(ctx, orderID, winnerID); err != nil {
		if errors.Is(err, errNotPayable) {
			return Order{}, httpx.InvalidParam("订单不可支付")
		}
		return Order{}, err
	}
	return s.Get(ctx, orderID, winnerID, "buyer")
}

func (s *Service) buildList(rows []orderRow, total int64, page int, size int) ListData {
	list := make([]Order, 0, len(rows))
	for _, row := range rows {
		list = append(list, mapOrder(row, s.loc))
	}
	return ListData{
		List:       list,
		Total:      total,
		Page:       page,
		Size:       size,
		ServerTime: time.Now().In(s.loc).Format(time.RFC3339Nano),
	}
}

func mapOrder(row orderRow, loc *time.Location) Order {
	o := Order{
		ID:        row.ID,
		AuctionID: row.AuctionID,
		Winner: UserLite{
			ID:       row.WinnerID,
			Nickname: row.WinnerNick,
			Avatar:   row.WinnerAvatar,
		},
		Seller: UserLite{
			ID:       row.SellerID,
			Nickname: row.SellerNick,
			Avatar:   row.SellerAvatar,
		},
		FinalPrice: row.FinalPrice,
		Status:     row.Status,
		CreatedAt:  row.CreatedAt.In(loc).Format(time.RFC3339Nano),
		UpdatedAt:  row.UpdatedAt.In(loc).Format(time.RFC3339Nano),
	}
	if row.PaidAt != nil {
		s := row.PaidAt.In(loc).Format(time.RFC3339Nano)
		o.PaidAt = &s
	}
	return o
}