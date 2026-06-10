package order

import (
	"strconv"

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

func (h *Handler) ListSeller(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthorized())
		return
	}
	status := c.Query("status")
	page := clamp(atoi(c.Query("page"), 1))
	size := clamp(atoi(c.Query("size"), 20))
	data, err := h.service.ListBySeller(c.Request.Context(), user.ID, status, page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, data)
}

func (h *Handler) ListMine(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthorized())
		return
	}
	status := c.Query("status")
	page := clamp(atoi(c.Query("page"), 1))
	size := clamp(atoi(c.Query("size"), 20))
	data, err := h.service.ListByWinner(c.Request.Context(), user.ID, status, page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, data)
}

func (h *Handler) Get(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthorized())
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Fail(c, httpx.InvalidParam("订单 ID 非法"))
		return
	}
	role := "buyer"
	if c.Query("role") == "seller" {
		role = "seller"
	}
	order, err := h.service.Get(c.Request.Context(), id, user.ID, role)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"order": order})
}

func (h *Handler) Pay(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthorized())
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Fail(c, httpx.InvalidParam("订单 ID 非法"))
		return
	}
	order, err := h.service.Pay(c.Request.Context(), id, user.ID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"order": order})
}

func atoi(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func clamp(v int) int {
	if v > 100 {
		return 100
	}
	return v
}
