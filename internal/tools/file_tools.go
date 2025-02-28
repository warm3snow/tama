package tools

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// FileSystemTool provides file system operations
type FileSystemTool struct {
	workspacePath string
	backupPath    string
}

// NewFileSystemTool creates a new file system tool
func NewFileSystemTool(workspacePath string) *FileSystemTool {
	backupPath := filepath.Join(workspacePath, ".tama", "backups")
	return &FileSystemTool{
		workspacePath: workspacePath,
		backupPath:    backupPath,
	}
}

func (t *FileSystemTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	operation, ok := args["operation"].(string)
	if !ok {
		return "", fmt.Errorf("operation not specified")
	}

	switch operation {
	case "write":
		return t.writeFile(args)
	case "read":
		return t.readFile(args)
	case "backup":
		return t.createBackup(args)
	case "restore":
		return t.restoreBackup(args)
	default:
		return "", fmt.Errorf("unknown operation: %s", operation)
	}
}

func (t *FileSystemTool) writeFile(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not specified")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content not specified")
	}

	fullPath := filepath.Join(t.workspacePath, path)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	// Write file
	if err := ioutil.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
}

func (t *FileSystemTool) readFile(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not specified")
	}

	fullPath := filepath.Join(t.workspacePath, path)
	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	return string(content), nil
}

func (t *FileSystemTool) createBackup(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not specified")
	}

	// Create backup directory with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupDir := filepath.Join(t.backupPath, timestamp)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %v", err)
	}

	// Copy file to backup
	srcPath := filepath.Join(t.workspacePath, path)
	dstPath := filepath.Join(backupDir, path)

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create backup subdirectory: %v", err)
	}

	// Read source file
	content, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to read source file: %v", err)
	}

	// Write to backup
	if err := ioutil.WriteFile(dstPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup file: %v", err)
	}

	return dstPath, nil
}

func (t *FileSystemTool) restoreBackup(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not specified")
	}

	backupPath, ok := args["backup_path"].(string)
	if !ok {
		return "", fmt.Errorf("backup_path not specified")
	}

	// Read backup file
	content, err := ioutil.ReadFile(backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to read backup file: %v", err)
	}

	// Restore to original location
	destPath := filepath.Join(t.workspacePath, path)
	if err := ioutil.WriteFile(destPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to restore file: %v", err)
	}

	return fmt.Sprintf("Successfully restored %s from backup", path), nil
}

// Description returns the tool description
func (t *FileSystemTool) Description() string {
	return "Provides file system operations (read, write, backup, restore)"
}

// Name returns the tool name
func (t *FileSystemTool) Name() string {
	return "filesystem"
}
