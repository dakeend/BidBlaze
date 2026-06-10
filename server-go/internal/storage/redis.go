package storage

import (
	"auction-system/server-go/internal/config"

	"github.com/redis/go-redis/v9"
)

func OpenRedis(cfg config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
		DB:   cfg.RedisDB,
	})
}
