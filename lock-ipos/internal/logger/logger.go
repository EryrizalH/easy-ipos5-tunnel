package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var (
	logFile *os.File
	enabled bool
)

// Init initializes the logger
func Init() error {
	// Get the executable directory
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	logPath := filepath.Join(exeDir, "log.txt")

	// Open log file in append mode, create if not exists
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	enabled = true

	// Write startup message
	Info("=== Application Started ===")
	Info(fmt.Sprintf("Executable: %s", exePath))
	Info(fmt.Sprintf("Log file: %s", logPath))

	return nil
}

// Close closes the log file
func Close() {
	if logFile != nil {
		Info("=== Application Closed ===")
		logFile.Close()
	}
	enabled = false
}

// Info logs an info message
func Info(message string) {
	log("INFO", message)
}

// Error logs an error message
func Error(message string) {
	log("ERROR", message)
}

// Debug logs a debug message
func Debug(message string) {
	log("DEBUG", message)
}

// Warn logs a warning message
func Warn(message string) {
	log("WARN", message)
}

// log writes a log entry with timestamp and level
func log(level, message string) {
	if !enabled || logFile == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, message)

	logFile.WriteString(logEntry)
	logFile.Sync() // Flush to disk immediately
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	Debug(fmt.Sprintf(format, args...))
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	Warn(fmt.Sprintf(format, args...))
}
