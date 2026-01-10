// Package consolidation provides types and interfaces for consolidating
// task branches into final deliverable branches and pull requests.
//
// The consolidation phase is responsible for taking completed task work
// from individual worktrees and combining them into cohesive branches
// suitable for PR creation.
package consolidation

import "time"

// StrategyType identifies different consolidation strategies.
type StrategyType string

const (
	// StrategyStacked creates one PR per execution group, stacked on each other.
	// Each group's PR is based on the previous group's branch, creating a
	// linear chain of dependent PRs.
	StrategyStacked StrategyType = "stacked"

	// StrategySingle consolidates all work from all groups into a single PR.
	// All task branches are merged into one consolidated branch.
	StrategySingle StrategyType = "single"

	// StrategySquash consolidates all commits from each task into a single
	// commit per task, then combines them. Useful for cleaner history.
	StrategySquash StrategyType = "squash"

	// StrategyRebase rebases task branches in dependency order rather than
	// cherry-picking. Preserves more git history context.
	StrategyRebase StrategyType = "rebase"
)

// Strategy defines the interface for consolidation strategies.
// Each strategy implements a different approach to combining task branches
// into deliverable branches.
type Strategy interface {
	// Name returns the unique identifier for this strategy.
	Name() string

	// Consolidate performs the consolidation of task groups according to
	// this strategy's approach. It takes the groups to consolidate and
	// configuration options, returning the result of the consolidation.
	Consolidate(groups []TaskGroup, config Config) (Result, error)

	// CanHandle returns true if this strategy can handle the given groups.
	// This enables automatic strategy selection based on group characteristics.
	// For example, a stacked strategy might require ordered dependencies,
	// while a single strategy can handle any configuration.
	CanHandle(groups []TaskGroup) bool
}

// TaskGroup represents a group of tasks that can be executed in parallel.
// Groups are ordered by dependency - all tasks in group N must complete
// before tasks in group N+1 can begin.
type TaskGroup struct {
	// Index is the 0-based position of this group in the execution order.
	Index int `json:"index"`

	// TaskIDs contains the IDs of all tasks in this group.
	TaskIDs []string `json:"task_ids"`

	// Tasks contains the full task information for each task in the group.
	// This is optional and may be nil if only IDs are needed.
	Tasks []TaskInfo `json:"tasks,omitempty"`

	// Branches maps task IDs to their corresponding branch names.
	// Populated during consolidation preparation.
	Branches map[string]string `json:"branches,omitempty"`

	// PreconsolidatedBranch is set if this group was already consolidated
	// by a per-group consolidator. If set, the strategy should use this
	// branch directly rather than cherry-picking individual tasks.
	PreconsolidatedBranch string `json:"preconsolidated_branch,omitempty"`

	// CompletionContext contains aggregated context from task completions.
	// Used for generating PR descriptions and tracking issues.
	CompletionContext *AggregatedContext `json:"completion_context,omitempty"`
}

// TaskInfo contains information about a single task for consolidation purposes.
type TaskInfo struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Files       []string `json:"files,omitempty"` // Expected files modified
	Branch      string   `json:"branch,omitempty"`
	Worktree    string   `json:"worktree,omitempty"`
}

// AggregatedContext holds aggregated context from all task completion files
// within a group. This information is used for PR descriptions and tracking.
type AggregatedContext struct {
	// TaskSummaries maps task IDs to their completion summaries.
	TaskSummaries map[string]string `json:"task_summaries,omitempty"`

	// Issues contains all issues flagged by tasks, prefixed with task IDs.
	Issues []string `json:"issues,omitempty"`

	// Suggestions contains integration suggestions from all tasks.
	Suggestions []string `json:"suggestions,omitempty"`

	// Dependencies lists new runtime dependencies added by tasks.
	Dependencies []string `json:"dependencies,omitempty"`

	// Notes contains implementation notes from tasks.
	Notes []string `json:"notes,omitempty"`
}

// HasContent returns true if there is any aggregated context worth displaying.
func (a *AggregatedContext) HasContent() bool {
	if a == nil {
		return false
	}
	return len(a.Issues) > 0 || len(a.Suggestions) > 0 ||
		len(a.Dependencies) > 0 || len(a.Notes) > 0
}

