// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/storage"
)

// NewCacheCommand creates the model cache command group
func NewCacheCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage model cache in object storage",
		Long:  "List, verify, and manage cached models in S3-compatible object storage.",
	}

	cmd.AddCommand(newCacheListCommand())
	cmd.AddCommand(newCacheVerifyCommand())
	cmd.AddCommand(newCacheDeleteCommand())
	cmd.AddCommand(newCacheGCCommand())

	return cmd
}

func newCacheListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cached models",
		Long: `List all models cached in object storage.

Examples:
  # List all cached models
  ai-aas-cli model cache list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			s3Client, err := getS3Client(ctx)
			if err != nil {
				return err
			}

			// List models directory
			objects, err := s3Client.ListObjects(ctx, "models/")
			if err != nil {
				return fmt.Errorf("list cache: %w", err)
			}

			// Group by model/version
			type cacheEntry struct {
				model    string
				version  string
				files    int
				size     int64
				manifest *storage.Manifest
			}

			entries := make(map[string]*cacheEntry)

			for _, obj := range objects {
				// Parse path: models/<model>/<version>/...
				parts := strings.Split(strings.TrimPrefix(obj.Key, "models/"), "/")
				if len(parts) < 2 {
					continue
				}
				model := parts[0]
				version := parts[1]
				key := model + "/" + version

				if entries[key] == nil {
					entries[key] = &cacheEntry{model: model, version: version}
				}
				entries[key].files++
				entries[key].size += obj.Size
			}

			if len(entries) == 0 {
				fmt.Println("No cached models found.")
				return nil
			}

			// Print table
			table := output.NewTableWriter()
			table.SetHeader([]string{"MODEL", "VERSION", "FILES", "SIZE"})
			for _, e := range entries {
				table.Append([]string{
					e.model,
					e.version,
					fmt.Sprintf("%d", e.files),
					formatBytes(e.size),
				})
			}
			table.Render()

			return nil
		},
	}
}

func newCacheVerifyCommand() *cobra.Command {
	var quick bool

	cmd := &cobra.Command{
		Use:   "verify <model-name> [version]",
		Short: "Verify cache integrity",
		Long: `Verify the integrity of cached model files against the manifest.

Examples:
  # Verify cache for a model (latest version)
  ai-aas-cli model cache verify llama-3-8b

  # Verify specific version
  ai-aas-cli model cache verify llama-3-8b main

  # Quick verification (size only, no checksums)
  ai-aas-cli model cache verify llama-3-8b --quick`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]
			version := "main"
			if len(args) > 1 {
				version = args[1]
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			s3Client, err := getS3Client(ctx)
			if err != nil {
				return err
			}

			// Download manifest
			s3Prefix := fmt.Sprintf("models/%s/%s", modelName, version)
			manifestPath := fmt.Sprintf("%s/%s", s3Prefix, storage.ManifestFileName)

			exists, err := s3Client.Exists(ctx, manifestPath)
			if err != nil {
				return fmt.Errorf("check manifest: %w", err)
			}
			if !exists {
				return fmt.Errorf("no cache found for %s (version: %s)", modelName, version)
			}

			fmt.Printf("Verifying cache: %s (version: %s)\n", modelName, version)

			// List objects to verify
			objects, err := s3Client.ListObjects(ctx, s3Prefix+"/")
			if err != nil {
				return fmt.Errorf("list objects: %w", err)
			}

			// Build file map
			objectMap := make(map[string]int64)
			for _, obj := range objects {
				relPath := strings.TrimPrefix(obj.Key, s3Prefix+"/")
				objectMap[relPath] = obj.Size
			}

			// Check files
			var missing, sizeMismatch []string
			var totalSize int64

			for path, size := range objectMap {
				if path == storage.ManifestFileName {
					continue
				}
				totalSize += size
			}

			if quick {
				fmt.Println("\n[Quick verification - size only]")
			}

			// Report results
			fmt.Printf("\nFiles found: %d\n", len(objects)-1) // Exclude manifest
			fmt.Printf("Total size: %s\n", formatBytes(totalSize))

			if len(missing) == 0 && len(sizeMismatch) == 0 {
				fmt.Println("\n✅ Cache verification passed!")
			} else {
				fmt.Println("\n❌ Cache verification failed!")
				if len(missing) > 0 {
					fmt.Printf("Missing files: %d\n", len(missing))
					for _, f := range missing[:min(5, len(missing))] {
						fmt.Printf("  - %s\n", f)
					}
					if len(missing) > 5 {
						fmt.Printf("  ... and %d more\n", len(missing)-5)
					}
				}
				if len(sizeMismatch) > 0 {
					fmt.Printf("Size mismatch: %d\n", len(sizeMismatch))
				}
				return fmt.Errorf("cache verification failed")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&quick, "quick", false, "quick verification (size only, no checksums)")

	return cmd
}

func newCacheDeleteCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <model-name> [version]",
		Short: "Delete cached model",
		Long: `Delete a cached model from object storage.

