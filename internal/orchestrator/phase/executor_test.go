package phase

import (
	"context"
	"errors"
	"testing"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// mockManager implements UltraPlanManagerInterface for testing
type mockManager struct {
	session UltraPlanSessionInterface
}

func (m *mockManager) Session() UltraPlanSessionInterface             { return m.session }
func (m *mockManager) SetPhase(phase UltraPlanPhase)                  {}
func (m *mockManager) SetPlan(plan any)                               {}
func (m *mockManager) MarkTaskComplete(taskID string)                 {}
func (m *mockManager) MarkTaskFailed(taskID, reason string)           {}
func (m *mockManager) AssignTaskToInstance(taskID, instanceID string) {}
func (m *mockManager) Stop()                                          {}

// mockOrchestrator implements OrchestratorInterface for testing
type mockOrchestrator struct {
	instance InstanceInterface
}

func (m *mockOrchestrator) AddInstance(session any, task string) (any, error) { return nil, nil }
func (m *mockOrchestrator) StartInstance(inst any) error                      { return nil }
func (m *mockOrchestrator) SaveSession() error                                { return nil }
func (m *mockOrchestrator) GetInstanceManager(id string) any                  { return nil }
func (m *mockOrchestrator) GetInstance(id string) InstanceInterface           { return m.instance }
func (m *mockOrchestrator) BranchPrefix() string                              { return "test" }

// mockSession implements UltraPlanSessionInterface for testing
type mockSession struct {
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

func (m *mockSession) GetTask(taskID string) any                  { return nil }
func (m *mockSession) GetReadyTasks() []string                    { return nil }
func (m *mockSession) IsCurrentGroupComplete() bool               { return false }
func (m *mockSession) AdvanceGroupIfComplete() (bool, int)        { return false, 0 }
func (m *mockSession) HasMoreGroups() bool                        { return false }
func (m *mockSession) Progress() float64                          { return 0 }
func (m *mockSession) GetObjective() string                       { return m.objective }
func (m *mockSession) GetCompletedTasks() []string                { return m.completedTasks }
func (m *mockSession) GetTaskToInstance() map[string]string       { return m.taskToInstance }
func (m *mockSession) GetTaskCommitCounts() map[string]int        { return m.taskCommitCounts }
func (m *mockSession) GetSynthesisID() string                     { return m.synthesisID }
func (m *mockSession) SetSynthesisID(id string)                   { m.synthesisID = id }
func (m *mockSession) GetRevisionRound() int                      { return m.revisionRound }
func (m *mockSession) SetSynthesisAwaitingApproval(awaiting bool) { m.awaitingApproval = awaiting }
func (m *mockSession) IsSynthesisAwaitingApproval() bool          { return m.awaitingApproval }
func (m *mockSession) SetSynthesisCompletion(completion *SynthesisCompletionFile) {
	m.synthesisComplete = completion
}
func (m *mockSession) GetPhase() UltraPlanPhase            { return m.phase }
func (m *mockSession) SetPhase(phase UltraPlanPhase)       { m.phase = phase }
func (m *mockSession) SetError(err string)                 { m.errorMsg = err }
func (m *mockSession) GetConfig() UltraPlanConfigInterface { return m.config }

// mockCallbacks implements CoordinatorCallbacksInterface for testing
type mockCallbacks struct{}

func (m *mockCallbacks) OnPhaseChange(phase UltraPlanPhase)                    {}
func (m *mockCallbacks) OnTaskStart(taskID, instanceID string)                 {}
func (m *mockCallbacks) OnTaskComplete(taskID string)                          {}
func (m *mockCallbacks) OnTaskFailed(taskID, reason string)                    {}
func (m *mockCallbacks) OnGroupComplete(groupIndex int)                        {}
func (m *mockCallbacks) OnPlanReady(plan any)                                  {}
func (m *mockCallbacks) OnProgress(completed, total int, phase UltraPlanPhase) {}
func (m *mockCallbacks) OnComplete(success bool, summary string)               {}

// mockPhaseExecutor implements PhaseExecutor for testing
type mockPhaseExecutor struct {
	phase     UltraPlanPhase
	executed  bool
	cancelled bool
	execErr   error
}

func (m *mockPhaseExecutor) Phase() UltraPlanPhase { return m.phase }
func (m *mockPhaseExecutor) Execute(ctx context.Context) error {
	m.executed = true
	return m.execErr
}
func (m *mockPhaseExecutor) Cancel() { m.cancelled = true }

func TestPhaseContextValidate(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *PhaseContext
		wantErr error
	}{
		{
			name: "valid context with all required fields",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			wantErr: nil,
		},
		{
			name: "valid context with all fields including optional",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
				Logger:       logging.NopLogger(),
				Callbacks:    &mockCallbacks{},
			},
			wantErr: nil,
		},
		{
			name: "nil manager returns ErrNilManager",
			ctx: &PhaseContext{
				Manager:      nil,
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			wantErr: ErrNilManager,
		},
		{
			name: "nil orchestrator returns ErrNilOrchestrator",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: nil,
				Session:      &mockSession{},
			},
			wantErr: ErrNilOrchestrator,
		},
		{
			name: "nil session returns ErrNilSession",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      nil,
			},
			wantErr: ErrNilSession,
		},
		{
			name: "multiple nil fields returns first error (manager)",
			ctx: &PhaseContext{
				Manager:      nil,
				Orchestrator: nil,
				Session:      nil,
			},
			wantErr: ErrNilManager,
		},
		{
			name: "nil logger is allowed",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
				Logger:       nil,
			},
			wantErr: nil,
		},
		{
			name: "nil callbacks is allowed",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
				Callbacks:    nil,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPhaseContextGetLogger(t *testing.T) {
	t.Run("returns logger when set", func(t *testing.T) {
		logger := logging.NopLogger()
		ctx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
			Logger:       logger,
		}

		got := ctx.GetLogger()
		if got != logger {
			t.Error("GetLogger() should return the set logger")
		}
	})

	t.Run("returns NopLogger when logger is nil", func(t *testing.T) {
		ctx := &PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
			Logger:       nil,
		}

		got := ctx.GetLogger()
		if got == nil {
			t.Error("GetLogger() should return a NopLogger, not nil")
		}
	})
}

