package phase

import (
	"context"
	"fmt"
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
	id            string
	worktreePath  string
	branch        string
	status        InstanceStatus
	filesModified []string
}

func (m *mockInstance) GetID() string             { return m.id }
func (m *mockInstance) GetWorktreePath() string   { return m.worktreePath }
func (m *mockInstance) GetBranch() string         { return m.branch }
func (m *mockInstance) GetStatus() InstanceStatus { return m.status }
func (m *mockInstance) GetFilesModified() []string {
	if m.filesModified == nil {
		return []string{}
	}
	return m.filesModified
}

// mockExecutionCoordinator implements ExecutionCoordinatorInterface for testing.
type mockExecutionCoordinator struct {
	baseBranches        map[int]string
	runningTasks        map[string]string
	taskGroups          map[string]int
	completionCalls     []TaskCompletion
	verifyResults       map[string]TaskCompletion
	completionFiles     map[string]bool
	taskStartCalls      []struct{ taskID, instanceID string }
	taskFailedCalls     []struct{ taskID, reason string }
	progressCalls       int
	finishCalls         int
	groupAddCalls       []string
	consolidationCalls  []int
	partialFailureCalls []int
	clearTaskCalls      []string
	saveCalls           int
	synthesisCalls      int
	completeCalls       []struct {
		success bool
		summary string
	}
	sessionPhase     UltraPlanPhase
	sessionError     string
	noSynthesis      bool
	taskCommitCounts map[string]int
	consolidationErr error
	synthesisErr     error
	mu               sync.Mutex
}

func newMockExecutionCoordinator() *mockExecutionCoordinator {
	return &mockExecutionCoordinator{
		baseBranches:     make(map[int]string),
		runningTasks:     make(map[string]string),
		taskGroups:       make(map[string]int),
		verifyResults:    make(map[string]TaskCompletion),
		completionFiles:  make(map[string]bool),
		taskCommitCounts: make(map[string]int),
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

func (m *mockExecutionCoordinator) StartGroupConsolidation(groupIndex int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consolidationCalls = append(m.consolidationCalls, groupIndex)
	return m.consolidationErr
}

func (m *mockExecutionCoordinator) HandlePartialGroupFailure(groupIndex int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.partialFailureCalls = append(m.partialFailureCalls, groupIndex)
}

func (m *mockExecutionCoordinator) ClearTaskFromInstance(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearTaskCalls = append(m.clearTaskCalls, taskID)
}

func (m *mockExecutionCoordinator) SaveSession() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveCalls++
	return nil
}

func (m *mockExecutionCoordinator) RunSynthesis() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.synthesisCalls++
	return m.synthesisErr
}

func (m *mockExecutionCoordinator) NotifyComplete(success bool, summary string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeCalls = append(m.completeCalls, struct {
		success bool
		summary string
	}{success, summary})
}

func (m *mockExecutionCoordinator) SetSessionPhase(phase UltraPlanPhase) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionPhase = phase
}

func (m *mockExecutionCoordinator) SetSessionError(err string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionError = err
}

func (m *mockExecutionCoordinator) GetNoSynthesis() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.noSynthesis
}

func (m *mockExecutionCoordinator) RecordTaskCommitCount(taskID string, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskCommitCounts[taskID] = count
}

func (m *mockExecutionCoordinator) ConsolidateGroupWithVerification(groupIndex int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consolidationCalls = append(m.consolidationCalls, groupIndex)
	return m.consolidationErr
}

func (m *mockExecutionCoordinator) EmitEvent(eventType, message string) {
	// No-op for testing - could track calls if needed
}

func (m *mockExecutionCoordinator) StartExecutionLoop() {
	// No-op for testing - could track calls if needed
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
	taskCompleteCalls  []string
	taskFailedCalls    []struct{ taskID, reason string }
	phaseChangeCalls   []UltraPlanPhase
	groupCompleteCalls []int
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

func (t *trackingCallbacks) OnGroupComplete(groupIndex int) {
	t.groupCompleteCalls = append(t.groupCompleteCalls, groupIndex)
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

// mockGroupTracker implements GroupTrackerInterface for testing.
type mockGroupTracker struct {
	taskGroups      map[string]int
	groupComplete   map[int]bool
	partialFailure  map[int]bool
	nextGroupMap    map[int]int
	doneMap         map[int]bool
	groupTasks      map[int][]GroupTaskInfo
	totalGroupCount int
	mu              sync.Mutex
}

func newMockGroupTracker() *mockGroupTracker {
	return &mockGroupTracker{
		taskGroups:     make(map[string]int),
		groupComplete:  make(map[int]bool),
		partialFailure: make(map[int]bool),
		nextGroupMap:   make(map[int]int),
		doneMap:        make(map[int]bool),
		groupTasks:     make(map[int][]GroupTaskInfo),
	}
}

func (m *mockGroupTracker) GetTaskGroupIndex(taskID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.taskGroups[taskID]
}

func (m *mockGroupTracker) IsGroupComplete(groupIndex int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.groupComplete[groupIndex]
}

func (m *mockGroupTracker) HasPartialFailure(groupIndex int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.partialFailure[groupIndex]
}

func (m *mockGroupTracker) AdvanceGroup(groupIndex int) (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.nextGroupMap[groupIndex]
	done := m.doneMap[groupIndex]
	return next, done
}

func (m *mockGroupTracker) GetGroupTasks(groupIndex int) []GroupTaskInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.groupTasks[groupIndex]
}

func (m *mockGroupTracker) TotalGroups() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totalGroupCount
}

func (m *mockGroupTracker) HasMoreGroups(groupIndex int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return groupIndex+1 < m.totalGroupCount
}

// mockSessionWithPhase extends mockSession with SetPhase and SetError.
type mockSessionWithPhase struct {
	mockSession
	phase UltraPlanPhase
	err   string
}

func (m *mockSessionWithPhase) SetPhase(phase UltraPlanPhase) { m.phase = phase }
func (m *mockSessionWithPhase) SetError(err string)           { m.err = err }
func (m *mockSessionWithPhase) GetPhase() UltraPlanPhase      { return m.phase }

func TestExecutionOrchestrator_HandleTaskCompletion_DuplicateDetection(t *testing.T) {
	mgr := &mockManager{}
	cb := &trackingCallbacks{}
	coord := newMockExecutionCoordinator()

	exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
		PhaseContext: &PhaseContext{
			Manager:      mgr,
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
			Callbacks:    cb,
		},
		Coordinator: coord,
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Add a running task
	exec.mu.Lock()
	exec.state.RunningTasks["task-1"] = "inst-1"
	exec.state.RunningCount = 1
	exec.mu.Unlock()

	// First completion should be processed
	completion := TaskCompletion{
		TaskID:      "task-1",
		InstanceID:  "inst-1",
		Success:     true,
		CommitCount: 2,
	}
	exec.handleTaskCompletion(completion)

	state := exec.State()
	if state.CompletedCount != 1 {
		t.Errorf("CompletedCount = %d, want 1", state.CompletedCount)
	}
	if !state.ProcessedTasks["task-1"] {
		t.Error("task-1 should be marked as processed")
	}

	// Reset completed count to verify duplicate detection
	completedBefore := state.CompletedCount

	// Second completion (duplicate) should be skipped
	exec.handleTaskCompletion(completion)

	state = exec.State()
	if state.CompletedCount != completedBefore {
		t.Errorf("CompletedCount changed from %d to %d, duplicate should have been skipped",
			completedBefore, state.CompletedCount)
	}
}

