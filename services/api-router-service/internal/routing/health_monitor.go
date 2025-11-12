// Package routing provides backend health monitoring and status tracking.
//
// Purpose:
//   This package implements a health probe scheduler that periodically checks
//   backend health status and tracks degradation metrics for intelligent routing.
//
// Key Responsibilities:
//   - Schedule periodic health checks for registered backends
//   - Track health status and degradation metrics
//   - Provide health status queries for routing decisions
//   - Emit health status change events
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-003 (Intelligent routing and fallback)
//   - specs/006-api-router-service/spec.md#FR-003 (Routing engine)
//
package routing

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthStatus represents the health status of a backend.
type HealthStatus string

const (
	// HealthStatusHealthy indicates the backend is healthy and ready to serve requests.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusDegraded indicates the backend is experiencing issues but may still serve requests.
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusUnhealthy indicates the backend is unavailable and should not receive requests.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	// HealthStatusUnknown indicates the health status is not yet known.
	HealthStatusUnknown HealthStatus = "unknown"
)

// BackendHealth represents the health state of a backend.
type BackendHealth struct {
	BackendID      string
	Status         HealthStatus
	LastCheck      time.Time
	ConsecutiveErrors int
	LastError      error
	Latency        time.Duration
	mu             sync.RWMutex
}

// HealthMonitor manages health checks for multiple backends.
type HealthMonitor struct {
	client       *BackendClient
	logger       *zap.Logger
	backends     map[string]*BackendHealth
	endpoints    map[string]*BackendEndpoint // Store endpoints for health checks
	mu           sync.RWMutex
	checkInterval time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewHealthMonitor creates a new health monitor.
func NewHealthMonitor(client *BackendClient, logger *zap.Logger, checkInterval time.Duration) *HealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &HealthMonitor{
		client:        client,
		logger:        logger,
		backends:      make(map[string]*BackendHealth),
		endpoints:     make(map[string]*BackendEndpoint),
		checkInterval: checkInterval,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// RegisterBackend registers a backend for health monitoring.
func (m *HealthMonitor) RegisterBackend(backendID string, endpoint *BackendEndpoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.backends[backendID]; !exists {
		m.backends[backendID] = &BackendHealth{
			BackendID:         backendID,
			Status:            HealthStatusUnknown,
			ConsecutiveErrors: 0,
		}
	}
	
	// Store endpoint for health checks
	if endpoint != nil {
		m.endpoints[backendID] = endpoint
	}
	
	m.logger.Info("registered backend for health monitoring",
		zap.String("backend_id", backendID),
	)
}

// UnregisterBackend removes a backend from health monitoring.
func (m *HealthMonitor) UnregisterBackend(backendID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.backends, backendID)
	delete(m.endpoints, backendID)
	m.logger.Info("unregistered backend from health monitoring",
		zap.String("backend_id", backendID),
	)
}

// Start begins periodic health checks for all registered backends.
func (m *HealthMonitor) Start() {
	m.logger.Info("starting health monitor",
		zap.Duration("check_interval", m.checkInterval),
	)

	m.wg.Add(1)
	go m.run()
}

// Stop stops health monitoring and waits for all checks to complete.
func (m *HealthMonitor) Stop() {
	m.logger.Info("stopping health monitor")
	m.cancel()
	m.wg.Wait()
}

// run performs periodic health checks.
func (m *HealthMonitor) run() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	// Perform initial check
	m.checkAllBackends()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAllBackends()
		}
	}
}

// checkAllBackends checks health for all registered backends.
func (m *HealthMonitor) checkAllBackends() {
	m.mu.RLock()
	backends := make([]string, 0, len(m.backends))
	for backendID := range m.backends {
		backends = append(backends, backendID)
	}
	m.mu.RUnlock()

	for _, backendID := range backends {
		m.checkBackend(backendID)
	}
}

