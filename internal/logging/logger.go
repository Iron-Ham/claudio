// Package logging provides structured logging for Claudio sessions.
// It wraps Go's log/slog package to provide JSON-formatted logs with
// context propagation support for debugging and post-hoc analysis.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Log levels supported by the logger
const (
	LevelDebug = "DEBUG"
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERROR"
)

// Logger provides structured logging with context propagation.
// It is safe for concurrent use.
type Logger struct {
	logger *slog.Logger
	file   *os.File
	mu     sync.Mutex // Protects file operations
	attrs  []slog.Attr // Persistent attributes (session, instance, phase)
}

// NewLogger creates a new Logger that writes JSON-formatted logs to a file
// in the specified session directory. The log file will be created at
// {sessionDir}/debug.log.
//
// The level parameter controls which messages are logged:
//   - DEBUG: All messages
//   - INFO: Info, Warn, and Error messages
//   - WARN: Warn and Error messages
//   - ERROR: Only Error messages
//
// If sessionDir is empty, logs will be written to stderr.
func NewLogger(sessionDir string, level string) (*Logger, error) {
	var writer io.Writer
	var file *os.File

	if sessionDir != "" {
		// Ensure the session directory exists
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create session directory: %w", err)
		}

		logPath := filepath.Join(sessionDir, "debug.log")
		var err error
		file, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		writer = file
	} else {
		writer = os.Stderr
	}

	slogLevel := parseLevel(level)

	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	handler := slog.NewJSONHandler(writer, opts)

	return &Logger{
		logger: slog.New(handler),
		file:   file,
		attrs:  make([]slog.Attr, 0),
	}, nil
}

// parseLevel converts a string log level to slog.Level.
// Defaults to INFO if the level string is not recognized.
func parseLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithSession returns a new Logger with the session ID added to all log entries.
// This creates a child logger that inherits all existing attributes.
func (l *Logger) WithSession(sessionID string) *Logger {
	return l.withAttr(slog.String("session_id", sessionID))
}

// WithInstance returns a new Logger with the instance ID added to all log entries.
// This creates a child logger that inherits all existing attributes.
func (l *Logger) WithInstance(instanceID string) *Logger {
	return l.withAttr(slog.String("instance_id", instanceID))
}

// WithPhase returns a new Logger with the phase name added to all log entries.
// This creates a child logger that inherits all existing attributes.
// Phases might include: "planning", "execution", "consolidation", etc.
func (l *Logger) WithPhase(phase string) *Logger {
	return l.withAttr(slog.String("phase", phase))
}

// With returns a new Logger with arbitrary key-value attributes.
// Keys and values are provided as alternating arguments.
// This creates a child logger that inherits all existing attributes.
func (l *Logger) With(args ...any) *Logger {
	if len(args) == 0 {
		return l
	}

	newAttrs := make([]slog.Attr, 0, len(l.attrs)+len(args)/2)
	newAttrs = append(newAttrs, l.attrs...)

	// Convert args to slog.Attr
	for i := 0; i < len(args)-1; i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		newAttrs = append(newAttrs, slog.Any(key, args[i+1]))
	}

	return &Logger{
		logger: l.logger,
		file:   l.file,
		attrs:  newAttrs,
	}
}

// withAttr creates a new Logger with an additional attribute.
func (l *Logger) withAttr(attr slog.Attr) *Logger {
	newAttrs := make([]slog.Attr, len(l.attrs)+1)
	copy(newAttrs, l.attrs)
	newAttrs[len(l.attrs)] = attr

	return &Logger{
		logger: l.logger,
		file:   l.file,
		attrs:  newAttrs,
	}
}

// Debug logs a message at DEBUG level with optional key-value pairs.
// Keys and values are provided as alternating arguments.
func (l *Logger) Debug(msg string, args ...any) {
	l.log(slog.LevelDebug, msg, args...)
}

// Info logs a message at INFO level with optional key-value pairs.
// Keys and values are provided as alternating arguments.
func (l *Logger) Info(msg string, args ...any) {
	l.log(slog.LevelInfo, msg, args...)
}

// Warn logs a message at WARN level with optional key-value pairs.
// Keys and values are provided as alternating arguments.
func (l *Logger) Warn(msg string, args ...any) {
	l.log(slog.LevelWarn, msg, args...)
}

// Error logs a message at ERROR level with optional key-value pairs.
// Keys and values are provided as alternating arguments.
func (l *Logger) Error(msg string, args ...any) {
	l.log(slog.LevelError, msg, args...)
}

// log is the internal logging method that combines persistent attributes
// with per-call arguments.
func (l *Logger) log(level slog.Level, msg string, args ...any) {
	// Combine persistent attrs with per-call args
	allArgs := make([]any, 0, len(l.attrs)*2+len(args))
	for _, attr := range l.attrs {
		allArgs = append(allArgs, attr.Key, attr.Value.Any())
	}
	allArgs = append(allArgs, args...)

	l.logger.Log(context.Background(), level, msg, allArgs...)
}

// Close flushes and closes the log file.
// If the logger was created without a session directory (writing to stderr),
// this method is a no-op.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		if err := l.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync log file: %w", err)
		}
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("failed to close log file: %w", err)
		}
		l.file = nil
	}
	return nil
}

// NopLogger returns a Logger that discards all log output.
// Useful for testing or when logging is disabled.
func NopLogger() *Logger {
	return &Logger{
		logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		attrs:  make([]slog.Attr, 0),
	}
}

// ParseLevel converts a string level to the corresponding constant.
// Returns LevelInfo if the level string is not recognized.
func ParseLevel(level string) string {
	switch strings.ToUpper(level) {
	case LevelDebug:
		return LevelDebug
	case LevelInfo:
		return LevelInfo
	case LevelWarn:
		return LevelWarn
	case LevelError:
		return LevelError
	default:
		return LevelInfo
	}
}

// ValidLevels returns the list of valid log level strings.
func ValidLevels() []string {
	return []string{LevelDebug, LevelInfo, LevelWarn, LevelError}
}
