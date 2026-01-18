package step

import (
	"errors"
	"testing"
)

// testableCoordinator extends mockCoordinator with state tracking for testing.
type testableCoordinator struct {
	session                   *mockSession
	planningOrchestrator      *mockPlanningOrchestrator
	executionOrchestrator     *testableExecutionOrchestrator
	synthesisOrchestrator     *testableSynthesisOrchestrator
	consolidationOrchestrator *testableConsolidationOrchestrator
	orchestrator              *testableOrchestrator
	retryManager              *mockRetryManager
	logger                    *mockLogger
	runningCount              int
	taskGroupIndices          map[string]int
	multiPassStrategyNames    []string

	// State tracking for test verification
	planningCalled            bool
	planManagerCalled         bool
	synthesisCalled           bool
	consolidationCalled       bool
	groupConsolidatorCalled   int
	groupConsolidatorGroupIdx int
}

func (m *testableCoordinator) Session() SessionInterface {
	if m.session == nil {
		return nil
	}
	return m.session
}
func (m *testableCoordinator) PlanningOrchestrator() PlanningOrchestratorInterface {
	if m.planningOrchestrator == nil {
		return nil
	}
	return m.planningOrchestrator
}
func (m *testableCoordinator) ExecutionOrchestrator() ExecutionOrchestratorInterface {
	if m.executionOrchestrator == nil {
		return nil
	}
	return m.executionOrchestrator
}
func (m *testableCoordinator) SynthesisOrchestrator() SynthesisOrchestratorInterface {
	if m.synthesisOrchestrator == nil {
		return nil
	}
	return m.synthesisOrchestrator
}
func (m *testableCoordinator) ConsolidationOrchestrator() ConsolidationOrchestratorInterface {
	if m.consolidationOrchestrator == nil {
		return nil
	}
	return m.consolidationOrchestrator
}
func (m *testableCoordinator) GetOrchestrator() OrchestratorInterface {
	if m.orchestrator == nil {
		return &mockOrchestrator{}
	}
	return m.orchestrator
}
func (m *testableCoordinator) GetRetryManager() RetryManagerInterface {
	if m.retryManager == nil {
		return &mockRetryManager{}
	}
	return m.retryManager
}
func (m *testableCoordinator) GetTaskGroupIndex(taskID string) int {
	if idx, ok := m.taskGroupIndices[taskID]; ok {
		return idx
	}
	return -1
}
func (m *testableCoordinator) GetRunningCount() int { return m.runningCount }
func (m *testableCoordinator) Lock()                {}
func (m *testableCoordinator) Unlock()              {}
func (m *testableCoordinator) SaveSession() error   { return nil }
func (m *testableCoordinator) RunPlanning() error {
	m.planningCalled = true
	return nil
}
func (m *testableCoordinator) RunPlanManager() error {
	m.planManagerCalled = true
	return nil
}
func (m *testableCoordinator) RunSynthesis() error {
	m.synthesisCalled = true
	return nil
}
func (m *testableCoordinator) StartConsolidation() error {
	m.consolidationCalled = true
	return nil
}
func (m *testableCoordinator) StartGroupConsolidatorSession(groupIndex int) error {
	m.groupConsolidatorCalled++
	m.groupConsolidatorGroupIdx = groupIndex
	return nil
}
func (m *testableCoordinator) Logger() LoggerInterface {
	if m.logger == nil {
		return &mockLogger{}
	}
	return m.logger
}
func (m *testableCoordinator) GetMultiPassStrategyNames() []string {
	return m.multiPassStrategyNames
}

// testableExecutionOrchestrator tracks state changes for testing.
type testableExecutionOrchestrator struct {
	state        *mockExecutionState
	resetCalled  bool
	startedTasks []string
	startErr     error
}

func (m *testableExecutionOrchestrator) State() ExecutionStateInterface {
	if m.state == nil {
		return &mockExecutionState{runningTasks: make(map[string]string)}
	}
	return m.state
}
func (m *testableExecutionOrchestrator) Reset() {
	m.resetCalled = true
}
func (m *testableExecutionOrchestrator) StartSingleTask(taskID string) (string, error) {
	if m.startErr != nil {
		return "", m.startErr
	}
	m.startedTasks = append(m.startedTasks, taskID)
	return "new-inst-" + taskID, nil
}

