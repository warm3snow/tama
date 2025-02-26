package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/warm3snow/tama/internal/config"
)

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIRequest represents a request to the OpenAI API
type OpenAIRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

// OpenAIResponse represents a response from the OpenAI API
type OpenAIResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// OllamaRequest represents a request to the Ollama API
type OllamaRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

type ProviderModel struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

// OllamaResponse represents a response from the Ollama API
type OllamaResponse struct {
	Model         string `json:"model"`
	CreatedAt     string `json:"created_at"`
	Response      string `json:"response"`
	Done          bool   `json:"done"`
	Context       []int  `json:"context,omitempty"`
	TotalDuration int64  `json:"total_duration,omitempty"`
	Error         string `json:"error,omitempty"`
}

// GetModels returns the available models
func GetModels(provider config.Provider) ([]string, error) {
	apiURL := fmt.Sprintf("%s/v1/models", provider.BaseURL)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get models: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// {
	// 	"object": "list",
	// 	"data": [
	// 	  {
	// 		"id": "qwen2.5-coder:1.5b-base",
	// 		"object": "model",
	// 		"created": 1740368924,
	// 		"owned_by": "library"
	// 	  },
	// 	  {
	// 		"id": "nomic-embed-text:latest",
	// 		"object": "model",
	// 		"created": 1740366660,
	// 		"owned_by": "library"
	// 	  }
	// 	]
	// }

	var response struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int    `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal models: %v", err)
	}

	modelNames := make([]string, len(response.Data))
	for i, model := range response.Data {
		modelNames[i] = model.ID
	}

	return modelNames, nil
}
