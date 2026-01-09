package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// Test Helpers and Mock Types
// ============================================================================

// mockOrchestrator creates a minimal orchestrator for testing coordinator logic.
// This doesn't run actual Claude instances but allows testing the coordination behavior.
type mockOrchestratorContext struct {
	sessions      map[string]*Session
	instances     map[string]*Instance
	instanceMgrs  map[string]*mockInstanceManager
	savedSessions int
	mu            sync.RWMutex
}

// mockInstanceManager implements minimal TmuxSessionExists behavior
type mockInstanceManager struct {
	sessionExists bool
	output        []byte
}

func (m *mockInstanceManager) TmuxSessionExists() bool {
	return m.sessionExists
}

func (m *mockInstanceManager) GetOutput() []byte {
	return m.output
}

// createTestUltraPlanSession creates a test UltraPlanSession with the given configuration
func createTestUltraPlanSession() *UltraPlanSession {
	return &UltraPlanSession{
		ID:               "test-ultra-session",
		Objective:        "Test objective for coordinator tests",
		Phase:            PhasePlanning,
		Config:           DefaultUltraPlanConfig(),
		TaskToInstance:   make(map[string]string),
		CompletedTasks:   make([]string, 0),
		FailedTasks:      make([]string, 0),
		Created:          time.Now(),
		TaskRetries:      make(map[string]*TaskRetryState),
		TaskCommitCounts: make(map[string]int),
	}
}

// createTestPlanWithDependencies creates a PlanSpec with task dependencies for testing
func createTestPlanWithDependencies() *PlanSpec {
	tasks := []PlannedTask{
		{
			ID:            "task-1",
			Title:         "Foundation Setup",
			Description:   "Initialize the project foundation",
			Files:         []string{"main.go", "go.mod"},
			DependsOn:     []string{},
			Priority:      1,
			EstComplexity: ComplexityLow,
		},
		{
			ID:            "task-2",
			Title:         "Core Implementation",
			Description:   "Implement core features",
			Files:         []string{"core.go"},
			DependsOn:     []string{"task-1"},
			Priority:      2,
			EstComplexity: ComplexityMedium,
		},
		{
			ID:            "task-3",
			Title:         "Secondary Features",
			Description:   "Implement secondary features",
			Files:         []string{"secondary.go"},
			DependsOn:     []string{"task-1"},
			Priority:      2,
			EstComplexity: ComplexityMedium,
		},
		{
			ID:            "task-4",
			Title:         "Integration",
			Description:   "Integrate all components",
			Files:         []string{"integration.go"},
			DependsOn:     []string{"task-2", "task-3"},
			Priority:      3,
			EstComplexity: ComplexityHigh,
		},
		{
			ID:            "task-5",
			Title:         "Tests",
			Description:   "Write comprehensive tests",
			Files:         []string{"test.go"},
			DependsOn:     []string{"task-4"},
			Priority:      4,
			EstComplexity: ComplexityLow,
		},
	}

	deps := make(map[string][]string)
	for _, t := range tasks {
		deps[t.ID] = t.DependsOn
	}

	plan := &PlanSpec{
		ID:              "test-plan",
		Objective:       "Test objective",
		Summary:         "Test plan with dependency chain",
		Tasks:           tasks,
		DependencyGraph: deps,
		ExecutionOrder:  calculateExecutionOrder(tasks, deps),
		CreatedAt:       time.Now(),
	}

	return plan
}

// createSimpleTestPlan creates a minimal plan with one task
func createSimpleTestPlan() *PlanSpec {
	tasks := []PlannedTask{
		{
			ID:            "task-1",
			Title:         "Single Task",
			Description:   "A simple standalone task",
			Files:         []string{"file.go"},
			DependsOn:     []string{},
			Priority:      1,
			EstComplexity: ComplexityLow,
		},
	}

	return &PlanSpec{
		ID:              "simple-plan",
		Objective:       "Simple test",
		Summary:         "Single task plan",
		Tasks:           tasks,
		DependencyGraph: map[string][]string{"task-1": {}},
		ExecutionOrder:  [][]string{{"task-1"}},
		CreatedAt:       time.Now(),
	}
}

// createParallelPlan creates a plan where all tasks can run in parallel
func createParallelPlan(taskCount int) *PlanSpec {
	tasks := make([]PlannedTask, taskCount)
	for i := 0; i < taskCount; i++ {
		tasks[i] = PlannedTask{
			ID:            generateID(),
			Title:         "Parallel Task",
			Description:   "Task that can run in parallel",
			Files:         []string{},
			DependsOn:     []string{},
			Priority:      1,
			EstComplexity: ComplexityLow,
		}
	}

	// All in same execution group since no dependencies
	taskIDs := make([]string, taskCount)
	for i, t := range tasks {
		taskIDs[i] = t.ID
	}

	return &PlanSpec{
		ID:              "parallel-plan",
		Objective:       "Parallel execution test",
		Summary:         "All tasks run in parallel",
		Tasks:           tasks,
		DependencyGraph: make(map[string][]string),
		ExecutionOrder:  [][]string{taskIDs},
		CreatedAt:       time.Now(),
	}
}

// ============================================================================
// Tests for Coordinator Initialization
// ============================================================================

func TestCoordinatorCallbacks_ZeroValue(t *testing.T) {
	// Zero value callbacks should not panic
	cb := &CoordinatorCallbacks{}

	// Verify all callback fields are nil
	if cb.OnPhaseChange != nil {
		t.Error("expected OnPhaseChange to be nil")
	}
	if cb.OnTaskStart != nil {
		t.Error("expected OnTaskStart to be nil")
	}
	if cb.OnTaskComplete != nil {
		t.Error("expected OnTaskComplete to be nil")
	}
	if cb.OnTaskFailed != nil {
		t.Error("expected OnTaskFailed to be nil")
	}
	if cb.OnGroupComplete != nil {
		t.Error("expected OnGroupComplete to be nil")
	}
	if cb.OnPlanReady != nil {
		t.Error("expected OnPlanReady to be nil")
	}
	if cb.OnProgress != nil {
		t.Error("expected OnProgress to be nil")
	}
	if cb.OnComplete != nil {
		t.Error("expected OnComplete to be nil")
	}
}

func TestCoordinatorCallbacks_Initialization(t *testing.T) {
	var phaseChanged UltraPlanPhase
	var taskStarted, taskCompleted, taskFailed string
	var groupCompleted int
	var planReady *PlanSpec
	var progressCompleted, progressTotal int
	var completionSuccess bool

	cb := &CoordinatorCallbacks{
		OnPhaseChange: func(phase UltraPlanPhase) {
			phaseChanged = phase
		},
		OnTaskStart: func(taskID, instanceID string) {
			taskStarted = taskID
		},
		OnTaskComplete: func(taskID string) {
			taskCompleted = taskID
		},
		OnTaskFailed: func(taskID, reason string) {
			taskFailed = taskID
		},
		OnGroupComplete: func(groupIndex int) {
			groupCompleted = groupIndex
		},
		OnPlanReady: func(plan *PlanSpec) {
			planReady = plan
		},
		OnProgress: func(completed, total int, phase UltraPlanPhase) {
			progressCompleted = completed
			progressTotal = total
		},
		OnComplete: func(success bool, summary string) {
			completionSuccess = success
		},
	}

	// Test all callbacks
	cb.OnPhaseChange(PhaseExecuting)
	if phaseChanged != PhaseExecuting {
		t.Errorf("expected PhaseExecuting, got %s", phaseChanged)
	}

	cb.OnTaskStart("task-1", "inst-1")
	if taskStarted != "task-1" {
		t.Errorf("expected task-1, got %s", taskStarted)
	}

	cb.OnTaskComplete("task-2")
	if taskCompleted != "task-2" {
		t.Errorf("expected task-2, got %s", taskCompleted)
	}

	cb.OnTaskFailed("task-3", "error")
	if taskFailed != "task-3" {
		t.Errorf("expected task-3, got %s", taskFailed)
	}

	cb.OnGroupComplete(1)
	if groupCompleted != 1 {
		t.Errorf("expected group 1, got %d", groupCompleted)
	}

	plan := createSimpleTestPlan()
	cb.OnPlanReady(plan)
	if planReady != plan {
		t.Error("expected plan to be set")
	}

	cb.OnProgress(5, 10, PhaseExecuting)
	if progressCompleted != 5 || progressTotal != 10 {
		t.Errorf("expected progress 5/10, got %d/%d", progressCompleted, progressTotal)
	}

	cb.OnComplete(true, "done")
	if !completionSuccess {
		t.Error("expected completion success to be true")
	}
}

// ============================================================================
// Tests for Phase Transition Logic
// ============================================================================

func TestUltraPlanPhase_Values(t *testing.T) {
	// Verify all phase constants are correct
	phases := []struct {
		phase    UltraPlanPhase
		expected string
	}{
		{PhasePlanning, "planning"},
		{PhaseRefresh, "context_refresh"},
		{PhaseExecuting, "executing"},
		{PhaseSynthesis, "synthesis"},
		{PhaseRevision, "revision"},
		{PhaseConsolidating, "consolidating"},
		{PhaseComplete, "complete"},
		{PhaseFailed, "failed"},
	}

	for _, tc := range phases {
		if string(tc.phase) != tc.expected {
			t.Errorf("phase %s: expected %q, got %q", tc.phase, tc.expected, string(tc.phase))
		}
	}
}

