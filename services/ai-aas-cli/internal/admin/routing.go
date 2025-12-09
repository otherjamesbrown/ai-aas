// Package commands provides routing policy management commands.
//
// Purpose:
//
//	Routing policy lifecycle commands: create, list, delete
//	via the Admin API Service REST endpoints.
//
// Architecture:
//
//	The admin-cli acts as a client to the admin-api-service, which provides
//	a centralized API for managing routing policies. This ensures all business
//	logic, validation, and auditing happens in one place.
//
package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

const (
	globalOrgID = "*"
)

// Policy represents a routing policy from the Admin API Service
type Policy struct {
	PolicyID          string          `json:"policy_id"`
	OrganizationID    string          `json:"organization_id"`
	Model             string          `json:"model"`
	Backends          []BackendWeight `json:"backends"`
	FailoverThreshold int             `json:"failover_threshold"`
	Enabled           bool            `json:"enabled"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	Version           int64           `json:"version"`
}

// BackendWeight defines a backend with its routing weight
type BackendWeight struct {
	BackendID string `json:"backend_id"`
	Weight    int    `json:"weight"` // Percentage (0-100)
}

// PolicyCreateRequest is the request body for creating a policy
type PolicyCreateRequest struct {
	OrganizationID    string          `json:"organization_id,omitempty"`
	Model             string          `json:"model"`
	Backends          []BackendWeight `json:"backends"`
	FailoverThreshold int             `json:"failover_threshold,omitempty"`
}

// PolicyListResponse is the response from listing policies
type PolicyListResponse struct {
	Policies   []Policy `json:"policies"`
	TotalCount int      `json:"total_count"`
}

// APIError represents an error from the Admin API Service
type APIError struct {
	Error struct {
		Type    string `json:"type"`
		Title   string `json:"title"`
		Detail  string `json:"detail"`
		TraceID string `json:"trace_id"`
	} `json:"error"`
}

// RoutingCommand creates the routing command group.
func RoutingCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routing",
		Short: "Manage routing policies for model inference",
		Long:  "Manage routing policies: create, list, delete policies for routing requests to model backends",
	}

	cmd.AddCommand(routingPolicyCommand())

	return cmd
}

func routingPolicyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage routing policies",
		Long:  "Create, list, and delete routing policies that control how requests are routed to model backends",
	}

	cmd.AddCommand(routingPolicyCreateCommand())
	cmd.AddCommand(routingPolicyListCommand())
	cmd.AddCommand(routingPolicyDeleteCommand())

	return cmd
}

func routingPolicyCreateCommand() *cobra.Command {
	var flagOrgID string
	var flagModel string
	var flagBackends string
	var flagGlobal bool
	var flagFormat string
	var flagQuiet bool
	var flagDryRun bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create or update a routing policy",
		Long: `Create or update a routing policy that defines how requests are routed to backends.

By default, creates an organization-specific policy. Use --global to create a policy
that applies to all organizations (organization_id: "*").

This command calls the Admin API Service at /v1/routing/policies.`,
		Example: `  # Create a global policy for qwen2-7b-instruct
  admin-cli routing policy create \
    --global \
    --model qwen2-7b-instruct \
    --backends qwen2-7b-backend:100

  # Create org-specific policy with multiple backends
  admin-cli routing policy create \
    --org-id aa6f9015-132a-4694-8b10-7d4d4550faed \
    --model gpt-4 \
    --backends "backend-1:70,backend-2:30"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoutingPolicyCreate(flagOrgID, flagModel, flagBackends, flagGlobal, flagFormat, flagQuiet, flagDryRun)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID (optional if --global is used)")
	cmd.Flags().StringVar(&flagModel, "model", "", "Model name (required)")
	cmd.Flags().StringVar(&flagBackends, "backends", "", "Comma-separated list of backend_id:weight pairs (e.g., 'backend-1:70,backend-2:30')")
	cmd.Flags().BoolVar(&flagGlobal, "global", false, "Create a global policy that applies to all organizations")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Simulate creation without applying changes")

	cmd.MarkFlagRequired("model")
	cmd.MarkFlagRequired("backends")

	return cmd
}

