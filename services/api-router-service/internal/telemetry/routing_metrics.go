// Package telemetry provides routing-specific metrics and alerting.
//
// Purpose:
//   This package implements metrics collection and alerting for routing decisions,
//   backend health, and failover events. It provides observability for the routing engine.
//
// Key Responsibilities:
//   - Track routing decision metrics
//   - Monitor backend health metrics
//   - Emit alerts for routing failures
//   - Provide routing performance metrics
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-003 (Intelligent routing and fallback)
//   - specs/006-api-router-service/spec.md#NFR-010 (RED metrics)
//   - specs/006-api-router-service/spec.md#NFR-012 (Alerting thresholds)
//
package telemetry

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// RoutingMetrics tracks routing-related metrics and alerts.
type RoutingMetrics struct {
	logger *zap.Logger

	// Metrics
	requestsTotal          metric.Int64Counter
	requestsByBackend      metric.Int64Counter
	requestsByDecision     metric.Int64Counter
	failoverCount          metric.Int64Counter
	backendHealthStatus   metric.Int64UpDownCounter
	backendLatency         metric.Float64Histogram
	routingDecisionLatency metric.Float64Histogram

	// Alert thresholds
	failoverThreshold      int
	errorRateThreshold     float64
	latencyThreshold       time.Duration

	mu sync.RWMutex
}

// NewRoutingMetrics creates a new routing metrics collector.
func NewRoutingMetrics(logger *zap.Logger) (*RoutingMetrics, error) {
	meter := otel.Meter("api-router-service")

	requestsTotal, err := meter.Int64Counter(
		"router_requests_total",
		metric.WithDescription("Total number of routing requests"),
	)
	if err != nil {
		return nil, err
	}

	requestsByBackend, err := meter.Int64Counter(
		"router_requests_by_backend_total",
		metric.WithDescription("Total routing requests by backend"),
	)
	if err != nil {
		return nil, err
	}

	requestsByDecision, err := meter.Int64Counter(
		"router_requests_by_decision_total",
		metric.WithDescription("Total routing requests by decision type"),
	)
	if err != nil {
		return nil, err
	}

	failoverCount, err := meter.Int64Counter(
		"router_failover_total",
		metric.WithDescription("Total number of failover events"),
	)
	if err != nil {
		return nil, err
	}

	backendHealthStatus, err := meter.Int64UpDownCounter(
		"router_backend_health_status",
		metric.WithDescription("Backend health status (1=healthy, 0=degraded, -1=unhealthy)"),
	)
	if err != nil {
		return nil, err
	}

	backendLatency, err := meter.Float64Histogram(
		"router_backend_latency_seconds",
		metric.WithDescription("Backend request latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	routingDecisionLatency, err := meter.Float64Histogram(
		"router_decision_latency_seconds",
		metric.WithDescription("Routing decision latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	return &RoutingMetrics{
		logger:                 logger,
		requestsTotal:          requestsTotal,
		requestsByBackend:      requestsByBackend,
		requestsByDecision:     requestsByDecision,
		failoverCount:          failoverCount,
		backendHealthStatus:    backendHealthStatus,
		backendLatency:         backendLatency,
		routingDecisionLatency: routingDecisionLatency,
		failoverThreshold:      3,
		errorRateThreshold:     0.1, // 10%
		latencyThreshold:       3 * time.Second,
	}, nil
}

// RecordRoutingDecision records a routing decision metric.
func (m *RoutingMetrics) RecordRoutingDecision(
	backendID string,
	decisionType string,
	success bool,
	latency time.Duration,
) {
	attrs := []attribute.KeyValue{
		attribute.String("backend_id", backendID),
		attribute.String("decision_type", decisionType),
		attribute.Bool("success", success),
	}

	ctx := context.Background()

	// Increment total requests
	m.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Increment by backend
	m.requestsByBackend.Add(ctx, 1, metric.WithAttributes(
		attribute.String("backend_id", backendID),
	))

	// Increment by decision type
	m.requestsByDecision.Add(ctx, 1, metric.WithAttributes(
		attribute.String("decision_type", decisionType),
	))

	// Record failover if applicable
	if decisionType == "FAILOVER" {
		m.failoverCount.Add(ctx, 1, metric.WithAttributes(
			attribute.String("backend_id", backendID),
		))
		m.logger.Warn("failover event recorded",
			zap.String("backend_id", backendID),
			zap.Duration("latency", latency),
		)
	}

	// Record decision latency
	m.routingDecisionLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(attrs...))

	// Check alert thresholds
	m.checkAlertThresholds(backendID, decisionType, success, latency)
}

// RecordBackendHealth records backend health status.
func (m *RoutingMetrics) RecordBackendHealth(
	backendID string,
	status string,
	latency time.Duration,
) {
	var statusValue int64
	switch status {
	case "healthy":
		statusValue = 1
	case "degraded":
		statusValue = 0
	case "unhealthy":
		statusValue = -1
	default:
		statusValue = 0
	}

	attrs := []attribute.KeyValue{
		attribute.String("backend_id", backendID),
		attribute.String("status", status),
	}

	ctx := context.Background()
	m.backendHealthStatus.Add(ctx, statusValue, metric.WithAttributes(attrs...))

	// Record latency if healthy
	if status == "healthy" {
		m.backendLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(
			attribute.String("backend_id", backendID),
		))
	}

	// Alert on unhealthy status
	if status == "unhealthy" {
		m.logger.Error("backend marked as unhealthy",
			zap.String("backend_id", backendID),
			zap.Duration("latency", latency),
		)
	}
}

// RecordBackendRequest records a backend request metric.
func (m *RoutingMetrics) RecordBackendRequest(
	backendID string,
	success bool,
	latency time.Duration,
) {
	attrs := []attribute.KeyValue{
		attribute.String("backend_id", backendID),
		attribute.Bool("success", success),
	}

	ctx := context.Background()
	m.backendLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(attrs...))

	if !success {
		m.logger.Warn("backend request failed",
			zap.String("backend_id", backendID),
			zap.Duration("latency", latency),
		)
	}
}

// checkAlertThresholds checks if alert thresholds are exceeded.
func (m *RoutingMetrics) checkAlertThresholds(
	backendID string,
	decisionType string,
	success bool,
	latency time.Duration,
) {
	// Alert on high latency
	if latency > m.latencyThreshold {
		m.logger.Warn("routing latency threshold exceeded",
			zap.String("backend_id", backendID),
			zap.String("decision_type", decisionType),
			zap.Duration("latency", latency),
			zap.Duration("threshold", m.latencyThreshold),
		)
	}

	// Alert on failover
	if decisionType == "FAILOVER" {
		m.logger.Warn("failover occurred",
			zap.String("backend_id", backendID),
			zap.Duration("latency", latency),
		)
	}

	// Alert on failure
	if !success {
		m.logger.Error("routing request failed",
			zap.String("backend_id", backendID),
			zap.String("decision_type", decisionType),
			zap.Duration("latency", latency),
		)
	}
}

// SetFailoverThreshold sets the failover alert threshold.
func (m *RoutingMetrics) SetFailoverThreshold(threshold int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failoverThreshold = threshold
}

// SetErrorRateThreshold sets the error rate alert threshold.
func (m *RoutingMetrics) SetErrorRateThreshold(threshold float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorRateThreshold = threshold
}

// SetLatencyThreshold sets the latency alert threshold.
func (m *RoutingMetrics) SetLatencyThreshold(threshold time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencyThreshold = threshold
}

