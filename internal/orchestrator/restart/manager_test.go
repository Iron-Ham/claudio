package restart

import (
	"errors"
	"testing"
)

// mockSessionOps implements SessionOperations for testing
type mockSessionOps struct {
	coordinatorID        string
	planCoordinatorIDs   []string
	planManagerID        string
	taskToInstance       map[string]string
	groupConsolidatorIDs []string
	synthesisID          string
	revisionID           string
	consolidationID      string
	plan                 any
	phase                UltraPlanPhase
	multiPass            bool
	tasks                map[string]any
	revision             any
	executionOrder       [][]string
	completedTasks       []string
	failedTasks          []string
	strategyNames        []string
	taskGroupIndices     map[string]int
}

func (m *mockSessionOps) GetCoordinatorID() string             { return m.coordinatorID }
func (m *mockSessionOps) SetCoordinatorID(id string)           { m.coordinatorID = id }
func (m *mockSessionOps) GetPlanCoordinatorIDs() []string      { return m.planCoordinatorIDs }
func (m *mockSessionOps) GetPlanManagerID() string             { return m.planManagerID }
func (m *mockSessionOps) SetPlanManagerID(id string)           { m.planManagerID = id }
func (m *mockSessionOps) GetTaskToInstance() map[string]string { return m.taskToInstance }
func (m *mockSessionOps) GetGroupConsolidatorIDs() []string    { return m.groupConsolidatorIDs }
func (m *mockSessionOps) GetSynthesisID() string               { return m.synthesisID }
func (m *mockSessionOps) SetSynthesisID(id string)             { m.synthesisID = id }
func (m *mockSessionOps) GetRevisionID() string                { return m.revisionID }
func (m *mockSessionOps) SetRevisionID(id string)              { m.revisionID = id }
func (m *mockSessionOps) GetConsolidationID() string           { return m.consolidationID }
func (m *mockSessionOps) SetConsolidationID(id string)         { m.consolidationID = id }
func (m *mockSessionOps) GetPlan() any                         { return m.plan }
func (m *mockSessionOps) SetPlan(plan any)                     { m.plan = plan }
func (m *mockSessionOps) GetPhase() UltraPlanPhase             { return m.phase }
func (m *mockSessionOps) SetPhase(phase UltraPlanPhase)        { m.phase = phase }
func (m *mockSessionOps) IsMultiPass() bool                    { return m.multiPass }
func (m *mockSessionOps) GetTask(taskID string) any {
	if m.tasks == nil {
		return nil
	}
	return m.tasks[taskID]
}
func (m *mockSessionOps) ClearSynthesisState()          { m.synthesisID = "" }
func (m *mockSessionOps) GetRevision() any              { return m.revision }
func (m *mockSessionOps) ClearConsolidationState()      { m.consolidationID = "" }
func (m *mockSessionOps) ClearPRUrls()                  {}
func (m *mockSessionOps) GetExecutionOrder() [][]string { return m.executionOrder }
func (m *mockSessionOps) ClearGroupConsolidatorID(groupIndex int) {
	if groupIndex < len(m.groupConsolidatorIDs) {
		m.groupConsolidatorIDs[groupIndex] = ""
	}
}
func (m *mockSessionOps) ClearGroupConsolidatedBranch(groupIndex int)   {}
func (m *mockSessionOps) ClearGroupConsolidationContext(groupIndex int) {}
func (m *mockSessionOps) GetCompletedTasks() []string                   { return m.completedTasks }
func (m *mockSessionOps) SetCompletedTasks(tasks []string)              { m.completedTasks = tasks }
func (m *mockSessionOps) GetFailedTasks() []string                      { return m.failedTasks }
func (m *mockSessionOps) SetFailedTasks(tasks []string)                 { m.failedTasks = tasks }
func (m *mockSessionOps) DeleteTaskFromInstance(taskID string) {
	delete(m.taskToInstance, taskID)
}
func (m *mockSessionOps) DeleteTaskCommitCount(taskID string) {}
func (m *mockSessionOps) ClearGroupDecision()                 {}
func (m *mockSessionOps) GetMultiPassStrategyNames() []string { return m.strategyNames }
func (m *mockSessionOps) GetTaskGroupIndex(taskID string) int {
	if m.taskGroupIndices == nil {
		return -1
	}
	if idx, ok := m.taskGroupIndices[taskID]; ok {
		return idx
	}
	return -1
}

