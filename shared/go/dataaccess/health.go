package dataaccess

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Probe represents a health check function that returns an error on failure.
type Probe func(ctx context.Context) error

// Registry maintains a set of named probes and evaluates them on demand.
type Registry struct {
	mu     sync.RWMutex
	probes map[string]Probe
}

// Status holds the evaluation result for a probe.
type Status struct {
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
	Latency string `json:"latency"`
}

// Result represents the overall health payload.
type Result struct {
	Checks map[string]Status `json:"checks"`
}

// NewRegistry initializes an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		probes: map[string]Probe{},
	}
}

// Register adds a new probe to the registry.
func (r *Registry) Register(name string, probe Probe) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.probes[name] = probe
}

// Evaluate executes every probe and returns a result map.
func (r *Registry) Evaluate(ctx context.Context) Result {
	r.mu.RLock()
	defer r.mu.RUnlock()

	checks := make(map[string]Status, len(r.probes))
	for name, probe := range r.probes {
		start := time.Now()
		err := probe(ctx)
		status := Status{
			Healthy: err == nil,
			Latency: time.Since(start).String(),
		}
		if err != nil {
			status.Error = err.Error()
		}
		checks[name] = status
	}
	return Result{Checks: checks}
}

// Handler returns an HTTP handler that emits JSON health responses.
func Handler(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		result := reg.Evaluate(ctx)
		status := http.StatusOK
		for _, check := range result.Checks {
			if !check.Healthy {
				status = http.StatusServiceUnavailable
				break
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(result)
	}
}

// SQLProbe returns a Probe that pings a sql.DB instance.
func SQLProbe(db *sql.DB) Probe {
	return func(ctx context.Context) error {
		return db.PingContext(ctx)
	}
}
