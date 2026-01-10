// Package ultraplan provides types and functionality for ultra-plan orchestration sessions.
//
// Ultra-plan is a multi-phase planning and execution system that:
// 1. Decomposes complex objectives into parallelizable tasks
// 2. Executes tasks across multiple Claude instances in isolated worktrees
// 3. Synthesizes results and identifies integration issues
// 4. Consolidates work into organized branches and pull requests
//
// This package contains type definitions for the ultra-plan system. The actual
// execution logic resides in the parent orchestrator package.
package ultraplan

import (
	"encoding/json"
	"strings"
	"time"
)

// =============================================================================
// Core Enums and Constants
// =============================================================================

// UltraPlanPhase represents the current phase of an ultra-plan session.
// The session progresses through these phases sequentially, with some phases
// being optional (e.g., revision only occurs if issues are found).
type UltraPlanPhase string

const (
	// PhasePlanning is the initial phase where the objective is decomposed into tasks.
	PhasePlanning UltraPlanPhase = "planning"

	// PhasePlanSelection occurs in multi-pass mode when comparing and selecting the best plan.
	PhasePlanSelection UltraPlanPhase = "plan_selection"

	// PhaseRefresh indicates a context refresh is occurring.
	PhaseRefresh UltraPlanPhase = "context_refresh"

	// PhaseExecuting is the main execution phase where tasks run in parallel.
	PhaseExecuting UltraPlanPhase = "executing"

	// PhaseSynthesis is the review phase where completed work is analyzed for issues.
	PhaseSynthesis UltraPlanPhase = "synthesis"

	// PhaseRevision occurs when synthesis identified issues that need fixing.
	PhaseRevision UltraPlanPhase = "revision"

	// PhaseConsolidating is when task branches are being merged into group branches.
	PhaseConsolidating UltraPlanPhase = "consolidating"

	// PhaseComplete indicates successful completion of the ultra-plan session.
	PhaseComplete UltraPlanPhase = "complete"

	// PhaseFailed indicates the session failed and cannot continue.
	PhaseFailed UltraPlanPhase = "failed"
)

// TaskComplexity represents the estimated complexity of a planned task.
// This is used by the planner to indicate expected effort and can influence
// task prioritization and parallel scheduling decisions.
type TaskComplexity string

const (
	// ComplexityLow indicates a straightforward task, typically under 15 minutes.
	ComplexityLow TaskComplexity = "low"

	// ComplexityMedium indicates a moderate task, typically 15-45 minutes.
	ComplexityMedium TaskComplexity = "medium"

	// ComplexityHigh indicates a complex task that may require significant effort.
	// High complexity tasks are candidates for further decomposition.
	ComplexityHigh TaskComplexity = "high"
)

// ConsolidationMode defines how task branches are consolidated into PRs.
type ConsolidationMode string

const (
	// ModeStackedPRs creates one PR per execution group, with each PR
	// based on the previous group's branch, forming a stack.
	ModeStackedPRs ConsolidationMode = "stacked"

	// ModeSinglePR creates a single PR containing all changes from all tasks.
	ModeSinglePR ConsolidationMode = "single"
)

// =============================================================================
// Task and Plan Types
// =============================================================================

// PlannedTask represents a single decomposed task from the planning phase.
// Each task is designed to be executed independently by a Claude instance
// in an isolated worktree.
type PlannedTask struct {
	// ID is a unique identifier for the task, typically like "task-1-setup"
	ID string `json:"id"`

	// Title is a short, human-readable name for the task
	Title string `json:"title"`

	// Description contains detailed instructions for the Claude instance
	// executing this task. It should be complete enough for independent execution.
	Description string `json:"description"`

	// Files lists the files this task is expected to modify.
	// Used to detect potential merge conflicts between parallel tasks.
	Files []string `json:"files,omitempty"`

	// DependsOn lists task IDs that must complete before this task can start.
	// Empty for tasks that can start immediately.
	DependsOn []string `json:"depends_on"`

	// Priority determines execution order within a dependency level.
	// Lower values indicate higher priority.
	Priority int `json:"priority"`

	// EstComplexity is the estimated complexity of the task.
	EstComplexity TaskComplexity `json:"est_complexity"`
}

