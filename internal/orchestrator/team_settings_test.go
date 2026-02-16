package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Ham/claudio/internal/ai"
)

// stubBackend implements ai.Backend with a configurable name for testing.
type stubBackend struct {
	ai.Backend
	name ai.BackendName
}

func (s *stubBackend) Name() ai.BackendName { return s.name }

func TestWriteWorktreeTeamSettings_CreatesFile(t *testing.T) {
	wtPath := t.TempDir()
	o := &Orchestrator{
		backend: &stubBackend{name: ai.BackendClaude},
	}

	o.writeWorktreeTeamSettings("inst-1", wtPath)

	settingsFile := filepath.Join(wtPath, ".claude", "settings.local.json")
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatalf("expected settings file to exist: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("failed to parse settings: %v", err)
	}

	if got := settings["teammateMode"]; got != "in-process" {
		t.Errorf("teammateMode = %v, want %q", got, "in-process")
	}
}

func TestWriteWorktreeTeamSettings_MergesExisting(t *testing.T) {
	wtPath := t.TempDir()
	claudeDir := filepath.Join(wtPath, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	existing := map[string]any{
		"permissions": map[string]any{"allow": []string{"Read"}},
		"otherKey":    42,
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	o := &Orchestrator{
		backend: &stubBackend{name: ai.BackendClaude},
	}

	o.writeWorktreeTeamSettings("inst-1", wtPath)

	result, err := os.ReadFile(filepath.Join(claudeDir, "settings.local.json"))
	if err != nil {
		t.Fatalf("expected settings file to exist: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(result, &settings); err != nil {
		t.Fatalf("failed to parse settings: %v", err)
	}

	// Verify new key was added
	if got := settings["teammateMode"]; got != "in-process" {
		t.Errorf("teammateMode = %v, want %q", got, "in-process")
	}

	// Verify existing keys were preserved
	if settings["otherKey"] == nil {
		t.Error("existing key 'otherKey' was lost during merge")
	}
	if settings["permissions"] == nil {
		t.Error("existing key 'permissions' was lost during merge")
	}
}

func TestWriteWorktreeTeamSettings_SkipsNonClaude(t *testing.T) {
	wtPath := t.TempDir()
	o := &Orchestrator{
		backend: &stubBackend{name: ai.BackendCodex},
	}

	o.writeWorktreeTeamSettings("inst-1", wtPath)

	settingsFile := filepath.Join(wtPath, ".claude", "settings.local.json")
	if _, err := os.Stat(settingsFile); !os.IsNotExist(err) {
		t.Error("expected no settings file for non-Claude backend")
	}
}

func TestWriteWorktreeTeamSettings_SkipsNilBackend(t *testing.T) {
	wtPath := t.TempDir()
	o := &Orchestrator{
		backend: nil,
	}

	o.writeWorktreeTeamSettings("inst-1", wtPath)

	settingsFile := filepath.Join(wtPath, ".claude", "settings.local.json")
	if _, err := os.Stat(settingsFile); !os.IsNotExist(err) {
		t.Error("expected no settings file when backend is nil")
	}
}

func TestWriteWorktreeTeamSettings_CreatesDotClaudeDir(t *testing.T) {
	wtPath := t.TempDir()
	o := &Orchestrator{
		backend: &stubBackend{name: ai.BackendClaude},
	}

	// .claude dir does not exist yet
	claudeDir := filepath.Join(wtPath, ".claude")
	if _, err := os.Stat(claudeDir); !os.IsNotExist(err) {
		t.Fatal("expected .claude dir to not exist before test")
	}

	o.writeWorktreeTeamSettings("inst-1", wtPath)

	// .claude dir should now exist
	info, err := os.Stat(claudeDir)
	if err != nil {
		t.Fatalf("expected .claude dir to be created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected .claude to be a directory")
	}
}

func TestWriteWorktreeTeamSettings_OverwritesInvalidJSON(t *testing.T) {
	wtPath := t.TempDir()
	claudeDir := filepath.Join(wtPath, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write invalid JSON
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	o := &Orchestrator{
		backend: &stubBackend{name: ai.BackendClaude},
	}

	o.writeWorktreeTeamSettings("inst-1", wtPath)

	result, err := os.ReadFile(filepath.Join(claudeDir, "settings.local.json"))
	if err != nil {
		t.Fatalf("expected settings file to exist: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(result, &settings); err != nil {
		t.Fatalf("failed to parse settings: %v", err)
	}

	if got := settings["teammateMode"]; got != "in-process" {
		t.Errorf("teammateMode = %v, want %q", got, "in-process")
	}
	if len(settings) != 1 {
		t.Errorf("expected exactly 1 key after overwriting invalid JSON, got %d", len(settings))
	}
}

func TestWriteWorktreeTeamSettings_UnreadableFileStillWrites(t *testing.T) {
	wtPath := t.TempDir()
	claudeDir := filepath.Join(wtPath, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settingsFile := filepath.Join(claudeDir, "settings.local.json")
	// Write a file with write-only permissions: ReadFile fails but WriteFile succeeds
	if err := os.WriteFile(settingsFile, []byte(`{"existing": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(settingsFile, 0o200); err != nil {
		t.Fatal(err)
	}

	o := &Orchestrator{
		backend: &stubBackend{name: ai.BackendClaude},
	}

	o.writeWorktreeTeamSettings("inst-1", wtPath)

	// Restore permissions so we can read the result
	if err := os.Chmod(settingsFile, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatalf("expected settings file to exist: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(result, &settings); err != nil {
		t.Fatalf("failed to parse settings: %v", err)
	}

	// Should have written fresh settings (existing content was unreadable)
	if got := settings["teammateMode"]; got != "in-process" {
		t.Errorf("teammateMode = %v, want %q", got, "in-process")
	}
	// The existing key should NOT be preserved since the file was unreadable
	if settings["existing"] != nil {
		t.Error("expected existing keys to be lost when file was unreadable")
	}
}