func TestExecutionOrchestrator_HandleTaskCompletion_RetryHandling(t *testing.T) {
	mgr := &mockManager{}
	coord := newMockExecutionCoordinator()

	exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
		PhaseContext: &PhaseContext{
			Manager:      mgr,
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		},
		Coordinator: coord,
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Add a running task
	exec.mu.Lock()
	exec.state.RunningTasks["task-1"] = "inst-1"
	exec.state.RunningCount = 1
	exec.mu.Unlock()

	// Completion with NeedsRetry should clear task from instance
	completion := TaskCompletion{
		TaskID:     "task-1",
		InstanceID: "inst-1",
		Success:    false,
		NeedsRetry: true,
	}
	exec.handleTaskCompletion(completion)

	state := exec.State()
	// Task should be removed from running but NOT marked as processed
	if state.RunningCount != 0 {
		t.Errorf("RunningCount = %d, want 0", state.RunningCount)
	}
	if state.ProcessedTasks["task-1"] {
		t.Error("task-1 should NOT be marked as processed for retry")
	}
	if state.CompletedCount != 0 {
		t.Errorf("CompletedCount = %d, want 0 (retry)", state.CompletedCount)
	}
	if state.FailedCount != 0 {
		t.Errorf("FailedCount = %d, want 0 (retry)", state.FailedCount)
	}

	// Coordinator should have been asked to clear the task
	coord.mu.Lock()
	if len(coord.clearTaskCalls) != 1 || coord.clearTaskCalls[0] != "task-1" {
		t.Errorf("ClearTaskFromInstance calls = %v, want [task-1]", coord.clearTaskCalls)
	}
	if coord.saveCalls != 1 {
		t.Errorf("SaveSession calls = %d, want 1", coord.saveCalls)
	}
	coord.mu.Unlock()
}

func TestExecutionOrchestrator_HandleTaskCompletion_RecordCommitCount(t *testing.T) {
	coord := newMockExecutionCoordinator()

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

	// Add a running task
	exec.mu.Lock()
	exec.state.RunningTasks["task-1"] = "inst-1"
	exec.state.RunningCount = 1
	exec.mu.Unlock()

	// Successful completion with commits should record count
	completion := TaskCompletion{
		TaskID:      "task-1",
		InstanceID:  "inst-1",
		Success:     true,
		CommitCount: 5,
	}
	exec.handleTaskCompletion(completion)

	coord.mu.Lock()
	if coord.taskCommitCounts["task-1"] != 5 {
		t.Errorf("taskCommitCounts[task-1] = %d, want 5", coord.taskCommitCounts["task-1"])
	}
	coord.mu.Unlock()
}

func TestExecutionOrchestrator_CheckAndAdvanceGroup(t *testing.T) {
	t.Run("advances group after completion", func(t *testing.T) {
		groupTracker := newMockGroupTracker()
		groupTracker.groupComplete[0] = true
		groupTracker.partialFailure[0] = false
		groupTracker.nextGroupMap[0] = 1
		groupTracker.doneMap[0] = false
		groupTracker.groupTasks[0] = []GroupTaskInfo{{ID: "task-1", Title: "Task 1"}}

		execSession := newMockExecutionSession()
		execSession.currentGroup = 0

		coord := newMockExecutionCoordinator()
		cb := &trackingCallbacks{}

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
				Callbacks:    cb,
			},
			Coordinator:      coord,
			ExecutionSession: execSession,
			GroupTracker:     groupTracker,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.checkAndAdvanceGroup()

		// Should have triggered consolidation
		coord.mu.Lock()
		if len(coord.consolidationCalls) != 1 || coord.consolidationCalls[0] != 0 {
			t.Errorf("consolidationCalls = %v, want [0]", coord.consolidationCalls)
		}
		coord.mu.Unlock()

		// Should have notified group complete callback
		if len(cb.groupCompleteCalls) != 1 || cb.groupCompleteCalls[0] != 0 {
			t.Errorf("OnGroupComplete calls = %v, want [0]", cb.groupCompleteCalls)
		}
	})

	t.Run("handles partial failure", func(t *testing.T) {
		groupTracker := newMockGroupTracker()
		groupTracker.groupComplete[0] = true
		groupTracker.partialFailure[0] = true // Partial failure

		execSession := newMockExecutionSession()
		execSession.currentGroup = 0

		coord := newMockExecutionCoordinator()

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			Coordinator:      coord,
			ExecutionSession: execSession,
			GroupTracker:     groupTracker,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.checkAndAdvanceGroup()

		// Should have called HandlePartialGroupFailure
		coord.mu.Lock()
		if len(coord.partialFailureCalls) != 1 || coord.partialFailureCalls[0] != 0 {
			t.Errorf("partialFailureCalls = %v, want [0]", coord.partialFailureCalls)
		}
		// Should NOT have called consolidation
		if len(coord.consolidationCalls) != 0 {
			t.Errorf("consolidationCalls = %v, want empty (partial failure)", coord.consolidationCalls)
		}
		coord.mu.Unlock()

		// Should have set local GroupDecision state
		state := exec.State()
		if state.GroupDecision == nil {
			t.Error("GroupDecision should be set after partial failure")
		} else if !state.GroupDecision.AwaitingDecision {
			t.Error("AwaitingDecision should be true")
		}
	})

	t.Run("does nothing when group not complete", func(t *testing.T) {
		groupTracker := newMockGroupTracker()
		groupTracker.groupComplete[0] = false // Not complete

		execSession := newMockExecutionSession()
		execSession.currentGroup = 0

		coord := newMockExecutionCoordinator()

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			Coordinator:      coord,
			ExecutionSession: execSession,
			GroupTracker:     groupTracker,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.checkAndAdvanceGroup()

		// Should not have called anything
		coord.mu.Lock()
		if len(coord.consolidationCalls) != 0 {
			t.Errorf("consolidationCalls = %v, want empty", coord.consolidationCalls)
		}
		if len(coord.partialFailureCalls) != 0 {
			t.Errorf("partialFailureCalls = %v, want empty", coord.partialFailureCalls)
		}
		coord.mu.Unlock()
	})
}

func TestExecutionOrchestrator_FinishExecution(t *testing.T) {
	t.Run("marks session failed when tasks failed", func(t *testing.T) {
		session := &mockSessionWithPhase{mockSession: mockSession{}}
		coord := newMockExecutionCoordinator()

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      session,
			},
			Coordinator: coord,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Set up state with failures
		exec.mu.Lock()
		exec.state.CompletedCount = 5
		exec.state.FailedCount = 2
		exec.state.TotalTasks = 7
		exec.mu.Unlock()

		exec.finishExecution()

		// Session phase should be failed
		if session.phase != PhaseFailed {
			t.Errorf("session.phase = %v, want %v", session.phase, PhaseFailed)
		}

		// Coordinator should have been notified
		coord.mu.Lock()
		if coord.sessionPhase != PhaseFailed {
			t.Errorf("coord.sessionPhase = %v, want %v", coord.sessionPhase, PhaseFailed)
		}
		if len(coord.completeCalls) != 1 {
			t.Errorf("completeCalls count = %d, want 1", len(coord.completeCalls))
		} else if coord.completeCalls[0].success {
			t.Error("completeCalls[0].success should be false")
		}
		coord.mu.Unlock()
	})

	t.Run("completes without synthesis when disabled", func(t *testing.T) {
		session := &mockSessionWithPhase{mockSession: mockSession{}}
		coord := newMockExecutionCoordinator()
		coord.noSynthesis = true

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      session,
			},
			Coordinator: coord,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// All tasks succeeded
		exec.mu.Lock()
		exec.state.CompletedCount = 5
		exec.state.FailedCount = 0
		exec.state.TotalTasks = 5
		exec.mu.Unlock()

		exec.finishExecution()

		// Session phase should be complete
		if session.phase != PhaseComplete {
			t.Errorf("session.phase = %v, want %v", session.phase, PhaseComplete)
		}

		// Coordinator should NOT have started synthesis
		coord.mu.Lock()
		if coord.synthesisCalls != 0 {
			t.Errorf("synthesisCalls = %d, want 0 (synthesis disabled)", coord.synthesisCalls)
		}
		if len(coord.completeCalls) != 1 || !coord.completeCalls[0].success {
			t.Errorf("Should have called NotifyComplete with success=true")
		}
		coord.mu.Unlock()
	})

	t.Run("starts synthesis when enabled", func(t *testing.T) {
		session := &mockSessionWithPhase{mockSession: mockSession{}}
		coord := newMockExecutionCoordinator()
		coord.noSynthesis = false

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      session,
			},
			Coordinator: coord,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// All tasks succeeded
		exec.mu.Lock()
		exec.state.CompletedCount = 5
		exec.state.FailedCount = 0
		exec.state.TotalTasks = 5
		exec.mu.Unlock()

		exec.finishExecution()

		// Should have started synthesis
		coord.mu.Lock()
		if coord.synthesisCalls != 1 {
			t.Errorf("synthesisCalls = %d, want 1", coord.synthesisCalls)
		}
		coord.mu.Unlock()
	})
}

