// Package ultraplan provides types and utilities for Ultra-Plan orchestration.
//
// Ultra-Plan is Claudio's parallel task execution system that decomposes complex
// objectives into independent tasks, executes them concurrently across isolated
// worktrees, and consolidates the results into pull requests.
//
// This package defines the core data types used throughout the Ultra-Plan lifecycle:
//   - Planning: PlanSpec, PlannedTask, TaskComplexity
//   - Execution: UltraPlanPhase, CoordinatorEvent, CoordinatorEventType
//   - Validation: ValidationResult, ValidationMessage, ValidationSeverity
//   - Consolidation: ConsolidationPhase, ConsolidationMode, ConsolidationState
//   - Results: GroupConsolidationResult, AggregatedTaskContext
//
// These are pure data types with no methods beyond basic getters/setters,
// designed to be used by the orchestrator and other packages.
package ultraplan

import "time"

// -----------------------------------------------------------------------------
// Plan Phase Enums
// -----------------------------------------------------------------------------

// UltraPlanPhase represents the current phase of an Ultra-Plan session.
//
// The Ultra-Plan lifecycle progresses through these phases:
//  1. PhasePlanning - Initial task decomposition and planning
//  2. PhasePlanSelection - (Multi-pass only) Comparing and selecting best plan
//  3. PhaseRefresh - Optional context refresh before execution
//  4. PhaseExecuting - Parallel task execution across worktrees
//  5. PhaseSynthesis - Review of completed work for integration issues
//  6. PhaseRevision - Fixing issues identified during synthesis
//  7. PhaseConsolidating - Merging task branches and creating PRs
//  8. PhaseComplete - Successfully finished
//  9. PhaseFailed - Terminated due to error
type UltraPlanPhase string

const (
	// PhasePlanning indicates the system is decomposing the objective into tasks.
	// During this phase, a planning agent explores the codebase and creates a PlanSpec.
	PhasePlanning UltraPlanPhase = "planning"

	// PhasePlanSelection indicates multi-pass planning is comparing candidate plans.
	// Multiple planning strategies generate plans, which are then evaluated and merged.
	PhasePlanSelection UltraPlanPhase = "plan_selection"

	// PhaseRefresh indicates the system is refreshing context before execution.
	// This optional phase updates understanding of the codebase state.
	PhaseRefresh UltraPlanPhase = "context_refresh"

	// PhaseExecuting indicates tasks are being executed in parallel.
	// Each task runs in its own worktree with an independent Claude session.
	PhaseExecuting UltraPlanPhase = "executing"

	// PhaseSynthesis indicates a review of all completed work is underway.
	// The synthesis agent checks for integration issues and conflicts.
	PhaseSynthesis UltraPlanPhase = "synthesis"

	// PhaseRevision indicates issues are being fixed based on synthesis review.
	// Tasks identified with problems are re-executed with targeted fixes.
	PhaseRevision UltraPlanPhase = "revision"

	// PhaseConsolidating indicates task branches are being merged and PRs created.
	// This is the final active phase before completion.
	PhaseConsolidating UltraPlanPhase = "consolidating"

	// PhaseComplete indicates the Ultra-Plan has finished successfully.
	// All tasks completed, synthesis passed, and PRs were created.
	PhaseComplete UltraPlanPhase = "complete"

	// PhaseFailed indicates the Ultra-Plan terminated due to an error.
	// Check the session's Error field for details.
	PhaseFailed UltraPlanPhase = "failed"
)

// String returns the string representation of the phase.
func (p UltraPlanPhase) String() string {
	return string(p)
}

// IsTerminal returns true if this phase represents a final state.
func (p UltraPlanPhase) IsTerminal() bool {
	return p == PhaseComplete || p == PhaseFailed
}

// IsActive returns true if this phase represents active processing.
func (p UltraPlanPhase) IsActive() bool {
	return !p.IsTerminal()
}

// -----------------------------------------------------------------------------
// Task Complexity
// -----------------------------------------------------------------------------

