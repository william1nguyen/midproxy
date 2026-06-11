package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/william1nguyen/midproxy/internal/config"
	"github.com/william1nguyen/midproxy/internal/fetch"
	"github.com/william1nguyen/midproxy/internal/proxy"
	"github.com/william1nguyen/midproxy/internal/solver"
	"github.com/william1nguyen/midproxy/internal/store"
)

func main() {
	configFile := flag.String("config", "configs/config.yaml", "config yaml file")
	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatal().Err(err).Msg("failed to connect redis")
	}
	defer rdb.Close()

	var slv *solver.Solver
	if cfg.Solver.Enabled {
		slv = solver.New(rdb, cfg.Solver.Timeout)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := proxy.NewServer(proxy.ServerConfig{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Manager:      proxy.NewManager(cfg.Proxies),
		FetchClient:  fetch.NewClient(cfg.Fetch.Timeout),
		Store:        store.New(rdb, cfg.Cache.TTL, cfg.RateLimit.MaxRPS),
		Solver:       slv,
		CacheEnabled: cfg.Cache.Enabled,
	})

	if err := srv.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
		log.Fatal().Err(err).Msg("server error")
	}

	log.Info().Msg("shutdown complete")
}