func runRoutingPolicyCreate(orgID, model, backends string, global bool, flagFormat string, quiet, dryRun bool) error {
	// Determine organization ID
	if global {
		orgID = globalOrgID
	} else if orgID == "" {
		return errors.NewValidationError(
			"organization ID required",
			"Provide --org-id or use --global flag",
		)
	}

	// Parse backends
	backendWeights, err := parseBackends(backends)
	if err != nil {
		return err
	}

	// Validate weights sum to 100
	totalWeight := 0
	for _, bw := range backendWeights {
		totalWeight += bw.Weight
	}
	if totalWeight != 100 {
		return errors.NewValidationError(
			fmt.Sprintf("backend weights must sum to 100, got %d", totalWeight),
			"Adjust your backend weights",
		)
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "Creating routing policy via Admin API Service...\n")
		fmt.Fprintf(os.Stderr, "  Organization: %s\n", orgID)
		fmt.Fprintf(os.Stderr, "  Model: %s\n", model)
		fmt.Fprintf(os.Stderr, "  Backends: %d configured\n", len(backendWeights))
		for _, bw := range backendWeights {
			fmt.Fprintf(os.Stderr, "    - %s: %d%%\n", bw.BackendID, bw.Weight)
		}
		if dryRun {
			fmt.Fprintf(os.Stderr, "  Mode: DRY RUN (no changes will be made)\n")
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	if dryRun {
		if !quiet {
			fmt.Fprintf(os.Stderr, "✓ Dry run successful - no changes made\n")
		}
		return nil
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	if cfg.APIEndpoint == "" {
		return errors.NewOperationError(
			"Admin API endpoint not configured",
			"Set AI_AAS_API_ENDPOINT environment variable or configure api_endpoint in ~/.ai-aas-cli.yaml",
		)
	}

	if cfg.APIKey == "" {
		return errors.NewOperationError(
			"API key not configured",
			"Set AI_AAS_API_KEY environment variable or configure api_key in ~/.ai-aas-cli.yaml",
		)
	}

	// Create request body
	reqBody := PolicyCreateRequest{
		OrganizationID:    orgID,
		Model:             model,
		Backends:          backendWeights,
		FailoverThreshold: 3, // Default failover threshold
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to serialize request: %v", err),
			"Internal error",
		)
	}

	// Call Admin API Service
	url := fmt.Sprintf("%s/v1/routing/policies", cfg.APIEndpoint)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqJSON))
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to create request: %v", err),
			"Internal error",
		)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to call Admin API Service: %v", err),
			"Check your AI_AAS_API_ENDPOINT configuration and network connectivity.",
		)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Detail != "" {
			return errors.NewOperationError(
				fmt.Sprintf("%s: %s", apiErr.Error.Title, apiErr.Error.Detail),
				"Check your request parameters.",
			)
		}
		return errors.NewOperationError(
			fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body)),
			"Check your request parameters and API key.",
		)
	}

	var policy Policy
	if err := json.Unmarshal(body, &policy); err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to parse response: %v", err),
			"Internal error",
		)
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "✓ Routing policy created successfully\n")
		fmt.Fprintf(os.Stderr, "  Policy ID: %s\n", policy.PolicyID)
	}

	// Output structured data
	if flagFormat == "json" {
		return output.PrintJSON(policy)
	}

	return nil
}

func routingPolicyListCommand() *cobra.Command {
	var flagFormat string
	var flagModel string
	var flagOrgID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all routing policies",
		Long:  "List all routing policies via the Admin API Service",
		Example: `  # List all policies
  admin-cli routing policy list

  # List policies in JSON format
  admin-cli routing policy list --format json

  # Filter by model
  admin-cli routing policy list --model gpt-4`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoutingPolicyList(flagFormat, flagModel, flagOrgID)
		},
	}

	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagModel, "model", "", "Filter by model name")
	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Filter by organization ID")

	return cmd
}

func runRoutingPolicyList(flagFormat, filterModel, filterOrgID string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	if cfg.APIEndpoint == "" {
		return errors.NewOperationError(
			"Admin API endpoint not configured",
			"Set AI_AAS_API_ENDPOINT environment variable or configure api_endpoint in ~/.ai-aas-cli.yaml",
		)
	}

	if cfg.APIKey == "" {
		return errors.NewOperationError(
			"API key not configured",
			"Set AI_AAS_API_KEY environment variable or configure api_key in ~/.ai-aas-cli.yaml",
		)
	}

	// Build URL with query parameters
	url := fmt.Sprintf("%s/v1/routing/policies", cfg.APIEndpoint)
	params := []string{}
	if filterModel != "" {
		params = append(params, fmt.Sprintf("model=%s", filterModel))
	}
	if filterOrgID != "" {
		params = append(params, fmt.Sprintf("organization_id=%s", filterOrgID))
	}
	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to create request: %v", err),
			"Internal error",
		)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to call Admin API Service: %v", err),
			"Check your AI_AAS_API_ENDPOINT configuration and network connectivity.",
		)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Detail != "" {
			return errors.NewOperationError(
				fmt.Sprintf("%s: %s", apiErr.Error.Title, apiErr.Error.Detail),
				"Check your API key and permissions.",
			)
		}
		return errors.NewOperationError(
			fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body)),
			"Check your API key.",
		)
	}

	var listResp PolicyListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to parse response: %v", err),
			"Internal error",
		)
	}

	if flagFormat == "json" {
		return output.PrintJSON(listResp.Policies)
	}

	// Table output
	if len(listResp.Policies) == 0 {
		fmt.Println("No routing policies found.")
		return nil
	}

	fmt.Printf("Found %d routing policies:\n\n", len(listResp.Policies))
	fmt.Printf("%-40s %-36s %-30s %-8s %s\n", "MODEL", "ORGANIZATION ID", "BACKENDS", "ENABLED", "UPDATED")
	fmt.Printf("%-40s %-36s %-30s %-8s %s\n", strings.Repeat("-", 40), strings.Repeat("-", 36), strings.Repeat("-", 30), strings.Repeat("-", 8), strings.Repeat("-", 20))

	for _, policy := range listResp.Policies {
		backendSummary := ""
		for i, bw := range policy.Backends {
			if i > 0 {
				backendSummary += ", "
			}
			backendSummary += fmt.Sprintf("%s:%d%%", bw.BackendID, bw.Weight)
			if len(backendSummary) > 27 {
				backendSummary = backendSummary[:24] + "..."
				break
			}
		}
		enabledStr := "No"
		if policy.Enabled {
			enabledStr = "Yes"
		}
		fmt.Printf("%-40s %-36s %-30s %-8s %s\n",
			policy.Model,
			policy.OrganizationID,
			backendSummary,
			enabledStr,
			policy.UpdatedAt.Format("2006-01-02 15:04:05"),
		)
	}

	return nil
}

