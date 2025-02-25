package main

import (
	"fmt"
	"os"

	"github.com/warm3snow/tama/cmd/tama"
	"github.com/warm3snow/tama/internal/config"
)

func main() {
	// Initialize configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error initializing config: %v\n", err)
		os.Exit(1)
	}

	// Create and run the application
	app := tama.NewTama(cfg)
	app.Run()
}
