package perf

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// repoRoot finds the repository root by looking for go.mod
func repoRoot() string {
	// Start from the test file location and walk up
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod
			return ""
		}
		dir = parent
	}
}

func TestRollbackDowntimeUnderOneMinute(t *testing.T) {
	version := os.Getenv("MIGRATION_TEST_VERSION")
	if version == "" {
		t.Skip("MIGRATION_TEST_VERSION not set; skipping downtime measurement")
	}

	envFile := os.Getenv("MIGRATION_ENV_FILE")
	if envFile == "" {
		envFile = "migrate.env"
	}

	root := repoRoot()
	if root == "" {
		t.Fatal("could not find repository root (go.mod)")
	}

	rollbackScript := filepath.Join(root, "scripts", "db", "rollback.sh")
	applyScript := filepath.Join(root, "scripts", "db", "apply.sh")

	start := time.Now()
	cmd := exec.Command(rollbackScript, "--component", "operational", "--env", envFile, "--version", version, "--yes")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "MIGRATION_REQUIRE_CONFIRMATION=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	rollbackDuration := time.Since(start)
	if rollbackDuration > time.Minute {
		t.Fatalf("rollback duration exceeded 60s: %s", rollbackDuration)
	}

	// Reapply to restore state for subsequent tests.
	apply := exec.Command(applyScript, "--component", "operational", "--env", envFile, "--version", version, "--yes")
	apply.Dir = root
	apply.Env = append(os.Environ(), "MIGRATION_REQUIRE_CONFIRMATION=0")
	apply.Stdout = os.Stdout
	apply.Stderr = os.Stderr
	if err := apply.Run(); err != nil {
		t.Fatalf("reapply failed: %v", err)
	}
}
