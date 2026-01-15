package phase

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockExecutionSession implements ExecutionSessionInterface for testing.
type mockExecutionSession struct {
	mockSession
	currentGroup   int
	completedCount int
	failedCount    int
	totalCount     int
	maxParallel    int
	multiPass      bool
	planSummary    string
	contexts       map[int]GroupConsolidationContextData
}

func newMockExecutionSession() *mockExecutionSession {
	return &mockExecutionSession{
		maxParallel: 3,
		totalCount:  5,
		planSummary: "Test Plan Summary",
		contexts:    make(map[int]GroupConsolidationContextData),
	}
}

func (m *mockExecutionSession) GetCurrentGroup() int       { return m.currentGroup }
func (m *mockExecutionSession) GetCompletedTaskCount() int { return m.completedCount }
func (m *mockExecutionSession) GetFailedTaskCount() int    { return m.failedCount }
func (m *mockExecutionSession) GetTotalTaskCount() int     { return m.totalCount }
func (m *mockExecutionSession) GetMaxParallel() int        { return m.maxParallel }
func (m *mockExecutionSession) IsMultiPass() bool          { return m.multiPass }
func (m *mockExecutionSession) GetPlanSummary() string     { return m.planSummary }
func (m *mockExecutionSession) GetGroupConsolidationContext(groupIndex int) GroupConsolidationContextData {
	return m.contexts[groupIndex]
}

// mockExecutionOrchestrator implements ExecutionOrchestratorInterface for testing.
type mockExecutionOrchestrator struct {
	mockOrchestrator
	instances          map[string]any
	instanceFromBranch bool
	stopCalls          []string
	mu                 sync.Mutex
}

func newMockExecutionOrchestrator() *mockExecutionOrchestrator {
	return &mockExecutionOrchestrator{
		instances: make(map[string]any),
	}
}

func (m *mockExecutionOrchestrator) AddInstanceFromBranch(session any, task string, baseBranch string) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instanceFromBranch = true
	inst := &mockInstance{id: "inst-from-branch", worktreePath: "/tmp/worktree"}
	m.instances[inst.id] = inst
	return inst, nil
}

func (m *mockExecutionOrchestrator) StopInstance(inst any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mi, ok := inst.(*mockInstance); ok {
		m.stopCalls = append(m.stopCalls, mi.id)
	}
	return nil
}

func (m *mockExecutionOrchestrator) GetInstanceByID(id string) any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.instances[id]
}

// mockInstance represents a test instance.
type mockInstance struct {
	id           string
	worktreePath string
}

func (m *mockInstance) GetID() string           { return m.id }
func (m *mockInstance) GetWorktreePath() string { return m.worktreePath }

// mockExecutionCoordinator implements ExecutionCoordinatorInterface for testing.
type mockExecutionCoordinator struct {
	baseBranches    map[int]string
	runningTasks    map[string]string
	taskGroups      map[string]int
	completionCalls []TaskCompletion
	verifyResults   map[string]TaskCompletion
	completionFiles map[string]bool
	taskStartCalls  []struct{ taskID, instanceID string }
	taskFailedCalls []struct{ taskID, reason string }
	progressCalls   int
	finishCalls     int
	groupAddCalls   []string
	mu              sync.Mutex
}

func newMockExecutionCoordinator() *mockExecutionCoordinator {
	return &mockExecutionCoordinator{
		baseBranches:    make(map[int]string),
		runningTasks:    make(map[string]string),
		taskGroups:      make(map[string]int),
		verifyResults:   make(map[string]TaskCompletion),
		completionFiles: make(map[string]bool),
	}
}

func (m *mockExecutionCoordinator) GetBaseBranchForGroup(groupIndex int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.baseBranches[groupIndex]
}

func (m *mockExecutionCoordinator) AddRunningTask(taskID, instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runningTasks[taskID] = instanceID
}

func (m *mockExecutionCoordinator) RemoveRunningTask(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.runningTasks[taskID]; exists {
		delete(m.runningTasks, taskID)
		return true
	}
	return false
}

func (m *mockExecutionCoordinator) GetRunningTaskCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.runningTasks)
}

func (m *mockExecutionCoordinator) IsTaskRunning(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.runningTasks[taskID]
	return exists
}

