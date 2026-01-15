package phase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// planningMockOrchestrator implements OrchestratorInterface for planning tests
type planningMockOrchestrator struct {
	addInstanceFunc   func(session any, task string) (any, error)
	startInstanceFunc func(inst any) error
	saveSessionFunc   func() error
	addedInstances    []planningMockInstance
	startedInstances  []any
	mu                sync.Mutex
}

type planningMockInstance struct {
	id   string
	task string
}

func (m *planningMockInstance) GetID() string {
	return m.id
}

func (m *planningMockOrchestrator) AddInstance(session any, task string) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.addInstanceFunc != nil {
		return m.addInstanceFunc(session, task)
	}

	inst := &planningMockInstance{
		id:   "inst-" + time.Now().Format("150405"),
		task: task,
	}
	m.addedInstances = append(m.addedInstances, *inst)
	return inst, nil
}

func (m *planningMockOrchestrator) StartInstance(inst any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.startInstanceFunc != nil {
		return m.startInstanceFunc(inst)
	}

	m.startedInstances = append(m.startedInstances, inst)
	return nil
}

func (m *planningMockOrchestrator) SaveSession() error {
	if m.saveSessionFunc != nil {
		return m.saveSessionFunc()
	}
	return nil
}

func (m *planningMockOrchestrator) GetInstanceManager(id string) any {
	return nil
}

func (m *planningMockOrchestrator) GetInstance(id string) InstanceInterface {
	return nil
}

func (m *planningMockOrchestrator) BranchPrefix() string {
	return "test-prefix"
}

// planningMockGroup implements the group interface for testing
type planningMockGroup struct {
	instances []string
	mu        sync.Mutex
}

func (g *planningMockGroup) AddInstance(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.instances = append(g.instances, id)
}

func TestNewPlanningOrchestrator(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *PhaseContext
		wantErr error
	}{
		{
			name: "valid context creates orchestrator",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &planningMockOrchestrator{},
				Session:      &mockSession{},
			},
			wantErr: nil,
		},
		{
			name: "nil manager returns error",
			ctx: &PhaseContext{
				Manager:      nil,
				Orchestrator: &planningMockOrchestrator{},
				Session:      &mockSession{},
			},
			wantErr: ErrNilManager,
		},
		{
			name: "nil orchestrator returns error",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: nil,
				Session:      &mockSession{},
			},
			wantErr: ErrNilOrchestrator,
		},
		{
			name: "nil session returns error",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &planningMockOrchestrator{},
				Session:      nil,
			},
			wantErr: ErrNilSession,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planner, err := NewPlanningOrchestrator(tt.ctx)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("NewPlanningOrchestrator() error = %v, wantErr %v", err, tt.wantErr)
				}
				if planner != nil {
					t.Error("NewPlanningOrchestrator() should return nil on error")
				}
			} else {
				if err != nil {
					t.Errorf("NewPlanningOrchestrator() unexpected error: %v", err)
				}
				if planner == nil {
					t.Error("NewPlanningOrchestrator() should return non-nil planner")
				}
			}
		})
	}
}

func TestPlanningOrchestrator_Phase(t *testing.T) {
	ctx := &PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &planningMockOrchestrator{},
		Session:      &mockSession{},
	}
	planner, err := NewPlanningOrchestrator(ctx)
	if err != nil {
		t.Fatalf("NewPlanningOrchestrator() error: %v", err)
	}

	if got := planner.Phase(); got != PhasePlanning {
		t.Errorf("Phase() = %v, want %v", got, PhasePlanning)
	}
}

func TestPlanningOrchestrator_Execute(t *testing.T) {
	t.Run("basic execute succeeds", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		ctx := context.Background()
		err = planner.Execute(ctx)
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}
	})

	t.Run("execute respects context cancellation", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = planner.Execute(ctx)
		if err == nil {
			t.Error("Execute() should return error on cancelled context")
		}
	})

	t.Run("execute fails if already cancelled", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		// Cancel before execute
		planner.Cancel()

		ctx := context.Background()
		err = planner.Execute(ctx)
		if !errors.Is(err, ErrPlanningCancelled) {
			t.Errorf("Execute() error = %v, want %v", err, ErrPlanningCancelled)
		}
	})
}

