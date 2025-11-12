// Package telemetry provides Prometheus metrics exporters for comprehensive observability.
//
// Purpose:
//   This package implements Prometheus metrics for per-backend tracking, usage record
//   export, and buffer store monitoring. These metrics complement the OpenTelemetry
//   metrics in routing_metrics.go and are exported via the /metrics endpoint.
//
// Key Responsibilities:
//   - Track per-backend request metrics (count, errors, latency) with org/model labels
//   - Monitor usage record export metrics
//   - Track buffer store metrics
//   - Provide metrics for operational visibility
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-005 (Operational visibility and reliability)
//   - specs/006-api-router-service/spec.md#NFR-010 (RED metrics)
//   - specs/006-api-router-service/spec.md#NFR-011 (Trace spans)
//
package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// BackendRequestTotal tracks total requests per backend with organization and model labels.
	BackendRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_router_backend_requests_total",
			Help: "Total number of requests per backend",
		},
		[]string{"backend_id", "organization_id", "model", "status"}, // status: "success", "error"
	)

	// BackendRequestDuration tracks request latency per backend with organization and model labels.
	BackendRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_router_backend_request_duration_seconds",
			Help:    "Request latency per backend in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"backend_id", "organization_id", "model"},
	)

	// BackendErrorRate tracks error rate per backend with organization and model labels.
	BackendErrorRate = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_router_backend_errors_total",
			Help: "Total number of errors per backend",
		},
		[]string{"backend_id", "organization_id", "model", "error_type"}, // error_type: "timeout", "connection", "http_4xx", "http_5xx"
	)

	// UsageRecordsPublishedTotal tracks total usage records published to Kafka.
	UsageRecordsPublishedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_router_usage_records_published_total",
			Help: "Total number of usage records published to Kafka",
		},
		[]string{"organization_id", "model", "backend_id", "status"}, // status: "success", "error"
	)

	// UsageRecordsPublishedDuration tracks latency for publishing usage records.
	UsageRecordsPublishedDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_router_usage_records_publish_duration_seconds",
			Help:    "Latency for publishing usage records to Kafka in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"organization_id", "model"},
	)

	// UsageRecordsBufferedTotal tracks total usage records buffered to disk.
	UsageRecordsBufferedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_router_usage_records_buffered_total",
			Help: "Total number of usage records buffered to disk",
		},
		[]string{"organization_id", "model", "reason"}, // reason: "kafka_unavailable", "buffer_full"
	)

	// BufferStoreSize tracks the current number of records in the buffer store.
	BufferStoreSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "api_router_buffer_store_size",
			Help: "Current number of records in the buffer store",
		},
		[]string{"organization_id"},
	)

	// BufferStoreRetryTotal tracks total retry attempts for buffered records.
	BufferStoreRetryTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_router_buffer_store_retries_total",
			Help: "Total number of retry attempts for buffered records",
		},
		[]string{"organization_id", "status"}, // status: "success", "error"
	)

	// BufferStoreAge tracks the age of the oldest record in the buffer store.
	BufferStoreAge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "api_router_buffer_store_age_seconds",
			Help: "Age of the oldest record in the buffer store in seconds",
		},
		[]string{"organization_id"},
	)
)

// RecordBackendRequest records a backend request metric.
func RecordBackendRequest(backendID, organizationID, model string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}

	BackendRequestTotal.WithLabelValues(backendID, organizationID, model, status).Inc()
	BackendRequestDuration.WithLabelValues(backendID, organizationID, model).Observe(duration.Seconds())
}

// RecordBackendError records a backend error metric.
func RecordBackendError(backendID, organizationID, model, errorType string) {
	BackendErrorRate.WithLabelValues(backendID, organizationID, model, errorType).Inc()
	BackendRequestTotal.WithLabelValues(backendID, organizationID, model, "error").Inc()
}

// RecordUsageRecordPublished records a usage record publication metric.
func RecordUsageRecordPublished(organizationID, model, backendID string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}

	UsageRecordsPublishedTotal.WithLabelValues(organizationID, model, backendID, status).Inc()
	if success {
		UsageRecordsPublishedDuration.WithLabelValues(organizationID, model).Observe(duration.Seconds())
	}
}

// RecordUsageRecordBuffered records a usage record buffering metric.
func RecordUsageRecordBuffered(organizationID, model, reason string) {
	UsageRecordsBufferedTotal.WithLabelValues(organizationID, model, reason).Inc()
}

// SetBufferStoreSize sets the current buffer store size.
func SetBufferStoreSize(organizationID string, size int) {
	BufferStoreSize.WithLabelValues(organizationID).Set(float64(size))
}

// RecordBufferStoreRetry records a buffer store retry attempt.
func RecordBufferStoreRetry(organizationID string, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	BufferStoreRetryTotal.WithLabelValues(organizationID, status).Inc()
}

// SetBufferStoreAge sets the age of the oldest record in the buffer store.
func SetBufferStoreAge(organizationID string, age time.Duration) {
	BufferStoreAge.WithLabelValues(organizationID).Set(age.Seconds())
}

