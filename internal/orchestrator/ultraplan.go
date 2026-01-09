package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// UltraPlanPhase represents the current phase of an ultra-plan session
type UltraPlanPhase string

const (
	PhasePlanning      UltraPlanPhase = "planning"
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

// PlannedTask represents a single decomposed task from the planning phase
type PlannedTask struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`     // Detailed task prompt for child session
	Files         []string       `json:"files,omitempty"` // Expected files to be modified
	DependsOn     []string       `json:"depends_on"`      // Task IDs this depends on
	Priority      int            `json:"priority"`        // Execution priority (lower = earlier)
	EstComplexity TaskComplexity `json:"est_complexity"`
}

// PlanSpec represents the output of the planning phase
type PlanSpec struct {
	ID              string              `json:"id"`
	Objective       string              `json:"objective"`        // Original user request
	Summary         string              `json:"summary"`          // Executive summary of the plan
	Tasks           []PlannedTask       `json:"tasks"`
	DependencyGraph map[string][]string `json:"dependency_graph"` // task_id -> depends_on[]
	ExecutionOrder  [][]string          `json:"execution_order"`  // Groups of parallelizable tasks
	Insights        []string            `json:"insights"`         // Key findings from exploration
	Constraints     []string            `json:"constraints"`      // Identified constraints/risks
	CreatedAt       time.Time           `json:"created_at"`
}

// UltraPlanConfig holds configuration for an ultra-plan session
type UltraPlanConfig struct {
	MaxParallel   int  `json:"max_parallel"`    // Maximum concurrent child sessions
	DryRun        bool `json:"dry_run"`         // Run planning only, don't execute
	NoSynthesis   bool `json:"no_synthesis"`    // Skip synthesis phase after execution
	AutoApprove   bool `json:"auto_approve"`    // Auto-approve spawned tasks without confirmation

	// Consolidation settings
	ConsolidationMode ConsolidationMode `json:"consolidation_mode,omitempty"` // "stacked" or "single"
	CreateDraftPRs    bool              `json:"create_draft_prs"`             // Create PRs as drafts
	PRLabels          []string          `json:"pr_labels,omitempty"`          // Labels to add to PRs
	BranchPrefix      string            `json:"branch_prefix,omitempty"`      // Branch prefix for consolidated branches

	// Task verification settings
	MaxTaskRetries         int  `json:"max_task_retries,omitempty"`   // Max retry attempts for tasks with no commits (default: 3)
	RequireVerifiedCommits bool `json:"require_verified_commits"`     // If true, tasks must produce commits to be marked successful (default: true)
}

// DefaultUltraPlanConfig returns the default configuration
func DefaultUltraPlanConfig() UltraPlanConfig {
	return UltraPlanConfig{
		MaxParallel:            3,
		DryRun:                 false,
		NoSynthesis:            false,
		AutoApprove:            false,
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
	TaskID      string   `json:"task_id"`               // Task ID that needs revision (empty for cross-cutting issues)
	Description string   `json:"description"`           // Description of the issue
	Files       []string `json:"files,omitempty"`       // Files affected by the issue
	Severity    string   `json:"severity,omitempty"`    // "critical", "major", "minor"
	Suggestion  string   `json:"suggestion,omitempty"`  // Suggested fix
}

// RevisionState tracks the state of the revision phase
type RevisionState struct {
	Issues           []RevisionIssue `json:"issues"`                      // Issues identified during synthesis
	RevisionRound    int             `json:"revision_round"`              // Current revision iteration (starts at 1)
	MaxRevisions     int             `json:"max_revisions"`               // Maximum allowed revision rounds
	TasksToRevise    []string        `json:"tasks_to_revise,omitempty"`   // Task IDs that need revision
	RevisedTasks     []string        `json:"revised_tasks,omitempty"`     // Task IDs that have been revised
	RevisionPrompts  map[string]string `json:"revision_prompts,omitempty"` // Task ID -> revision prompt
	StartedAt        *time.Time      `json:"started_at,omitempty"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
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
	ID              string            `json:"id"`
	Objective       string            `json:"objective"`
	Plan            *PlanSpec         `json:"plan,omitempty"`
	Phase           UltraPlanPhase    `json:"phase"`
	Config          UltraPlanConfig   `json:"config"`
	CoordinatorID   string            `json:"coordinator_id,omitempty"`   // Instance ID of the planning coordinator
	SynthesisID     string            `json:"synthesis_id,omitempty"`     // Instance ID of the synthesis reviewer
	RevisionID      string            `json:"revision_id,omitempty"`      // Instance ID of the current revision coordinator
	ConsolidationID string            `json:"consolidation_id,omitempty"` // Instance ID of the consolidation agent
	TaskToInstance  map[string]string `json:"task_to_instance"`           // PlannedTask.ID -> Instance.ID
	CompletedTasks  []string          `json:"completed_tasks"`
	FailedTasks     []string          `json:"failed_tasks"`
	CurrentGroup    int               `json:"current_group"`      // Index into ExecutionOrder
	Created         time.Time         `json:"created"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
	Error           string            `json:"error,omitempty"` // Error message if failed

	// Revision state (persisted for recovery and display)
	Revision *RevisionState `json:"revision,omitempty"`

	// Synthesis completion context (populated from sentinel file)
	SynthesisCompletion *SynthesisCompletionFile `json:"synthesis_completion,omitempty"`

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
)

// UltraPlanManager manages the execution of an ultra-plan session
type UltraPlanManager struct {
	session    *UltraPlanSession
	orch       *Orchestrator
	baseSession *Session // The underlying Claudio session

	// Event handling
	eventChan chan CoordinatorEvent
	eventCallback func(CoordinatorEvent)

	// Synchronization
	mu sync.RWMutex
	wg sync.WaitGroup

	// Cancellation
	stopChan chan struct{}
	stopped  bool
}

// NewUltraPlanManager creates a new ultra-plan manager
func NewUltraPlanManager(orch *Orchestrator, baseSession *Session, ultraSession *UltraPlanSession) *UltraPlanManager {
	return &UltraPlanManager{
		session:     ultraSession,
		orch:        orch,
		baseSession: baseSession,
		eventChan:   make(chan CoordinatorEvent, 100),
		stopChan:    make(chan struct{}),
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

// MarkTaskComplete marks a task as completed
func (m *UltraPlanManager) MarkTaskComplete(taskID string) {
	m.mu.Lock()
	m.session.CompletedTasks = append(m.session.CompletedTasks, taskID)
	delete(m.session.TaskToInstance, taskID)
	m.mu.Unlock()

	m.emitEvent(CoordinatorEvent{
		Type:   EventTaskComplete,
		TaskID: taskID,
	})
}

// MarkTaskFailed marks a task as failed
func (m *UltraPlanManager) MarkTaskFailed(taskID string, reason string) {
	m.mu.Lock()
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

// ParsePlanFromFile reads and parses a plan from a JSON file
func ParsePlanFromFile(filepath string, objective string) (*PlanSpec, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	// Parse the JSON
	var rawPlan struct {
		Summary     string        `json:"summary"`
		Tasks       []PlannedTask `json:"tasks"`
		Insights    []string      `json:"insights"`
		Constraints []string      `json:"constraints"`
	}

	if err := json.Unmarshal(data, &rawPlan); err != nil {
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

// TaskCompletionFileName is the name of the sentinel file that tasks write when complete
const TaskCompletionFileName = ".claudio-task-complete.json"

// TaskCompletionFile represents the completion report written by a task
// This file serves as both a sentinel (existence = task done) and a context carrier
type TaskCompletionFile struct {
	TaskID        string   `json:"task_id"`
	Status        string   `json:"status"` // "complete", "blocked", or "failed"
	Summary       string   `json:"summary"`
	FilesModified []string `json:"files_modified"`
	// Rich context for consolidation
	Notes        string   `json:"notes,omitempty"`        // Free-form implementation notes
	Issues       []string `json:"issues,omitempty"`       // Blocking issues or concerns found
	Suggestions  []string `json:"suggestions,omitempty"`  // Integration suggestions for other tasks
	Dependencies []string `json:"dependencies,omitempty"` // Runtime dependencies added
}

// TaskCompletionFilePath returns the full path to the task completion file for a given worktree
func TaskCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, TaskCompletionFileName)
}

// ParseTaskCompletionFile reads and parses a task completion file
func ParseTaskCompletionFile(worktreePath string) (*TaskCompletionFile, error) {
	completionPath := TaskCompletionFilePath(worktreePath)
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
	IssuesAddressed []string `json:"issues_addressed"`  // Issue descriptions that were fixed
	Summary         string   `json:"summary"`           // What was changed
	FilesModified   []string `json:"files_modified"`
	RemainingIssues []string `json:"remaining_issues"`  // Issues that couldn't be fixed
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
	Status           string                     `json:"status"` // "complete", "partial", "failed"
	Mode             string                     `json:"mode"`   // "stacked" or "single"
	GroupResults     []GroupConsolidationInfo   `json:"group_results"`
	PRsCreated       []PRInfo                   `json:"prs_created"`
	SynthesisContext *SynthesisCompletionFile   `json:"synthesis_context,omitempty"`
	TotalCommits     int                        `json:"total_commits"`
	FilesChanged     []string                   `json:"files_changed"`
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

// GroupConsolidationCompletionFileName is the sentinel file that per-group consolidators write when complete
const GroupConsolidationCompletionFileName = ".claudio-group-consolidation-complete.json"

// ConflictResolution describes how a merge conflict was resolved
type ConflictResolution struct {
	File       string `json:"file"`       // File that had the conflict
	Resolution string `json:"resolution"` // Description of how it was resolved
}

// VerificationResult holds the results of build/lint/test verification
// The consolidator determines appropriate commands based on project type
type VerificationResult struct {
	ProjectType  string             `json:"project_type,omitempty"` // Detected: "go", "node", "ios", "python", etc.
	CommandsRun  []VerificationStep `json:"commands_run"`
	OverallSuccess bool             `json:"overall_success"`
	Summary      string             `json:"summary,omitempty"` // Brief summary of verification outcome
}

// VerificationStep represents a single verification command and its result
type VerificationStep struct {
	Name    string `json:"name"`    // e.g., "build", "lint", "test"
	Command string `json:"command"` // Actual command run
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"` // Truncated output on failure
}

