package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/logging"
)

// codeCmd represents the code command
var codeCmd = &cobra.Command{
	Use:   "code [request]",
	Short: "Get AI assistance with code",
	Long: `Get AI assistance with your code. You can:
- Start a coding project
- Let AI automatically implement features
- Fix bugs and improve code quality
- Review and rollback changes`,
	Run: func(cmd *cobra.Command, args []string) {
		// Print logo before starting
		PrintLogo("Code")

		// Get copilot instance from context
		cop := GetCopilot(cmd)
		if cop == nil {
			fmt.Println("Error: Failed to initialize copilot")
			os.Exit(1)
		}

		// Get project path
		projectPath, _ := cmd.Flags().GetString("project")
		if projectPath == "" {
			// Use current directory if not specified
			var err error
			projectPath, err = os.Getwd()
			if err != nil {
				logging.LogError("Failed to get current directory", "error", err)
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Convert to absolute path
			var err error
			projectPath, err = filepath.Abs(projectPath)
			if err != nil {
				logging.LogError("Failed to resolve project path", "error", err)
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
		}

		// Set project path in copilot
		if err := cop.SetProjectPath(projectPath); err != nil {
			logging.LogError("Failed to set project path", "error", err)
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Check if we have a request
		if len(args) > 0 {
			// Process single request in agent mode
			request := strings.Join(args, " ")
			if err := cop.StartAgentMode(request); err != nil {
				logging.LogError("Agent mode failed", "error", err)
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
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
	codeCmd.Flags().StringP("project", "d", "", "Specify the project directory (default: current directory)")
}
