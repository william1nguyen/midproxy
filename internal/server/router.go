package server

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/william1nguyen/midproxy/internal/cache"
	"github.com/william1nguyen/midproxy/internal/config"
	"github.com/william1nguyen/midproxy/internal/fetcher"
	"github.com/william1nguyen/midproxy/internal/handler"
	"github.com/william1nguyen/midproxy/internal/middleware"
	"github.com/william1nguyen/midproxy/internal/proxy"
	"github.com/william1nguyen/midproxy/internal/ratelimit"
)

func setupRouter(cfg config.Config, rdb *redis.Client) *gin.Engine {
	f := fetcher.New(cfg.Server.Fetcher.Timeout)
	m := proxy.NewManager(cfg.Proxy.URLs)
	l := ratelimit.New(rdb, cfg.RateLimit.MaxRPSPerDomain)
	c := cache.New(rdb, cfg.Cache.TTL)

	fetchHandler := handler.NewFetchHandler(handler.Deps{
		Fetcher:     f,
		ProxyPicker: &proxyPickerAdapter{proxyManager: m},
		CFDetector:  cfDetectorAdapter{},
	})

	r := gin.Default()
	r.GET("/health", handler.HandlerHealth)

	fetch := r.Group("/fetch")
	fetch.Use(middleware.WithFetchParams())
	fetch.Use(middleware.RateLimit(l))
	if cfg.Cache.Enabled {
		fetch.Use(middleware.CacheResponse(c))
	}
	fetch.POST("", fetchHandler.Handle)
	return r
}
