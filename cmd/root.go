package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/config"
)

var (
	// Used for flags
	cfgFile string
	Config  config.Config
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
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/tama/config.json)")
}

func initConfig() {
	// Load config from file
	var err error
	Config, err = config.LoadConfig(cfgFile)
	if err != nil {
		// If config file doesn't exist or has errors, use defaults
		Config = config.GetDefaultConfig()
	}
}