// GroupConsolidationCompletionFile is written by the per-group consolidator session
// when it finishes consolidating a group's task branches
type GroupConsolidationCompletionFile struct {
	GroupIndex         int                    `json:"group_index"`
	Status             string                 `json:"status"` // "complete", "failed"
	BranchName         string                 `json:"branch_name"`
	TasksConsolidated  []string               `json:"tasks_consolidated"`
	ConflictsResolved  []ConflictResolution   `json:"conflicts_resolved,omitempty"`
	Verification       VerificationResult     `json:"verification"`
	AggregatedContext  *AggregatedTaskContext `json:"aggregated_context,omitempty"`
	Notes              string                 `json:"notes,omitempty"`               // Consolidator's observations
	IssuesForNextGroup []string               `json:"issues_for_next_group,omitempty"` // Warnings/concerns to pass forward
}

// GroupConsolidationCompletionFilePath returns the full path to the group consolidation completion file
func GroupConsolidationCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, GroupConsolidationCompletionFileName)
}

// ParseGroupConsolidationCompletionFile reads and parses a group consolidation completion file
func ParseGroupConsolidationCompletionFile(worktreePath string) (*GroupConsolidationCompletionFile, error) {
	completionPath := GroupConsolidationCompletionFilePath(worktreePath)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion GroupConsolidationCompletionFile
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
3. **Write your plan** to the file ` + "`" + PlanFileName + "`" + ` in JSON format

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

- Prefer granular tasks that can run in parallel over large sequential ones
- Assign clear file ownership to avoid merge conflicts
- Each task description should be complete enough for independent execution
- Use Write tool to create the plan file when ready`

// SynthesisPromptTemplate is the prompt used for the synthesis phase
const SynthesisPromptTemplate = `You are reviewing the results of a parallel execution plan.

## Original Objective
%s

## Completed Tasks
%s

## Task Results Summary
%s

## Instructions

1. **Review** all completed work to ensure it meets the original objective
2. **Identify** any integration issues, bugs, or conflicts that need resolution
3. **Verify** that all pieces work together correctly
4. **Check** for any missing functionality or incomplete implementations

## Completion Protocol

When your review is complete, you MUST write a completion file to signal the orchestrator:

1. Use Write tool to create ` + "`" + SynthesisCompletionFileName + "`" + ` in your worktree root
2. Include this JSON structure:
` + "```json" + `
{
  "status": "complete",
  "revision_round": %d,
  "issues_found": [
    {
      "task_id": "task-id-here",
      "description": "Clear description of the issue",
      "files": ["file1.go", "file2.go"],
      "severity": "critical|major|minor",
      "suggestion": "How to fix this issue"
    }
  ],
  "tasks_affected": ["task-1", "task-2"],
  "integration_notes": "Observations about how the pieces integrate",
  "recommendations": ["Suggestions for the consolidation phase"]
}
` + "```" + `

3. Set status to "needs_revision" if critical/major issues require fixing, "complete" otherwise
4. Leave issues_found as empty array [] if no issues found
5. Include integration_notes with observations about cross-task integration
6. Add recommendations for the consolidation phase (merge order, potential conflicts, etc.)

This file signals that your review is done and provides context for subsequent phases.`

// RevisionPromptTemplate is the prompt used for the revision phase
// It instructs Claude to fix the identified issues in a specific task's worktree
const RevisionPromptTemplate = `You are addressing issues identified during review of completed work.

## Original Objective
%s

## Task Being Revised
- Task ID: %s
- Task Title: %s
- Original Description: %s
- Revision Round: %d

## Issues to Address
%s

## Worktree Information
You are working in the same worktree that was used for the original task.
All previous changes from this task are already present.

## Instructions

1. **Review** the issues identified above
2. **Fix** each issue in the codebase
3. **Test** that your fixes don't break existing functionality
4. **Commit** your changes with a clear message describing the fixes

Focus only on addressing the identified issues. Do not refactor or make other changes unless directly related to fixing the issues.

## Completion Protocol

When your revision is complete, you MUST write a completion file:

1. Use Write tool to create ` + "`" + RevisionCompletionFileName + "`" + ` in your worktree root
2. Include this JSON structure:
` + "```json" + `
{
  "task_id": "%s",
  "revision_round": %d,
  "issues_addressed": ["Description of issue 1 that was fixed", "Description of issue 2"],
  "summary": "Brief summary of the changes made",
  "files_modified": ["file1.go", "file2.go"],
  "remaining_issues": ["Any issues that could not be fixed"]
}
` + "```" + `

3. List all issues you successfully addressed in issues_addressed
4. Leave remaining_issues empty if all issues were fixed
5. This file signals that your revision is done`

// ConsolidationPromptTemplate is the prompt used for the consolidation phase
// This prompts Claude to consolidate task branches into group branches and create PRs
const ConsolidationPromptTemplate = `You are consolidating completed ultraplan task branches into pull requests.

## Objective
%s

## Branch Configuration
- Branch prefix: %s
- Main branch: %s
- Consolidation mode: %s
- Create drafts: %v

## Execution Groups and Task Branches
%s

## Task Worktree Details
The following are the exact worktree paths for each completed task. Use these paths to review the work if needed:
%s

## Synthesis Review Context
The following is context from the synthesis review phase:
%s

## Instructions

Your job is to consolidate all the task branches into group branches and create pull requests.

### For Stacked PRs Mode
1. For each execution group (starting from group 1):
   - Create a consolidated branch from the appropriate base:
     - Group 1: branch from main
     - Group N: branch from group N-1's branch
   - Cherry-pick all task commits from that group's task branches
   - Push the consolidated branch
   - Create a PR with appropriate title and description

2. PRs should be stacked: each PR's base is the previous group's branch.

### For Single PR Mode
1. Create one consolidated branch from main
2. Cherry-pick all task commits in execution order
3. Push the branch and create a single PR

### Commands to Use
- Use ` + "`" + `git cherry-pick` + "`" + ` to bring commits from task branches
- Use ` + "`" + `git push -u origin <branch>` + "`" + ` to push branches
- Use ` + "`" + `gh pr create` + "`" + ` to create pull requests
- If cherry-pick has conflicts, resolve them or report them clearly
- You can use the worktree paths above to review file changes if needed

### PR Format
Title: "ultraplan: group N - <objective summary>" (for stacked) or "ultraplan: <objective summary>" (for single)
Body should include:
- The objective
- Which tasks are included
- Integration notes from synthesis review (if any)
- For stacked PRs: note the merge order dependency

## Completion Protocol

When consolidation is complete, you MUST write a completion file:

1. Use Write tool to create ` + "`" + ConsolidationCompletionFileName + "`" + ` in your worktree root
2. Include this JSON structure:
` + "```json" + `
{
  "status": "complete",
  "mode": "%s",
  "group_results": [
    {
      "group_index": 0,
      "branch_name": "branch-name",
      "tasks_included": ["task-1", "task-2"],
      "commit_count": 5,
      "success": true
    }
  ],
  "prs_created": [
    {
      "url": "https://github.com/owner/repo/pull/123",
      "title": "PR title",
      "group_index": 0
    }
  ],
  "total_commits": 10,
  "files_changed": ["file1.go", "file2.go"]
}
` + "```" + `

3. Set status to "complete" if all PRs were created successfully
4. Set status to "partial" if some PRs failed to create
5. Set status to "failed" if consolidation could not complete
6. List all PR URLs in prs_created array

This file signals that consolidation is done and provides a record of the PRs created.`
