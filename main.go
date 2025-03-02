package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/warm3snow/tama/cmd"
)

func main() {
	// Print banner
	printBanner()

	// Execute the command
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

// printBanner prints a welcome banner
func printBanner() {
	banner := `
  _______                      
 |__   __|                     
    | | __ _ _ __ ___   __ _   
    | |/ _' | '_ ' _ \ / _' |  
    | | (_| | | | | | | (_| |  
    |_|\__,_|_| |_| |_|\__,_|  
                               
 Copilot Agent - Your AI Coding Assistant
 OS: %s
 Version: 0.1.0
`
	fmt.Printf(banner, runtime.GOOS)
}
