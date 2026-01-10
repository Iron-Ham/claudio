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
