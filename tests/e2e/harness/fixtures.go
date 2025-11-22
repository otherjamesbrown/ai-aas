package harness

import (
	"fmt"
	"sync"
	"time"
)

// FixtureManager manages test fixtures and cleanup
type FixtureManager struct {
	runID    string
	workerID string
	fixtures []Fixture
	mu       sync.RWMutex
}

// Fixture represents a test fixture that needs cleanup
type Fixture struct {
	Type      string
	ID        string
	CreatedAt time.Time
	Metadata  map[string]string
}

// NewFixtureManager creates a new fixture manager
func NewFixtureManager(runID, workerID string) *FixtureManager {
	return &FixtureManager{
		runID:    runID,
		workerID: workerID,
		fixtures: []Fixture{},
	}
}

// Register registers a fixture for cleanup
func (fm *FixtureManager) Register(fixtureType, id string, metadata map[string]string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.fixtures = append(fm.fixtures, Fixture{
		Type:      fixtureType,
		ID:        id,
		CreatedAt: time.Now(),
		Metadata:  metadata,
	})
}

// List returns all registered fixtures
func (fm *FixtureManager) List() []Fixture {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	result := make([]Fixture, len(fm.fixtures))
	copy(result, fm.fixtures)
	return result
}

// Cleanup performs cleanup of all registered fixtures
func (fm *FixtureManager) Cleanup() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	var errors []error
	for _, fixture := range fm.fixtures {
		if err := fm.cleanupFixture(fixture); err != nil {
			errors = append(errors, fmt.Errorf("cleanup %s %s: %w", fixture.Type, fixture.ID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	return nil
}

// cleanupFixture performs cleanup for a single fixture
func (fm *FixtureManager) cleanupFixture(fixture Fixture) error {
	// Implementation will be added in fixture-specific files
	// This is a placeholder that will be extended
	// Actual cleanup is handled by fixture-specific Delete methods
	return nil
}

// CleanupFixtureByType performs cleanup for fixtures of a specific type
func (fm *FixtureManager) CleanupFixtureByType(fixtureType string, cleanupFn func(id string) error) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	var errors []error
	for _, fixture := range fm.fixtures {
		if fixture.Type == fixtureType {
			if err := cleanupFn(fixture.ID); err != nil {
				errors = append(errors, fmt.Errorf("cleanup %s %s: %w", fixture.Type, fixture.ID, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	return nil
}

