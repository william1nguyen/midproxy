package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
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
	ProxyURL  string   `json:"proxyURL"`
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

type CachedResponse struct {
	StatusCode int         `json:"statusCode"`
	Header     http.Header `json:"header"`
	Body       string      `json:"body"`
}

func cacheKey(method, url string) string {
	return "cache:" + method + ":" + url
}

func (s *Store) GetCachedResponse(ctx context.Context, method, url string) (*CachedResponse, error) {
	data, err := s.rdb.Get(ctx, cacheKey(method, url)).Bytes()
	if err != nil {
		return nil, err
	}
	var resp CachedResponse
	return &resp, json.Unmarshal(data, &resp)
}

func (s *Store) SetCachedResponse(ctx context.Context, method, url string, resp *CachedResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, cacheKey(method, url), data, s.cacheTTL).Err()
}

func EncodeCachedResponse(statusCode int, header http.Header, body []byte) *CachedResponse {
	return &CachedResponse{
		StatusCode: statusCode,
		Header:     header,
		Body:       base64.StdEncoding.EncodeToString(body),
	}
}

func (c *CachedResponse) DecodeBody() ([]byte, error) {
	return base64.StdEncoding.DecodeString(c.Body)
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
