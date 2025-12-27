package apikey

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestComputeFingerprint(t *testing.T) {
	testKey := "test-api-key-123"

	// Compute expected values manually
	hash := sha256.Sum256([]byte(testKey))
	expectedHex := hex.EncodeToString(hash[:])
	expectedBase64 := base64.RawURLEncoding.EncodeToString(hash[:])

	tests := []struct {
		name     string
		apiKey   string
		format   FingerprintFormat
		expected string
	}{
		{
			name:     "hex format",
			apiKey:   testKey,
			format:   FingerprintHex,
			expected: expectedHex,
		},
		{
			name:     "base64url format",
			apiKey:   testKey,
			format:   FingerprintBase64URL,
			expected: expectedBase64,
		},
		{
			name:     "empty key hex",
			apiKey:   "",
			format:   FingerprintHex,
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeFingerprint(tt.apiKey, tt.format)
			if got != tt.expected {
				t.Errorf("ComputeFingerprint(%q, %d) = %q, want %q", tt.apiKey, tt.format, got, tt.expected)
			}
		})
	}
}

func TestComputeFingerprintHex(t *testing.T) {
	testKey := "my-secret-key"
	got := ComputeFingerprintHex(testKey)

	// Verify it matches manual computation
	hash := sha256.Sum256([]byte(testKey))
	expected := hex.EncodeToString(hash[:])

	if got != expected {
		t.Errorf("ComputeFingerprintHex(%q) = %q, want %q", testKey, got, expected)
	}

	// Verify it's 64 characters (256 bits = 32 bytes = 64 hex chars)
	if len(got) != 64 {
		t.Errorf("ComputeFingerprintHex length = %d, want 64", len(got))
	}
}

func TestComputeFingerprintBase64URL(t *testing.T) {
	testKey := "my-secret-key"
	got := ComputeFingerprintBase64URL(testKey)

	// Verify it matches manual computation
	hash := sha256.Sum256([]byte(testKey))
	expected := base64.RawURLEncoding.EncodeToString(hash[:])

	if got != expected {
		t.Errorf("ComputeFingerprintBase64URL(%q) = %q, want %q", testKey, got, expected)
	}

	// Verify it's 43 characters (256 bits = 32 bytes = 43 base64 chars without padding)
	if len(got) != 43 {
		t.Errorf("ComputeFingerprintBase64URL length = %d, want 43", len(got))
	}

	// Verify no padding characters
	if got[len(got)-1] == '=' {
		t.Error("ComputeFingerprintBase64URL should not have padding")
	}
}

func TestFingerprintConsistency(t *testing.T) {
	// Test that the same key always produces the same fingerprint
	testKey := "consistent-key-test"

	fp1 := ComputeFingerprintHex(testKey)
	fp2 := ComputeFingerprintHex(testKey)
	fp3 := ComputeFingerprintHex(testKey)

	if fp1 != fp2 || fp2 != fp3 {
		t.Error("Fingerprint should be consistent for the same key")
	}
}

func TestFingerprintUniqueness(t *testing.T) {
	// Test that different keys produce different fingerprints
	key1 := "key-one"
	key2 := "key-two"
	key3 := "key-three"

	fp1 := ComputeFingerprintHex(key1)
	fp2 := ComputeFingerprintHex(key2)
	fp3 := ComputeFingerprintHex(key3)

	if fp1 == fp2 || fp2 == fp3 || fp1 == fp3 {
		t.Error("Different keys should produce different fingerprints")
	}
}
