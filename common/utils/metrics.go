package utils

import (
	"net/http"
	"potat-api/common"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	httpRequestCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_inbound_requests",
		Help: "Inbound requests to bot endpoints",
	}, []string{"host", "endpoint", "ip", "method", "status"})
)

func ObserveMetrics(config common.Config) error {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		httpRequestCounter,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	http.Handle("/metrics", promhttp.Handler())
	connString := config.Prometheus.Host + ":" + config.Prometheus.Port
	Info.Printf("Metrics listening on %s", connString)

	return http.ListenAndServe(connString, nil);
}

func ObserveInboundRequests(host, endpoint, ip, method, status string) {
	httpRequestCounter.WithLabelValues(
		host,
		endpoint,
		ip,
		method,
		status,
	).Inc()
}