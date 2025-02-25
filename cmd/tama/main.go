package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/warm3snow/tama/internal/config"
	"github.com/warm3snow/tama/internal/llm"
	"github.com/warm3snow/tama/internal/ui"
)

// showConfigFile displays the contents of the config file
func showConfigFile() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}

	configDir := filepath.Join(homeDir, ".config", "tama")
	configPath := filepath.Join(configDir, "config.json")
	
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found at %s", configPath)
	}
	
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	fmt.Println("\n--- Tama Configuration File ---")
	fmt.Printf("File: %s\n\n", configPath)
	fmt.Println(string(content))
	fmt.Println("------------------------------")
	return nil
}

// runChatInterface starts the interactive chat session
func runChatInterface(client *llm.Client) {
	reader := bufio.NewReader(os.Stdin)
	userPrinter, aiPrinter := ui.CreateColoredPrinters()
	
	ui.PrintModelInfo(client.GetProvider(), client.GetModel())
	fmt.Println("Type '/config' to view your configuration file")
	
	for {
		ui.ShowPrompt()
		
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		
		if input == "" {
			continue
		}
		
		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// Handle /config command - check with or without the slash
		if input == "/config" || input == "config" {
			err := showConfigFile()
			if err != nil {
				fmt.Printf("Error showing config file: %v\n", err)
			}
			continue
		}
		
		// Only send non-command input to the LLM
		userPrinter(input)
		
		response, err := client.SendMessage(input)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}
		
		aiPrinter(response)
		client.UpdateConversation(input, response)
	}
}

func main() {
	// Initialize configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error initializing config: %v\n", err)
		os.Exit(1)
	}
	
	slog.Info("Using model", "model", cfg.Defaults.Model)
	
	// Create LLM client
	client := llm.NewClient(cfg)

	// Show UI screens
	ui.ShowInitialScreen()
	
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
	
	// Clear screen and show second screen
	ui.ClearScreen()
	ui.ShowSecondScreen()
	
	reader.ReadString('\n')
	
	// Start chat interface
	runChatInterface(client)
} 