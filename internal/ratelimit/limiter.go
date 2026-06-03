package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/william1nguyen/midproxy/internal/redisclient"
)

type Limiter struct {
	rdb    *redis.Client
	maxRPS int64
}

func New(rdb *redis.Client, maxRPS int64) *Limiter {
	return &Limiter{rdb: rdb, maxRPS: maxRPS}
}

func (l *Limiter) Allow(ctx context.Context, domain string) (bool, error) {
	key := l.key(domain)
	pipe := l.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		return true, err // fail over
	}

	count, _ := incr.Result()
	return count <= l.maxRPS, nil
}

func (limiter *Limiter) key(domain string) string {
	return redisclient.BuildRedisKey("ratelimit", domain)
}
