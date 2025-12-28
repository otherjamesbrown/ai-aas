package routing

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

func TestNewHealthMonitor(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewBackendClient(logger, 5*time.Second)
	checkInterval := 10 * time.Second

	monitor := NewHealthMonitor(client, logger, checkInterval)

	if monitor == nil {
		t.Fatal("NewHealthMonitor returned nil")
	}

	if monitor.client != client {
		t.Error("client not set correctly")
	}

	if monitor.logger == nil {
		t.Error("logger is nil")
	}

	if monitor.checkInterval != checkInterval {
		t.Errorf("checkInterval = %v, want %v", monitor.checkInterval, checkInterval)
	}

	if monitor.backends == nil {
		t.Error("backends map is nil")
	}

	if monitor.endpoints == nil {
		t.Error("endpoints map is nil")
	}

	if monitor.ctx == nil {
		t.Error("context is nil")
	}

	if monitor.cancel == nil {
		t.Error("cancel function is nil")
	}
}

func TestHealthMonitor_RegisterBackend(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewBackendClient(logger, 5*time.Second)
	monitor := NewHealthMonitor(client, logger, 10*time.Second)

	endpoint := &BackendEndpoint{
		ID:         "backend-1",
		URI:        "http://localhost:8001",
		Timeout:    5 * time.Second,
		HealthPath: "/health",
	}

	monitor.RegisterBackend("backend-1", endpoint)

	// Verify backend was registered
	monitor.mu.RLock()
	health, exists := monitor.backends["backend-1"]
	monitor.mu.RUnlock()

	if !exists {
		t.Error("backend not registered")
		return
	}

	if health.BackendID != "backend-1" {
		t.Errorf("health.BackendID = %s, want backend-1", health.BackendID)
	}

	if health.Status != HealthStatusUnknown {
		t.Errorf("health.Status = %s, want %s", health.Status, HealthStatusUnknown)
	}

	// Verify endpoint was stored
	monitor.mu.RLock()
	storedEndpoint, hasEndpoint := monitor.endpoints["backend-1"]
	monitor.mu.RUnlock()

	if !hasEndpoint {
		t.Error("endpoint not stored")
	}

	if storedEndpoint != endpoint {
		t.Error("stored endpoint does not match")
	}
}

func TestHealthMonitor_UnregisterBackend(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewBackendClient(logger, 5*time.Second)
	monitor := NewHealthMonitor(client, logger, 10*time.Second)

	endpoint := &BackendEndpoint{
		ID:      "backend-1",
		URI:     "http://localhost:8001",
		Timeout: 5 * time.Second,
	}

	monitor.RegisterBackend("backend-1", endpoint)
	monitor.UnregisterBackend("backend-1")

	// Verify backend was unregistered
	monitor.mu.RLock()
	_, exists := monitor.backends["backend-1"]
	_, hasEndpoint := monitor.endpoints["backend-1"]
	monitor.mu.RUnlock()

	if exists {
		t.Error("backend still registered")
	}

	if hasEndpoint {
		t.Error("endpoint still stored")
	}
}

func TestHealthMonitor_GetHealth(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewBackendClient(logger, 5*time.Second)
	monitor := NewHealthMonitor(client, logger, 10*time.Second)

	endpoint := &BackendEndpoint{
		ID:      "backend-1",
		URI:     "http://localhost:8001",
		Timeout: 5 * time.Second,
	}

	monitor.RegisterBackend("backend-1", endpoint)

	health, exists := monitor.GetHealth("backend-1")
	if !exists {
		t.Error("GetHealth() returned false for existing backend")
		return
	}

	if health.BackendID != "backend-1" {
		t.Errorf("health.BackendID = %s, want backend-1", health.BackendID)
	}

	// Test non-existent backend
	_, exists = monitor.GetHealth("nonexistent")
	if exists {
		t.Error("GetHealth() returned true for non-existent backend")
	}
}

func TestHealthMonitor_IsHealthy(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewBackendClient(logger, 5*time.Second)
	monitor := NewHealthMonitor(client, logger, 10*time.Second)

	endpoint := &BackendEndpoint{
		ID:      "backend-1",
		URI:     "http://localhost:8001",
		Timeout: 5 * time.Second,
	}

	monitor.RegisterBackend("backend-1", endpoint)

	// Initially unknown, not healthy
	if monitor.IsHealthy("backend-1") {
		t.Error("IsHealthy() returned true for unknown status")
	}

	// Set to healthy
	monitor.mu.RLock()
	health := monitor.backends["backend-1"]
	monitor.mu.RUnlock()

	health.mu.Lock()
	health.Status = HealthStatusHealthy
	health.mu.Unlock()

	if !monitor.IsHealthy("backend-1") {
		t.Error("IsHealthy() returned false for healthy backend")
	}

	// Test non-existent backend
	if monitor.IsHealthy("nonexistent") {
		t.Error("IsHealthy() returned true for non-existent backend")
	}
}

