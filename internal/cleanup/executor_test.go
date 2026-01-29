package cleanup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewExecutor(t *testing.T) {
	// Create a temporary directory with a git repo
	tmpDir, err := os.MkdirTemp("", "executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initialize git repo
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	job := NewJob(tmpDir)
	executor, err := NewExecutor(job)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}

	if executor == nil {
		t.Fatal("NewExecutor() returned nil")
	}
	if executor.job != job {
		t.Error("NewExecutor() did not set job correctly")
	}
}

func TestExecutor_Execute_EmptyJob(t *testing.T) {
	// Create a temporary directory with a git repo
	tmpDir, err := os.MkdirTemp("", "executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	// Create job with no resources to clean
	job := NewJob(tmpDir)
	job.CleanAll = true
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	executor, err := NewExecutor(job)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}

	// Execute should succeed with empty results
	if err := executor.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	// Reload job to check results
	loaded, err := LoadJob(tmpDir, job.ID)
	if err != nil {
		t.Fatalf("LoadJob() error = %v", err)
	}

	if loaded.Status != JobStatusCompleted {
		t.Errorf("Job status = %s, want %s", loaded.Status, JobStatusCompleted)
	}

	if loaded.Results == nil {
		t.Fatal("Job results should not be nil")
	}

	if loaded.Results.TotalRemoved != 0 {
		t.Errorf("TotalRemoved = %d, want 0", loaded.Results.TotalRemoved)
	}

	if !loaded.StartedAt.IsZero() {
		if loaded.StartedAt.After(loaded.EndedAt) {
			t.Error("StartedAt should be before EndedAt")
		}
	}
}

func TestExecutor_Execute_UpdatesJobStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	job := NewJob(tmpDir)
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	executor, err := NewExecutor(job)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}

	// Verify initial status
	if job.Status != JobStatusPending {
		t.Errorf("Initial status = %s, want %s", job.Status, JobStatusPending)
	}

	if err := executor.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	// Verify final status
	loaded, _ := LoadJob(tmpDir, job.ID)
	if loaded.Status != JobStatusCompleted {
		t.Errorf("Final status = %s, want %s", loaded.Status, JobStatusCompleted)
	}

	if loaded.EndedAt.IsZero() {
		t.Error("EndedAt should be set")
	}
}

func TestExecutor_Execute_NonexistentWorktree(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	// Create job with a non-existent worktree
	job := NewJob(tmpDir)
	job.Worktrees = true
	job.StaleWorktrees = []StaleWorktree{
		{
			Path:           filepath.Join(tmpDir, "nonexistent-worktree"),
			Branch:         "claudio/test-branch",
			HasUncommitted: false,
			ExistsOnRemote: false,
		},
	}
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	executor, err := NewExecutor(job)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}

	// Execute should succeed (non-existent resources are skipped)
	if err := executor.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	loaded, _ := LoadJob(tmpDir, job.ID)
	// Should complete successfully, just with 0 worktrees removed
	if loaded.Status != JobStatusCompleted {
		t.Errorf("Status = %s, want %s", loaded.Status, JobStatusCompleted)
	}
	if loaded.Results.WorktreesRemoved != 0 {
		t.Errorf("WorktreesRemoved = %d, want 0", loaded.Results.WorktreesRemoved)
	}
}

func TestExecutor_Execute_SkipsUncommittedWithoutForce(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	// Create a fake worktree directory
	wtDir := filepath.Join(tmpDir, "test-worktree")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatalf("Failed to create worktree dir: %v", err)
	}

	// Create job with uncommitted worktree and Force=false
	job := NewJob(tmpDir)
	job.Worktrees = true
	job.Force = false
	job.StaleWorktrees = []StaleWorktree{
		{
			Path:           wtDir,
			Branch:         "claudio/test-branch",
			HasUncommitted: true, // Has uncommitted changes
			ExistsOnRemote: false,
		},
	}
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	executor, err := NewExecutor(job)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}

	if err := executor.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	loaded, _ := LoadJob(tmpDir, job.ID)
	// Should have an error about skipping
	if len(loaded.Results.Errors) == 0 {
		t.Error("Expected error about skipping worktree with uncommitted changes")
	}
}

func TestListAllClaudioTmuxSessions(t *testing.T) {
	// This test just verifies the function doesn't panic
	// The actual result depends on whether tmux is running
	sessions := ListAllClaudioTmuxSessions()
	// sessions can be nil or empty, that's fine
	_ = sessions
}

func TestTmuxSessionExists(t *testing.T) {
	// Test with a session that definitely doesn't exist
	exists := tmuxSessionExists("nonexistent-session-12345678")
	if exists {
		t.Error("tmuxSessionExists() should return false for nonexistent session")
	}
}

func TestJob_StatusTransitions(t *testing.T) {
	tests := []struct {
		name          string
		initialStatus JobStatus
	}{
		{"from pending", JobStatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "executor-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			if err := initTestGitRepo(tmpDir); err != nil {
				t.Skipf("Skipping test - git not available: %v", err)
			}

			job := NewJob(tmpDir)
			job.Status = tt.initialStatus
			if err := job.Save(); err != nil {
				t.Fatalf("Failed to save job: %v", err)
			}

			executor, _ := NewExecutor(job)
			err = executor.Execute()

			loaded, _ := LoadJob(tmpDir, job.ID)

			if tt.initialStatus == JobStatusPending {
				if err != nil {
					t.Errorf("Execute() error = %v", err)
				}
				if loaded.Status != JobStatusCompleted {
					t.Errorf("Status = %s, want %s", loaded.Status, JobStatusCompleted)
				}
			}
		})
	}
}

