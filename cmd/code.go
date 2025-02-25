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

// CodeChangeResponse indicates the user's decision about a code change
type CodeChangeResponse int

const (
	Accept CodeChangeResponse = iota
	Reject
	Cancel
)

// GitIgnorePattern 表示单个 gitignore 模式
type GitIgnorePattern struct {
	Pattern   string
	Negated   bool
	Directory bool
}

// PathPromptPattern is the regex pattern to extract path and prompt
var PathPromptPattern = regexp.MustCompile(`^(?:@([^\s]+)\s+)?(.*?)$`)

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

	// 初始化readline
	rl := initializeReadline()
	if rl == nil {
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
			cmdHandled, needReset, _ := handleSlashCommand(input, commands)
			if cmdHandled {
				// 命令成功执行后添加到历史记录
				rl.SaveHistory(input)

				// 如果需要重置readline（执行了交互式命令）
				if needReset {
					rl.Close()
					rl = initializeReadline()
					if rl == nil {
						fmt.Println("Error reinitializing readline after command execution")
						return
					}
				}
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

			// 如果是交互式命令，重新初始化readline
			if isInteractiveTerminalCommand(cmdStr) {
				rl.Close()
				rl = initializeReadline()
				if rl == nil {
					fmt.Println("Error reinitializing readline after command execution")
					return
				}
			}
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

		// Check if this is a code-related request
		// Extract path and prompt from input if it has the @path format
		matches := PathPromptPattern.FindStringSubmatch(input)
		var targetPath string
		var promptText string
		var response string

		if len(matches) >= 3 {
			targetPath = matches[1]
			promptText = matches[2]
		} else {
			promptText = input
		}

		// If no specific path is given, use current directory
		if targetPath == "" {
			targetPath = "."
		}

		// Get absolute path for clarity
		absPath, err := filepath.Abs(targetPath)
		if err == nil {
			targetPath = absPath
		}

		// Check if target path exists
		fileInfo, err := os.Stat(targetPath)
		if err != nil {
			if promptText == input { // Only report error if path was explicitly specified
				fmt.Printf("Path does not exist or is not accessible: %s\n", targetPath)
				response, err = client.SendMessage(input)
			} else {
				errorStyle.Printf("Error accessing path %s: %v\n", targetPath, err)
				continue
			}
		} else if fileInfo.IsDir() {
			// Process directory
			fmt.Printf("\nAnalyzing directory: %s\n", targetPath)

			// 读取 .gitignore 模式
			gitignorePatterns, err := readGitIgnore(targetPath)
			if err != nil {
				fmt.Printf("Warning: Error reading .gitignore: %v\n", err)
				// 继续处理，但不应用 gitignore 规则
				gitignorePatterns = []GitIgnorePattern{}
			}

			dirContent, err := getDirectoryContent(targetPath, gitignorePatterns)
			if err != nil {
				errorStyle.Printf("Error reading directory: %v\n", err)
				continue
			}

			// Send directory content to LLM with the prompt
			analysisPrompt := fmt.Sprintf("Analyze this directory structure and the following request:\n\nDirectory: %s\n\n%s\n\nRequest: %s",
				targetPath, dirContent, promptText)
			response, err = client.SendMessage(analysisPrompt)
			if err != nil {
				errorStyle.Printf("Error analyzing directory: %v\n", err)
				continue
			}
		} else {
			// Process file
			fmt.Printf("\nAnalyzing file: %s\n", targetPath)

			// 检查是否应该忽略此文件
			gitignorePatterns, _ := readGitIgnore(filepath.Dir(targetPath))
			if shouldIgnore(targetPath, filepath.Dir(targetPath), gitignorePatterns) {
				fmt.Printf("Warning: This file is listed in .gitignore. Analysis may not be relevant.\n")
			}

			content, err := readFile(targetPath)
			if err != nil {
				errorStyle.Printf("Error reading file: %v\n", err)
				continue
			}

			// Send file content to LLM with the prompt
			analysisPrompt := fmt.Sprintf("Analyze this file and the following request:\n\nFile: %s\n\n```\n%s\n```\n\nRequest: %s",
				targetPath, content, promptText)

			// Check if this is a request to modify code
			if containsModificationKeywords(promptText) {
				// For requests that might modify the code
				modifiedContent, err := processCodeModification(client, targetPath, content, promptText)
				if err != nil {
					errorStyle.Printf("Error processing code modification: %v\n", err)
					continue
				}

				// If there are changes, show them and ask for confirmation
				if modifiedContent != content {
					fmt.Println("\nProposed changes:")
					showDiff(content, modifiedContent, targetPath, codeStyle)

					// Ask for confirmation
					userResponse := promptForConfirmation(rl)
					if userResponse == Accept {
						// Apply the changes
						err = writeFile(targetPath, modifiedContent)
						if err != nil {
							errorStyle.Printf("Error writing file: %v\n", err)
							continue
						}
						fmt.Printf("\nChanges applied to %s\n", targetPath)
						// Get a summary of the changes
						summaryPrompt := fmt.Sprintf("I've just modified %s based on this request: \"%s\". Please provide a brief summary of what was done.",
							targetPath, promptText)
						response, err = client.SendMessage(summaryPrompt)
						if err != nil {
							errorStyle.Printf("Error summarizing changes: %v\n", err)
							continue
						}
					} else if userResponse == Reject {
						fmt.Println("\nChanges were rejected.")
						response = "Changes were rejected."
					} else { // Cancel
						fmt.Println("\nOperation cancelled.")
						continue
					}
				} else {
					// No changes were needed
					response, err = client.SendMessage(analysisPrompt)
					if err != nil {
						errorStyle.Printf("Error analyzing file: %v\n", err)
						continue
					}
				}
			} else {
				// Regular file analysis
				response, err = client.SendMessage(analysisPrompt)
				if err != nil {
					errorStyle.Printf("Error analyzing file: %v\n", err)
					continue
				}
			}
		}

		// Check for errors in LLM response
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

// initializeReadline 创建并初始化readline实例
func initializeReadline() *readline.Instance {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "Code Assistant > ",
		HistoryFile:     filepath.Join(os.TempDir(), "tama_code_history.txt"),
		AutoComplete:    completer{},
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})

	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		return nil
	}

	return rl
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
			fmt.Println("  /!command - Execute a command directly, or use @command, such as @ls")
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
// 返回值：
// - bool: 是否处理了命令
// - bool: 是否需要重新初始化readline（执行了交互式终端命令）
// - string: 执行的命令（如果是交互式命令）
func handleSlashCommand(input string, commands map[string]SlashCommand) (bool, bool, string) {
	// 检查是否是直接执行命令（以/!开头）
	if strings.HasPrefix(input, "/!") {
		// 移除/!前缀
		cmdStr := strings.TrimPrefix(input, "/!")
		cmdStr = strings.TrimSpace(cmdStr)

		if cmdStr == "" {
			fmt.Println("Error: No command specified after /!")
			return true, false, ""
		}

		fmt.Printf("\nDirectly executing: %s\n\n", cmdStr)

		err := executeCommand(cmdStr)
		if err != nil {
			fmt.Printf("Error executing command: %v\n", err)
		}

		// 检查是否执行了交互式命令
		needReset := isInteractiveTerminalCommand(cmdStr)
		return true, needReset, cmdStr
	}

	// 处理 /cd 命令，支持带参数的情况
	if strings.HasPrefix(input, "/cd ") {
		// 提取目录参数
		dirPath := strings.TrimPrefix(input, "/cd ")
		dirPath = strings.TrimSpace(dirPath)

		if dirPath == "" {
			fmt.Println("Error: No directory specified")
			fmt.Printf("Current directory: %s\n", getCurrentDirectory())
			return true, false, ""
		}

		// 处理 ~ 符号表示用户主目录
		if strings.HasPrefix(dirPath, "~") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				fmt.Printf("Error getting home directory: %v\n", err)
				return true, false, ""
			}
			dirPath = filepath.Join(homeDir, strings.TrimPrefix(dirPath, "~"))
		}

		// 切换目录
		err := os.Chdir(dirPath)
		if err != nil {
			fmt.Printf("Error changing directory: %v\n", err)
			return true, false, ""
		}

		fmt.Printf("Changed directory to: %s\n", getCurrentDirectory())
		return true, false, ""
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
		return true, false, ""
	}

	fmt.Printf("Unknown command: /%s\n", cmdName)
	fmt.Println("Type /help for available commands")
	return true, false, ""
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

	// 只设置最基本的环境变量，避免干扰终端输出
	cmd.Env = os.Environ()

	// Connect command's stdout and stderr directly to our process stdout and stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// 检查是否是交互式终端命令
	isInteractive := isInteractiveTerminalCommand(cmdStr)

	// 如果是交互式命令，保存终端状态
	var savedTermSettings []byte
	if isInteractive {
		sttyCmd := exec.Command("stty", "-g")
		sttyCmd.Stdin = os.Stdin
		var err error
		savedTermSettings, err = sttyCmd.Output()
		if err != nil {
			fmt.Printf("Warning: Failed to save terminal settings: %v\n", err)
		}
	}

	// Execute the command
	err := cmd.Run()

	// 如果是交互式命令并且成功保存了终端设置，恢复终端状态
	if isInteractive && len(savedTermSettings) > 0 {
		fmt.Println("\nRestoring terminal state...")

		// 使用保存的设置恢复终端
		restoreCmd := exec.Command("stty", string(savedTermSettings))
		restoreCmd.Stdin = os.Stdin
		if restoreErr := restoreCmd.Run(); restoreErr != nil {
			fmt.Printf("Warning: Failed to restore terminal settings: %v\n", restoreErr)
			// 如果无法恢复，使用通用的stty sane作为备选
			resetTerminal()
		}
	}

	return err
}

