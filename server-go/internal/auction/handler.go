package auction

import (
	"strconv"
	"time"

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

func (h *Handler) Create(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthorized())
		return
	}
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, httpx.InvalidParam("请求体格式错误"))
		return
	}
	auction, err := h.service.Create(c.Request.Context(), user.ID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"auction": auction})
}

func (h *Handler) Update(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthorized())
		return
	}
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, httpx.InvalidParam("请求体格式错误"))
		return
	}
	auction, err := h.service.Update(c.Request.Context(), id, user.ID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"auction": auction})
}

func (h *Handler) List(c *gin.Context) {
	var sellerID *int64
	if c.Query("seller_id") == "me" {
		if user, ok := auth.CurrentUser(c); ok {
			sellerID = &user.ID
		}
	} else {
		var err error
		sellerID, err = parseInt64Param(c.Query("seller_id"))
		if err != nil {
			httpx.Fail(c, err)
			return
		}
	}
	query := ListQuery{
		Status:   c.Query("status"),
		SellerID: sellerID,
		Page:     parsePositiveInt(c.Query("page"), 1),
		Size:     clampSize(parsePositiveInt(c.Query("size"), 20)),
	}
	list, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"list":        list,
		"total":       total,
		"page":        query.Page,
		"size":        query.Size,
		"server_time": time.Now().Format(time.RFC3339Nano),
	})
}

func (h *Handler) Get(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	auction, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"auction": auction})
}

// Status 返回拍卖快照（含当前价、Top 出价、事件序号），供移动端初始化。
func (h *Handler) Status(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	resp, err := h.service.Status(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) Cancel(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthorized())
		return
	}
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	auction, err := h.service.Cancel(c.Request.Context(), id, user.ID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"auction": auction})
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, httpx.InvalidParam("ID 参数非法")
	}
	return id, nil
}

func parsePositiveInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func clampSize(size int) int {
	if size > 100 {
		return 100
	}
	return size
}
