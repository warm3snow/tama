package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/warm3snow/tama/internal/config"
)

// Action represents an action to be taken by the agent
type Action struct {
	Tool       string                 `json:"tool"`
	Args       map[string]interface{} `json:"args"`
	IsComplete bool                   `json:"is_complete"`
	Reasoning  string                 `json:"reasoning,omitempty"` // Explanation for the decision
}

// Interface defines the interface for LLM clients
type Interface interface {
	GetNextAction(prompt string) (*Action, error)
}

// Client implements the LLM interface
type Client struct {
	config config.LLMConfig
	client *http.Client
}

// OpenAIRequest represents a request to the OpenAI-compatible API
type OpenAIRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatMessage represents a message in the chat
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse represents a response from the OpenAI-compatible API
type OpenAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// NewClient creates a new LLM client
func NewClient(cfg config.LLMConfig) Interface {
	return &Client{
		config: cfg,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GetNextAction gets the next action from the LLM
func (c *Client) GetNextAction(prompt string) (*Action, error) {
	// In development mode, use the mock implementation
	if c.config.Provider == "mock" {
		return c.mockGetNextAction(prompt)
	}

	// Prepare the request to the OpenAI-compatible API
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: "You are a copilot agent that helps users complete coding tasks. You should analyze the context and determine the next action to take. Respond with a JSON object containing the tool to execute, arguments for the tool, and whether the task is complete.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// Create the request
	reqBody := OpenAIRequest{
		Model:       c.config.Model,
		Messages:    messages,
		Temperature: c.config.Temperature,
		MaxTokens:   c.config.MaxTokens,
	}

	// Convert the request to JSON
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Determine the API endpoint based on the provider
	endpoint := c.config.BaseURL
	if endpoint == "" {
		switch c.config.Provider {
		case "openai":
			endpoint = "https://api.openai.com/v1"
		case "ollama":
			endpoint = "http://localhost:11434/v1"
		default:
			endpoint = "http://localhost:11434/v1" // Default to Ollama
		}
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", endpoint+"/chat/completions", bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if we got any choices
	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	// Get the content
	content := openAIResp.Choices[0].Message.Content

	// Try to parse the content as JSON
	var action Action

	// Extract JSON from the content (it might be wrapped in markdown code blocks)
	jsonStr := extractJSON(content)

	if err := json.Unmarshal([]byte(jsonStr), &action); err != nil {
		// If parsing fails, try to infer the action from the content
		return inferActionFromContent(content)
	}

	return &action, nil
}

// extractJSON extracts JSON from a string that might contain markdown
func extractJSON(content string) string {
	// Check if the content is wrapped in markdown code blocks
	if strings.Contains(content, "```json") {
		parts := strings.Split(content, "```json")
		if len(parts) > 1 {
			jsonPart := parts[1]
			endIndex := strings.Index(jsonPart, "```")
			if endIndex > 0 {
				return strings.TrimSpace(jsonPart[:endIndex])
			}
			return strings.TrimSpace(jsonPart)
		}
	}

	// Check if the content is wrapped in regular code blocks
	if strings.Contains(content, "```") {
		parts := strings.Split(content, "```")
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	}

	// Return the original content
	return content
}

// inferActionFromContent tries to infer an action from the content
func inferActionFromContent(content string) (*Action, error) {
	content = strings.ToLower(content)

	// Check for completion
	if strings.Contains(content, "complete") || strings.Contains(content, "finished") || strings.Contains(content, "done") {
		return &Action{
			Tool:       "",
			Args:       nil,
			IsComplete: true,
			Reasoning:  "Task appears to be complete based on LLM response.",
		}, nil
	}

	// Check for file operations
	if strings.Contains(content, "read") && strings.Contains(content, "file") {
		return &Action{
			Tool: "file_read",
			Args: map[string]interface{}{
				"path": inferFilePath(content),
			},
			IsComplete: false,
			Reasoning:  "LLM response suggests reading a file.",
		}, nil
	}

	if (strings.Contains(content, "edit") || strings.Contains(content, "modify") ||
		strings.Contains(content, "create") || strings.Contains(content, "write")) &&
		strings.Contains(content, "file") {
		return &Action{
			Tool: "file_edit",
			Args: map[string]interface{}{
				"path":    inferFilePath(content),
				"content": "", // This will need to be filled in by the agent
			},
			IsComplete: false,
			Reasoning:  "LLM response suggests editing a file.",
		}, nil
	}

	// Check for terminal operations
	if strings.Contains(content, "run") || strings.Contains(content, "execute") || strings.Contains(content, "command") {
		return &Action{
			Tool: "terminal_run",
			Args: map[string]interface{}{
				"command": inferCommand(content),
			},
			IsComplete: false,
			Reasoning:  "LLM response suggests running a command.",
		}, nil
	}

	// Default to reading a file
	return &Action{
		Tool: "file_read",
		Args: map[string]interface{}{
			"path": "main.go",
		},
		IsComplete: false,
		Reasoning:  "Defaulting to reading main.go based on LLM response.",
	}, nil
}

// mockGetNextAction is a mock implementation for development
func (c *Client) mockGetNextAction(prompt string) (*Action, error) {
	// Parse the prompt to extract the task
	task := extractTask(prompt)

	// Check if the prompt contains previous errors
	hasErrors := strings.Contains(prompt, "Error:")

	// Check if the prompt contains a request to read a file
	if containsAny(strings.ToLower(task), []string{"read file", "open file", "show file", "view file", "cat file"}) {
		return &Action{
			Tool: "file_read",
			Args: map[string]interface{}{
				"path": inferFilePath(prompt),
			},
			IsComplete: false,
			Reasoning:  "The task requires reading a file to understand its contents.",
		}, nil
	}

	// Check if the prompt contains a request to edit a file
	if containsAny(strings.ToLower(task), []string{"edit file", "modify file", "change file", "update file", "create file"}) {
		return &Action{
			Tool: "file_edit",
			Args: map[string]interface{}{
				"path":    inferFilePath(prompt),
				"content": generateMockContent(prompt),
			},
			IsComplete: false,
			Reasoning:  "The task requires editing a file to implement the requested changes.",
		}, nil
	}

	// Check if the prompt contains a request to run a command
	if containsAny(strings.ToLower(task), []string{"run command", "execute command", "run", "execute", "terminal"}) {
		return &Action{
			Tool: "terminal_run",
			Args: map[string]interface{}{
				"command": inferCommand(prompt),
			},
			IsComplete: false,
			Reasoning:  "The task requires running a command in the terminal.",
		}, nil
	}

	// Check if the prompt contains a request to run tests
	if containsAny(strings.ToLower(task), []string{"run test", "execute test", "test"}) {
		return &Action{
			Tool: "test_run",
			Args: map[string]interface{}{
				"path": inferTestPath(prompt),
			},
			IsComplete: false,
			Reasoning:  "The task requires running tests to verify functionality.",
		}, nil
	}

	// If there were errors and this is not the first iteration, try a different approach
	if hasErrors && strings.Contains(prompt, "Result:") {
		// This is a follow-up action after an error
		return &Action{
			Tool: suggestAlternativeAction(prompt),
			Args: map[string]interface{}{
				"path": inferFilePath(prompt),
			},
			IsComplete: false,
			Reasoning:  "Previous action resulted in an error, trying an alternative approach.",
		}, nil
	}

	// Default to completing the task if we can't determine a specific action
	// or if we've already performed several actions
	if strings.Count(prompt, "Result:") > 3 {
		return &Action{
			Tool:       "",
			Args:       nil,
			IsComplete: true,
			Reasoning:  "The task appears to be complete based on the sequence of actions performed.",
		}, nil
	}

	// If we can't determine a specific action, default to reading a relevant file
	return &Action{
		Tool: "file_read",
		Args: map[string]interface{}{
			"path": "main.go", // Default to reading main.go
		},
		IsComplete: false,
		Reasoning:  "Starting by examining the main entry point of the application.",
	}, nil
}

// Helper functions for the mock implementation

// extractTask extracts the task from the prompt
func extractTask(prompt string) string {
	if strings.Contains(prompt, "Task:") {
		parts := strings.SplitN(prompt, "Task:", 2)
		if len(parts) > 1 {
			taskPart := parts[1]
			endIndex := strings.Index(taskPart, "\n\n")
			if endIndex > 0 {
				return strings.TrimSpace(taskPart[:endIndex])
			}
			return strings.TrimSpace(taskPart)
		}
	}
	return prompt
}

// inferFilePath tries to infer a file path from the prompt
func inferFilePath(prompt string) string {
	// Look for common file extensions
	for _, ext := range []string{".go", ".yaml", ".json", ".md", ".txt"} {
		index := strings.LastIndex(prompt, ext)
		if index > 0 {
			// Try to extract the filename
			start := strings.LastIndex(prompt[:index], " ")
			if start >= 0 {
				return strings.TrimSpace(prompt[start : index+len(ext)])
			}
		}
	}

	// Default to main.go if we can't find a specific file
	return "main.go"
}

// inferCommand tries to infer a command from the prompt
func inferCommand(prompt string) string {
	// Look for common command patterns
	if strings.Contains(prompt, "ls") || strings.Contains(prompt, "list") {
		return "ls -la"
	}
	if strings.Contains(prompt, "build") || strings.Contains(prompt, "compile") {
		return "go build"
	}
	if strings.Contains(prompt, "test") {
		return "go test ./..."
	}

	// Default to a simple command
	return "echo 'Hello, World!'"
}

// inferTestPath tries to infer a test path from the prompt
func inferTestPath(prompt string) string {
	// Look for package names
	for _, pkg := range []string{"agent", "config", "llm", "tools", "workspace"} {
		if strings.Contains(prompt, pkg) {
			return "./internal/" + pkg
		}
	}

	// Default to all tests
	return "./..."
}

// generateMockContent generates mock content for file edits
func generateMockContent(prompt string) string {
	// This is just a placeholder - in a real implementation, the LLM would generate actual content
	return "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}\n"
}

// suggestAlternativeAction suggests an alternative action based on the prompt
func suggestAlternativeAction(prompt string) string {
	// If the previous action was a file_read, suggest file_edit
	if strings.Contains(prompt, "file_read") {
		return "file_edit"
	}

	// If the previous action was a file_edit, suggest terminal_run
	if strings.Contains(prompt, "file_edit") {
		return "terminal_run"
	}

	// If the previous action was a terminal_run, suggest test_run
	if strings.Contains(prompt, "terminal_run") {
		return "test_run"
	}

	// Default to file_read
	return "file_read"
}

// containsAny checks if the string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
