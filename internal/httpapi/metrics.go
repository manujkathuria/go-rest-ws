package httpapi

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics holds the Prometheus metric collectors for HTTP services.
type Metrics struct {
	Registry        *prometheus.Registry
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	InFlightGauge   *prometheus.GaugeVec
}

// NewMetrics initializes and registers bounded HTTP metrics in a dedicated registry.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	// Register standard process and Go runtime collectors
	reg.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)

	m := &Metrics{
		Registry: reg,
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "http",
				Name:      "requests_total",
				Help:      "Total count of HTTP requests processed, partitioned by method, route, and status code.",
			},
			[]string{"method", "route", "status_code"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "http",
				Name:      "request_duration_seconds",
				Help:      "Histogram of HTTP request latencies in seconds.",
				Buckets: []float64{
					0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
				},
			},
			[]string{"method", "route", "status_code"},
		),
		InFlightGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "http",
				Name:      "requests_in_flight",
				Help:      "Current number of in-flight HTTP requests.",
			},
			[]string{"method"},
		),
	}

	reg.MustRegister(m.RequestsTotal, m.RequestDuration, m.InFlightGauge)
	return m
}
