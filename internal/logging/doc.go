// Package logging provides structured logging for Claudio sessions.
//
// This package wraps Go's log/slog to provide JSON-formatted logs with
// context propagation support for debugging and post-hoc analysis. It is
// designed to help troubleshoot multi-instance Claude sessions by providing
// structured, filterable logs that can be analyzed after the fact.
//
// # Features
//
//   - JSON-formatted structured logging via slog
//   - Configurable log levels (DEBUG, INFO, WARN, ERROR)
//   - Context propagation (session ID, instance ID, phase)
//   - Log rotation with configurable size limits
//   - Optional gzip compression for rotated logs
//   - Log aggregation and filtering utilities
//   - Export to JSON, text, or CSV formats
//
// # Thread Safety
//
// All types in this package are safe for concurrent use. The [Logger] type
// uses Go's slog internally which is designed for concurrent access. The
// [RotatingWriter] type uses a mutex to protect file operations during
// rotation. Child loggers created via With* methods share the underlying
// writer safely.
//
// # Basic Usage
//
// Create a logger for a session directory:
//
//	logger, err := logging.NewLogger("/path/to/session", "INFO")
//	if err != nil {
//	    return err
//	}
//	defer logger.Close()
//
//	// Log messages at various levels
//	logger.Debug("detailed info", "key", "value")
//	logger.Info("operation completed", "duration_ms", 150)
//	logger.Warn("potential issue", "threshold", 100)
//	logger.Error("operation failed", "error", err.Error())
//
// # Context Propagation
//
// Create child loggers with persistent context attributes:
//
//	// Add session context
//	sessionLogger := logger.WithSession("session-abc123")
//
//	// Add instance context
//	instanceLogger := sessionLogger.WithInstance("instance-def456")
//
//	// Add phase context
//	phaseLogger := instanceLogger.WithPhase("execution")
//
//	// All logs from phaseLogger will include session_id, instance_id, and phase
//	phaseLogger.Info("task completed", "task", "implement auth")
//
// Output:
//
//	{"time":"...","level":"INFO","msg":"task completed","session_id":"session-abc123","instance_id":"instance-def456","phase":"execution","task":"implement auth"}
//
// # Log Rotation
//
// For long-running sessions, use log rotation to prevent unbounded growth:
//
//	config := logging.RotationConfig{
//	    MaxSizeMB:  10,    // Rotate when file exceeds 10MB
//	    MaxBackups: 3,     // Keep 3 backup files
//	    Compress:   true,  // Gzip compress rotated files
//	}
//
//	logger, err := logging.NewLoggerWithRotation("/path/to/session", "INFO", config)
//	if err != nil {
//	    return err
//	}
//	defer logger.Close()
//
// Rotated files are named: debug.log.1, debug.log.2, etc., where .1 is the
// most recent backup. When compression is enabled, rotated files become
// debug.log.1.gz, etc.
//
// # Testing
//
// For testing, use [NopLogger] to discard all log output:
//
//	func TestSomething(t *testing.T) {
//	    logger := logging.NopLogger()
//	    // Use logger in tests without creating files
//	}
//
// # Log Aggregation and Filtering
//
// Read and analyze logs after a session:
//
//	// Load all logs from a session
//	entries, err := logging.AggregateLogs("/path/to/session")
//	if err != nil {
//	    return err
//	}
//
//	// Filter logs by various criteria
//	filter := logging.LogFilter{
//	    Level:      "WARN",           // Minimum level
//	    InstanceID: "instance-123",   // Specific instance
//	    Phase:      "consolidation",  // Specific phase
//	    StartTime:  time.Now().Add(-1 * time.Hour),  // Last hour
//	}
//	filtered := logging.FilterLogs(entries, filter)
//
//	// Export to various formats
//	logging.ExportLogEntries(filtered, "errors.json", "json")
//	logging.ExportLogEntries(filtered, "errors.txt", "text")
//	logging.ExportLogEntries(filtered, "errors.csv", "csv")
//
// # Log Levels
//
// The package defines four log levels:
//
//   - [LevelDebug]: Detailed information for debugging
//   - [LevelInfo]: General operational information (default)
//   - [LevelWarn]: Warning conditions that may need attention
//   - [LevelError]: Error conditions that affect functionality
//
// Use [ValidLevels] to get the list of valid level strings, and [ParseLevel]
// to normalize user-provided level strings.
//
// # Configuration
//
// The logging system is typically configured via Claudio's config file:
//
//	logging:
//	  enabled: true
//	  level: info
//	  max_size_mb: 10
//	  max_backups: 3
//
// See the Claudio README for complete configuration documentation.
package logging
