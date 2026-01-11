package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestSession(t *testing.T, baseDir, sessionID, name string, instanceCount int) {
	t.Helper()
	sessionDir := GetSessionDir(baseDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}

	// Create instances slice
	instances := make([]map[string]string, instanceCount)
	for i := 0; i < instanceCount; i++ {
		instances[i] = map[string]string{"id": "inst" + string(rune('0'+i))}
	}

	data := map[string]interface{}{
		"id":        sessionID,
		"name":      name,
		"created":   time.Now(),
		"instances": instances,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal session data: %v", err)
	}

	sessionFile := filepath.Join(sessionDir, SessionFileName)
	if err := os.WriteFile(sessionFile, jsonData, 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}
}

func TestFindEmptySessions(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "claudio-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Create sessions with different instance counts
	setupTestSession(t, tempDir, "session-empty-1", "empty-one", 0)
	setupTestSession(t, tempDir, "session-empty-2", "empty-two", 0)
	setupTestSession(t, tempDir, "session-with-instances", "has-work", 3)

	// Find empty sessions
	empty, err := FindEmptySessions(tempDir)
	if err != nil {
		t.Fatalf("FindEmptySessions() error = %v", err)
	}

	// Should find 2 empty sessions
	if len(empty) != 2 {
		t.Errorf("FindEmptySessions() returned %d sessions, want 2", len(empty))
	}

	// Verify they are the expected empty sessions
	foundEmpty1 := false
	foundEmpty2 := false
	for _, s := range empty {
		if s.ID == "session-empty-1" {
			foundEmpty1 = true
		}
		if s.ID == "session-empty-2" {
			foundEmpty2 = true
		}
		if s.InstanceCount != 0 {
			t.Errorf("Empty session %s has InstanceCount = %d, want 0", s.ID, s.InstanceCount)
		}
	}
	if !foundEmpty1 {
		t.Error("FindEmptySessions() did not return session-empty-1")
	}
	if !foundEmpty2 {
		t.Error("FindEmptySessions() did not return session-empty-2")
	}
}

func TestFindEmptySessions_NoSessions(t *testing.T) {
	// Create temp directory without any sessions
	tempDir, err := os.MkdirTemp("", "claudio-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	empty, err := FindEmptySessions(tempDir)
	if err != nil {
		t.Fatalf("FindEmptySessions() error = %v", err)
	}

	if len(empty) != 0 {
		t.Errorf("FindEmptySessions() returned %d sessions, want 0", len(empty))
	}
}

func TestRemoveSession(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "claudio-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Create a session
	sessionID := "session-to-remove"
	setupTestSession(t, tempDir, sessionID, "test-session", 0)

	// Verify it exists
	if !SessionExists(tempDir, sessionID) {
		t.Fatal("Session should exist before removal")
	}

	// Remove the session
	err = RemoveSession(tempDir, sessionID)
	if err != nil {
		t.Fatalf("RemoveSession() error = %v", err)
	}

	// Verify it's gone
	if SessionExists(tempDir, sessionID) {
		t.Error("Session should not exist after removal")
	}
}

func TestRemoveSession_NonExistent(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "claudio-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Try to remove a non-existent session (should not error)
	err = RemoveSession(tempDir, "non-existent-session")
	if err != nil {
		t.Errorf("RemoveSession() for non-existent session should not error, got %v", err)
	}
}

func TestGetSessionsDir(t *testing.T) {
	baseDir := "/some/path"
	expected := "/some/path/.claudio/sessions"
	result := GetSessionsDir(baseDir)
	if result != expected {
		t.Errorf("GetSessionsDir(%q) = %q, want %q", baseDir, result, expected)
	}
}

func TestGetSessionDir(t *testing.T) {
	baseDir := "/some/path"
	sessionID := "abc12345"
	expected := "/some/path/.claudio/sessions/abc12345"
	result := GetSessionDir(baseDir, sessionID)
	if result != expected {
		t.Errorf("GetSessionDir(%q, %q) = %q, want %q", baseDir, sessionID, result, expected)
	}
}

func TestTruncateID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		maxLen   int
		expected string
	}{
		{"normal truncation", "abcdefghij", 8, "abcdefgh"},
		{"exact length", "abcdefgh", 8, "abcdefgh"},
		{"shorter than max", "abc", 8, "abc"},
		{"empty string", "", 8, ""},
		{"zero max length", "abcdefgh", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateID(tt.id, tt.maxLen)
			if result != tt.expected {
				t.Errorf("TruncateID(%q, %d) = %q, want %q", tt.id, tt.maxLen, result, tt.expected)
			}
		})
	}
}