func TestGroupDecisionState(t *testing.T) {
	t.Run("fields are accessible", func(t *testing.T) {
		gds := GroupDecisionState{
			GroupIndex:       2,
			SucceededTasks:   []string{"task-1", "task-2"},
			FailedTasks:      []string{"task-3"},
			AwaitingDecision: true,
		}

		if gds.GroupIndex != 2 {
			t.Errorf("GroupIndex = %d, want 2", gds.GroupIndex)
		}
		if len(gds.SucceededTasks) != 2 {
			t.Errorf("SucceededTasks len = %d, want 2", len(gds.SucceededTasks))
		}
		if len(gds.FailedTasks) != 1 {
			t.Errorf("FailedTasks len = %d, want 1", len(gds.FailedTasks))
		}
		if !gds.AwaitingDecision {
			t.Error("AwaitingDecision should be true")
		}
	})
}

func TestGroupTaskInfo(t *testing.T) {
	t.Run("fields are accessible", func(t *testing.T) {
		gti := GroupTaskInfo{
			ID:    "task-1",
			Title: "Test Task",
		}

		if gti.ID != "task-1" {
			t.Errorf("ID = %q, want %q", gti.ID, "task-1")
		}
		if gti.Title != "Test Task" {
			t.Errorf("Title = %q, want %q", gti.Title, "Test Task")
		}
	})
}

func TestExecutionState_ProcessedTasks(t *testing.T) {
	t.Run("ProcessedTasks map is initialized", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		state := exec.State()
		if state.ProcessedTasks == nil {
			t.Error("ProcessedTasks should not be nil")
		}
	})

	t.Run("Reset clears ProcessedTasks", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Add some processed tasks
		exec.mu.Lock()
		exec.state.ProcessedTasks["task-1"] = true
		exec.state.ProcessedTasks["task-2"] = true
		exec.mu.Unlock()

		exec.Reset()

		state := exec.State()
		if len(state.ProcessedTasks) != 0 {
			t.Errorf("ProcessedTasks len = %d, want 0 after Reset", len(state.ProcessedTasks))
		}
	})
}

// =============================================================================
// Retry, Recovery, and Partial Failure Handling Tests
// =============================================================================

func TestExecutionOrchestrator_HasPartialGroupFailure(t *testing.T) {
	tests := []struct {
		name       string
		groupIndex int
		hasPartial bool
	}{
		{
			name:       "no partial failure",
			groupIndex: 0,
			hasPartial: false,
		},
		{
			name:       "has partial failure",
			groupIndex: 1,
			hasPartial: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groupTracker := &mockGroupTracker{
				partialFailure: map[int]bool{tt.groupIndex: tt.hasPartial},
			}

			exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
				PhaseContext: &PhaseContext{
					Manager:      &mockManager{},
					Orchestrator: &mockOrchestrator{},
					Session:      &mockSession{},
				},
				GroupTracker: groupTracker,
			})
			if err != nil {
				t.Fatalf("failed to create orchestrator: %v", err)
			}

			got := exec.HasPartialGroupFailure(tt.groupIndex)
			if got != tt.hasPartial {
				t.Errorf("HasPartialGroupFailure(%d) = %v, want %v", tt.groupIndex, got, tt.hasPartial)
			}
		})
	}

	t.Run("returns false without GroupTracker", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		if exec.HasPartialGroupFailure(0) {
			t.Error("HasPartialGroupFailure should return false without GroupTracker")
		}
	})
}

func TestExecutionOrchestrator_GetGroupDecision(t *testing.T) {
	t.Run("returns nil when no decision pending", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		if exec.GetGroupDecision() != nil {
			t.Error("GetGroupDecision should return nil when no decision is pending")
		}
	})

	t.Run("returns copy of decision state", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Set up a decision state
		exec.mu.Lock()
		exec.state.GroupDecision = &GroupDecisionState{
			GroupIndex:       2,
			SucceededTasks:   []string{"task-1", "task-2"},
			FailedTasks:      []string{"task-3"},
			AwaitingDecision: true,
		}
		exec.mu.Unlock()

		decision := exec.GetGroupDecision()
		if decision == nil {
			t.Fatal("GetGroupDecision returned nil, expected decision state")
		}

		if decision.GroupIndex != 2 {
			t.Errorf("GroupIndex = %d, want 2", decision.GroupIndex)
		}
		if len(decision.SucceededTasks) != 2 {
			t.Errorf("SucceededTasks len = %d, want 2", len(decision.SucceededTasks))
		}
		if len(decision.FailedTasks) != 1 {
			t.Errorf("FailedTasks len = %d, want 1", len(decision.FailedTasks))
		}
		if !decision.AwaitingDecision {
			t.Error("AwaitingDecision should be true")
		}

		// Verify it's a copy (modifying returned slice doesn't affect internal state)
		decision.SucceededTasks[0] = "modified"
		exec.mu.RLock()
		if exec.state.GroupDecision.SucceededTasks[0] == "modified" {
			t.Error("GetGroupDecision should return a copy, not the original slice")
		}
		exec.mu.RUnlock()
	})
}

func TestExecutionOrchestrator_IsAwaitingDecision(t *testing.T) {
	t.Run("returns false when no decision", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		if exec.IsAwaitingDecision() {
			t.Error("IsAwaitingDecision should return false when no decision state")
		}
	})

	t.Run("returns true when awaiting", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.mu.Lock()
		exec.state.GroupDecision = &GroupDecisionState{
			GroupIndex:       0,
			AwaitingDecision: true,
		}
		exec.mu.Unlock()

		if !exec.IsAwaitingDecision() {
			t.Error("IsAwaitingDecision should return true when awaiting decision")
		}
	})

	t.Run("returns false when decision resolved", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.mu.Lock()
		exec.state.GroupDecision = &GroupDecisionState{
			GroupIndex:       0,
			AwaitingDecision: false, // Already resolved
		}
		exec.mu.Unlock()

		if exec.IsAwaitingDecision() {
			t.Error("IsAwaitingDecision should return false when decision already resolved")
		}
	})
}

func TestExecutionOrchestrator_ResumeWithPartialWork(t *testing.T) {
	t.Run("returns error when no decision pending", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		if err := exec.ResumeWithPartialWork(); err == nil {
			t.Error("ResumeWithPartialWork should return error when no decision pending")
		}
	})

	t.Run("resumes successfully with coordinator", func(t *testing.T) {
		coord := newMockExecutionCoordinator()
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

		// Set up decision state
		exec.mu.Lock()
		exec.state.GroupDecision = &GroupDecisionState{
			GroupIndex:       1,
			SucceededTasks:   []string{"task-1", "task-2"},
			FailedTasks:      []string{"task-3"},
			AwaitingDecision: true,
		}
		exec.mu.Unlock()

		if err := exec.ResumeWithPartialWork(); err != nil {
			t.Errorf("ResumeWithPartialWork() error = %v", err)
		}

		// Verify decision state cleared
		if exec.IsAwaitingDecision() {
			t.Error("Decision should be cleared after resume")
		}

		// Verify coordinator was called
		coord.mu.Lock()
		if len(coord.consolidationCalls) == 0 {
			t.Error("ConsolidateGroupWithVerification should have been called")
		}
		if coord.saveCalls == 0 {
			t.Error("SaveSession should have been called")
		}
		coord.mu.Unlock()
	})

	t.Run("returns error on consolidation failure", func(t *testing.T) {
		coord := newMockExecutionCoordinator()
		coord.consolidationErr = context.DeadlineExceeded

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

		exec.mu.Lock()
		exec.state.GroupDecision = &GroupDecisionState{
			GroupIndex:       0,
			SucceededTasks:   []string{"task-1"},
			FailedTasks:      []string{"task-2"},
			AwaitingDecision: true,
		}
		exec.mu.Unlock()

		if err := exec.ResumeWithPartialWork(); err == nil {
			t.Error("ResumeWithPartialWork should return error on consolidation failure")
		}
	})
}