func (m *mockExecutionCoordinator) GetBaseSession() any {
	return nil
}

func (m *mockExecutionCoordinator) GetTaskGroupIndex(taskID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.taskGroups[taskID]
}

func (m *mockExecutionCoordinator) VerifyTaskWork(taskID string, inst any) TaskCompletion {
	m.mu.Lock()
	defer m.mu.Unlock()
	if result, ok := m.verifyResults[taskID]; ok {
		return result
	}
	return TaskCompletion{TaskID: taskID, Success: true}
}

func (m *mockExecutionCoordinator) CheckForTaskCompletionFile(inst any) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mi, ok := inst.(*mockInstance); ok {
		return m.completionFiles[mi.id]
	}
	return false
}

func (m *mockExecutionCoordinator) HandleTaskCompletion(completion TaskCompletion) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completionCalls = append(m.completionCalls, completion)
}

func (m *mockExecutionCoordinator) PollTaskCompletions(completionChan chan<- TaskCompletion) {
	// No-op for testing
}

func (m *mockExecutionCoordinator) NotifyTaskStart(taskID, instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskStartCalls = append(m.taskStartCalls, struct{ taskID, instanceID string }{taskID, instanceID})
}

func (m *mockExecutionCoordinator) NotifyTaskFailed(taskID, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskFailedCalls = append(m.taskFailedCalls, struct{ taskID, reason string }{taskID, reason})
}

func (m *mockExecutionCoordinator) NotifyProgress() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressCalls++
}

func (m *mockExecutionCoordinator) FinishExecution() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.finishCalls++
}

func (m *mockExecutionCoordinator) AddInstanceToGroup(instanceID string, isMultiPass bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groupAddCalls = append(m.groupAddCalls, instanceID)
}

// mockPlannedTask implements PlannedTaskData for testing.
type mockPlannedTask struct {
	id          string
	title       string
	description string
	files       []string
	noCode      bool
}

func (m *mockPlannedTask) GetID() string          { return m.id }
func (m *mockPlannedTask) GetTitle() string       { return m.title }
func (m *mockPlannedTask) GetDescription() string { return m.description }
func (m *mockPlannedTask) GetFiles() []string     { return m.files }
func (m *mockPlannedTask) IsNoCode() bool         { return m.noCode }

// mockGroupConsolidationContext implements GroupConsolidationContextData for testing.
type mockGroupConsolidationContext struct {
	notes               string
	issuesForNextGroup  []string
	verificationSuccess bool
}

func (m *mockGroupConsolidationContext) GetNotes() string                { return m.notes }
func (m *mockGroupConsolidationContext) GetIssuesForNextGroup() []string { return m.issuesForNextGroup }
func (m *mockGroupConsolidationContext) IsVerificationSuccess() bool     { return m.verificationSuccess }

func TestNewExecutionOrchestrator(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *PhaseContext
		wantErr bool
	}{
		{
			name: "valid context creates orchestrator",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			wantErr: false,
		},
		{
			name: "valid context with all optional fields",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
				Logger:       nil, // Will use NopLogger
				Callbacks:    &mockCallbacks{},
			},
			wantErr: false,
		},
		{
			name: "nil manager returns error",
			ctx: &PhaseContext{
				Manager:      nil,
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			wantErr: true,
		},
		{
			name: "nil orchestrator returns error",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: nil,
				Session:      &mockSession{},
			},
			wantErr: true,
		},
		{
			name: "nil session returns error",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, err := NewExecutionOrchestrator(tt.ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("NewExecutionOrchestrator() expected error, got nil")
				}
				if exec != nil {
					t.Error("NewExecutionOrchestrator() should return nil on error")
				}
			} else {
				if err != nil {
					t.Errorf("NewExecutionOrchestrator() unexpected error: %v", err)
				}
				if exec == nil {
					t.Error("NewExecutionOrchestrator() should return non-nil orchestrator")
				}
			}
		})
	}
}

