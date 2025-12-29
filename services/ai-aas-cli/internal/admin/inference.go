// Package commands provides inference commands for interacting with the inference API.
//
// Purpose:
//
//	Inference commands: get-models, send-request for testing and interacting
//	with the AI inference API. Supports structured output and timing metrics.
package admin

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/inference"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// InferenceCommand creates the inference command group.
func InferenceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inference",
		Short: "Interact with the inference API",
		Long:  "Interact with the inference API: list models, send requests",
	}

	cmd.AddCommand(inferenceGetModelsCommand())
	cmd.AddCommand(inferenceSendRequestCommand())

	return cmd
}

func inferenceGetModelsCommand() *cobra.Command {
	var flagFormat string
	var flagInferenceEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "get-models",
		Short: "List available models",
		Long:  "List all available models from the inference API",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInferenceGetModels(cmd, flagFormat, flagInferenceEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagInferenceEndpoint, "inference-endpoint", "", "Inference API endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runInferenceGetModels(cmd *cobra.Command, flagFormat, flagInferenceEndpoint, flagAPIKey string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	// Apply flag overrides
	if flagInferenceEndpoint != "" {
		cfg.InferenceEndpoint = flagInferenceEndpoint
	}
	if flagAPIKey != "" {
		cfg.APIKey = flagAPIKey
	}
	if flagFormat != "" {
		cfg.OutputFormat = flagFormat
	}

	// Validate configuration
	if cfg.InferenceEndpoint == "" {
		return errors.NewValidationError(
			"inference endpoint is required",
			"Set via --inference-endpoint flag or ADMIN_CLI_INFERENCE_ENDPOINT environment variable",
		)
	}

	if cfg.APIKey == "" {
		return errors.NewValidationError(
			"API key is required",
			"Set via --api-key flag or ADMIN_CLI_API_KEY environment variable",
		)
	}

	// Create client and list models
	client := inference.NewClient(cfg.InferenceEndpoint, cfg.APIKey)
	models, err := client.ListModels(cmd.Context())
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to list models: %v", err),
			"Verify your API key is valid and the inference endpoint is reachable.",
		)
	}

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(models)
	}

	// Table output
	if len(models.Data) == 0 {
		fmt.Println("No models available.")
		return nil
	}

	headers := []string{"Model ID", "Object", "Owned By"}
	var rows [][]string
	for _, model := range models.Data {
		rows = append(rows, []string{
			model.ID,
			model.Object,
			model.OwnedBy,
		})
	}
	return output.PrintTable(headers, rows)
}

func inferenceSendRequestCommand() *cobra.Command {
	var flagModel string
	var flagMaxTokens int
	var flagTemperature float64
	var flagFormat string
	var flagInferenceEndpoint string
	var flagAPIKey string
	var flagSystemPrompt string

	cmd := &cobra.Command{
		Use:   "send-request [question]",
		Short: "Send a request to the inference API",
		Long: `Send a chat completion request to the inference API.

The question can be provided as a positional argument or will be prompted for.
Shows response time, token usage, and the model's response.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var question string
			if len(args) > 0 {
				question = args[0]
			}
			return runInferenceSendRequest(cmd, question, flagModel, flagMaxTokens, flagTemperature,
				flagFormat, flagInferenceEndpoint, flagAPIKey, flagSystemPrompt)
		},
	}

	cmd.Flags().StringVar(&flagModel, "model", "", "Model to use (defaults to first available)")
	cmd.Flags().IntVar(&flagMaxTokens, "max-tokens", 256, "Maximum tokens in response")
	cmd.Flags().Float64Var(&flagTemperature, "temperature", 0.7, "Temperature for sampling (0.0-1.0)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagInferenceEndpoint, "inference-endpoint", "", "Inference API endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")
	cmd.Flags().StringVar(&flagSystemPrompt, "system", "You are a helpful assistant.", "System prompt for the conversation")

	return cmd
}

func runInferenceSendRequest(cmd *cobra.Command, question, flagModel string, flagMaxTokens int, flagTemperature float64,
	flagFormat, flagInferenceEndpoint, flagAPIKey, flagSystemPrompt string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	// Apply flag overrides
	if flagInferenceEndpoint != "" {
		cfg.InferenceEndpoint = flagInferenceEndpoint
	}
	if flagAPIKey != "" {
		cfg.APIKey = flagAPIKey
	}
	if flagFormat != "" {
		cfg.OutputFormat = flagFormat
	}

	// Validate configuration
	if cfg.InferenceEndpoint == "" {
		return errors.NewValidationError(
			"inference endpoint is required",
			"Set via --inference-endpoint flag or ADMIN_CLI_INFERENCE_ENDPOINT environment variable",
		)
	}

	if cfg.APIKey == "" {
		return errors.NewValidationError(
			"API key is required",
			"Set via --api-key flag or ADMIN_CLI_API_KEY environment variable",
		)
	}

	// Validate question
	if question == "" {
		return errors.NewValidationError(
			"question is required",
			"Provide a question as a positional argument: admin-cli inference send-request \"your question here\"",
		)
	}

	// Create client
	client := inference.NewClient(cfg.InferenceEndpoint, cfg.APIKey)

	// If no model specified, get the first available model
	model := flagModel
	if model == "" {
		models, err := client.ListModels(cmd.Context())
		if err != nil {
			return errors.NewOperationError(
				fmt.Sprintf("failed to list models: %v", err),
				"Verify your API key is valid and the inference endpoint is reachable.",
			)
		}
		if len(models.Data) == 0 {
			return errors.NewOperationError(
				"no models available",
				"Ensure at least one model is deployed and available.",
			)
		}
		model = models.Data[0].ID
	}

	// Build the request
	req := inference.ChatCompletionRequest{
		Model: model,
		Messages: []inference.ChatMessage{
			{Role: "system", Content: flagSystemPrompt},
			{Role: "user", Content: question},
		},
		MaxTokens:   flagMaxTokens,
		Temperature: flagTemperature,
	}

	// Send request and measure time
	startTime := time.Now()
	resp, err := client.ChatCompletion(cmd.Context(), req)
	duration := time.Since(startTime)

	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to send request: %v", err),
			"Verify your API key is valid and the model is available.",
		)
	}

	// Format output
	if cfg.OutputFormat == "json" {
		// Include timing info in JSON output
		result := map[string]interface{}{
			"response":      resp,
			"duration_secs": duration.Seconds(),
		}
		return output.PrintJSON(result)
	}

	// Table/text output with stats
	fmt.Printf("Model: %s\n", resp.Model)
	fmt.Printf("Response Time: %.2f seconds\n", duration.Seconds())
	fmt.Printf("Tokens Used:\n")
	fmt.Printf("  - Prompt: %d\n", resp.Usage.PromptTokens)
	fmt.Printf("  - Completion: %d\n", resp.Usage.CompletionTokens)
	fmt.Printf("  - Total: %d\n", resp.Usage.TotalTokens)
	fmt.Println()
	fmt.Println("Response:")
	fmt.Println("─────────────────────────────────────────────────────────────")
	if len(resp.Choices) > 0 {
		fmt.Println(resp.Choices[0].Message.Content)
	} else {
		fmt.Println("(no response content)")
	}
	fmt.Println("─────────────────────────────────────────────────────────────")

	return nil
}