func TestExecutionOrchestrator_RetryFailedTasks(t *testing.T) {
	t.Run("returns error when no decision pending", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		if err := exec.RetryFailedTasks(); err == nil {
			t.Error("RetryFailedTasks should return error when no decision pending")
		}
	})

	t.Run("returns error without coordinator", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.mu.Lock()
		exec.state.GroupDecision = &GroupDecisionState{
			GroupIndex:       0,
			FailedTasks:      []string{"task-1"},
			AwaitingDecision: true,
		}
		exec.mu.Unlock()

		if err := exec.RetryFailedTasks(); err == nil {
			t.Error("RetryFailedTasks should return error without coordinator")
		}
	})

	t.Run("retries successfully with coordinator", func(t *testing.T) {
		coord := newMockExecutionCoordinator()
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

		// Set up decision state with processed tasks
		exec.mu.Lock()
		exec.state.GroupDecision = &GroupDecisionState{
			GroupIndex:       0,
			SucceededTasks:   []string{"task-1"},
			FailedTasks:      []string{"task-2", "task-3"},
			AwaitingDecision: true,
		}
		exec.state.ProcessedTasks["task-2"] = true
		exec.state.ProcessedTasks["task-3"] = true
		exec.state.FailedCount = 2
		exec.mu.Unlock()

		if err := exec.RetryFailedTasks(); err != nil {
			t.Errorf("RetryFailedTasks() error = %v", err)
		}

		// Verify decision state cleared
		if exec.IsAwaitingDecision() {
			t.Error("Decision should be cleared after retry")
		}

		// Verify local state updated
		exec.mu.RLock()
		if exec.state.ProcessedTasks["task-2"] {
			t.Error("task-2 should be removed from ProcessedTasks")
		}
		if exec.state.ProcessedTasks["task-3"] {
			t.Error("task-3 should be removed from ProcessedTasks")
		}
		if exec.state.FailedCount != 0 {
			t.Errorf("FailedCount = %d, want 0", exec.state.FailedCount)
		}
		exec.mu.RUnlock()

		// Verify coordinator was called
		coord.mu.Lock()
		if len(coord.clearTaskCalls) != 2 {
			t.Errorf("ClearTaskFromInstance called %d times, want 2", len(coord.clearTaskCalls))
		}
		if coord.saveCalls == 0 {
			t.Error("SaveSession should have been called")
		}
		coord.mu.Unlock()
	})
}

// mockSessionWithExecutionOrder extends mockSession with execution order support.
type mockSessionWithExecutionOrder struct {
	mockSession
	executionOrder [][]string
}

func (m *mockSessionWithExecutionOrder) GetExecutionOrder() [][]string {
	return m.executionOrder
}

// mockManagerWithPhaseTracking extends mockManager to track SetPhase calls.
type mockManagerWithPhaseTracking struct {
	mockManager
	currentPhase UltraPlanPhase
}

func (m *mockManagerWithPhaseTracking) SetPhase(phase UltraPlanPhase) {
	m.currentPhase = phase
}

func TestExecutionOrchestrator_RetriggerGroup(t *testing.T) {
	t.Run("returns error without coordinator", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		if err := exec.RetriggerGroup(0); err == nil {
			t.Error("RetriggerGroup should return error without coordinator")
		}
	})

	t.Run("returns error with invalid group index", func(t *testing.T) {
		session := &mockSessionWithExecutionOrder{
			executionOrder: [][]string{
				{"task-1"},
				{"task-2"},
			},
		}
		coord := newMockExecutionCoordinator()

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      session,
			},
			Coordinator: coord,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Test negative index
		if err := exec.RetriggerGroup(-1); err == nil {
			t.Error("RetriggerGroup should return error for negative group index")
		}

		// Test index too high
		if err := exec.RetriggerGroup(5); err == nil {
			t.Error("RetriggerGroup should return error for group index >= numGroups")
		}
	})

	t.Run("returns error when tasks are running", func(t *testing.T) {
		session := &mockSessionWithExecutionOrder{
			executionOrder: [][]string{{"task-1"}, {"task-2"}},
		}
		coord := newMockExecutionCoordinator()

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      session,
			},
			Coordinator: coord,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Set running tasks
		exec.mu.Lock()
		exec.state.RunningCount = 2
		exec.mu.Unlock()

		if err := exec.RetriggerGroup(0); err == nil {
			t.Error("RetriggerGroup should return error when tasks are running")
		}
	})

	t.Run("returns error when awaiting decision", func(t *testing.T) {
		session := &mockSessionWithExecutionOrder{
			executionOrder: [][]string{{"task-1"}, {"task-2"}},
		}
		coord := newMockExecutionCoordinator()

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      session,
			},
			Coordinator: coord,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.mu.Lock()
		exec.state.GroupDecision = &GroupDecisionState{
			GroupIndex:       0,
			AwaitingDecision: true,
		}
		exec.mu.Unlock()

		if err := exec.RetriggerGroup(0); err == nil {
			t.Error("RetriggerGroup should return error when awaiting decision")
		}
	})

	t.Run("retriggers successfully", func(t *testing.T) {
		session := &mockSessionWithExecutionOrder{
			executionOrder: [][]string{
				{"task-1", "task-2"},
				{"task-3"},
				{"task-4"},
			},
		}
		coord := newMockExecutionCoordinator()
		mgr := &mockManagerWithPhaseTracking{}

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      mgr,
				Orchestrator: &mockOrchestrator{},
				Session:      session,
			},
			Coordinator: coord,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Set up some state that should be cleared
		exec.mu.Lock()
		exec.state.ProcessedTasks["task-3"] = true
		exec.state.ProcessedTasks["task-4"] = true
		exec.state.CompletedCount = 2
		exec.state.FailedCount = 1
		exec.mu.Unlock()

		// Retrigger from group 1 (should clear groups 1 and 2)
		if err := exec.RetriggerGroup(1); err != nil {
			t.Errorf("RetriggerGroup(1) error = %v", err)
		}

		// Verify local state cleared
		exec.mu.RLock()
		if exec.state.ProcessedTasks["task-3"] {
			t.Error("task-3 should be removed from ProcessedTasks")
		}
		if exec.state.ProcessedTasks["task-4"] {
			t.Error("task-4 should be removed from ProcessedTasks")
		}
		if exec.state.GroupDecision != nil {
			t.Error("GroupDecision should be nil after retrigger")
		}
		exec.mu.RUnlock()

		// Verify phase set to executing
		if mgr.currentPhase != PhaseExecuting {
			t.Errorf("Phase = %v, want %v", mgr.currentPhase, PhaseExecuting)
		}

		// Verify coordinator calls
		coord.mu.Lock()
		if coord.saveCalls == 0 {
			t.Error("SaveSession should have been called")
		}
		coord.mu.Unlock()
	})
}

// =============================================================================
// Additional Tests for Task Spawning, Monitoring, and Notification
// =============================================================================

// mockOrchestratorForStartTask provides control over AddInstance and StartInstance behavior.
type mockOrchestratorForStartTask struct {
	mockOrchestrator
	addInstanceErr   error
	startInstanceErr error
	addedInstances   []struct {
		session any
		prompt  string
	}
	startedInstances []any
	instances        map[string]InstanceInterface
	mu               sync.Mutex
}

func newMockOrchestratorForStartTask() *mockOrchestratorForStartTask {
	return &mockOrchestratorForStartTask{
		instances: make(map[string]InstanceInterface),
	}
}

