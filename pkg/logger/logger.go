package logger

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	
	"k8s.io/klog/v2"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
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
	level     LogLevel
	outputs   []io.Writer
	mu        sync.Mutex
	filePath  string // Store the file path for line management
	lineCount int    // Current number of lines in the log file
	maxLines  int    // Maximum lines to keep in the file
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Options defines options for initializing the logger
type Options struct {
	Level    LogLevel
	FilePath string
	Verbose  bool
}

// DefaultOptions returns the default options
func DefaultOptions() *Options {
	return &Options{
		Level:    INFO,
		FilePath: "", // Default path is now set by the caller (rootCmd)
	}
}

// Initialize initializes the default logger with the given options
func Initialize(opts *Options) {
	if opts == nil {
		opts = DefaultOptions()
	}

	once.Do(func() {
		outputs := []io.Writer{}

		// Only add file output if FilePath is specified
		// No stdout output - logs go only to file
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
			level:     opts.Level,
			outputs:   outputs,
			filePath:  opts.FilePath,
			lineCount: 0,
			maxLines:  constants.MaxLogLines,
		}

		// Configure klog to use the same log file and disable stderr output
		configureKubernetesLogging(opts)

		// Count existing lines in the log file
		if opts.FilePath != "" {
			defaultLogger.lineCount = countLinesInFile(opts.FilePath)
		}

		// Log initialization parameters for debugging (will only appear in file if level is DEBUG)
		defaultLogger.Debug("Logger initialized. Level: %s, FilePath: %s, CurrentLines: %d",
			levelNames[opts.Level], opts.FilePath, defaultLogger.lineCount)
	})
}

// configureKubernetesLogging configures klog to use the same file as our logger
func configureKubernetesLogging(opts *Options) {
	// Disable klog output to stderr to avoid duplicates
	klog.LogToStderr(false)
	
	// If we have a log file, configure klog to use it too
	if opts.FilePath != "" {
		// Configure klog to log to the same file
		klog.SetOutput(getLogFileWriter(opts.FilePath))
	}
	
	// Set klog verbosity based on our logger level
	if opts.Level == DEBUG {
		// Enable more verbose klog output in debug mode
		klog.V(2).Info("Kubernetes client logging configured")
	}
}

// getLogFileWriter returns a writer for the log file
func getLogFileWriter(filePath string) io.Writer {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fallback to stderr if file can't be opened
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to open log file for klog: %v\n", err)
		return os.Stderr
	}
	return file
}

// countLinesInFile counts the number of lines in a file
func countLinesInFile(filePath string) int {
	file, err := os.Open(filePath)
	if err != nil {
		return 0 // File doesn't exist or can't be opened
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}
	
	return lineCount
}

// GetLogger returns the default logger instance
func GetLogger() *Logger {
	if defaultLogger == nil {
		// Initialize with defaults if not already initialized.
		// This might happen if a log function is called before cmd.Execute() initializes it.
		Initialize(DefaultOptions())
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

	// Check if file rotation is needed
	l.checkAndRotateFile()

	// Format the message
	now := time.Now().Format(time.RFC3339)
	levelName := levelNames[level]
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s: %s\n", now, levelName, message)

	// Write to all configured outputs
	for _, output := range l.outputs {
		// Ignore potential errors during logging to avoid complex error handling here
		_, _ = fmt.Fprint(output, logLine)
	}
	
	// Increment line count for file rotation
	if l.filePath != "" {
		l.lineCount++
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

// checkAndRotateFile checks if the log file needs truncation and performs it
func (l *Logger) checkAndRotateFile() {
	if l.filePath == "" || l.maxLines <= 0 {
		return // No line limit configured
	}
	
	if l.lineCount >= l.maxLines {
		l.truncateFile()
	}
}

// truncateFile keeps only the most recent lines and removes older ones
func (l *Logger) truncateFile() {
	if l.filePath == "" {
		return
	}
	
	// Read all lines from the file
	lines, err := l.readAllLines()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Failed to read log file for truncation: %v\n", err)
		return
	}
	
	// Calculate how many lines to keep (keep newest 50% when max is reached)
	keepLines := l.maxLines / 2
	if len(lines) > keepLines {
		lines = lines[len(lines)-keepLines:]
	}
	
	// Rewrite the file with only the kept lines
	err = l.rewriteFile(lines)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Failed to truncate log file: %v\n", err)
		return
	}
	
	// Update line count
	l.lineCount = len(lines)
}

// readAllLines reads all lines from the log file
func (l *Logger) readAllLines() ([]string, error) {
	file, err := os.Open(l.filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	
	return lines, scanner.Err()
}

// rewriteFile rewrites the log file with the given lines
func (l *Logger) rewriteFile(lines []string) error {
	// Create a temporary file
	tempFile := l.filePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return err
	}
	
	// Write the lines
	for _, line := range lines {
		if _, err := fmt.Fprintln(file, line); err != nil {
			file.Close()
			os.Remove(tempFile)
			return err
		}
	}
	file.Close()
	
	// Replace the original file with the temporary file
	return os.Rename(tempFile, l.filePath)
}
