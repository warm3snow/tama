package agent

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
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

	// Create a reader for user input
	reader := bufio.NewReader(os.Stdin)

	// Main agent loop
	for {
		// Get user input (prompt)
		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}

		// Trim whitespace and newlines
		input = strings.TrimSpace(input)

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
	initialPrompt := a.createInitialPrompt(task, workspaceContext)

	// Create a conversation history to track the interaction
	conversation := []llm.ChatMessage{
		{
			Role:    "system",
			Content: a.createSystemPrompt(),
		},
		{
			Role:    "user",
			Content: initialPrompt,
		},
	}

	// Main execution loop as shown in the diagram
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		// Step 3: Send conversation to LLM
		fmt.Println("Thinking...")
		action, err := a.llm.GetNextActionFromConversation(conversation)
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

		// Step 6: Add result or error to conversation for next iteration
		var resultMessage string
		if err != nil {
			errorMessage := fmt.Sprintf("Error executing tool %s: %s", action.Tool, err)
			fmt.Println(errorMessage)
			resultMessage = errorMessage
		} else {
			// Summarize result if it's too long for display
			resultSummary := result
			if len(result) > 500 {
				resultSummary = result[:500] + "... (truncated)"
			}
			fmt.Printf("Result: %s\n", resultSummary)
			resultMessage = fmt.Sprintf("Tool execution result for %s: %s", action.Tool, result)
		}

		// Add the assistant's action and the tool result to the conversation
		conversation = append(conversation, llm.ChatMessage{
			Role:    "assistant",
			Content: a.formatActionAsMessage(action),
		})

		conversation = append(conversation, llm.ChatMessage{
			Role:    "user",
			Content: resultMessage,
		})

		// Small delay to avoid overwhelming the system
		time.Sleep(100 * time.Millisecond)
	}

	return errors.New("maximum iterations reached without completing the task")
}

// createSystemPrompt creates the system prompt for the LLM
func (a *Agent) createSystemPrompt() string {
	return `You are a copilot agent that helps users complete coding tasks. 
You should analyze the context and determine the next action to take.

For each step, you should:
1. Analyze the current state and context
2. Decide on the next action to take
3. Respond with a JSON object containing the tool to execute, arguments for the tool, and whether the task is complete

Your response must be a valid JSON object with the following structure:
{
  "tool": "tool_name",  // The tool to execute (leave empty if task is complete)
  "args": {             // Arguments for the tool
    "key1": "value1",
    "key2": "value2"
  },
  "is_complete": false, // Set to true if the task is complete
  "reasoning": "Explanation for why this action was chosen"
}

After each tool execution, you will receive the result and should decide on the next action.
Think step by step and make sure each action brings you closer to completing the task.`
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

Please help me complete this task by determining the appropriate actions to take.`,
		task, osContext, workspaceContext, toolsDescription)
}

// formatActionAsMessage formats an action as a message for the conversation
func (a *Agent) formatActionAsMessage(action *llm.Action) string {
	// Convert the action to JSON
	actionJSON := fmt.Sprintf(`{
  "tool": "%s",
  "args": %v,
  "is_complete": %t,
  "reasoning": "%s"
}`, action.Tool, action.Args, action.IsComplete, action.Reasoning)

	// Replace the args placeholder with the actual args
	argsStr := "{"
	for k, v := range action.Args {
		if str, ok := v.(string); ok {
			argsStr += fmt.Sprintf(`"%s": "%s", `, k, str)
		} else {
			argsStr += fmt.Sprintf(`"%s": %v, `, k, v)
		}
	}
	if len(action.Args) > 0 {
		argsStr = argsStr[:len(argsStr)-2] // Remove trailing comma and space
	}
	argsStr += "}"

	actionJSON = strings.Replace(actionJSON, "map[string]interface {}{}", argsStr, 1)

	return actionJSON
}

// getOSContext gets information about the operating system
func (a *Agent) getOSContext() string {
	// Get OS info from the config package
	osContext := config.GetOSContext()
	return fmt.Sprintf("%s %s (%s)", osContext.Name, osContext.Version, osContext.Arch)
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
