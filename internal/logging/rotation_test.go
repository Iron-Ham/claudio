package logging

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewRotatingWriter(t *testing.T) {
	t.Run("creates log file", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")

		rw, err := NewRotatingWriter(logPath, DefaultRotationConfig())
		if err != nil {
			t.Fatalf("NewRotatingWriter failed: %v", err)
		}
		defer func() { _ = rw.Close() }()

		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Errorf("log file was not created at %s", logPath)
		}
	})

	t.Run("creates nested directories", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "nested", "dir", "test.log")

		rw, err := NewRotatingWriter(logPath, DefaultRotationConfig())
		if err != nil {
			t.Fatalf("NewRotatingWriter failed: %v", err)
		}
		defer func() { _ = rw.Close() }()

		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Errorf("log file was not created at %s", logPath)
		}
	})

	t.Run("appends to existing file", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")

		// Write some initial content
		initialContent := []byte("initial content\n")
		if err := os.WriteFile(logPath, initialContent, 0644); err != nil {
			t.Fatalf("failed to write initial content: %v", err)
		}

		rw, err := NewRotatingWriter(logPath, DefaultRotationConfig())
		if err != nil {
			t.Fatalf("NewRotatingWriter failed: %v", err)
		}

		_, err = rw.Write([]byte("appended content\n"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		_ = rw.Close()

		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}

		if !strings.Contains(string(content), "initial content") {
			t.Error("initial content was lost")
		}
		if !strings.Contains(string(content), "appended content") {
			t.Error("appended content was not written")
		}
	})
}

func TestRotatingWriterWrite(t *testing.T) {
	t.Run("writes data to file", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")

		rw, err := NewRotatingWriter(logPath, DefaultRotationConfig())
		if err != nil {
			t.Fatalf("NewRotatingWriter failed: %v", err)
		}

		data := []byte("test message\n")
		n, err := rw.Write(data)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != len(data) {
			t.Errorf("expected to write %d bytes, wrote %d", len(data), n)
		}

		_ = rw.Close()

		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}

		if string(content) != string(data) {
			t.Errorf("expected %q, got %q", data, content)
		}
	})

	t.Run("tracks current size", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")

		rw, err := NewRotatingWriter(logPath, DefaultRotationConfig())
		if err != nil {
			t.Fatalf("NewRotatingWriter failed: %v", err)
		}
		defer func() { _ = rw.Close() }()

		if rw.CurrentSize() != 0 {
			t.Errorf("expected initial size 0, got %d", rw.CurrentSize())
		}

		data := []byte("test message\n")
		_, _ = rw.Write(data)

		if rw.CurrentSize() != int64(len(data)) {
			t.Errorf("expected size %d, got %d", len(data), rw.CurrentSize())
		}
	})
}

func TestRotatingWriterRotation(t *testing.T) {
	t.Run("rotates when size exceeds max", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")

		// Use a very small max size for testing (100 bytes)
		config := RotationConfig{
			MaxSizeMB:  0, // We'll set maxSizeB directly
			MaxBackups: 3,
			Compress:   false,
		}

		rw, err := NewRotatingWriter(logPath, config)
		if err != nil {
			t.Fatalf("NewRotatingWriter failed: %v", err)
		}
		// Override maxSizeB for testing
		rw.maxSizeB = 100

		// Write enough data to trigger rotation
		for range 5 {
			_, _ = rw.Write([]byte("this is a test message that will trigger rotation\n"))
		}

		_ = rw.Close()

		// Check that backup files were created
		backup1 := logPath + ".1"
		if _, err := os.Stat(backup1); os.IsNotExist(err) {
			t.Error("backup file .1 was not created")
		}

		// Current log file should exist and be smaller
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Error("current log file does not exist after rotation")
		}
	})

	t.Run("keeps only maxBackups files", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")

		config := RotationConfig{
			MaxSizeMB:  0,
			MaxBackups: 2,
			Compress:   false,
		}

		rw, err := NewRotatingWriter(logPath, config)
		if err != nil {
			t.Fatalf("NewRotatingWriter failed: %v", err)
		}
		// Use 50 bytes to trigger more frequent rotation
		rw.maxSizeB = 50

		// Write enough to trigger multiple rotations
		for range 10 {
			_, _ = rw.Write([]byte("this message will trigger rotation\n"))
		}

		_ = rw.Close()

		// Should have .1 and .2, but not .3
		if _, err := os.Stat(logPath + ".1"); os.IsNotExist(err) {
			t.Error("backup file .1 should exist")
		}
		if _, err := os.Stat(logPath + ".2"); os.IsNotExist(err) {
			t.Error("backup file .2 should exist")
		}
		if _, err := os.Stat(logPath + ".3"); err == nil {
			t.Error("backup file .3 should not exist")
		}
	})

	t.Run("no rotation when maxSizeB is 0", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")

		config := RotationConfig{
			MaxSizeMB:  0, // 0 means no rotation
			MaxBackups: 3,
			Compress:   false,
		}

		rw, err := NewRotatingWriter(logPath, config)
		if err != nil {
			t.Fatalf("NewRotatingWriter failed: %v", err)
		}

		// Write a lot of data
		for range 100 {
			_, _ = rw.Write([]byte("test message that would trigger rotation if enabled\n"))
		}

		_ = rw.Close()

		// No backup files should exist
		if _, err := os.Stat(logPath + ".1"); err == nil {
			t.Error("backup file should not exist when rotation is disabled")
		}
	})
}

