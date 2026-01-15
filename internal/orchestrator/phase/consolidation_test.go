package phase

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockManagerForConsolidation implements UltraPlanManagerInterface for consolidation tests
type mockManagerForConsolidation struct {
	session       UltraPlanSessionInterface
	currentPhase  UltraPlanPhase
	phaseChanges  []UltraPlanPhase
	setPhaseCalls int
}

func (m *mockManagerForConsolidation) Session() UltraPlanSessionInterface {
	return m.session
}

func (m *mockManagerForConsolidation) SetPhase(phase UltraPlanPhase) {
	m.currentPhase = phase
	m.phaseChanges = append(m.phaseChanges, phase)
	m.setPhaseCalls++
}

func (m *mockManagerForConsolidation) SetPlan(plan any)                            {}
func (m *mockManagerForConsolidation) MarkTaskComplete(taskID string)              {}
func (m *mockManagerForConsolidation) MarkTaskFailed(taskID string, reason string) {}
func (m *mockManagerForConsolidation) AssignTaskToInstance(taskID, instanceID string) {
}
func (m *mockManagerForConsolidation) Stop() {}

// mockOrchestratorForConsolidation implements OrchestratorInterface for consolidation tests
type mockOrchestratorForConsolidation struct {
	branchPrefix string
	instance     InstanceInterface
}

func (m *mockOrchestratorForConsolidation) AddInstance(session any, task string) (any, error) {
	return nil, nil
}
func (m *mockOrchestratorForConsolidation) StartInstance(inst any) error { return nil }
func (m *mockOrchestratorForConsolidation) SaveSession() error           { return nil }
func (m *mockOrchestratorForConsolidation) GetInstanceManager(id string) any {
	return nil
}
func (m *mockOrchestratorForConsolidation) GetInstance(id string) InstanceInterface {
	return m.instance
}
func (m *mockOrchestratorForConsolidation) BranchPrefix() string { return m.branchPrefix }

// mockSessionForConsolidation implements UltraPlanSessionInterface for consolidation tests
type mockSessionForConsolidation struct {
	tasks             map[string]any
	readyTasks        []string
	groupComplete     bool
	advancedGroup     bool
	hasMoreGroups     bool
	progress          float64
	previousGroup     int
	objective         string
	completedTasks    []string
	taskToInstance    map[string]string
	taskCommitCounts  map[string]int
	synthesisID       string
	revisionRound     int
	awaitingApproval  bool
	phase             UltraPlanPhase
	errorMsg          string
	config            UltraPlanConfigInterface
	synthesisComplete *SynthesisCompletionFile
}

func (m *mockSessionForConsolidation) GetTask(taskID string) any {
	return m.tasks[taskID]
}
func (m *mockSessionForConsolidation) GetReadyTasks() []string { return m.readyTasks }
func (m *mockSessionForConsolidation) IsCurrentGroupComplete() bool {
	return m.groupComplete
}
func (m *mockSessionForConsolidation) AdvanceGroupIfComplete() (advanced bool, previousGroup int) {
	if m.groupComplete {
		m.advancedGroup = true
		return true, m.previousGroup
	}
	return false, 0
}
func (m *mockSessionForConsolidation) HasMoreGroups() bool                  { return m.hasMoreGroups }
func (m *mockSessionForConsolidation) Progress() float64                    { return m.progress }
func (m *mockSessionForConsolidation) GetObjective() string                 { return m.objective }
func (m *mockSessionForConsolidation) GetCompletedTasks() []string          { return m.completedTasks }
func (m *mockSessionForConsolidation) GetTaskToInstance() map[string]string { return m.taskToInstance }
func (m *mockSessionForConsolidation) GetTaskCommitCounts() map[string]int  { return m.taskCommitCounts }
func (m *mockSessionForConsolidation) GetSynthesisID() string               { return m.synthesisID }
func (m *mockSessionForConsolidation) SetSynthesisID(id string)             { m.synthesisID = id }
func (m *mockSessionForConsolidation) GetRevisionRound() int                { return m.revisionRound }
func (m *mockSessionForConsolidation) SetSynthesisAwaitingApproval(awaiting bool) {
	m.awaitingApproval = awaiting
}
func (m *mockSessionForConsolidation) IsSynthesisAwaitingApproval() bool { return m.awaitingApproval }
func (m *mockSessionForConsolidation) SetSynthesisCompletion(completion *SynthesisCompletionFile) {
	m.synthesisComplete = completion
}
func (m *mockSessionForConsolidation) GetPhase() UltraPlanPhase            { return m.phase }
func (m *mockSessionForConsolidation) SetPhase(phase UltraPlanPhase)       { m.phase = phase }
func (m *mockSessionForConsolidation) SetError(err string)                 { m.errorMsg = err }
func (m *mockSessionForConsolidation) GetConfig() UltraPlanConfigInterface { return m.config }

