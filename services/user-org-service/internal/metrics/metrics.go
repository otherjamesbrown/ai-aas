// Package metrics provides Prometheus metrics collectors for the user-org service.
//
// Purpose:
//
//	This package defines and exports Prometheus metrics for authentication,
//	authorization, and identity lifecycle operations. Metrics are registered
//	globally and can be accessed via the /metrics endpoint.
//
// Dependencies:
//   - github.com/prometheus/client_golang/prometheus: Prometheus Go client
//
// Key Responsibilities:
//   - Define metric collectors (counters, histograms)
//   - Register metrics with Prometheus registry
//   - Provide helper functions to record metric values
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#T013 (Metrics & Observability)
//
// Usage:
//
//	Metrics are automatically registered when the package is imported.
//	Use the exported functions to record metric values:
//	  metrics.RecordAuthSuccess("password")
//	  metrics.RecordAuthFailure("password", "invalid_credentials")
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "user_org_service"
	subsystem = "auth"
)

var (
	// AuthAttemptsTotal counts authentication attempts by method and result.
	AuthAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "attempts_total",
			Help:      "Total number of authentication attempts by method and result",
		},
		[]string{"method", "result"}, // method: password, oidc_google, oidc_github; result: success, failure
	)

	// AuthFailuresTotal counts authentication failures by method and reason.
	AuthFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "failures_total",
			Help:      "Total number of authentication failures by method and reason",
		},
		[]string{"method", "reason"}, // reason: invalid_credentials, account_locked, mfa_required, etc.
	)

	// MFAAttemptsTotal counts MFA verification attempts by result.
	MFAAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "mfa_attempts_total",
			Help:      "Total number of MFA verification attempts by result",
		},
		[]string{"result"}, // result: success, failure
	)

	// MFAAttemptDurationSeconds measures MFA verification duration.
	MFAAttemptDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "mfa_attempt_duration_seconds",
			Help:      "Duration of MFA verification attempts in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"result"}, // result: success, failure
	)

	// SessionsCreatedTotal counts OAuth session creations.
	SessionsCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "sessions_created_total",
			Help:      "Total number of OAuth sessions created",
		},
	)

	// SessionsRevokedTotal counts OAuth session revocations.
	SessionsRevokedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "sessions_revoked_total",
			Help:      "Total number of OAuth sessions revoked",
		},
	)

	// APIKeysIssuedTotal counts API key issuances.
	APIKeysIssuedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "apikeys",
			Name:      "issued_total",
			Help:      "Total number of API keys issued",
		},
	)

	// APIKeysRevokedTotal counts API key revocations.
	APIKeysRevokedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "apikeys",
			Name:      "revoked_total",
			Help:      "Total number of API keys revoked",
		},
	)

	// OIDCLoginAttemptsTotal counts OIDC login attempts by provider.
	OIDCLoginAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "oidc_login_attempts_total",
			Help:      "Total number of OIDC login attempts by provider",
		},
		[]string{"provider"}, // provider: google, github, etc.
	)

	// OIDCCallbackTotal counts OIDC callback completions by provider and result.
	OIDCCallbackTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "oidc_callback_total",
			Help:      "Total number of OIDC callback completions by provider and result",
		},
		[]string{"provider", "result"}, // result: success, failure
	)

	// RecoveryAttemptsTotal counts password recovery attempts.
	RecoveryAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "recovery_attempts_total",
			Help:      "Total number of password recovery attempts by action",
		},
		[]string{"action"}, // action: initiate, verify, reset
	)
)

// RecordAuthSuccess records a successful authentication attempt.
func RecordAuthSuccess(method string) {
	AuthAttemptsTotal.WithLabelValues(method, "success").Inc()
}

// RecordAuthFailure records a failed authentication attempt.
func RecordAuthFailure(method, reason string) {
	AuthAttemptsTotal.WithLabelValues(method, "failure").Inc()
	AuthFailuresTotal.WithLabelValues(method, reason).Inc()
}

// RecordMFASuccess records a successful MFA verification.
func RecordMFASuccess(durationSeconds float64) {
	MFAAttemptsTotal.WithLabelValues("success").Inc()
	MFAAttemptDurationSeconds.WithLabelValues("success").Observe(durationSeconds)
}

// RecordMFAFailure records a failed MFA verification.
func RecordMFAFailure(durationSeconds float64) {
	MFAAttemptsTotal.WithLabelValues("failure").Inc()
	MFAAttemptDurationSeconds.WithLabelValues("failure").Observe(durationSeconds)
}

// RecordSessionCreated records an OAuth session creation.
func RecordSessionCreated() {
	SessionsCreatedTotal.Inc()
}

// RecordSessionRevoked records an OAuth session revocation.
func RecordSessionRevoked() {
	SessionsRevokedTotal.Inc()
}

// RecordAPIKeyIssued records an API key issuance.
func RecordAPIKeyIssued() {
	APIKeysIssuedTotal.Inc()
}

// RecordAPIKeyRevoked records an API key revocation.
func RecordAPIKeyRevoked() {
	APIKeysRevokedTotal.Inc()
}

// RecordOIDCLoginAttempt records an OIDC login attempt.
func RecordOIDCLoginAttempt(provider string) {
	OIDCLoginAttemptsTotal.WithLabelValues(provider).Inc()
}

// RecordOIDCCallbackSuccess records a successful OIDC callback.
func RecordOIDCCallbackSuccess(provider string) {
	OIDCCallbackTotal.WithLabelValues(provider, "success").Inc()
	RecordAuthSuccess("oidc_" + provider)
}

// RecordOIDCCallbackFailure records a failed OIDC callback.
func RecordOIDCCallbackFailure(provider, reason string) {
	OIDCCallbackTotal.WithLabelValues(provider, "failure").Inc()
	RecordAuthFailure("oidc_"+provider, reason)
}

// RecordRecoveryAttempt records a password recovery attempt.
func RecordRecoveryAttempt(action string) {
	RecoveryAttemptsTotal.WithLabelValues(action).Inc()
}
