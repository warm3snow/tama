package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	LLM       LLMConfig       `yaml:"llm"`
	Tools     ToolsConfig     `yaml:"tools"`
	Workspace WorkspaceConfig `yaml:"workspace"`
	UI        UIConfig        `yaml:"ui"`
}

// LLMConfig represents the LLM configuration
type LLMConfig struct {
	Provider    string            `yaml:"provider"`
	Model       string            `yaml:"model"`
	APIKey      string            `yaml:"api_key"`
	BaseURL     string            `yaml:"base_url"`
	Temperature float64           `yaml:"temperature"`
	MaxTokens   int               `yaml:"max_tokens"`
	Options     map[string]string `yaml:"options"`
}

// ToolsConfig represents the tools configuration
type ToolsConfig struct {
	Enabled []string `yaml:"enabled"`
}

// WorkspaceConfig represents the workspace configuration
type WorkspaceConfig struct {
	IgnoreDirs  []string `yaml:"ignore_dirs"`
	IgnoreFiles []string `yaml:"ignore_files"`
	MaxFileSize int64    `yaml:"max_file_size"` // in bytes
}

// UIConfig represents the UI configuration
type UIConfig struct {
	ColorEnabled bool   `yaml:"color_enabled"`
	LogLevel     string `yaml:"log_level"`
	Verbose      bool   `yaml:"verbose"`
}

// OSContext represents the OS context
type OSContext struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Arch    string `yaml:"arch"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider:    "ollama",
			Model:       "llama3.2:latest",
			BaseURL:     "http://localhost:11434/v1",
			Temperature: 0.7,
			MaxTokens:   4096,
			Options:     map[string]string{},
		},
		Tools: ToolsConfig{
			Enabled: []string{
				"file_read",
				"file_edit",
				"terminal_run",
				"test_run",
				"file_search",
				"dir_list",
			},
		},
		Workspace: WorkspaceConfig{
			IgnoreDirs: []string{
				".git",
				"node_modules",
				"vendor",
				"dist",
				"build",
			},
			IgnoreFiles: []string{
				".DS_Store",
				"*.log",
				"*.lock",
			},
			MaxFileSize: 1024 * 1024, // 1MB
		},
		UI: UIConfig{
			ColorEnabled: true,
			LogLevel:     "info",
			Verbose:      false,
		},
	}
}

// Load loads the configuration from the tama.yaml file
func Load() (*Config, error) {
	// Start with default config
	cfg := DefaultConfig()

	// Look for config file in current directory
	configPath := "tama.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// If not found, look in user's ~/.tama directory
		home, err := os.UserHomeDir()
		if err != nil {
			return cfg, nil // Return default if can't get home dir
		}

		tamaDir := filepath.Join(home, ".tama")
		configPath = filepath.Join(tamaDir, "tama.yaml")

		// Check if the config file exists in ~/.tama
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// Create the ~/.tama directory if it doesn't exist
			if err := os.MkdirAll(tamaDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to create directory %s: %s\n", tamaDir, err)
				return cfg, nil // Return default if can't create directory
			}

			// Save the default config to ~/.tama/tama.yaml
			data, err := yaml.Marshal(cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to marshal default config: %s\n", err)
				return cfg, nil // Return default if can't marshal
			}

			if err := os.WriteFile(configPath, data, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to write default config to %s: %s\n", configPath, err)
				return cfg, nil // Return default if can't write
			}

			fmt.Printf("Created default configuration at %s\n", configPath)
			return cfg, nil // Return default config that was just saved
		}
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Load API key from environment variable if not set in config
	if cfg.LLM.APIKey == "" {
		cfg.LLM.APIKey = os.Getenv("TAMA_API_KEY")
	}

	return cfg, nil
}

// Save saves the configuration to the specified path or ~/.tama/tama.yaml by default
func (c *Config) Save(path string) error {
	if path == "" {
		// Default to ~/.tama/tama.yaml
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("error getting user home directory: %w", err)
		}

		tamaDir := filepath.Join(home, ".tama")
		if err := os.MkdirAll(tamaDir, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", tamaDir, err)
		}

		path = filepath.Join(tamaDir, "tama.yaml")
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// GetOSContext returns the current OS context
func GetOSContext() OSContext {
	return OSContext{
		Name:    runtime.GOOS,
		Version: runtime.Version(),
		Arch:    runtime.GOARCH,
	}
}
