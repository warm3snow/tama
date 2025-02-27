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
			for _, cmd := range []string{"help", "cd", "!"} {
				if command, ok := commands[cmd]; ok {
					cmdStyle.Printf("  /%s", command.Name)
					descStyle.Printf(" - %s\n", command.Description)
				}
			}

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

	// Shell command
	commands["!"] = SlashCommand{
		Name:        "!",
		Description: "Execute a shell command",
		Execute: func() error {
			fmt.Println("\nUse the command after ! to execute a shell command")
			return nil
		},
	}

	return commands
}

// handleSlashCommand handles slash commands
func (h *Handler) handleSlashCommand(input string) (bool, bool, string) {
	// 特殊处理!命令的情况，允许!ls这样的格式
	if strings.HasPrefix(input, "!") {
		// 将!后面的部分作为命令参数
		shellCmd := strings.TrimPrefix(input, "!")
		shellCmd = strings.TrimSpace(shellCmd)
		if shellCmd != "" {
			// 如果!后面直接跟着命令，如!ls
			// 仅高亮显示命令本身
			err := executeCommand(shellCmd)
			if err != nil {
				fmt.Printf("Error executing command: %v\n", err)
			}
			return true, isInteractiveTerminalCommand(shellCmd), ""
		}
		return true, false, ""
	}

	// Split the input into command and arguments
	parts := strings.SplitN(input, " ", 2)
	cmdName := strings.TrimPrefix(parts[0], "/")
	var args string
	if len(parts) > 1 {
		args = parts[1]
	}

	// Check if the command exists
	cmd, ok := h.commands[cmdName]
	if !ok {
		fmt.Printf("\nUnknown command: /%s\n", cmdName)
		fmt.Println("Type /help to see available commands")
		return false, false, ""
	}

	// Special case for CD command with argument
	if cmdName == "cd" && args != "" {
		// Change directory
		targetDir := args
		err := os.Chdir(targetDir)
		if err != nil {
			fmt.Printf("\nError changing directory: %v\n", err)
			return true, false, ""
		}

		fmt.Printf("\nChanged directory to: %s\n", getCurrentDirectory())
		return true, false, ""
	}

	// Special case for ! command with argument
	if cmdName == "!" && args != "" {
		// Execute shell command
		err := executeCommand(args)
		if err != nil {
			fmt.Printf("Error executing command: %v\n", err)
		}

		return true, isInteractiveTerminalCommand(args), ""
	}

	// Execute the command
	err := cmd.Execute()
	if err != nil {
		fmt.Printf("\nError executing command: %v\n", err)
	}

	return true, false, ""
}

// getCurrentDirectory returns the current working directory
func getCurrentDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}
