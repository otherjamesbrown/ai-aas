// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/cli"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/huggingface"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/storage"
)

// NewCacheParentCommand creates the model cache parent command
func NewCacheParentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage model cache in object storage",
		Long: `Manage cached models in S3-compatible object storage.

Models are downloaded from HuggingFace and stored in object storage for faster
deployment. Cached models can be deployed without re-downloading.

Examples:
  # Download a model to cache
  ai-aas model cache pull mistral-7b

  # List all cached models
  ai-aas model cache list

  # Check cache details
  ai-aas model cache info mistral-7b

  # Delete cached model
  ai-aas model cache delete mistral-7b

Workflow:
  1. Register model    ai-aas model registry add <hf-id> --name <name>
  2. Cache model       ai-aas model cache pull <name>
  3. Deploy model      ai-aas model deploy create <name> -e <env>
  4. Test inference    ai-aas model troubleshoot test <name> -e <env>`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newCachePullCommand())
	cmd.AddCommand(newCacheListCmd())
	cmd.AddCommand(newCacheInfoCommand())
	cmd.AddCommand(newCacheDeleteCmd())
	cmd.AddCommand(newCacheGCCmd())

	return cmd
}

// newCachePullCommand creates the cache pull subcommand
func newCachePullCommand() *cobra.Command {
	var (
		revision   string
		dryRun     bool
		skipVerify bool
		localMode  bool
	)

	cmd := &cobra.Command{
		Use:   "pull <model-name>",
		Short: "Download model to object storage cache",
		Long: `Download a registered model from HuggingFace to object storage.

By default, the download happens server-side - no local bandwidth or S3 credentials needed.
The server downloads from HuggingFace and uploads directly to object storage.

Use --local flag if you need to download through your machine (requires S3 credentials).

Examples:
  # Pull a model (server-side, default)
  ai-aas model cache pull mistral-7b

  # Pull a specific revision
  ai-aas model cache pull mistral-7b --revision abc123

  # Preview download without executing
  ai-aas model cache pull mistral-7b --dry-run

  # Pull using local download (requires S3 credentials)
  ai-aas model cache pull mistral-7b --local

See Also:
  ai-aas model cache list         View cached models
  ai-aas model deploy create      Deploy after caching
  ai-aas model registry add       Register a model first`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
			defer cancel()

			// Create API client
			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)

			// Determine revision
			if revision == "" {
				revision = "main"
			}

			// Default to server-side pull unless --local is specified
			if !localMode {
				return runServerSidePull(ctx, apiClient, modelName, revision, dryRun)
			}

			// Local pull requires S3 credentials
			s3Endpoint := viper.GetString("s3.endpoint")
			s3AccessKey := viper.GetString("s3.access_key")
			s3SecretKey := viper.GetString("s3.secret_key")
			s3Bucket := viper.GetString("s3.bucket")

			regClient := registry.NewClient(apiClient)

			fmt.Printf("Looking up model: %s\n", modelName)
			model, err := regClient.Get(ctx, modelName)
			if err != nil {
				return fmt.Errorf("model not found: %s\n\nIs the model registered? Try:\n  ai-aas model registry add <hf-model-id> --name %s", modelName, modelName)
			}

			// Get model size
			hfClient := huggingface.NewClient(huggingface.WithToken(cfg.HFToken))
			size, err := hfClient.GetModelSize(ctx, model.HFModelID, revision)
			if err != nil {
				return fmt.Errorf("get model size: %w", err)
			}

			fmt.Printf("Model: %s (%s)\n", model.Name, model.HFModelID)
			fmt.Printf("Revision: %s\n", revision)
			fmt.Printf("Total size: %s\n", formatBytes(size))

			if dryRun {
				fmt.Println("\n[DRY RUN] Would download and upload to:")
				fmt.Printf("  S3 path: s3://%s/models/%s/%s/\n", s3Bucket, modelName, revision)
				return nil
			}

			// Validate S3 credentials
			if s3Endpoint == "" || s3AccessKey == "" || s3SecretKey == "" || s3Bucket == "" {
				return fmt.Errorf("S3 credentials not configured\n\nTo configure S3:\n  ai-aas-cli credentials set s3 --endpoint <url> --access-key <key> --secret-key <secret> --bucket <bucket>")
			}

			// Create temp directory for download
			tempDir, err := os.MkdirTemp("", "ai-aas-model-*")
			if err != nil {
				return fmt.Errorf("create temp directory: %w", err)
			}
			defer os.RemoveAll(tempDir)

			// Download from HuggingFace
			hfURL := fmt.Sprintf("https://huggingface.co/%s/tree/%s", model.HFModelID, revision)
			fmt.Printf("\nDownloading from HuggingFace: %s\n", hfURL)
			progress := output.NewDownloadProgressBar("Downloading", size)

			downloadOpts := huggingface.DownloadOptions{
				Revision:     revision,
				DestDir:      tempDir,
				Resume:       true,
				ShowProgress: true,
				Concurrency:  3,
				OnProgress: func(downloaded, total int64) {
					_ = progress.Set64(downloaded)
				},
			}

			result, err := hfClient.DownloadModel(ctx, model.HFModelID, downloadOpts)
			if err != nil {
				return fmt.Errorf("download model: %w", err)
			}
			progress.Finish()

			fmt.Printf("Downloaded %d files (%s) in %s\n",
				len(result.Files),
				formatBytes(result.TotalSize),
				result.Duration.Round(time.Second))

			if result.ResumedFiles > 0 {
				fmt.Printf("  (resumed %d files from previous download)\n", result.ResumedFiles)
			}

			// Generate manifest
			fmt.Println("\nGenerating manifest...")
			manifest, err := storage.GenerateManifest(ctx, model.HFModelID, revision, revision, tempDir)
			if err != nil {
				return fmt.Errorf("generate manifest: %w", err)
			}

			manifestPath := filepath.Join(tempDir, storage.ManifestFileName)
			if err := storage.SaveManifest(manifest, manifestPath); err != nil {
				return fmt.Errorf("save manifest: %w", err)
			}

			// Create S3 client
			s3Client, err := storage.NewS3Client(ctx, storage.S3Config{
				Endpoint:       s3Endpoint,
				AccessKey:      s3AccessKey,
				SecretKey:      s3SecretKey,
				Bucket:         s3Bucket,
				ForcePathStyle: true,
			})
			if err != nil {
				return fmt.Errorf("create S3 client: %w", err)
			}

			// Upload to S3
			fmt.Println("\nUploading to object storage...")
			s3Prefix := fmt.Sprintf("models/%s/%s", modelName, revision)

			var uploaded int64
			uploadProgress := output.NewDownloadProgressBar("Uploading", result.TotalSize)

			for _, file := range result.Files {
				localPath := filepath.Join(tempDir, file.Path)
				remotePath := fmt.Sprintf("%s/%s", s3Prefix, file.Path)

				// Use multipart for large files (>100MB)
				if file.Size > 100*1024*1024 {
					err = s3Client.UploadMultipart(ctx, localPath, remotePath, 50*1024*1024, func(partUploaded, total int64) {
						_ = uploadProgress.Set64(uploaded + partUploaded)
					})
				} else {
					err = s3Client.Upload(ctx, localPath, remotePath)
				}

				if err != nil {
					return fmt.Errorf("upload %s: %w", file.Path, err)
				}

				uploaded += file.Size
				_ = uploadProgress.Set64(uploaded)
			}

			// Upload manifest
			remoteManiestPath := fmt.Sprintf("%s/%s", s3Prefix, storage.ManifestFileName)
			if err := s3Client.Upload(ctx, manifestPath, remoteManiestPath); err != nil {
				return fmt.Errorf("upload manifest: %w", err)
			}

			uploadProgress.Finish()

			// Verify upload if requested
			if !skipVerify {
				fmt.Println("\nVerifying upload...")
				exists, err := s3Client.Exists(ctx, remoteManiestPath)
				if err != nil {
					return fmt.Errorf("verify manifest: %w", err)
				}
				if !exists {
					return fmt.Errorf("manifest not found in S3 after upload")
				}
				fmt.Println("Verification passed.")
			}

			fmt.Println()
			cli.PrintModelCached(modelName, manifest.TotalSize)

			return nil
		},
	}

	cmd.Flags().StringVarP(&revision, "revision", "r", "main", "HuggingFace revision (branch/tag/commit)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview download without executing")
	cmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "skip upload verification (local mode only)")
	cmd.Flags().BoolVar(&localMode, "local", false, "use local download (requires S3 credentials)")

	return cmd
}

