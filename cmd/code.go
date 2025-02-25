package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/config"
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

// CodeAction represents a code-related action that can be performed
type CodeAction struct {
	Type        string `json:"type"`        // "analyze", "edit", "create", etc.
	FilePath    string `json:"file_path"`   // Path to the file to be analyzed/edited
	Content     string `json:"content"`     // New content for edit/create actions
	StartLine   int    `json:"start_line"`  // Starting line for edits (optional)
	EndLine     int    `json:"end_line"`    // Ending line for edits (optional)
	Description string `json:"description"` // Description of the action
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

// completer 实现readline.AutoCompleter接口，提供命令补全功能
type completer struct{}

// Do 实现自动补全逻辑
func (c completer) Do(line []rune, pos int) (newLine [][]rune, length int) {
	// 获取当前输入的前缀
	lineStr := string(line[:pos])

	// 仅当输入以/开头时提供命令补全
	if strings.HasPrefix(lineStr, "/") {
		prefix := strings.TrimPrefix(lineStr, "/")

		// 定义可用的命令
		var cmds = []string{"help", "config", "cd", "!"}

		// 过滤匹配的命令
		var matches []string
		for _, cmd := range cmds {
			if strings.HasPrefix(cmd, prefix) {
				matches = append(matches, cmd)
			}
		}

		// 转换为需要的格式
		result := make([][]rune, len(matches))
		for i, match := range matches {
			result[i] = []rune("/" + match)
		}

		return result, len(prefix) + 1
	}

	return nil, 0
}

// startCodeAssistant begins the interactive code assistant session
func startCodeAssistant() {
	client := llm.NewClient(Config)
	userStyle, aiStyle := createStyledPrinters()
	commands := setupSlashCommands()
	cmdStyle := color.New(color.FgYellow).Add(color.Bold)
	codeStyle := color.New(color.FgGreen)
	errorStyle := color.New(color.FgRed)

	// Show welcome message
	showCodeWelcomeMessage(Config)

	// 创建readline实例
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "Code Assistant > ",
		HistoryFile:     filepath.Join(os.TempDir(), "tama_code_history.txt"),
		AutoComplete:    completer{},
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})

	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		return
	}
	defer rl.Close()

	// Main interaction loop
	for {
		// 使用readline获取输入
		input, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				if len(input) == 0 {
					break
				} else {
					continue
				}
			} else if err == io.EOF {
				break
			}
			fmt.Printf("Error reading input: %v\n", err)
			break
		}

		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// 有效输入才显示和处理
		userStyle(input)

		// Check if this is a slash command
		if strings.HasPrefix(input, "/") {
			if handleSlashCommand(input, commands) {
				// 命令成功执行后添加到历史记录
				rl.SaveHistory(input)
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

			cmdStyle.Printf("\nDirectly executing: %s\n\n", cmdStr)

			err := executeCommand(cmdStr)
			if err != nil {
				fmt.Printf("Error executing command: %v\n", err)
			}

			// 命令成功执行后添加到历史记录
			rl.SaveHistory(input)
			continue
		}

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

		// Check if this is a code-related request
		codeActions, isCodeRequest := analyzeCodeRequest(client, input)
		if isCodeRequest && len(codeActions) > 0 {
			fmt.Println("\nProcessing code request, please wait...")

			// Execute code actions
			for _, action := range codeActions {
				switch action.Type {
				case "analyze":
					fmt.Printf("\nAnalyzing file: %s\n", action.FilePath)
					content, err := readFile(action.FilePath)
					if err != nil {
						errorStyle.Printf("Error reading file: %v\n", err)
						continue
					}

					// Send file content to LLM for analysis
					analysisPrompt := fmt.Sprintf("Please analyze this code from %s:\n\n```\n%s\n```\n\n%s",
						action.FilePath, content, action.Description)
					analysisResponse, err := client.SendMessage(analysisPrompt)
					if err != nil {
						errorStyle.Printf("Error analyzing code: %v\n", err)
						continue
					}

					aiStyle(analysisResponse)

				case "edit":
					fmt.Printf("\nEditing file: %s\n", action.FilePath)
					oldContent, err := readFile(action.FilePath)
					if err != nil {
						errorStyle.Printf("Error reading file: %v\n", err)
						continue
					}

					// Apply the edit
					err = writeFile(action.FilePath, action.Content)
					if err != nil {
						errorStyle.Printf("Error writing file: %v\n", err)
						continue
					}

					// Show diff
					showDiff(oldContent, action.Content, action.FilePath, codeStyle)
					fmt.Printf("\nSuccessfully edited %s\n", action.FilePath)

				case "create":
					fmt.Printf("\nCreating file: %s\n", action.FilePath)

					// Ensure directory exists
					dir := filepath.Dir(action.FilePath)
					if dir != "." {
						err := os.MkdirAll(dir, 0755)
						if err != nil {
							errorStyle.Printf("Error creating directory: %v\n", err)
							continue
						}
					}

					// Create the file
					err := writeFile(action.FilePath, action.Content)
					if err != nil {
						errorStyle.Printf("Error creating file: %v\n", err)
						continue
					}

					// Show the created content
					fmt.Printf("\nCreated %s with content:\n", action.FilePath)
					codeStyle.Printf("\n```\n%s\n```\n", action.Content)
				}
			}

			// Get a summary of the changes
			summaryPrompt := fmt.Sprintf("I've just made the following code changes based on your request:\n\n%s\n\nPlease provide a brief summary of what was done.",
				formatCodeActions(codeActions))
			response, err = client.SendMessage(summaryPrompt)
			if err != nil {
				errorStyle.Printf("Error getting summary: %v\n", err)
				continue
			}
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

		// 将有效消息添加到历史记录
		rl.SaveHistory(input)
	}
}

