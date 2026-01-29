package session

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/cleanup"
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

func TestShowJobStatus_NonexistentJob(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	err = showJobStatus(tmpDir, "nonexistent-job-id")
	if err == nil {
		t.Error("showJobStatus() should return error for non-existent job")
	}
	if !strings.Contains(err.Error(), "failed to load job") {
		t.Errorf("showJobStatus() error = %v, want error containing 'failed to load job'", err)
	}
}

func TestShowJobStatus_PendingJob(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a pending job
	job := cleanup.NewJob(tmpDir)
	job.Status = cleanup.JobStatusPending
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	output := captureOutput(func() {
		err = showJobStatus(tmpDir, job.ID)
	})
	if err != nil {
		t.Errorf("showJobStatus() error = %v", err)
	}

	expectedContent := []string{
		"Cleanup Job: " + job.ID,
		"Status: pending",
		"Created:",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(output, expected) {
			t.Errorf("showJobStatus() output missing expected content: %q\nFull output:\n%s", expected, output)
		}
	}
}

func TestShowJobStatus_CompletedJob(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a completed job with results
	job := cleanup.NewJob(tmpDir)
	job.Status = cleanup.JobStatusCompleted
	job.StartedAt = time.Now().Add(-time.Second)
	job.EndedAt = time.Now()
	job.Results = &cleanup.JobResults{
		WorktreesRemoved:   2,
		BranchesDeleted:    1,
		TmuxSessionsKilled: 3,
		SessionsRemoved:    0,
		TotalRemoved:       6,
	}
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	output := captureOutput(func() {
		err = showJobStatus(tmpDir, job.ID)
	})
	if err != nil {
		t.Errorf("showJobStatus() error = %v", err)
	}

	expectedContent := []string{
		"Cleanup Job: " + job.ID,
		"Status: completed",
		"Started:",
		"Ended:",
		"Duration:",
		"Results:",
		"Worktrees removed: 2",
		"Branches deleted: 1",
		"Tmux sessions killed: 3",
		"Sessions removed: 0",
		"Total: 6",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(output, expected) {
			t.Errorf("showJobStatus() output missing expected content: %q\nFull output:\n%s", expected, output)
		}
	}
}

func TestShowJobStatus_FailedJobWithError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a failed job with error message
	job := cleanup.NewJob(tmpDir)
	job.Status = cleanup.JobStatusFailed
	job.Error = "something went wrong"
	job.StartedAt = time.Now().Add(-time.Second)
	job.EndedAt = time.Now()
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	output := captureOutput(func() {
		err = showJobStatus(tmpDir, job.ID)
	})
	if err != nil {
		t.Errorf("showJobStatus() error = %v", err)
	}

	expectedContent := []string{
		"Status: failed",
		"Error: something went wrong",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(output, expected) {
			t.Errorf("showJobStatus() output missing expected content: %q\nFull output:\n%s", expected, output)
		}
	}
}

func TestShowJobStatus_ResultsWithErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a completed job with some errors in results
	job := cleanup.NewJob(tmpDir)
	job.Status = cleanup.JobStatusCompleted
	job.StartedAt = time.Now().Add(-time.Second)
	job.EndedAt = time.Now()
	job.Results = &cleanup.JobResults{
		WorktreesRemoved: 1,
		TotalRemoved:     1,
		Errors:           []string{"failed to remove worktree xyz", "permission denied on branch"},
	}
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	output := captureOutput(func() {
		err = showJobStatus(tmpDir, job.ID)
	})
	if err != nil {
		t.Errorf("showJobStatus() error = %v", err)
	}

	expectedContent := []string{
		"Warnings/Errors (2):",
		"failed to remove worktree xyz",
		"permission denied on branch",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(output, expected) {
			t.Errorf("showJobStatus() output missing expected content: %q\nFull output:\n%s", expected, output)
		}
	}
}