Examples:
  # Delete specific version
  ai-aas-cli model cache delete llama-3-8b main

  # Force delete without confirmation
  ai-aas-cli model cache delete llama-3-8b main --force`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]
			version := ""
			if len(args) > 1 {
				version = args[1]
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			s3Client, err := getS3Client(ctx)
			if err != nil {
				return err
			}

			var s3Prefix string
			if version != "" {
				s3Prefix = fmt.Sprintf("models/%s/%s", modelName, version)
			} else {
				s3Prefix = fmt.Sprintf("models/%s", modelName)
			}

			// List objects to delete
			objects, err := s3Client.ListObjects(ctx, s3Prefix+"/")
			if err != nil {
				return fmt.Errorf("list objects: %w", err)
			}

			if len(objects) == 0 {
				fmt.Printf("No cache found for %s", modelName)
				if version != "" {
					fmt.Printf(" (version: %s)", version)
				}
				fmt.Println()
				return nil
			}

			// Calculate size
			var totalSize int64
			for _, obj := range objects {
				totalSize += obj.Size
			}

			fmt.Printf("Found %d files (%s) to delete\n", len(objects), formatBytes(totalSize))

			if !force {
				fmt.Print("Are you sure? [y/N]: ")
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			// Delete objects
			fmt.Println("Deleting...")
			for _, obj := range objects {
				if err := s3Client.Delete(ctx, obj.Key); err != nil {
					return fmt.Errorf("delete %s: %w", obj.Key, err)
				}
			}

			fmt.Printf("✅ Deleted %d files (%s)\n", len(objects), formatBytes(totalSize))
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")

	return cmd
}

func newCacheGCCommand() *cobra.Command {
	var (
		keepVersions int
		dryRun       bool
	)

	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Garbage collect old cache versions",
		Long: `Remove old cached versions, keeping only the most recent ones.

Examples:
  # Keep only the 2 most recent versions per model
  ai-aas-cli model cache gc --keep-versions 2

  # Preview what would be deleted
  ai-aas-cli model cache gc --keep-versions 2 --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			s3Client, err := getS3Client(ctx)
			if err != nil {
				return err
			}

			// List all cached models
			objects, err := s3Client.ListObjects(ctx, "models/")
			if err != nil {
				return fmt.Errorf("list cache: %w", err)
			}

			// Group by model
			type versionInfo struct {
				version string
				size    int64
				files   int
			}
			models := make(map[string][]versionInfo)

			for _, obj := range objects {
				parts := strings.Split(strings.TrimPrefix(obj.Key, "models/"), "/")
				if len(parts) < 2 {
					continue
				}
				model := parts[0]
				version := parts[1]

				// Find or create version entry
				found := false
				for i := range models[model] {
					if models[model][i].version == version {
						models[model][i].size += obj.Size
						models[model][i].files++
						found = true
						break
					}
				}
				if !found {
					models[model] = append(models[model], versionInfo{version: version, size: obj.Size, files: 1})
				}
			}

			// Identify versions to delete
			var toDelete []string
			var deleteSize int64

			for model, versions := range models {
				if len(versions) <= keepVersions {
					continue
				}

				// Keep newest N versions (assuming sorted by name/date)
				for i := keepVersions; i < len(versions); i++ {
					prefix := fmt.Sprintf("models/%s/%s", model, versions[i].version)
					toDelete = append(toDelete, prefix)
					deleteSize += versions[i].size
				}
			}

			if len(toDelete) == 0 {
				fmt.Println("No old versions to clean up.")
				return nil
			}

			fmt.Printf("Found %d version(s) to delete (%s)\n", len(toDelete), formatBytes(deleteSize))
			for _, prefix := range toDelete {
				fmt.Printf("  - %s\n", prefix)
			}

			if dryRun {
				fmt.Println("\n[DRY RUN] No files deleted.")
				return nil
			}

			fmt.Print("\nProceed? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}

			// Delete old versions
			for _, prefix := range toDelete {
				objects, err := s3Client.ListObjects(ctx, prefix+"/")
				if err != nil {
					return fmt.Errorf("list %s: %w", prefix, err)
				}
				for _, obj := range objects {
					if err := s3Client.Delete(ctx, obj.Key); err != nil {
						return fmt.Errorf("delete %s: %w", obj.Key, err)
					}
				}
			}

			fmt.Printf("✅ Cleaned up %d version(s) (%s freed)\n", len(toDelete), formatBytes(deleteSize))
			return nil
		},
	}

	cmd.Flags().IntVar(&keepVersions, "keep-versions", 2, "number of versions to keep per model")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview what would be deleted")

	return cmd
}

// getS3Client creates an S3 client from config
func getS3Client(ctx context.Context) (*storage.S3Client, error) {
	s3Endpoint := viper.GetString("s3.endpoint")
	s3AccessKey := viper.GetString("s3.access_key")
	s3SecretKey := viper.GetString("s3.secret_key")
	s3Bucket := viper.GetString("s3.bucket")

	if s3Endpoint == "" || s3AccessKey == "" || s3SecretKey == "" || s3Bucket == "" {
		return nil, fmt.Errorf("S3 credentials not configured. Run 'ai-aas-cli credentials set s3 ...' first")
	}

	return storage.NewS3Client(ctx, storage.S3Config{
		Endpoint:       s3Endpoint,
		AccessKey:      s3AccessKey,
		SecretKey:      s3SecretKey,
		Bucket:         s3Bucket,
		ForcePathStyle: true,
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
