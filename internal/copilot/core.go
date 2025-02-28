package copilot

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

// DecisionPhase represents the current phase of decision making
type DecisionPhase string

const (
	PhaseAnalysis     DecisionPhase = "analysis"     // Initial analysis of the prompt
	PhaseContext      DecisionPhase = "context"      // Context gathering
	PhaseModification DecisionPhase = "modification" // Code modification
	PhaseVerification DecisionPhase = "verification" // Verification and testing
)

// Decision represents an LLM's decision about how to handle the prompt
type Decision struct {
	Phase     DecisionPhase
	Action    string
	Reasoning string
	Context   []string // Required context files/directories
	Tools     []string // Required tools
	Changes   []Change // Proposed changes
}

// ConfirmationStatus represents the user's response to proposed changes
type ConfirmationStatus string

const (
	StatusPending  ConfirmationStatus = "pending"
	StatusAccepted ConfirmationStatus = "accepted"
	StatusRejected ConfirmationStatus = "rejected"
)

// ChangeConfirmation represents a user's confirmation of changes
type ChangeConfirmation struct {
	Status    ConfirmationStatus
	Changes   []Change
	Timestamp time.Time
	Comment   string
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

	// Create machine context
	machineCtx := machine.NewContext()

	// Create tool registry and register tools
	tr := tools.NewRegistry()
	tr.RegisterTool(tools.NewGrepSearchTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewRunTerminalTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewGitTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewFileSystemTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewLanguageDetector(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewLinterTool(ws.GetWorkspacePath()))

	// Create style colors
	userStyle := color.New(color.FgGreen).Add(color.Bold)
	aiStyle := color.New(color.FgBlue)
	cmdStyle := color.New(color.FgYellow).Add(color.Bold)

	// Create copilot instance
	cop := &Copilot{
		ctx:       ctx,
		cancel:    cancel,
		machine:   machineCtx,
		llm:       llm.NewClient(cfg),
		tools:     tr,
		workspace: ws,
		userStyle: userStyle,
		aiStyle:   aiStyle,
		cmdStyle:  cmdStyle,
	}

	return cop
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

	// Check if this is an auto-fix request
	if isAutoFixRequest(prompt) {
		go func() {
			defer close(respChan)
			if err := c.AutoFixCode(c.ctx, respChan); err != nil {
				respChan <- fmt.Sprintf("\nError during auto-fix: %v\n", err)
			}
		}()
		return respChan, nil
	}

	// Get workspace context and tool descriptions
	wsContext := c.workspace.GetSummary()
	toolDescs := c.tools.GetToolDescriptions()

	// Create system message
	systemMsg := fmt.Sprintf(`You are a powerful AI coding assistant. You will process requests in distinct phases:

1. Analysis Phase:
   - Understand the user's request
   - Determine required tools and context
   - Plan the implementation strategy

2. Context Gathering Phase:
   - Collect relevant code context
   - Analyze dependencies
   - Understand the current state

3. Modification Phase:
   - Propose specific code changes
   - Use appropriate tools to implement changes
   - Maintain code quality and consistency

4. Verification Phase:
   - Verify changes meet requirements
   - Run tests if applicable
   - Present changes for user confirmation

For each action, explain your reasoning and wait for user confirmation before proceeding.

Available tools:
%s

Current workspace: %s
`, formatTools(toolDescs), wsContext["root"])

	// Add system message to LLM
	c.llm.AddSystemMessage(systemMsg)

	// Process in background
	go func() {
		defer close(respChan)

		// Get initial decision
		decision, err := c.getInitialDecision(prompt)
		if err != nil {
			respChan <- fmt.Sprintf("Error analyzing prompt: %v", err)
			return
		}

		// Process each phase sequentially
		phases := []struct {
			phase   DecisionPhase
			handler func(*Decision, chan<- string) error
			message string
		}{
			{PhaseAnalysis, c.handleAnalysisPhase, "Starting analysis phase..."},
			{PhaseContext, c.handleContextPhase, "Gathering context..."},
			{PhaseModification, c.handleModificationPhase, "Making modifications..."},
			{PhaseVerification, c.handleVerificationPhase, "Verifying changes..."},
		}

		currentPhase := decision.Phase
		phaseIndex := -1

		// Find the starting phase
		for i, p := range phases {
			if p.phase == currentPhase {
				phaseIndex = i
				break
			}
		}

		if phaseIndex == -1 {
			respChan <- fmt.Sprintf("Error: Invalid phase '%s'", currentPhase)
			return
		}

		// Execute phases sequentially
		for i := phaseIndex; i < len(phases); i++ {
			phase := phases[i]
			respChan <- fmt.Sprintf("\n=== %s ===\n", phase.message)

			if err := phase.handler(decision, respChan); err != nil {
				respChan <- fmt.Sprintf("\nError in %s phase: %v\n", phase.phase, err)
				return
			}

			// Create callback for handling tool calls
			callback := func(chunk string) {
				// Check if it's a tool call
				if toolCall := c.tools.ParseToolCall(chunk); toolCall != nil {
					result := toolCall.Execute(c.ctx)
					respChan <- fmt.Sprintf("\nTool result: %s\n", result)
				} else {
					// Stream regular response
					select {
					case <-c.ctx.Done():
						return
					case respChan <- chunk:
					}
				}
			}

			// Get LLM's response for the current phase
			response, err := c.llm.SendMessageWithCallback(
				fmt.Sprintf("Continue with %s phase. Current state: %s",
					phase.phase, decision.Action),
				callback,
			)
			if err != nil {
				respChan <- fmt.Sprintf("\nError getting LLM response: %v\n", err)
				return
			}

			// Update conversation
			c.llm.UpdateConversation(prompt, response)

			// Ask for confirmation before proceeding to next phase
			if i < len(phases)-1 {
				respChan <- fmt.Sprintf("\nProceed to %s phase? (yes/no): ", phases[i+1].phase)
				// Note: In a real implementation, we would need to handle user input here
				// For now, we'll automatically proceed
			}
		}
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
	if fsTool := c.tools.GetTool("filesystem"); fsTool != nil {
		return fsTool.Execute(c.ctx, map[string]interface{}{
			"operation": "read",
			"path":      filePath,
		})
	}
	return "", fmt.Errorf("filesystem tool not available")
}

// GetCodebaseContext retrieves information about the codebase structure
func (c *Copilot) GetCodebaseContext(depth int) (string, error) {
	if grepTool := c.tools.GetTool("grep_search"); grepTool != nil {
		return grepTool.Execute(c.ctx, map[string]interface{}{
			"pattern": ".",
			"depth":   depth,
		})
	}
	return "", fmt.Errorf("grep_search tool not available")
}

// GetGitContext retrieves git-related information
func (c *Copilot) GetGitContext(command string) (string, error) {
	if gitTool := c.tools.GetTool("git"); gitTool != nil {
		return gitTool.Execute(c.ctx, map[string]interface{}{
			"operation": command,
		})
	}
	return "", fmt.Errorf("git tool not available")
}

// SetProjectPath sets the project path for the workspace
func (c *Copilot) SetProjectPath(path string) error {
	if err := c.workspace.SetWorkspacePath(path); err != nil {
		return err
	}

	// Update tool workspace paths
	workspacePath := c.workspace.GetWorkspacePath()
	c.tools.RegisterTool(tools.NewGrepSearchTool(workspacePath))
	c.tools.RegisterTool(tools.NewRunTerminalTool(workspacePath))
	c.tools.RegisterTool(tools.NewGitTool(workspacePath))
	c.tools.RegisterTool(tools.NewFileSystemTool(workspacePath))
	c.tools.RegisterTool(tools.NewLanguageDetector(workspacePath))
	c.tools.RegisterTool(tools.NewLinterTool(workspacePath))

	// Detect languages in workspace
	if langTool := c.tools.GetTool("language_detector"); langTool != nil {
		if result, err := langTool.Execute(c.ctx, nil); err == nil {
			// Parse language detection results and update machine context
			lines := strings.Split(result, "\n")
			languages := make(map[string]float64)
			for _, line := range lines {
				if strings.HasPrefix(line, "- ") {
					parts := strings.Split(line[2:], ":")
					if len(parts) == 2 {
						langName := strings.TrimSpace(parts[0])
						percentStr := strings.TrimSpace(strings.Split(parts[1], "(")[1])
						percentStr = strings.TrimSuffix(percentStr, "%)")
						if percent, err := strconv.ParseFloat(percentStr, 64); err == nil {
							languages[langName] = percent
						}
					}
				}
			}
			c.machine.UpdateLanguages(languages)
		}
	}

	return nil
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
	gitTool := c.tools.GetTool("git")
	if gitTool == nil {
		return fmt.Errorf("git tool not available")
	}

	output, err := gitTool.Execute(c.ctx, map[string]interface{}{
		"operation": "status",
		"format":    "porcelain",
	})
	if err != nil {
		return fmt.Errorf("failed to get modified files: %v", err)
	}

	fsTool := c.tools.GetTool("filesystem")
	if fsTool == nil {
		return fmt.Errorf("filesystem tool not available")
	}

	// Create backup directory
	backupDir := filepath.Join(c.workspace.GetWorkspacePath(), ".tama", "backups",
		time.Now().Format("20060102_150405"))

	_, err = fsTool.Execute(c.ctx, map[string]interface{}{
		"operation": "mkdir",
		"path":      backupDir,
		"recursive": true,
	})
	if err != nil {
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

		srcPath := filepath.Join(c.workspace.GetWorkspacePath(), file)
		dstPath := filepath.Join(backupDir, file)

		// Create destination directory if needed
		dstDir := filepath.Dir(dstPath)
		_, err := fsTool.Execute(c.ctx, map[string]interface{}{
			"operation": "mkdir",
			"path":      dstDir,
			"recursive": true,
		})
		if err != nil {
			return fmt.Errorf("failed to create backup subdirectory for %s: %v", file, err)
		}

		// Copy file
		if err := c.copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to backup %s: %v", file, err)
		}
	}

	return nil
}

// copyFile copies a file from src to dst using FileSystemTool
func (c *Copilot) copyFile(src, dst string) error {
	if fsTool := c.tools.GetTool("filesystem"); fsTool != nil {
		// Read source file
		content, err := fsTool.Execute(c.ctx, map[string]interface{}{
			"operation": "read",
			"path":      src,
		})
		if err != nil {
			return fmt.Errorf("failed to read source file: %v", err)
		}

		// Write to destination file
		_, err = fsTool.Execute(c.ctx, map[string]interface{}{
			"operation": "write",
			"path":      dst,
			"content":   content,
		})
		if err != nil {
			return fmt.Errorf("failed to write destination file: %v", err)
		}
		return nil
	}
	return fmt.Errorf("filesystem tool not available")
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

// getInitialDecision analyzes the prompt and returns the initial decision
func (c *Copilot) getInitialDecision(prompt string) (*Decision, error) {
	// Create analysis prompt
	analysisPrompt := fmt.Sprintf(`You are an AI assistant analyzing a user request to determine the next action.
Please analyze the following request and determine the best approach:

Request: %s

You MUST respond in the following format EXACTLY, including all fields:

Phase: [analysis/context/modification/verification]
Action: [specific action to take]
Reasoning: [why this approach]
Context: [comma-separated list of files/directories needed]
Tools: [comma-separated list of tools needed]
Changes: [list of file changes in the format: filepath|description]

If this is a follow-up request, treat it as a new analysis phase.
Do not reference previous responses or assume any context from previous interactions.
Always provide ALL fields in your response, even if some are empty (use empty string or N/A).
`, prompt)

	// Get LLM response
	var response strings.Builder
	callback := func(chunk string) {
		response.WriteString(chunk)
	}

	// Send message with callback
	_, err := c.llm.SendMessageWithCallback(analysisPrompt, callback)
	if err != nil {
		return nil, fmt.Errorf("failed to get initial decision: %v", err)
	}

	// Parse response into Decision struct
	decision := &Decision{
		Phase:   PhaseAnalysis, // Default to analysis phase
		Context: make([]string, 0),
		Tools:   make([]string, 0),
		Changes: make([]Change, 0),
	}

	// Split response into lines
	lines := strings.Split(response.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Phase":
			if phase := DecisionPhase(value); isValidPhase(phase) {
				decision.Phase = phase
			}
		case "Action":
			if value != "" && value != "N/A" {
				decision.Action = value
			}
		case "Reasoning":
			if value != "" && value != "N/A" {
				decision.Reasoning = value
			}
		case "Context":
			if value != "" && value != "N/A" {
				decision.Context = splitAndTrim(value, ",")
			}
		case "Tools":
			if value != "" && value != "N/A" {
				decision.Tools = splitAndTrim(value, ",")
			}
		case "Changes":
			if value != "" && value != "N/A" {
				// Split multiple changes
				changesList := strings.Split(value, "\n")
				for _, change := range changesList {
					if change == "" || change == "N/A" {
						continue
					}
					// Split filepath and description
					changeParts := strings.Split(change, "|")
					if len(changeParts) == 2 {
						decision.Changes = append(decision.Changes, Change{
							FilePath:    strings.TrimSpace(changeParts[0]),
							Description: strings.TrimSpace(changeParts[1]),
							Timestamp:   time.Now(),
						})
					}
				}
			}
		}
	}

	// Validate decision
	if err := validateDecision(decision); err != nil {
		return nil, fmt.Errorf("invalid decision: %v", err)
	}

	return decision, nil
}

// isValidPhase checks if the given phase is valid
func isValidPhase(phase DecisionPhase) bool {
	switch phase {
	case PhaseAnalysis, PhaseContext, PhaseModification, PhaseVerification:
		return true
	default:
		return false
	}
}

// splitAndTrim splits a string by delimiter and trims each part
func splitAndTrim(s string, delimiter string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, delimiter)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" && trimmed != "N/A" {
			result = append(result, trimmed)
		}
	}
	return result
}

