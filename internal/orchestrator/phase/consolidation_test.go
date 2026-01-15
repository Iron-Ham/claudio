package phase

import (
	"context"
	"errors"
	"os"
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

		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go", "file2.go"})

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
		if state.ConflictTaskID != "task-1" {
			t.Errorf("ConflictTaskID = %v, want %v", state.ConflictTaskID, "task-1")
		}
		if state.ConflictWorktree != "/path/to/worktree" {
			t.Errorf("ConflictWorktree = %v, want %v", state.ConflictWorktree, "/path/to/worktree")
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
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		orch.ClearConflict()

		state := orch.State()
		if state.CurrentTask != "" {
			t.Errorf("CurrentTask = %v, want empty string", state.CurrentTask)
		}
		if state.ConflictTaskID != "" {
			t.Errorf("ConflictTaskID = %v, want empty string", state.ConflictTaskID)
		}
		if state.ConflictWorktree != "" {
			t.Errorf("ConflictWorktree = %v, want empty string", state.ConflictWorktree)
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
		orch.SetConflict("task-1", "/path/to/worktree", files)

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

// mockWorktreeOperator implements WorktreeOperator for testing
type mockWorktreeOperator struct {
	conflictFiles        []string
	conflictErr          error
	cherryPickInProgress bool
	continueErr          error
	continueCalled       bool
}

func (m *mockWorktreeOperator) GetConflictingFiles(worktreePath string) ([]string, error) {
	return m.conflictFiles, m.conflictErr
}

func (m *mockWorktreeOperator) IsCherryPickInProgress(worktreePath string) bool {
	return m.cherryPickInProgress
}

func (m *mockWorktreeOperator) ContinueCherryPick(worktreePath string) error {
	m.continueCalled = true
	return m.continueErr
}

// mockSessionSaver implements ConsolidationSessionSaver for testing
type mockSessionSaver struct {
	saveErr    error
	saveCalled bool
}

func (m *mockSessionSaver) SaveSession() error {
	m.saveCalled = true
	return m.saveErr
}

func TestConsolidationOrchestrator_ResumeConsolidation(t *testing.T) {
	t.Run("returns ErrNotPaused when not paused", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		// State is not paused by default

		worktreeOp := &mockWorktreeOperator{}
		sessionSaver := &mockSessionSaver{}
		restartCalled := false

		err := orch.ResumeConsolidation(worktreeOp, sessionSaver, func() error {
			restartCalled = true
			return nil
		})

		if !errors.Is(err, ErrNotPaused) {
			t.Errorf("ResumeConsolidation() = %v, want error containing %v", err, ErrNotPaused)
		}
		if restartCalled {
			t.Error("restart callback should not have been called")
		}
	})

	t.Run("returns ErrNoConflictWorktree when no worktree recorded", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		// Set paused state but no worktree
		orch.SetState(ConsolidationState{
			SubPhase:         "paused",
			ConflictWorktree: "", // No worktree
		})

		worktreeOp := &mockWorktreeOperator{}
		sessionSaver := &mockSessionSaver{}

		err := orch.ResumeConsolidation(worktreeOp, sessionSaver, func() error {
			return nil
		})

		if err != ErrNoConflictWorktree {
			t.Errorf("ResumeConsolidation() = %v, want %v", err, ErrNoConflictWorktree)
		}
	})

	t.Run("returns error when checking conflicts fails", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		worktreeOp := &mockWorktreeOperator{
			conflictErr: errors.New("git error"),
		}
		sessionSaver := &mockSessionSaver{}

		err := orch.ResumeConsolidation(worktreeOp, sessionSaver, func() error {
			return nil
		})

		if err == nil || !errors.Is(err, errors.New("git error")) && err.Error() != "failed to check for conflicts in worktree /path/to/worktree: git error" {
			t.Errorf("ResumeConsolidation() = %v, want error about checking conflicts", err)
		}
	})

	t.Run("returns ErrUnresolvedConflicts when conflicts remain", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		worktreeOp := &mockWorktreeOperator{
			conflictFiles: []string{"file1.go", "file2.go"}, // Still has conflicts
		}
		sessionSaver := &mockSessionSaver{}

		err := orch.ResumeConsolidation(worktreeOp, sessionSaver, func() error {
			return nil
		})

		if !errors.Is(err, ErrUnresolvedConflicts) {
			t.Errorf("ResumeConsolidation() = %v, want error containing %v", err, ErrUnresolvedConflicts)
		}
	})

	t.Run("continues cherry-pick when in progress", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		worktreeOp := &mockWorktreeOperator{
			conflictFiles:        []string{}, // No conflicts remaining
			cherryPickInProgress: true,
		}
		sessionSaver := &mockSessionSaver{}
		restartCalled := false

		err := orch.ResumeConsolidation(worktreeOp, sessionSaver, func() error {
			restartCalled = true
			return nil
		})

		if err != nil {
			t.Errorf("ResumeConsolidation() = %v, want nil", err)
		}
		if !worktreeOp.continueCalled {
			t.Error("ContinueCherryPick should have been called")
		}
		if !sessionSaver.saveCalled {
			t.Error("SaveSession should have been called")
		}
		if !restartCalled {
			t.Error("restart callback should have been called")
		}
	})

	t.Run("returns error when cherry-pick continue fails", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		worktreeOp := &mockWorktreeOperator{
			conflictFiles:        []string{},
			cherryPickInProgress: true,
			continueErr:          errors.New("continue failed"),
		}
		sessionSaver := &mockSessionSaver{}

		err := orch.ResumeConsolidation(worktreeOp, sessionSaver, func() error {
			return nil
		})

		if err == nil || !errors.Is(err, errors.New("continue failed")) && err.Error() != "failed to continue cherry-pick: continue failed" {
			t.Errorf("ResumeConsolidation() = %v, want error about cherry-pick", err)
		}
	})

	t.Run("succeeds without cherry-pick in progress", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		worktreeOp := &mockWorktreeOperator{
			conflictFiles:        []string{},
			cherryPickInProgress: false, // No cherry-pick in progress
		}
		sessionSaver := &mockSessionSaver{}
		restartCalled := false

		err := orch.ResumeConsolidation(worktreeOp, sessionSaver, func() error {
			restartCalled = true
			return nil
		})

		if err != nil {
			t.Errorf("ResumeConsolidation() = %v, want nil", err)
		}
		if worktreeOp.continueCalled {
			t.Error("ContinueCherryPick should not have been called")
		}
		if !restartCalled {
			t.Error("restart callback should have been called")
		}
	})

	t.Run("returns error when session save fails", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		worktreeOp := &mockWorktreeOperator{
			conflictFiles: []string{},
		}
		sessionSaver := &mockSessionSaver{
			saveErr: errors.New("save failed"),
		}

		err := orch.ResumeConsolidation(worktreeOp, sessionSaver, func() error {
			return nil
		})

		if err == nil || !errors.Is(err, errors.New("save failed")) && err.Error() != "failed to save session state: save failed" {
			t.Errorf("ResumeConsolidation() = %v, want error about save", err)
		}
	})

	t.Run("returns error when restart callback fails", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		worktreeOp := &mockWorktreeOperator{
			conflictFiles: []string{},
		}
		sessionSaver := &mockSessionSaver{}

		err := orch.ResumeConsolidation(worktreeOp, sessionSaver, func() error {
			return errors.New("restart failed")
		})

		if err == nil || !errors.Is(err, errors.New("restart failed")) && err.Error() != "failed to restart consolidation: restart failed" {
			t.Errorf("ResumeConsolidation() = %v, want error about restart", err)
		}
	})

	t.Run("clears conflict state on success", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		worktreeOp := &mockWorktreeOperator{
			conflictFiles: []string{},
		}
		sessionSaver := &mockSessionSaver{}

		err := orch.ResumeConsolidation(worktreeOp, sessionSaver, func() error {
			return nil
		})

		if err != nil {
			t.Errorf("ResumeConsolidation() = %v, want nil", err)
		}

		// Verify conflict state is cleared
		state := orch.State()
		if state.ConflictTaskID != "" {
			t.Errorf("ConflictTaskID = %v, want empty string", state.ConflictTaskID)
		}
		if state.ConflictWorktree != "" {
			t.Errorf("ConflictWorktree = %v, want empty string", state.ConflictWorktree)
		}
		if state.ConflictFiles != nil {
			t.Error("ConflictFiles should be nil")
		}

		// Verify instance ID is cleared
		if orch.GetInstanceID() != "" {
			t.Errorf("GetInstanceID() = %v, want empty string", orch.GetInstanceID())
		}
	})
}

