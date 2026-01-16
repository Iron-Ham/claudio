package session

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// captureOutput captures stdout during function execution
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrintCleanupSummary(t *testing.T) {
	result := &CleanupResult{
		StaleWorktrees: []StaleWorktree{
			{Path: "/path/to/wt1", Branch: "claudio/abc-feature", HasUncommitted: false},
			{Path: "/path/to/wt2", Branch: "claudio/def-bugfix", HasUncommitted: true},
		},
		StaleBranches:    []string{"claudio/orphan-branch"},
		OrphanedTmuxSess: []string{"claudio-orphan123"},
	}

	// Temporarily set flags to enable all output sections
	originalWorktrees := cleanupWorktrees
	originalBranches := cleanupBranches
	originalTmux := cleanupTmux
	cleanupWorktrees = true
	cleanupBranches = true
	cleanupTmux = true
	defer func() {
		cleanupWorktrees = originalWorktrees
		cleanupBranches = originalBranches
		cleanupTmux = originalTmux
	}()

	output := captureOutput(func() {
		printCleanupSummary(result, false) // cleanAll=false, use specific flags instead
	})

	// Verify output contains expected sections (using current format)
	expectedContent := []string{
		"Stale Resources Found",
		"Worktrees (2):",
		"wt1",
		"wt2",
		"[uncommitted changes]",
		"Branches (1):",
		"claudio/orphan-branch",
		"Orphaned Tmux Sessions (1):",
		"claudio-orphan123",
	}

	for _, expected := range expectedContent {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("printCleanupSummary() output missing expected content: %q\nFull output:\n%s", expected, output)
		}
	}
}

func TestPrintCleanupSummary_NoResources(t *testing.T) {
	result := &CleanupResult{
		StaleWorktrees:    []StaleWorktree{},
		StaleBranches:     []string{},
		OrphanedTmuxSess:  []string{},
		ActiveInstanceIDs: make(map[string]bool),
	}

	output := captureOutput(func() {
		printCleanupSummary(result, true) // cleanAll=true to print all sections
	})

	// With no resources, printCleanupSummary still prints the header
	// The "Nothing to clean up" message is in runCleanup, not printCleanupSummary
	if !bytes.Contains([]byte(output), []byte("Stale Resources Found")) {
		t.Errorf("printCleanupSummary() should output header, got: %q", output)
	}

	// Should NOT contain any resource listings (no worktrees, branches, etc.)
	unexpectedContent := []string{
		"Worktrees",
		"Branches",
		"Orphaned Tmux Sessions",
	}

	for _, unexpected := range unexpectedContent {
		if bytes.Contains([]byte(output), []byte(unexpected)) {
			t.Errorf("printCleanupSummary() with no resources should not contain %q\nFull output:\n%s", unexpected, output)
		}
	}
}