// validateDecision ensures the decision has all required fields
func validateDecision(d *Decision) error {
	if d.Phase == "" {
		return fmt.Errorf("phase is required")
	}
	if d.Action == "" {
		return fmt.Errorf("action is required")
	}
	if d.Reasoning == "" {
		return fmt.Errorf("reasoning is required")
	}
	return nil
}

// handleAnalysisPhase processes the analysis phase
func (c *Copilot) handleAnalysisPhase(decision *Decision, respChan chan<- string) error {
	respChan <- fmt.Sprintf("Analysis:\n%s\n\nProposed action:\n%s\n",
		decision.Reasoning, decision.Action)

	// Gather required context
	if fsTool := c.tools.GetTool("filesystem"); fsTool != nil {
		for _, contextPath := range decision.Context {
			content, err := fsTool.Execute(c.ctx, map[string]interface{}{
				"operation": "read",
				"path":      contextPath,
			})
			if err == nil {
				respChan <- fmt.Sprintf("\nRelevant context from %s:\n%s\n", contextPath, content)
			}
		}
	}
	return nil
}

// handleContextPhase processes the context gathering phase
func (c *Copilot) handleContextPhase(decision *Decision, respChan chan<- string) error {
	respChan <- "Gathering context...\n"

	// Use grep tool to search through the codebase
	if grepTool := c.tools.GetTool("grep_search"); grepTool != nil {
		for _, pattern := range decision.Tools {
			result, err := grepTool.Execute(c.ctx, map[string]interface{}{
				"pattern": pattern,
			})
			if err != nil {
				respChan <- fmt.Sprintf("\nError searching for pattern %s: %v\n", pattern, err)
				continue
			}
			if result != "" {
				respChan <- fmt.Sprintf("\nFound matches for pattern %s:\n%s\n", pattern, result)
			}
		}
	}
	return nil
}