func TestConsolidationOrchestrator_GetConsolidation(t *testing.T) {
	t.Run("returns nil when no consolidation in progress", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		result := orch.GetConsolidation()
		if result != nil {
			t.Errorf("GetConsolidation() = %v, want nil", result)
		}
	})

	t.Run("returns state when consolidation has started", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetState(ConsolidationState{
			SubPhase:     "merging",
			TotalGroups:  3,
			CurrentGroup: 1,
		})

		result := orch.GetConsolidation()
		if result == nil {
			t.Fatal("GetConsolidation() returned nil, want non-nil")
		}
		if result.SubPhase != "merging" {
			t.Errorf("SubPhase = %v, want %v", result.SubPhase, "merging")
		}
	})

	t.Run("returns state when TotalGroups is set", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetState(ConsolidationState{
			TotalGroups: 5,
		})

		result := orch.GetConsolidation()
		if result == nil {
			t.Fatal("GetConsolidation() returned nil, want non-nil")
		}
	})
}

func TestConsolidationOrchestrator_ClearStateForRestart(t *testing.T) {
	t.Run("clears conflict-related state", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})
		orch.setInstanceID("test-instance")

		orch.ClearStateForRestart()

		state := orch.State()
		if state.SubPhase != "" {
			t.Errorf("SubPhase = %v, want empty string", state.SubPhase)
		}
		if state.CurrentTask != "" {
			t.Errorf("CurrentTask = %v, want empty string", state.CurrentTask)
		}
		if state.ConflictTaskID != "" {
			t.Errorf("ConflictTaskID = %v, want empty string", state.ConflictTaskID)
		}
		if state.ConflictWorktree != "" {
			t.Errorf("ConflictWorktree = %v, want empty string", state.ConflictWorktree)
		}
		if state.ConflictFiles != nil {
			t.Error("ConflictFiles should be nil")
		}
		if orch.GetInstanceID() != "" {
			t.Errorf("GetInstanceID() = %v, want empty string", orch.GetInstanceID())
		}
	})

	t.Run("preserves progress tracking state", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetState(ConsolidationState{
			SubPhase:      "merging",
			CurrentGroup:  2,
			TotalGroups:   5,
			GroupBranches: []string{"branch-1", "branch-2"},
			PRUrls:        []string{"https://pr-1"},
			Error:         "previous error",
		})

		orch.ClearStateForRestart()

		state := orch.State()
		// These should be preserved
		if state.CurrentGroup != 2 {
			t.Errorf("CurrentGroup = %v, want %v", state.CurrentGroup, 2)
		}
		if state.TotalGroups != 5 {
			t.Errorf("TotalGroups = %v, want %v", state.TotalGroups, 5)
		}
		if len(state.GroupBranches) != 2 {
			t.Errorf("len(GroupBranches) = %v, want %v", len(state.GroupBranches), 2)
		}
		if len(state.PRUrls) != 1 {
			t.Errorf("len(PRUrls) = %v, want %v", len(state.PRUrls), 1)
		}
		if state.Error != "previous error" {
			t.Errorf("Error = %v, want %v", state.Error, "previous error")
		}
	})

	t.Run("resets completion and cancellation flags", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.setCompleted(true)
		orch.Cancel()

		orch.ClearStateForRestart()

		if orch.IsComplete() {
			t.Error("IsComplete() should be false after ClearStateForRestart")
		}
		if orch.IsCancelled() {
			t.Error("IsCancelled() should be false after ClearStateForRestart")
		}
		if orch.IsRunning() {
			t.Error("IsRunning() should be false after ClearStateForRestart")
		}
	})
}

func TestConsolidationOrchestrator_GetConflictInfo(t *testing.T) {
	t.Run("returns empty when not paused", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		taskID, worktree, files := orch.GetConflictInfo()
		if taskID != "" {
			t.Errorf("taskID = %v, want empty string", taskID)
		}
		if worktree != "" {
			t.Errorf("worktree = %v, want empty string", worktree)
		}
		if files != nil {
			t.Errorf("files = %v, want nil", files)
		}
	})

	t.Run("returns conflict info when paused", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go", "file2.go"})

		taskID, worktree, files := orch.GetConflictInfo()
		if taskID != "task-1" {
			t.Errorf("taskID = %v, want %v", taskID, "task-1")
		}
		if worktree != "/path/to/worktree" {
			t.Errorf("worktree = %v, want %v", worktree, "/path/to/worktree")
		}
		if len(files) != 2 {
			t.Errorf("len(files) = %v, want %v", len(files), 2)
		}
	})

	t.Run("returns copy of files slice", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		_, _, files := orch.GetConflictInfo()
		files[0] = "modified"

		// Verify internal state is unchanged
		_, _, files2 := orch.GetConflictInfo()
		if files2[0] == "modified" {
			t.Error("GetConflictInfo() should return a copy of files slice")
		}
	})
}

func TestConsolidationOrchestrator_IsPaused(t *testing.T) {
	t.Run("returns false initially", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if orch.IsPaused() {
			t.Error("IsPaused() should return false initially")
		}
	})

	t.Run("returns true when paused", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		if !orch.IsPaused() {
			t.Error("IsPaused() should return true after SetConflict")
		}
	})
}

func TestConsolidationOrchestrator_CanResume(t *testing.T) {
	t.Run("returns false when not paused", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)

		if orch.CanResume() {
			t.Error("CanResume() should return false when not paused")
		}
	})

	t.Run("returns false when paused without worktree", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetState(ConsolidationState{
			SubPhase:         "paused",
			ConflictWorktree: "", // No worktree
		})

		if orch.CanResume() {
			t.Error("CanResume() should return false without worktree")
		}
	})

	t.Run("returns true when paused with worktree", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}

		orch := NewConsolidationOrchestrator(phaseCtx)
		orch.SetConflict("task-1", "/path/to/worktree", []string{"file1.go"})

		if !orch.CanResume() {
			t.Error("CanResume() should return true after SetConflict")
		}
	})
}

func TestResumeErrors(t *testing.T) {
	t.Run("ErrNoConsolidation has correct message", func(t *testing.T) {
		if ErrNoConsolidation.Error() != "no consolidation in progress" {
			t.Errorf("ErrNoConsolidation = %v, want %v", ErrNoConsolidation.Error(), "no consolidation in progress")
		}
	})

	t.Run("ErrNotPaused has correct message", func(t *testing.T) {
		if ErrNotPaused.Error() != "consolidation is not paused" {
			t.Errorf("ErrNotPaused = %v, want %v", ErrNotPaused.Error(), "consolidation is not paused")
		}
	})

	t.Run("ErrNoConflictWorktree has correct message", func(t *testing.T) {
		if ErrNoConflictWorktree.Error() != "no conflict worktree recorded" {
			t.Errorf("ErrNoConflictWorktree = %v, want %v", ErrNoConflictWorktree.Error(), "no conflict worktree recorded")
		}
	})

	t.Run("ErrUnresolvedConflicts has correct message", func(t *testing.T) {
		if ErrUnresolvedConflicts.Error() != "unresolved conflicts remain" {
			t.Errorf("ErrUnresolvedConflicts = %v, want %v", ErrUnresolvedConflicts.Error(), "unresolved conflicts remain")
		}
	})
}

// ============================================================================
// Per-Group Consolidation Tests
// ============================================================================