// mockCallbacksForConsolidation implements CoordinatorCallbacksInterface for consolidation tests
type mockCallbacksForConsolidation struct {
	phaseChanges []UltraPlanPhase
}

func (m *mockCallbacksForConsolidation) OnPhaseChange(phase UltraPlanPhase) {
	m.phaseChanges = append(m.phaseChanges, phase)
}
func (m *mockCallbacksForConsolidation) OnTaskStart(taskID, instanceID string) {}
func (m *mockCallbacksForConsolidation) OnTaskComplete(taskID string)          {}
func (m *mockCallbacksForConsolidation) OnTaskFailed(taskID, reason string)    {}
func (m *mockCallbacksForConsolidation) OnGroupComplete(groupIndex int)        {}
func (m *mockCallbacksForConsolidation) OnPlanReady(plan any)                  {}
func (m *mockCallbacksForConsolidation) OnProgress(completed, total int, phase UltraPlanPhase) {
}
func (m *mockCallbacksForConsolidation) OnComplete(success bool, summary string) {}

func TestNewConsolidationOrchestrator(t *testing.T) {
	tests := []struct {
		name     string
		phaseCtx *PhaseContext
	}{
		{
			name: "creates orchestrator with valid context",
			phaseCtx: &PhaseContext{
				Manager:      &mockManagerForConsolidation{},
				Orchestrator: &mockOrchestratorForConsolidation{},
				Session:      &mockSessionForConsolidation{},
			},
		},
		{
			name: "creates orchestrator with nil logger (uses NopLogger)",
			phaseCtx: &PhaseContext{
				Manager:      &mockManagerForConsolidation{},
				Orchestrator: &mockOrchestratorForConsolidation{},
				Session:      &mockSessionForConsolidation{},
				Logger:       nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orch := NewConsolidationOrchestrator(tt.phaseCtx)

			if orch == nil {
				t.Fatal("NewConsolidationOrchestrator returned nil")
			}
			if orch.phaseCtx != tt.phaseCtx {
				t.Error("phaseCtx not set correctly")
			}
			if orch.logger == nil {
				t.Error("logger should not be nil (should use NopLogger)")
			}
			if orch.state == nil {
				t.Error("state should be initialized")
			}
			if orch.ctx == nil {
				t.Error("ctx should be initialized")
			}
			if orch.cancel == nil {
				t.Error("cancel function should be initialized")
			}
			if orch.cancelled {
				t.Error("cancelled should be false initially")
			}
			if orch.running {
				t.Error("running should be false initially")
			}
		})
	}
}

func TestConsolidationOrchestrator_Phase(t *testing.T) {
	phaseCtx := &PhaseContext{
		Manager:      &mockManagerForConsolidation{},
		Orchestrator: &mockOrchestratorForConsolidation{},
		Session:      &mockSessionForConsolidation{},
	}

	orch := NewConsolidationOrchestrator(phaseCtx)
	phase := orch.Phase()

	if phase != PhaseConsolidating {
		t.Errorf("Phase() = %v, want %v", phase, PhaseConsolidating)
	}
}