// PlanSpec represents the complete output of the planning phase.
// It contains all tasks, their dependencies, and metadata about the plan.
type PlanSpec struct {
	// ID is a unique identifier for this plan
	ID string `json:"id"`

	// Objective is the original user request that this plan addresses
	Objective string `json:"objective"`

	// Summary is an executive summary of the plan's approach
	Summary string `json:"summary"`

	// Tasks is the list of all planned tasks
	Tasks []PlannedTask `json:"tasks"`

	// DependencyGraph maps task IDs to their dependencies (task_id -> depends_on[])
	DependencyGraph map[string][]string `json:"dependency_graph"`

	// ExecutionOrder groups tasks by when they can be executed.
	// Each inner slice contains tasks that can run in parallel.
	// Groups are processed sequentially.
	ExecutionOrder [][]string `json:"execution_order"`

	// Insights contains key findings from codebase exploration
	Insights []string `json:"insights"`

	// Constraints lists identified risks or constraints
	Constraints []string `json:"constraints"`

	// CreatedAt records when this plan was generated
	CreatedAt time.Time `json:"created_at"`
}

// =============================================================================
// Configuration Types
// =============================================================================

// UltraPlanConfig holds configuration for an ultra-plan session.
type UltraPlanConfig struct {
	// MaxParallel is the maximum number of concurrent child sessions
	MaxParallel int `json:"max_parallel"`

	// DryRun runs planning only without executing tasks
	DryRun bool `json:"dry_run"`

	// NoSynthesis skips the synthesis phase after execution
	NoSynthesis bool `json:"no_synthesis"`

	// AutoApprove auto-approves spawned tasks without confirmation
	AutoApprove bool `json:"auto_approve"`

	// Review forces the plan editor to open for review (overrides AutoApprove)
	Review bool `json:"review"`

	// MultiPass enables multi-pass planning with plan comparison
	MultiPass bool `json:"multi_pass"`

	// ConsolidationMode is how branches are consolidated ("stacked" or "single")
	ConsolidationMode ConsolidationMode `json:"consolidation_mode,omitempty"`

	// CreateDraftPRs creates PRs as drafts
	CreateDraftPRs bool `json:"create_draft_prs"`

	// PRLabels are labels to add to created PRs
	PRLabels []string `json:"pr_labels,omitempty"`

	// BranchPrefix is the prefix for consolidated branches
	BranchPrefix string `json:"branch_prefix,omitempty"`

	// MaxTaskRetries is the max retry attempts for tasks with no commits (default: 3)
	MaxTaskRetries int `json:"max_task_retries,omitempty"`

	// RequireVerifiedCommits requires tasks to produce commits to be marked successful
	RequireVerifiedCommits bool `json:"require_verified_commits"`
}

// DefaultUltraPlanConfig returns the default configuration for ultra-plan sessions.
func DefaultUltraPlanConfig() UltraPlanConfig {
	return UltraPlanConfig{
		MaxParallel:            3,
		DryRun:                 false,
		NoSynthesis:            false,
		AutoApprove:            false,
		MultiPass:              false,
		ConsolidationMode:      ModeStackedPRs,
		CreateDraftPRs:         true,
		PRLabels:               []string{"ultraplan"},
		BranchPrefix:           "", // Uses config.Branch.Prefix if empty
		MaxTaskRetries:         3,
		RequireVerifiedCommits: true,
	}
}

// =============================================================================
// Revision Types
// =============================================================================

// RevisionIssue represents an issue identified during synthesis that needs to be addressed.
type RevisionIssue struct {
	// TaskID is the task ID that needs revision (empty for cross-cutting issues)
	TaskID string `json:"task_id"`

	// Description explains what the issue is
	Description string `json:"description"`

	// Files lists files affected by the issue
	Files []string `json:"files,omitempty"`

	// Severity is "critical", "major", or "minor"
	Severity string `json:"severity,omitempty"`

	// Suggestion is the recommended fix
	Suggestion string `json:"suggestion,omitempty"`
}

