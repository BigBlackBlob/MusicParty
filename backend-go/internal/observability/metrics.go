package observability

import "github.com/prometheus/client_golang/prometheus"
import "github.com/prometheus/client_golang/prometheus/collectors"

type Metrics struct {
	Registry     *prometheus.Registry
	HTTPRequests *prometheus.CounterVec
	HTTPDuration *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_server_requests_total",
		Help: "Total HTTP requests handled by the MusicParty Go backend.",
	}, []string{"method", "uri", "status", "outcome"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_requests_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "uri", "status", "outcome"})
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), requests, duration)
	return &Metrics{Registry: registry, HTTPRequests: requests, HTTPDuration: duration}
}
