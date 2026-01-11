// Package logging provides structured logging for Claudio sessions.
// This file contains utilities for aggregating and exporting logs
// for post-hoc debugging and analysis.
package logging

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LogEntry represents a parsed log entry with all structured fields.
type LogEntry struct {
	Timestamp  time.Time      `json:"time"`
	Level      string         `json:"level"`
	Message    string         `json:"msg"`
	SessionID  string         `json:"session_id,omitempty"`
	InstanceID string         `json:"instance_id,omitempty"`
	Phase      string         `json:"phase,omitempty"`
	Attrs      map[string]any `json:"attrs,omitempty"`
}

// LogFilter defines criteria for filtering log entries.
type LogFilter struct {
	// Level filters to entries at or above this level (DEBUG < INFO < WARN < ERROR)
	// Empty string means no level filtering.
	Level string

	// StartTime filters to entries at or after this time.
	// Zero value means no start time filtering.
	StartTime time.Time

	// EndTime filters to entries at or before this time.
	// Zero value means no end time filtering.
	EndTime time.Time

	// InstanceID filters to entries from this specific instance.
	// Empty string means no instance filtering.
	InstanceID string

	// Phase filters to entries from this specific phase.
	// Empty string means no phase filtering.
	Phase string

	// SessionID filters to entries from this specific session.
	// Empty string means no session filtering.
	SessionID string

	// MessageContains filters to entries whose message contains this substring.
	// Empty string means no message filtering.
	MessageContains string
}

// levelOrder defines the ordering of log levels for filtering.
var levelOrder = map[string]int{
	LevelDebug: 0,
	LevelInfo:  1,
	LevelWarn:  2,
	LevelError: 3,
}

// AggregateLogs reads and parses all log entries from a session directory.
// It looks for the debug.log file in the specified directory and parses
// each line as a JSON log entry.
// Entries are returned sorted by timestamp in ascending order.
func AggregateLogs(sessionDir string) ([]LogEntry, error) {
	logPath := filepath.Join(sessionDir, "debug.log")

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no log file found in session directory: %w", err)
		}
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var entries []LogEntry
	scanner := bufio.NewScanner(file)

	// Increase buffer size for potentially long log lines
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		entry, err := parseLogEntry(line)
		if err != nil {
			// Log parse errors but continue processing
			// This allows partial recovery from corrupted logs
			continue
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	// Sort entries by timestamp
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	return entries, nil
}

// parseLogEntry parses a single JSON log line into a LogEntry.
func parseLogEntry(line string) (LogEntry, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return LogEntry{}, fmt.Errorf("invalid JSON: %w", err)
	}

	entry := LogEntry{
		Attrs: make(map[string]any),
	}

	// Extract standard fields
	if timeStr, ok := raw["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, timeStr); err == nil {
			entry.Timestamp = t
		}
	}

	if level, ok := raw["level"].(string); ok {
		entry.Level = level
	}

	if msg, ok := raw["msg"].(string); ok {
		entry.Message = msg
	}

	if sessionID, ok := raw["session_id"].(string); ok {
		entry.SessionID = sessionID
	}

	if instanceID, ok := raw["instance_id"].(string); ok {
		entry.InstanceID = instanceID
	}

	if phase, ok := raw["phase"].(string); ok {
		entry.Phase = phase
	}

	// Collect remaining fields as attrs
	standardFields := map[string]bool{
		"time":        true,
		"level":       true,
		"msg":         true,
		"session_id":  true,
		"instance_id": true,
		"phase":       true,
	}

	for k, v := range raw {
		if !standardFields[k] {
			entry.Attrs[k] = v
		}
	}

	return entry, nil
}

// FilterLogs filters log entries based on the provided filter criteria.
// Multiple filter criteria are combined with AND logic.
func FilterLogs(entries []LogEntry, filter LogFilter) []LogEntry {
	if isEmptyFilter(filter) {
		return entries
	}

	var filtered []LogEntry
	for _, entry := range entries {
		if matchesFilter(entry, filter) {
			filtered = append(filtered, entry)
		}
	}

	return filtered
}

// isEmptyFilter checks if no filter criteria are set.
func isEmptyFilter(f LogFilter) bool {
	return f.Level == "" &&
		f.StartTime.IsZero() &&
		f.EndTime.IsZero() &&
		f.InstanceID == "" &&
		f.Phase == "" &&
		f.SessionID == "" &&
		f.MessageContains == ""
}

