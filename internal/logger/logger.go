package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

type Logger struct {
	debugMode bool
	logFile   *os.File
	logger    *log.Logger
	mu        sync.Mutex
}

var (
	instance *Logger
	once     sync.Once
)

// NewLogger creates a new logger instance
func NewLogger(debug bool, logFilePath string) (*Logger, error) {
	var logFile *os.File
	var err error

	if debug && logFilePath != "" {
		// Ensure the directory exists
		if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
	}

	// Only create a logger if we have a log file
	var logger *log.Logger
	if logFile != nil {
		logger = log.New(logFile, "", log.LstdFlags|log.Lmsgprefix)
	} else {
		// If no log file, use a no-op logger
		logger = log.New(io.Discard, "", 0)
	}

	return &Logger{
		debugMode: debug,
		logFile:   logFile,
		logger:    logger,
	}, nil
}

// GetLogger returns a singleton instance of the logger
func GetLogger() *Logger {
	once.Do(func() {
		// Default logger if not initialized
		instance = &Logger{
			logger: log.New(os.Stdout, "", log.LstdFlags|log.Lmsgprefix),
		}
	})
	return instance
}

// SetDebug sets the debug mode
func (l *Logger) SetDebug(debug bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugMode = debug
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	if !l.debugMode {
		return
	}
	l.log(LevelDebug, format, v...)
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	l.log(LevelInfo, format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.log(LevelWarn, format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.log(LevelError, format, v...)
}

// Fatal logs a fatal error message and exits
func (l *Logger) Fatal(format string, v ...interface{}) {
	l.log(LevelError, format, v...)
	os.Exit(1)
}

// log is the internal logging function
func (l *Logger) log(level LogLevel, format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	prefix := ""
	switch level {
	case LevelDebug:
		prefix = "[DEBUG] "
	case LevelInfo:
		prefix = "[INFO]  "
	case LevelWarn:
		prefix = "[WARN]  "
	case LevelError:
		prefix = "[ERROR] "
	}

	l.logger.SetPrefix(prefix)
	l.logger.Printf(format, v...)
}

// Close closes the log file if it's open
func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}
