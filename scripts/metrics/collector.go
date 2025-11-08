package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Metric struct {
	RunID            string  `json:"run_id"`
	Service          string  `json:"service"`
	Command          string  `json:"command"`
	Status           string  `json:"status"`
	StartedAt        string  `json:"started_at"`
	FinishedAt       string  `json:"finished_at"`
	DurationSeconds  float64 `json:"duration_seconds"`
	CommitSHA        string  `json:"commit_sha"`
	Actor            string  `json:"actor"`
	Environment      string  `json:"environment"`
	CollectorVersion string  `json:"collector_version"`
}

func main() {
	outDir := flag.String("out-dir", "scripts/metrics/output", "directory to write metric files")
	runID := flag.String("run-id", "", "unique run identifier")
	service := flag.String("service", "all", "service name")
	command := flag.String("command", "", "command executed")
	status := flag.String("status", "success", "status (success/failure/cancelled)")
	duration := flag.Float64("duration", 0, "duration in seconds")
	commit := flag.String("commit", os.Getenv("GITHUB_SHA"), "commit SHA")
	actor := flag.String("actor", os.Getenv("GITHUB_ACTOR"), "triggering actor")
	env := flag.String("env", defaultEnv(), "environment label")
	collectorVersion := flag.String("collector-version", "1.0.0", "collector semantic version")
	flag.Parse()

	if *runID == "" {
		fmt.Fprintln(os.Stderr, "--run-id is required")
		os.Exit(1)
	}
	if *command == "" {
		fmt.Fprintln(os.Stderr, "--command is required")
		os.Exit(1)
	}

	now := time.Now().UTC()
	metric := Metric{
		RunID:            *runID,
		Service:          *service,
		Command:          *command,
		Status:           *status,
		StartedAt:        now.Add(-time.Duration(*duration * float64(time.Second))).Format(time.RFC3339),
		FinishedAt:       now.Format(time.RFC3339),
		DurationSeconds:  *duration,
		CommitSHA:        *commit,
		Actor:            *actor,
		Environment:      *env,
		CollectorVersion: *collectorVersion,
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output dir: %v\n", err)
		os.Exit(1)
	}

	filePath := filepath.Join(*outDir, fmt.Sprintf("%s.json", *runID))
	f, err := os.Create(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(metric); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode metric: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Metric written to %s\n", filePath)
}

func defaultEnv() string {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return "github-actions"
	}
	return "local"
}
