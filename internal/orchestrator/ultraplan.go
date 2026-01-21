package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/issue"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator/retry"
	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
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

// TaskComplexity represents the estimated complexity of a planned task
type TaskComplexity string

const (
	ComplexityLow    TaskComplexity = "low"
	ComplexityMedium TaskComplexity = "medium"
	ComplexityHigh   TaskComplexity = "high"
)

// StepType represents the type of an ultraplan step for restart/input mode operations
type StepType string

const (
	StepTypePlanning          StepType = "planning"
	StepTypePlanManager       StepType = "plan_manager"
	StepTypeTask              StepType = "task"
	StepTypeSynthesis         StepType = "synthesis"
	StepTypeRevision          StepType = "revision"
	StepTypeConsolidation     StepType = "consolidation"
	StepTypeGroupConsolidator StepType = "group_consolidator"
)

// StepInfo contains information about an ultraplan step for restart/input operations
type StepInfo struct {
	Type       StepType
	InstanceID string
	TaskID     string // Only set for task and group_consolidator steps
	GroupIndex int    // Only set for task and group_consolidator steps (-1 otherwise)
	Label      string // Human-readable description
}

// PlannedTask represents a single decomposed task from the planning phase
type PlannedTask struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`     // Detailed task prompt for child session
	Files         []string       `json:"files,omitempty"` // Expected files to be modified
	DependsOn     []string       `json:"depends_on"`      // Task IDs this depends on
	Priority      int            `json:"priority"`        // Execution priority (lower = earlier)
	EstComplexity TaskComplexity `json:"est_complexity"`
	IssueURL      string         `json:"issue_url,omitempty"` // External issue tracker URL (GitHub, Linear, Notion, etc.)
	NoCode        bool           `json:"no_code,omitempty"`   // Task doesn't require code changes (verification/testing tasks)
}

// GetID returns the task's unique identifier.
// This method enables PlannedTask to satisfy the prompt.PlannedTaskLike interface.
func (t *PlannedTask) GetID() string { return t.ID }

// GetTitle returns the task's short title.
// This method enables PlannedTask to satisfy the prompt.PlannedTaskLike interface.
func (t *PlannedTask) GetTitle() string { return t.Title }

// GetDescription returns the detailed task instructions.
// This method enables PlannedTask to satisfy the prompt.PlannedTaskLike interface.
func (t *PlannedTask) GetDescription() string { return t.Description }

// GetFiles returns the list of files this task is expected to modify.
// This method enables PlannedTask to satisfy the prompt.PlannedTaskLike interface.
func (t *PlannedTask) GetFiles() []string { return t.Files }

// GetDependsOn returns the IDs of tasks this task depends on.
// This method enables PlannedTask to satisfy the prompt.PlannedTaskLike interface.
func (t *PlannedTask) GetDependsOn() []string { return t.DependsOn }

// GetPriority returns the task's execution priority (lower = earlier).
// This method enables PlannedTask to satisfy the prompt.PlannedTaskLike interface.
func (t *PlannedTask) GetPriority() int { return t.Priority }

// GetEstComplexity returns the estimated complexity as a string.
// This method enables PlannedTask to satisfy the prompt.PlannedTaskLike interface.
func (t *PlannedTask) GetEstComplexity() string { return string(t.EstComplexity) }

// PlanSpec represents the output of the planning phase
type PlanSpec struct {
	ID              string              `json:"id"`
	Objective       string              `json:"objective"` // Original user request
	Summary         string              `json:"summary"`   // Executive summary of the plan
	Tasks           []PlannedTask       `json:"tasks"`
	DependencyGraph map[string][]string `json:"dependency_graph"` // task_id -> depends_on[]
	ExecutionOrder  [][]string          `json:"execution_order"`  // Groups of parallelizable tasks
	Insights        []string            `json:"insights"`         // Key findings from exploration
	Constraints     []string            `json:"constraints"`      // Identified constraints/risks
	CreatedAt       time.Time           `json:"created_at"`
}

// PlannedTaskLike is an interface that PlannedTask satisfies.
// This enables the prompt package to work with tasks via interface without
// creating an import cycle. The prompt package can define its own compatible
// interface and use these getter methods.
type PlannedTaskLike interface {
	GetID() string
	GetTitle() string
	GetDescription() string
	GetFiles() []string
	GetDependsOn() []string
	GetPriority() int
	GetEstComplexity() string
}

// GetSummary returns the executive summary of the plan.
// This method enables PlanSpec to satisfy prompt.PlanLike interface.
func (p *PlanSpec) GetSummary() string { return p.Summary }

// GetTasks returns the planned tasks as a slice of PlannedTaskLike interfaces.
// This method enables PlanSpec to satisfy prompt.PlanLike interface.
func (p *PlanSpec) GetTasks() []PlannedTaskLike {
	result := make([]PlannedTaskLike, len(p.Tasks))
	for i := range p.Tasks {
		result[i] = &p.Tasks[i]
	}
	return result
}

// GetExecutionOrder returns the groups of parallelizable task IDs.
// This method enables PlanSpec to satisfy prompt.PlanLike interface.
func (p *PlanSpec) GetExecutionOrder() [][]string { return p.ExecutionOrder }

// GetInsights returns key findings from codebase exploration.
// This method enables PlanSpec to satisfy prompt.PlanLike interface.
func (p *PlanSpec) GetInsights() []string { return p.Insights }

// GetConstraints returns identified constraints and risks.
// This method enables PlanSpec to satisfy prompt.PlanLike interface.
func (p *PlanSpec) GetConstraints() []string { return p.Constraints }

// UltraPlanConfig holds configuration for an ultra-plan session
type UltraPlanConfig struct {
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

// DefaultUltraPlanConfig returns the default configuration
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

// RevisionIssue represents an issue identified during synthesis that needs to be addressed
type RevisionIssue struct {
	TaskID      string   `json:"task_id"`              // Task ID that needs revision (empty for cross-cutting issues)
	Description string   `json:"description"`          // Description of the issue
	Files       []string `json:"files,omitempty"`      // Files affected by the issue
	Severity    string   `json:"severity,omitempty"`   // "critical", "major", "minor"
	Suggestion  string   `json:"suggestion,omitempty"` // Suggested fix
}

// PlanScore represents the evaluation of a single candidate plan
type PlanScore struct {
	Strategy   string `json:"strategy"`
	Score      int    `json:"score"`
	Strengths  string `json:"strengths"`
	Weaknesses string `json:"weaknesses"`
}

// PlanDecision captures the coordinator-manager's decision when evaluating multiple plans
type PlanDecision struct {
	Action        string      `json:"action"`         // "select" or "merge"
	SelectedIndex int         `json:"selected_index"` // 0-2 or -1 for merge
	Reasoning     string      `json:"reasoning"`
	PlanScores    []PlanScore `json:"plan_scores"`
}

// RevisionState tracks the state of the revision phase
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

// NewRevisionState creates a new revision state
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

// extractTasksToRevise extracts unique task IDs from issues
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

// IsComplete returns true if all tasks have been revised
func (r *RevisionState) IsComplete() bool {
	return len(r.RevisedTasks) >= len(r.TasksToRevise)
}

// ParseRevisionIssuesFromOutput extracts revision issues from synthesis output
// It looks for JSON wrapped in <revision_issues></revision_issues> tags
func ParseRevisionIssuesFromOutput(output string) ([]RevisionIssue, error) {
	// Look for <revision_issues>...</revision_issues> tags
	re := regexp.MustCompile(`(?s)<revision_issues>\s*(.*?)\s*</revision_issues>`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		// No revision issues block found - assume no issues
		return nil, nil
	}

	jsonStr := strings.TrimSpace(matches[1])

	// Handle empty array
	if jsonStr == "[]" || jsonStr == "" {
		return nil, nil
	}

	// Parse the JSON array
	var issues []RevisionIssue
	if err := json.Unmarshal([]byte(jsonStr), &issues); err != nil {
		return nil, fmt.Errorf("failed to parse revision issues JSON: %w", err)
	}

	// Filter to only include issues with actual content
	var validIssues []RevisionIssue
	for _, issue := range issues {
		if issue.Description != "" {
			validIssues = append(validIssues, issue)
		}
	}

	return validIssues, nil
}

// ParsePlanDecisionFromOutput extracts the plan decision from coordinator-manager output
// It looks for JSON wrapped in <plan_decision></plan_decision> tags
func ParsePlanDecisionFromOutput(output string) (*PlanDecision, error) {
	// Look for <plan_decision>...</plan_decision> tags
	re := regexp.MustCompile(`(?s)<plan_decision>\s*(.*?)\s*</plan_decision>`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return nil, fmt.Errorf("no plan decision found in output (expected <plan_decision>JSON</plan_decision>)")
	}

	jsonStr := strings.TrimSpace(matches[1])

	if jsonStr == "" {
		return nil, fmt.Errorf("empty plan decision block")
	}

	// Parse the JSON
	var decision PlanDecision
	if err := json.Unmarshal([]byte(jsonStr), &decision); err != nil {
		return nil, fmt.Errorf("failed to parse plan decision JSON: %w", err)
	}

	// Validate the decision
	if decision.Action != "select" && decision.Action != "merge" {
		return nil, fmt.Errorf("invalid plan decision action: %q (expected \"select\" or \"merge\")", decision.Action)
	}

	if decision.Action == "select" && (decision.SelectedIndex < 0 || decision.SelectedIndex > 2) {
		return nil, fmt.Errorf("invalid selected_index for select action: %d (expected 0-2)", decision.SelectedIndex)
	}

	if decision.Action == "merge" && decision.SelectedIndex != -1 {
		return nil, fmt.Errorf("selected_index should be -1 for merge action, got %d", decision.SelectedIndex)
	}

	return &decision, nil
}

// TaskWorktreeInfo holds information about a task's worktree for consolidation
type TaskWorktreeInfo struct {
	TaskID       string `json:"task_id"`
	TaskTitle    string `json:"task_title"`
	WorktreePath string `json:"worktree_path"`
	Branch       string `json:"branch"`
}

// TaskRetryState tracks retry attempts for a task that produces no commits.
// This is an alias to retry.TaskState for backward compatibility.
type TaskRetryState = retry.TaskState

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
	GroupID       string          `json:"group_id,omitempty"` // Link to InstanceGroup for TUI display
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
	GroupConsolidationContexts []*types.GroupConsolidationCompletionFile `json:"group_consolidation_contexts,omitempty"`

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

// NewUltraPlanSession creates a new ultra-plan session
func NewUltraPlanSession(objective string, config UltraPlanConfig) *UltraPlanSession {
	return &UltraPlanSession{
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

// CoordinatorEvent represents an event from the coordinator during execution
type CoordinatorEvent struct {
	Type       CoordinatorEventType `json:"type"`
	TaskID     string               `json:"task_id,omitempty"`
	InstanceID string               `json:"instance_id,omitempty"`
	Message    string               `json:"message,omitempty"`
	Timestamp  time.Time            `json:"timestamp"`
	// Multi-pass planning fields
	PlanIndex int    `json:"plan_index,omitempty"` // Which plan was generated/selected (0-indexed)
	Strategy  string `json:"strategy,omitempty"`   // Planning strategy name (e.g., "maximize-parallelism", "minimize-complexity", "balanced-approach")
}

// CoordinatorEventType represents the type of coordinator event
type CoordinatorEventType string

const (
	EventTaskStarted   CoordinatorEventType = "task_started"
	EventTaskComplete  CoordinatorEventType = "task_complete"
	EventTaskFailed    CoordinatorEventType = "task_failed"
	EventTaskBlocked   CoordinatorEventType = "task_blocked"
	EventGroupComplete CoordinatorEventType = "group_complete"
	EventPhaseChange   CoordinatorEventType = "phase_change"
	EventConflict      CoordinatorEventType = "conflict"
	EventPlanReady     CoordinatorEventType = "plan_ready"

	// Multi-pass planning events
	EventMultiPassPlanGenerated CoordinatorEventType = "multipass_plan_generated" // One coordinator finished planning
	EventAllPlansGenerated      CoordinatorEventType = "all_plans_generated"      // All coordinators finished
	EventPlanSelectionStarted   CoordinatorEventType = "plan_selection_started"   // Manager started evaluating
	EventPlanSelected           CoordinatorEventType = "plan_selected"            // Final plan chosen
)

// UltraPlanManager manages the execution of an ultra-plan session
type UltraPlanManager struct {
	session      *UltraPlanSession
	orch         *Orchestrator
	baseSession  *Session // The underlying Claudio session
	logger       *logging.Logger
	issueService *issue.Service

	// Event handling
	eventChan     chan CoordinatorEvent
	eventCallback func(CoordinatorEvent)

	// Synchronization
	mu sync.RWMutex
	wg sync.WaitGroup

	// Cancellation
	stopChan chan struct{}
	stopped  bool
}

// NewUltraPlanManager creates a new ultra-plan manager.
// The logger parameter should be passed from the Coordinator and will be used
// for structured logging throughout plan lifecycle events.
func NewUltraPlanManager(orch *Orchestrator, baseSession *Session, ultraSession *UltraPlanSession, logger *logging.Logger) *UltraPlanManager {
	if logger == nil {
		logger = logging.NopLogger()
	}
	planLogger := logger.WithPhase("ultraplan")
	return &UltraPlanManager{
		session:      ultraSession,
		orch:         orch,
		baseSession:  baseSession,
		logger:       planLogger,
		issueService: issue.NewService(planLogger),
		eventChan:    make(chan CoordinatorEvent, 100),
		stopChan:     make(chan struct{}),
	}
}

// SetEventCallback sets the callback for coordinator events
func (m *UltraPlanManager) SetEventCallback(cb func(CoordinatorEvent)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventCallback = cb
}

// Session returns the ultra-plan session
func (m *UltraPlanManager) Session() *UltraPlanSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session
}

// emitEvent sends an event to the event channel and callback
func (m *UltraPlanManager) emitEvent(event CoordinatorEvent) {
	event.Timestamp = time.Now()

	// Non-blocking send to channel
	select {
	case m.eventChan <- event:
	default:
		// Channel full, skip
	}

	// Call callback if set
	m.mu.RLock()
	cb := m.eventCallback
	m.mu.RUnlock()
	if cb != nil {
		cb(event)
	}
}

// Events returns the event channel for monitoring
func (m *UltraPlanManager) Events() <-chan CoordinatorEvent {
	return m.eventChan
}

// Stop stops the ultra-plan execution
func (m *UltraPlanManager) Stop() {
	m.mu.Lock()
	if !m.stopped {
		m.stopped = true
		close(m.stopChan)
	}
	m.mu.Unlock()

	// Wait for any running goroutines
	m.wg.Wait()
}

// SetPhase updates the session phase and emits an event
func (m *UltraPlanManager) SetPhase(phase UltraPlanPhase) {
	m.mu.Lock()
	m.session.Phase = phase
	m.mu.Unlock()

	m.emitEvent(CoordinatorEvent{
		Type:    EventPhaseChange,
		Message: string(phase),
	})
}

// SetPlan sets the plan on the session and logs the plan creation.
// The objective is truncated to 100 characters in the log for readability.
func (m *UltraPlanManager) SetPlan(plan *PlanSpec) {
	m.mu.Lock()
	m.session.Plan = plan
	m.mu.Unlock()

	if plan != nil {
		// Truncate objective for logging
		objective := plan.Objective
		if len(objective) > 100 {
			objective = objective[:97] + "..."
		}

		m.logger.Info("plan created",
			"plan_id", plan.ID,
			"task_count", len(plan.Tasks),
			"objective", objective,
		)
	}

	m.emitEvent(CoordinatorEvent{
		Type:    EventPlanReady,
		Message: "plan ready",
	})
}

// SetSelectedPlanIndex sets the selected plan index and logs the selection.
// This is used during multi-pass planning when a plan is chosen from candidates.
func (m *UltraPlanManager) SetSelectedPlanIndex(index int, action string) {
	m.mu.Lock()
	m.session.SelectedPlanIndex = index
	m.mu.Unlock()

	m.logger.Info("plan selected",
		"selected_index", index,
		"action", action,
	)

	m.emitEvent(CoordinatorEvent{
		Type:      EventPlanSelected,
		PlanIndex: index,
		Message:   action,
	})
}

// ValidatePlanWithLogging validates a plan and logs DEBUG-level information about the validation.
// It logs the dependency graph structure, execution order calculation, and any validation errors.
func (m *UltraPlanManager) ValidatePlanWithLogging(plan *PlanSpec) error {
	if plan == nil {
		m.logger.Error("plan validation failed", "error", "plan is nil")
		return fmt.Errorf("plan is nil")
	}

	// Log dependency graph construction at DEBUG level
	m.logger.Debug("dependency graph constructed",
		"task_count", len(plan.Tasks),
		"dependency_count", countDependencies(plan.DependencyGraph),
	)

	// Log execution order calculation at DEBUG level
	if len(plan.ExecutionOrder) > 0 {
		groupSizes := make([]int, len(plan.ExecutionOrder))
		for i, group := range plan.ExecutionOrder {
			groupSizes[i] = len(group)
		}
		m.logger.Debug("execution order calculated",
			"group_count", len(plan.ExecutionOrder),
			"group_sizes", groupSizes,
		)
	}

	// Perform validation
	err := ValidatePlan(plan)
	if err != nil {
		// Check if it's an invalid dependency error
		if strings.Contains(err.Error(), "depends on unknown task") {
			m.logger.Error("invalid dependency error", "error", err.Error())
		} else {
			m.logger.Debug("plan validation completed with error", "error", err.Error())
		}
		return err
	}

	m.logger.Debug("plan validation completed", "valid", true)
	return nil
}

// countDependencies counts the total number of dependencies in the graph
func countDependencies(deps map[string][]string) int {
	count := 0
	for _, d := range deps {
		count += len(d)
	}
	return count
}

// ParsePlanFromOutputWithLogging parses a plan from Claude's output and logs any errors.
// On success, it logs DEBUG-level information about the parsed plan.
// On failure, it logs ERROR-level information about the parsing failure.
func (m *UltraPlanManager) ParsePlanFromOutputWithLogging(output string, objective string) (*PlanSpec, error) {
	plan, err := ParsePlanFromOutput(output, objective)
	if err != nil {
		m.logger.Error("plan parsing failed",
			"error", err.Error(),
			"output_length", len(output),
		)
		return nil, err
	}

	m.logger.Debug("plan parsed successfully",
		"plan_id", plan.ID,
		"task_count", len(plan.Tasks),
	)
	return plan, nil
}

// ParsePlanFromFileWithLogging parses a plan from a file and logs any errors.
// On success, it logs DEBUG-level information about the parsed plan.
// On failure, it logs ERROR-level information about the parsing failure.
func (m *UltraPlanManager) ParsePlanFromFileWithLogging(filepath string, objective string) (*PlanSpec, error) {
	plan, err := ParsePlanFromFile(filepath, objective)
	if err != nil {
		m.logger.Error("plan parsing failed",
			"error", err.Error(),
			"filepath", filepath,
		)
		return nil, err
	}

	m.logger.Debug("plan parsed successfully",
		"plan_id", plan.ID,
		"task_count", len(plan.Tasks),
		"filepath", filepath,
	)
	return plan, nil
}

// StoreCandidatePlan stores a candidate plan at the given index with proper mutex protection.
// It initializes the CandidatePlans slice if needed, marks the coordinator as processed,
// and returns the count of non-nil plans collected.
// This method is safe for concurrent access from multiple goroutines.
// Pass nil for plan to mark a coordinator as completed but failed to produce a valid plan.
func (m *UltraPlanManager) StoreCandidatePlan(planIndex int, plan *PlanSpec) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize CandidatePlans slice if needed
	numCoordinators := len(m.session.PlanCoordinatorIDs)
	if len(m.session.CandidatePlans) < numCoordinators {
		newPlans := make([]*PlanSpec, numCoordinators)
		copy(newPlans, m.session.CandidatePlans)
		m.session.CandidatePlans = newPlans
	}

	// Initialize ProcessedCoordinators map if needed
	if m.session.ProcessedCoordinators == nil {
		m.session.ProcessedCoordinators = make(map[int]bool)
	}

	// Store the plan at the correct index and mark as processed
	if planIndex >= 0 && planIndex < len(m.session.CandidatePlans) {
		m.session.CandidatePlans[planIndex] = plan
		m.session.ProcessedCoordinators[planIndex] = true
	}

	// Count collected (non-nil) plans
	count := 0
	for _, p := range m.session.CandidatePlans {
		if p != nil {
			count++
		}
	}

	// Log candidate plan storage (INFO level for multi-pass planning)
	if plan != nil {
		m.logger.Info("candidate plan stored",
			"plan_index", planIndex,
			"task_count", len(plan.Tasks),
		)
	}

	return count
}

// CountCandidatePlans returns the number of non-nil candidate plans collected.
// This method is safe for concurrent access.
func (m *UltraPlanManager) CountCandidatePlans() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, p := range m.session.CandidatePlans {
		if p != nil {
			count++
		}
	}
	return count
}

// CountCoordinatorsCompleted returns the number of coordinators that have completed
// (regardless of whether they produced a valid plan or not).
// This method is safe for concurrent access.
func (m *UltraPlanManager) CountCoordinatorsCompleted() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session.ProcessedCoordinators == nil {
		return 0
	}
	return len(m.session.ProcessedCoordinators)
}

// MarkTaskComplete marks a task as completed and closes any linked external issue
func (m *UltraPlanManager) MarkTaskComplete(taskID string) {
	m.mu.Lock()
	// Defense-in-depth: check if task is already marked complete to prevent duplicates
	// This can happen due to race conditions between task monitoring and polling
	for _, completedID := range m.session.CompletedTasks {
		if completedID == taskID {
			m.mu.Unlock()
			return
		}
	}
	m.session.CompletedTasks = append(m.session.CompletedTasks, taskID)
	delete(m.session.TaskToInstance, taskID)
	task := m.session.GetTask(taskID)
	m.mu.Unlock()

	// Close linked external issue if present
	if task != nil && task.IssueURL != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := m.issueService.Close(ctx, task.IssueURL); err != nil {
				m.logger.Warn("failed to close linked issue",
					"task_id", taskID,
					"issue_url", task.IssueURL,
					"error", err,
				)
			}
		}()
	}

	m.emitEvent(CoordinatorEvent{
		Type:   EventTaskComplete,
		TaskID: taskID,
	})
}

// MarkTaskFailed marks a task as failed
func (m *UltraPlanManager) MarkTaskFailed(taskID string, reason string) {
	m.mu.Lock()
	// Defense-in-depth: check if task is already marked failed to prevent duplicates
	for _, failedID := range m.session.FailedTasks {
		if failedID == taskID {
			m.mu.Unlock()
			return
		}
	}
	// Also check if already completed (shouldn't happen, but be safe)
	for _, completedID := range m.session.CompletedTasks {
		if completedID == taskID {
			m.mu.Unlock()
			return
		}
	}
	m.session.FailedTasks = append(m.session.FailedTasks, taskID)
	delete(m.session.TaskToInstance, taskID)
	m.mu.Unlock()

	m.emitEvent(CoordinatorEvent{
		Type:    EventTaskFailed,
		TaskID:  taskID,
		Message: reason,
	})
}

// AssignTaskToInstance records the mapping from task to instance
func (m *UltraPlanManager) AssignTaskToInstance(taskID, instanceID string) {
	m.mu.Lock()
	m.session.TaskToInstance[taskID] = instanceID
	m.mu.Unlock()

	m.logger.Info("task assigned to instance",
		"task_id", taskID,
		"instance_id", instanceID,
	)

	m.emitEvent(CoordinatorEvent{
		Type:       EventTaskStarted,
		TaskID:     taskID,
		InstanceID: instanceID,
	})
}

// ParsePlanFromOutput extracts a PlanSpec from Claude's output
// It looks for JSON wrapped in <plan></plan> tags
func ParsePlanFromOutput(output string, objective string) (*PlanSpec, error) {
	// Look for <plan>...</plan> tags
	re := regexp.MustCompile(`(?s)<plan>\s*(.*?)\s*</plan>`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return nil, fmt.Errorf("no plan found in output (expected <plan>JSON</plan>)")
	}

	jsonStr := strings.TrimSpace(matches[1])

	// Parse the JSON
	var rawPlan struct {
		Summary     string        `json:"summary"`
		Tasks       []PlannedTask `json:"tasks"`
		Insights    []string      `json:"insights"`
		Constraints []string      `json:"constraints"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawPlan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	if len(rawPlan.Tasks) == 0 {
		return nil, fmt.Errorf("plan contains no tasks")
	}

	// Build the PlanSpec
	plan := &PlanSpec{
		ID:              generateID(),
		Objective:       objective,
		Summary:         rawPlan.Summary,
		Tasks:           rawPlan.Tasks,
		Insights:        rawPlan.Insights,
		Constraints:     rawPlan.Constraints,
		DependencyGraph: make(map[string][]string),
		CreatedAt:       time.Now(),
	}

	// Build dependency graph
	for _, task := range plan.Tasks {
		plan.DependencyGraph[task.ID] = task.DependsOn
	}

	// Calculate execution order (topological sort with parallel grouping)
	plan.ExecutionOrder = calculateExecutionOrder(plan.Tasks, plan.DependencyGraph)

	return plan, nil
}

