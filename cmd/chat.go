package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/chat"
	"github.com/warm3snow/tama/internal/llm"
	"github.com/warm3snow/tama/internal/logging"
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Chat with the AI",
	Long: `Start a conversation with the AI.
You can provide a message directly as an argument, or omit it
to enter interactive chat mode.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Print logo before starting chat
		PrintLogo("Chat")

		// Create LLM client
		client := llm.NewClient(Config)
		logging.LogLLMRequest(client.GetProvider(), client.GetModel(), 0) // Log chat initialization

		// Check if we're in interactive mode or single message mode
		isInteractive := len(args) == 0

		// Create chat handler
		handler := chat.NewChatHandler(client, isInteractive)

		if isInteractive {
			// Start interactive chat session
			err := handler.StartInteractiveChat()
			if err != nil {
				logging.LogError("Interactive chat failed", "error", err)
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Send a single message
			message := strings.Join(args, " ")
			response, err := handler.SendMessage(message)
			if err != nil {
				logging.LogError("Failed to get response", "error", err)
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			// Print the response
			fmt.Println(response)
		}
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)

	// Add flags specific to chat
	chatCmd.Flags().StringP("model", "m", "", "Specify the AI model to use")
	chatCmd.Flags().StringP("provider", "p", "", "Specify the AI provider (openai, ollama)")
}
