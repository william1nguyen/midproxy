package middleware

import (
	"context"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/gin-gonic/gin"
)

type RateLimiter interface {
	Allow(ctx context.Context, domain string) (bool, error)
}

func RateLimit(limiter RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		params := GetFetchParams(c)
		if params == nil {
			c.Next()
			return
		}

		ok, _ := limiter.Allow(c.Request.Context(), params.Domain)
		if !ok {
			c.AbortWithStatusJSON(fhttp.StatusTooManyRequests, gin.H{
				"error":   "rate_limited",
				"message": "slow down",
			})
			return
		}
		c.Next()
	}
}
