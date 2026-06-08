package bid

import (
	"strconv"
	"strings"

	"auction-system/server-go/internal/auth"
	httpx "auction-system/server-go/internal/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Place(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthorized())
		return
	}
	auctionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || auctionID <= 0 {
		httpx.Fail(c, httpx.InvalidParam("拍卖 ID 非法"))
		return
	}
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, httpx.InvalidParam("请求体格式错误"))
		return
	}
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	requestID, _ := c.Get(httpx.RequestIDKey)
	requestIDValue, _ := requestID.(string)
	result, err := h.service.Place(
		c.Request.Context(),
		auctionID,
		Bidder{ID: user.ID, Nickname: user.Nickname, Avatar: user.Avatar},
		req.Amount,
		idempotencyKey,
		requestIDValue,
	)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}
