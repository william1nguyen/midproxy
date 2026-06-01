package handler

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type FetchRequest struct {
	URL string `json:"url" binding:"required"`
}

type FetchResponse struct {
	HTML       string `json:"html"`
	StatusCode int    `json:"status_code"`
	URL        string `json:"url"`
}

func HandlerFetch(c *gin.Context) {
	var req FetchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	html, statusCode, err := fetchURL(c.Request.Context(), req.URL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "fetch_failed", "message": err.Error()})
		return
	}
	c.PureJSON(http.StatusOK, FetchResponse{HTML: html, StatusCode: statusCode, URL: req.URL})
}

func fetchURL(parent context.Context, url string) (string, int, error) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", 400, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 400, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 400, err
	}

	return string(body), resp.StatusCode, nil
}
