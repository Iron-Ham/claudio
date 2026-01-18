package ralph

import (
	"errors"
	"testing"
)

// Mock implementations for coordinator tests

type mockOrchestrator struct {
	addInstanceFunc           func(session SessionInterface, task string) (InstanceInterface, error)
	addInstanceToWorktreeFunc func(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error)
	startInstanceFunc         func(inst InstanceInterface) error
	saveSessionFunc           func() error

	addInstanceCalls           int
	addInstanceToWorktreeCalls int
	startInstanceCalls         int
	saveSessionCalls           int
}

func newMockOrchestrator() *mockOrchestrator {
	return &mockOrchestrator{}
}

func (m *mockOrchestrator) AddInstance(session SessionInterface, task string) (InstanceInterface, error) {
	m.addInstanceCalls++
	if m.addInstanceFunc != nil {
		return m.addInstanceFunc(session, task)
	}
	return &mockInstance{id: "mock-inst-1", worktreePath: "/tmp/mock-worktree", branch: "mock-branch"}, nil
}

func (m *mockOrchestrator) AddInstanceToWorktree(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error) {
	m.addInstanceToWorktreeCalls++
	if m.addInstanceToWorktreeFunc != nil {
		return m.addInstanceToWorktreeFunc(session, task, worktreePath, branch)
	}
	return &mockInstance{id: "mock-inst-2", worktreePath: worktreePath, branch: "mock-branch"}, nil
}

func (m *mockOrchestrator) StartInstance(inst InstanceInterface) error {
	m.startInstanceCalls++
	if m.startInstanceFunc != nil {
		return m.startInstanceFunc(inst)
	}
	return nil
}

func (m *mockOrchestrator) SaveSession() error {
	m.saveSessionCalls++
	if m.saveSessionFunc != nil {
		return m.saveSessionFunc()
	}
	return nil
}

type mockBaseSession struct {
	groups    map[string]GroupInterface
	instances map[string]InstanceInterface
}

func newMockBaseSession() *mockBaseSession {
	return &mockBaseSession{
		groups:    make(map[string]GroupInterface),
		instances: make(map[string]InstanceInterface),
	}
}

func (m *mockBaseSession) GetGroup(id string) GroupInterface {
	return m.groups[id]
}

func (m *mockBaseSession) GetGroupBySessionType(sessionType string) GroupInterface {
	return m.groups[sessionType]
}

func (m *mockBaseSession) GetInstance(id string) InstanceInterface {
	return m.instances[id]
}

type mockInstance struct {
	id           string
	worktreePath string
	branch       string
}

func (m *mockInstance) GetID() string           { return m.id }
func (m *mockInstance) GetWorktreePath() string { return m.worktreePath }
func (m *mockInstance) GetBranch() string       { return m.branch }

type mockGroup struct {
	instances []string
}

func (m *mockGroup) AddInstance(instanceID string) {
	m.instances = append(m.instances, instanceID)
}

// Test cases

func TestNewCoordinator(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	orch := newMockOrchestrator()
	baseSession := newMockBaseSession()

	cfg := CoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  baseSession,
		RalphSession: session,
		Logger:       nil,
		SessionType:  "ralph",
	}

	coord := NewCoordinator(cfg)

	if coord == nil {
		t.Fatal("NewCoordinator returned nil")
	}
	if coord.Session() != session {
		t.Error("Session() should return the same session")
	}
}

func TestCoordinator_SetCallbacks(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	var iterationStartCalled bool
	callbacks := &CoordinatorCallbacks{
		OnIterationStart: func(iteration int, instanceID string) {
			iterationStartCalled = true
		},
	}

	coord.SetCallbacks(callbacks)

	// Trigger callback
	coord.notifyIterationStart(1, "test-inst")

	if !iterationStartCalled {
		t.Error("OnIterationStart callback was not called")
	}
}

func TestCoordinator_Session(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	got := coord.Session()

	if got != session {
		t.Error("Session() should return the correct session")
	}
}

func TestCoordinator_NotifyIterationStart(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	var receivedIteration int
	var receivedInstanceID string
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnIterationStart: func(iteration int, instanceID string) {
			receivedIteration = iteration
			receivedInstanceID = instanceID
		},
	})

	coord.notifyIterationStart(3, "inst-123")

	if receivedIteration != 3 {
		t.Errorf("callback received iteration = %d, want 3", receivedIteration)
	}
	if receivedInstanceID != "inst-123" {
		t.Errorf("callback received instanceID = %q, want %q", receivedInstanceID, "inst-123")
	}
}