// mockTask implements a minimal task for testing
type mockTask struct {
	id    string
	title string
}

func (t *mockTask) GetTitle() string { return t.title }

func TestNewManager(t *testing.T) {
	t.Run("with nil context", func(t *testing.T) {
		m := NewManager(nil)
		if m == nil {
			t.Fatal("NewManager returned nil")
		}
	})

	t.Run("with context", func(t *testing.T) {
		ctx := &Context{
			Session: &mockSessionOps{},
		}
		m := NewManager(ctx)
		if m == nil {
			t.Fatal("NewManager returned nil")
		}
	})
}

func TestManager_GetStepInfo(t *testing.T) {
	tests := []struct {
		name       string
		session    *mockSessionOps
		instanceID string
		want       *StepInfo
	}{
		{
			name:       "empty instance ID",
			session:    &mockSessionOps{},
			instanceID: "",
			want:       nil,
		},
		{
			name: "planning coordinator",
			session: &mockSessionOps{
				coordinatorID: "coord-123",
			},
			instanceID: "coord-123",
			want: &StepInfo{
				Type:       StepTypePlanning,
				InstanceID: "coord-123",
				GroupIndex: -1,
				Label:      "Planning Coordinator",
			},
		},
		{
			name: "multi-pass plan coordinator",
			session: &mockSessionOps{
				planCoordinatorIDs: []string{"plan-1", "plan-2", "plan-3"},
			},
			instanceID: "plan-2",
			want: &StepInfo{
				Type:       StepTypePlanning,
				InstanceID: "plan-2",
				GroupIndex: 1,
				Label:      "Plan Coordinator 2",
			},
		},
		{
			name: "plan manager",
			session: &mockSessionOps{
				planManagerID: "manager-123",
			},
			instanceID: "manager-123",
			want: &StepInfo{
				Type:       StepTypePlanManager,
				InstanceID: "manager-123",
				GroupIndex: -1,
				Label:      "Plan Manager",
			},
		},
		{
			name: "task instance",
			session: &mockSessionOps{
				taskToInstance: map[string]string{
					"task-1": "inst-1",
					"task-2": "inst-2",
				},
				tasks: map[string]any{
					"task-1": &mockTask{id: "task-1", title: "Test Task 1"},
					"task-2": &mockTask{id: "task-2", title: "Test Task 2"},
				},
			},
			instanceID: "inst-1",
			want: &StepInfo{
				Type:       StepTypeTask,
				InstanceID: "inst-1",
				TaskID:     "task-1",
				GroupIndex: -1,
				Label:      "Test Task 1",
			},
		},
		{
			name: "group consolidator",
			session: &mockSessionOps{
				groupConsolidatorIDs: []string{"gc-1", "gc-2"},
			},
			instanceID: "gc-2",
			want: &StepInfo{
				Type:       StepTypeGroupConsolidator,
				InstanceID: "gc-2",
				GroupIndex: 1,
				Label:      "Group 2 Consolidator",
			},
		},
		{
			name: "synthesis instance",
			session: &mockSessionOps{
				synthesisID: "synth-123",
			},
			instanceID: "synth-123",
			want: &StepInfo{
				Type:       StepTypeSynthesis,
				InstanceID: "synth-123",
				GroupIndex: -1,
				Label:      "Synthesis",
			},
		},
		{
			name: "revision instance",
			session: &mockSessionOps{
				revisionID: "rev-123",
			},
			instanceID: "rev-123",
			want: &StepInfo{
				Type:       StepTypeRevision,
				InstanceID: "rev-123",
				GroupIndex: -1,
				Label:      "Revision",
			},
		},
		{
			name: "consolidation instance",
			session: &mockSessionOps{
				consolidationID: "consol-123",
			},
			instanceID: "consol-123",
			want: &StepInfo{
				Type:       StepTypeConsolidation,
				InstanceID: "consol-123",
				GroupIndex: -1,
				Label:      "Consolidation",
			},
		},
		{
			name: "unknown instance",
			session: &mockSessionOps{
				coordinatorID: "other",
			},
			instanceID: "unknown-123",
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{Session: tt.session}
			m := NewManager(ctx)
			got := m.GetStepInfo(tt.instanceID)

			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("expected %+v, got nil", tt.want)
			}

			if got.Type != tt.want.Type {
				t.Errorf("Type: expected %s, got %s", tt.want.Type, got.Type)
			}
			if got.InstanceID != tt.want.InstanceID {
				t.Errorf("InstanceID: expected %s, got %s", tt.want.InstanceID, got.InstanceID)
			}
			if got.GroupIndex != tt.want.GroupIndex {
				t.Errorf("GroupIndex: expected %d, got %d", tt.want.GroupIndex, got.GroupIndex)
			}
			if got.Label != tt.want.Label {
				t.Errorf("Label: expected %s, got %s", tt.want.Label, got.Label)
			}
			if got.TaskID != tt.want.TaskID {
				t.Errorf("TaskID: expected %s, got %s", tt.want.TaskID, got.TaskID)
			}
		})
	}
}

