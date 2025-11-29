// Package model provides CLI commands for model management.
package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// NewAliasCommand creates the model alias command group
func NewAliasCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage model aliases",
		Long: `Manage model aliases for version abstraction.

Aliases allow you to create friendly names that point to specific models,
enabling version-agnostic routing and easy model updates.

Examples:
  # List all aliases
  ai-aas-cli model alias list

  # Create an alias
  ai-aas-cli model alias create gpt --model-name llama-3-8b --description "Default GPT model"

  # Update an alias to point to a different model
  ai-aas-cli model alias update gpt --model-name llama-3-70b

  # Delete an alias
  ai-aas-cli model alias delete gpt`,
	}

	cmd.AddCommand(aliasListCommand())
	cmd.AddCommand(aliasCreateCommand())
	cmd.AddCommand(aliasUpdateCommand())
	cmd.AddCommand(aliasDeleteCommand())
	cmd.AddCommand(aliasGetCommand())

	return cmd
}

func aliasListCommand() *cobra.Command {
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all model aliases",
		Long:  "List all model aliases with their target models.",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			query := `
				SELECT a.id, a.alias_name, a.model_id, r.name as model_name,
				       COALESCE(a.description, '') as description,
				       a.created_at, a.updated_at
				FROM model_aliases a
				JOIN model_registry r ON a.model_id = r.id
				ORDER BY a.alias_name
			`

			rows, err := db.QueryContext(ctx, query)
			if err != nil {
				return fmt.Errorf("query aliases: %w", err)
			}
			defer rows.Close()

			type aliasEntry struct {
				ID          string    `json:"id"`
				AliasName   string    `json:"alias_name"`
				ModelID     string    `json:"model_id"`
				ModelName   string    `json:"model_name"`
				Description string    `json:"description"`
				CreatedAt   time.Time `json:"created_at"`
				UpdatedAt   time.Time `json:"updated_at"`
			}

			var aliases []aliasEntry
			for rows.Next() {
				var a aliasEntry
				if err := rows.Scan(&a.ID, &a.AliasName, &a.ModelID, &a.ModelName, &a.Description, &a.CreatedAt, &a.UpdatedAt); err != nil {
					return fmt.Errorf("scan row: %w", err)
				}
				aliases = append(aliases, a)
			}

			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate rows: %w", err)
			}

			if formatFlag == "json" {
				return output.PrintJSON(aliases)
			}

			if len(aliases) == 0 {
				fmt.Println("No aliases found.")
				return nil
			}

			// Table output
			fmt.Printf("%-20s %-25s %-30s\n", "ALIAS", "TARGET MODEL", "DESCRIPTION")
			fmt.Println("────────────────────────────────────────────────────────────────────────────")
			for _, a := range aliases {
				desc := a.Description
				if len(desc) > 30 {
					desc = desc[:27] + "..."
				}
				fmt.Printf("%-20s %-25s %-30s\n", a.AliasName, a.ModelName, desc)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&formatFlag, "format", "table", "Output format: table, json")

	return cmd
}