func TestCoordinator_NotifyIterationComplete(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	var receivedIteration int
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnIterationComplete: func(iteration int) {
			receivedIteration = iteration
		},
	})

	coord.notifyIterationComplete(5)

	if receivedIteration != 5 {
		t.Errorf("callback received iteration = %d, want 5", receivedIteration)
	}
}

func TestCoordinator_NotifyPromiseFound(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	var promiseFoundCalled bool
	var receivedIteration int
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnPromiseFound: func(iteration int) {
			promiseFoundCalled = true
			receivedIteration = iteration
		},
	})

	coord.notifyPromiseFound(2)

	if !promiseFoundCalled {
		t.Error("OnPromiseFound callback should have been called")
	}
	if receivedIteration != 2 {
		t.Errorf("callback received iteration = %d, want 2", receivedIteration)
	}
}

func TestCoordinator_NotifyMaxIterations(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	var maxIterationsCalled bool
	var receivedIteration int
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnMaxIterations: func(iteration int) {
			maxIterationsCalled = true
			receivedIteration = iteration
		},
	})

	coord.notifyMaxIterations(50)

	if !maxIterationsCalled {
		t.Error("OnMaxIterations callback should have been called")
	}
	if receivedIteration != 50 {
		t.Errorf("callback received iteration = %d, want 50", receivedIteration)
	}
}

func TestCoordinator_NotifyComplete(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	var receivedPhase Phase
	var receivedSummary string
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnComplete: func(phase Phase, summary string) {
			receivedPhase = phase
			receivedSummary = summary
		},
	})

	coord.notifyComplete(PhaseComplete, "Loop completed successfully")

	if receivedPhase != PhaseComplete {
		t.Errorf("callback received phase = %q, want %q", receivedPhase, PhaseComplete)
	}
	if receivedSummary != "Loop completed successfully" {
		t.Errorf("callback received summary = %q, want %q", receivedSummary, "Loop completed successfully")
	}
}

func TestCoordinator_StartIteration_FirstIteration(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	orch := newMockOrchestrator()
	baseSession := newMockBaseSession()
	group := &mockGroup{}
	baseSession.groups["ralph"] = group

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  baseSession,
		RalphSession: session,
		SessionType:  "ralph",
	})

	err := coord.StartIteration()

	if err != nil {
		t.Fatalf("StartIteration() error = %v", err)
	}
	if orch.addInstanceCalls != 1 {
		t.Errorf("AddInstance called %d times, want 1", orch.addInstanceCalls)
	}
	if orch.startInstanceCalls != 1 {
		t.Errorf("StartInstance called %d times, want 1", orch.startInstanceCalls)
	}
	if session.CurrentIteration != 1 {
		t.Errorf("CurrentIteration = %d, want 1", session.CurrentIteration)
	}
	if session.InstanceID == "" {
		t.Error("session.InstanceID should be set")
	}
	if len(group.instances) != 1 {
		t.Errorf("group should have 1 instance, got %d", len(group.instances))
	}
}

func TestCoordinator_StartIteration_SubsequentIteration(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	session.CurrentIteration = 1 // Already completed first iteration
	orch := newMockOrchestrator()
	baseSession := newMockBaseSession()

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  baseSession,
		RalphSession: session,
		SessionType:  "ralph",
	})

	// Set worktree from previous iteration
	coord.SetWorktree("/tmp/existing-worktree")

	err := coord.StartIteration()

	if err != nil {
		t.Fatalf("StartIteration() error = %v", err)
	}
	if orch.addInstanceToWorktreeCalls != 1 {
		t.Errorf("AddInstanceToWorktree called %d times, want 1", orch.addInstanceToWorktreeCalls)
	}
	if session.CurrentIteration != 2 {
		t.Errorf("CurrentIteration = %d, want 2", session.CurrentIteration)
	}
}

