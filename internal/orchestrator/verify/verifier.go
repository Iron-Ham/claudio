// Package verify provides task verification logic for the orchestrator.
//
// This package encapsulates the verification of task completion, including
// checking for completion files and validating that expected commits were produced.
package verify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// TaskCompletionFileName is the sentinel file that tasks write when complete.
const TaskCompletionFileName = ".claudio-task-complete.json"

// RevisionCompletionFileName is the sentinel file that revision tasks write when complete.
const RevisionCompletionFileName = ".claudio-revision-complete.json"

// TaskCompletionResult represents the result of verifying a task's work.
type TaskCompletionResult struct {
	TaskID      string
	InstanceID  string
	Success     bool
	Error       string
	NeedsRetry  bool
	CommitCount int
}

// TaskCompletionFile represents the completion report written by a task.
type TaskCompletionFile struct {
	TaskID        string   `json:"task_id"`
	Status        string   `json:"status"` // "complete", "blocked", or "failed"
	Summary       string   `json:"summary"`
	FilesModified []string `json:"files_modified"`
	Notes         string   `json:"notes,omitempty"`
	Issues        []string `json:"issues,omitempty"`
	Suggestions   []string `json:"suggestions,omitempty"`
	Dependencies  []string `json:"dependencies,omitempty"`
}

// RevisionCompletionFile represents the completion report from a revision task.
type RevisionCompletionFile struct {
	TaskID          string   `json:"task_id"`
	RevisionRound   int      `json:"revision_round"`
	IssuesAddressed []string `json:"issues_addressed"`
	Summary         string   `json:"summary"`
	FilesModified   []string `json:"files_modified"`
	RemainingIssues []string `json:"remaining_issues"`
}

// WorktreeOperations defines the git operations needed for verification.
type WorktreeOperations interface {
	// CountCommitsBetween returns the number of commits between base and head branches.
	CountCommitsBetween(worktreePath, baseBranch, headBranch string) (int, error)

	// FindMainBranch returns the name of the main/master branch.
	FindMainBranch() string
}

// RetryTracker tracks retry state for tasks.
type RetryTracker interface {
	// GetRetryCount returns the current retry count for a task.
	GetRetryCount(taskID string) int

	// IncrementRetry increments the retry count and returns the new value.
	IncrementRetry(taskID string) int

	// RecordCommitCount records the commit count for a retry attempt.
	RecordCommitCount(taskID string, count int)

	// GetMaxRetries returns the maximum retry count for a task.
	GetMaxRetries(taskID string) int
}

// EventEmitter emits verification events.
type EventEmitter interface {
	// EmitWarning emits a warning event.
	EmitWarning(taskID, message string)

	// EmitRetry emits a retry notification event.
	EmitRetry(taskID string, attempt, maxRetries int, reason string)

	// EmitFailure emits a failure event.
	EmitFailure(taskID, reason string)
}

// Config holds configuration for task verification.
type Config struct {
	// RequireVerifiedCommits determines whether to verify commits were produced.
	RequireVerifiedCommits bool

	// MaxTaskRetries is the default maximum retry count if not set.
	MaxTaskRetries int
}

// DefaultConfig returns sensible defaults for verification configuration.
func DefaultConfig() Config {
	return Config{
		RequireVerifiedCommits: false,
		MaxTaskRetries:         3,
	}
}

// TaskVerifier verifies that tasks have completed their work correctly.
type TaskVerifier struct {
	wt           WorktreeOperations
	retryTracker RetryTracker
	events       EventEmitter
	config       Config
	logger       *logging.Logger
}

// Option is a functional option for configuring TaskVerifier.
type Option func(*TaskVerifier)

// WithLogger sets the logger for the verifier.
func WithLogger(logger *logging.Logger) Option {
	return func(v *TaskVerifier) {
		v.logger = logger
	}
}

// WithConfig sets the configuration for the verifier.
func WithConfig(cfg Config) Option {
	return func(v *TaskVerifier) {
		v.config = cfg
	}
}

