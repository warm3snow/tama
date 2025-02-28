package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/logging"
)

// codeCmd represents the code command
var codeCmd = &cobra.Command{
	Use:   "code [request]",
	Short: "Get AI assistance with code",
	Long: `Get AI assistance with your code. You can:
- Ask questions about code
- Get code explanations
- Request code reviews
- Get suggestions for improvements`,
	Run: func(cmd *cobra.Command, args []string) {
		// Print logo before starting
		PrintLogo("Code")

		// Get copilot instance from context
		cop := GetCopilot(cmd)
		if cop == nil {
			fmt.Println("Error: Failed to initialize copilot")
			os.Exit(1)
		}

		// Check if we have a request
		if len(args) > 0 {
			// Process single request
			request := strings.Join(args, " ")
			respChan, err := cop.ProcessPrompt(request)
			if err != nil {
				logging.LogError("Failed to process code request", "error", err)
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			// Print response
			for chunk := range respChan {
				fmt.Print(chunk)
			}
			fmt.Println()
		} else {
			// Start interactive session
			if err := cop.StartInteractiveChat(); err != nil {
				logging.LogError("Interactive code session failed", "error", err)
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(codeCmd)

	// Add flags specific to code command
	codeCmd.Flags().StringP("model", "m", "", "Specify the AI model to use")
	codeCmd.Flags().StringP("provider", "p", "", "Specify the AI provider (openai, ollama)")
}
