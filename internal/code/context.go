package code

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// parseContextRequest parses a context command from user input
// Format:
//
//	@file_path [question] - For file context (e.g., @main.go 这段代码的功能是什么?)
//	@folder_path [question] - For folder context (e.g., @internal/ 这个目录结构如何?)
//	@codebase [depth=n] [question] - For codebase context
//	@git command [question] - For git commands
//	@web "search query" [question] - For web search
func (h *Handler) parseContextRequest(input string) (*ContextRequest, error) {
	if !strings.HasPrefix(input, "@") {
		return nil, nil
	}

	// Remove the @ prefix
	input = strings.TrimPrefix(input, "@")

	// Split the input into parts
	parts := strings.SplitN(input, " ", 2)
	firstPart := parts[0]

	// Initialize the context request
	request := &ContextRequest{
		Depth: 1, // Default depth
	}

	var remainingText string
	if len(parts) > 1 {
		remainingText = parts[1]
	}

	// Check if the first part is a known context type
	knownTypes := map[string]ContextType{
		"file":     FileContext,
		"folder":   FolderContext,
		"codebase": CodebaseContext,
		"git":      GitContext,
		"web":      WebContext,
	}

	if contextType, exists := knownTypes[firstPart]; exists {
		// It's an explicit context type (like @codebase or @web)
		request.Type = contextType

		// Parse the remaining parts after the context type
		if remainingText != "" {
			// For git, the rest might be the command followed by a question
			if contextType == GitContext {
				// Try to extract command and question
				cmdParts := strings.SplitN(remainingText, " ", 2)
				request.Command = cmdParts[0]

				// If there's text after the command, it's the question
				if len(cmdParts) > 1 {
					request.Question = strings.TrimSpace(cmdParts[1])
				}
			} else if contextType == WebContext {
				// For web, try to extract the search query (which might be in quotes)
				// and the question
				if strings.HasPrefix(remainingText, "\"") || strings.HasPrefix(remainingText, "'") {
					// Extract quoted search query
					endQuoteIdx := strings.IndexAny(remainingText[1:], "\"'")
					if endQuoteIdx != -1 {
						endQuoteIdx++ // Adjust for the slice offset
						request.Target = remainingText[:endQuoteIdx+1]

						// If there's more text after the quoted part, it's the question
						if len(remainingText) > endQuoteIdx+1 {
							request.Question = strings.TrimSpace(remainingText[endQuoteIdx+1:])
						}
					} else {
						// No end quote found, use the first word as target
						parts := strings.SplitN(remainingText, " ", 2)
						request.Target = parts[0]
						if len(parts) > 1 {
							request.Question = strings.TrimSpace(parts[1])
						}
					}
				} else {
					// No quotes, use the first word as target
					parts := strings.SplitN(remainingText, " ", 2)
					request.Target = parts[0]
					if len(parts) > 1 {
						request.Question = strings.TrimSpace(parts[1])
					}
				}
			} else {
				// For other types, extract target, depth, and question
				// First check for depth parameter
				depthIdx := strings.Index(remainingText, "depth=")

				if depthIdx != -1 {
					// There's a depth parameter
					beforeDepth := remainingText[:depthIdx]
					depthPart := remainingText[depthIdx:]

					// Extract the depth value
					var depth int
					depthEndIdx := strings.IndexAny(depthPart, " \t\n")
					if depthEndIdx == -1 {
						depthEndIdx = len(depthPart)
					}

					fmt.Sscanf(depthPart[:depthEndIdx], "depth=%d", &depth)
					if depth > 0 {
						request.Depth = depth
					}

					// Extract target from before depth
					if beforeDepth != "" {
						targetParts := strings.SplitN(strings.TrimSpace(beforeDepth), " ", 2)
						request.Target = targetParts[0]

						// If there's more text before depth, it's part of the question
						if len(targetParts) > 1 {
							request.Question = strings.TrimSpace(targetParts[1])
						}
					}

					// If there's text after depth, it's the rest of the question
					if depthEndIdx < len(depthPart) {
						afterText := strings.TrimSpace(depthPart[depthEndIdx:])
						if request.Question != "" {
							request.Question += " " + afterText
						} else {
							request.Question = afterText
						}
					}
				} else {
					// No depth parameter, just target and possibly question
					targetParts := strings.SplitN(remainingText, " ", 2)
					request.Target = targetParts[0]

					if len(targetParts) > 1 {
						request.Question = strings.TrimSpace(targetParts[1])
					}
				}
			}
		}
	} else {
		// It's not an explicit type, so it must be a file or folder path
		// Check if it ends with / to determine if it's a folder
		isFolder := strings.HasSuffix(firstPart, "/")

		// If it's not clearly a folder by ending with /, check if it exists
		if !isFolder {
			fileInfo, err := os.Stat(firstPart)
			if err == nil {
				isFolder = fileInfo.IsDir()
			}
		}

		if isFolder {
			request.Type = FolderContext
			request.Target = firstPart
		} else {
			request.Type = FileContext
			request.Target = firstPart
		}

		// Parse depth and/or question from remaining text
		if remainingText != "" {
			depthIdx := strings.Index(remainingText, "depth=")

			if depthIdx != -1 {
				// There's a depth parameter
				beforeDepth := remainingText[:depthIdx]
				depthPart := remainingText[depthIdx:]

				// Extract the depth value
				var depth int
				depthEndIdx := strings.IndexAny(depthPart, " \t\n")
				if depthEndIdx == -1 {
					depthEndIdx = len(depthPart)
				}

				fmt.Sscanf(depthPart[:depthEndIdx], "depth=%d", &depth)
				if depth > 0 {
					request.Depth = depth
				}

				// If there's text before depth, it's part of the question
				if beforeDepth != "" {
					request.Question = strings.TrimSpace(beforeDepth)
				}

				// If there's text after depth, it's the rest of the question
				if depthEndIdx < len(depthPart) {
					afterText := strings.TrimSpace(depthPart[depthEndIdx:])
					if request.Question != "" {
						request.Question += " " + afterText
					} else {
						request.Question = afterText
					}
				}
			} else {
				// No depth parameter, the remaining text is the question
				request.Question = remainingText
			}
		}
	}

	return request, nil
}