func TestHealthMonitor_IsDegraded(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewBackendClient(logger, 5*time.Second)
	monitor := NewHealthMonitor(client, logger, 10*time.Second)

	endpoint := &BackendEndpoint{
		ID:      "backend-1",
		URI:     "http://localhost:8001",
		Timeout: 5 * time.Second,
	}

	monitor.RegisterBackend("backend-1", endpoint)

	tests := []struct {
		name       string
		status     HealthStatus
		wantResult bool
	}{
		{
			name:       "healthy backend is not degraded",
			status:     HealthStatusHealthy,
			wantResult: false,
		},
		{
			name:       "degraded backend is degraded",
			status:     HealthStatusDegraded,
			wantResult: true,
		},
		{
			name:       "unhealthy backend is degraded",
			status:     HealthStatusUnhealthy,
			wantResult: true,
		},
		{
			name:       "unknown backend is not degraded",
			status:     HealthStatusUnknown,
			wantResult: false, // Unknown status exists but hasn't been checked yet
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor.mu.RLock()
			health := monitor.backends["backend-1"]
			monitor.mu.RUnlock()

			health.mu.Lock()
			health.Status = tt.status
			health.mu.Unlock()

			result := monitor.IsDegraded("backend-1")
			if result != tt.wantResult {
				t.Errorf("IsDegraded() = %v, want %v for status %s", result, tt.wantResult, tt.status)
			}
		})
	}

	// Test non-existent backend (should be treated as degraded)
	if !monitor.IsDegraded("nonexistent") {
		t.Error("IsDegraded() returned false for non-existent backend")
	}
}

func TestHealthMonitor_CheckBackendNow(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name               string
		backendHandler     http.HandlerFunc
		wantErr            bool
		wantStatus         HealthStatus
		wantConsecutiveErr int
	}{
		{
			name: "healthy backend",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr:            false,
			wantStatus:         HealthStatusHealthy,
			wantConsecutiveErr: 0,
		},
		{
			name: "unhealthy backend - first error",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr:            true,
			wantStatus:         HealthStatusDegraded,
			wantConsecutiveErr: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.backendHandler)
			defer server.Close()

			client := NewBackendClient(logger, 5*time.Second)
			monitor := NewHealthMonitor(client, logger, 10*time.Second)

			endpoint := &BackendEndpoint{
				ID:         "backend-1",
				URI:        server.URL,
				Timeout:    5 * time.Second,
				HealthPath: "/health",
			}

			err := monitor.CheckBackendNow("backend-1", endpoint)

			if tt.wantErr && err == nil {
				t.Error("CheckBackendNow() expected error, got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("CheckBackendNow() unexpected error: %v", err)
			}

			// Verify health status
			health, exists := monitor.GetHealth("backend-1")
			if !exists {
				t.Error("backend not found after health check")
				return
			}

			if health.Status != tt.wantStatus {
				t.Errorf("health.Status = %s, want %s", health.Status, tt.wantStatus)
			}

			if health.ConsecutiveErrors != tt.wantConsecutiveErr {
				t.Errorf("health.ConsecutiveErrors = %d, want %d", health.ConsecutiveErrors, tt.wantConsecutiveErr)
			}
		})
	}
}

func TestHealthMonitor_CheckBackendNow_ConsecutiveErrors(t *testing.T) {
	logger := zaptest.NewLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewBackendClient(logger, 5*time.Second)
	monitor := NewHealthMonitor(client, logger, 10*time.Second)

	endpoint := &BackendEndpoint{
		ID:         "backend-1",
		URI:        server.URL,
		Timeout:    5 * time.Second,
		HealthPath: "/health",
	}

	tests := []struct {
		errorCount int
		wantStatus HealthStatus
	}{
		{errorCount: 1, wantStatus: HealthStatusDegraded},
		{errorCount: 2, wantStatus: HealthStatusDegraded},
		{errorCount: 3, wantStatus: HealthStatusUnhealthy},
		{errorCount: 4, wantStatus: HealthStatusUnhealthy},
	}

	for i, tt := range tests {
		t.Run(string(rune('0'+i)), func(t *testing.T) {
			_ = monitor.CheckBackendNow("backend-1", endpoint)

			health, exists := monitor.GetHealth("backend-1")
			if !exists {
				t.Error("backend not found")
				return
			}

			if health.ConsecutiveErrors != tt.errorCount {
				t.Errorf("ConsecutiveErrors = %d, want %d", health.ConsecutiveErrors, tt.errorCount)
			}

			if health.Status != tt.wantStatus {
				t.Errorf("Status = %s, want %s after %d errors", health.Status, tt.wantStatus, tt.errorCount)
			}
		})
	}
}