func TestCoordinator_StartIteration_AddInstanceError(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	orch := newMockOrchestrator()
	orch.addInstanceFunc = func(session SessionInterface, task string) (InstanceInterface, error) {
		return nil, errors.New("failed to create instance")
	}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	err := coord.StartIteration()

	if err == nil {
		t.Error("StartIteration() should return error")
	}
	if session.Phase != PhaseError {
		t.Errorf("session.Phase = %q, want %q", session.Phase, PhaseError)
	}
}

func TestCoordinator_StartIteration_StartInstanceError(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	orch := newMockOrchestrator()
	orch.startInstanceFunc = func(inst InstanceInterface) error {
		return errors.New("failed to start instance")
	}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	err := coord.StartIteration()

	if err == nil {
		t.Error("StartIteration() should return error")
	}
	if session.Phase != PhaseError {
		t.Errorf("session.Phase = %q, want %q", session.Phase, PhaseError)
	}
}

func TestCoordinator_StartIteration_SessionNotWorking(t *testing.T) {
	tests := []struct {
		name    string
		phase   Phase
		wantErr string
	}{
		{"cancelled", PhaseCancelled, "cancelled"},
		{"complete", PhaseComplete, "complete"},
		{"max iterations", PhaseMaxIterations, "cannot continue"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewSession("test prompt", DefaultConfig())
			session.Phase = tt.phase

			coord := NewCoordinator(CoordinatorConfig{
				Orchestrator: newMockOrchestrator(),
				BaseSession:  newMockBaseSession(),
				RalphSession: session,
				SessionType:  "ralph",
			})

			err := coord.StartIteration()

			if err == nil {
				t.Error("StartIteration() should return error for non-working phase")
			}
		})
	}
}

func TestCoordinator_StartIteration_MaxIterationsReached(t *testing.T) {
	session := NewSession("test prompt", &Config{MaxIterations: 3})
	session.CurrentIteration = 3 // Already at max

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	err := coord.StartIteration()

	if err == nil {
		t.Error("StartIteration() should return error when max iterations reached")
	}
}

func TestCoordinator_CheckCompletionInOutput(t *testing.T) {
	session := NewSession("test prompt", &Config{CompletionPromise: "TASK_COMPLETE"})

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{"promise found", "Output with TASK_COMPLETE in it", true},
		{"promise not found", "Some other output", false},
		{"empty output", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coord.CheckCompletionInOutput(tt.output)
			if got != tt.want {
				t.Errorf("CheckCompletionInOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCoordinator_ProcessIterationCompletion_PromiseFound(t *testing.T) {
	session := NewSession("test prompt", &Config{CompletionPromise: "DONE", MaxIterations: 10})
	session.CurrentIteration = 3
	orch := newMockOrchestrator()

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	var completeCalled bool
	var completePhase Phase
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnIterationComplete: func(iteration int) {},
		OnPromiseFound:      func(iteration int) {},
		OnComplete: func(phase Phase, summary string) {
			completeCalled = true
			completePhase = phase
		},
	})

	continueLoop, err := coord.ProcessIterationCompletion("Some output with DONE in it")

	if err != nil {
		t.Fatalf("ProcessIterationCompletion() error = %v", err)
	}
	if continueLoop {
		t.Error("expected continueLoop = false when promise found")
	}
	if !completeCalled {
		t.Error("OnComplete callback should have been called")
	}
	if completePhase != PhaseComplete {
		t.Errorf("complete phase = %q, want %q", completePhase, PhaseComplete)
	}
	if session.Phase != PhaseComplete {
		t.Errorf("session.Phase = %q, want %q", session.Phase, PhaseComplete)
	}
}

func TestCoordinator_ProcessIterationCompletion_MaxIterationsReached(t *testing.T) {
	session := NewSession("test prompt", &Config{CompletionPromise: "DONE", MaxIterations: 3})
	session.CurrentIteration = 3
	orch := newMockOrchestrator()

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	var maxIterationsCalled bool
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnIterationComplete: func(iteration int) {},
		OnMaxIterations: func(iteration int) {
			maxIterationsCalled = true
		},
		OnComplete: func(phase Phase, summary string) {},
	})

	continueLoop, err := coord.ProcessIterationCompletion("Output without promise")

	if err != nil {
		t.Fatalf("ProcessIterationCompletion() error = %v", err)
	}
	if continueLoop {
		t.Error("expected continueLoop = false when max iterations reached")
	}
	if !maxIterationsCalled {
		t.Error("OnMaxIterations callback should have been called")
	}
	if session.Phase != PhaseMaxIterations {
		t.Errorf("session.Phase = %q, want %q", session.Phase, PhaseMaxIterations)
	}
}

