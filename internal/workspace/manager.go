package workspace

import (
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

	// Check if file is already open
	if file, ok := m.openFiles[path]; ok {
		// Check if file has been modified
		if stat, err := os.Stat(absPath); err == nil {
			if stat.ModTime().Unix() > file.ModTime {
				// File has been modified, reload it
				content, err := os.ReadFile(absPath)
				if err != nil {
					return nil, err
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
		return nil, err
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, err
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

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(absPath, content, 0644); err != nil {
		return err
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return err
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
