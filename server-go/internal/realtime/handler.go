package realtime

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkOrigin,
}

var defaultWSAllowedOrigins = []string{
	"http://localhost:5173",
	"http://localhost:5174",
	"http://127.0.0.1:5173",
	"http://127.0.0.1:5174",
}

func checkOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	return originAllowed(origin, configuredWSAllowedOrigins())
}

func configuredWSAllowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("WS_ALLOWED_ORIGINS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("CORS_ORIGINS"))
	}
	if raw == "" {
		return defaultWSAllowedOrigins
	}
	return parseAllowedOrigins(raw)
}

func parseAllowedOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func originAllowed(origin string, allowedOrigins []string) bool {
	for _, allowedOrigin := range allowedOrigins {
		if allowedOrigin == "*" || strings.EqualFold(origin, allowedOrigin) {
			return true
		}
	}
	return false
}

func RegisterRoutes(router gin.IRoutes, hub *Hub) {
	router.GET("/ws/auction/:id", hub.ServeAuction)
	router.GET("/api/auctions/:id/events", hub.ServeEvents)
}

func (h *Hub) ServeAuction(c *gin.Context) {
	auctionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || auctionID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "msg": "invalid auction id", "data": nil})
		return
	}

	lastSeq := int64(0)
	if rawLastSeq := c.Query("last_seq"); rawLastSeq != "" {
		lastSeq, err = strconv.ParseInt(rawLastSeq, 10, 64)
		if err != nil || lastSeq < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "msg": "invalid last_seq", "data": nil})
			return
		}
	}

	// TODO(prod): replace with JWT or safer WS auth. Empty token is anonymous read-only.
	token := c.Query("token")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := NewClient(h, auctionID, token, lastSeq, conn)

	go client.writePump()
	for _, event := range h.replayOrSnapshot(c.Request.Context(), auctionID, lastSeq) {
		client.enqueueEvent(event)
	}
	h.Register(client)
	client.readPump()
}

func (h *Hub) ServeEvents(c *gin.Context) {
	auctionID, err := parsePositiveInt64(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "msg": "invalid auction id", "data": nil})
		return
	}

	afterSeq, err := parseRequiredNonNegativeInt64(c.Query("after_seq"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "msg": "invalid after_seq", "data": nil})
		return
	}

	limit, err := parseOptionalLimit(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "msg": "invalid limit", "data": nil})
		return
	}

	replay, err := h.provider.EventsAfter(c.Request.Context(), auctionID, afterSeq, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 9999, "msg": "internal error", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": gin.H{
			"events":            replay.Events,
			"has_more":          replay.HasMore,
			"snapshot_required": replay.SnapshotRequired,
			"server_time":       nowServerTime(),
		},
	})
}

func parsePositiveInt64(raw string) (int64, error) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid positive int64")
	}
	return value, nil
}

func parseRequiredNonNegativeInt64(raw string) (int64, error) {
	if raw == "" {
		return 0, errors.New("missing int64")
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, errors.New("invalid non-negative int64")
	}
	return value, nil
}

func parseOptionalLimit(raw string) (int, error) {
	if raw == "" {
		return defaultReplayLimit, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid limit")
	}
	return normalizeReplayLimit(value), nil
}
