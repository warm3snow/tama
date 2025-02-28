package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/warm3snow/tama/internal/config"
)

// sendChatCompletionToOpenAI sends a ChatCompletionRequest to the OpenAI API
func sendChatCompletionToOpenAI(provider config.Provider, request ChatCompletionRequest) (string, error) {
	// Prepare request URL
	apiURL := fmt.Sprintf("%s/chat/completions", provider.BaseURL)

	// Convert request to JSON
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response
	var openAIResp ChatCompletionResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	// Check for API errors
	if openAIResp.Error != nil {
		return "", fmt.Errorf("API error: %s", openAIResp.Error.Message)
	}

	// Check if we have any choices
	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	// Return the content of the first choice
	return openAIResp.Choices[0].Message.Content, nil
}

// sendChatCompletionToOllama sends a ChatCompletionRequest to the Ollama API using OpenAI-compatible format when possible
func sendChatCompletionToOllama(provider config.Provider, request ChatCompletionRequest) (string, error) {
	// Try Ollama's OpenAI-compatible API first (if available in newer versions of Ollama)
	ollamaOpenAIURL := fmt.Sprintf("%s/v1/chat/completions", provider.BaseURL)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", ollamaOpenAIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)

	// If the OpenAI-compatible endpoint works, use it
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response: %v", err)
		}

		// Parse as OpenAI format
		var openAIResp ChatCompletionResponse
		if err := json.Unmarshal(body, &openAIResp); err == nil && len(openAIResp.Choices) > 0 {
			return openAIResp.Choices[0].Message.Content, nil
		}
	}

	// Close response if it was opened
	if resp != nil {
		resp.Body.Close()
	}

	// Fall back to Ollama's native API
	// Determine if we should use Ollama's chat API or generate API based on the messages
	useOllamaChat := len(request.Messages) > 1

	var apiURL string

	if useOllamaChat {
		// Use chat API
		apiURL = fmt.Sprintf("%s/api/chat", provider.BaseURL)

		// Convert to Ollama format
		ollamaReq := OllamaRequest{
			Model:       request.Model,
			Messages:    request.Messages,
			Temperature: request.Temperature,
			Stream:      false,
		}

		jsonBody, err = json.Marshal(ollamaReq)
	} else {
		// Use generate API
		apiURL = fmt.Sprintf("%s/api/generate", provider.BaseURL)

		// Get the user's message from the last message
		prompt := ""
		if len(request.Messages) > 0 {
			prompt = request.Messages[len(request.Messages)-1].Content
		}

		// Convert to Ollama generate format
		ollamaReq := OllamaRequest{
			Model:       request.Model,
			Prompt:      prompt,
			Temperature: request.Temperature,
			MaxTokens:   request.MaxTokens,
		}

		jsonBody, err = json.Marshal(ollamaReq)
	}

	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	req, err = http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Parse Ollama's response
	response, err := parseOllamaResponse(body, useOllamaChat)
	if err != nil {
		return "", err
	}

	return response, nil
}

// parseOllamaResponse handles Ollama's response formats
func parseOllamaResponse(responseBody []byte, isChatResponse bool) (string, error) {
	// Check if the response is empty
	trimmedBody := strings.TrimSpace(string(responseBody))
	if len(trimmedBody) == 0 {
		return "", fmt.Errorf("empty response from Ollama")
	}

	// If it doesn't start with '{', it's not valid JSON
	if trimmedBody[0] != '{' {
		return "", fmt.Errorf("invalid response format")
	}

	// Process based on response type
	if isChatResponse {
		// For chat API responses
		var resp struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Error string `json:"error,omitempty"`
		}

		if err := json.Unmarshal(responseBody, &resp); err != nil {
			return "", fmt.Errorf("failed to parse chat response: %v", err)
		}

		if resp.Error != "" {
			return "", fmt.Errorf("API error: %s", resp.Error)
		}

		return resp.Message.Content, nil
	} else {
		// For generate API responses
		// If it doesn't contain newlines, it might be a single JSON object
		if !strings.Contains(trimmedBody, "\n") {
			var resp OllamaResponse
			if err := json.Unmarshal(responseBody, &resp); err != nil {
				return "", fmt.Errorf("failed to parse response: %v", err)
			}
			if resp.Error != "" {
				return "", fmt.Errorf("API error: %s", resp.Error)
			}

			return resp.Response, nil
		}

		// It's a stream of JSON objects, separated by newlines
		var fullResponse strings.Builder
		decoder := json.NewDecoder(bytes.NewReader(responseBody))

		for decoder.More() {
			var resp OllamaResponse
			if err := decoder.Decode(&resp); err != nil {
				// Log the specific error for debugging
				slog.Error("JSON stream unmarshal error", "error", err)
				// Skip the invalid JSON and continue
				continue
			}

			if resp.Error != "" {
				return "", fmt.Errorf("API error: %s", resp.Error)
			}

			fullResponse.WriteString(resp.Response)
		}

		return fullResponse.String(), nil
	}
}
