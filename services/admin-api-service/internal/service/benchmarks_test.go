package service

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// TestModelAccessError_Implementation verifies that ModelAccessError exists and has expected fields
func TestModelAccessError_Implementation(t *testing.T) {
	modelName := "test-model"
	message := "test message"

	err := &ModelAccessError{
		ModelName: modelName,
		Message:   message,
	}

	if err.ModelName != modelName {
		t.Errorf("expected ModelName %s, got %s", modelName, err.ModelName)
	}

	if err.Message != message {
		t.Errorf("expected Message %s, got %s", message, err.Message)
	}

	if err.Error() != message {
		t.Errorf("expected Error() to return %s, got %s", message, err.Error())
	}
}

// TestBenchmarkValidationError_Implementation verifies that BenchmarkValidationError exists
func TestBenchmarkValidationError_Implementation(t *testing.T) {
	message := "test validation error"

	err := &BenchmarkValidationError{
		Message: message,
	}

	if err.Message != message {
		t.Errorf("expected Message %s, got %s", message, err.Message)
	}

	if err.Error() != message {
		t.Errorf("expected Error() to return %s, got %s", message, err.Error())
	}
}

// TestConflictError_Implementation verifies that ConflictError exists
func TestConflictError_Implementation(t *testing.T) {
	message := "test conflict error"

	err := &ConflictError{
		Message: message,
	}

	if err.Message != message {
		t.Errorf("expected Message %s, got %s", message, err.Message)
	}

	if err.Error() != message {
		t.Errorf("expected Error() to return %s, got %s", message, err.Error())
	}
}

// TestModelAccessError_ErrorMessage verifies the error message format
func TestModelAccessError_ErrorMessage(t *testing.T) {
	testCases := []struct {
		name          string
		modelName     string
		message       string
		expectedError string
	}{
		{
			name:          "standard error message",
			modelName:     "llama-7b",
			message:       "organization does not have access to model 'llama-7b'. Please check routing policies or contact your administrator.",
			expectedError: "organization does not have access to model 'llama-7b'. Please check routing policies or contact your administrator.",
		},
		{
			name:          "restricted model",
			modelName:     "restricted-model",
			message:       "organization does not have access to model 'restricted-model'. Please check routing policies or contact your administrator.",
			expectedError: "organization does not have access to model 'restricted-model'. Please check routing policies or contact your administrator.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := &ModelAccessError{
				ModelName: tc.modelName,
				Message:   tc.message,
			}

			if err.Error() != tc.expectedError {
				t.Errorf("expected error message:\n%s\ngot:\n%s", tc.expectedError, err.Error())
			}

			// Verify error contains model name
			if !contains(err.Message, tc.modelName) {
				t.Errorf("expected error message to contain model name '%s', got: %s", tc.modelName, err.Message)
			}

			// Verify error contains helpful information
			if !contains(err.Message, "access") {
				t.Error("expected error message to contain 'access'")
			}
		})
	}
}

// TestBenchmarkService_CreateTarget_ErrorTypes verifies error type handling
func TestBenchmarkService_CreateTarget_ErrorTypes(t *testing.T) {
	t.Run("ModelAccessError should be distinct from other errors", func(t *testing.T) {
		orgID := uuid.New()
		modelName := "test-model"

		// Create error as error interface
		var err error = &ModelAccessError{
			ModelName: modelName,
			Message:   fmt.Sprintf("organization does not have access to model '%s'. Please check routing policies or contact your administrator.", modelName),
		}

		// Type assertion should succeed
		_, ok := err.(*ModelAccessError)
		if !ok {
			t.Fatal("expected error to be *ModelAccessError")
		}

		// Should not be other error types
		if _, ok := err.(*BenchmarkValidationError); ok {
			t.Error("ModelAccessError should not be BenchmarkValidationError")
		}

		if _, ok := err.(*ConflictError); ok {
			t.Error("ModelAccessError should not be ConflictError")
		}

		// Verify error contains org-relevant information
		t.Logf("Error for org %s: %s", orgID, err.Error())
	})

	t.Run("BenchmarkValidationError should be distinct", func(t *testing.T) {
		var err error = &BenchmarkValidationError{
			Message: "scenario 'nonexistent' does not exist",
		}

		// Type assertion should succeed
		_, ok := err.(*BenchmarkValidationError)
		if !ok {
			t.Fatal("expected error to be *BenchmarkValidationError")
		}

		// Should not be other error types
		if _, ok := err.(*ModelAccessError); ok {
			t.Error("BenchmarkValidationError should not be ModelAccessError")
		}
	})

	t.Run("ConflictError should be distinct", func(t *testing.T) {
		var err error = &ConflictError{
			Message: "target with name 'test' already exists in environment 'development'",
		}

		// Type assertion should succeed
		_, ok := err.(*ConflictError)
		if !ok {
			t.Fatal("expected error to be *ConflictError")
		}

		// Should not be other error types
		if _, ok := err.(*ModelAccessError); ok {
			t.Error("ConflictError should not be ModelAccessError")
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
