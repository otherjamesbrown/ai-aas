// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/huggingface"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewAddCommand creates the model add command
func NewAddCommand() *cobra.Command {
	var (
		name          string
		requiresAuth  bool
		licenseType   string
		acceptLicense bool
		gpuMemory     int
		cpuMemory     int
	)

	cmd := &cobra.Command{
		Use:   "add <hf-model-id>",
		Short: "Register a model from HuggingFace",
		Long: `Register a model from HuggingFace Hub in the platform registry.

Examples:
  # Add a public model
  ai-aas-cli model add meta-llama/Llama-3-8B-Instruct --name llama-3-8b

  # Add a gated model with license acceptance
  ai-aas-cli model add meta-llama/Llama-3-8B-Instruct --name llama-3-8b --accept-license

  # Add with resource recommendations
  ai-aas-cli model add mistralai/Mistral-7B-v0.1 --name mistral-7b --gpu-memory 24`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hfModelID := args[0]

			// Get configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if cfg.APIEndpoint == "" || cfg.APIEndpoint == "http://localhost:8080" {
				return fmt.Errorf("API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// Check model on HuggingFace
			hfClient := huggingface.NewClient(huggingface.WithToken(cfg.HFToken))
			
			fmt.Printf("Checking model on HuggingFace Hub: %s\n", hfModelID)
			
			modelInfo, err := hfClient.GetModel(ctx, hfModelID)
			if err != nil {
				if err == huggingface.ErrModelNotFound {
					return fmt.Errorf("model not found on HuggingFace Hub: %s", hfModelID)
				}
				if err == huggingface.ErrForbidden {
					return fmt.Errorf("access denied to model %s. This may be a gated model requiring license acceptance.\nVisit: https://huggingface.co/%s", hfModelID, hfModelID)
				}
				return fmt.Errorf("failed to check model: %w", err)
			}

			// Check if model is gated
			if modelInfo.Gated {
				if !acceptLicense {
					fmt.Println("\n⚠️  This is a gated model requiring license acceptance.")
					fmt.Printf("   License: %s\n", huggingface.GetLicenseFullName(licenseType))
					fmt.Printf("   Accept license at: https://huggingface.co/%s\n\n", hfModelID)
					fmt.Println("After accepting the license on HuggingFace, run again with --accept-license")
					os.Exit(1)
				}
				requiresAuth = true
			}

			// Determine if auth is required
			if modelInfo.Private {
				requiresAuth = true
			}

			// Extract license info
			if licenseType == "" {
				if info, err := hfClient.GetLicenseInfo(ctx, hfModelID); err == nil {
					licenseType = info.LicenseType
				}
			}

			// Register in platform
			fmt.Printf("Registering model in platform as: %s\n", name)

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			model, err := regClient.Add(ctx, registry.AddRequest{
				HFModelID:     hfModelID,
				Name:          name,
				RequiresAuth:  requiresAuth,
				LicenseType:   licenseType,
				AcceptLicense: acceptLicense,
				GPUMemoryGB:   gpuMemory,
				CPUMemoryGB:   cpuMemory,
			})
			if err != nil {
				return fmt.Errorf("failed to register model: %w", err)
			}

			fmt.Printf("\n✅ Model registered successfully!\n")
			fmt.Printf("   Name:      %s\n", model.Name)
			fmt.Printf("   HF Model:  %s\n", model.HFModelID)
			fmt.Printf("   ID:        %s\n", model.ID)
			if model.IsGated {
				fmt.Printf("   License:   %s (accepted)\n", model.LicenseType)
			}

			fmt.Println("\nNext steps:")
			fmt.Printf("   ai-aas-cli model pull %s          # Cache model to object storage\n", name)
			fmt.Printf("   ai-aas-cli model deploy %s -e dev # Deploy to development\n", name)

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "internal name for the model (required)")
	cmd.Flags().BoolVar(&requiresAuth, "requires-auth", false, "model requires HuggingFace authentication")
	cmd.Flags().StringVar(&licenseType, "license", "", "license type (auto-detected if not specified)")
	cmd.Flags().BoolVar(&acceptLicense, "accept-license", false, "accept gated model license (for CI/CD)")
	cmd.Flags().IntVar(&gpuMemory, "gpu-memory", 0, "recommended GPU memory in GB")
	cmd.Flags().IntVar(&cpuMemory, "cpu-memory", 0, "recommended CPU memory in GB")
	
	cmd.MarkFlagRequired("name")

	return cmd
}

