package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/agent"
	"github.com/warm3snow/tama/internal/config"
)

var rootCmd = &cobra.Command{
	Use:   "tama",
	Short: "Tama is an autonomous AI coding assistant",
	Long: `Tama is an autonomous AI coding assistant that acts as a peer programmer.
It can perform multi-step coding tasks by analyzing your codebase, reading files,
proposing edits, and running commands.`,
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Tama agent",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %s\n", err)
			os.Exit(1)
		}

		a := agent.New(cfg)
		if err := a.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting agent: %s\n", err)
			os.Exit(1)
		}
	},
}

var execCmd = &cobra.Command{
	Use:   "exec [task]",
	Short: "Execute a specific task",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		task := args[0]

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %s\n", err)
			os.Exit(1)
		}

		a := agent.New(cfg)
		if err := a.ExecuteTask(task); err != nil {
			fmt.Fprintf(os.Stderr, "Error executing task: %s\n", err)
			os.Exit(1)
		}
	},
}

// Execute executes the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(execCmd)
}