// handleModificationPhase processes the modification phase
func (c *Copilot) handleModificationPhase(decision *Decision, respChan chan<- string) error {
	respChan <- "Implementing changes...\n"

	// Track all changes for potential rollback
	var appliedChanges []Change

	// Create a rollback function
	rollback := func() {
		respChan <- "\nRolling back changes...\n"
		if gitTool := c.tools.GetTool("git"); gitTool != nil {
			if _, err := gitTool.Execute(c.ctx, map[string]interface{}{
				"operation": "reset",
				"hard":      true,
			}); err != nil {
				respChan <- fmt.Sprintf("Warning: Failed to reset git changes: %v\n", err)
			}
		}
	}

	// Apply each proposed change
	fsTool := c.tools.GetTool("filesystem")
	if fsTool == nil {
		return fmt.Errorf("filesystem tool not available")
	}

	for _, change := range decision.Changes {
		respChan <- fmt.Sprintf("\nProcessing change for %s:\n%s\n", change.FilePath, change.Description)

		// Create backup
		_, err := fsTool.Execute(c.ctx, map[string]interface{}{
			"operation": "backup",
			"path":      change.FilePath,
		})
		if err != nil {
			respChan <- fmt.Sprintf("Warning: Failed to create backup: %v\n", err)
			rollback()
			return fmt.Errorf("backup creation failed: %v", err)
		}

		// Get current file content
		content, err := fsTool.Execute(c.ctx, map[string]interface{}{
			"operation": "read",
			"path":      change.FilePath,
		})
		if err != nil {
			respChan <- fmt.Sprintf("Error: Failed to read file: %v\n", err)
			rollback()
			return fmt.Errorf("file read failed: %v", err)
		}

		// Generate modified content
		modificationPrompt := fmt.Sprintf(`Given the current file content and the proposed change, generate the complete modified content.
Current content:
%s

Proposed change:
%s

Provide the complete modified content that can be written to the file. Ensure:
1. All necessary imports are included
2. The code follows best practices and conventions
3. The changes are properly documented
4. The code is properly formatted
`, content, change.Description)

		var modifiedContent strings.Builder
		callback := func(chunk string) {
			modifiedContent.WriteString(chunk)
		}

		if _, err := c.llm.SendMessageWithCallback(modificationPrompt, callback); err != nil {
			respChan <- fmt.Sprintf("Error: Failed to generate modified content: %v\n", err)
			rollback()
			return fmt.Errorf("content generation failed: %v", err)
		}

		// Write modified content
		_, err = fsTool.Execute(c.ctx, map[string]interface{}{
			"operation": "write",
			"path":      change.FilePath,
			"content":   modifiedContent.String(),
		})
		if err != nil {
			respChan <- fmt.Sprintf("Error: Failed to write file: %v\n", err)
			rollback()
			return fmt.Errorf("file write failed: %v", err)
		}
		respChan <- "Successfully wrote changes to file\n"

		// Run linter check
		if lintTool := c.tools.GetTool("linter"); lintTool != nil {
			checkResult, err := lintTool.Execute(c.ctx, map[string]interface{}{
				"operation": "check",
				"path":      change.FilePath,
			})
			if err != nil {
				respChan <- fmt.Sprintf("Warning: Linter check failed: %v\n", err)
			} else if checkResult != "No issues found" {
				respChan <- fmt.Sprintf("Linter found issues:\n%s\n", checkResult)
			} else {
				respChan <- "Code passed linter checks\n"
			}
		}

		// Add to git staging
		if gitTool := c.tools.GetTool("git"); gitTool != nil {
			if _, err := gitTool.Execute(c.ctx, map[string]interface{}{
				"operation": "add",
				"path":      change.FilePath,
			}); err != nil {
				respChan <- fmt.Sprintf("Warning: Failed to stage changes: %v\n", err)
			} else {
				respChan <- "Added changes to git staging area\n"
			}
		}

		// Track successful change
		appliedChanges = append(appliedChanges, change)
	}

	return nil
}