// ParsePlanFromFile reads and parses a plan from a JSON file.
// It supports two formats:
//  1. Root-level format: {"summary": "...", "tasks": [...]}
//  2. Nested format: {"plan": {"summary": "...", "tasks": [...]}}
//
// It also handles alternative field names that Claude may generate:
//   - "depends" as alias for "depends_on"
//   - "complexity" as alias for "est_complexity"
func ParsePlanFromFile(filepath string, objective string) (*PlanSpec, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	// flexibleTask handles alternative field names that Claude may generate
	type flexibleTask struct {
		ID            string   `json:"id"`
		Title         string   `json:"title"`
		Description   string   `json:"description"`
		Files         []string `json:"files,omitempty"`
		DependsOn     []string `json:"depends_on"`
		Depends       []string `json:"depends"` // Alternative name
		Priority      int      `json:"priority"`
		EstComplexity string   `json:"est_complexity"`
		Complexity    string   `json:"complexity"`          // Alternative name
		IssueURL      string   `json:"issue_url,omitempty"` // External issue tracker URL
		NoCode        bool     `json:"no_code,omitempty"`   // Task doesn't require code changes
	}

	type planContent struct {
		Summary     string         `json:"summary"`
		Tasks       []flexibleTask `json:"tasks"`
		Insights    []string       `json:"insights"`
		Constraints []string       `json:"constraints"`
	}

	// Try parsing as root-level format first
	var rawPlan planContent
	if err := json.Unmarshal(data, &rawPlan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	// If no tasks found, try nested "plan" wrapper format
	if len(rawPlan.Tasks) == 0 {
		var wrapped struct {
			Plan planContent `json:"plan"`
		}
		nestedErr := json.Unmarshal(data, &wrapped)
		if nestedErr == nil && len(wrapped.Plan.Tasks) > 0 {
			rawPlan = wrapped.Plan
		} else if nestedErr != nil {
			// Both formats failed - provide more context about the nested parse failure
			return nil, fmt.Errorf("failed to parse plan JSON (tried both root-level and nested formats): %w", nestedErr)
		}
	}

	if len(rawPlan.Tasks) == 0 {
		return nil, fmt.Errorf("plan contains no tasks (checked both root-level and nested 'plan' wrapper formats)")
	}

	// Convert flexible tasks to PlannedTask, handling alternative field names
	tasks := make([]PlannedTask, len(rawPlan.Tasks))
	for i, ft := range rawPlan.Tasks {
		// Use depends_on if set, otherwise fall back to depends
		dependsOn := ft.DependsOn
		if len(dependsOn) == 0 && len(ft.Depends) > 0 {
			dependsOn = ft.Depends
		}

		// Use est_complexity if set, otherwise fall back to complexity
		complexity := ft.EstComplexity
		if complexity == "" && ft.Complexity != "" {
			complexity = ft.Complexity
		}

		tasks[i] = PlannedTask{
			ID:            ft.ID,
			Title:         ft.Title,
			Description:   ft.Description,
			Files:         ft.Files,
			DependsOn:     dependsOn,
			Priority:      ft.Priority,
			EstComplexity: TaskComplexity(complexity),
			IssueURL:      ft.IssueURL,
			NoCode:        ft.NoCode,
		}
	}

	// Build the PlanSpec
	plan := &PlanSpec{
		ID:              generateID(),
		Objective:       objective,
		Summary:         rawPlan.Summary,
		Tasks:           tasks,
		Insights:        rawPlan.Insights,
		Constraints:     rawPlan.Constraints,
		DependencyGraph: make(map[string][]string),
		CreatedAt:       time.Now(),
	}

	// Build dependency graph
	for _, task := range plan.Tasks {
		plan.DependencyGraph[task.ID] = task.DependsOn
	}

	// Calculate execution order (topological sort with parallel grouping)
	plan.ExecutionOrder = calculateExecutionOrder(plan.Tasks, plan.DependencyGraph)

	return plan, nil
}

// PlanFilePath returns the full path to the plan file for a given worktree
func PlanFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, PlanFileName)
}

