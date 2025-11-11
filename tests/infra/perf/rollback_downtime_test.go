package perf

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestRollbackDowntimeUnderOneMinute(t *testing.T) {
	version := os.Getenv("MIGRATION_TEST_VERSION")
	if version == "" {
		t.Skip("MIGRATION_TEST_VERSION not set; skipping downtime measurement")
	}

	envFile := os.Getenv("MIGRATION_ENV_FILE")
	if envFile == "" {
		envFile = "migrate.env"
	}

	start := time.Now()
	cmd := exec.Command("scripts/db/rollback.sh", "--component", "operational", "--env", envFile, "--version", version, "--yes")
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
	apply := exec.Command("scripts/db/apply.sh", "--component", "operational", "--env", envFile, "--version", version, "--yes")
	apply.Env = append(os.Environ(), "MIGRATION_REQUIRE_CONFIRMATION=0")
	apply.Stdout = os.Stdout
	apply.Stderr = os.Stderr
	if err := apply.Run(); err != nil {
		t.Fatalf("reapply failed: %v", err)
	}
}
