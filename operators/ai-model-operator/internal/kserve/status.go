package kserve

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// InferenceServiceStatus represents the status of an InferenceService
type InferenceServiceStatus struct {
	URL           string
	Ready         bool
	ReadyReplicas int32
	Conditions    []Condition
}

// Condition represents a status condition
type Condition struct {
	Type    string
	Status  string
	Reason  string
	Message string
}

// GetStatus extracts status from an unstructured InferenceService
func GetStatus(u *unstructured.Unstructured) (*InferenceServiceStatus, error) {
	if u == nil {
		return nil, fmt.Errorf("unstructured object is nil")
	}

	status := &InferenceServiceStatus{
		Conditions: []Condition{},
	}

	// Get status field
	statusObj, found, err := unstructured.NestedMap(u.Object, "status")
	if err != nil {
		return nil, fmt.Errorf("error getting status: %w", err)
	}
	if !found {
		return status, nil // Empty status is valid
	}

	// Extract URL
	status.URL = getString(statusObj, "url")

	// Extract conditions
	conditions, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
	if err != nil {
		return nil, fmt.Errorf("error getting conditions: %w", err)
	}
	if found {
		for _, c := range conditions {
			condMap, ok := c.(map[string]interface{})
			if !ok {
				continue
			}

			condition := Condition{
				Type:    getString(condMap, "type"),
				Status:  getString(condMap, "status"),
				Reason:  getString(condMap, "reason"),
				Message: getString(condMap, "message"),
			}
			status.Conditions = append(status.Conditions, condition)

			// Check if this is the Ready condition
			if condition.Type == "Ready" && condition.Status == "True" {
				status.Ready = true
			}
		}
	}

	// Extract ready replicas (if available)
	if readyReplicas, found, err := unstructured.NestedInt64(u.Object, "status", "readyReplicas"); err == nil && found {
		status.ReadyReplicas = int32(readyReplicas)
	}

	return status, nil
}

// getString safely extracts a string from a map
func getString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	val, ok := m[key]
	if !ok {
		return ""
	}
	str, ok := val.(string)
	if !ok {
		return ""
	}
	return str
}

// IsReady checks if the InferenceService is ready
func (s *InferenceServiceStatus) IsReady() bool {
	return s.Ready
}

// GetCondition returns the condition with the given type
func (s *InferenceServiceStatus) GetCondition(condType string) *Condition {
	for i := range s.Conditions {
		if s.Conditions[i].Type == condType {
			return &s.Conditions[i]
		}
	}
	return nil
}

// String returns a human-readable representation of the status
func (s *InferenceServiceStatus) String() string {
	if s.Ready {
		return fmt.Sprintf("Ready (URL: %s, Replicas: %d)", s.URL, s.ReadyReplicas)
	}

	// Find the most relevant condition to display
	if readyCond := s.GetCondition("Ready"); readyCond != nil {
		return fmt.Sprintf("Not Ready: %s - %s", readyCond.Reason, readyCond.Message)
	}

	return "Not Ready"
}

// IsTransientFailure determines if the failure is transient (retryable) or permanent
// Transient failures include resource constraints, scheduling issues, etc.
// Permanent failures include invalid configuration, bad model paths, etc.
func (s *InferenceServiceStatus) IsTransientFailure() bool {
	// If Ready, it's not a failure at all
	if s.Ready {
		return false
	}

	// Check for transient failure indicators in conditions
	for _, cond := range s.Conditions {
		// Scheduling failures are transient
		if cond.Reason == "InsufficientResources" ||
			cond.Reason == "Unschedulable" ||
			cond.Reason == "PodUnschedulable" {
			return true
		}

		// Check message content for common transient errors
		if containsTransientError(cond.Message) {
			return true
		}

		// Image pull backoff is often transient (network issues, registry throttling)
		if cond.Reason == "ImagePullBackOff" || cond.Reason == "ErrImagePull" {
			return true
		}

		// Pending states are often transient
		if cond.Reason == "Pending" || cond.Reason == "ContainerCreating" {
			return true
		}
	}

	return false
}

// IsPermanentFailure determines if the failure is permanent (not retryable)
func (s *InferenceServiceStatus) IsPermanentFailure() bool {
	if s.Ready {
		return false
	}

	// Check for permanent failure indicators
	for _, cond := range s.Conditions {
		// Configuration errors are permanent
		if cond.Reason == "InvalidSpec" ||
			cond.Reason == "InvalidConfiguration" ||
			cond.Reason == "ValidationFailed" {
			return true
		}

		// Check message for permanent error patterns
		if containsPermanentError(cond.Message) {
			return true
		}

		// CrashLoopBackOff might be permanent (bad model config, missing files)
		// We'll treat this as permanent after several occurrences
		if cond.Reason == "CrashLoopBackOff" {
			return true
		}
	}

	return false
}

// containsTransientError checks if a message contains transient error patterns
func containsTransientError(message string) bool {
	transientPatterns := []string{
		"Insufficient",
		"insufficient",
		"does not have enough",
		"no nodes available",
		"FailedScheduling",
		"pending",
		"waiting for",
	}

	for _, pattern := range transientPatterns {
		if contains(message, pattern) {
			return true
		}
	}
	return false
}

// containsPermanentError checks if a message contains permanent error patterns
func containsPermanentError(message string) bool {
	permanentPatterns := []string{
		"invalid",
		"Invalid",
		"not found",
		"does not exist",
		"forbidden",
		"Forbidden",
		"unauthorized",
		"Unauthorized",
		"validation failed",
		"Validation failed",
	}

	for _, pattern := range permanentPatterns {
		if contains(message, pattern) {
			return true
		}
	}
	return false
}

// contains is a simple string contains helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
