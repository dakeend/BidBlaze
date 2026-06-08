package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"time"

	"auction-system/server-go/internal/realtime"
	"auction-system/server-go/internal/upload"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	provider, closeProvider := newRealtimeProvider()
	defer closeProvider()

	r := gin.Default()
	hub := realtime.NewHub(provider)
	hubCtx, cancelHub := context.WithCancel(context.Background())
	defer cancelHub()
	go hub.Run(hubCtx)

	corsCfg := cors.DefaultConfig()
	corsCfg.AllowOrigins = []string{"http://localhost:5173", "http://localhost:5174"}
	corsCfg.AllowHeaders = []string{
		"Origin", "Content-Type", "Authorization",
		"X-Request-Id", "Idempotency-Key", "X-Client-Type",
	}
	r.Use(cors.New(corsCfg))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"mysql":  "ok",
			"redis":  "ok",
		})
	})

	realtime.RegisterRoutes(r, hub)
	upload.RegisterRoutes(r, upload.NewHandlerFromEnv())
	r.Static("/static", upload.StaticDirFromEnv())

	if err := r.Run(":8080"); err != nil {
		panic(err)
	}
}

func newRealtimeProvider() (realtime.Provider, func()) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		return realtime.StaticProvider{}, func() {}
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		slog.Warn("mysql realtime provider disabled; falling back to static provider", "err", err)
		return realtime.StaticProvider{}, func() {}
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		slog.Warn("mysql realtime provider unavailable; falling back to static provider", "err", err)
		return realtime.StaticProvider{}, func() {}
	}

	return realtime.NewOutboxProvider(db), func() {
		_ = db.Close()
	}
}
