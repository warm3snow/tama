package copilot

import (
	"context"
	"fmt"
	"io"
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

// AgentState represents the current state of the agent
type AgentState struct {
	Goal        string
	CurrentTask string
	Changes     []string
	StartTime   time.Time
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

	// Get available tools
	tools := c.tools.GetToolDescriptions()

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

Available tools:
%s

Current workspace: %s
`, formatTools(tools), wsContext["root"])

	// Add system message to LLM
	c.llm.AddSystemMessage(systemMsg)

	// Process in background
	go func() {
		defer close(respChan)

		// Create callback for streaming responses
		callback := func(chunk string) {
			// Check if it's a tool call
			if tool := c.tools.ParseToolCall(chunk); tool != nil {
				// Execute tool but don't print the call details
				result := tool.Execute(c.ctx)
				// Only print errors if any
				if strings.Contains(result, "error") || strings.Contains(result, "Error") {
					select {
					case <-c.ctx.Done():
						return
					case respChan <- fmt.Sprintf("\nError: %s\n", result):
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
		Goal:      goal,
		StartTime: time.Now(),
		Changes:   make([]string, 0),
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

		// Process response
		var response strings.Builder
		for chunk := range respChan {
			response.WriteString(chunk)
			// Also print the chunk to show real-time progress
			fmt.Print(chunk)
		}

		// Ask user if they want to continue
		c.cmdStyle.Print("\nContinue with the next step? (yes/no/rollback): ")
		rl, err := readline.New("")
		if err != nil {
			return err
		}
		input, err := rl.Readline()
		if err != nil {
			return err
		}

		switch strings.ToLower(strings.TrimSpace(input)) {
		case "yes", "y":
			continue
		case "no", "n":
			return nil
		case "rollback":
			return c.rollbackChanges()
		default:
			c.cmdStyle.Println("Invalid input. Continuing...")
		}
	}
}

// rollbackChanges rolls back the changes made by the agent
func (c *Copilot) rollbackChanges() error {
	c.cmdStyle.Println("Rolling back changes...")
	// TODO: Implement rollback functionality
	// This would involve:
	// 1. Keeping track of file changes
	// 2. Creating backup copies
	// 3. Restoring from backups
	return nil
}
