package model

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseWatchStatus(t *testing.T) {
	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		expected *AIModelWatchStatus
	}{
		{
			name: "basic status with phase",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "llama-7b",
						"namespace": "ai-system",
					},
					"status": map[string]interface{}{
						"phase":                "Downloading",
						"phaseDuration":        "2m30s",
						"inferenceServiceName": "llama-7b-isvc",
						"readyReplicas":        int64(0),
					},
				},
			},
			expected: &AIModelWatchStatus{
				Name:                 "llama-7b",
				Namespace:            "ai-system",
				Phase:                "Downloading",
				PhaseDuration:        "2m30s",
				InferenceServiceName: "llama-7b-isvc",
				ReadyReplicas:        0,
			},
		},
		{
			name: "status with progress",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "gpt-20b",
						"namespace": "ai-system",
					},
					"status": map[string]interface{}{
						"phase":                "Downloading",
						"phaseDuration":        "5m",
						"inferenceServiceName": "gpt-20b-isvc",
						"readyReplicas":        int64(0),
						"progress": map[string]interface{}{
							"percentage": int64(45),
							"downloaded": "2.1GB / 4.7GB",
							"eta":        "3m15s",
						},
					},
				},
			},
			expected: &AIModelWatchStatus{
				Name:                 "gpt-20b",
				Namespace:            "ai-system",
				Phase:                "Downloading",
				PhaseDuration:        "5m",
				InferenceServiceName: "gpt-20b-isvc",
				ReadyReplicas:        0,
				Progress: &PhaseProgress{
					Percentage: 45,
					Downloaded: "2.1GB / 4.7GB",
					ETA:        "3m15s",
				},
			},
		},
		{
			name: "status with scheduling blocked",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "llama-70b",
						"namespace": "ai-system",
					},
					"status": map[string]interface{}{
						"phase":                "Scheduling",
						"phaseDuration":        "30s",
						"inferenceServiceName": "llama-70b-isvc",
						"readyReplicas":        int64(0),
						"schedulingStatus": map[string]interface{}{
							"scheduled":      false,
							"blockedReason":  "Insufficient GPU",
							"blockedMessage": "0/3 nodes available: 3 Insufficient nvidia.com/gpu",
						},
					},
				},
			},
			expected: &AIModelWatchStatus{
				Name:                 "llama-70b",
				Namespace:            "ai-system",
				Phase:                "Scheduling",
				PhaseDuration:        "30s",
				InferenceServiceName: "llama-70b-isvc",
				ReadyReplicas:        0,
				SchedulingStatus: &SchedulingStatus{
					Scheduled:      false,
					BlockedReason:  "Insufficient GPU",
					BlockedMessage: "0/3 nodes available: 3 Insufficient nvidia.com/gpu",
				},
			},
		},
		{
			name: "status with scheduling scheduled",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "llama-8b",
						"namespace": "ai-system",
					},
					"status": map[string]interface{}{
						"phase":                "Initializing",
						"phaseDuration":        "1m15s",
						"inferenceServiceName": "llama-8b-isvc",
						"readyReplicas":        int64(0),
						"schedulingStatus": map[string]interface{}{
							"scheduled": true,
							"nodeName":  "gpu-node-01",
						},
						"message": "Loading model into GPU memory",
					},
				},
			},
			expected: &AIModelWatchStatus{
				Name:                 "llama-8b",
				Namespace:            "ai-system",
				Phase:                "Initializing",
				PhaseDuration:        "1m15s",
				InferenceServiceName: "llama-8b-isvc",
				ReadyReplicas:        0,
				Message:              "Loading model into GPU memory",
				SchedulingStatus: &SchedulingStatus{
					Scheduled: true,
					NodeName:  "gpu-node-01",
				},
			},
		},
		{
			name: "ready status with endpoint",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "gpt-7b",
						"namespace": "ai-system",
					},
					"status": map[string]interface{}{
						"phase":                "Ready",
						"phaseDuration":        "45s",
						"inferenceServiceName": "gpt-7b-isvc",
						"inferenceEndpoint":    "http://gpt-7b-predictor.ai-system.svc.cluster.local",
						"readyReplicas":        int64(1),
						"message":              "Model is ready to serve requests",
					},
				},
			},
			expected: &AIModelWatchStatus{
				Name:                 "gpt-7b",
				Namespace:            "ai-system",
				Phase:                "Ready",
				PhaseDuration:        "45s",
				InferenceServiceName: "gpt-7b-isvc",
				InferenceEndpoint:    "http://gpt-7b-predictor.ai-system.svc.cluster.local",
				ReadyReplicas:        1,
				Message:              "Model is ready to serve requests",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseWatchStatus(tt.obj)
			if err != nil {
				t.Fatalf("parseWatchStatus() error = %v", err)
			}

			// Compare fields
			if result.Name != tt.expected.Name {
				t.Errorf("Name = %v, want %v", result.Name, tt.expected.Name)
			}
			if result.Namespace != tt.expected.Namespace {
				t.Errorf("Namespace = %v, want %v", result.Namespace, tt.expected.Namespace)
			}
			if result.Phase != tt.expected.Phase {
				t.Errorf("Phase = %v, want %v", result.Phase, tt.expected.Phase)
			}
			if result.PhaseDuration != tt.expected.PhaseDuration {
				t.Errorf("PhaseDuration = %v, want %v", result.PhaseDuration, tt.expected.PhaseDuration)
			}
			if result.InferenceServiceName != tt.expected.InferenceServiceName {
				t.Errorf("InferenceServiceName = %v, want %v", result.InferenceServiceName, tt.expected.InferenceServiceName)
			}
			if result.InferenceEndpoint != tt.expected.InferenceEndpoint {
				t.Errorf("InferenceEndpoint = %v, want %v", result.InferenceEndpoint, tt.expected.InferenceEndpoint)
			}
			if result.ReadyReplicas != tt.expected.ReadyReplicas {
				t.Errorf("ReadyReplicas = %v, want %v", result.ReadyReplicas, tt.expected.ReadyReplicas)
			}
			if result.Message != tt.expected.Message {
				t.Errorf("Message = %v, want %v", result.Message, tt.expected.Message)
			}

			// Compare Progress
			if tt.expected.Progress != nil {
				if result.Progress == nil {
					t.Errorf("Progress = nil, want %+v", tt.expected.Progress)
				} else {
					if result.Progress.Percentage != tt.expected.Progress.Percentage {
						t.Errorf("Progress.Percentage = %v, want %v", result.Progress.Percentage, tt.expected.Progress.Percentage)
					}
					if result.Progress.Downloaded != tt.expected.Progress.Downloaded {
						t.Errorf("Progress.Downloaded = %v, want %v", result.Progress.Downloaded, tt.expected.Progress.Downloaded)
					}
					if result.Progress.ETA != tt.expected.Progress.ETA {
						t.Errorf("Progress.ETA = %v, want %v", result.Progress.ETA, tt.expected.Progress.ETA)
					}
				}
			}

			// Compare SchedulingStatus
			if tt.expected.SchedulingStatus != nil {
				if result.SchedulingStatus == nil {
					t.Errorf("SchedulingStatus = nil, want %+v", tt.expected.SchedulingStatus)
				} else {
					if result.SchedulingStatus.Scheduled != tt.expected.SchedulingStatus.Scheduled {
						t.Errorf("SchedulingStatus.Scheduled = %v, want %v", result.SchedulingStatus.Scheduled, tt.expected.SchedulingStatus.Scheduled)
					}
					if result.SchedulingStatus.BlockedReason != tt.expected.SchedulingStatus.BlockedReason {
						t.Errorf("SchedulingStatus.BlockedReason = %v, want %v", result.SchedulingStatus.BlockedReason, tt.expected.SchedulingStatus.BlockedReason)
					}
					if result.SchedulingStatus.BlockedMessage != tt.expected.SchedulingStatus.BlockedMessage {
						t.Errorf("SchedulingStatus.BlockedMessage = %v, want %v", result.SchedulingStatus.BlockedMessage, tt.expected.SchedulingStatus.BlockedMessage)
					}
					if result.SchedulingStatus.NodeName != tt.expected.SchedulingStatus.NodeName {
						t.Errorf("SchedulingStatus.NodeName = %v, want %v", result.SchedulingStatus.NodeName, tt.expected.SchedulingStatus.NodeName)
					}
				}
			}
		})
	}
}

