package auth

type User struct {
	ID       int64   `json:"id"`
	Nickname string  `json:"nickname"`
	Avatar   *string `json:"avatar"`
	Token    string  `json:"-"`
}

type LoginRequest struct {
	Nickname string  `json:"nickname"`
	Avatar   *string `json:"avatar"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