// calculateExecutionOrder performs a topological sort and groups tasks that can run in parallel
func calculateExecutionOrder(tasks []PlannedTask, deps map[string][]string) [][]string {
	// Build in-degree map
	inDegree := make(map[string]int)
	taskSet := make(map[string]bool)
	for _, task := range tasks {
		taskSet[task.ID] = true
		inDegree[task.ID] = len(task.DependsOn)
	}

	// Find tasks with no dependencies (in-degree 0)
	var groups [][]string
	completed := make(map[string]bool)

	for len(completed) < len(tasks) {
		var currentGroup []string

		// Find all tasks that can run now (in-degree 0 and not completed)
		for _, task := range tasks {
			if completed[task.ID] {
				continue
			}
			if inDegree[task.ID] == 0 {
				currentGroup = append(currentGroup, task.ID)
			}
		}

		if len(currentGroup) == 0 {
			// Cycle detected or invalid graph
			break
		}

		// Sort by priority within the group
		taskPriority := make(map[string]int)
		for _, task := range tasks {
			taskPriority[task.ID] = task.Priority
		}
		sort.Slice(currentGroup, func(i, j int) bool {
			return taskPriority[currentGroup[i]] < taskPriority[currentGroup[j]]
		})

		groups = append(groups, currentGroup)

		// Mark these tasks as completed and update in-degrees
		for _, taskID := range currentGroup {
			completed[taskID] = true
			// Reduce in-degree for tasks that depend on this one
			for _, task := range tasks {
				for _, depID := range task.DependsOn {
					if depID == taskID {
						inDegree[task.ID]--
					}
				}
			}
		}
	}

	return groups
}

