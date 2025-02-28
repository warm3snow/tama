package machine

import (
	"os"
	"runtime"
)

// Context represents the machine's operating context
type Context struct {
	OS            string
	Architecture  string
	WorkspacePath string
	Shell         string
}

// NewContext creates a new machine context
func NewContext() *Context {
	return &Context{
		OS:            runtime.GOOS,
		Architecture:  runtime.GOARCH,
		WorkspacePath: os.Getenv("PWD"),
		Shell:         os.Getenv("SHELL"),
	}
}

// GetSystemInfo returns detailed system information
func (c *Context) GetSystemInfo() map[string]string {
	return map[string]string{
		"os":         c.OS,
		"arch":       c.Architecture,
		"workspace":  c.WorkspacePath,
		"shell":      c.Shell,
		"num_cpu":    string(runtime.NumCPU()),
		"go_version": runtime.Version(),
	}
}
