package code

import (
	"fmt"
	"os"
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
				"help", "cd", "file", "folder", "codebase", "git", "web", "!",
			} {
				if command, ok := commands[cmd]; ok {
					cmdStyle.Printf("  /%s", command.Name)
					descStyle.Printf(" - %s\n", command.Description)
				}
			}

			// Display context prefixes
			fmt.Println("\nContext shortcuts (can be used anywhere in your message):")
			cmdStyle.Printf("  @file <path>")
			descStyle.Printf(" - View file contents\n")
			cmdStyle.Printf("  @folder <path> [depth=n]")
			descStyle.Printf(" - View folder structure\n")
			cmdStyle.Printf("  @codebase [depth=n]")
			descStyle.Printf(" - Analyze codebase structure\n")
			cmdStyle.Printf("  @git <command>")
			descStyle.Printf(" - Run git command and analyze results\n")
			cmdStyle.Printf("  @web \"<search query>\"")
			descStyle.Printf(" - Search the web for information\n")

			return nil
		},
	}

	// CD command
	commands["cd"] = SlashCommand{
		Name:        "cd",
		Description: "Change current directory",
		Execute: func() error {
			fmt.Printf("\nCurrent directory: %s\n", getCurrentDirectory())
			return nil
		},
	}

	// File command
	commands["file"] = SlashCommand{
		Name:        "file",
		Description: "View or analyze a file",
		Execute: func() error {
			// This is just a help command - actual file operations are done via context requests
			fmt.Println("\nUse @file <path> to view or analyze a file.")
			fmt.Println("Example: @file main.go")
			return nil
		},
	}

	// Folder command
	commands["folder"] = SlashCommand{
		Name:        "folder",
		Description: "View folder structure",
		Execute: func() error {
			// This is just a help command - actual folder operations are done via context requests
			fmt.Println("\nUse @folder <path> [depth=n] to view a folder structure.")
			fmt.Println("Example: @folder ./internal depth=2")
			return nil
		},
	}

	// Codebase command
	commands["codebase"] = SlashCommand{
		Name:        "codebase",
		Description: "Analyze the codebase structure",
		Execute: func() error {
			// This is just a help command - actual codebase operations are done via context requests
			fmt.Println("\nUse @codebase [depth=n] to analyze the codebase structure.")
			fmt.Println("Example: @codebase depth=3")
			return nil
		},
	}

	// Git command
	commands["git"] = SlashCommand{
		Name:        "git",
		Description: "Run git commands with AI analysis",
		Execute: func() error {
			// This is just a help command - actual git operations are done via context requests
			fmt.Println("\nUse @git <command> to run a git command with AI analysis.")
			fmt.Println("Example: @git status")
			fmt.Println("Example: @git log --oneline -n 5")
			return nil
		},
	}

	// Web command
	commands["web"] = SlashCommand{
		Name:        "web",
		Description: "Search the web for information",
		Execute: func() error {
			// This is just a help command - actual web operations are done via context requests
			fmt.Println("\nUse @web \"query\" to search the web for information.")
			fmt.Println("Example: @web \"golang context switching\"")
			return nil
		},
	}

	// Shell command
	commands["!"] = SlashCommand{
		Name:        "!",
		Description: "Execute a shell command",
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

// getCurrentDirectory gets the current working directory
func getCurrentDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		return "Error: Unable to determine current directory"
	}
	return dir
}
