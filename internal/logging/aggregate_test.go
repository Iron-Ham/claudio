package logging

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAggregateLogs(t *testing.T) {
	t.Run("parses log entries from session directory", func(t *testing.T) {
		dir := t.TempDir()

		// Create a logger and write some entries
		logger, err := NewLogger(dir, LevelDebug)
		if err != nil {
			t.Fatalf("NewLogger failed: %v", err)
		}

		logger.WithSession("session-1").WithInstance("inst-1").WithPhase("planning").Info("message 1", "extra", "data")
		logger.WithSession("session-1").WithInstance("inst-2").WithPhase("execution").Debug("message 2")
		logger.WithSession("session-1").Error("message 3", "code", 500)

		_ = logger.Close()

		// Aggregate the logs
		entries, err := AggregateLogs(dir)
		if err != nil {
			t.Fatalf("AggregateLogs failed: %v", err)
		}

		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}

		// Verify first entry
		if entries[0].Message != "message 1" {
			t.Errorf("expected message 'message 1', got %q", entries[0].Message)
		}
		if entries[0].Level != "INFO" {
			t.Errorf("expected level INFO, got %q", entries[0].Level)
		}
		if entries[0].SessionID != "session-1" {
			t.Errorf("expected session_id 'session-1', got %q", entries[0].SessionID)
		}
		if entries[0].InstanceID != "inst-1" {
			t.Errorf("expected instance_id 'inst-1', got %q", entries[0].InstanceID)
		}
		if entries[0].Phase != "planning" {
			t.Errorf("expected phase 'planning', got %q", entries[0].Phase)
		}
		if entries[0].Attrs["extra"] != "data" {
			t.Errorf("expected extra=data, got %v", entries[0].Attrs["extra"])
		}
	})

	t.Run("returns error for missing log file", func(t *testing.T) {
		dir := t.TempDir()

		_, err := AggregateLogs(dir)
		if err == nil {
			t.Error("expected error for missing log file")
		}
		if !strings.Contains(err.Error(), "no log file found") {
			t.Errorf("expected 'no log file found' error, got: %v", err)
		}
	})

	t.Run("handles empty log file", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "debug.log")

		// Create empty log file
		if err := os.WriteFile(logPath, []byte(""), 0644); err != nil {
			t.Fatalf("failed to create empty log file: %v", err)
		}

		entries, err := AggregateLogs(dir)
		if err != nil {
			t.Fatalf("AggregateLogs failed: %v", err)
		}

		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("skips malformed JSON lines", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "debug.log")

		content := `{"time":"2024-01-01T12:00:00Z","level":"INFO","msg":"valid"}
invalid json line
{"time":"2024-01-01T12:00:01Z","level":"ERROR","msg":"also valid"}
`
		if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create log file: %v", err)
		}

		entries, err := AggregateLogs(dir)
		if err != nil {
			t.Fatalf("AggregateLogs failed: %v", err)
		}

		if len(entries) != 2 {
			t.Fatalf("expected 2 valid entries, got %d", len(entries))
		}
	})

	t.Run("sorts entries by timestamp", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "debug.log")

		// Write entries out of order
		content := `{"time":"2024-01-01T12:00:02Z","level":"INFO","msg":"third"}
{"time":"2024-01-01T12:00:00Z","level":"INFO","msg":"first"}
{"time":"2024-01-01T12:00:01Z","level":"INFO","msg":"second"}
`
		if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create log file: %v", err)
		}

		entries, err := AggregateLogs(dir)
		if err != nil {
			t.Fatalf("AggregateLogs failed: %v", err)
		}

		if entries[0].Message != "first" || entries[1].Message != "second" || entries[2].Message != "third" {
			t.Errorf("entries not sorted by timestamp: %v, %v, %v",
				entries[0].Message, entries[1].Message, entries[2].Message)
		}
	})
}

