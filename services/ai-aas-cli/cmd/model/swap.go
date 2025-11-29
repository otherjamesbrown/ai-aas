// Package model provides CLI commands for model management.
package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
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

			cfg, err := config.Load()
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

			// Record disable in history
			if err := recordStateHistory(ctx, cfg, disableModel, environment, "swapped_out", reason); err != nil {
				fmt.Printf("  Warning: could not record history: %v\n", err)
			}

			fmt.Printf("  ✓ Disabled %s\n", disableModel)

			// Wait briefly for resources to be released
			fmt.Print("  Waiting for resources to be released...")
			time.Sleep(5 * time.Second)
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

			// Record enable in history
			if err := recordStateHistory(ctx, cfg, enableModel, environment, "swapped_in", reason); err != nil {
				fmt.Printf("  Warning: could not record history: %v\n", err)
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

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.DatabaseURL == "" {
				return fmt.Errorf("DATABASE_URL not configured")
			}

			db, err := sql.Open("postgres", cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("connect to database: %w", err)
			}
			defer db.Close()

			// Build query
			query := `
				SELECT h.action, h.performed_by, h.reason, h.scheduled_at, h.executed_at,
				       d.environment
				FROM model_state_history h
				JOIN model_deployments d ON h.deployment_id = d.id
				JOIN model_registry r ON d.model_id = r.id
				WHERE r.name = $1
			`
			queryArgs := []interface{}{modelName}
			argNum := 2

			if environment != "" {
				query += fmt.Sprintf(" AND d.environment = $%d", argNum)
				queryArgs = append(queryArgs, environment)
				argNum++
			}

			query += " ORDER BY h.executed_at DESC"
			query += fmt.Sprintf(" LIMIT $%d", argNum)
			queryArgs = append(queryArgs, limit)

			rows, err := db.QueryContext(ctx, query, queryArgs...)
			if err != nil {
				return fmt.Errorf("query history: %w", err)
			}
			defer rows.Close()

			type historyEntry struct {
				Action      string     `json:"action"`
				PerformedBy string     `json:"performed_by"`
				Reason      string     `json:"reason"`
				ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
				ExecutedAt  time.Time  `json:"executed_at"`
				Environment string     `json:"environment"`
			}

			var entries []historyEntry
			for rows.Next() {
				var e historyEntry
				var performedBy, reason sql.NullString
				var scheduledAt sql.NullTime

				if err := rows.Scan(&e.Action, &performedBy, &reason, &scheduledAt, &e.ExecutedAt, &e.Environment); err != nil {
					return fmt.Errorf("scan row: %w", err)
				}

				if performedBy.Valid {
					e.PerformedBy = performedBy.String
				} else {
					e.PerformedBy = "system"
				}
				if reason.Valid {
					e.Reason = reason.String
				}
				if scheduledAt.Valid {
					e.ScheduledAt = &scheduledAt.Time
				}

				entries = append(entries, e)
			}

			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate rows: %w", err)
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

// recordStateHistory records an action in the model_state_history table
func recordStateHistory(ctx context.Context, cfg *config.Config, modelName, environment, action, reason string) error {
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL not configured")
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

	// Get deployment ID
	var deploymentID sql.NullString
	err = db.QueryRowContext(ctx, `
		SELECT d.id FROM model_deployments d
		JOIN model_registry r ON d.model_id = r.id
		WHERE r.name = $1 AND d.environment = $2
	`, modelName, environment).Scan(&deploymentID)

	if err == sql.ErrNoRows || !deploymentID.Valid {
		// Create a deployment record if it doesn't exist
		var modelID string
		err = db.QueryRowContext(ctx, `SELECT id FROM model_registry WHERE name = $1`, modelName).Scan(&modelID)
		if err != nil {
			return fmt.Errorf("model not found: %s", modelName)
		}

		// Insert deployment record
		err = db.QueryRowContext(ctx, `
			INSERT INTO model_deployments (model_id, environment, namespace, status)
			VALUES ($1, $2, $2, 'unknown')
			RETURNING id
		`, modelID, environment).Scan(&deploymentID)
		if err != nil {
			return fmt.Errorf("create deployment record: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("query deployment: %w", err)
	}

	// Insert history entry
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO model_state_history (deployment_id, action, performed_by, reason)
		VALUES ($1, $2, $3, $4)
	`, deploymentID, action, "cli-user", reasonPtr)
	if err != nil {
		return fmt.Errorf("insert history: %w", err)
	}

	return nil
}
