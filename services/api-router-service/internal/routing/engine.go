// Package routing provides intelligent routing engine with weighted selection and failover.
//
// Purpose:
//   This package implements the core routing engine that selects backends based on
//   configured weights, health status, and failover policies. It provides intelligent
//   traffic distribution and automatic failover capabilities.
//
// Key Responsibilities:
//   - Weighted backend selection
//   - Health-aware routing
//   - Automatic failover on errors
//   - Routing decision tracking
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-003 (Intelligent routing and fallback)
//   - specs/006-api-router-service/spec.md#FR-003 (Routing engine)
//
package routing

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
)

// RoutingDecision represents a routing decision made by the engine.
type RoutingDecision struct {
	BackendID      string
	DecisionType   string // "PRIMARY", "FAILOVER", "WEIGHTED"
	Reason         string
	Timestamp      time.Time
	AttemptNumber  int
}

// Engine provides intelligent routing with weighted selection and failover.
type Engine struct {
	healthMonitor *HealthMonitor
	backendRegistry *config.BackendRegistry
	logger        *zap.Logger
	decisions     []RoutingDecision // For metrics/debugging
	mu            sync.RWMutex
}

// NewEngine creates a new routing engine.
func NewEngine(healthMonitor *HealthMonitor, backendRegistry *config.BackendRegistry, logger *zap.Logger) *Engine {
	return &Engine{
		healthMonitor:   healthMonitor,
		backendRegistry: backendRegistry,
		logger:          logger,
		decisions:       make([]RoutingDecision, 0),
	}
}

// SelectBackend selects a backend based on routing policy, weights, and health status.
func (e *Engine) SelectBackend(ctx context.Context, policy *config.RoutingPolicy) (*BackendEndpoint, *RoutingDecision, error) {
	if policy == nil || len(policy.Backends) == 0 {
		return nil, nil, fmt.Errorf("no backends configured in policy")
	}

	// Get available backends (excluding degraded ones)
	availableBackends := e.getAvailableBackends(policy)
	if len(availableBackends) == 0 {
		return nil, nil, fmt.Errorf("no available backends")
	}

	// Select backend using weighted selection
	selected := e.selectWeightedBackend(availableBackends)
	if selected == nil {
		return nil, nil, fmt.Errorf("failed to select backend")
	}

	// Build backend endpoint
	endpoint, err := e.buildBackendEndpoint(selected.BackendID, policy.Model)
	if err != nil {
		return nil, nil, fmt.Errorf("build backend endpoint: %w", err)
	}

	decision := &RoutingDecision{
		BackendID:     selected.BackendID,
		DecisionType:  "WEIGHTED",
		Reason:        fmt.Sprintf("weighted selection (weight: %d)", selected.Weight),
		Timestamp:     time.Now(),
		AttemptNumber: 1,
	}

	e.recordDecision(decision)

	return endpoint, decision, nil
}

// RouteWithFailover routes a request with automatic failover on errors.
func (e *Engine) RouteWithFailover(
	ctx context.Context,
	policy *config.RoutingPolicy,
	request *BackendRequest,
	client *BackendClient,
) (*BackendResponse, *RoutingDecision, error) {
	if policy == nil || len(policy.Backends) == 0 {
		return nil, nil, fmt.Errorf("no backends configured in policy")
	}

	// Get available backends sorted by weight (highest first for failover)
	availableBackends := e.getAvailableBackends(policy)
	if len(availableBackends) == 0 {
		return nil, nil, fmt.Errorf("no available backends")
	}

	// Sort by weight descending for failover order
	e.sortBackendsByWeight(availableBackends)

	var lastErr error
	var lastDecision *RoutingDecision

	// Try backends in order until one succeeds
	for attempt, backendWeight := range availableBackends {
		endpoint, err := e.buildBackendEndpoint(backendWeight.BackendID, policy.Model)
		if err != nil {
			e.logger.Warn("failed to build backend endpoint",
				zap.String("backend_id", backendWeight.BackendID),
				zap.Error(err),
			)
			continue
		}

		// Determine decision type
		decisionType := "PRIMARY"
		if attempt > 0 {
			decisionType = "FAILOVER"
		}

		decision := &RoutingDecision{
			BackendID:     backendWeight.BackendID,
			DecisionType:  decisionType,
			Reason:        fmt.Sprintf("attempt %d (weight: %d)", attempt+1, backendWeight.Weight),
			Timestamp:     time.Now(),
			AttemptNumber: attempt + 1,
		}

		// Forward request to backend
		response, err := client.ForwardRequest(ctx, endpoint, request)
		if err == nil {
			// Success
			e.recordDecision(decision)
			return response, decision, nil
		}

		// Record failure
		decision.Reason = fmt.Sprintf("%s - error: %v", decision.Reason, err)
		e.recordDecision(decision)

		lastErr = err
		lastDecision = decision

		e.logger.Warn("backend request failed, trying failover",
			zap.String("backend_id", backendWeight.BackendID),
			zap.Int("attempt", attempt+1),
			zap.Int("total_backends", len(availableBackends)),
			zap.Error(err),
		)

		// Check if we should continue (failover threshold)
		if attempt+1 >= policy.FailoverThreshold && attempt+1 < len(availableBackends) {
			// Continue to next backend
			continue
		}
	}

	// All backends failed
	return nil, lastDecision, fmt.Errorf("all backends failed, last error: %w", lastErr)
}