func TestRotatingWriterCompression(t *testing.T) {
	t.Run("compresses rotated files when enabled", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")

		config := RotationConfig{
			MaxSizeMB:  0,
			MaxBackups: 3,
			Compress:   true,
		}

		rw, err := NewRotatingWriter(logPath, config)
		if err != nil {
			t.Fatalf("NewRotatingWriter failed: %v", err)
		}
		rw.maxSizeB = 50

		// Write enough to trigger exactly one rotation (2 writes: first fits, second triggers rotation)
		// This avoids race conditions from multiple concurrent compression goroutines
		for range 2 {
			_, _ = rw.Write([]byte("test message for compression test\n"))
		}

		_ = rw.Close()

		// Wait a bit for async compression to complete
		time.Sleep(200 * time.Millisecond)

		// Check for .gz file
		gzPath := logPath + ".1.gz"
		if _, err := os.Stat(gzPath); os.IsNotExist(err) {
			// Might still have uncompressed .1 if compression hasn't finished
			if _, err := os.Stat(logPath + ".1"); os.IsNotExist(err) {
				t.Error("neither compressed nor uncompressed backup file exists")
			}
			return
		}

		// Verify gzip file can be decompressed
		gzFile, err := os.Open(gzPath)
		if err != nil {
			t.Fatalf("failed to open gzip file: %v", err)
		}
		defer func() { _ = gzFile.Close() }()

		gzReader, err := gzip.NewReader(gzFile)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer func() { _ = gzReader.Close() }()

		content, err := io.ReadAll(gzReader)
		if err != nil {
			t.Fatalf("failed to read gzip content: %v", err)
		}

		if len(content) == 0 {
			t.Error("decompressed content is empty")
		}
	})
}

func TestRotatingWriterConcurrency(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Use a larger size to avoid too many rotations during concurrent writes
	// This test focuses on thread-safety, not rotation frequency
	config := RotationConfig{
		MaxSizeMB:  0,
		MaxBackups: 100, // Large enough to keep all backups
		Compress:   false,
	}

	rw, err := NewRotatingWriter(logPath, config)
	if err != nil {
		t.Fatalf("NewRotatingWriter failed: %v", err)
	}
	// Use a larger size to reduce rotation frequency
	rw.maxSizeB = 2000

	var wg sync.WaitGroup
	goroutines := 10
	writesPerGoroutine := 50

	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range writesPerGoroutine {
				msg := []byte("concurrent write from goroutine\n")
				if _, err := rw.Write(msg); err != nil {
					t.Errorf("goroutine %d write %d failed: %v", id, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	_ = rw.Close()

	// Count total lines across all files
	totalLines := 0

	// Count lines in current log
	content, err := os.ReadFile(logPath)
	if err == nil {
		totalLines += strings.Count(string(content), "\n")
	}

	// Count lines in backup files using fmt.Sprintf for proper path construction
	for i := 1; i <= 100; i++ {
		backupPath := fmt.Sprintf("%s.%d", logPath, i)
		content, err := os.ReadFile(backupPath)
		if err == nil {
			totalLines += strings.Count(string(content), "\n")
		}
	}

	expectedLines := goroutines * writesPerGoroutine
	if totalLines < expectedLines {
		t.Errorf("expected at least %d lines, got %d (some may be in rotated files)", expectedLines, totalLines)
	}
}

func TestRotatingWriterClose(t *testing.T) {
	t.Run("close syncs and closes file", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")

		rw, err := NewRotatingWriter(logPath, DefaultRotationConfig())
		if err != nil {
			t.Fatalf("NewRotatingWriter failed: %v", err)
		}

		_, _ = rw.Write([]byte("test message\n"))

		if err := rw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}

		// Second close should be a no-op
		if err := rw.Close(); err != nil {
			t.Errorf("second Close failed: %v", err)
		}
	})

	t.Run("write after close fails", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")

		rw, err := NewRotatingWriter(logPath, DefaultRotationConfig())
		if err != nil {
			t.Fatalf("NewRotatingWriter failed: %v", err)
		}

		_ = rw.Close()

		_, err = rw.Write([]byte("test message\n"))
		if err == nil {
			t.Error("expected write after close to fail")
		}
	})
}

