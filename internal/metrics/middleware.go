package metrics

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// Middleware returns a Gin middleware that records HTTP request metrics.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		HTTPRequestsInFlight.Inc()
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", c.Writer.Status())
		path := normalizePath(c.FullPath())

		HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
		HTTPRequestsInFlight.Dec()
	}
}

// normalizePath returns the route pattern or "unknown" for unmatched routes.
// This prevents high-cardinality labels from path parameters.
func normalizePath(path string) string {
	if path == "" {
		return "unknown"
	}
	return path
}
