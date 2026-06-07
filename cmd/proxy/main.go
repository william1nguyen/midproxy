package main

import (
	"flag"
	"os"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/william1nguyen/midproxy/internal/config"
	"github.com/william1nguyen/midproxy/internal/redisclient"
	"github.com/william1nguyen/midproxy/internal/server"
)

func main() {
	cfg := loadConfig()
	rdb := mustConnectRedis(cfg.Redis)
	srv := server.New(cfg, rdb)
	srv.Run()
}

func loadConfig() *config.Config {
	configYamlFile := flag.String("config", "configs/config.yaml", "config yaml file")
	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg, err := config.Load(*configYamlFile)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	log.Info().Any("config", cfg).Msg("loaded config")
	return cfg
}

func mustConnectRedis(cfg config.RedisConfig) *redis.Client {
	rdb, err := redisclient.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect redis")
	}
	return rdb
}