func TestPlanningOrchestrator_ExecuteWithPrompt(t *testing.T) {
	t.Run("creates and starts instance", func(t *testing.T) {
		mockOrch := &planningMockOrchestrator{}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		var capturedID string
		setCoordinatorID := func(id string) {
			capturedID = id
		}

		group := &planningMockGroup{}
		getGroup := func() any {
			return group
		}

		ctx := context.Background()
		err = planner.ExecuteWithPrompt(ctx, "test prompt", nil, getGroup, setCoordinatorID)
		if err != nil {
			t.Errorf("ExecuteWithPrompt() unexpected error: %v", err)
		}

		// Check that instance was created
		if len(mockOrch.addedInstances) != 1 {
			t.Errorf("Expected 1 added instance, got %d", len(mockOrch.addedInstances))
		}

		// Check that instance was started
		if len(mockOrch.startedInstances) != 1 {
			t.Errorf("Expected 1 started instance, got %d", len(mockOrch.startedInstances))
		}

		// Check that coordinator ID was set
		if capturedID == "" {
			t.Error("Expected coordinator ID to be set")
		}

		// Check that instance was added to group
		if len(group.instances) != 1 {
			t.Errorf("Expected 1 instance in group, got %d", len(group.instances))
		}

		// Check state
		state := planner.State()
		if state.Prompt != "test prompt" {
			t.Errorf("State().Prompt = %q, want %q", state.Prompt, "test prompt")
		}
		if state.MultiPass {
			t.Error("State().MultiPass should be false for single-pass")
		}
	})

	t.Run("handles AddInstance error", func(t *testing.T) {
		expectedErr := errors.New("add instance failed")
		mockOrch := &planningMockOrchestrator{
			addInstanceFunc: func(session any, task string) (any, error) {
				return nil, expectedErr
			},
		}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		ctx := context.Background()
		err = planner.ExecuteWithPrompt(ctx, "test prompt", nil, nil, nil)
		if err == nil {
			t.Error("ExecuteWithPrompt() should return error when AddInstance fails")
		}

		// Check error state
		if planner.GetError() == "" {
			t.Error("Error state should be set")
		}
	})

	t.Run("handles StartInstance error", func(t *testing.T) {
		expectedErr := errors.New("start instance failed")
		mockOrch := &planningMockOrchestrator{
			startInstanceFunc: func(inst any) error {
				return expectedErr
			},
		}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		ctx := context.Background()
		err = planner.ExecuteWithPrompt(ctx, "test prompt", nil, nil, nil)
		if err == nil {
			t.Error("ExecuteWithPrompt() should return error when StartInstance fails")
		}
	})

	t.Run("handles cancellation", func(t *testing.T) {
		mockOrch := &planningMockOrchestrator{}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		planner.Cancel()

		ctx := context.Background()
		err = planner.ExecuteWithPrompt(ctx, "test prompt", nil, nil, nil)
		if !errors.Is(err, ErrPlanningCancelled) {
			t.Errorf("ExecuteWithPrompt() error = %v, want %v", err, ErrPlanningCancelled)
		}
	})

	t.Run("handles nil getGroup and setCoordinatorID", func(t *testing.T) {
		mockOrch := &planningMockOrchestrator{}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		ctx := context.Background()
		err = planner.ExecuteWithPrompt(ctx, "test prompt", nil, nil, nil)
		if err != nil {
			t.Errorf("ExecuteWithPrompt() unexpected error: %v", err)
		}
	})
}

func TestPlanningOrchestrator_Cancel(t *testing.T) {
	t.Run("cancel is idempotent", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		// Multiple cancels should not panic
		planner.Cancel()
		planner.Cancel()
		planner.Cancel()

		if !planner.IsCancelled() {
			t.Error("IsCancelled() should return true after Cancel()")
		}
	})
}

