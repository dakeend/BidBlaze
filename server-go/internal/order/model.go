package order

type UserLite struct {
	ID       int64   `json:"id"`
	Nickname string  `json:"nickname"`
	Avatar   *string `json:"avatar"`
}

type Order struct {
	ID         int64    `json:"id"`
	AuctionID  int64    `json:"auction_id"`
	Winner     UserLite `json:"winner"`
	Seller     UserLite `json:"seller"`
	FinalPrice int64    `json:"final_price"`
	Status     string   `json:"status"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
	PaidAt     *string  `json:"paid_at"`
}

type ListData struct {
	List       []Order `json:"list"`
	Total      int64   `json:"total"`
	Page       int     `json:"page"`
	Size       int     `json:"size"`
	ServerTime string  `json:"server_time"`
}
