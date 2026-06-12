package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter interface {
	Allow(ctx context.Context, key string) bool
}

type Config struct {
	Strategy    string
	MaxRequests int64
	Window      time.Duration
	BucketSize  int64
	RefillRate  int64
}

func NewFromConfig(rdb *redis.Client, cfg Config) (Limiter, error) {
	switch cfg.Strategy {
	case "window", "":
		return NewFixedWindow(rdb, cfg.MaxRequests, cfg.Window), nil
	case "token_bucket":
		return NewTokenBucket(rdb, cfg.BucketSize, cfg.RefillRate), nil
	default:
		return nil, fmt.Errorf("unknown rate limit strategy: %s", cfg.Strategy)
	}
}
