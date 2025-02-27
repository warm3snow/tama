package code

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/warm3snow/tama/internal/chat"
	"github.com/warm3snow/tama/internal/completion"
	"github.com/warm3snow/tama/internal/config"
	"github.com/warm3snow/tama/internal/llm"
	"github.com/warm3snow/tama/internal/logging"
)

// Handler manages code assistant sessions
type Handler struct {
	client      *llm.Client
	chatHandler *chat.ChatHandler
	userStyle   func(string)
	aiStyle     func(string)
	cmdStyle    *color.Color
	codeStyle   *color.Color
	errorStyle  *color.Color
	config      config.Config
	commands    map[string]SlashCommand
}

// NewHandler creates a new code assistant handler
func NewHandler(config config.Config) *Handler {
	client := llm.NewClient(config)
	logging.LogLLMRequest(client.GetProvider(), client.GetModel(), 0)

	chatHandler := chat.NewChatHandler(client, true)

	handler := &Handler{
		client:      client,
		chatHandler: chatHandler,
		userStyle:   chatHandler.GetUserStyler(),
		aiStyle:     chatHandler.GetAIStyler(),
		cmdStyle:    color.New(color.FgYellow).Add(color.Bold),
		codeStyle:   color.New(color.FgGreen),
		errorStyle:  color.New(color.FgRed),
		config:      config,
	}

	handler.commands = handler.setupSlashCommands()

	return handler
}

// Start begins the interactive code assistant session
func (h *Handler) Start() {
	// Show welcome message
	h.showWelcomeMessage()

	// Initialize readline
	rl := h.initializeReadline()
	if rl == nil {
		return
	}
	defer rl.Close()

	// Main interaction loop
	h.interactionLoop(rl)
}

// interactionLoop handles the main interaction loop
func (h *Handler) interactionLoop(rl *readline.Instance) {
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
			fmt.Printf("Error reading input: %v\n", err)
			break
		}

		// 处理输入
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// 处理!命令
		if strings.HasPrefix(input, "!") {
			cmdStr := strings.TrimPrefix(input, "!")
			cmdStr = strings.TrimSpace(cmdStr)
			if cmdStr != "" {
				if err := executeCommand(cmdStr); err != nil {
					fmt.Printf("Error executing command: %v\n", err)
				}
				continue
			}
		}

		// 处理/命令
		if strings.HasPrefix(input, "/") {
			handled, isInteractive, _ := h.handleSlashCommand(input)
			if handled {
				if isInteractive {
					// 重新初始化readline，因为交互式命令可能会影响终端状态
					rl.Close()
					rl = h.initializeReadline()
					if rl == nil {
						fmt.Println("Error: failed to reinitialize readline")
						return
					}
				}
				continue
			}
		}

		// Display valid input
		h.userStyle(input)

		// Process the input
		needReset := h.processInput(input)

		// Add to history
		rl.SaveHistory(input)

		// Reset readline if needed
		if needReset {
			rl.Close()
			rl = h.initializeReadline()
			if rl == nil {
				fmt.Println("Error reinitializing readline after command execution")
				return
			}
		}
	}
}

// processInput processes user input and returns true if readline needs to be reset
func (h *Handler) processInput(input string) bool {
	// Check if this is a slash command
	if strings.HasPrefix(input, "/") {
		cmdHandled, needReset, _ := h.handleSlashCommand(input)
		return needReset && cmdHandled
	}

	// Check if this is a code-related request
	actions, success := h.analyzeCodeRequest(input)
	if success && len(actions) > 0 {
		h.handleCodeActions(actions)
		return false
	}

	// If all else fails, treat as a regular chat message
	response, err := h.client.SendMessage(input)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return false
	}

	// Display AI response
	h.aiStyle(response)

	// Update conversation history
	h.client.UpdateConversation(input, response)

	return false
}

// handleCodeActions processes code actions returned by the LLM
func (h *Handler) handleCodeActions(actions []CodeAction) {
	h.cmdStyle.Printf("\nSuggested code actions:\n\n")

	for i, action := range actions {
		fmt.Printf("[%d] %s: %s\n", i+1, action.Type, action.Description)
	}

	fmt.Println("\nImplementation of code actions will be added in future versions.")

	// In a full implementation, you would:
	// 1. Display the actions to the user
	// 2. Let the user select which action to take
	// 3. Execute the selected action (edit file, create file, etc.)
	// 4. Show the results
}

// showWelcomeMessage displays a welcome message at the start of the session
func (h *Handler) showWelcomeMessage() {
	modelInfo := color.New(color.FgCyan)
	fmt.Println("Welcome to the Tama AI Code Assistant")
	modelInfo.Printf("Connected to %s model: %s\n",
		h.client.GetProvider(),
		h.client.GetModel())
	fmt.Println("Type 'exit' or 'quit' to end the session.")
	fmt.Println("Type '/help' to see available commands.")
}

// initializeReadline initializes the readline instance
func (h *Handler) initializeReadline() *readline.Instance {
	// 代码模式特定的命令
	codeSpecificCommands := []string{"cd"}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[32m>\033[0m ",
		HistoryFile:     "/tmp/tama_history.txt",
		AutoComplete:    completion.NewReadlineCompleter(codeSpecificCommands),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})

	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		return nil
	}

	return rl
}

// executeCommand executes a shell command
func executeCommand(cmdStr string) error {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// isInteractiveTerminalCommand checks if the command is interactive
func isInteractiveTerminalCommand(cmdStr string) bool {
	// Add common interactive commands here
	interactiveCommands := []string{
		"vim", "nano", "less", "more", "top", "htop",
		"vi", "emacs", "pico", "jed", "joe", "ne",
	}

	for _, cmd := range interactiveCommands {
		if strings.HasPrefix(cmdStr, cmd+" ") || cmdStr == cmd {
			return true
		}
	}

	return false
}
