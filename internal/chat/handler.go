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

	// 聊天模式特定的命令
	chatSpecificCommands := []string{}

	// 初始化readline
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
		// 使用readline获取输入
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

		// 检查是否是!命令
		if strings.HasPrefix(input, "!") {
			cmdStr := strings.TrimPrefix(input, "!")
			cmdStr = strings.TrimSpace(cmdStr)

			if cmdStr == "" {
				fmt.Println("Error: No command specified after !")
				continue
			}

			// 使用系统默认的shell输出
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

		// 添加/help命令
		if input == "/help" {
			h.showHelpMessage()
			continue
		}

		// Show user input with styling
		h.userStyle.Printf("\nYou: %s\n", input)

		// Print AI prompt without the response
		fmt.Print("\nAI: ")

		// Define the callback for streaming responses
		callback := func(chunk string) {
			// Print each chunk without newlines to simulate streaming
			fmt.Print(chunk)
		}

		// Get response from AI with streaming
		response, err := h.client.SendMessageWithCallback(input, callback)
		if err != nil {
			fmt.Printf("\nError: %v\n\n", err)
			continue
		}

		// Print newlines after the response
		fmt.Print("\n\n")

		// Update conversation history
		h.client.UpdateConversation(input, response)

		// 添加到readline历史
		rl.SaveHistory(input)
	}

	return nil
}

// SendMessage sends a single message and returns the response
func (h *ChatHandler) SendMessage(message string) (string, error) {
	// Define the callback for streaming responses
	callback := func(chunk string) {
		// Print each chunk without newlines to simulate streaming
		fmt.Print(chunk)
	}

	// Get response from AI with streaming
	response, err := h.client.SendMessageWithCallback(message, callback)
	if err != nil {
		return "", err
	}

	// Update conversation history
	h.client.UpdateConversation(message, response)

	return response, nil
}

// AddSystemMessage adds a system message to the conversation history
func (h *ChatHandler) AddSystemMessage(message string) {
	// Add a new system message to the client's conversation
	h.client.AddSystemMessage(message)
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
