// Package config provides configuration management for the CLI.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// PathInfo contains information about the CLI's PATH status
type PathInfo struct {
	InPath      bool
	BinaryPath  string
	Shell       string
	Instruction string
}

// CheckPath checks if the CLI binary is in the user's PATH
func CheckPath() PathInfo {
	info := PathInfo{
		Shell: detectShell(),
	}

	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		info.BinaryPath = "ai-aas-cli"
	} else {
		info.BinaryPath = execPath
	}

	// Check if we can find ai-aas-cli in PATH
	which, err := exec.LookPath("ai-aas-cli")
	if err == nil {
		info.InPath = true
		// Verify it's the same binary
		if which != info.BinaryPath {
			info.BinaryPath = which
		}
	} else {
		info.InPath = false
		info.Instruction = generatePathInstruction(info.Shell, filepath.Dir(info.BinaryPath))
	}

	return info
}

// detectShell attempts to detect the user's shell
func detectShell() string {
	// Check SHELL environment variable
	shell := os.Getenv("SHELL")
	if shell != "" {
		return filepath.Base(shell)
	}

	// Windows detection
	if runtime.GOOS == "windows" {
		// Check if running in PowerShell
		if os.Getenv("PSModulePath") != "" {
			return "powershell"
		}
		return "cmd"
	}

	return "bash" // Default fallback
}

// generatePathInstruction generates shell-specific PATH instructions
func generatePathInstruction(shell, binaryDir string) string {
	switch shell {
	case "bash":
		return fmt.Sprintf(`Add the following to your ~/.bashrc or ~/.bash_profile:

  export PATH="$PATH:%s"

Then run:
  source ~/.bashrc`, binaryDir)

	case "zsh":
		return fmt.Sprintf(`Add the following to your ~/.zshrc:

  export PATH="$PATH:%s"

Then run:
  source ~/.zshrc`, binaryDir)

	case "fish":
		return fmt.Sprintf(`Run the following command:

  set -Ua fish_user_paths %s`, binaryDir)

	case "powershell":
		return fmt.Sprintf(`Add the following to your $PROFILE:

  $env:PATH += ";%s"

Or add it permanently via:
  [Environment]::SetEnvironmentVariable("PATH", $env:PATH + ";%s", "User")`, binaryDir, binaryDir)

	case "cmd":
		return fmt.Sprintf(`Add %s to your PATH:

1. Open System Properties > Advanced > Environment Variables
2. Under User variables, select PATH and click Edit
3. Add: %s
4. Click OK and restart your terminal`, binaryDir, binaryDir)

	default:
		return fmt.Sprintf(`Add %s to your PATH environment variable`, binaryDir)
	}
}

// IsInPath returns true if the CLI is in PATH
func IsInPath() bool {
	_, err := exec.LookPath("ai-aas-cli")
	return err == nil
}

// GetBinaryDir returns the directory containing the CLI binary
func GetBinaryDir() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}
	return filepath.Dir(execPath), nil
}

// PrintPathInstructions prints PATH setup instructions to stdout
func PrintPathInstructions() {
	info := CheckPath()
	if info.InPath {
		fmt.Printf("✓ ai-aas-cli is in your PATH at: %s\n", info.BinaryPath)
		return
	}

	fmt.Println("⚠ ai-aas-cli is not in your PATH")
	fmt.Println()
	fmt.Printf("Detected shell: %s\n", info.Shell)
	fmt.Println()
	fmt.Println(info.Instruction)
}

// ExpandPath expands ~ and environment variables in a path
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	return os.ExpandEnv(path)
}

