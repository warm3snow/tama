package completion

import (
	"os"
	"strings"
)

// CommandCompleter implements generic command completion logic
type CommandCompleter struct {
	// Allow for mode-specific commands to be added in the future
	SpecificCommands []string
}

// NewCommandCompleter creates a new command completer
func NewCommandCompleter(specificCommands []string) *CommandCompleter {
	return &CommandCompleter{
		SpecificCommands: specificCommands,
	}
}

// DoComplete implements generic command completion logic
func (c *CommandCompleter) DoComplete(line []rune, pos int) (newLine [][]rune, length int) {
	// Get current input prefix
	lineStr := string(line[:pos])

	// Handle auto-completion for ! commands
	if len(lineStr) >= 1 && lineStr[0] == '!' {
		return c.completeShellCommands(lineStr[1:])
	}

	// Normal command completion - only handle commands starting with /
	if len(lineStr) > 0 && lineStr[0] == '/' {
		// Common commands + mode-specific commands
		commands := append([]string{"help"}, c.SpecificCommands...)
		prefix := lineStr[1:]

		// Filter commands based on prefix
		var candidates [][]rune
		for _, cmd := range commands {
			if strings.HasPrefix(cmd, prefix) {
				candidates = append(candidates, []rune(cmd))
			}
		}

		if len(candidates) == 0 {
			return nil, 0
		}

		// Return prefix length, keeping the / prefix
		return candidates, len(prefix)
	}

	return nil, 0
}

// completeShellCommands handles shell command auto-completion
func (c *CommandCompleter) completeShellCommands(cmdPrefix string) (newLine [][]rune, length int) {
	// Get all executable commands in the system
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil, 0
	}

	paths := strings.Split(pathEnv, ":")
	var matchingCommands []string

	// Check executable files in each PATH directory
	for _, path := range paths {
		files, err := os.ReadDir(path)
		if err != nil {
			continue
		}

		for _, file := range files {
			// Skip directories
			if file.IsDir() {
				continue
			}

			// Only match files starting with cmdPrefix
			fileName := file.Name()
			if strings.HasPrefix(fileName, cmdPrefix) {
				matchingCommands = append(matchingCommands, fileName)
			}

			// Limit the number of candidates to avoid overwhelming
			if len(matchingCommands) > 100 {
				break
			}
		}

		// Stop searching if we have enough candidates
		if len(matchingCommands) > 100 {
			break
		}
	}

	if len(matchingCommands) == 0 {
		return nil, 0
	}

	// If there's only one match, return the difference to append
	if len(matchingCommands) == 1 {
		// Only return the suffix that hasn't been typed
		suffix := matchingCommands[0][len(cmdPrefix):]
		return [][]rune{[]rune(suffix)}, 0 // Length 0 means don't replace anything, just append
	}

	// Find common prefix among all matches
	commonPrefix := matchingCommands[0]
	for _, cmd := range matchingCommands[1:] {
		i := 0
		for i < len(commonPrefix) && i < len(cmd) && commonPrefix[i] == cmd[i] {
			i++
		}
		commonPrefix = commonPrefix[:i]
	}

	// If common prefix is longer than input prefix, return the difference
	if len(commonPrefix) > len(cmdPrefix) {
		// Only return the suffix that hasn't been typed
		suffix := commonPrefix[len(cmdPrefix):]
		return [][]rune{[]rune(suffix)}, 0 // Length 0 means don't replace anything, just append
	}

	// If common prefix isn't longer than input prefix, show all matches
	// To match readline behavior, we need to return bare commands (without ! prefix)
	var candidates [][]rune
	for _, cmd := range matchingCommands {
		candidates = append(candidates, []rune(cmd))
	}

	// Return prefix length so readline will replace the current command part
	return candidates, len(cmdPrefix)
}