// isInteractiveTerminalCommand 检查命令是否是可能会修改终端状态的交互式程序
func isInteractiveTerminalCommand(cmdStr string) bool {
	cmdStr = strings.TrimSpace(cmdStr)
	cmdLower := strings.ToLower(cmdStr)

	// 提取主命令（去除参数）
	mainCmd := cmdLower
	if spaceIndex := strings.Index(mainCmd, " "); spaceIndex > 0 {
		mainCmd = mainCmd[:spaceIndex]
	}

	// 检查是否是已知的交互式终端程序
	interactiveCommands := map[string]bool{
		"vim": true, "vi": true, "nvim": true,
		"nano": true, "emacs": true,
		"less": true, "more": true,
		"top": true, "htop": true,
		"man": true, "pico": true,
		"screen": true, "tmux": true,
		"joe": true, "jed": true,
		"mutt": true, "pine": true,
		"mc": true, // Midnight Commander
	}

	return interactiveCommands[mainCmd]
}

// resetTerminal 重置终端状态
func resetTerminal() {
	// 尝试使用多种方法重置终端状态
	methods := []struct {
		cmd  string
		args []string
	}{
		{"stty", []string{"sane"}},           // 最基本的终端重置
		{"tput", []string{"reset"}},          // 使用tput重置
		{"reset", []string{}},                // 使用reset命令
		{"stty", []string{"echo"}},           // 确保回显打开
		{"stty", []string{"icanon", "echo"}}, // 打开规范模式和回显
	}

	for _, method := range methods {
		cmd := exec.Command(method.cmd, method.args...)
		cmd.Stdin = os.Stdin
		// 忽略输出，避免干扰界面
		cmd.Stdout = nil
		cmd.Stderr = nil
		// 忽略错误，尝试下一种方法
		_ = cmd.Run()
	}

	// 清空输入缓冲区
	exec.Command("bash", "-c", "while read -t 0.1; do :; done").Run()

	// 重新显示提示符
	fmt.Print("\r")
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

// getDirectoryContent returns a formatted string with directory contents
func getDirectoryContent(dirPath string, gitignorePatterns []GitIgnorePattern) (string, error) {
	var result strings.Builder

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files/directories
		if strings.HasPrefix(filepath.Base(path), ".") && path != dirPath {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查是否应该根据 .gitignore 规则忽略此文件
		if shouldIgnore(path, dirPath, gitignorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Calculate relative path from the target directory
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create proper indentation based on directory depth
		indent := strings.Repeat("  ", strings.Count(relPath, string(filepath.Separator)))

		// Add directory indicator for directories
		if info.IsDir() {
			result.WriteString(fmt.Sprintf("%s📁 %s\n", indent, filepath.Base(path)))
		} else {
			// For files, also show size
			sizeStr := formatFileSize(info.Size())
			result.WriteString(fmt.Sprintf("%s📄 %s (%s)\n", indent, filepath.Base(path), sizeStr))
		}

		return nil
	})

	return result.String(), err
}

// formatFileSize returns a human-readable file size
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// containsModificationKeywords checks if the prompt contains keywords indicating code modification
func containsModificationKeywords(prompt string) bool {
	prompt = strings.ToLower(prompt)
	keywords := []string{
		"fix", "change", "modify", "update", "add", "implement", "edit",
		"remove", "delete", "create", "format", "refactor", "optimize",
		"improve", "rewrite", "clean",
	}

	for _, keyword := range keywords {
		if strings.Contains(prompt, keyword) {
			return true
		}
	}

	return false
}

// processCodeModification handles requests to modify code
func processCodeModification(client *llm.Client, filePath, content, prompt string) (string, error) {
	// Construct a prompt for code modification
	modificationPrompt := fmt.Sprintf(
		"I have a file %s with the following content:\n\n```\n%s\n```\n\n"+
			"I want to: %s\n\n"+
			"Please provide the complete modified file content that addresses this request. "+
			"Return ONLY the modified code without any explanation or markdown, as I will directly use your output.",
		filePath, content, prompt,
	)

	// Send to LLM for modification
	response, err := client.SendMessage(modificationPrompt)
	if err != nil {
		return "", err
	}

	// Extract code from response (if LLM wrapped it in code blocks)
	codePattern := regexp.MustCompile("(?s)```(?:\\w+)?\\s*(.+?)\\s*```")
	matches := codePattern.FindStringSubmatch(response)
	if len(matches) > 1 {
		return matches[1], nil
	}

	// If no code blocks, return the raw response
	return response, nil
}

// promptForConfirmation asks the user to confirm code changes
func promptForConfirmation(rl *readline.Instance) CodeChangeResponse {
	originalPrompt := rl.Config.Prompt
	defer func() {
		rl.SetPrompt(originalPrompt)
	}()

	rl.SetPrompt("Apply changes? (yes/no/cancel): ")

	for {
		input, err := rl.Readline()
		if err != nil {
			return Cancel
		}

		input = strings.ToLower(strings.TrimSpace(input))

		if input == "yes" || input == "y" {
			return Accept
		} else if input == "no" || input == "n" {
			return Reject
		} else if input == "cancel" || input == "c" {
			return Cancel
		}

		fmt.Println("Please enter 'yes', 'no', or 'cancel'")
	}
}

// readGitIgnore 读取并解析 .gitignore 文件
func readGitIgnore(rootDir string) ([]GitIgnorePattern, error) {
	var patterns []GitIgnorePattern

	// 查找 .gitignore 文件
	gitignorePath := filepath.Join(rootDir, ".gitignore")

	// 检查文件是否存在
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		// 如果文件不存在，返回空的模式列表
		return patterns, nil
	}

	// 读取 .gitignore 文件
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		return nil, fmt.Errorf("error reading .gitignore: %v", err)
	}

	// 按行解析
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		// 忽略空行和注释
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析模式
		pattern := GitIgnorePattern{}

		// 检查是否是否定模式 (以 ! 开头)
		if strings.HasPrefix(line, "!") {
			pattern.Negated = true
			line = line[1:]
		}

		// 检查是否是目录 (以 / 结尾)
		if strings.HasSuffix(line, "/") {
			pattern.Directory = true
			line = line[:len(line)-1]
		}

		// 删除前导斜杠，gitignore 模式是相对于 git 仓库根目录的
		if strings.HasPrefix(line, "/") {
			line = line[1:]
		}

		pattern.Pattern = line
		patterns = append(patterns, pattern)
	}

	return patterns, nil
}