func aliasCreateCommand() *cobra.Command {
	var (
		modelName   string
		description string
	)

	cmd := &cobra.Command{
		Use:   "create <alias-name>",
		Short: "Create a new model alias",
		Long: `Create a new alias pointing to a model.

Examples:
  ai-aas-cli model alias create gpt --model-name llama-3-8b
  ai-aas-cli model alias create gpt --model-name llama-3-8b --description "Default GPT model"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			aliasName := args[0]

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

			// Get model ID
			var modelID string
			err = db.QueryRowContext(ctx, `SELECT id FROM model_registry WHERE name = $1`, modelName).Scan(&modelID)
			if err == sql.ErrNoRows {
				return fmt.Errorf("model not found: %s", modelName)
			} else if err != nil {
				return fmt.Errorf("query model: %w", err)
			}

			// Insert alias
			var desc *string
			if description != "" {
				desc = &description
			}

			query := `
				INSERT INTO model_aliases (alias_name, model_id, description)
				VALUES ($1, $2, $3)
				RETURNING id, created_at
			`

			var id string
			var createdAt time.Time
			err = db.QueryRowContext(ctx, query, aliasName, modelID, desc).Scan(&id, &createdAt)
			if err != nil {
				if isUniqueViolation(err) {
					return fmt.Errorf("alias already exists: %s", aliasName)
				}
				return fmt.Errorf("create alias: %w", err)
			}

			fmt.Printf("Created alias '%s' -> '%s'\n", aliasName, modelName)
			if description != "" {
				fmt.Printf("  Description: %s\n", description)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&modelName, "model-name", "", "Target model name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Alias description")
	_ = cmd.MarkFlagRequired("model-name")

	return cmd
}

func aliasUpdateCommand() *cobra.Command {
	var (
		modelName   string
		description string
	)

	cmd := &cobra.Command{
		Use:   "update <alias-name>",
		Short: "Update an existing model alias",
		Long: `Update an alias to point to a different model or change its description.

Examples:
  ai-aas-cli model alias update gpt --model-name llama-3-70b
  ai-aas-cli model alias update gpt --description "Updated GPT model"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			aliasName := args[0]

			if modelName == "" && description == "" {
				return fmt.Errorf("at least one of --model-name or --description must be provided")
			}

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

			// Build update query
			updates := []string{"updated_at = NOW()"}
			queryArgs := []interface{}{aliasName}
			argNum := 2

			if modelName != "" {
				// Get new model ID
				var modelID string
				err = db.QueryRowContext(ctx, `SELECT id FROM model_registry WHERE name = $1`, modelName).Scan(&modelID)
				if err == sql.ErrNoRows {
					return fmt.Errorf("model not found: %s", modelName)
				} else if err != nil {
					return fmt.Errorf("query model: %w", err)
				}

				updates = append(updates, fmt.Sprintf("model_id = $%d", argNum))
				queryArgs = append(queryArgs, modelID)
				argNum++
			}

			if description != "" {
				updates = append(updates, fmt.Sprintf("description = $%d", argNum))
				queryArgs = append(queryArgs, description)
				argNum++
			}

			query := fmt.Sprintf(`
				UPDATE model_aliases
				SET %s
				WHERE alias_name = $1
			`, joinStrings(updates, ", "))

			result, err := db.ExecContext(ctx, query, queryArgs...)
			if err != nil {
				return fmt.Errorf("update alias: %w", err)
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				return fmt.Errorf("alias not found: %s", aliasName)
			}

			fmt.Printf("Updated alias '%s'\n", aliasName)
			if modelName != "" {
				fmt.Printf("  New target: %s\n", modelName)
			}
			if description != "" {
				fmt.Printf("  Description: %s\n", description)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&modelName, "model-name", "", "New target model name")
	cmd.Flags().StringVar(&description, "description", "", "New alias description")

	return cmd
}

func aliasDeleteCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <alias-name>",
		Short: "Delete a model alias",
		Long: `Delete an alias.

Examples:
  ai-aas-cli model alias delete gpt
  ai-aas-cli model alias delete gpt --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			aliasName := args[0]

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

			// Check if alias exists
			var targetModel string
			err = db.QueryRowContext(ctx, `
				SELECT r.name
				FROM model_aliases a
				JOIN model_registry r ON a.model_id = r.id
				WHERE a.alias_name = $1
			`, aliasName).Scan(&targetModel)
			if err == sql.ErrNoRows {
				return fmt.Errorf("alias not found: %s", aliasName)
			} else if err != nil {
				return fmt.Errorf("query alias: %w", err)
			}

			if !force {
				fmt.Printf("Delete alias '%s' (currently pointing to '%s')? [y/N]: ", aliasName, targetModel)
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			// Delete alias
			result, err := db.ExecContext(ctx, `DELETE FROM model_aliases WHERE alias_name = $1`, aliasName)
			if err != nil {
				return fmt.Errorf("delete alias: %w", err)
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				return fmt.Errorf("alias not found: %s", aliasName)
			}

			fmt.Printf("Deleted alias '%s'\n", aliasName)

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")

	return cmd
}

func aliasGetCommand() *cobra.Command {
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "get <alias-name>",
		Short: "Get details of a specific alias",
		Long:  "Get details of a specific model alias including the target model.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			aliasName := args[0]

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

			query := `
				SELECT a.id, a.alias_name, a.model_id, r.name as model_name,
				       COALESCE(a.description, '') as description,
				       a.created_at, a.updated_at
				FROM model_aliases a
				JOIN model_registry r ON a.model_id = r.id
				WHERE a.alias_name = $1
			`

			var alias struct {
				ID          string    `json:"id"`
				AliasName   string    `json:"alias_name"`
				ModelID     string    `json:"model_id"`
				ModelName   string    `json:"model_name"`
				Description string    `json:"description"`
				CreatedAt   time.Time `json:"created_at"`
				UpdatedAt   time.Time `json:"updated_at"`
			}

			err = db.QueryRowContext(ctx, query, aliasName).Scan(
				&alias.ID, &alias.AliasName, &alias.ModelID, &alias.ModelName,
				&alias.Description, &alias.CreatedAt, &alias.UpdatedAt,
			)
			if err == sql.ErrNoRows {
				return fmt.Errorf("alias not found: %s", aliasName)
			} else if err != nil {
				return fmt.Errorf("query alias: %w", err)
			}

			if formatFlag == "json" {
				return output.PrintJSON(alias)
			}

			fmt.Printf("Alias: %s\n", alias.AliasName)
			fmt.Printf("  Target Model: %s\n", alias.ModelName)
			if alias.Description != "" {
				fmt.Printf("  Description: %s\n", alias.Description)
			}
			fmt.Printf("  Created: %s\n", alias.CreatedAt.Format(time.RFC3339))
			fmt.Printf("  Updated: %s\n", alias.UpdatedAt.Format(time.RFC3339))

			return nil
		},
	}

	cmd.Flags().StringVar(&formatFlag, "format", "table", "Output format: table, json")

	return cmd
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation
func isUniqueViolation(err error) bool {
	return err != nil && (contains(err.Error(), "23505") || contains(err.Error(), "unique constraint"))
}

// contains checks if str contains substr
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(str) > 0 && containsHelper(str, substr))
}

func containsHelper(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// joinStrings joins strings with a separator (avoiding import of strings package)
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
