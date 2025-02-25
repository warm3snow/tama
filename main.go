package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"log/slog"
	"github.com/fatih/color"
)

type Provider struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

type Config struct {
	Providers map[string]Provider `json:"providers"`
	Defaults  struct {
		Provider    string  `json:"provider"`
		Model      string  `json:"model"`
		Temperature float64 `json:"temperature"`
		MaxTokens  int     `json:"max_tokens"`
	} `json:"defaults"`
}

func getDefaultConfig() Config {
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

func initConfig() (Config, error) {
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
		config := getDefaultConfig()
		
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

const (
	logo = `
████████╗ █████╗ ███╗   ███╗ █████╗     ██████╗ ██████╗ ██████╗ ███████╗
╚══██╔══╝██╔══██╗████╗ ████║██╔══██╗   ██╔════╝██╔═══██╗██╔══██╗██╔════╝
   ██║   ███████║██╔████╔██║███████║   ██║     ██║   ██║██║  ██║█████╗  
   ██║   ██╔══██║██║╚██╔╝██║██╔══██║   ██║     ██║   ██║██║  ██║██╔══╝  
   ██║   ██║  ██║██║ ╚═╝ ██║██║  ██║   ╚██████╗╚██████╔╝██████╔╝███████╗
   ╚═╝   ╚═╝  ╚═╝╚═╝     ╚═╝╚═╝  ╚═╝    ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝`
)

func showInitialScreen() {
	fmt.Println("\n* Welcome to Tama Code research preview!")
	fmt.Println("\nLet's get started.")
	fmt.Println("\nChoose the text style that looks best with your terminal:")
	fmt.Println("To change this later, run /config")
	
	lightText := color.New(color.FgHiWhite)
	lightText.Println("> Light text")
	fmt.Println("  Dark text")
	fmt.Println("  Light text (colorblind-friendly)")
	fmt.Println("  Dark text (colorblind-friendly)")
	
	fmt.Println("\nPreview")
	
	fmt.Println("1 function greet() {")
	color.Green(`2   console.log("Hello, World!");`)
	color.Green(`3   console.log("Hello, Tama!");`)
	fmt.Println("4 }")
}

func showSecondScreen() {
	coral := color.New(color.FgRed).Add(color.FgYellow)
	
	fmt.Println("\n* Welcome to Tama Code!")
	coral.Println(logo)
	fmt.Println("\nPress Enter to continue...")
}

func showPrompt() {
	fmt.Print("Paste code here if prompted > ")
}

func main() {
	// Initialize configuration
	config, err := initConfig()
	if err != nil {
		fmt.Printf("Error initializing config: %v\n", err)
		os.Exit(1)
	}
	
	// Now you can use config throughout your application
	// For example: fmt.Printf("Using model: %s\n", config.Defaults.Model)
	slog.Info("Using model", "model", config.Defaults.Model)

	showInitialScreen()
	
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
	
	// Clear screen
	fmt.Print("\033[H\033[2J")
	
	showSecondScreen()
	
	reader.ReadString('\n')
	
	// Clear screen again
	fmt.Print("\033[H\033[2J")
	
	showPrompt()
	reader.ReadString('\n')
}