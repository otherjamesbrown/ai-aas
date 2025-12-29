// Package worker implements the background job worker for server-side model caching.
package worker

import (
	"context"
	"testing"
	"time"
)

// mockCredentialGetter implements CredentialGetter for testing
type mockCredentialGetter struct {
	credentials map[string]string
}

func (m *mockCredentialGetter) GetCredential(ctx context.Context, credType string) (string, error) {
	if val, ok := m.credentials[credType]; ok {
		return val, nil
	}
	return "", ErrCredentialNotFound
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.PollInterval != 5*time.Second {
		t.Errorf("expected PollInterval 5s, got %v", cfg.PollInterval)
	}

	if cfg.ProgressUpdateInterval != 10*time.Second {
		t.Errorf("expected ProgressUpdateInterval 10s, got %v", cfg.ProgressUpdateInterval)
	}
}

func TestWorkerStartStop(t *testing.T) {
	// Test that worker can start and stop without panicking
	// Uses nil pool since we won't actually process jobs
	w := New(nil, &mockCredentialGetter{}, DefaultConfig(), nil)

	// We can't actually start without a valid pool, so just test the interface
	if w == nil {
		t.Error("worker should not be nil")
	}
}

func TestMockCredentialGetter(t *testing.T) {
	mock := &mockCredentialGetter{
		credentials: map[string]string{
			"hf-token":  "hf_test_token",
			"s3-access": "test_access_key",
		},
	}

	ctx := context.Background()

	// Test existing credential
	token, err := mock.GetCredential(ctx, "hf-token")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if token != "hf_test_token" {
		t.Errorf("expected hf_test_token, got %s", token)
	}

	// Test missing credential
	_, err = mock.GetCredential(ctx, "nonexistent")
	if err != ErrCredentialNotFound {
		t.Errorf("expected ErrCredentialNotFound, got %v", err)
	}
}