func TestCoordinator_ProcessIterationCompletion_Continue(t *testing.T) {
	session := NewSession("test prompt", &Config{CompletionPromise: "DONE", MaxIterations: 10})
	session.CurrentIteration = 2
	orch := newMockOrchestrator()

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	coord.SetCallbacks(&CoordinatorCallbacks{
		OnIterationComplete: func(iteration int) {},
	})

	continueLoop, err := coord.ProcessIterationCompletion("Output without promise")

	if err != nil {
		t.Fatalf("ProcessIterationCompletion() error = %v", err)
	}
	if !continueLoop {
		t.Error("expected continueLoop = true when promise not found and iterations remaining")
	}
}

func TestCoordinator_Cancel(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	session.CurrentIteration = 5
	orch := newMockOrchestrator()

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	var completeCalled bool
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnComplete: func(phase Phase, summary string) {
			completeCalled = true
		},
	})

	coord.Cancel()

	if session.Phase != PhaseCancelled {
		t.Errorf("session.Phase = %q, want %q", session.Phase, PhaseCancelled)
	}
	if !completeCalled {
		t.Error("OnComplete callback should have been called")
	}
	if orch.saveSessionCalls == 0 {
		t.Error("SaveSession should have been called")
	}
}

func TestCoordinator_Stop(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	// Should not panic
	coord.Stop()
}

func TestCoordinator_GetWorktree(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	coord.SetWorktree("/tmp/test-worktree")

	got := coord.GetWorktree()

	if got != "/tmp/test-worktree" {
		t.Errorf("GetWorktree() = %q, want %q", got, "/tmp/test-worktree")
	}
}

func TestCoordinator_SetWorktree(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	coord.SetWorktree("/tmp/restored-worktree")

	if coord.GetWorktree() != "/tmp/restored-worktree" {
		t.Errorf("worktree = %q, want %q", coord.GetWorktree(), "/tmp/restored-worktree")
	}
}

func TestCoordinator_GetCurrentInstanceID(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	session.InstanceID = "test-inst-id"

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	got := coord.GetCurrentInstanceID()

	if got != "test-inst-id" {
		t.Errorf("GetCurrentInstanceID() = %q, want %q", got, "test-inst-id")
	}
}

func TestCoordinator_NilCallbacks(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: newMockOrchestrator(),
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	// Should not panic with nil callbacks
	coord.notifyIterationStart(1, "inst-1")
	coord.notifyIterationComplete(1)
	coord.notifyPromiseFound(1)
	coord.notifyMaxIterations(1)
	coord.notifyComplete(PhaseComplete, "test")
}

func TestCoordinator_StartIteration_WithGroupID(t *testing.T) {
	session := NewSession("test prompt", DefaultConfig())
	session.GroupID = "specific-group-id"

	orch := newMockOrchestrator()
	baseSession := newMockBaseSession()
	group := &mockGroup{}
	baseSession.groups["specific-group-id"] = group

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  baseSession,
		RalphSession: session,
		SessionType:  "ralph",
	})

	err := coord.StartIteration()

	if err != nil {
		t.Fatalf("StartIteration() error = %v", err)
	}
	if len(group.instances) != 1 {
		t.Errorf("group should have 1 instance, got %d", len(group.instances))
	}
}

func TestCoordinator_ProcessIterationCompletion_NoLimitIterations(t *testing.T) {
	session := NewSession("test prompt", &Config{CompletionPromise: "DONE", MaxIterations: 0}) // No limit
	session.CurrentIteration = 100                                                             // High iteration count
	orch := newMockOrchestrator()

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  newMockBaseSession(),
		RalphSession: session,
		SessionType:  "ralph",
	})

	coord.SetCallbacks(&CoordinatorCallbacks{
		OnIterationComplete: func(iteration int) {},
	})

	continueLoop, err := coord.ProcessIterationCompletion("Output without promise")

	if err != nil {
		t.Fatalf("ProcessIterationCompletion() error = %v", err)
	}
	if !continueLoop {
		t.Error("expected continueLoop = true when MaxIterations is 0 (no limit)")
	}
}
