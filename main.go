package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/warm3snow/tama/internal/config"
	"github.com/warm3snow/tama/internal/llm"
)

// Command represents a slash command that can be executed
type Command struct {
	Name        string
	Description string
	Execute     func() error
}

// Tama represents the application state
type Tama struct {
	config     config.Config
	client     *llm.Client
	reader     *bufio.Reader
	commands   map[string]Command
	userStyle  func(string)
	aiStyle    func(string)
}

// NewTama creates a new application instance
func NewTama(cfg config.Config) *Tama {
	client := llm.NewClient(cfg)
	userPrinter, aiPrinter := createStyledPrinters()
	
	tama := &Tama{
		config:    cfg,
		client:    client,
		reader:    bufio.NewReader(os.Stdin),
		commands:  make(map[string]Command),
		userStyle: userPrinter,
		aiStyle:   aiPrinter,
	}
	
	// Register built-in commands
	tama.registerBuiltInCommands()
	
	return tama
}

// registerBuiltInCommands registers the default commands
func (t *Tama) registerBuiltInCommands() {
	t.RegisterCommand(Command{
		Name:        "help",
		Description: "Show this help message",
		Execute: func() error {
			fmt.Println("\nAvailable commands:")
			for name, cmd := range t.commands {
				fmt.Printf("  /%-8s - %s\n", name, cmd.Description)
			}
			fmt.Println("  exit      - Exit the program")
			fmt.Println("  quit      - Exit the program")
			return nil
		},
	})
	
	t.RegisterCommand(Command{
		Name:        "config",
		Description: "Show configuration file",
		Execute: func() error {
			return t.showConfigFile()
		},
	})
}

// RegisterCommand adds a new command to the app
func (t *Tama) RegisterCommand(cmd Command) {
	t.commands[cmd.Name] = cmd
}

// showConfigFile displays the contents of the config file
func (t *Tama) showConfigFile() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}

	configDir := filepath.Join(homeDir, ".config", "tama")
	configPath := filepath.Join(configDir, "config.json")
	
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found at %s", configPath)
	}
	
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	fmt.Println("\n--- Tama Configuration File ---")
	fmt.Printf("File: %s\n\n", configPath)
	fmt.Println(string(content))
	fmt.Println("------------------------------")
	return nil
}

// handleCommand processes commands that start with "/"
func (t *Tama) handleCommand(input string) bool {
	// Strip the leading "/"
	cmdName := strings.TrimPrefix(input, "/")
	
	// Check if command exists
	if cmd, exists := t.commands[cmdName]; exists {
		err := cmd.Execute()
		if err != nil {
			fmt.Printf("Error executing command %s: %v\n", cmdName, err)
		}
		return true
	}
	
	fmt.Printf("Unknown command: /%s\n", cmdName)
	fmt.Println("Type /help for available commands")
	return true
}

// createStyledPrinters returns styled printer functions for user and AI messages
func createStyledPrinters() (userPrinter, aiPrinter func(string)) {
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

// Run starts the application
func (t *Tama) Run() {
	t.showInitialScreens()
	t.startChatLoop()
}

// showInitialScreens displays the welcome screens
func (t *Tama) showInitialScreens() {
	// t.showInitialScreen()
	
	// t.reader.ReadString('\n')
	
	// // Clear screen
	// fmt.Print("\033[H\033[2J")
	
	t.showSecondScreen()
	
	t.reader.ReadString('\n')
}

// showInitialScreen displays the first welcome screen
func (t *Tama) showInitialScreen() {
	fmt.Println("\n* Welcome to Tama Code research preview!")
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

// logo is the ascii art logo for Tama Code
const logo = `
████████╗ █████╗ ███╗   ███╗ █████╗     ██████╗ ██████╗ ██████╗ ███████╗
╚══██╔══╝██╔══██╗████╗ ████║██╔══██╗   ██╔════╝██╔═══██╗██╔══██╗██╔════╝
   ██║   ███████║██╔████╔██║███████║   ██║     ██║   ██║██║  ██║█████╗  
   ██║   ██╔══██║██║╚██╔╝██║██╔══██║   ██║     ██║   ██║██║  ██║██╔══╝  
   ██║   ██║  ██║██║ ╚═╝ ██║██║  ██║   ╚██████╗╚██████╔╝██████╔╝███████╗
   ╚═╝   ╚═╝  ╚═╝╚═╝     ╚═╝╚═╝  ╚═╝    ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝`

// showSecondScreen displays the logo and welcome message
func (t *Tama) showSecondScreen() {
	coral := color.New(color.FgRed).Add(color.FgYellow)
	
	fmt.Println("\n* Welcome to Tama Code!")
	coral.Println(logo)
	fmt.Println("\nPress Enter to continue...")
}

// showPrompt displays the input prompt
func (t *Tama) showPrompt() {
	fmt.Print("Paste code here if prompted > ")
}

// startChatLoop begins the interactive chat session with the LLM
func (t *Tama) startChatLoop() {
	modelInfo := color.New(color.FgCyan)
	
	modelInfo.Printf("\nConnected to %s model: %s\n", t.config.Defaults.Provider, t.config.Defaults.Model)
	fmt.Println("Type 'exit' or 'quit' to end the conversation.")
	fmt.Println("Type '/help' for available commands")
	
	for {
		t.showPrompt()
		
		input, _ := t.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		
		if input == "" {
			continue
		}
		
		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}
		
		// Check if this is a command (starts with /)
		if strings.HasPrefix(input, "/") {
			if t.handleCommand(input) {
				continue
			}
		}
		
		t.userStyle(input)
		
		response, err := t.client.SendMessage(input)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}
		
		t.aiStyle(response)
		
		// Update conversation history
		t.client.UpdateConversation(input, response)
	}
}

func main() {
	// Initialize configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error initializing config: %v\n", err)
		os.Exit(1)
	}
	
	slog.Info("Using model", "model", cfg.Defaults.Model)

	// Create and run the application
	tama := NewTama(cfg)
	tama.Run()
}