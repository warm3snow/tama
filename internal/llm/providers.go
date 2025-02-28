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

// sendStreamingChatCompletionToOpenAI sends a streaming request to OpenAI's API
func (c *Client) sendStreamingChatCompletionToOpenAI(provider config.Provider, request ChatCompletionRequest, callback func(string)) (string, error) {
	return c.sendStreamingChatCompletionRequest(provider, request, callback)
}

// sendChatCompletionToOpenAI sends a request to OpenAI's API
func (c *Client) sendChatCompletionToOpenAI(provider config.Provider, request ChatCompletionRequest) (string, error) {
	return c.sendChatCompletionRequest(provider, request)
}

// sendStreamingChatCompletionToOllama sends a streaming request to Ollama's API using OpenAI-compatible endpoint
func (c *Client) sendStreamingChatCompletionToOllama(provider config.Provider, request ChatCompletionRequest, callback func(string)) (string, error) {
	apiURL := fmt.Sprintf("%s/v1/chat/completions", provider.BaseURL)
	request.Stream = true

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
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

		lineStr := strings.TrimSpace(string(line))
		if lineStr == "" {
			continue
		}

		const prefix = "data: "
		if strings.HasPrefix(lineStr, prefix) {
			data := strings.TrimPrefix(lineStr, prefix)
			if data == "[DONE]" {
				break
			}

			var chunk ChatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return fullResponse.String(), fmt.Errorf("error parsing chunk: %v", err)
			}

			if chunk.Error != nil {
				return fullResponse.String(), fmt.Errorf("API error: %s", chunk.Error.Message)
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				content := chunk.Choices[0].Delta.Content
				fullResponse.WriteString(content)
				if callback != nil {
					callback(content)
				}
			}
		}
	}

	return fullResponse.String(), nil
}

// sendChatCompletionToOllama sends a request to Ollama's API using OpenAI-compatible endpoint
func (c *Client) sendChatCompletionToOllama(provider config.Provider, request ChatCompletionRequest) (string, error) {
	apiURL := fmt.Sprintf("%s/v1/chat/completions", provider.BaseURL)
	request.Stream = false

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	var chatResponse ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResponse); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	if chatResponse.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResponse.Error.Message)
	}

	if len(chatResponse.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return chatResponse.Choices[0].Message.Content, nil
}
