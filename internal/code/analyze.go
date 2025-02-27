package code

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// analyzeIfCommand analyzes if the input is meant to be a shell command
func (h *Handler) analyzeIfCommand(input string) (bool, string, error) {
	// Get the client
	client := h.client

	// Prepare the prompt
	prompt := fmt.Sprintf(`
Determine if the following input should be interpreted as a shell command.
Be strict and only return true for input that clearly looks like a shell command.
Common shell commands include: ls, cd, git, npm, etc.

Input: %s

Respond only with a JSON object in the following format:
{
  "is_command": true/false,
  "command": "the command to run if is_command is true, otherwise empty",
  "reason": "brief reason for your decision"
}
`, input)

	// Send the request to the LLM
	response, err := client.SendMessage(prompt)
	if err != nil {
		return false, "", fmt.Errorf("failed to analyze command: %v", err)
	}

	// Extract JSON from response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return false, "", fmt.Errorf("failed to parse LLM response")
	}

	// Parse the response
	var result CommandAnalysisResponse
	err = json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse JSON: %v", err)
	}

	return result.IsCommand, result.Command, nil
}

// analyzeCodeRequest analyzes a code-related request and generates suggested actions
func (h *Handler) analyzeCodeRequest(input string) ([]CodeAction, bool) {
	client := h.client

	// Extract path and prompt
	var filePath string
	var promptText string

	// Use a regex to extract path and prompt
	matches := extractPathAndPrompt(input)
	if len(matches) >= 3 {
		filePath = matches[1]
		promptText = matches[2]
	} else {
		promptText = input
	}

	// Prepare the context
	context := ""
	fileContent := ""

	// If a file path is specified, try to read its content
	if filePath != "" {
		var err error
		fileContent, err = readFile(filePath)
		if err != nil {
			h.errorStyle.Printf("Error reading file %s: %v\n", filePath, err)
			return nil, false
		}

		context = fmt.Sprintf("File content of %s:\n```\n%s\n```\n", filePath, fileContent)
	}

	// Prepare the prompt
	prompt := fmt.Sprintf(`
You are a code assistant. Based on the request, suggest appropriate code actions.
%s
User request: %s

Return a JSON array of actions to take, each with the following format:
[
  {
    "type": "analyze",  // or "edit", "create", etc.
    "file_path": "path/to/file",  // required for edit/create
    "content": "new content",  // required for edit/create
    "start_line": 0,  // optional, for editing specific lines
    "end_line": 0,  // optional, for editing specific lines
    "description": "description of the action"
  }
]
`, context, promptText)

	// Send the request
	response, err := client.SendMessage(prompt)
	if err != nil {
		h.errorStyle.Printf("Error analyzing code request: %v\n", err)
		return nil, false
	}

	// Extract JSON
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		h.errorStyle.Printf("Failed to parse LLM response\n")
		return nil, false
	}

	// Parse the response
	var actions []CodeAction
	err = json.Unmarshal([]byte(jsonStr), &actions)
	if err != nil {
		h.errorStyle.Printf("Failed to parse JSON: %v\n", err)
		return nil, false
	}

	return actions, true
}

// extractJSON extracts JSON from an LLM response
func extractJSON(response string) string {
	// Look for content between triple backticks with json
	jsonPattern := "```json\n"
	jsonEnd := "```"

	jsonStart := strings.Index(response, jsonPattern)
	if jsonStart >= 0 {
		jsonStart += len(jsonPattern)
		jsonEndPos := strings.Index(response[jsonStart:], jsonEnd)
		if jsonEndPos >= 0 {
			return response[jsonStart : jsonStart+jsonEndPos]
		}
	}

	// Look for content between triple backticks without json
	plainPattern := "```\n"
	plainEnd := "```"

	plainStart := strings.Index(response, plainPattern)
	if plainStart >= 0 {
		plainStart += len(plainPattern)
		plainEndPos := strings.Index(response[plainStart:], plainEnd)
		if plainEndPos >= 0 {
			return response[plainStart : plainStart+plainEndPos]
		}
	}

	// Look for content between square brackets (array)
	if strings.Contains(response, "[") && strings.Contains(response, "]") {
		arrayStart := strings.Index(response, "[")
		arrayEnd := strings.LastIndex(response, "]")
		if arrayStart >= 0 && arrayEnd > arrayStart {
			return response[arrayStart : arrayEnd+1]
		}
	}

	// Look for content between curly braces (object)
	if strings.Contains(response, "{") && strings.Contains(response, "}") {
		objStart := strings.Index(response, "{")
		objEnd := strings.LastIndex(response, "}")
		if objStart >= 0 && objEnd > objStart {
			return response[objStart : objEnd+1]
		}
	}

	return response
}

// extractPathAndPrompt extracts file path and prompt from input string
func extractPathAndPrompt(input string) []string {
	// Simple implementation to extract path after @ symbol
	if strings.HasPrefix(input, "@") {
		parts := strings.SplitN(input[1:], " ", 2)
		if len(parts) == 2 {
			return []string{input, parts[0], parts[1]}
		} else if len(parts) == 1 {
			return []string{input, parts[0], ""}
		}
	}

	return []string{input, "", input}
}

// readFile reads the content of a file
func readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
