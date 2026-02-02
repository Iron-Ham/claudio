// Package ultraplan provides types and utilities for Ultra-Plan orchestration.
package ultraplan

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Config holds configuration for an ultra-plan session.
type Config struct {
	MaxParallel int  `json:"max_parallel"` // Maximum concurrent child sessions
	DryRun      bool `json:"dry_run"`      // Run planning only, don't execute
	NoSynthesis bool `json:"no_synthesis"` // Skip synthesis phase after execution
	AutoApprove bool `json:"auto_approve"` // Auto-approve spawned tasks without confirmation
	Review      bool `json:"review"`       // Force plan editor to open for review (overrides AutoApprove)
	MultiPass   bool `json:"multi_pass"`   // Enable multi-pass planning with plan comparison

	// Consolidation settings
	ConsolidationMode ConsolidationMode `json:"consolidation_mode,omitempty"` // "stacked" or "single"
	CreateDraftPRs    bool              `json:"create_draft_prs"`             // Create PRs as drafts
	PRLabels          []string          `json:"pr_labels,omitempty"`          // Labels to add to PRs
	BranchPrefix      string            `json:"branch_prefix,omitempty"`      // Branch prefix for consolidated branches

	// Task verification settings
	MaxTaskRetries         int  `json:"max_task_retries,omitempty"` // Max retry attempts for tasks with no commits (default: 3)
	RequireVerifiedCommits bool `json:"require_verified_commits"`   // If true, tasks must produce commits to be marked successful (default: true)
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
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

// RevisionState tracks the state of the revision phase.
type RevisionState struct {
	Issues          []RevisionIssue   `json:"issues"`                     // Issues identified during synthesis
	RevisionRound   int               `json:"revision_round"`             // Current revision iteration (starts at 1)
	MaxRevisions    int               `json:"max_revisions"`              // Maximum allowed revision rounds
	TasksToRevise   []string          `json:"tasks_to_revise,omitempty"`  // Task IDs that need revision
	RevisedTasks    []string          `json:"revised_tasks,omitempty"`    // Task IDs that have been revised
	RevisionPrompts map[string]string `json:"revision_prompts,omitempty"` // Task ID -> revision prompt
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
}

// NewRevisionState creates a new revision state.
func NewRevisionState(issues []RevisionIssue) *RevisionState {
	return &RevisionState{
		Issues:          issues,
		RevisionRound:   1,
		MaxRevisions:    3, // Default max revision rounds
		TasksToRevise:   extractTasksToRevise(issues),
		RevisedTasks:    make([]string, 0),
		RevisionPrompts: make(map[string]string),
	}
}

// extractTasksToRevise extracts unique task IDs from issues.
func extractTasksToRevise(issues []RevisionIssue) []string {
	taskSet := make(map[string]bool)
	var tasks []string
	for _, issue := range issues {
		if issue.TaskID != "" && !taskSet[issue.TaskID] {
			taskSet[issue.TaskID] = true
			tasks = append(tasks, issue.TaskID)
		}
	}
	return tasks
}

// IsComplete returns true if all tasks have been revised.
func (r *RevisionState) IsComplete() bool {
	return len(r.RevisedTasks) >= len(r.TasksToRevise)
}

// SynthesisCompletionFile represents the sentinel file written when synthesis completes.
type SynthesisCompletionFile struct {
	HasIssues   bool   `json:"has_issues"`
	IssueCount  int    `json:"issue_count"`
	Summary     string `json:"summary"`
	CompletedAt string `json:"completed_at"`
}

// GroupConsolidationCompletionFile represents context passed between group consolidations.
type GroupConsolidationCompletionFile struct {
	GroupIndex   int      `json:"group_index"`
	BranchName   string   `json:"branch_name"`
	TasksSummary string   `json:"tasks_summary"`
	FilesChanged []string `json:"files_changed"`
	CommitCount  int      `json:"commit_count"`
	Notes        string   `json:"notes,omitempty"`
	CompletedAt  string   `json:"completed_at"`
}

// Session represents an ultra-plan orchestration session.
type Session struct {
	ID            string         `json:"id"`
	Objective     string         `json:"objective"`
	Plan          *PlanSpec      `json:"plan,omitempty"`
	Phase         UltraPlanPhase `json:"phase"`
	Config        Config         `json:"config"`
	CoordinatorID string         `json:"coordinator_id,omitempty"` // Instance ID of the planning coordinator

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
	// Each group has a dedicated backend session that consolidates its task branches
	GroupConsolidatorIDs []string `json:"group_consolidator_ids,omitempty"`

	// Per-group consolidation contexts: index -> completion file data
	// Stores the context from each group's consolidator to pass to the next group
	GroupConsolidationContexts []*GroupConsolidationCompletionFile `json:"group_consolidation_contexts,omitempty"`

	// Consolidation results (persisted for recovery and display)
	Consolidation *ConsolidatorState `json:"consolidation,omitempty"`
	PRUrls        []string           `json:"pr_urls,omitempty"`

	// Task retry tracking: task ID -> retry state
	TaskRetries map[string]*TaskRetryState `json:"task_retries,omitempty"`

	// Group decision state (set when group has mix of success/failure)
	GroupDecision *GroupDecisionState `json:"group_decision,omitempty"`

	// Verified commit counts per task (populated after task completion)
	TaskCommitCounts map[string]int `json:"task_commit_counts,omitempty"`
}

// NewSession creates a new ultra-plan session.
func NewSession(objective string, config Config) *Session {
	return &Session{
		ID:               generateID(),
		Objective:        objective,
		Phase:            PhasePlanning,
		Config:           config,
		TaskToInstance:   make(map[string]string),
		CompletedTasks:   make([]string, 0),
		FailedTasks:      make([]string, 0),
		Created:          time.Now(),
		TaskRetries:      make(map[string]*TaskRetryState),
		TaskCommitCounts: make(map[string]int),
		// Multi-pass planning state
		CandidatePlans:        make([]*PlanSpec, 0),
		PlanCoordinatorIDs:    make([]string, 0),
		ProcessedCoordinators: make(map[int]bool),
		SelectedPlanIndex:     -1,
	}
}

// GetTask returns a planned task by ID.
func (s *Session) GetTask(taskID string) *PlannedTask {
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

// IsTaskReady returns true if all dependencies for a task have completed.
func (s *Session) IsTaskReady(taskID string) bool {
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

// GetReadyTasks returns all tasks that are ready to execute (in current group, dependencies met, not yet started).
// This respects group boundaries - only tasks from the current execution group are considered.
func (s *Session) GetReadyTasks() []string {
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

// IsCurrentGroupComplete returns true if all tasks in the current group are completed or failed.
func (s *Session) IsCurrentGroupComplete() bool {
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
func (s *Session) AdvanceGroupIfComplete() (advanced bool, previousGroup int) {
	if !s.IsCurrentGroupComplete() {
		return false, s.CurrentGroup
	}

	previousGroup = s.CurrentGroup
	s.CurrentGroup++
	return true, previousGroup
}

// HasMoreGroups returns true if there are more groups to execute after the current one.
func (s *Session) HasMoreGroups() bool {
	if s.Plan == nil || len(s.Plan.ExecutionOrder) == 0 {
		return false
	}
	return s.CurrentGroup < len(s.Plan.ExecutionOrder)
}

// Progress returns the completion progress as a percentage (0-100).
func (s *Session) Progress() float64 {
	if s.Plan == nil || len(s.Plan.Tasks) == 0 {
		return 0
	}
	return float64(len(s.CompletedTasks)) / float64(len(s.Plan.Tasks)) * 100
}

// generateID creates a short random hex ID.
// Falls back to timestamp-based ID if crypto/rand fails.
func generateID() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		// This is better than returning "00000000" which could cause collisions
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
	}
	return hex.EncodeToString(bytes)
}

// GenerateID generates a new random 8-character hex ID.
// Exported for use by other packages that need to generate session/instance IDs.
func GenerateID() string {
	return generateID()
}