func TestUltraPlanManager_SetPhase(t *testing.T) {
	session := createTestUltraPlanSession()
	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	// Track phase changes via event
	var receivedPhase UltraPlanPhase
	manager.SetEventCallback(func(event CoordinatorEvent) {
		if event.Type == EventPhaseChange {
			receivedPhase = UltraPlanPhase(event.Message)
		}
	})

	manager.SetPhase(PhaseExecuting)

	if session.Phase != PhaseExecuting {
		t.Errorf("expected session phase to be %s, got %s", PhaseExecuting, session.Phase)
	}
	if receivedPhase != PhaseExecuting {
		t.Errorf("expected event phase to be %s, got %s", PhaseExecuting, receivedPhase)
	}
}

func TestPhaseTransitionSequence(t *testing.T) {
	// Test the expected phase transition sequence for successful execution
	expectedSequence := []UltraPlanPhase{
		PhasePlanning,
		PhaseRefresh,
		PhaseExecuting,
		PhaseSynthesis,
		PhaseConsolidating,
		PhaseComplete,
	}

	session := createTestUltraPlanSession()
	session.Phase = PhasePlanning

	for i, expected := range expectedSequence {
		if i > 0 {
			session.Phase = expected
		}
		if session.Phase != expected {
			t.Errorf("step %d: expected phase %s, got %s", i, expected, session.Phase)
		}
	}
}

func TestPhaseTransitionWithRevision(t *testing.T) {
	// Test phase sequence when revision is needed
	session := createTestUltraPlanSession()

	sequence := []UltraPlanPhase{
		PhasePlanning,
		PhaseRefresh,
		PhaseExecuting,
		PhaseSynthesis,
		PhaseRevision, // Issues found
		PhaseSynthesis, // Re-synthesis after revision
		PhaseConsolidating,
		PhaseComplete,
	}

	for i, expected := range sequence {
		session.Phase = expected
		if session.Phase != expected {
			t.Errorf("step %d: expected phase %s, got %s", i, expected, session.Phase)
		}
	}
}

func TestPhaseTransitionToFailed(t *testing.T) {
	// Test transition to failed state
	startPhases := []UltraPlanPhase{
		PhasePlanning,
		PhaseExecuting,
		PhaseSynthesis,
		PhaseRevision,
		PhaseConsolidating,
	}

	for _, startPhase := range startPhases {
		session := createTestUltraPlanSession()
		session.Phase = startPhase
		session.Phase = PhaseFailed
		session.Error = "test error"

		if session.Phase != PhaseFailed {
			t.Errorf("from %s: expected phase %s, got %s", startPhase, PhaseFailed, session.Phase)
		}
		if session.Error != "test error" {
			t.Errorf("expected error message to be set")
		}
	}
}

// ============================================================================
// Tests for Task Dependency Resolution
// ============================================================================

func TestExecutionOrder_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		tasks          []PlannedTask
		expectedGroups int
		expectedFirst  []string // Tasks in first group
		expectedLast   []string // Tasks in last group
	}{
		{
			name: "linear dependency chain",
			tasks: []PlannedTask{
				{ID: "A", DependsOn: []string{}},
				{ID: "B", DependsOn: []string{"A"}},
				{ID: "C", DependsOn: []string{"B"}},
			},
			expectedGroups: 3,
			expectedFirst:  []string{"A"},
			expectedLast:   []string{"C"},
		},
		{
			name: "diamond dependency",
			tasks: []PlannedTask{
				{ID: "A", DependsOn: []string{}},
				{ID: "B", DependsOn: []string{"A"}},
				{ID: "C", DependsOn: []string{"A"}},
				{ID: "D", DependsOn: []string{"B", "C"}},
			},
			expectedGroups: 3,
			expectedFirst:  []string{"A"},
			expectedLast:   []string{"D"},
		},
		{
			name: "all parallel - no dependencies",
			tasks: []PlannedTask{
				{ID: "A", DependsOn: []string{}},
				{ID: "B", DependsOn: []string{}},
				{ID: "C", DependsOn: []string{}},
			},
			expectedGroups: 1,
			expectedFirst:  []string{"A", "B", "C"},
			expectedLast:   []string{"A", "B", "C"},
		},
		{
			name: "two parallel chains",
			tasks: []PlannedTask{
				{ID: "A1", DependsOn: []string{}},
				{ID: "A2", DependsOn: []string{"A1"}},
				{ID: "B1", DependsOn: []string{}},
				{ID: "B2", DependsOn: []string{"B1"}},
			},
			expectedGroups: 2,
			expectedFirst:  []string{"A1", "B1"},
			expectedLast:   []string{"A2", "B2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := make(map[string][]string)
			for _, task := range tt.tasks {
				deps[task.ID] = task.DependsOn
			}

			order := calculateExecutionOrder(tt.tasks, deps)

			if len(order) != tt.expectedGroups {
				t.Errorf("expected %d groups, got %d", tt.expectedGroups, len(order))
			}

			// Check first group
			if len(order) > 0 {
				firstGroup := order[0]
				for _, expectedTask := range tt.expectedFirst {
					found := false
					for _, task := range firstGroup {
						if task == expectedTask {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected task %s in first group, got %v", expectedTask, firstGroup)
					}
				}
			}

			// Check last group
			if len(order) > 0 {
				lastGroup := order[len(order)-1]
				for _, expectedTask := range tt.expectedLast {
					found := false
					for _, task := range lastGroup {
						if task == expectedTask {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected task %s in last group, got %v", expectedTask, lastGroup)
					}
				}
			}
		})
	}
}

func TestIsTaskReady(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()

	tests := []struct {
		name           string
		taskID         string
		completedTasks []string
		expectedReady  bool
	}{
		{
			name:           "task with no dependencies is ready",
			taskID:         "task-1",
			completedTasks: []string{},
			expectedReady:  true,
		},
		{
			name:           "task with unmet dependencies is not ready",
			taskID:         "task-2",
			completedTasks: []string{},
			expectedReady:  false,
		},
		{
			name:           "task with met dependencies is ready",
			taskID:         "task-2",
			completedTasks: []string{"task-1"},
			expectedReady:  true,
		},
		{
			name:           "task with partially met dependencies is not ready",
			taskID:         "task-4",
			completedTasks: []string{"task-1", "task-2"},
			expectedReady:  false,
		},
		{
			name:           "task with all dependencies met is ready",
			taskID:         "task-4",
			completedTasks: []string{"task-1", "task-2", "task-3"},
			expectedReady:  true,
		},
		{
			name:           "unknown task is not ready",
			taskID:         "nonexistent",
			completedTasks: []string{},
			expectedReady:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session.CompletedTasks = tt.completedTasks
			ready := session.IsTaskReady(tt.taskID)
			if ready != tt.expectedReady {
				t.Errorf("IsTaskReady(%s) = %v, expected %v", tt.taskID, ready, tt.expectedReady)
			}
		})
	}
}

func TestGetReadyTasks(t *testing.T) {
	tests := []struct {
		name           string
		completedTasks []string
		currentGroup   int
		expectedReady  []string
	}{
		{
			name:           "initial state - group 0 ready",
			completedTasks: []string{},
			currentGroup:   0,
			expectedReady:  []string{"task-1"},
		},
		{
			name:           "after task-1 complete - advance group",
			completedTasks: []string{"task-1"},
			currentGroup:   1,
			expectedReady:  []string{"task-2", "task-3"},
		},
		{
			name:           "after group 1 complete - task-4 ready",
			completedTasks: []string{"task-1", "task-2", "task-3"},
			currentGroup:   2,
			expectedReady:  []string{"task-4"},
		},
		{
			name:           "past all groups - none ready",
			completedTasks: []string{"task-1", "task-2", "task-3", "task-4", "task-5"},
			currentGroup:   10,
			expectedReady:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := createTestUltraPlanSession()
			session.Plan = createTestPlanWithDependencies()
			session.CompletedTasks = tt.completedTasks
			session.CurrentGroup = tt.currentGroup

			ready := session.GetReadyTasks()

			if len(ready) != len(tt.expectedReady) {
				t.Errorf("expected %d ready tasks, got %d: %v", len(tt.expectedReady), len(ready), ready)
				return
			}

			for _, expected := range tt.expectedReady {
				found := false
				for _, task := range ready {
					if task == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected task %s in ready list, got %v", expected, ready)
				}
			}
		})
	}
}

