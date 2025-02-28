package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// RunTerminalTool implements terminal command execution functionality
type RunTerminalTool struct {
	workspacePath string
}

// NewRunTerminalTool creates a new terminal command execution tool
func NewRunTerminalTool(workspacePath string) *RunTerminalTool {
	return &RunTerminalTool{
		workspacePath: workspacePath,
	}
}

func (t *RunTerminalTool) Name() string {
	return "run_terminal"
}

func (t *RunTerminalTool) Description() string {
	return "Execute a terminal command in the workspace"
}

func (t *RunTerminalTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract arguments
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command argument required")
	}

	// Optional arguments
	background, _ := args["background"].(bool)

	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Create command
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = t.workspacePath

	// Run command
	if background {
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("failed to start command: %v", err)
		}
		return fmt.Sprintf("Started command in background: %s", command), nil
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %v\nOutput: %s", err, string(output))
	}

	return string(output), nil
}
