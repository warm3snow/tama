package llm

import (
	"fmt"
	"github.com/warm3snow/tama/internal/config"
)

// Client represents an LLM client
type Client struct {
	config config.Config
	conversation []ChatMessage
}

// NewClient creates a new LLM client with the given configuration
func NewClient(cfg config.Config) *Client {
	return &Client{
		config: cfg,
		conversation: make([]ChatMessage, 0),
	}
}

// SendMessage sends a message to the LLM and returns the response
func (c *Client) SendMessage(message string) (string, error) {
	provider := c.config.Defaults.Provider
	providerConfig, ok := c.config.Providers[provider]
	if !ok {
		return "", fmt.Errorf("provider %s not configured", provider)
	}

	switch provider {
	case "openai":
		return sendToOpenAI(providerConfig, c.config.Defaults, message, c.conversation)
	case "ollama":
		return sendToOllama(providerConfig, c.config.Defaults, message)
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}
}

// UpdateConversation updates the conversation history if the provider supports it
func (c *Client) UpdateConversation(userMessage, aiResponse string) {
	if c.config.Defaults.Provider == "openai" {
		c.conversation = append(c.conversation,
			ChatMessage{Role: "user", Content: userMessage},
			ChatMessage{Role: "assistant", Content: aiResponse})
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