// RevisionState tracks the state of the revision phase.
type RevisionState struct {
	// Issues contains all issues identified during synthesis
	Issues []RevisionIssue `json:"issues"`

	// RevisionRound is the current revision iteration (starts at 1)
	RevisionRound int `json:"revision_round"`

	// MaxRevisions is the maximum allowed revision rounds
	MaxRevisions int `json:"max_revisions"`

	// TasksToRevise lists task IDs that need revision
	TasksToRevise []string `json:"tasks_to_revise,omitempty"`

	// RevisedTasks lists task IDs that have been revised
	RevisedTasks []string `json:"revised_tasks,omitempty"`

	// RevisionPrompts maps task ID to the revision prompt used
	RevisionPrompts map[string]string `json:"revision_prompts,omitempty"`

	// StartedAt records when revision started
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt records when revision completed
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// IsComplete returns true if all tasks have been revised.
func (r *RevisionState) IsComplete() bool {
	return len(r.RevisedTasks) >= len(r.TasksToRevise)
}

// =============================================================================
// Multi-Pass Planning Types
// =============================================================================

// PlanScore represents the evaluation of a single candidate plan.
type PlanScore struct {
	// Strategy is the planning strategy used
	Strategy string `json:"strategy"`

	// Score is the numeric score (1-10)
	Score int `json:"score"`

	// Strengths describes what the plan does well
	Strengths string `json:"strengths"`

	// Weaknesses describes plan deficiencies
	Weaknesses string `json:"weaknesses"`
}

// PlanDecision captures the coordinator-manager's decision when evaluating multiple plans.
type PlanDecision struct {
	// Action is "select" or "merge"
	Action string `json:"action"`

	// SelectedIndex is 0-2 for select, or -1 for merge
	SelectedIndex int `json:"selected_index"`

	// Reasoning explains the decision
	Reasoning string `json:"reasoning"`

	// PlanScores contains the evaluation of each candidate plan
	PlanScores []PlanScore `json:"plan_scores"`
}

// MultiPassPlanningStrategy defines a strategic approach for multi-pass planning.
type MultiPassPlanningStrategy struct {
	// Strategy is the unique identifier for the strategy
	Strategy string

	// Description is a human-readable description
	Description string

	// Prompt is the additional guidance to append to the base planning prompt
	Prompt string
}

// =============================================================================
// Task Execution State Types
// =============================================================================

// TaskWorktreeInfo holds information about a task's worktree for consolidation.
type TaskWorktreeInfo struct {
	// TaskID is the task identifier
	TaskID string `json:"task_id"`

	// TaskTitle is the human-readable task name
	TaskTitle string `json:"task_title"`

	// WorktreePath is the absolute path to the worktree
	WorktreePath string `json:"worktree_path"`

	// Branch is the git branch name for this task
	Branch string `json:"branch"`
}

// TaskRetryState tracks retry attempts for a task that produces no commits.
type TaskRetryState struct {
	// TaskID is the task identifier
	TaskID string `json:"task_id"`

	// RetryCount is the current number of retry attempts
	RetryCount int `json:"retry_count"`

	// MaxRetries is the maximum allowed retries
	MaxRetries int `json:"max_retries"`

	// LastError is the error message from the last attempt
	LastError string `json:"last_error,omitempty"`

	// CommitCounts tracks commits per attempt for debugging
	CommitCounts []int `json:"commit_counts,omitempty"`
}

// GroupDecisionState tracks state when a group has partial success/failure.
type GroupDecisionState struct {
	// GroupIndex is the execution group index
	GroupIndex int `json:"group_index"`

	// SucceededTasks lists tasks with verified commits
	SucceededTasks []string `json:"succeeded_tasks"`

	// FailedTasks lists tasks that failed or produced no commits
	FailedTasks []string `json:"failed_tasks"`

	// AwaitingDecision is true when paused for user input
	AwaitingDecision bool `json:"awaiting_decision"`
}

// =============================================================================
// Event Types
// =============================================================================

// CoordinatorEventType represents the type of coordinator event.
type CoordinatorEventType string

const (
	// EventTaskStarted indicates a task has begun execution
	EventTaskStarted CoordinatorEventType = "task_started"

	// EventTaskComplete indicates a task finished successfully
	EventTaskComplete CoordinatorEventType = "task_complete"

	// EventTaskFailed indicates a task failed
	EventTaskFailed CoordinatorEventType = "task_failed"

	// EventTaskBlocked indicates a task is blocked
	EventTaskBlocked CoordinatorEventType = "task_blocked"

	// EventGroupComplete indicates all tasks in a group are done
	EventGroupComplete CoordinatorEventType = "group_complete"

	// EventPhaseChange indicates the session moved to a new phase
	EventPhaseChange CoordinatorEventType = "phase_change"

	// EventConflict indicates a merge conflict was detected
	EventConflict CoordinatorEventType = "conflict"

	// EventPlanReady indicates the plan is ready for review
	EventPlanReady CoordinatorEventType = "plan_ready"

	// EventMultiPassPlanGenerated indicates one coordinator finished planning
	EventMultiPassPlanGenerated CoordinatorEventType = "multipass_plan_generated"

	// EventAllPlansGenerated indicates all coordinators finished
	EventAllPlansGenerated CoordinatorEventType = "all_plans_generated"

	// EventPlanSelectionStarted indicates the manager started evaluating
	EventPlanSelectionStarted CoordinatorEventType = "plan_selection_started"

	// EventPlanSelected indicates the final plan was chosen
	EventPlanSelected CoordinatorEventType = "plan_selected"
)

// CoordinatorEvent represents an event from the coordinator during execution.
type CoordinatorEvent struct {
	// Type is the event type
	Type CoordinatorEventType `json:"type"`

	// TaskID is the related task ID, if applicable
	TaskID string `json:"task_id,omitempty"`

	// InstanceID is the Claude instance ID, if applicable
	InstanceID string `json:"instance_id,omitempty"`

	// Message contains additional event information
	Message string `json:"message,omitempty"`

	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// PlanIndex is which plan was generated/selected (0-indexed), for multi-pass events
	PlanIndex int `json:"plan_index,omitempty"`

	// Strategy is the planning strategy name, for multi-pass events
	Strategy string `json:"strategy,omitempty"`
}

// =============================================================================
// Completion File Types
// =============================================================================

// FlexibleString is a type that can unmarshal from either a JSON string or an array of strings.
// When unmarshaling an array, the strings are joined with newlines.
// This provides flexibility for Claude instances that may write notes as either format.
type FlexibleString string

// UnmarshalJSON implements json.Unmarshaler for FlexibleString.
func (f *FlexibleString) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexibleString(s)
		return nil
	}

	// Try to unmarshal as an array of strings
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*f = FlexibleString(strings.Join(arr, "\n"))
		return nil
	}

	// If both fail, treat as empty
	*f = ""
	return nil
}