func TestFilterLogs(t *testing.T) {
	now := time.Now()
	entries := []LogEntry{
		{Timestamp: now, Level: "DEBUG", Message: "debug msg", InstanceID: "inst-1", Phase: "planning", SessionID: "sess-1"},
		{Timestamp: now.Add(time.Second), Level: "INFO", Message: "info msg", InstanceID: "inst-1", Phase: "execution", SessionID: "sess-1"},
		{Timestamp: now.Add(2 * time.Second), Level: "WARN", Message: "warn msg", InstanceID: "inst-2", Phase: "execution", SessionID: "sess-1"},
		{Timestamp: now.Add(3 * time.Second), Level: "ERROR", Message: "error msg", InstanceID: "inst-2", Phase: "consolidation", SessionID: "sess-2"},
	}

	t.Run("returns all entries with empty filter", func(t *testing.T) {
		filtered := FilterLogs(entries, LogFilter{})
		if len(filtered) != 4 {
			t.Errorf("expected 4 entries, got %d", len(filtered))
		}
	})

	t.Run("filters by level", func(t *testing.T) {
		filtered := FilterLogs(entries, LogFilter{Level: "WARN"})
		if len(filtered) != 2 {
			t.Errorf("expected 2 entries (WARN and ERROR), got %d", len(filtered))
		}
		for _, e := range filtered {
			if e.Level != "WARN" && e.Level != "ERROR" {
				t.Errorf("unexpected level: %s", e.Level)
			}
		}
	})

	t.Run("filters by level case insensitive", func(t *testing.T) {
		filtered := FilterLogs(entries, LogFilter{Level: "warn"})
		if len(filtered) != 2 {
			t.Errorf("expected 2 entries, got %d", len(filtered))
		}
	})

	t.Run("filters by time range", func(t *testing.T) {
		filtered := FilterLogs(entries, LogFilter{
			StartTime: now.Add(500 * time.Millisecond),
			EndTime:   now.Add(2500 * time.Millisecond),
		})
		if len(filtered) != 2 {
			t.Errorf("expected 2 entries, got %d", len(filtered))
		}
	})

	t.Run("filters by instance ID", func(t *testing.T) {
		filtered := FilterLogs(entries, LogFilter{InstanceID: "inst-2"})
		if len(filtered) != 2 {
			t.Errorf("expected 2 entries, got %d", len(filtered))
		}
		for _, e := range filtered {
			if e.InstanceID != "inst-2" {
				t.Errorf("unexpected instance ID: %s", e.InstanceID)
			}
		}
	})

	t.Run("filters by phase", func(t *testing.T) {
		filtered := FilterLogs(entries, LogFilter{Phase: "execution"})
		if len(filtered) != 2 {
			t.Errorf("expected 2 entries, got %d", len(filtered))
		}
	})

	t.Run("filters by session ID", func(t *testing.T) {
		filtered := FilterLogs(entries, LogFilter{SessionID: "sess-2"})
		if len(filtered) != 1 {
			t.Errorf("expected 1 entry, got %d", len(filtered))
		}
	})

	t.Run("filters by message contains", func(t *testing.T) {
		filtered := FilterLogs(entries, LogFilter{MessageContains: "msg"})
		if len(filtered) != 4 {
			t.Errorf("expected 4 entries, got %d", len(filtered))
		}

		filtered = FilterLogs(entries, LogFilter{MessageContains: "warn"})
		if len(filtered) != 1 {
			t.Errorf("expected 1 entry, got %d", len(filtered))
		}
	})

	t.Run("combines multiple filters with AND logic", func(t *testing.T) {
		filtered := FilterLogs(entries, LogFilter{
			Level:      "INFO",
			InstanceID: "inst-2",
		})
		// Only WARN and ERROR level entries from inst-2
		if len(filtered) != 2 {
			t.Errorf("expected 2 entries, got %d", len(filtered))
		}
	})
}

