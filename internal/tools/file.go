package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ReadFileTool implements file reading functionality
type ReadFileTool struct {
	workspacePath string
}

// NewReadFileTool creates a new read file tool
func NewReadFileTool(workspacePath string) *ReadFileTool {
	return &ReadFileTool{
		workspacePath: workspacePath,
	}
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file in the workspace"
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract arguments
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument required")
	}

	// Resolve full path
	fullPath := filepath.Join(t.workspacePath, path)

	// Ensure path is within workspace
	if !filepath.HasPrefix(fullPath, t.workspacePath) {
		return "", fmt.Errorf("path must be within workspace")
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	return string(content), nil
}