// String returns the underlying string value.
func (f FlexibleString) String() string {
	return string(f)
}

// TaskCompletionFile represents the completion report written by a task.
// This file serves as both a sentinel (existence = task done) and a context carrier.
type TaskCompletionFile struct {
	// TaskID is the task identifier
	TaskID string `json:"task_id"`

	// Status is "complete", "blocked", or "failed"
	Status string `json:"status"`

	// Summary is a brief description of what was accomplished
	Summary string `json:"summary"`

	// FilesModified lists files that were changed
	FilesModified []string `json:"files_modified"`

	// Notes contains free-form implementation notes (accepts string or array)
	Notes FlexibleString `json:"notes,omitempty"`

	// Issues lists blocking issues or concerns found
	Issues []string `json:"issues,omitempty"`

	// Suggestions contains integration suggestions for other tasks
	Suggestions []string `json:"suggestions,omitempty"`

	// Dependencies lists runtime dependencies added
	Dependencies []string `json:"dependencies,omitempty"`
}

// SynthesisCompletionFile represents the completion report from the synthesis phase.
type SynthesisCompletionFile struct {
	// Status is "complete" or "needs_revision"
	Status string `json:"status"`

	// RevisionRound is the current round (0 for first synthesis)
	RevisionRound int `json:"revision_round"`

	// IssuesFound contains all issues identified
	IssuesFound []RevisionIssue `json:"issues_found"`

	// TasksAffected lists task IDs needing revision
	TasksAffected []string `json:"tasks_affected"`

	// IntegrationNotes contains free-form observations about integration
	IntegrationNotes string `json:"integration_notes"`

	// Recommendations contains suggestions for the consolidation phase
	Recommendations []string `json:"recommendations"`
}

