package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/config"
	"github.com/warm3snow/tama/internal/copilot"
	"github.com/warm3snow/tama/internal/logging"
)

var (
	// Used for flags
	cfgFile string
	Config  config.Config
)

// contextKey is a custom type for context keys
type contextKey string

const (
	// copilotKey is the key for storing the copilot instance in context
	copilotKey contextKey = "copilot"
)

// PrintLogo prints the TAMA ASCII art logo with the given subcommand name
func PrintLogo(subcommand string) {
	logoColor := color.New(color.FgCyan, color.Bold)

	// Print unified TAMA AI logo for all subcommands
	logoColor.Printf(`
████████╗ █████╗ ███╗   ███╗ █████╗      █████╗ ██╗
╚══██╔══╝██╔══██╗████╗ ████║██╔══██╗    ██╔══██╗██║
   ██║   ███████║██╔████╔██║███████║    ███████║██║
   ██║   ██╔══██║██║╚██╔╝██║██╔══██║    ██╔══██║██║
   ██║   ██║  ██║██║ ╚═╝ ██║██║  ██║    ██║  ██║██║
   ╚═╝   ╚═╝  ╚═╝╚═╝     ╚═╝╚═╝  ╚═╝    ╚═╝  ╚═╝╚═╝
                          
`)
	// Display subcommand name if it's not the default
	if subcommand != "" && subcommand != "TAMA" {
		fmt.Printf("         %s\n", subcommand)
	}

	fmt.Println()
}

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "tama",
	Short: "Tama is an AI-powered terminal assistant",
	Long: `Tama is a terminal-based AI assistant that helps you interact 
with language models directly from your command line. You can chat with 
AI models, execute commands with AI analysis, and more.`,
	// Add Run function to display logo when root command is executed
	Run: func(cmd *cobra.Command, args []string) {
		PrintLogo("TAMA")
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	// Initialize logger
	if err := logging.InitLogger(); err != nil {
		fmt.Printf("Error initializing logger: %v\n", err)
		// Continue execution even if logger fails, just without file logging
	}
	// Ensure logger is closed on exit
	defer logging.Close()

	logging.LogAppStart("1.0.0")

	if err := rootCmd.Execute(); err != nil {
		logging.LogError("Command execution failed", "error", err)
		fmt.Println(err)
		os.Exit(1)
	}

	logging.LogAppExit()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/tama/config.json)")

	// Set PreRun hook for all commands to ensure they have the context
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if cmd.Context() == nil {
			if ctx := rootCmd.Context(); ctx != nil {
				cmd.SetContext(ctx)
			}
		}
	}
}

func initConfig() {
	// Load config from file
	var err error
	Config, err = config.LoadConfig(cfgFile)
	if err != nil {
		logging.LogError("Failed to load config, using defaults", "error", err)
		// If config file doesn't exist or has errors, use defaults
		Config = config.GetDefaultConfig()
	}

	// Create copilot instance
	cop := copilot.New(Config)

	// Create context with copilot instance
	ctx := context.WithValue(context.Background(), copilotKey, cop)

	// Set context for root command and all subcommands
	rootCmd.SetContext(ctx)
	for _, cmd := range rootCmd.Commands() {
		cmd.SetContext(ctx)
	}
}

// GetCopilot retrieves the copilot instance from the command context
func GetCopilot(cmd *cobra.Command) *copilot.Copilot {
	if cop, ok := cmd.Context().Value(copilotKey).(*copilot.Copilot); ok {
		return cop
	}
	return nil
}
