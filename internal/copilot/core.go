package copilot

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/warm3snow/tama/internal/completion"
	"github.com/warm3snow/tama/internal/config"
	"github.com/warm3snow/tama/internal/llm"
	"github.com/warm3snow/tama/internal/machine"
	"github.com/warm3snow/tama/internal/tools"
	"github.com/warm3snow/tama/internal/workspace"
)

// Change represents a single file change
type Change struct {
	FilePath    string
	Description string
	Timestamp   time.Time
	Backup      string // Path to backup file
	Status      string // Status of the change (e.g., "modified", "added", "deleted")
}

// TaskState represents the state of a task
type TaskState struct {
	Description string
	StartTime   time.Time
	EndTime     time.Time
	Status      string // "in_progress", "completed", "failed", "rejected"
	Changes     []Change
}

// AgentState represents the current state of the agent
type AgentState struct {
	Goal           string
	CurrentTask    *TaskState
	CompletedTasks []TaskState
	StartTime      time.Time
	LastActivity   time.Time
}

// Copilot orchestrates the interaction between user, LLM, and tools
type Copilot struct {
	ctx       context.Context
	cancel    context.CancelFunc
	machine   *machine.Context
	llm       *llm.Client
	tools     *tools.Registry
	workspace *workspace.Manager
	userStyle *color.Color
	aiStyle   *color.Color
	cmdStyle  *color.Color
	agent     *AgentState
	mu        sync.RWMutex
}

// New creates a new Copilot instance
func New(cfg config.Config) *Copilot {
	ctx, cancel := context.WithCancel(context.Background())

	// Create workspace manager
	ws := workspace.NewManager()

	// Create tool registry and register tools
	tr := tools.NewRegistry()
	tr.RegisterTool(tools.NewEditFileTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewReadFileTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewGrepSearchTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewRunTerminalTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewGitTool(ws.GetWorkspacePath()))

	// Create style colors
	userStyle := color.New(color.FgGreen).Add(color.Bold)
	aiStyle := color.New(color.FgBlue)
	cmdStyle := color.New(color.FgYellow).Add(color.Bold)

	return &Copilot{
		ctx:       ctx,
		cancel:    cancel,
		machine:   machine.NewContext(),
		llm:       llm.NewClient(cfg),
		tools:     tr,
		workspace: ws,
		userStyle: userStyle,
		aiStyle:   aiStyle,
		cmdStyle:  cmdStyle,
	}
}

