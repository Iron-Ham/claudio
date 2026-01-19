// Package consolidation provides the consolidation orchestration for combining
// completed task branches into pull requests. It coordinates branch operations,
// PR content generation, and conflict handling through a modular architecture.
package consolidation

import (
	"context"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
)

// Mode defines how work is consolidated after ultraplan execution.
type Mode string

const (
	// ModeStacked creates one PR per execution group, stacked on each other.
	ModeStacked Mode = "stacked"
	// ModeSingle consolidates all work into a single PR.
	ModeSingle Mode = "single"
)

// Phase represents sub-phases within consolidation.
type Phase string

const (
	PhaseIdle             Phase = "idle"
	PhaseDetecting        Phase = "detecting_conflicts"
	PhaseCreatingBranches Phase = "creating_branches"
	PhaseMergingTasks     Phase = "merging_tasks"
	PhasePushing          Phase = "pushing"
	PhaseCreatingPRs      Phase = "creating_prs"
	PhasePaused           Phase = "paused"
	PhaseComplete         Phase = "complete"
	PhaseFailed           Phase = "failed"
)

// State tracks the progress of consolidation.
type State struct {
	Phase            Phase      `json:"phase"`
	CurrentGroup     int        `json:"current_group"`
	TotalGroups      int        `json:"total_groups"`
	CurrentTask      string     `json:"current_task,omitempty"`
	GroupBranches    []string   `json:"group_branches"`
	PRUrls           []string   `json:"pr_urls"`
	ConflictFiles    []string   `json:"conflict_files,omitempty"`
	ConflictTaskID   string     `json:"conflict_task_id,omitempty"`
	ConflictWorktree string     `json:"conflict_worktree,omitempty"`
	Error            string     `json:"error,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

// HasConflict returns true if consolidation is paused due to a conflict.
func (s *State) HasConflict() bool {
	return s.Phase == PhasePaused && len(s.ConflictFiles) > 0
}

// GroupResult holds the result of consolidating one group.
type GroupResult struct {
	GroupIndex   int      `json:"group_index"`
	TaskIDs      []string `json:"task_ids"`
	BranchName   string   `json:"branch_name"`
	CommitCount  int      `json:"commit_count"`
	FilesChanged []string `json:"files_changed"`
	PRUrl        string   `json:"pr_url,omitempty"`
	Success      bool     `json:"success"`
	Error        string   `json:"error,omitempty"`
}

// Config holds configuration for branch consolidation.
type Config struct {
	Mode           Mode
	BranchPrefix   string
	CreateDraftPRs bool
	PRLabels       []string
}

// EventType represents events during consolidation.
type EventType string

const (
	EventStarted       EventType = "consolidation_started"
	EventGroupStarted  EventType = "consolidation_group_started"
	EventTaskMerging   EventType = "consolidation_task_merging"
	EventTaskMerged    EventType = "consolidation_task_merged"
	EventGroupComplete EventType = "consolidation_group_complete"
	EventPRCreating    EventType = "consolidation_pr_creating"
	EventPRCreated     EventType = "consolidation_pr_created"
	EventConflict      EventType = "consolidation_conflict"
	EventComplete      EventType = "consolidation_complete"
	EventFailed        EventType = "consolidation_failed"
)

// Event represents an event during consolidation.
type Event struct {
	Type      EventType `json:"type"`
	GroupIdx  int       `json:"group_idx,omitempty"`
	TaskID    string    `json:"task_id,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// PRContent holds PR title and body.
type PRContent struct {
	Title      string
	Body       string
	Labels     []string
	Assignees  []string
	BaseBranch string
	HeadBranch string
}

// CompletedTask represents a task that has been completed and is ready for consolidation.
type CompletedTask struct {
	ID           string
	Title        string
	Branch       string
	WorktreePath string
	Files        []string
	Description  string
	Completion   *types.TaskCompletionFile
}

// ConflictInfo holds information about a merge conflict.
type ConflictInfo struct {
	TaskID       string   `json:"task_id"`
	TaskTitle    string   `json:"task_title,omitempty"`
	Branch       string   `json:"branch"`
	Files        []string `json:"files"`
	WorktreePath string   `json:"worktree_path"`
}

// Result holds the overall result of a consolidation run.
type Result struct {
	PRs            []PRInfo
	MergedBranches []string
	Conflicts      []ConflictInfo
	Duration       time.Duration
}

// PRInfo holds information about a created PR.
type PRInfo struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	GroupIndex int    `json:"group_index"`
}

// TaskGroup represents a group of tasks to be consolidated together.
// Tasks within a group are executed together and may be combined into
// a single PR (single mode) or stacked PRs (stacked mode).
type TaskGroup struct {
	Index int
	Tasks []CompletedTask
}

