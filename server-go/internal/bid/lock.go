package bid

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const releaseLockLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`

type Locker interface {
	Acquire(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string, value string) error
}

type RedisLocker struct {
	client *redis.Client
}

func NewRedisLocker(client *redis.Client) *RedisLocker {
	return &RedisLocker{client: client}
}

func (l *RedisLocker) Acquire(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return l.client.SetNX(ctx, key, value, ttl).Result()
}

func (l *RedisLocker) Release(ctx context.Context, key string, value string) error {
	return l.client.Eval(ctx, releaseLockLua, []string{key}, value).Err()
}