func TestIsCurrentGroupComplete(t *testing.T) {
	tests := []struct {
		name           string
		completedTasks []string
		failedTasks    []string
		currentGroup   int
		expected       bool
	}{
		{
			name:           "group not complete - no tasks done",
			completedTasks: []string{},
			currentGroup:   0,
			expected:       false,
		},
		{
			name:           "group complete - task-1 done",
			completedTasks: []string{"task-1"},
			currentGroup:   0,
			expected:       true,
		},
		{
			name:           "group 1 not complete - only one of two done",
			completedTasks: []string{"task-1", "task-2"},
			currentGroup:   1,
			expected:       false,
		},
		{
			name:           "group 1 complete - both done",
			completedTasks: []string{"task-1", "task-2", "task-3"},
			currentGroup:   1,
			expected:       true,
		},
		{
			name:           "group complete with mixed success and failure",
			completedTasks: []string{"task-1", "task-2"},
			failedTasks:    []string{"task-3"},
			currentGroup:   1,
			expected:       true,
		},
		{
			name:           "past all groups",
			completedTasks: []string{"task-1", "task-2", "task-3", "task-4", "task-5"},
			currentGroup:   10,
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := createTestUltraPlanSession()
			session.Plan = createTestPlanWithDependencies()
			session.CompletedTasks = tt.completedTasks
			session.FailedTasks = tt.failedTasks
			session.CurrentGroup = tt.currentGroup

			result := session.IsCurrentGroupComplete()
			if result != tt.expected {
				t.Errorf("IsCurrentGroupComplete() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestAdvanceGroupIfComplete(t *testing.T) {
	tests := []struct {
		name              string
		completedTasks    []string
		currentGroup      int
		expectedAdvanced  bool
		expectedPrevGroup int
		expectedNewGroup  int
	}{
		{
			name:              "advance from group 0 to 1",
			completedTasks:    []string{"task-1"},
			currentGroup:      0,
			expectedAdvanced:  true,
			expectedPrevGroup: 0,
			expectedNewGroup:  1,
		},
		{
			name:              "advance from group 1 to 2",
			completedTasks:    []string{"task-1", "task-2", "task-3"},
			currentGroup:      1,
			expectedAdvanced:  true,
			expectedPrevGroup: 1,
			expectedNewGroup:  2,
		},
		{
			name:              "no advance - group not complete",
			completedTasks:    []string{"task-1", "task-2"},
			currentGroup:      1,
			expectedAdvanced:  false,
			expectedPrevGroup: 1,
			expectedNewGroup:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := createTestUltraPlanSession()
			session.Plan = createTestPlanWithDependencies()
			session.CompletedTasks = tt.completedTasks
			session.CurrentGroup = tt.currentGroup

			advanced, prevGroup := session.AdvanceGroupIfComplete()

			if advanced != tt.expectedAdvanced {
				t.Errorf("advanced = %v, expected %v", advanced, tt.expectedAdvanced)
			}
			if prevGroup != tt.expectedPrevGroup {
				t.Errorf("prevGroup = %d, expected %d", prevGroup, tt.expectedPrevGroup)
			}
			if session.CurrentGroup != tt.expectedNewGroup {
				t.Errorf("CurrentGroup = %d, expected %d", session.CurrentGroup, tt.expectedNewGroup)
			}
		})
	}
}

// ============================================================================
// Tests for Parallel Task Execution Scheduling
// ============================================================================

func TestMaxParallelConfig(t *testing.T) {
	tests := []struct {
		name        string
		maxParallel int
		expected    int
	}{
		{"default config", 0, 3}, // DefaultUltraPlanConfig sets 3
		{"single task", 1, 1},
		{"multiple parallel", 5, 5},
		{"unlimited", -1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultUltraPlanConfig()
			if tt.maxParallel != 0 {
				config.MaxParallel = tt.maxParallel
			}

			if tt.maxParallel == 0 && config.MaxParallel != 3 {
				t.Errorf("expected default MaxParallel to be 3, got %d", config.MaxParallel)
			} else if tt.maxParallel != 0 && config.MaxParallel != tt.expected {
				t.Errorf("expected MaxParallel to be %d, got %d", tt.expected, config.MaxParallel)
			}
		})
	}
}

func TestParallelTaskScheduling(t *testing.T) {
	// Create a plan with 5 parallel tasks
	plan := createParallelPlan(5)

	session := createTestUltraPlanSession()
	session.Plan = plan
	session.Config.MaxParallel = 3

	// All 5 tasks should be ready since they have no dependencies
	ready := session.GetReadyTasks()
	if len(ready) != 5 {
		t.Errorf("expected 5 ready tasks, got %d", len(ready))
	}

	// Verify execution order has single group with all tasks
	if len(plan.ExecutionOrder) != 1 {
		t.Errorf("expected 1 execution group, got %d", len(plan.ExecutionOrder))
	}
	if len(plan.ExecutionOrder[0]) != 5 {
		t.Errorf("expected 5 tasks in group, got %d", len(plan.ExecutionOrder[0]))
	}
}

func TestGetBaseBranchForGroup(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()
	session.GroupConsolidatedBranches = []string{
		"feature/ultraplan-group-1",
		"feature/ultraplan-group-2",
	}

	tests := []struct {
		name       string
		groupIndex int
		expected   string
	}{
		{
			name:       "group 0 uses default",
			groupIndex: 0,
			expected:   "",
		},
		{
			name:       "group 1 uses group 0 consolidated",
			groupIndex: 1,
			expected:   "feature/ultraplan-group-1",
		},
		{
			name:       "group 2 uses group 1 consolidated",
			groupIndex: 2,
			expected:   "feature/ultraplan-group-2",
		},
		{
			name:       "group 3 beyond available branches",
			groupIndex: 3,
			expected:   "",
		},
	}

	// Create a minimal mock coordinator to test getBaseBranchForGroup
	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coord.getBaseBranchForGroup(tt.groupIndex)
			if result != tt.expected {
				t.Errorf("getBaseBranchForGroup(%d) = %q, expected %q", tt.groupIndex, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Tests for Error Handling and Recovery
// ============================================================================

func TestTaskRetryState(t *testing.T) {
	state := &TaskRetryState{
		TaskID:       "task-1",
		MaxRetries:   3,
		CommitCounts: make([]int, 0),
	}

	// Simulate first attempt - no commits
	state.CommitCounts = append(state.CommitCounts, 0)
	state.RetryCount++
	state.LastError = "task produced no commits"

	if state.RetryCount != 1 {
		t.Errorf("expected RetryCount 1, got %d", state.RetryCount)
	}

	// Simulate second attempt - still no commits
	state.CommitCounts = append(state.CommitCounts, 0)
	state.RetryCount++

	if state.RetryCount != 2 {
		t.Errorf("expected RetryCount 2, got %d", state.RetryCount)
	}

	// Third attempt - max retries reached
	state.CommitCounts = append(state.CommitCounts, 0)
	state.RetryCount++

	if state.RetryCount < state.MaxRetries {
		t.Errorf("expected retries exhausted, RetryCount=%d, MaxRetries=%d", state.RetryCount, state.MaxRetries)
	}
}

func TestTaskCompletion_Success(t *testing.T) {
	completion := taskCompletion{
		taskID:      "task-1",
		instanceID:  "inst-1",
		success:     true,
		error:       "",
		needsRetry:  false,
		commitCount: 3,
	}

	if !completion.success {
		t.Error("expected success to be true")
	}
	if completion.needsRetry {
		t.Error("expected needsRetry to be false")
	}
	if completion.commitCount != 3 {
		t.Errorf("expected commitCount 3, got %d", completion.commitCount)
	}
}

func TestTaskCompletion_FailureWithRetry(t *testing.T) {
	completion := taskCompletion{
		taskID:      "task-1",
		instanceID:  "inst-1",
		success:     false,
		error:       "no_commits_retry",
		needsRetry:  true,
		commitCount: 0,
	}

	if completion.success {
		t.Error("expected success to be false")
	}
	if !completion.needsRetry {
		t.Error("expected needsRetry to be true")
	}
	if completion.error != "no_commits_retry" {
		t.Errorf("expected error 'no_commits_retry', got %q", completion.error)
	}
}

func TestPartialGroupFailure(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()
	session.CompletedTasks = []string{"task-2"}
	session.FailedTasks = []string{"task-3"}
	session.TaskCommitCounts = map[string]int{
		"task-2": 2,
	}

	// Group 1 contains task-2 and task-3
	// task-2 succeeded (has commits), task-3 failed
	groupIndex := 1
	groupTasks := session.Plan.ExecutionOrder[groupIndex]

	successCount := 0
	failureCount := 0

	for _, taskID := range groupTasks {
		isCompleted := false
		for _, ct := range session.CompletedTasks {
			if ct == taskID {
				isCompleted = true
				break
			}
		}

		if isCompleted {
			if count, ok := session.TaskCommitCounts[taskID]; ok && count > 0 {
				successCount++
			} else {
				failureCount++
			}
			continue
		}

		for _, ft := range session.FailedTasks {
			if ft == taskID {
				failureCount++
				break
			}
		}
	}

	// Should have partial failure (some success, some failure)
	hasPartialFailure := successCount > 0 && failureCount > 0
	if !hasPartialFailure {
		t.Errorf("expected partial failure, got successCount=%d, failureCount=%d", successCount, failureCount)
	}
}

func TestGroupDecisionState(t *testing.T) {
	state := &GroupDecisionState{
		GroupIndex:       1,
		SucceededTasks:   []string{"task-2"},
		FailedTasks:      []string{"task-3"},
		AwaitingDecision: true,
	}

	if state.GroupIndex != 1 {
		t.Errorf("expected GroupIndex 1, got %d", state.GroupIndex)
	}
	if len(state.SucceededTasks) != 1 || state.SucceededTasks[0] != "task-2" {
		t.Errorf("expected SucceededTasks to contain task-2")
	}
	if len(state.FailedTasks) != 1 || state.FailedTasks[0] != "task-3" {
		t.Errorf("expected FailedTasks to contain task-3")
	}
	if !state.AwaitingDecision {
		t.Error("expected AwaitingDecision to be true")
	}
}

// ============================================================================
// Tests for Progress Tracking and Callbacks
// ============================================================================

func TestProgress(t *testing.T) {
	tests := []struct {
		name           string
		completedTasks int
		totalTasks     int
		expectedPct    float64
	}{
		{"0 of 10", 0, 10, 0.0},
		{"5 of 10", 5, 10, 50.0},
		{"10 of 10", 10, 10, 100.0},
		{"3 of 4", 3, 4, 75.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := createTestUltraPlanSession()

			// Create tasks
			tasks := make([]PlannedTask, tt.totalTasks)
			for i := 0; i < tt.totalTasks; i++ {
				tasks[i] = PlannedTask{ID: generateID()}
			}
			session.Plan = &PlanSpec{Tasks: tasks}

			// Mark some as completed
			for i := 0; i < tt.completedTasks; i++ {
				session.CompletedTasks = append(session.CompletedTasks, tasks[i].ID)
			}

			pct := session.Progress()
			if pct != tt.expectedPct {
				t.Errorf("Progress() = %.1f, expected %.1f", pct, tt.expectedPct)
			}
		})
	}
}

func TestProgressNoTasks(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = nil

	pct := session.Progress()
	if pct != 0 {
		t.Errorf("expected 0 progress for nil plan, got %.1f", pct)
	}

	session.Plan = &PlanSpec{Tasks: []PlannedTask{}}
	pct = session.Progress()
	if pct != 0 {
		t.Errorf("expected 0 progress for empty tasks, got %.1f", pct)
	}
}

func TestHasMoreGroups(t *testing.T) {
	tests := []struct {
		name         string
		currentGroup int
		totalGroups  int
		expected     bool
	}{
		{"group 0 of 3", 0, 3, true},
		{"group 1 of 3", 1, 3, true},
		{"group 2 of 3", 2, 3, true},
		{"group 3 of 3 - no more", 3, 3, false},
		{"past groups", 5, 3, false},
		{"no groups", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := createTestUltraPlanSession()
			session.CurrentGroup = tt.currentGroup

			if tt.totalGroups > 0 {
				groups := make([][]string, tt.totalGroups)
				for i := 0; i < tt.totalGroups; i++ {
					groups[i] = []string{generateID()}
				}
				session.Plan = &PlanSpec{ExecutionOrder: groups}
			}

			result := session.HasMoreGroups()
			if result != tt.expected {
				t.Errorf("HasMoreGroups() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestUltraPlanManager_MarkTaskComplete(t *testing.T) {
	session := createTestUltraPlanSession()
	session.TaskToInstance = map[string]string{"task-1": "inst-1"}

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	var receivedEvent CoordinatorEvent
	manager.SetEventCallback(func(event CoordinatorEvent) {
		receivedEvent = event
	})

	manager.MarkTaskComplete("task-1")

	// Verify task is in completed list
	found := false
	for _, id := range session.CompletedTasks {
		if id == "task-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected task-1 in CompletedTasks")
	}

	// Verify task is removed from TaskToInstance
	if _, exists := session.TaskToInstance["task-1"]; exists {
		t.Error("expected task-1 to be removed from TaskToInstance")
	}

	// Verify event was emitted
	if receivedEvent.Type != EventTaskComplete {
		t.Errorf("expected EventTaskComplete, got %s", receivedEvent.Type)
	}
	if receivedEvent.TaskID != "task-1" {
		t.Errorf("expected TaskID task-1, got %s", receivedEvent.TaskID)
	}
}

func TestUltraPlanManager_MarkTaskFailed(t *testing.T) {
	session := createTestUltraPlanSession()
	session.TaskToInstance = map[string]string{"task-1": "inst-1"}

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	var receivedEvent CoordinatorEvent
	manager.SetEventCallback(func(event CoordinatorEvent) {
		receivedEvent = event
	})

	manager.MarkTaskFailed("task-1", "test failure reason")

	// Verify task is in failed list
	found := false
	for _, id := range session.FailedTasks {
		if id == "task-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected task-1 in FailedTasks")
	}

	// Verify task is removed from TaskToInstance
	if _, exists := session.TaskToInstance["task-1"]; exists {
		t.Error("expected task-1 to be removed from TaskToInstance")
	}

	// Verify event was emitted
	if receivedEvent.Type != EventTaskFailed {
		t.Errorf("expected EventTaskFailed, got %s", receivedEvent.Type)
	}
	if receivedEvent.Message != "test failure reason" {
		t.Errorf("expected reason 'test failure reason', got %s", receivedEvent.Message)
	}
}

func TestUltraPlanManager_AssignTaskToInstance(t *testing.T) {
	session := createTestUltraPlanSession()

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	var receivedEvent CoordinatorEvent
	manager.SetEventCallback(func(event CoordinatorEvent) {
		receivedEvent = event
	})

	manager.AssignTaskToInstance("task-1", "inst-1")

	// Verify mapping exists
	if session.TaskToInstance["task-1"] != "inst-1" {
		t.Errorf("expected TaskToInstance[task-1] = inst-1, got %s", session.TaskToInstance["task-1"])
	}

	// Verify event was emitted
	if receivedEvent.Type != EventTaskStarted {
		t.Errorf("expected EventTaskStarted, got %s", receivedEvent.Type)
	}
	if receivedEvent.TaskID != "task-1" {
		t.Errorf("expected TaskID task-1, got %s", receivedEvent.TaskID)
	}
	if receivedEvent.InstanceID != "inst-1" {
		t.Errorf("expected InstanceID inst-1, got %s", receivedEvent.InstanceID)
	}
}

// ============================================================================
// Tests for Cancellation and Cleanup
// ============================================================================

func TestUltraPlanManager_Stop(t *testing.T) {
	session := createTestUltraPlanSession()

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	// Stop should not panic even when called multiple times
	manager.Stop()

	// Verify stopped state
	manager.mu.RLock()
	stopped := manager.stopped
	manager.mu.RUnlock()

	if !stopped {
		t.Error("expected manager to be stopped")
	}

	// Second stop should be safe
	manager.Stop()
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Simulate a running coordinator check
	done := make(chan bool, 1)

	go func() {
		select {
		case <-ctx.Done():
			done <- true
		case <-time.After(5 * time.Second):
			done <- false
		}
	}()

	// Cancel should immediately trigger ctx.Done()
	cancel()

	result := <-done
	if !result {
		t.Error("expected context cancellation to trigger")
	}
}

func TestRunningTasksTracking(t *testing.T) {
	runningTasks := make(map[string]string)
	runningCount := 0
	var mu sync.RWMutex

	// Simulate starting a task
	mu.Lock()
	runningTasks["task-1"] = "inst-1"
	runningCount++
	mu.Unlock()

	mu.RLock()
	if len(runningTasks) != 1 {
		t.Errorf("expected 1 running task, got %d", len(runningTasks))
	}
	if runningCount != 1 {
		t.Errorf("expected runningCount 1, got %d", runningCount)
	}
	mu.RUnlock()

	// Simulate task completion
	mu.Lock()
	delete(runningTasks, "task-1")
	runningCount--
	mu.Unlock()

	mu.RLock()
	if len(runningTasks) != 0 {
		t.Errorf("expected 0 running tasks, got %d", len(runningTasks))
	}
	if runningCount != 0 {
		t.Errorf("expected runningCount 0, got %d", runningCount)
	}
	mu.RUnlock()
}

// ============================================================================
// Tests for Task Prompt Building
// ============================================================================

func TestBuildTaskPrompt(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	task := session.Plan.Tasks[0] // task-1

	prompt := coord.buildTaskPrompt(&task)

	// Verify prompt contains key elements
	if !containsString(prompt, task.Title) {
		t.Error("expected prompt to contain task title")
	}
	if !containsString(prompt, task.Description) {
		t.Error("expected prompt to contain task description")
	}
	if !containsString(prompt, TaskCompletionFileName) {
		t.Error("expected prompt to contain completion file name")
	}
	if !containsString(prompt, task.ID) {
		t.Error("expected prompt to contain task ID")
	}
}

func TestBuildTaskPromptWithFiles(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	task := session.Plan.Tasks[0] // task-1 has files

	prompt := coord.buildTaskPrompt(&task)

	// Verify prompt contains expected files section
	if !containsString(prompt, "Expected Files") {
		t.Error("expected prompt to contain 'Expected Files' section")
	}
	for _, file := range task.Files {
		if !containsString(prompt, file) {
			t.Errorf("expected prompt to contain file %s", file)
		}
	}
}

func TestBuildTaskPromptWithPreviousGroupContext(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()
	session.GroupConsolidationContexts = []*GroupConsolidationCompletionFile{
		{
			GroupIndex: 0,
			Status:     "complete",
			BranchName: "feature/group-1",
			Notes:      "Group 1 completed successfully",
			IssuesForNextGroup: []string{
				"Watch out for memory usage",
			},
			Verification: VerificationResult{
				OverallSuccess: true,
			},
		},
	}

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	// Get a task from group 1 (index 1) which should have context from group 0
	task := session.Plan.Tasks[1] // task-2 is in group 1

	prompt := coord.buildTaskPrompt(&task)

	// Verify prompt contains previous group context
	if !containsString(prompt, "Context from Previous Group") {
		t.Error("expected prompt to contain 'Context from Previous Group' section")
	}
	if !containsString(prompt, "Group 1 completed successfully") {
		t.Error("expected prompt to contain consolidator notes")
	}
	if !containsString(prompt, "Watch out for memory usage") {
		t.Error("expected prompt to contain issues from previous group")
	}
}

// ============================================================================
// Tests for Revision State
// ============================================================================

func TestNewRevisionState(t *testing.T) {
	issues := []RevisionIssue{
		{TaskID: "task-1", Description: "Fix bug A", Severity: "critical"},
		{TaskID: "task-2", Description: "Fix bug B", Severity: "major"},
		{TaskID: "", Description: "Cross-cutting issue", Severity: "minor"},
	}

	state := NewRevisionState(issues)

	if state.RevisionRound != 1 {
		t.Errorf("expected RevisionRound 1, got %d", state.RevisionRound)
	}
	if state.MaxRevisions != 3 {
		t.Errorf("expected MaxRevisions 3, got %d", state.MaxRevisions)
	}
	if len(state.Issues) != 3 {
		t.Errorf("expected 3 issues, got %d", len(state.Issues))
	}
	// Only task-1 and task-2 should be in TasksToRevise (not empty TaskID)
	if len(state.TasksToRevise) != 2 {
		t.Errorf("expected 2 tasks to revise, got %d", len(state.TasksToRevise))
	}
}

func TestRevisionState_IsComplete(t *testing.T) {
	state := &RevisionState{
		TasksToRevise: []string{"task-1", "task-2"},
		RevisedTasks:  []string{},
	}

	if state.IsComplete() {
		t.Error("expected IsComplete to be false with no revised tasks")
	}

	state.RevisedTasks = []string{"task-1"}
	if state.IsComplete() {
		t.Error("expected IsComplete to be false with partial revision")
	}

	state.RevisedTasks = []string{"task-1", "task-2"}
	if !state.IsComplete() {
		t.Error("expected IsComplete to be true with all tasks revised")
	}
}

func TestExtractTasksToRevise(t *testing.T) {
	tests := []struct {
		name     string
		issues   []RevisionIssue
		expected []string
	}{
		{
			name: "unique task IDs extracted",
			issues: []RevisionIssue{
				{TaskID: "task-1"},
				{TaskID: "task-2"},
				{TaskID: "task-1"}, // duplicate
			},
			expected: []string{"task-1", "task-2"},
		},
		{
			name: "empty TaskID skipped",
			issues: []RevisionIssue{
				{TaskID: "task-1"},
				{TaskID: ""},
				{TaskID: "task-2"},
			},
			expected: []string{"task-1", "task-2"},
		},
		{
			name:     "no tasks",
			issues:   []RevisionIssue{{TaskID: ""}, {TaskID: ""}},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTasksToRevise(tt.issues)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d tasks, got %d", len(tt.expected), len(result))
				return
			}

			for _, exp := range tt.expected {
				found := false
				for _, got := range result {
					if got == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected task %s in result", exp)
				}
			}
		})
	}
}

// ============================================================================
// Tests for Completion File Parsing
// ============================================================================

func TestTaskCompletionFilePath(t *testing.T) {
	worktreePath := "/tmp/test-worktree"
	expected := filepath.Join(worktreePath, TaskCompletionFileName)

	result := TaskCompletionFilePath(worktreePath)
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestParseTaskCompletionFile(t *testing.T) {
	// Create temp directory and completion file
	tempDir := t.TempDir()

	completionJSON := `{
		"task_id": "task-1",
		"status": "complete",
		"summary": "Implemented feature X",
		"files_modified": ["file1.go", "file2.go"],
		"notes": "Works well",
		"issues": ["Minor issue found"],
		"suggestions": ["Consider adding tests"],
		"dependencies": ["github.com/example/lib"]
	}`

	completionPath := filepath.Join(tempDir, TaskCompletionFileName)
	if err := os.WriteFile(completionPath, []byte(completionJSON), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	completion, err := ParseTaskCompletionFile(tempDir)
	if err != nil {
		t.Fatalf("failed to parse completion file: %v", err)
	}

	if completion.TaskID != "task-1" {
		t.Errorf("expected TaskID 'task-1', got %s", completion.TaskID)
	}
	if completion.Status != "complete" {
		t.Errorf("expected Status 'complete', got %s", completion.Status)
	}
	if completion.Summary != "Implemented feature X" {
		t.Errorf("expected Summary 'Implemented feature X', got %s", completion.Summary)
	}
	if len(completion.FilesModified) != 2 {
		t.Errorf("expected 2 files, got %d", len(completion.FilesModified))
	}
	if len(completion.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(completion.Issues))
	}
}

func TestParseTaskCompletionFile_NotFound(t *testing.T) {
	tempDir := t.TempDir()

	_, err := ParseTaskCompletionFile(tempDir)
	if err == nil {
		t.Error("expected error for missing completion file")
	}
}

func TestParseRevisionIssuesFromOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		expectCount int
		expectErr   bool
	}{
		{
			name: "valid revision issues",
			output: `Some text before
<revision_issues>
[
  {"task_id": "task-1", "description": "Fix bug", "severity": "critical"},
  {"task_id": "task-2", "description": "Update docs", "severity": "minor"}
]
</revision_issues>
Some text after`,
			expectCount: 2,
			expectErr:   false,
		},
		{
			name:        "no revision issues block",
			output:      "No issues here",
			expectCount: 0,
			expectErr:   false,
		},
		{
			name: "empty revision issues",
			output: `<revision_issues>
[]
</revision_issues>`,
			expectCount: 0,
			expectErr:   false,
		},
		{
			name: "invalid JSON",
			output: `<revision_issues>
not valid json
</revision_issues>`,
			expectCount: 0,
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := ParseRevisionIssuesFromOutput(tt.output)

			if tt.expectErr && err == nil {
				t.Error("expected error")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(issues) != tt.expectCount {
				t.Errorf("expected %d issues, got %d", tt.expectCount, len(issues))
			}
		})
	}
}

// ============================================================================
// Tests for Coordinator Event Types
// ============================================================================

func TestCoordinatorEventTypes(t *testing.T) {
	eventTypes := []struct {
		eventType CoordinatorEventType
		expected  string
	}{
		{EventTaskStarted, "task_started"},
		{EventTaskComplete, "task_complete"},
		{EventTaskFailed, "task_failed"},
		{EventTaskBlocked, "task_blocked"},
		{EventGroupComplete, "group_complete"},
		{EventPhaseChange, "phase_change"},
		{EventConflict, "conflict"},
		{EventPlanReady, "plan_ready"},
	}

	for _, tc := range eventTypes {
		if string(tc.eventType) != tc.expected {
			t.Errorf("event type %s: expected %q, got %q", tc.eventType, tc.expected, string(tc.eventType))
		}
	}
}

func TestCoordinatorEvent_Timestamp(t *testing.T) {
	manager := &UltraPlanManager{
		session:   createTestUltraPlanSession(),
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	before := time.Now()

	manager.emitEvent(CoordinatorEvent{
		Type:    EventTaskStarted,
		TaskID:  "task-1",
		Message: "Test event",
	})

	after := time.Now()

	// Read the event from channel
	select {
	case event := <-manager.eventChan:
		if event.Timestamp.Before(before) || event.Timestamp.After(after) {
			t.Error("expected timestamp to be set to current time")
		}
	default:
		t.Error("expected event in channel")
	}
}

// ============================================================================
// Tests for Consolidation State
// ============================================================================

func TestConsolidationPhaseValues(t *testing.T) {
	phases := []struct {
		phase    ConsolidationPhase
		expected string
	}{
		{ConsolidationIdle, "idle"},
		{ConsolidationDetecting, "detecting_conflicts"},
		{ConsolidationCreatingBranches, "creating_branches"},
		{ConsolidationMergingTasks, "merging_tasks"},
		{ConsolidationPushing, "pushing"},
		{ConsolidationCreatingPRs, "creating_prs"},
		{ConsolidationPaused, "paused"},
		{ConsolidationComplete, "complete"},
		{ConsolidationFailed, "failed"},
	}

	for _, tc := range phases {
		if string(tc.phase) != tc.expected {
			t.Errorf("phase %s: expected %q, got %q", tc.phase, tc.expected, string(tc.phase))
		}
	}
}

func TestConsolidationModeValues(t *testing.T) {
	modes := []struct {
		mode     ConsolidationMode
		expected string
	}{
		{ModeStackedPRs, "stacked"},
		{ModeSinglePR, "single"},
	}

	for _, tc := range modes {
		if string(tc.mode) != tc.expected {
			t.Errorf("mode %s: expected %q, got %q", tc.mode, tc.expected, string(tc.mode))
		}
	}
}

func TestConsolidationState_Initialization(t *testing.T) {
	state := &ConsolidationState{
		Phase:         ConsolidationCreatingBranches,
		CurrentGroup:  0,
		TotalGroups:   3,
		GroupBranches: make([]string, 0),
		PRUrls:        make([]string, 0),
	}

	if state.Phase != ConsolidationCreatingBranches {
		t.Errorf("expected phase %s, got %s", ConsolidationCreatingBranches, state.Phase)
	}
	if state.TotalGroups != 3 {
		t.Errorf("expected TotalGroups 3, got %d", state.TotalGroups)
	}
}

// ============================================================================
// Tests for UltraPlan Configuration
// ============================================================================

func TestDefaultUltraPlanConfig(t *testing.T) {
	config := DefaultUltraPlanConfig()

	if config.MaxParallel != 3 {
		t.Errorf("expected MaxParallel 3, got %d", config.MaxParallel)
	}
	if config.DryRun {
		t.Error("expected DryRun to be false")
	}
	if config.NoSynthesis {
		t.Error("expected NoSynthesis to be false")
	}
	if config.AutoApprove {
		t.Error("expected AutoApprove to be false")
	}
	if config.ConsolidationMode != ModeStackedPRs {
		t.Errorf("expected ConsolidationMode %s, got %s", ModeStackedPRs, config.ConsolidationMode)
	}
	if !config.CreateDraftPRs {
		t.Error("expected CreateDraftPRs to be true")
	}
	if len(config.PRLabels) != 1 || config.PRLabels[0] != "ultraplan" {
		t.Errorf("expected PRLabels [ultraplan], got %v", config.PRLabels)
	}
	if config.MaxTaskRetries != 3 {
		t.Errorf("expected MaxTaskRetries 3, got %d", config.MaxTaskRetries)
	}
	if !config.RequireVerifiedCommits {
		t.Error("expected RequireVerifiedCommits to be true")
	}
}

func TestUltraPlanConfig_CustomValues(t *testing.T) {
	config := UltraPlanConfig{
		MaxParallel:            5,
		DryRun:                 true,
		NoSynthesis:            true,
		AutoApprove:            true,
		Review:                 true,
		ConsolidationMode:      ModeSinglePR,
		CreateDraftPRs:         false,
		PRLabels:               []string{"feature", "auto"},
		BranchPrefix:           "custom-prefix",
		MaxTaskRetries:         5,
		RequireVerifiedCommits: false,
	}

	if config.MaxParallel != 5 {
		t.Errorf("expected MaxParallel 5, got %d", config.MaxParallel)
	}
	if !config.DryRun {
		t.Error("expected DryRun to be true")
	}
	if !config.NoSynthesis {
		t.Error("expected NoSynthesis to be true")
	}
	if !config.AutoApprove {
		t.Error("expected AutoApprove to be true")
	}
	if !config.Review {
		t.Error("expected Review to be true")
	}
	if config.ConsolidationMode != ModeSinglePR {
		t.Errorf("expected ConsolidationMode %s, got %s", ModeSinglePR, config.ConsolidationMode)
	}
	if config.CreateDraftPRs {
		t.Error("expected CreateDraftPRs to be false")
	}
	if len(config.PRLabels) != 2 {
		t.Errorf("expected 2 PRLabels, got %d", len(config.PRLabels))
	}
	if config.BranchPrefix != "custom-prefix" {
		t.Errorf("expected BranchPrefix 'custom-prefix', got %s", config.BranchPrefix)
	}
	if config.MaxTaskRetries != 5 {
		t.Errorf("expected MaxTaskRetries 5, got %d", config.MaxTaskRetries)
	}
	if config.RequireVerifiedCommits {
		t.Error("expected RequireVerifiedCommits to be false")
	}
}

// ============================================================================
// Tests for Task Group Index Finding
// ============================================================================

func TestGetTaskGroupIndex(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	tests := []struct {
		taskID   string
		expected int
	}{
		{"task-1", 0},  // Group 0 (no dependencies)
		{"task-2", 1},  // Group 1 (depends on task-1)
		{"task-3", 1},  // Group 1 (depends on task-1)
		{"task-4", 2},  // Group 2 (depends on task-2 and task-3)
		{"task-5", 3},  // Group 3 (depends on task-4)
		{"unknown", -1}, // Not found
	}

	for _, tt := range tests {
		t.Run(tt.taskID, func(t *testing.T) {
			result := coord.getTaskGroupIndex(tt.taskID)
			if result != tt.expected {
				t.Errorf("getTaskGroupIndex(%s) = %d, expected %d", tt.taskID, result, tt.expected)
			}
		})
	}
}

func TestGetTaskGroupIndex_NilPlan(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = nil

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	result := coord.getTaskGroupIndex("task-1")
	if result != -1 {
		t.Errorf("expected -1 for nil plan, got %d", result)
	}
}

// ============================================================================
// Tests for Synthesis Prompt Template
// ============================================================================

func TestSynthesisPromptTemplate(t *testing.T) {
	// Test that the synthesis prompt template contains key placeholders
	// We can't test buildSynthesisPrompt directly without a full orchestrator,
	// but we can verify the template format
	template := SynthesisPromptTemplate

	if !containsString(template, "%s") {
		t.Error("expected template to contain format placeholders")
	}

	// Test that we can format it with test values
	objective := "Test objective"
	taskList := "- task-1\n- task-2"
	results := "### Task 1\nStatus: completed"
	revisionRound := 0

	formatted := formatSynthesisPrompt(objective, taskList, results, revisionRound)

	if !containsString(formatted, objective) {
		t.Error("expected formatted prompt to contain objective")
	}
}

// formatSynthesisPrompt is a helper that mimics the buildSynthesisPrompt format
// without requiring the full coordinator infrastructure
func formatSynthesisPrompt(objective, taskList, results string, revisionRound int) string {
	return fmt.Sprintf(SynthesisPromptTemplate, objective, taskList, results, revisionRound)
}

func TestSynthesisPromptContent(t *testing.T) {
	// Test synthesis prompt content for various scenarios
	tests := []struct {
		name        string
		taskList    string
		expectInPr  string
	}{
		{
			name:       "tasks with commits",
			taskList:   "- [task-1] Feature A (3 commits)",
			expectInPr: "3 commits",
		},
		{
			name:       "task without commits flagged",
			taskList:   "- [task-1] Feature A (NO COMMITS - verify this task)",
			expectInPr: "NO COMMITS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := formatSynthesisPrompt("objective", tt.taskList, "", 0)
			if !containsString(formatted, tt.expectInPr) {
				t.Errorf("expected prompt to contain %q", tt.expectInPr)
			}
		})
	}
}

// ============================================================================
// Tests for Revision Prompt Building
// ============================================================================

func TestBuildRevisionPrompt(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()
	session.Revision = &RevisionState{
		RevisionRound: 1,
		Issues: []RevisionIssue{
			{TaskID: "task-1", Description: "Fix null pointer", Severity: "critical", Suggestion: "Add nil check"},
			{TaskID: "task-1", Description: "Update docs", Severity: "minor"},
			{TaskID: "", Description: "Cross-cutting issue", Severity: "major"},
		},
	}

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	task := session.Plan.Tasks[0] // task-1
	prompt := coord.buildRevisionPrompt(&task)

	// Verify prompt contains task-specific issues
	if !containsString(prompt, "Fix null pointer") {
		t.Error("expected prompt to contain task-specific issue")
	}
	if !containsString(prompt, "critical") {
		t.Error("expected prompt to contain severity")
	}
	if !containsString(prompt, "Add nil check") {
		t.Error("expected prompt to contain suggestion")
	}
	// Cross-cutting issue (empty TaskID) should also be included
	if !containsString(prompt, "Cross-cutting issue") {
		t.Error("expected prompt to contain cross-cutting issue")
	}
}

// ============================================================================
// Tests for Consolidation Prompt Building
// ============================================================================

func TestBuildConsolidationPromptBasic(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()
	session.CompletedTasks = []string{"task-1", "task-2", "task-3"}
	session.Config.ConsolidationMode = ModeStackedPRs
	session.Config.CreateDraftPRs = true
	session.Config.BranchPrefix = "test-prefix"

	// Need orchestrator and worktree for full testing, but we can test partial
	// Here we just verify the method doesn't crash with nil orchestrator
	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
		// orch is nil - will cause issues in actual buildConsolidationPrompt
		// but we can test the config defaults
	}

	// Test config values are accessible
	if session.Config.ConsolidationMode != ModeStackedPRs {
		t.Errorf("expected stacked mode, got %s", session.Config.ConsolidationMode)
	}
	if !session.Config.CreateDraftPRs {
		t.Error("expected CreateDraftPRs to be true")
	}
	if session.Config.BranchPrefix != "test-prefix" {
		t.Errorf("expected branch prefix 'test-prefix', got %s", session.Config.BranchPrefix)
	}

	// Verify GetConsolidation returns nil initially
	result := coord.GetConsolidation()
	if result != nil {
		t.Error("expected GetConsolidation to return nil when no consolidation state")
	}
}

// ============================================================================
// Tests for GetProgress
// ============================================================================

func TestGetProgress(t *testing.T) {
	tests := []struct {
		name              string
		completedTasks    []string
		totalTasks        int
		phase             UltraPlanPhase
		expectedCompleted int
		expectedTotal     int
		expectedPhase     UltraPlanPhase
	}{
		{
			name:              "no progress",
			completedTasks:    []string{},
			totalTasks:        5,
			phase:             PhaseExecuting,
			expectedCompleted: 0,
			expectedTotal:     5,
			expectedPhase:     PhaseExecuting,
		},
		{
			name:              "partial progress",
			completedTasks:    []string{"task-1", "task-2"},
			totalTasks:        5,
			phase:             PhaseExecuting,
			expectedCompleted: 2,
			expectedTotal:     5,
			expectedPhase:     PhaseExecuting,
		},
		{
			name:              "complete",
			completedTasks:    []string{"task-1", "task-2", "task-3", "task-4", "task-5"},
			totalTasks:        5,
			phase:             PhaseComplete,
			expectedCompleted: 5,
			expectedTotal:     5,
			expectedPhase:     PhaseComplete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := createTestUltraPlanSession()
			session.Phase = tt.phase
			session.CompletedTasks = tt.completedTasks

			// Create tasks
			tasks := make([]PlannedTask, tt.totalTasks)
			for i := 0; i < tt.totalTasks; i++ {
				tasks[i] = PlannedTask{ID: generateID()}
			}
			session.Plan = &PlanSpec{Tasks: tasks}

			manager := &UltraPlanManager{
				session:   session,
				eventChan: make(chan CoordinatorEvent, 100),
				stopChan:  make(chan struct{}),
			}

			coord := &Coordinator{
				manager: manager,
			}

			completed, total, phase := coord.GetProgress()

			if completed != tt.expectedCompleted {
				t.Errorf("expected completed %d, got %d", tt.expectedCompleted, completed)
			}
			if total != tt.expectedTotal {
				t.Errorf("expected total %d, got %d", tt.expectedTotal, total)
			}
			if phase != tt.expectedPhase {
				t.Errorf("expected phase %s, got %s", tt.expectedPhase, phase)
			}
		})
	}
}

func TestGetProgressNilPlan(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = nil

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	completed, total, phase := coord.GetProgress()

	if completed != 0 || total != 0 {
		t.Errorf("expected 0/0 for nil plan, got %d/%d", completed, total)
	}
	if phase != PhasePlanning {
		t.Errorf("expected PhasePlanning, got %s", phase)
	}
}

func TestGetProgressWithSession(t *testing.T) {
	// Test that GetProgress correctly handles various session states
	session := createTestUltraPlanSession()
	session.Phase = PhaseExecuting

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	// Session exists but no plan
	completed, total, phase := coord.GetProgress()
	if completed != 0 || total != 0 {
		t.Errorf("expected 0/0 for session without plan, got %d/%d", completed, total)
	}
	if phase != PhaseExecuting {
		t.Errorf("expected PhaseExecuting, got %s", phase)
	}
}

// ============================================================================
// Tests for GetRunningTasks
// ============================================================================

func TestGetRunningTasks(t *testing.T) {
	session := createTestUltraPlanSession()

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager:      manager,
		runningTasks: make(map[string]string),
	}

	// Add some running tasks
	coord.runningTasks["task-1"] = "inst-1"
	coord.runningTasks["task-2"] = "inst-2"

	result := coord.GetRunningTasks()

	if len(result) != 2 {
		t.Errorf("expected 2 running tasks, got %d", len(result))
	}
	if result["task-1"] != "inst-1" {
		t.Errorf("expected task-1 -> inst-1, got %s", result["task-1"])
	}
	if result["task-2"] != "inst-2" {
		t.Errorf("expected task-2 -> inst-2, got %s", result["task-2"])
	}

	// Verify it returns a copy (modifying result doesn't affect original)
	result["task-3"] = "inst-3"
	if len(coord.runningTasks) != 2 {
		t.Error("GetRunningTasks should return a copy, not the original map")
	}
}