func TestConsolidationOrchestrator_Execute(t *testing.T) {
	t.Run("executes successfully with valid context", func(t *testing.T) {
		manager := &mockManagerForConsolidation{
			session: &mockSessionForConsolidation{},
		}
		callbacks := &mockCallbacksForConsolidation{}
		phaseCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
			Callbacks:    callbacks,
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		err := orch.Execute(context.Background())

		if err != nil {
			t.Errorf("Execute() returned unexpected error: %v", err)
		}
		if manager.setPhaseCalls != 1 {
			t.Errorf("SetPhase called %d times, want 1", manager.setPhaseCalls)
		}
		if manager.currentPhase != PhaseConsolidating {
			t.Errorf("Phase set to %v, want %v", manager.currentPhase, PhaseConsolidating)
		}
		if len(callbacks.phaseChanges) != 1 {
			t.Errorf("OnPhaseChange called %d times, want 1", len(callbacks.phaseChanges))
		}
	})

	t.Run("returns error with invalid context (nil manager)", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      nil,
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		err := orch.Execute(context.Background())

		if err != ErrNilManager {
			t.Errorf("Execute() = %v, want %v", err, ErrNilManager)
		}
	})

	t.Run("returns ErrCancelled when already cancelled", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.Cancel()

		err := orch.Execute(context.Background())

		if err != ErrCancelled {
			t.Errorf("Execute() = %v, want %v", err, ErrCancelled)
		}
	})

	t.Run("returns context error when context is cancelled", func(t *testing.T) {
		manager := &mockManagerForConsolidation{
			session: &mockSessionForConsolidation{},
		}
		phaseCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		orch := NewConsolidationOrchestrator(phaseCtx)
		err := orch.Execute(ctx)

		if err != context.Canceled {
			t.Errorf("Execute() = %v, want %v", err, context.Canceled)
		}
	})

	t.Run("executes without callbacks", func(t *testing.T) {
		manager := &mockManagerForConsolidation{
			session: &mockSessionForConsolidation{},
		}
		phaseCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
			Callbacks:    nil, // No callbacks
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		err := orch.Execute(context.Background())

		if err != nil {
			t.Errorf("Execute() returned unexpected error: %v", err)
		}
	})
}

func TestConsolidationOrchestrator_Cancel(t *testing.T) {
	t.Run("cancels successfully", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.Cancel()

		if !orch.IsCancelled() {
			t.Error("IsCancelled() should return true after Cancel()")
		}
	})

	t.Run("cancel is idempotent", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		// Call Cancel multiple times - should not panic
		orch.Cancel()
		orch.Cancel()
		orch.Cancel()

		if !orch.IsCancelled() {
			t.Error("IsCancelled() should return true after multiple Cancel() calls")
		}
	})
}

func TestConsolidationOrchestrator_State(t *testing.T) {
	t.Run("returns copy of state", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		// Set some state
		orch.SetState(ConsolidationState{
			SubPhase:      "merging",
			CurrentGroup:  1,
			TotalGroups:   3,
			CurrentTask:   "task-1",
			GroupBranches: []string{"branch-1", "branch-2"},
			PRUrls:        []string{"https://pr-1", "https://pr-2"},
			ConflictFiles: []string{"file1.go", "file2.go"},
		})

		state := orch.State()

		// Verify values
		if state.SubPhase != "merging" {
			t.Errorf("SubPhase = %v, want %v", state.SubPhase, "merging")
		}
		if state.CurrentGroup != 1 {
			t.Errorf("CurrentGroup = %v, want %v", state.CurrentGroup, 1)
		}
		if state.TotalGroups != 3 {
			t.Errorf("TotalGroups = %v, want %v", state.TotalGroups, 3)
		}
		if len(state.GroupBranches) != 2 {
			t.Errorf("GroupBranches length = %v, want %v", len(state.GroupBranches), 2)
		}
		if len(state.PRUrls) != 2 {
			t.Errorf("PRUrls length = %v, want %v", len(state.PRUrls), 2)
		}
		if len(state.ConflictFiles) != 2 {
			t.Errorf("ConflictFiles length = %v, want %v", len(state.ConflictFiles), 2)
		}

		// Verify it's a copy (modifying returned state shouldn't affect internal state)
		state.CurrentGroup = 99
		internalState := orch.State()
		if internalState.CurrentGroup == 99 {
			t.Error("State() should return a copy, not the internal state")
		}
	})

	t.Run("returns deep copy of slices", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetState(ConsolidationState{
			GroupBranches: []string{"branch-1"},
		})

		state := orch.State()
		state.GroupBranches[0] = "modified"

		// Internal state should not be affected
		internalState := orch.State()
		if internalState.GroupBranches[0] == "modified" {
			t.Error("State() should return a deep copy of slices")
		}
	})
}