func routingPolicyDeleteCommand() *cobra.Command {
	var flagPolicyID string
	var flagQuiet bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a routing policy",
		Long:  "Delete a routing policy via the Admin API Service",
		Example: `  # Delete a policy by ID
  admin-cli routing policy delete --policy-id <policy-uuid>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoutingPolicyDelete(flagPolicyID, flagQuiet)
		},
	}

	cmd.Flags().StringVar(&flagPolicyID, "policy-id", "", "Policy ID to delete (required)")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")

	cmd.MarkFlagRequired("policy-id")

	return cmd
}

func runRoutingPolicyDelete(policyID string, quiet bool) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	if cfg.APIEndpoint == "" {
		return errors.NewOperationError(
			"Admin API endpoint not configured",
			"Set AI_AAS_API_ENDPOINT environment variable or configure api_endpoint in ~/.ai-aas-cli.yaml",
		)
	}

	if cfg.APIKey == "" {
		return errors.NewOperationError(
			"API key not configured",
			"Set AI_AAS_API_KEY environment variable or configure api_key in ~/.ai-aas-cli.yaml",
		)
	}

	// Call Admin API Service
	url := fmt.Sprintf("%s/v1/routing/policies/%s", cfg.APIEndpoint, policyID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to create request: %v", err),
			"Internal error",
		)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to call Admin API Service: %v", err),
			"Check your AI_AAS_API_ENDPOINT configuration and network connectivity.",
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errors.NewValidationError(
			fmt.Sprintf("policy not found: %s", policyID),
			"Check the policy ID. Use 'admin-cli routing policy list' to see available policies.",
		)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Detail != "" {
			return errors.NewOperationError(
				fmt.Sprintf("%s: %s", apiErr.Error.Title, apiErr.Error.Detail),
				"Check your API key and permissions.",
			)
		}
		return errors.NewOperationError(
			fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body)),
			"Check your API key.",
		)
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "✓ Routing policy deleted successfully\n")
		fmt.Fprintf(os.Stderr, "  Policy ID: %s\n", policyID)
	}

	return nil
}

// parseBackends parses a backend string like "backend-1:70,backend-2:30" into BackendWeight structs
func parseBackends(backends string) ([]BackendWeight, error) {
	parts := strings.Split(backends, ",")
	weights := make([]BackendWeight, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		subparts := strings.Split(part, ":")
		if len(subparts) != 2 {
			return nil, errors.NewValidationError(
				fmt.Sprintf("invalid backend format: %s", part),
				"Use format: backend_id:weight (e.g., 'backend-1:70,backend-2:30')",
			)
		}

		backendID := strings.TrimSpace(subparts[0])
		var weight int
		_, err := fmt.Sscanf(strings.TrimSpace(subparts[1]), "%d", &weight)
		if err != nil || weight < 0 || weight > 100 {
			return nil, errors.NewValidationError(
				fmt.Sprintf("invalid weight: %s", subparts[1]),
				"Weight must be an integer between 0 and 100",
			)
		}

		weights = append(weights, BackendWeight{
			BackendID: backendID,
			Weight:    weight,
		})
	}

	if len(weights) == 0 {
		return nil, errors.NewValidationError(
			"no backends specified",
			"Provide at least one backend in format: backend_id:weight",
		)
	}

	return weights, nil
}
