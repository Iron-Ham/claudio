package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/session"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View session logs",
	Long: `View and filter logs for Claudio sessions.

By default, shows logs from the most recent session. Use flags to filter
and format the output.

Examples:
  # Show last 50 lines from most recent session
  claudio logs

  # Show all logs from a specific session
  claudio logs -s abc123 -n 0

  # Follow logs in real-time
  claudio logs -f

  # Filter by log level
  claudio logs --level warn

  # Show logs from the last hour
  claudio logs --since 1h

  # Search for specific patterns
  claudio logs --grep "error|failed"`,
	RunE: runLogs,
}

var (
	logsSessionID string
	logsTail      int
	logsFollow    bool
	logsLevel     string
	logsSince     string
	logsGrep      string
)

func init() {
	rootCmd.AddCommand(logsCmd)

	logsCmd.Flags().StringVarP(&logsSessionID, "session", "s", "", "Session ID (default: most recent)")
	logsCmd.Flags().IntVarP(&logsTail, "tail", "n", 50, "Number of lines to show (0 for all)")
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output (like tail -f)")
	logsCmd.Flags().StringVar(&logsLevel, "level", "", "Filter by minimum level (debug/info/warn/error)")
	logsCmd.Flags().StringVar(&logsSince, "since", "", "Show logs since duration ago (e.g., 1h, 30m)")
	logsCmd.Flags().StringVar(&logsGrep, "grep", "", "Filter logs matching pattern (regex)")
}

// logEntry represents a parsed JSON log line
type logEntry struct {
	Time       time.Time              `json:"time"`
	Level      string                 `json:"level"`
	Msg        string                 `json:"msg"`
	SessionID  string                 `json:"session_id,omitempty"`
	InstanceID string                 `json:"instance_id,omitempty"`
	Phase      string                 `json:"phase,omitempty"`
	Extra      map[string]any `json:"-"` // Captures additional fields
}

// UnmarshalJSON implements custom unmarshaling to capture extra fields
func (e *logEntry) UnmarshalJSON(data []byte) error {
	// First, unmarshal known fields using a type alias to avoid recursion
	type Alias logEntry
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Then unmarshal all fields to capture extras
	var all map[string]any
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}

	// Remove known fields, keep the rest as extra
	delete(all, "time")
	delete(all, "level")
	delete(all, "msg")
	delete(all, "session_id")
	delete(all, "instance_id")
	delete(all, "phase")

	if len(all) > 0 {
		e.Extra = all
	}

	return nil
}

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorGray   = "\033[90m"
	colorBlue   = "\033[34m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
)

// levelColor returns the ANSI color code for a log level
func levelColor(level string) string {
	switch strings.ToUpper(level) {
	case logging.LevelDebug:
		return colorGray
	case logging.LevelInfo:
		return colorBlue
	case logging.LevelWarn:
		return colorYellow
	case logging.LevelError:
		return colorRed
	default:
		return colorReset
	}
}

// levelPriority returns the priority of a log level for filtering
func levelPriority(level string) int {
	switch strings.ToUpper(level) {
	case logging.LevelDebug:
		return 0
	case logging.LevelInfo:
		return 1
	case logging.LevelWarn:
		return 2
	case logging.LevelError:
		return 3
	default:
		return -1
	}
}

// formatLogEntry formats a log entry for terminal output
func formatLogEntry(entry *logEntry) string {
	var sb strings.Builder

	// Timestamp
	sb.WriteString(colorGray)
	sb.WriteString("[")
	sb.WriteString(entry.Time.Format("15:04:05.000"))
	sb.WriteString("]")
	sb.WriteString(colorReset)

	// Level with color
	sb.WriteString(" ")
	sb.WriteString(levelColor(entry.Level))
	sb.WriteString("[")
	sb.WriteString(strings.ToUpper(entry.Level))
	sb.WriteString("]")
	sb.WriteString(colorReset)

	// Message
	sb.WriteString(" ")
	sb.WriteString(entry.Msg)

	// Context fields (instance_id, phase, etc.)
	if entry.InstanceID != "" {
		sb.WriteString(" ")
		sb.WriteString(colorCyan)
		sb.WriteString("instance_id=")
		sb.WriteString(entry.InstanceID)
		sb.WriteString(colorReset)
	}
	if entry.Phase != "" {
		sb.WriteString(" ")
		sb.WriteString(colorCyan)
		sb.WriteString("phase=")
		sb.WriteString(entry.Phase)
		sb.WriteString(colorReset)
	}

	// Extra fields
	for key, value := range entry.Extra {
		sb.WriteString(" ")
		sb.WriteString(colorCyan)
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(colorReset)
		sb.WriteString(fmt.Sprintf("%v", value))
	}

	return sb.String()
}

