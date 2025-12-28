// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/huggingface"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewInfoCommand creates the model info command
func NewInfoCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "info <model-name>",
		Short: "Show detailed model information",
		Long: `Display detailed information about a registered model.

Examples:
  ai-aas-cli model info llama-3-8b
  ai-aas-cli model info llama-3-8b --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Use Admin API endpoint for registry operations
			adminEndpoint := cfg.GetAdminEndpoint()

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			regClient := registry.NewClient(apiClient)

			model, err := regClient.Get(ctx, name)
			if err != nil {
				return fmt.Errorf("failed to get model: %w", err)
			}

			if format == "json" {
				return output.PrintJSON(model, true)
			}

			// Display model information
			fmt.Printf("Model: %s\n", model.Name)
			fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			
			// Registry info
			fmt.Printf("\n📋 Registry\n")
			fmt.Printf("   ID:            %s\n", model.ID)
			fmt.Printf("   HF Model ID:   %s\n", model.HFModelID)
			fmt.Printf("   HF Revision:   %s\n", model.HFRevision)
			fmt.Printf("   Created:       %s\n", model.CreatedAt.Format(time.RFC3339))
			fmt.Printf("   Updated:       %s\n", model.UpdatedAt.Format(time.RFC3339))

			// License info
			if model.IsGated || model.LicenseType != "" {
				fmt.Printf("\n📜 License\n")
				if model.LicenseType != "" {
					fmt.Printf("   Type:          %s\n", huggingface.GetLicenseFullName(model.LicenseType))
				}
				if model.IsGated {
					fmt.Printf("   Gated:         Yes\n")
				}
				if model.LicenseAcceptedAt != nil {
					fmt.Printf("   Accepted:      %s\n", model.LicenseAcceptedAt.Format(time.RFC3339))
					fmt.Printf("   Accepted By:   %s\n", model.LicenseAcceptedBy)
				}
			}

			// Authentication
			if model.RequiresAuth {
				fmt.Printf("\n🔐 Authentication\n")
				fmt.Printf("   Requires HF Token: Yes\n")
			}

			// Resource recommendations
			if model.RecommendedGPUMemoryGB > 0 || model.RecommendedCPUMemoryGB > 0 {
				fmt.Printf("\n💾 Resource Recommendations\n")
				if model.RecommendedGPUMemoryGB > 0 {
					fmt.Printf("   GPU Memory:    %d GB\n", model.RecommendedGPUMemoryGB)
				}
				if model.RecommendedCPUMemoryGB > 0 {
					fmt.Printf("   CPU Memory:    %d GB\n", model.RecommendedCPUMemoryGB)
				}
			}

			// Cache status
			fmt.Printf("\n📦 Cache Status\n")
			if model.CacheStatus == "" || model.CacheStatus == "none" {
				fmt.Printf("   Status:        Not cached\n")
			} else {
				fmt.Printf("   Status:        %s\n", model.CacheStatus)
				
				// Get cache details
				cacheEntries, err := regClient.GetCache(ctx, name)
				if err == nil && len(cacheEntries) > 0 {
					latest := cacheEntries[0]
					fmt.Printf("   Version:       %s\n", latest.Version)
					fmt.Printf("   Size:          %s\n", output.FormatBytes(latest.SizeBytes))
					fmt.Printf("   Files:         %d\n", latest.FileCount)
					fmt.Printf("   Cached At:     %s\n", latest.CachedAt.Format(time.RFC3339))
				}
			}

			// Deployment status
			fmt.Printf("\n🚀 Deployment Status\n")
			if model.DeploymentStatus == "" || model.DeploymentStatus == "none" {
				fmt.Printf("   Status:        Not deployed\n")
			} else {
				fmt.Printf("   Status:        %s\n", model.DeploymentStatus)
			}

			// Version pinning
			if model.PinnedVersion != "" {
				fmt.Printf("\n📌 Version Pinning\n")
				fmt.Printf("   Pinned To:     %s\n", model.PinnedVersion)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

