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
		defer func() { _ = logger.Close() }()

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
		defer func() { _ = logger.Close() }()

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
		defer func() { _ = logger.Close() }()

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

	_ = logger.Close()

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

	_ = logger.Close()

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

	_ = logger.Close()

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

	_ = logger.Close()

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

	_ = logger.Close()

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

func TestNewLoggerInvalidPath(t *testing.T) {
	t.Run("fails with invalid directory path", func(t *testing.T) {
		// Use a path that cannot be created (null byte is invalid in paths)
		invalidPath := "/nonexistent\x00directory/logs"

		_, err := NewLogger(invalidPath, LevelInfo)
		if err == nil {
			t.Error("expected error for invalid path containing null byte")
		}
	})

	t.Run("fails when directory creation is not possible", func(t *testing.T) {
		// Create a file where we want to create a directory
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "blocking_file")
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create blocking file: %v", err)
		}

		// Try to create a logger with a path that would require creating a directory
		// where a file already exists
		invalidPath := filepath.Join(filePath, "subdir")
		_, err := NewLogger(invalidPath, LevelInfo)
		if err == nil {
			t.Error("expected error when directory creation fails")
		}
	})
}

func TestFilePermissions(t *testing.T) {
	dir := t.TempDir()

	logger, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer func() { _ = logger.Close() }()

	logPath := filepath.Join(dir, "debug.log")
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}

	// Verify file permissions (0644 expected)
	// On Unix, check that file is readable/writable by owner
	mode := info.Mode()
	if mode.Perm()&0600 != 0600 {
		t.Errorf("log file should be readable/writable by owner, got %o", mode.Perm())
	}
}

func TestLogLevelFilteringINFO(t *testing.T) {
	dir := t.TempDir()

	// Create logger at INFO level - should filter out DEBUG only
	logger, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	_ = logger.Close()

	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Should have INFO, WARN, and ERROR (3 lines)
	if len(lines) != 3 {
		t.Fatalf("expected 3 log lines (INFO, WARN, ERROR), got %d: %s", len(lines), string(content))
	}

	// Verify the levels
	expectedLevels := []string{"INFO", "WARN", "ERROR"}
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
			continue
		}
		if entry["level"] != expectedLevels[i] {
			t.Errorf("line %d: expected level %s, got %v", i, expectedLevels[i], entry["level"])
		}
	}
}

func TestLogLevelFilteringERROR(t *testing.T) {
	dir := t.TempDir()

	// Create logger at ERROR level - should only log ERROR
	logger, err := NewLogger(dir, LevelError)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	_ = logger.Close()

	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Should have only ERROR (1 line)
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line (ERROR only), got %d: %s", len(lines), string(content))
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v", err)
	}
	if entry["level"] != "ERROR" {
		t.Errorf("expected level ERROR, got %v", entry["level"])
	}
}

func TestJSONFormatValidation(t *testing.T) {
	dir := t.TempDir()

	logger, err := NewLogger(dir, LevelDebug)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// Log with various data types to test JSON serialization
	logger.Info("test message",
		"string_key", "string_value",
		"int_key", 42,
		"float_key", 3.14,
		"bool_key", true,
		"nil_key", nil,
	)

	_ = logger.Close()

	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("failed to parse JSON log entry: %v", err)
	}

	// Verify required fields are present
	if _, ok := entry["time"]; !ok {
		t.Error("JSON log entry missing 'time' field")
	}
	if _, ok := entry["level"]; !ok {
		t.Error("JSON log entry missing 'level' field")
	}
	if _, ok := entry["msg"]; !ok {
		t.Error("JSON log entry missing 'msg' field")
	}

	// Verify custom fields
	if entry["string_key"] != "string_value" {
		t.Errorf("string_key = %v, want 'string_value'", entry["string_key"])
	}
	if entry["int_key"] != float64(42) { // JSON numbers are float64
		t.Errorf("int_key = %v, want 42", entry["int_key"])
	}
	if entry["float_key"] != 3.14 {
		t.Errorf("float_key = %v, want 3.14", entry["float_key"])
	}
	if entry["bool_key"] != true {
		t.Errorf("bool_key = %v, want true", entry["bool_key"])
	}
	if entry["nil_key"] != nil {
		t.Errorf("nil_key = %v, want nil", entry["nil_key"])
	}
}

