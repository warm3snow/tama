package cmd

import (
	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/config"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Tama configuration",
	Long:  `View and modify Tama configuration settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, show the current config
		PrintLogo("Config")
		config.ShowConfig()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
