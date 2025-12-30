package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSchedulingMessage(t *testing.T) {
	r := &AIModelReconciler{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Insufficient CPU",
			input:    "0/6 nodes are available: 2 Insufficient cpu, 4 Insufficient memory.",
			expected: "0/6 nodes are available: 2 Insufficient CPU, 4 Insufficient Memory.",
		},
		{
			name:     "Insufficient GPU",
			input:    "0/3 nodes are available: 3 Insufficient nvidia.com/gpu.",
			expected: "0/3 nodes are available: 3 Insufficient GPU.",
		},
		{
			name:     "Node selector mismatch",
			input:    "0/3 nodes are available: 3 node(s) didn't match Pod's node affinity/selector.",
			expected: "0/3 nodes are available: 3 nodes don't match pod's node selector.",
		},
		{
			name:     "No changes needed",
			input:    "Some other message",
			expected: "Some other message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.parseSchedulingMessage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