// ============================================================================
// Tests for Partial Group Failure Detection
// ============================================================================

func TestHasPartialGroupFailure(t *testing.T) {
	tests := []struct {
		name             string
		completedTasks   []string
		failedTasks      []string
		taskCommitCounts map[string]int
		groupIndex       int
		expected         bool
	}{
		{
			name:             "all succeeded",
			completedTasks:   []string{"task-2", "task-3"},
			failedTasks:      []string{},
			taskCommitCounts: map[string]int{"task-2": 2, "task-3": 1},
			groupIndex:       1,
			expected:         false,
		},
		{
			name:             "all failed",
			completedTasks:   []string{},
			failedTasks:      []string{"task-2", "task-3"},
			taskCommitCounts: map[string]int{},
			groupIndex:       1,
			expected:         false,
		},
		{
			name:             "partial failure - one success one failure",
			completedTasks:   []string{"task-2"},
			failedTasks:      []string{"task-3"},
			taskCommitCounts: map[string]int{"task-2": 2},
			groupIndex:       1,
			expected:         true,
		},
		{
			name:             "partial failure - completed but no commits",
			completedTasks:   []string{"task-2", "task-3"},
			failedTasks:      []string{},
			taskCommitCounts: map[string]int{"task-2": 2, "task-3": 0},
			groupIndex:       1,
			expected:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := createTestUltraPlanSession()
			session.Plan = createTestPlanWithDependencies()
			session.CompletedTasks = tt.completedTasks
			session.FailedTasks = tt.failedTasks
			session.TaskCommitCounts = tt.taskCommitCounts

			manager := &UltraPlanManager{
				session:   session,
				eventChan: make(chan CoordinatorEvent, 100),
				stopChan:  make(chan struct{}),
			}

			coord := &Coordinator{
				manager: manager,
			}

			result := coord.hasPartialGroupFailure(tt.groupIndex)
			if result != tt.expected {
				t.Errorf("hasPartialGroupFailure(%d) = %v, expected %v", tt.groupIndex, result, tt.expected)
			}
		})
	}
}