func (m *mockOrchestratorForStartTask) AddInstance(session any, prompt string) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addedInstances = append(m.addedInstances, struct {
		session any
		prompt  string
	}{session, prompt})
	if m.addInstanceErr != nil {
		return nil, m.addInstanceErr
	}
	inst := &mockInstance{id: fmt.Sprintf("inst-%d", len(m.addedInstances)), worktreePath: "/tmp/worktree"}
	m.instances[inst.id] = inst
	return inst, nil
}

func (m *mockOrchestratorForStartTask) StartInstance(inst any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startedInstances = append(m.startedInstances, inst)
	return m.startInstanceErr
}

func (m *mockOrchestratorForStartTask) GetInstance(id string) InstanceInterface {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.instances[id]
}

// mockExecutionOrchestratorForSpawn extends mockExecutionOrchestrator with more control.
type mockExecutionOrchestratorForSpawn struct {
	mockExecutionOrchestrator
	addFromBranchErr error
	stopCalls        []string
	addedFromBranch  []struct {
		session    any
		task       string
		baseBranch string
	}
	mu sync.Mutex
}

func newMockExecutionOrchestratorForSpawn() *mockExecutionOrchestratorForSpawn {
	return &mockExecutionOrchestratorForSpawn{
		mockExecutionOrchestrator: mockExecutionOrchestrator{
			instances: make(map[string]any),
		},
	}
}

func (m *mockExecutionOrchestratorForSpawn) AddInstanceFromBranch(session any, task string, baseBranch string) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addedFromBranch = append(m.addedFromBranch, struct {
		session    any
		task       string
		baseBranch string
	}{session, task, baseBranch})
	if m.addFromBranchErr != nil {
		return nil, m.addFromBranchErr
	}
	inst := &mockInstance{id: fmt.Sprintf("inst-branch-%d", len(m.addedFromBranch)), worktreePath: "/tmp/worktree-branch"}
	m.instances[inst.id] = inst
	return inst, nil
}

func (m *mockExecutionOrchestratorForSpawn) StopInstance(inst any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mi, ok := inst.(*mockInstance); ok {
		m.stopCalls = append(m.stopCalls, mi.id)
	}
	return nil
}

// mockSessionWithReadyTasks extends mockSession to return ready tasks.
type mockSessionWithReadyTasks struct {
	mockSession
	readyTasks []string
}

func (m *mockSessionWithReadyTasks) GetReadyTasks() []string {
	return m.readyTasks
}

func TestExecutionOrchestrator_StartTask(t *testing.T) {
	t.Run("successfully starts a task with default orchestrator", func(t *testing.T) {
		task := &mockPlannedTask{
			id:          "task-1",
			title:       "Test Task",
			description: "Test description",
			files:       []string{"file.go"},
		}
		session := &mockSessionWithReadyTasks{
			mockSession: mockSession{
				tasks: map[string]any{"task-1": task},
			},
			readyTasks: []string{"task-1"},
		}
		orch := newMockOrchestratorForStartTask()
		mgr := &mockManager{}
		cb := &trackingCallbacks{}

		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      mgr,
			Orchestrator: orch,
			Session:      session,
			Callbacks:    cb,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Set up context for monitor goroutine (startTask spawns a goroutine that reads e.ctx)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		exec.mu.Lock()
		exec.ctx = ctx
		exec.mu.Unlock()

		err = exec.startTask("task-1")
		if err != nil {
			t.Errorf("startTask() error = %v", err)
		}

		// Cancel context to stop monitor goroutine
		cancel()
		exec.wg.Wait()

		// Verify instance was added
		orch.mu.Lock()
		if len(orch.addedInstances) != 1 {
			t.Errorf("addedInstances count = %d, want 1", len(orch.addedInstances))
		}
		// Verify instance was started
		if len(orch.startedInstances) != 1 {
			t.Errorf("startedInstances count = %d, want 1", len(orch.startedInstances))
		}
		orch.mu.Unlock()

		// Verify task is tracked as running
		state := exec.State()
		if state.RunningCount != 1 {
			t.Errorf("RunningCount = %d, want 1", state.RunningCount)
		}
		if _, ok := state.RunningTasks["task-1"]; !ok {
			t.Error("task-1 should be in RunningTasks")
		}
	})

	t.Run("returns error when task not found", func(t *testing.T) {
		session := &mockSession{
			tasks: map[string]any{},
		}
		orch := newMockOrchestratorForStartTask()

		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: orch,
			Session:      session,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		err = exec.startTask("nonexistent")
		if err == nil {
			t.Error("startTask() should return error for nonexistent task")
		}
	})

	t.Run("returns error when AddInstance fails", func(t *testing.T) {
		task := &mockPlannedTask{
			id:    "task-1",
			title: "Test Task",
		}
		session := &mockSession{
			tasks: map[string]any{"task-1": task},
		}
		orch := newMockOrchestratorForStartTask()
		orch.addInstanceErr = fmt.Errorf("failed to add instance")

		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: orch,
			Session:      session,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		err = exec.startTask("task-1")
		if err == nil {
			t.Error("startTask() should return error when AddInstance fails")
		}
	})

	t.Run("returns error when StartInstance fails and cleans up state", func(t *testing.T) {
		task := &mockPlannedTask{
			id:    "task-1",
			title: "Test Task",
		}
		session := &mockSession{
			tasks: map[string]any{"task-1": task},
		}
		orch := newMockOrchestratorForStartTask()
		orch.startInstanceErr = fmt.Errorf("failed to start instance")

		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: orch,
			Session:      session,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		err = exec.startTask("task-1")
		if err == nil {
			t.Error("startTask() should return error when StartInstance fails")
		}

		// Verify state was cleaned up
		state := exec.State()
		if state.RunningCount != 0 {
			t.Errorf("RunningCount = %d, want 0 after failure cleanup", state.RunningCount)
		}
	})

	t.Run("uses base branch when coordinator provides one", func(t *testing.T) {
		task := &mockPlannedTask{
			id:    "task-1",
			title: "Test Task",
		}
		session := &mockSessionWithReadyTasks{
			mockSession: mockSession{
				tasks: map[string]any{"task-1": task},
			},
		}
		execSession := newMockExecutionSession()
		execSession.currentGroup = 1

		coord := newMockExecutionCoordinator()
		coord.baseBranches[1] = "consolidated-group-0"

		execOrch := newMockExecutionOrchestratorForSpawn()

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: newMockOrchestratorForStartTask(),
				Session:      session,
			},
			Coordinator:           coord,
			ExecutionSession:      execSession,
			ExecutionOrchestrator: execOrch,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Set up context for monitor goroutine
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		exec.mu.Lock()
		exec.ctx = ctx
		exec.mu.Unlock()

		err = exec.startTask("task-1")
		if err != nil {
			t.Errorf("startTask() error = %v", err)
		}

		// Cancel context to stop monitor goroutine
		cancel()
		exec.wg.Wait()

		// Verify AddInstanceFromBranch was called with the base branch
		execOrch.mu.Lock()
		if len(execOrch.addedFromBranch) != 1 {
			t.Errorf("addedFromBranch count = %d, want 1", len(execOrch.addedFromBranch))
		} else if execOrch.addedFromBranch[0].baseBranch != "consolidated-group-0" {
			t.Errorf("baseBranch = %q, want %q", execOrch.addedFromBranch[0].baseBranch, "consolidated-group-0")
		}
		execOrch.mu.Unlock()

		// Verify coordinator was notified
		coord.mu.Lock()
		if len(coord.taskStartCalls) != 1 {
			t.Errorf("taskStartCalls count = %d, want 1", len(coord.taskStartCalls))
		}
		coord.mu.Unlock()
	})
}

func TestExecutionOrchestrator_IsTaskRunning(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("returns false when task not running", func(t *testing.T) {
		if exec.isTaskRunning("nonexistent") {
			t.Error("isTaskRunning should return false for non-running task")
		}
	})

	t.Run("returns true when task is running", func(t *testing.T) {
		exec.mu.Lock()
		exec.state.RunningTasks["task-1"] = "inst-1"
		exec.mu.Unlock()

		if !exec.isTaskRunning("task-1") {
			t.Error("isTaskRunning should return true for running task")
		}
	})
}