func TestNewExecutionOrchestratorWithContext(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *ExecutionContext
		wantErr bool
	}{
		{
			name: "valid execution context",
			ctx: &ExecutionContext{
				PhaseContext: &PhaseContext{
					Manager:      &mockManager{},
					Orchestrator: &mockOrchestrator{},
					Session:      &mockSession{},
				},
				Coordinator:           newMockExecutionCoordinator(),
				ExecutionSession:      newMockExecutionSession(),
				ExecutionOrchestrator: newMockExecutionOrchestrator(),
			},
			wantErr: false,
		},
		{
			name:    "nil execution context returns error",
			ctx:     nil,
			wantErr: true,
		},
		{
			name: "nil phase context returns error",
			ctx: &ExecutionContext{
				PhaseContext: nil,
			},
			wantErr: true,
		},
		{
			name: "invalid phase context returns error",
			ctx: &ExecutionContext{
				PhaseContext: &PhaseContext{
					Manager:      nil,
					Orchestrator: &mockOrchestrator{},
					Session:      &mockSession{},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, err := NewExecutionOrchestratorWithContext(tt.ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("NewExecutionOrchestratorWithContext() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("NewExecutionOrchestratorWithContext() unexpected error: %v", err)
				}
				if exec == nil {
					t.Error("NewExecutionOrchestratorWithContext() should return non-nil orchestrator")
				}
			}
		})
	}
}

func TestExecutionOrchestrator_Phase(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	if exec.Phase() != PhaseExecuting {
		t.Errorf("Phase() = %v, want %v", exec.Phase(), PhaseExecuting)
	}
}

func TestExecutionOrchestrator_Execute(t *testing.T) {
	t.Run("Execute with background context completes", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Execute should complete without error since there are no ready tasks
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- exec.Execute(ctx)
		}()

		select {
		case err := <-done:
			if err != nil && err != context.DeadlineExceeded {
				t.Errorf("Execute() unexpected error: %v", err)
			}
		case <-time.After(3 * time.Second):
			exec.Cancel()
			t.Log("Execute() timed out as expected (no tasks to complete)")
		}
	})

	t.Run("Execute when already cancelled returns error", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Cancel before Execute
		exec.Cancel()

		err = exec.Execute(context.Background())
		if err != ErrExecutionCancelled {
			t.Errorf("Execute() error = %v, want %v", err, ErrExecutionCancelled)
		}
	})
}

func TestExecutionOrchestrator_Cancel(t *testing.T) {
	t.Run("Cancel is idempotent", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Call Cancel multiple times - should not panic
		exec.Cancel()
		exec.Cancel()
		exec.Cancel()
	})

	t.Run("Cancel before Execute", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Cancel before Execute is called - should not panic
		exec.Cancel()
	})

	t.Run("Cancel during Execute stops execution", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Start Execute in a goroutine
		done := make(chan struct{})
		go func() {
			_ = exec.Execute(context.Background())
			close(done)
		}()

		// Give Execute a moment to start, then cancel
		time.Sleep(50 * time.Millisecond)
		exec.Cancel()

		// Wait for Execute to complete
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Error("Execute did not complete after Cancel")
		}
	})
}

func TestExecutionOrchestrator_State(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("initial state has empty running tasks", func(t *testing.T) {
		state := exec.State()
		if state.RunningCount != 0 {
			t.Errorf("State().RunningCount = %d, want 0", state.RunningCount)
		}
		if state.CompletedCount != 0 {
			t.Errorf("State().CompletedCount = %d, want 0", state.CompletedCount)
		}
		if state.FailedCount != 0 {
			t.Errorf("State().FailedCount = %d, want 0", state.FailedCount)
		}
		if len(state.RunningTasks) != 0 {
			t.Errorf("State().RunningTasks len = %d, want 0", len(state.RunningTasks))
		}
	})

	t.Run("State returns a copy", func(t *testing.T) {
		state1 := exec.State()
		state2 := exec.State()

		// Modify state1 - should not affect state2
		state1.RunningCount = 99
		if state2.RunningCount == 99 {
			t.Error("State() should return independent copies")
		}
	})
}

func TestExecutionOrchestrator_GetRunningCount(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	if exec.GetRunningCount() != 0 {
		t.Errorf("GetRunningCount() = %d, want 0", exec.GetRunningCount())
	}
}

func TestExecutionOrchestrator_GetCompletedCount(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	if exec.GetCompletedCount() != 0 {
		t.Errorf("GetCompletedCount() = %d, want 0", exec.GetCompletedCount())
	}
}