func TestExportLogs(t *testing.T) {
	// Create a session with logs
	sessionDir := t.TempDir()

	logger, err := NewLogger(sessionDir, LevelDebug)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	logger.WithSession("sess-1").WithInstance("inst-1").WithPhase("planning").Info("test message", "key", "value")
	logger.WithSession("sess-1").Error("error message", "code", 500)
	_ = logger.Close()

	t.Run("exports to JSON format", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "output.json")

		err := ExportLogs(sessionDir, outputPath, "json")
		if err != nil {
			t.Fatalf("ExportLogs failed: %v", err)
		}

		content, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		var entries []LogEntry
		if err := json.Unmarshal(content, &entries); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}

		if len(entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(entries))
		}
	})

	t.Run("exports to text format", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "output.txt")

		err := ExportLogs(sessionDir, outputPath, "text")
		if err != nil {
			t.Fatalf("ExportLogs failed: %v", err)
		}

		content, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(lines) != 2 {
			t.Errorf("expected 2 lines, got %d", len(lines))
		}

		// Verify text format contains expected parts
		if !strings.Contains(lines[0], "INFO") {
			t.Error("expected first line to contain INFO")
		}
		if !strings.Contains(lines[0], "test message") {
			t.Error("expected first line to contain message")
		}
		if !strings.Contains(lines[0], "session=sess-1") {
			t.Error("expected first line to contain session context")
		}
	})

	t.Run("exports to CSV format", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "output.csv")

		err := ExportLogs(sessionDir, outputPath, "csv")
		if err != nil {
			t.Fatalf("ExportLogs failed: %v", err)
		}

		file, err := os.Open(outputPath)
		if err != nil {
			t.Fatalf("failed to open output file: %v", err)
		}
		defer func() { _ = file.Close() }()

		reader := csv.NewReader(file)
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("failed to parse CSV output: %v", err)
		}

		// Should have header + 2 data rows
		if len(records) != 3 {
			t.Errorf("expected 3 rows (header + 2 data), got %d", len(records))
		}

		// Verify header
		expectedHeaders := []string{"timestamp", "level", "message", "session_id", "instance_id", "phase", "attrs"}
		for i, h := range expectedHeaders {
			if records[0][i] != h {
				t.Errorf("expected header[%d] = %q, got %q", i, h, records[0][i])
			}
		}
	})

	t.Run("returns error for unsupported format", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "output.xml")

		err := ExportLogs(sessionDir, outputPath, "xml")
		if err == nil {
			t.Error("expected error for unsupported format")
		}
		if !strings.Contains(err.Error(), "unsupported export format") {
			t.Errorf("expected 'unsupported export format' error, got: %v", err)
		}
	})

	t.Run("format is case insensitive", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "output.json")

		err := ExportLogs(sessionDir, outputPath, "JSON")
		if err != nil {
			t.Errorf("ExportLogs failed with uppercase format: %v", err)
		}
	})
}

func TestExportLogEntries(t *testing.T) {
	entries := []LogEntry{
		{
			Timestamp:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Level:      "INFO",
			Message:    "test message",
			SessionID:  "sess-1",
			InstanceID: "inst-1",
			Phase:      "planning",
			Attrs:      map[string]any{"key": "value"},
		},
	}

	t.Run("exports filtered entries", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "filtered.json")

		err := ExportLogEntries(entries, outputPath, "json")
		if err != nil {
			t.Fatalf("ExportLogEntries failed: %v", err)
		}

		content, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		var exported []LogEntry
		if err := json.Unmarshal(content, &exported); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}

		if len(exported) != 1 {
			t.Errorf("expected 1 entry, got %d", len(exported))
		}

		if exported[0].Message != "test message" {
			t.Errorf("expected message 'test message', got %q", exported[0].Message)
		}
	})
}

func TestParseLogEntry(t *testing.T) {
	t.Run("parses all standard fields", func(t *testing.T) {
		line := `{"time":"2024-01-01T12:00:00.123456789Z","level":"INFO","msg":"test","session_id":"sess","instance_id":"inst","phase":"exec"}`

		entry, err := parseLogEntry(line)
		if err != nil {
			t.Fatalf("parseLogEntry failed: %v", err)
		}

		if entry.Level != "INFO" {
			t.Errorf("expected level INFO, got %q", entry.Level)
		}
		if entry.Message != "test" {
			t.Errorf("expected message 'test', got %q", entry.Message)
		}
		if entry.SessionID != "sess" {
			t.Errorf("expected session_id 'sess', got %q", entry.SessionID)
		}
		if entry.InstanceID != "inst" {
			t.Errorf("expected instance_id 'inst', got %q", entry.InstanceID)
		}
		if entry.Phase != "exec" {
			t.Errorf("expected phase 'exec', got %q", entry.Phase)
		}
	})

	t.Run("collects extra fields as attrs", func(t *testing.T) {
		line := `{"time":"2024-01-01T12:00:00Z","level":"INFO","msg":"test","custom":"value","count":42}`

		entry, err := parseLogEntry(line)
		if err != nil {
			t.Fatalf("parseLogEntry failed: %v", err)
		}

		if entry.Attrs["custom"] != "value" {
			t.Errorf("expected attrs.custom = 'value', got %v", entry.Attrs["custom"])
		}
		if entry.Attrs["count"] != float64(42) {
			t.Errorf("expected attrs.count = 42, got %v", entry.Attrs["count"])
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		_, err := parseLogEntry("not json")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}
