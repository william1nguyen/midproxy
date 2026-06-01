package server

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/william1nguyen/midproxy/internal/fetcher"
	"github.com/william1nguyen/midproxy/internal/handler"
)

func setupRouter() *gin.Engine {
	f := fetcher.New(30 * time.Second)

	router := gin.Default()
	router.GET("/health", handler.HandlerHealth)
	router.POST("/fetch", handler.NewFetchHandler(f))
	return router
}