// EnsurePlanComputed fills in computed fields (DependencyGraph and ExecutionOrder)
// if they are missing. This is used when loading a plan file that only has tasks
// with depends_on fields but not the pre-computed graph and execution order.
func EnsurePlanComputed(plan *PlanSpec) {
	if plan == nil || len(plan.Tasks) == 0 {
		return
	}

	// Build DependencyGraph from task DependsOn fields if missing
	if len(plan.DependencyGraph) == 0 {
		plan.DependencyGraph = make(map[string][]string)
		for _, task := range plan.Tasks {
			plan.DependencyGraph[task.ID] = task.DependsOn
		}
	}

	// Calculate ExecutionOrder if missing
	if len(plan.ExecutionOrder) == 0 {
		plan.ExecutionOrder = calculateExecutionOrder(plan.Tasks, plan.DependencyGraph)
	}
}

// ValidatePlan checks the plan for validity (no cycles, valid dependencies)
func ValidatePlan(plan *PlanSpec) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	if len(plan.Tasks) == 0 {
		return fmt.Errorf("plan has no tasks")
	}

	// Check for valid task IDs in dependencies
	taskSet := make(map[string]bool)
	for _, task := range plan.Tasks {
		taskSet[task.ID] = true
	}

	for _, task := range plan.Tasks {
		for _, depID := range task.DependsOn {
			if !taskSet[depID] {
				return fmt.Errorf("task %s depends on unknown task %s", task.ID, depID)
			}
		}
	}

	// Check for cycles by verifying all tasks appear in execution order
	if plan.ExecutionOrder != nil {
		scheduledTasks := 0
		for _, group := range plan.ExecutionOrder {
			scheduledTasks += len(group)
		}
		if scheduledTasks < len(plan.Tasks) {
			return fmt.Errorf("dependency cycle detected: only %d of %d tasks can be scheduled",
				scheduledTasks, len(plan.Tasks))
		}
	}

	return nil
}

