package cleanup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewJob(t *testing.T) {
	baseDir := "/tmp/test"
	job := NewJob(baseDir)

	if job.ID == "" {
		t.Error("NewJob() should generate a non-empty ID")
	}

	if len(job.ID) != 8 {
		t.Errorf("NewJob() ID length = %d, want 8", len(job.ID))
	}

	if job.BaseDir != baseDir {
		t.Errorf("NewJob() BaseDir = %s, want %s", job.BaseDir, baseDir)
	}

	if job.Status != JobStatusPending {
		t.Errorf("NewJob() Status = %s, want %s", job.Status, JobStatusPending)
	}

	if job.CreatedAt.IsZero() {
		t.Error("NewJob() CreatedAt should not be zero")
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		id := generateID()
		if seen[id] {
			t.Errorf("generateID() produced duplicate ID: %s", id)
		}
		seen[id] = true
	}
}

func TestGenerateID_Format(t *testing.T) {
	id := generateID()

	// Should be 8 hex characters
	if len(id) != 8 {
		t.Errorf("generateID() length = %d, want 8", len(id))
	}

	// Should be valid hex
	for _, c := range id {
		isDigit := c >= '0' && c <= '9'
		isHexLower := c >= 'a' && c <= 'f'
		if !isDigit && !isHexLower {
			t.Errorf("generateID() contains invalid hex character: %c", c)
		}
	}
}

func TestJob_SaveAndLoad(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a job with test data
	job := NewJob(tmpDir)
	job.Force = true
	job.AllSessions = true
	job.StaleWorktrees = []StaleWorktree{
		{Path: "/path/to/wt1", Branch: "claudio/abc-feature", HasUncommitted: false, ExistsOnRemote: false},
		{Path: "/path/to/wt2", Branch: "claudio/def-bugfix", HasUncommitted: true, ExistsOnRemote: true},
	}
	job.StaleBranches = []string{"claudio/orphan1", "claudio/orphan2"}
	job.OrphanedTmuxSess = []string{"claudio-abc123", "claudio-def456"}
	job.EmptySessions = []StaleSession{
		{ID: "sess1", Name: "Session 1"},
		{ID: "sess2", Name: ""},
	}

	// Save the job
	if err := job.Save(); err != nil {
		t.Fatalf("Job.Save() error = %v", err)
	}

	// Verify job file exists
	jobPath := GetJobPath(tmpDir, job.ID)
	if _, err := os.Stat(jobPath); os.IsNotExist(err) {
		t.Errorf("Job file not created at %s", jobPath)
	}

	// Load the job
	loaded, err := LoadJob(tmpDir, job.ID)
	if err != nil {
		t.Fatalf("LoadJob() error = %v", err)
	}

	// Verify loaded job matches
	if loaded.ID != job.ID {
		t.Errorf("LoadJob() ID = %s, want %s", loaded.ID, job.ID)
	}
	if loaded.Force != job.Force {
		t.Errorf("LoadJob() Force = %v, want %v", loaded.Force, job.Force)
	}
	if loaded.AllSessions != job.AllSessions {
		t.Errorf("LoadJob() AllSessions = %v, want %v", loaded.AllSessions, job.AllSessions)
	}
	if len(loaded.StaleWorktrees) != len(job.StaleWorktrees) {
		t.Errorf("LoadJob() StaleWorktrees count = %d, want %d", len(loaded.StaleWorktrees), len(job.StaleWorktrees))
	}
	if len(loaded.StaleBranches) != len(job.StaleBranches) {
		t.Errorf("LoadJob() StaleBranches count = %d, want %d", len(loaded.StaleBranches), len(job.StaleBranches))
	}
}

func TestLoadJob_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	_, err = LoadJob(tmpDir, "nonexistent")
	if err == nil {
		t.Error("LoadJob() should return error for non-existent job")
	}
}