// NewTaskVerifier creates a new TaskVerifier with the given dependencies.
// All dependencies (wt, retryTracker, events) must be non-nil.
func NewTaskVerifier(wt WorktreeOperations, retryTracker RetryTracker, events EventEmitter, opts ...Option) *TaskVerifier {
	if wt == nil {
		panic("verify.NewTaskVerifier: wt must not be nil")
	}
	if retryTracker == nil {
		panic("verify.NewTaskVerifier: retryTracker must not be nil")
	}
	if events == nil {
		panic("verify.NewTaskVerifier: events must not be nil")
	}
	v := &TaskVerifier{
		wt:           wt,
		retryTracker: retryTracker,
		events:       events,
		config:       DefaultConfig(),
		logger:       logging.NopLogger(),
	}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

// CheckCompletionFile checks if the task has written its completion sentinel file.
// This checks for both regular task completion (.claudio-task-complete.json) and
// revision task completion (.claudio-revision-complete.json) since both use this monitor.
func (v *TaskVerifier) CheckCompletionFile(worktreePath string) (bool, error) {
	if worktreePath == "" {
		return false, nil
	}

	// First check for regular task completion file
	taskCompletionPath := filepath.Join(worktreePath, TaskCompletionFileName)
	if _, err := os.Stat(taskCompletionPath); err == nil {
		// File exists - try to parse it to ensure it's valid
		completion, err := v.ParseTaskCompletionFile(worktreePath)
		if err == nil && completion.Status != "" {
			return true, nil
		}
	}

	// Also check for revision completion file (revision tasks write this instead)
	revisionCompletionPath := filepath.Join(worktreePath, RevisionCompletionFileName)
	if _, err := os.Stat(revisionCompletionPath); err == nil {
		// File exists - try to parse it to ensure it's valid
		completion, err := v.ParseRevisionCompletionFile(worktreePath)
		if err == nil && completion.TaskID != "" {
			return true, nil
		}
	}

	return false, nil
}

// ParseTaskCompletionFile reads and parses a task completion file.
func (v *TaskVerifier) ParseTaskCompletionFile(worktreePath string) (*TaskCompletionFile, error) {
	completionPath := filepath.Join(worktreePath, TaskCompletionFileName)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion TaskCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse task completion JSON: %w", err)
	}

	return &completion, nil
}

// ParseRevisionCompletionFile reads and parses a revision completion file.
func (v *TaskVerifier) ParseRevisionCompletionFile(worktreePath string) (*RevisionCompletionFile, error) {
	completionPath := filepath.Join(worktreePath, RevisionCompletionFileName)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion RevisionCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse revision completion JSON: %w", err)
	}

	return &completion, nil
}

// VerifyTaskWork checks if a task produced actual commits and determines success/retry.
func (v *TaskVerifier) VerifyTaskWork(taskID, instanceID, worktreePath, baseBranch string) TaskCompletionResult {
	result := TaskCompletionResult{
		TaskID:     taskID,
		InstanceID: instanceID,
		Success:    true,
	}

	// Skip verification if not required
	if !v.config.RequireVerifiedCommits {
		return result
	}

	// Determine the base branch if not provided
	if baseBranch == "" {
		baseBranch = v.wt.FindMainBranch()
	}

	// Count commits on the task branch beyond the base
	commitCount, err := v.wt.CountCommitsBetween(worktreePath, baseBranch, "HEAD")
	if err != nil {
		// If we can't count commits, log warning but don't fail
		v.events.EmitWarning(taskID, fmt.Sprintf("Warning: could not verify commits for task %s: %v", taskID, err))
		return result
	}

	result.CommitCount = commitCount

	// Check if task produced any commits
	if commitCount == 0 {
		// No commits - check retry status
		maxRetries := v.retryTracker.GetMaxRetries(taskID)
		if maxRetries == 0 {
			maxRetries = v.config.MaxTaskRetries
		}

		currentRetries := v.retryTracker.GetRetryCount(taskID)
		v.retryTracker.RecordCommitCount(taskID, 0)

		if currentRetries < maxRetries {
			// Trigger retry
			newRetryCount := v.retryTracker.IncrementRetry(taskID)

			result.Success = false
			result.NeedsRetry = true
			result.Error = "no_commits_retry"

			v.events.EmitRetry(taskID, newRetryCount, maxRetries, "task produced no commits")
		} else {
			// Max retries exhausted
			result.Success = false
			result.NeedsRetry = false
			result.Error = fmt.Sprintf("task produced no commits after %d attempts", maxRetries)

			v.events.EmitFailure(taskID, fmt.Sprintf("Task %s failed: no commits after %d retry attempts", taskID, maxRetries))
		}
	}

	return result
}

// TaskCompletionFilePath returns the full path to the task completion file for a given worktree.
func TaskCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, TaskCompletionFileName)
}

// RevisionCompletionFilePath returns the full path to the revision completion file for a given worktree.
func RevisionCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, RevisionCompletionFileName)
}
