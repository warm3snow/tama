package llm

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/warm3snow/tama/internal/config"
	"github.com/warm3snow/tama/internal/logging"
)

// Client represents an LLM client that can communicate with different providers
type Client struct {
	cfg          config.Config
	httpClient   *http.Client
	conversation []Message
}

// NewClient creates a new LLM client
func NewClient(cfg config.Config) *Client {
	return &Client{
		cfg:          cfg,
		httpClient:   &http.Client{},
		conversation: make([]Message, 0),
	}
}

// Stream sends a streaming chat completion request to the specified provider
func (c *Client) Stream(provider config.Provider, request ChatCompletionRequest, callback func(string)) (string, error) {
	switch provider.Type {
	case config.OpenAI:
		return c.sendStreamingChatCompletionToOpenAI(provider, request, callback)
	case config.Ollama:
		return c.sendStreamingChatCompletionToOllama(provider, request, callback)
	default:
		return "", fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
}

// Complete sends a chat completion request to the specified provider
func (c *Client) Complete(provider config.Provider, request ChatCompletionRequest) (string, error) {
	switch provider.Type {
	case config.OpenAI:
		return c.sendChatCompletionToOpenAI(provider, request)
	case config.Ollama:
		return c.sendChatCompletionToOllama(provider, request)
	default:
		return "", fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
}

// SendMessage sends a message to the LLM and returns the response
func (c *Client) SendMessage(message string) (string, error) {
	return c.SendMessageWithCallback(message, nil)
}

// SendMessageWithCallback sends a message to the LLM and streams the response through a callback
func (c *Client) SendMessageWithCallback(message string, callback func(string)) (string, error) {
	provider := c.cfg.Defaults.Provider
	providerConfig, ok := c.cfg.Providers[provider]
	if !ok {
		return "", fmt.Errorf("provider %s not configured", provider)
	}

	// Log the LLM request
	logging.LogLLMRequest(provider, c.cfg.Defaults.Model, len(message))

	// Create the chat completion request with conversation history
	messages := append(c.conversation, Message{Role: "user", Content: message})
	request := ChatCompletionRequest{
		Model:       c.cfg.Defaults.Model,
		Messages:    messages,
		Temperature: c.cfg.Defaults.Temperature,
		MaxTokens:   c.cfg.Defaults.MaxTokens,
		Stream:      callback != nil, // Enable streaming if callback is provided
	}

	var response string
	var err error

	if callback != nil {
		// Use streaming for the response
		response, err = c.Stream(providerConfig, request, func(chunk string) {
			// Try to parse as tool call
			var toolCall ToolCall
			if err := json.Unmarshal([]byte(chunk), &toolCall); err == nil && toolCall.Tool != "" {
				// This is a tool call
				callback(chunk)
				return
			}

			// Regular response chunk
			callback(chunk)
		})
	} else {
		// Use regular request
		response, err = c.Complete(providerConfig, request)
	}

	// Log the LLM response
	logging.LogLLMResponse(provider, c.cfg.Defaults.Model, len(response), err)

	if err != nil {
		return "", err
	}

	return response, nil
}

// UpdateConversation updates the conversation history
func (c *Client) UpdateConversation(userMessage, aiResponse string) {
	c.conversation = append(c.conversation,
		Message{Role: "user", Content: userMessage},
		Message{Role: "assistant", Content: aiResponse})

	// Limit conversation history to prevent token overflow
	if len(c.conversation) > 10 {
		c.conversation = c.conversation[len(c.conversation)-10:]
	}
}

// AddSystemMessage adds a system message to the conversation history
func (c *Client) AddSystemMessage(message string) {
	c.conversation = append(c.conversation, Message{Role: "system", Content: message})

	// Limit conversation history to prevent token overflow
	if len(c.conversation) > 10 {
		c.conversation = c.conversation[len(c.conversation)-10:]
	}
}

// GetConversation returns the current conversation history
func (c *Client) GetConversation() []Message {
	return c.conversation
}

// ResetConversation clears all conversation history
func (c *Client) ResetConversation() {
	c.conversation = make([]Message, 0)
	logging.Logger.Info("Conversation history has been reset")
}

// ClearSystemMessages removes all system messages from the conversation history
func (c *Client) ClearSystemMessages() {
	// Create a new slice to hold non-system messages
	newMessages := make([]Message, 0)

	// Keep only non-system messages
	for _, msg := range c.conversation {
		if msg.Role != "system" {
			newMessages = append(newMessages, msg)
		}
	}

	// Update the conversation with filtered messages
	c.conversation = newMessages
}

// Close closes the client and releases resources
func (c *Client) Close() {
	// Nothing to close for now
}

// GetProvider returns the current provider name
func (c *Client) GetProvider() string {
	return c.cfg.Defaults.Provider
}

// GetModel returns the current model name
func (c *Client) GetModel() string {
	return c.cfg.Defaults.Model
}

// GetModels returns the available models
func (c *Client) GetModels() ([]string, error) {
	provider := c.cfg.Defaults.Provider
	providerConfig, ok := c.cfg.Providers[provider]
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
		var modelList ModelList
		if err := json.NewDecoder(resp.Body).Decode(&modelList); err != nil {
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
	switch providerConfig.Type {
	case config.OpenAI:
		return []string{"gpt-3.5-turbo", "gpt-4"}, nil
	case config.Ollama:
		return []string{"llama2", "codellama", "mistral"}, nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerConfig.Type)
	}
}

// SwitchModel switches the model for the given provider
func (c *Client) SwitchModel(model string) error {
	logging.LogLLMRequest(c.cfg.Defaults.Provider, model, 0) // Log model switch

	c.cfg.Defaults.Model = model
	if err := c.cfg.SwitchModel(model); err != nil {
		return err
	}
	return nil
}