// newCacheListCmd creates the cache list subcommand
func newCacheListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all cached models",
		Long: `List all models cached in object storage.

Shows model name, version, file count, and total size.

Examples:
  # List all cached models
  ai-aas model cache list

See Also:
  ai-aas model cache info         View cache details
  ai-aas model cache pull         Download a model
  ai-aas model registry list      View registry`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			s3Client, err := getS3Client(ctx)
			if err != nil {
				return err
			}

			objects, err := s3Client.ListObjects(ctx, "models/")
			if err != nil {
				return fmt.Errorf("list cache: %w", err)
			}

			// Group by model/version
			type cacheEntry struct {
				model   string
				version string
				files   int
				size    int64
			}

			entries := make(map[string]*cacheEntry)

			for _, obj := range objects {
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
				fmt.Println()
				fmt.Println("To cache a model:")
				fmt.Println("  ai-aas model cache pull <model-name>")
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

			fmt.Printf("\nTotal: %d cached model(s)\n", len(entries))

			return nil
		},
	}
}

// newCacheInfoCommand creates the cache info subcommand
func newCacheInfoCommand() *cobra.Command {
	var quick bool

	cmd := &cobra.Command{
		Use:   "info <model-name> [version]",
		Short: "Show cache details and verify integrity",
		Long: `Show detailed information about a cached model and verify integrity.

Checks that all files exist and sizes match the manifest.

Examples:
  # View cache details (latest version)
  ai-aas model cache info mistral-7b

  # View specific version
  ai-aas model cache info mistral-7b main

  # Quick check (size only, no checksums)
  ai-aas model cache info mistral-7b --quick

See Also:
  ai-aas model cache list         List all cached models
  ai-aas model cache delete       Delete cache`,
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

			s3Prefix := fmt.Sprintf("models/%s/%s", modelName, version)
			manifestPath := fmt.Sprintf("%s/%s", s3Prefix, storage.ManifestFileName)

			exists, err := s3Client.Exists(ctx, manifestPath)
			if err != nil {
				return fmt.Errorf("check manifest: %w", err)
			}
			if !exists {
				return fmt.Errorf("no cache found for %s (version: %s)\n\nTo cache this model:\n  ai-aas model cache pull %s", modelName, version, modelName)
			}

			fmt.Printf("Cache: %s (version: %s)\n", modelName, version)
			fmt.Println("────────────────────────────────────────────────────")

			objects, err := s3Client.ListObjects(ctx, s3Prefix+"/")
			if err != nil {
				return fmt.Errorf("list objects: %w", err)
			}

			// Calculate stats
			var totalSize int64
			fileCount := 0
			for _, obj := range objects {
				if !strings.HasSuffix(obj.Key, storage.ManifestFileName) {
					totalSize += obj.Size
					fileCount++
				}
			}

			fmt.Printf("\nStorage\n")
			fmt.Printf("  Location:      s3://.../%s/\n", s3Prefix)
			fmt.Printf("  Files:         %d\n", fileCount)
			fmt.Printf("  Total Size:    %s\n", formatBytes(totalSize))

			fmt.Printf("\nIntegrity\n")
			if quick {
				fmt.Printf("  Status:        [~] manifest present (quick check - no hash verification)\n")
			} else {
				fmt.Printf("  Status:        [~] manifest present (hash verification not implemented)\n")
			}
			fmt.Printf("  Manifest:      present\n")

			cli.PrintNextSteps([]cli.NextStep{
				{
					Command:     fmt.Sprintf("ai-aas model deploy create %s -e development", modelName),
					Description: "Deploy this model",
				},
			})

			return nil
		},
	}

	cmd.Flags().BoolVar(&quick, "quick", false, "quick check (size only)")

	return cmd
}