func TestExecutionOrchestrator_GetInstanceID(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("extracts ID via GetID() method", func(t *testing.T) {
		inst := &mockInstance{id: "test-id-123"}
		result := exec.getInstanceID(inst)
		if result != "test-id-123" {
			t.Errorf("getInstanceID() = %q, want %q", result, "test-id-123")
		}
	})

	t.Run("extracts ID via ID() method", func(t *testing.T) {
		inst := &instanceWithIDMethod{id: "id-method-456"}
		result := exec.getInstanceID(inst)
		if result != "id-method-456" {
			t.Errorf("getInstanceID() = %q, want %q", result, "id-method-456")
		}
	})

	t.Run("falls back to string conversion", func(t *testing.T) {
		inst := "some-string-value"
		result := exec.getInstanceID(inst)
		if result != "some-string-value" {
			t.Errorf("getInstanceID() = %q, want %q", result, "some-string-value")
		}
	})
}

// instanceWithIDMethod implements ID() instead of GetID().
type instanceWithIDMethod struct {
	id string
}

func (i *instanceWithIDMethod) ID() string { return i.id }

func TestExecutionOrchestrator_NotifyTaskStart(t *testing.T) {
	t.Run("notifies via coordinator", func(t *testing.T) {
		mgr := &mockManager{}
		coord := newMockExecutionCoordinator()

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      mgr,
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			Coordinator: coord,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.notifyTaskStart("task-1", "inst-1")

		coord.mu.Lock()
		if len(coord.taskStartCalls) != 1 {
			t.Errorf("taskStartCalls count = %d, want 1", len(coord.taskStartCalls))
		} else {
			if coord.taskStartCalls[0].taskID != "task-1" || coord.taskStartCalls[0].instanceID != "inst-1" {
				t.Errorf("taskStartCalls[0] = {%q, %q}, want {%q, %q}",
					coord.taskStartCalls[0].taskID, coord.taskStartCalls[0].instanceID, "task-1", "inst-1")
			}
		}
		coord.mu.Unlock()
	})

	t.Run("notifies via callbacks when no coordinator", func(t *testing.T) {
		cb := &trackingCallbacks{}

		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
			Callbacks:    cb,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.notifyTaskStart("task-2", "inst-2")

		// trackingCallbacks doesn't track OnTaskStart, so we just verify no panic
	})
}

func TestExecutionOrchestrator_NotifyTaskFailed(t *testing.T) {
	t.Run("notifies via coordinator and updates state", func(t *testing.T) {
		coord := newMockExecutionCoordinator()
		mgr := &mockManager{}

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      mgr,
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			Coordinator: coord,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.notifyTaskFailed("task-1", "some error")

		// Verify state updated
		state := exec.State()
		if state.FailedCount != 1 {
			t.Errorf("FailedCount = %d, want 1", state.FailedCount)
		}

		// Verify coordinator notified
		coord.mu.Lock()
		if len(coord.taskFailedCalls) != 1 {
			t.Errorf("taskFailedCalls count = %d, want 1", len(coord.taskFailedCalls))
		} else {
			if coord.taskFailedCalls[0].taskID != "task-1" || coord.taskFailedCalls[0].reason != "some error" {
				t.Errorf("taskFailedCalls[0] = {%q, %q}, want {%q, %q}",
					coord.taskFailedCalls[0].taskID, coord.taskFailedCalls[0].reason, "task-1", "some error")
			}
		}
		coord.mu.Unlock()
	})

	t.Run("notifies via callbacks when no coordinator", func(t *testing.T) {
		cb := &trackingCallbacks{}

		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
			Callbacks:    cb,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.notifyTaskFailed("task-1", "error")

		if len(cb.taskFailedCalls) != 1 {
			t.Errorf("taskFailedCalls count = %d, want 1", len(cb.taskFailedCalls))
		}
	})
}

func TestExecutionOrchestrator_NotifyProgress(t *testing.T) {
	t.Run("notifies via coordinator", func(t *testing.T) {
		coord := newMockExecutionCoordinator()

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

		exec.notifyProgress()

		coord.mu.Lock()
		if coord.progressCalls != 1 {
			t.Errorf("progressCalls = %d, want 1", coord.progressCalls)
		}
		coord.mu.Unlock()
	})

	t.Run("notifies via callbacks when no coordinator", func(t *testing.T) {
		progressCalls := 0
		cb := &progressTrackingCallbacks{
			onProgress: func(completed, total int, phase UltraPlanPhase) {
				progressCalls++
			},
		}

		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
			Callbacks:    cb,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Set some state
		exec.mu.Lock()
		exec.state.CompletedCount = 3
		exec.state.TotalTasks = 10
		exec.mu.Unlock()

		exec.notifyProgress()

		if progressCalls != 1 {
			t.Errorf("progressCalls = %d, want 1", progressCalls)
		}
	})
}

// progressTrackingCallbacks allows tracking of OnProgress calls.
type progressTrackingCallbacks struct {
	mockCallbacks
	onProgress func(completed, total int, phase UltraPlanPhase)
}

func (p *progressTrackingCallbacks) OnProgress(completed, total int, phase UltraPlanPhase) {
	if p.onProgress != nil {
		p.onProgress(completed, total, phase)
	}
}

func TestExecutionOrchestrator_SetRetryRecoveryContext(t *testing.T) {
	t.Run("sets retry recovery context", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Initially no execCtx
		if exec.execCtx != nil {
			t.Error("execCtx should be nil initially")
		}

		// Set retry recovery context
		execCtx := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			Coordinator: newMockExecutionCoordinator(),
		}
		retryCtx := &RetryRecoveryContext{
			ExecutionContext: execCtx,
		}

		exec.SetRetryRecoveryContext(retryCtx)

		if exec.execCtx == nil {
			t.Error("execCtx should be set after SetRetryRecoveryContext")
		}
		if exec.execCtx.Coordinator == nil {
			t.Error("Coordinator should be set in execCtx")
		}
	})

	t.Run("handles nil context gracefully", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Set to nil - should not panic
		exec.SetRetryRecoveryContext(nil)

		if exec.execCtx != nil {
			t.Error("execCtx should remain nil when nil context is set")
		}
	})
}