// PlanFileName is the name of the file where the planning agent writes its plan
const PlanFileName = ".claudio-plan.json"

// TaskCompletionFileName is imported from types package
const TaskCompletionFileName = types.TaskCompletionFileName

// TaskCompletionFilePath returns the full path to the task completion file for a given worktree
func TaskCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, TaskCompletionFileName)
}

// ParseTaskCompletionFile reads and parses a task completion file
func ParseTaskCompletionFile(worktreePath string) (*types.TaskCompletionFile, error) {
	completionPath := TaskCompletionFilePath(worktreePath)
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

// SynthesisCompletionFileName is the sentinel file that synthesis writes when complete
const SynthesisCompletionFileName = ".claudio-synthesis-complete.json"

// SynthesisCompletionFile represents the completion report from the synthesis phase
type SynthesisCompletionFile struct {
	Status           string          `json:"status"`            // "complete", "needs_revision"
	RevisionRound    int             `json:"revision_round"`    // Current round (0 for first synthesis)
	IssuesFound      []RevisionIssue `json:"issues_found"`      // All issues identified
	TasksAffected    []string        `json:"tasks_affected"`    // Task IDs needing revision
	IntegrationNotes string          `json:"integration_notes"` // Free-form observations about integration
	Recommendations  []string        `json:"recommendations"`   // Suggestions for consolidation phase
}

// SynthesisCompletionFilePath returns the full path to the synthesis completion file
func SynthesisCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, SynthesisCompletionFileName)
}

