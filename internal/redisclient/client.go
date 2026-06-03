package redisclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/william1nguyen/midproxy/internal/config"
)

func New(cfg config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return rdb, nil
}

func BuildRedisKey(prefix string, key string) string {
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%s:%s", prefix, hex.EncodeToString(h[:]))
}
