package server

import (
	"github.com/gin-gonic/gin"
	"github.com/william1nguyen/midproxy/internal/config"
	"github.com/william1nguyen/midproxy/internal/fetcher"
	"github.com/william1nguyen/midproxy/internal/handler"
	"github.com/william1nguyen/midproxy/internal/proxy"
)

func setupRouter(cfg *config.Config) *gin.Engine {
	proxyManager := proxy.NewManager(cfg.Proxy.URLs)
	f := fetcher.New(cfg.Server.Fetcher.Timeout, proxyManager)

	r := gin.Default()
	r.GET("/health", handler.HandlerHealth)
	r.POST("/fetch", handler.NewFetchHandler(f))
	return r
}
