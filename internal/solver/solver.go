package solver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type job struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type Solver struct {
	rdb     *redis.Client
	lockTTL time.Duration
}

func New(rdb *redis.Client, lockTTL time.Duration) *Solver {
	return &Solver{rdb: rdb, lockTTL: lockTTL}
}

func solvingKey(domain string) string {
	return "solving:" + domain
}

func (s *Solver) Trigger(ctx context.Context, targetURL, domain string, force bool) int {
	id := newJobID()
	key := solvingKey(domain)

	if force {
		s.rdb.Set(ctx, key, id, s.lockTTL)
	} else {
		ok, err := s.rdb.SetNX(ctx, key, id, s.lockTTL).Result()
		if err != nil {
			log.Error().Err(err).Str("domain", domain).Msg("failed to acquire solve lock")
			return int(s.lockTTL.Seconds())
		}
		if !ok {
			remaining, _ := s.rdb.TTL(ctx, key).Result()
			log.Info().Str("domain", domain).Msg("solve already in progress, skipping")
			if remaining > 0 {
				return int(remaining.Seconds())
			}
			return int(s.lockTTL.Seconds())
		}
	}

	payload, _ := json.Marshal(job{ID: id, URL: targetURL})

	if err := s.rdb.LPush(ctx, "queue:solve", payload).Err(); err != nil {
		log.Error().Err(err).Msg("failed to push solve job")
		s.rdb.Del(ctx, key)
		return int(s.lockTTL.Seconds())
	}

	log.Info().Str("id", id).Str("url", targetURL).Msg("solve job triggered")
	return int(s.lockTTL.Seconds())
}

func newJobID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