// ParseSynthesisCompletionFile reads and parses a synthesis completion file
func ParseSynthesisCompletionFile(worktreePath string) (*SynthesisCompletionFile, error) {
	completionPath := SynthesisCompletionFilePath(worktreePath)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion SynthesisCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse synthesis completion JSON: %w", err)
	}

	return &completion, nil
}

// RevisionCompletionFileName is the sentinel file that revision tasks write when complete
const RevisionCompletionFileName = ".claudio-revision-complete.json"

// RevisionCompletionFile represents the completion report from a revision task
type RevisionCompletionFile struct {
	TaskID          string   `json:"task_id"`
	RevisionRound   int      `json:"revision_round"`
	IssuesAddressed []string `json:"issues_addressed"` // Issue descriptions that were fixed
	Summary         string   `json:"summary"`          // What was changed
	FilesModified   []string `json:"files_modified"`
	RemainingIssues []string `json:"remaining_issues"` // Issues that couldn't be fixed
}

// RevisionCompletionFilePath returns the full path to the revision completion file
func RevisionCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, RevisionCompletionFileName)
}

// ParseRevisionCompletionFile reads and parses a revision completion file
func ParseRevisionCompletionFile(worktreePath string) (*RevisionCompletionFile, error) {
	completionPath := RevisionCompletionFilePath(worktreePath)
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

// ConsolidationCompletionFileName is the sentinel file that consolidation writes when complete
const ConsolidationCompletionFileName = ".claudio-consolidation-complete.json"

// ConsolidationCompletionFile represents the completion report from consolidation
type ConsolidationCompletionFile struct {
	Status           string                   `json:"status"` // "complete", "partial", "failed"
	Mode             string                   `json:"mode"`   // "stacked" or "single"
	GroupResults     []GroupConsolidationInfo `json:"group_results"`
	PRsCreated       []PRInfo                 `json:"prs_created"`
	SynthesisContext *SynthesisCompletionFile `json:"synthesis_context,omitempty"`
	TotalCommits     int                      `json:"total_commits"`
	FilesChanged     []string                 `json:"files_changed"`
}

// GroupConsolidationInfo holds info about a consolidated group
type GroupConsolidationInfo struct {
	GroupIndex    int      `json:"group_index"`
	BranchName    string   `json:"branch_name"`
	TasksIncluded []string `json:"tasks_included"`
	CommitCount   int      `json:"commit_count"`
	Success       bool     `json:"success"`
}

// PRInfo holds information about a created PR
type PRInfo struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	GroupIndex int    `json:"group_index"`
}

