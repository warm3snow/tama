package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProviderType represents the type of LLM provider
type ProviderType string

const (
	// OpenAI represents the OpenAI provider
	OpenAI ProviderType = "openai"
	// Ollama represents the Ollama provider
	Ollama ProviderType = "ollama"
)

// Provider represents an LLM API provider configuration
type Provider struct {
	Type    ProviderType `json:"type"`
	APIKey  string       `json:"api_key"`
	BaseURL string       `json:"base_url"`
}

// Config represents the application configuration
type Config struct {
	Providers map[string]Provider `json:"providers"`
	Defaults  DefaultProvider     `json:"defaults"`
}

// DefaultProvider represents the default provider configuration
type DefaultProvider struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

// GetDefaultConfig returns the default configuration
func GetDefaultConfig() Config {
	config := Config{
		Providers: map[string]Provider{
			"openai": {
				Type:    OpenAI,
				APIKey:  "sk-xxx",
				BaseURL: "https://api.openai.com/v1",
			},
			"ollama": {
				Type:    Ollama,
				BaseURL: "http://localhost:11434",
				APIKey:  "ollama",
			},
		},
		Defaults: DefaultProvider{
			Provider:    "ollama",
			Model:       "llama3.2:latest",
			Temperature: 0.7,
			MaxTokens:   2048,
		},
	}

	// Note: Can't use logging here as it's not initialized yet during app startup
	return config
}

// LoadConfig initializes and loads the configuration
func LoadConfig(configPath string) (Config, error) {
	var configFile string

	// If configPath is provided, use it
	if configPath != "" {
		configFile = configPath
	} else {
		// Otherwise use default location
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("failed to get home directory: %v", err)
		}

		configDir := filepath.Join(homeDir, ".config", "tama")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return Config{}, fmt.Errorf("failed to create config directory: %v", err)
		}

		configFile = filepath.Join(configDir, "config.json")
	}

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Config doesn't exist, create default
		config := GetDefaultConfig()

		// Save the default config to file
		if err := config.SaveToFile(configFile); err != nil {
			return Config{}, err
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

		// Ensure provider types are properly set
		for name, provider := range config.Providers {
			switch provider.Type {
			case OpenAI:
				config.Providers[name] = Provider{
					Type:    OpenAI,
					APIKey:  provider.APIKey,
					BaseURL: provider.BaseURL,
				}
			case Ollama:
				config.Providers[name] = Provider{
					Type:    Ollama,
					APIKey:  provider.APIKey,
					BaseURL: provider.BaseURL,
				}
			default:
				return Config{}, fmt.Errorf("unsupported provider type: %s", provider.Type)
			}
		}

		return config, nil
	}
}

// SaveToFile saves the configuration to the specified file
func (c *Config) SaveToFile(configFile string) error {
	file, err := os.Create(configFile)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// SwitchModel switches the model for the given provider
func (c *Config) SwitchModel(model string) error {
	c.Defaults.Model = model

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}
	configFile := filepath.Join(homeDir, ".config", "tama", "config.json")

	return c.SaveToFile(configFile)
}

// showConfig displays the contents of the config file
func ShowConfig() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error: failed to get home directory: %v\n", err)
		return
	}

	configDir := filepath.Join(homeDir, ".config", "tama")
	configPath := filepath.Join(configDir, "config.json")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("Config file not found at %s\n", configPath)
		fmt.Println("Run 'tama config init' to create a new configuration file.")
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("Error: failed to read config file: %v\n", err)
		return
	}

	fmt.Println("--- Tama Configuration File ---")
	fmt.Printf("File: %s\n\n", configPath)
	fmt.Println(string(content))
	fmt.Println("------------------------------")
}
