package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type FixedWindow struct {
	rdb         *redis.Client
	maxRequests int64
	window      time.Duration
}

func NewFixedWindow(rdb *redis.Client, maxRequests int64, window time.Duration) *FixedWindow {
	return &FixedWindow{rdb: rdb, maxRequests: maxRequests, window: window}
}

func (fw *FixedWindow) Allow(ctx context.Context, key string) bool {
	if fw.maxRequests <= 0 {
		return true
	}

	k := "rl:" + key
	count, err := fw.rdb.Incr(ctx, k).Result()
	if err != nil {
		return true
	}

	if count == 1 {
		fw.rdb.Expire(ctx, k, fw.window)
	}

	return count <= fw.maxRequests
}
