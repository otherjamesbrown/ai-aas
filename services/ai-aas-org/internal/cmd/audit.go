package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
)

// auditCmd represents the audit command
var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View audit logs",
	Long: `View audit logs for your organization.

Audit logs track administrative actions like user creation, API key
management, and permission changes. Use these logs for security
monitoring and compliance.

Examples:
  ai-aas-org audit list
  ai-aas-org audit list --action user.created
  ai-aas-org audit list --limit 50`,
}

func init() {
	rootCmd.AddCommand(auditCmd)
}

// --- audit list ---

var (
	auditListAction   string
	auditListResource string
	auditListActor    string
	auditListLimit    int
)

var auditListCmd = &cobra.Command{
	Use:   "list",
	Short: "List audit events",
	Long: `List recent audit events for your organization.

Filters:
  --action       Filter by action (e.g., user.created, apikey.deleted)
  --resource     Filter by resource type (user, apikey, model)
  --actor        Filter by actor email
  --limit        Maximum number of events to return

Examples:
  ai-aas-org audit list
  ai-aas-org audit list --action user.created
  ai-aas-org audit list --resource apikey --limit 20`,
	RunE: runAuditList,
}

func init() {
	auditCmd.AddCommand(auditListCmd)

	auditListCmd.Flags().StringVar(&auditListAction, "action", "", "filter by action")
	auditListCmd.Flags().StringVar(&auditListResource, "resource", "", "filter by resource type")
	auditListCmd.Flags().StringVar(&auditListActor, "actor", "", "filter by actor email")
	auditListCmd.Flags().IntVar(&auditListLimit, "limit", 25, "maximum events to return")
}

func runAuditList(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	filters := &api.AuditFilters{
		Action:       auditListAction,
		ResourceType: auditListResource,
		ActorID:      auditListActor,
		Limit:        auditListLimit,
	}

	result, err := client.ListAuditEvents(ctx, config.GetOrgID(), filters)
	if err != nil {
		return errors.NewOperationError("failed to list audit events", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(result.Events)
	}

	if len(result.Events) == 0 {
		output.InfoMsg("No audit events found.")
		return nil
	}

	output.Header("Audit Log")
	fmt.Println()

	headers := []string{"Time", "Action", "Resource", "Actor", "Status"}
	var rows [][]string
	for _, e := range result.Events {
		resource := e.ResourceType
		if e.ResourceID != "" {
			resource += "/" + truncate(e.ResourceID, 12)
		}
		rows = append(rows, []string{
			e.Timestamp.Format("01-02 15:04"),
			e.Action,
			resource,
			e.ActorEmail,
			output.StatusBadge(e.Status),
		})
	}

	output.PrintTable(headers, rows)

	if result.HasMore {
		fmt.Println()
		output.InfoMsg("More events available. Use --limit to see more.")
	}

	return nil
}

// truncate shortens a string if it's too long.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
