// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewSwapCommand creates the model swap command
func NewSwapCommand() *cobra.Command {
	var (
		environment string
		force       bool
		reason      string
	)

	cmd := &cobra.Command{
		Use:   "swap <disable-model> <enable-model>",
		Short: "Atomically swap models",
		Long: `Atomically disable one model and enable another.

This performs a graceful swap: the first model is disabled (graceful drain),
then the second model is enabled. This is useful for capacity management
when you need to swap models without having both running simultaneously.

Examples:
  # Swap models in production
  ai-aas-cli model swap mistral-7b llama-3-8b -e production

  # Swap with reason for audit
  ai-aas-cli model swap mistral-7b llama-3-8b -e production --reason "Upgrading to larger model"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			disableModel := args[0]
			enableModel := args[1]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Get kubeconfig for environment
			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))
			s3Bucket := viper.GetString("s3.bucket")

			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  environment,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			disableIsvc := fmt.Sprintf("%s-%s", disableModel, environment)
			enableIsvc := fmt.Sprintf("%s-%s", enableModel, environment)

			// Check that disable model exists
			disableStatus, err := k8sClient.GetInferenceService(ctx, disableIsvc, environment)
			if err != nil {
				return fmt.Errorf("model %s is not deployed in %s", disableModel, environment)
			}

			fmt.Printf("Swapping models in %s:\n", environment)
			fmt.Printf("  Disable: %s (ready: %v)\n", disableModel, disableStatus.Ready)
			fmt.Printf("  Enable:  %s\n", enableModel)
			if reason != "" {
				fmt.Printf("  Reason:  %s\n", reason)
			}

			if !force {
				fmt.Print("\nConfirm swap? [y/N]: ")
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			fmt.Println()

			// Step 1: Disable the first model
			fmt.Printf("Step 1: Disabling %s...\n", disableModel)
			if err := k8sClient.DeleteInferenceService(ctx, disableIsvc, environment); err != nil {
				return fmt.Errorf("disable %s: %w", disableModel, err)
			}

			// Get API client for recording state history
			apiClient, err := getAPIClient(cfg)
			var regClient *registry.Client
			if err == nil {
				regClient = registry.NewClient(apiClient)
			}

			// Record disable in history via Admin API
			if regClient != nil {
				if err := regClient.RecordState(ctx, disableModel, registry.RecordStateRequest{
					Environment: environment,
					Action:      "swapped_out",
					Reason:      reason,
				}); err != nil {
					fmt.Printf("  Warning: could not record history: %v\n", err)
				}
			}

			fmt.Printf("  ✓ Disabled %s\n", disableModel)

			// Wait for resources to be released using proper polling
			fmt.Print("  Waiting for resources to be released...")
			if err := k8sClient.WaitForDelete(ctx, disableIsvc, environment, 30*time.Second); err != nil {
				// Not a fatal error, just log it
				fmt.Printf(" (still terminating)")
			}
			fmt.Println(" done")

			// Step 2: Enable the second model
			fmt.Printf("Step 2: Enabling %s...\n", enableModel)

			// Build storage URI
			storageURI := fmt.Sprintf("s3://%s/models/%s/main/", s3Bucket, enableModel)

			isvcCfg := kubernetes.InferenceServiceConfig{
				Name:        enableIsvc,
				Namespace:   environment,
				ModelName:   enableModel,
				StorageURI:  storageURI,
				GPUCount:    1, // Default, could be configurable
				MemoryGB:    24,
				MinReplicas: 1,
				MaxReplicas: 1,
				Environment: environment,
				Labels: map[string]string{
					"ai-aas.io/model":       enableModel,
					"ai-aas.io/environment": environment,
				},
			}

			if err := k8sClient.CreateInferenceService(ctx, isvcCfg); err != nil {
				// Try to rollback - re-enable the disabled model
				fmt.Printf("  ERROR: %v\n", err)
				fmt.Printf("  Attempting rollback: re-enabling %s...\n", disableModel)

				rollbackCfg := isvcCfg
				rollbackCfg.Name = disableIsvc
				rollbackCfg.ModelName = disableModel
				rollbackCfg.StorageURI = fmt.Sprintf("s3://%s/models/%s/main/", s3Bucket, disableModel)
				rollbackCfg.Labels["ai-aas.io/model"] = disableModel

				if rollbackErr := k8sClient.CreateInferenceService(ctx, rollbackCfg); rollbackErr != nil {
					return fmt.Errorf("enable %s failed and rollback also failed: %v (rollback error: %v)",
						enableModel, err, rollbackErr)
				}
				fmt.Printf("  ✓ Rollback successful, %s re-enabled\n", disableModel)
				return fmt.Errorf("enable %s: %w", enableModel, err)
			}

			// Record enable in history via Admin API
			if regClient != nil {
				if err := regClient.RecordState(ctx, enableModel, registry.RecordStateRequest{
					Environment: environment,
					Action:      "swapped_in",
					Reason:      reason,
				}); err != nil {
					fmt.Printf("  Warning: could not record history: %v\n", err)
				}
			}

			fmt.Printf("  ✓ Created InferenceService for %s\n", enableModel)

			// Wait for ready
			fmt.Print("  Waiting for ready...")
			waitOpts := kubernetes.WaitOptions{
				Timeout:      10 * time.Minute,
				PollInterval: 5 * time.Second,
			}
			if err := k8sClient.WaitForReady(ctx, enableIsvc, environment, waitOpts); err != nil {
				fmt.Printf(" TIMEOUT\n")
				fmt.Printf("  Warning: %s did not become ready in time\n", enableModel)
			} else {
				fmt.Printf(" READY\n")
			}

			fmt.Printf("\n✅ Swap complete: %s -> %s\n", disableModel, enableModel)

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for swap (for audit)")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// NewHistoryCommand creates the model history command
func NewHistoryCommand() *cobra.Command {
	var (
		environment string
		limit       int
		formatFlag  string
	)

	cmd := &cobra.Command{
		Use:   "history <model-name>",
		Short: "Show enable/disable history",
		Long: `Display the audit log of enable/disable events for a model.

Shows when the model was enabled, disabled, or swapped, who performed
the action, and any reason provided.

Examples:
  # Show history for a model
  ai-aas-cli model history llama-3-8b

  # Filter by environment
  ai-aas-cli model history llama-3-8b -e production

  # Limit entries
  ai-aas-cli model history llama-3-8b --limit 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiClient, err := getAPIClient(cfg)
			if err != nil {
				return err
			}
			regClient := registry.NewClient(apiClient)

			// Get history via Admin API
			entries, err := regClient.GetHistory(ctx, modelName, environment, limit)
			if err != nil {
				return fmt.Errorf("get history: %w", err)
			}

			// Output
			if formatFlag == "json" {
				return output.PrintJSON(entries)
			}

			fmt.Printf("History for %s", modelName)
			if environment != "" {
				fmt.Printf(" (environment: %s)", environment)
			}
			fmt.Println(":")
			fmt.Println()

			if len(entries) == 0 {
				fmt.Println("No history found.")
				return nil
			}

			fmt.Printf("%-20s %-12s %-20s %-30s\n", "TIMESTAMP", "ACTION", "PERFORMED BY", "REASON")
			fmt.Println("────────────────────────────────────────────────────────────────────────────────")

			for _, e := range entries {
				reason := e.Reason
				if len(reason) > 30 {
					reason = reason[:27] + "..."
				}
				if reason == "" {
					reason = "-"
				}
				performedBy := e.PerformedBy
				if performedBy == "" {
					performedBy = "system"
				}
				if len(performedBy) > 20 {
					performedBy = performedBy[:17] + "..."
				}

				fmt.Printf("%-20s %-12s %-20s %-30s\n",
					e.ExecutedAt.Format("2006-01-02 15:04"),
					e.Action,
					performedBy,
					reason)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment")
	cmd.Flags().IntVar(&limit, "limit", 20, "max entries to show")
	cmd.Flags().StringVar(&formatFlag, "format", "table", "Output format: table, json")

	return cmd
}
