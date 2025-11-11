package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var telemetryExporterFailures = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "shared_telemetry_export_failures_total",
		Help: "Number of telemetry exporter initialization failures by exporter protocol.",
	},
	[]string{"service_name", "exporter"},
)

func recordExporterFailure(serviceName, exporter string) {
	if serviceName == "" {
		serviceName = "unknown"
	}
	telemetryExporterFailures.WithLabelValues(serviceName, exporter).Inc()
}

// TelemetryExporterFailures exposes the failure counter for integration tests and dashboards.
func TelemetryExporterFailures() *prometheus.CounterVec {
	return telemetryExporterFailures
}