func TestConsolidationOrchestrator_SetState(t *testing.T) {
	t.Run("sets state correctly", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		newState := ConsolidationState{
			SubPhase:     "pushing",
			CurrentGroup: 2,
			TotalGroups:  5,
			Error:        "some error",
		}

		orch.SetState(newState)
		state := orch.State()

		if state.SubPhase != "pushing" {
			t.Errorf("SubPhase = %v, want %v", state.SubPhase, "pushing")
		}
		if state.CurrentGroup != 2 {
			t.Errorf("CurrentGroup = %v, want %v", state.CurrentGroup, 2)
		}
		if state.TotalGroups != 5 {
			t.Errorf("TotalGroups = %v, want %v", state.TotalGroups, 5)
		}
		if state.Error != "some error" {
			t.Errorf("Error = %v, want %v", state.Error, "some error")
		}
	})

	t.Run("makes deep copy of input slices", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		branches := []string{"branch-1", "branch-2"}
		orch.SetState(ConsolidationState{
			GroupBranches: branches,
		})

		// Modify the original slice
		branches[0] = "modified"

		// Internal state should not be affected
		state := orch.State()
		if state.GroupBranches[0] == "modified" {
			t.Error("SetState() should make a deep copy of slices")
		}
	})
}

func TestConsolidationOrchestrator_IsRunning(t *testing.T) {
	t.Run("returns false initially", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if orch.IsRunning() {
			t.Error("IsRunning() should return false initially")
		}
	})

	t.Run("returns false after Execute completes", func(t *testing.T) {
		manager := &mockManagerForConsolidation{
			session: &mockSessionForConsolidation{},
		}
		phaseCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		_ = orch.Execute(context.Background())

		if orch.IsRunning() {
			t.Error("IsRunning() should return false after Execute completes")
		}
	})
}

func TestConsolidationOrchestrator_IsCancelled(t *testing.T) {
	t.Run("returns false initially", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if orch.IsCancelled() {
			t.Error("IsCancelled() should return false initially")
		}
	})

	t.Run("returns true after Cancel", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.Cancel()

		if !orch.IsCancelled() {
			t.Error("IsCancelled() should return true after Cancel()")
		}
	})
}

func TestConsolidationOrchestrator_Implements_PhaseExecutor(t *testing.T) {
	phaseCtx := &PhaseContext{
		Manager:      &mockManagerForConsolidation{},
		Orchestrator: &mockOrchestratorForConsolidation{},
		Session:      &mockSessionForConsolidation{},
	}

	orch := NewConsolidationOrchestrator(phaseCtx)

	// Verify that ConsolidationOrchestrator implements PhaseExecutor
	var _ PhaseExecutor = orch
}

func TestConsolidationOrchestrator_ConcurrentSafety(t *testing.T) {
	t.Run("concurrent State and SetState calls", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		// Run concurrent operations
		done := make(chan bool)

		// Goroutine 1: repeatedly set state
		go func() {
			for i := 0; i < 100; i++ {
				orch.SetState(ConsolidationState{
					CurrentGroup: i,
					TotalGroups:  100,
				})
			}
			done <- true
		}()

		// Goroutine 2: repeatedly get state
		go func() {
			for i := 0; i < 100; i++ {
				_ = orch.State()
			}
			done <- true
		}()

		// Goroutine 3: check running/cancelled
		go func() {
			for i := 0; i < 100; i++ {
				_ = orch.IsRunning()
				_ = orch.IsCancelled()
			}
			done <- true
		}()

		// Wait for all goroutines with timeout
		timeout := time.After(5 * time.Second)
		for i := 0; i < 3; i++ {
			select {
			case <-done:
				// OK
			case <-timeout:
				t.Fatal("Test timed out - possible deadlock")
			}
		}
	})
}

func TestErrCancelled(t *testing.T) {
	if ErrCancelled.Error() != "consolidation phase cancelled" {
		t.Errorf("ErrCancelled message = %v, want %v", ErrCancelled.Error(), "consolidation phase cancelled")
	}
}

func TestErrConsolidationFailed(t *testing.T) {
	if ErrConsolidationFailed.Error() != "consolidation phase failed" {
		t.Errorf("ErrConsolidationFailed message = %v, want %v", ErrConsolidationFailed.Error(), "consolidation phase failed")
	}
}