// handleContextRequest processes a context request and returns the context information
func (h *Handler) handleContextRequest(request *ContextRequest) (string, error) {
	switch request.Type {
	case FileContext:
		return h.getFileContext(request.Target)
	case FolderContext:
		return h.getFolderContext(request.Target, request.Depth)
	case CodebaseContext:
		return h.getCodebaseContext(request.Depth)
	case GitContext:
		return h.getGitContext(request.Command)
	case WebContext:
		return h.getWebContext(request.Target)
	default:
		return "", fmt.Errorf("unknown context type: %s", request.Type)
	}
}

// getFileContext retrieves the content of a file
func (h *Handler) getFileContext(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("file path not specified")
	}

	content, err := readFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	return fmt.Sprintf("File: %s\n\n%s", path, content), nil
}

// getFolderContext retrieves the structure of a folder
func (h *Handler) getFolderContext(path string, depth int) (string, error) {
	if path == "" {
		path = "."
	}

	// Use a custom find command to get directory structure with limited depth
	cmd := exec.Command("find", path, "-type", "f", "-o", "-type", "d", "-not", "-path", "*/\\.*", "-maxdepth", fmt.Sprintf("%d", depth))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get folder structure: %v", err)
	}

	return fmt.Sprintf("Folder structure of %s (depth: %d):\n\n%s", path, depth, string(output)), nil
}

// getCodebaseContext retrieves a high-level overview of the codebase
func (h *Handler) getCodebaseContext(depth int) (string, error) {
	// Get root directory structure
	rootStructure, err := h.getFolderContext(".", depth)
	if err != nil {
		return "", err
	}

	// Get information about important files
	var importantFiles string
	for _, file := range []string{"go.mod", "go.sum", "main.go", "README.md"} {
		if fileInfo, err := os.Stat(file); err == nil && !fileInfo.IsDir() {
			content, _ := readFile(file)
			importantFiles += fmt.Sprintf("\n--- %s ---\n%s\n", file, content)
		}
	}

	return fmt.Sprintf("Codebase Overview:\n\n%s\n\nImportant Files:%s", rootStructure, importantFiles), nil
}

// getGitContext retrieves information from git
func (h *Handler) getGitContext(command string) (string, error) {
	if command == "" {
		command = "status"
	}

	parts := strings.Fields(command)
	gitCmd := exec.Command("git", parts...)
	output, err := gitCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %v", err)
	}

	return fmt.Sprintf("Git (%s):\n\n%s", command, string(output)), nil
}

// getWebContext performs a web search and retrieves relevant information
func (h *Handler) getWebContext(query string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("search query not specified")
	}

	// In a real implementation, we would integrate with a search API
	// For now, we'll return a message that acknowledges the search but indicates
	// it's not fully implemented

	// Remove quotes if present
	query = strings.Trim(query, "\"'")

	return fmt.Sprintf("Web search for: %s\n\n"+
		"Note: Web search is simulated in this version.\n"+
		"In a full implementation, this would integrate with a search API to provide real results.\n\n"+
		"The AI will use its knowledge to provide information about: %s",
		query, query), nil
}