func TestExecutionOrchestrator_MonitorTaskInstance(t *testing.T) {
	t.Run("reports completion when sentinel file found", func(t *testing.T) {
		verifier := newMockTaskVerifier()
		verifier.completionFileResults["/tmp/worktree"] = true
		verifier.verifyResults["task-1"] = TaskVerifyResult{
			TaskID:      "task-1",
			InstanceID:  "inst-1",
			Success:     true,
			CommitCount: 2,
		}

		execOrch := newMockExecutionOrchestratorForSpawn()
		inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"}
		execOrch.instances["inst-1"] = inst

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			ExecutionOrchestrator: execOrch,
			Verifier:              verifier,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Create context for the monitor
		ctx, cancel := context.WithCancel(context.Background())
		exec.ctx = ctx

		// Start monitoring in a goroutine
		exec.wg.Add(1)
		go func() {
			defer exec.wg.Done()
			exec.monitorTaskInstance("task-1", "inst-1")
		}()

		// Wait for completion
		select {
		case completion := <-exec.completionChan:
			if completion.TaskID != "task-1" {
				t.Errorf("TaskID = %q, want %q", completion.TaskID, "task-1")
			}
			if !completion.Success {
				t.Error("Success should be true")
			}
			if completion.CommitCount != 2 {
				t.Errorf("CommitCount = %d, want 2", completion.CommitCount)
			}
		case <-time.After(3 * time.Second):
			t.Error("timeout waiting for completion")
		}

		cancel()
		exec.wg.Wait()

		// Verify instance was stopped
		execOrch.mu.Lock()
		if len(execOrch.stopCalls) != 1 {
			t.Errorf("stopCalls count = %d, want 1", len(execOrch.stopCalls))
		}
		execOrch.mu.Unlock()
	})

	t.Run("reports error when instance not found", func(t *testing.T) {
		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			ExecutionOrchestrator: newMockExecutionOrchestratorForSpawn(), // Empty instances
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		exec.ctx = ctx

		exec.wg.Add(1)
		go func() {
			defer exec.wg.Done()
			exec.monitorTaskInstance("task-1", "nonexistent-inst")
		}()

		select {
		case completion := <-exec.completionChan:
			if completion.Success {
				t.Error("Success should be false for not found instance")
			}
			if completion.Error != "instance not found" {
				t.Errorf("Error = %q, want %q", completion.Error, "instance not found")
			}
		case <-time.After(3 * time.Second):
			t.Error("timeout waiting for completion")
		}

		exec.wg.Wait()
	})

	t.Run("exits on context cancellation", func(t *testing.T) {
		execOrch := newMockExecutionOrchestratorForSpawn()
		inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"}
		execOrch.instances["inst-1"] = inst

		verifier := newMockTaskVerifier()
		// No completion file set

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			ExecutionOrchestrator: execOrch,
			Verifier:              verifier,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		exec.ctx = ctx

		done := make(chan struct{})
		exec.wg.Add(1)
		go func() {
			defer exec.wg.Done()
			exec.monitorTaskInstance("task-1", "inst-1")
			close(done)
		}()

		// Cancel after a short delay
		time.Sleep(100 * time.Millisecond)
		cancel()

		select {
		case <-done:
			// Success - monitor exited
		case <-time.After(3 * time.Second):
			t.Error("monitor did not exit after context cancellation")
		}

		exec.wg.Wait()
	})

	t.Run("handles status-based completion for StatusError", func(t *testing.T) {
		execOrch := newMockExecutionOrchestratorForSpawn()
		inst := &mockInstanceWithStatus{
			mockInstance: mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"},
			status:       StatusError,
		}
		execOrch.instances["inst-1"] = inst

		verifier := newMockTaskVerifier()
		// No completion file

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			ExecutionOrchestrator: execOrch,
			Verifier:              verifier,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		exec.ctx = ctx

		exec.wg.Add(1)
		go func() {
			defer exec.wg.Done()
			exec.monitorTaskInstance("task-1", "inst-1")
		}()

		select {
		case completion := <-exec.completionChan:
			if completion.Success {
				t.Error("Success should be false for StatusError")
			}
			if completion.Error != string(StatusError) {
				t.Errorf("Error = %q, want %q", completion.Error, string(StatusError))
			}
		case <-time.After(3 * time.Second):
			t.Error("timeout waiting for completion")
		}

		exec.wg.Wait()
	})

	t.Run("handles status-based completion for StatusTimeout", func(t *testing.T) {
		execOrch := newMockExecutionOrchestratorForSpawn()
		inst := &mockInstanceWithStatus{
			mockInstance: mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"},
			status:       StatusTimeout,
		}
		execOrch.instances["inst-1"] = inst

		verifier := newMockTaskVerifier()

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			ExecutionOrchestrator: execOrch,
			Verifier:              verifier,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		exec.ctx = ctx

		exec.wg.Add(1)
		go func() {
			defer exec.wg.Done()
			exec.monitorTaskInstance("task-1", "inst-1")
		}()

		select {
		case completion := <-exec.completionChan:
			if completion.Success {
				t.Error("Success should be false for StatusTimeout")
			}
			if completion.Error != string(StatusTimeout) {
				t.Errorf("Error = %q, want %q", completion.Error, string(StatusTimeout))
			}
		case <-time.After(3 * time.Second):
			t.Error("timeout waiting for completion")
		}

		exec.wg.Wait()
	})

	t.Run("handles status-based completion for StatusStuck", func(t *testing.T) {
		execOrch := newMockExecutionOrchestratorForSpawn()
		inst := &mockInstanceWithStatus{
			mockInstance: mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"},
			status:       StatusStuck,
		}
		execOrch.instances["inst-1"] = inst

		verifier := newMockTaskVerifier()

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			ExecutionOrchestrator: execOrch,
			Verifier:              verifier,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		exec.ctx = ctx

		exec.wg.Add(1)
		go func() {
			defer exec.wg.Done()
			exec.monitorTaskInstance("task-1", "inst-1")
		}()

		select {
		case completion := <-exec.completionChan:
			if completion.Success {
				t.Error("Success should be false for StatusStuck")
			}
			if completion.Error != string(StatusStuck) {
				t.Errorf("Error = %q, want %q", completion.Error, string(StatusStuck))
			}
		case <-time.After(3 * time.Second):
			t.Error("timeout waiting for completion")
		}

		exec.wg.Wait()
	})
}

func TestExecutionOrchestrator_VerifyTaskWorkWithNoCodeFlag(t *testing.T) {
	t.Run("passes NoCode flag to verifier", func(t *testing.T) {
		task := &mockPlannedTask{
			id:     "task-1",
			noCode: true,
		}
		session := &mockSession{
			tasks: map[string]any{"task-1": task},
		}

		var capturedOpts *TaskVerifyOptions
		verifier := &verifierCapturingOpts{
			captureOpts: func(opts *TaskVerifyOptions) {
				capturedOpts = opts
			},
		}

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      session,
			},
			Verifier: verifier,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/worktree"}
		exec.verifyTaskWork("task-1", "inst-1", inst)

		if capturedOpts == nil {
			t.Fatal("opts should have been passed to verifier")
		}
		if !capturedOpts.NoCode {
			t.Error("NoCode flag should be true")
		}
	})
}

// verifierCapturingOpts captures the TaskVerifyOptions passed to VerifyTaskWork.
type verifierCapturingOpts struct {
	captureOpts func(*TaskVerifyOptions)
}

func (v *verifierCapturingOpts) CheckCompletionFile(worktreePath string) (bool, error) {
	return false, nil
}

func (v *verifierCapturingOpts) VerifyTaskWork(taskID, instanceID, worktreePath, baseBranch string, opts *TaskVerifyOptions) TaskVerifyResult {
	if v.captureOpts != nil {
		v.captureOpts(opts)
	}
	return TaskVerifyResult{TaskID: taskID, InstanceID: instanceID, Success: true}
}

func TestExecutionOrchestrator_ExecutionLoopIntegration(t *testing.T) {
	t.Run("spawns tasks up to MaxParallel limit", func(t *testing.T) {
		task1 := &mockPlannedTask{id: "task-1", title: "Task 1"}
		task2 := &mockPlannedTask{id: "task-2", title: "Task 2"}
		task3 := &mockPlannedTask{id: "task-3", title: "Task 3"}

		session := &mockSessionWithReadyTasks{
			mockSession: mockSession{
				tasks: map[string]any{
					"task-1": task1,
					"task-2": task2,
					"task-3": task3,
				},
			},
			readyTasks: []string{"task-1", "task-2", "task-3"},
		}

		execSession := newMockExecutionSession()
		execSession.maxParallel = 2 // Limit to 2 concurrent
		execSession.totalCount = 3

		orch := newMockOrchestratorForStartTask()

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: orch,
				Session:      session,
			},
			ExecutionSession: execSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			_ = exec.Execute(ctx)
			close(done)
		}()

		// Let execution run briefly
		time.Sleep(800 * time.Millisecond)

		// Verify we didn't exceed MaxParallel
		state := exec.State()
		if state.RunningCount > 2 {
			t.Errorf("RunningCount = %d, should not exceed MaxParallel=2", state.RunningCount)
		}

		cancel()
		<-done
	})
}

func TestExecutionOrchestrator_FinishExecutionWithoutCoordinator(t *testing.T) {
	t.Run("completes via callbacks when no coordinator", func(t *testing.T) {
		session := &mockSessionWithPhase{}
		var completedSuccess bool
		var completedSummary string

		cb := &completionTrackingCallbacks{
			onComplete: func(success bool, summary string) {
				completedSuccess = success
				completedSummary = summary
			},
		}

		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      session,
			Callbacks:    cb,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// All tasks failed
		exec.mu.Lock()
		exec.state.CompletedCount = 3
		exec.state.FailedCount = 2
		exec.state.TotalTasks = 5
		exec.mu.Unlock()

		exec.finishExecution()

		if completedSuccess {
			t.Error("completedSuccess should be false when tasks failed")
		}
		if completedSummary == "" {
			t.Error("completedSummary should not be empty")
		}
	})
}

