package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	t.Run("creates log file in session directory", func(t *testing.T) {
		dir := t.TempDir()

		logger, err := NewLogger(dir, LevelDebug)
		if err != nil {
			t.Fatalf("NewLogger failed: %v", err)
		}
		defer logger.Close()

		logPath := filepath.Join(dir, "debug.log")
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Errorf("log file was not created at %s", logPath)
		}
	})

	t.Run("writes to stderr when sessionDir is empty", func(t *testing.T) {
		logger, err := NewLogger("", LevelInfo)
		if err != nil {
			t.Fatalf("NewLogger failed: %v", err)
		}
		defer logger.Close()

		if logger.file != nil {
			t.Error("expected file to be nil when sessionDir is empty")
		}
	})

	t.Run("defaults to INFO level for invalid level string", func(t *testing.T) {
		dir := t.TempDir()

		logger, err := NewLogger(dir, "invalid")
		if err != nil {
			t.Fatalf("NewLogger failed: %v", err)
		}
		defer logger.Close()

		// Logger should have been created successfully
		if logger.logger == nil {
			t.Error("expected logger to be created")
		}
	})
}

func TestLogLevels(t *testing.T) {
	dir := t.TempDir()

	logger, err := NewLogger(dir, LevelDebug)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// Log at all levels
	logger.Debug("debug message", "key", "value")
	logger.Info("info message", "key", "value")
	logger.Warn("warn message", "key", "value")
	logger.Error("error message", "key", "value")

	logger.Close()

	// Read and verify log file
	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 log lines, got %d", len(lines))
	}

	// Verify each log line is valid JSON with expected fields
	expectedLevels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	expectedMsgs := []string{"debug message", "info message", "warn message", "error message"}

	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
			continue
		}

		if entry["level"] != expectedLevels[i] {
			t.Errorf("line %d: expected level %s, got %v", i, expectedLevels[i], entry["level"])
		}
		if entry["msg"] != expectedMsgs[i] {
			t.Errorf("line %d: expected msg %s, got %v", i, expectedMsgs[i], entry["msg"])
		}
		if entry["key"] != "value" {
			t.Errorf("line %d: expected key=value, got key=%v", i, entry["key"])
		}
	}
}

func TestLogLevelFiltering(t *testing.T) {
	dir := t.TempDir()

	// Create logger at WARN level - should filter out DEBUG and INFO
	logger, err := NewLogger(dir, LevelWarn)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	logger.Close()

	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Should only have WARN and ERROR
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines (WARN and ERROR only), got %d: %s", len(lines), string(content))
	}
}

func TestContextPropagation(t *testing.T) {
	dir := t.TempDir()

	logger, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// Create child logger with context
	childLogger := logger.WithSession("session-123").WithInstance("instance-456").WithPhase("execution")

	childLogger.Info("test message", "extra", "data")

	logger.Close()

	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	// Verify all context fields are present
	if entry["session_id"] != "session-123" {
		t.Errorf("expected session_id=session-123, got %v", entry["session_id"])
	}
	if entry["instance_id"] != "instance-456" {
		t.Errorf("expected instance_id=instance-456, got %v", entry["instance_id"])
	}
	if entry["phase"] != "execution" {
		t.Errorf("expected phase=execution, got %v", entry["phase"])
	}
	if entry["extra"] != "data" {
		t.Errorf("expected extra=data, got %v", entry["extra"])
	}
}

func TestWith(t *testing.T) {
	dir := t.TempDir()

	logger, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	childLogger := logger.With("foo", "bar", "count", 42)
	childLogger.Info("test message")

	logger.Close()

	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	if entry["foo"] != "bar" {
		t.Errorf("expected foo=bar, got %v", entry["foo"])
	}
	// JSON numbers are float64
	if entry["count"] != float64(42) {
		t.Errorf("expected count=42, got %v", entry["count"])
	}
}

func TestNopLogger(t *testing.T) {
	logger := NopLogger()

	// These should not panic
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	// Close should also not fail
	if err := logger.Close(); err != nil {
		t.Errorf("NopLogger.Close() returned error: %v", err)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"DEBUG", LevelDebug},
		{"debug", LevelDebug},
		{"INFO", LevelInfo},
		{"info", LevelInfo},
		{"WARN", LevelWarn},
		{"warn", LevelWarn},
		{"ERROR", LevelError},
		{"error", LevelError},
		{"invalid", LevelInfo},
		{"", LevelInfo},
	}

	for _, tc := range tests {
		result := ParseLevel(tc.input)
		if result != tc.expected {
			t.Errorf("ParseLevel(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestValidLevels(t *testing.T) {
	levels := ValidLevels()

	expected := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	if len(levels) != len(expected) {
		t.Fatalf("expected %d levels, got %d", len(expected), len(levels))
	}

	for i, level := range levels {
		if level != expected[i] {
			t.Errorf("ValidLevels()[%d] = %q, expected %q", i, level, expected[i])
		}
	}
}

func TestClose(t *testing.T) {
	dir := t.TempDir()

	logger, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	logger.Info("test message")

	// Close should flush and close the file
	if err := logger.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Second close should be a no-op (file is nil)
	if err := logger.Close(); err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}

	// Verify log was written
	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("log file is empty, expected content")
	}
}

func TestConcurrentWrites(t *testing.T) {
	dir := t.TempDir()

	logger, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// Write from multiple goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				logger.Info("concurrent write", "goroutine", n, "iteration", j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	logger.Close()

	// Verify log file has content
	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 1000 {
		t.Errorf("expected 1000 log lines, got %d", len(lines))
	}

	// Verify all lines are valid JSON
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}
