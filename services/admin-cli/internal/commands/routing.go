// Package commands provides routing policy management commands.
//
// Purpose:
//
//	Routing policy lifecycle commands: create, list, delete
//	with etcd backend integration.
//
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/output"
)

const (
	etcdKeyPrefix   = "/ai-aas/routing/policies/"
	etcdGlobalOrgID = "*"
)

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

// RoutingPolicy represents a routing policy structure matching the API router config
type RoutingPolicy struct {
	PolicyID         string          `json:"policy_id"`
	OrganizationID   string          `json:"organization_id"` // "*" for global
	Model            string          `json:"model"`
	Backends         []BackendWeight `json:"backends"`
	FailoverThreshold int            `json:"failover_threshold"`
	UpdatedAt        time.Time       `json:"updated_at"`
	Version          int64           `json:"version"`
}

// BackendWeight defines a backend with its routing weight
type BackendWeight struct {
	BackendID string `json:"backend_id"`
	Weight    int    `json:"weight"` // Percentage (0-100)
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
that applies to all organizations (organization_id: "*").`,
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
		orgID = etcdGlobalOrgID
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
		fmt.Fprintf(os.Stderr, "Creating routing policy...\n")
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

	// Connect to etcd
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{cfg.ConfigServiceEndpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to connect to etcd: %v", err),
			"Check your CONFIG_SERVICE_ENDPOINT configuration.",
		)
	}
	defer etcdClient.Close()

	// Create policy object
	policyID := fmt.Sprintf("%s:%s", orgID, model)
	policy := RoutingPolicy{
		PolicyID:          policyID,
		OrganizationID:    orgID,
		Model:             model,
		Backends:          backendWeights,
		FailoverThreshold: 3, // Default failover threshold
		UpdatedAt:         time.Now(),
		Version:           time.Now().Unix(),
	}

	// Serialize to JSON
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to serialize policy: %v", err),
			"Internal error",
		)
	}

	// Store in etcd
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key := etcdKeyPrefix + policyID
	_, err = etcdClient.Put(ctx, key, string(policyJSON))
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to store policy in etcd: %v", err),
			"Check etcd connectivity and permissions.",
		)
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "✓ Routing policy created successfully\n")
		fmt.Fprintf(os.Stderr, "  Policy ID: %s\n", policyID)
		fmt.Fprintf(os.Stderr, "  Key: %s\n", key)
	}

	// Output structured data
	if flagFormat == "json" {
		return output.PrintJSON(policy)
	}

	return nil
}

func routingPolicyListCommand() *cobra.Command {
	var flagFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all routing policies",
		Long:  "List all routing policies stored in etcd",
		Example: `  # List all policies
  admin-cli routing policy list

  # List policies in JSON format
  admin-cli routing policy list --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoutingPolicyList(flagFormat)
		},
	}

	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")

	return cmd
}

func runRoutingPolicyList(flagFormat string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	// Connect to etcd
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{cfg.ConfigServiceEndpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to connect to etcd: %v", err),
			"Check your CONFIG_SERVICE_ENDPOINT configuration.",
		)
	}
	defer etcdClient.Close()

	// Get all policies
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := etcdClient.Get(ctx, etcdKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to list policies: %v", err),
			"Check etcd connectivity.",
		)
	}

	policies := make([]RoutingPolicy, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var policy RoutingPolicy
		if err := json.Unmarshal(kv.Value, &policy); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse policy %s: %v\n", string(kv.Key), err)
			continue
		}
		policies = append(policies, policy)
	}

	if flagFormat == "json" {
		return output.PrintJSON(policies)
	}

	// Table output
	if len(policies) == 0 {
		fmt.Println("No routing policies found.")
		return nil
	}

	fmt.Printf("Found %d routing policies:\n\n", len(policies))
	fmt.Printf("%-40s %-50s %-30s %s\n", "ORGANIZATION ID", "MODEL", "BACKENDS", "UPDATED")
	fmt.Printf("%-40s %-50s %-30s %s\n", strings.Repeat("-", 40), strings.Repeat("-", 50), strings.Repeat("-", 30), strings.Repeat("-", 20))

	for _, policy := range policies {
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
		fmt.Printf("%-40s %-50s %-30s %s\n",
			policy.OrganizationID,
			policy.Model,
			backendSummary,
			policy.UpdatedAt.Format("2006-01-02 15:04:05"),
		)
	}

	return nil
}

func routingPolicyDeleteCommand() *cobra.Command {
	var flagOrgID string
	var flagModel string
	var flagGlobal bool
	var flagQuiet bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a routing policy",
		Long:  "Delete a routing policy from etcd",
		Example: `  # Delete global policy
  admin-cli routing policy delete --global --model qwen2-7b-instruct

  # Delete org-specific policy
  admin-cli routing policy delete \
    --org-id aa6f9015-132a-4694-8b10-7d4d4550faed \
    --model gpt-4`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoutingPolicyDelete(flagOrgID, flagModel, flagGlobal, flagQuiet)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID (optional if --global is used)")
	cmd.Flags().StringVar(&flagModel, "model", "", "Model name (required)")
	cmd.Flags().BoolVar(&flagGlobal, "global", false, "Delete the global policy")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")

	cmd.MarkFlagRequired("model")

	return cmd
}

func runRoutingPolicyDelete(orgID, model string, global bool, quiet bool) error {
	// Determine organization ID
	if global {
		orgID = etcdGlobalOrgID
	} else if orgID == "" {
		return errors.NewValidationError(
			"organization ID required",
			"Provide --org-id or use --global flag",
		)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	// Connect to etcd
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{cfg.ConfigServiceEndpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to connect to etcd: %v", err),
			"Check your CONFIG_SERVICE_ENDPOINT configuration.",
		)
	}
	defer etcdClient.Close()

	// Delete from etcd
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	policyID := fmt.Sprintf("%s:%s", orgID, model)
	key := etcdKeyPrefix + policyID
	resp, err := etcdClient.Delete(ctx, key)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to delete policy: %v", err),
			"Check etcd connectivity.",
		)
	}

	if resp.Deleted == 0 {
		return errors.NewValidationError(
			fmt.Sprintf("policy not found: %s", policyID),
			"Check the organization ID and model name",
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