// StartInteractiveChat starts an interactive chat session
func (c *Copilot) StartInteractiveChat() error {
	// Show welcome message
	c.showWelcomeMessage()

	// Initialize readline
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[32m>\033[0m ",
		HistoryFile:     "/tmp/tama_history.txt",
		AutoComplete:    completion.NewReadlineCompleter([]string{"/help", "/reset", "/exit"}),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("error initializing readline: %v", err)
	}
	defer rl.Close()

	// Main interaction loop
	for {
		// Get input using readline
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
			return fmt.Errorf("error reading input: %v", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// Handle special commands
		if c.handleSpecialCommands(input) {
			continue
		}

		// Display user input
		c.userStyle.Printf("\nYou: %s\n", input)

		// Process the input
		respChan, err := c.ProcessPrompt(input)
		if err != nil {
			c.cmdStyle.Printf("Error: %v\n", err)
			continue
		}

		// Print AI response
		c.aiStyle.Print("\nAI: ")
		for chunk := range respChan {
			fmt.Print(chunk)
		}
		fmt.Print("\n\n")

		// Add to readline history
		rl.SaveHistory(input)
	}

	return nil
}

// handleSpecialCommands handles special commands like /help and /reset
func (c *Copilot) handleSpecialCommands(input string) bool {
	switch input {
	case "/help":
		c.showHelpMessage()
		return true
	case "/reset":
		c.llm.ResetConversation()
		c.cmdStyle.Printf("\nConversation has been reset.\n")
		return true
	}
	return false
}

// showWelcomeMessage displays the welcome message
func (c *Copilot) showWelcomeMessage() {
	modelInfo := color.New(color.FgCyan)
	fmt.Println("Welcome to the Tama AI Assistant")
	modelInfo.Printf("Connected to %s model: %s\n",
		c.llm.GetProvider(),
		c.llm.GetModel())
	fmt.Println("Type 'exit' or 'quit' to end the session.")
	fmt.Println("Type '/help' to see available commands.")
}

// showHelpMessage displays the help message
func (c *Copilot) showHelpMessage() {
	fmt.Println("\nAvailable commands:")
	c.cmdStyle.Print("  /help")
	fmt.Println(" - Show this help message")
	c.cmdStyle.Print("  /reset")
	fmt.Println(" - Reset the conversation")
	c.cmdStyle.Print("  exit")
	fmt.Println(" or quit - End the session")
}

// ProcessPrompt handles a user prompt and returns a streamed response
func (c *Copilot) ProcessPrompt(prompt string) (<-chan string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create response channel
	respChan := make(chan string)

	// Get workspace context
	wsContext := c.workspace.GetSummary()

	// Get available tools descriptions
	toolDescs := c.tools.GetToolDescriptions()

	// Create system message with tools
	systemMsg := fmt.Sprintf(`You are a powerful AI coding assistant. You have access to various tools to help with coding tasks.
Your responses should follow this format:

1. First, explain your thinking process and what you plan to do
2. Then, if you need to create or modify files, explain the changes you'll make
3. Finally, execute the necessary actions using the tools available to you

When writing code:
- Always add necessary imports
- Ensure the code is complete and can run
- Follow best practices and conventions
- Add helpful comments to explain complex logic

Important:
- Always use edit_file tool to make code changes
- Always use git tool to show changes
- Start each response with "Task: <brief task description>"

Available tools:
%s

Current workspace: %s
`, formatTools(toolDescs), wsContext["root"])

	// Add system message to LLM
	c.llm.AddSystemMessage(systemMsg)

	// Process in background
	go func() {
		defer close(respChan)

		// Create callback for streaming responses
		callback := func(chunk string) {
			// Check if it's a tool call
			if toolCall := c.tools.ParseToolCall(chunk); toolCall != nil {
				// Execute tool and get result
				result := toolCall.Execute(c.ctx)

				// If it's a git diff operation, print the result directly
				if strings.Contains(chunk, `"tool":"git"`) && strings.Contains(chunk, `"operation":"diff"`) {
					select {
					case <-c.ctx.Done():
						return
					case respChan <- "\nChanges made:\n" + result:
					}
					return
				}

				// For other tools, only print errors
				if strings.Contains(result, "error") || strings.Contains(result, "Error") {
					select {
					case <-c.ctx.Done():
						return
					case respChan <- fmt.Sprintf("\nError: %s\n", result):
					}
				} else {
					// Print success message for edit operations
					if strings.Contains(chunk, `"tool":"edit_file"`) {
						select {
						case <-c.ctx.Done():
							return
						case respChan <- "\nFile updated successfully.\n":
						}
						// After file edit, show git diff
						if gitTool := c.tools.GetTool("git"); gitTool != nil {
							if diff, err := gitTool.Execute(c.ctx, map[string]interface{}{
								"operation": "diff",
							}); err == nil && diff != "No changes detected" {
								select {
								case <-c.ctx.Done():
									return
								case respChan <- "\nChanges made:\n" + diff:
								}
							}
						}
					}
				}
				return
			}

			// Stream response to user
			select {
			case <-c.ctx.Done():
				return
			case respChan <- chunk:
			}
		}

		// Send message with streaming
		response, err := c.llm.SendMessageWithCallback(prompt, callback)
		if err != nil {
			respChan <- "Error: " + err.Error()
			return
		}

		// Update conversation with final response
		c.llm.UpdateConversation(prompt, response)
	}()

	return respChan, nil
}

// formatTools formats tool descriptions into a readable string
func formatTools(tools []map[string]string) string {
	var sb strings.Builder
	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", tool["name"], tool["description"]))
	}
	return sb.String()
}

// Shutdown gracefully shuts down the copilot
func (c *Copilot) Shutdown() {
	c.cancel()
	c.llm.Close()
	c.workspace.Cleanup()
}

// GetContext returns the copilot's context
func (c *Copilot) GetContext() context.Context {
	return c.ctx
}

// AddSystemMessage adds a system message to the conversation
func (c *Copilot) AddSystemMessage(message string) {
	c.llm.AddSystemMessage(message)
}

// GetFileContext retrieves the content of a file
func (c *Copilot) GetFileContext(filePath string) (string, error) {
	tool := tools.NewReadFileTool(c.workspace.GetWorkspacePath())
	return tool.Execute(c.ctx, map[string]interface{}{
		"path": filePath,
	})
}

// GetCodebaseContext retrieves information about the codebase structure
func (c *Copilot) GetCodebaseContext(depth int) (string, error) {
	// Use grep tool to search through the codebase
	tool := tools.NewGrepSearchTool(c.workspace.GetWorkspacePath())
	return tool.Execute(c.ctx, map[string]interface{}{
		"pattern": ".",
		"depth":   depth,
	})
}

// GetGitContext retrieves git-related information
func (c *Copilot) GetGitContext(command string) (string, error) {
	// Use terminal tool to run git commands
	tool := tools.NewRunTerminalTool(c.workspace.GetWorkspacePath())
	return tool.Execute(c.ctx, map[string]interface{}{
		"command": fmt.Sprintf("git %s", command),
	})
}

// SetProjectPath sets the project path for the workspace
func (c *Copilot) SetProjectPath(path string) error {
	return c.workspace.SetWorkspacePath(path)
}

// StartAgentMode starts the AI agent mode with a specific goal
func (c *Copilot) StartAgentMode(goal string) error {
	c.mu.Lock()
	c.agent = &AgentState{
		Goal:           goal,
		StartTime:      time.Now(),
		LastActivity:   time.Now(),
		CompletedTasks: make([]TaskState, 0),
	}
	c.mu.Unlock()

	// Show welcome message
	c.cmdStyle.Printf("\nStarting AI Agent mode with goal: %s\n\n", goal)

	// Create system message for agent mode
	systemMsg := fmt.Sprintf(`You are a powerful AI coding assistant working on the following goal:

%s

Follow these steps for each task:

1. ANALYZE: First, analyze the current state and explain your thinking process
2. PLAN: Describe what changes you plan to make and why
3. IMPLEMENT: Make the necessary code changes
4. VERIFY: Explain how the changes achieve the goal

When writing code:
- Always add necessary imports
- Ensure the code is complete and can run
- Follow best practices and conventions
- Add helpful comments to explain complex logic
- Start each response with "Task: <brief task description>"

Available tools:
%s

Current workspace: %s
`, goal, formatTools(c.tools.GetToolDescriptions()), c.workspace.GetSummary()["root"])

	// Add system message to LLM
	c.llm.AddSystemMessage(systemMsg)

	// Start the agent loop
	return c.runAgentLoop()
}

// runAgentLoop runs the main agent loop
func (c *Copilot) runAgentLoop() error {
	for {
		// Get next action from LLM
		respChan, err := c.ProcessPrompt("Continue working on the goal. What's your next step?")
		if err != nil {
			return fmt.Errorf("agent error: %v", err)
		}

		// Process response and extract task description
		var response strings.Builder
		taskDesc := ""
		for chunk := range respChan {
			response.WriteString(chunk)
			// Extract task description from the first line
			if taskDesc == "" && strings.HasPrefix(chunk, "Task:") {
				taskDesc = strings.TrimSpace(strings.TrimPrefix(chunk, "Task:"))
			}
			// Also print the chunk to show real-time progress
			fmt.Print(chunk)
		}

		// Update agent state with new task
		c.mu.Lock()
		if c.agent.CurrentTask != nil {
			// Complete the previous task
			c.agent.CurrentTask.EndTime = time.Now()
			c.agent.CurrentTask.Status = "completed"
			c.agent.CompletedTasks = append(c.agent.CompletedTasks, *c.agent.CurrentTask)
		}
		c.agent.CurrentTask = &TaskState{
			Description: taskDesc,
			StartTime:   time.Now(),
			Status:      "in_progress",
			Changes:     make([]Change, 0),
		}
		c.agent.LastActivity = time.Now()
		c.mu.Unlock()

		// Show changes if any were made
		gitTool := tools.NewGitTool(c.workspace.GetWorkspacePath())
		diff, err := gitTool.Execute(c.ctx, map[string]interface{}{"operation": "diff"})
		if err != nil {
			c.cmdStyle.Printf("\nError getting changes: %v\n", err)
		} else if diff != "No changes detected" {
			fmt.Print(diff) // Print the colored diff output with file status
		}

		// Create a backup of changed files
		if err := c.backupChangedFiles(); err != nil {
			c.cmdStyle.Printf("\nWarning: Failed to create backup: %v\n", err)
		}

		// Ask user for action
		c.cmdStyle.Print("\nWhat would you like to do?\n")
		c.cmdStyle.Println("  [a]ccept     - Accept and commit the current changes")
		c.cmdStyle.Println("  [r]eject     - Reject and rollback the current changes")
		c.cmdStyle.Println("  [A]ll        - Reject all changes and exit")
		c.cmdStyle.Println("  [d]iff       - Show detailed changes")
		c.cmdStyle.Println("  [s]ummary    - Show task summary")
		c.cmdStyle.Println("  [p]rogress   - Show overall progress")
		c.cmdStyle.Println("  [q]uit       - Exit agent mode")
		c.cmdStyle.Print("\nEnter your choice: ")

		rl, err := readline.New("")
		if err != nil {
			return err
		}
		input, err := rl.Readline()
		if err != nil {
			return err
		}

		switch strings.ToLower(strings.TrimSpace(input)) {
		case "a", "accept":
			// Add the change to history before committing
			c.mu.Lock()
			if c.agent.CurrentTask != nil {
				c.agent.CurrentTask.Status = "completed"
			}
			c.mu.Unlock()

			// Commit changes
			if _, err := gitTool.Execute(c.ctx, map[string]interface{}{
				"operation": "commit",
				"message":   fmt.Sprintf("Auto commit: %s", taskDesc),
			}); err != nil {
				c.cmdStyle.Printf("Failed to commit changes: %v\n", err)
			} else {
				c.cmdStyle.Println("Changes committed successfully.")
			}
			continue

		case "r", "reject":
			// Update task status
			c.mu.Lock()
			if c.agent.CurrentTask != nil {
				c.agent.CurrentTask.Status = "rejected"
			}
			c.mu.Unlock()

			// Reset current changes
			if _, err := gitTool.Execute(c.ctx, map[string]interface{}{"operation": "reset"}); err != nil {
				c.cmdStyle.Printf("Failed to reset changes: %v\n", err)
			} else {
				c.cmdStyle.Println("Changes reset successfully.")
			}
			continue

		case "d", "diff":
			// Show detailed diff
			if diff, err := gitTool.Execute(c.ctx, map[string]interface{}{"operation": "diff"}); err == nil {
				fmt.Print("\nDetailed changes:\n", diff)
			}
			continue

		case "s", "summary":
			c.showTaskSummary()
			continue

		case "p", "progress":
			c.showProgress()
			continue

		case "A", "all":
			// Update all incomplete tasks as rejected
			c.mu.Lock()
			if c.agent.CurrentTask != nil {
				c.agent.CurrentTask.Status = "rejected"
			}
			c.mu.Unlock()

			// Reset all changes and exit
			if _, err := gitTool.Execute(c.ctx, map[string]interface{}{"operation": "reset"}); err != nil {
				c.cmdStyle.Printf("Failed to reset changes: %v\n", err)
			} else {
				c.cmdStyle.Println("All changes reset successfully.")
			}
			return nil

		case "q", "quit":
			return nil

		default:
			c.cmdStyle.Println("Invalid input. Please try again.")
			continue
		}
	}
}

// backupChangedFiles creates backups of modified files
func (c *Copilot) backupChangedFiles() error {
	// Get list of modified files
	gitTool := tools.NewRunTerminalTool(c.workspace.GetWorkspacePath())
	output, err := gitTool.Execute(c.ctx, map[string]interface{}{
		"command": "git status --porcelain",
	})
	if err != nil {
		return fmt.Errorf("failed to get modified files: %v", err)
	}

	// Create backup directory if it doesn't exist
	backupDir := filepath.Join(c.workspace.GetWorkspacePath(), ".tama", "backups",
		time.Now().Format("20060102_150405"))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %v", err)
	}

	// Process each modified file
	for _, line := range strings.Split(output, "\n") {
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		file := strings.TrimSpace(line[3:])

		// Skip untracked files
		if status == "??" {
			continue
		}

		// Copy file to backup directory
		srcPath := filepath.Join(c.workspace.GetWorkspacePath(), file)
		dstPath := filepath.Join(backupDir, file)

		// Create destination directory if needed
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return fmt.Errorf("failed to create backup subdirectory for %s: %v", file, err)
		}

		// Copy file
		if err := c.copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to backup %s: %v", file, err)
		}
	}

	return nil
}

