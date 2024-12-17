package utils

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func init() {
	http.Handle("/metrics", promhttp.Handler())
	Debug.Println("Metrics server started on :2112/metrics")
	Debug.Fatal(http.ListenAndServe(":2112", nil))
}