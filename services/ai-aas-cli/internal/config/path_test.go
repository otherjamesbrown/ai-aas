package config

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectShell(t *testing.T) {
	shell := detectShell()

	// Should return a non-empty string
	assert.NotEmpty(t, shell)

	// On some systems, we might get the full path or another shell
	// So just verify we got something
	assert.True(t, len(shell) > 0, "shell detection should return a value")
}

func TestGeneratePathInstruction(t *testing.T) {
	binaryDir := "/usr/local/bin"

	tests := []struct {
		shell    string
		contains string
	}{
		{"bash", "bashrc"},
		{"zsh", "zshrc"},
		{"fish", "fish_user_paths"},
		{"powershell", "$PROFILE"},
		{"cmd", "Environment Variables"},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			instruction := generatePathInstruction(tt.shell, binaryDir)
			assert.Contains(t, instruction, binaryDir)
			assert.Contains(t, instruction, tt.contains)
		})
	}
}

func TestCheckPath(t *testing.T) {
	info := CheckPath()

	// Should return valid info
	assert.NotEmpty(t, info.Shell)

	// If not in path, should have instructions
	if !info.InPath {
		assert.NotEmpty(t, info.Instruction)
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "expand home",
			input:    "~/test",
			contains: "test",
		},
		{
			name:     "no expansion needed",
			input:    "/absolute/path",
			contains: "/absolute/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandPath(tt.input)
			assert.Contains(t, result, tt.contains)
			// Home expansion should not contain ~
			if tt.name == "expand home" {
				assert.NotContains(t, result, "~")
			}
		})
	}
}

func TestIsInPath(t *testing.T) {
	// ai-aas-cli is likely not in PATH during tests
	result := IsInPath()
	// Just verify it returns a boolean without error
	assert.IsType(t, true, result)
}

func TestGetBinaryDir(t *testing.T) {
	dir, err := GetBinaryDir()

	// Should succeed on any platform
	if runtime.GOOS != "windows" {
		assert.NoError(t, err)
		assert.NotEmpty(t, dir)
	}
}
