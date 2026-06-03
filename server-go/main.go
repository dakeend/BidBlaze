package main

import (
	"context"
	"net/http"

	"auction-system/server-go/internal/realtime"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// 这是 D1 之前的占位骨架，提供契约对齐的 /health、/ready 与 Role B 的 WS 网关骨架。
// 完整实现见 docs/tasks/backend-agent-tasks.md「Task A」：
//   - 统一响应结构 + 错误码（contract-v2.md §1.1/§1.2）
//   - X-Request-Id 追踪 middleware
//   - /ready 真正探测 MySQL + Redis
//   - B1 mock login 桩 POST /api/login（dev-setup.md §5）
// Role A 在 Task A 中按分层（handler/service/domain/repository）重建，不要在本文件堆业务逻辑。

func main() {
	r := gin.Default()
	hub := realtime.NewHub(nil)
	hubCtx, cancelHub := context.WithCancel(context.Background())
	defer cancelHub()
	go hub.Run(hubCtx)

	// CORS 白名单按 dev-setup.md §2；生产用 CORS_ORIGINS 覆盖。
	corsCfg := cors.DefaultConfig()
	corsCfg.AllowOrigins = []string{"http://localhost:5173", "http://localhost:5174"}
	corsCfg.AllowHeaders = []string{
		"Origin", "Content-Type", "Authorization",
		"X-Request-Id", "Idempotency-Key", "X-Client-Type",
	}
	r.Use(cors.New(corsCfg))

	// 进程存活检查，不访问外部依赖（contract-v2.md §2.7）。
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 就绪检查雏形。TODO(Task A): 真正探测 MySQL + Redis 后再返回 ok。
	r.GET("/ready", func(c *gin.Context) {
	// 就绪检查雏形。TODO(Task A): 真正探测 MySQL + Redis 后再返回 ok。
	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"mysql":  "ok",
			"redis":  "ok",
			"status": "ok",
			"mysql":  "ok",
			"redis":  "ok",
		})
	})

	realtime.RegisterRoutes(r, hub)

	if err := r.Run(":8080"); err != nil {
		panic(err)
	}
}
