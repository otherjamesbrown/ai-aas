// Package benchmark provides CLI commands for benchmark management.
package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// PrometheusQueryResult represents the Prometheus query API response
type PrometheusQueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// BenchmarkResult represents aggregated benchmark results for a model
type BenchmarkResult struct {
	Model         string    `json:"model"`
	TTFT          float64   `json:"ttft_ms,omitempty"`
	ITL           float64   `json:"itl_ms,omitempty"`
	TokensPerSec  float64   `json:"tokens_per_sec,omitempty"`
	LatencyP50    float64   `json:"latency_p50_ms,omitempty"`
	LatencyP95    float64   `json:"latency_p95_ms,omitempty"`
	LatencyP99    float64   `json:"latency_p99_ms,omitempty"`
	SuccessCount  float64   `json:"success_count,omitempty"`
	ErrorCount    float64   `json:"error_count,omitempty"`
	LastTimestamp time.Time `json:"last_timestamp"`
}

// newResultsCommand creates the benchmark results command
func newResultsCommand() *cobra.Command {
	var (
		format        string
		modelFilter   string
		prometheusURL string
	)

	cmd := &cobra.Command{
		Use:   "results",
		Short: "Display latest benchmark results from Prometheus",
		Long: `Query Prometheus for the latest benchmark results from guidellm-runner.

Displays metrics including:
  - TTFT (Time to First Token)
  - ITL (Inter-Token Latency)
  - Tokens/sec throughput
  - Request latency (p50, p95, p99)
  - Success/error counts

By default, uses the in-cluster Prometheus endpoint. Override with --prometheus-url.

Examples:
  # Show all model results
  ai-aas-cli benchmark results

  # Show results for a specific model
  ai-aas-cli benchmark results --model llama-7b

  # Output as JSON
  ai-aas-cli benchmark results --format json

  # Use custom Prometheus URL
  ai-aas-cli benchmark results --prometheus-url http://localhost:9090`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Default Prometheus URL (in-cluster staging)
			if prometheusURL == "" {
				prometheusURL = "http://kube-prometheus-stack-stag-prometheus.monitoring.svc.cluster.local:9090"
			}

			results, err := queryBenchmarkResults(ctx, prometheusURL, modelFilter)
			if err != nil {
				return fmt.Errorf("failed to query Prometheus: %w", err)
			}

			if len(results) == 0 {
				fmt.Println("No benchmark results found in Prometheus.")
				fmt.Println("\nEnsure guidellm-runner is running and exporting metrics.")
				return nil
			}

			if format == "json" {
				return output.PrintJSON(results, true)
			}

			// Table output
			table := output.NewTableWriter()
			table.SetHeader([]string{"MODEL", "TTFT (ms)", "ITL (ms)", "TOKENS/SEC", "P50 (ms)", "P95 (ms)", "P99 (ms)", "SUCCESS", "ERRORS", "LAST UPDATE"})

			for _, r := range results {
				table.Append([]string{
					r.Model,
					formatFloat(r.TTFT),
					formatFloat(r.ITL),
					formatFloat(r.TokensPerSec),
					formatFloat(r.LatencyP50),
					formatFloat(r.LatencyP95),
					formatFloat(r.LatencyP99),
					formatFloat(r.SuccessCount),
					formatFloat(r.ErrorCount),
					formatTimestamp(r.LastTimestamp),
				})
			}

			table.Render()
			fmt.Printf("\nTotal: %d model(s)\n", len(results))

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")
	cmd.Flags().StringVar(&modelFilter, "model", "", "filter by model name")
	cmd.Flags().StringVar(&prometheusURL, "prometheus-url", "", "Prometheus URL (default: in-cluster staging endpoint)")

	return cmd
}

// queryBenchmarkResults queries Prometheus for latest benchmark results
func queryBenchmarkResults(ctx context.Context, prometheusURL, modelFilter string) ([]BenchmarkResult, error) {
	// Build map of model -> metrics
	resultsMap := make(map[string]*BenchmarkResult)

	// Define queries for each metric
	queries := map[string]string{
		"ttft": `guidellm_benchmark_ttft_seconds{quantile="0.5"}`,
		"itl":  `guidellm_benchmark_itl_seconds{quantile="0.5"}`,
		"tps":  `guidellm_benchmark_tokens_per_second`,
		"p50":  `guidellm_benchmark_latency_seconds{quantile="0.5"}`,
		"p95":  `guidellm_benchmark_latency_seconds{quantile="0.95"}`,
		"p99":  `guidellm_benchmark_latency_seconds{quantile="0.99"}`,
		"succ": `guidellm_benchmark_success_total`,
		"err":  `guidellm_benchmark_error_total`,
	}

	// Query each metric
	for metricName, query := range queries {
		values, err := queryPrometheus(ctx, prometheusURL, query)
		if err != nil {
			// Non-fatal - some metrics may not exist yet
			continue
		}

		// Aggregate by model
		for model, value := range values {
			// Apply model filter if specified
			if modelFilter != "" && model != modelFilter {
				continue
			}

			if _, exists := resultsMap[model]; !exists {
				resultsMap[model] = &BenchmarkResult{
					Model: model,
				}
			}

			// Store metric value
			switch metricName {
			case "ttft":
				resultsMap[model].TTFT = value * 1000 // Convert to ms
			case "itl":
				resultsMap[model].ITL = value * 1000 // Convert to ms
			case "tps":
				resultsMap[model].TokensPerSec = value
			case "p50":
				resultsMap[model].LatencyP50 = value * 1000 // Convert to ms
			case "p95":
				resultsMap[model].LatencyP95 = value * 1000 // Convert to ms
			case "p99":
				resultsMap[model].LatencyP99 = value * 1000 // Convert to ms
			case "succ":
				resultsMap[model].SuccessCount = value
			case "err":
				resultsMap[model].ErrorCount = value
			}
		}
	}

	// Convert map to slice
	var results []BenchmarkResult
	for _, r := range resultsMap {
		r.LastTimestamp = time.Now() // Use current time as proxy
		results = append(results, *r)
	}

	return results, nil
}

// queryPrometheus executes a PromQL query and returns model -> value mapping
func queryPrometheus(ctx context.Context, prometheusURL, query string) (map[string]float64, error) {
	// Build query URL
	u, err := url.Parse(prometheusURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Prometheus URL: %w", err)
	}
	u.Path = "/api/v1/query"

	params := url.Values{}
	params.Set("query", query)
	u.RawQuery = params.Encode()

	// Execute HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result PrometheusQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("query failed: %s", result.Status)
	}

	// Extract model -> value mapping
	values := make(map[string]float64)
	for _, item := range result.Data.Result {
		// Get model name from metric labels
		model := item.Metric["model"]
		if model == "" {
			// Try other common label names
			if m := item.Metric["model_name"]; m != "" {
				model = m
			} else if m := item.Metric["job"]; m != "" {
				model = m
			} else {
				model = "unknown"
			}
		}

		// Get value (prometheus returns [timestamp, "value_as_string"])
		if len(item.Value) >= 2 {
			if valueStr, ok := item.Value[1].(string); ok {
				var value float64
				fmt.Sscanf(valueStr, "%f", &value)
				values[model] = value
			}
		}
	}

	return values, nil
}

// formatFloat formats a float64 for table display
func formatFloat(v float64) string {
	if v == 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f", v)
}

// formatTimestamp formats a timestamp for table display
func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}
