package ui

import (
	"fmt"

	"github.com/fatih/color"
)

const (
	Logo = `
████████╗ █████╗ ███╗   ███╗ █████╗      █████╗ ██╗
╚══██╔══╝██╔══██╗████╗ ████║██╔══██╗    ██╔══██╗██║
   ██║   ███████║██╔████╔██║███████║    ███████║██║
   ██║   ██╔══██║██║╚██╔╝██║██╔══██║    ██╔══██║██║
   ██║   ██║  ██║██║ ╚═╝ ██║██║  ██║    ██║  ██║██║
   ╚═╝   ╚═╝  ╚═╝╚═╝     ╚═╝╚═╝  ╚═╝    ╚═╝  ╚═╝╚═╝`
)

// ShowInitialScreen displays the initial screen with style options
func ShowInitialScreen() {
	fmt.Println("\n* Welcome to Tama AI Assistant!")
	fmt.Println("\nLet's get started.")
	fmt.Println("\nChoose the text style that looks best with your terminal:")
	fmt.Println("To change this later, run /config")

	lightText := color.New(color.FgHiWhite)
	lightText.Println("> Light text")
	fmt.Println("  Dark text")
	fmt.Println("  Light text (colorblind-friendly)")
	fmt.Println("  Dark text (colorblind-friendly)")

	fmt.Println("\nPreview")

	fmt.Println("1 function greet() {")
	color.Green(`2   console.log("Hello, World!");`)
	color.Green(`3   console.log("Hello, Tama!");`)
	fmt.Println("4 }")
}

// ShowSecondScreen displays the logo and welcome message
func ShowSecondScreen() {
	coral := color.New(color.FgRed).Add(color.FgYellow)

	fmt.Println("\n* Welcome to Tama AI Assistant!")
	coral.Println(Logo)
	fmt.Println("\nPress Enter to continue...")
}

// ShowPrompt displays the input prompt
func ShowPrompt() {
	fmt.Print("> ")
}

// ClearScreen clears the terminal screen
func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

// CreateColoredPrinters returns styled printer functions for user and AI messages
func CreateColoredPrinters() (userPrinter, aiPrinter func(string)) {
	userStyle := color.New(color.FgGreen).Add(color.Bold)
	aiStyle := color.New(color.FgBlue)

	userPrinter = func(msg string) {
		userStyle.Printf("\nYou: %s\n", msg)
	}

	aiPrinter = func(msg string) {
		aiStyle.Printf("\nAI: %s\n\n", msg)
	}

	return
}

// PrintModelInfo displays information about the currently connected model
func PrintModelInfo(provider, model string) {
	modelInfo := color.New(color.FgCyan)
	modelInfo.Printf("\nConnected to %s model: %s\n", provider, model)
	fmt.Println("Type 'exit' or 'quit' to end the conversation.")
}