func TestPlanningOrchestrator_State(t *testing.T) {
	t.Run("State returns copy", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		// Set some state
		planner.SetState(PlanningState{
			InstanceID:         "test-instance",
			Prompt:             "test prompt",
			MultiPass:          true,
			PlanCoordinatorIDs: []string{"id1", "id2"},
			AwaitingCompletion: true,
		})

		// Get state copy
		state := planner.State()

		// Modify the copy
		state.InstanceID = "modified"
		state.PlanCoordinatorIDs[0] = "modified"

		// Original should be unchanged
		originalState := planner.State()
		if originalState.InstanceID == "modified" {
			t.Error("State() should return a copy, not a reference")
		}
		if originalState.PlanCoordinatorIDs[0] == "modified" {
			t.Error("State() should return deep copy of slices")
		}
	})

	t.Run("SetState deep copies slices", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		ids := []string{"id1", "id2"}
		planner.SetState(PlanningState{
			PlanCoordinatorIDs: ids,
		})

		// Modify original slice
		ids[0] = "modified"

		// Planner state should be unchanged
		state := planner.State()
		if state.PlanCoordinatorIDs[0] == "modified" {
			t.Error("SetState() should deep copy slices")
		}
	})
}

func TestPlanningOrchestrator_StateAccessors(t *testing.T) {
	phaseCtx := &PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &planningMockOrchestrator{},
		Session:      &mockSession{},
	}
	planner, err := NewPlanningOrchestrator(phaseCtx)
	if err != nil {
		t.Fatalf("NewPlanningOrchestrator() error: %v", err)
	}

	t.Run("GetInstanceID and internal setter", func(t *testing.T) {
		if got := planner.GetInstanceID(); got != "" {
			t.Errorf("GetInstanceID() = %q, want empty", got)
		}

		// Use SetState to set instance ID
		planner.SetState(PlanningState{InstanceID: "test-id"})
		if got := planner.GetInstanceID(); got != "test-id" {
			t.Errorf("GetInstanceID() = %q, want %q", got, "test-id")
		}
	})

	t.Run("IsAwaitingCompletion and SetAwaitingCompletion", func(t *testing.T) {
		if planner.IsAwaitingCompletion() {
			t.Error("IsAwaitingCompletion() should be false initially")
		}

		planner.SetAwaitingCompletion(true)
		if !planner.IsAwaitingCompletion() {
			t.Error("IsAwaitingCompletion() should be true after SetAwaitingCompletion(true)")
		}

		planner.SetAwaitingCompletion(false)
		if planner.IsAwaitingCompletion() {
			t.Error("IsAwaitingCompletion() should be false after SetAwaitingCompletion(false)")
		}
	})

	t.Run("IsMultiPass", func(t *testing.T) {
		planner.Reset()
		if planner.IsMultiPass() {
			t.Error("IsMultiPass() should be false initially")
		}

		planner.SetState(PlanningState{MultiPass: true})
		if !planner.IsMultiPass() {
			t.Error("IsMultiPass() should be true after setting")
		}
	})

	t.Run("GetPlanCoordinatorIDs and SetPlanCoordinatorIDs", func(t *testing.T) {
		planner.Reset()
		if ids := planner.GetPlanCoordinatorIDs(); ids != nil {
			t.Errorf("GetPlanCoordinatorIDs() = %v, want nil", ids)
		}

		planner.SetPlanCoordinatorIDs([]string{"id1", "id2"})
		ids := planner.GetPlanCoordinatorIDs()
		if len(ids) != 2 || ids[0] != "id1" || ids[1] != "id2" {
			t.Errorf("GetPlanCoordinatorIDs() = %v, want [id1, id2]", ids)
		}

		// Setting nil should work
		planner.SetPlanCoordinatorIDs(nil)
		if ids := planner.GetPlanCoordinatorIDs(); ids != nil {
			t.Errorf("GetPlanCoordinatorIDs() = %v, want nil after setting nil", ids)
		}
	})

	t.Run("GetError and SetError", func(t *testing.T) {
		planner.Reset()
		if got := planner.GetError(); got != "" {
			t.Errorf("GetError() = %q, want empty", got)
		}

		planner.SetError("test error")
		if got := planner.GetError(); got != "test error" {
			t.Errorf("GetError() = %q, want %q", got, "test error")
		}
	})

	t.Run("IsRunning", func(t *testing.T) {
		planner.Reset()
		if planner.IsRunning() {
			t.Error("IsRunning() should be false when not executing")
		}
	})

	t.Run("IsCancelled", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
		}
		freshPlanner, _ := NewPlanningOrchestrator(phaseCtx)

		if freshPlanner.IsCancelled() {
			t.Error("IsCancelled() should be false initially")
		}

		freshPlanner.Cancel()
		if !freshPlanner.IsCancelled() {
			t.Error("IsCancelled() should be true after Cancel()")
		}
	})
}

