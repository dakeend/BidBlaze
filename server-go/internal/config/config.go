package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv      string
	AppPort     string
	AppTimezone string
	Location    *time.Location

	MySQLDSN     string
	MySQLMaxOpen int
	MySQLMaxIdle int

	RedisAddr string
	RedisDB   int

	LifecycleTick      time.Duration
	LifecycleBatchSize int

	OutboxPollInterval   time.Duration
	OutboxBatchSize      int
	OutboxMaxRetries     int
	OutboxRetryBase      time.Duration
	OutboxRetryMax       time.Duration
	OutboxPublishTimeout time.Duration
	OutboxRedisPrefix    string

	CORSOrigins []string
	LogLevel    slog.Level
}

func Load() Config {
	location := mustLocation(env("APP_TIMEZONE", "Asia/Shanghai"))
	return Config{
		AppEnv:       env("APP_ENV", "dev"),
		AppPort:      env("APP_PORT", "8080"),
		AppTimezone:  env("APP_TIMEZONE", "Asia/Shanghai"),
		Location:     location,
		MySQLDSN:     env("MYSQL_DSN", "auction:auction_root@tcp(127.0.0.1:3306)/auction?parseTime=true&loc=Asia%2FShanghai&charset=utf8mb4"),
		MySQLMaxOpen: envInt("MYSQL_MAX_OPEN", 50),
		MySQLMaxIdle: envInt("MYSQL_MAX_IDLE", 10),
		RedisAddr:    env("REDIS_ADDR", "127.0.0.1:6379"),
		RedisDB:      envInt("REDIS_DB", 0),
		LifecycleTick: time.Duration(
			envInt("LIFECYCLE_TICK_MS", 500),
		) * time.Millisecond,
		LifecycleBatchSize: envInt("LIFECYCLE_BATCH_SIZE", 100),
		OutboxPollInterval: time.Duration(
			envInt("OUTBOX_POLL_INTERVAL_MS", 200),
		) * time.Millisecond,
		OutboxBatchSize:  envInt("OUTBOX_BATCH_SIZE", 100),
		OutboxMaxRetries: envInt("OUTBOX_MAX_RETRIES", 10),
		OutboxRetryBase:  time.Duration(envInt("OUTBOX_RETRY_BASE_MS", 200)) * time.Millisecond,
		OutboxRetryMax:   time.Duration(envInt("OUTBOX_RETRY_MAX_MS", 5000)) * time.Millisecond,
		OutboxPublishTimeout: time.Duration(
			envInt("OUTBOX_PUBLISH_TIMEOUT_MS", 1000),
		) * time.Millisecond,
		OutboxRedisPrefix: env("OUTBOX_REDIS_PREFIX", "auction:events:"),
		CORSOrigins:       envCSV("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174"),
		LogLevel:          logLevel(env("LOG_LEVEL", "info")),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envCSV(key, fallback string) []string {
	value := env(key, fallback)
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func mustLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err == nil {
		return location
	}
	return time.Local
}

func logLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
