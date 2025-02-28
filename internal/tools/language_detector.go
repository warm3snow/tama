package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LanguageDetector implements language detection functionality
type LanguageDetector struct {
	workspacePath string
}

// NewLanguageDetector creates a new language detector tool
func NewLanguageDetector(workspacePath string) *LanguageDetector {
	return &LanguageDetector{
		workspacePath: workspacePath,
	}
}

func (t *LanguageDetector) Name() string {
	return "language_detector"
}

func (t *LanguageDetector) Description() string {
	return "Detect programming languages in the workspace"
}

// LanguageInfo contains information about a detected language
type LanguageInfo struct {
	Name       string  // Language name
	Files      int     // Number of files
	Percentage float64 // Percentage in the workspace
}

func (t *LanguageDetector) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Verify workspace path exists
	if _, err := os.Stat(t.workspacePath); err != nil {
		return "", fmt.Errorf("workspace path error: %v", err)
	}

	// Map of file extensions to languages
	languageMap := map[string]string{
		".go":      "Go",
		".py":      "Python",
		".js":      "JavaScript",
		".ts":      "TypeScript",
		".jsx":     "React",
		".tsx":     "React TypeScript",
		".vue":     "Vue",
		".java":    "Java",
		".cpp":     "C++",
		".c":       "C",
		".h":       "C/C++ Header",
		".rb":      "Ruby",
		".php":     "PHP",
		".rs":      "Rust",
		".swift":   "Swift",
		".kt":      "Kotlin",
		".scala":   "Scala",
		".cs":      "C#",
		".fs":      "F#",
		".r":       "R",
		".dart":    "Dart",
		".lua":     "Lua",
		".pl":      "Perl",
		".sh":      "Shell",
		".yaml":    "YAML",
		".yml":     "YAML",
		".json":    "JSON",
		".xml":     "XML",
		".html":    "HTML",
		".css":     "CSS",
		".scss":    "SCSS",
		".less":    "Less",
		".md":      "Markdown",
		".toml":    "TOML",
		".sql":     "SQL",
		".graphql": "GraphQL",
	}

	// Count files by language
	languageCount := make(map[string]int)
	totalFiles := 0
	var debugInfo strings.Builder
	debugInfo.WriteString(fmt.Sprintf("Scanning workspace: %s\n", t.workspacePath))

	// Walk through workspace
	err := filepath.Walk(t.workspacePath, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			debugInfo.WriteString(fmt.Sprintf("Error accessing %s: %v\n", path, err))
			return nil // Skip files we can't access
		}

		// Get relative path for logging
		relPath, err := filepath.Rel(t.workspacePath, path)
		if err != nil {
			debugInfo.WriteString(fmt.Sprintf("Error getting relative path for %s: %v\n", path, err))
			return nil
		}

		// Skip directories and hidden files
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" || info.Name() == "node_modules" {
				debugInfo.WriteString(fmt.Sprintf("Skipping directory: %s\n", relPath))
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasPrefix(info.Name(), ".") {
			debugInfo.WriteString(fmt.Sprintf("Skipping hidden file: %s\n", relPath))
			return nil
		}

		// Get file extension
		ext := strings.ToLower(filepath.Ext(path))
		if lang, ok := languageMap[ext]; ok {
			debugInfo.WriteString(fmt.Sprintf("Found %s file: %s\n", lang, relPath))
			languageCount[lang]++
			totalFiles++
		} else {
			debugInfo.WriteString(fmt.Sprintf("Ignoring unknown extension %s: %s\n", ext, relPath))
		}

		return nil
	})

	if err != nil {
		if err == context.Canceled {
			return debugInfo.String(), fmt.Errorf("scan canceled: %v", err)
		}
		return debugInfo.String(), fmt.Errorf("failed to walk workspace: %v", err)
	}

	if totalFiles == 0 {
		return fmt.Sprintf("%s\nNo source files detected in workspace", debugInfo.String()), nil
	}

	// Sort languages by file count
	var languages []LanguageInfo
	for lang, count := range languageCount {
		percentage := float64(count) / float64(totalFiles) * 100
		languages = append(languages, LanguageInfo{
			Name:       lang,
			Files:      count,
			Percentage: percentage,
		})
	}

	// Sort by percentage in descending order
	sort.Slice(languages, func(i, j int) bool {
		return languages[i].Percentage > languages[j].Percentage
	})

	// Format output
	var output strings.Builder
	output.WriteString(debugInfo.String())
	output.WriteString("\nDetected Languages:\n")
	for _, lang := range languages {
		output.WriteString(fmt.Sprintf("- %s: %d files (%.1f%%)\n",
			lang.Name, lang.Files, lang.Percentage))
	}

	return output.String(), nil
}