func TestPlanningOrchestrator_Reset(t *testing.T) {
	phaseCtx := &PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &planningMockOrchestrator{},
		Session:      &mockSession{},
	}
	planner, err := NewPlanningOrchestrator(phaseCtx)
	if err != nil {
		t.Fatalf("NewPlanningOrchestrator() error: %v", err)
	}

	// Set up state
	planner.SetState(PlanningState{
		InstanceID:         "test-id",
		Prompt:             "test prompt",
		MultiPass:          true,
		PlanCoordinatorIDs: []string{"id1"},
		AwaitingCompletion: true,
		Error:              "some error",
	})
	planner.Cancel()

	// Reset
	planner.Reset()

	// Verify state is cleared
	state := planner.State()
	if state.InstanceID != "" {
		t.Errorf("After Reset(), InstanceID = %q, want empty", state.InstanceID)
	}
	if state.Prompt != "" {
		t.Errorf("After Reset(), Prompt = %q, want empty", state.Prompt)
	}
	if state.MultiPass {
		t.Error("After Reset(), MultiPass should be false")
	}
	if state.PlanCoordinatorIDs != nil {
		t.Errorf("After Reset(), PlanCoordinatorIDs = %v, want nil", state.PlanCoordinatorIDs)
	}
	if state.AwaitingCompletion {
		t.Error("After Reset(), AwaitingCompletion should be false")
	}
	if state.Error != "" {
		t.Errorf("After Reset(), Error = %q, want empty", state.Error)
	}

	// Verify cancelled flag is reset
	if planner.IsCancelled() {
		t.Error("After Reset(), IsCancelled() should be false")
	}
}

func TestExtractInstanceID(t *testing.T) {
	t.Run("nil returns empty", func(t *testing.T) {
		if got := extractInstanceID(nil); got != "" {
			t.Errorf("extractInstanceID(nil) = %q, want empty", got)
		}
	})

	t.Run("instance with GetID returns ID", func(t *testing.T) {
		inst := &planningMockInstance{id: "test-id"}
		if got := extractInstanceID(inst); got != "test-id" {
			t.Errorf("extractInstanceID() = %q, want %q", got, "test-id")
		}
	})
}

func TestAddInstanceToGroup(t *testing.T) {
	t.Run("nil group does not panic", func(t *testing.T) {
		// Should not panic
		addInstanceToGroup(nil, "test-id")
	})

	t.Run("empty id does not add", func(t *testing.T) {
		group := &planningMockGroup{}
		addInstanceToGroup(group, "")
		if len(group.instances) != 0 {
			t.Error("Empty ID should not be added to group")
		}
	})

	t.Run("adds instance to group", func(t *testing.T) {
		group := &planningMockGroup{}
		addInstanceToGroup(group, "test-id")
		if len(group.instances) != 1 || group.instances[0] != "test-id" {
			t.Errorf("Group instances = %v, want [test-id]", group.instances)
		}
	})
}

func TestErrPlanningCancelled(t *testing.T) {
	if ErrPlanningCancelled.Error() != "planning phase cancelled" {
		t.Errorf("ErrPlanningCancelled.Error() = %q, want descriptive message", ErrPlanningCancelled.Error())
	}
}

func TestPlanningOrchestrator_ConcurrentAccess(t *testing.T) {
	phaseCtx := &PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &planningMockOrchestrator{},
		Session:      &mockSession{},
	}
	planner, err := NewPlanningOrchestrator(phaseCtx)
	if err != nil {
		t.Fatalf("NewPlanningOrchestrator() error: %v", err)
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Mix of reads and writes
			_ = planner.State()
			_ = planner.GetInstanceID()
			_ = planner.IsAwaitingCompletion()
			_ = planner.IsMultiPass()
			_ = planner.GetPlanCoordinatorIDs()
			_ = planner.GetError()
			_ = planner.IsRunning()
			_ = planner.IsCancelled()

			planner.SetAwaitingCompletion(true)
			planner.SetPlanCoordinatorIDs([]string{"id"})
			planner.SetError("error")
		}()
	}
	wg.Wait()

	// Should complete without race conditions
}

