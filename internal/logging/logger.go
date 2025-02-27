package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const (
	// MaxLogSize is the maximum size in bytes for the log file (10MB)
	MaxLogSize = 10 * 1024 * 1024

	// DefaultLogDir is the default directory for log files
	DefaultLogDir = "~/.config/tama/logs"

	// DefaultLogFile is the default log file name
	DefaultLogFile = "server.log"
)

var (
	// Logger is the global logger instance
	Logger *slog.Logger

	// logFile is the current log file
	logFile *os.File
)

// InitLogger initializes the logger with file output only
func InitLogger() error {
	// Expand home directory if needed
	logDir, err := expandPath(DefaultLogDir)
	if err != nil {
		return fmt.Errorf("failed to expand log directory path: %v", err)
	}

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Set up log file
	logFilePath := filepath.Join(logDir, DefaultLogFile)

	// Open log file with append mode, create if not exists
	logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	// Create a JSON handler with timestamp
	handler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(time.Now().Format(time.RFC3339))
			}
			return a
		},
	})

	// Set up the logger
	Logger = slog.New(handler)

	// Replace the default logger
	slog.SetDefault(Logger)

	// Log initialization
	Logger.Info("Logger initialized", "path", logFilePath)

	// Set up log rotation check
	go monitorLogSize(logFilePath)

	return nil
}

// expandPath expands the ~ to the user's home directory
func expandPath(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, path[1:]), nil
}

// monitorLogSize periodically checks the log file size and rotates if needed
func monitorLogSize(logFilePath string) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if logFile == nil {
			continue
		}

		// Get file info
		fileInfo, err := logFile.Stat()
		if err != nil {
			Logger.Error("Failed to get log file info", "error", err)
			continue
		}

		// Check if rotation is needed
		if fileInfo.Size() >= MaxLogSize {
			rotateLogFile(logFilePath)
		}
	}
}

// rotateLogFile rotates the current log file
func rotateLogFile(logFilePath string) {
	// Close current file
	if logFile != nil {
		logFile.Close()
	}

	// Create backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s", logFilePath, timestamp)

	// Rename current log file to backup
	err := os.Rename(logFilePath, backupPath)
	if err != nil {
		Logger.Error("Failed to rotate log file", "error", err)
		return
	}

	// Open a new log file
	newLogFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		Logger.Error("Failed to create new log file", "error", err)
		return
	}

	// Update the logFile reference
	logFile = newLogFile

	// Update the logger to use the new file
	handler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	Logger = slog.New(handler)
	slog.SetDefault(Logger)

	Logger.Info("Log file rotated", "old", backupPath, "new", logFilePath)
}

// Close properly closes the log file
func Close() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// LogLLMRequest logs an LLM request
func LogLLMRequest(provider string, model string, messageLength int) {
	Logger.Info("LLM Request",
		"provider", provider,
		"model", model,
		"messageLength", messageLength)
}

// LogLLMResponse logs an LLM response
func LogLLMResponse(provider string, model string, responseLength int, error error) {
	if error != nil {
		Logger.Error("LLM Response Failed",
			"provider", provider,
			"model", model,
			"error", error)
	} else {
		Logger.Info("LLM Response",
			"provider", provider,
			"model", model,
			"responseLength", responseLength)
	}
}

// LogAppStart logs application startup
func LogAppStart(version string) {
	Logger.Info("App Started", "version", version)
}

// LogAppExit logs application exit
func LogAppExit() {
	Logger.Info("App Exited")
}

// LogError logs an error
func LogError(msg string, args ...any) {
	Logger.Error(msg, args...)
}