func TestExecutor_RecordsExecutionTimes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	beforeExecute := time.Now()

	job := NewJob(tmpDir)
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	executor, _ := NewExecutor(job)
	if err := executor.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	afterExecute := time.Now()

	loaded, _ := LoadJob(tmpDir, job.ID)

	// StartedAt should be between before and after
	if loaded.StartedAt.Before(beforeExecute) || loaded.StartedAt.After(afterExecute) {
		t.Errorf("StartedAt %v is outside expected range [%v, %v]",
			loaded.StartedAt, beforeExecute, afterExecute)
	}

	// EndedAt should be after StartedAt
	if loaded.EndedAt.Before(loaded.StartedAt) {
		t.Error("EndedAt should be after StartedAt")
	}
}

func TestExecutor_cleanAllTmuxSessions_NonexistentSessions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	// Create job with --all-sessions targeting non-existent tmux sessions
	job := NewJob(tmpDir)
	job.AllSessions = true
	job.AllTmuxSessions = []string{
		"claudio-nonexistent-1",
		"claudio-nonexistent-2",
	}
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	executor, err := NewExecutor(job)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}

	// Execute should succeed (non-existent sessions are skipped gracefully)
	if err := executor.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	loaded, _ := LoadJob(tmpDir, job.ID)
	// Should complete successfully with 0 sessions killed (they don't exist)
	if loaded.Status != JobStatusCompleted {
		t.Errorf("Status = %s, want %s", loaded.Status, JobStatusCompleted)
	}
	if loaded.Results.TmuxSessionsKilled != 0 {
		t.Errorf("TmuxSessionsKilled = %d, want 0", loaded.Results.TmuxSessionsKilled)
	}
}

func TestExecutor_cleanAllSessions_SkipsNonexistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	// Create job with --deep-clean --all-sessions targeting non-existent sessions
	job := NewJob(tmpDir)
	job.DeepClean = true
	job.AllSessions = true
	job.AllSessionIDs = []StaleSession{
		{ID: "nonexistent-session-id-1", Name: "Test Session 1"},
		{ID: "nonexistent-session-id-2", Name: "Test Session 2"},
	}
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	executor, err := NewExecutor(job)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}

	// Execute should succeed (non-existent sessions are skipped gracefully)
	if err := executor.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	loaded, _ := LoadJob(tmpDir, job.ID)
	// Should complete successfully with 0 sessions removed (they don't exist)
	if loaded.Status != JobStatusCompleted {
		t.Errorf("Status = %s, want %s", loaded.Status, JobStatusCompleted)
	}
	if loaded.Results.SessionsRemoved != 0 {
		t.Errorf("SessionsRemoved = %d, want 0", loaded.Results.SessionsRemoved)
	}
}

func TestExecutor_cleanAllSessions_DeduplicatesWithEmptySessions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	// Create job where same session appears in both EmptySessions and AllSessionIDs
	job := NewJob(tmpDir)
	job.DeepClean = true
	job.AllSessions = true
	job.Sessions = true
	job.EmptySessions = []StaleSession{
		{ID: "shared-session-id", Name: "Shared Session"},
	}
	job.AllSessionIDs = []StaleSession{
		{ID: "shared-session-id", Name: "Shared Session"},   // Should be skipped (deduped)
		{ID: "another-session-id", Name: "Another Session"}, // Non-existent, will be skipped
	}
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	executor, err := NewExecutor(job)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}

	// Execute should succeed
	if err := executor.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	loaded, _ := LoadJob(tmpDir, job.ID)
	if loaded.Status != JobStatusCompleted {
		t.Errorf("Status = %s, want %s", loaded.Status, JobStatusCompleted)
	}
	// No errors about double deletion - deduplication should work
	for _, e := range loaded.Results.Errors {
		if e == "failed to remove session shared-se: already removed" {
			t.Error("Session was attempted to be removed twice")
		}
	}
}

func TestExecutor_killTmuxSessions_EmptyList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	job := NewJob(tmpDir)
	executor, err := NewExecutor(job)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}

	// Call with empty list
	killed, errors := executor.killTmuxSessions(nil)
	if killed != 0 {
		t.Errorf("killTmuxSessions(nil) killed = %d, want 0", killed)
	}
	if len(errors) != 0 {
		t.Errorf("killTmuxSessions(nil) errors = %v, want empty", errors)
	}

	// Call with empty slice
	killed, errors = executor.killTmuxSessions([]string{})
	if killed != 0 {
		t.Errorf("killTmuxSessions([]) killed = %d, want 0", killed)
	}
	if len(errors) != 0 {
		t.Errorf("killTmuxSessions([]) errors = %v, want empty", errors)
	}
}

func TestExecutor_removeSessions_EmptyList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	job := NewJob(tmpDir)
	executor, err := NewExecutor(job)
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}

	// Call with nil
	removed, errors := executor.removeSessions(nil, nil)
	if removed != 0 {
		t.Errorf("removeSessions(nil, nil) removed = %d, want 0", removed)
	}
	if len(errors) != 0 {
		t.Errorf("removeSessions(nil, nil) errors = %v, want empty", errors)
	}

	// Call with empty slice
	removed, errors = executor.removeSessions([]StaleSession{}, nil)
	if removed != 0 {
		t.Errorf("removeSessions([], nil) removed = %d, want 0", removed)
	}
	if len(errors) != 0 {
		t.Errorf("removeSessions([], nil) errors = %v, want empty", errors)
	}
}

// initTestGitRepo initializes a git repository for testing
func initTestGitRepo(dir string) error {
	// This is a simple helper that creates a minimal git repo
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		return err
	}

	// Create minimal git config
	configContent := `[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
`
	return os.WriteFile(filepath.Join(gitDir, "config"), []byte(configContent), 0644)
}
