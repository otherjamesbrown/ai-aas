// Package telemetry provides Prometheus metrics for limit denials.
//
// Purpose:
//   This package implements Prometheus metrics for tracking rate limit,
//   budget, and quota denials.
//
// Dependencies:
//   - github.com/prometheus/client_golang: Prometheus metrics
//
// Key Responsibilities:
//   - Track rate limit denials
//   - Track budget/quota denials
//   - Provide metrics for observability
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-002 (Enforce budgets and safe usage)
//
package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RateLimitDenialsTotal tracks total rate limit denials.
	RateLimitDenialsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_router_rate_limit_denials_total",
			Help: "Total number of rate limit denials",
		},
		[]string{"limit_type"}, // "org" or "key"
	)

	// BudgetDenialsTotal tracks total budget denials.
	BudgetDenialsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_router_budget_denials_total",
			Help: "Total number of budget denials",
		},
		[]string{"quota_type"}, // "budget", "daily_quota", "monthly_quota"
	)

	// QuotaDenialsTotal tracks total quota denials.
	QuotaDenialsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_router_quota_denials_total",
			Help: "Total number of quota denials",
		},
		[]string{"quota_type"}, // "daily_quota", "monthly_quota"
	)
)

// RecordRateLimitDenial records a rate limit denial metric.
func RecordRateLimitDenial(limitType string) {
	RateLimitDenialsTotal.WithLabelValues(limitType).Inc()
}

// RecordBudgetDenial records a budget denial metric.
func RecordBudgetDenial(quotaType string) {
	BudgetDenialsTotal.WithLabelValues(quotaType).Inc()
}

// RecordQuotaDenial records a quota denial metric.
func RecordQuotaDenial(quotaType string) {
	QuotaDenialsTotal.WithLabelValues(quotaType).Inc()
}

