package tools

import (
	"context"
	"fmt"
	"os/exec"
)

// GitTool implements git operations
type GitTool struct {
	workspacePath string
}

// NewGitTool creates a new git tool
func NewGitTool(workspacePath string) *GitTool {
	return &GitTool{
		workspacePath: workspacePath,
	}
}

func (t *GitTool) Name() string {
	return "git"
}

func (t *GitTool) Description() string {
	return "Execute git operations in the workspace"
}

func (t *GitTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract arguments
	operation, ok := args["operation"].(string)
	if !ok {
		return "", fmt.Errorf("operation argument required")
	}

	switch operation {
	case "diff":
		return t.getDiff(ctx)
	case "commit":
		message, _ := args["message"].(string)
		return t.commit(ctx, message)
	case "reset":
		return t.reset(ctx)
	default:
		return "", fmt.Errorf("unknown git operation: %s", operation)
	}
}

// getDiff returns the current changes in the workspace
func (t *GitTool) getDiff(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff")
	cmd.Dir = t.workspacePath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %v", err)
	}

	return string(output), nil
}

// commit stages and commits all changes
func (t *GitTool) commit(ctx context.Context, message string) (string, error) {
	if message == "" {
		message = "Auto commit by Tama"
	}

	// Stage all changes
	stageCmd := exec.CommandContext(ctx, "git", "add", ".")
	stageCmd.Dir = t.workspacePath
	if err := stageCmd.Run(); err != nil {
		return "", fmt.Errorf("git add failed: %v", err)
	}

	// Commit changes
	commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	commitCmd.Dir = t.workspacePath
	output, err := commitCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git commit failed: %v", err)
	}

	return string(output), nil
}

// reset discards all uncommitted changes
func (t *GitTool) reset(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "reset", "--hard", "HEAD")
	cmd.Dir = t.workspacePath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git reset failed: %v", err)
	}

	return string(output), nil
}
