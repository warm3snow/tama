package code

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// parseContextRequest parses a context command from user input
// Format:
//
//	@file_path [question] - For file context (e.g., @main.go 这段代码的功能是什么?)
//	@folder_path [question] - For folder context (e.g., @internal/ 这个目录结构如何?)
//	@codebase [depth=n] [question] - For codebase context (e.g., @codebase 分析一下)
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
			if contextType == GitContext {
				// For git, the rest might be the command followed by a question
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
					// No depth parameter, just question (for codebase) or target and question (for others)
					if contextType == CodebaseContext {
						// For codebase without depth, entire text is the question
						request.Question = remainingText
					} else {
						// For other types, extract target and question
						targetParts := strings.SplitN(remainingText, " ", 2)
						request.Target = targetParts[0]

						if len(targetParts) > 1 {
							request.Question = strings.TrimSpace(targetParts[1])
						}
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

	// 自动识别和扫描重要文件
	var importantFiles string

	// 定义要扫描的文件类型（按语言分类）
	codeFileExtensions := map[string]bool{
		// Go
		".go":  true,
		".mod": true,
		".sum": true,
		// Python
		".py":  true,
		".pyw": true,
		".pyx": true,
		".pxd": true,
		".pyi": true,
		// JavaScript/TypeScript
		".js":  true,
		".jsx": true,
		".ts":  true,
		".tsx": true,
		// Java
		".java":  true,
		".jar":   true,
		".class": true,
		// Rust
		".rs":   true,
		".rlib": true,
		// C/C++
		".c":   true,
		".cpp": true,
		".h":   true,
		".hpp": true,
		// 配置文件
		".proto": true,
		".yaml":  true,
		".yml":   true,
		".json":  true,
		".toml":  true,
		".ini":   true,
		".conf":  true,
	}

	// 定义重要的文件名
	importantFileNames := map[string]bool{
		// 文档
		"README.md":  true,
		"README.rst": true,
		"README.txt": true,
		"LICENSE":    true,
		// 构建和依赖
		"Makefile":         true,
		"setup.py":         true,
		"requirements.txt": true,
		"package.json":     true,
		"Cargo.toml":       true,
		"CMakeLists.txt":   true,
		// 容器化
		"Dockerfile":         true,
		"docker-compose.yml": true,
		// 配置文件
		".gitignore":     true,
		"tox.ini":        true,
		"pyproject.toml": true,
		".env.example":   true,
	}

	// 读取.gitignore文件
	gitignorePatterns := make([]string, 0)
	if gitignoreContent, err := readFile(".gitignore"); err == nil {
		lines := strings.Split(gitignoreContent, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				gitignorePatterns = append(gitignorePatterns, line)
			}
		}
	}

	// 检查路径是否应该被忽略
	isIgnored := func(path string) bool {
		// 始终忽略.git目录
		if strings.Contains(path, "/.git/") || path == ".git" {
			return true
		}

		// 检查是否匹配.gitignore模式
		for _, pattern := range gitignorePatterns {
			// 处理通配符模式
			if strings.HasPrefix(pattern, "*") {
				suffix := strings.TrimPrefix(pattern, "*")
				if strings.HasSuffix(path, suffix) {
					return true
				}
			} else if strings.HasSuffix(pattern, "/*") {
				prefix := strings.TrimSuffix(pattern, "/*")
				if strings.HasPrefix(path, prefix+"/") {
					return true
				}
			} else if strings.Contains(pattern, "*") {
				// TODO: 实现更复杂的通配符匹配
				continue
			} else {
				// 直接匹配
				if path == pattern || strings.HasPrefix(path, pattern+"/") {
					return true
				}
			}
		}
		return false
	}

	// 使用filepath.Walk遍历目录
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过隐藏目录和文件（除了.gitignore等特定文件）
		if strings.HasPrefix(filepath.Base(path), ".") && path != "." &&
			!importantFileNames[filepath.Base(path)] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查是否应该忽略该路径
		if isIgnored(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 跳过常见的依赖目录
		if info.IsDir() && (path == "vendor" || path == "node_modules" ||
			path == "__pycache__" || path == "venv" || path == "env" ||
			path == "target" || path == "dist" || path == "build") {
			return filepath.SkipDir
		}

		// 检查是否是文件
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			baseName := filepath.Base(path)

			// 检查是否是重要文件名或支持的代码文件类型
			if importantFileNames[baseName] || codeFileExtensions[ext] {
				// 读取文件内容
				content, err := readFile(path)
				if err != nil {
					return nil // 继续处理其他文件
				}

				// 对于较大的文件，只读取前几行
				if len(content) > 1000 {
					lines := strings.SplitN(content, "\n", 21)
					if len(lines) > 20 {
						content = strings.Join(lines[:20], "\n") + "\n... (file truncated)"
					}
				}

				importantFiles += fmt.Sprintf("\n--- %s ---\n%s\n", path, content)
			}
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to walk directory: %v", err)
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
