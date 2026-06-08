package solver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/william1nguyen/midproxy/internal/store"
)

type job struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Proxy string `json:"proxy"`
}

type reply struct {
	Cookies []store.Cookie `json:"cookies"`
	Error   string         `json:"error,omitempty"`
}

type Solver struct {
	rdb     *redis.Client
	timeout time.Duration
}

func New(rdb *redis.Client, timeout time.Duration) *Solver {
	return &Solver{rdb: rdb, timeout: timeout}
}

func (s *Solver) Solve(ctx context.Context, targetURL, proxyURL string) ([]store.Cookie, error) {
	id := newJobID()

	payload, _ := json.Marshal(job{ID: id, URL: targetURL, Proxy: proxyURL})

	if err := s.rdb.LPush(ctx, "queue:solve", payload).Err(); err != nil {
		return nil, fmt.Errorf("push solve job: %w", err)
	}

	log.Debug().Str("id", id).Str("url", targetURL).Msg("solve job pushed")

	replyKey := "reply:" + id
	result, err := s.rdb.BRPop(ctx, s.timeout, replyKey).Result()
	if err != nil {
		return nil, fmt.Errorf("solver timeout or error: %w", err)
	}

	// cleanup reply key
	s.rdb.Del(ctx, replyKey)

	var r reply
	if err := json.Unmarshal([]byte(result[1]), &r); err != nil {
		return nil, fmt.Errorf("decode solver reply: %w", err)
	}

	if r.Error != "" {
		return nil, fmt.Errorf("solver: %s", r.Error)
	}

	log.Debug().Str("id", id).Int("cookies", len(r.Cookies)).Msg("solve reply received")
	return r.Cookies, nil
}

func newJobID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
