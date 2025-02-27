package cmd

import (
	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/code"
)

// codeCmd represents the code command
var codeCmd = &cobra.Command{
	Use:   "code",
	Short: "AI-powered code assistant",
	Long: `A code assistant powered by AI that can help analyze code,
answer programming questions, and execute terminal commands.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Print logo before starting code assistant
		PrintLogo("Code")

		// Create code assistant handler
		handler := code.NewHandler(Config)

		// Start the code assistant
		handler.Start()
	},
}

func init() {
	rootCmd.AddCommand(codeCmd)

	// Flags specific to code
	codeCmd.Flags().StringP("model", "m", "", "Specify the AI model to use")
	codeCmd.Flags().StringP("provider", "p", "", "Specify the AI provider (openai, ollama)")
}
