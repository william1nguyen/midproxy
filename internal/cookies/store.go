package cookies

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/redis/go-redis/v9"
	"github.com/william1nguyen/midproxy/internal/redisclient"
)

type Store struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func (s *Store) Set(ctx context.Context, domain string, cookies []*fhttp.Cookie, ttl time.Duration) error {
	key := s.key(domain)
	data, err := json.Marshal(cookies)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, key, data, ttl).Err()
}

func (s *Store) Get(ctx context.Context, domain string) ([]*fhttp.Cookie, error) {
	key := s.key(domain)
	data, err := s.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cookies []*fhttp.Cookie
	return cookies, json.Unmarshal(data, &cookies)
}

func (store *Store) key(domain string) string {
	return redisclient.BuildRedisKey("cookies:domain", domain)
}
