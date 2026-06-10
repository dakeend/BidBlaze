package worker

import "time"

type CycleResult struct {
	Started int
	Ended   int
	Orders  int
}

type EndResult struct {
	Ended   bool
	OrderID *int64
}

type lifecycleAuction struct {
	ID                    int64
	Title                 string
	SellerID              int64
	Status                string
	CurrentPrice          int64
	CurrentLeaderID       *int64
	CurrentLeaderNickname *string
	CurrentLeaderAvatar   *string
	EndTime               time.Time
	Version               int64
}
