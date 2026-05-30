package main

import (
	"flag"
	"os"

	"github.com/rs/zerolog/log"

	"github.com/rs/zerolog"
	"github.com/william1nguyen/midproxy/internal/config"
)

func main() {
	configYamlFile := flag.String("config", "configs/config.yaml", "config yaml file")
	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg, err := config.Load(*configYamlFile)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	log.Info().Any("config", cfg).Msg("loaded config")
}