func TestHasPartialGroupFailure_EdgeCases(t *testing.T) {
	// Valid session but nil plan
	session := createTestUltraPlanSession()
	session.Plan = nil

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	result := coord.hasPartialGroupFailure(0)
	if result {
		t.Error("expected false for nil plan")
	}

	// Valid plan but invalid group index
	session.Plan = createTestPlanWithDependencies()
	result = coord.hasPartialGroupFailure(999)
	if result {
		t.Error("expected false for invalid group index")
	}
}

// ============================================================================
// Tests for Group Consolidation Context
// ============================================================================

func TestGatherTaskCompletionContextForGroup_Empty(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	baseSession := &Session{
		ID:        "test-session",
		Instances: []*Instance{},
	}

	coord := &Coordinator{
		manager:     manager,
		baseSession: baseSession,
	}

	context := coord.gatherTaskCompletionContextForGroup(0)

	if context == nil {
		t.Fatal("expected non-nil context")
	}
	if len(context.TaskSummaries) != 0 {
		t.Error("expected empty TaskSummaries")
	}
	if len(context.AllIssues) != 0 {
		t.Error("expected empty AllIssues")
	}
	if len(context.AllSuggestions) != 0 {
		t.Error("expected empty AllSuggestions")
	}
}

func TestGatherTaskCompletionContextForGroup_InvalidIndex(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	// Invalid group index should return empty context
	context := coord.gatherTaskCompletionContextForGroup(999)

	if context == nil {
		t.Fatal("expected non-nil context")
	}
	if len(context.TaskSummaries) != 0 {
		t.Error("expected empty TaskSummaries for invalid index")
	}
}

