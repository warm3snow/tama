package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Tama configuration",
	Long:  `View and modify Tama configuration settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, show the current config
		showConfig()
	},
}

// showCmd represents the show subcommand
var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration settings of Tama.`,
	Run: func(cmd *cobra.Command, args []string) {
		showConfig()
	},
}

// initCmd represents the init subcommand
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration",
	Long:  `Create a new configuration file with default settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		initializeConfig()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(showCmd)
	configCmd.AddCommand(initCmd)
}

// showConfig displays the contents of the config file
func showConfig() {
	PrintLogo("Config")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error: failed to get home directory: %v\n", err)
		return
	}

	configDir := filepath.Join(homeDir, ".config", "tama")
	configPath := filepath.Join(configDir, "config.json")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("Config file not found at %s\n", configPath)
		fmt.Println("Run 'tama config init' to create a new configuration file.")
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("Error: failed to read config file: %v\n", err)
		return
	}

	fmt.Println("--- Tama Configuration File ---")
	fmt.Printf("File: %s\n\n", configPath)
	fmt.Println(string(content))
	fmt.Println("------------------------------")
}

// initializeConfig creates a new configuration file with default settings
func initializeConfig() {
	PrintLogo("Config")
	// Use the defaults from the config package
	cfg := Config

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error: failed to get home directory: %v\n", err)
		return
	}

	configDir := filepath.Join(homeDir, ".config", "tama")
	configPath := filepath.Join(configDir, "config.json")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("Error: failed to create config directory: %v\n", err)
		return
	}

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Config file already exists at %s\n", configPath)
		fmt.Println("Use --force to overwrite the existing configuration.")
		return
	}

	// Save to file
	if err := cfg.SaveToFile(configPath); err != nil {
		fmt.Printf("Error: failed to save config file: %v\n", err)
		return
	}

	fmt.Printf("Configuration file created at %s\n", configPath)
}
