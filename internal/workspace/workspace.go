package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Manager manages the workspace
type Manager struct {
	workingDir string
	cache      map[string]cacheEntry
}

// cacheEntry represents a cached file content
type cacheEntry struct {
	content   string
	timestamp time.Time
}

// NewManager creates a new workspace manager
func NewManager() *Manager {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	return &Manager{
		workingDir: wd,
		cache:      make(map[string]cacheEntry),
	}
}

// AnalyzeWorkspace analyzes the workspace and returns a context string
func (m *Manager) AnalyzeWorkspace() (string, error) {
	var sb strings.Builder

	// Get the project structure
	structure, err := m.getProjectStructure()
	if err != nil {
		return "", fmt.Errorf("failed to get project structure: %w", err)
	}

	sb.WriteString("Project Structure:\n")
	sb.WriteString(structure)
	sb.WriteString("\n")

	// Get the Go module info
	moduleInfo, err := m.getGoModuleInfo()
	if err == nil {
		sb.WriteString("Go Module Info:\n")
		sb.WriteString(moduleInfo)
		sb.WriteString("\n")
	}

	// Get summary of key files
	summary, err := m.getKeySummary()
	if err == nil && summary != "" {
		sb.WriteString("Key Files Summary:\n")
		sb.WriteString(summary)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// getProjectStructure returns a string representation of the project structure
func (m *Manager) getProjectStructure() (string, error) {
	var sb strings.Builder

	err := filepath.Walk(m.workingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip vendor directory
		if info.IsDir() && filepath.Base(path) == "vendor" {
			return filepath.SkipDir
		}

		// Get the relative path
		relPath, err := filepath.Rel(m.workingDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory
		if relPath == "." {
			return nil
		}

		// Add indentation based on the directory depth
		depth := strings.Count(relPath, string(os.PathSeparator))
		indent := strings.Repeat("  ", depth)

		// Add the file or directory to the structure
		if info.IsDir() {
			sb.WriteString(fmt.Sprintf("%s- %s/\n", indent, filepath.Base(path)))
		} else {
			sb.WriteString(fmt.Sprintf("%s- %s\n", indent, filepath.Base(path)))
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return sb.String(), nil
}

// getGoModuleInfo returns information about the Go module
func (m *Manager) getGoModuleInfo() (string, error) {
	// Check if go.mod exists
	goModPath := filepath.Join(m.workingDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return "", fmt.Errorf("go.mod not found")
	}

	// Read the go.mod file
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	return string(data), nil
}

// getKeySummary returns a summary of key files in the project
func (m *Manager) getKeySummary() (string, error) {
	var sb strings.Builder

	// List of key files to summarize
	keyFiles := []string{
		"main.go",
		"README.md",
		"tama.yaml",
	}

	for _, file := range keyFiles {
		content, err := m.ReadFile(file)
		if err == nil {
			// Add a brief summary (first few lines)
			lines := strings.Split(content, "\n")
			summary := lines
			if len(lines) > 5 {
				summary = lines[:5]
			}

			sb.WriteString(fmt.Sprintf("File: %s\n", file))
			sb.WriteString(fmt.Sprintf("  Summary: %s\n", strings.Join(summary, "\n  ")))
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// ReadFile reads a file from the workspace
func (m *Manager) ReadFile(path string) (string, error) {
	// Resolve the absolute path
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(m.workingDir, path)
	}

	// Check if the file is in cache and not too old
	if entry, ok := m.cache[absPath]; ok {
		if time.Since(entry.timestamp) < 5*time.Second {
			return entry.content, nil
		}
	}

	// Read the file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Cache the content
	m.cache[absPath] = cacheEntry{
		content:   string(data),
		timestamp: time.Now(),
	}

	return string(data), nil
}

// WriteFile writes a file to the workspace
func (m *Manager) WriteFile(path string, content string) error {
	// Resolve the absolute path
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(m.workingDir, path)
	}

	// Create the directory if it doesn't exist
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the file
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Update the cache
	m.cache[absPath] = cacheEntry{
		content:   content,
		timestamp: time.Now(),
	}

	return nil
}

// ListFiles lists files in a directory with optional pattern matching
func (m *Manager) ListFiles(dir string, pattern string) ([]string, error) {
	// Resolve the absolute path
	absPath := dir
	if !filepath.IsAbs(dir) {
		absPath = filepath.Join(m.workingDir, dir)
	}

	// Check if the directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	// Read the directory
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Filter entries based on the pattern
	var files []string
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Apply pattern matching if provided
		if pattern != "" && !strings.Contains(name, pattern) {
			continue
		}

		// Add to the list
		files = append(files, name)
	}

	return files, nil
}

// SearchInFiles searches for a pattern in files
func (m *Manager) SearchInFiles(pattern string, dir string, fileExt string) (map[string][]string, error) {
	results := make(map[string][]string)

	// Resolve the absolute path
	absPath := dir
	if !filepath.IsAbs(dir) {
		absPath = filepath.Join(m.workingDir, dir)
	}

	// Walk the directory
	err := filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip files that don't match the extension
		if fileExt != "" && !strings.HasSuffix(path, fileExt) {
			return nil
		}

		// Read the file
		content, err := m.ReadFile(path)
		if err != nil {
			return nil
		}

		// Search for the pattern
		lines := strings.Split(content, "\n")
		var matches []string

		for i, line := range lines {
			if strings.Contains(line, pattern) {
				// Add context (line number and content)
				matches = append(matches, fmt.Sprintf("Line %d: %s", i+1, line))
			}
		}

		// Add to results if there are matches
		if len(matches) > 0 {
			relPath, _ := filepath.Rel(m.workingDir, path)
			results[relPath] = matches
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return results, nil
}