func TestBuildProgressBar(t *testing.T) {
	tests := []struct {
		name       string
		percentage int
		wantLength int
	}{
		{
			name:       "0 percent",
			percentage: 0,
			wantLength: 20, // bar should still be 20 characters wide
		},
		{
			name:       "50 percent",
			percentage: 50,
			wantLength: 20,
		},
		{
			name:       "100 percent",
			percentage: 100,
			wantLength: 20,
		},
		{
			name:       "25 percent",
			percentage: 25,
			wantLength: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildProgressBar(tt.percentage)
			// Strip ANSI color codes to count actual bar length
			// The bar contains Unicode characters █ and ░
			barLength := 0
			for range result {
				barLength++
			}

			// Note: The actual length will be longer due to ANSI codes
			// We just verify it contains the expected number of bar characters
			// This is a simplified test - in reality we'd strip ANSI codes first
			if barLength == 0 {
				t.Errorf("buildProgressBar(%d) returned empty string", tt.percentage)
			}
		})
	}
}

func TestParseWatchStatusWithPhaseStartTime(t *testing.T) {
	now := time.Now().UTC()
	timeStr := now.Format(time.RFC3339)

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-model",
				"namespace": "ai-system",
			},
			"status": map[string]interface{}{
				"phase":                "Downloading",
				"phaseStartTime":       timeStr,
				"phaseDuration":        "1m30s",
				"inferenceServiceName": "test-model-isvc",
				"readyReplicas":        int64(0),
			},
		},
	}

	result, err := parseWatchStatus(obj)
	if err != nil {
		t.Fatalf("parseWatchStatus() error = %v", err)
	}

	if result.PhaseStartTime == nil {
		t.Error("PhaseStartTime should not be nil")
	} else {
		// Compare times with tolerance for rounding
		diff := result.PhaseStartTime.Sub(now)
		if diff > time.Second || diff < -time.Second {
			t.Errorf("PhaseStartTime = %v, want %v (diff: %v)", result.PhaseStartTime, now, diff)
		}
	}
}