// testableSynthesisOrchestrator tracks state changes for testing.
type testableSynthesisOrchestrator struct {
	instanceID      string
	state           *mockSynthesisState
	resetCalled     bool
	revisionStarted bool
	revisionIssues  []RevisionIssue
}

func (m *testableSynthesisOrchestrator) GetInstanceID() string { return m.instanceID }
func (m *testableSynthesisOrchestrator) State() SynthesisStateInterface {
	if m.state == nil {
		return &mockSynthesisState{runningRevisionTasks: make(map[string]string)}
	}
	return m.state
}
func (m *testableSynthesisOrchestrator) Reset() {
	m.resetCalled = true
}
func (m *testableSynthesisOrchestrator) StartRevision(issues []RevisionIssue) error {
	m.revisionStarted = true
	m.revisionIssues = issues
	return nil
}

// testableConsolidationOrchestrator tracks state changes for testing.
type testableConsolidationOrchestrator struct {
	instanceID       string
	resetCalled      bool
	clearStateCalled bool
}

func (m *testableConsolidationOrchestrator) GetInstanceID() string { return m.instanceID }
func (m *testableConsolidationOrchestrator) Reset() {
	m.resetCalled = true
}
func (m *testableConsolidationOrchestrator) ClearStateForRestart() {
	m.clearStateCalled = true
}

// testableOrchestrator tracks instance operations for testing.
type testableOrchestrator struct {
	instances      map[string]*mockInstance
	stoppedInstIDs []string
	stopErr        error
}

func (m *testableOrchestrator) GetInstance(id string) InstanceInterface {
	if inst, ok := m.instances[id]; ok {
		return inst
	}
	return nil
}
func (m *testableOrchestrator) StopInstance(inst InstanceInterface) error {
	if m.stopErr != nil {
		return m.stopErr
	}
	if inst != nil {
		m.stoppedInstIDs = append(m.stoppedInstIDs, inst.GetID())
	}
	return nil
}
func (m *testableOrchestrator) SaveSession() error { return nil }

func TestRestarter_RestartStep_NilStepInfo(t *testing.T) {
	coord := &testableCoordinator{session: &mockSession{}}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(nil)
	if err == nil {
		t.Error("RestartStep(nil) should return error")
	}
}

func TestRestarter_RestartStep_NilSession(t *testing.T) {
	coord := &testableCoordinator{session: nil}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{Type: StepTypePlanning, InstanceID: "inst-1"})
	if err == nil {
		t.Error("RestartStep with nil session should return error")
	}
}

func TestRestarter_RestartStep_StopsExistingInstance(t *testing.T) {
	orch := &testableOrchestrator{
		instances: map[string]*mockInstance{
			"old-inst": {id: "old-inst"},
		},
	}
	coord := &testableCoordinator{
		session:              &mockSession{phase: PhasePlanning},
		orchestrator:         orch,
		planningOrchestrator: &mockPlanningOrchestrator{},
	}
	restarter := NewRestarter(coord)

	_, _ = restarter.RestartStep(&StepInfo{
		Type:       StepTypePlanning,
		InstanceID: "old-inst",
	})

	if len(orch.stoppedInstIDs) != 1 || orch.stoppedInstIDs[0] != "old-inst" {
		t.Errorf("Expected old instance to be stopped, got stopped: %v", orch.stoppedInstIDs)
	}
}

func TestRestarter_RestartStep_Planning(t *testing.T) {
	coord := &testableCoordinator{
		session: &mockSession{
			coordinatorID: "old-coord",
			phase:         "some-phase",
		},
		planningOrchestrator: &mockPlanningOrchestrator{},
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypePlanning,
		InstanceID: "old-coord",
	})

	if err != nil {
		t.Errorf("RestartStep(planning) error = %v", err)
	}
	if !coord.planningCalled {
		t.Error("RunPlanning was not called")
	}
	if coord.session.phase != PhasePlanning {
		t.Errorf("Phase = %v, want %v", coord.session.phase, PhasePlanning)
	}
}

