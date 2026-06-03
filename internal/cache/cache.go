package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/william1nguyen/midproxy/internal/redisclient"
)

type Entry struct {
	HTML       string `json:"html"`
	StatusCode int    `json:"status_code"`
}

type Cache struct {
	rdb     *redis.Client
	ttl     time.Duration
	enabled bool
}

func New(rdb *redis.Client, ttl time.Duration, enabled bool) *Cache {
	return &Cache{
		rdb:     rdb,
		ttl:     ttl,
		enabled: enabled,
	}
}

func (c *Cache) Get(ctx context.Context, url string) (*Entry, error) {
	if !c.enabled {
		return nil, nil
	}

	key := c.key(url)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var entry Entry
	return &entry, json.Unmarshal(data, &entry)
}

func (c *Cache) Set(ctx context.Context, url string, entry *Entry) error {
	if !c.enabled {
		return nil
	}
	key := c.key(url)
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal cache entry: %w", err)
	}
	return c.rdb.Set(ctx, key, data, c.ttl).Err()
}

func (c *Cache) key(url string) string {
	return redisclient.BuildRedisKey("cache:request", url)
}
