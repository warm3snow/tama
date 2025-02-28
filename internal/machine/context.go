package machine

import (
	"os"
	"runtime"
	"sync"
)

// Context represents the machine's operating context
type Context struct {
	OS            string
	Architecture  string
	WorkspacePath string
	Shell         string
	Languages     map[string]float64 // Map of language to percentage
	mu            sync.RWMutex
}

// NewContext creates a new machine context
func NewContext() *Context {
	ctx := &Context{
		OS:            runtime.GOOS,
		Architecture:  runtime.GOARCH,
		WorkspacePath: os.Getenv("PWD"),
		Shell:         os.Getenv("SHELL"),
		Languages:     make(map[string]float64),
	}
	return ctx
}

// GetSystemInfo returns detailed system information
func (c *Context) GetSystemInfo() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]string{
		"os":         c.OS,
		"arch":       c.Architecture,
		"workspace":  c.WorkspacePath,
		"shell":      c.Shell,
		"num_cpu":    string(runtime.NumCPU()),
		"go_version": runtime.Version(),
	}
}

// UpdateLanguages updates the detected languages in the workspace
func (c *Context) UpdateLanguages(languages map[string]float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Languages = languages
}

// GetLanguages returns the detected languages and their percentages
func (c *Context) GetLanguages() map[string]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a copy to avoid map races
	languages := make(map[string]float64, len(c.Languages))
	for k, v := range c.Languages {
		languages[k] = v
	}
	return languages
}

// GetPrimaryLanguage returns the most used language in the workspace
func (c *Context) GetPrimaryLanguage() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var maxLang string
	var maxPercentage float64

	for lang, percentage := range c.Languages {
		if percentage > maxPercentage {
			maxLang = lang
			maxPercentage = percentage
		}
	}

	return maxLang
}
