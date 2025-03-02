package agent

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/warm3snow/tama/internal/config"
	"github.com/warm3snow/tama/internal/llm"
	"github.com/warm3snow/tama/internal/tools"
	"github.com/warm3snow/tama/internal/workspace"
)

// Agent represents the copilot agent
type Agent struct {
	config    *config.Config
	llm       llm.Interface
	workspace *workspace.Manager
	tools     *tools.Registry
}

// New creates a new agent
func New(cfg *config.Config) *Agent {
	// Initialize LLM client based on config
	llmClient := llm.NewClient(cfg.LLM)

	// Initialize workspace manager
	wsManager := workspace.NewManager()

	// Initialize tools registry
	toolsRegistry := tools.NewRegistry(cfg.Tools.Enabled)

	return &Agent{
		config:    cfg,
		llm:       llmClient,
		workspace: wsManager,
		tools:     toolsRegistry,
	}
}

// Start starts the agent in interactive mode
func (a *Agent) Start() error {
	fmt.Println("Starting Tama copilot agent...")
	fmt.Printf("Using LLM provider: %s, model: %s\n", a.config.LLM.Provider, a.config.LLM.Model)
	fmt.Println("Type 'exit' to quit.")

	// Main agent loop
	for {
		// Get user input (prompt)
		var input string
		fmt.Print("> ")
		fmt.Scanln(&input)

		if input == "exit" {
			break
		}

		// Execute the task
		if err := a.ExecuteTask(input); err != nil {
			fmt.Printf("Error: %s\n", err)
		}
	}

	return nil
}

// ExecuteTask executes a specific task
func (a *Agent) ExecuteTask(task string) error {
	fmt.Printf("Executing task: %s\n", task)

	// Step 1: Analyze the workspace to gather context
	workspaceContext, err := a.workspace.AnalyzeWorkspace()
	if err != nil {
		return fmt.Errorf("workspace analysis failed: %w", err)
	}

	// Step 2: Create initial prompt with task and context
	prompt := a.createInitialPrompt(task, workspaceContext)

	// Main execution loop as shown in the diagram
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		// Step 3: Send prompt to LLM
		fmt.Println("Thinking...")
		action, err := a.llm.GetNextAction(prompt)
		if err != nil {
			return fmt.Errorf("failed to get next action from LLM: %w", err)
		}

		// Step 4: Check if task is complete
		if action.IsComplete {
			fmt.Println("Task completed successfully!")
			if action.Reasoning != "" {
				fmt.Printf("Reasoning: %s\n", action.Reasoning)
			}
			return nil
		}

		// Step 5: Execute the tool
		fmt.Printf("Executing tool: %s\n", action.Tool)
		if action.Reasoning != "" {
			fmt.Printf("Reasoning: %s\n", action.Reasoning)
		}

		result, err := a.executeTool(action.Tool, action.Args)

		// Step 6: Add result or error to prompt for next iteration
		if err != nil {
			errorMessage := fmt.Sprintf("Error: %s", err)
			fmt.Println(errorMessage)
			prompt = a.appendToPrompt(prompt, errorMessage)
		} else {
			// Summarize result if it's too long
			resultSummary := result
			if len(result) > 500 {
				resultSummary = result[:500] + "... (truncated)"
			}
			fmt.Printf("Result: %s\n", resultSummary)
			prompt = a.appendToPrompt(prompt, fmt.Sprintf("Result: %s", result))
		}

		// Small delay to avoid overwhelming the system
		time.Sleep(100 * time.Millisecond)
	}

	return errors.New("maximum iterations reached without completing the task")
}

// createInitialPrompt creates the initial prompt for the LLM
func (a *Agent) createInitialPrompt(task string, workspaceContext string) string {
	// Get OS context
	osContext := fmt.Sprintf("OS: %s", a.getOSContext())

	// Get available tools with descriptions
	toolsDescription := a.tools.ListTools()

	// Format the prompt according to the diagram
	return fmt.Sprintf(`Task: %s

OS Context:
%s

Workspace Context:
%s

Available Tools:
%s

You are a copilot agent that helps users complete coding tasks. Analyze the context and determine the next action to take.
Respond with a JSON object containing:
{
  "tool": "tool_name",  // The tool to execute (leave empty if task is complete)
  "args": {             // Arguments for the tool
    "key1": "value1",
    "key2": "value2"
  },
  "is_complete": false, // Set to true if the task is complete
  "reasoning": "Explanation for why this action was chosen"
}`,
		task, osContext, workspaceContext, toolsDescription)
}

// getOSContext gets information about the operating system
func (a *Agent) getOSContext() string {
	// Get OS info from the config package
	osContext := config.GetOSContext()
	return fmt.Sprintf("%s %s (%s)", osContext.Name, osContext.Version, osContext.Arch)
}

// appendToPrompt appends information to the existing prompt
func (a *Agent) appendToPrompt(prompt, info string) string {
	return fmt.Sprintf("%s\n\n%s", prompt, info)
}

// executeTool executes a tool with the given arguments
func (a *Agent) executeTool(toolName string, args map[string]interface{}) (string, error) {
	tool, err := a.tools.GetTool(toolName)
	if err != nil {
		return "", err
	}

	log.Printf("Executing tool: %s with args: %v", toolName, args)

	result, err := tool.Execute(args)
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	return result, nil
}