// ConsolidationCompletionFilePath returns the full path to the consolidation completion file
func ConsolidationCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, ConsolidationCompletionFileName)
}

// ParseConsolidationCompletionFile reads and parses a consolidation completion file
func ParseConsolidationCompletionFile(worktreePath string) (*ConsolidationCompletionFile, error) {
	completionPath := ConsolidationCompletionFilePath(worktreePath)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion ConsolidationCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse consolidation completion JSON: %w", err)
	}

	return &completion, nil
}

// GroupConsolidationCompletionFileName is imported from types package
const GroupConsolidationCompletionFileName = types.GroupConsolidationCompletionFileName

// GroupConsolidationCompletionFilePath returns the full path to the group consolidation completion file
func GroupConsolidationCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, GroupConsolidationCompletionFileName)
}

// ParseGroupConsolidationCompletionFile reads and parses a group consolidation completion file
func ParseGroupConsolidationCompletionFile(worktreePath string) (*types.GroupConsolidationCompletionFile, error) {
	completionPath := GroupConsolidationCompletionFilePath(worktreePath)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion types.GroupConsolidationCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse group consolidation completion JSON: %w", err)
	}

	return &completion, nil
}

// PlanningPromptTemplate is the prompt used for the planning phase
const PlanningPromptTemplate = `You are a senior software architect planning a complex task.

## Objective
%s

## Instructions

1. **Explore** the codebase to understand its structure and patterns
2. **Decompose** the objective into discrete, parallelizable tasks
3. **Write your plan** to ` + "`" + PlanFileName + "`" + ` **at the repository root** (not in any subdirectory) in JSON format

## Plan JSON Schema

Write a JSON file with this structure:
- "summary": Brief executive summary (string)
- "tasks": Array of task objects, each with:
  - "id": Unique identifier like "task-1-setup" (string)
  - "title": Short title (string)
  - "description": Detailed instructions for another Claude instance to execute independently (string)
  - "files": Files this task will modify (array of strings)
  - "depends_on": IDs of tasks that must complete first (array of strings, empty for independent tasks)
  - "priority": Lower = higher priority within dependency level (number)
  - "est_complexity": "low", "medium", or "high" (string)
- "insights": Key findings about the codebase (array of strings)
- "constraints": Risks or constraints to consider (array of strings)

## Guidelines

- **Prefer many small tasks over fewer large ones** - 10 small tasks are better than 3 medium/large tasks
- Each task should be completable within a single session without context exhaustion
- Target "low" complexity tasks; split "medium" or "high" complexity work into multiple smaller tasks
- Prefer granular tasks that can run in parallel over large sequential ones
- Assign clear file ownership to avoid merge conflicts
- Each task description should be complete enough for independent execution
- Use Write tool to create the plan file when ready

## Validation

After writing the plan file, **validate it** by running:
` + "```bash" + `
claudio validate ` + PlanFileName + `
` + "```" + `

This ensures your plan has valid JSON syntax, correct structure, and no dependency cycles.
If validation fails, fix the issues and run validation again until it passes.`