func TestAggregatedTaskContext_HasContent(t *testing.T) {
	tests := []struct {
		name    string
		context *AggregatedTaskContext
		want    bool
	}{
		{
			name:    "empty context has no content",
			context: &AggregatedTaskContext{},
			want:    false,
		},
		{
			name: "context with issues has content",
			context: &AggregatedTaskContext{
				AllIssues: []string{"issue 1"},
			},
			want: true,
		},
		{
			name: "context with suggestions has content",
			context: &AggregatedTaskContext{
				AllSuggestions: []string{"suggestion 1"},
			},
			want: true,
		},
		{
			name: "context with dependencies has content",
			context: &AggregatedTaskContext{
				Dependencies: []string{"dep 1"},
			},
			want: true,
		},
		{
			name: "context with notes has content",
			context: &AggregatedTaskContext{
				Notes: []string{"note 1"},
			},
			want: true,
		},
		{
			name: "context with only summaries has no content",
			context: &AggregatedTaskContext{
				TaskSummaries: map[string]string{"task-1": "summary"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.context.HasContent(); got != tt.want {
				t.Errorf("HasContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupConsolidationCompletionFileName(t *testing.T) {
	expected := ".claudio-group-consolidation-complete.json"
	if GroupConsolidationCompletionFileName != expected {
		t.Errorf("GroupConsolidationCompletionFileName = %v, want %v",
			GroupConsolidationCompletionFileName, expected)
	}
}

func TestConsolidationTaskWorktreeInfo(t *testing.T) {
	info := ConsolidationTaskWorktreeInfo{
		TaskID:       "task-1",
		TaskTitle:    "Test Task",
		WorktreePath: "/path/to/worktree",
		Branch:       "feature/test",
	}

	if info.TaskID != "task-1" {
		t.Errorf("TaskID = %v, want task-1", info.TaskID)
	}
	if info.TaskTitle != "Test Task" {
		t.Errorf("TaskTitle = %v, want Test Task", info.TaskTitle)
	}
	if info.WorktreePath != "/path/to/worktree" {
		t.Errorf("WorktreePath = %v, want /path/to/worktree", info.WorktreePath)
	}
	if info.Branch != "feature/test" {
		t.Errorf("Branch = %v, want feature/test", info.Branch)
	}
}

func TestConflictResolution(t *testing.T) {
	resolution := ConflictResolution{
		File:       "path/to/file.go",
		Resolution: "Merged both changes",
	}

	if resolution.File != "path/to/file.go" {
		t.Errorf("File = %v, want path/to/file.go", resolution.File)
	}
	if resolution.Resolution != "Merged both changes" {
		t.Errorf("Resolution = %v, want 'Merged both changes'", resolution.Resolution)
	}
}

func TestVerificationResult(t *testing.T) {
	result := VerificationResult{
		ProjectType: "go",
		CommandsRun: []VerificationStep{
			{Name: "build", Command: "go build ./...", Success: true},
			{Name: "test", Command: "go test ./...", Success: false, Output: "test failed"},
		},
		OverallSuccess: false,
		Summary:        "Build passed, tests failed",
	}

	if result.ProjectType != "go" {
		t.Errorf("ProjectType = %v, want go", result.ProjectType)
	}
	if len(result.CommandsRun) != 2 {
		t.Errorf("len(CommandsRun) = %d, want 2", len(result.CommandsRun))
	}
	if result.CommandsRun[1].Output != "test failed" {
		t.Errorf("CommandsRun[1].Output = %v, want 'test failed'", result.CommandsRun[1].Output)
	}
	if result.OverallSuccess {
		t.Error("OverallSuccess should be false")
	}
}

func TestGroupConsolidationCompletionFile(t *testing.T) {
	file := GroupConsolidationCompletionFile{
		GroupIndex:        0,
		Status:            "complete",
		BranchName:        "feature/consolidated",
		TasksConsolidated: []string{"task-1", "task-2"},
		ConflictsResolved: []ConflictResolution{
			{File: "file.go", Resolution: "merged"},
		},
		Verification: VerificationResult{
			OverallSuccess: true,
		},
		AggregatedContext: &AggregatedTaskContext{
			AllIssues: []string{"issue 1"},
		},
		Notes:              "Consolidation notes",
		IssuesForNextGroup: []string{"watch out for X"},
	}

	if file.GroupIndex != 0 {
		t.Errorf("GroupIndex = %d, want 0", file.GroupIndex)
	}
	if file.Status != "complete" {
		t.Errorf("Status = %v, want complete", file.Status)
	}
	if len(file.TasksConsolidated) != 2 {
		t.Errorf("len(TasksConsolidated) = %d, want 2", len(file.TasksConsolidated))
	}
	if len(file.ConflictsResolved) != 1 {
		t.Errorf("len(ConflictsResolved) = %d, want 1", len(file.ConflictsResolved))
	}
	if !file.Verification.OverallSuccess {
		t.Error("Verification.OverallSuccess should be true")
	}
	if file.AggregatedContext == nil {
		t.Error("AggregatedContext should not be nil")
	}
	if len(file.IssuesForNextGroup) != 1 {
		t.Errorf("len(IssuesForNextGroup) = %d, want 1", len(file.IssuesForNextGroup))
	}
}

func TestTaskCompletionFile(t *testing.T) {
	file := TaskCompletionFile{
		TaskID:        "task-1",
		Status:        "complete",
		Summary:       "Implemented feature X",
		FilesModified: []string{"file1.go", "file2.go"},
		Issues:        []string{"issue 1"},
		Suggestions:   []string{"suggestion 1"},
		Dependencies:  []string{"dep 1"},
		Notes:         "Implementation notes",
	}

	if file.TaskID != "task-1" {
		t.Errorf("TaskID = %v, want task-1", file.TaskID)
	}
	if len(file.FilesModified) != 2 {
		t.Errorf("len(FilesModified) = %d, want 2", len(file.FilesModified))
	}
	if len(file.Issues) != 1 {
		t.Errorf("len(Issues) = %d, want 1", len(file.Issues))
	}
}

// ============================================================================
// Mock implementations for per-group consolidation tests
// ============================================================================

// mockGroupConsolidationSession implements GroupConsolidationSessionInterface
type mockGroupConsolidationSession struct {
	mockSessionForConsolidation
	executionOrder            [][]string
	planSummary               string
	sessionID                 string
	branchPrefix              string
	groupConsolidatedBranches []string
	groupConsolidatorIDs      []string
	groupConsolidationCtxs    []*GroupConsolidationCompletionFile
	isMultiPass               bool
}

func newMockGroupConsolidationSession() *mockGroupConsolidationSession {
	return &mockGroupConsolidationSession{
		mockSessionForConsolidation: mockSessionForConsolidation{
			tasks:            make(map[string]any),
			taskCommitCounts: make(map[string]int),
		},
		groupConsolidatedBranches: make([]string, 0),
		groupConsolidatorIDs:      make([]string, 0),
		groupConsolidationCtxs:    make([]*GroupConsolidationCompletionFile, 0),
	}
}

func (m *mockGroupConsolidationSession) GetPlanExecutionOrder() [][]string {
	return m.executionOrder
}

func (m *mockGroupConsolidationSession) GetPlanSummary() string {
	return m.planSummary
}

func (m *mockGroupConsolidationSession) GetID() string {
	return m.sessionID
}

func (m *mockGroupConsolidationSession) GetBranchPrefix() string {
	return m.branchPrefix
}

func (m *mockGroupConsolidationSession) GetGroupConsolidatedBranches() []string {
	return m.groupConsolidatedBranches
}

func (m *mockGroupConsolidationSession) SetGroupConsolidatedBranch(groupIndex int, branch string) {
	for len(m.groupConsolidatedBranches) <= groupIndex {
		m.groupConsolidatedBranches = append(m.groupConsolidatedBranches, "")
	}
	m.groupConsolidatedBranches[groupIndex] = branch
}

func (m *mockGroupConsolidationSession) GetGroupConsolidatorIDs() []string {
	return m.groupConsolidatorIDs
}

func (m *mockGroupConsolidationSession) SetGroupConsolidatorID(groupIndex int, instanceID string) {
	for len(m.groupConsolidatorIDs) <= groupIndex {
		m.groupConsolidatorIDs = append(m.groupConsolidatorIDs, "")
	}
	m.groupConsolidatorIDs[groupIndex] = instanceID
}

func (m *mockGroupConsolidationSession) GetGroupConsolidationContexts() []*GroupConsolidationCompletionFile {
	return m.groupConsolidationCtxs
}

func (m *mockGroupConsolidationSession) SetGroupConsolidationContext(groupIndex int, ctx *GroupConsolidationCompletionFile) {
	for len(m.groupConsolidationCtxs) <= groupIndex {
		m.groupConsolidationCtxs = append(m.groupConsolidationCtxs, nil)
	}
	m.groupConsolidationCtxs[groupIndex] = ctx
}

func (m *mockGroupConsolidationSession) IsMultiPass() bool {
	return m.isMultiPass
}

// mockInstanceForGroupConsolidation implements InstanceInterface for per-group consolidation tests
type mockInstanceForGroupConsolidation struct {
	id            string
	worktreePath  string
	branch        string
	status        InstanceStatus
	filesModified []string
}

func (m *mockInstanceForGroupConsolidation) GetID() string           { return m.id }
func (m *mockInstanceForGroupConsolidation) GetWorktreePath() string { return m.worktreePath }
func (m *mockInstanceForGroupConsolidation) GetBranch() string       { return m.branch }
func (m *mockInstanceForGroupConsolidation) GetStatus() InstanceStatus {
	if m.status == "" {
		return StatusRunning
	}
	return m.status
}
func (m *mockInstanceForGroupConsolidation) GetFilesModified() []string { return m.filesModified }

// mockGroupConsolidationBaseSession implements GroupConsolidationBaseSessionInterface
type mockGroupConsolidationBaseSession struct {
	instancesByTask map[string]*mockInstanceForGroupConsolidation
	ultraGroup      *mockInstanceGroup
	instances       []InstanceInterface
}

func newMockGroupConsolidationBaseSession() *mockGroupConsolidationBaseSession {
	return &mockGroupConsolidationBaseSession{
		instancesByTask: make(map[string]*mockInstanceForGroupConsolidation),
		ultraGroup:      &mockInstanceGroup{instances: make([]string, 0)},
		instances:       make([]InstanceInterface, 0),
	}
}

func (m *mockGroupConsolidationBaseSession) GetInstanceByTask(taskID string) InstanceInterface {
	if inst, ok := m.instancesByTask[taskID]; ok {
		return inst
	}
	return nil
}

func (m *mockGroupConsolidationBaseSession) GetGroupBySessionType(sessionType string) InstanceGroupInterface {
	return m.ultraGroup
}

func (m *mockGroupConsolidationBaseSession) GetInstances() []InstanceInterface {
	return m.instances
}

// mockInstanceGroup implements InstanceGroupInterface - tracks instances added to a group
type mockInstanceGroup struct {
	instances []string
}

func (m *mockInstanceGroup) AddInstance(id string) {
	m.instances = append(m.instances, id)
}

// mockTaskCompletionParser implements TaskCompletionFileParser
type mockTaskCompletionParser struct {
	taskCompletions  map[string]*TaskCompletionFile
	groupCompletions map[string]*GroupConsolidationCompletionFile
	parseErr         error
}

func newMockTaskCompletionParser() *mockTaskCompletionParser {
	return &mockTaskCompletionParser{
		taskCompletions:  make(map[string]*TaskCompletionFile),
		groupCompletions: make(map[string]*GroupConsolidationCompletionFile),
	}
}

func (m *mockTaskCompletionParser) ParseTaskCompletionFile(worktreePath string) (*TaskCompletionFile, error) {
	if m.parseErr != nil {
		return nil, m.parseErr
	}
	if completion, ok := m.taskCompletions[worktreePath]; ok {
		return completion, nil
	}
	return nil, errors.New("completion file not found")
}

func (m *mockTaskCompletionParser) ParseGroupConsolidationCompletionFile(worktreePath string) (*GroupConsolidationCompletionFile, error) {
	if m.parseErr != nil {
		return nil, m.parseErr
	}
	if completion, ok := m.groupCompletions[worktreePath]; ok {
		return completion, nil
	}
	return nil, errors.New("group completion file not found")
}

func (m *mockTaskCompletionParser) GroupConsolidationCompletionFilePath(worktreePath string) string {
	return worktreePath + "/" + GroupConsolidationCompletionFileName
}

// mockGroupConsolidationOrchestrator implements GroupConsolidationOrchestratorInterface
type mockGroupConsolidationOrchestrator struct {
	mockOrchestratorForConsolidation
	mainBranch         string
	claudioDir         string
	tmuxExists         map[string]bool
	addedFromBranch    []addFromBranchCall
	startedInstances   []InstanceInterface
	stoppedInstances   []any
	createdBranches    []createBranchCall
	createdWorktrees   []createWorktreeCall
	removedWorktrees   []string
	cherryPickCalls    []cherryPickCall
	abortedCherryPicks []string
	commitCounts       map[string]int
	pushCalls          []pushCall
	saveSessionCalls   int
	addInstanceErr     error
	startInstanceErr   error
	cherryPickErr      error
	countCommitsErr    error
	pushErr            error
	createBranchErr    error
	createWorktreeErr  error
}

type addFromBranchCall struct {
	session    any
	task       string
	baseBranch string
}

type createBranchCall struct {
	newBranch  string
	baseBranch string
}

type createWorktreeCall struct {
	worktreePath string
	branch       string
}

type cherryPickCall struct {
	worktreePath string
	branch       string
}

type pushCall struct {
	worktreePath string
	force        bool
}

func newMockGroupConsolidationOrchestrator() *mockGroupConsolidationOrchestrator {
	return &mockGroupConsolidationOrchestrator{
		tmuxExists:   make(map[string]bool),
		commitCounts: make(map[string]int),
	}
}

func (m *mockGroupConsolidationOrchestrator) AddInstanceFromBranch(session any, task string, baseBranch string) (InstanceInterface, error) {
	m.addedFromBranch = append(m.addedFromBranch, addFromBranchCall{session, task, baseBranch})
	if m.addInstanceErr != nil {
		return nil, m.addInstanceErr
	}
	inst := &mockInstanceForGroupConsolidation{
		id:           "consolidated-inst-" + baseBranch,
		worktreePath: "/tmp/worktree-" + baseBranch,
		branch:       baseBranch,
	}
	return inst, nil
}

func (m *mockGroupConsolidationOrchestrator) StopInstance(inst any) error {
	m.stoppedInstances = append(m.stoppedInstances, inst)
	return nil
}

func (m *mockGroupConsolidationOrchestrator) FindMainBranch() string {
	if m.mainBranch == "" {
		return "main"
	}
	return m.mainBranch
}

func (m *mockGroupConsolidationOrchestrator) CreateBranchFrom(newBranch, baseBranch string) error {
	m.createdBranches = append(m.createdBranches, createBranchCall{newBranch, baseBranch})
	return m.createBranchErr
}

func (m *mockGroupConsolidationOrchestrator) CreateWorktreeFromBranch(worktreePath, branch string) error {
	m.createdWorktrees = append(m.createdWorktrees, createWorktreeCall{worktreePath, branch})
	return m.createWorktreeErr
}

func (m *mockGroupConsolidationOrchestrator) RemoveWorktree(worktreePath string) error {
	m.removedWorktrees = append(m.removedWorktrees, worktreePath)
	return nil
}

func (m *mockGroupConsolidationOrchestrator) CherryPickBranch(worktreePath, branch string) error {
	m.cherryPickCalls = append(m.cherryPickCalls, cherryPickCall{worktreePath, branch})
	return m.cherryPickErr
}

func (m *mockGroupConsolidationOrchestrator) AbortCherryPick(worktreePath string) error {
	m.abortedCherryPicks = append(m.abortedCherryPicks, worktreePath)
	return nil
}

func (m *mockGroupConsolidationOrchestrator) CountCommitsBetween(worktreePath, base, head string) (int, error) {
	if m.countCommitsErr != nil {
		return 0, m.countCommitsErr
	}
	key := worktreePath + ":" + base + ":" + head
	if count, ok := m.commitCounts[key]; ok {
		return count, nil
	}
	// Default: return number of cherry-pick calls as proxy for commits
	return len(m.cherryPickCalls), nil
}

func (m *mockGroupConsolidationOrchestrator) Push(worktreePath string, force bool) error {
	m.pushCalls = append(m.pushCalls, pushCall{worktreePath, force})
	return m.pushErr
}

func (m *mockGroupConsolidationOrchestrator) GetClaudioDir() string {
	if m.claudioDir == "" {
		return "/tmp/.claudio"
	}
	return m.claudioDir
}

func (m *mockGroupConsolidationOrchestrator) TmuxSessionExists(instanceID string) bool {
	return m.tmuxExists[instanceID]
}

func (m *mockGroupConsolidationOrchestrator) SaveSession() error {
	m.saveSessionCalls++
	return nil
}

func (m *mockGroupConsolidationOrchestrator) StartInstance(inst any) error {
	if i, ok := inst.(InstanceInterface); ok {
		m.startedInstances = append(m.startedInstances, i)
	}
	return m.startInstanceErr
}

// mockGroupConsolidationEventEmitter implements GroupConsolidationEventEmitter
type mockGroupConsolidationEventEmitter struct {
	events []groupConsolidationEvent
}

type groupConsolidationEvent struct {
	eventType  string
	groupIndex int
	message    string
}

func (m *mockGroupConsolidationEventEmitter) EmitGroupConsolidationEvent(eventType string, groupIndex int, message string) {
	m.events = append(m.events, groupConsolidationEvent{eventType, groupIndex, message})
}

// ============================================================================
// GatherTaskCompletionContextForGroup Tests
// ============================================================================

func TestConsolidationOrchestrator_GatherTaskCompletionContextForGroup(t *testing.T) {
	t.Run("returns empty context for invalid group index", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1", "task-2"}}

		baseSession := newMockGroupConsolidationBaseSession()
		parser := newMockTaskCompletionParser()

		// Request group 5 when only group 0 exists
		ctx := orch.GatherTaskCompletionContextForGroup(5, session, baseSession, parser)

		if ctx == nil {
			t.Fatal("Context should not be nil")
		}
		if len(ctx.TaskSummaries) != 0 {
			t.Errorf("TaskSummaries should be empty, got %d", len(ctx.TaskSummaries))
		}
	})

	t.Run("gathers context from task completion files", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1", "task-2"}}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:           "inst-1",
			worktreePath: "/worktree/task-1",
			branch:       "feature/task-1",
		}
		baseSession.instancesByTask["task-2"] = &mockInstanceForGroupConsolidation{
			id:           "inst-2",
			worktreePath: "/worktree/task-2",
			branch:       "feature/task-2",
		}

		parser := newMockTaskCompletionParser()
		parser.taskCompletions["/worktree/task-1"] = &TaskCompletionFile{
			TaskID:       "task-1",
			Summary:      "Implemented feature A",
			Issues:       []string{"issue 1"},
			Suggestions:  []string{"suggestion 1"},
			Dependencies: []string{"dep-1"},
			Notes:        "Notes for task 1",
		}
		parser.taskCompletions["/worktree/task-2"] = &TaskCompletionFile{
			TaskID:       "task-2",
			Summary:      "Implemented feature B",
			Issues:       []string{"issue 2"},
			Dependencies: []string{"dep-1", "dep-2"}, // dep-1 is duplicate
			Notes:        "Notes for task 2",
		}

		ctx := orch.GatherTaskCompletionContextForGroup(0, session, baseSession, parser)

		// Check summaries
		if len(ctx.TaskSummaries) != 2 {
			t.Errorf("TaskSummaries length = %d, want 2", len(ctx.TaskSummaries))
		}
		if ctx.TaskSummaries["task-1"] != "Implemented feature A" {
			t.Errorf("TaskSummaries[task-1] = %v, want 'Implemented feature A'", ctx.TaskSummaries["task-1"])
		}

		// Check issues (should be prefixed with task ID)
		if len(ctx.AllIssues) != 2 {
			t.Errorf("AllIssues length = %d, want 2", len(ctx.AllIssues))
		}

		// Check suggestions
		if len(ctx.AllSuggestions) != 1 {
			t.Errorf("AllSuggestions length = %d, want 1", len(ctx.AllSuggestions))
		}

		// Check deduplicated dependencies
		if len(ctx.Dependencies) != 2 {
			t.Errorf("Dependencies length = %d, want 2 (deduplicated)", len(ctx.Dependencies))
		}

		// Check notes
		if len(ctx.Notes) != 2 {
			t.Errorf("Notes length = %d, want 2", len(ctx.Notes))
		}
	})

	t.Run("skips tasks without instances", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1", "task-2"}}

		baseSession := newMockGroupConsolidationBaseSession()
		// Only task-1 has an instance
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:           "inst-1",
			worktreePath: "/worktree/task-1",
		}

		parser := newMockTaskCompletionParser()
		parser.taskCompletions["/worktree/task-1"] = &TaskCompletionFile{
			TaskID:  "task-1",
			Summary: "Task 1 summary",
		}

		ctx := orch.GatherTaskCompletionContextForGroup(0, session, baseSession, parser)

		// Should only have task-1
		if len(ctx.TaskSummaries) != 1 {
			t.Errorf("TaskSummaries length = %d, want 1", len(ctx.TaskSummaries))
		}
	})

	t.Run("skips tasks with empty worktree path", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:           "inst-1",
			worktreePath: "", // Empty worktree
		}

		parser := newMockTaskCompletionParser()

		ctx := orch.GatherTaskCompletionContextForGroup(0, session, baseSession, parser)

		if len(ctx.TaskSummaries) != 0 {
			t.Errorf("TaskSummaries length = %d, want 0", len(ctx.TaskSummaries))
		}
	})

	t.Run("handles empty strings in issues and suggestions", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:           "inst-1",
			worktreePath: "/worktree/task-1",
		}

		parser := newMockTaskCompletionParser()
		parser.taskCompletions["/worktree/task-1"] = &TaskCompletionFile{
			TaskID:       "task-1",
			Summary:      "Summary",
			Issues:       []string{"", "real issue", ""},
			Suggestions:  []string{"", "real suggestion"},
			Dependencies: []string{"", "real-dep"},
		}

		ctx := orch.GatherTaskCompletionContextForGroup(0, session, baseSession, parser)

		// Empty strings should be filtered out
		if len(ctx.AllIssues) != 1 {
			t.Errorf("AllIssues length = %d, want 1", len(ctx.AllIssues))
		}
		if len(ctx.AllSuggestions) != 1 {
			t.Errorf("AllSuggestions length = %d, want 1", len(ctx.AllSuggestions))
		}
		if len(ctx.Dependencies) != 1 {
			t.Errorf("Dependencies length = %d, want 1", len(ctx.Dependencies))
		}
	})
}

