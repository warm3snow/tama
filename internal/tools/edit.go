package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// EditFileTool implements file editing functionality
type EditFileTool struct {
	workspacePath string
}

// NewEditFileTool creates a new edit file tool
func NewEditFileTool(workspacePath string) *EditFileTool {
	return &EditFileTool{
		workspacePath: workspacePath,
	}
}

func (t *EditFileTool) Name() string {
	return "edit_file"
}

func (t *EditFileTool) Description() string {
	return "Edit the contents of a file in the workspace"
}

func (t *EditFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract arguments
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content argument required")
	}

	// Resolve full path
	fullPath := filepath.Join(t.workspacePath, path)

	// Ensure path is within workspace
	if !filepath.HasPrefix(fullPath, t.workspacePath) {
		return "", fmt.Errorf("path must be within workspace")
	}

	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %v", err)
	}

	// Write file content
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return fmt.Sprintf("Successfully edited file: %s", path), nil
}