// showCodeWelcomeMessage displays the welcome message for the code assistant
func showCodeWelcomeMessage(cfg config.Config) {
	PrintLogo("Code")
	modelInfo := color.New(color.FgCyan)
	fmt.Println("AI-powered coding companion")
	modelInfo.Printf("Connected to %s model: %s\n", cfg.Defaults.Provider, cfg.Defaults.Model)
	fmt.Println("Type 'exit' or 'quit' to end the session")
	fmt.Println("Type '/help' to show available commands")
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
			fmt.Println("  @command  - Execute a command directly, e.g. @ls")
			fmt.Println("  /!command - Execute a command directly, e.g. /!pwd")
			fmt.Println("  ↑/↓       - Browse message history")
			return nil
		},
	}

	commands["config"] = SlashCommand{
		Name:        "config",
		Description: "Show the current configuration",
		Execute: func() error {
			showConfig()
			return nil
		},
	}

	commands["cd"] = SlashCommand{
		Name:        "cd",
		Description: "Change the current working directory",
		Execute: func() error {
			fmt.Println("Usage: /cd <directory>")
			fmt.Printf("Current directory: %s\n", getCurrentDirectory())
			return nil
		},
	}

	return commands
}

// handleSlashCommand processes commands that start with "/"
func handleSlashCommand(input string, commands map[string]SlashCommand) bool {
	// 检查是否是直接执行命令（以/!开头）
	if strings.HasPrefix(input, "/!") {
		// 移除/!前缀
		cmdStr := strings.TrimPrefix(input, "/!")
		cmdStr = strings.TrimSpace(cmdStr)

		if cmdStr == "" {
			fmt.Println("Error: No command specified after /!")
			return true
		}

		fmt.Printf("\nDirectly executing: %s\n\n", cmdStr)

		err := executeCommand(cmdStr)
		if err != nil {
			fmt.Printf("Error executing command: %v\n", err)
		}
		return true
	}

	// 处理 /cd 命令，支持带参数的情况
	if strings.HasPrefix(input, "/cd ") {
		// 提取目录参数
		dirPath := strings.TrimPrefix(input, "/cd ")
		dirPath = strings.TrimSpace(dirPath)

		if dirPath == "" {
			fmt.Println("Error: No directory specified")
			fmt.Printf("Current directory: %s\n", getCurrentDirectory())
			return true
		}

		// 处理 ~ 符号表示用户主目录
		if strings.HasPrefix(dirPath, "~") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				fmt.Printf("Error getting home directory: %v\n", err)
				return true
			}
			dirPath = filepath.Join(homeDir, strings.TrimPrefix(dirPath, "~"))
		}

		// 切换目录
		err := os.Chdir(dirPath)
		if err != nil {
			fmt.Printf("Error changing directory: %v\n", err)
			return true
		}

		fmt.Printf("Changed directory to: %s\n", getCurrentDirectory())
		return true
	}

	// Strip the leading "/"
	cmdName := strings.TrimPrefix(input, "/")
	// Handle arguments if any
	parts := strings.SplitN(cmdName, " ", 2)
	cmdName = parts[0]

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

// getCurrentDirectory returns the current working directory
func getCurrentDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return cwd
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

