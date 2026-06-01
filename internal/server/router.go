package server

import (
	"github.com/gin-gonic/gin"
	"github.com/william1nguyen/midproxy/internal/handler"
)

func setupRouter() *gin.Engine {
	router := gin.Default()
	router.GET("/health", handler.HandlerHealth)
	router.POST("/fetch", handler.HandlerFetch)
	return router
}
