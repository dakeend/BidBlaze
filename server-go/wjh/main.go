package main

import (
	"context"
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
	httpx "auction-system/server-go/internal/http"
	"auction-system/server-go/internal/outbox"
	"auction-system/server-go/internal/storage"
	"auction-system/server-go/internal/worker"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	mysqlDB, err := storage.OpenMySQL(cfg)
	if err != nil {
		logger.Error("open mysql", "error", err)
		os.Exit(1)
	}
	defer mysqlDB.Close()

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
	workerRepo := worker.NewRepository(mysqlDB, cfg.Location)
	lifecycleWorker := worker.NewRunner(
		workerRepo,
		logger,
		cfg.LifecycleTick,
		cfg.LifecycleBatchSize,
	)
	outboxRepo := outbox.NewRepository(
		mysqlDB,
		cfg.OutboxMaxRetries,
		cfg.OutboxRetryBase,
		cfg.OutboxRetryMax,
	)
	outboxSink := outbox.NewRedisSink(redisClient, cfg.OutboxRedisPrefix)
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
		AuthMW:         auth.Middleware(authRepo),
	})

	server := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	backgroundCtx, stopBackground := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		lifecycleWorker.Run(backgroundCtx)
	}()
	publisherDone := make(chan struct{})
	go func() {
		defer close(publisherDone)
		outboxPublisher.Run(backgroundCtx)
	}()

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
	select {
	case <-workerDone:
	case <-ctx.Done():
		logger.Warn("lifecycle worker shutdown timed out")
	}
	select {
	case <-publisherDone:
	case <-ctx.Done():
		logger.Warn("outbox publisher shutdown timed out")
	}
}