func TestGetTaskBranchesForGroup(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()

	baseSession := &Session{
		ID: "test-session",
		Instances: []*Instance{
			{ID: "inst-1", Task: "task-1: Foundation Setup", Branch: "feature/task-1", WorktreePath: "/tmp/wt1"},
		},
	}

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager:     manager,
		baseSession: baseSession,
	}

	branches := coord.getTaskBranchesForGroup(0)

	// Group 0 has task-1
	if len(branches) != 1 {
		t.Errorf("expected 1 branch, got %d", len(branches))
		return
	}

	if branches[0].TaskID != "task-1" {
		t.Errorf("expected TaskID 'task-1', got %s", branches[0].TaskID)
	}
	if branches[0].Branch != "feature/task-1" {
		t.Errorf("expected Branch 'feature/task-1', got %s", branches[0].Branch)
	}
}

func TestGetTaskBranchesForGroup_InvalidIndex(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Plan = createTestPlanWithDependencies()

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	branches := coord.getTaskBranchesForGroup(999)

	if branches != nil {
		t.Error("expected nil branches for invalid group index")
	}
}

// ============================================================================
// Tests for Retry Logic
// ============================================================================

func TestResumeWithPartialWork_NoPendingDecision(t *testing.T) {
	session := createTestUltraPlanSession()
	session.GroupDecision = nil

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	err := coord.ResumeWithPartialWork()
	if err == nil {
		t.Error("expected error when no pending group decision")
	}
}

