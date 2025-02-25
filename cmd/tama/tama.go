package tama

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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
	config    config.Config
	client    *llm.Client
	reader    *bufio.Reader
	commands  map[string]Command
	userStyle func(string)
	aiStyle   func(string)
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

// CommandAnalysisResponse represents the structured response for command analysis
type CommandAnalysisResponse struct {
	IsCommand bool   `json:"is_command"`
	Command   string `json:"command"`
	Reason    string `json:"reason"`
}

// analyzeIfCommand checks if the user input is a Linux command
func (t *Tama) analyzeIfCommand(input string) (bool, string, error) {
	// Create a special prompt for the LLM to analyze if this is a command
	prompt := fmt.Sprintf(`Analyze if the following text is a valid Linux/Unix shell command:
"%s"

Return your analysis in this JSON format:
{
  "is_command": true/false,
  "command": "the command if it is one, or empty string",
  "reason": "brief explanation of your decision"
}

Only respond with the JSON, nothing else.`, input)

	// Send to LLM for analysis
	response, err := t.client.SendMessage(prompt)
	if err != nil {
		return false, "", err
	}

	// Try to parse the response as JSON
	var result CommandAnalysisResponse

	// Extract JSON object from response (in case the LLM adds extra text)
	jsonPattern := regexp.MustCompile(`(?s)\{.*\}`)
	match := jsonPattern.FindString(response)
	if match == "" {
		return false, "", fmt.Errorf("couldn't extract JSON from LLM response")
	}

	err = json.Unmarshal([]byte(match), &result)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse LLM response: %v", err)
	}

	return result.IsCommand, result.Command, nil
}

// executeCommand executes a shell command and returns the output
func (t *Tama) executeCommand(cmdStr string) error {
	cmd := exec.Command("sh", "-c", cmdStr)

	// Set environment variables to encourage color output
	env := os.Environ()
	cmd.Env = append(env,
		"FORCE_COLOR=true",
		"CLICOLOR_FORCE=1",
		"CLICOLOR=1",
		"COLORTERM=truecolor")

	// Connect command's stdout and stderr directly to our process stdout and stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command
	return cmd.Run()
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

// isCodebaseAnalysisRequest checks if the user is asking for codebase information
func isCodebaseAnalysisRequest(input string) bool {
	input = strings.ToLower(input)
	patterns := []string{
		"overview of (this |the |)codebase",
		"(explain|describe|understand|analyze|summarize) (this |the |)codebase",
		"what('s| is) (this |the |)codebase (about|doing)",
		"how (is|does) (this |the |)codebase (work|structured)",
		"tell me about (this |the |)codebase",
	}

	for _, pattern := range patterns {
		matched, _ := regexp.MatchString(pattern, input)
		if matched {
			return true
		}
	}

	return false
}

// collectCodebaseInfo gathers information about the codebase
func (t *Tama) collectCodebaseInfo() string {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "Error getting current directory: " + err.Error()
	}

	// Extract project name from path
	projectName := filepath.Base(cwd)

	// Collect basic structure information
	var info strings.Builder
	info.WriteString(fmt.Sprintf("# %s Codebase Overview\n\n", projectName))
	info.WriteString(fmt.Sprintf("Current directory: %s\n", cwd))
	info.WriteString(fmt.Sprintf("Time of analysis: %s\n\n", time.Now().Format(time.RFC1123)))

	// Try to find common directories and files
	var dirs []string
	var files []string
	commonDirs := []string{"cmd", "internal", "pkg", "api", "web", "config", "docs"}
	commonFiles := []string{"main.go", "go.mod", "go.sum", "README.md", "LICENSE"}

	for _, dir := range commonDirs {
		if fileInfo, err := os.Stat(filepath.Join(cwd, dir)); err == nil && fileInfo.IsDir() {
			dirs = append(dirs, dir)
		}
	}

	for _, file := range commonFiles {
		if fileInfo, err := os.Stat(filepath.Join(cwd, file)); err == nil && !fileInfo.IsDir() {
			files = append(files, file)
		}
	}

	// Add directory information
	if len(dirs) > 0 {
		info.WriteString("## Key Directories\n\n")
		for _, dir := range dirs {
			info.WriteString(fmt.Sprintf("- %s/\n", dir))
		}
		info.WriteString("\n")
	}

	// Add file information
	if len(files) > 0 {
		info.WriteString("## Key Files\n\n")
		for _, file := range files {
			info.WriteString(fmt.Sprintf("- %s\n", file))
		}
		info.WriteString("\n")
	}

	// Try to read go.mod for dependencies
	if gomodContent, err := os.ReadFile("go.mod"); err == nil {
		info.WriteString("## Dependencies\n\n")
		info.WriteString("From go.mod:\n```\n")
		info.WriteString(string(gomodContent))
		info.WriteString("```\n\n")
	}

	// Try to read README.md for project description
	if readmeContent, err := os.ReadFile("README.md"); err == nil {
		info.WriteString("## Project Description\n\n")
		info.WriteString("From README.md:\n")
		// Limit the size to avoid overwhelming LLM
		readmeStr := string(readmeContent)
		if len(readmeStr) > 1000 {
			readmeStr = readmeStr[:1000] + "...(truncated)"
		}
		info.WriteString(readmeStr)
		info.WriteString("\n\n")
	}

	info.WriteString("## Analysis Request\n\n")
	info.WriteString("Please analyze this codebase and provide:\n")
	info.WriteString("1. A high-level overview of what this project does\n")
	info.WriteString("2. The main components and their interactions\n")
	info.WriteString("3. The technology stack being used\n")
	info.WriteString("4. Any notable design patterns or architecture choices\n")

	return info.String()
}

