package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenBucket struct {
	rdb        *redis.Client
	bucketSize int64
	refillRate int64
}

func NewTokenBucket(rdb *redis.Client, bucketSize int64, refillRate int64) *TokenBucket {
	return &TokenBucket{rdb: rdb, bucketSize: bucketSize, refillRate: refillRate}
}

func (tb *TokenBucket) Allow(ctx context.Context, key string) bool {
	if tb.bucketSize <= 0 {
		return true
	}

	k := "rl:tb:" + key
	now := time.Now().UnixMilli()

	txf := func(tx *redis.Tx) error {
		vals, err := tx.HMGet(ctx, k, "tokens", "last").Result()
		if err != nil {
			return err
		}

		var tokens float64
		var last int64

		if vals[0] == nil {
			tokens = float64(tb.bucketSize)
			last = now
		} else {
			fmt.Sscanf(vals[0].(string), "%f", &tokens)
			fmt.Sscanf(vals[1].(string), "%d", &last)
		}

		elapsed := float64(now-last) / 1000.0
		tokens += elapsed * float64(tb.refillRate)
		if tokens > float64(tb.bucketSize) {
			tokens = float64(tb.bucketSize)
		}

		if tokens < 1 {
			return redis.TxFailedErr
		}

		tokens--
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.HSet(ctx, k, "tokens", fmt.Sprintf("%.2f", tokens), "last", now)
			pipe.Expire(ctx, k, time.Duration(tb.bucketSize/tb.refillRate+1)*time.Second)
			return nil
		})
		return err
	}

	for range 3 {
		err := tb.rdb.Watch(ctx, txf, "rl:tb:"+key)
		if err == nil {
			return true
		}
		if err == redis.TxFailedErr {
			return false
		}
	}
	return true
}