func TestManager_RestartStep_Errors(t *testing.T) {
	t.Run("nil step info", func(t *testing.T) {
		m := NewManager(&Context{Session: &mockSessionOps{}})
		_, err := m.RestartStep(nil)
		if err == nil {
			t.Error("expected error for nil step info")
		}
	})

	t.Run("nil session", func(t *testing.T) {
		m := NewManager(&Context{})
		_, err := m.RestartStep(&StepInfo{Type: StepTypePlanning})
		if err == nil {
			t.Error("expected error for nil session")
		}
	})

	t.Run("unknown step type", func(t *testing.T) {
		m := NewManager(&Context{Session: &mockSessionOps{}})
		_, err := m.RestartStep(&StepInfo{Type: "unknown"})
		if err == nil {
			t.Error("expected error for unknown step type")
		}
	})

	t.Run("plan manager without multi-pass", func(t *testing.T) {
		session := &mockSessionOps{multiPass: false}
		m := NewManager(&Context{Session: session})
		_, err := m.RestartStep(&StepInfo{Type: StepTypePlanManager})
		if err == nil {
			t.Error("expected error when restarting plan manager without multi-pass")
		}
	})

	t.Run("task with empty ID", func(t *testing.T) {
		m := NewManager(&Context{Session: &mockSessionOps{}})
		_, err := m.RestartStep(&StepInfo{Type: StepTypeTask, TaskID: ""})
		if err == nil {
			t.Error("expected error for empty task ID")
		}
	})

	t.Run("task not found", func(t *testing.T) {
		session := &mockSessionOps{
			tasks: map[string]any{},
		}
		m := NewManager(&Context{Session: session})
		_, err := m.RestartStep(&StepInfo{Type: StepTypeTask, TaskID: "nonexistent"})
		if err == nil {
			t.Error("expected error for nonexistent task")
		}
	})

	t.Run("group consolidator with invalid index", func(t *testing.T) {
		session := &mockSessionOps{
			executionOrder: [][]string{{"task-1"}, {"task-2"}},
		}
		m := NewManager(&Context{Session: session})
		_, err := m.RestartStep(&StepInfo{Type: StepTypeGroupConsolidator, GroupIndex: 5})
		if err == nil {
			t.Error("expected error for invalid group index")
		}
	})
}

// mockPlanningOps implements PlanningOperations for testing
type mockPlanningOps struct {
	runPlanningCalled    bool
	runPlanManagerCalled bool
	resetCalled          bool
	planningError        error
	planManagerError     error
}

func (m *mockPlanningOps) RunPlanning() error {
	m.runPlanningCalled = true
	return m.planningError
}