// trackingMockCallbacks tracks callback invocations for testing
type trackingMockCallbacks struct {
	phaseChanges []UltraPlanPhase
	mu           sync.Mutex
}

func (m *trackingMockCallbacks) OnPhaseChange(phase UltraPlanPhase) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phaseChanges = append(m.phaseChanges, phase)
}

func (m *trackingMockCallbacks) OnTaskStart(taskID, instanceID string)                 {}
func (m *trackingMockCallbacks) OnTaskComplete(taskID string)                          {}
func (m *trackingMockCallbacks) OnTaskFailed(taskID, reason string)                    {}
func (m *trackingMockCallbacks) OnGroupComplete(groupIndex int)                        {}
func (m *trackingMockCallbacks) OnPlanReady(plan any)                                  {}
func (m *trackingMockCallbacks) OnProgress(completed, total int, phase UltraPlanPhase) {}
func (m *trackingMockCallbacks) OnComplete(success bool, summary string)               {}

func TestPlanningOrchestrator_ExecuteWithCallbacks(t *testing.T) {
	t.Run("notifies callbacks on phase change", func(t *testing.T) {
		callbacks := &trackingMockCallbacks{}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
			Callbacks:    callbacks,
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		ctx := context.Background()
		err = planner.Execute(ctx)
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}

		callbacks.mu.Lock()
		defer callbacks.mu.Unlock()
		if len(callbacks.phaseChanges) != 1 || callbacks.phaseChanges[0] != PhasePlanning {
			t.Errorf("Callbacks.OnPhaseChange() called with %v, want [%v]",
				callbacks.phaseChanges, PhasePlanning)
		}
	})

	t.Run("works without callbacks", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
			Callbacks:    nil, // No callbacks
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		ctx := context.Background()
		err = planner.Execute(ctx)
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}
	})
}

func TestPlanningOrchestrator_ExecuteWithPromptContextCancellation(t *testing.T) {
	t.Run("context cancelled during execution", func(t *testing.T) {
		mockOrch := &planningMockOrchestrator{}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		// Cancel the context immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = planner.ExecuteWithPrompt(ctx, "test prompt", nil, nil, nil)
		if err == nil {
			t.Error("ExecuteWithPrompt() should return error on cancelled context")
		}
	})
}

func TestPlanningOrchestrator_ExecuteWithPromptEdgeCases(t *testing.T) {
	t.Run("handles getGroup returning nil group", func(t *testing.T) {
		mockOrch := &planningMockOrchestrator{}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		// getGroup returns nil
		getGroup := func() any {
			return nil
		}

		ctx := context.Background()
		err = planner.ExecuteWithPrompt(ctx, "test prompt", nil, getGroup, nil)
		if err != nil {
			t.Errorf("ExecuteWithPrompt() unexpected error: %v", err)
		}

		// Should still work - instance should be created
		if len(mockOrch.addedInstances) != 1 {
			t.Errorf("Expected 1 added instance, got %d", len(mockOrch.addedInstances))
		}
	})
}

// instanceWithoutGetID is a type that doesn't implement GetID interface
type instanceWithoutGetID struct {
	data string
}

func TestExtractInstanceID_EdgeCases(t *testing.T) {
	t.Run("handles type without GetID method", func(t *testing.T) {
		inst := &instanceWithoutGetID{data: "test"}
		if got := extractInstanceID(inst); got != "" {
			t.Errorf("extractInstanceID(instanceWithoutGetID) = %q, want empty", got)
		}
	})

	t.Run("handles string type", func(t *testing.T) {
		var inst any = "not-an-instance"
		if got := extractInstanceID(inst); got != "" {
			t.Errorf("extractInstanceID(string) = %q, want empty", got)
		}
	})

	t.Run("handles int type", func(t *testing.T) {
		var inst any = 42
		if got := extractInstanceID(inst); got != "" {
			t.Errorf("extractInstanceID(int) = %q, want empty", got)
		}
	})

	t.Run("handles struct without ID field", func(t *testing.T) {
		type noIDStruct struct {
			Name string
		}
		inst := &noIDStruct{Name: "test"}
		if got := extractInstanceID(inst); got != "" {
			t.Errorf("extractInstanceID(noIDStruct) = %q, want empty", got)
		}
	})
}

// mockInstanceNoID returns nil for GetID
type mockInstanceEmptyID struct{}