// startChatLoop begins the interactive chat session with the LLM
func (t *Tama) startChatLoop() {
	modelInfo := color.New(color.FgCyan)
	cmdStyle := color.New(color.FgYellow).Add(color.Bold)

	modelInfo.Printf("\nConnected to %s model: %s\n", t.config.Defaults.Provider, t.config.Defaults.Model)
	fmt.Println("Type 'exit' or 'quit' to end the conversation.")
	fmt.Println("Type '/help' for available commands")
	fmt.Println("Type 'give me an overview of this codebase' to analyze the current project")
	fmt.Println("Type '@command' to directly execute terminal commands")
	fmt.Println("You can also enter Linux commands normally and they will be analyzed by AI")

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

		// Check if this is a direct terminal command (starts with @)
		if strings.HasPrefix(input, "@") {
			// Remove the @ prefix
			cmdStr := strings.TrimPrefix(input, "@")
			cmdStr = strings.TrimSpace(cmdStr)

			if cmdStr == "" {
				fmt.Println("Error: No command specified after @")
				continue
			}

			t.userStyle(input)
			cmdStyle.Printf("\nDirectly executing: %s\n\n", cmdStr)

			err := t.executeCommand(cmdStr)
			if err != nil {
				fmt.Printf("Error executing command: %v\n", err)
			}
			continue
		}

		t.userStyle(input)

		// First, check if this might be a shell command
		isCmd, cmdStr, err := t.analyzeIfCommand(input)
		if err == nil && isCmd {
			cmdStyle.Printf("\nExecuting command: %s\n\n", cmdStr)
			err := t.executeCommand(cmdStr)
			if err != nil {
				fmt.Printf("Error executing command: %v\n", err)
			}
			continue
		}

		// Check if this is a codebase analysis request
		var response string

		if isCodebaseAnalysisRequest(input) {
			// Collect codebase info and enhance the prompt
			codebaseInfo := t.collectCodebaseInfo()
			fmt.Println("\nAnalyzing codebase, please wait...")
			enhancedPrompt := fmt.Sprintf("%s\n\n%s", input, codebaseInfo)
			response, err = t.client.SendMessage(enhancedPrompt)
		} else {
			// Regular message
			response, err = t.client.SendMessage(input)
		}

		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		t.aiStyle(response)

		// Update conversation history
		t.client.UpdateConversation(input, response)
	}
}
