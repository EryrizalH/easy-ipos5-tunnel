package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logFile *os.File
	enabled bool
	mu      sync.Mutex
)

// Init initializes the logger
func Init() error {
	// Get the executable directory
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	paths := []string{filepath.Join(filepath.Dir(exePath), "log.txt")}
	if programData := os.Getenv("ProgramData"); programData != "" {
		paths = append(paths, filepath.Join(programData, "nusatunnel-client", "logs", "setup.log"))
	}
	paths = append(paths, filepath.Join(os.TempDir(), "nusatunnel-client", "setup.log"))
	if err := initAtPaths(paths); err != nil {
		return err
	}

	// Write startup message
	Info("=== Application Started ===")
	Info(fmt.Sprintf("Executable: %s", exePath))
	Info(fmt.Sprintf("Log file: %s", Path()))

	return nil
}

func initAtPaths(paths []string) error {
	mu.Lock()
	defer mu.Unlock()
	var failures []error
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			failures = append(failures, err)
			continue
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if logFile != nil {
			logFile.Close()
		}
		logFile, enabled = file, true
		return nil
	}
	return fmt.Errorf("failed to open setup log: %w", errors.Join(failures...))
}

// Path is the actual setup log location, including a writable fallback.
func Path() string {
	mu.Lock()
	defer mu.Unlock()
	if !enabled || logFile == nil {
		return ""
	}
	return logFile.Name()
}

// Close closes the log file
func Close() {
	Info("=== Application Closed ===")
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
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
	mu.Lock()
	defer mu.Unlock()
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