// ============================================================================
// GetTaskBranchesForGroup Tests
// ============================================================================

func TestConsolidationOrchestrator_GetTaskBranchesForGroup(t *testing.T) {
	t.Run("returns nil for invalid group index", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}

		baseSession := newMockGroupConsolidationBaseSession()

		branches := orch.GetTaskBranchesForGroup(10, session, baseSession)

		if branches != nil {
			t.Errorf("Expected nil for invalid group index, got %v", branches)
		}
	})

	t.Run("returns task branches for valid group", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1", "task-2"}}
		session.tasks["task-1"] = map[string]any{"title": "Task 1 Title"}
		session.tasks["task-2"] = map[string]any{"title": "Task 2 Title"}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:           "inst-1",
			worktreePath: "/worktree/task-1",
			branch:       "feature/task-1",
		}
		baseSession.instancesByTask["task-2"] = &mockInstanceForGroupConsolidation{
			id:           "inst-2",
			worktreePath: "/worktree/task-2",
			branch:       "feature/task-2",
		}

		branches := orch.GetTaskBranchesForGroup(0, session, baseSession)

		if len(branches) != 2 {
			t.Fatalf("Expected 2 branches, got %d", len(branches))
		}
		if branches[0].TaskID != "task-1" {
			t.Errorf("First branch TaskID = %v, want task-1", branches[0].TaskID)
		}
		if branches[0].Branch != "feature/task-1" {
			t.Errorf("First branch Branch = %v, want feature/task-1", branches[0].Branch)
		}
		if branches[0].TaskTitle != "Task 1 Title" {
			t.Errorf("First branch TaskTitle = %v, want 'Task 1 Title'", branches[0].TaskTitle)
		}
	})

	t.Run("skips tasks without instances", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1", "task-2"}}
		session.tasks["task-1"] = map[string]any{"title": "Task 1"}
		session.tasks["task-2"] = map[string]any{"title": "Task 2"}

		baseSession := newMockGroupConsolidationBaseSession()
		// Only task-1 has an instance
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}

		branches := orch.GetTaskBranchesForGroup(0, session, baseSession)

		if len(branches) != 1 {
			t.Errorf("Expected 1 branch, got %d", len(branches))
		}
	})

	t.Run("skips tasks with nil task info", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1", "task-2"}}
		session.tasks["task-1"] = map[string]any{"title": "Task 1"}
		// task-2 has no entry in tasks map

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}

		branches := orch.GetTaskBranchesForGroup(0, session, baseSession)

		if len(branches) != 1 {
			t.Errorf("Expected 1 branch, got %d", len(branches))
		}
	})
}

