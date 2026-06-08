package auth

import (
	"net/http"

	httpx "auction-system/server-go/internal/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, httpx.InvalidParam("请求体格式错误"))
		return
	}
	resp, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) Me(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthorized())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": httpx.CodeOK,
		"msg":  "ok",
		"data": gin.H{"user": user},
	})
}
