package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
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

		// Get copilot instance from context
		cop := GetCopilot(cmd)
		if cop == nil {
			fmt.Println("Error: Failed to initialize copilot")
			os.Exit(1)
		}

		// Check if we're in interactive mode or single message mode
		isInteractive := len(args) == 0

		if isInteractive {
			// Start interactive chat session
			if err := cop.StartInteractiveChat(); err != nil {
				logging.LogError("Interactive chat failed", "error", err)
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Send a single message
			message := strings.Join(args, " ")
			respChan, err := cop.ProcessPrompt(message)
			if err != nil {
				logging.LogError("Failed to process prompt", "error", err)
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			// Print the response
			for chunk := range respChan {
				fmt.Print(chunk)
			}
			fmt.Println()
		}
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)

	// Add flags specific to chat
	chatCmd.Flags().StringP("model", "m", "", "Specify the AI model to use")
	chatCmd.Flags().StringP("provider", "p", "", "Specify the AI provider (openai, ollama)")
}