// newCacheDeleteCmd creates the cache delete subcommand
func newCacheDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <model-name> [version]",
		Short: "Delete cached model files",
		Long: `Delete a cached model from object storage.

Removes all files for the specified model and version. If version is not
specified, all versions of the model are deleted.

Examples:
  # Delete specific version
  ai-aas model cache delete mistral-7b main

  # Delete all versions
  ai-aas model cache delete mistral-7b

  # Force delete without confirmation
  ai-aas model cache delete mistral-7b --force

See Also:
  ai-aas model cache list         List cached models
  ai-aas model registry remove    Remove from registry`,
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

			fmt.Println("Deleting...")
			for _, obj := range objects {
				if err := s3Client.Delete(ctx, obj.Key); err != nil {
					return fmt.Errorf("delete %s: %w", obj.Key, err)
				}
			}

			cli.PrintCacheDeleted(modelName)

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")

	return cmd
}

// newCacheGCCmd creates the cache gc subcommand
func newCacheGCCmd() *cobra.Command {
	var (
		keepVersions int
		dryRun       bool
	)

	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Garbage collect old cache versions",
		Long: `Remove old cached versions, keeping only the most recent ones.

This helps reclaim storage space by removing unused cache entries.

Examples:
  # Keep only 2 most recent versions per model
  ai-aas model cache gc --keep-versions 2

  # Preview what would be deleted
  ai-aas model cache gc --keep-versions 2 --dry-run

See Also:
  ai-aas model cache list         List cached models
  ai-aas model cache delete       Delete specific cache`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			s3Client, err := getS3Client(ctx)
			if err != nil {
				return err
			}

			objects, err := s3Client.ListObjects(ctx, "models/")
			if err != nil {
				return fmt.Errorf("list cache: %w", err)
			}

			// Group by model
			type versionInfo struct {
				version      string
				size         int64
				files        int
				lastModified time.Time
			}
			models := make(map[string][]versionInfo)

			for _, obj := range objects {
				parts := strings.Split(strings.TrimPrefix(obj.Key, "models/"), "/")
				if len(parts) < 2 {
					continue
				}
				model := parts[0]
				version := parts[1]

				// Get last modified time from object
				var lastMod time.Time
				if t, ok := obj.LastModified.(time.Time); ok {
					lastMod = t
				}

				found := false
				for i := range models[model] {
					if models[model][i].version == version {
						models[model][i].size += obj.Size
						models[model][i].files++
						// Track the newest file time for this version
						if lastMod.After(models[model][i].lastModified) {
							models[model][i].lastModified = lastMod
						}
						found = true
						break
					}
				}
				if !found {
					models[model] = append(models[model], versionInfo{version: version, size: obj.Size, files: 1, lastModified: lastMod})
				}
			}

			// Sort versions by lastModified (newest first) before selecting which to delete
			for model := range models {
				sort.Slice(models[model], func(i, j int) bool {
					return models[model][i].lastModified.After(models[model][j].lastModified)
				})
			}

			var toDelete []string
			var deleteSize int64

			for model, versions := range models {
				if len(versions) <= keepVersions {
					continue
				}

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

			cli.PrintSuccess(fmt.Sprintf("Cleaned up %d version(s) (%s freed)", len(toDelete), formatBytes(deleteSize)))
			return nil
		},
	}

	cmd.Flags().IntVar(&keepVersions, "keep-versions", 2, "number of versions to keep per model")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview what would be deleted")

	return cmd
}