// RevisionCompletionFile represents the completion report from a revision task.
type RevisionCompletionFile struct {
	// TaskID is the task that was revised
	TaskID string `json:"task_id"`

	// RevisionRound is which revision iteration this was
	RevisionRound int `json:"revision_round"`

	// IssuesAddressed lists issue descriptions that were fixed
	IssuesAddressed []string `json:"issues_addressed"`

	// Summary describes what was changed
	Summary string `json:"summary"`

	// FilesModified lists files that were changed
	FilesModified []string `json:"files_modified"`

	// RemainingIssues lists issues that couldn't be fixed
	RemainingIssues []string `json:"remaining_issues"`
}

// =============================================================================
// Consolidation Types
// =============================================================================

// ConsolidationState tracks the state of the consolidation phase.
type ConsolidationState struct {
	// Mode is "stacked" or "single"
	Mode ConsolidationMode `json:"mode"`

	// GroupResults contains results for each group consolidation
	GroupResults []GroupConsolidationInfo `json:"group_results,omitempty"`

	// PRsCreated contains info about created PRs
	PRsCreated []PRInfo `json:"prs_created,omitempty"`

	// TotalCommits is the total number of commits consolidated
	TotalCommits int `json:"total_commits,omitempty"`

	// FilesChanged lists all files affected
	FilesChanged []string `json:"files_changed,omitempty"`

	// StartedAt records when consolidation started
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt records when consolidation completed
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ConsolidationCompletionFile represents the completion report from consolidation.
type ConsolidationCompletionFile struct {
	// Status is "complete", "partial", or "failed"
	Status string `json:"status"`

	// Mode is "stacked" or "single"
	Mode string `json:"mode"`

	// GroupResults contains info about each consolidated group
	GroupResults []GroupConsolidationInfo `json:"group_results"`

	// PRsCreated contains info about created PRs
	PRsCreated []PRInfo `json:"prs_created"`

	// SynthesisContext contains context from the synthesis phase
	SynthesisContext *SynthesisCompletionFile `json:"synthesis_context,omitempty"`

	// TotalCommits is the total number of commits consolidated
	TotalCommits int `json:"total_commits"`

	// FilesChanged lists all files affected
	FilesChanged []string `json:"files_changed"`
}

// GroupConsolidationInfo holds info about a consolidated group.
type GroupConsolidationInfo struct {
	// GroupIndex is the execution group index
	GroupIndex int `json:"group_index"`

	// BranchName is the consolidated branch name
	BranchName string `json:"branch_name"`

	// TasksIncluded lists task IDs in this group
	TasksIncluded []string `json:"tasks_included"`

	// CommitCount is the number of commits in this group
	CommitCount int `json:"commit_count"`

	// Success indicates if consolidation succeeded
	Success bool `json:"success"`
}

// PRInfo holds information about a created PR.
type PRInfo struct {
	// URL is the PR URL
	URL string `json:"url"`

	// Title is the PR title
	Title string `json:"title"`

	// GroupIndex is the execution group this PR represents
	GroupIndex int `json:"group_index"`
}

// ConflictResolution describes how a merge conflict was resolved.
type ConflictResolution struct {
	// File is the file that had the conflict
	File string `json:"file"`

	// Resolution describes how it was resolved
	Resolution string `json:"resolution"`
}

// VerificationResult holds the results of build/lint/test verification.
// The consolidator determines appropriate commands based on project type.
type VerificationResult struct {
	// ProjectType is the detected project type: "go", "node", "ios", "python", etc.
	ProjectType string `json:"project_type,omitempty"`

	// CommandsRun lists verification steps that were executed
	CommandsRun []VerificationStep `json:"commands_run"`

	// OverallSuccess indicates if all verification steps passed
	OverallSuccess bool `json:"overall_success"`

	// Summary is a brief summary of the verification outcome
	Summary string `json:"summary,omitempty"`
}

// VerificationStep represents a single verification command and its result.
type VerificationStep struct {
	// Name is the step name, e.g., "build", "lint", "test"
	Name string `json:"name"`

	// Command is the actual command that was run
	Command string `json:"command"`

	// Success indicates if the command passed
	Success bool `json:"success"`

	// Output contains truncated output on failure
	Output string `json:"output,omitempty"`
}

// GroupConsolidationCompletionFile is written by the per-group consolidator session
// when it finishes consolidating a group's task branches.
type GroupConsolidationCompletionFile struct {
	// GroupIndex is the execution group index
	GroupIndex int `json:"group_index"`

	// Status is "complete" or "failed"
	Status string `json:"status"`

	// BranchName is the consolidated branch name
	BranchName string `json:"branch_name"`

	// TasksConsolidated lists task IDs that were consolidated
	TasksConsolidated []string `json:"tasks_consolidated"`

	// ConflictsResolved lists any merge conflicts that were resolved
	ConflictsResolved []ConflictResolution `json:"conflicts_resolved,omitempty"`

	// Verification contains build/lint/test results
	Verification VerificationResult `json:"verification"`

	// AggregatedContext contains combined task context
	AggregatedContext *AggregatedTaskContext `json:"aggregated_context,omitempty"`

	// Notes contains consolidator's observations
	Notes string `json:"notes,omitempty"`

	// IssuesForNextGroup lists warnings/concerns to pass forward
	IssuesForNextGroup []string `json:"issues_for_next_group,omitempty"`
}

// AggregatedTaskContext is a placeholder for combined task context.
// The actual type may be defined elsewhere or expanded later.
type AggregatedTaskContext struct {
	// TaskContexts maps task ID to its completion context
	TaskContexts map[string]*TaskCompletionFile `json:"task_contexts,omitempty"`
}

// =============================================================================
// Session Type
// =============================================================================

// UltraPlanSession represents an ultra-plan orchestration session.
// This is the main state container that tracks all aspects of an ultra-plan execution.
type UltraPlanSession struct {
	// ID is the unique session identifier
	ID string `json:"id"`

	// Objective is the original user request
	Objective string `json:"objective"`

	// Plan is the execution plan (nil during planning phase)
	Plan *PlanSpec `json:"plan,omitempty"`

	// Phase is the current execution phase
	Phase UltraPlanPhase `json:"phase"`

	// Config is the session configuration
	Config UltraPlanConfig `json:"config"`

	// CoordinatorID is the instance ID of the planning coordinator
	CoordinatorID string `json:"coordinator_id,omitempty"`

	// Multi-pass planning state

	// CandidatePlans contains plans from each coordinator in multi-pass mode
	CandidatePlans []*PlanSpec `json:"candidate_plans,omitempty"`

	// PlanCoordinatorIDs contains instance IDs of planning coordinators
	PlanCoordinatorIDs []string `json:"plan_coordinator_ids,omitempty"`

	// ProcessedCoordinators tracks which coordinators have completed (index -> completed)
	ProcessedCoordinators map[int]bool `json:"processed_coordinators,omitempty"`

	// PlanManagerID is the instance ID of the coordinator-manager
	PlanManagerID string `json:"plan_manager_id,omitempty"`

	// SelectedPlanIndex is the index of selected plan (-1 if merged)
	SelectedPlanIndex int `json:"selected_plan_index,omitempty"`

	// Phase instance IDs

	// SynthesisID is the instance ID of the synthesis reviewer
	SynthesisID string `json:"synthesis_id,omitempty"`

	// RevisionID is the instance ID of the current revision coordinator
	RevisionID string `json:"revision_id,omitempty"`

	// ConsolidationID is the instance ID of the consolidation agent
	ConsolidationID string `json:"consolidation_id,omitempty"`

	// Task execution state

	// TaskToInstance maps PlannedTask.ID to Instance.ID
	TaskToInstance map[string]string `json:"task_to_instance"`

	// CompletedTasks lists successfully completed task IDs
	CompletedTasks []string `json:"completed_tasks"`

	// FailedTasks lists failed task IDs
	FailedTasks []string `json:"failed_tasks"`

	// CurrentGroup is the index into ExecutionOrder
	CurrentGroup int `json:"current_group"`

	// Timing information

	// Created is when the session was created
	Created time.Time `json:"created"`

	// StartedAt is when execution started
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when execution completed
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Error is the error message if failed
	Error string `json:"error,omitempty"`

	// Revision state

	// Revision tracks the revision phase state
	Revision *RevisionState `json:"revision,omitempty"`

	// Synthesis state

	// SynthesisCompletion is populated from the sentinel file
	SynthesisCompletion *SynthesisCompletionFile `json:"synthesis_completion,omitempty"`

	// SynthesisAwaitingApproval is true when synthesis is complete but waiting for user review
	SynthesisAwaitingApproval bool `json:"synthesis_awaiting_approval,omitempty"`

	// Consolidation state

	// TaskWorktrees contains worktree information for consolidation
	TaskWorktrees []TaskWorktreeInfo `json:"task_worktrees,omitempty"`

	// GroupConsolidatedBranches maps group index to consolidated branch name
	GroupConsolidatedBranches []string `json:"group_consolidated_branches,omitempty"`

	// GroupConsolidatorIDs maps group index to consolidator instance ID
	GroupConsolidatorIDs []string `json:"group_consolidator_ids,omitempty"`

	// GroupConsolidationContexts stores context from each group's consolidator
	GroupConsolidationContexts []*GroupConsolidationCompletionFile `json:"group_consolidation_contexts,omitempty"`

	// Consolidation is the consolidation phase state
	Consolidation *ConsolidationState `json:"consolidation,omitempty"`

	// PRUrls contains URLs of created PRs
	PRUrls []string `json:"pr_urls,omitempty"`

	// Task verification state

	// TaskRetries maps task ID to retry state
	TaskRetries map[string]*TaskRetryState `json:"task_retries,omitempty"`

	// GroupDecision is set when a group has mixed success/failure
	GroupDecision *GroupDecisionState `json:"group_decision,omitempty"`

	// TaskCommitCounts tracks verified commits per task
	TaskCommitCounts map[string]int `json:"task_commit_counts,omitempty"`
}

// =============================================================================
// File Name Constants
// =============================================================================

const (
	// PlanFileName is the name of the file where the planning agent writes its plan
	PlanFileName = ".claudio-plan.json"

	// TaskCompletionFileName is the sentinel file that tasks write when complete
	TaskCompletionFileName = ".claudio-task-complete.json"

	// SynthesisCompletionFileName is the sentinel file that synthesis writes when complete
	SynthesisCompletionFileName = ".claudio-synthesis-complete.json"

	// RevisionCompletionFileName is the sentinel file that revision tasks write when complete
	RevisionCompletionFileName = ".claudio-revision-complete.json"

	// ConsolidationCompletionFileName is the sentinel file that consolidation writes when complete
	ConsolidationCompletionFileName = ".claudio-consolidation-complete.json"

	// GroupConsolidationCompletionFileName is the sentinel file that per-group consolidators write
	GroupConsolidationCompletionFileName = ".claudio-group-consolidation-complete.json"
)
