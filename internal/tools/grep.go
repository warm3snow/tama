package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GrepSearchTool implements code search functionality
type GrepSearchTool struct {
	workspacePath string
}

// NewGrepSearchTool creates a new grep search tool
func NewGrepSearchTool(workspacePath string) *GrepSearchTool {
	return &GrepSearchTool{
		workspacePath: workspacePath,
	}
}

func (t *GrepSearchTool) Name() string {
	return "grep_search"
}

func (t *GrepSearchTool) Description() string {
	return "Search for patterns in files"
}

func (t *GrepSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract arguments
	pattern, ok := args["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("pattern argument required")
	}

	// Optional arguments
	includePattern, _ := args["include"].(string)
	excludePattern, _ := args["exclude"].(string)
	caseSensitive, _ := args["case_sensitive"].(bool)
	maxDepth, _ := args["depth"].(float64)

	// Convert maxDepth to int
	depth := -1
	if maxDepth > 0 {
		depth = int(maxDepth)
	}

	// Store results
	var results []string
	resultCount := 0

	// Walk through workspace
	err := filepath.Walk(t.workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Get relative path
		relPath, err := filepath.Rel(t.workspacePath, path)
		if err != nil {
			return nil
		}

		// Check depth
		if depth > 0 {
			if strings.Count(relPath, string(os.PathSeparator)) > depth {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Check include/exclude patterns
		if includePattern != "" {
			matched, err := filepath.Match(includePattern, info.Name())
			if err != nil || !matched {
				return nil
			}
		}
		if excludePattern != "" {
			matched, err := filepath.Match(excludePattern, info.Name())
			if err == nil && matched {
				return nil
			}
		}

		// If pattern is ".", just return the file path
		if pattern == "." {
			results = append(results, relPath)
			resultCount++
			if resultCount >= 50 {
				return fmt.Errorf("max results reached")
			}
			return nil
		}

		// Open and scan file
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Check if line contains pattern
			found := false
			if caseSensitive {
				found = strings.Contains(line, pattern)
			} else {
				found = strings.Contains(strings.ToLower(line), strings.ToLower(pattern))
			}

			if found {
				result := fmt.Sprintf("%s:%d:%s", relPath, lineNum, line)
				results = append(results, result)
				resultCount++
				if resultCount >= 50 {
					return fmt.Errorf("max results reached")
				}
			}
		}

		return nil
	})

	if err != nil && err.Error() != "max results reached" {
		return "", fmt.Errorf("search failed: %v", err)
	}

	if len(results) == 0 {
		return "No matches found", nil
	}

	return strings.Join(results, "\n"), nil
}
