// Package commands provides batch operation support.
//
// Purpose:
//
//	Batch operation processing with file input parsing (JSON/YAML), dry-run preview,
//	checkpoint/resume capability, partial failure handling, and structured error reporting.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#US-002 (Day-2 Management)
//   - specs/009-admin-cli/spec.md#FR-011 (batch operations)
//
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/output"
)

// BatchOperation represents a single operation in a batch file.
type BatchOperation struct {
	Type      string                 `json:"type" yaml:"type"`           // "org_create", "org_update", "user_create", etc.
	OrgID     string                 `json:"orgId,omitempty" yaml:"orgId,omitempty"`
	UserID    string                 `json:"userId,omitempty" yaml:"userId,omitempty"`
	Email     string                 `json:"email,omitempty" yaml:"email,omitempty"`
	Data      map[string]interface{} `json:"data" yaml:"data"`          // Operation-specific data
	Index     int                    `json:"-" yaml:"-"`                // Index in batch (internal)
}

// BatchFile represents the structure of a batch operation file.
type BatchFile struct {
	Operations []BatchOperation `json:"operations" yaml:"operations"`
	Metadata   map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// BatchResult represents the result of processing a batch operation.
type BatchResult struct {
	Index       int                    `json:"index"`
	Type        string                 `json:"type"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	ProcessedAt string                 `json:"processedAt"`
}

// BatchSummary represents the summary of a batch operation execution.
type BatchSummary struct {
	Total      int           `json:"total"`
	Successful int           `json:"successful"`
	Failed     int           `json:"failed"`
	Skipped    int           `json:"skipped"`
	Results    []BatchResult `json:"results"`
	Checkpoint string        `json:"checkpoint,omitempty"` // Path to checkpoint file
	Duration   string        `json:"duration"`
}

// CheckpointFile represents a checkpoint for resuming batch operations.
type CheckpointFile struct {
	BatchFile  string                 `json:"batchFile"`
	Processed  []int                  `json:"processed"` // Indices of successfully processed operations
	Failed     []int                  `json:"failed"`    // Indices of failed operations
	LastIndex  int                    `json:"lastIndex"` // Last processed index
	CreatedAt  string                 `json:"createdAt"`
	Results    []BatchResult           `json:"results"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ParseBatchFile parses a batch operation file (JSON or YAML).
func ParseBatchFile(filePath string) (*BatchFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch file: %w", err)
	}

	var batch BatchFile
	
	// Try JSON first, then YAML
	if err := json.Unmarshal(data, &batch); err != nil {
		if err := yaml.Unmarshal(data, &batch); err != nil {
			return nil, fmt.Errorf("failed to parse batch file (must be valid JSON or YAML): %w", err)
		}
	}

	// Assign indices to operations
	for i := range batch.Operations {
		batch.Operations[i].Index = i
	}

	return &batch, nil
}

// LoadCheckpoint loads a checkpoint file for resuming batch operations.
func LoadCheckpoint(checkpointPath string) (*CheckpointFile, error) {
	data, err := os.ReadFile(checkpointPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint file: %w", err)
	}

	var checkpoint CheckpointFile
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to parse checkpoint file: %w", err)
	}

	return &checkpoint, nil
}

// SaveCheckpoint saves a checkpoint file for resuming batch operations.
func SaveCheckpoint(checkpointPath string, checkpoint *CheckpointFile) error {
	checkpoint.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	if err := os.WriteFile(checkpointPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write checkpoint file: %w", err)
	}

	return nil
}

// CreateCheckpointPath generates a checkpoint file path based on the batch file path.
func CreateCheckpointPath(batchFilePath string) string {
	ext := filepath.Ext(batchFilePath)
	base := batchFilePath[:len(batchFilePath)-len(ext)]
	timestamp := time.Now().Format("2006-01-02-150405")
	return fmt.Sprintf("%s-checkpoint-%s.json", base, timestamp)
}

// GenerateDiff generates a diff preview for batch operations.
func GenerateDiff(operations []BatchOperation, existing map[int]interface{}) map[int]map[string]interface{} {
	diffs := make(map[int]map[string]interface{})
	
	for _, op := range operations {
		diff := make(map[string]interface{})
		diff["type"] = op.Type
		diff["operation"] = "create" // Default, can be "update" or "delete" based on type
		
		if existing != nil {
			if existing[op.Index] != nil {
				diff["operation"] = "update"
				diff["existing"] = existing[op.Index]
			}
		}
		
		diff["proposed"] = op.Data
		diffs[op.Index] = diff
	}
	
	return diffs
}