func TestRestarter_RestartStep_PlanManager_NotMultiPass(t *testing.T) {
	coord := &testableCoordinator{
		session: &mockSession{
			config: &mockConfig{multiPass: false},
		},
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypePlanManager,
		InstanceID: "pm-inst",
	})

	if err == nil {
		t.Error("RestartStep(plan_manager) in non-multi-pass mode should return error")
	}
}

func TestRestarter_RestartStep_PlanManager(t *testing.T) {
	coord := &testableCoordinator{
		session: &mockSession{
			planManagerID: "old-pm",
			config:        &mockConfig{multiPass: true},
		},
		planningOrchestrator: &mockPlanningOrchestrator{},
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypePlanManager,
		InstanceID: "old-pm",
	})

	if err != nil {
		t.Errorf("RestartStep(plan_manager) error = %v", err)
	}
	if !coord.planManagerCalled {
		t.Error("RunPlanManager was not called")
	}
}

func TestRestarter_RestartStep_Task(t *testing.T) {
	execOrch := &testableExecutionOrchestrator{}
	coord := &testableCoordinator{
		session: &mockSession{
			tasks: map[string]*mockTask{
				"task-1": {id: "task-1", title: "Test Task"},
			},
			taskToInstance: map[string]string{"task-1": "old-task-inst"},
		},
		executionOrchestrator: execOrch,
		retryManager:          &mockRetryManager{states: make(map[string]any)},
		runningCount:          0,
	}
	restarter := NewRestarter(coord)

	newID, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypeTask,
		InstanceID: "old-task-inst",
		TaskID:     "task-1",
	})

	if err != nil {
		t.Errorf("RestartStep(task) error = %v", err)
	}
	if !execOrch.resetCalled {
		t.Error("ExecutionOrchestrator.Reset was not called")
	}
	if len(execOrch.startedTasks) != 1 || execOrch.startedTasks[0] != "task-1" {
		t.Errorf("StartSingleTask not called with task-1, got: %v", execOrch.startedTasks)
	}
	if newID != "new-inst-task-1" {
		t.Errorf("newID = %v, want new-inst-task-1", newID)
	}
}

func TestRestarter_RestartStep_Task_WhileRunning(t *testing.T) {
	coord := &testableCoordinator{
		session: &mockSession{
			tasks: map[string]*mockTask{
				"task-1": {id: "task-1", title: "Test Task"},
			},
		},
		runningCount: 2, // Tasks are running
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypeTask,
		InstanceID: "old-task-inst",
		TaskID:     "task-1",
	})

	if err == nil {
		t.Error("RestartStep(task) while tasks running should return error")
	}
}

func TestRestarter_RestartStep_Task_NotFound(t *testing.T) {
	coord := &testableCoordinator{
		session: &mockSession{
			tasks: map[string]*mockTask{},
		},
		runningCount: 0,
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:   StepTypeTask,
		TaskID: "nonexistent-task",
	})

	if err == nil {
		t.Error("RestartStep(task) with nonexistent task should return error")
	}
}

func TestRestarter_RestartStep_Synthesis(t *testing.T) {
	synthOrch := &testableSynthesisOrchestrator{}
	coord := &testableCoordinator{
		session: &mockSession{
			synthesisID: "old-synth",
		},
		synthesisOrchestrator: synthOrch,
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypeSynthesis,
		InstanceID: "old-synth",
	})

	if err != nil {
		t.Errorf("RestartStep(synthesis) error = %v", err)
	}
	if !synthOrch.resetCalled {
		t.Error("SynthesisOrchestrator.Reset was not called")
	}
	if !coord.synthesisCalled {
		t.Error("RunSynthesis was not called")
	}
	if coord.session.phase != PhaseSynthesis {
		t.Errorf("Phase = %v, want %v", coord.session.phase, PhaseSynthesis)
	}
}