// ============================================================================
// extractTaskTitle Tests
// ============================================================================

func TestExtractTaskTitle(t *testing.T) {
	t.Run("extracts title from map with lowercase key", func(t *testing.T) {
		task := map[string]any{"title": "My Task Title"}
		got := extractTaskTitle(task)
		if got != "My Task Title" {
			t.Errorf("extractTaskTitle() = %v, want 'My Task Title'", got)
		}
	})

	t.Run("extracts title from map with uppercase key", func(t *testing.T) {
		task := map[string]any{"Title": "My Task Title"}
		got := extractTaskTitle(task)
		if got != "My Task Title" {
			t.Errorf("extractTaskTitle() = %v, want 'My Task Title'", got)
		}
	})

	t.Run("returns Unknown Task for nil", func(t *testing.T) {
		got := extractTaskTitle(nil)
		if got != "Unknown Task" {
			t.Errorf("extractTaskTitle(nil) = %v, want 'Unknown Task'", got)
		}
	})

	t.Run("returns Unknown Task for empty map", func(t *testing.T) {
		task := map[string]any{}
		got := extractTaskTitle(task)
		if got != "Unknown Task" {
			t.Errorf("extractTaskTitle() = %v, want 'Unknown Task'", got)
		}
	})

	t.Run("returns Unknown Task for non-string title", func(t *testing.T) {
		task := map[string]any{"title": 123}
		got := extractTaskTitle(task)
		if got != "Unknown Task" {
			t.Errorf("extractTaskTitle() = %v, want 'Unknown Task'", got)
		}
	})

	t.Run("extracts title from struct with GetTitle method", func(t *testing.T) {
		task := &mockTaskWithTitle{title: "Struct Task Title"}
		got := extractTaskTitle(task)
		if got != "Struct Task Title" {
			t.Errorf("extractTaskTitle() = %v, want 'Struct Task Title'", got)
		}
	})

	t.Run("returns Unknown Task for struct without GetTitle", func(t *testing.T) {
		task := struct{ Name string }{Name: "test"}
		got := extractTaskTitle(task)
		if got != "Unknown Task" {
			t.Errorf("extractTaskTitle() = %v, want 'Unknown Task'", got)
		}
	})
}

