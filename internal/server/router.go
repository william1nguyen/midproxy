package server

import (
	"github.com/gin-gonic/gin"
	"github.com/william1nguyen/midproxy/internal/fetcher"
	"github.com/william1nguyen/midproxy/internal/handler"
)

func setupRouter(f *fetcher.Fetcher) *gin.Engine {
	r := gin.Default()
	r.GET("/health", handler.HandlerHealth)
	r.POST("/fetch", handler.NewFetchHandler(f))
	return r
}
