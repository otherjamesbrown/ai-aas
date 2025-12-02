// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/cache"
)

// NewPullCommand creates the model pull command
func NewPullCommand() *cobra.Command {
	var (
		revision   string
		dryRun     bool
		skipVerify bool
	)

	cmd := &cobra.Command{
		Use:   "pull <model-name>",
		Short: "Download model to object storage cache",
		Long: `Download a registered model from HuggingFace to object storage.

The model is first downloaded to a local temp directory, then uploaded to S3.
A manifest file is created to track file integrity.

Examples:
  # Pull a model using default revision (main)
  ai-aas-cli model pull llama-3-8b

  # Pull a specific revision
  ai-aas-cli model pull llama-3-8b --revision abc123

  # Preview download without executing
  ai-aas-cli model pull llama-3-8b --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour) // Long timeout for large models
			defer cancel()

			if err := cache.PullModelToCache(ctx, cache.PullOptions{
				ModelName:  modelName,
				Revision:   revision,
				DryRun:     dryRun,
				SkipVerify: skipVerify,
			}); err != nil {
				return err
			}

			fmt.Println("\nNext steps:")
			fmt.Printf("   ai-aas-cli model deploy %s -e development\n", modelName)

			return nil
		},
	}

	cmd.Flags().StringVarP(&revision, "revision", "r", "main", "HuggingFace revision (branch/tag/commit)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview download without executing")
	cmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "skip upload verification")

	return cmd
}
