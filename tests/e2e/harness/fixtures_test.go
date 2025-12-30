package harness

import (
	"errors"
	"testing"
)

// TestCleanupFixture verifies that cleanup functions are properly invoked
func TestCleanupFixture(t *testing.T) {
	tests := []struct {
		name          string
		cleanupFn     func() error
		expectError   bool
		errorContains string
	}{
		{
			name: "successful cleanup",
			cleanupFn: func() error {
				return nil
			},
			expectError: false,
		},
		{
			name: "cleanup with error",
			cleanupFn: func() error {
				return errors.New("cleanup failed")
			},
			expectError:   true,
			errorContains: "cleanup failed",
		},
		{
			name:        "nil cleanup function",
			cleanupFn:   nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewFixtureManager("test-run", "worker-1")

			// Register a fixture with the cleanup function
			fm.Register("test_type", "test-id-123", map[string]string{
				"test": "metadata",
			}, tt.cleanupFn)

			// Cleanup should invoke the cleanup function
			err := fm.Cleanup()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && tt.errorContains != "" {
				if err == nil {
					t.Errorf("Expected error containing %q but got nil", tt.errorContains)
				} else if !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q but got %q", tt.errorContains, err.Error())
				}
			}
		})
	}
}

// TestCleanupMultipleFixtures verifies that all fixtures are cleaned up
func TestCleanupMultipleFixtures(t *testing.T) {
	fm := NewFixtureManager("test-run", "worker-1")

	cleanupOrder := []string{}

	// Register multiple fixtures
	fm.Register("user", "user-1", map[string]string{"email": "user1@test.com"}, func() error {
		cleanupOrder = append(cleanupOrder, "user-1")
		return nil
	})

	fm.Register("org", "org-1", map[string]string{"name": "test-org"}, func() error {
		cleanupOrder = append(cleanupOrder, "org-1")
		return nil
	})

	fm.Register("api_key", "key-1", map[string]string{"name": "test-key"}, func() error {
		cleanupOrder = append(cleanupOrder, "key-1")
		return nil
	})

	// Cleanup should call all cleanup functions
	err := fm.Cleanup()
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify all cleanup functions were called
	if len(cleanupOrder) != 3 {
		t.Errorf("Expected 3 cleanups but got %d", len(cleanupOrder))
	}

	// Verify the expected cleanup order (matches registration order)
	expectedOrder := []string{"user-1", "org-1", "key-1"}
	for i, expected := range expectedOrder {
		if i >= len(cleanupOrder) || cleanupOrder[i] != expected {
			t.Errorf("Expected cleanup order %v but got %v", expectedOrder, cleanupOrder)
			break
		}
	}
}

// TestCleanupPartialFailure verifies that cleanup continues even if one fails
func TestCleanupPartialFailure(t *testing.T) {
	fm := NewFixtureManager("test-run", "worker-1")

	cleanupCalled := map[string]bool{}

	fm.Register("fixture1", "id1", nil, func() error {
		cleanupCalled["fixture1"] = true
		return nil
	})

	fm.Register("fixture2", "id2", nil, func() error {
		cleanupCalled["fixture2"] = true
		return errors.New("fixture2 cleanup failed")
	})

	fm.Register("fixture3", "id3", nil, func() error {
		cleanupCalled["fixture3"] = true
		return nil
	})

	err := fm.Cleanup()

	// Should return an error because fixture2 failed
	if err == nil {
		t.Error("Expected error from failed cleanup")
	}

	// But all cleanup functions should have been called
	if !cleanupCalled["fixture1"] {
		t.Error("Expected fixture1 cleanup to be called")
	}
	if !cleanupCalled["fixture2"] {
		t.Error("Expected fixture2 cleanup to be called")
	}
	if !cleanupCalled["fixture3"] {
		t.Error("Expected fixture3 cleanup to be called")
	}
}

// TestListFixtures verifies that registered fixtures are listed correctly
func TestListFixtures(t *testing.T) {
	fm := NewFixtureManager("test-run", "worker-1")

	// Register some fixtures
	fm.Register("user", "user-1", map[string]string{"email": "test@example.com"}, nil)
	fm.Register("org", "org-1", map[string]string{"name": "test-org"}, nil)

	fixtures := fm.List()

	if len(fixtures) != 2 {
		t.Errorf("Expected 2 fixtures but got %d", len(fixtures))
	}

	// Verify fixture details
	if fixtures[0].Type != "user" || fixtures[0].ID != "user-1" {
		t.Errorf("First fixture mismatch: got %+v", fixtures[0])
	}

	if fixtures[1].Type != "org" || fixtures[1].ID != "org-1" {
		t.Errorf("Second fixture mismatch: got %+v", fixtures[1])
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
