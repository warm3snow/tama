package cmd

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
	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/llm"
)

// codeCmd represents the code command
var codeCmd = &cobra.Command{
	Use:   "code",
	Short: "AI-powered code assistant",
	Long: `A code assistant powered by AI that can help analyze code,
answer programming questions, and execute terminal commands.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Start the code assistant
		startCodeAssistant()
	},
}

// Command represents a slash command that can be executed
type SlashCommand struct {
	Name        string
	Description string
	Execute     func() error
}

func init() {
	rootCmd.AddCommand(codeCmd)

	// Flags specific to code
	codeCmd.Flags().StringP("model", "m", "", "Specify the AI model to use")
	codeCmd.Flags().StringP("provider", "p", "", "Specify the AI provider (openai, ollama)")
}

// CommandAnalysisResponse represents the structured response for command analysis
type CommandAnalysisResponse struct {
	IsCommand bool   `json:"is_command"`
	Command   string `json:"command"`
	Reason    string `json:"reason"`
}

// startCodeAssistant begins the interactive code assistant session
func startCodeAssistant() {
	client := llm.NewClient(Config)
	reader := bufio.NewReader(os.Stdin)
	userStyle, aiStyle := createStyledPrinters()
	commands := setupSlashCommands()
	cmdStyle := color.New(color.FgYellow).Add(color.Bold)

	// Show welcome message
	showCodeWelcomeMessage()

	// Main interaction loop
	for {
		fmt.Print("Code Assistant > ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// Check if this is a slash command
		if strings.HasPrefix(input, "/") {
			if handleSlashCommand(input, commands) {
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

			userStyle(input)
			cmdStyle.Printf("\nDirectly executing: %s\n\n", cmdStr)

			err := executeCommand(cmdStr)
			if err != nil {
				fmt.Printf("Error executing command: %v\n", err)
			}
			continue
		}

		userStyle(input)

		// First, check if this might be a shell command
		isCmd, cmdStr, err := analyzeIfCommand(client, input)
		if err == nil && isCmd {
			cmdStyle.Printf("\nExecuting command: %s\n\n", cmdStr)
			err := executeCommand(cmdStr)
			if err != nil {
				fmt.Printf("Error executing command: %v\n", err)
			}
			continue
		}

		// Check if this is a codebase analysis request
		var response string

		if isCodebaseAnalysisRequest(input) {
			// Collect codebase info and enhance the prompt
			codebaseInfo := collectCodebaseInfo()
			fmt.Println("\nAnalyzing codebase, please wait...")
			enhancedPrompt := fmt.Sprintf("%s\n\n%s", input, codebaseInfo)
			response, err = client.SendMessage(enhancedPrompt)
		} else {
			// Regular message
			response, err = client.SendMessage(input)
		}

		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		aiStyle(response)

		// Update conversation history
		client.UpdateConversation(input, response)
	}
}

// showCodeWelcomeMessage displays the welcome message for the code assistant
func showCodeWelcomeMessage() {
	PrintLogo("Code")
	fmt.Println("AI-powered coding companion")
	fmt.Println("Type 'exit' or 'quit' to end the session")
	fmt.Println("Type '@command' to directly execute terminal commands")
}

// setupSlashCommands registers built-in slash commands
func setupSlashCommands() map[string]SlashCommand {
	commands := make(map[string]SlashCommand)

	commands["help"] = SlashCommand{
		Name:        "help",
		Description: "Show this help message",
		Execute: func() error {
			fmt.Println("\nAvailable commands:")
			for name, cmd := range commands {
				fmt.Printf("  /%-8s - %s\n", name, cmd.Description)
			}
			fmt.Println("  exit      - Exit the program")
			fmt.Println("  quit      - Exit the program")
			return nil
		},
	}

	return commands
}

// handleSlashCommand processes commands that start with "/"
func handleSlashCommand(input string, commands map[string]SlashCommand) bool {
	// Strip the leading "/"
	cmdName := strings.TrimPrefix(input, "/")

	// Check if command exists
	if cmd, exists := commands[cmdName]; exists {
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

// analyzeIfCommand checks if the user input is a Linux command
func analyzeIfCommand(client *llm.Client, input string) (bool, string, error) {
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
	response, err := client.SendMessage(prompt)
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

// executeCommand executes a shell command with proper terminal connection
func executeCommand(cmdStr string) error {
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
func collectCodebaseInfo() string {
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
