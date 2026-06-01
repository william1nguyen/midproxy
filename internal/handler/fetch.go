package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/william1nguyen/midproxy/internal/fetcher"
)

type FetchRequest struct {
	URL string `json:"url" binding:"required"`
}

type FetchResponse struct {
	HTML       string `json:"html"`
	StatusCode int    `json:"status_code"`
	URL        string `json:"url"`
}

func NewFetchHandler(fetcher *fetcher.Fetcher) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req FetchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
			return
		}

		html, statusCode, err := fetcher.Fetch(c.Request.Context(), req.URL, nil, "", nil)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "fetch_failed", "message": err.Error()})
			return
		}
		c.PureJSON(http.StatusOK, FetchResponse{HTML: html, StatusCode: statusCode, URL: req.URL})
	}
}
