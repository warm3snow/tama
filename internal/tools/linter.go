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
	return "Check and fix high priority code issues using linters"
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

	// Get severity level, default to high priority only
	severity, _ := args["severity"].(string)
	if severity == "" {
		severity = "high"
	}

	switch operation {
	case "check":
		return t.checkCode(ctx, path, severity)
	case "fix":
		return t.fixCode(ctx, path, severity)
	default:
		return "", fmt.Errorf("unknown operation: %s", operation)
	}
}

// checkCode runs linters to check the code
func (t *LinterTool) checkCode(ctx context.Context, path string, severity string) (string, error) {
	fullPath := filepath.Join(t.workspacePath, path)

	// Run golangci-lint for Go files
	if isGoFile(path) {
		args := []string{"run", "--out-format=line-number"}

		// Add severity filter
		switch severity {
		case "high":
			args = append(args, "--severity=error")
		case "medium":
			args = append(args, "--severity=warning")
		case "low":
			args = append(args, "--severity=info")
		}

		args = append(args, fullPath)
		cmd := exec.CommandContext(ctx, "golangci-lint", args...)
		cmd.Dir = t.workspacePath
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Don't return error as it might just be linter findings
			return string(output), nil
		}
		return "No high priority issues found", nil
	}

	// Add more language-specific linters here
	return "", fmt.Errorf("no linter available for this file type")
}

// fixCode attempts to automatically fix linter issues
func (t *LinterTool) fixCode(ctx context.Context, path string, severity string) (string, error) {
	fullPath := filepath.Join(t.workspacePath, path)

	// Fix Go files
	if isGoFile(path) {
		// Run gofmt
		gofmtCmd := exec.CommandContext(ctx, "gofmt", "-w", fullPath)
		if output, err := gofmtCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("gofmt failed: %v\n%s", err, output)
		}

		// Run golangci-lint with --fix flag and severity filter
		args := []string{"run", "--fix"}

		// Add severity filter
		switch severity {
		case "high":
			args = append(args, "--severity=error")
		case "medium":
			args = append(args, "--severity=warning")
		case "low":
			args = append(args, "--severity=info")
		}

		args = append(args, fullPath)
		lintCmd := exec.CommandContext(ctx, "golangci-lint", args...)
		lintCmd.Dir = t.workspacePath
		if output, err := lintCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("golangci-lint fix failed: %v\n%s", err, output)
		}

		return "Fixed high priority code issues", nil
	}

	// Add more language-specific fixers here
	return "", fmt.Errorf("no fixer available for this file type")
}

// isGoFile checks if the file is a Go source file
func isGoFile(path string) bool {
	return strings.HasSuffix(path, ".go")
}
