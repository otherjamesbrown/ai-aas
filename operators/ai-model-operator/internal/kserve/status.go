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
