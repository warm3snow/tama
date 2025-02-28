package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/warm3snow/tama/internal/config"
)

// sendStreamingChatCompletionRequest sends a streaming request to the provider's API endpoint
func (c *Client) sendStreamingChatCompletionRequest(provider config.Provider, request ChatCompletionRequest, callback func(string)) (string, error) {
	// Always try to use the OpenAI-compatible endpoint first
	apiURL := fmt.Sprintf("%s/v1/chat/completions", provider.BaseURL)

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

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	// Send request
	resp, err := c.httpClient.Do(req)

	// If we get a 404 or other error, the provider might not support OpenAI-compatible API
	// Fall back to provider-specific implementation if needed
	if err != nil || (resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode >= 400)) {
		if resp != nil {
			resp.Body.Close()
		}

		// Fall back to provider-specific implementation
		switch provider.Type {
		case config.OpenAI:
			return c.sendStreamingChatCompletionToOpenAI(provider, request, callback)
		case config.Ollama:
			return c.sendStreamingChatCompletionToOllama(provider, request, callback)
		default:
			return "", fmt.Errorf("unsupported provider type: %s", provider.Type)
		}
	}

	defer resp.Body.Close()

	// For streaming responses, we need to read line by line
	reader := bufio.NewReader(resp.Body)
	var fullResponse strings.Builder

	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return fullResponse.String(), fmt.Errorf("error reading stream: %v", err)
		}

		// Skip empty lines
		lineStr := strings.TrimSpace(string(line))
		if lineStr == "" {
			continue
		}

		// SSE format: lines starting with "data: "
		const prefix = "data: "
		if strings.HasPrefix(lineStr, prefix) {
			data := strings.TrimPrefix(lineStr, prefix)

			// The final message is just "data: [DONE]"
			if data == "[DONE]" {
				break
			}

			// Parse the chunk
			var chunk ChatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return fullResponse.String(), fmt.Errorf("error parsing chunk: %v", err)
			}

			// Handle errors in the chunk
			if chunk.Error != nil {
				return fullResponse.String(), fmt.Errorf("API error: %s", chunk.Error.Message)
			}

			// Check if there are choices in the chunk
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					fullResponse.WriteString(content)
					if callback != nil {
						callback(content)
					}
				}
			}
		}
	}

	return fullResponse.String(), nil
}

// sendChatCompletionRequest sends a request to the provider's API endpoint
func (c *Client) sendChatCompletionRequest(provider config.Provider, request ChatCompletionRequest) (string, error) {
	// Always try to use the OpenAI-compatible endpoint first
	apiURL := fmt.Sprintf("%s/v1/chat/completions", provider.BaseURL)

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

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	// Send request
	resp, err := c.httpClient.Do(req)

	// If we get a 404 or other error, the provider might not support OpenAI-compatible API
	// Fall back to provider-specific implementation if needed
	if err != nil || (resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode >= 400)) {
		if resp != nil {
			resp.Body.Close()
		}

		// Fall back to provider-specific implementation
		switch provider.Type {
		case config.OpenAI:
			return c.sendChatCompletionToOpenAI(provider, request)
		case config.Ollama:
			return c.sendChatCompletionToOllama(provider, request)
		default:
			return "", fmt.Errorf("unsupported provider type: %s", provider.Type)
		}
	}

	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response - this should work for all OpenAI-compatible APIs
	var chatResponse ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResponse); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	// Check for API errors
	if chatResponse.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResponse.Error.Message)
	}

	// Check if we have any choices
	if len(chatResponse.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	// Return the content of the first choice
	return chatResponse.Choices[0].Message.Content, nil
}
