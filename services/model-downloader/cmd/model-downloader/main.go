package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/ai-aas/shared-go/modelcache/pull"
)

const (
	defaultConcurrency        = 1
	defaultMultipartThreshold = 100 * 1024 * 1024 // 100MB
	defaultPartSize           = 50 * 1024 * 1024  // 50MB
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("FATAL: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	// Required environment variables
	modelID := os.Getenv("MODEL_ID")
	s3Bucket := os.Getenv("S3_BUCKET")
	s3Key := os.Getenv("S3_KEY")

	if modelID == "" {
		return fmt.Errorf("MODEL_ID environment variable is required")
	}
	if s3Bucket == "" {
		return fmt.Errorf("S3_BUCKET environment variable is required")
	}
	if s3Key == "" {
		return fmt.Errorf("S3_KEY environment variable is required")
	}

	// Optional environment variables
	hfToken := os.Getenv("HF_TOKEN")
	s3Endpoint := getEnvWithFallback("S3_ENDPOINT", "AWS_ENDPOINT_URL_S3")
	s3AccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	s3SecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Region := os.Getenv("AWS_REGION")
	if s3Region == "" {
		s3Region = "us-east-1"
	}

	// Configuration for S3 upload behavior
	uploadConcurrency := getEnvInt("S3_UPLOAD_CONCURRENCY", defaultConcurrency)
	multipartThreshold := getEnvInt64("S3_MULTIPART_THRESHOLD", defaultMultipartThreshold)
	partSize := getEnvInt64("S3_PART_SIZE", defaultPartSize)

	log.Printf("Starting model download")
	log.Printf("  Model ID: %s", modelID)
	log.Printf("  S3 Bucket: %s", s3Bucket)
	log.Printf("  S3 Key: %s", s3Key)
	log.Printf("  S3 Endpoint: %s", s3Endpoint)
	log.Printf("  Upload Concurrency: %d", uploadConcurrency)
	log.Printf("  Multipart Threshold: %d bytes (%.1f MB)", multipartThreshold, float64(multipartThreshold)/(1024*1024))
	log.Printf("  Part Size: %d bytes (%.1f MB)", partSize, float64(partSize)/(1024*1024))

	// Create pull service
	cfg := pull.Config{
		HFToken:          hfToken,
		S3Endpoint:       s3Endpoint,
		S3AccessKey:      s3AccessKey,
		S3SecretKey:      s3SecretKey,
		S3Bucket:         s3Bucket,
		S3Region:         s3Region,
		S3ForcePathStyle: true, // Required for Linode Object Storage and other S3-compatible services
		Concurrency:      uploadConcurrency,
	}

	svc, err := pull.NewService(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create pull service: %w", err)
	}

	// Set up progress callbacks
	opts := pull.PullOptions{
		ModelName: modelID, // Use model ID as name
		HFModelID: modelID,
		Revision:  "main",
		S3Prefix:  s3Key,
		OnProgress: func(phase string, bytesCompleted, bytesTotal int64) {
			if bytesTotal > 0 {
				pct := float64(bytesCompleted) / float64(bytesTotal) * 100
				log.Printf("Progress [%s]: %.1f%% (%d/%d bytes)", phase, pct, bytesCompleted, bytesTotal)
			} else {
				log.Printf("Progress [%s]: %d bytes", phase, bytesCompleted)
			}
		},
		OnFileStart: func(filename string) {
			log.Printf("  Downloading: %s", filename)
		},
		OnFileComplete: func(filename string, size int64) {
			sizeMB := float64(size) / (1024 * 1024)
			log.Printf("  ✓ Completed: %s (%.1f MB)", filename, sizeMB)
		},
	}

	// Execute the pull operation
	log.Printf("Starting download from HuggingFace Hub...")
	result, err := svc.Pull(ctx, opts)
	if err != nil {
		return fmt.Errorf("pull model: %w", err)
	}

	// Log success
	log.Printf("SUCCESS!")
	log.Printf("  Model: %s", result.ModelName)
	log.Printf("  Revision: %s", result.Revision)
	log.Printf("  S3 Path: %s", result.S3Path)
	log.Printf("  Files: %d", result.FileCount)
	log.Printf("  Total Size: %.2f MB", float64(result.TotalSize)/(1024*1024))
	log.Printf("  Duration: %s", result.Duration)
	log.Printf("  Manifest: %s", result.ManifestPath)

	return nil
}

// getEnvWithFallback returns the value of the first env var that is set, or empty string
func getEnvWithFallback(names ...string) string {
	for _, name := range names {
		if val := os.Getenv(name); val != "" {
			return val
		}
	}
	return ""
}

// getEnvInt parses an environment variable as an integer, returning defaultValue if not set or invalid
func getEnvInt(name string, defaultValue int) int {
	val := os.Getenv(name)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("WARNING: Invalid value for %s: %s (using default: %d)", name, val, defaultValue)
		return defaultValue
	}
	return i
}

// getEnvInt64 parses an environment variable as an int64, returning defaultValue if not set or invalid
func getEnvInt64(name string, defaultValue int64) int64 {
	val := os.Getenv(name)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		log.Printf("WARNING: Invalid value for %s: %s (using default: %d)", name, val, defaultValue)
		return defaultValue
	}
	return i
}
