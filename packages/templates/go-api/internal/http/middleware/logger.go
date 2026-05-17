package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		log.Info(
			"http request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.FullPath()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("request_id", requestID(c)),
		)
	}
}

func requestID(c *gin.Context) string {
	value, ok := c.Get(RequestIDKey)
	if !ok {
		return ""
	}
	if requestID, ok := value.(string); ok {
		return requestID
	}
	return ""
}
