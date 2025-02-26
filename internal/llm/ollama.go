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
func sendToOllama(provider config.Provider, defaults config.DefaultProvider, message string) (string, error) {
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

// GetModels returns the available models
func GetModels(provider config.Provider) ([]string, error) {
	apiURL := fmt.Sprintf("%s/api/tags", provider.BaseURL)
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
	// 	"models": [
	// 	  {
	// 		"name": "codellama:13b",
	// 		"modified_at": "2023-11-04T14:56:49.277302595-07:00",
	// 		"size": 7365960935,
	// 		"digest": "9f438cb9cd581fc025612d27f7c1a6669ff83a8bb0ed86c94fcf4c5440555697",
	// 		"details": {
	// 		  "format": "gguf",
	// 		  "family": "llama",
	// 		  "families": null,
	// 		  "parameter_size": "13B",
	// 		  "quantization_level": "Q4_0"
	// 		}
	// 	  },
	// 	  {
	// 		"name": "llama3:latest",
	// 		"modified_at": "2023-12-07T09:32:18.757212583-08:00",
	// 		"size": 3825819519,
	// 		"digest": "fe938a131f40e6f6d40083c9f0f430a515233eb2edaa6d72eb85c50d64f2300e",
	// 		"details": {
	// 		  "format": "gguf",
	// 		  "family": "llama",
	// 		  "families": null,
	// 		  "parameter_size": "7B",
	// 		  "quantization_level": "Q4_0"
	// 		}
	// 	  }
	// 	]
	//   }

	// ollama returns a json object with a models array
	var response struct {
		Models []struct {
			Name       string `json:"name"`
			ModifiedAt string `json:"modified_at"`
			Size       int    `json:"size"`
			Digest     string `json:"digest"`
			Details    struct {
				Format            string   `json:"format"`
				Family            string   `json:"family"`
				Families          []string `json:"families"`
				ParameterSize     string   `json:"parameter_size"`
				QuantizationLevel string   `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal models: %v", err)
	}

	modelNames := make([]string, len(response.Models))
	for i, model := range response.Models {
		modelNames[i] = model.Name
	}

	return modelNames, nil
}
