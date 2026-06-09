package store

import (
	"context"
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

type SolveResult struct {
	UserAgent string   `json:"userAgent"`
	Cookies   []Cookie `json:"cookies"`
}

type Store struct {
	rdb      *redis.Client
	cacheTTL time.Duration
	maxRPS   int64
}

func New(rdb *redis.Client, cacheTTL time.Duration, maxRPS int64) *Store {
	return &Store{rdb: rdb, cacheTTL: cacheTTL, maxRPS: maxRPS}
}

func buildKey(prefix, value string) string {
	return prefix + ":" + value
}

func (s *Store) GetCache(ctx context.Context, url string) ([]byte, error) {
	return s.rdb.Get(ctx, buildKey("cache", url)).Bytes()
}

func (s *Store) SetCache(ctx context.Context, url string, data []byte) error {
	return s.rdb.Set(ctx, buildKey("cache", url), data, s.cacheTTL).Err()
}

func (s *Store) GetSolveResult(ctx context.Context, domain string) (*SolveResult, error) {
	k := buildKey("cookies", domain)
	data, err := s.rdb.LMove(ctx, k, k, "RIGHT", "LEFT").Result()
	if err != nil {
		return nil, err
	}
	var result SolveResult
	return &result, json.Unmarshal([]byte(data), &result)
}

func (s *Store) SetCookies(ctx context.Context, domain string, cookies []Cookie) error {
	data, err := json.Marshal(cookies)
	if err != nil {
		return err
	}
	k := buildKey("cookies", domain)
	s.rdb.LPush(ctx, k, data)
	return s.rdb.Expire(ctx, k, 20*time.Minute).Err()
}

func (s *Store) AllowRequest(ctx context.Context, domain string) bool {
	if s.maxRPS <= 0 {
		return true
	}
	k := buildKey("rl", domain)
	pipe := s.rdb.Pipeline()
	incr := pipe.Incr(ctx, k)
	pipe.Expire(ctx, k, time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return true
	}
	return incr.Val() <= s.maxRPS
}
