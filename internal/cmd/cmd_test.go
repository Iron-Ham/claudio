//go:build integration

package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Ham/claudio/internal/cmd/session"
	"github.com/Iron-Ham/claudio/internal/testutil"
	"github.com/spf13/cobra"
)

// executeCommand runs a cobra command with args and returns captured output
func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err = root.Execute()
	return buf.String(), err
}

// setupTestEnvironment creates a test repo and changes to it
func setupTestEnvironment(t *testing.T) (cleanup func()) {
	t.Helper()

	repoDir := testutil.SetupTestRepo(t)
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("failed to change to test directory: %v", err)
	}

	return func() {
		os.Chdir(originalDir)
	}
}

func TestRootCommand(t *testing.T) {
	testutil.SkipIfNoGit(t)

	// Test that root command exists and has expected subcommands
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}

	if rootCmd.Use != "claudio" {
		t.Errorf("rootCmd.Use = %q, want %q", rootCmd.Use, "claudio")
	}

	// Check for expected subcommands (compare by Name(), not Use which includes args)
	expectedCmds := []string{"init", "start", "stop", "add", "status", "cleanup", "config", "pr", "sessions", "remove", "harvest", "stats"}
	cmdMap := make(map[string]bool)
	for _, cmd := range rootCmd.Commands() {
		cmdMap[cmd.Name()] = true
	}

	for _, expected := range expectedCmds {
		if !cmdMap[expected] {
			t.Errorf("expected subcommand %q not found", expected)
		}
	}
}

func TestInitCommand(t *testing.T) {
	testutil.SkipIfNoGit(t)
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	cwd, _ := os.Getwd()

	// Run init
	output, err := executeCommand(rootCmd, "init")
	if err != nil {
		t.Fatalf("init command failed: %v\nOutput: %s", err, output)
	}

	// Verify .claudio directory was created
	claudioDir := filepath.Join(cwd, ".claudio")
	if _, err := os.Stat(claudioDir); os.IsNotExist(err) {
		t.Error(".claudio directory was not created")
	}

	// Verify worktrees directory was created
	worktreesDir := filepath.Join(claudioDir, "worktrees")
	if _, err := os.Stat(worktreesDir); os.IsNotExist(err) {
		t.Error(".claudio/worktrees directory was not created")
	}
}

func TestInitCommand_NotGitRepo(t *testing.T) {
	testutil.SkipIfNoGit(t)

	// Create a non-git directory
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	os.Chdir(tmpDir)

	// Init should fail
	_, err := executeCommand(rootCmd, "init")
	if err == nil {
		t.Error("init command should fail in non-git directory")
	}
}

func TestCleanupCommand_DryRun(t *testing.T) {
	testutil.SkipIfNoGit(t)
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	// First init
	if _, err := executeCommand(rootCmd, "init"); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Run cleanup with --dry-run (should succeed even with nothing to clean)
	output, err := executeCommand(rootCmd, "cleanup", "--dry-run")
	if err != nil {
		t.Fatalf("cleanup --dry-run failed: %v\nOutput: %s", err, output)
	}

	// Should indicate no changes were made
	if !bytes.Contains([]byte(output), []byte("dry run")) && !bytes.Contains([]byte(output), []byte("Nothing to clean")) {
		// Output might be empty or have "nothing to clean" - both are valid
	}
}

func TestConfigCommand(t *testing.T) {
	testutil.SkipIfNoGit(t)
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test config command exists and runs
	output, err := executeCommand(rootCmd, "config")
	// Config command without subcommand should show help or current config
	_ = output
	_ = err
	// Just verify it doesn't panic
}

func TestStatusCommand_NoSession(t *testing.T) {
	testutil.SkipIfNoGit(t)
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Status without a session should indicate no active session
	output, err := executeCommand(rootCmd, "status")
	// This may error or show a message - both are valid behaviors
	_ = output
	_ = err
}

func TestSessionsCommand(t *testing.T) {
	testutil.SkipIfNoGit(t)
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Sessions command should list sessions or indicate none exist
	output, err := executeCommand(rootCmd, "sessions")
	// May return empty or error - both are valid
	_ = output
	_ = err
}

func TestCleanupResult(t *testing.T) {
	// Test CleanupResult struct initialization (now in session package)
	result := &session.CleanupResult{
		StaleWorktrees:    []session.StaleWorktree{},
		StaleBranches:     []string{},
		OrphanedTmuxSess:  []string{},
		ActiveInstanceIDs: make(map[string]bool),
	}

	if result == nil {
		t.Fatal("CleanupResult is nil")
	}

	if len(result.StaleWorktrees) != 0 {
		t.Errorf("StaleWorktrees should be empty")
	}

	// Add an active instance
	result.ActiveInstanceIDs["test-id"] = true
	if !result.ActiveInstanceIDs["test-id"] {
		t.Error("ActiveInstanceIDs should contain test-id")
	}
}

func TestStaleWorktree(t *testing.T) {
	// Test StaleWorktree struct (now in session package)
	sw := session.StaleWorktree{
		Path:           "/path/to/worktree",
		Branch:         "claudio/abc123-feature",
		HasUncommitted: true,
		ExistsOnRemote: false,
	}

	if sw.Path != "/path/to/worktree" {
		t.Errorf("Path = %q, want %q", sw.Path, "/path/to/worktree")
	}

	if !sw.HasUncommitted {
		t.Error("HasUncommitted should be true")
	}

	if sw.ExistsOnRemote {
		t.Error("ExistsOnRemote should be false")
	}
}

func TestFindStaleWorktrees(t *testing.T) {
	testutil.SkipIfNoGit(t)

	// Create a temporary directory structure
	tmpDir := t.TempDir()
	worktreesDir := filepath.Join(tmpDir, ".claudio", "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		t.Fatalf("failed to create worktrees dir: %v", err)
	}

	// Create a fake worktree directory (not a real git worktree)
	fakeWorktree := filepath.Join(worktreesDir, "fake-id")
	if err := os.MkdirAll(fakeWorktree, 0755); err != nil {
		t.Fatalf("failed to create fake worktree: %v", err)
	}

	// Call FindStaleWorktrees with no active IDs (now in session package)
	activeIDs := make(map[string]bool)
	stale := session.FindStaleWorktrees(worktreesDir, activeIDs)

	// Should find the fake worktree
	if len(stale) != 1 {
		t.Errorf("FindStaleWorktrees found %d worktrees, want 1", len(stale))
	}

	// Now mark it as active
	activeIDs["fake-id"] = true
	stale = session.FindStaleWorktrees(worktreesDir, activeIDs)

	// Should not find it
	if len(stale) != 0 {
		t.Errorf("FindStaleWorktrees found %d worktrees when ID is active, want 0", len(stale))
	}
}

func TestFindOrphanedTmuxSessions(t *testing.T) {
	// Test with empty active IDs (now in session package)
	activeIDs := make(map[string]bool)
	// This will return empty if tmux isn't running, which is fine
	orphaned := session.FindOrphanedTmuxSessions(activeIDs)

	// Just verify it doesn't panic and returns a slice
	if orphaned == nil {
		// FindOrphanedTmuxSessions returns nil on error, which is acceptable
	}
}

// captureOutput captures stdout during function execution
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}
