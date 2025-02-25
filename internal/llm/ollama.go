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

// sendToOllama sends a message to the Ollama API and returns the response
func sendToOllama(provider config.Provider, defaults struct {
	Provider    string  `json:"provider"`
	Model      string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens  int     `json:"max_tokens"`
}, message string) (string, error) {
	// Prepare request
	apiURL := fmt.Sprintf("%s/api/generate", provider.BaseURL)
	reqBody := OllamaRequest{
		Model:       defaults.Model,
		Prompt:      message,
		Temperature: defaults.Temperature,
		MaxTokens:   defaults.MaxTokens,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response as bytes
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Ollama returns a stream of JSON objects, so we'll parse it directly as a stream
	return parseOllamaResponse(body)
}

// parseOllamaResponse handles both single response and streaming response formats from Ollama
func parseOllamaResponse(responseBody []byte) (string, error) {
	// Check if the response is a single JSON object or a stream
	trimmedBody := strings.TrimSpace(string(responseBody))
	
	// If it doesn't start with '{', it's not valid JSON
	if len(trimmedBody) == 0 || trimmedBody[0] != '{' {
		return "", fmt.Errorf("invalid response format")
	}
	
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