// Config holds configuration for branch consolidation.
type Config struct {
	// Strategy specifies which consolidation strategy to use.
	Strategy StrategyType `json:"strategy"`

	// BranchPrefix is the prefix for generated branch names (e.g., "Iron-Ham").
	BranchPrefix string `json:"branch_prefix"`

	// PlanID is the ID of the ultraplan session, used in branch naming.
	PlanID string `json:"plan_id"`

	// BaseBranch is the main branch to base consolidation on (e.g., "main").
	BaseBranch string `json:"base_branch"`

	// CreateDraftPRs indicates whether to create PRs as drafts.
	CreateDraftPRs bool `json:"create_draft_prs"`

	// PRLabels are labels to apply to created PRs.
	PRLabels []string `json:"pr_labels,omitempty"`

	// Objective is the original user request, used in PR descriptions.
	Objective string `json:"objective,omitempty"`

	// WorktreeBase is the directory where consolidation worktrees are created.
	WorktreeBase string `json:"worktree_base,omitempty"`

	// DryRun prevents actual branch creation and PR submission.
	DryRun bool `json:"dry_run"`
}

// Result holds the outcome of a consolidation operation.
type Result struct {
	// FinalBranch is the name of the final consolidated branch.
	// For stacked mode, this is the last group's branch.
	// For single mode, this is the single consolidated branch.
	FinalBranch string `json:"final_branch"`

	// GroupResults contains results for each consolidated group.
	GroupResults []GroupResult `json:"group_results"`

	// MergedGroups lists the indices of groups that were merged.
	MergedGroups []int `json:"merged_groups"`

	// Conflicts lists any merge conflicts encountered during consolidation.
	Conflicts []Conflict `json:"conflicts,omitempty"`

	// PRUrls contains URLs of created pull requests (in group order).
	PRUrls []string `json:"pr_urls,omitempty"`

	// Success indicates whether the consolidation completed successfully.
	Success bool `json:"success"`

	// Error contains the error message if Success is false.
	Error string `json:"error,omitempty"`

	// StartedAt is when consolidation began.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when consolidation finished (or failed).
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// GroupResult holds the result of consolidating a single group.
type GroupResult struct {
	// GroupIndex is the 0-based index of the group.
	GroupIndex int `json:"group_index"`

	// TaskIDs lists the tasks that were consolidated in this group.
	TaskIDs []string `json:"task_ids"`

	// BranchName is the name of the branch created for this group.
	BranchName string `json:"branch_name"`

	// BaseBranch is the branch this group's branch was based on.
	BaseBranch string `json:"base_branch"`

	// CommitCount is the number of commits added to this group's branch.
	CommitCount int `json:"commit_count"`

	// FilesChanged lists files modified in this group.
	FilesChanged []string `json:"files_changed,omitempty"`

	// PRUrl is the URL of the PR created for this group (if any).
	PRUrl string `json:"pr_url,omitempty"`

	// Success indicates whether this group consolidated successfully.
	Success bool `json:"success"`

	// Error contains the error message if Success is false.
	Error string `json:"error,omitempty"`
}

// Conflict represents a merge conflict encountered during consolidation.
type Conflict struct {
	// TaskID is the ID of the task whose branch caused the conflict.
	TaskID string `json:"task_id"`

	// GroupIndex is the index of the group being consolidated.
	GroupIndex int `json:"group_index"`

	// Branch is the name of the branch that conflicted.
	Branch string `json:"branch"`

	// Files lists the files with conflicts.
	Files []string `json:"files"`

	// WorktreePath is the path to the worktree where the conflict occurred.
	WorktreePath string `json:"worktree_path,omitempty"`

	// ConflictType indicates the type of conflict.
	ConflictType ConflictType `json:"conflict_type"`

	// Description provides additional context about the conflict.
	Description string `json:"description,omitempty"`

	// Resolved indicates whether this conflict has been resolved.
	Resolved bool `json:"resolved"`

	// Resolution describes how the conflict was resolved (if Resolved is true).
	Resolution string `json:"resolution,omitempty"`
}

// ConflictType categorizes different types of merge conflicts.
type ConflictType string

const (
	// ConflictTypeMerge is a standard git merge conflict.
	ConflictTypeMerge ConflictType = "merge"

	// ConflictTypeCherryPick is a conflict during cherry-pick.
	ConflictTypeCherryPick ConflictType = "cherry_pick"

	// ConflictTypeRebase is a conflict during rebase.
	ConflictTypeRebase ConflictType = "rebase"

	// ConflictTypeFile indicates conflicting file changes (both modified same file).
	ConflictTypeFile ConflictType = "file"

	// ConflictTypeDelete indicates a file was modified and deleted.
	ConflictTypeDelete ConflictType = "delete_modify"
)

// ConflictError is an error type that wraps conflict information.
// It implements the error interface and provides structured access
// to conflict details for handling in the consolidation flow.
type ConflictError struct {
	Conflict   Conflict
	Underlying error
}

// Error implements the error interface.
func (e *ConflictError) Error() string {
	if e.Underlying != nil {
		return e.Underlying.Error()
	}
	return "merge conflict in task " + e.Conflict.TaskID
}

// Unwrap returns the underlying error for use with errors.Is/As.
func (e *ConflictError) Unwrap() error {
	return e.Underlying
}

// Phase represents sub-phases within consolidation.
type Phase string

const (
	// PhaseIdle indicates consolidation has not started.
	PhaseIdle Phase = "idle"

	// PhasePreparing indicates consolidation is gathering task information.
	PhasePreparing Phase = "preparing"

	// PhaseDetecting indicates conflict detection is in progress.
	PhaseDetecting Phase = "detecting_conflicts"

	// PhaseCreatingBranches indicates branches are being created.
	PhaseCreatingBranches Phase = "creating_branches"

	// PhaseMergingTasks indicates tasks are being merged into branches.
	PhaseMergingTasks Phase = "merging_tasks"

	// PhasePushing indicates branches are being pushed to remote.
	PhasePushing Phase = "pushing"

	// PhaseCreatingPRs indicates pull requests are being created.
	PhaseCreatingPRs Phase = "creating_prs"

	// PhasePaused indicates consolidation is paused (e.g., waiting for conflict resolution).
	PhasePaused Phase = "paused"

	// PhaseComplete indicates consolidation finished successfully.
	PhaseComplete Phase = "complete"

	// PhaseFailed indicates consolidation failed.
	PhaseFailed Phase = "failed"
)

// State tracks the progress of a consolidation operation.
type State struct {
	// Phase is the current consolidation phase.
	Phase Phase `json:"phase"`

	// CurrentGroup is the index of the group being consolidated.
	CurrentGroup int `json:"current_group"`

	// TotalGroups is the total number of groups to consolidate.
	TotalGroups int `json:"total_groups"`

	// CurrentTask is the ID of the task currently being processed.
	CurrentTask string `json:"current_task,omitempty"`

	// GroupBranches lists the branches created for each group.
	GroupBranches []string `json:"group_branches"`

	// PRUrls lists the URLs of created PRs.
	PRUrls []string `json:"pr_urls"`

	// ActiveConflict holds information about the current conflict (if any).
	ActiveConflict *Conflict `json:"active_conflict,omitempty"`

	// Error holds the error message if phase is Failed.
	Error string `json:"error,omitempty"`

	// StartedAt is when consolidation began.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when consolidation finished.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Progress tracks completion percentage (0-100).
	Progress int `json:"progress"`
}

// EventType represents the type of consolidation event.
type EventType string

const (
	// EventStarted indicates consolidation has begun.
	EventStarted EventType = "consolidation_started"

	// EventPhaseChanged indicates a phase transition.
	EventPhaseChanged EventType = "consolidation_phase_changed"

	// EventGroupStarted indicates a group consolidation has begun.
	EventGroupStarted EventType = "consolidation_group_started"

	// EventTaskMerging indicates a task is being merged.
	EventTaskMerging EventType = "consolidation_task_merging"

	// EventTaskMerged indicates a task was successfully merged.
	EventTaskMerged EventType = "consolidation_task_merged"

	// EventTaskSkipped indicates a task was skipped (e.g., no commits).
	EventTaskSkipped EventType = "consolidation_task_skipped"

	// EventGroupComplete indicates a group was successfully consolidated.
	EventGroupComplete EventType = "consolidation_group_complete"

	// EventGroupFailed indicates a group consolidation failed.
	EventGroupFailed EventType = "consolidation_group_failed"

	// EventBranchCreated indicates a branch was created.
	EventBranchCreated EventType = "consolidation_branch_created"

	// EventBranchPushed indicates a branch was pushed to remote.
	EventBranchPushed EventType = "consolidation_branch_pushed"

	// EventPRCreating indicates a PR is being created.
	EventPRCreating EventType = "consolidation_pr_creating"

	// EventPRCreated indicates a PR was created successfully.
	EventPRCreated EventType = "consolidation_pr_created"

	// EventConflict indicates a merge conflict was encountered.
	EventConflict EventType = "consolidation_conflict"

	// EventConflictResolved indicates a conflict was resolved.
	EventConflictResolved EventType = "consolidation_conflict_resolved"

	// EventProgress indicates a progress update.
	EventProgress EventType = "consolidation_progress"

	// EventComplete indicates consolidation finished successfully.
	EventComplete EventType = "consolidation_complete"

	// EventFailed indicates consolidation failed.
	EventFailed EventType = "consolidation_failed"
)

// Event represents an event during consolidation.
type Event struct {
	// Type identifies the event type.
	Type EventType `json:"type"`

	// GroupIndex is the index of the relevant group (-1 if not applicable).
	GroupIndex int `json:"group_index,omitempty"`

	// TaskID is the ID of the relevant task (empty if not applicable).
	TaskID string `json:"task_id,omitempty"`

	// Branch is the name of the relevant branch (empty if not applicable).
	Branch string `json:"branch,omitempty"`

	// Message provides human-readable context about the event.
	Message string `json:"message,omitempty"`

	// Progress is the completion percentage (0-100) for progress events.
	Progress int `json:"progress,omitempty"`

	// Phase is the new phase for phase change events.
	Phase Phase `json:"phase,omitempty"`

	// Conflict contains conflict details for conflict events.
	Conflict *Conflict `json:"conflict,omitempty"`

	// PRUrl is the PR URL for PR created events.
	PRUrl string `json:"pr_url,omitempty"`

	// Error contains error details for failure events.
	Error string `json:"error,omitempty"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
}

// EventCallback is the signature for consolidation event handlers.
type EventCallback func(Event)

// CompletionFile represents the JSON structure written when consolidation completes.
// This sentinel file signals completion and provides context for subsequent phases.
type CompletionFile struct {
	// Status is "complete", "failed", or "blocked".
	Status string `json:"status"`

	// Strategy is the consolidation strategy that was used.
	Strategy StrategyType `json:"strategy"`

	// GroupCount is the number of groups consolidated.
	GroupCount int `json:"group_count"`

	// FinalBranch is the name of the final branch.
	FinalBranch string `json:"final_branch"`

	// GroupBranches lists branches created for each group.
	GroupBranches []string `json:"group_branches"`

	// PRUrls lists URLs of created PRs.
	PRUrls []string `json:"pr_urls"`

	// Conflicts lists any conflicts encountered.
	Conflicts []Conflict `json:"conflicts,omitempty"`

	// Error contains error details if Status is not "complete".
	Error string `json:"error,omitempty"`

	// StartedAt is when consolidation began.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when consolidation finished.
	CompletedAt time.Time `json:"completed_at"`
}

// CompletionFileName is the name of the consolidation completion sentinel file.
const CompletionFileName = ".claudio-consolidation-complete.json"

// PRContent holds the generated content for a pull request.
type PRContent struct {
	// Title is the PR title.
	Title string `json:"title"`

	// Body is the PR body in markdown format.
	Body string `json:"body"`

	// Labels are labels to apply to the PR.
	Labels []string `json:"labels,omitempty"`

	// Draft indicates whether to create as a draft PR.
	Draft bool `json:"draft"`

	// BaseBranch is the branch to merge into.
	BaseBranch string `json:"base_branch"`

	// HeadBranch is the branch being merged.
	HeadBranch string `json:"head_branch"`
}