// shouldIgnore 判断文件是否应该被忽略
func shouldIgnore(path string, rootDir string, gitignorePatterns []GitIgnorePattern) bool {
	// 获取相对路径
	relPath, err := filepath.Rel(rootDir, path)
	if err != nil {
		// 如果获取相对路径出错，保守起见不忽略
		return false
	}

	// 替换Windows路径分隔符为Unix风格
	relPath = filepath.ToSlash(relPath)

	// 文件的基本名称（用于部分匹配）
	baseName := filepath.Base(relPath)

	// 获取文件信息以检查是否是目录
	fileInfo, err := os.Stat(path)
	isDir := err == nil && fileInfo.IsDir()

	// 默认不忽略
	ignored := false

	// 遍历所有模式
	for _, pattern := range gitignorePatterns {
		// 准备匹配的模式
		patternToUse := pattern.Pattern

		// 进行模式特殊处理
		isMatched := false

		// 检查是否是简单的基本名称匹配
		if !strings.Contains(patternToUse, "/") {
			// 如果模式中没有 /，则匹配任何目录下的文件名
			isMatched = matchGitignorePattern(baseName, patternToUse) ||
				matchGitignorePattern(relPath, patternToUse)
		} else {
			// 如果有 /，则按照 gitignore 的规则匹配
			isMatched = matchGitignorePattern(relPath, patternToUse)
		}

		// 处理目录模式的特殊情况
		if pattern.Directory && isDir {
			dirPath := relPath + "/"
			isMatched = isMatched || matchGitignorePattern(dirPath, patternToUse+"/")
		}

		// 如果匹配成功
		if isMatched {
			// 根据是否是否定模式来设置忽略状态
			if pattern.Negated {
				ignored = false
			} else {
				ignored = true
			}
		}
	}

	return ignored
}

// matchGitignorePattern 使用类似 gitignore 的规则匹配模式
func matchGitignorePattern(name, pattern string) bool {
	// 处理 gitignore 中的特殊通配符

	// 处理 ** 用于匹配任意目录层级
	pattern = strings.Replace(pattern, "**", ".*", -1)

	// 处理 * 用于匹配除了 / 之外的任意字符
	pattern = strings.Replace(pattern, "*", "[^/]*", -1)

	// 处理 ? 用于匹配单个字符
	pattern = strings.Replace(pattern, "?", ".", -1)

	// 转换成正则表达式
	pattern = "^" + pattern + "$"

	// 使用正则表达式匹配
	matched, err := regexp.MatchString(pattern, name)
	return err == nil && matched
}