func TestPhaseExecutorInterface(t *testing.T) {
	t.Run("executor returns correct phase", func(t *testing.T) {
		executor := &mockPhaseExecutor{phase: PhasePlanning}
		if executor.Phase() != PhasePlanning {
			t.Errorf("Phase() = %v, want %v", executor.Phase(), PhasePlanning)
		}
	})

	t.Run("executor Execute is called", func(t *testing.T) {
		executor := &mockPhaseExecutor{phase: PhaseExecuting}
		ctx := context.Background()

		err := executor.Execute(ctx)
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}
		if !executor.executed {
			t.Error("Execute() was not called")
		}
	})

	t.Run("executor Execute returns error", func(t *testing.T) {
		expectedErr := errors.New("execution failed")
		executor := &mockPhaseExecutor{
			phase:   PhaseExecuting,
			execErr: expectedErr,
		}
		ctx := context.Background()

		err := executor.Execute(ctx)
		if err != expectedErr {
			t.Errorf("Execute() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("executor Cancel is called", func(t *testing.T) {
		executor := &mockPhaseExecutor{phase: PhaseExecuting}

		executor.Cancel()
		if !executor.cancelled {
			t.Error("Cancel() was not called")
		}
	})

	t.Run("executor Cancel is idempotent", func(t *testing.T) {
		executor := &mockPhaseExecutor{phase: PhaseExecuting}

		// Call Cancel multiple times - should not panic
		executor.Cancel()
		executor.Cancel()
		executor.Cancel()

		if !executor.cancelled {
			t.Error("Cancel() was not called")
		}
	})
}

func TestPhaseConstants(t *testing.T) {
	tests := []struct {
		phase UltraPlanPhase
		want  string
	}{
		{PhasePlanning, "planning"},
		{PhasePlanSelection, "plan_selection"},
		{PhaseRefresh, "context_refresh"},
		{PhaseExecuting, "executing"},
		{PhaseSynthesis, "synthesis"},
		{PhaseRevision, "revision"},
		{PhaseConsolidating, "consolidating"},
		{PhaseComplete, "complete"},
		{PhaseFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.phase) != tt.want {
				t.Errorf("Phase constant = %v, want %v", string(tt.phase), tt.want)
			}
		})
	}
}

func TestValidationErrorMessages(t *testing.T) {
	t.Run("ErrNilManager has descriptive message", func(t *testing.T) {
		msg := ErrNilManager.Error()
		if msg != "phase context: manager is required" {
			t.Errorf("ErrNilManager.Error() = %q, want descriptive message", msg)
		}
	})

	t.Run("ErrNilOrchestrator has descriptive message", func(t *testing.T) {
		msg := ErrNilOrchestrator.Error()
		if msg != "phase context: orchestrator is required" {
			t.Errorf("ErrNilOrchestrator.Error() = %q, want descriptive message", msg)
		}
	})

	t.Run("ErrNilSession has descriptive message", func(t *testing.T) {
		msg := ErrNilSession.Error()
		if msg != "phase context: session is required" {
			t.Errorf("ErrNilSession.Error() = %q, want descriptive message", msg)
		}
	})
}