// copyFile copies a file from src to dst
func (c *Copilot) copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}

// showTaskSummary displays a summary of the current task and changes
func (c *Copilot) showTaskSummary() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.cmdStyle.Println("\nTask Summary:")
	fmt.Printf("Goal: %s\n", c.agent.Goal)
	fmt.Printf("Current Task: %s\n", c.agent.CurrentTask.Description)
	fmt.Printf("Start Time: %s\n", c.agent.CurrentTask.StartTime.Format(time.RFC3339))
	fmt.Printf("Duration: %s\n", time.Since(c.agent.CurrentTask.StartTime).Round(time.Second))

	if len(c.agent.CompletedTasks) > 0 {
		fmt.Println("\nCompleted Tasks:")
		for i, task := range c.agent.CompletedTasks {
			duration := task.EndTime.Sub(task.StartTime).Round(time.Second)
			fmt.Printf("%d. %s (%s) - %s\n", i+1, task.Description, task.Status, duration)
		}
	}
}

// showProgress displays overall progress information
func (c *Copilot) showProgress() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.cmdStyle.Println("\nOverall Progress:")
	fmt.Printf("Goal: %s\n", c.agent.Goal)
	fmt.Printf("Started: %s\n", c.agent.StartTime.Format(time.RFC3339))
	fmt.Printf("Duration: %s\n", time.Since(c.agent.StartTime).Round(time.Second))
	fmt.Printf("Last Activity: %s\n", time.Since(c.agent.LastActivity).Round(time.Second))

	if len(c.agent.CompletedTasks) > 0 {
		fmt.Println("\nCompleted Tasks:")
		for i, task := range c.agent.CompletedTasks {
			duration := task.EndTime.Sub(task.StartTime).Round(time.Second)
			fmt.Printf("%d. %s (%s) - %s\n", i+1, task.Description, task.Status, duration)
		}
	}

	if c.agent.CurrentTask != nil {
		fmt.Printf("\nCurrent Task: %s\n", c.agent.CurrentTask.Description)
		fmt.Printf("Status: %s\n", c.agent.CurrentTask.Status)
		duration := time.Since(c.agent.CurrentTask.StartTime).Round(time.Second)
		fmt.Printf("Duration: %s\n", duration)
	}
}