type mockTaskWithTitle struct {
	title string
}

func (m *mockTaskWithTitle) GetTitle() string {
	return m.title
}

// ============================================================================
// fileExists Tests
// ============================================================================

func TestFileExists(t *testing.T) {
	// Save original and restore after test
	originalStatFile := statFile
	defer func() { statFile = originalStatFile }()

	t.Run("returns true when file exists", func(t *testing.T) {
		statFile = func(path string) (os.FileInfo, error) {
			return nil, nil // No error means file exists
		}

		if !fileExists("/some/path") {
			t.Error("fileExists should return true when file exists")
		}
	})

	t.Run("returns false when file does not exist", func(t *testing.T) {
		statFile = func(path string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		}

		if fileExists("/some/path") {
			t.Error("fileExists should return false when file does not exist")
		}
	})
}

// ============================================================================
// BuildGroupConsolidatorPrompt Tests
// ============================================================================

func TestConsolidationOrchestrator_BuildGroupConsolidatorPrompt(t *testing.T) {
	t.Run("builds prompt with all sections", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}
		session.planSummary = "Test Plan Summary"
		session.sessionID = "session-12345678"
		session.branchPrefix = "test-prefix"
		session.tasks["task-1"] = map[string]any{"title": "Task 1 Title"}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:           "inst-1",
			worktreePath: "/worktree/task-1",
			branch:       "feature/task-1",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.mainBranch = "main"

		parser := newMockTaskCompletionParser()
		parser.taskCompletions["/worktree/task-1"] = &TaskCompletionFile{
			TaskID:      "task-1",
			Summary:     "Task 1 summary",
			Issues:      []string{"issue 1"},
			Suggestions: []string{"suggestion 1"},
			Notes:       "Task 1 notes",
		}

		prompt := orch.BuildGroupConsolidatorPrompt(0, session, baseSession, mockOrch, parser)

		// Check header
		if !contains(prompt, "# Group 1 Consolidation") {
			t.Error("Prompt should contain group header")
		}
		if !contains(prompt, "Test Plan Summary") {
			t.Error("Prompt should contain plan summary")
		}

		// Check tasks section
		if !contains(prompt, "## Tasks Completed in This Group") {
			t.Error("Prompt should contain tasks section")
		}
		if !contains(prompt, "task-1: Task 1 Title") {
			t.Error("Prompt should contain task info")
		}

		// Check context sections
		if !contains(prompt, "## Implementation Notes from Tasks") {
			t.Error("Prompt should contain notes section")
		}
		if !contains(prompt, "## Issues Raised by Tasks") {
			t.Error("Prompt should contain issues section")
		}
		if !contains(prompt, "## Integration Suggestions from Tasks") {
			t.Error("Prompt should contain suggestions section")
		}

		// Check branch configuration
		if !contains(prompt, "## Branch Configuration") {
			t.Error("Prompt should contain branch configuration")
		}
		if !contains(prompt, "main") {
			t.Error("Prompt should contain base branch")
		}

		// Check instructions
		if !contains(prompt, "## Your Tasks") {
			t.Error("Prompt should contain instructions")
		}

		// Check completion protocol
		if !contains(prompt, "## Completion Protocol") {
			t.Error("Prompt should contain completion protocol")
		}
		if !contains(prompt, GroupConsolidationCompletionFileName) {
			t.Error("Prompt should contain completion file name")
		}
	})

	t.Run("includes previous group context for group > 0", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}, {"task-2"}}
		session.planSummary = "Test Plan"
		session.sessionID = "session-12345678"
		session.tasks["task-2"] = map[string]any{"title": "Task 2"}
		session.groupConsolidationCtxs = []*GroupConsolidationCompletionFile{
			{
				GroupIndex: 0,
				Notes:      "Notes from group 0",
				IssuesForNextGroup: []string{
					"Watch out for X",
					"Consider Y",
				},
			},
		}
		session.groupConsolidatedBranches = []string{"group-0-branch"}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-2"] = &mockInstanceForGroupConsolidation{
			id:     "inst-2",
			branch: "feature/task-2",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		parser := newMockTaskCompletionParser()

		prompt := orch.BuildGroupConsolidatorPrompt(1, session, baseSession, mockOrch, parser)

		// Check previous context section
		if !contains(prompt, "## Context from Previous Group's Consolidator") {
			t.Error("Prompt should contain previous group context")
		}
		if !contains(prompt, "Notes from group 0") {
			t.Error("Prompt should contain previous group notes")
		}
		if !contains(prompt, "Watch out for X") {
			t.Error("Prompt should contain previous group issues")
		}

		// Base branch should be previous group's consolidated branch
		if !contains(prompt, "group-0-branch") {
			t.Error("Prompt should use previous group's branch as base")
		}
	})

	t.Run("uses main branch for group 0", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}
		session.planSummary = "Test Plan"
		session.sessionID = "session-12345678"
		session.tasks["task-1"] = map[string]any{"title": "Task 1"}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.mainBranch = "master"

		parser := newMockTaskCompletionParser()

		prompt := orch.BuildGroupConsolidatorPrompt(0, session, baseSession, mockOrch, parser)

		if !contains(prompt, "master") {
			t.Error("Prompt should use main branch for group 0")
		}
	})
}

// ============================================================================
// determineBaseBranchForGroup Tests
// ============================================================================

func TestConsolidationOrchestrator_determineBaseBranchForGroup(t *testing.T) {
	t.Run("returns main branch for group 0", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.mainBranch = "develop"

		branch := orch.determineBaseBranchForGroup(0, session, mockOrch)

		if branch != "develop" {
			t.Errorf("Expected 'develop', got '%s'", branch)
		}
	})

	t.Run("returns previous group branch for group > 0", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.groupConsolidatedBranches = []string{"group-0-consolidated"}
		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.mainBranch = "main"

		branch := orch.determineBaseBranchForGroup(1, session, mockOrch)

		if branch != "group-0-consolidated" {
			t.Errorf("Expected 'group-0-consolidated', got '%s'", branch)
		}
	})

	t.Run("falls back to main branch if previous group branch is empty", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.groupConsolidatedBranches = []string{""} // Empty branch
		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.mainBranch = "main"

		branch := orch.determineBaseBranchForGroup(1, session, mockOrch)

		if branch != "main" {
			t.Errorf("Expected 'main', got '%s'", branch)
		}
	})

	t.Run("falls back to main branch if previous group not in slice", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.groupConsolidatedBranches = []string{} // Empty slice
		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.mainBranch = "main"

		branch := orch.determineBaseBranchForGroup(5, session, mockOrch)

		if branch != "main" {
			t.Errorf("Expected 'main', got '%s'", branch)
		}
	})
}

// ============================================================================
// generateGroupBranchName Tests
// ============================================================================

func TestConsolidationOrchestrator_generateGroupBranchName(t *testing.T) {
	t.Run("uses session branch prefix", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.branchPrefix = "my-prefix"
		session.sessionID = "abcdefghij"

		mockOrch := newMockGroupConsolidationOrchestrator()

		branchName := orch.generateGroupBranchName(0, session, mockOrch)

		if branchName != "my-prefix/ultraplan-abcdefgh-group-1" {
			t.Errorf("Expected 'my-prefix/ultraplan-abcdefgh-group-1', got '%s'", branchName)
		}
	})

	t.Run("falls back to orchestrator branch prefix", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.branchPrefix = ""
		session.sessionID = "abcdefghij"

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.branchPrefix = "orch-prefix"

		branchName := orch.generateGroupBranchName(1, session, mockOrch)

		if branchName != "orch-prefix/ultraplan-abcdefgh-group-2" {
			t.Errorf("Expected 'orch-prefix/ultraplan-abcdefgh-group-2', got '%s'", branchName)
		}
	})

	t.Run("uses default prefix when both are empty", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.branchPrefix = ""
		session.sessionID = "shortid"

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.branchPrefix = ""

		branchName := orch.generateGroupBranchName(0, session, mockOrch)

		if branchName != "Iron-Ham/ultraplan-shortid-group-1" {
			t.Errorf("Expected 'Iron-Ham/ultraplan-shortid-group-1', got '%s'", branchName)
		}
	})

	t.Run("truncates long session IDs", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.branchPrefix = "prefix"
		session.sessionID = "very-long-session-id-12345"

		mockOrch := newMockGroupConsolidationOrchestrator()

		branchName := orch.generateGroupBranchName(0, session, mockOrch)

		// Should truncate to first 8 chars
		if branchName != "prefix/ultraplan-very-lon-group-1" {
			t.Errorf("Expected 'prefix/ultraplan-very-lon-group-1', got '%s'", branchName)
		}
	})
}

