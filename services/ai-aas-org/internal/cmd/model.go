package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// OpenAIModelsResponse represents the response from /v1/models
type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// OpenAIModel represents a model in the OpenAI API response
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func runModelList(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	// Get inference endpoint
	inferenceEndpoint := config.GetInferenceEndpoint()
	if inferenceEndpoint == "" {
		return errors.NewOperationError(
			"inference endpoint not configured",
			"Run 'ai-aas-org configure --inference-endpoint <url>' to set the inference API endpoint.",
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	modelsURL := inferenceEndpoint + "/v1/models"

	// Create HTTP client with TLS skip option for development
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Allow insecure TLS for development environments
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	transport.TLSClientConfig.InsecureSkipVerify = true
	httpClient.Transport = transport

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return errors.NewOperationError("failed to create request", err.Error())
	}

	// Set auth headers
	apiKey := config.GetAPIKey()
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-API-Key", apiKey)
	}

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.NewOperationError("failed to query models", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return errors.NewOperationError(
			fmt.Sprintf("authentication failed (status %d)", resp.StatusCode),
			"Check your API key is valid and has permission to list models.",
		)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return errors.NewOperationError(
			fmt.Sprintf("API error (status %d)", resp.StatusCode),
			string(body),
		)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.NewOperationError("failed to read response", err.Error())
	}

	var modelsResp OpenAIModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return errors.NewOperationError("failed to parse response", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(modelsResp.Data)
	}

	if len(modelsResp.Data) == 0 {
		output.InfoMsg("No models available.")
		fmt.Println()
		fmt.Println("This may indicate no models are deployed or accessible to your organization.")
		return nil
	}

	headers := []string{"Model ID", "Owner", "Status"}
	var rows [][]string
	for _, m := range modelsResp.Data {
		owner := m.OwnedBy
		if owner == "" {
			owner = "platform"
		}
		rows = append(rows, []string{
			m.ID,
			owner,
			output.StatusBadge("available"),
		})
	}

	output.PrintTable(headers, rows)
	fmt.Printf("\nTotal: %d models\n", len(modelsResp.Data))
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
