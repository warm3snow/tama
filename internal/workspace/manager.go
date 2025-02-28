package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Manager handles workspace operations and state
type Manager struct {
	root      string
	mu        sync.RWMutex
	openFiles map[string]*File
}

// File represents a workspace file
type File struct {
	Path    string
	Content []byte
	ModTime int64
}

// NewManager creates a new workspace manager
func NewManager() *Manager {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	return &Manager{
		root:      wd,
		openFiles: make(map[string]*File),
	}
}

// GetSummary returns a summary of the workspace state
func (m *Manager) GetSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := make([]string, 0, len(m.openFiles))
	for path := range m.openFiles {
		files = append(files, path)
	}

	return map[string]interface{}{
		"root":       m.root,
		"open_files": files,
	}
}

// ReadFile reads a file from the workspace
func (m *Manager) ReadFile(path string) (*File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	absPath := filepath.Join(m.root, path)

	// Check if path is within workspace
	if !filepath.HasPrefix(absPath, m.root) {
		return nil, fmt.Errorf("path is outside workspace: %s", path)
	}

	// Check if file is already open
	if file, ok := m.openFiles[path]; ok {
		// Check if file has been modified
		if stat, err := os.Stat(absPath); err == nil {
			if stat.ModTime().Unix() > file.ModTime {
				// File has been modified, reload it
				content, err := os.ReadFile(absPath)
				if err != nil {
					if os.IsNotExist(err) {
						return nil, fmt.Errorf("file does not exist: %s", path)
					}
					if os.IsPermission(err) {
						return nil, fmt.Errorf("permission denied: %s", path)
					}
					return nil, fmt.Errorf("failed to read file: %v", err)
				}
				file.Content = content
				file.ModTime = stat.ModTime().Unix()
			}
		}
		return file, nil
	}

	// Read new file
	content, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %s", path)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied: %s", path)
		}
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	file := &File{
		Path:    path,
		Content: content,
		ModTime: stat.ModTime().Unix(),
	}

	m.openFiles[path] = file
	return file, nil
}

// WriteFile writes content to a file in the workspace
func (m *Manager) WriteFile(path string, content []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	absPath := filepath.Join(m.root, path)

	// Check if path is within workspace
	if !filepath.HasPrefix(absPath, m.root) {
		return fmt.Errorf("path is outside workspace: %s", path)
	}

	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied creating directories: %s", filepath.Dir(path))
		}
		return fmt.Errorf("failed to create directories: %v", err)
	}

	// Write file
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied writing file: %s", path)
		}
		return fmt.Errorf("failed to write file: %v", err)
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	m.openFiles[path] = &File{
		Path:    path,
		Content: content,
		ModTime: stat.ModTime().Unix(),
	}

	return nil
}

// Cleanup performs any necessary cleanup
func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.openFiles = make(map[string]*File)
}

// GetWorkspacePath returns the root path of the workspace
func (m *Manager) GetWorkspacePath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.root
}

// SetWorkspacePath sets the workspace root path
func (m *Manager) SetWorkspacePath(path string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Check if path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", absPath)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: %s", absPath)
		}
		return fmt.Errorf("failed to access directory: %v", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Check if directory is readable
	f, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("failed to open directory: %v", err)
	}
	f.Close()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing open files
	m.openFiles = make(map[string]*File)

	// Set new root path
	m.root = absPath
	return nil
}