// TaskComplexity represents the estimated complexity of a planned task.
//
// Complexity affects scheduling decisions and helps identify tasks that
// may benefit from being split into smaller units. The planning agent
// assigns complexity based on scope, file count, and required changes.
type TaskComplexity string

const (
	// ComplexityLow indicates a simple, well-scoped task.
	// Expected to complete quickly with minimal risk of issues.
	// Examples: adding a single function, fixing a typo, updating imports.
	ComplexityLow TaskComplexity = "low"

	// ComplexityMedium indicates a moderate task with some scope.
	// May touch multiple files but has clear boundaries.
	// Examples: implementing a feature component, refactoring a module.
	ComplexityMedium TaskComplexity = "medium"

	// ComplexityHigh indicates a complex task requiring significant work.
	// Consider splitting high-complexity tasks into smaller units.
	// Examples: large refactors, new subsystems, cross-cutting concerns.
	ComplexityHigh TaskComplexity = "high"
)

// String returns the string representation of the complexity.
func (c TaskComplexity) String() string {
	return string(c)
}

// IsValid returns true if this is a recognized complexity value.
func (c TaskComplexity) IsValid() bool {
	switch c {
	case ComplexityLow, ComplexityMedium, ComplexityHigh:
		return true
	default:
		return false
	}
}

// -----------------------------------------------------------------------------
// Planned Task
// -----------------------------------------------------------------------------

// PlannedTask represents a single decomposed task from the planning phase.
//
// Each PlannedTask is designed to be executed independently by a Claude session
// in its own git worktree. Tasks should be granular enough to complete within
// a single session without context exhaustion.
//
// Task dependencies form a directed acyclic graph (DAG) that determines
// execution order. Tasks with no dependencies can run in parallel.
type PlannedTask struct {
	// ID uniquely identifies this task within the plan.
	// Should be descriptive and follow a pattern like "task-1-setup" or "task-auth-middleware".
	ID string `json:"id"`

	// Title is a short, human-readable name for the task.
	// Displayed in the UI and used in branch names and PR descriptions.
	Title string `json:"title"`

	// Description contains detailed instructions for the executing Claude session.
	// Should be comprehensive enough for independent execution without additional context.
	// Include specific file paths, function names, and acceptance criteria.
	Description string `json:"description"`

	// Files lists the expected files this task will modify.
	// Used for conflict detection and assigning clear ownership.
	// Multiple tasks modifying the same file may cause merge conflicts.
	Files []string `json:"files,omitempty"`

	// DependsOn lists task IDs that must complete before this task can start.
	// Forms the edges of the task dependency graph.
	// An empty list means the task has no dependencies and can run immediately.
	DependsOn []string `json:"depends_on"`

	// Priority determines execution order within tasks that have the same dependencies.
	// Lower values indicate higher priority (executed earlier).
	// Default is 0; use negative values for critical-path tasks.
	Priority int `json:"priority"`

	// EstComplexity is the estimated complexity of this task.
	// High complexity tasks may benefit from being split.
	EstComplexity TaskComplexity `json:"est_complexity"`

	// IssueURL optionally links to an external issue tracker URL.
	// Supports GitHub Issues, Linear, Notion, and other trackers.
	// When set, the issue will be auto-closed upon task completion.
	IssueURL string `json:"issue_url,omitempty"`
}

// HasDependencies returns true if this task depends on other tasks.
func (t *PlannedTask) HasDependencies() bool {
	return len(t.DependsOn) > 0
}

// HasFiles returns true if this task has expected files specified.
func (t *PlannedTask) HasFiles() bool {
	return len(t.Files) > 0
}

// -----------------------------------------------------------------------------
// Plan Specification
// -----------------------------------------------------------------------------