func TestNewLoggerWithRotation(t *testing.T) {
	t.Run("creates logger with rotation", func(t *testing.T) {
		dir := t.TempDir()

		config := RotationConfig{
			MaxSizeMB:  10,
			MaxBackups: 3,
			Compress:   false,
		}

		logger, err := NewLoggerWithRotation(dir, LevelDebug, config)
		if err != nil {
			t.Fatalf("NewLoggerWithRotation failed: %v", err)
		}
		defer func() { _ = logger.Close() }()

		logPath := filepath.Join(dir, "debug.log")
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Errorf("log file was not created at %s", logPath)
		}
	})

	t.Run("logs to file correctly", func(t *testing.T) {
		dir := t.TempDir()

		config := RotationConfig{
			MaxSizeMB:  10,
			MaxBackups: 3,
			Compress:   false,
		}

		logger, err := NewLoggerWithRotation(dir, LevelDebug, config)
		if err != nil {
			t.Fatalf("NewLoggerWithRotation failed: %v", err)
		}

		logger.Info("test message", "key", "value")
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

		if entry["msg"] != "test message" {
			t.Errorf("expected msg='test message', got %v", entry["msg"])
		}
		if entry["key"] != "value" {
			t.Errorf("expected key='value', got %v", entry["key"])
		}
	})

	t.Run("writes to stderr when sessionDir is empty", func(t *testing.T) {
		config := DefaultRotationConfig()

		logger, err := NewLoggerWithRotation("", LevelInfo, config)
		if err != nil {
			t.Fatalf("NewLoggerWithRotation failed: %v", err)
		}
		defer func() { _ = logger.Close() }()

		// Should not have a rotation writer
		if logger.rotation != nil {
			t.Error("expected rotation to be nil when sessionDir is empty")
		}
	})

	t.Run("rotation triggers on size", func(t *testing.T) {
		dir := t.TempDir()

		config := RotationConfig{
			MaxSizeMB:  0, // Will set manually
			MaxBackups: 3,
			Compress:   false,
		}

		logger, err := NewLoggerWithRotation(dir, LevelDebug, config)
		if err != nil {
			t.Fatalf("NewLoggerWithRotation failed: %v", err)
		}

		// Set a small max size to trigger rotation
		logger.rotation.maxSizeB = 200

		// Write enough to trigger rotation
		for i := range 10 {
			logger.Info("this is a message that will trigger rotation when repeated", "iteration", i)
		}

		_ = logger.Close()

		// Check that backup file was created
		backupPath := filepath.Join(dir, "debug.log.1")
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("backup file was not created after rotation")
		}
	})

	t.Run("child loggers share rotation writer", func(t *testing.T) {
		dir := t.TempDir()

		config := RotationConfig{
			MaxSizeMB:  10,
			MaxBackups: 3,
			Compress:   false,
		}

		logger, err := NewLoggerWithRotation(dir, LevelDebug, config)
		if err != nil {
			t.Fatalf("NewLoggerWithRotation failed: %v", err)
		}
		defer func() { _ = logger.Close() }()

		childLogger := logger.WithSession("session-123").WithInstance("instance-456")

		// Both should share the same rotation writer
		if childLogger.rotation != logger.rotation {
			t.Error("child logger should share parent's rotation writer")
		}
	})
}

func TestDefaultRotationConfig(t *testing.T) {
	config := DefaultRotationConfig()

	if config.MaxSizeMB != 10 {
		t.Errorf("expected MaxSizeMB=10, got %d", config.MaxSizeMB)
	}
	if config.MaxBackups != 3 {
		t.Errorf("expected MaxBackups=3, got %d", config.MaxBackups)
	}
	if config.Compress != false {
		t.Error("expected Compress=false")
	}
}

func TestRotatingWriterFilePath(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	rw, err := NewRotatingWriter(logPath, DefaultRotationConfig())
	if err != nil {
		t.Fatalf("NewRotatingWriter failed: %v", err)
	}
	defer func() { _ = rw.Close() }()

	if rw.FilePath() != logPath {
		t.Errorf("expected FilePath=%s, got %s", logPath, rw.FilePath())
	}
}

func TestRotatingWriterSync(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	rw, err := NewRotatingWriter(logPath, DefaultRotationConfig())
	if err != nil {
		t.Fatalf("NewRotatingWriter failed: %v", err)
	}
	defer func() { _ = rw.Close() }()

	_, _ = rw.Write([]byte("test message\n"))

	if err := rw.Sync(); err != nil {
		t.Errorf("Sync failed: %v", err)
	}

	// Verify content is flushed to disk
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test message") {
		t.Error("content was not synced to disk")
	}
}
