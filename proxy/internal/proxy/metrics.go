package proxy

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	RequestsTotal      *prometheus.CounterVec
	CacheHitsTotal     prometheus.Counter
	CacheMissesTotal   prometheus.Counter
	RequestDuration    *prometheus.HistogramVec
	GeminiDuration     prometheus.Histogram
	CircuitBreakerOpen *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "proxy_requests_total",
				Help: "Total number of HTTP requests processed",
			},
			[]string{"method", "status"},
		),
		CacheHitsTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "proxy_cache_hits_total",
				Help: "Total number of cache hits",
			},
		),
		CacheMissesTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "proxy_cache_misses_total",
				Help: "Total number of cache misses requiring Gemini call",
			},
		),
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "proxy_request_duration_seconds",
				Help:    "Latency of HTTP requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method"},
		),
		GeminiDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "proxy_gemini_duration_seconds",
				Help:    "Latency of Gemini API calls in seconds",
				Buckets: prometheus.DefBuckets,
			},
		),
		CircuitBreakerOpen: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "proxy_circuit_breaker_state",
				Help: "Circuit breaker state: 0=closed, 1=open, 2=half-open",
			},
			[]string{"target"},
		),
	}
}
