// Package gitops provides git operations for GitOps-based deployments.
// It manages model deployment manifests in the ai-aas-config repository.
package gitops

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client manages git operations for the ai-aas-config repository.
type Client struct {
	repoPath string
}

// NewClient creates a new GitOps client.
// repoPath is the local path to the ai-aas-config repository.
func NewClient(repoPath string) (*Client, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(repoPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		repoPath = filepath.Join(home, repoPath[2:])
	}

	// Verify the path exists and is a git repository
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a git repository: %s", repoPath)
	}

	return &Client{
		repoPath: repoPath,
	}, nil
}

// GetRepoPath returns the repository path.
func (c *Client) GetRepoPath() string {
	return c.repoPath
}

// BranchForEnvironment returns the git branch name for an environment.
// Maps: development -> develop, staging -> staging, production -> main
func BranchForEnvironment(environment string) string {
	switch environment {
	case "development":
		return "develop"
	case "staging":
		return "staging"
	case "production":
		return "main"
	default:
		// Default to environment name as branch
		return environment
	}
}

// CheckoutBranch checks out the specified branch and pulls latest changes.
func (c *Client) CheckoutBranch(ctx context.Context, branch string) error {
	// Fetch latest
	if err := c.runGit(ctx, "fetch", "origin", branch); err != nil {
		return fmt.Errorf("fetch branch %s: %w", branch, err)
	}

	// Checkout branch
	if err := c.runGit(ctx, "checkout", branch); err != nil {
		return fmt.Errorf("checkout branch %s: %w", branch, err)
	}

	// Reset to origin to ensure we're in sync
	if err := c.runGit(ctx, "reset", "--hard", fmt.Sprintf("origin/%s", branch)); err != nil {
		return fmt.Errorf("reset to origin/%s: %w", branch, err)
	}

	return nil
}

// CurrentBranch returns the current branch name.
func (c *Client) CurrentBranch(ctx context.Context) (string, error) {
	output, err := c.runGitOutput(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// WriteModelManifest writes an AIModel manifest to the repository.
// It creates the file at environments/<environment>/models/<modelName>.yaml
func (c *Client) WriteModelManifest(environment, modelName string, manifest []byte) (string, error) {
	// Build path
	relativePath := filepath.Join("environments", environment, "models", modelName+".yaml")
	fullPath := filepath.Join(c.repoPath, relativePath)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(fullPath, manifest, 0644); err != nil {
		return "", fmt.Errorf("write file %s: %w", fullPath, err)
	}

	return relativePath, nil
}

// DeleteModelManifest deletes an AIModel manifest from the repository.
func (c *Client) DeleteModelManifest(environment, modelName string) (string, error) {
	relativePath := filepath.Join("environments", environment, "models", modelName+".yaml")
	fullPath := filepath.Join(c.repoPath, relativePath)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("model manifest not found: %s", relativePath)
	}

	// Delete file
	if err := os.Remove(fullPath); err != nil {
		return "", fmt.Errorf("delete file %s: %w", fullPath, err)
	}

	return relativePath, nil
}

// ModelManifestExists checks if a model manifest exists in the repository.
func (c *Client) ModelManifestExists(environment, modelName string) bool {
	relativePath := filepath.Join("environments", environment, "models", modelName+".yaml")
	fullPath := filepath.Join(c.repoPath, relativePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

// StageFile stages a file for commit.
func (c *Client) StageFile(ctx context.Context, relativePath string) error {
	if err := c.runGit(ctx, "add", relativePath); err != nil {
		return fmt.Errorf("stage file %s: %w", relativePath, err)
	}
	return nil
}

// StageDeleted stages a deleted file for commit.
func (c *Client) StageDeleted(ctx context.Context, relativePath string) error {
	if err := c.runGit(ctx, "add", "-u", relativePath); err != nil {
		return fmt.Errorf("stage deleted file %s: %w", relativePath, err)
	}
	return nil
}

// Commit creates a commit with the given message.
func (c *Client) Commit(ctx context.Context, message string) error {
	if err := c.runGit(ctx, "commit", "-m", message); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Push pushes the current branch to origin.
func (c *Client) Push(ctx context.Context) error {
	if err := c.runGit(ctx, "push", "origin", "HEAD"); err != nil {
		return fmt.Errorf("push: %w", err)
	}
	return nil
}

// HasUncommittedChanges checks if there are uncommitted changes.
func (c *Client) HasUncommittedChanges(ctx context.Context) (bool, error) {
	output, err := c.runGitOutput(ctx, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("check status: %w", err)
	}
	return len(strings.TrimSpace(output)) > 0, nil
}

// runGit runs a git command and returns any error.
func (c *Client) runGit(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = c.repoPath
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// runGitOutput runs a git command and returns the output.
func (c *Client) runGitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = c.repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
