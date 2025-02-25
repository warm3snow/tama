package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/config"
	"github.com/warm3snow/tama/internal/llm"
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start a chat session with AI",
	Long:  `Start an interactive chat session with an AI language model.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Start the chat session
		startChat()
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)

	// Flags specific to chat
	chatCmd.Flags().StringP("model", "m", "", "Specify the AI model to use")
	chatCmd.Flags().StringP("provider", "p", "", "Specify the AI provider (openai, ollama)")
}

// startChat begins the interactive chat session with the AI
func startChat() {
	// Create a client using the loaded config
	client := llm.NewClient(Config)
	reader := bufio.NewReader(os.Stdin)
	userStyle, aiStyle := createStyledPrinters()

	// Show welcome message
	showWelcomeMessage(Config)

	// Main chat loop
	for {
		fmt.Print("> ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// Show user input
		userStyle(input)

		// Get response from AI
		response, err := client.SendMessage(input)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		// Show AI response
		aiStyle(response)

		// Update conversation history
		client.UpdateConversation(input, response)
	}
}

// showWelcomeMessage displays a welcome message at the start of the chat
func showWelcomeMessage(cfg config.Config) {
	PrintLogo("Chat")
	modelInfo := color.New(color.FgCyan)
	fmt.Println("Start talking with AI")
	modelInfo.Printf("Connected to %s model: %s\n", cfg.Defaults.Provider, cfg.Defaults.Model)
	fmt.Println("Type 'exit' or 'quit' to end the conversation.")
}

// createStyledPrinters returns styled printer functions for user and AI messages
func createStyledPrinters() (userPrinter, aiPrinter func(string)) {
	userStyle := color.New(color.FgGreen).Add(color.Bold)
	aiStyle := color.New(color.FgBlue)

	userPrinter = func(msg string) {
		userStyle.Printf("\nYou: %s\n", msg)
	}

	aiPrinter = func(msg string) {
		aiStyle.Printf("\nAI: %s\n\n", msg)
	}

	return userPrinter, aiPrinter
}