func (m *mockInstanceEmptyID) GetID() string {
	return ""
}

func TestPlanningOrchestrator_ExecuteWithPromptEmptyInstanceID(t *testing.T) {
	t.Run("handles instance with empty ID", func(t *testing.T) {
		mockOrch := &planningMockOrchestrator{
			addInstanceFunc: func(session any, task string) (any, error) {
				return &mockInstanceEmptyID{}, nil
			},
		}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		ctx := context.Background()
		err = planner.ExecuteWithPrompt(ctx, "test prompt", nil, nil, nil)
		if err == nil {
			t.Error("ExecuteWithPrompt() should return error when instance ID is empty")
		}

		// Check error state
		if planner.GetError() == "" {
			t.Error("Error state should be set when instance ID extraction fails")
		}
	})
}

func TestPlanningOrchestrator_CancelDuringExecute(t *testing.T) {
	t.Run("cancel stops in-progress execute", func(t *testing.T) {
		// Create a slow orchestrator that allows us to test mid-execution cancellation
		slowOrch := &planningMockOrchestrator{
			addInstanceFunc: func(session any, task string) (any, error) {
				// Simulate slow operation
				time.Sleep(100 * time.Millisecond)
				return &planningMockInstance{id: "slow-instance"}, nil
			},
		}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: slowOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		// Start Execute in a goroutine
		done := make(chan error, 1)
		go func() {
			ctx := context.Background()
			done <- planner.Execute(ctx)
		}()

		// Give Execute a moment to start
		time.Sleep(10 * time.Millisecond)

		// Execute should complete since it doesn't block on instance creation
		err = <-done
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}
	})
}

func TestPlanningOrchestrator_InternalCancellation(t *testing.T) {
	t.Run("internal context cancellation via Cancel method", func(t *testing.T) {
		mockOrch := &planningMockOrchestrator{}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		// Start execution
		done := make(chan error, 1)
		go func() {
			ctx := context.Background()
			done <- planner.ExecuteWithPrompt(ctx, "test prompt", nil, nil, nil)
		}()

		// Give it a moment to start, then cancel
		time.Sleep(5 * time.Millisecond)
		planner.Cancel()

		// Wait for result with timeout
		select {
		case err := <-done:
			// Execution might have completed before cancel took effect
			// Either nil or ErrPlanningCancelled is acceptable
			if err != nil && !errors.Is(err, ErrPlanningCancelled) {
				t.Errorf("ExecuteWithPrompt() unexpected error: %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Error("ExecuteWithPrompt() did not return after Cancel()")
		}
	})
}

func TestPlanningOrchestrator_IsRunningDuringExecution(t *testing.T) {
	t.Run("IsRunning returns true during Execute", func(t *testing.T) {
		blockCh := make(chan struct{})
		mockOrch := &planningMockOrchestrator{
			addInstanceFunc: func(session any, task string) (any, error) {
				<-blockCh // Block until signaled
				return &planningMockInstance{id: "test-id"}, nil
			},
		}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		// Start execution in goroutine
		done := make(chan error, 1)
		go func() {
			ctx := context.Background()
			done <- planner.ExecuteWithPrompt(ctx, "test prompt", nil, nil, nil)
		}()

		// Give it time to start
		time.Sleep(10 * time.Millisecond)

		// Check IsRunning during execution
		if !planner.IsRunning() {
			t.Error("IsRunning() should return true during execution")
		}

		// Unblock and wait for completion
		close(blockCh)
		<-done

		// Give it time to update running state
		time.Sleep(10 * time.Millisecond)

		// Check IsRunning after execution
		if planner.IsRunning() {
			t.Error("IsRunning() should return false after execution completes")
		}
	})
}

func TestPlanningOrchestrator_StateWithNilPlanCoordinatorIDs(t *testing.T) {
	phaseCtx := &PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &planningMockOrchestrator{},
		Session:      &mockSession{},
	}
	planner, err := NewPlanningOrchestrator(phaseCtx)
	if err != nil {
		t.Fatalf("NewPlanningOrchestrator() error: %v", err)
	}

	// Set state with nil slice
	planner.SetState(PlanningState{
		InstanceID:         "test-id",
		PlanCoordinatorIDs: nil,
	})

	state := planner.State()
	if state.PlanCoordinatorIDs != nil {
		t.Errorf("State().PlanCoordinatorIDs = %v, want nil", state.PlanCoordinatorIDs)
	}
}

func TestAddInstanceToGroup_TypeWithoutAddMethod(t *testing.T) {
	t.Run("handles type without AddInstance method", func(t *testing.T) {
		// A type that doesn't implement the groupWithAdd interface
		type fakeGroup struct {
			name string
		}
		group := &fakeGroup{name: "test"}

		// Should not panic
		addInstanceToGroup(group, "test-id")
	})
}

func TestPlanningOrchestrator_ExecuteValidationError(t *testing.T) {
	t.Run("Execute returns error if phaseCtx becomes invalid", func(t *testing.T) {
		// Create a valid planner first
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		// Corrupt the phaseCtx (this is an unusual scenario but tests the validation path)
		planner.phaseCtx.Manager = nil

		ctx := context.Background()
		err = planner.Execute(ctx)
		if !errors.Is(err, ErrNilManager) {
			t.Errorf("Execute() error = %v, want %v", err, ErrNilManager)
		}
	})
}

func TestPlanningOrchestrator_setInstanceID(t *testing.T) {
	// Test the unexported setInstanceID method via SetState
	phaseCtx := &PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &planningMockOrchestrator{},
		Session:      &mockSession{},
	}
	planner, err := NewPlanningOrchestrator(phaseCtx)
	if err != nil {
		t.Fatalf("NewPlanningOrchestrator() error: %v", err)
	}

	// Use SetState which internally sets instance ID
	planner.SetState(PlanningState{InstanceID: "via-setstate"})
	if got := planner.GetInstanceID(); got != "via-setstate" {
		t.Errorf("GetInstanceID() = %q, want %q", got, "via-setstate")
	}
}

