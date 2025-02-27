package code

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// setupSlashCommands sets up the available slash commands
func (h *Handler) setupSlashCommands() map[string]SlashCommand {
	commands := make(map[string]SlashCommand)

	// Help command
	commands["help"] = SlashCommand{
		Name:        "help",
		Description: "Display available commands",
		Execute: func() error {
			cmdStyle := color.New(color.FgYellow).Add(color.Bold)
			descStyle := color.New(color.FgWhite)

			fmt.Println("\nAvailable commands:")

			// Sort and display commands
			for _, cmd := range []string{
				"help", "!", "@", "reset",
			} {
				if command, ok := commands[cmd]; ok {
					if cmd == "help" || cmd == "reset" {
						cmdStyle.Printf("  /%s", command.Name)
					} else {
						cmdStyle.Printf("  %s", command.Name)
					}
					descStyle.Printf(" - %s\n", command.Description)
				}
			}

			// Display context shortcuts
			fmt.Println("\nContext shortcuts (can be used anywhere in your message):")
			cmdStyle.Printf("  @file_name")
			descStyle.Printf(" - File as context\n")
			cmdStyle.Printf("  @folder_name")
			descStyle.Printf(" - Folder as context\n")
			cmdStyle.Printf("  @codebase")
			descStyle.Printf(" - Codebase as context\n")
			cmdStyle.Printf("  @web")
			descStyle.Printf(" - Enable web browsing\n")

			return nil
		},
	}

	// Reset command
	commands["reset"] = SlashCommand{
		Name:        "reset",
		Description: "Reset conversation history",
		Execute: func() error {
			h.client.ResetConversation()
			h.cmdStyle.Printf("\nConversation has been reset.\n")
			return nil
		},
	}

	// @XYZ
	commands["@"] = SlashCommand{
		Name:        "@",
		Description: "Add a context to the LLM, e.g. @main.go",
		Execute: func() error {
			return nil
		},
	}

	// Shell command
	commands["!"] = SlashCommand{
		Name:        "!",
		Description: "Execute a shell command, e.g. /!ls -la",
		Execute: func() error {
			fmt.Println("\nUse /! followed by a shell command to execute it.")
			fmt.Println("Example: /! ls -la")
			return nil
		},
	}

	return commands
}

// handleSlashCommand processes slash commands
func (h *Handler) handleSlashCommand(input string) (bool, bool, string) {
	if !strings.HasPrefix(input, "/") {
		return false, false, ""
	}

	// Extract command and arguments
	parts := strings.SplitN(strings.TrimPrefix(input, "/"), " ", 2)
	cmdName := parts[0]

	// Special case for shell command execution with /!
	if cmdName == "!" {
		var shellCmd string
		if len(parts) > 1 {
			shellCmd = parts[1]
			h.cmdStyle.Printf("Running command: %s\n", shellCmd)
			if err := executeCommand(shellCmd); err != nil {
				h.errorStyle.Printf("Error executing command: %v\n", err)
			}
		} else {
			h.errorStyle.Printf("No command specified after /!\n")
		}
		return true, false, ""
	}

	// Handle other commands
	cmd, ok := h.commands[cmdName]
	if !ok {
		h.errorStyle.Printf("Unknown command: /%s\n", cmdName)
		return true, false, ""
	}

	// Execute the command
	if err := cmd.Execute(); err != nil {
		h.errorStyle.Printf("Error executing command: %v\n", err)
	}

	return true, false, ""
}
