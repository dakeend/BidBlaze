package realtime

import (
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