// PlanSpec represents the complete output of the planning phase.
//
// A PlanSpec contains all tasks to execute, their dependencies, and metadata
// about the planning process. It is persisted to disk and used throughout
// the Ultra-Plan lifecycle.
//
// The DependencyGraph and ExecutionOrder are computed from the task dependencies
// and represent the same relationship in different formats for different uses.
type PlanSpec struct {
	// ID uniquely identifies this plan.
	// Generated automatically during plan creation.
	ID string `json:"id"`

	// Objective is the original user request that spawned this plan.
	// Preserved for context in synthesis and PR descriptions.
	Objective string `json:"objective"`

	// Summary is an executive summary of the plan.
	// Provides a high-level overview of the approach and scope.
	Summary string `json:"summary"`

	// Tasks contains all planned tasks in this plan.
	// Order in this slice does not determine execution order (see ExecutionOrder).
	Tasks []PlannedTask `json:"tasks"`

	// DependencyGraph maps each task ID to its list of dependencies.
	// Computed from task.DependsOn fields for efficient lookup.
	// Key: task ID, Value: slice of task IDs this task depends on.
	DependencyGraph map[string][]string `json:"dependency_graph"`

	// ExecutionOrder groups tasks into parallelizable batches.
	// Each inner slice contains task IDs that can run concurrently.
	// Outer slice is ordered: group 0 runs first, then group 1, etc.
	// Computed via topological sort of the dependency graph.
	ExecutionOrder [][]string `json:"execution_order"`

	// Insights contains key findings from codebase exploration during planning.
	// May include architecture notes, patterns observed, or relevant context.
	Insights []string `json:"insights"`

	// Constraints lists identified risks, limitations, or concerns.
	// Used to inform task execution and synthesis review.
	Constraints []string `json:"constraints"`

	// CreatedAt is the timestamp when this plan was created.
	CreatedAt time.Time `json:"created_at"`
}

// TaskCount returns the total number of tasks in the plan.
func (p *PlanSpec) TaskCount() int {
	return len(p.Tasks)
}

// GroupCount returns the number of execution groups.
func (p *PlanSpec) GroupCount() int {
	return len(p.ExecutionOrder)
}