func TestRestarter_RestartStep_Revision(t *testing.T) {
	synthOrch := &testableSynthesisOrchestrator{}
	coord := &testableCoordinator{
		session: &mockSession{
			revisionID: "old-rev",
			revision: &mockRevision{
				issues: []RevisionIssue{
					{TaskID: "task-1", Description: "Issue 1"},
					{TaskID: "task-2", Description: "Issue 2"},
				},
			},
		},
		synthesisOrchestrator: synthOrch,
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypeRevision,
		InstanceID: "old-rev",
	})

	if err != nil {
		t.Errorf("RestartStep(revision) error = %v", err)
	}
	if !synthOrch.resetCalled {
		t.Error("SynthesisOrchestrator.Reset was not called")
	}
	if !synthOrch.revisionStarted {
		t.Error("SynthesisOrchestrator.StartRevision was not called")
	}
	if len(synthOrch.revisionIssues) != 2 {
		t.Errorf("StartRevision called with %d issues, want 2", len(synthOrch.revisionIssues))
	}
	if coord.session.phase != PhaseRevision {
		t.Errorf("Phase = %v, want %v", coord.session.phase, PhaseRevision)
	}
}

func TestRestarter_RestartStep_Revision_NoIssues(t *testing.T) {
	coord := &testableCoordinator{
		session: &mockSession{
			revision: nil,
		},
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypeRevision,
		InstanceID: "old-rev",
	})

	if err == nil {
		t.Error("RestartStep(revision) with no issues should return error")
	}
}

func TestRestarter_RestartStep_Consolidation(t *testing.T) {
	consolOrch := &testableConsolidationOrchestrator{}
	coord := &testableCoordinator{
		session: &mockSession{
			consolidationID: "old-consol",
		},
		consolidationOrchestrator: consolOrch,
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypeConsolidation,
		InstanceID: "old-consol",
	})

	if err != nil {
		t.Errorf("RestartStep(consolidation) error = %v", err)
	}
	if !consolOrch.resetCalled {
		t.Error("ConsolidationOrchestrator.Reset was not called")
	}
	if !coord.consolidationCalled {
		t.Error("StartConsolidation was not called")
	}
	if coord.session.phase != PhaseConsolidating {
		t.Errorf("Phase = %v, want %v", coord.session.phase, PhaseConsolidating)
	}
}

func TestRestarter_RestartStep_GroupConsolidator(t *testing.T) {
	consolOrch := &testableConsolidationOrchestrator{}
	coord := &testableCoordinator{
		session: &mockSession{
			plan: &mockPlan{
				executionOrder: [][]string{{"task-1"}, {"task-2"}, {"task-3"}},
			},
			groupConsolidatorIDs: []string{"gc-0", "gc-1", "gc-2"},
		},
		consolidationOrchestrator: consolOrch,
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypeGroupConsolidator,
		InstanceID: "gc-1",
		GroupIndex: 1,
	})

	if err != nil {
		t.Errorf("RestartStep(group_consolidator) error = %v", err)
	}
	if !consolOrch.clearStateCalled {
		t.Error("ConsolidationOrchestrator.ClearStateForRestart was not called")
	}
	if coord.groupConsolidatorCalled != 1 {
		t.Errorf("StartGroupConsolidatorSession called %d times, want 1", coord.groupConsolidatorCalled)
	}
	if coord.groupConsolidatorGroupIdx != 1 {
		t.Errorf("StartGroupConsolidatorSession called with group %d, want 1", coord.groupConsolidatorGroupIdx)
	}
}

func TestRestarter_RestartStep_GroupConsolidator_InvalidIndex(t *testing.T) {
	coord := &testableCoordinator{
		session: &mockSession{
			plan: &mockPlan{
				executionOrder: [][]string{{"task-1"}, {"task-2"}},
			},
		},
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypeGroupConsolidator,
		GroupIndex: 5, // Invalid
	})

	if err == nil {
		t.Error("RestartStep(group_consolidator) with invalid index should return error")
	}
}