// handleVerificationPhase processes the verification phase
func (c *Copilot) handleVerificationPhase(decision *Decision, respChan chan<- string) error {
	respChan <- "Verifying changes...\n"

	// Show git diff
	if gitTool := c.tools.GetTool("git"); gitTool != nil {
		diff, err := gitTool.Execute(c.ctx, map[string]interface{}{
			"operation": "diff",
		})
		if err != nil {
			respChan <- fmt.Sprintf("\nError getting changes: %v\n", err)
		} else if diff != "" {
			respChan <- fmt.Sprintf("\nProposed changes:\n%s\n", diff)
		}
	}

	respChan <- "\nPlease review the changes and confirm (yes/no): "
	return nil
}

// HandleConfirmation processes the user's confirmation response
func (c *Copilot) HandleConfirmation(confirmation string, changes []Change) (*ChangeConfirmation, error) {
	conf := &ChangeConfirmation{
		Changes:   changes,
		Timestamp: time.Now(),
	}

	// Process user response
	switch strings.ToLower(strings.TrimSpace(confirmation)) {
	case "yes", "y":
		conf.Status = StatusAccepted
		// Commit changes if git is available
		if gitTool := c.tools.GetTool("git"); gitTool != nil {
			_, err := gitTool.Execute(c.ctx, map[string]interface{}{
				"operation": "commit",
				"message":   "Apply accepted changes",
			})
			if err != nil {
				return nil, fmt.Errorf("failed to commit changes: %v", err)
			}
		}
		// Remove backups
		for _, change := range changes {
			if change.Backup != "" {
				os.Remove(change.Backup)
			}
		}

	case "no", "n":
		conf.Status = StatusRejected
		// Restore from backups
		if fsTool := c.tools.GetTool("filesystem"); fsTool != nil {
			for _, change := range changes {
				if change.Backup != "" {
					_, err := fsTool.Execute(c.ctx, map[string]interface{}{
						"operation":   "restore",
						"path":        change.FilePath,
						"backup_path": change.Backup,
					})
					if err != nil {
						return nil, fmt.Errorf("failed to restore %s: %v", change.FilePath, err)
					}
					// Remove backup after restore
					os.Remove(change.Backup)
				}
			}
		}
		// Reset git changes if available
		if gitTool := c.tools.GetTool("git"); gitTool != nil {
			_, err := gitTool.Execute(c.ctx, map[string]interface{}{
				"operation": "reset",
				"hard":      true,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to reset changes: %v", err)
			}
		}

	default:
		return nil, fmt.Errorf("invalid confirmation response: %s", confirmation)
	}

	return conf, nil
}

// UpdateTaskState updates the current task state with confirmation results
func (c *Copilot) UpdateTaskState(conf *ChangeConfirmation) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.agent.CurrentTask != nil {
		switch conf.Status {
		case StatusAccepted:
			c.agent.CurrentTask.Status = "completed"
			c.agent.CurrentTask.Changes = conf.Changes
			c.agent.CurrentTask.EndTime = conf.Timestamp

		case StatusRejected:
			c.agent.CurrentTask.Status = "rejected"
			c.agent.CurrentTask.EndTime = conf.Timestamp
		}

		// Move current task to completed tasks
		c.agent.CompletedTasks = append(c.agent.CompletedTasks, *c.agent.CurrentTask)
		c.agent.CurrentTask = nil
	}
}

