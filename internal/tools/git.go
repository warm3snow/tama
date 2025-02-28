package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	// First check if there are any changes
	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = t.workspacePath
	status, err := statusCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git status failed: %v", err)
	}

	// Process status output to show file states
	var result strings.Builder
	if len(status) > 0 {
		result.WriteString("\nChanged files:\n")
		for _, line := range strings.Split(string(status), "\n") {
			if len(line) < 3 {
				continue
			}
			state := line[:2]
			file := strings.TrimSpace(line[3:])
			switch state {
			case "M ":
				result.WriteString(fmt.Sprintf("  Modified:   %s\n", file))
			case " M":
				result.WriteString(fmt.Sprintf("  Modified (unstaged): %s\n", file))
			case "A ":
				result.WriteString(fmt.Sprintf("  Added:      %s\n", file))
			case "D ":
				result.WriteString(fmt.Sprintf("  Deleted:    %s\n", file))
			case "R ":
				result.WriteString(fmt.Sprintf("  Renamed:    %s\n", file))
			case "C ":
				result.WriteString(fmt.Sprintf("  Copied:     %s\n", file))
			case "??":
				result.WriteString(fmt.Sprintf("  Untracked:  %s\n", file))
			}
		}
		result.WriteString("\n")
	} else {
		return "No changes detected", nil
	}

	// Get both staged and unstaged changes
	cmd := exec.CommandContext(ctx, "git", "diff", "--color")
	cmd.Dir = t.workspacePath

	// Capture both stdout and stderr
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git diff failed: %s", stderr.String())
		}
		return "", fmt.Errorf("git diff failed: %v", err)
	}

	// Get staged changes
	stagedCmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--color")
	stagedCmd.Dir = t.workspacePath

	var stagedOut strings.Builder
	stagedCmd.Stdout = &stagedOut
	stagedCmd.Stderr = &stderr

	err = stagedCmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git diff --cached failed: %s", stderr.String())
		}
		return "", fmt.Errorf("git diff --cached failed: %v", err)
	}

	// Show staged changes
	if stagedOut.Len() > 0 {
		result.WriteString("\nStaged changes:\n")
		result.WriteString(stagedOut.String())
	}

	// Show unstaged changes
	if stdout.Len() > 0 {
		result.WriteString("\nUnstaged changes:\n")
		result.WriteString(stdout.String())
	}

	// Get untracked files content
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "??") {
			file := strings.TrimSpace(line[3:])
			content, err := os.ReadFile(filepath.Join(t.workspacePath, file))
			if err == nil {
				result.WriteString(fmt.Sprintf("\nNew file: %s\n", file))
				result.WriteString(string(content))
				result.WriteString("\n")
			}
		}
	}

	return result.String(), nil
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