// completionTrackingCallbacks tracks OnComplete calls.
type completionTrackingCallbacks struct {
	mockCallbacks
	onComplete func(success bool, summary string)
}

func (c *completionTrackingCallbacks) OnComplete(success bool, summary string) {
	if c.onComplete != nil {
		c.onComplete(success, summary)
	}
}

func TestExecutionOrchestrator_CheckAndAdvanceGroup_ConsolidationError(t *testing.T) {
	t.Run("marks session failed when consolidation fails", func(t *testing.T) {
		groupTracker := newMockGroupTracker()
		groupTracker.groupComplete[0] = true
		groupTracker.partialFailure[0] = false
		groupTracker.nextGroupMap[0] = 1

		execSession := newMockExecutionSession()
		execSession.currentGroup = 0

		coord := newMockExecutionCoordinator()
		coord.consolidationErr = fmt.Errorf("consolidation failed")

		session := &mockSessionWithPhase{}

		exec, err := NewExecutionOrchestratorWithContext(&ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      session,
			},
			Coordinator:      coord,
			ExecutionSession: execSession,
			GroupTracker:     groupTracker,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		exec.checkAndAdvanceGroup()

		// Verify coordinator was notified of failure
		coord.mu.Lock()
		if coord.sessionPhase != PhaseFailed {
			t.Errorf("sessionPhase = %v, want %v", coord.sessionPhase, PhaseFailed)
		}
		if len(coord.completeCalls) != 1 || coord.completeCalls[0].success {
			t.Error("Should have called NotifyComplete with success=false")
		}
		coord.mu.Unlock()
	})
}

func TestExecutionOrchestrator_GetInstanceWorktreePath_AlternateInterfaces(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("extracts path via WorktreePath() method", func(t *testing.T) {
		inst := &instanceWithWorktreePathMethod{path: "/alt/path"}
		result := exec.getInstanceWorktreePath(inst)
		if result != "/alt/path" {
			t.Errorf("getInstanceWorktreePath() = %q, want %q", result, "/alt/path")
		}
	})

	t.Run("returns empty for unknown type", func(t *testing.T) {
		inst := struct{ foo string }{foo: "bar"}
		result := exec.getInstanceWorktreePath(inst)
		if result != "" {
			t.Errorf("getInstanceWorktreePath() = %q, want empty string", result)
		}
	})
}

// instanceWithWorktreePathMethod implements WorktreePath() instead of GetWorktreePath().
type instanceWithWorktreePathMethod struct {
	path string
}

func (i *instanceWithWorktreePathMethod) WorktreePath() string { return i.path }

func TestExecutionOrchestrator_GetInstanceStatus_AlternateInterfaces(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("extracts status via Status() method", func(t *testing.T) {
		inst := &instanceWithStatusMethod{status: StatusCompleted}
		result := exec.getInstanceStatus(inst)
		if result != StatusCompleted {
			t.Errorf("getInstanceStatus() = %v, want %v", result, StatusCompleted)
		}
	})

	t.Run("returns empty for unknown type", func(t *testing.T) {
		inst := struct{ foo string }{foo: "bar"}
		result := exec.getInstanceStatus(inst)
		if result != "" {
			t.Errorf("getInstanceStatus() = %v, want empty", result)
		}
	})
}

// instanceWithStatusMethod implements Status() instead of GetStatus().
type instanceWithStatusMethod struct {
	status InstanceStatus
}

func (i *instanceWithStatusMethod) Status() InstanceStatus { return i.status }

func TestExecutionOrchestrator_GetExecutionOrder(t *testing.T) {
	t.Run("returns nil when session doesn't support interface", func(t *testing.T) {
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		result := exec.getExecutionOrder()
		if result != nil {
			t.Errorf("getExecutionOrder() = %v, want nil", result)
		}
	})

	t.Run("returns execution order when session supports interface", func(t *testing.T) {
		session := &mockSessionWithExecutionOrder{
			executionOrder: [][]string{{"task-1"}, {"task-2", "task-3"}},
		}
		exec, err := NewExecutionOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      session,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		result := exec.getExecutionOrder()
		if len(result) != 2 {
			t.Errorf("getExecutionOrder() len = %d, want 2", len(result))
		}
	})
}

func TestExecutionOrchestrator_Reset_DrainsChan(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Fill the completion channel with some items
	exec.completionChan <- TaskCompletion{TaskID: "task-1"}
	exec.completionChan <- TaskCompletion{TaskID: "task-2"}

	// Reset should drain the channel
	exec.Reset()

	// Channel should be empty
	select {
	case <-exec.completionChan:
		t.Error("completionChan should be empty after Reset")
	default:
		// Expected
	}
}

func TestRetryTaskState(t *testing.T) {
	t.Run("all fields are accessible", func(t *testing.T) {
		state := RetryTaskState{
			TaskID:       "task-1",
			RetryCount:   2,
			MaxRetries:   3,
			LastError:    "some error",
			CommitCounts: []int{0, 1},
			Succeeded:    false,
		}

		if state.TaskID != "task-1" {
			t.Errorf("TaskID = %q, want %q", state.TaskID, "task-1")
		}
		if state.RetryCount != 2 {
			t.Errorf("RetryCount = %d, want 2", state.RetryCount)
		}
		if state.MaxRetries != 3 {
			t.Errorf("MaxRetries = %d, want 3", state.MaxRetries)
		}
		if state.LastError != "some error" {
			t.Errorf("LastError = %q, want %q", state.LastError, "some error")
		}
		if len(state.CommitCounts) != 2 {
			t.Errorf("CommitCounts len = %d, want 2", len(state.CommitCounts))
		}
		if state.Succeeded {
			t.Error("Succeeded should be false")
		}
	})
}

func TestRetryRecoveryContext(t *testing.T) {
	t.Run("all fields are accessible", func(t *testing.T) {
		execCtx := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
		}
		retryCtx := RetryRecoveryContext{
			ExecutionContext: execCtx,
			RetryManager:     nil,
		}

		if retryCtx.ExecutionContext == nil {
			t.Error("ExecutionContext should not be nil")
		}
		if retryCtx.RetryManager != nil {
			t.Error("RetryManager should be nil")
		}
	})
}

func TestExecutionContext_Validate(t *testing.T) {
	t.Run("validates successfully with valid PhaseContext", func(t *testing.T) {
		ctx := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
		}

		err := ctx.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})
}

func TestExecutionOrchestrator_State_CopiesGroupDecision(t *testing.T) {
	exec, err := NewExecutionOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Set up state with GroupDecision
	exec.mu.Lock()
	exec.state.GroupDecision = &GroupDecisionState{
		GroupIndex:       1,
		SucceededTasks:   []string{"task-1"},
		FailedTasks:      []string{"task-2"},
		AwaitingDecision: true,
	}
	exec.mu.Unlock()

	state := exec.State()

	// Verify the copy includes GroupDecision
	if state.GroupDecision == nil {
		t.Fatal("GroupDecision should be copied")
	}
	if state.GroupDecision.GroupIndex != 1 {
		t.Errorf("GroupDecision.GroupIndex = %d, want 1", state.GroupDecision.GroupIndex)
	}

	// Verify it's a deep copy (modifying doesn't affect original)
	state.GroupDecision.SucceededTasks[0] = "modified"

	exec.mu.RLock()
	if exec.state.GroupDecision.SucceededTasks[0] == "modified" {
		t.Error("State() should return a deep copy of GroupDecision")
	}
	exec.mu.RUnlock()
}

// Compile-time interface checks
func init() {
	// Ensure mockGroupTracker implements GroupTrackerInterface
	var _ GroupTrackerInterface = (*mockGroupTracker)(nil)
}
