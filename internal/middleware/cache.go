package middleware

import (
	"bytes"
	"context"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/gin-gonic/gin"
)

type ResponseCache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, data []byte) error
}

type cachingWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *cachingWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func CacheResponse(cache ResponseCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		params := GetFetchParams(c)
		if params == nil {
			c.Next()
			return
		}

		if data, err := cache.Get(c.Request.Context(), params.URL); err == nil && data != nil {
			c.Data(fhttp.StatusOK, "application/json", data)
			c.Abort()
			return
		}

		buf := &bytes.Buffer{}
		cw := &cachingWriter{ResponseWriter: c.Writer, body: buf}
		c.Writer = cw
		c.Next()

		if c.Writer.Status() == fhttp.StatusOK && cw.body.Len() > 0 {
			cache.Set(c.Request.Context(), params.URL, cw.body.Bytes())
		}
	}
}