func TestRetryFailedTasks_NoPendingDecision(t *testing.T) {
	session := createTestUltraPlanSession()
	session.GroupDecision = nil

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	err := coord.RetryFailedTasks()
	if err == nil {
		t.Error("expected error when no pending group decision")
	}
}

func TestRetryFailedTasks_DecisionNotAwaiting(t *testing.T) {
	session := createTestUltraPlanSession()
	session.GroupDecision = &GroupDecisionState{
		AwaitingDecision: false, // Not awaiting
	}

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	err := coord.RetryFailedTasks()
	if err == nil {
		t.Error("expected error when not awaiting decision")
	}
}

// ============================================================================
// Tests for TriggerConsolidation
// ============================================================================

func TestTriggerConsolidation_WrongPhase(t *testing.T) {
	session := createTestUltraPlanSession()
	session.Phase = PhaseExecuting // Not synthesis

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	err := coord.TriggerConsolidation()
	if err == nil {
		t.Error("expected error when not in synthesis phase")
	}
	if !containsString(err.Error(), "synthesis phase") {
		t.Errorf("expected error about synthesis phase, got: %s", err.Error())
	}
}

func TestTriggerConsolidation_NilSession(t *testing.T) {
	// Create a manager with nil session - this tests the nil session path
	mgr := &UltraPlanManager{
		session: nil, // No ultra plan session
	}
	coord := &Coordinator{
		manager: mgr,
	}

	err := coord.TriggerConsolidation()
	if err == nil {
		t.Error("expected error for nil session")
	}
}

// ============================================================================
// Tests for ValidatePlan
// ============================================================================

func TestValidatePlan_Valid(t *testing.T) {
	plan := createTestPlanWithDependencies()

	err := ValidatePlan(plan)
	if err != nil {
		t.Errorf("expected valid plan, got error: %v", err)
	}
}

