// Package cleanup provides background cleanup job functionality for removing
// stale worktrees, branches, tmux sessions, and empty sessions.
package cleanup

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// JobsDir is the directory name within .claudio that contains cleanup job files
const JobsDir = "cleanup-jobs"

// JobStatus represents the current state of a cleanup job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// StaleWorktree represents a worktree that was marked for cleanup at snapshot time
type StaleWorktree struct {
	Path           string `json:"path"`
	Branch         string `json:"branch"`
	HasUncommitted bool   `json:"has_uncommitted"`
	ExistsOnRemote bool   `json:"exists_on_remote"`
}

// StaleSession represents a session marked for cleanup at snapshot time
type StaleSession struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Job represents a cleanup job with its snapshotted resources
type Job struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	StartedAt time.Time `json:"started_at,omitzero"`
	EndedAt   time.Time `json:"ended_at,omitzero"`
	Status    JobStatus `json:"status"`

	// Configuration
	BaseDir     string `json:"base_dir"`
	Force       bool   `json:"force"`
	AllSessions bool   `json:"all_sessions"`
	DeepClean   bool   `json:"deep_clean"`
	CleanAll    bool   `json:"clean_all"`
	Worktrees   bool   `json:"worktrees"`
	Branches    bool   `json:"branches"`
	Tmux        bool   `json:"tmux"`
	Sessions    bool   `json:"sessions"`

	// Snapshotted resources (captured at job creation time)
	StaleWorktrees   []StaleWorktree `json:"stale_worktrees"`
	StaleBranches    []string        `json:"stale_branches"`
	OrphanedTmuxSess []string        `json:"orphaned_tmux_sessions"`
	EmptySessions    []StaleSession  `json:"empty_sessions"`
	AllTmuxSessions  []string        `json:"all_tmux_sessions,omitempty"` // For --all-sessions
	AllSessionIDs    []StaleSession  `json:"all_session_ids,omitempty"`   // For --deep-clean --all-sessions

	// Results
	Results *JobResults `json:"results,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// JobResults contains the outcome of a cleanup job
type JobResults struct {
	WorktreesRemoved   int      `json:"worktrees_removed"`
	BranchesDeleted    int      `json:"branches_deleted"`
	TmuxSessionsKilled int      `json:"tmux_sessions_killed"`
	SessionsRemoved    int      `json:"sessions_removed"`
	TotalRemoved       int      `json:"total_removed"`
	Errors             []string `json:"errors,omitempty"`
}

// NewJob creates a new cleanup job with a unique ID
func NewJob(baseDir string) *Job {
	return &Job{
		ID:        generateID(),
		CreatedAt: time.Now(),
		Status:    JobStatusPending,
		BaseDir:   baseDir,
	}
}

// generateID creates a short random hex ID.
// Falls back to timestamp-based ID if random generation fails.
func generateID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID on entropy failure
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
	}
	return hex.EncodeToString(b)
}

// GetJobsDir returns the path to the cleanup jobs directory
func GetJobsDir(baseDir string) string {
	return filepath.Join(baseDir, ".claudio", JobsDir)
}

// GetJobPath returns the path to a specific job file
func GetJobPath(baseDir, jobID string) string {
	return filepath.Join(GetJobsDir(baseDir), jobID+".json")
}

// Save persists the job to disk
func (j *Job) Save() error {
	jobsDir := GetJobsDir(j.BaseDir)
	if err := os.MkdirAll(jobsDir, 0755); err != nil {
		return fmt.Errorf("failed to create jobs directory: %w", err)
	}

	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	jobPath := GetJobPath(j.BaseDir, j.ID)
	if err := os.WriteFile(jobPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write job file: %w", err)
	}

	return nil
}

// LoadJob reads a job from disk
func LoadJob(baseDir, jobID string) (*Job, error) {
	jobPath := GetJobPath(baseDir, jobID)
	data, err := os.ReadFile(jobPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read job file: %w", err)
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("failed to parse job file: %w", err)
	}

	return &job, nil
}

// ListJobs returns all cleanup jobs
func ListJobs(baseDir string) ([]*Job, error) {
	jobsDir := GetJobsDir(baseDir)
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var jobs []*Job
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		jobID := entry.Name()[:len(entry.Name())-5] // Remove .json
		job, err := LoadJob(baseDir, jobID)
		if err != nil {
			continue // Skip invalid job files
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// RemoveJobFile removes the job file from disk
func RemoveJobFile(baseDir, jobID string) error {
	jobPath := GetJobPath(baseDir, jobID)
	return os.Remove(jobPath)
}

// CleanupOldJobs removes completed/failed job files older than the given duration
func CleanupOldJobs(baseDir string, maxAge time.Duration) (int, error) {
	jobs, err := ListJobs(baseDir)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, job := range jobs {
		if !job.isFinished() {
			continue
		}

		endTime := job.EndedAt
		if endTime.IsZero() {
			endTime = job.CreatedAt
		}

		if endTime.Before(cutoff) {
			if err := RemoveJobFile(baseDir, job.ID); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// isFinished returns true if the job has reached a terminal state
func (j *Job) isFinished() bool {
	switch j.Status {
	case JobStatusCompleted, JobStatusFailed, JobStatusCancelled:
		return true
	default:
		return false
	}
}
