package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) (string, func()) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "git-tool-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git
	configCmd := exec.Command("git", "config", "user.name", "Test User")
	configCmd.Dir = tmpDir
	if err := configCmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to configure git: %v", err)
	}

	configCmd = exec.Command("git", "config", "user.email", "test@example.com")
	configCmd.Dir = tmpDir
	if err := configCmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to configure git: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create test file: %v", err)
	}

	addCmd := exec.Command("git", "add", "test.txt")
	addCmd.Dir = tmpDir
	if err := addCmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to add test file: %v", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "Initial commit")
	commitCmd.Dir = tmpDir
	if err := commitCmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestGitTool_Execute(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	tool := NewGitTool(tmpDir)
	ctx := context.Background()

	// Test cases
	tests := []struct {
		name      string
		setup     func() error
		args      map[string]interface{}
		wantErr   bool
		checkFunc func(t *testing.T, got string, err error)
	}{
		{
			name:    "missing operation",
			args:    map[string]interface{}{},
			wantErr: true,
			checkFunc: func(t *testing.T, got string, err error) {
				if err == nil || !strings.Contains(err.Error(), "operation argument required") {
					t.Errorf("Expected operation required error, got %v", err)
				}
			},
		},
		{
			name: "unknown operation",
			args: map[string]interface{}{
				"operation": "unknown",
			},
			wantErr: true,
			checkFunc: func(t *testing.T, got string, err error) {
				if err == nil || !strings.Contains(err.Error(), "unknown git operation") {
					t.Errorf("Expected unknown operation error, got %v", err)
				}
			},
		},
		{
			name: "diff with no changes",
			args: map[string]interface{}{
				"operation": "diff",
			},
			checkFunc: func(t *testing.T, got string, err error) {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if got != "No changes detected" {
					t.Errorf("Expected 'No changes detected', got %q", got)
				}
			},
		},
		{
			name: "diff with changes",
			setup: func() error {
				return os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("modified content"), 0644)
			},
			args: map[string]interface{}{
				"operation": "diff",
			},
			checkFunc: func(t *testing.T, got string, err error) {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if !strings.Contains(got, "Modified") {
					t.Errorf("Expected diff to contain 'Modified', got %q", got)
				}
			},
		},
		{
			name: "commit changes",
			setup: func() error {
				return os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("commit test content"), 0644)
			},
			args: map[string]interface{}{
				"operation": "commit",
				"message":   "Test commit",
			},
			checkFunc: func(t *testing.T, got string, err error) {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if !strings.Contains(got, "Test commit") {
					t.Errorf("Expected commit output to contain commit message, got %q", got)
				}
				if !strings.Contains(got, "1 file changed") {
					t.Errorf("Expected commit output to contain file change info, got %q", got)
				}
			},
		},
		{
			name: "reset changes",
			setup: func() error {
				return os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("reset test content"), 0644)
			},
			args: map[string]interface{}{
				"operation": "reset",
			},
			checkFunc: func(t *testing.T, got string, err error) {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				// Check if file was reset
				content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
				if err != nil {
					t.Errorf("Failed to read test file: %v", err)
				}
				if string(content) == "reset test content" {
					t.Error("File content should have been reset")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}

			got, err := tool.Execute(ctx, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, got, err)
			}
		})
	}
}

func TestGitTool_Name(t *testing.T) {
	tool := NewGitTool("")
	if got := tool.Name(); got != "git" {
		t.Errorf("Name() = %v, want %v", got, "git")
	}
}

func TestGitTool_Description(t *testing.T) {
	tool := NewGitTool("")
	if got := tool.Description(); got == "" {
		t.Error("Description() should not return empty string")
	}
}

func TestGitTool_getDiff(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	tool := NewGitTool(tmpDir)
	ctx := context.Background()

	// Test initial state
	diff, err := tool.getDiff(ctx)
	if err != nil {
		t.Fatalf("getDiff() error = %v", err)
	}
	if diff != "No changes detected" {
		t.Errorf("Expected no changes, got %q", diff)
	}

	// Test with modified file
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("modified for diff test"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	diff, err = tool.getDiff(ctx)
	if err != nil {
		t.Fatalf("getDiff() error = %v", err)
	}
	if !strings.Contains(diff, "Modified") {
		t.Errorf("Expected diff to contain 'Modified', got %q", diff)
	}

	// Test with new file
	if err := os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("new file content"), 0644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	diff, err = tool.getDiff(ctx)
	if err != nil {
		t.Fatalf("getDiff() error = %v", err)
	}
	if !strings.Contains(diff, "Untracked") {
		t.Errorf("Expected diff to contain 'Untracked', got %q", diff)
	}
}

func TestGitTool_commit(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	tool := NewGitTool(tmpDir)
	ctx := context.Background()

	// Test commit with no changes
	_, err := tool.commit(ctx, "test commit")
	if err == nil {
		t.Error("Expected error when committing with no changes")
	}

	// Test commit with changes
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("commit test"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	output, err := tool.commit(ctx, "test commit")
	if err != nil {
		t.Fatalf("commit() error = %v", err)
	}
	if !strings.Contains(output, "test commit") {
		t.Errorf("Expected commit output to contain commit message, got %q", output)
	}
	if !strings.Contains(output, "1 file changed") {
		t.Errorf("Expected commit output to contain file change info, got %q", output)
	}

	// Verify commit
	logCmd := exec.Command("git", "log", "-1", "--pretty=format:%s")
	logCmd.Dir = tmpDir
	logOutput, err := logCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get git log: %v", err)
	}
	if string(logOutput) != "test commit" {
		t.Errorf("Expected commit message 'test commit', got %q", string(logOutput))
	}
}

func TestGitTool_reset(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	tool := NewGitTool(tmpDir)
	ctx := context.Background()

	// Create some changes
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("reset test"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Reset changes
	output, err := tool.reset(ctx)
	if err != nil {
		t.Fatalf("reset() error = %v", err)
	}
	if !strings.Contains(output, "HEAD") {
		t.Errorf("Expected reset output to contain 'HEAD', got %q", output)
	}

	// Verify reset
	content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) == "reset test" {
		t.Error("File content should have been reset")
	}
}
