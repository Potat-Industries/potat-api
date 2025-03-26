// Package utils provides utility functions and types for all routes.
package utils

import (
	"net/http"
	"time"

	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics is a struct that holds the Prometheus metrics for the application.
type Metrics struct {
	httpRequestCounter *prometheus.CounterVec
	socketGauge        prometheus.Gauge
}

// ObserveMetrics initializes and starts the Prometheus metrics server.
func ObserveMetrics(config common.Config) (*Metrics, *http.Server) {
	httpRequestCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_inbound_requests",
		Help: "Inbound requests to bot endpoints",
	}, []string{"host", "endpoint", "ip", "method", "status", "cachehit"})

	socketGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "socket_connections",
		Help: "Number of active socket connections",
	})

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		httpRequestCounter,
		socketGauge,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	http.Handle("/metrics", promhttp.Handler())
	connString := config.Prometheus.Host + ":" + config.Prometheus.Port
	logger.Info.Printf("Metrics listening on %s", connString)

	server := &http.Server{
		Addr:         connString,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	metrics := &Metrics{
		httpRequestCounter: httpRequestCounter,
		socketGauge:        socketGauge,
	}

	return metrics, server
}

// ObserveInboundRequests increments the counter for inbound requests to bot endpoints.
func (m *Metrics) ObserveInboundRequests(host, endpoint, ip, method, status, cachehit string) {
	m.httpRequestCounter.WithLabelValues(
		host,
		endpoint,
		ip,
		method,
		status,
		cachehit,
	).Inc()
}

// GaugeSocketConnections is a gauge to track the number of active socket connections.
func (m *Metrics) GaugeSocketConnections(value float64) {
	m.socketGauge.Set(value)
}
