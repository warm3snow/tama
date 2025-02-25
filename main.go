package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

type Provider struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

type Config struct {
	Providers map[string]Provider `json:"providers"`
	Defaults  struct {
		Provider    string  `json:"provider"`
		Model      string  `json:"model"`
		Temperature float64 `json:"temperature"`
		MaxTokens  int     `json:"max_tokens"`
	} `json:"defaults"`
}

// OpenAI API request structure
type OpenAIRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

// Ollama API request structure
type OllamaRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAI API response structure
type OpenAIResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Ollama API response structure
type OllamaResponse struct {
	Model         string   `json:"model"`
	CreatedAt     string   `json:"created_at"`
	Response      string   `json:"response"`
	Done          bool     `json:"done"`
	Context       []int    `json:"context,omitempty"`
	TotalDuration int64    `json:"total_duration,omitempty"`
	Error         string   `json:"error,omitempty"`
}

func getDefaultConfig() Config {
	config := Config{
		Providers: map[string]Provider{
			"openai": {
				APIKey:  "sk-xxx",
				BaseURL: "https://api.openai.com/v1",
			},
			"ollama": {
				BaseURL: "http://localhost:11434",
				APIKey:  "ollama",
			},
		},
	}
	config.Defaults.Provider = "ollama"
	config.Defaults.Model = "llama3.2:latest"
	config.Defaults.Temperature = 0.7
	config.Defaults.MaxTokens = 2048
	return config
}

func initConfig() (Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("failed to get home directory: %v", err)
	}

	configDir := filepath.Join(homeDir, ".config", "tama")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return Config{}, fmt.Errorf("failed to create config directory: %v", err)
	}

	configFile := filepath.Join(configDir, "config.yaml")
	
	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Config doesn't exist, create default
		config := getDefaultConfig()
		
		file, err := os.Create(configFile)
		if err != nil {
			return Config{}, fmt.Errorf("failed to create config file: %v", err)
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(config); err != nil {
			return Config{}, fmt.Errorf("failed to write config file: %v", err)
		}
		
		return config, nil
	} else {
		// Config exists, load it
		file, err := os.Open(configFile)
		if err != nil {
			return Config{}, fmt.Errorf("failed to open config file: %v", err)
		}
		defer file.Close()

		var config Config
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&config); err != nil {
			return Config{}, fmt.Errorf("failed to parse config file: %v", err)
		}
		
		return config, nil
	}
}

const (
	logo = `
████████╗ █████╗ ███╗   ███╗ █████╗     ██████╗ ██████╗ ██████╗ ███████╗
╚══██╔══╝██╔══██╗████╗ ████║██╔══██╗   ██╔════╝██╔═══██╗██╔══██╗██╔════╝
   ██║   ███████║██╔████╔██║███████║   ██║     ██║   ██║██║  ██║█████╗  
   ██║   ██╔══██║██║╚██╔╝██║██╔══██║   ██║     ██║   ██║██║  ██║██╔══╝  
   ██║   ██║  ██║██║ ╚═╝ ██║██║  ██║   ╚██████╗╚██████╔╝██████╔╝███████╗
   ╚═╝   ╚═╝  ╚═╝╚═╝     ╚═╝╚═╝  ╚═╝    ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝`
)

func showInitialScreen() {
	fmt.Println("\n* Welcome to Tama Code research preview!")
	fmt.Println("\nLet's get started.")
	fmt.Println("\nChoose the text style that looks best with your terminal:")
	fmt.Println("To change this later, run /config")
	
	lightText := color.New(color.FgHiWhite)
	lightText.Println("> Light text")
	fmt.Println("  Dark text")
	fmt.Println("  Light text (colorblind-friendly)")
	fmt.Println("  Dark text (colorblind-friendly)")
	
	fmt.Println("\nPreview")
	
	fmt.Println("1 function greet() {")
	color.Green(`2   console.log("Hello, World!");`)
	color.Green(`3   console.log("Hello, Tama!");`)
	fmt.Println("4 }")
}

func showSecondScreen() {
	coral := color.New(color.FgRed).Add(color.FgYellow)
	
	fmt.Println("\n* Welcome to Tama Code!")
	coral.Println(logo)
	fmt.Println("\nPress Enter to continue...")
}

func showPrompt() {
	fmt.Print("Paste code here if prompted > ")
}

// chatWithLLM starts an interactive chat session with the configured LLM
func chatWithLLM(config Config) {
	reader := bufio.NewReader(os.Stdin)
	conversation := []ChatMessage{}
	modelInfo := color.New(color.FgCyan)
	
	modelInfo.Printf("\nConnected to %s model: %s\n", config.Defaults.Provider, config.Defaults.Model)
	fmt.Println("Type 'exit' or 'quit' to end the conversation.")
	
	userStyle := color.New(color.FgGreen).Add(color.Bold)
	aiStyle := color.New(color.FgBlue)
	
	for {
		showPrompt()
		
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		
		if input == "" {
			continue
		}
		
		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}
		
		userStyle.Printf("\nYou: %s\n", input)
		
		response, err := sendToLLM(config, input, conversation)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}
		
		aiStyle.Printf("\nAI: %s\n\n", response)
		
		// Update conversation history for models that support it
		if config.Defaults.Provider == "openai" {
			conversation = append(conversation, 
				ChatMessage{Role: "user", Content: input},
				ChatMessage{Role: "assistant", Content: response})
		}
	}
}

// sendToLLM sends a message to the configured LLM provider and returns the response
func sendToLLM(config Config, message string, conversation []ChatMessage) (string, error) {
	provider := config.Defaults.Provider
	providerConfig, ok := config.Providers[provider]
	if !ok {
		return "", fmt.Errorf("provider %s not configured", provider)
	}

	switch provider {
	case "openai":
		return sendToOpenAI(providerConfig, config.Defaults, message, conversation)
	case "ollama":
		return sendToOllama(providerConfig, config.Defaults, message)
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}
}

func sendToOpenAI(provider Provider, defaults struct {
	Provider    string  `json:"provider"`
	Model      string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens  int     `json:"max_tokens"`
}, message string, conversation []ChatMessage) (string, error) {
	// Add user message to conversation
	conversation = append(conversation, ChatMessage{Role: "user", Content: message})

	// Prepare request
	apiURL := fmt.Sprintf("%s/chat/completions", provider.BaseURL)
	reqBody := OpenAIRequest{
		Model:       defaults.Model,
		Messages:    conversation,
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

	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if openAIResp.Error != nil {
		return "", fmt.Errorf("API error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

func sendToOllama(provider Provider, defaults struct {
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

func main() {
	// Initialize configuration
	config, err := initConfig()
	if err != nil {
		fmt.Printf("Error initializing config: %v\n", err)
		os.Exit(1)
	}
	
	slog.Info("Using model", "model", config.Defaults.Model)

	showInitialScreen()
	
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
	
	// Clear screen
	fmt.Print("\033[H\033[2J")
	
	showSecondScreen()
	
	reader.ReadString('\n')
	
	// No longer clearing the screen here
	// Instead, start conversation with LLM
	chatWithLLM(config)
}