func TestExecutionOrchestrator_GetFailedCount(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	if exec.GetFailedCount() != 0 {
		t.Errorf("GetFailedCount() = %d, want 0", exec.GetFailedCount())
	}
}

func TestExecutionOrchestrator_Reset(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Modify state
	exec.mu.Lock()
	exec.state.RunningCount = 5
	exec.state.CompletedCount = 10
	exec.state.FailedCount = 2
	exec.state.TotalTasks = 17
	exec.state.RunningTasks["task-1"] = "inst-1"
	exec.mu.Unlock()

	// Reset
	exec.Reset()

	// Verify state is cleared
	state := exec.State()
	if state.RunningCount != 0 {
		t.Errorf("After Reset, RunningCount = %d, want 0", state.RunningCount)
	}
	if state.CompletedCount != 0 {
		t.Errorf("After Reset, CompletedCount = %d, want 0", state.CompletedCount)
	}
	if state.FailedCount != 0 {
		t.Errorf("After Reset, FailedCount = %d, want 0", state.FailedCount)
	}
	if state.TotalTasks != 0 {
		t.Errorf("After Reset, TotalTasks = %d, want 0", state.TotalTasks)
	}
	if len(state.RunningTasks) != 0 {
		t.Errorf("After Reset, RunningTasks len = %d, want 0", len(state.RunningTasks))
	}
}

func TestExecutionOrchestrator_BuildTaskPrompt(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("builds prompt with task data interface", func(t *testing.T) {
		task := &mockPlannedTask{
			id:          "task-1",
			title:       "Implement Feature X",
			description: "Add the new feature X to the system",
			files:       []string{"src/feature.go", "src/feature_test.go"},
		}

		prompt := exec.buildTaskPrompt("task-1", task)

		// Check prompt contains expected sections
		if !contains(prompt, "# Task: Implement Feature X") {
			t.Error("Prompt should contain task title")
		}
		if !contains(prompt, "## Your Task") {
			t.Error("Prompt should contain 'Your Task' section")
		}
		if !contains(prompt, "Add the new feature X") {
			t.Error("Prompt should contain task description")
		}
		if !contains(prompt, "src/feature.go") {
			t.Error("Prompt should contain expected files")
		}
		if !contains(prompt, "## Completion Protocol") {
			t.Error("Prompt should contain completion protocol")
		}
		if !contains(prompt, TaskCompletionFileName) {
			t.Error("Prompt should contain completion file name")
		}
	})

	t.Run("builds fallback prompt for non-interface task", func(t *testing.T) {
		task := "not a PlannedTaskData"

		prompt := exec.buildTaskPrompt("task-2", task)

		if !contains(prompt, "# Task: task-2") {
			t.Error("Fallback prompt should contain task ID")
		}
	})
}

func TestExecutionOrchestrator_BuildTaskPromptWithContext(t *testing.T) {
	execSession := newMockExecutionSession()
	execSession.planSummary = "Major Refactoring Project"
	execSession.contexts[0] = &mockGroupConsolidationContext{
		notes:               "Group 1 completed with minor issues",
		issuesForNextGroup:  []string{"Check API compatibility", "Update tests"},
		verificationSuccess: true,
	}

	execCoord := newMockExecutionCoordinator()
	execCoord.taskGroups["task-2"] = 1 // Task is in group 1

	exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
		PhaseContext: &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		},
		Coordinator:           execCoord,
		ExecutionSession:      execSession,
		ExecutionOrchestrator: newMockExecutionOrchestrator(),
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	task := &mockPlannedTask{
		id:          "task-2",
		title:       "Follow-up Task",
		description: "Build on previous work",
		files:       []string{"src/updated.go"},
	}

	prompt := exec.buildTaskPrompt("task-2", task)

	// Check prompt includes plan summary
	if !contains(prompt, "Major Refactoring Project") {
		t.Error("Prompt should contain plan summary")
	}

	// Check prompt includes context from previous group
	if !contains(prompt, "Context from Previous Group") {
		t.Error("Prompt should contain previous group context section")
	}
	if !contains(prompt, "Group 1 completed with minor issues") {
		t.Error("Prompt should contain consolidator notes")
	}
	if !contains(prompt, "Check API compatibility") {
		t.Error("Prompt should contain issues for next group")
	}
	if !contains(prompt, "verified (build/lint/tests passed)") {
		t.Error("Prompt should indicate verification success")
	}
}

