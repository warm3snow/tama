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

// processInput processes user input and returns true if the session should end
func (h *Handler) processInput(input string) bool {
	// Check if the input is a slash command
	if isSlashCommand, shouldExit, userInput := h.handleSlashCommand(input); isSlashCommand {
		if shouldExit {
			return true
		}
		if userInput != "" {
			input = userInput
		} else {
			return false
		}
	}

	// Check if input contains multiple @ commands
	if strings.Contains(input, " @") || strings.HasPrefix(input, "@") {
		// Split input by spaces to handle each potential command
		tokens := strings.Fields(input)
		var userQuestion string
		var nonContextText []string

		// Process each token that might be a context command
		for _, token := range tokens {
			if strings.HasPrefix(token, "@") {
				// This is a potential context command
				contextRequest, err := h.parseContextRequest(token)
				if err != nil {
					h.errorStyle.Printf("Failed to parse context request: %v\n", err)
					nonContextText = append(nonContextText, token)
					continue
				}

				if contextRequest != nil {
					// Extract the question if any
					if contextRequest.Question != "" {
						// Use the last question found
						userQuestion = contextRequest.Question
						// Clear the question so it doesn't get processed multiple times
						contextRequest.Question = ""
					}

					// Handle context request
					contextInfo, err := h.handleContextRequest(contextRequest)
					if err != nil {
						h.errorStyle.Printf("Failed to get context: %v\n", err)
						nonContextText = append(nonContextText, token)
						continue
					}

					// Add context to chat history for subsequent LLM conversations
					message := fmt.Sprintf("Context (%s): %s", contextRequest.Type, contextInfo)
					h.chatHandler.AddSystemMessage(message)

					// Let the user know that context was loaded
					h.cmdStyle.Printf("Added %s context to the conversation: %s\n",
						contextRequest.Type,
						getContextSummary(contextRequest))
				} else {
					// Not a valid context command, treat as regular text
					nonContextText = append(nonContextText, token)
				}
			} else {
				// Not a context command, add to non-context text
				nonContextText = append(nonContextText, token)
			}
		}

		// If there's any non-context text, use it as the question
		if len(nonContextText) > 0 {
			combinedText := strings.Join(nonContextText, " ")
			if strings.TrimSpace(combinedText) != "" {
				userQuestion = strings.TrimSpace(combinedText)
			}
		}

		// If we have a question to ask, send it to the LLM
		if userQuestion != "" {
			// Display the user question
			h.userStyle(userQuestion)

			// Send the question to the LLM
			_, err := h.chatHandler.SendMessage(userQuestion)
			if err != nil {
				h.errorStyle.Printf("Error: %v\n", err)
			}
		}

		return false
	}

	// Check if this is a single context request (processed as before)
	contextRequest, err := h.parseContextRequest(input)
	if err != nil {
		h.errorStyle.Printf("Failed to parse context request: %v\n", err)
		return false
	}

	if contextRequest != nil {
		// Handle context request
		contextInfo, err := h.handleContextRequest(contextRequest)
		if err != nil {
			h.errorStyle.Printf("Failed to get context: %v\n", err)
			return false
		}

		// Add context to chat history for subsequent LLM conversations
		message := fmt.Sprintf("Context (%s): %s", contextRequest.Type, contextInfo)
		h.chatHandler.AddSystemMessage(message)

		// Let the user know that context was loaded without displaying the entire content
		h.cmdStyle.Printf("Added %s context to the conversation: %s\n",
			contextRequest.Type,
			getContextSummary(contextRequest))

		// If the user included a question with the context, send it to the LLM
		if contextRequest.Question != "" {
			// Display the user question
			h.userStyle(contextRequest.Question)

			// Send the question to the LLM
			_, err := h.chatHandler.SendMessage(contextRequest.Question)
			if err != nil {
				h.errorStyle.Printf("Error: %v\n", err)
			}
		}

		return false
	}

	// Check if the input is a terminal command
	isCommand, command, err := h.analyzeIfCommand(input)
	if err != nil {
		h.errorStyle.Printf("Error analyzing input: %v\n", err)
		return false
	}

	if isCommand {
		h.cmdStyle.Printf("Running command: %s\n", command)
		if err := executeCommand(command); err != nil {
			h.errorStyle.Printf("Error executing command: %v\n", err)
		}
		return false
	}

	// Process normal input with the chat handler
	_, err = h.chatHandler.SendMessage(input)
	if err != nil {
		h.errorStyle.Printf("Error: %v\n", err)
	}

	return false
}

// getContextSummary provides a user-friendly summary of the context that was added
func getContextSummary(request *ContextRequest) string {
	switch request.Type {
	case FileContext:
		return fmt.Sprintf("file '%s'", request.Target)
	case FolderContext:
		return fmt.Sprintf("folder '%s' (depth: %d)", request.Target, request.Depth)
	case CodebaseContext:
		return fmt.Sprintf("codebase structure (depth: %d)", request.Depth)
	case GitContext:
		return fmt.Sprintf("git command '%s'", request.Command)
	case WebContext:
		return fmt.Sprintf("web search for '%s'", strings.Trim(request.Target, "\"'"))
	default:
		return string(request.Type)
	}
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
