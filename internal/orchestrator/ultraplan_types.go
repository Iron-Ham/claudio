package orchestrator

import (
	"time"
)

// UltraPlanPhase represents the current phase of an ultra-plan session
type UltraPlanPhase string

const (
	PhasePlanning      UltraPlanPhase = "planning"
	PhasePlanSelection UltraPlanPhase = "plan_selection" // Multi-pass: comparing and selecting best plan
	PhaseRefresh       UltraPlanPhase = "context_refresh"
	PhaseExecuting     UltraPlanPhase = "executing"
	PhaseSynthesis     UltraPlanPhase = "synthesis"
	PhaseRevision      UltraPlanPhase = "revision"
	PhaseConsolidating UltraPlanPhase = "consolidating"
	PhaseComplete      UltraPlanPhase = "complete"
	PhaseFailed        UltraPlanPhase = "failed"
)

// UltraPlanConfig holds configuration for an ultra-plan session
type UltraPlanConfig struct {
	MaxParallel int  `json:"max_parallel"`  // Maximum concurrent child sessions
	DryRun      bool `json:"dry_run"`       // Run planning only, don't execute
	NoSynthesis bool `json:"no_synthesis"`  // Skip synthesis phase after execution
	AutoApprove bool `json:"auto_approve"`  // Auto-approve spawned tasks without confirmation
	Review      bool `json:"review"`        // Force plan editor to open for review (overrides AutoApprove)
	MultiPass   bool `json:"multi_pass"`    // Enable multi-pass planning with plan comparison

	// Consolidation settings
	ConsolidationMode ConsolidationMode `json:"consolidation_mode,omitempty"` // "stacked" or "single"
	CreateDraftPRs    bool              `json:"create_draft_prs"`             // Create PRs as drafts
	PRLabels          []string          `json:"pr_labels,omitempty"`          // Labels to add to PRs
	BranchPrefix      string            `json:"branch_prefix,omitempty"`      // Branch prefix for consolidated branches

	// Task verification settings
	MaxTaskRetries         int  `json:"max_task_retries,omitempty"`   // Max retry attempts for tasks with no commits (default: 3)
	RequireVerifiedCommits bool `json:"require_verified_commits"`     // If true, tasks must produce commits to be marked successful (default: true)
}

// RevisionIssue represents an issue identified during synthesis that needs to be addressed
type RevisionIssue struct {
	TaskID      string   `json:"task_id"`               // Task ID that needs revision (empty for cross-cutting issues)
	Description string   `json:"description"`           // Description of the issue
	Files       []string `json:"files,omitempty"`       // Files affected by the issue
	Severity    string   `json:"severity,omitempty"`    // "critical", "major", "minor"
	Suggestion  string   `json:"suggestion,omitempty"`  // Suggested fix
}

// RevisionState tracks the state of the revision phase
type RevisionState struct {
	Issues           []RevisionIssue   `json:"issues"`                      // Issues identified during synthesis
	RevisionRound    int               `json:"revision_round"`              // Current revision iteration (starts at 1)
	MaxRevisions     int               `json:"max_revisions"`               // Maximum allowed revision rounds
	TasksToRevise    []string          `json:"tasks_to_revise,omitempty"`   // Task IDs that need revision
	RevisedTasks     []string          `json:"revised_tasks,omitempty"`     // Task IDs that have been revised
	RevisionPrompts  map[string]string `json:"revision_prompts,omitempty"`  // Task ID -> revision prompt
	StartedAt        *time.Time        `json:"started_at,omitempty"`
	CompletedAt      *time.Time        `json:"completed_at,omitempty"`
}

// IsComplete returns true if all tasks have been revised
func (r *RevisionState) IsComplete() bool {
	return len(r.RevisedTasks) >= len(r.TasksToRevise)
}

// TaskWorktreeInfo holds information about a task's worktree for consolidation
type TaskWorktreeInfo struct {
	TaskID       string `json:"task_id"`
	TaskTitle    string `json:"task_title"`
	WorktreePath string `json:"worktree_path"`
	Branch       string `json:"branch"`
}

// TaskRetryState tracks retry attempts for a task that produces no commits
type TaskRetryState struct {
	TaskID       string `json:"task_id"`
	RetryCount   int    `json:"retry_count"`
	MaxRetries   int    `json:"max_retries"`
	LastError    string `json:"last_error,omitempty"`
	CommitCounts []int  `json:"commit_counts,omitempty"` // Commits per attempt (for debugging)
}