func TestExecutionOrchestrator_HandleTaskCompletion(t *testing.T) {
	mgr := &mockManager{}
	cb := &trackingCallbacks{}

	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      mgr,
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
		Callbacks:    cb,
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Add a running task
	exec.mu.Lock()
	exec.state.RunningTasks["task-1"] = "inst-1"
	exec.state.RunningCount = 1
	exec.mu.Unlock()

	t.Run("successful completion updates state", func(t *testing.T) {
		completion := TaskCompletion{
			TaskID:     "task-1",
			InstanceID: "inst-1",
			Success:    true,
		}

		exec.handleTaskCompletion(completion)

		state := exec.State()
		if state.RunningCount != 0 {
			t.Errorf("RunningCount = %d, want 0", state.RunningCount)
		}
		if state.CompletedCount != 1 {
			t.Errorf("CompletedCount = %d, want 1", state.CompletedCount)
		}
		if state.FailedCount != 0 {
			t.Errorf("FailedCount = %d, want 0", state.FailedCount)
		}
	})

	// Reset for next test
	exec.Reset()
	exec.mu.Lock()
	exec.state.RunningTasks["task-2"] = "inst-2"
	exec.state.RunningCount = 1
	exec.mu.Unlock()

	t.Run("failed completion updates state", func(t *testing.T) {
		completion := TaskCompletion{
			TaskID:     "task-2",
			InstanceID: "inst-2",
			Success:    false,
			Error:      "task failed",
		}

		exec.handleTaskCompletion(completion)

		state := exec.State()
		if state.RunningCount != 0 {
			t.Errorf("RunningCount = %d, want 0", state.RunningCount)
		}
		if state.FailedCount != 1 {
			t.Errorf("FailedCount = %d, want 1", state.FailedCount)
		}
	})
}

func TestExecutionOrchestrator_ConcurrentAccess(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Test concurrent access to state methods
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			_ = exec.GetRunningCount()
			_ = exec.GetCompletedCount()
			_ = exec.GetFailedCount()
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = exec.State()
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			exec.mu.Lock()
			exec.state.RunningCount = i % 5
			exec.mu.Unlock()
		}
		done <- struct{}{}
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent access test timed out")
		}
	}
}

func TestTaskCompletion(t *testing.T) {
	t.Run("completion struct fields", func(t *testing.T) {
		completion := TaskCompletion{
			TaskID:      "task-1",
			InstanceID:  "inst-1",
			Success:     true,
			Error:       "",
			NeedsRetry:  false,
			CommitCount: 3,
		}

		if completion.TaskID != "task-1" {
			t.Errorf("TaskID = %q, want %q", completion.TaskID, "task-1")
		}
		if completion.InstanceID != "inst-1" {
			t.Errorf("InstanceID = %q, want %q", completion.InstanceID, "inst-1")
		}
		if !completion.Success {
			t.Error("Success = false, want true")
		}
		if completion.CommitCount != 3 {
			t.Errorf("CommitCount = %d, want 3", completion.CommitCount)
		}
	})
}

func TestExecutionState(t *testing.T) {
	t.Run("state struct fields", func(t *testing.T) {
		state := ExecutionState{
			RunningTasks:   map[string]string{"t1": "i1", "t2": "i2"},
			RunningCount:   2,
			CompletedCount: 5,
			FailedCount:    1,
			TotalTasks:     8,
		}

		if len(state.RunningTasks) != 2 {
			t.Errorf("RunningTasks len = %d, want 2", len(state.RunningTasks))
		}
		if state.RunningCount != 2 {
			t.Errorf("RunningCount = %d, want 2", state.RunningCount)
		}
		if state.CompletedCount != 5 {
			t.Errorf("CompletedCount = %d, want 5", state.CompletedCount)
		}
		if state.FailedCount != 1 {
			t.Errorf("FailedCount = %d, want 1", state.FailedCount)
		}
		if state.TotalTasks != 8 {
			t.Errorf("TotalTasks = %d, want 8", state.TotalTasks)
		}
	})
}

