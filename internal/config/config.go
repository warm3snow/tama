package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Provider represents an LLM API provider configuration
type Provider struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

// Config represents the application configuration
type Config struct {
	Providers map[string]Provider `json:"providers"`
	Defaults  struct {
		Provider    string  `json:"provider"`
		Model      string  `json:"model"`
		Temperature float64 `json:"temperature"`
		MaxTokens  int     `json:"max_tokens"`
	} `json:"defaults"`
}

// GetDefaultConfig returns the default configuration
func GetDefaultConfig() Config {
	config := Config{
		Providers: map[string]Provider{
			"openai": {
				APIKey:  "sk-xxx",
				BaseURL: "https://api.openai.com/v1",
			},
			"ollama": {
				BaseURL: "http://localhost:11434",
				APIKey:  "ollama",
			},
		},
	}
	config.Defaults.Provider = "ollama"
	config.Defaults.Model = "llama3.2:latest"
	config.Defaults.Temperature = 0.7
	config.Defaults.MaxTokens = 2048
	return config
}

// LoadConfig initializes and loads the configuration
func LoadConfig() (Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("failed to get home directory: %v", err)
	}

	configDir := filepath.Join(homeDir, ".config", "tama")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return Config{}, fmt.Errorf("failed to create config directory: %v", err)
	}

	configFile := filepath.Join(configDir, "config.yaml")
	
	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Config doesn't exist, create default
		config := GetDefaultConfig()
		
		file, err := os.Create(configFile)
		if err != nil {
			return Config{}, fmt.Errorf("failed to create config file: %v", err)
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(config); err != nil {
			return Config{}, fmt.Errorf("failed to write config file: %v", err)
		}
		
		return config, nil
	} else {
		// Config exists, load it
		file, err := os.Open(configFile)
		if err != nil {
			return Config{}, fmt.Errorf("failed to open config file: %v", err)
		}
		defer file.Close()

		var config Config
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&config); err != nil {
			return Config{}, fmt.Errorf("failed to parse config file: %v", err)
		}
		
		return config, nil
	}
} 