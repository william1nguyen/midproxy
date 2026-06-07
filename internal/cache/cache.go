package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/william1nguyen/midproxy/internal/redisclient"
)

type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

func New(rdb *redis.Client, ttl time.Duration) *Cache {
	return &Cache{
		rdb: rdb,
		ttl: ttl,
	}
}

func (c *Cache) Get(ctx context.Context, url string) ([]byte, error) {
	key := c.key(url)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return data, err
}

func (c *Cache) Set(ctx context.Context, url string, data []byte) error {
	key := c.key(url)
	return c.rdb.Set(ctx, key, data, c.ttl).Err()
}

func (c *Cache) key(url string) string {
	return redisclient.BuildRedisKey("cache:request", url)
}
