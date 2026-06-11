package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenBucket struct {
	rdb    *redis.Client
	maxRPS int64
}

func NewTokenBucket(rdb *redis.Client, maxRPS int64) *TokenBucket {
	return &TokenBucket{rdb: rdb, maxRPS: maxRPS}
}

func (tb *TokenBucket) Allow(ctx context.Context, key string) bool {
	if tb.maxRPS <= 0 {
		return true
	}

	k := "rl:" + key
	now := time.Now().UnixMilli()

	vals, err := tb.rdb.HMGet(ctx, k, "tokens", "last").Result()
	if err != nil {
		return true
	}

	var tokens float64
	var last int64

	if vals[0] == nil {
		tokens = float64(tb.maxRPS)
		last = now
	} else {
		fmt.Sscanf(vals[0].(string), "%f", &tokens)
		fmt.Sscanf(vals[1].(string), "%d", &last)
	}

	elapsed := float64(now-last) / 1000.0
	tokens += elapsed * float64(tb.maxRPS)
	if tokens > float64(tb.maxRPS) {
		tokens = float64(tb.maxRPS)
	}

	if tokens < 1 {
		return false
	}

	tokens--
	tb.rdb.HSet(ctx, k, "tokens", fmt.Sprintf("%.2f", tokens), "last", now)
	tb.rdb.Expire(ctx, k, time.Duration(tb.maxRPS+1)*time.Second)
	return true
}
