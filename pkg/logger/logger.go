package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents the log level type
type LogLevel int

const (
	// Log levels
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
)

// levelNames maps LogLevel to its string representation
var levelNames = map[LogLevel]string{
	DEBUG:   "DEBUG",
	INFO:    "INFO",
	WARNING: "WARN",
	ERROR:   "ERROR",
}

// Logger is our logging structure
type Logger struct {
	level    LogLevel
	outputs  []io.Writer
	uiActive bool // Indicates if an interactive UI is active
	mu       sync.Mutex
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// LoggerOptions defines options for initializing the logger
type LoggerOptions struct {
	Level    LogLevel
	FilePath string
	Verbose  bool
	UIActive bool // Indicates if an interactive UI is active
}

// DefaultLoggerOptions returns the default options
func DefaultLoggerOptions() *LoggerOptions {
	return &LoggerOptions{
		Level:    INFO,
		FilePath: "",    // Default path is now set by the caller (rootCmd)
		UIActive: false, // Default to false, commands enabling UI should set it
	}
}

// Initialize initializes the default logger with the given options
func Initialize(opts *LoggerOptions) {
	if opts == nil {
		opts = DefaultLoggerOptions()
	}

	once.Do(func() {
		outputs := []io.Writer{}

		// If verbosity is enabled AND UI is NOT active, add stdout
		// Note: Verbose flag isn't explicitly used here currently,
		// stdout logging depends only on UIActive state.
		// Consider adding opts.Verbose check if needed.
		if !opts.UIActive {
			outputs = append(outputs, os.Stdout)
		}

		// ALWAYS add file output if FilePath is specified
		if opts.FilePath != "" {
			// Create the directory if necessary
			if err := os.MkdirAll(filepath.Dir(opts.FilePath), 0755); err != nil {
				// Use fmt.Fprintf here as logger might not be fully initialized
				// Corrected the unterminated string literal here
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to create log directory '%s': %v\n", filepath.Dir(opts.FilePath), err)
			}

			// Open the log file for appending
			// File is opened with 0644 permissions
			file, err := os.OpenFile(opts.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				// Use fmt.Fprintf here as logger might not be fully initialized
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to open log file '%s': %v\n", opts.FilePath, err)
			} else {
				// Add file writer to outputs
				outputs = append(outputs, file)
				// Closing the file is usually handled by the OS when the process exits.
			}
		}

		defaultLogger = &Logger{
			level:    opts.Level,
			outputs:  outputs,
			uiActive: opts.UIActive, // Set uiActive state from options
		}

		// Log initialization parameters for debugging (will only appear in file if level is DEBUG)
		defaultLogger.Debug("Logger initialized. Level: %s, FilePath: %s, UIActive: %t",
			levelNames[opts.Level], opts.FilePath, opts.UIActive)
	})
}

// GetLogger returns the default logger instance
func GetLogger() *Logger {
	if defaultLogger == nil {
		// Initialize with defaults if not already initialized.
		// This might happen if a log function is called before cmd.Execute() initializes it.
		Initialize(DefaultLoggerOptions())
	}
	return defaultLogger
}

// log writes a message at the specified level
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	// Check level first without locking for performance
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Format the message
	now := time.Now().Format(time.RFC3339)
	levelName := levelNames[level]
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s: %s\n", now, levelName, message)

	// Write to all configured outputs
	for _, output := range l.outputs {
		// Skip writing to stdout if UI is active
		if output == os.Stdout && l.uiActive {
			continue
		}
		// Ignore potential errors during logging to avoid complex error handling here
		_, _ = fmt.Fprint(output, logLine)
	}
}

// Debug logs a message at the DEBUG level
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs a message at the INFO level
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warning logs a message at the WARNING level
func (l *Logger) Warning(format string, args ...interface{}) {
	l.log(WARNING, format, args...)
}

// Error logs a message at the ERROR level
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Package-level functions for convenience using the default logger

// Debug logs a message at the DEBUG level using the default logger
func Debug(format string, args ...interface{}) {
	GetLogger().Debug(format, args...)
}

// Info logs a message at the INFO level using the default logger
func Info(format string, args ...interface{}) {
	GetLogger().Info(format, args...)
}

// Warning logs a message at the WARNING level using the default logger
func Warning(format string, args ...interface{}) {
	GetLogger().Warning(format, args...)
}

// Error logs a message at the ERROR level using the default logger
func Error(format string, args ...interface{}) {
	GetLogger().Error(format, args...)
}