func TestListJobs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create multiple jobs
	job1 := NewJob(tmpDir)
	job1.Status = JobStatusCompleted
	if err := job1.Save(); err != nil {
		t.Fatalf("Failed to save job1: %v", err)
	}

	job2 := NewJob(tmpDir)
	job2.Status = JobStatusRunning
	if err := job2.Save(); err != nil {
		t.Fatalf("Failed to save job2: %v", err)
	}

	// List jobs
	jobs, err := ListJobs(tmpDir)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("ListJobs() returned %d jobs, want 2", len(jobs))
	}
}

func TestListJobs_EmptyDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	jobs, err := ListJobs(tmpDir)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}

	if len(jobs) != 0 {
		t.Errorf("ListJobs() returned %d jobs, want 0", len(jobs))
	}
}

func TestRemoveJobFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	job := NewJob(tmpDir)
	if err := job.Save(); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Verify file exists
	jobPath := GetJobPath(tmpDir, job.ID)
	if _, err := os.Stat(jobPath); os.IsNotExist(err) {
		t.Fatalf("Job file not created")
	}

	// Remove the job file
	if err := RemoveJobFile(tmpDir, job.ID); err != nil {
		t.Errorf("RemoveJobFile() error = %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(jobPath); !os.IsNotExist(err) {
		t.Error("RemoveJobFile() did not remove the file")
	}
}

func TestCleanupOldJobs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create an old completed job
	oldJob := NewJob(tmpDir)
	oldJob.Status = JobStatusCompleted
	oldJob.EndedAt = time.Now().Add(-48 * time.Hour)
	if err := oldJob.Save(); err != nil {
		t.Fatalf("Failed to save old job: %v", err)
	}

	// Create a recent completed job
	recentJob := NewJob(tmpDir)
	recentJob.Status = JobStatusCompleted
	recentJob.EndedAt = time.Now().Add(-1 * time.Hour)
	if err := recentJob.Save(); err != nil {
		t.Fatalf("Failed to save recent job: %v", err)
	}

	// Create a running job (should not be cleaned)
	runningJob := NewJob(tmpDir)
	runningJob.Status = JobStatusRunning
	if err := runningJob.Save(); err != nil {
		t.Fatalf("Failed to save running job: %v", err)
	}

	// Clean jobs older than 24 hours
	removed, err := CleanupOldJobs(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldJobs() error = %v", err)
	}

	if removed != 1 {
		t.Errorf("CleanupOldJobs() removed = %d, want 1", removed)
	}

	// Verify old job is gone
	if _, err := LoadJob(tmpDir, oldJob.ID); err == nil {
		t.Error("Old job should have been removed")
	}

	// Verify recent and running jobs still exist
	if _, err := LoadJob(tmpDir, recentJob.ID); err != nil {
		t.Error("Recent job should still exist")
	}
	if _, err := LoadJob(tmpDir, runningJob.ID); err != nil {
		t.Error("Running job should still exist")
	}
}

func TestGetJobsDir(t *testing.T) {
	baseDir := "/home/user/project"
	expected := filepath.Join(baseDir, ".claudio", JobsDir)
	result := GetJobsDir(baseDir)
	if result != expected {
		t.Errorf("GetJobsDir() = %s, want %s", result, expected)
	}
}

func TestGetJobPath(t *testing.T) {
	baseDir := "/home/user/project"
	jobID := "abc12345"
	expected := filepath.Join(baseDir, ".claudio", JobsDir, jobID+".json")
	result := GetJobPath(baseDir, jobID)
	if result != expected {
		t.Errorf("GetJobPath() = %s, want %s", result, expected)
	}
}

func TestJob_isFinished(t *testing.T) {
	tests := []struct {
		status   JobStatus
		expected bool
	}{
		{JobStatusPending, false},
		{JobStatusRunning, false},
		{JobStatusCompleted, true},
		{JobStatusFailed, true},
		{JobStatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			job := &Job{Status: tt.status}
			if got := job.isFinished(); got != tt.expected {
				t.Errorf("isFinished() = %v, want %v", got, tt.expected)
			}
		})
	}
}