// ============================================================================
// ConsolidateGroupWithVerification Tests
// ============================================================================

func TestConsolidationOrchestrator_ConsolidateGroupWithVerification(t *testing.T) {
	t.Run("returns error for invalid group index", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}

		baseSession := newMockGroupConsolidationBaseSession()
		mockOrch := newMockGroupConsolidationOrchestrator()

		err := orch.ConsolidateGroupWithVerification(-1, session, baseSession, mockOrch, nil)
		if err == nil || !contains(err.Error(), "invalid group index") {
			t.Errorf("Expected error for invalid group index, got %v", err)
		}

		err = orch.ConsolidateGroupWithVerification(10, session, baseSession, mockOrch, nil)
		if err == nil || !contains(err.Error(), "invalid group index") {
			t.Errorf("Expected error for out-of-bounds group index, got %v", err)
		}
	})

	t.Run("returns nil for empty group", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{}} // Empty group

		baseSession := newMockGroupConsolidationBaseSession()
		mockOrch := newMockGroupConsolidationOrchestrator()

		err := orch.ConsolidateGroupWithVerification(0, session, baseSession, mockOrch, nil)
		if err != nil {
			t.Errorf("Expected nil for empty group, got %v", err)
		}
	})

	t.Run("returns error when no tasks have commits", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1", "task-2"}}
		session.taskCommitCounts = map[string]int{
			"task-1": 0, // No commits
			"task-2": 0, // No commits
		}

		baseSession := newMockGroupConsolidationBaseSession()
		mockOrch := newMockGroupConsolidationOrchestrator()

		err := orch.ConsolidateGroupWithVerification(0, session, baseSession, mockOrch, nil)
		if err == nil || !contains(err.Error(), "no task branches with verified commits") {
			t.Errorf("Expected error for no commits, got %v", err)
		}
	})

	t.Run("consolidates successfully", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1", "task-2"}}
		session.sessionID = "session-id"
		session.branchPrefix = "test"
		session.taskCommitCounts = map[string]int{
			"task-1": 3,
			"task-2": 2,
		}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}
		baseSession.instancesByTask["task-2"] = &mockInstanceForGroupConsolidation{
			id:     "inst-2",
			branch: "feature/task-2",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.mainBranch = "main"
		mockOrch.claudioDir = "/tmp/.claudio"
		// Set commit count for verification
		mockOrch.commitCounts["/tmp/.claudio/consolidation-group-0:main:HEAD"] = 5

		emitter := &mockGroupConsolidationEventEmitter{}

		err := orch.ConsolidateGroupWithVerification(0, session, baseSession, mockOrch, emitter)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Verify branch was created
		if len(mockOrch.createdBranches) != 1 {
			t.Errorf("Expected 1 branch creation, got %d", len(mockOrch.createdBranches))
		}

		// Verify worktree was created and removed
		if len(mockOrch.createdWorktrees) != 1 {
			t.Errorf("Expected 1 worktree creation, got %d", len(mockOrch.createdWorktrees))
		}
		if len(mockOrch.removedWorktrees) != 1 {
			t.Errorf("Expected 1 worktree removal, got %d", len(mockOrch.removedWorktrees))
		}

		// Verify cherry-picks
		if len(mockOrch.cherryPickCalls) != 2 {
			t.Errorf("Expected 2 cherry-pick calls, got %d", len(mockOrch.cherryPickCalls))
		}

		// Verify push
		if len(mockOrch.pushCalls) != 1 {
			t.Errorf("Expected 1 push call, got %d", len(mockOrch.pushCalls))
		}

		// Verify consolidated branch was set in session
		if len(session.groupConsolidatedBranches) < 1 {
			t.Error("Consolidated branch should be set in session")
		}

		// Verify event was emitted
		if len(emitter.events) < 1 {
			t.Error("Expected at least one event")
		}
	})

	t.Run("handles cherry-pick failure", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}
		session.sessionID = "session-id"
		session.taskCommitCounts = map[string]int{"task-1": 1}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.cherryPickErr = errors.New("cherry-pick conflict")

		err := orch.ConsolidateGroupWithVerification(0, session, baseSession, mockOrch, nil)
		if err == nil || !contains(err.Error(), "failed to cherry-pick") {
			t.Errorf("Expected cherry-pick error, got %v", err)
		}

		// Verify cherry-pick was aborted
		if len(mockOrch.abortedCherryPicks) != 1 {
			t.Errorf("Expected 1 aborted cherry-pick, got %d", len(mockOrch.abortedCherryPicks))
		}
	})

	t.Run("handles zero commits after consolidation", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}
		session.sessionID = "session-id"
		session.taskCommitCounts = map[string]int{"task-1": 1}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		// Force zero commit count
		mockOrch.commitCounts["/tmp/.claudio/consolidation-group-0:main:HEAD"] = 0

		err := orch.ConsolidateGroupWithVerification(0, session, baseSession, mockOrch, nil)
		if err == nil || !contains(err.Error(), "no commits") {
			t.Errorf("Expected error for zero commits, got %v", err)
		}
	})

	t.Run("handles push failure gracefully", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}
		session.sessionID = "session-id"
		session.taskCommitCounts = map[string]int{"task-1": 1}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.claudioDir = "/tmp/.claudio"
		mockOrch.commitCounts["/tmp/.claudio/consolidation-group-0:main:HEAD"] = 1
		mockOrch.pushErr = errors.New("push failed")

		emitter := &mockGroupConsolidationEventEmitter{}

		// Push failure should not cause overall failure
		err := orch.ConsolidateGroupWithVerification(0, session, baseSession, mockOrch, emitter)
		if err != nil {
			t.Errorf("Push failure should not cause overall failure, got %v", err)
		}

		// Should emit a warning event
		foundWarning := false
		for _, e := range emitter.events {
			if e.eventType == "group_push_warning" {
				foundWarning = true
				break
			}
		}
		if !foundWarning {
			t.Error("Expected push warning event")
		}
	})

	t.Run("handles branch creation failure", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}
		session.sessionID = "session-id"
		session.taskCommitCounts = map[string]int{"task-1": 1}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.createBranchErr = errors.New("branch exists")

		err := orch.ConsolidateGroupWithVerification(0, session, baseSession, mockOrch, nil)
		if err == nil || !contains(err.Error(), "failed to create consolidated branch") {
			t.Errorf("Expected branch creation error, got %v", err)
		}
	})

	t.Run("handles worktree creation failure", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}
		session.sessionID = "session-id"
		session.taskCommitCounts = map[string]int{"task-1": 1}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.createWorktreeErr = errors.New("worktree error")

		err := orch.ConsolidateGroupWithVerification(0, session, baseSession, mockOrch, nil)
		if err == nil || !contains(err.Error(), "failed to create consolidation worktree") {
			t.Errorf("Expected worktree creation error, got %v", err)
		}
	})

	t.Run("handles count commits failure", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}
		session.sessionID = "session-id"
		session.taskCommitCounts = map[string]int{"task-1": 1}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.countCommitsErr = errors.New("git error")

		err := orch.ConsolidateGroupWithVerification(0, session, baseSession, mockOrch, nil)
		if err == nil || !contains(err.Error(), "failed to verify consolidated branch") {
			t.Errorf("Expected commit count error, got %v", err)
		}
	})
}

// ============================================================================
// StartGroupConsolidatorSession Tests
// ============================================================================