// ProcessBatch processes a batch of operations with checkpoint and error handling support.
func ProcessBatch(
	batch *BatchFile,
	checkpoint *CheckpointFile,
	continueOnError bool,
	processor func(op BatchOperation) (map[string]interface{}, error),
) (*BatchSummary, error) {
	startTime := time.Now()
	summary := &BatchSummary{
		Total:    len(batch.Operations),
		Results:  make([]BatchResult, 0, len(batch.Operations)),
	}

	// Determine starting index from checkpoint
	startIndex := 0
	if checkpoint != nil {
		startIndex = checkpoint.LastIndex + 1
		summary.Skipped = len(checkpoint.Processed)
		summary.Successful = len(checkpoint.Processed)
		summary.Failed = len(checkpoint.Failed)
		summary.Results = checkpoint.Results
	}

	// Process operations
	for i := startIndex; i < len(batch.Operations); i++ {
		op := batch.Operations[i]
		
		result := BatchResult{
			Index:       op.Index,
			Type:        op.Type,
			ProcessedAt: time.Now().UTC().Format(time.RFC3339),
		}

		// Process operation
		details, err := processor(op)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			summary.Failed++
			
			if !continueOnError {
				// Stop processing on first error
				return summary, fmt.Errorf("batch processing failed at index %d: %w", i, err)
			}
			// Continue processing with --continue-on-error
		} else {
			result.Success = true
			result.Details = details
			summary.Successful++
		}

		summary.Results = append(summary.Results, result)
	}

	summary.Duration = time.Since(startTime).String()
	return summary, nil
}

// PrintBatchPreview prints a dry-run preview of batch operations with diff output and summary counts.
func PrintBatchPreview(operations []BatchOperation, existing map[int]interface{}, format string) error {
	diffs := GenerateDiff(operations, existing)
	
	if format == "json" {
		preview := map[string]interface{}{
			"mode":       "dry-run",
			"total":      len(operations),
			"operations": diffs,
			"summary": map[string]int{
				"total":    len(operations),
				"creates":  0, // Count by operation type
				"updates":  0,
				"deletes":  0,
			},
		}
		
		// Count operation types
		for _, op := range operations {
			if existing != nil && existing[op.Index] != nil {
				preview["summary"].(map[string]int)["updates"]++
			} else {
				if op.Type == "org_delete" || op.Type == "user_delete" {
					preview["summary"].(map[string]int)["deletes"]++
				} else {
					preview["summary"].(map[string]int)["creates"]++
				}
			}
		}
		
		return output.PrintJSON(preview)
	}
	
	// Table format
	fmt.Println("BATCH OPERATION PREVIEW (DRY-RUN)")
	fmt.Println("============================================================")
	fmt.Printf("Total operations: %d\n", len(operations))
	
	creates := 0
	updates := 0
	deletes := 0
	
	for _, op := range operations {
		if existing != nil && existing[op.Index] != nil {
			updates++
		} else {
			if op.Type == "org_delete" || op.Type == "user_delete" {
				deletes++
			} else {
				creates++
			}
		}
	}
	
	fmt.Printf("  Creates: %d\n", creates)
	fmt.Printf("  Updates: %d\n", updates)
	fmt.Printf("  Deletes: %d\n", deletes)
	fmt.Println("\nOperations:")
	
	for i, op := range operations {
		diff, exists := diffs[i]
		if !exists {
			continue
		}
		
		operationType := diff["operation"].(string)
		fmt.Printf("\n[%d] %s: %s\n", i, operationType, op.Type)
		if op.OrgID != "" {
			fmt.Printf("     Org ID: %s\n", op.OrgID)
		}
		if op.Email != "" {
			fmt.Printf("     Email: %s\n", op.Email)
		}
		if proposed, ok := diff["proposed"].(map[string]interface{}); ok {
			fmt.Printf("     Data: %+v\n", proposed)
		}
	}
	
	return nil
}

// ValidateBatchFile validates that a batch file has valid operations.
func ValidateBatchFile(batch *BatchFile) error {
	if len(batch.Operations) == 0 {
		return fmt.Errorf("batch file contains no operations")
	}

	for i, op := range batch.Operations {
		if op.Type == "" {
			return fmt.Errorf("operation at index %d missing required field 'type'", i)
		}

		// Validate required fields based on operation type
		switch op.Type {
		case "org_create", "org_update", "org_delete":
			if op.OrgID == "" && op.Type != "org_create" {
				// Org ID required for update/delete
				return fmt.Errorf("operation at index %d (type: %s) missing required field 'orgId'", i, op.Type)
			}
		case "user_create", "user_update", "user_delete":
			if op.OrgID == "" {
				return fmt.Errorf("operation at index %d (type: %s) missing required field 'orgId'", i, op.Type)
			}
			if op.Type == "user_create" && op.Email == "" {
				return fmt.Errorf("operation at index %d (type: %s) missing required field 'email'", i, op.Type)
			}
		default:
			return fmt.Errorf("operation at index %d has unknown type: %s", i, op.Type)
		}
	}

	return nil
}

