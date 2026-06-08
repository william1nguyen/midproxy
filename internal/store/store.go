package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

type Store struct {
	rdb      *redis.Client
	cacheTTL time.Duration
	maxRPS   int64
}

func New(rdb *redis.Client, cacheTTL time.Duration, maxRPS int64) *Store {
	return &Store{rdb: rdb, cacheTTL: cacheTTL, maxRPS: maxRPS}
}

func key(prefix, raw string) string {
	h := sha256.Sum256([]byte(raw))
	return prefix + ":" + hex.EncodeToString(h[:8])
}

// Cache

func (s *Store) GetCache(ctx context.Context, url string) ([]byte, error) {
	return s.rdb.Get(ctx, key("cache", url)).Bytes()
}

func (s *Store) SetCache(ctx context.Context, url string, data []byte) error {
	return s.rdb.Set(ctx, key("cache", url), data, s.cacheTTL).Err()
}

// Cookies

func (s *Store) GetCookies(ctx context.Context, domain string) ([]Cookie, error) {
	data, err := s.rdb.Get(ctx, key("cookie", domain)).Bytes()
	if err != nil {
		return nil, err
	}
	var cookies []Cookie
	return cookies, json.Unmarshal(data, &cookies)
}

func (s *Store) SetCookies(ctx context.Context, domain string, cookies []Cookie) error {
	data, err := json.Marshal(cookies)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, key("cookie", domain), data, 24*time.Hour).Err()
}

// Rate limit

func (s *Store) AllowRequest(ctx context.Context, domain string) bool {
	if s.maxRPS <= 0 {
		return true
	}
	k := key("rl", domain)
	pipe := s.rdb.Pipeline()
	incr := pipe.Incr(ctx, k)
	pipe.Expire(ctx, k, time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return true // fail open
	}
	return incr.Val() <= s.maxRPS
}
