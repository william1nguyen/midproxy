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
	"github.com/william1nguyen/midproxy/internal/config"
)

type Server struct {
	httpServer *http.Server
}

func New(cfg *config.Config) *Server {
	router := setupRouter(cfg)

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
