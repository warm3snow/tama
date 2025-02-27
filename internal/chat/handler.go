package chat

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/warm3snow/tama/internal/completion"
	"github.com/warm3snow/tama/internal/llm"
	"github.com/warm3snow/tama/internal/logging"
)

// ChatHandler manages chat sessions
type ChatHandler struct {
	client        *llm.Client
	isInteractive bool
	userStyle     *color.Color
	aiStyle       *color.Color
}

// NewChatHandler creates a new chat handler
func NewChatHandler(client *llm.Client, isInteractive bool) *ChatHandler {
	userStyle := color.New(color.FgGreen).Add(color.Bold)
	aiStyle := color.New(color.FgBlue)

	return &ChatHandler{
		client:        client,
		isInteractive: isInteractive,
		userStyle:     userStyle,
		aiStyle:       aiStyle,
	}
}

// GetUserStyler returns a function to style user messages
func (h *ChatHandler) GetUserStyler() func(string) {
	return func(msg string) {
		h.userStyle.Printf("\nYou: %s\n", msg)
	}
}

// GetAIStyler returns a function to style AI messages
func (h *ChatHandler) GetAIStyler() func(string) {
	return func(msg string) {
		h.aiStyle.Printf("\nAI: %s\n\n", msg)
	}
}

// StartInteractiveChat starts an interactive chat session
func (h *ChatHandler) StartInteractiveChat() error {
	// Show welcome message
	h.showWelcomeMessage()

	// Chat mode specific commands
	chatSpecificCommands := []string{}

	// Initialize readline
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "> ",
		HistoryFile:     "/tmp/tama_chat_history.txt",
		AutoComplete:    completion.NewReadlineCompleter(chatSpecificCommands),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("error initializing readline: %v", err)
	}
	defer rl.Close()

	// Main chat loop
	for {
		// Get input using readline
		input, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				if len(input) == 0 {
					break
				} else {
					continue
				}
			} else if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading input: %v", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// Check if it's a ! command
		if strings.HasPrefix(input, "!") {
			cmdStr := strings.TrimPrefix(input, "!")
			cmdStr = strings.TrimSpace(cmdStr)

			if cmdStr == "" {
				fmt.Println("Error: No command specified after !")
				continue
			}

			// Use system default shell for output
			cmd := exec.Command("sh", "-c", cmdStr)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err := cmd.Run()
			if err != nil {
				fmt.Printf("Error executing command: %v\n", err)
			}

			continue
		}

		// Add /help command
		if input == "/help" {
			h.showHelpMessage()
			continue
		}

		// Process input and get response
		response, err := h.client.SendMessage(input)
		if err != nil {
			return fmt.Errorf("error sending message: %v", err)
		}

		// Display response (if not already displayed by streaming)
		if !h.isInteractive {
			h.aiStyle.Printf("\nAI: %s\n\n", response)
		}

		// Update conversation history
		h.client.UpdateConversation(input, response)

		// Add to readline history
		rl.SaveHistory(input)
	}

	return nil
}

// SendMessage sends a single message and returns the response
func (h *ChatHandler) SendMessage(message string) (string, error) {
	// Print AI prefix first
	h.aiStyle.Print("\nAI: ")

	// Log the conversation context before sending
	conversation := h.client.GetConversation()
	logging.Logger.Info("Sending message to LLM",
		"message", message,
		"contextCount", len(conversation))

	// Log each context message for debugging
	for i, msg := range conversation {
		logging.Logger.Debug("Context message",
			"index", i+1,
			"role", msg.Role,
			"content", summarizeContent(msg.Content))
	}

	// Define the callback for streaming responses
	callback := func(chunk string) {
		// Print each chunk with proper formatting
		h.aiStyle.Printf("%s", chunk)
	}

	// Get response from AI with streaming
	response, err := h.client.SendMessageWithCallback(message, callback)
	if err != nil {
		logging.LogError("Failed to get response from LLM", "error", err)
		return "", fmt.Errorf("failed to get response from LLM: %v", err)
	}

	// Add extra newlines for clean formatting
	fmt.Print("\n\n")

	// Log the response for debugging
	logging.Logger.Debug("Received response from LLM",
		"responseLength", len(response),
		"responseSummary", summarizeContent(response))

	// Update conversation history
	h.client.UpdateConversation(message, response)

	return response, nil
}

// summarizeContent returns a summary of the content (first 50 chars + "..." if longer)
func summarizeContent(content string) string {
	if len(content) <= 50 {
		return content
	}
	return content[:50] + "..."
}

// AddSystemMessage adds a system message to the conversation history
func (h *ChatHandler) AddSystemMessage(message string) {
	// Clear previous system messages to avoid context confusion
	h.client.ClearSystemMessages()

	// Add the new system message
	h.client.AddSystemMessage(message)

	logging.Logger.Info("Added system message to conversation",
		"content", summarizeContent(message))
}

// showWelcomeMessage displays a welcome message at the start of the chat
func (h *ChatHandler) showWelcomeMessage() {
	modelInfo := color.New(color.FgCyan)
	fmt.Println("Start talking with AI")
	modelInfo.Printf("Connected to %s model: %s\n",
		h.client.GetProvider(),
		h.client.GetModel())
	fmt.Println("Type 'exit' or 'quit' to end the conversation.")
	fmt.Println("Type '/help' to see available commands.")
}

// showHelpMessage displays available commands
func (h *ChatHandler) showHelpMessage() {
	cmdStyle := color.New(color.FgYellow).Add(color.Bold)
	descStyle := color.New(color.FgWhite)

	fmt.Println("\nAvailable commands:")

	// Display commands
	cmdStyle.Printf("  /help")
	descStyle.Printf(" - Display available commands\n")

	cmdStyle.Printf("  !")
	descStyle.Printf(" - Execute a shell command\n")
}
