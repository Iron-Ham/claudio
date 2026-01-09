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
}

// DefaultUltraPlanConfig returns the default configuration
func DefaultUltraPlanConfig() UltraPlanConfig {
	return UltraPlanConfig{
		MaxParallel:       3,
		DryRun:            false,
		NoSynthesis:       false,
		AutoApprove:       false,
		ConsolidationMode: ModeStackedPRs,
		CreateDraftPRs:    true,
		PRLabels:          []string{"ultraplan"},
		BranchPrefix:      "", // Uses config.Branch.Prefix if empty
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

	// Task worktree information for consolidation context
	TaskWorktrees []TaskWorktreeInfo `json:"task_worktrees,omitempty"`

	// Consolidation results (persisted for recovery and display)
	Consolidation *ConsolidationState `json:"consolidation,omitempty"`
	PRUrls        []string            `json:"pr_urls,omitempty"`
}

// NewUltraPlanSession creates a new ultra-plan session
func NewUltraPlanSession(objective string, config UltraPlanConfig) *UltraPlanSession {
	return &UltraPlanSession{
		ID:             generateID(),
		Objective:      objective,
		Phase:          PhasePlanning,
		Config:         config,
		TaskToInstance: make(map[string]string),
		CompletedTasks: make([]string, 0),
		FailedTasks:    make([]string, 0),
		Created:        time.Now(),
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

## Output Format

After your review, you MUST output your findings in a structured format.

If there are issues that need to be addressed, output them in a <revision_issues> block:

<revision_issues>
[
  {
    "task_id": "task-id-here",
    "description": "Clear description of the issue",
    "files": ["file1.go", "file2.go"],
    "severity": "critical|major|minor",
    "suggestion": "How to fix this issue"
  }
]
</revision_issues>

If there are NO issues and the work is complete, output:

<revision_issues>
[]
</revision_issues>

IMPORTANT: You MUST always include a <revision_issues> block, even if empty.
After the issues block, provide a summary of what was accomplished.`

// RevisionPromptTemplate is the prompt used for the revision phase
// It instructs Claude to fix the identified issues in a specific task's worktree
const RevisionPromptTemplate = `You are addressing issues identified during review of completed work.

## Original Objective
%s

## Task Being Revised
- Task ID: %s
- Task Title: %s
- Original Description: %s

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

When complete, summarize what you fixed.`

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
- For stacked PRs: note the merge order dependency

### On Completion
Report the URLs of all created PRs.`
