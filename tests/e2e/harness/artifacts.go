package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ArtifactCollector collects and stores test artifacts
type ArtifactCollector struct {
	outputDir string
	runID     string
	artifacts []Artifact
}

// Artifact represents a test artifact
type Artifact struct {
	ID           string
	TestCaseID   string
	Type         string
	Name         string
	Path         string
	SizeBytes    int64
	CreatedAt    time.Time
	Metadata     map[string]string
}

// NewArtifactCollector creates a new artifact collector
func NewArtifactCollector(outputDir, runID string) *ArtifactCollector {
	return &ArtifactCollector{
		outputDir: outputDir,
		runID:     runID,
		artifacts: []Artifact{},
	}
}

// CollectRequestResponse collects request/response artifacts
func (ac *ArtifactCollector) CollectRequestResponse(testCaseID, stepID string, request *RequestDetails, response *ResponseDetails, correlationIDs map[string]string) error {
	artifactDir := filepath.Join(ac.outputDir, ac.runID, testCaseID)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}

	// Save request
	requestPath := filepath.Join(artifactDir, fmt.Sprintf("request-%s.json", stepID))
	requestData, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	if err := os.WriteFile(requestPath, requestData, 0644); err != nil {
		return fmt.Errorf("write request file: %w", err)
	}

	// Save response
	responsePath := filepath.Join(artifactDir, fmt.Sprintf("response-%s.json", stepID))
	responseData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if err := os.WriteFile(responsePath, responseData, 0644); err != nil {
		return fmt.Errorf("write response file: %w", err)
	}

	// Save correlation IDs
	if len(correlationIDs) > 0 {
		corrPath := filepath.Join(artifactDir, fmt.Sprintf("correlation-%s.json", stepID))
		corrData, err := json.MarshalIndent(correlationIDs, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal correlation IDs: %w", err)
		}
		if err := os.WriteFile(corrPath, corrData, 0644); err != nil {
			return fmt.Errorf("write correlation file: %w", err)
		}
	}

	// Register artifact
	artifact := Artifact{
		ID:         stepID,
		TestCaseID: testCaseID,
		Type:       "request-response",
		Name:       fmt.Sprintf("request-response-%s", stepID),
		Path:       artifactDir,
		SizeBytes:  int64(len(requestData) + len(responseData)),
		CreatedAt:  time.Now(),
		Metadata:   correlationIDs,
	}

	ac.artifacts = append(ac.artifacts, artifact)
	return nil
}

// List returns all collected artifacts
func (ac *ArtifactCollector) List() []Artifact {
	return ac.artifacts
}

