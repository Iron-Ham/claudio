// Package logging provides structured logging for Claudio sessions.
package logging

import (
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RotationConfig holds configuration for log rotation.
type RotationConfig struct {
	// MaxSizeMB is the maximum size of a log file in megabytes before rotation.
	// A value of 0 disables rotation.
	MaxSizeMB int
	// MaxBackups is the number of old log files to keep.
	// A value of 0 keeps no backups.
	MaxBackups int
	// Compress determines whether rotated log files are gzip compressed.
	Compress bool
}

// DefaultRotationConfig returns a RotationConfig with sensible defaults.
func DefaultRotationConfig() RotationConfig {
	return RotationConfig{
		MaxSizeMB:  10,
		MaxBackups: 3,
		Compress:   false,
	}
}

// RotatingWriter wraps an io.Writer and implements automatic log rotation
// based on file size. It is safe for concurrent use.
type RotatingWriter struct {
	mu sync.Mutex

	// Configuration
	filePath   string
	maxSizeB   int64 // Maximum size in bytes
	maxBackups int
	compress   bool

	// State
	file        *os.File
	currentSize int64
}

// NewRotatingWriter creates a new RotatingWriter that writes to the specified
// file path and rotates when the file exceeds maxSizeMB megabytes.
//
// If maxSizeMB is 0, rotation is disabled and the writer behaves like a
// regular file writer.
func NewRotatingWriter(filePath string, config RotationConfig) (*RotatingWriter, error) {
	rw := &RotatingWriter{
		filePath:   filePath,
		maxSizeB:   int64(config.MaxSizeMB) * 1024 * 1024,
		maxBackups: config.MaxBackups,
		compress:   config.Compress,
	}

	if err := rw.openFile(); err != nil {
		return nil, err
	}

	return rw, nil
}

// openFile opens the log file for writing and sets the current size.
// The caller must hold the mutex.
func (rw *RotatingWriter) openFile() error {
	// Ensure parent directory exists
	dir := filepath.Dir(rw.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.OpenFile(rw.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	rw.file = file
	rw.currentSize = info.Size()
	return nil
}

// Write implements io.Writer. It writes data to the log file and rotates
// if the file size exceeds the maximum.
func (rw *RotatingWriter) Write(p []byte) (n int, err error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.file == nil {
		return 0, fmt.Errorf("log file is closed")
	}

	// Check if we need to rotate before writing
	if rw.maxSizeB > 0 && rw.currentSize+int64(len(p)) > rw.maxSizeB {
		if err := rw.rotate(); err != nil {
			// Log rotation failed, but we should still try to write
			// to the current file to avoid losing log data.
			// Write to stderr so operators are aware rotation is failing.
			fmt.Fprintf(os.Stderr, "Warning: log rotation failed: %v\n", err)
		}
	}

	n, err = rw.file.Write(p)
	rw.currentSize += int64(n)
	return n, err
}

// rotate performs the log rotation. The caller must hold the mutex.
func (rw *RotatingWriter) rotate() error {
	// Close current file
	if err := rw.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync log file: %w", err)
	}
	if err := rw.file.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
	}
	rw.file = nil

	// Rotate existing backup files (shift numbers up)
	// Delete the oldest if we have too many
	if err := rw.rotateBackups(); err != nil {
		// Continue even if backup rotation fails
		_ = err
	}

	// Rename current log to .1
	backupPath := rw.backupPath(1)
	if err := os.Rename(rw.filePath, backupPath); err != nil {
		// If rename fails, try to reopen the original file
		if openErr := rw.openFile(); openErr != nil {
			return fmt.Errorf("failed to rename log file and reopen: %w", openErr)
		}
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	// Compress the new backup asynchronously if configured
	if rw.compress {
		go rw.compressFile(backupPath)
	}

	// Open a new log file
	return rw.openFile()
}

// rotateBackups shifts backup files and removes the oldest if necessary.
// Files are numbered: .1 (newest) to .N (oldest).
func (rw *RotatingWriter) rotateBackups() error {
	if rw.maxBackups <= 0 {
		// No backups, just remove any existing .1 file
		os.Remove(rw.backupPath(1))
		os.Remove(rw.backupPath(1) + ".gz")
		return nil
	}

	// Remove the oldest backup if it exists
	oldestPath := rw.backupPath(rw.maxBackups)
	os.Remove(oldestPath)
	os.Remove(oldestPath + ".gz")

	// Shift all backups up by one
	for i := rw.maxBackups - 1; i >= 1; i-- {
		oldPath := rw.backupPath(i)
		newPath := rw.backupPath(i + 1)

		// Try both compressed and uncompressed versions
		if _, err := os.Stat(oldPath + ".gz"); err == nil {
			os.Rename(oldPath+".gz", newPath+".gz")
		} else if _, err := os.Stat(oldPath); err == nil {
			os.Rename(oldPath, newPath)
		}
	}

	return nil
}

// backupPath returns the path for a backup file with the given number.
func (rw *RotatingWriter) backupPath(n int) string {
	return fmt.Sprintf("%s.%d", rw.filePath, n)
}

// compressFile compresses a file using gzip and removes the original.
// Errors are logged to stderr since this runs asynchronously.
func (rw *RotatingWriter) compressFile(path string) {
	// Read the original file
	data, err := os.ReadFile(path)
	if err != nil {
		// Log but continue - the uncompressed backup is still there
		fmt.Fprintf(os.Stderr, "Warning: failed to read log file for compression %s: %v\n", path, err)
		return
	}

	// Create the compressed file
	gzPath := path + ".gz"
	gzFile, err := os.Create(gzPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create compressed log file %s: %v\n", gzPath, err)
		return
	}
	defer gzFile.Close()

	gzWriter := gzip.NewWriter(gzFile)
	if _, err := gzWriter.Write(data); err != nil {
		os.Remove(gzPath) // Clean up partial file
		fmt.Fprintf(os.Stderr, "Warning: failed to write compressed log data to %s: %v\n", gzPath, err)
		return
	}

	if err := gzWriter.Close(); err != nil {
		os.Remove(gzPath) // Clean up partial file
		fmt.Fprintf(os.Stderr, "Warning: failed to finalize compressed log file %s: %v\n", gzPath, err)
		return
	}

	// Only remove the original after successful compression
	os.Remove(path)
}

// Sync flushes any buffered data to the underlying file.
func (rw *RotatingWriter) Sync() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.file == nil {
		return nil
	}

	return rw.file.Sync()
}

// Close closes the RotatingWriter. It syncs and closes the underlying file.
func (rw *RotatingWriter) Close() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.file == nil {
		return nil
	}

	if err := rw.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync log file: %w", err)
	}

	if err := rw.file.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
	}

	rw.file = nil
	return nil
}

// CurrentSize returns the current size of the log file in bytes.
func (rw *RotatingWriter) CurrentSize() int64 {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.currentSize
}

// FilePath returns the path to the log file.
func (rw *RotatingWriter) FilePath() string {
	return rw.filePath
}

// File returns the underlying os.File. This is primarily for use by
// the Logger to support the existing Close() behavior.
func (rw *RotatingWriter) File() *os.File {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.file
}