// GetTask returns the task with the given ID, or nil if not found.
func (p *PlanSpec) GetTask(taskID string) *PlannedTask {
	for i := range p.Tasks {
		if p.Tasks[i].ID == taskID {
			return &p.Tasks[i]
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Validation Types
// -----------------------------------------------------------------------------

// ValidationSeverity represents the severity level of a validation message.
//
// Used by the plan editor to categorize issues found during plan validation.
// Errors prevent execution while warnings and info are advisory.
type ValidationSeverity string

const (
	// SeverityError indicates a blocking issue that must be fixed.
	// Plans with errors cannot proceed to execution.
	// Examples: dependency cycles, missing dependencies, duplicate IDs.
	SeverityError ValidationSeverity = "error"

	// SeverityWarning indicates a potential issue that should be reviewed.
	// Plans with warnings can proceed but may have problems.
	// Examples: file conflicts, high complexity tasks, missing descriptions.
	SeverityWarning ValidationSeverity = "warning"

	// SeverityInfo indicates informational feedback.
	// Not a problem, just helpful context for the user.
	// Examples: suggestions for improvement, statistics.
	SeverityInfo ValidationSeverity = "info"
)

// String returns the string representation of the severity.
func (s ValidationSeverity) String() string {
	return string(s)
}

// ValidationMessage represents a single validation issue with structured information.
//
// Messages are generated during plan validation and displayed in the plan editor.
// Each message identifies what's wrong and often suggests how to fix it.
type ValidationMessage struct {
	// Severity indicates how critical this issue is.
	Severity ValidationSeverity `json:"severity"`

	// Message is a human-readable description of the issue.
	Message string `json:"message"`

	// TaskID identifies the task this message relates to.
	// Empty for plan-level issues that don't relate to a specific task.
	TaskID string `json:"task_id,omitempty"`

	// Field identifies the specific field causing the issue.
	// Examples: "depends_on", "description", "files".
	Field string `json:"field,omitempty"`

	// Suggestion provides guidance on how to fix the issue.
	// Should be actionable and specific.
	Suggestion string `json:"suggestion,omitempty"`

	// RelatedIDs lists other task IDs related to this issue.
	// Used for cycles, conflicts, and cross-task issues.
	RelatedIDs []string `json:"related_ids,omitempty"`
}

// IsError returns true if this message is an error.
func (m *ValidationMessage) IsError() bool {
	return m.Severity == SeverityError
}

// IsWarning returns true if this message is a warning.
func (m *ValidationMessage) IsWarning() bool {
	return m.Severity == SeverityWarning
}

// ValidationResult contains the complete validation results for a plan.
//
// Generated by ValidatePlanForEditor() and used to determine if a plan
// is ready for execution and to display issues in the plan editor UI.
type ValidationResult struct {
	// IsValid is true if there are no errors (warnings allowed).
	// A valid plan can proceed to execution.
	IsValid bool `json:"is_valid"`

	// Messages contains all validation messages found.
	Messages []ValidationMessage `json:"messages"`

	// ErrorCount is the number of error-level messages.
	ErrorCount int `json:"error_count"`

	// WarningCount is the number of warning-level messages.
	WarningCount int `json:"warning_count"`

	// InfoCount is the number of info-level messages.
	InfoCount int `json:"info_count"`
}

// HasErrors returns true if there are any error-level messages.
func (v *ValidationResult) HasErrors() bool {
	return v.ErrorCount > 0
}

// HasWarnings returns true if there are any warning-level messages.
func (v *ValidationResult) HasWarnings() bool {
	return v.WarningCount > 0
}

// GetMessagesForTask returns all validation messages for a specific task.
func (v *ValidationResult) GetMessagesForTask(taskID string) []ValidationMessage {
	var messages []ValidationMessage
	for _, msg := range v.Messages {
		if msg.TaskID == taskID {
			messages = append(messages, msg)
		}
	}
	return messages
}

// GetMessagesBySeverity returns all messages of a specific severity.
func (v *ValidationResult) GetMessagesBySeverity(severity ValidationSeverity) []ValidationMessage {
	var messages []ValidationMessage
	for _, msg := range v.Messages {
		if msg.Severity == severity {
			messages = append(messages, msg)
		}
	}
	return messages
}

// -----------------------------------------------------------------------------
// Consolidation Types
// -----------------------------------------------------------------------------

// ConsolidationMode defines how work is consolidated after Ultra-Plan execution.
//
// Determines the branching and PR strategy for merging completed task work
// back into the main branch.
type ConsolidationMode string

const (
	// ModeStackedPRs creates one PR per execution group, stacked on each other.
	// Group 1 PR targets main, Group 2 PR targets Group 1's branch, etc.
	// Allows incremental review and merge of related changes.
	ModeStackedPRs ConsolidationMode = "stacked"

	// ModeSinglePR consolidates all work into a single PR targeting main.
	// Simpler workflow but all changes must be reviewed together.
	ModeSinglePR ConsolidationMode = "single"
)

// String returns the string representation of the consolidation mode.
func (m ConsolidationMode) String() string {
	return string(m)
}

// ConsolidationPhase represents sub-phases within the consolidation process.
//
// Provides granular status updates during the branch merging and PR creation
// workflow. Used for progress tracking and error recovery.
type ConsolidationPhase string

const (
	// ConsolidationIdle indicates consolidation has not started.
	ConsolidationIdle ConsolidationPhase = "idle"

	// ConsolidationDetecting indicates the system is detecting potential conflicts.
	ConsolidationDetecting ConsolidationPhase = "detecting_conflicts"

	// ConsolidationCreatingBranches indicates group branches are being created.
	ConsolidationCreatingBranches ConsolidationPhase = "creating_branches"

	// ConsolidationMergingTasks indicates task commits are being cherry-picked.
	ConsolidationMergingTasks ConsolidationPhase = "merging_tasks"

	// ConsolidationPushing indicates branches are being pushed to remote.
	ConsolidationPushing ConsolidationPhase = "pushing"

	// ConsolidationCreatingPRs indicates pull requests are being created.
	ConsolidationCreatingPRs ConsolidationPhase = "creating_prs"

	// ConsolidationPaused indicates consolidation is paused, usually for conflict resolution.
	ConsolidationPaused ConsolidationPhase = "paused"

	// ConsolidationComplete indicates consolidation finished successfully.
	ConsolidationComplete ConsolidationPhase = "complete"

	// ConsolidationFailed indicates consolidation terminated due to error.
	ConsolidationFailed ConsolidationPhase = "failed"
)

// String returns the string representation of the consolidation phase.
func (p ConsolidationPhase) String() string {
	return string(p)
}

// IsTerminal returns true if this phase represents a final state.
func (p ConsolidationPhase) IsTerminal() bool {
	return p == ConsolidationComplete || p == ConsolidationFailed
}

// ConsolidationState tracks the progress of consolidation.
//
// Persisted to the Ultra-Plan session for recovery and status display.
// Updated as consolidation progresses through its phases.
type ConsolidationState struct {
	// Phase is the current consolidation sub-phase.
	Phase ConsolidationPhase `json:"phase"`

	// CurrentGroup is the index of the group currently being processed.
	CurrentGroup int `json:"current_group"`

	// TotalGroups is the total number of execution groups to consolidate.
	TotalGroups int `json:"total_groups"`

	// CurrentTask is the ID of the task currently being merged (if any).
	CurrentTask string `json:"current_task,omitempty"`

	// GroupBranches contains the branch names created for each group.
	// Index corresponds to group index.
	GroupBranches []string `json:"group_branches"`

	// PRUrls contains the URLs of created pull requests.
	PRUrls []string `json:"pr_urls"`

	// ConflictFiles lists files with merge conflicts (when paused).
	ConflictFiles []string `json:"conflict_files,omitempty"`

	// ConflictTaskID is the task that caused a conflict (when paused).
	ConflictTaskID string `json:"conflict_task_id,omitempty"`

	// ConflictWorktree is the worktree path where conflict occurred (when paused).
	ConflictWorktree string `json:"conflict_worktree,omitempty"`

	// Error contains the error message if consolidation failed.
	Error string `json:"error,omitempty"`

	// StartedAt is when consolidation began.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when consolidation finished (success or failure).
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// HasConflict returns true if consolidation is paused due to a conflict.
func (s *ConsolidationState) HasConflict() bool {
	return s.Phase == ConsolidationPaused && len(s.ConflictFiles) > 0
}

// Duration returns the duration of consolidation, or zero if not started/completed.
func (s *ConsolidationState) Duration() time.Duration {
	if s.StartedAt == nil {
		return 0
	}
	if s.CompletedAt == nil {
		return time.Since(*s.StartedAt)
	}
	return s.CompletedAt.Sub(*s.StartedAt)
}

// GroupConsolidationResult holds the result of consolidating one execution group.
//
// Created after each group's task branches are merged into a group branch.
// Aggregated into the final consolidation report.
type GroupConsolidationResult struct {
	// GroupIndex is the zero-based index of this group in ExecutionOrder.
	GroupIndex int `json:"group_index"`

	// TaskIDs lists all task IDs included in this group.
	TaskIDs []string `json:"task_ids"`

	// BranchName is the name of the consolidated group branch.
	BranchName string `json:"branch_name"`

	// CommitCount is the number of commits cherry-picked into the group branch.
	CommitCount int `json:"commit_count"`

	// FilesChanged lists all files modified in this group's commits.
	FilesChanged []string `json:"files_changed"`

	// PRUrl is the URL of the PR created for this group (if any).
	PRUrl string `json:"pr_url,omitempty"`

	// Success indicates whether this group was consolidated successfully.
	Success bool `json:"success"`

	// Error contains the error message if consolidation failed for this group.
	Error string `json:"error,omitempty"`
}

// -----------------------------------------------------------------------------
// Aggregated Task Context
// -----------------------------------------------------------------------------

// AggregatedTaskContext holds aggregated context from all task completion files.
//
// Gathered during consolidation from the .claudio-task-complete.json files
// written by each task. Used in PR descriptions and consolidation prompts.
type AggregatedTaskContext struct {
	// TaskSummaries maps task ID to the summary from its completion file.
	TaskSummaries map[string]string `json:"task_summaries"`

	// AllIssues contains all issues/concerns flagged by tasks.
	// Each entry is prefixed with [task-id] for context.
	AllIssues []string `json:"all_issues"`

	// AllSuggestions contains integration suggestions from tasks.
	// Each entry is prefixed with [task-id] for context.
	AllSuggestions []string `json:"all_suggestions"`

	// Dependencies is a deduplicated list of new runtime dependencies added.
	Dependencies []string `json:"dependencies"`

	// Notes contains implementation notes from all tasks.
	Notes []string `json:"notes"`
}

// HasContent returns true if there is any aggregated context worth displaying.
func (a *AggregatedTaskContext) HasContent() bool {
	return len(a.AllIssues) > 0 || len(a.AllSuggestions) > 0 ||
		len(a.Dependencies) > 0 || len(a.Notes) > 0
}

// -----------------------------------------------------------------------------
// Coordinator Event Types
// -----------------------------------------------------------------------------

// CoordinatorEventType represents the type of event emitted during Ultra-Plan execution.
//
// Events are emitted by the UltraPlanManager to notify listeners (like the TUI)
// of progress and state changes during plan execution.
type CoordinatorEventType string

const (
	// EventTaskStarted indicates a task has begun execution.
	EventTaskStarted CoordinatorEventType = "task_started"

	// EventTaskComplete indicates a task finished successfully.
	EventTaskComplete CoordinatorEventType = "task_complete"

	// EventTaskFailed indicates a task failed.
	EventTaskFailed CoordinatorEventType = "task_failed"

	// EventTaskBlocked indicates a task is blocked (dependencies not met).
	EventTaskBlocked CoordinatorEventType = "task_blocked"

	// EventGroupComplete indicates all tasks in an execution group completed.
	EventGroupComplete CoordinatorEventType = "group_complete"

	// EventPhaseChange indicates the Ultra-Plan phase changed.
	EventPhaseChange CoordinatorEventType = "phase_change"

	// EventConflict indicates a merge conflict was detected.
	EventConflict CoordinatorEventType = "conflict"

	// EventPlanReady indicates the plan has been created and is ready for review.
	EventPlanReady CoordinatorEventType = "plan_ready"

	// EventMultiPassPlanGenerated indicates one planning coordinator finished (multi-pass mode).
	EventMultiPassPlanGenerated CoordinatorEventType = "multipass_plan_generated"

	// EventAllPlansGenerated indicates all planning coordinators finished (multi-pass mode).
	EventAllPlansGenerated CoordinatorEventType = "all_plans_generated"

	// EventPlanSelectionStarted indicates the plan manager started evaluating candidates.
	EventPlanSelectionStarted CoordinatorEventType = "plan_selection_started"

	// EventPlanSelected indicates the final plan has been chosen (multi-pass mode).
	EventPlanSelected CoordinatorEventType = "plan_selected"
)

// String returns the string representation of the event type.
func (e CoordinatorEventType) String() string {
	return string(e)
}

// CoordinatorEvent represents an event from the coordinator during execution.
//
// Emitted through the event channel and callback to notify listeners
// of progress during Ultra-Plan execution.
type CoordinatorEvent struct {
	// Type identifies what kind of event this is.
	Type CoordinatorEventType `json:"type"`

	// TaskID is the task this event relates to (if applicable).
	TaskID string `json:"task_id,omitempty"`

	// InstanceID is the Claude session instance ID (if applicable).
	InstanceID string `json:"instance_id,omitempty"`

	// Message provides human-readable context for the event.
	Message string `json:"message,omitempty"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// PlanIndex indicates which plan was generated/selected (multi-pass mode).
	// Zero-indexed; only meaningful for multi-pass planning events.
	PlanIndex int `json:"plan_index,omitempty"`

	// Strategy is the planning strategy name for multi-pass events.
	// Examples: "maximize-parallelism", "minimize-complexity", "balanced-approach".
	Strategy string `json:"strategy,omitempty"`
}

// -----------------------------------------------------------------------------
// Revision Types
// -----------------------------------------------------------------------------

// RevisionIssue represents an issue identified during synthesis that needs to be addressed.
//
// Created by the synthesis agent when reviewing completed task work.
// Tasks with issues enter the revision phase for targeted fixes.
type RevisionIssue struct {
	// TaskID is the task that needs revision.
	// Empty string for cross-cutting issues that don't relate to a specific task.
	TaskID string `json:"task_id"`

	// Description explains what the issue is.
	Description string `json:"description"`

	// Files lists files affected by the issue.
	Files []string `json:"files,omitempty"`

	// Severity indicates how critical the issue is.
	// Values: "critical", "major", "minor".
	Severity string `json:"severity,omitempty"`

	// Suggestion provides guidance on how to fix the issue.
	Suggestion string `json:"suggestion,omitempty"`
}

// IsCritical returns true if this is a critical severity issue.
func (r *RevisionIssue) IsCritical() bool {
	return r.Severity == "critical"
}

// IsMajor returns true if this is a major severity issue.
func (r *RevisionIssue) IsMajor() bool {
	return r.Severity == "major"
}

// -----------------------------------------------------------------------------
// Plan Scoring Types (Multi-Pass Planning)
// -----------------------------------------------------------------------------

// PlanScore represents the evaluation of a single candidate plan.
//
// Generated by the plan manager when comparing multiple planning strategies
// in multi-pass mode. Used to select or merge the best approach.
type PlanScore struct {
	// Strategy is the planning strategy identifier.
	// Examples: "maximize-parallelism", "minimize-complexity", "balanced-approach".
	Strategy string `json:"strategy"`

	// Score is the numerical score (1-10) assigned to this plan.
	Score int `json:"score"`

	// Strengths describes what this plan does well.
	Strengths string `json:"strengths"`

	// Weaknesses describes areas where this plan could be improved.
	Weaknesses string `json:"weaknesses"`
}

// PlanDecision captures the coordinator-manager's decision when evaluating multiple plans.
//
// Generated during multi-pass planning when the plan manager chooses
// how to proceed after evaluating all candidate plans.
type PlanDecision struct {
	// Action is either "select" (use one plan as-is) or "merge" (combine plans).
	Action string `json:"action"`

	// SelectedIndex is the index of the chosen plan (0-2 for select, -1 for merge).
	SelectedIndex int `json:"selected_index"`

	// Reasoning explains why this decision was made.
	Reasoning string `json:"reasoning"`

	// PlanScores contains the evaluation of each candidate plan.
	PlanScores []PlanScore `json:"plan_scores"`
}

// IsSelect returns true if the decision was to select a single plan.
func (d *PlanDecision) IsSelect() bool {
	return d.Action == "select"
}

// IsMerge returns true if the decision was to merge multiple plans.
func (d *PlanDecision) IsMerge() bool {
	return d.Action == "merge"
}

// -----------------------------------------------------------------------------
// Task Worktree Info
// -----------------------------------------------------------------------------

// TaskWorktreeInfo holds information about a task's worktree for consolidation.
//
// Used to track the mapping between tasks and their execution environments,
// enabling the consolidation phase to locate and merge task work.
type TaskWorktreeInfo struct {
	// TaskID is the unique identifier of the task.
	TaskID string `json:"task_id"`

	// TaskTitle is the human-readable title of the task.
	TaskTitle string `json:"task_title"`

	// WorktreePath is the filesystem path to the task's worktree.
	WorktreePath string `json:"worktree_path"`

	// Branch is the git branch name for this task's work.
	Branch string `json:"branch"`
}

// -----------------------------------------------------------------------------
// Task Retry State
// -----------------------------------------------------------------------------

// TaskRetryState tracks retry attempts for a task that produces no commits.
//
// When a task completes but has no commits (likely a failure or no-op),
// the system can retry it. This tracks retry state for that process.
type TaskRetryState struct {
	// TaskID is the task being retried.
	TaskID string `json:"task_id"`

	// RetryCount is the current retry attempt number.
	RetryCount int `json:"retry_count"`

	// MaxRetries is the maximum retry attempts allowed.
	MaxRetries int `json:"max_retries"`

	// LastError is the error message from the last failed attempt.
	LastError string `json:"last_error,omitempty"`

	// CommitCounts tracks commits per attempt for debugging.
	// Index i is the commit count after attempt i.
	CommitCounts []int `json:"commit_counts,omitempty"`
}

// CanRetry returns true if more retry attempts are available.
func (t *TaskRetryState) CanRetry() bool {
	return t.RetryCount < t.MaxRetries
}

// -----------------------------------------------------------------------------
// Group Decision State
// -----------------------------------------------------------------------------

// GroupDecisionState tracks state when a group has partial success/failure.
//
// When some tasks in a group succeed and others fail, the user must decide
// how to proceed. This state captures that situation.
type GroupDecisionState struct {
	// GroupIndex is the index of the affected execution group.
	GroupIndex int `json:"group_index"`

	// SucceededTasks lists task IDs that completed with verified commits.
	SucceededTasks []string `json:"succeeded_tasks"`

	// FailedTasks lists task IDs that failed or produced no commits.
	FailedTasks []string `json:"failed_tasks"`

	// AwaitingDecision is true when paused waiting for user input.
	AwaitingDecision bool `json:"awaiting_decision"`
}

// HasFailures returns true if any tasks in this group failed.
func (g *GroupDecisionState) HasFailures() bool {
	return len(g.FailedTasks) > 0
}

// AllFailed returns true if all tasks in this group failed.
func (g *GroupDecisionState) AllFailed() bool {
	return len(g.SucceededTasks) == 0 && len(g.FailedTasks) > 0
}

// AllSucceeded returns true if all tasks in this group succeeded.
func (g *GroupDecisionState) AllSucceeded() bool {
	return len(g.FailedTasks) == 0 && len(g.SucceededTasks) > 0
}

// -----------------------------------------------------------------------------
// Consolidation Event Types
// -----------------------------------------------------------------------------

// ConsolidationEventType represents events during the consolidation process.
//
// More granular than CoordinatorEventType, these events track
// the specific steps within the consolidation phase.
type ConsolidationEventType string

const (
	// EventConsolidationStarted indicates consolidation has begun.
	EventConsolidationStarted ConsolidationEventType = "consolidation_started"

	// EventConsolidationGroupStarted indicates processing of a group began.
	EventConsolidationGroupStarted ConsolidationEventType = "consolidation_group_started"

	// EventConsolidationTaskMerging indicates a task is being cherry-picked.
	EventConsolidationTaskMerging ConsolidationEventType = "consolidation_task_merging"

	// EventConsolidationTaskMerged indicates a task was successfully merged.
	EventConsolidationTaskMerged ConsolidationEventType = "consolidation_task_merged"

	// EventConsolidationGroupComplete indicates a group was fully consolidated.
	EventConsolidationGroupComplete ConsolidationEventType = "consolidation_group_complete"

	// EventConsolidationPRCreating indicates a PR is being created.
	EventConsolidationPRCreating ConsolidationEventType = "consolidation_pr_creating"

	// EventConsolidationPRCreated indicates a PR was successfully created.
	EventConsolidationPRCreated ConsolidationEventType = "consolidation_pr_created"

	// EventConsolidationConflict indicates a merge conflict was detected.
	EventConsolidationConflict ConsolidationEventType = "consolidation_conflict"

	// EventConsolidationComplete indicates consolidation finished successfully.
	EventConsolidationComplete ConsolidationEventType = "consolidation_complete"

	// EventConsolidationFailed indicates consolidation failed.
	EventConsolidationFailed ConsolidationEventType = "consolidation_failed"
)

// String returns the string representation of the consolidation event type.
func (e ConsolidationEventType) String() string {
	return string(e)
}

// ConsolidationEvent represents an event during consolidation.
//
// Emitted by the Consolidator to provide progress updates
// during branch merging and PR creation.
type ConsolidationEvent struct {
	// Type identifies what kind of consolidation event this is.
	Type ConsolidationEventType `json:"type"`

	// GroupIdx is the group index this event relates to (if applicable).
	GroupIdx int `json:"group_idx,omitempty"`

	// TaskID is the task this event relates to (if applicable).
	TaskID string `json:"task_id,omitempty"`

	// Message provides human-readable context for the event.
	Message string `json:"message,omitempty"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
}
