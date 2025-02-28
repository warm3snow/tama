package tools

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// LinterTool implements code linting functionality
type LinterTool struct {
	workspacePath string
}

// NewLinterTool creates a new linter tool
func NewLinterTool(workspacePath string) *LinterTool {
	return &LinterTool{
		workspacePath: workspacePath,
	}
}

func (t *LinterTool) Name() string {
	return "linter"
}

func (t *LinterTool) Description() string {
	return "Check and fix code issues using linters"
}

func (t *LinterTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract arguments
	operation, ok := args["operation"].(string)
	if !ok {
		return "", fmt.Errorf("operation argument required")
	}

	// Optional arguments
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	switch operation {
	case "check":
		return t.checkCode(ctx, path)
	case "fix":
		return t.fixCode(ctx, path)
	default:
		return "", fmt.Errorf("unknown operation: %s", operation)
	}
}

// checkCode runs linters to check the code
func (t *LinterTool) checkCode(ctx context.Context, path string) (string, error) {
	fullPath := filepath.Join(t.workspacePath, path)

	// Run golangci-lint for Go files
	if isGoFile(path) {
		cmd := exec.CommandContext(ctx, "golangci-lint", "run", "--out-format=line-number", fullPath)
		cmd.Dir = t.workspacePath
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Don't return error as it might just be linter findings
			return string(output), nil
		}
		return "No issues found", nil
	}

	// Add more language-specific linters here
	return "", fmt.Errorf("no linter available for this file type")
}

// fixCode attempts to automatically fix linter issues
func (t *LinterTool) fixCode(ctx context.Context, path string) (string, error) {
	fullPath := filepath.Join(t.workspacePath, path)

	// Fix Go files
	if isGoFile(path) {
		// Run gofmt
		gofmtCmd := exec.CommandContext(ctx, "gofmt", "-w", fullPath)
		if output, err := gofmtCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("gofmt failed: %v\n%s", err, output)
		}

		// Run golangci-lint with --fix flag
		lintCmd := exec.CommandContext(ctx, "golangci-lint", "run", "--fix", fullPath)
		lintCmd.Dir = t.workspacePath
		if output, err := lintCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("golangci-lint fix failed: %v\n%s", err, output)
		}

		return "Fixed code style issues", nil
	}

	// Add more language-specific fixers here
	return "", fmt.Errorf("no fixer available for this file type")
}

// isGoFile checks if the file is a Go source file
func isGoFile(path string) bool {
	return strings.HasSuffix(path, ".go")
}
