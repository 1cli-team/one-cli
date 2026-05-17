package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by method, route, and status.",
		},
		[]string{"method", "route", "status"},
	)
	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration by method, route, and status.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)
)

func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		status := strconv.Itoa(c.Writer.Status())
		labels := prometheus.Labels{
			"method": c.Request.Method,
			"route":  route,
			"status": status,
		}
		requestsTotal.With(labels).Inc()
		requestDuration.With(labels).Observe(time.Since(start).Seconds())
	}
}
