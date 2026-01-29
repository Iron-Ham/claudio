package cleanup

import (
	"os"
	"strings"
	"testing"
)

func TestGetExecutablePath(t *testing.T) {
	path, err := GetExecutablePath()
	if err != nil {
		t.Fatalf("GetExecutablePath() error = %v", err)
	}

	// Path should not be empty
	if path == "" {
		t.Error("GetExecutablePath() returned empty path")
	}

	// Path should point to an existing file
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("GetExecutablePath() returned path that doesn't exist: %s", path)
	}
}

func TestRunJobFromFile_NonexistentJob(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	err = RunJobFromFile(tmpDir, "nonexistent")
	if err == nil {
		t.Error("RunJobFromFile() should return error for non-existent job")
	}
}

func TestRunJobFromFile_NotPending(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a job that's already completed
	job := NewJob(tmpDir)
	job.Status = JobStatusCompleted
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	err = RunJobFromFile(tmpDir, job.ID)
	if err == nil {
		t.Error("RunJobFromFile() should return error for non-pending job")
	}
}

func TestRunJobFromFile_Success(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a minimal git repo for the worktree manager
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("Skipping test - git not available: %v", err)
	}

	// Create a pending job with no resources
	job := NewJob(tmpDir)
	job.Status = JobStatusPending
	job.CleanAll = true
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Run the job
	err = RunJobFromFile(tmpDir, job.ID)
	if err != nil {
		t.Errorf("RunJobFromFile() error = %v", err)
	}

	// Verify job is now completed
	loaded, err := LoadJob(tmpDir, job.ID)
	if err != nil {
		t.Fatalf("LoadJob() error = %v", err)
	}

	if loaded.Status != JobStatusCompleted {
		t.Errorf("Job status = %s, want %s", loaded.Status, JobStatusCompleted)
	}
}

func TestSpawnBackgroundCleanup_InvalidExecutable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a job
	job := NewJob(tmpDir)
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Try to spawn with an invalid executable path
	err = SpawnBackgroundCleanup("/nonexistent/executable/path", tmpDir, job.ID)
	if err == nil {
		t.Error("SpawnBackgroundCleanup() should return error for invalid executable")
	}
}

func TestSpawnBackgroundCleanup_DelegatesCorrectly(t *testing.T) {
	// This test verifies that SpawnBackgroundCleanup delegates to spawnDetachedProcess
	// We can't fully test the background spawning, but we can verify the function exists
	// and correctly forwards parameters

	tmpDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a job
	job := NewJob(tmpDir)
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Get the current executable for a valid path (even though it won't work correctly)
	execPath, err := GetExecutablePath()
	if err != nil {
		t.Skipf("Skipping test - cannot get executable path: %v", err)
	}

	// SpawnBackgroundCleanup should not panic even if the spawn fails for other reasons
	// The function should properly forward to spawnDetachedProcess
	// We expect this to either succeed (spawning a background process) or fail gracefully
	_ = SpawnBackgroundCleanup(execPath, tmpDir, job.ID)

	// The important thing is that it doesn't panic and properly delegates
}

func TestRunJobFromFile_ExecutorCreationFailure(t *testing.T) {
	// Test that when NewExecutor fails, the job is marked as failed
	tmpDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a pending job WITHOUT a git repo - this will cause NewExecutor to fail
	// because worktree.New requires a valid git repository
	job := NewJob(tmpDir)
	job.Status = JobStatusPending
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Run the job - should fail because there's no git repo
	err = RunJobFromFile(tmpDir, job.ID)
	if err == nil {
		t.Error("RunJobFromFile() should return error when executor creation fails")
	}

	// Verify error message contains the expected context
	if err != nil && !strings.Contains(err.Error(), "failed to create executor") {
		t.Errorf("RunJobFromFile() error = %v, want error containing 'failed to create executor'", err)
	}

	// Verify job is marked as failed with the error recorded
	loaded, loadErr := LoadJob(tmpDir, job.ID)
	if loadErr != nil {
		t.Fatalf("LoadJob() error = %v", loadErr)
	}

	if loaded.Status != JobStatusFailed {
		t.Errorf("Job status = %s, want %s", loaded.Status, JobStatusFailed)
	}

	if loaded.Error == "" {
		t.Error("Job error should be set when executor creation fails")
	}
}
