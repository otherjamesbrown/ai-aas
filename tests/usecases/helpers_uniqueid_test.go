package usecases_test

import (
	"testing"
)

// TestGenerateUniqueID verifies that generateUniqueID produces unique values
// even when called repeatedly in rapid succession
func TestGenerateUniqueID(t *testing.T) {
	const iterations = 1000
	seen := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		id := generateUniqueID()

		// Verify ID format: 14-digit timestamp + 6-char hex suffix
		if len(id) != 20 {
			t.Errorf("ID should be 20 characters (14 timestamp + 6 hex), got %d: %s", len(id), id)
		}

		// Check for collisions
		if seen[id] {
			t.Errorf("Collision detected: ID %s generated more than once", id)
		}
		seen[id] = true
	}

	t.Logf("Generated %d unique IDs successfully", iterations)
}

// TestGenerateUniqueID_Format verifies the structure of generated IDs
func TestGenerateUniqueID_Format(t *testing.T) {
	id := generateUniqueID()

	// Should be exactly 20 characters: YYYYMMDDHHMMSS (14 chars) + 6 hex chars
	if len(id) != 20 {
		t.Errorf("Expected 20 characters, got %d: %s", len(id), id)
	}

	// First 14 chars should be numeric (timestamp)
	timestamp := id[:14]
	for i, c := range timestamp {
		if c < '0' || c > '9' {
			t.Errorf("Timestamp portion (first 14 chars) should be numeric, found non-digit at position %d: %c in %s", i, c, id)
		}
	}

	// Last 6 chars should be hex (0-9a-f)
	suffix := id[14:]
	for i, c := range suffix {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Suffix (last 6 chars) should be hex, found invalid char at position %d: %c in %s", i, c, id)
		}
	}

	t.Logf("Generated ID has correct format: %s (timestamp: %s, suffix: %s)", id, timestamp, suffix)
}