// StrategyResult holds the result of a consolidation strategy execution.
type StrategyResult struct {
	PRs          []PRInfo
	GroupResults []GroupResult
	TotalCommits int
	FilesChanged []string
	Duration     time.Duration
	Error        error
}

// Strategy defines how tasks are consolidated into PRs.
type Strategy interface {
	// Execute runs the consolidation strategy on the given task groups.
	Execute(ctx context.Context, groups []TaskGroup) (*StrategyResult, error)

	// Name returns the strategy name for logging.
	Name() string

	// SupportsParallel indicates if tasks can be processed in parallel.
	SupportsParallel() bool
}

// PRBuilder generates PR content from completed tasks.
type PRBuilder interface {
	// Build generates PR content from the given tasks.
	Build(tasks []CompletedTask, opts PRBuildOptions) (*PRContent, error)

	// BuildTitle generates a PR title from tasks.
	BuildTitle(tasks []CompletedTask, opts PRBuildOptions) string

	// BuildBody generates a PR body from tasks.
	BuildBody(tasks []CompletedTask, opts PRBuildOptions) string

	// BuildLabels determines appropriate labels for the PR.
	BuildLabels(tasks []CompletedTask, opts PRBuildOptions) []string
}

// PRBuildOptions provides options for building PR content.
type PRBuildOptions struct {
	Mode            Mode
	GroupIndex      int
	TotalGroups     int
	Objective       string
	SynthesisNotes  string
	Recommendations []string
	FilesChanged    []string
	BaseBranch      string
	HeadBranch      string
}

// BranchManager handles git branch operations for consolidation.
type BranchManager interface {
	// FindMainBranch returns the name of the main branch (main or master).
	FindMainBranch(ctx context.Context) string

	// CreateGroupBranch creates a branch for a consolidation group.
	CreateGroupBranch(ctx context.Context, groupIdx int, baseBranch string) (string, error)

	// CreateSingleBranch creates a branch for single PR mode consolidation.
	CreateSingleBranch(ctx context.Context, baseBranch string) (string, error)

	// CreateWorktree creates a worktree for the given branch.
	CreateWorktree(ctx context.Context, path, branch string) error

	// CherryPickBranch cherry-picks commits from source branch into the worktree.
	CherryPickBranch(ctx context.Context, worktreePath, sourceBranch string) error

	// GetChangedFiles returns files changed in the worktree compared to main.
	GetChangedFiles(ctx context.Context, worktreePath string) ([]string, error)

	// GetConflictingFiles returns files with merge conflicts.
	GetConflictingFiles(ctx context.Context, worktreePath string) ([]string, error)

	// CountCommitsBetween returns the number of commits between base and head.
	CountCommitsBetween(ctx context.Context, worktreePath, baseBranch, headBranch string) (int, error)

	// Push pushes the branch to the remote.
	Push(ctx context.Context, worktreePath string, force bool) error

	// RemoveWorktree removes a worktree.
	RemoveWorktree(ctx context.Context, path string) error

	// DeleteBranch deletes a branch.
	DeleteBranch(ctx context.Context, branch string) error

	// ContinueCherryPick continues a cherry-pick after conflict resolution.
	ContinueCherryPick(ctx context.Context, worktreePath string) error

	// AbortCherryPick aborts an in-progress cherry-pick.
	AbortCherryPick(ctx context.Context, worktreePath string) error
}

// ConflictManager handles merge conflict detection and resolution.
type ConflictManager interface {
	// CheckCherryPick attempts a cherry-pick and reports any conflicts.
	// Returns nil if the cherry-pick succeeds, or a ConflictInfo if conflicts occur.
	CheckCherryPick(ctx context.Context, worktreePath, sourceBranch, taskID, taskTitle string) (*ConflictInfo, error)

	// AbortCherryPick aborts an in-progress cherry-pick operation.
	AbortCherryPick(ctx context.Context, worktreePath string) error

	// ContinueCherryPick continues a cherry-pick after conflicts have been resolved.
	ContinueCherryPick(ctx context.Context, worktreePath string) error

	// GetConflictingFiles returns the files with conflicts in the given worktree.
	GetConflictingFiles(ctx context.Context, worktreePath string) ([]string, error)
}

// EventEmitter notifies listeners of consolidation progress.
type EventEmitter interface {
	// Emit sends an event to all listeners.
	Emit(event Event)
}

// Logger provides structured logging for consolidation operations.
type Logger interface {
	Debug(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}

// GitExecutor executes git commands.
type GitExecutor interface {
	// Run executes a git command in the repository root.
	Run(ctx context.Context, args ...string) (string, error)

	// RunInDir executes a git command in the specified directory.
	RunInDir(ctx context.Context, dir string, args ...string) (string, error)
}
