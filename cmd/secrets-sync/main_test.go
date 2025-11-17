package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectRepo(t *testing.T) {
	// Test repository detection from git remote
	// This is a basic test - in production would mock git commands
	err := detectRepo()
	if err != nil {
		// If git repo not available in test environment, skip
		t.Skipf("Cannot test repo detection: %v", err)
	}

	if repoOwner == "" || repoName == "" {
		t.Error("detectRepo should set repoOwner and repoName")
	}
}

func TestValidateGitignore(t *testing.T) {
	// Create temporary directory with .gitignore
	tmpDir := t.TempDir()
	gitignorePath := filepath.Join(tmpDir, ".gitignore")

	// Test missing .gitignore
	err := validateGitignore()
	if err == nil {
		t.Error("validateGitignore should fail when .gitignore is missing")
	}

	// Create .gitignore without required patterns
	err = os.WriteFile(gitignorePath, []byte("# test\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write .gitignore: %v", err)
	}

	// Temporarily change to tmpDir to test validation
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	os.Chdir(tmpDir)
	err = validateGitignore()
	if err == nil {
		t.Error("validateGitignore should fail when required patterns are missing")
	}

	// Add required patterns
	gitignoreContent := `.env.linode
.env.local
.env.*
`
	err = os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
	if err != nil {
		t.Fatalf("Failed to update .gitignore: %v", err)
	}

	err = validateGitignore()
	if err != nil {
		t.Errorf("validateGitignore should pass with required patterns: %v", err)
	}
}

func TestWriteEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env.test")

	secrets := map[string]string{
		"POSTGRES_PASSWORD": "testpass",
		"REDIS_PASSWORD":    "redispass",
		"MINIO_ROOT_USER":   "minioadmin",
	}

	err := writeEnvFile(envPath, secrets, false)
	if err != nil {
		t.Fatalf("writeEnvFile failed: %v", err)
	}

	// Check file exists and has correct permissions
	info, err := os.Stat(envPath)
	if err != nil {
		t.Fatalf("File should exist: %v", err)
	}

	// Check permissions (0600)
	if info.Mode().Perm() != 0600 {
		t.Errorf("File permissions should be 0600, got %o", info.Mode().Perm())
	}

	// Check file contents
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	if !contains(contentStr, "POSTGRES_PASSWORD=testpass") {
		t.Error("File should contain POSTGRES_PASSWORD")
	}
	if !contains(contentStr, "MINIO_ROOT_USER=minioadmin") {
		t.Error("File should contain MINIO_ROOT_USER")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || len(s) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
