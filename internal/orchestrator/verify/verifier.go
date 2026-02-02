// Package verify provides task verification logic for the orchestrator.
//
// This package encapsulates the verification of task completion, including
// checking for completion files and validating that expected commits were produced.
package verify

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
	"github.com/Iron-Ham/claudio/internal/worktree"
)

// TaskCompletionFileName is the sentinel file that tasks write when complete.
const TaskCompletionFileName = types.TaskCompletionFileName

// RevisionCompletionFileName is the sentinel file that revision tasks write when complete.
const RevisionCompletionFileName = ".claudio-revision-complete.json"

// maxSearchDepth is the maximum directory depth to search for completion files.
// This prevents excessive traversal in deeply nested directory structures.
const maxSearchDepth = 5

// skippedDirectories lists directories to skip during recursive search.
// These are typically large or irrelevant directories that slow down the search.
var skippedDirectories = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".claudio":     true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	"build":        true,
	"dist":         true,
	".next":        true,
	".nuxt":        true,
	"target":       true, // Rust/Java build output
	"Pods":         true, // iOS CocoaPods
	".build":       true, // Swift Package Manager
}

// TaskCompletionResult represents the result of verifying a task's work.
type TaskCompletionResult struct {
	TaskID      string
	InstanceID  string
	Success     bool
	Error       string
	NeedsRetry  bool
	CommitCount int
}

// TaskVerifyOptions provides additional context for task verification.
type TaskVerifyOptions struct {
	// NoCode indicates the task doesn't require code changes.
	// When true, the task succeeds even without commits.
	NoCode bool
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
//
// The search first checks the worktree root (fast path), then falls back to a
// recursive search of subdirectories to handle cases where the backend may have changed
// directories during task execution.
func (v *TaskVerifier) CheckCompletionFile(worktreePath string) (bool, error) {
	if worktreePath == "" {
		return false, nil
	}

	// First check for regular task completion file (with subdirectory fallback)
	taskCompletionPath := v.findCompletionFile(worktreePath, TaskCompletionFileName)
	if taskCompletionPath != "" {
		// File found - try to parse it to ensure it's valid
		completion, err := v.parseTaskCompletionFileAtPath(taskCompletionPath)
		if err == nil && completion.Status != "" {
			return true, nil
		}
	}

	// Also check for revision completion file (revision tasks write this instead)
	revisionCompletionPath := v.findCompletionFile(worktreePath, RevisionCompletionFileName)
	if revisionCompletionPath != "" {
		// File found - try to parse it to ensure it's valid
		completion, err := v.parseRevisionCompletionFileAtPath(revisionCompletionPath)
		if err == nil && completion.TaskID != "" {
			return true, nil
		}
	}

	return false, nil
}

// findCompletionFile searches for a completion file in the worktree.
// It first checks the root directory (fast path), then falls back to a recursive
// search with depth limiting and directory skipping for performance.
func (v *TaskVerifier) findCompletionFile(worktreePath, filename string) string {
	// Fast path: check root first
	rootPath := filepath.Join(worktreePath, filename)
	if _, err := os.Stat(rootPath); err == nil {
		return rootPath
	}

	// Fallback: search subdirectories with depth limit
	var found string
	_ = filepath.WalkDir(worktreePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors (permission denied, etc.)
		}

		// Stop if we already found the file
		if found != "" {
			return fs.SkipAll
		}

		// Check depth relative to worktree root
		rel, relErr := filepath.Rel(worktreePath, path)
		if relErr != nil {
			return nil
		}
		depth := strings.Count(rel, string(filepath.Separator))
		if depth > maxSearchDepth {
			return fs.SkipDir
		}

		// Skip known large/irrelevant directories
		if d.IsDir() && skippedDirectories[d.Name()] {
			return fs.SkipDir
		}

		// Skip git submodule directories to avoid errors when traversing
		// uninitialized or partially initialized submodules
		if d.IsDir() && worktree.IsSubmoduleDir(path) {
			return fs.SkipDir
		}

		// Check if this is the file we're looking for
		if !d.IsDir() && d.Name() == filename {
			found = path
			return fs.SkipAll
		}

		return nil
	})

	return found
}

// ParseTaskCompletionFile reads and parses a task completion file from the worktree root.
// Use FindAndParseTaskCompletionFile for recursive search when the file location is unknown.
func (v *TaskVerifier) ParseTaskCompletionFile(worktreePath string) (*types.TaskCompletionFile, error) {
	completionPath := filepath.Join(worktreePath, TaskCompletionFileName)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion types.TaskCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse task completion JSON: %w", err)
	}

	return &completion, nil
}

// FindAndParseTaskCompletionFile searches for and parses a task completion file.
// Unlike ParseTaskCompletionFile, this uses recursive search to find the file
// in subdirectories if not found at the root. This handles cases where the backend
// changed directories during task execution.
func (v *TaskVerifier) FindAndParseTaskCompletionFile(worktreePath string) (*types.TaskCompletionFile, error) {
	completionPath := v.findCompletionFile(worktreePath, TaskCompletionFileName)
	if completionPath == "" {
		return nil, os.ErrNotExist
	}
	return v.parseTaskCompletionFileAtPath(completionPath)
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

// parseTaskCompletionFileAtPath reads and parses a task completion file at the given path.
// Unlike ParseTaskCompletionFile, this takes a full file path rather than a worktree path.
func (v *TaskVerifier) parseTaskCompletionFileAtPath(filePath string) (*types.TaskCompletionFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var completion types.TaskCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse task completion JSON: %w", err)
	}

	return &completion, nil
}

// parseRevisionCompletionFileAtPath reads and parses a revision completion file at the given path.
// Unlike ParseRevisionCompletionFile, this takes a full file path rather than a worktree path.
func (v *TaskVerifier) parseRevisionCompletionFileAtPath(filePath string) (*RevisionCompletionFile, error) {
	data, err := os.ReadFile(filePath)
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
// The opts parameter provides task-specific context (e.g., NoCode flag for verification tasks).
func (v *TaskVerifier) VerifyTaskWork(taskID, instanceID, worktreePath, baseBranch string, opts *TaskVerifyOptions) TaskCompletionResult {
	result := TaskCompletionResult{
		TaskID:     taskID,
		InstanceID: instanceID,
		Success:    true,
	}

	// Skip verification if not required
	if !v.config.RequireVerifiedCommits {
		return result
	}

	// Skip commit verification for no-code tasks (verification, testing, documentation-only)
	if opts != nil && opts.NoCode {
		v.logger.Debug("skipping commit verification for no-code task", "task_id", taskID)
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
		// Before failing, check if task wrote a completion file with status="complete"
		// This allows tasks to explicitly signal success without code changes.
		// Use FindAndParseTaskCompletionFile to search subdirectories in case the backend
		// changed directories during task execution.
		completion, parseErr := v.FindAndParseTaskCompletionFile(worktreePath)
		if parseErr == nil {
			if completion.Status == "complete" {
				v.logger.Debug("task has no commits but completion file indicates success",
					"task_id", taskID,
					"summary", completion.Summary)
				return result // Success - completion file overrides commit requirement
			}
		} else if !os.IsNotExist(parseErr) {
			// Log if file exists but couldn't be parsed (likely corruption or bug)
			v.logger.Warn("failed to parse task completion file",
				"task_id", taskID,
				"error", parseErr)
		}

		// No commits and no completion file override - check retry status
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
