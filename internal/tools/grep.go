package tools

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GrepSearchTool implements code search functionality
type GrepSearchTool struct {
	workspacePath string
}

// NewGrepSearchTool creates a new grep search tool
func NewGrepSearchTool(workspacePath string) *GrepSearchTool {
	return &GrepSearchTool{
		workspacePath: workspacePath,
	}
}

func (t *GrepSearchTool) Name() string {
	return "grep_search"
}

func (t *GrepSearchTool) Description() string {
	return "Search for patterns in files using grep"
}

func (t *GrepSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract arguments
	pattern, ok := args["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("pattern argument required")
	}

	// Optional arguments
	includePattern, _ := args["include"].(string)
	excludePattern, _ := args["exclude"].(string)
	caseSensitive, _ := args["case_sensitive"].(bool)

	// Build grep command
	cmd := exec.CommandContext(ctx, "grep", "-r", "-n")

	if !caseSensitive {
		cmd.Args = append(cmd.Args, "-i")
	}

	if includePattern != "" {
		cmd.Args = append(cmd.Args, "--include", includePattern)
	}

	if excludePattern != "" {
		cmd.Args = append(cmd.Args, "--exclude", excludePattern)
	}

	cmd.Args = append(cmd.Args, pattern, t.workspacePath)
	cmd.Dir = t.workspacePath

	// Run grep command
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Exit code 1 means no matches found
			return "No matches found", nil
		}
		return "", fmt.Errorf("search failed: %v", err)
	}

	// Process and format results
	results := strings.Split(string(output), "\n")
	if len(results) > 50 {
		results = results[:50]
		results = append(results, "... (results truncated)")
	}

	// Make paths relative to workspace
	for i, result := range results {
		if result == "" {
			continue
		}
		parts := strings.SplitN(result, ":", 2)
		if len(parts) > 1 {
			relPath, err := filepath.Rel(t.workspacePath, parts[0])
			if err == nil {
				results[i] = relPath + ":" + parts[1]
			}
		}
	}

	return strings.Join(results, "\n"), nil
}
