package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/william1nguyen/midproxy/internal/cache"
	"github.com/william1nguyen/midproxy/internal/config"
	"github.com/william1nguyen/midproxy/internal/cookies"
	"github.com/william1nguyen/midproxy/internal/fetcher"
	"github.com/william1nguyen/midproxy/internal/proxy"
	"github.com/william1nguyen/midproxy/internal/ratelimit"
	"github.com/william1nguyen/midproxy/internal/redisclient"
)

type Server struct {
	httpServer *http.Server
}

func New(cfg *config.Config) *Server {
	rdb, err := redisclient.New(cfg.Redis)

	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect redis")
	}

	proxyManager := proxy.NewManager(cfg.Proxy.URLs)
	limiter := ratelimit.New(rdb, cfg.RateLimit.MaxRPSPerDomain)
	cacheClient := cache.New(rdb, cfg.Cache.TTL, cfg.Cache.Enabled)
	cookieStore := cookies.New(rdb)
	f := fetcher.New(cfg.Server.Fetcher.Timeout, proxyManager, cacheClient, cookieStore, limiter)

	router := setupRouter(f)

	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
			Handler:      router,
			ReadTimeout:  120 * time.Second,
			WriteTimeout: 120 * time.Second,
		},
	}
}

func (s *Server) Run() {
	go func() {
		log.Info().Str("addr", s.httpServer.Addr).Msg("server starting...")
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	s.waitForShutdown()
}

func (s *Server) waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("server shutdown error")
	}
	log.Info().Msg("server stopped")
}
