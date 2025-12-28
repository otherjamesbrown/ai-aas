package postgres

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func mustJSONB(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func jsonSliceString(b []byte) ([]string, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func jsonSliceStringDefault(b []byte) ([]string, error) {
	s, err := jsonSliceString(b)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return []string{}, nil
	}
	return s, nil
}

func jsonStringMap(b []byte) (map[string]any, error) {
	if len(b) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func uuidPtr(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	val, err := uuid.FromBytes(u.Bytes[:])
	if err != nil {
		return nil
	}
	return &val
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	val := t.String
	return &val
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	ts := t.Time
	return &ts
}

// generateKeyID creates a short, human-readable key ID with the given prefix.
// Format: prefix_xxxxxxxxxxxx (12 random chars, base32 encoded, lowercase)
func generateKeyID(prefix string) string {
	// 8 bytes = 64 bits of entropy, base32 encodes to ~13 chars
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	encoded := base32.StdEncoding.EncodeToString(b)
	// Trim padding and convert to lowercase for readability
	encoded = strings.TrimRight(strings.ToLower(encoded), "=")
	// Truncate to 12 chars for consistent length
	if len(encoded) > 12 {
		encoded = encoded[:12]
	}
	return prefix + "_" + encoded
}