func TestHealthMonitor_CheckBackendNow_Recovery(t *testing.T) {
	logger := zaptest.NewLogger(t)

	errorCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if errorCount < 3 {
			errorCount++
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewBackendClient(logger, 5*time.Second)
	monitor := NewHealthMonitor(client, logger, 10*time.Second)

	endpoint := &BackendEndpoint{
		ID:         "backend-1",
		URI:        server.URL,
		Timeout:    5 * time.Second,
		HealthPath: "/health",
	}

	// Trigger 3 errors to make it unhealthy
	for i := 0; i < 3; i++ {
		_ = monitor.CheckBackendNow("backend-1", endpoint)
	}

	health, _ := monitor.GetHealth("backend-1")
	if health.Status != HealthStatusUnhealthy {
		t.Errorf("Status = %s, want %s after 3 errors", health.Status, HealthStatusUnhealthy)
	}

	// Now it should recover
	err := monitor.CheckBackendNow("backend-1", endpoint)
	if err != nil {
		t.Errorf("CheckBackendNow() unexpected error on recovery: %v", err)
	}

	health, _ = monitor.GetHealth("backend-1")
	if health.Status != HealthStatusHealthy {
		t.Errorf("Status = %s, want %s after recovery", health.Status, HealthStatusHealthy)
	}

	if health.ConsecutiveErrors != 0 {
		t.Errorf("ConsecutiveErrors = %d, want 0 after recovery", health.ConsecutiveErrors)
	}
}

func TestHealthMonitor_StartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewBackendClient(logger, 5*time.Second)
	monitor := NewHealthMonitor(client, logger, 50*time.Millisecond)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	endpoint := &BackendEndpoint{
		ID:         "backend-1",
		URI:        server.URL,
		Timeout:    5 * time.Second,
		HealthPath: "/health",
	}

	monitor.RegisterBackend("backend-1", endpoint)

	// Start monitoring
	monitor.Start()

	// Wait for at least one check cycle
	time.Sleep(100 * time.Millisecond)

	// Verify backend was checked
	health, exists := monitor.GetHealth("backend-1")
	if !exists {
		t.Error("backend not found")
		return
	}

	if health.Status == HealthStatusUnknown {
		t.Error("backend status still unknown after health check cycle")
	}

	if health.LastCheck.IsZero() {
		t.Error("backend was not checked (LastCheck is zero)")
	}

	// Record the time before stopping
	beforeStop := time.Now()

	// Stop monitoring
	monitor.Stop()

	// Wait a bit to ensure no more checks happen
	time.Sleep(150 * time.Millisecond)

	health, _ = monitor.GetHealth("backend-1")
	// The last check should be before we called Stop()
	if health.LastCheck.After(beforeStop.Add(50 * time.Millisecond)) {
		t.Errorf("health checks continued after Stop() - last check at %v, stopped at %v", health.LastCheck, beforeStop)
	}
}

func TestHealthMonitor_CheckBackend_NoEndpoint(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewBackendClient(logger, 5*time.Second)
	monitor := NewHealthMonitor(client, logger, 10*time.Second)

	// Register backend without endpoint
	monitor.RegisterBackend("backend-1", nil)

	// checkBackend should handle missing endpoint gracefully
	monitor.checkBackend("backend-1")

	// Verify backend still exists
	_, exists := monitor.GetHealth("backend-1")
	if !exists {
		t.Error("backend was removed after check with no endpoint")
	}
}

func TestBackendHealth_ThreadSafety(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewBackendClient(logger, 5*time.Second)
	monitor := NewHealthMonitor(client, logger, 10*time.Second)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	endpoint := &BackendEndpoint{
		ID:         "backend-1",
		URI:        server.URL,
		Timeout:    5 * time.Second,
		HealthPath: "/health",
	}

	monitor.RegisterBackend("backend-1", endpoint)

	// Concurrent reads and writes
	done := make(chan bool)

	// Writer goroutines
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_ = monitor.CheckBackendNow("backend-1", endpoint)
				time.Sleep(time.Millisecond)
			}
			done <- true
		}()
	}

	// Reader goroutines
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_, _ = monitor.GetHealth("backend-1")
				_ = monitor.IsHealthy("backend-1")
				_ = monitor.IsDegraded("backend-1")
				time.Sleep(time.Millisecond)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestHealthStatus_Constants(t *testing.T) {
	// Verify health status constants are as expected
	if HealthStatusHealthy != "healthy" {
		t.Errorf("HealthStatusHealthy = %s, want healthy", HealthStatusHealthy)
	}

	if HealthStatusDegraded != "degraded" {
		t.Errorf("HealthStatusDegraded = %s, want degraded", HealthStatusDegraded)
	}

	if HealthStatusUnhealthy != "unhealthy" {
		t.Errorf("HealthStatusUnhealthy = %s, want unhealthy", HealthStatusUnhealthy)
	}

	if HealthStatusUnknown != "unknown" {
		t.Errorf("HealthStatusUnknown = %s, want unknown", HealthStatusUnknown)
	}
}