// MultiPassPlanningStrategy defines a strategic approach for multi-pass planning
type MultiPassPlanningStrategy struct {
	Strategy    string // Unique identifier for the strategy
	Description string // Human-readable description
	Prompt      string // Additional guidance to append to the base planning prompt
}

// MultiPassPlanningPrompts provides different strategic perspectives for multi-pass planning.
// Each strategy offers a distinct approach to task decomposition, enabling the multi-pass
// planning system to generate diverse plans that can be compared and combined.
var MultiPassPlanningPrompts = []MultiPassPlanningStrategy{
	{
		Strategy:    "maximize-parallelism",
		Description: "Optimize for maximum parallel execution",
		Prompt: `## Strategic Focus: Maximize Parallelism

When creating your plan, prioritize these principles:

1. **Minimize Dependencies**: Structure tasks to have as few inter-task dependencies as possible. When a dependency seems necessary, consider if the tasks can be restructured to eliminate it.

2. **Prefer Smaller Tasks**: Break work into many small, independent units rather than fewer large ones. A task that can be split into two independent pieces should be split.

3. **Isolate File Ownership**: Assign each file to exactly one task where possible. When multiple tasks must touch the same file, see if the work can be restructured to avoid this.

4. **Flatten the Dependency Graph**: Aim for a wide, shallow execution graph rather than a deep, narrow one. More tasks in the first execution group means more parallelism.

5. **Accept Some Redundancy**: It's acceptable for tasks to have slight overlap in setup or context-building if it means they can run independently.`,
	},
	{
		Strategy:    "minimize-complexity",
		Description: "Optimize for simplicity and clarity",
		Prompt: `## Strategic Focus: Minimize Complexity

When creating your plan, prioritize these principles:

1. **Single Responsibility**: Each task should do exactly one thing well. If a task description contains "and" or multiple objectives, consider splitting it.

2. **Clear Boundaries**: Tasks should have well-defined inputs and outputs. Another developer should be able to understand the task's scope without reading other task descriptions.

3. **Natural Code Boundaries**: Align task boundaries with the codebase's natural structure (packages, modules, components). Don't split work that naturally belongs together.

4. **Explicit Over Implicit**: Make dependencies explicit even if it reduces parallelism. A clear sequential flow is better than a parallel structure with hidden assumptions.

5. **Prefer Clarity Over Parallelism**: When there's a tradeoff between task clarity and parallel execution potential, choose clarity. A well-understood task is easier to execute correctly.`,
	},
	{
		Strategy:    "balanced-approach",
		Description: "Balance parallelism, complexity, and dependencies",
		Prompt: `## Strategic Focus: Balanced Approach

When creating your plan, balance these competing concerns:

1. **Respect Natural Structure**: Follow the codebase's existing architecture. Group changes that affect related functionality, even if this reduces parallelism.

2. **Pragmatic Dependencies**: Include dependencies that reflect genuine execution order requirements, but don't over-constrain the graph. Consider which dependencies are truly necessary vs. merely convenient.

3. **Right-Sized Tasks**: Tasks should be large enough to represent meaningful work units but small enough to complete in a single focused session. Target 15-45 minutes of work per task.

4. **Consider Integration**: Group changes that will need to be tested together. Tasks that affect the same feature or user flow may benefit from shared context.

5. **Maintain Flexibility**: Leave room for parallel execution where natural, but don't force artificial splits. The goal is a plan that's both efficient and maintainable.`,
	},
}

// GetMultiPassPlanningPrompt combines the base PlanningPromptTemplate with strategy-specific
// guidance for multi-pass planning. The strategy parameter should match one of the Strategy
// fields in MultiPassPlanningPrompts.
func GetMultiPassPlanningPrompt(strategy string, objective string) string {
	// Find the strategy-specific guidance
	var strategyPrompt string
	for _, s := range MultiPassPlanningPrompts {
		if s.Strategy == strategy {
			strategyPrompt = s.Prompt
			break
		}
	}

	// Build the base prompt with the objective
	basePrompt := fmt.Sprintf(PlanningPromptTemplate, objective)

	// If no matching strategy found, return just the base prompt
	if strategyPrompt == "" {
		return basePrompt
	}

	// Combine base prompt with strategy-specific guidance
	return basePrompt + "\n\n" + strategyPrompt
}

// GetMultiPassStrategyNames returns the list of available strategy names
func GetMultiPassStrategyNames() []string {
	names := make([]string, len(MultiPassPlanningPrompts))
	for i, s := range MultiPassPlanningPrompts {
		names[i] = s.Strategy
	}
	return names
}
