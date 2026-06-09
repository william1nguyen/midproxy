package solver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type job struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Proxy string `json:"proxy"`
}

type Solver struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Solver {
	return &Solver{rdb: rdb}
}

func (s *Solver) Trigger(ctx context.Context, targetURL, proxyURL string) {
	id := newJobID()
	payload, _ := json.Marshal(job{ID: id, URL: targetURL, Proxy: proxyURL})

	if err := s.rdb.LPush(ctx, "queue:solve", payload).Err(); err != nil {
		log.Error().Err(err).Msg("failed to push solve job")
		return
	}

	log.Info().Str("id", id).Str("url", targetURL).Msg("solve job triggered")
}

func newJobID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
