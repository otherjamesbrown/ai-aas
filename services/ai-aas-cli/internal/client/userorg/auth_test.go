// Package userorg provides tests for token validation.
package userorg

import (
	"testing"
	"time"
)

func TestValidateToken(t *testing.T) {
	now := time.Now()

	// Valid token (expires in 1 hour)
	expiresAt := now.Add(1 * time.Hour)
	if err := ValidateToken(expiresAt); err != nil {
		t.Errorf("ValidateToken() should accept valid token: %v", err)
	}

	// Token expired (1 hour ago)
	expiresAt = now.Add(-1 * time.Hour)
	if err := ValidateToken(expiresAt); err == nil {
		t.Error("ValidateToken() should reject expired token")
	}

	// Token within clock skew tolerance (expired 4 minutes ago, tolerance ±5 minutes)
	expiresAt = now.Add(-4 * time.Minute)
	if err := ValidateToken(expiresAt); err != nil {
		t.Errorf("ValidateToken() should accept token within clock skew tolerance: %v", err)
	}

	// Token too far in future (indicates clock issues)
	expiresAt = now.Add(25 * time.Hour)
	if err := ValidateToken(expiresAt); err == nil {
		t.Error("ValidateToken() should reject token too far in future")
	}
}

