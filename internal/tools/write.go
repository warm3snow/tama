package tools

import (
	"context"
	"fmt"

	"github.com/warm3snow/tama/internal/workspace"
)

// FileWriteTool implements the file writing tool
type FileWriteTool struct {
	workspace *workspace.Manager
}

// NewFileWriteTool creates a new file writing tool
func NewFileWriteTool(ws *workspace.Manager) *FileWriteTool {
	return &FileWriteTool{
		workspace: ws,
	}
}

// Name returns the tool name
func (t *FileWriteTool) Name() string {
	return "write_file"
}

// Description returns the tool description
func (t *FileWriteTool) Description() string {
	return "Write content to a file in the workspace"
}

// Execute runs the file writing operation
func (t *FileWriteTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get path from arguments
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	// Get content from arguments
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content argument is required")
	}

	// Write file to workspace
	err := t.workspace.WriteFile(path, []byte(content))
	if err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
}
