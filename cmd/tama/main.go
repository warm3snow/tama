package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/warm3snow/tama/internal/config"
	"github.com/warm3snow/tama/internal/llm"
	"github.com/warm3snow/tama/internal/ui"
)

// runChatInterface starts the interactive chat session
func runChatInterface(client *llm.Client) {
	reader := bufio.NewReader(os.Stdin)
	userPrinter, aiPrinter := ui.CreateColoredPrinters()
	
	ui.PrintModelInfo(client.GetProvider(), client.GetModel())
	
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