func TestErrExecutionCancelled(t *testing.T) {
	if ErrExecutionCancelled == nil {
		t.Error("ErrExecutionCancelled should not be nil")
	}
	if ErrExecutionCancelled.Error() != "execution phase cancelled" {
		t.Errorf("ErrExecutionCancelled.Error() = %q, want %q",
			ErrExecutionCancelled.Error(), "execution phase cancelled")
	}
}

// trackingCallbacks tracks callback invocations for testing.
type trackingCallbacks struct {
	mockCallbacks
	taskCompleteCalls []string
	taskFailedCalls   []struct{ taskID, reason string }
	phaseChangeCalls  []UltraPlanPhase
}

func (t *trackingCallbacks) OnTaskComplete(taskID string) {
	t.taskCompleteCalls = append(t.taskCompleteCalls, taskID)
}

func (t *trackingCallbacks) OnTaskFailed(taskID, reason string) {
	t.taskFailedCalls = append(t.taskFailedCalls, struct{ taskID, reason string }{taskID, reason})
}

func (t *trackingCallbacks) OnPhaseChange(phase UltraPlanPhase) {
	t.phaseChangeCalls = append(t.phaseChangeCalls, phase)
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mockTaskVerifier implements TaskVerifierInterface for testing.
type mockTaskVerifier struct {
	completionFileResults map[string]bool
	completionFileErrors  map[string]error
	verifyResults         map[string]TaskVerifyResult
	mu                    sync.Mutex
}

func newMockTaskVerifier() *mockTaskVerifier {
	return &mockTaskVerifier{
		completionFileResults: make(map[string]bool),
		completionFileErrors:  make(map[string]error),
		verifyResults:         make(map[string]TaskVerifyResult),
	}
}

func (m *mockTaskVerifier) CheckCompletionFile(worktreePath string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.completionFileErrors[worktreePath]
	return m.completionFileResults[worktreePath], err
}

func (m *mockTaskVerifier) VerifyTaskWork(taskID, instanceID, worktreePath, baseBranch string, opts *TaskVerifyOptions) TaskVerifyResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	if result, ok := m.verifyResults[taskID]; ok {
		return result
	}
	return TaskVerifyResult{TaskID: taskID, InstanceID: instanceID, Success: true}
}

// mockInstanceWithStatus extends mockInstance with status support.
type mockInstanceWithStatus struct {
	mockInstance
	status InstanceStatus
}

func (m *mockInstanceWithStatus) GetStatus() InstanceStatus  { return m.status }
func (m *mockInstanceWithStatus) SetStatus(s InstanceStatus) { m.status = s }

// mockInstanceManagerChecker implements InstanceManagerCheckerInterface.
type mockInstanceManagerChecker struct {
	tmuxExists bool
}

func (m *mockInstanceManagerChecker) TmuxSessionExists() bool { return m.tmuxExists }

// mockOrchestratorWithManager extends mockOrchestrator to return managers.
type mockOrchestratorWithManager struct {
	mockOrchestrator
	managers map[string]any
}

func newMockOrchestratorWithManager() *mockOrchestratorWithManager {
	return &mockOrchestratorWithManager{
		managers: make(map[string]any),
	}
}

func (m *mockOrchestratorWithManager) GetInstanceManager(id string) any {
	return m.managers[id]
}

func TestExecutionOrchestrator_CheckForTaskCompletionFile(t *testing.T) {
	t.Run("returns false for nil instance", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		result := exec.checkForTaskCompletionFile(nil)
		if result {
			t.Error("checkForTaskCompletionFile should return false for nil instance")
		}
	})

	t.Run("returns false for empty worktree path", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		inst := &mockInstance{id: "inst-1", worktreePath: ""}
		result := exec.checkForTaskCompletionFile(inst)
		if result {
			t.Error("checkForTaskCompletionFile should return false for empty worktree path")
		}
	})

	t.Run("uses local verifier when available", func(t *testing.T) {
		verifier := newMockTaskVerifier()
		verifier.completionFileResults["/tmp/worktree"] = true

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			Verifier: verifier,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"}
		result := exec.checkForTaskCompletionFile(inst)
		if !result {
			t.Error("checkForTaskCompletionFile should return true when verifier finds file")
		}
	})

	t.Run("falls back to coordinator when no verifier", func(t *testing.T) {
		coord := newMockExecutionCoordinator()
		coord.completionFiles["inst-1"] = true

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			Coordinator: coord,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"}
		result := exec.checkForTaskCompletionFile(inst)
		if !result {
			t.Error("checkForTaskCompletionFile should return true via coordinator fallback")
		}
	})
}