func TestConsolidationOrchestrator_StartGroupConsolidatorSession(t *testing.T) {
	t.Run("returns error for invalid group index", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}

		baseSession := newMockGroupConsolidationBaseSession()
		mockOrch := newMockGroupConsolidationOrchestrator()
		parser := newMockTaskCompletionParser()

		err := orch.StartGroupConsolidatorSession(-1, session, baseSession, mockOrch, parser, nil)
		if err == nil || !contains(err.Error(), "invalid group index") {
			t.Errorf("Expected error for invalid group index, got %v", err)
		}
	})

	t.Run("returns nil for empty group", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{}} // Empty group

		baseSession := newMockGroupConsolidationBaseSession()
		mockOrch := newMockGroupConsolidationOrchestrator()
		parser := newMockTaskCompletionParser()

		err := orch.StartGroupConsolidatorSession(0, session, baseSession, mockOrch, parser, nil)
		if err != nil {
			t.Errorf("Expected nil for empty group, got %v", err)
		}
	})

	t.Run("returns error when no tasks have commits", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}
		session.taskCommitCounts = map[string]int{"task-1": 0}

		baseSession := newMockGroupConsolidationBaseSession()
		mockOrch := newMockGroupConsolidationOrchestrator()
		parser := newMockTaskCompletionParser()

		err := orch.StartGroupConsolidatorSession(0, session, baseSession, mockOrch, parser, nil)
		if err == nil || !contains(err.Error(), "no task branches with verified commits") {
			t.Errorf("Expected error for no commits, got %v", err)
		}
	})

	t.Run("returns error when instance creation fails", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}
		session.sessionID = "session-id"
		session.taskCommitCounts = map[string]int{"task-1": 1}
		session.tasks["task-1"] = map[string]any{"title": "Task 1"}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.addInstanceErr = errors.New("instance creation failed")
		parser := newMockTaskCompletionParser()

		err := orch.StartGroupConsolidatorSession(0, session, baseSession, mockOrch, parser, nil)
		if err == nil || !contains(err.Error(), "failed to create group consolidator instance") {
			t.Errorf("Expected instance creation error, got %v", err)
		}
	})

	t.Run("returns error when instance start fails", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		session.executionOrder = [][]string{{"task-1"}}
		session.sessionID = "session-id"
		session.taskCommitCounts = map[string]int{"task-1": 1}
		session.tasks["task-1"] = map[string]any{"title": "Task 1"}

		baseSession := newMockGroupConsolidationBaseSession()
		baseSession.instancesByTask["task-1"] = &mockInstanceForGroupConsolidation{
			id:     "inst-1",
			branch: "feature/task-1",
		}

		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.startInstanceErr = errors.New("start failed")
		parser := newMockTaskCompletionParser()
		emitter := &mockGroupConsolidationEventEmitter{}

		err := orch.StartGroupConsolidatorSession(0, session, baseSession, mockOrch, parser, emitter)
		if err == nil || !contains(err.Error(), "failed to start group consolidator instance") {
			t.Errorf("Expected start instance error, got %v", err)
		}

		// Verify event was emitted for start
		foundStart := false
		for _, e := range emitter.events {
			if e.eventType == "group_consolidator_started" {
				foundStart = true
				break
			}
		}
		if !foundStart {
			t.Error("Expected group_consolidator_started event")
		}
	})
}

// ============================================================================
// MonitorGroupConsolidator Tests
// ============================================================================

func TestConsolidationOrchestrator_MonitorGroupConsolidator(t *testing.T) {
	// Save original statFile and restore after tests
	originalStatFile := statFile
	defer func() { statFile = originalStatFile }()

	t.Run("returns error when instance not found", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.instance = nil // Instance not found
		parser := newMockTaskCompletionParser()

		err := orch.MonitorGroupConsolidator(0, "inst-1", session, mockOrch, parser, nil)
		if err == nil || !contains(err.Error(), "consolidator instance not found") {
			t.Errorf("Expected instance not found error, got %v", err)
		}
	})

	t.Run("returns error when completion file indicates failure", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.instance = &mockInstanceForGroupConsolidation{
			id:           "inst-1",
			worktreePath: "/tmp/worktree",
			status:       StatusRunning,
		}

		parser := newMockTaskCompletionParser()
		parser.groupCompletions["/tmp/worktree"] = &GroupConsolidationCompletionFile{
			GroupIndex: 0,
			Status:     "failed",
			Notes:      "Something went wrong",
		}

		// Mock file exists
		statFile = func(path string) (os.FileInfo, error) {
			if path == "/tmp/worktree/"+GroupConsolidationCompletionFileName {
				return nil, nil
			}
			return nil, os.ErrNotExist
		}

		err := orch.MonitorGroupConsolidator(0, "inst-1", session, mockOrch, parser, nil)
		if err == nil || !contains(err.Error(), "consolidation failed") {
			t.Errorf("Expected consolidation failed error, got %v", err)
		}

		// Verify instance was stopped even on failure
		if len(mockOrch.stoppedInstances) != 1 {
			t.Errorf("Expected instance to be stopped on failure, got %d stopped", len(mockOrch.stoppedInstances))
		}
	})

	t.Run("returns error when context cancelled", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		// Cancel immediately
		orch.Cancel()

		session := newMockGroupConsolidationSession()
		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.instance = &mockInstanceForGroupConsolidation{
			id:           "inst-1",
			worktreePath: "/tmp/worktree",
			status:       StatusRunning,
		}

		parser := newMockTaskCompletionParser()

		err := orch.MonitorGroupConsolidator(0, "inst-1", session, mockOrch, parser, nil)
		if err == nil || !contains(err.Error(), "context cancelled") {
			t.Errorf("Expected context cancelled error, got %v", err)
		}
	})

	t.Run("returns error when instance status is Error", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.instance = &mockInstanceForGroupConsolidation{
			id:           "inst-1",
			worktreePath: "/tmp/worktree",
			status:       StatusError,
		}

		parser := newMockTaskCompletionParser()

		// Mock file doesn't exist
		statFile = func(path string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		}

		err := orch.MonitorGroupConsolidator(0, "inst-1", session, mockOrch, parser, nil)
		if err == nil || !contains(err.Error(), "failed with error") {
			t.Errorf("Expected instance failed error, got %v", err)
		}
	})

	t.Run("returns error when instance completes without completion file", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.instance = &mockInstanceForGroupConsolidation{
			id:           "inst-1",
			worktreePath: "/tmp/worktree",
			status:       StatusCompleted,
		}
		mockOrch.tmuxExists["inst-1"] = false // Tmux session doesn't exist

		parser := newMockTaskCompletionParser()

		// Mock file doesn't exist
		statFile = func(path string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		}

		err := orch.MonitorGroupConsolidator(0, "inst-1", session, mockOrch, parser, nil)
		if err == nil || !contains(err.Error(), "without writing completion file") {
			t.Errorf("Expected 'without writing completion file' error, got %v", err)
		}
	})

	t.Run("completes successfully when completion file indicates success", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManagerForConsolidation{},
			Orchestrator: &mockOrchestratorForConsolidation{},
			Session:      &mockSessionForConsolidation{},
		}
		orch := NewConsolidationOrchestrator(phaseCtx)

		session := newMockGroupConsolidationSession()
		mockOrch := newMockGroupConsolidationOrchestrator()
		mockOrch.instance = &mockInstanceForGroupConsolidation{
			id:           "inst-1",
			worktreePath: "/tmp/worktree",
			status:       StatusRunning,
		}

		parser := newMockTaskCompletionParser()
		parser.groupCompletions["/tmp/worktree"] = &GroupConsolidationCompletionFile{
			GroupIndex:        0,
			Status:            "complete",
			BranchName:        "consolidated-branch",
			TasksConsolidated: []string{"task-1", "task-2"},
			Verification:      VerificationResult{OverallSuccess: true},
		}

		emitter := &mockGroupConsolidationEventEmitter{}

		// Mock file exists
		statFile = func(path string) (os.FileInfo, error) {
			if path == "/tmp/worktree/"+GroupConsolidationCompletionFileName {
				return nil, nil
			}
			return nil, os.ErrNotExist
		}

		err := orch.MonitorGroupConsolidator(0, "inst-1", session, mockOrch, parser, emitter)
		if err != nil {
			t.Errorf("Expected success, got error: %v", err)
		}

		// Verify branch was stored in session
		if len(session.groupConsolidatedBranches) < 1 || session.groupConsolidatedBranches[0] != "consolidated-branch" {
			t.Error("Consolidated branch should be stored in session")
		}

		// Verify context was stored
		if len(session.groupConsolidationCtxs) < 1 || session.groupConsolidationCtxs[0] == nil {
			t.Error("Consolidation context should be stored in session")
		}

		// Verify instance was stopped
		if len(mockOrch.stoppedInstances) != 1 {
			t.Errorf("Expected instance to be stopped, got %d stopped", len(mockOrch.stoppedInstances))
		}

		// Verify session was saved
		if mockOrch.saveSessionCalls < 1 {
			t.Error("Session should be saved after completion")
		}

		// Verify completion event was emitted
		foundComplete := false
		for _, e := range emitter.events {
			if e.eventType == "group_consolidation_complete" {
				foundComplete = true
				break
			}
		}
		if !foundComplete {
			t.Error("Expected group_consolidation_complete event")
		}
	})
}