func TestConsolidationOrchestrator_InstanceManagement(t *testing.T) {
	t.Run("GetInstanceID returns empty initially", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if id := orch.GetInstanceID(); id != "" {
			t.Errorf("GetInstanceID() = %v, want empty string", id)
		}
	})

	t.Run("setInstanceID and GetInstanceID work correctly", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.setInstanceID("test-instance-123")

		if id := orch.GetInstanceID(); id != "test-instance-123" {
			t.Errorf("GetInstanceID() = %v, want %v", id, "test-instance-123")
		}
	})
}

func TestConsolidationOrchestrator_CompletionTracking(t *testing.T) {
	t.Run("IsComplete returns false initially", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if orch.IsComplete() {
			t.Error("IsComplete() should return false initially")
		}
	})

	t.Run("GetStartedAt returns nil initially", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if orch.GetStartedAt() != nil {
			t.Error("GetStartedAt() should return nil initially")
		}
	})

	t.Run("GetCompletedAt returns nil initially", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if orch.GetCompletedAt() != nil {
			t.Error("GetCompletedAt() should return nil initially")
		}
	})

	t.Run("setStartedAt and GetStartedAt work correctly", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		now := time.Now()
		orch.setStartedAt(now)

		if got := orch.GetStartedAt(); got == nil {
			t.Error("GetStartedAt() should not return nil after setStartedAt")
		} else if !got.Equal(now) {
			t.Errorf("GetStartedAt() = %v, want %v", got, now)
		}
	})

	t.Run("setCompletedAt and GetCompletedAt work correctly", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		now := time.Now()
		orch.setCompletedAt(now)

		if got := orch.GetCompletedAt(); got == nil {
			t.Error("GetCompletedAt() should not return nil after setCompletedAt")
		} else if !got.Equal(now) {
			t.Errorf("GetCompletedAt() = %v, want %v", got, now)
		}
	})

	t.Run("setCompleted and IsComplete work correctly", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.setCompleted(true)

		if !orch.IsComplete() {
			t.Error("IsComplete() should return true after setCompleted(true)")
		}

		orch.setCompleted(false)
		if orch.IsComplete() {
			t.Error("IsComplete() should return false after setCompleted(false)")
		}
	})
}

func TestConsolidationOrchestrator_CompletionFile(t *testing.T) {
	t.Run("GetCompletionFile returns nil initially", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if orch.GetCompletionFile() != nil {
			t.Error("GetCompletionFile() should return nil initially")
		}
	})

	t.Run("setCompletionFile and GetCompletionFile work correctly", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		cf := &ConsolidationCompletionFile{
			Status:       "complete",
			Mode:         "stacked",
			TotalCommits: 10,
			PRsCreated: []PRInfo{
				{URL: "https://github.com/owner/repo/pull/1", Title: "PR 1", GroupIndex: 0},
			},
			GroupResults: []GroupResult{
				{GroupIndex: 0, BranchName: "branch-1", Success: true},
			},
		}
		orch.setCompletionFile(cf)

		got := orch.GetCompletionFile()
		if got == nil {
			t.Fatal("GetCompletionFile() should not return nil after setCompletionFile")
		}
		if got.Status != "complete" {
			t.Errorf("Status = %v, want %v", got.Status, "complete")
		}
		if got.TotalCommits != 10 {
			t.Errorf("TotalCommits = %v, want %v", got.TotalCommits, 10)
		}
		if len(got.PRsCreated) != 1 {
			t.Errorf("len(PRsCreated) = %v, want %v", len(got.PRsCreated), 1)
		}
	})

	t.Run("GetCompletionFile returns a copy", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		cf := &ConsolidationCompletionFile{
			Status:       "complete",
			TotalCommits: 10,
		}
		orch.setCompletionFile(cf)

		got := orch.GetCompletionFile()
		got.Status = "modified"

		// Verify internal state is unchanged
		internal := orch.GetCompletionFile()
		if internal.Status == "modified" {
			t.Error("GetCompletionFile() should return a copy, not the internal state")
		}
	})
}