func TestWithEmptyArgs(t *testing.T) {
	dir := t.TempDir()

	logger, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// With() with empty args should return the same logger (or equivalent)
	sameLogger := logger.With()

	sameLogger.Info("test message")
	_ = logger.Close()

	// Verify log was written
	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("log file is empty")
	}
}

func TestWithNonStringKey(t *testing.T) {
	dir := t.TempDir()

	logger, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// With() with non-string key should skip that key-value pair
	childLogger := logger.With(42, "value", "valid_key", "valid_value")

	childLogger.Info("test message")
	_ = logger.Close()

	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	// The valid key should be present
	if entry["valid_key"] != "valid_value" {
		t.Errorf("expected valid_key=valid_value, got %v", entry["valid_key"])
	}
}

func TestChildLoggerInheritance(t *testing.T) {
	dir := t.TempDir()

	logger, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// Create chain of child loggers
	child1 := logger.WithSession("session-1")
	child2 := child1.WithInstance("instance-1")
	child3 := child2.WithPhase("planning")

	// Log from the deepest child
	child3.Info("test message", "extra", "data")

	// Also log from parent - should NOT have child attrs
	logger.Info("parent message")

	_ = logger.Close()

	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(lines))
	}

	// First line should have all inherited attrs
	var child3Entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &child3Entry); err != nil {
		t.Fatalf("failed to parse child3 log entry: %v", err)
	}

	if child3Entry["session_id"] != "session-1" {
		t.Errorf("child3 missing session_id")
	}
	if child3Entry["instance_id"] != "instance-1" {
		t.Errorf("child3 missing instance_id")
	}
	if child3Entry["phase"] != "planning" {
		t.Errorf("child3 missing phase")
	}

	// Second line (parent) should NOT have inherited attrs
	var parentEntry map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &parentEntry); err != nil {
		t.Fatalf("failed to parse parent log entry: %v", err)
	}

	if _, ok := parentEntry["session_id"]; ok {
		t.Error("parent should not have session_id")
	}
	if _, ok := parentEntry["instance_id"]; ok {
		t.Error("parent should not have instance_id")
	}
	if _, ok := parentEntry["phase"]; ok {
		t.Error("parent should not have phase")
	}
}

func TestConcurrentChildLoggers(t *testing.T) {
	dir := t.TempDir()

	logger, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// Create multiple child loggers and write concurrently
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func(n int) {
			childLogger := logger.WithInstance(string(rune('A' + n)))
			for j := 0; j < 20; j++ {
				childLogger.Info("message", "iteration", j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	_ = logger.Close()

	// Verify log file has all entries
	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 100 {
		t.Errorf("expected 100 log lines, got %d", len(lines))
	}

	// Verify all lines are valid JSON with instance_id
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
			continue
		}
		if _, ok := entry["instance_id"]; !ok {
			t.Errorf("line %d missing instance_id", i)
		}
	}
}

func TestDirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	nestedDir := filepath.Join(tempDir, "nested", "deep", "directory")

	// NewLogger should create nested directories
	logger, err := NewLogger(nestedDir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Verify directory was created
	info, err := os.Stat(nestedDir)
	if err != nil {
		t.Fatalf("nested directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}

	// Verify log file exists
	logPath := filepath.Join(nestedDir, "debug.log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("log file was not created at %s", logPath)
	}
}

func TestAppendToExistingLog(t *testing.T) {
	dir := t.TempDir()

	// Create first logger and write
	logger1, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	logger1.Info("first message")
	_ = logger1.Close()

	// Create second logger and write
	logger2, err := NewLogger(dir, LevelInfo)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	logger2.Info("second message")
	_ = logger2.Close()

	// Verify both messages are in the log
	logPath := filepath.Join(dir, "debug.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(lines))
	}

	var entry1, entry2 map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry1); err != nil {
		t.Fatalf("failed to parse first log entry: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &entry2); err != nil {
		t.Fatalf("failed to parse second log entry: %v", err)
	}

	if entry1["msg"] != "first message" {
		t.Errorf("first message = %v, want 'first message'", entry1["msg"])
	}
	if entry2["msg"] != "second message" {
		t.Errorf("second message = %v, want 'second message'", entry2["msg"])
	}
}
