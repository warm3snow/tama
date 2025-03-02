package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Tool represents a tool that can be executed by the agent
type Tool interface {
	Name() string
	Description() string
	Execute(args map[string]interface{}) (string, error)
}

// Registry manages the available tools
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tools registry
func NewRegistry(enabledTools []string) *Registry {
	registry := &Registry{
		tools: make(map[string]Tool),
	}

	// Register all available tools
	allTools := []Tool{
		&FileReadTool{},
		&FileEditTool{},
		&TerminalRunTool{},
		&TestRunTool{},
		&FileSearchTool{},
		&DirectoryListTool{},
	}

	// Only register enabled tools
	for _, tool := range allTools {
		for _, enabled := range enabledTools {
			if tool.Name() == enabled {
				registry.tools[tool.Name()] = tool
				break
			}
		}
	}

	// If no tools were enabled, register all tools
	if len(registry.tools) == 0 {
		for _, tool := range allTools {
			registry.tools[tool.Name()] = tool
		}
	}

	return registry
}

// GetTool gets a tool by name
func (r *Registry) GetTool(name string) (Tool, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	return tool, nil
}

// ListTools returns a string listing all available tools
func (r *Registry) ListTools() string {
	var sb strings.Builder

	for _, tool := range r.tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description()))
	}

	return sb.String()
}

// FileReadTool implements the file_read tool
type FileReadTool struct{}

func (t *FileReadTool) Name() string {
	return "file_read"
}

func (t *FileReadTool) Description() string {
	return "Reads the contents of a file. Args: {\"path\": \"path/to/file.ext\"}"
}

func (t *FileReadTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(data), nil
}

// FileEditTool implements the file_edit tool
type FileEditTool struct{}

func (t *FileEditTool) Name() string {
	return "file_edit"
}

func (t *FileEditTool) Description() string {
	return "Edits the contents of a file. Args: {\"path\": \"path/to/file.ext\", \"content\": \"new content\"}"
}

func (t *FileEditTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path argument is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content argument is required")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Check if file exists and create a backup
	if _, err := os.Stat(path); err == nil {
		backupPath := path + ".bak." + time.Now().Format("20060102150405")
		if data, err := os.ReadFile(path); err == nil {
			if err := os.WriteFile(backupPath, data, 0644); err != nil {
				// Just log the error, don't fail the operation
				fmt.Fprintf(os.Stderr, "Warning: Failed to create backup: %s\n", err)
			}
		}
	}

	// Write the file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("File %s updated successfully", path), nil
}

// TerminalRunTool implements the terminal_run tool
type TerminalRunTool struct{}

func (t *TerminalRunTool) Name() string {
	return "terminal_run"
}

func (t *TerminalRunTool) Description() string {
	return "Runs a command in the terminal. Args: {\"command\": \"command to run\"}"
}

func (t *TerminalRunTool) Execute(args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command argument is required")
	}

	// Split the command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Create the command
	cmd := exec.Command(parts[0], parts[1:]...)

	// Set working directory if provided
	if workDir, ok := args["working_dir"].(string); ok && workDir != "" {
		cmd.Dir = workDir
	}

	// Set environment variables if provided
	if env, ok := args["env"].(map[string]interface{}); ok {
		for k, v := range env {
			if strVal, ok := v.(string); ok {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, strVal))
			}
		}
	}

	// Capture the output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// TestRunTool implements the test_run tool
type TestRunTool struct{}

func (t *TestRunTool) Name() string {
	return "test_run"
}

func (t *TestRunTool) Description() string {
	return "Runs tests in the project. Args: {\"path\": \"./path/to/package\"}"
}

func (t *TestRunTool) Execute(args map[string]interface{}) (string, error) {
	// Default to running all tests
	path, _ := args["path"].(string)
	if path == "" {
		path = "./..."
	}

	// Create the command
	cmd := exec.Command("go", "test", "-v", path)

	// Capture the output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tests failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// FileSearchTool implements the file_search tool
type FileSearchTool struct{}

func (t *FileSearchTool) Name() string {
	return "file_search"
}

func (t *FileSearchTool) Description() string {
	return "Searches for a pattern in files. Args: {\"pattern\": \"search pattern\", \"dir\": \"./\", \"ext\": \".go\"}"
}

func (t *FileSearchTool) Execute(args map[string]interface{}) (string, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("pattern argument is required")
	}

	// Default to current directory
	dir, _ := args["dir"].(string)
	if dir == "" {
		dir = "."
	}

	// Default to all files
	ext, _ := args["ext"].(string)

	// Use grep command for searching
	var cmd *exec.Cmd
	if ext == "" {
		cmd = exec.Command("grep", "-r", "--include=*", pattern, dir)
	} else {
		cmd = exec.Command("grep", "-r", "--include=*"+ext, pattern, dir)
	}

	// Capture the output
	output, err := cmd.CombinedOutput()
	if err != nil {
		// grep returns non-zero if no matches are found, which is not an error for us
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "No matches found", nil
		}
		return "", fmt.Errorf("search failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// DirectoryListTool implements the dir_list tool
type DirectoryListTool struct{}

func (t *DirectoryListTool) Name() string {
	return "dir_list"
}

func (t *DirectoryListTool) Description() string {
	return "Lists files in a directory. Args: {\"dir\": \"./\", \"pattern\": \"*.go\"}"
}

func (t *DirectoryListTool) Execute(args map[string]interface{}) (string, error) {
	// Default to current directory
	dir, _ := args["dir"].(string)
	if dir == "" {
		dir = "."
	}

	// Get pattern if provided
	pattern, _ := args["pattern"].(string)

	// Use ls command for listing
	var cmd *exec.Cmd
	if pattern == "" {
		cmd = exec.Command("ls", "-la", dir)
	} else {
		cmd = exec.Command("ls", "-la", filepath.Join(dir, pattern))
	}

	// Capture the output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("listing failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}