// getAvailableBackends returns available backends excluding degraded ones.
func (e *Engine) getAvailableBackends(policy *config.RoutingPolicy) []config.BackendWeight {
	if len(policy.Backends) == 0 {
		return nil
	}

	// Build degraded map
	degradedMap := make(map[string]bool)
	for _, degradedID := range policy.DegradedBackends {
		degradedMap[degradedID] = true
	}

	// Also check health monitor for unhealthy backends
	if e.healthMonitor != nil {
		for _, backendWeight := range policy.Backends {
			if e.healthMonitor.IsDegraded(backendWeight.BackendID) {
				degradedMap[backendWeight.BackendID] = true
			}
		}
	}

	// Filter out degraded backends
	availableBackends := make([]config.BackendWeight, 0)
	for _, backend := range policy.Backends {
		if !degradedMap[backend.BackendID] {
			availableBackends = append(availableBackends, backend)
		}
	}

	// If all backends are degraded, fall back to all backends
	if len(availableBackends) == 0 {
		e.logger.Warn("all backends degraded, using all backends as fallback")
		availableBackends = policy.Backends
	}

	return availableBackends
}

// selectWeightedBackend selects a backend using weighted random selection.
func (e *Engine) selectWeightedBackend(backends []config.BackendWeight) *config.BackendWeight {
	if len(backends) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0
	for _, backend := range backends {
		if backend.Weight > 0 {
			totalWeight += backend.Weight
		}
	}

	if totalWeight == 0 {
		// No weights specified, return first backend
		return &backends[0]
	}

	// Generate random number in range [0, totalWeight)
	selectedWeight, err := randomInt64(int64(totalWeight))
	if err != nil {
		// Fallback to time-based if crypto/rand fails
		selectedWeight = time.Now().UnixNano() % int64(totalWeight)
	}

	// Find backend corresponding to selected weight
	currentWeight := 0
	for i := range backends {
		if backends[i].Weight > 0 {
			currentWeight += backends[i].Weight
			if int64(currentWeight) > selectedWeight {
				return &backends[i]
			}
		}
	}

	// Fallback to first backend
	return &backends[0]
}

// sortBackendsByWeight sorts backends by weight in descending order.
func (e *Engine) sortBackendsByWeight(backends []config.BackendWeight) {
	for i := 0; i < len(backends)-1; i++ {
		for j := i + 1; j < len(backends); j++ {
			if backends[i].Weight < backends[j].Weight {
				backends[i], backends[j] = backends[j], backends[i]
			}
		}
	}
}

// buildBackendEndpoint constructs a BackendEndpoint from a backend ID.
func (e *Engine) buildBackendEndpoint(backendID, model string) (*BackendEndpoint, error) {
	if e.backendRegistry == nil {
		return nil, fmt.Errorf("backend registry not configured")
	}

	backendCfg, err := e.backendRegistry.GetBackend(backendID)
	if err != nil {
		return nil, fmt.Errorf("backend not found: %w", err)
	}

	return &BackendEndpoint{
		ID:          backendCfg.ID,
		URI:         backendCfg.URI,
		ModelVariant: model,
		Timeout:     backendCfg.Timeout,
	}, nil
}

// recordDecision records a routing decision for metrics/debugging.
func (e *Engine) recordDecision(decision *RoutingDecision) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Keep last 100 decisions (circular buffer)
	if len(e.decisions) >= 100 {
		e.decisions = e.decisions[1:]
	}
	e.decisions = append(e.decisions, *decision)
}

// GetRecentDecisions returns recent routing decisions.
func (e *Engine) GetRecentDecisions(limit int) []RoutingDecision {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.decisions) {
		limit = len(e.decisions)
	}

	start := len(e.decisions) - limit
	if start < 0 {
		start = 0
	}

	result := make([]RoutingDecision, limit)
	copy(result, e.decisions[start:])
	return result
}

// randomInt64 generates a random int64 in the range [0, max).
func randomInt64(max int64) (int64, error) {
	if max <= 0 {
		return 0, fmt.Errorf("max must be positive")
	}

	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}

	val := binary.BigEndian.Uint64(buf[:])
	return int64(val % uint64(max)), nil
}

