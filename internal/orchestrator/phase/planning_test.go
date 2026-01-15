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
