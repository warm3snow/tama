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
	"github.com/warm3snow/tama/internal/logging"
)

// Client represents an LLM client
type Client struct {
	config       config.Config
	conversation []ChatMessage
	httpClient   *http.Client
}

// NewClient creates a new LLM client with the given configuration
func NewClient(cfg config.Config) *Client {
	return &Client{
		config:       cfg,
		conversation: make([]ChatMessage, 0),
		httpClient:   &http.Client{},
	}
}

// SendMessage sends a message to the LLM and returns the response
func (c *Client) SendMessage(message string) (string, error) {
	return c.SendMessageWithCallback(message, nil)
}

// SendMessageWithCallback sends a message to the LLM and streams the response through a callback
func (c *Client) SendMessageWithCallback(message string, callback func(string)) (string, error) {
	provider := c.config.Defaults.Provider
	providerConfig, ok := c.config.Providers[provider]
	if !ok {
		return "", fmt.Errorf("provider %s not configured", provider)
	}

	// Log the LLM request
	logging.LogLLMRequest(provider, c.config.Defaults.Model, len(message))

	// Add user message to conversation
	userMessage := ChatMessage{Role: "user", Content: message}
	messages := append(c.conversation, userMessage)

	// Prepare the chat completion request
	request := ChatCompletionRequest{
		Model:       c.config.Defaults.Model,
		Messages:    messages,
		Temperature: c.config.Defaults.Temperature,
		MaxTokens:   c.config.Defaults.MaxTokens,
		Stream:      callback != nil, // Enable streaming if callback is provided
	}

	var response string
	var err error

	if callback != nil {
		// Use streaming for the response
		response, err = c.sendStreamingChatCompletionRequest(providerConfig, request, callback)
	} else {
		// Use regular request
		response, err = c.sendChatCompletionRequest(providerConfig, request)
	}

	// Log the LLM response
	logging.LogLLMResponse(provider, c.config.Defaults.Model, len(response), err)

	if err != nil {
		return "", err
	}

	return response, nil
}

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
		switch c.config.Defaults.Provider {
		case "openai":
			return c.sendStreamingChatCompletionToOpenAI(provider, request, callback)
		case "ollama":
			return c.sendStreamingChatCompletionToOllama(provider, request, callback)
		default:
			return "", fmt.Errorf("unsupported provider: %s", c.config.Defaults.Provider)
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

// sendStreamingChatCompletionToOpenAI sends a streaming request to the OpenAI API
func (c *Client) sendStreamingChatCompletionToOpenAI(provider config.Provider, request ChatCompletionRequest, callback func(string)) (string, error) {
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

// sendStreamingChatCompletionToOllama sends a streaming request to the Ollama API
func (c *Client) sendStreamingChatCompletionToOllama(provider config.Provider, request ChatCompletionRequest, callback func(string)) (string, error) {
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
			Stream:      true,
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
			Stream:      true,
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

	// For Ollama streaming responses
	decoder := json.NewDecoder(resp.Body)
	var fullResponse strings.Builder

	for {
		var resp OllamaResponse
		if err := decoder.Decode(&resp); err != nil {
			if err == io.EOF {
				break
			}
			return fullResponse.String(), fmt.Errorf("error parsing response: %v", err)
		}

		if resp.Error != "" {
			return fullResponse.String(), fmt.Errorf("API error: %s", resp.Error)
		}

		fullResponse.WriteString(resp.Response)
		if callback != nil {
			callback(resp.Response)
		}

		if resp.Done {
			break
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
		switch c.config.Defaults.Provider {
		case "openai":
			return sendChatCompletionToOpenAI(provider, request)
		case "ollama":
			return sendChatCompletionToOllama(provider, request)
		default:
			return "", fmt.Errorf("unsupported provider: %s", c.config.Defaults.Provider)
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

// UpdateConversation updates the conversation history
func (c *Client) UpdateConversation(userMessage, aiResponse string) {
	c.conversation = append(c.conversation,
		ChatMessage{Role: "user", Content: userMessage},
		ChatMessage{Role: "assistant", Content: aiResponse})

	// Limit conversation history to prevent token overflow
	if len(c.conversation) > 10 {
		c.conversation = c.conversation[len(c.conversation)-10:]
	}
}

// AddSystemMessage adds a system message to the conversation history
func (c *Client) AddSystemMessage(message string) {
	c.conversation = append(c.conversation, ChatMessage{Role: "system", Content: message})

	// Limit conversation history to prevent token overflow
	if len(c.conversation) > 10 {
		c.conversation = c.conversation[len(c.conversation)-10:]
	}
}

// GetProvider returns the current provider name
func (c *Client) GetProvider() string {
	return c.config.Defaults.Provider
}

// GetModel returns the current model name
func (c *Client) GetModel() string {
	return c.config.Defaults.Model
}

// GetModels returns the available models
func (c *Client) GetModels() ([]string, error) {
	provider := c.config.Defaults.Provider
	providerConfig, ok := c.config.Providers[provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}

	// Try to use OpenAI-compatible endpoint first
	apiURL := fmt.Sprintf("%s/v1/models", providerConfig.BaseURL)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Add authorization header if API key is provided
	if providerConfig.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+providerConfig.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %v", err)
		}

		var modelList ModelList
		if err := json.Unmarshal(body, &modelList); err != nil {
			return nil, fmt.Errorf("failed to unmarshal models: %v", err)
		}

		modelNames := make([]string, len(modelList.Data))
		for i, model := range modelList.Data {
			modelNames[i] = model.ID
		}

		return modelNames, nil
	}

	// Close response body if it exists but status is not OK
	if resp != nil {
		resp.Body.Close()
	}

	// Fall back to provider-specific implementation
	models, err := GetModels(providerConfig)
	if err != nil {
		return nil, err
	}

	return models, nil
}

// SwitchModel switches the model for the given provider
func (c *Client) SwitchModel(model string) error {
	logging.LogLLMRequest(c.config.Defaults.Provider, model, 0) // Log model switch

	c.config.Defaults.Model = model
	if err := c.config.SwitchModel(model); err != nil {
		return err
	}
	return nil
}