func TestExecutionOrchestrator_VerifyTaskWork(t *testing.T) {
	t.Run("uses local verifier when available", func(t *testing.T) {
		verifier := newMockTaskVerifier()
		verifier.verifyResults["task-1"] = TaskVerifyResult{
			TaskID:      "task-1",
			InstanceID:  "inst-1",
			Success:     true,
			CommitCount: 3,
		}

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			Verifier: verifier,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"}
		result := exec.verifyTaskWork("task-1", "inst-1", inst)

		if !result.Success {
			t.Error("verifyTaskWork should return success")
		}
		if result.CommitCount != 3 {
			t.Errorf("CommitCount = %d, want 3", result.CommitCount)
		}
	})

	t.Run("falls back to coordinator when no verifier", func(t *testing.T) {
		coord := newMockExecutionCoordinator()
		coord.verifyResults["task-1"] = TaskCompletion{
			TaskID:      "task-1",
			InstanceID:  "inst-1",
			Success:     true,
			CommitCount: 5,
		}

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			Coordinator: coord,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"}
		result := exec.verifyTaskWork("task-1", "inst-1", inst)

		if !result.Success {
			t.Error("verifyTaskWork should return success via coordinator fallback")
		}
		if result.CommitCount != 5 {
			t.Errorf("CommitCount = %d, want 5", result.CommitCount)
		}
	})

	t.Run("returns success when no verifier or coordinator available", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"}
		result := exec.verifyTaskWork("task-1", "inst-1", inst)

		if !result.Success {
			t.Error("verifyTaskWork should return success in lenient mode")
		}
	})
}

func TestExecutionOrchestrator_PollTaskCompletions(t *testing.T) {
	t.Run("skips tasks with no instance", func(t *testing.T) {
		session := &mockSession{
			taskToInstance: map[string]string{"task-1": "inst-1"},
		}

		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      session,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Mark task as running locally
		exec.mu.Lock()
		exec.state.RunningTasks["task-1"] = "inst-1"
		exec.mu.Unlock()

		completionChan := make(chan TaskCompletion, 10)
		exec.pollTaskCompletions(completionChan)

		// Should not send any completions since instance is nil
		select {
		case <-completionChan:
			t.Error("pollTaskCompletions should not send completion when instance is nil")
		default:
			// Expected - no completion sent
		}
	})

	t.Run("detects completion file and sends result", func(t *testing.T) {
		session := &mockSession{
			taskToInstance: map[string]string{"task-1": "inst-1"},
		}

		execOrch := newMockExecutionOrchestrator()
		inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"}
		execOrch.instances["inst-1"] = inst

		verifier := newMockTaskVerifier()
		verifier.completionFileResults["/tmp/worktree"] = true

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      session,
			},
			ExecutionOrchestrator: execOrch,
			Verifier:              verifier,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Mark task as running locally
		exec.mu.Lock()
		exec.state.RunningTasks["task-1"] = "inst-1"
		exec.mu.Unlock()

		completionChan := make(chan TaskCompletion, 10)
		exec.pollTaskCompletions(completionChan)

		// Should send a completion
		select {
		case completion := <-completionChan:
			if completion.TaskID != "task-1" {
				t.Errorf("TaskID = %q, want %q", completion.TaskID, "task-1")
			}
		default:
			t.Error("pollTaskCompletions should send completion when file is detected")
		}
	})

	t.Run("skips already completed tasks", func(t *testing.T) {
		session := &mockSession{
			taskToInstance: map[string]string{"task-1": "inst-1"},
			completedTasks: []string{"task-1"},
		}

		execOrch := newMockExecutionOrchestrator()
		inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"}
		execOrch.instances["inst-1"] = inst

		verifier := newMockTaskVerifier()
		verifier.completionFileResults["/tmp/worktree"] = true

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      session,
			},
			ExecutionOrchestrator: execOrch,
			Verifier:              verifier,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		completionChan := make(chan TaskCompletion, 10)
		exec.pollTaskCompletions(completionChan)

		// Should not send completion for already completed task
		select {
		case <-completionChan:
			t.Error("pollTaskCompletions should skip already completed tasks")
		default:
			// Expected
		}
	})
}

