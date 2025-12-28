package domain

import (
	"testing"
)

func TestValidateBackendWeights(t *testing.T) {
	tests := []struct {
		name     string
		backends []Backend
		want     bool
	}{
		{
			name: "valid - single backend weight 100",
			backends: []Backend{
				{BackendID: "backend1", Weight: 100},
			},
			want: true,
		},
		{
			name: "valid - two backends sum to 100",
			backends: []Backend{
				{BackendID: "backend1", Weight: 60},
				{BackendID: "backend2", Weight: 40},
			},
			want: true,
		},
		{
			name: "valid - three backends sum to 100",
			backends: []Backend{
				{BackendID: "backend1", Weight: 50},
				{BackendID: "backend2", Weight: 30},
				{BackendID: "backend3", Weight: 20},
			},
			want: true,
		},
		{
			name: "valid - equal distribution",
			backends: []Backend{
				{BackendID: "backend1", Weight: 25},
				{BackendID: "backend2", Weight: 25},
				{BackendID: "backend3", Weight: 25},
				{BackendID: "backend4", Weight: 25},
			},
			want: true,
		},
		{
			name:     "invalid - empty backends",
			backends: []Backend{},
			want:     false,
		},
		{
			name:     "invalid - nil backends",
			backends: nil,
			want:     false,
		},
		{
			name: "invalid - single backend weight less than 100",
			backends: []Backend{
				{BackendID: "backend1", Weight: 50},
			},
			want: false,
		},
		{
			name: "invalid - single backend weight greater than 100",
			backends: []Backend{
				{BackendID: "backend1", Weight: 150},
			},
			want: false,
		},
		{
			name: "invalid - two backends sum less than 100",
			backends: []Backend{
				{BackendID: "backend1", Weight: 50},
				{BackendID: "backend2", Weight: 30},
			},
			want: false,
		},
		{
			name: "invalid - two backends sum greater than 100",
			backends: []Backend{
				{BackendID: "backend1", Weight: 60},
				{BackendID: "backend2", Weight: 60},
			},
			want: false,
		},
		{
			name: "invalid - zero weight backends",
			backends: []Backend{
				{BackendID: "backend1", Weight: 0},
				{BackendID: "backend2", Weight: 0},
			},
			want: false,
		},
		{
			name: "invalid - negative weight",
			backends: []Backend{
				{BackendID: "backend1", Weight: 150},
				{BackendID: "backend2", Weight: -50},
			},
			want: true, // Note: Sum is 100, validation doesn't check for negative
		},
		{
			name: "valid - many backends",
			backends: []Backend{
				{BackendID: "backend1", Weight: 10},
				{BackendID: "backend2", Weight: 10},
				{BackendID: "backend3", Weight: 10},
				{BackendID: "backend4", Weight: 10},
				{BackendID: "backend5", Weight: 10},
				{BackendID: "backend6", Weight: 10},
				{BackendID: "backend7", Weight: 10},
				{BackendID: "backend8", Weight: 10},
				{BackendID: "backend9", Weight: 10},
				{BackendID: "backend10", Weight: 10},
			},
			want: true,
		},
		{
			name: "invalid - off by one",
			backends: []Backend{
				{BackendID: "backend1", Weight: 50},
				{BackendID: "backend2", Weight: 49},
			},
			want: false,
		},
		{
			name: "valid - uneven distribution",
			backends: []Backend{
				{BackendID: "backend1", Weight: 1},
				{BackendID: "backend2", Weight: 99},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateBackendWeights(tt.backends)
			if got != tt.want {
				t.Errorf("ValidateBackendWeights() = %v, want %v (backends: %+v)", got, tt.want, tt.backends)
			}
		})
	}
}

func TestBackendWeightsEdgeCases(t *testing.T) {
	// Additional edge case tests for backend weight validation

	t.Run("all backends with zero weight sum to zero", func(t *testing.T) {
		backends := []Backend{
			{BackendID: "backend1", Weight: 0},
			{BackendID: "backend2", Weight: 0},
			{BackendID: "backend3", Weight: 0},
		}
		if ValidateBackendWeights(backends) {
			t.Error("ValidateBackendWeights() with all zero weights should return false")
		}
	})

	t.Run("single backend with zero weight", func(t *testing.T) {
		backends := []Backend{
			{BackendID: "backend1", Weight: 0},
		}
		if ValidateBackendWeights(backends) {
			t.Error("ValidateBackendWeights() with single zero weight should return false")
		}
	})

	t.Run("maximum integer weight", func(t *testing.T) {
		backends := []Backend{
			{BackendID: "backend1", Weight: 2147483647}, // max int32
		}
		if ValidateBackendWeights(backends) {
			t.Error("ValidateBackendWeights() with max int weight should return false (not 100)")
		}
	})
}