func TestConsolidationOrchestrator_PRUrls(t *testing.T) {
	t.Run("GetPRUrls returns nil initially", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if urls := orch.GetPRUrls(); urls != nil {
			t.Errorf("GetPRUrls() = %v, want nil", urls)
		}
	})

	t.Run("addPRUrl and GetPRUrls work correctly", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.addPRUrl("https://github.com/owner/repo/pull/1")
		orch.addPRUrl("https://github.com/owner/repo/pull/2")

		urls := orch.GetPRUrls()
		if len(urls) != 2 {
			t.Errorf("len(GetPRUrls()) = %v, want %v", len(urls), 2)
		}
		if urls[0] != "https://github.com/owner/repo/pull/1" {
			t.Errorf("urls[0] = %v, want %v", urls[0], "https://github.com/owner/repo/pull/1")
		}
	})

	t.Run("GetPRUrls returns a copy", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.addPRUrl("https://github.com/owner/repo/pull/1")

		urls := orch.GetPRUrls()
		urls[0] = "modified"

		// Verify internal state is unchanged
		internal := orch.GetPRUrls()
		if internal[0] == "modified" {
			t.Error("GetPRUrls() should return a copy")
		}
	})
}

func TestConsolidationOrchestrator_Error(t *testing.T) {
	t.Run("GetError returns empty initially", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if err := orch.GetError(); err != "" {
			t.Errorf("GetError() = %v, want empty string", err)
		}
	})

	t.Run("setError and GetError work correctly", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.setError("something went wrong")

		if got := orch.GetError(); got != "something went wrong" {
			t.Errorf("GetError() = %v, want %v", got, "something went wrong")
		}
	})
}

// mockInstanceForMonitor implements ConsolidationInstanceInfo for testing
type mockInstanceForMonitor struct {
	id     string
	status InstanceStatus
}

func (m *mockInstanceForMonitor) GetStatus() InstanceStatus { return m.status }
func (m *mockInstanceForMonitor) GetID() string             { return m.id }

func TestConsolidationOrchestrator_MonitorInstance(t *testing.T) {
	t.Run("returns error when no instance ID set", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		err := orch.MonitorInstance(
			func(id string) ConsolidationInstanceInfo { return nil },
			100*time.Millisecond,
		)

		if err == nil {
			t.Error("MonitorInstance() should return error when no instance ID set")
		}
	})

	t.Run("returns nil when instance is nil (assumed complete)", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.setInstanceID("test-instance")

		err := orch.MonitorInstance(
			func(id string) ConsolidationInstanceInfo { return nil },
			50*time.Millisecond,
		)

		if err != nil {
			t.Errorf("MonitorInstance() = %v, want nil", err)
		}
	})

	t.Run("returns nil when instance status is completed", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.setInstanceID("test-instance")

		inst := &mockInstanceForMonitor{id: "test-instance", status: StatusCompleted}

		err := orch.MonitorInstance(
			func(id string) ConsolidationInstanceInfo { return inst },
			50*time.Millisecond,
		)

		if err != nil {
			t.Errorf("MonitorInstance() = %v, want nil", err)
		}
	})

	t.Run("returns nil when instance status is waiting_input", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.setInstanceID("test-instance")

		inst := &mockInstanceForMonitor{id: "test-instance", status: StatusWaitingInput}

		err := orch.MonitorInstance(
			func(id string) ConsolidationInstanceInfo { return inst },
			50*time.Millisecond,
		)

		if err != nil {
			t.Errorf("MonitorInstance() = %v, want nil", err)
		}
	})

	t.Run("returns error when instance status is error", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.setInstanceID("test-instance")

		inst := &mockInstanceForMonitor{id: "test-instance", status: StatusError}

		err := orch.MonitorInstance(
			func(id string) ConsolidationInstanceInfo { return inst },
			50*time.Millisecond,
		)

		if err == nil {
			t.Error("MonitorInstance() should return error for error status")
		}
		if !errors.Is(err, ErrConsolidationFailed) {
			t.Errorf("MonitorInstance() error = %v, want error wrapping ErrConsolidationFailed", err)
		}
	})

	t.Run("returns ErrCancelled when cancelled", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.setInstanceID("test-instance")

		inst := &mockInstanceForMonitor{id: "test-instance", status: StatusRunning}

		// Start monitoring in a goroutine
		done := make(chan error, 1)
		go func() {
			done <- orch.MonitorInstance(
				func(id string) ConsolidationInstanceInfo { return inst },
				50*time.Millisecond,
			)
		}()

		// Cancel after a short delay
		time.Sleep(100 * time.Millisecond)
		orch.Cancel()

		// Wait for monitoring to finish
		select {
		case err := <-done:
			if err != ErrCancelled {
				t.Errorf("MonitorInstance() = %v, want %v", err, ErrCancelled)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("MonitorInstance did not return after cancellation")
		}
	})
}

