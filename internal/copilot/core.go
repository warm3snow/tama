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

	// Create tool registry and register tools
	tr := tools.NewRegistry()
	tr.RegisterTool(tools.NewEditFileTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewReadFileTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewGrepSearchTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewRunTerminalTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewGitTool(ws.GetWorkspacePath()))
	tr.RegisterTool(tools.NewFileSystemTool(ws.GetWorkspacePath()))

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

	// Create system message with enhanced decision making
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

		// First, get the initial decision
		decision, err := c.getInitialDecision(prompt)
		if err != nil {
			respChan <- fmt.Sprintf("Error analyzing prompt: %v", err)
			return
		}

		// Process based on the decision
		switch decision.Phase {
		case PhaseAnalysis:
			c.handleAnalysisPhase(decision, respChan)
		case PhaseContext:
			c.handleContextPhase(decision, respChan)
		case PhaseModification:
			c.handleModificationPhase(decision, respChan)
		case PhaseVerification:
			c.handleVerificationPhase(decision, respChan)
		}

		// Create callback for streaming responses
		callback := func(chunk string) {
			// Check if it's a tool call
			if toolCall := c.tools.ParseToolCall(chunk); toolCall != nil {
				// Execute tool and get result
				result := toolCall.Execute(c.ctx)

				// Handle specific tool responses
				toolName := ""
				if strings.Contains(chunk, `"tool":"edit_file"`) {
					toolName = "edit_file"
				} else if strings.Contains(chunk, `"tool":"run_terminal"`) {
					toolName = "run_terminal"
				}

				switch toolName {
				case "edit_file":
					respChan <- fmt.Sprintf("\nApplied changes to file: %s\n", result)
					// Show git diff after edit
					if gitTool := c.tools.GetTool("git"); gitTool != nil {
						if diff, err := gitTool.Execute(c.ctx, map[string]interface{}{
							"operation": "diff",
						}); err == nil && diff != "" {
							respChan <- fmt.Sprintf("\nChanges made:\n%s\n", diff)
						}
					}
				case "run_terminal":
					respChan <- fmt.Sprintf("\nCommand output:\n%s\n", result)
				default:
					respChan <- fmt.Sprintf("\nTool result: %s\n", result)
				}
				return
			}

			// Stream regular response to user
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

// getInitialDecision analyzes the prompt and returns the initial decision
func (c *Copilot) getInitialDecision(prompt string) (*Decision, error) {
	// Create analysis prompt
	analysisPrompt := fmt.Sprintf(`Analyze the following request and determine the best approach:
Request: %s

You must respond in the following format exactly:
Phase: [analysis/context/modification/verification]
Action: [specific action to take]
Reasoning: [why this approach]
Context: [comma-separated list of files/directories needed]
Tools: [comma-separated list of tools needed]
Changes: [list of file changes in the format: filepath|description]

Example response:
Phase: modification
Action: Add error handling to the main function
Reasoning: The code needs better error handling
Context: cmd/main.go, internal/errors/errors.go
Tools: filesystem,git
Changes: cmd/main.go|Add try-catch block around main logic
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
	decision := &Decision{}

	// Split response into lines
	lines := strings.Split(response.String(), "\n")
	for _, line := range lines {
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
			decision.Phase = DecisionPhase(value)
		case "Action":
			decision.Action = value
		case "Reasoning":
			decision.Reasoning = value
		case "Context":
			if value != "" {
				decision.Context = strings.Split(value, ",")
				// Trim spaces
				for i := range decision.Context {
					decision.Context[i] = strings.TrimSpace(decision.Context[i])
				}
			}
		case "Tools":
			if value != "" {
				decision.Tools = strings.Split(value, ",")
				// Trim spaces
				for i := range decision.Tools {
					decision.Tools[i] = strings.TrimSpace(decision.Tools[i])
				}
			}
		case "Changes":
			if value != "" {
				// Split multiple changes
				changesList := strings.Split(value, "\n")
				for _, change := range changesList {
					if change == "" {
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

	return decision, nil
}

// handleAnalysisPhase processes the analysis phase
func (c *Copilot) handleAnalysisPhase(decision *Decision, respChan chan<- string) {
	respChan <- fmt.Sprintf("Analysis:\n%s\n\nProposed action:\n%s\n",
		decision.Reasoning, decision.Action)

	// Gather required context
	for _, contextPath := range decision.Context {
		if content, err := c.workspace.ReadFile(contextPath); err == nil {
			respChan <- fmt.Sprintf("\nRelevant context from %s:\n%s\n", contextPath, content)
		}
	}
}

// handleContextPhase processes the context gathering phase
func (c *Copilot) handleContextPhase(decision *Decision, respChan chan<- string) {
	respChan <- "Gathering context...\n"

	// Use workspace tools to gather context
	for _, toolName := range decision.Tools {
		if t := c.tools.GetTool(toolName); t != nil {
			result, err := t.Execute(c.ctx, map[string]interface{}{
				"operation": "read",
			})
			if err != nil {
				respChan <- fmt.Sprintf("\nError from %s: %v\n", toolName, err)
				continue
			}
			if result != "" {
				respChan <- fmt.Sprintf("\nContext from %s:\n%s\n", toolName, result)
			}
		}
	}
}

// handleModificationPhase processes the modification phase
func (c *Copilot) handleModificationPhase(decision *Decision, respChan chan<- string) {
	respChan <- "Implementing changes...\n"

	// Apply each proposed change
	for _, change := range decision.Changes {
		respChan <- fmt.Sprintf("\nProcessing change for %s:\n%s\n", change.FilePath, change.Description)

		// Create backup using filesystem tool
		if fsTool := c.tools.GetTool("filesystem"); fsTool != nil {
			result, err := fsTool.Execute(c.ctx, map[string]interface{}{
				"operation": "backup",
				"path":      change.FilePath,
			})
			if err != nil {
				respChan <- fmt.Sprintf("\nWarning: Failed to create backup: %v\n", err)
			} else {
				change.Backup = result
				respChan <- fmt.Sprintf("Created backup at: %s\n", result)
			}
		}

		// Get current file content if it exists
		var currentContent string
		if fsTool := c.tools.GetTool("filesystem"); fsTool != nil {
			content, err := fsTool.Execute(c.ctx, map[string]interface{}{
				"operation": "read",
				"path":      change.FilePath,
			})
			if err == nil {
				currentContent = content
			}
		}

		// Generate the modified content using LLM
		modificationPrompt := fmt.Sprintf(`Given the current file content and the proposed change, generate the complete modified content.
Current content:
%s

Proposed change:
%s

Provide the complete modified content that can be written to the file:
`, currentContent, change.Description)

		var modifiedContent strings.Builder
		callback := func(chunk string) {
			modifiedContent.WriteString(chunk)
		}

		_, err := c.llm.SendMessageWithCallback(modificationPrompt, callback)
		if err != nil {
			respChan <- fmt.Sprintf("\nError generating modified content: %v\n", err)
			continue
		}

		// Write the modified content using filesystem tool
		if fsTool := c.tools.GetTool("filesystem"); fsTool != nil {
			result, err := fsTool.Execute(c.ctx, map[string]interface{}{
				"operation": "write",
				"path":      change.FilePath,
				"content":   modifiedContent.String(),
			})
			if err != nil {
				respChan <- fmt.Sprintf("\nError writing to file %s: %v\n", change.FilePath, err)
				continue
			}
			respChan <- fmt.Sprintf("Successfully wrote changes to file: %s\n", result)

			// Run go fmt if it's a Go file
			if strings.HasSuffix(change.FilePath, ".go") {
				if runTool := c.tools.GetTool("run_terminal"); runTool != nil {
					result, err := runTool.Execute(c.ctx, map[string]interface{}{
						"command": fmt.Sprintf("go fmt %s", change.FilePath),
					})
					if err != nil {
						respChan <- fmt.Sprintf("\nWarning: Failed to format file: %v\n", err)
					} else {
						respChan <- fmt.Sprintf("Formatted file: %s\n", result)
					}
				}
			}
		}

		// Add to git if available
		if gitTool := c.tools.GetTool("git"); gitTool != nil {
			result, err := gitTool.Execute(c.ctx, map[string]interface{}{
				"operation": "add",
				"path":      change.FilePath,
			})
			if err != nil {
				respChan <- fmt.Sprintf("\nWarning: Failed to add to git: %v\n", err)
			} else {
				respChan <- fmt.Sprintf("Added to git staging area: %s\n", result)
			}
		}
	}
}

// handleVerificationPhase processes the verification phase
func (c *Copilot) handleVerificationPhase(decision *Decision, respChan chan<- string) {
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