func (m *mockPlanningOps) RunPlanManager() error {
	m.runPlanManagerCalled = true
	return m.planManagerError
}

func (m *mockPlanningOps) ResetPlanningOrchestrator() {
	m.resetCalled = true
}

func TestManager_RestartPlanning(t *testing.T) {
	t.Run("successful restart", func(t *testing.T) {
		session := &mockSessionOps{
			coordinatorID: "old-coord",
			plan:          "old-plan",
			phase:         PhaseExecuting,
		}
		planning := &mockPlanningOps{}

		m := NewManager(&Context{
			Session:  session,
			Planning: planning,
		})

		_, err := m.RestartStep(&StepInfo{Type: StepTypePlanning, InstanceID: "old-coord"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !planning.resetCalled {
			t.Error("expected planning orchestrator reset")
		}
		if !planning.runPlanningCalled {
			t.Error("expected RunPlanning to be called")
		}
		if session.plan != nil {
			t.Error("expected plan to be cleared")
		}
		if session.phase != PhasePlanning {
			t.Errorf("expected phase %s, got %s", PhasePlanning, session.phase)
		}
	})

	t.Run("planning error", func(t *testing.T) {
		session := &mockSessionOps{}
		planning := &mockPlanningOps{
			planningError: errors.New("planning failed"),
		}

		m := NewManager(&Context{
			Session:  session,
			Planning: planning,
		})

		_, err := m.RestartStep(&StepInfo{Type: StepTypePlanning})
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestManager_RestartPlanManager(t *testing.T) {
	t.Run("successful restart", func(t *testing.T) {
		session := &mockSessionOps{
			multiPass:     true,
			planManagerID: "old-manager",
			plan:          "old-plan",
			phase:         PhaseExecuting,
		}
		planning := &mockPlanningOps{}

		m := NewManager(&Context{
			Session:  session,
			Planning: planning,
		})

		_, err := m.RestartStep(&StepInfo{Type: StepTypePlanManager, InstanceID: "old-manager"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !planning.resetCalled {
			t.Error("expected planning orchestrator reset")
		}
		if !planning.runPlanManagerCalled {
			t.Error("expected RunPlanManager to be called")
		}
		if session.plan != nil {
			t.Error("expected plan to be cleared")
		}
		if session.phase != PhasePlanSelection {
			t.Errorf("expected phase %s, got %s", PhasePlanSelection, session.phase)
		}
	})
}

// mockSynthesisOps implements SynthesisOperations for testing
type mockSynthesisOps struct {
	runSynthesisCalled  bool
	startRevisionCalled bool
	resetCalled         bool
	synthesisError      error
	revisionError       error
}

func (m *mockSynthesisOps) RunSynthesis() error {
	m.runSynthesisCalled = true
	return m.synthesisError
}

func (m *mockSynthesisOps) StartRevision(issues []any) error {
	m.startRevisionCalled = true
	return m.revisionError
}

func (m *mockSynthesisOps) ResetSynthesisOrchestrator() {
	m.resetCalled = true
}

func TestManager_RestartSynthesis(t *testing.T) {
	t.Run("successful restart", func(t *testing.T) {
		session := &mockSessionOps{
			synthesisID: "old-synth",
			phase:       PhaseExecuting,
		}
		synthesis := &mockSynthesisOps{}

		m := NewManager(&Context{
			Session:   session,
			Synthesis: synthesis,
		})

		_, err := m.RestartStep(&StepInfo{Type: StepTypeSynthesis, InstanceID: "old-synth"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !synthesis.resetCalled {
			t.Error("expected synthesis orchestrator reset")
		}
		if !synthesis.runSynthesisCalled {
			t.Error("expected RunSynthesis to be called")
		}
		if session.phase != PhaseSynthesis {
			t.Errorf("expected phase %s, got %s", PhaseSynthesis, session.phase)
		}
	})
}

// mockRevision implements a revision with issues
type mockRevision struct {
	issues []any
}

func (r *mockRevision) GetIssues() []any { return r.issues }

func TestManager_RestartRevision(t *testing.T) {
	t.Run("no revision state", func(t *testing.T) {
		session := &mockSessionOps{
			revision: nil,
		}

		m := NewManager(&Context{Session: session})
		_, err := m.RestartStep(&StepInfo{Type: StepTypeRevision})
		if err == nil {
			t.Error("expected error when no revision state")
		}
	})

	t.Run("no issues", func(t *testing.T) {
		session := &mockSessionOps{
			revision: &mockRevision{issues: []any{}},
		}

		m := NewManager(&Context{Session: session})
		_, err := m.RestartStep(&StepInfo{Type: StepTypeRevision})
		if err == nil {
			t.Error("expected error when no issues")
		}
	})

	t.Run("successful restart", func(t *testing.T) {
		session := &mockSessionOps{
			revisionID: "old-rev",
			revision:   &mockRevision{issues: []any{"issue1", "issue2"}},
			phase:      PhaseExecuting,
		}
		synthesis := &mockSynthesisOps{}

		m := NewManager(&Context{
			Session:   session,
			Synthesis: synthesis,
		})

		_, err := m.RestartStep(&StepInfo{Type: StepTypeRevision, InstanceID: "old-rev"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !synthesis.resetCalled {
			t.Error("expected synthesis orchestrator reset")
		}
		if !synthesis.startRevisionCalled {
			t.Error("expected StartRevision to be called")
		}
		if session.phase != PhaseRevision {
			t.Errorf("expected phase %s, got %s", PhaseRevision, session.phase)
		}
	})
}

// mockConsolidationOps implements ConsolidationOperations for testing
type mockConsolidationOps struct {
	startConsolidationCalled bool
	startGroupCalled         bool
	startGroupIndex          int
	resetCalled              bool
	consolidationError       error
	groupError               error
}

func (m *mockConsolidationOps) StartConsolidation() error {
	m.startConsolidationCalled = true
	return m.consolidationError
}

func (m *mockConsolidationOps) StartGroupConsolidatorSession(groupIndex int) error {
	m.startGroupCalled = true
	m.startGroupIndex = groupIndex
	return m.groupError
}

func (m *mockConsolidationOps) ResetConsolidationOrchestrator() {
	m.resetCalled = true
}

func TestManager_RestartConsolidation(t *testing.T) {
	t.Run("successful restart", func(t *testing.T) {
		session := &mockSessionOps{
			consolidationID: "old-consol",
			phase:           PhaseExecuting,
		}
		consolidation := &mockConsolidationOps{}

		m := NewManager(&Context{
			Session:       session,
			Consolidation: consolidation,
		})

		_, err := m.RestartStep(&StepInfo{Type: StepTypeConsolidation, InstanceID: "old-consol"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !consolidation.resetCalled {
			t.Error("expected consolidation orchestrator reset")
		}
		if !consolidation.startConsolidationCalled {
			t.Error("expected StartConsolidation to be called")
		}
		if session.phase != PhaseConsolidating {
			t.Errorf("expected phase %s, got %s", PhaseConsolidating, session.phase)
		}
	})
}

func TestManager_RestartGroupConsolidator(t *testing.T) {
	t.Run("successful restart", func(t *testing.T) {
		session := &mockSessionOps{
			executionOrder:       [][]string{{"task-1"}, {"task-2"}},
			groupConsolidatorIDs: []string{"gc-1", "gc-2"},
		}
		consolidation := &mockConsolidationOps{}

		m := NewManager(&Context{
			Session:       session,
			Consolidation: consolidation,
		})

		_, err := m.RestartStep(&StepInfo{
			Type:       StepTypeGroupConsolidator,
			InstanceID: "gc-1",
			GroupIndex: 1,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !consolidation.resetCalled {
			t.Error("expected consolidation orchestrator reset")
		}
		if !consolidation.startGroupCalled {
			t.Error("expected StartGroupConsolidatorSession to be called")
		}
		if consolidation.startGroupIndex != 1 {
			t.Errorf("expected group index 1, got %d", consolidation.startGroupIndex)
		}
	})
}