// matchesFilter checks if an entry matches all filter criteria.
func matchesFilter(entry LogEntry, filter LogFilter) bool {
	// Level filter: entry level must be >= filter level
	if filter.Level != "" {
		filterLevelOrder, filterOk := levelOrder[strings.ToUpper(filter.Level)]
		entryLevelOrder, entryOk := levelOrder[entry.Level]
		if filterOk && entryOk && entryLevelOrder < filterLevelOrder {
			return false
		}
	}

	// Time range filters
	if !filter.StartTime.IsZero() && entry.Timestamp.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && entry.Timestamp.After(filter.EndTime) {
		return false
	}

	// Instance ID filter
	if filter.InstanceID != "" && entry.InstanceID != filter.InstanceID {
		return false
	}

	// Phase filter
	if filter.Phase != "" && entry.Phase != filter.Phase {
		return false
	}

	// Session ID filter
	if filter.SessionID != "" && entry.SessionID != filter.SessionID {
		return false
	}

	// Message contains filter
	if filter.MessageContains != "" && !strings.Contains(entry.Message, filter.MessageContains) {
		return false
	}

	return true
}

// ExportLogs exports log entries to a file in the specified format.
// Supported formats: "json", "text", "csv".
func ExportLogs(sessionDir, outputPath string, format string) error {
	entries, err := AggregateLogs(sessionDir)
	if err != nil {
		return fmt.Errorf("failed to aggregate logs: %w", err)
	}

	return ExportLogEntries(entries, outputPath, format)
}

// ExportLogEntries exports the given log entries to a file in the specified format.
// This allows exporting filtered logs that have already been aggregated.
// Supported formats: "json", "text", "csv".
func ExportLogEntries(entries []LogEntry, outputPath string, format string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = file.Close() }()

	switch strings.ToLower(format) {
	case "json":
		return exportJSON(file, entries)
	case "text":
		return exportText(file, entries)
	case "csv":
		return exportCSV(file, entries)
	default:
		return fmt.Errorf("unsupported export format: %s (supported: json, text, csv)", format)
	}
}

// exportJSON writes entries as a JSON array.
func exportJSON(file *os.File, entries []LogEntry) error {
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

// exportText writes entries in a human-readable text format.
func exportText(file *os.File, entries []LogEntry) error {
	for _, entry := range entries {
		// Format: [TIMESTAMP] LEVEL - MESSAGE (context) {attrs}
		var parts []string

		// Add timestamp
		ts := entry.Timestamp.Format("2006-01-02 15:04:05.000")
		parts = append(parts, fmt.Sprintf("[%s]", ts))

		// Add level
		parts = append(parts, entry.Level)

		// Add message
		parts = append(parts, "-", entry.Message)

		// Add context fields if present
		var context []string
		if entry.SessionID != "" {
			context = append(context, fmt.Sprintf("session=%s", entry.SessionID))
		}
		if entry.InstanceID != "" {
			context = append(context, fmt.Sprintf("instance=%s", entry.InstanceID))
		}
		if entry.Phase != "" {
			context = append(context, fmt.Sprintf("phase=%s", entry.Phase))
		}
		if len(context) > 0 {
			parts = append(parts, fmt.Sprintf("(%s)", strings.Join(context, ", ")))
		}

		// Add extra attrs if present
		if len(entry.Attrs) > 0 {
			attrsJSON, _ := json.Marshal(entry.Attrs)
			parts = append(parts, string(attrsJSON))
		}

		line := strings.Join(parts, " ") + "\n"
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write text entry: %w", err)
		}
	}

	return nil
}

// exportCSV writes entries as CSV with headers.
func exportCSV(file *os.File, entries []LogEntry) error {
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	headers := []string{"timestamp", "level", "message", "session_id", "instance_id", "phase", "attrs"}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write entries
	for _, entry := range entries {
		attrsJSON := ""
		if len(entry.Attrs) > 0 {
			if b, err := json.Marshal(entry.Attrs); err == nil {
				attrsJSON = string(b)
			}
		}

		record := []string{
			entry.Timestamp.Format(time.RFC3339Nano),
			entry.Level,
			entry.Message,
			entry.SessionID,
			entry.InstanceID,
			entry.Phase,
			attrsJSON,
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}