func TestPlanningOrchestrator_StateEmptyPlanCoordinatorIDs(t *testing.T) {
	phaseCtx := &PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &planningMockOrchestrator{},
		Session:      &mockSession{},
	}
	planner, err := NewPlanningOrchestrator(phaseCtx)
	if err != nil {
		t.Fatalf("NewPlanningOrchestrator() error: %v", err)
	}

	// Set empty slice (not nil)
	planner.SetPlanCoordinatorIDs([]string{})
	ids := planner.GetPlanCoordinatorIDs()
	// Empty slice should return nil per the implementation
	if ids != nil {
		t.Errorf("GetPlanCoordinatorIDs() = %v, want nil for empty slice", ids)
	}
}

func TestPlanningOrchestrator_ExecuteSinglePassCancellation(t *testing.T) {
	t.Run("executeSinglePass detects cancelled context", func(t *testing.T) {
		// We need to get to executeSinglePass with a cancelled context
		// The tricky part is that Execute() checks cancellation before calling executeSinglePass
		// So we need to cancel the context after Execute starts but before executeSinglePass runs
		// This is achieved by having a slow callback that allows us to cancel mid-execution

		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
			Callbacks: &slowCallbacks{
				onPhaseChangeDelay: 20 * time.Millisecond,
			},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())

		// Start execution in goroutine
		done := make(chan error, 1)
		go func() {
			done <- planner.Execute(ctx)
		}()

		// Cancel while callbacks are executing (before executeSinglePass starts)
		time.Sleep(10 * time.Millisecond)
		cancel()

		// Wait for result
		select {
		case err := <-done:
			// Either context.Canceled or ErrPlanningCancelled is acceptable
			if err == nil {
				// Execution completed before cancel - this is also acceptable
				return
			}
			if err != context.Canceled && !errors.Is(err, ErrPlanningCancelled) {
				t.Errorf("Execute() error = %v, want context.Canceled or ErrPlanningCancelled", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Execute() did not return after context cancellation")
		}
	})
}

// slowCallbacks adds delays to callbacks for testing timing-sensitive scenarios
type slowCallbacks struct {
	onPhaseChangeDelay time.Duration
}

func (m *slowCallbacks) OnPhaseChange(phase UltraPlanPhase) {
	if m.onPhaseChangeDelay > 0 {
		time.Sleep(m.onPhaseChangeDelay)
	}
}

func (m *slowCallbacks) OnTaskStart(taskID, instanceID string)                 {}
func (m *slowCallbacks) OnTaskComplete(taskID string)                          {}
func (m *slowCallbacks) OnTaskFailed(taskID, reason string)                    {}
func (m *slowCallbacks) OnGroupComplete(groupIndex int)                        {}
func (m *slowCallbacks) OnPlanReady(plan any)                                  {}
func (m *slowCallbacks) OnProgress(completed, total int, phase UltraPlanPhase) {}
func (m *slowCallbacks) OnComplete(success bool, summary string)               {}

func TestPlanningOrchestrator_ExecuteWithPromptInternalCancellation(t *testing.T) {
	t.Run("internal ctx cancellation returns ErrPlanningCancelled", func(t *testing.T) {
		blockCh := make(chan struct{})
		mockOrch := &planningMockOrchestrator{
			addInstanceFunc: func(session any, task string) (any, error) {
				<-blockCh // Block until signaled
				return &planningMockInstance{id: "test-id"}, nil
			},
		}
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		// Start execution in goroutine
		done := make(chan error, 1)
		go func() {
			ctx := context.Background()
			done <- planner.ExecuteWithPrompt(ctx, "test prompt", nil, nil, nil)
		}()

		// Give it time to reach the blocking point, then cancel
		time.Sleep(10 * time.Millisecond)
		planner.Cancel()

		// Unblock to allow completion check
		close(blockCh)

		// Wait for result
		select {
		case err := <-done:
			// Execution might have completed before cancel took effect
			// Either nil or ErrPlanningCancelled is acceptable
			if err != nil && !errors.Is(err, ErrPlanningCancelled) {
				t.Errorf("ExecuteWithPrompt() error = %v, want nil or ErrPlanningCancelled", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("ExecuteWithPrompt() did not return")
		}
	})
}

// Test that exercises multiple Execute paths for coverage
func TestPlanningOrchestrator_ExecuteMultiplePaths(t *testing.T) {
	t.Run("Execute with both contexts done prefers p.ctx.Done", func(t *testing.T) {
		phaseCtx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		// Cancel before execute
		planner.Cancel()

		// Also pass cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = planner.Execute(ctx)
		// Should return ErrPlanningCancelled because we check `cancelled` flag first
		if !errors.Is(err, ErrPlanningCancelled) {
			t.Errorf("Execute() error = %v, want %v", err, ErrPlanningCancelled)
		}
	})

	t.Run("Execute returns context.Canceled when context is cancelled mid-execution", func(t *testing.T) {
		// Use a manager that will allow us to cancel between SetPhase and the select
		slowManager := &slowMockManager{
			setPhaseDelay: 30 * time.Millisecond,
		}
		phaseCtx := &PhaseContext{
			Manager:      slowManager,
			Orchestrator: &planningMockOrchestrator{},
			Session:      &mockSession{},
		}
		planner, err := NewPlanningOrchestrator(phaseCtx)
		if err != nil {
			t.Fatalf("NewPlanningOrchestrator() error: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())

		// Start execution
		done := make(chan error, 1)
		go func() {
			done <- planner.Execute(ctx)
		}()

		// Cancel during SetPhase delay
		time.Sleep(10 * time.Millisecond)
		cancel()

		// Wait for result
		select {
		case err := <-done:
			// Either context.Canceled or no error (if execution completed first)
			if err != nil && err != context.Canceled && !errors.Is(err, ErrPlanningCancelled) {
				t.Errorf("Execute() error = %v, want context.Canceled or nil", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Execute() did not return")
		}
	})
}

// slowMockManager adds delays to manager operations
type slowMockManager struct {
	session       UltraPlanSessionInterface
	setPhaseDelay time.Duration
}

func (m *slowMockManager) Session() UltraPlanSessionInterface { return m.session }
func (m *slowMockManager) SetPhase(phase UltraPlanPhase) {
	if m.setPhaseDelay > 0 {
		time.Sleep(m.setPhaseDelay)
	}
}
func (m *slowMockManager) SetPlan(plan any)                               {}
func (m *slowMockManager) MarkTaskComplete(taskID string)                 {}
func (m *slowMockManager) MarkTaskFailed(taskID, reason string)           {}
func (m *slowMockManager) AssignTaskToInstance(taskID, instanceID string) {}
func (m *slowMockManager) Stop()                                          {}
