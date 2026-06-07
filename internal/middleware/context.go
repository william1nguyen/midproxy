package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"

	fhttp "github.com/bogdanfinn/fhttp"

	"github.com/gin-gonic/gin"
)

const requestParamKey = "params"

type FetchParams struct {
	URL     string            `json:"url"`
	Browser string            `json:"browser"`
	Headers map[string]string `json:"headers"`
	Domain  string
}

func WithFetchParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(fhttp.StatusBadRequest, gin.H{
				"error": "read_body_failed",
			})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		var params FetchParams
		if err := json.Unmarshal(body, &params); err != nil || params.URL == "" {
			c.AbortWithStatusJSON(fhttp.StatusBadRequest, gin.H{
				"error":   "invalid_request",
				"message": "url is required",
			})
		}

		if u, err := url.Parse(params.URL); err == nil {
			params.Domain = u.Hostname()
		}

		c.Set(requestParamKey, &params)
		c.Next()
	}
}

func GetFetchParams(c *gin.Context) *FetchParams {
	v, ok := c.Get(requestParamKey)
	if !ok {
		return nil
	}
	p, _ := v.(*FetchParams)
	return p
}