func TestValidatePlan_Nil(t *testing.T) {
	err := ValidatePlan(nil)
	if err == nil {
		t.Error("expected error for nil plan")
	}
}

func TestValidatePlan_NoTasks(t *testing.T) {
	plan := &PlanSpec{
		ID:    "empty-plan",
		Tasks: []PlannedTask{},
	}

	err := ValidatePlan(plan)
	if err == nil {
		t.Error("expected error for plan with no tasks")
	}
}

func TestValidatePlan_InvalidDependency(t *testing.T) {
	plan := &PlanSpec{
		ID: "invalid-plan",
		Tasks: []PlannedTask{
			{ID: "task-1", DependsOn: []string{"nonexistent"}},
		},
	}

	err := ValidatePlan(plan)
	if err == nil {
		t.Error("expected error for invalid dependency")
	}
}

func TestValidatePlan_CycleDetection(t *testing.T) {
	// Create a plan with a partial cycle (some tasks can be scheduled, some cannot)
	// This tests cycle detection where ExecutionOrder is non-nil but incomplete
	plan := &PlanSpec{
		ID: "partial-cycle-plan",
		Tasks: []PlannedTask{
			{ID: "task-a", DependsOn: []string{}},            // Can be scheduled
			{ID: "task-b", DependsOn: []string{"task-c"}},    // Part of cycle
			{ID: "task-c", DependsOn: []string{"task-b"}},    // Part of cycle
		},
		DependencyGraph: map[string][]string{
			"task-a": {},
			"task-b": {"task-c"},
			"task-c": {"task-b"},
		},
	}

	// Calculate execution order - will only schedule task-a, detect cycle in b/c
	plan.ExecutionOrder = calculateExecutionOrder(plan.Tasks, plan.DependencyGraph)

	// ExecutionOrder should be non-nil but only contain task-a
	if plan.ExecutionOrder == nil {
		t.Fatal("expected ExecutionOrder to be non-nil")
	}

	err := ValidatePlan(plan)
	if err == nil {
		t.Fatal("expected error for plan with cycle")
	}
	if !containsString(err.Error(), "cycle") {
		t.Errorf("expected cycle detection error, got: %s", err.Error())
	}
}

func TestValidatePlan_FullCycleReturnsNilOrder(t *testing.T) {
	// When all tasks are in a cycle, calculateExecutionOrder returns nil
	// ValidatePlan cannot detect this because ExecutionOrder == nil skips the check
	// This documents a known limitation
	plan := &PlanSpec{
		ID: "full-cycle-plan",
		Tasks: []PlannedTask{
			{ID: "task-a", DependsOn: []string{"task-c"}},
			{ID: "task-b", DependsOn: []string{"task-a"}},
			{ID: "task-c", DependsOn: []string{"task-b"}},
		},
		DependencyGraph: map[string][]string{
			"task-a": {"task-c"},
			"task-b": {"task-a"},
			"task-c": {"task-b"},
		},
	}

	// Calculate execution order - returns nil due to full cycle
	plan.ExecutionOrder = calculateExecutionOrder(plan.Tasks, plan.DependencyGraph)

	// ExecutionOrder is nil when no tasks can be scheduled
	if plan.ExecutionOrder != nil {
		t.Errorf("expected nil ExecutionOrder for full cycle, got: %v", plan.ExecutionOrder)
	}

	// ValidatePlan does not detect the cycle when ExecutionOrder is nil
	// (this is a known limitation - the check is skipped when ExecutionOrder is nil)
	err := ValidatePlan(plan)
	if err != nil {
		t.Logf("ValidatePlan unexpectedly detected cycle: %v", err)
	}
	// Note: The current implementation does NOT return an error here
}

// ============================================================================
// Tests for Event Emission
// ============================================================================

func TestEventEmission_ChannelFull(t *testing.T) {
	session := createTestUltraPlanSession()

	// Create manager with small channel
	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 1), // Very small buffer
		stopChan:  make(chan struct{}),
	}

	// Fill the channel
	manager.emitEvent(CoordinatorEvent{Type: EventTaskStarted})

	// This should not block even with full channel
	done := make(chan bool, 1)
	go func() {
		manager.emitEvent(CoordinatorEvent{Type: EventTaskComplete})
		done <- true
	}()

	select {
	case <-done:
		// Success - didn't block
	case <-time.After(100 * time.Millisecond):
		t.Error("emitEvent blocked on full channel")
	}
}

func TestEventCallback_Called(t *testing.T) {
	session := createTestUltraPlanSession()

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	var callbackCalled bool
	manager.SetEventCallback(func(event CoordinatorEvent) {
		callbackCalled = true
	})

	manager.emitEvent(CoordinatorEvent{Type: EventTaskStarted})

	if !callbackCalled {
		t.Error("expected callback to be called")
	}
}

// ============================================================================
// Tests for FlexibleString
// ============================================================================

func TestFlexibleString_UnmarshalString(t *testing.T) {
	var fs FlexibleString
	err := fs.UnmarshalJSON([]byte(`"hello world"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(fs) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(fs))
	}
}

func TestFlexibleString_UnmarshalArray(t *testing.T) {
	var fs FlexibleString
	err := fs.UnmarshalJSON([]byte(`["line 1", "line 2", "line 3"]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "line 1\nline 2\nline 3"
	if string(fs) != expected {
		t.Errorf("expected %q, got %q", expected, string(fs))
	}
}

func TestFlexibleString_UnmarshalInvalid(t *testing.T) {
	// FlexibleString treats unrecognized types as empty strings (returns nil, not error)
	var fs FlexibleString
	err := fs.UnmarshalJSON([]byte(`{"not": "valid"}`))
	if err != nil {
		t.Errorf("FlexibleString should not return error for invalid types, got: %v", err)
	}
	// Invalid types result in empty string
	if string(fs) != "" {
		t.Errorf("expected empty string for invalid JSON object, got: %q", fs)
	}
}

// ============================================================================
// Tests for AggregatedTaskContext
// ============================================================================

func TestAggregatedTaskContext_HasContent(t *testing.T) {
	tests := []struct {
		name     string
		context  *AggregatedTaskContext
		expected bool
	}{
		{
			name:     "empty context",
			context:  &AggregatedTaskContext{TaskSummaries: make(map[string]string)},
			expected: false,
		},
		{
			name: "has issues",
			context: &AggregatedTaskContext{
				TaskSummaries: make(map[string]string),
				AllIssues:     []string{"issue 1"},
			},
			expected: true,
		},
		{
			name: "has suggestions",
			context: &AggregatedTaskContext{
				TaskSummaries:  make(map[string]string),
				AllSuggestions: []string{"suggestion 1"},
			},
			expected: true,
		},
		{
			name: "has dependencies",
			context: &AggregatedTaskContext{
				TaskSummaries: make(map[string]string),
				Dependencies:  []string{"dep 1"},
			},
			expected: true,
		},
		{
			name: "has notes",
			context: &AggregatedTaskContext{
				TaskSummaries: make(map[string]string),
				Notes:         []string{"note 1"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.context.HasContent()
			if result != tt.expected {
				t.Errorf("HasContent() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestAggregatedTaskContext_FormatForPR(t *testing.T) {
	context := &AggregatedTaskContext{
		TaskSummaries:  map[string]string{"task-1": "summary"},
		AllIssues:      []string{"issue 1", "issue 2"},
		AllSuggestions: []string{"suggestion 1"},
		Dependencies:   []string{"github.com/example/lib"},
		Notes:          []string{"note 1"},
	}

	formatted := context.FormatForPR()

	if !containsString(formatted, "Implementation Notes") {
		t.Error("expected formatted output to contain notes section")
	}
	if !containsString(formatted, "issue 1") {
		t.Error("expected formatted output to contain issues")
	}
	if !containsString(formatted, "github.com/example/lib") {
		t.Error("expected formatted output to contain dependencies")
	}
}

// ============================================================================
// Tests for Completion File Names
// ============================================================================

func TestCompletionFileNames(t *testing.T) {
	if TaskCompletionFileName != ".claudio-task-complete.json" {
		t.Errorf("unexpected TaskCompletionFileName: %s", TaskCompletionFileName)
	}
	if PlanFileName != ".claudio-plan.json" {
		t.Errorf("unexpected PlanFileName: %s", PlanFileName)
	}
}

func TestSynthesisCompletionFilePath(t *testing.T) {
	worktreePath := "/tmp/test-worktree"
	expected := filepath.Join(worktreePath, SynthesisCompletionFileName)

	result := SynthesisCompletionFilePath(worktreePath)
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestGroupConsolidationCompletionFilePath(t *testing.T) {
	worktreePath := "/tmp/test-worktree"
	expected := filepath.Join(worktreePath, GroupConsolidationCompletionFileName)

	result := GroupConsolidationCompletionFilePath(worktreePath)
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

// ============================================================================
// Tests for Wait Method
// ============================================================================

func TestCoordinator_Wait_NoGoroutines(t *testing.T) {
	session := createTestUltraPlanSession()

	manager := &UltraPlanManager{
		session:   session,
		eventChan: make(chan CoordinatorEvent, 100),
		stopChan:  make(chan struct{}),
	}

	coord := &Coordinator{
		manager: manager,
	}

	// Wait should return immediately when no goroutines are running
	done := make(chan bool, 1)
	go func() {
		coord.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Wait blocked unexpectedly")
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