// analyzeCodeRequest determines if the user's request is code-related and what actions to take
func analyzeCodeRequest(client *llm.Client, input string) ([]CodeAction, bool) {
	// Create a prompt for the LLM to analyze if this is a code-related request
	prompt := fmt.Sprintf(`Analyze if the following request is related to code analysis, editing, or creation:
"%s"

If it is, return a JSON array of actions to take. Each action should have:
- "type": "analyze", "edit", or "create"
- "file_path": the path to the file to analyze/edit/create
- "content": the new content for edit/create actions (leave empty for analyze)
- "description": a brief description of what to do

For "edit" actions, you MUST include the complete file content in the "content" field.
NEVER return an empty or partial content for an edit action.
If the request is to format a file, read the file first and then format its content.

Example response for code-related request:
[
  {
    "type": "analyze",
    "file_path": "main.go",
    "content": "",
    "description": "Analyze the main function"
  },
  {
    "type": "edit",
    "file_path": "utils/helpers.go",
    "content": "package utils\n\nfunc NewHelper() {...}",
    "description": "Add a new helper function"
  }
]

If it's not a code-related request, return an empty array [].
Only respond with the JSON array, nothing else.`, input)

	// Send to LLM for analysis
	response, err := client.SendMessage(prompt)
	if err != nil {
		fmt.Printf("Error analyzing code request: %v\n", err)
		return nil, false
	}

	// Extract JSON array from response
	jsonArrayPattern := regexp.MustCompile(`(?s)\[.*\]`)
	match := jsonArrayPattern.FindString(response)
	if match == "" {
		return nil, false
	}

	// Parse the actions
	var actions []CodeAction
	err = json.Unmarshal([]byte(match), &actions)
	if err != nil {
		fmt.Printf("Error parsing code actions: %v\n", err)
		return nil, false
	}

	// Validate and fix actions
	for i, action := range actions {
		if action.Type == "edit" {
			// Ensure we have content for edit actions
			if action.Content == "" {
				// If content is empty, read the file content
				content, err := readFile(action.FilePath)
				if err != nil {
					fmt.Printf("Error reading file for edit action: %v\n", err)
					// Remove this action
					actions = append(actions[:i], actions[i+1:]...)
					continue
				}

				// Update the action with the file content
				actions[i].Content = content

				// If this is a format request, ask LLM to format the content
				if strings.Contains(strings.ToLower(action.Description), "format") {
					formatPrompt := fmt.Sprintf("Format the following Go code according to Go best practices:\n\n```go\n%s\n```\n\nReturn ONLY the formatted code, nothing else.", content)
					formattedContent, err := client.SendMessage(formatPrompt)
					if err != nil {
						fmt.Printf("Error formatting code: %v\n", err)
						continue
					}

					// Extract code from the response
					codePattern := regexp.MustCompile("(?s)```go\\s*(.*?)\\s*```")
					codeMatch := codePattern.FindStringSubmatch(formattedContent)
					if len(codeMatch) > 1 {
						actions[i].Content = codeMatch[1]
					} else {
						// If no code block found, use the whole response
						actions[i].Content = formattedContent
					}
				}
			}
		}
	}

	return actions, len(actions) > 0
}

// readFile reads the content of a file
func readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// writeFile writes content to a file
func writeFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// showDiff displays the differences between old and new content in git format
func showDiff(oldContent, newContent, filePath string, codeStyle *color.Color) {
	// Safety check to prevent empty content
	if newContent == "" {
		fmt.Printf("\nError: New content is empty. Aborting to prevent data loss.\n")
		return
	}

	// Split content into lines
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	fmt.Printf("\nChanges in %s:\n", filePath)
	fmt.Println("\n```diff")

	// Print file header in git format
	fmt.Printf("--- a/%s\n", filePath)
	fmt.Printf("+++ b/%s\n", filePath)

	// Create a simple diff by comparing lines
	// This is a simplified implementation that shows all changes
	fmt.Printf("@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines))

	// Use a simple diff algorithm to identify common lines
	// and show only the differences with some context
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldContent, newContent, false)
	dmp.DiffCleanupSemantic(diffs)

	// Process the diffs
	for _, diff := range diffs {
		lines := strings.Split(diff.Text, "\n")

		// Skip empty last line that comes from splitting
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		// Print each line with appropriate prefix
		for _, line := range lines {
			switch diff.Type {
			case diffmatchpatch.DiffInsert:
				fmt.Printf("+%s\n", line)
			case diffmatchpatch.DiffDelete:
				fmt.Printf("-%s\n", line)
			case diffmatchpatch.DiffEqual:
				fmt.Printf(" %s\n", line)
			}
		}
	}

	fmt.Println("```")
}

// formatCodeActions formats the code actions for the summary prompt
func formatCodeActions(actions []CodeAction) string {
	var result strings.Builder

	for i, action := range actions {
		result.WriteString(fmt.Sprintf("%d. %s %s", i+1, strings.ToUpper(action.Type), action.FilePath))
		if action.Description != "" {
			result.WriteString(fmt.Sprintf(" - %s", action.Description))
		}
		result.WriteString("\n")
	}

	return result.String()
}
