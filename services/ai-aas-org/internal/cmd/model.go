package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
)

// modelCmd represents the model command
var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "View available AI models",
	Long: `View AI models available to your organization.

Models are the AI services your users can access through their API keys.
The platform administrator configures which models are available to your org.

Examples:
  ai-aas-org model list
  ai-aas-org model show gpt-4`,
}

func init() {
	rootCmd.AddCommand(modelCmd)
}

// --- model list ---

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available models",
	Long: `List all AI models available to your organization.

Shows model name, provider, status, and context window size.

Examples:
  ai-aas-org model list
  ai-aas-org model list --json`,
	RunE: runModelList,
}

func init() {
	modelCmd.AddCommand(modelListCmd)
}

func runModelList(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	result, err := client.ListModels(ctx, config.GetOrgID())
	if err != nil {
		return errors.NewOperationError("failed to list models", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(result.Models)
	}

	if len(result.Models) == 0 {
		output.InfoMsg("No models available to your organization.")
		fmt.Println()
		fmt.Println("Contact your platform administrator to enable models for your organization.")
		return nil
	}

	headers := []string{"NAME", "PROVIDER", "STATUS", "CONTEXT SIZE"}
	var rows [][]string
	for _, m := range result.Models {
		contextSize := fmt.Sprintf("%dk", m.ContextSize/1000)
		rows = append(rows, []string{
			m.Name,
			m.Provider,
			output.StatusBadge(m.Status),
			contextSize,
		})
	}

	output.PrintTable(headers, rows)
	fmt.Printf("\nTotal: %d models\n", len(result.Models))
	return nil
}

// --- model show ---

var modelShowCmd = &cobra.Command{
	Use:   "show <model-name>",
	Short: "Show details for a model",
	Long: `Show detailed information about a specific model.

Examples:
  ai-aas-org model show gpt-4
  ai-aas-org model show llama-2-70b`,
	Args: cobra.ExactArgs(1),
	RunE: runModelShow,
}

func init() {
	modelCmd.AddCommand(modelShowCmd)
}

func runModelShow(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	modelID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	model, err := client.GetModel(ctx, config.GetOrgID(), modelID)
	if err != nil {
		return errors.NewOperationError("failed to get model", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(model)
	}

	output.Header("Model Details")
	output.KeyValue("Name", model.Name)
	output.KeyValue("ID", model.ID)
	output.KeyValue("Provider", model.Provider)
	output.KeyValue("Status", output.StatusBadge(model.Status))
	output.KeyValue("Context Size", fmt.Sprintf("%d tokens", model.ContextSize))
	if model.Description != "" {
		fmt.Println()
		output.KeyValue("Description", model.Description)
	}

	return nil
}