// checkBackend performs a health check for a specific backend.
func (m *HealthMonitor) checkBackend(backendID string) {
	m.mu.RLock()
	health, exists := m.backends[backendID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	// Get stored endpoint
	m.mu.RLock()
	endpoint, hasEndpoint := m.endpoints[backendID]
	m.mu.RUnlock()

	if !hasEndpoint || endpoint == nil {
		m.logger.Warn("no endpoint stored for backend health check",
			zap.String("backend_id", backendID),
		)
		return
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
	defer cancel()

	err := m.client.HealthCheck(ctx, endpoint)
	latency := time.Since(startTime)

	health.mu.Lock()
	defer health.mu.Unlock()

	health.LastCheck = time.Now()
	health.Latency = latency

	if err != nil {
		health.ConsecutiveErrors++
		health.LastError = err

		// Update status based on consecutive errors
		if health.ConsecutiveErrors >= 3 {
			oldStatus := health.Status
			health.Status = HealthStatusUnhealthy
			if oldStatus != HealthStatusUnhealthy {
				m.logger.Warn("backend marked as unhealthy",
					zap.String("backend_id", backendID),
					zap.Int("consecutive_errors", health.ConsecutiveErrors),
					zap.Error(err),
				)
			}
		} else if health.ConsecutiveErrors >= 1 {
			oldStatus := health.Status
			health.Status = HealthStatusDegraded
			if oldStatus != HealthStatusDegraded && oldStatus != HealthStatusUnhealthy {
				m.logger.Warn("backend marked as degraded",
					zap.String("backend_id", backendID),
					zap.Int("consecutive_errors", health.ConsecutiveErrors),
					zap.Error(err),
				)
			}
		}
	} else {
		// Success - reset error count
		oldStatus := health.Status
		health.ConsecutiveErrors = 0
		health.LastError = nil
		health.Status = HealthStatusHealthy

		if oldStatus != HealthStatusHealthy {
			m.logger.Info("backend recovered to healthy",
				zap.String("backend_id", backendID),
				zap.Duration("latency", latency),
			)
		}
	}
}

// GetHealth returns the current health status for a backend.
func (m *HealthMonitor) GetHealth(backendID string) (*BackendHealth, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, exists := m.backends[backendID]
	if !exists {
		return nil, false
	}

	health.mu.RLock()
	defer health.mu.RUnlock()

	// Return a copy to avoid race conditions
	return &BackendHealth{
		BackendID:         health.BackendID,
		Status:            health.Status,
		LastCheck:         health.LastCheck,
		ConsecutiveErrors: health.ConsecutiveErrors,
		LastError:         health.LastError,
		Latency:          health.Latency,
	}, true
}

// IsHealthy returns true if the backend is healthy.
func (m *HealthMonitor) IsHealthy(backendID string) bool {
	health, exists := m.GetHealth(backendID)
	if !exists {
		return false
	}
	return health.Status == HealthStatusHealthy
}

// IsDegraded returns true if the backend is degraded or unhealthy.
func (m *HealthMonitor) IsDegraded(backendID string) bool {
	health, exists := m.GetHealth(backendID)
	if !exists {
		return true // Treat unknown as degraded
	}
	return health.Status == HealthStatusDegraded || health.Status == HealthStatusUnhealthy
}

// CheckBackendNow performs an immediate health check for a backend.
// This is useful for on-demand health checks or testing.
func (m *HealthMonitor) CheckBackendNow(backendID string, endpoint *BackendEndpoint) error {
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
	defer cancel()

	startTime := time.Now()
	err := m.client.HealthCheck(ctx, endpoint)
	latency := time.Since(startTime)

	m.mu.RLock()
	health, exists := m.backends[backendID]
	m.mu.RUnlock()

	if !exists {
		// Register if not exists
		m.RegisterBackend(backendID, endpoint)
		m.mu.RLock()
		health = m.backends[backendID]
		m.mu.RUnlock()
	}

	health.mu.Lock()
	defer health.mu.Unlock()

	health.LastCheck = time.Now()
	health.Latency = latency

	if err != nil {
		health.ConsecutiveErrors++
		health.LastError = err
		if health.ConsecutiveErrors >= 3 {
			health.Status = HealthStatusUnhealthy
		} else if health.ConsecutiveErrors >= 1 {
			health.Status = HealthStatusDegraded
		}
	} else {
		health.ConsecutiveErrors = 0
		health.LastError = nil
		health.Status = HealthStatusHealthy
	}

	return err
}

