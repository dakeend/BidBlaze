package http

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"auction-system/server-go/internal/config"

	"github.com/gin-gonic/gin"
)

type noopAuthHandler struct{}

func (noopAuthHandler) Login(*gin.Context) {}
func (noopAuthHandler) Me(*gin.Context)    {}

type noopAuctionHandler struct{}

func (noopAuctionHandler) Create(*gin.Context) {}
func (noopAuctionHandler) Update(*gin.Context) {}
func (noopAuctionHandler) List(*gin.Context)   {}
func (noopAuctionHandler) Get(*gin.Context)    {}
func (noopAuctionHandler) Status(*gin.Context) {}
func (noopAuctionHandler) Cancel(*gin.Context) {}

type noopBidHandler struct{}

func (noopBidHandler) Place(*gin.Context) {}

func TestNewRouterRegistersIntegratedBackendRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterDeps{
		Config: config.Config{
			CORSOrigins: []string{"http://localhost:5173"},
		},
		Logger:         slog.Default(),
		AuthHandler:    noopAuthHandler{},
		AuctionHandler: noopAuctionHandler{},
		BidHandler:     noopBidHandler{},
		AuthMW: func(c *gin.Context) {
			c.Next()
		},
		OptionalAuthMW: func(c *gin.Context) {
			c.Next()
		},
		RegisterRealtime: func(routes gin.IRoutes) {
			routes.GET("/ws/auction/:id", func(*gin.Context) {})
			routes.GET("/api/auctions/:id/events", func(*gin.Context) {})
		},
		RegisterUpload: func(routes gin.IRoutes) {
			routes.POST("/api/uploads", func(*gin.Context) {})
		},
	})

	want := map[string]string{
		"GET /health":                   "",
		"GET /ready":                    "",
		"POST /api/login":               "",
		"GET /api/users/me":             "",
		"GET /api/auctions":             "",
		"GET /api/auctions/:id":         "",
		"GET /api/auctions/:id/status":  "",
		"POST /api/auctions":            "",
		"PUT /api/auctions/:id":         "",
		"POST /api/auctions/:id/cancel": "",
		"POST /api/auctions/:id/bid":    "",
		"GET /api/auctions/:id/events":  "",
		"POST /api/uploads":             "",
		"GET /ws/auction/:id":           "",
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = route.Handler
		}
	}
	for key, handler := range want {
		if handler == "" {
			t.Fatalf("missing route %s", key)
		}
	}
}

func TestReadyReportsUnavailableDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterDeps{
		Config: config.Config{
			CORSOrigins: []string{"http://localhost:5173"},
		},
		Logger: slog.Default(),
	})

	resp := performRequest(router, http.MethodGet, "/ready")
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.Code)
	}
}

func performRequest(router http.Handler, method string, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}