func runLogs(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Determine which session to use
	sessionID := logsSessionID
	if sessionID == "" {
		// Find the most recent session
		sessions, err := session.ListSessions(cwd)
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}
		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return nil
		}

		// Sort by creation time (most recent first)
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].Created.After(sessions[j].Created)
		})
		sessionID = sessions[0].ID
	}

	// Locate the log file
	sessionDir := session.GetSessionDir(cwd, sessionID)
	logPath := filepath.Join(sessionDir, "debug.log")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Printf("No logs found for session %s\n", sessionID)
		fmt.Println("Logs are stored at:", logPath)
		return nil
	}

	// Parse filter options
	var minLevel int = -1
	if logsLevel != "" {
		validLevel := logging.ParseLevel(logsLevel)
		minLevel = levelPriority(validLevel)
	}

	var sinceTime time.Time
	if logsSince != "" {
		duration, err := time.ParseDuration(logsSince)
		if err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}
		sinceTime = time.Now().Add(-duration)
	}

	var grepRegex *regexp.Regexp
	if logsGrep != "" {
		var err error
		grepRegex, err = regexp.Compile(logsGrep)
		if err != nil {
			return fmt.Errorf("invalid grep pattern: %w", err)
		}
	}

	// Follow mode
	if logsFollow {
		return followLogs(logPath, minLevel, sinceTime, grepRegex)
	}

	// Non-follow mode: read and display logs
	return displayLogs(logPath, logsTail, minLevel, sinceTime, grepRegex)
}

// displayLogs reads the log file and displays filtered entries
func displayLogs(logPath string, tail int, minLevel int, sinceTime time.Time, grepRegex *regexp.Regexp) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	var entries []string
	scanner := bufio.NewScanner(file)

	// Increase buffer size for potentially long log lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry logEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// If we can't parse as JSON, display raw line
			entries = append(entries, line)
			continue
		}

		// Apply filters
		if !passesFilters(&entry, minLevel, sinceTime, grepRegex) {
			continue
		}

		entries = append(entries, formatLogEntry(&entry))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading log file: %w", err)
	}

	// Apply tail limit
	if tail > 0 && len(entries) > tail {
		entries = entries[len(entries)-tail:]
	}

	// Print entries
	for _, entry := range entries {
		fmt.Println(entry)
	}

	if len(entries) == 0 {
		fmt.Println("No matching log entries found.")
	}

	return nil
}

// followLogs implements tail -f behavior for the log file
func followLogs(logPath string, minLevel int, sinceTime time.Time, grepRegex *regexp.Regexp) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Seek to end of file
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to end: %w", err)
	}

	fmt.Printf("Following logs... (Ctrl+C to stop)\n\n")

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// No new data, wait briefly and try again
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("error reading log file: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry logEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// If we can't parse as JSON, display raw line
			fmt.Println(line)
			continue
		}

		// Apply filters
		if !passesFilters(&entry, minLevel, sinceTime, grepRegex) {
			continue
		}

		fmt.Println(formatLogEntry(&entry))
	}
}

// passesFilters checks if a log entry passes all filter criteria
func passesFilters(entry *logEntry, minLevel int, sinceTime time.Time, grepRegex *regexp.Regexp) bool {
	// Level filter
	if minLevel >= 0 && levelPriority(entry.Level) < minLevel {
		return false
	}

	// Time filter
	if !sinceTime.IsZero() && entry.Time.Before(sinceTime) {
		return false
	}

	// Grep filter - search in message and extra fields
	if grepRegex != nil {
		searchText := entry.Msg
		for _, v := range entry.Extra {
			searchText += " " + fmt.Sprintf("%v", v)
		}
		if !grepRegex.MatchString(searchText) {
			return false
		}
	}

	return true
}
