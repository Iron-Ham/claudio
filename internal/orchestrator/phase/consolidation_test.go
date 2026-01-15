package phase

import (
	"context"
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
}

func (m *mockOrchestratorForConsolidation) AddInstance(session any, task string) (any, error) {
	return nil, nil
}
func (m *mockOrchestratorForConsolidation) StartInstance(inst any) error { return nil }
func (m *mockOrchestratorForConsolidation) SaveSession() error           { return nil }
func (m *mockOrchestratorForConsolidation) GetInstanceManager(id string) any {
	return nil
}
func (m *mockOrchestratorForConsolidation) BranchPrefix() string { return m.branchPrefix }

// mockSessionForConsolidation implements UltraPlanSessionInterface for consolidation tests
type mockSessionForConsolidation struct {
	tasks         map[string]any
	readyTasks    []string
	groupComplete bool
	advancedGroup bool
	hasMoreGroups bool
	progress      float64
	previousGroup int
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
func (m *mockSessionForConsolidation) HasMoreGroups() bool { return m.hasMoreGroups }
func (m *mockSessionForConsolidation) Progress() float64   { return m.progress }

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