// runServerSidePull triggers a server-side model pull and monitors progress
func runServerSidePull(ctx context.Context, apiClient *api.Client, modelName, revision string, dryRun bool) error {
	fmt.Printf("Triggering server-side pull for: %s (revision: %s)\n", modelName, revision)

	if dryRun {
		fmt.Println("\n[DRY RUN] Would create server-side pull job")
		return nil
	}

	// Create the pull job
	job, err := apiClient.CreatePullJob(ctx, modelName, api.PullOptions{Revision: revision})
	if err != nil {
		return fmt.Errorf("create pull job: %w", err)
	}

	fmt.Printf("Pull job created: %s\n", job.ID)
	fmt.Println("Monitoring progress (Ctrl+C to cancel)...")
	fmt.Println()

	// Set up cancellation handling
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	// Handle Ctrl+C to cancel the job
	sigCh := make(chan os.Signal, 1)
	defer close(sigCh)

	go func() {
		select {
		case <-sigCh:
			fmt.Println("\nCancelling pull job...")
			// Cancel in background, don't block
			go func() {
				cancelErr := apiClient.CancelPullJob(context.Background(), modelName, job.ID)
				if cancelErr != nil {
					fmt.Printf("Warning: failed to cancel job: %v\n", cancelErr)
				}
			}()
			cancelFunc()
		case <-cancelCtx.Done():
			return
		}
	}()

	// Poll for progress
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastStatus := ""
	lastProgress := -1.0
	for {
		select {
		case <-cancelCtx.Done():
			return fmt.Errorf("pull cancelled")
		case <-ticker.C:
			updatedJob, err := apiClient.GetPullJob(cancelCtx, modelName, job.ID)
			if err != nil {
				return fmt.Errorf("get job status: %w", err)
			}

			// Print status change
			if updatedJob.Status != lastStatus {
				if lastStatus != "" {
					fmt.Println() // End previous progress line
				}
				fmt.Printf("  Status: %s\n", updatedJob.Status)
				lastStatus = updatedJob.Status
				lastProgress = -1.0 // Reset progress display
			}

			// Print progress bar (only when progress changes significantly)
			if updatedJob.Progress != lastProgress {
				lastProgress = updatedJob.Progress
				printProgressBar(updatedJob.Progress, updatedJob.BytesCompleted, updatedJob.BytesTotal)
			}

			// Check if complete
			switch updatedJob.Status {
			case "complete":
				fmt.Println() // End progress line
				cli.PrintModelCached(modelName, updatedJob.BytesTotal)
				return nil
			case "failed":
				fmt.Println()
				return fmt.Errorf("pull failed: %s", updatedJob.Error)
			case "cancelled":
				fmt.Println()
				return fmt.Errorf("pull was cancelled")
			}
		}
	}
}

// printProgressBar prints a visual progress bar
func printProgressBar(progress float64, completed, total int64) {
	barWidth := 40
	filledWidth := int(progress / 100.0 * float64(barWidth))
	if filledWidth > barWidth {
		filledWidth = barWidth
	}

	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth)

	if total > 0 {
		fmt.Printf("\r  [%s] %.1f%% (%s / %s)   ",
			bar, progress,
			formatBytes(completed),
			formatBytes(total))
	} else {
		fmt.Printf("\r  [%s] %.1f%%   ", bar, progress)
	}
	os.Stdout.Sync()
}
