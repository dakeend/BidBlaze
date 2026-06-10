package http

import (
	"context"
	"database/sql"
	"log/slog"
	stdhttp "net/http"
	"time"

	"auction-system/server-go/internal/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type AuthHandler interface {
	Login(*gin.Context)
	Me(*gin.Context)
}

type AuctionHandler interface {
	Create(*gin.Context)
	Update(*gin.Context)
	List(*gin.Context)
	Get(*gin.Context)
	Status(*gin.Context)
	Cancel(*gin.Context)
}

type BidHandler interface {
	Place(*gin.Context)
}

type OrderHandler interface {
	ListSeller(*gin.Context)
	ListMine(*gin.Context)
	Get(*gin.Context)
	Pay(*gin.Context)
}

type RouterDeps struct {
	Config         config.Config
	Logger         *slog.Logger
	MySQL          *sql.DB
	Redis          *redis.Client
	AuthHandler    AuthHandler
	AuctionHandler AuctionHandler
	BidHandler     BidHandler
	OrderHandler   OrderHandler
	AuthMW         gin.HandlerFunc
	OptionalAuthMW gin.HandlerFunc
	RegisterRealtime func(gin.IRoutes)
	RegisterUpload   func(gin.IRoutes)
	StaticUploadDir  string
	StaticVideoDir   string
}

func NewRouter(deps RouterDeps) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestID())
	router.Use(accessLog(deps.Logger))
	router.Use(cors.New(cors.Config{
		AllowOrigins:     deps.Config.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-Id", "Idempotency-Key", "X-Client-Type"},
		ExposeHeaders:    []string{"X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(stdhttp.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/ready", readyHandler(deps.MySQL, deps.Redis))

	api := router.Group("/api")
	if deps.AuthHandler != nil {
		api.POST("/login", deps.AuthHandler.Login)
		if deps.AuthMW != nil {
			api.GET("/users/me", deps.AuthMW, deps.AuthHandler.Me)
		}
	}
	if deps.AuctionHandler != nil {
		listHandlers := make([]gin.HandlerFunc, 0, 2)
		listHandlers = append(listHandlers, deps.AuctionHandler.List)
		if deps.OptionalAuthMW != nil {
			listHandlers = append([]gin.HandlerFunc{deps.OptionalAuthMW}, listHandlers...)
		}
		api.GET("/auctions", listHandlers...)
		api.GET("/auctions/:id", deps.AuctionHandler.Get)
		api.GET("/auctions/:id/status", deps.AuctionHandler.Status)
		if deps.AuthMW != nil {
			api.POST("/auctions", deps.AuthMW, deps.AuctionHandler.Create)
			api.PUT("/auctions/:id", deps.AuthMW, deps.AuctionHandler.Update)
			api.POST("/auctions/:id/cancel", deps.AuthMW, deps.AuctionHandler.Cancel)
		}
	}
	if deps.BidHandler != nil && deps.AuthMW != nil {
		api.POST("/auctions/:id/bid", deps.AuthMW, deps.BidHandler.Place)
	}
	if deps.OrderHandler != nil && deps.AuthMW != nil {
		api.GET("/orders/seller", deps.AuthMW, deps.OrderHandler.ListSeller)
		api.GET("/orders/mine", deps.AuthMW, deps.OrderHandler.ListMine)
		api.GET("/orders/:id", deps.AuthMW, deps.OrderHandler.Get)
		api.POST("/orders/:id/pay", deps.AuthMW, deps.OrderHandler.Pay)
	}
	if deps.RegisterUpload != nil && deps.AuthMW != nil {
		deps.RegisterUpload(router.Group("", deps.AuthMW))
	}
	if deps.RegisterRealtime != nil {
		deps.RegisterRealtime(router)
	}
	if deps.StaticUploadDir != "" {
		router.Static("/static", deps.StaticUploadDir)
	}
	if deps.StaticVideoDir != "" {
		router.Static("/video", deps.StaticVideoDir)
	}

	return router
}

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = "req-" + uuid.NewString()
		}
		c.Set(RequestIDKey, requestID)
		c.Header("X-Request-Id", requestID)
		c.Next()
	}
}

func accessLog(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		requestID, _ := c.Get(RequestIDKey)
		logger.Info(
			"http_request",
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(started).Milliseconds(),
		)
	}
}

func readyHandler(db *sql.DB, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		mysqlStatus := "ok"
		redisStatus := "ok"
		if db == nil || db.PingContext(ctx) != nil {
			mysqlStatus = "error"
		}
		if redisClient == nil || redisClient.Ping(ctx).Err() != nil {
			redisStatus = "error"
		}

		status := stdhttp.StatusOK
		if mysqlStatus != "ok" || redisStatus != "ok" {
			status = stdhttp.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{
			"status": map[bool]string{true: "ok", false: "error"}[status == stdhttp.StatusOK],
			"mysql":  mysqlStatus,
			"redis":  redisStatus,
		})
	}
}
