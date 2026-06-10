package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auction-system/server-go/internal/auction"
	"auction-system/server-go/internal/auth"
	"auction-system/server-go/internal/bid"
	"auction-system/server-go/internal/config"
	"auction-system/server-go/internal/order"
	httpx "auction-system/server-go/internal/http"
	"auction-system/server-go/internal/outbox"
	"auction-system/server-go/internal/realtime"
	"auction-system/server-go/internal/storage"
	"auction-system/server-go/internal/upload"
	"auction-system/server-go/internal/worker"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	// dev 环境下让错误响应携带真实错误信息，方便调试。
	httpx.DevMode = cfg.AppEnv == "dev"

	mysqlDB, err := storage.OpenMySQL(cfg)
	if err != nil {
		logger.Error("open mysql", "error", err)
		os.Exit(1)
	}
	defer mysqlDB.Close()

	// 启动时验证数据库可达，避免服务静默启动后首次请求才暴露连接问题。
	{
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()
		if err := mysqlDB.PingContext(pingCtx); err != nil {
			logger.Error("mysql ping failed", "error", err)
			os.Exit(1)
		}
	}

	redisClient := storage.OpenRedis(cfg)
	defer redisClient.Close()

	authRepo := auth.NewRepository(mysqlDB)
	authService := auth.NewService(authRepo)
	authHandler := auth.NewHandler(authService)

	auctionRepo := auction.NewRepository(mysqlDB)
	auctionService := auction.NewService(auctionRepo, cfg.Location)
	auctionHandler := auction.NewHandler(auctionService)

	bidRepo := bid.NewRepository(mysqlDB)
	bidLocker := bid.NewRedisLocker(redisClient)
	bidService := bid.NewService(bidRepo, bidLocker, cfg.Location)
	bidHandler := bid.NewHandler(bidService)

	orderRepo := order.NewRepository(mysqlDB)
	orderService := order.NewService(orderRepo, cfg.Location)
	orderHandler := order.NewHandler(orderService)

	provider := realtime.NewOutboxProvider(mysqlDB)
	// 将 WebSocket 房间的真实在线人数定期同步到 DB，供 HTTP 轮询端读取
	viewerSink := func(auctionID int64, count int) {
		_, _ = mysqlDB.ExecContext(context.Background(),
			"UPDATE auctions SET viewer_count = ? WHERE id = ?", count, auctionID)
	}
	hub := realtime.NewHub(provider, viewerSink)
	uploadHandler := upload.NewHandlerFromEnv()

	workerRepo := worker.NewRepository(mysqlDB, cfg.Location)
	lifecycleWorker := worker.NewRunner(workerRepo, logger, cfg.LifecycleTick, cfg.LifecycleBatchSize)

	outboxRepo := outbox.NewRepository(
		mysqlDB,
		cfg.OutboxMaxRetries,
		cfg.OutboxRetryBase,
		cfg.OutboxRetryMax,
	)
	outboxSink := outbox.NewChannelSink(cfg.OutboxBatchSize)
	outboxPublisher := outbox.NewPublisher(
		outboxRepo,
		outboxSink,
		&outbox.Metrics{},
		logger,
		cfg.OutboxPollInterval,
		cfg.OutboxBatchSize,
		cfg.OutboxPublishTimeout,
	)

	router := httpx.NewRouter(httpx.RouterDeps{
		Config:         cfg,
		Logger:         logger,
		MySQL:          mysqlDB,
		Redis:          redisClient,
		AuthHandler:    authHandler,
		AuctionHandler: auctionHandler,
		BidHandler:     bidHandler,
		OrderHandler:   orderHandler,
		AuthMW:         auth.Middleware(authRepo),
		OptionalAuthMW: auth.OptionalAuth(authRepo),
		RegisterRealtime: func(routes gin.IRoutes) {
			realtime.RegisterRoutes(routes, hub)
		},
		RegisterUpload: func(routes gin.IRoutes) {
			upload.RegisterRoutes(routes, uploadHandler)
		},
		StaticUploadDir: upload.StaticDirFromEnv(),
		StaticVideoDir:  "/tmp/auction-video",
	})

	server := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	backgroundCtx, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()

	go hub.Run(backgroundCtx)
	go forwardOutboxEvents(backgroundCtx, logger, outboxSink.Events(), hub)
	go lifecycleWorker.Run(backgroundCtx)
	go outboxPublisher.Run(backgroundCtx)

	go func() {
		logger.Info("server starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	stopBackground()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown", "error", err)
	}
}

func forwardOutboxEvents(
	ctx context.Context,
	logger *slog.Logger,
	events <-chan outbox.Event,
	hub *realtime.Hub,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			var envelope realtime.EventEnvelope
			if err := json.Unmarshal(event.Payload, &envelope); err != nil {
				logger.Warn(
					"drop malformed outbox event payload",
					"event_id", event.ID,
					"auction_id", event.AggregateID,
					"error", err,
				)
				continue
			}
			hub.Publish(envelope)
		}
	}
}
