// Package model provides CLI commands for model management.
package model

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewListCommand creates the model list command
func NewListCommand() *cobra.Command {
	var (
		cached      bool
		deployed    bool
		orphaned    bool
		environment string
		format      string
		live        bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List models",
		Long: `List models available in the platform.

By default, lists models from the registry (what's been registered).
Use --live to query the inference API directly (what's actually available for inference).

Examples:
  # List models available for inference (recommended)
  ai-aas-cli model list --live

  # List all registered models
  ai-aas-cli model list

  # List only cached models
  ai-aas-cli model list --cached

  # List deployed models in production
  ai-aas-cli model list --deployed --environment production

  # Output as JSON
  ai-aas-cli model list --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --live flag is set, query the inference API directly
			if live {
				return listLiveModels(cmd, format)
			}

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

			models, err := regClient.List(ctx, registry.ListOptions{
				Cached:      cached,
				Deployed:    deployed,
				Orphaned:    orphaned,
				Environment: environment,
			})
			if err != nil {
				return fmt.Errorf("failed to list models: %w", err)
			}

			if len(models) == 0 {
				fmt.Println("No models found.")
				return nil
			}

			// Output based on format
			if format == "json" {
				return output.PrintJSON(models, true)
			}

			// Table output
			table := output.NewTableWriter()
			table.SetHeader([]string{"NAME", "HF MODEL ID", "CACHED", "DEPLOYED", "STATUS"})

			for _, m := range models {
				cached := "No"
				if m.CacheStatus == "ready" {
					cached = "Yes"
				}
				
				deployed := "No"
				if m.DeploymentStatus != "" && m.DeploymentStatus != "none" {
					deployed = "Yes"
				}

				status := "registered"
				if m.DeploymentStatus == "ready" {
					status = "ready"
				} else if m.CacheStatus == "ready" {
					status = "cached"
				}

				table.Append([]string{
					m.Name,
					truncate(m.HFModelID, 35),
					cached,
					deployed,
					status,
				})
			}

			table.Render()
			fmt.Printf("\nTotal: %d model(s)\n", len(models))

			return nil
		},
	}

	cmd.Flags().BoolVar(&live, "live", false, "query inference API for actually available models")
	cmd.Flags().BoolVar(&cached, "cached", false, "show only cached models (registry mode)")
	cmd.Flags().BoolVar(&deployed, "deployed", false, "show only deployed models (registry mode)")
	cmd.Flags().BoolVar(&orphaned, "orphaned", false, "show orphaned cache entries (registry mode)")
	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment (registry mode)")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

// truncate truncates a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// OpenAIModelsResponse represents the response from /v1/models
type OpenAIModelsResponse struct {
	Object string          `json:"object"`
	Data   []OpenAIModel   `json:"data"`
}

// OpenAIModel represents a model in the OpenAI API response
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// listLiveModels queries the inference API's /v1/models endpoint
func listLiveModels(cmd *cobra.Command, format string) error {
	// Get profile flag and load config with profile support
	profileName, _ := cmd.Flags().GetString("profile")
	cfg, _, err := config.GetEffectiveConfig(profileName)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get inference endpoint
	inferenceEndpoint := cfg.InferenceEndpoint
	if inferenceEndpoint == "" {
		return fmt.Errorf("inference endpoint not configured. Set inference_endpoint in config or profile")
	}

	modelsURL := inferenceEndpoint + "/v1/models"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Check if TLS verification should be skipped
	if cfg.TLSInsecure {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
		httpClient.Transport = transport
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Set auth headers
	apiKey := cfg.APIKey
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-API-Key", apiKey)
	}

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to query models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (status %d). Check your API key", resp.StatusCode)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var modelsResp OpenAIModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if len(modelsResp.Data) == 0 {
		fmt.Println("No models available.")
		fmt.Println()
		fmt.Println("This may indicate no models are deployed or the API router is not connected to any backends.")
		return nil
	}

	// Output based on format
	if format == "json" {
		return output.PrintJSON(modelsResp.Data, true)
	}

	// Table output
	table := output.NewTableWriter()
	table.SetHeader([]string{"MODEL ID", "OWNER", "STATUS"})

	for _, m := range modelsResp.Data {
		owner := m.OwnedBy
		if owner == "" {
			owner = "platform"
		}
		table.Append([]string{
			m.ID,
			owner,
			"available",
		})
	}

	table.Render()
	fmt.Printf("\nTotal: %d model(s) available for inference\n", len(modelsResp.Data))
	fmt.Printf("\nEndpoint: %s\n", inferenceEndpoint)

	return nil
}