func TestExecutionOrchestrator_GetInstanceWorktreePath(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("returns empty for nil instance", func(t *testing.T) {
		result := exec.getInstanceWorktreePath(nil)
		if result != "" {
			t.Errorf("getInstanceWorktreePath(nil) = %q, want empty string", result)
		}
	})

	t.Run("extracts path from InstanceInterface", func(t *testing.T) {
		inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"}
		result := exec.getInstanceWorktreePath(inst)
		if result != "/tmp/worktree" {
			t.Errorf("getInstanceWorktreePath() = %q, want %q", result, "/tmp/worktree")
		}
	})
}

func TestExecutionOrchestrator_GetInstanceStatus(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("returns empty for nil instance", func(t *testing.T) {
		result := exec.getInstanceStatus(nil)
		if result != "" {
			t.Errorf("getInstanceStatus(nil) = %q, want empty string", result)
		}
	})

	t.Run("extracts status from instance with GetStatus", func(t *testing.T) {
		inst := &mockInstanceWithStatus{
			mockInstance: mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"},
			status:       StatusCompleted,
		}
		result := exec.getInstanceStatus(inst)
		if result != StatusCompleted {
			t.Errorf("getInstanceStatus() = %q, want %q", result, StatusCompleted)
		}
	})
}

func TestExecutionOrchestrator_SetInstanceStatus(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("returns false for nil instance", func(t *testing.T) {
		result := exec.setInstanceStatus(nil, StatusRunning)
		if result {
			t.Error("setInstanceStatus(nil) should return false")
		}
	})

	t.Run("sets status on instance with SetStatus", func(t *testing.T) {
		inst := &mockInstanceWithStatus{
			mockInstance: mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"},
			status:       StatusPending,
		}
		result := exec.setInstanceStatus(inst, StatusRunning)
		if !result {
			t.Error("setInstanceStatus should return true")
		}
		if inst.status != StatusRunning {
			t.Errorf("status = %q, want %q", inst.status, StatusRunning)
		}
	})
}

func TestExecutionOrchestrator_GetInstanceManager(t *testing.T) {
	t.Run("returns nil when orchestrator is nil", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{}, // Returns nil for GetInstanceManager
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		result := exec.getInstanceManager("inst-1")
		if result != nil {
			t.Error("getInstanceManager should return nil when orchestrator returns nil")
		}
	})

	t.Run("returns checker when manager implements interface", func(t *testing.T) {
		orch := newMockOrchestratorWithManager()
		orch.managers["inst-1"] = &mockInstanceManagerChecker{tmuxExists: true}

		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: orch,
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		result := exec.getInstanceManager("inst-1")
		if result == nil {
			t.Error("getInstanceManager should return the manager")
		}
		if !result.TmuxSessionExists() {
			t.Error("TmuxSessionExists should return true")
		}
	})
}

func TestTaskVerifyOptions(t *testing.T) {
	t.Run("NoCode field is accessible", func(t *testing.T) {
		opts := TaskVerifyOptions{NoCode: true}
		if !opts.NoCode {
			t.Error("NoCode should be true")
		}
	})
}

func TestTaskVerifyResult(t *testing.T) {
	t.Run("all fields are accessible", func(t *testing.T) {
		result := TaskVerifyResult{
			TaskID:      "task-1",
			InstanceID:  "inst-1",
			Success:     true,
			Error:       "some error",
			NeedsRetry:  true,
			CommitCount: 5,
		}

		if result.TaskID != "task-1" {
			t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
		}
		if result.InstanceID != "inst-1" {
			t.Errorf("InstanceID = %q, want %q", result.InstanceID, "inst-1")
		}
		if !result.Success {
			t.Error("Success should be true")
		}
		if result.Error != "some error" {
			t.Errorf("Error = %q, want %q", result.Error, "some error")
		}
		if !result.NeedsRetry {
			t.Error("NeedsRetry should be true")
		}
		if result.CommitCount != 5 {
			t.Errorf("CommitCount = %d, want 5", result.CommitCount)
		}
	})
}