func TestConsolidationOrchestrator_FinishConsolidation(t *testing.T) {
	t.Run("updates state correctly", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		prUrls := []string{"https://pr-1", "https://pr-2"}

		orch.FinishConsolidation(prUrls)

		if !orch.IsComplete() {
			t.Error("IsComplete() should return true after FinishConsolidation")
		}
		if orch.GetCompletedAt() == nil {
			t.Error("GetCompletedAt() should not be nil after FinishConsolidation")
		}

		state := orch.State()
		if state.SubPhase != "complete" {
			t.Errorf("SubPhase = %v, want %v", state.SubPhase, "complete")
		}
		if len(state.PRUrls) != 2 {
			t.Errorf("len(PRUrls) = %v, want %v", len(state.PRUrls), 2)
		}
	})

	t.Run("handles nil prUrls", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.FinishConsolidation(nil)

		if !orch.IsComplete() {
			t.Error("IsComplete() should return true after FinishConsolidation")
		}
	})
}

func TestConsolidationOrchestrator_MarkFailed(t *testing.T) {
	t.Run("updates state correctly", func(t *testing.T) {
		callbacks := &mockCallbacksForConsolidation{}
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
			Callbacks:    callbacks,
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.MarkFailed(errors.New("test error"))

		if orch.IsComplete() {
			t.Error("IsComplete() should return false after MarkFailed")
		}

		state := orch.State()
		if state.SubPhase != "failed" {
			t.Errorf("SubPhase = %v, want %v", state.SubPhase, "failed")
		}
		if state.Error != "test error" {
			t.Errorf("Error = %v, want %v", state.Error, "test error")
		}

		// Verify callback was invoked
		if len(callbacks.phaseChanges) != 1 {
			t.Errorf("OnPhaseChange called %d times, want 1", len(callbacks.phaseChanges))
		}
		if callbacks.phaseChanges[0] != PhaseFailed {
			t.Errorf("Phase change = %v, want %v", callbacks.phaseChanges[0], PhaseFailed)
		}
	})
}

func TestConsolidationOrchestrator_UpdateProgress(t *testing.T) {
	t.Run("updates state correctly", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.UpdateProgress(2, 5, "merging")

		state := orch.State()
		if state.CurrentGroup != 2 {
			t.Errorf("CurrentGroup = %v, want %v", state.CurrentGroup, 2)
		}
		if state.TotalGroups != 5 {
			t.Errorf("TotalGroups = %v, want %v", state.TotalGroups, 5)
		}
		if state.SubPhase != "merging" {
			t.Errorf("SubPhase = %v, want %v", state.SubPhase, "merging")
		}
	})
}

func TestConsolidationOrchestrator_AddGroupBranch(t *testing.T) {
	t.Run("adds branch to state", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.AddGroupBranch("branch-1")
		orch.AddGroupBranch("branch-2")

		state := orch.State()
		if len(state.GroupBranches) != 2 {
			t.Errorf("len(GroupBranches) = %v, want %v", len(state.GroupBranches), 2)
		}
		if state.GroupBranches[0] != "branch-1" {
			t.Errorf("GroupBranches[0] = %v, want %v", state.GroupBranches[0], "branch-1")
		}
	})
}

func TestConsolidationOrchestrator_Conflict(t *testing.T) {
	t.Run("SetConflict and HasConflict work correctly", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if orch.HasConflict() {
			t.Error("HasConflict() should return false initially")
		}

		orch.SetConflict("task-1", []string{"file1.go", "file2.go"})

		if !orch.HasConflict() {
			t.Error("HasConflict() should return true after SetConflict")
		}

		state := orch.State()
		if state.SubPhase != "paused" {
			t.Errorf("SubPhase = %v, want %v", state.SubPhase, "paused")
		}
		if state.CurrentTask != "task-1" {
			t.Errorf("CurrentTask = %v, want %v", state.CurrentTask, "task-1")
		}
		if len(state.ConflictFiles) != 2 {
			t.Errorf("len(ConflictFiles) = %v, want %v", len(state.ConflictFiles), 2)
		}
	})

	t.Run("ClearConflict clears conflict state", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", []string{"file1.go"})

		orch.ClearConflict()

		state := orch.State()
		if state.CurrentTask != "" {
			t.Errorf("CurrentTask = %v, want empty string", state.CurrentTask)
		}
		if state.ConflictFiles != nil {
			t.Error("ConflictFiles should be nil after ClearConflict")
		}
	})

	t.Run("SetConflict makes a copy of files slice", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		files := []string{"file1.go", "file2.go"}
		orch.SetConflict("task-1", files)

		// Modify the original slice
		files[0] = "modified.go"

		// Internal state should not be affected
		state := orch.State()
		if state.ConflictFiles[0] == "modified.go" {
			t.Error("SetConflict() should make a copy of files slice")
		}
	})
}

