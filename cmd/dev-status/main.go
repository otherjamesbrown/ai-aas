// Command dev-status checks dependency health and outputs JSON status.
//
// Purpose:
//   Polls dependency endpoints (PostgreSQL, Redis, NATS, MinIO, mock inference) and
//   returns structured component states in JSON format for tooling consumption.
//
// Usage:
//   dev-status [flags]
//
// Flags:
//   --mode MODE           Mode: remote or local (default: local)
//   --host HOST           Remote workspace host (required for remote mode)
//   --json                Output JSON format (default)
//   --human               Output human-readable format
//   --timeout SECONDS     Component check timeout (default: 2)
//   --component NAME      Check specific component only
//   --diagnose            Show diagnostic information (port conflicts, etc.)
//
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

type ComponentStatus struct {
	Name      string `json:"name"`
	State     string `json:"state"` // healthy, unhealthy, unknown
	LatencyMs int64  `json:"latency_ms"`
	Message   string `json:"message,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
}

type StatusOutput struct {
	Timestamp string            `json:"timestamp"`
	Mode      string            `json:"mode"`
	Components []ComponentStatus `json:"components"`
	Overall   string            `json:"overall"` // healthy, unhealthy, partial
}

var (
	mode      string
	host      string
	jsonOutput bool
	humanOutput bool
	timeout   int
	component string
	diagnose  bool
)

var rootCmd = &cobra.Command{
	Use:   "dev-status",
	Short: "Check development stack component health",
	Long: `Check health status of development stack components (PostgreSQL, Redis, NATS, MinIO, mock inference).

Supports both local and remote modes. For remote mode, use SSH to execute checks on the workspace.`,
	RunE: runStatus,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&mode, "mode", "local", "Mode: remote or local")
	rootCmd.PersistentFlags().StringVar(&host, "host", "", "Remote workspace host (required for remote mode)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", true, "Output JSON format")
	rootCmd.PersistentFlags().BoolVar(&humanOutput, "human", false, "Output human-readable format")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 2, "Component check timeout in seconds")
	rootCmd.PersistentFlags().StringVar(&component, "component", "", "Check specific component only")
	rootCmd.PersistentFlags().BoolVar(&diagnose, "diagnose", false, "Show diagnostic information")
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	// Handle diagnose mode
	if diagnose {
		return runDiagnose(cmd, args)
	}
	
	var components []ComponentStatus

	if mode == "remote" {
		if host == "" {
			return errors.New("--host required for remote mode")
		}
		// For remote mode, would SSH and run checks
		// Placeholder: return status indicating remote checks would run
		return fmt.Errorf("remote mode not yet implemented (requires SSH integration)")
	}

	// Local mode: check local components
	components = checkLocalComponents(ctx, component)

	// Determine overall status
	overall := "healthy"
	unhealthyCount := 0
	for _, c := range components {
		if c.State != "healthy" {
			unhealthyCount++
			if overall == "healthy" {
				overall = "unhealthy"
			}
		}
	}
	if unhealthyCount > 0 && unhealthyCount < len(components) {
		overall = "partial"
	}

	output := StatusOutput{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Mode:       mode,
		Components: components,
		Overall:    overall,
	}

	// Capture telemetry metrics (latency tracking)
	if mode == "local" {
		captureLocalTelemetry(output)
	}

	// Output results
	if humanOutput {
		printHumanOutput(output)
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(output); err != nil {
			return fmt.Errorf("encode JSON: %w", err)
		}
	}

	// Exit with non-zero if unhealthy
	if overall == "unhealthy" {
		os.Exit(1)
	}

	return nil
}

func checkLocalComponents(ctx context.Context, filter string) []ComponentStatus {
	var components []ComponentStatus
	componentsToCheck := []string{"postgres", "redis", "nats", "minio", "mock-inference"}

	if filter != "" {
		componentsToCheck = []string{filter}
	}

	// Load port mappings from .specify/local/ports.yaml if available
	portsMap := loadPortMappings()

	for _, name := range componentsToCheck {
		status := checkComponent(ctx, name)
		
		// Override endpoint with port from config if available
		if port, ok := portsMap[name]; ok && status.Endpoint != "" {
			// Update endpoint port if it matches default
			status.Endpoint = updateEndpointPort(status.Endpoint, port)
		}
		
		components = append(components, status)
	}

	return components
}

// loadPortMappings reads port mappings from .specify/local/ports.yaml
// Returns a map of service name to port number
func loadPortMappings() map[string]string {
	portsFile := ".specify/local/ports.yaml"
	if _, err := os.Stat(portsFile); os.IsNotExist(err) {
		return make(map[string]string)
	}

	// For now, return defaults from ports.yaml structure
	// In production, would parse YAML properly
	ports := make(map[string]string)
	ports["postgres"] = os.Getenv("POSTGRES_PORT")
	if ports["postgres"] == "" {
		ports["postgres"] = "5432"
	}
	
	ports["redis"] = os.Getenv("REDIS_PORT")
	if ports["redis"] == "" {
		ports["redis"] = "6379"
	}
	
	ports["nats"] = os.Getenv("NATS_CLIENT_PORT")
	if ports["nats"] == "" {
		ports["nats"] = "4222"
	}
	
	ports["minio"] = os.Getenv("MINIO_API_PORT")
	if ports["minio"] == "" {
		ports["minio"] = "9000"
	}
	
	ports["mock-inference"] = os.Getenv("MOCK_INFERENCE_PORT")
	if ports["mock-inference"] == "" {
		ports["mock-inference"] = "8000"
	}
	
	return ports
}

// DiagnosticResult contains diagnostic information
type DiagnosticResult struct {
	PortConflicts []PortConflict `json:"port_conflicts,omitempty"`
	TTLWarnings   []TTLWarning   `json:"ttl_warnings,omitempty"`
	NetworkIssues []string       `json:"network_issues,omitempty"`
	ConfigIssues  []string       `json:"config_issues,omitempty"`
}

// PortConflict describes a port conflict
type PortConflict struct {
	Port        string `json:"port"`
	Service     string `json:"service"`
	Process     string `json:"process,omitempty"`
	Remediation string `json:"remediation"`
}

// TTLWarning describes a TTL expiration warning
type TTLWarning struct {
	Workspace   string `json:"workspace"`
	TTLHours    int    `json:"ttl_hours"`
	AgeHours    int    `json:"age_hours"`
	Expired     bool   `json:"expired"`
	Remediation string `json:"remediation"`
}

func runDiagnose(cmd *cobra.Command, args []string) error {
	var result DiagnosticResult
	
	if mode == "local" {
		// Check port conflicts
		result.PortConflicts = checkPortConflicts()
		
		// Check network issues
		result.NetworkIssues = checkNetworkIssues()
		
		// Check config issues
		result.ConfigIssues = checkConfigIssues()
	} else if mode == "remote" {
		if host == "" {
			return errors.New("--host required for remote mode")
		}
		
		// Check TTL warnings
		result.TTLWarnings = checkRemoteTTL()
		
		// Remote network/config checks would go here
		result.NetworkIssues = []string{"Remote diagnostics not fully implemented"}
	}
	
	// Output diagnostic results
	if humanOutput {
		printDiagnosticHuman(result)
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return fmt.Errorf("encode JSON: %w", err)
		}
	}
	
	// Exit with non-zero if issues found
	if len(result.PortConflicts) > 0 || len(result.TTLWarnings) > 0 || len(result.NetworkIssues) > 0 || len(result.ConfigIssues) > 0 {
		os.Exit(1)
	}
	
	return nil
}

func checkPortConflicts() []PortConflict {
	var conflicts []PortConflict
	
	// Default ports to check
	ports := map[string]string{
		"5432": "postgres",
		"6379": "redis",
		"4222": "nats",
		"8222": "nats-http",
		"9000": "minio-api",
		"9001": "minio-console",
		"8000": "mock-inference",
	}
	
	// Check each port
	for port, service := range ports {
		conflict := checkPort(port, service)
		if conflict != nil {
			conflicts = append(conflicts, *conflict)
		}
	}
	
	return conflicts
}

func checkPort(port, service string) *PortConflict {
	// Try to connect to port
	conn, err := net.DialTimeout("tcp", "localhost:"+port, 100*time.Millisecond)
	if err != nil {
		return nil // Port not in use
	}
	conn.Close()
	
	// Port is in use - check if it's Docker (placeholder, would check docker ps output)
	dockerUsing := false
	if dockerPs := os.Getenv("DOCKER_PS_CHECK"); dockerPs != "" {
		dockerUsing = false // Placeholder
	}
	
	if dockerUsing {
		return nil // Docker is using it, not a conflict
	}
	
	// Port conflict detected
	remediation := fmt.Sprintf("Override port via environment variable: export %s_PORT=<alternate>", 
		strings.ToUpper(strings.ReplaceAll(service, "-", "_")))
	
	return &PortConflict{
		Port:        port,
		Service:     service,
		Remediation: remediation,
	}
}

func checkRemoteTTL() []TTLWarning {
	var warnings []TTLWarning
	
	// Placeholder: In real implementation, would SSH to remote and check TTL metadata
	// For now, return empty (would need SSH integration)
	
	return warnings
}

func checkNetworkIssues() []string {
	var issues []string
	
	// Check if Docker network exists
	// Placeholder: Would check docker network ls
	
	return issues
}

func checkConfigIssues() []string {
	var issues []string
	
	// Check if compose files exist
	composeBase := ".dev/compose/compose.base.yaml"
	composeLocal := ".dev/compose/compose.local.yaml"
	
	if _, err := os.Stat(composeBase); os.IsNotExist(err) {
		issues = append(issues, fmt.Sprintf("Compose base file not found: %s", composeBase))
	}
	
	if _, err := os.Stat(composeLocal); os.IsNotExist(err) {
		issues = append(issues, fmt.Sprintf("Compose local override not found: %s", composeLocal))
	}
	
	return issues
}

func printDiagnosticHuman(result DiagnosticResult) {
	if len(result.PortConflicts) > 0 {
		fmt.Fprintf(os.Stderr, "Port Conflicts:\n")
		for _, conflict := range result.PortConflicts {
			fmt.Fprintf(os.Stderr, "  - Port %s (%s) is in use\n", conflict.Port, conflict.Service)
			fmt.Fprintf(os.Stderr, "    Remediation: %s\n", conflict.Remediation)
		}
	}
	
	if len(result.TTLWarnings) > 0 {
		fmt.Fprintf(os.Stderr, "TTL Warnings:\n")
		for _, warning := range result.TTLWarnings {
			status := "expired"
			if !warning.Expired {
				status = fmt.Sprintf("expires in %d hours", warning.TTLHours-warning.AgeHours)
			}
			fmt.Fprintf(os.Stderr, "  - Workspace %s: %s (age: %dh, TTL: %dh)\n", 
				warning.Workspace, status, warning.AgeHours, warning.TTLHours)
			fmt.Fprintf(os.Stderr, "    Remediation: %s\n", warning.Remediation)
		}
	}
	
	if len(result.NetworkIssues) > 0 {
		fmt.Fprintf(os.Stderr, "Network Issues:\n")
		for _, issue := range result.NetworkIssues {
			fmt.Fprintf(os.Stderr, "  - %s\n", issue)
		}
	}
	
	if len(result.ConfigIssues) > 0 {
		fmt.Fprintf(os.Stderr, "Config Issues:\n")
		for _, issue := range result.ConfigIssues {
			fmt.Fprintf(os.Stderr, "  - %s\n", issue)
		}
	}
	
	if len(result.PortConflicts) == 0 && len(result.TTLWarnings) == 0 && 
		len(result.NetworkIssues) == 0 && len(result.ConfigIssues) == 0 {
		fmt.Fprintf(os.Stderr, "No issues detected\n")
	}
}

// updateEndpointPort updates the port in an endpoint URL/string
func updateEndpointPort(endpoint, newPort string) string {
	// Simple port replacement for common formats
	// postgres://host:oldport/db -> postgres://host:newport/db
	// localhost:oldport -> localhost:newport
	// http://localhost:oldport -> http://localhost:newport
	
	if strings.Contains(endpoint, ":") {
		// Try to replace port number in endpoint
		re := regexp.MustCompile(`:\d+`)
		return re.ReplaceAllString(endpoint, ":"+newPort)
	}
	
	return endpoint
}

func checkComponent(ctx context.Context, name string) ComponentStatus {
	start := time.Now()
	
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	status := ComponentStatus{
		Name:  name,
		State: "unknown",
	}

	switch name {
	case "postgres":
		status = checkPostgres(ctx)
	case "redis":
		status = checkRedis(ctx)
	case "nats":
		status = checkNATS(ctx)
	case "minio":
		status = checkMinIO(ctx)
	case "mock-inference":
		status = checkMockInference(ctx)
	default:
		status.Message = fmt.Sprintf("unknown component: %s", name)
	}

	status.LatencyMs = time.Since(start).Milliseconds()
	return status
}

func checkPostgres(ctx context.Context) ComponentStatus {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/ai_aas?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return ComponentStatus{
			Name:     "postgres",
			State:    "unhealthy",
			Message:  fmt.Sprintf("connection failed: %v", err),
			Endpoint: dsn,
		}
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return ComponentStatus{
			Name:     "postgres",
			State:    "unhealthy",
			Message:  fmt.Sprintf("ping failed: %v", err),
			Endpoint: dsn,
		}
	}

	return ComponentStatus{
		Name:     "postgres",
		State:    "healthy",
		Message:  "connection successful",
		Endpoint: dsn,
	}
}

func checkRedis(ctx context.Context) ComponentStatus {
	// Simplified Redis check - in production would use redis client
	endpoint := "localhost:6379"
	if os.Getenv("REDIS_ADDR") != "" {
		endpoint = os.Getenv("REDIS_ADDR")
	}

	// Try TCP connection as a basic check
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return ComponentStatus{
			Name:     "redis",
			State:    "unhealthy",
			Message:  fmt.Sprintf("connection failed: %v", err),
			Endpoint: endpoint,
		}
	}
	defer conn.Close()

	return ComponentStatus{
		Name:     "redis",
		State:    "healthy",
		Message:  "connection successful",
		Endpoint: endpoint,
	}
}

func checkNATS(ctx context.Context) ComponentStatus {
	endpoint := "http://localhost:8222/healthz"
	if os.Getenv("NATS_HTTP_ADDR") != "" {
		endpoint = fmt.Sprintf("http://%s/healthz", os.Getenv("NATS_HTTP_ADDR"))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return ComponentStatus{
			Name:     "nats",
			State:    "unhealthy",
			Message:  fmt.Sprintf("request creation failed: %v", err),
			Endpoint: endpoint,
		}
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ComponentStatus{
			Name:     "nats",
			State:    "unhealthy",
			Message:  fmt.Sprintf("request failed: %v", err),
			Endpoint: endpoint,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ComponentStatus{
			Name:     "nats",
			State:    "unhealthy",
			Message:  fmt.Sprintf("unexpected status: %d", resp.StatusCode),
			Endpoint: endpoint,
		}
	}

	return ComponentStatus{
		Name:     "nats",
		State:    "healthy",
		Message:  "health check passed",
		Endpoint: endpoint,
	}
}

func checkMinIO(ctx context.Context) ComponentStatus {
	endpoint := "http://localhost:9000/minio/health/live"
	if os.Getenv("MINIO_ENDPOINT") != "" {
		endpoint = fmt.Sprintf("%s/minio/health/live", os.Getenv("MINIO_ENDPOINT"))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return ComponentStatus{
			Name:     "minio",
			State:    "unhealthy",
			Message:  fmt.Sprintf("request creation failed: %v", err),
			Endpoint: endpoint,
		}
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ComponentStatus{
			Name:     "minio",
			State:    "unhealthy",
			Message:  fmt.Sprintf("request failed: %v", err),
			Endpoint: endpoint,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ComponentStatus{
			Name:     "minio",
			State:    "unhealthy",
			Message:  fmt.Sprintf("unexpected status: %d", resp.StatusCode),
			Endpoint: endpoint,
		}
	}

	return ComponentStatus{
		Name:     "minio",
		State:    "healthy",
		Message:  "health check passed",
		Endpoint: endpoint,
	}
}

func checkMockInference(ctx context.Context) ComponentStatus {
	endpoint := "http://localhost:8000/health"
	if os.Getenv("MOCK_INFERENCE_ENDPOINT") != "" {
		endpoint = os.Getenv("MOCK_INFERENCE_ENDPOINT")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return ComponentStatus{
			Name:     "mock-inference",
			State:    "unhealthy",
			Message:  fmt.Sprintf("request creation failed: %v", err),
			Endpoint: endpoint,
		}
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ComponentStatus{
			Name:     "mock-inference",
			State:    "unhealthy",
			Message:  fmt.Sprintf("request failed: %v", err),
			Endpoint: endpoint,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ComponentStatus{
			Name:     "mock-inference",
			State:    "unhealthy",
			Message:  fmt.Sprintf("unexpected status: %d", resp.StatusCode),
			Endpoint: endpoint,
		}
	}

	return ComponentStatus{
		Name:     "mock-inference",
		State:    "healthy",
		Message:  "health check passed",
		Endpoint: endpoint,
	}
}

func printHumanOutput(output StatusOutput) {
	fmt.Printf("Development Stack Status\n")
	fmt.Printf("Mode: %s\n", output.Mode)
	fmt.Printf("Timestamp: %s\n", output.Timestamp)
	fmt.Printf("Overall: %s\n\n", output.Overall)
	
	fmt.Printf("Components:\n")
	for _, c := range output.Components {
		statusIcon := "✓"
		if c.State != "healthy" {
			statusIcon = "✗"
		}
		fmt.Printf("  %s %s: %s (%dms)\n", statusIcon, c.Name, c.State, c.LatencyMs)
		if c.Message != "" {
			fmt.Printf("      %s\n", c.Message)
		}
	}
}

// captureLocalTelemetry emits telemetry metrics for local mode
// Compatible with CI latency checks (JSON output to metrics endpoint)
func captureLocalTelemetry(output StatusOutput) {
	if os.Getenv("METRICS") != "true" {
		return
	}

	// Calculate aggregate metrics
	totalLatency := int64(0)
	healthyCount := 0
	
	for _, c := range output.Components {
		totalLatency += c.LatencyMs
		if c.State == "healthy" {
			healthyCount++
		}
	}

	avgLatency := int64(0)
	if len(output.Components) > 0 {
		avgLatency = totalLatency / int64(len(output.Components))
	}

	metrics := map[string]interface{}{
		"timestamp":      output.Timestamp,
		"mode":           "local",
		"overall":        output.Overall,
		"total_latency_ms": totalLatency,
		"avg_latency_ms":   avgLatency,
		"healthy_count":    healthyCount,
		"total_components": len(output.Components),
		"components":       output.Components,
	}

	// Emit metrics (in production, would send to metrics endpoint)
	// Metrics are always captured when METRICS=true, but only output when in JSON mode
	if jsonOutput {
		enc := json.NewEncoder(os.Stderr) // Use stderr to avoid interfering with status output
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]interface{}{
			"metrics": metrics,
		})
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

