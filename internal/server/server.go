package server

import (
	"context"
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

func New(config *config.Config) *Server {
	router := setupRouter()

	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", config.Server.Port),
			Handler:      router,
			ReadTimeout:  120 * time.Second,
			WriteTimeout: 120 * time.Second,
		},
	}
}

func (server *Server) Run() {
	go func() {
		log.Info().Str("addr", server.httpServer.Addr).Msg("server starting...")
		if err := server.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	server.waitForShutdown()
}

func (server *Server) waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.httpServer.Shutdown(ctx)
	log.Info().Msg("server stopped")
}