func TestConsolidationOrchestrator_Reset(t *testing.T) {
	t.Run("resets all state", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		// Set various state
		orch.setInstanceID("test-instance")
		orch.setCompletionFile(&ConsolidationCompletionFile{Status: "complete"})
		orch.setStartedAt(time.Now())
		orch.setCompletedAt(time.Now())
		orch.setCompleted(true)
		orch.addPRUrl("https://pr-1")
		orch.SetState(ConsolidationState{
			SubPhase:      "complete",
			CurrentGroup:  3,
			TotalGroups:   5,
			GroupBranches: []string{"branch-1"},
		})
		orch.Cancel()

		// Reset
		orch.Reset()

		// Verify everything is reset
		if orch.GetInstanceID() != "" {
			t.Error("instanceID should be empty after Reset")
		}
		if orch.GetCompletionFile() != nil {
			t.Error("completionFile should be nil after Reset")
		}
		if orch.GetStartedAt() != nil {
			t.Error("startedAt should be nil after Reset")
		}
		if orch.GetCompletedAt() != nil {
			t.Error("completedAt should be nil after Reset")
		}
		if orch.IsComplete() {
			t.Error("completed should be false after Reset")
		}
		if orch.IsCancelled() {
			t.Error("cancelled should be false after Reset")
		}
		if orch.IsRunning() {
			t.Error("running should be false after Reset")
		}

		state := orch.State()
		if state.SubPhase != "" {
			t.Errorf("SubPhase = %v, want empty string", state.SubPhase)
		}
		if state.CurrentGroup != 0 {
			t.Errorf("CurrentGroup = %v, want 0", state.CurrentGroup)
		}
	})
}

func TestConsolidationCompletionFile(t *testing.T) {
	t.Run("GroupResult fields are accessible", func(t *testing.T) {
		result := GroupResult{
			GroupIndex:    0,
			BranchName:    "test-branch",
			TasksIncluded: []string{"task-1", "task-2"},
			CommitCount:   5,
			Success:       true,
		}

		if result.GroupIndex != 0 {
			t.Errorf("GroupIndex = %v, want 0", result.GroupIndex)
		}
		if result.BranchName != "test-branch" {
			t.Errorf("BranchName = %v, want %v", result.BranchName, "test-branch")
		}
		if len(result.TasksIncluded) != 2 {
			t.Errorf("len(TasksIncluded) = %v, want 2", len(result.TasksIncluded))
		}
		if result.CommitCount != 5 {
			t.Errorf("CommitCount = %v, want 5", result.CommitCount)
		}
		if !result.Success {
			t.Error("Success should be true")
		}
	})

	t.Run("PRInfo fields are accessible", func(t *testing.T) {
		info := PRInfo{
			URL:        "https://github.com/owner/repo/pull/1",
			Title:      "Test PR",
			GroupIndex: 0,
		}

		if info.URL != "https://github.com/owner/repo/pull/1" {
			t.Errorf("URL = %v, want %v", info.URL, "https://github.com/owner/repo/pull/1")
		}
		if info.Title != "Test PR" {
			t.Errorf("Title = %v, want %v", info.Title, "Test PR")
		}
		if info.GroupIndex != 0 {
			t.Errorf("GroupIndex = %v, want 0", info.GroupIndex)
		}
	})
}

func TestInstanceStatus_Constants(t *testing.T) {
	// Verify status constants have expected values
	tests := []struct {
		status InstanceStatus
		want   string
	}{
		{StatusRunning, "running"},
		{StatusCompleted, "completed"},
		{StatusWaitingInput, "waiting_input"},
		{StatusError, "error"},
		{StatusTimeout, "timeout"},
		{StatusStuck, "stuck"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("InstanceStatus %v = %v, want %v", tt.status, string(tt.status), tt.want)
		}
	}
}