func TestRestarter_RestartStep_UnknownType(t *testing.T) {
	coord := &testableCoordinator{
		session: &mockSession{},
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type: "unknown_type",
	})

	if err == nil {
		t.Error("RestartStep with unknown type should return error")
	}
}

func TestRestarter_ExtractTasksToRevise(t *testing.T) {
	issues := []RevisionIssue{
		{TaskID: "task-1", Description: "Issue 1"},
		{TaskID: "task-2", Description: "Issue 2"},
		{TaskID: "task-1", Description: "Another issue for task-1"}, // Duplicate
		{TaskID: "task-3", Description: "Issue 3"},
	}

	tasks := extractTasksToRevise(issues)

	if len(tasks) != 3 {
		t.Errorf("extractTasksToRevise returned %d tasks, want 3", len(tasks))
	}

	// Verify unique tasks
	seen := make(map[string]bool)
	for _, taskID := range tasks {
		if seen[taskID] {
			t.Errorf("Duplicate task ID in result: %s", taskID)
		}
		seen[taskID] = true
	}

	expected := []string{"task-1", "task-2", "task-3"}
	for _, exp := range expected {
		if !seen[exp] {
			t.Errorf("Expected task %s not in result", exp)
		}
	}
}

func TestRestarter_RestartStep_StopInstanceError_ContinuesRestart(t *testing.T) {
	orch := &testableOrchestrator{
		instances: map[string]*mockInstance{
			"old-inst": {id: "old-inst"},
		},
		stopErr: errors.New("stop failed"),
	}
	coord := &testableCoordinator{
		session:              &mockSession{phase: PhasePlanning},
		orchestrator:         orch,
		planningOrchestrator: &mockPlanningOrchestrator{},
		logger:               &mockLogger{},
	}
	restarter := NewRestarter(coord)

	// Should continue despite stop error (best-effort stop)
	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypePlanning,
		InstanceID: "old-inst",
	})

	if err != nil {
		t.Errorf("RestartStep should succeed despite stop error, got: %v", err)
	}
	if !coord.planningCalled {
		t.Error("RunPlanning should still be called despite stop error")
	}
}

func TestRestarter_RestartStep_Task_StartError(t *testing.T) {
	execOrch := &testableExecutionOrchestrator{
		startErr: errors.New("start failed"),
	}
	coord := &testableCoordinator{
		session: &mockSession{
			tasks: map[string]*mockTask{
				"task-1": {id: "task-1", title: "Test Task"},
			},
		},
		executionOrchestrator: execOrch,
		retryManager:          &mockRetryManager{states: make(map[string]any)},
		runningCount:          0,
	}
	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypeTask,
		InstanceID: "old-task-inst",
		TaskID:     "task-1",
	})

	if err == nil {
		t.Error("RestartStep(task) with start error should return error")
	}
}

func TestRestarter_RestartStep_Task_ClearsFromLists(t *testing.T) {
	session := &mockSession{
		tasks: map[string]*mockTask{
			"task-1": {id: "task-1", title: "Test Task"},
		},
		taskToInstance: map[string]string{"task-1": "old-task-inst"},
	}
	// Add task-1 to completed and failed lists
	completed := []string{"task-0", "task-1", "task-2"}
	failed := []string{"task-1", "task-3"}

	execOrch := &testableExecutionOrchestrator{}
	coord := &testableCoordinator{
		session:               session,
		executionOrchestrator: execOrch,
		retryManager:          &mockRetryManager{states: make(map[string]any)},
		runningCount:          0,
	}

	// Manually set the lists that will be modified by restarter
	session.SetCompletedTasks(completed)
	session.SetFailedTasks(failed)

	restarter := NewRestarter(coord)

	_, err := restarter.RestartStep(&StepInfo{
		Type:       StepTypeTask,
		InstanceID: "old-task-inst",
		TaskID:     "task-1",
	})

	if err != nil {
		t.Errorf("RestartStep(task) error = %v", err)
	}

	// Verify task-1 was removed from taskToInstance
	if _, exists := session.taskToInstance["task-1"]; exists {
		t.Error("task-1 should be removed from taskToInstance")
	}
}
