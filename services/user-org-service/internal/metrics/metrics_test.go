package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// TestMetricsRegistration verifies that all metrics are properly registered.
// This test ensures metrics can be created without errors.
func TestMetricsRegistration(t *testing.T) {
	// Verify metrics are not nil (they should be auto-registered on package import)
	if AuthAttemptsTotal == nil {
		t.Error("AuthAttemptsTotal metric not registered")
	}
	if AuthFailuresTotal == nil {
		t.Error("AuthFailuresTotal metric not registered")
	}
	if MFAAttemptsTotal == nil {
		t.Error("MFAAttemptsTotal metric not registered")
	}
	if MFAAttemptDurationSeconds == nil {
		t.Error("MFAAttemptDurationSeconds metric not registered")
	}
	if SessionsCreatedTotal == nil {
		t.Error("SessionsCreatedTotal metric not registered")
	}
	if SessionsRevokedTotal == nil {
		t.Error("SessionsRevokedTotal metric not registered")
	}
	if APIKeysIssuedTotal == nil {
		t.Error("APIKeysIssuedTotal metric not registered")
	}
	if APIKeysRevokedTotal == nil {
		t.Error("APIKeysRevokedTotal metric not registered")
	}
	if OIDCLoginAttemptsTotal == nil {
		t.Error("OIDCLoginAttemptsTotal metric not registered")
	}
	if OIDCCallbackTotal == nil {
		t.Error("OIDCCallbackTotal metric not registered")
	}
	if RecoveryAttemptsTotal == nil {
		t.Error("RecoveryAttemptsTotal metric not registered")
	}
}

// TestRecordAuthSuccess verifies that RecordAuthSuccess increments the counter.
func TestRecordAuthSuccess(t *testing.T) {
	// Get initial value
	initialValue := getCounterValue(AuthAttemptsTotal.WithLabelValues("password", "success"))

	// Record success
	RecordAuthSuccess("password")

	// Verify counter incremented
	newValue := getCounterValue(AuthAttemptsTotal.WithLabelValues("password", "success"))
	if newValue <= initialValue {
		t.Errorf("Expected counter to increment, got initial=%f, new=%f", initialValue, newValue)
	}
}

// TestRecordAuthFailure verifies that RecordAuthFailure increments both counters.
func TestRecordAuthFailure(t *testing.T) {
	initialAttempts := getCounterValue(AuthAttemptsTotal.WithLabelValues("password", "failure"))
	initialFailures := getCounterValue(AuthFailuresTotal.WithLabelValues("password", "invalid_credentials"))

	RecordAuthFailure("password", "invalid_credentials")

	newAttempts := getCounterValue(AuthAttemptsTotal.WithLabelValues("password", "failure"))
	newFailures := getCounterValue(AuthFailuresTotal.WithLabelValues("password", "invalid_credentials"))

	if newAttempts <= initialAttempts {
		t.Error("Expected AuthAttemptsTotal to increment")
	}
	if newFailures <= initialFailures {
		t.Error("Expected AuthFailuresTotal to increment")
	}
}

// TestRecordMFASuccess verifies MFA success recording.
func TestRecordMFASuccess(t *testing.T) {
	initialCount := getCounterValue(MFAAttemptsTotal.WithLabelValues("success"))

	RecordMFASuccess(0.5)

	newCount := getCounterValue(MFAAttemptsTotal.WithLabelValues("success"))

	if newCount <= initialCount {
		t.Error("Expected MFAAttemptsTotal to increment")
	}
}

// TestRecordSessionCreated verifies session creation recording.
func TestRecordSessionCreated(t *testing.T) {
	initial := getCounterValue(SessionsCreatedTotal)

	RecordSessionCreated()

	new := getCounterValue(SessionsCreatedTotal)
	if new <= initial {
		t.Error("Expected SessionsCreatedTotal to increment")
	}
}

// TestRecordAPIKeyIssued verifies API key issuance recording.
func TestRecordAPIKeyIssued(t *testing.T) {
	initial := getCounterValue(APIKeysIssuedTotal)

	RecordAPIKeyIssued()

	new := getCounterValue(APIKeysIssuedTotal)
	if new <= initial {
		t.Error("Expected APIKeysIssuedTotal to increment")
	}
}

// TestRecordOIDCLoginAttempt verifies OIDC login attempt recording.
func TestRecordOIDCLoginAttempt(t *testing.T) {
	initial := getCounterValue(OIDCLoginAttemptsTotal.WithLabelValues("google"))

	RecordOIDCLoginAttempt("google")

	new := getCounterValue(OIDCLoginAttemptsTotal.WithLabelValues("google"))
	if new <= initial {
		t.Error("Expected OIDCLoginAttemptsTotal to increment")
	}
}

// TestRecordRecoveryAttempt verifies recovery attempt recording.
func TestRecordRecoveryAttempt(t *testing.T) {
	initial := getCounterValue(RecoveryAttemptsTotal.WithLabelValues("initiate"))

	RecordRecoveryAttempt("initiate")

	new := getCounterValue(RecoveryAttemptsTotal.WithLabelValues("initiate"))
	if new <= initial {
		t.Error("Expected RecoveryAttemptsTotal to increment")
	}
}

// Helper function to extract counter value for testing
func getCounterValue(counter prometheus.Counter) float64 {
	metric := &dto.Metric{}
	if err := counter.Write(metric); err != nil {
		return 0
	}
	if metric.Counter != nil {
		return metric.Counter.GetValue()
	}
	return 0
}
