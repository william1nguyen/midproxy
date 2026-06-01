package server

import (
	"github.com/gin-gonic/gin"
	"github.com/william1nguyen/midproxy/internal/config"
	"github.com/william1nguyen/midproxy/internal/fetcher"
	"github.com/william1nguyen/midproxy/internal/handler"
	"github.com/william1nguyen/midproxy/internal/proxy"
)

func setupRouter(config *config.Config) *gin.Engine {
	proxyManager := proxy.NewManager(config.Proxy.URLs)
	f := fetcher.New(config.Server.Fetcher.Timeout, proxyManager)

	router := gin.Default()
	router.GET("/health", handler.HandlerHealth)
	router.POST("/fetch", handler.NewFetchHandler(f))
	return router
}