// AutoFixCode automatically analyzes and fixes code issues
func (c *Copilot) AutoFixCode(ctx context.Context, respChan chan<- string) error {
	respChan <- "Starting automatic code analysis and fix...\n"

	// Step 1: Detect languages in workspace
	respChan <- "\nStep 1: Detecting programming languages...\n"
	if langTool := c.tools.GetTool("language_detector"); langTool != nil {
		result, err := langTool.Execute(ctx, nil)
		if err != nil {
			respChan <- fmt.Sprintf("Warning: Failed to detect languages: %v\n", err)
		} else {
			respChan <- result
		}
	}

	// Step 2: Find all source files
	respChan <- "\nStep 2: Scanning for source files...\n"
	var sourceFiles []string
	if fsTool := c.tools.GetTool("filesystem"); fsTool != nil {
		result, err := fsTool.Execute(ctx, map[string]interface{}{
			"operation": "list",
			"recursive": true,
		})
		if err != nil {
			return fmt.Errorf("failed to scan files: %v", err)
		}

		// Filter source files
		for _, line := range strings.Split(result, "\n") {
			if line == "" {
				continue
			}
			if isSourceFile(line) {
				sourceFiles = append(sourceFiles, line)
			}
		}
		respChan <- fmt.Sprintf("Found %d source files\n", len(sourceFiles))
	}

	// Step 3: Check each file for issues
	respChan <- "\nStep 3: Analyzing files for issues...\n"
	type FileIssue struct {
		Path    string
		Content string
		Issues  string
	}
	var filesWithIssues []FileIssue

	fsTool := c.tools.GetTool("filesystem")
	if fsTool == nil {
		return fmt.Errorf("filesystem tool not available")
	}

	for _, file := range sourceFiles {
		respChan <- fmt.Sprintf("\nChecking %s...\n", file)

		// Read file content
		content, err := fsTool.Execute(ctx, map[string]interface{}{
			"operation": "read",
			"path":      file,
		})
		if err != nil {
			respChan <- fmt.Sprintf("Warning: Failed to read file: %v\n", err)
			continue
		}

		// Run linter check
		if lintTool := c.tools.GetTool("linter"); lintTool != nil {
			issues, err := lintTool.Execute(ctx, map[string]interface{}{
				"operation": "check",
				"path":      file,
			})
			if err != nil {
				respChan <- fmt.Sprintf("Warning: Failed to check file: %v\n", err)
				continue
			}

			if issues != "No issues found" {
				filesWithIssues = append(filesWithIssues, FileIssue{
					Path:    file,
					Content: content,
					Issues:  issues,
				})
				respChan <- fmt.Sprintf("Found issues:\n%s\n", issues)
			} else {
				respChan <- "No issues found\n"
			}
		}
	}

	// Step 4: Generate and apply fixes
	if len(filesWithIssues) > 0 {
		respChan <- fmt.Sprintf("\nStep 4: Fixing issues in %d files...\n", len(filesWithIssues))
		for _, file := range filesWithIssues {
			respChan <- fmt.Sprintf("\nFixing %s...\n", file.Path)

			// Create backup
			_, err := fsTool.Execute(ctx, map[string]interface{}{
				"operation": "backup",
				"path":      file.Path,
			})
			if err != nil {
				respChan <- fmt.Sprintf("Warning: Failed to create backup: %v\n", err)
				continue
			}
			respChan <- "Created backup successfully\n"

			// Generate fix using LLM
			fixPrompt := fmt.Sprintf(`Analyze the following code and its issues, then provide a fixed version:

File: %s

Current code:
%s

Issues found:
%s

Please provide the complete fixed code that resolves these issues:
`, file.Path, file.Content, file.Issues)

			var fixedContent strings.Builder
			callback := func(chunk string) {
				fixedContent.WriteString(chunk)
			}

			_, err = c.llm.SendMessageWithCallback(fixPrompt, callback)
			if err != nil {
				respChan <- fmt.Sprintf("Error generating fix: %v\n", err)
				continue
			}

			// Apply the fix
			_, err = fsTool.Execute(ctx, map[string]interface{}{
				"operation": "write",
				"path":      file.Path,
				"content":   fixedContent.String(),
			})
			if err != nil {
				respChan <- fmt.Sprintf("Error applying fix: %v\n", err)
				continue
			}

			// Run linter again to verify fix
			if lintTool := c.tools.GetTool("linter"); lintTool != nil {
				verifyResult, err := lintTool.Execute(ctx, map[string]interface{}{
					"operation": "check",
					"path":      file.Path,
				})
				if err != nil {
					respChan <- fmt.Sprintf("Warning: Failed to verify fix: %v\n", err)
				} else if verifyResult == "No issues found" {
					respChan <- "Fix successful - no issues remaining\n"
				} else {
					respChan <- fmt.Sprintf("Some issues remain:\n%s\n", verifyResult)
				}
			}

			// Format Go files
			if strings.HasSuffix(file.Path, ".go") {
				if runTool := c.tools.GetTool("run_terminal"); runTool != nil {
					_, err := runTool.Execute(ctx, map[string]interface{}{
						"command": fmt.Sprintf("go fmt %s", file.Path),
					})
					if err != nil {
						respChan <- fmt.Sprintf("Warning: Failed to format file: %v\n", err)
					} else {
						respChan <- "Formatted Go code\n"
					}
				}
			}
		}
	} else {
		respChan <- "\nNo issues found in any files!\n"
	}

	return nil
}

// isSourceFile checks if a file is a source code file
func isSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	sourceExts := map[string]bool{
		".go":    true,
		".py":    true,
		".js":    true,
		".ts":    true,
		".jsx":   true,
		".tsx":   true,
		".java":  true,
		".cpp":   true,
		".c":     true,
		".h":     true,
		".rb":    true,
		".php":   true,
		".rs":    true,
		".swift": true,
		".kt":    true,
		".scala": true,
		".cs":    true,
		".fs":    true,
	}
	return sourceExts[ext]
}

// isAutoFixRequest checks if the prompt is requesting automatic code fixing
func isAutoFixRequest(prompt string) bool {
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	fixKeywords := []string{
		"fix code",
		"fix issues",
		"fix bugs",
		"repair code",
		"auto fix",
		"autofix",
		"fix errors",
		"修复代码",
		"修复问题",
		"修复错误",
		"自动修复",
	}

	for _, keyword := range fixKeywords {
		if strings.Contains(prompt, keyword) {
			return true
		}
	}
	return false
}