// GroupDecisionState tracks state when a group has partial success/failure
type GroupDecisionState struct {
	GroupIndex       int      `json:"group_index"`
	SucceededTasks   []string `json:"succeeded_tasks"`   // Tasks with verified commits
	FailedTasks      []string `json:"failed_tasks"`      // Tasks that failed or produced no commits
	AwaitingDecision bool     `json:"awaiting_decision"` // True when paused for user input
}

// UltraPlanSession represents an ultra-plan orchestration session
type UltraPlanSession struct {
	ID            string          `json:"id"`
	Objective     string          `json:"objective"`
	Plan          *PlanSpec       `json:"plan,omitempty"`
	Phase         UltraPlanPhase  `json:"phase"`
	Config        UltraPlanConfig `json:"config"`
	CoordinatorID string          `json:"coordinator_id,omitempty"` // Instance ID of the planning coordinator

	// Multi-pass planning state
	CandidatePlans        []*PlanSpec  `json:"candidate_plans,omitempty"`        // Plans from each coordinator (multi-pass)
	PlanCoordinatorIDs    []string     `json:"plan_coordinator_ids,omitempty"`   // Instance IDs of planning coordinators
	ProcessedCoordinators map[int]bool `json:"processed_coordinators,omitempty"` // Tracks which coordinators have completed (index -> completed)
	PlanManagerID         string       `json:"plan_manager_id,omitempty"`        // Instance ID of the coordinator-manager
	SelectedPlanIndex     int          `json:"selected_plan_index,omitempty"`    // Index of selected plan (-1 if merged)

	SynthesisID     string            `json:"synthesis_id,omitempty"`     // Instance ID of the synthesis reviewer
	RevisionID      string            `json:"revision_id,omitempty"`      // Instance ID of the current revision coordinator
	ConsolidationID string            `json:"consolidation_id,omitempty"` // Instance ID of the consolidation agent
	TaskToInstance  map[string]string `json:"task_to_instance"`           // PlannedTask.ID -> Instance.ID
	CompletedTasks  []string          `json:"completed_tasks"`
	FailedTasks     []string          `json:"failed_tasks"`
	CurrentGroup    int               `json:"current_group"` // Index into ExecutionOrder
	Created         time.Time         `json:"created"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
	Error           string            `json:"error,omitempty"` // Error message if failed

	// Revision state (persisted for recovery and display)
	Revision *RevisionState `json:"revision,omitempty"`

	// Synthesis completion context (populated from sentinel file)
	SynthesisCompletion *SynthesisCompletionFile `json:"synthesis_completion,omitempty"`

	// SynthesisAwaitingApproval is true when synthesis is complete but waiting for user review
	// User must press [s] to approve and proceed to revision/consolidation
	SynthesisAwaitingApproval bool `json:"synthesis_awaiting_approval,omitempty"`

	// Task worktree information for consolidation context
	TaskWorktrees []TaskWorktreeInfo `json:"task_worktrees,omitempty"`

	// Per-group consolidated branches: index -> branch name
	// After each group completes, parallel task branches are merged into one consolidated branch
	// The next group's tasks use this consolidated branch as their base
	GroupConsolidatedBranches []string `json:"group_consolidated_branches,omitempty"`

	// Per-group consolidator instance IDs: index -> instance ID
	// Each group has a dedicated Claude session that consolidates its task branches
	GroupConsolidatorIDs []string `json:"group_consolidator_ids,omitempty"`

	// Per-group consolidation contexts: index -> completion file data
	// Stores the context from each group's consolidator to pass to the next group
	GroupConsolidationContexts []*GroupConsolidationCompletionFile `json:"group_consolidation_contexts,omitempty"`

	// Consolidation results (persisted for recovery and display)
	Consolidation *ConsolidationState `json:"consolidation,omitempty"`
	PRUrls        []string            `json:"pr_urls,omitempty"`

	// Task retry tracking: task ID -> retry state
	TaskRetries map[string]*TaskRetryState `json:"task_retries,omitempty"`

	// Group decision state (set when group has mix of success/failure)
	GroupDecision *GroupDecisionState `json:"group_decision,omitempty"`

	// Verified commit counts per task (populated after task completion)
	TaskCommitCounts map[string]int `json:"task_commit_counts,omitempty"`
}

// GetTask returns a planned task by ID
func (s *UltraPlanSession) GetTask(taskID string) *PlannedTask {
	if s.Plan == nil {
		return nil
	}
	for i := range s.Plan.Tasks {
		if s.Plan.Tasks[i].ID == taskID {
			return &s.Plan.Tasks[i]
		}
	}
	return nil
}

// IsTaskReady returns true if all dependencies for a task have completed
func (s *UltraPlanSession) IsTaskReady(taskID string) bool {
	task := s.GetTask(taskID)
	if task == nil {
		return false
	}
	for _, depID := range task.DependsOn {
		found := false
		for _, completed := range s.CompletedTasks {
			if completed == depID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// GetReadyTasks returns all tasks that are ready to execute (in current group, dependencies met, not yet started)
// This respects group boundaries - only tasks from the current execution group are considered
func (s *UltraPlanSession) GetReadyTasks() []string {
	if s.Plan == nil {
		return nil
	}

	// CRITICAL: Never return tasks from the next group while awaiting a decision
	// about a partial failure. The next group cannot start without consolidation.
	if s.GroupDecision != nil && s.GroupDecision.AwaitingDecision {
		return nil
	}

	// Build set of started/completed tasks
	startedOrCompleted := make(map[string]bool)
	for _, taskID := range s.CompletedTasks {
		startedOrCompleted[taskID] = true
	}
	for _, taskID := range s.FailedTasks {
		startedOrCompleted[taskID] = true
	}
	for taskID := range s.TaskToInstance {
		startedOrCompleted[taskID] = true
	}

	// If we have execution order groups defined, only consider tasks from the current group
	if len(s.Plan.ExecutionOrder) > 0 {
		// Ensure CurrentGroup is valid
		if s.CurrentGroup >= len(s.Plan.ExecutionOrder) {
			return nil // All groups complete
		}

		currentGroupTasks := make(map[string]bool)
		for _, taskID := range s.Plan.ExecutionOrder[s.CurrentGroup] {
			currentGroupTasks[taskID] = true
		}

		var ready []string
		for _, taskID := range s.Plan.ExecutionOrder[s.CurrentGroup] {
			if startedOrCompleted[taskID] {
				continue
			}
			if s.IsTaskReady(taskID) {
				ready = append(ready, taskID)
			}
		}
		return ready
	}

	// Fallback: no execution order defined, use dependency-only logic
	var ready []string
	for _, task := range s.Plan.Tasks {
		if startedOrCompleted[task.ID] {
			continue
		}
		if s.IsTaskReady(task.ID) {
			ready = append(ready, task.ID)
		}
	}
	return ready
}

// IsCurrentGroupComplete returns true if all tasks in the current group are completed or failed
func (s *UltraPlanSession) IsCurrentGroupComplete() bool {
	if s.Plan == nil || len(s.Plan.ExecutionOrder) == 0 {
		return false
	}
	if s.CurrentGroup >= len(s.Plan.ExecutionOrder) {
		return true // Already past last group
	}

	// Build set of completed/failed tasks
	doneSet := make(map[string]bool)
	for _, taskID := range s.CompletedTasks {
		doneSet[taskID] = true
	}
	for _, taskID := range s.FailedTasks {
		doneSet[taskID] = true
	}

	// Check if all tasks in current group are done
	for _, taskID := range s.Plan.ExecutionOrder[s.CurrentGroup] {
		if !doneSet[taskID] {
			return false
		}
	}
	return true
}

// AdvanceGroupIfComplete checks if the current group is complete and advances to the next group.
// Returns true if the group was advanced, along with the previous group index.
func (s *UltraPlanSession) AdvanceGroupIfComplete() (advanced bool, previousGroup int) {
	if !s.IsCurrentGroupComplete() {
		return false, s.CurrentGroup
	}

	previousGroup = s.CurrentGroup
	s.CurrentGroup++
	return true, previousGroup
}

// HasMoreGroups returns true if there are more groups to execute after the current one
func (s *UltraPlanSession) HasMoreGroups() bool {
	if s.Plan == nil || len(s.Plan.ExecutionOrder) == 0 {
		return false
	}
	return s.CurrentGroup < len(s.Plan.ExecutionOrder)
}

// Progress returns the completion progress as a percentage (0-100)
func (s *UltraPlanSession) Progress() float64 {
	if s.Plan == nil || len(s.Plan.Tasks) == 0 {
		return 0
	}
	return float64(len(s.CompletedTasks)) / float64(len(s.Plan.Tasks)) * 100
}
