package step

import (
	"testing"
)

// mockSession implements SessionInterface for testing.
type mockSession struct {
	coordinatorID        string
	planCoordinatorIDs   []string
	planManagerID        string
	synthesisID          string
	revisionID           string
	consolidationID      string
	groupConsolidatorIDs []string
	taskToInstance       map[string]string
	tasks                map[string]*mockTask
	config               *mockConfig
	phase                string
	plan                 *mockPlan
	revision             *mockRevision
}

func (m *mockSession) GetCoordinatorID() string          { return m.coordinatorID }
func (m *mockSession) GetPlanCoordinatorIDs() []string   { return m.planCoordinatorIDs }
func (m *mockSession) GetPlanManagerID() string          { return m.planManagerID }
func (m *mockSession) GetSynthesisID() string            { return m.synthesisID }
func (m *mockSession) GetRevisionID() string             { return m.revisionID }
func (m *mockSession) GetConsolidationID() string        { return m.consolidationID }
func (m *mockSession) GetGroupConsolidatorIDs() []string { return m.groupConsolidatorIDs }
func (m *mockSession) GetTaskToInstance() map[string]string {
	if m.taskToInstance == nil {
		return make(map[string]string)
	}
	return m.taskToInstance
}
func (m *mockSession) GetTask(taskID string) TaskInterface {
	if t, ok := m.tasks[taskID]; ok {
		return t
	}
	return nil
}
func (m *mockSession) GetConfig() ConfigInterface {
	if m.config == nil {
		return &mockConfig{}
	}
	return m.config
}
func (m *mockSession) GetPhase() string { return m.phase }
func (m *mockSession) GetPlan() PlanInterface {
	if m.plan == nil {
		return nil
	}
	return m.plan
}
func (m *mockSession) GetRevision() RevisionInterface {
	if m.revision == nil {
		return nil
	}
	return m.revision
}

// Setters (no-op for read-only tests)
func (m *mockSession) SetCoordinatorID(id string)                 { m.coordinatorID = id }
func (m *mockSession) SetPlanManagerID(id string)                 { m.planManagerID = id }
func (m *mockSession) SetSynthesisID(id string)                   { m.synthesisID = id }
func (m *mockSession) SetRevisionID(id string)                    { m.revisionID = id }
func (m *mockSession) SetConsolidationID(id string)               { m.consolidationID = id }
func (m *mockSession) SetPhase(phase string)                      { m.phase = phase }
func (m *mockSession) SetPlan(plan PlanInterface)                 {}
func (m *mockSession) SetSynthesisCompletion(completion any)      {}
func (m *mockSession) SetSynthesisAwaitingApproval(awaiting bool) {}
func (m *mockSession) SetConsolidation(state any)                 {}
func (m *mockSession) SetPRUrls(urls []string)                    {}
func (m *mockSession) SetGroupDecision(decision any)              {}
func (m *mockSession) SetGroupConsolidatorID(groupIndex int, id string) {
	if groupIndex >= 0 && groupIndex < len(m.groupConsolidatorIDs) {
		m.groupConsolidatorIDs[groupIndex] = id
	}
}
func (m *mockSession) SetGroupConsolidatedBranch(groupIndex int, branch string) {}
func (m *mockSession) SetGroupConsolidationContext(groupIndex int, ctx any)     {}
func (m *mockSession) GetCompletedTasks() []string                              { return nil }
func (m *mockSession) GetFailedTasks() []string                                 { return nil }
func (m *mockSession) SetCompletedTasks(tasks []string)                         {}
func (m *mockSession) SetFailedTasks(tasks []string)                            {}
func (m *mockSession) DeleteTaskToInstance(taskID string) {
	delete(m.taskToInstance, taskID)
}
func (m *mockSession) DeleteTaskCommitCount(taskID string)   {}
func (m *mockSession) GetTaskRetries() map[string]any        { return nil }
func (m *mockSession) SetTaskRetries(retries map[string]any) {}

// mockTask implements TaskInterface.
type mockTask struct {
	id    string
	title string
}

func (m *mockTask) GetID() string    { return m.id }
func (m *mockTask) GetTitle() string { return m.title }

// mockConfig implements ConfigInterface.
type mockConfig struct {
	multiPass bool
}

func (m *mockConfig) IsMultiPass() bool { return m.multiPass }

// mockPlan implements PlanInterface.
type mockPlan struct {
	executionOrder [][]string
}

func (m *mockPlan) GetExecutionOrder() [][]string { return m.executionOrder }

// mockRevision implements RevisionInterface.
type mockRevision struct {
	issues        []RevisionIssue
	revisedTasks  []string
	tasksToRevise []string
}

func (m *mockRevision) GetIssues() []RevisionIssue      { return m.issues }
func (m *mockRevision) GetRevisedTasks() []string       { return m.revisedTasks }
func (m *mockRevision) GetTasksToRevise() []string      { return m.tasksToRevise }
func (m *mockRevision) SetRevisedTasks(tasks []string)  { m.revisedTasks = tasks }
func (m *mockRevision) SetTasksToRevise(tasks []string) { m.tasksToRevise = tasks }

// mockPlanningOrchestrator implements PlanningOrchestratorInterface.
type mockPlanningOrchestrator struct {
	instanceID         string
	planCoordinatorIDs []string
}

func (m *mockPlanningOrchestrator) GetInstanceID() string           { return m.instanceID }
func (m *mockPlanningOrchestrator) GetPlanCoordinatorIDs() []string { return m.planCoordinatorIDs }
func (m *mockPlanningOrchestrator) Reset()                          {}

// mockExecutionOrchestrator implements ExecutionOrchestratorInterface.
type mockExecutionOrchestrator struct {
	state *mockExecutionState
}

func (m *mockExecutionOrchestrator) State() ExecutionStateInterface {
	if m.state == nil {
		return &mockExecutionState{runningTasks: make(map[string]string)}
	}
	return m.state
}
func (m *mockExecutionOrchestrator) Reset()                                        {}
func (m *mockExecutionOrchestrator) StartSingleTask(taskID string) (string, error) { return "", nil }

// mockExecutionState implements ExecutionStateInterface.
type mockExecutionState struct {
	runningTasks map[string]string
}

func (m *mockExecutionState) GetRunningTasks() map[string]string {
	if m.runningTasks == nil {
		return make(map[string]string)
	}
	return m.runningTasks
}

// mockSynthesisOrchestrator implements SynthesisOrchestratorInterface.
type mockSynthesisOrchestrator struct {
	instanceID string
	state      *mockSynthesisState
}

func (m *mockSynthesisOrchestrator) GetInstanceID() string { return m.instanceID }
func (m *mockSynthesisOrchestrator) State() SynthesisStateInterface {
	if m.state == nil {
		return &mockSynthesisState{runningRevisionTasks: make(map[string]string)}
	}
	return m.state
}
func (m *mockSynthesisOrchestrator) Reset()                                     {}
func (m *mockSynthesisOrchestrator) StartRevision(issues []RevisionIssue) error { return nil }

// mockSynthesisState implements SynthesisStateInterface.
type mockSynthesisState struct {
	runningRevisionTasks map[string]string
}

func (m *mockSynthesisState) GetRunningRevisionTasks() map[string]string {
	if m.runningRevisionTasks == nil {
		return make(map[string]string)
	}
	return m.runningRevisionTasks
}

// mockConsolidationOrchestrator implements ConsolidationOrchestratorInterface.
type mockConsolidationOrchestrator struct {
	instanceID string
}

func (m *mockConsolidationOrchestrator) GetInstanceID() string { return m.instanceID }
func (m *mockConsolidationOrchestrator) Reset()                {}
func (m *mockConsolidationOrchestrator) ClearStateForRestart() {}

// mockOrchestrator implements OrchestratorInterface.
type mockOrchestrator struct {
	instances map[string]*mockInstance
}

func (m *mockOrchestrator) GetInstance(id string) InstanceInterface {
	if inst, ok := m.instances[id]; ok {
		return inst
	}
	return nil
}
func (m *mockOrchestrator) StopInstance(inst InstanceInterface) error { return nil }
func (m *mockOrchestrator) SaveSession() error                        { return nil }

// mockInstance implements InstanceInterface.
type mockInstance struct {
	id string
}

func (m *mockInstance) GetID() string { return m.id }

// mockRetryManager implements RetryManagerInterface.
type mockRetryManager struct {
	states map[string]any
}

func (m *mockRetryManager) Reset(taskID string) {
	delete(m.states, taskID)
}
func (m *mockRetryManager) GetAllStates() map[string]any {
	if m.states == nil {
		return make(map[string]any)
	}
	return m.states
}

// mockLogger implements LoggerInterface.
type mockLogger struct{}

func (m *mockLogger) Warn(msg string, keysAndValues ...any)  {}
func (m *mockLogger) Error(msg string, keysAndValues ...any) {}

// mockCoordinator implements StepCoordinatorInterface for testing.
type mockCoordinator struct {
	session                   *mockSession
	planningOrchestrator      *mockPlanningOrchestrator
	executionOrchestrator     *mockExecutionOrchestrator
	synthesisOrchestrator     *mockSynthesisOrchestrator
	consolidationOrchestrator *mockConsolidationOrchestrator
	orchestrator              *mockOrchestrator
	retryManager              *mockRetryManager
	logger                    *mockLogger
	runningCount              int
	taskGroupIndices          map[string]int
	multiPassStrategyNames    []string
}

func (m *mockCoordinator) Session() SessionInterface {
	if m.session == nil {
		return nil
	}
	return m.session
}
func (m *mockCoordinator) PlanningOrchestrator() PlanningOrchestratorInterface {
	if m.planningOrchestrator == nil {
		return nil
	}
	return m.planningOrchestrator
}
func (m *mockCoordinator) ExecutionOrchestrator() ExecutionOrchestratorInterface {
	if m.executionOrchestrator == nil {
		return nil
	}
	return m.executionOrchestrator
}
func (m *mockCoordinator) SynthesisOrchestrator() SynthesisOrchestratorInterface {
	if m.synthesisOrchestrator == nil {
		return nil
	}
	return m.synthesisOrchestrator
}
func (m *mockCoordinator) ConsolidationOrchestrator() ConsolidationOrchestratorInterface {
	if m.consolidationOrchestrator == nil {
		return nil
	}
	return m.consolidationOrchestrator
}
func (m *mockCoordinator) GetOrchestrator() OrchestratorInterface {
	if m.orchestrator == nil {
		return &mockOrchestrator{}
	}
	return m.orchestrator
}
func (m *mockCoordinator) GetRetryManager() RetryManagerInterface {
	if m.retryManager == nil {
		return &mockRetryManager{}
	}
	return m.retryManager
}
func (m *mockCoordinator) GetTaskGroupIndex(taskID string) int {
	if idx, ok := m.taskGroupIndices[taskID]; ok {
		return idx
	}
	return -1
}
func (m *mockCoordinator) GetRunningCount() int                               { return m.runningCount }
func (m *mockCoordinator) Lock()                                              {}
func (m *mockCoordinator) Unlock()                                            {}
func (m *mockCoordinator) SaveSession() error                                 { return nil }
func (m *mockCoordinator) RunPlanning() error                                 { return nil }
func (m *mockCoordinator) RunPlanManager() error                              { return nil }
func (m *mockCoordinator) RunSynthesis() error                                { return nil }
func (m *mockCoordinator) StartConsolidation() error                          { return nil }
func (m *mockCoordinator) StartGroupConsolidatorSession(groupIndex int) error { return nil }
func (m *mockCoordinator) Logger() LoggerInterface {
	if m.logger == nil {
		return &mockLogger{}
	}
	return m.logger
}
func (m *mockCoordinator) GetMultiPassStrategyNames() []string {
	return m.multiPassStrategyNames
}

func TestResolver_GetStepInfo_NilSession(t *testing.T) {
	coord := &mockCoordinator{session: nil}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("inst-1")
	if result != nil {
		t.Errorf("GetStepInfo() with nil session = %v, want nil", result)
	}
}

func TestResolver_GetStepInfo_EmptyInstanceID(t *testing.T) {
	coord := &mockCoordinator{session: &mockSession{}}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("")
	if result != nil {
		t.Errorf("GetStepInfo() with empty instanceID = %v, want nil", result)
	}
}

func TestResolver_GetStepInfo_PlanningCoordinator(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			coordinatorID: "planning-inst-1",
		},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("planning-inst-1")
	if result == nil {
		t.Fatal("GetStepInfo() returned nil, want StepInfo")
	}
	if result.Type != StepTypePlanning {
		t.Errorf("Type = %v, want %v", result.Type, StepTypePlanning)
	}
	if result.InstanceID != "planning-inst-1" {
		t.Errorf("InstanceID = %v, want planning-inst-1", result.InstanceID)
	}
	if result.Label != "Planning Coordinator" {
		t.Errorf("Label = %v, want Planning Coordinator", result.Label)
	}
}

func TestResolver_GetStepInfo_PlanningOrchestratorFallback(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			coordinatorID: "",
		},
		planningOrchestrator: &mockPlanningOrchestrator{
			instanceID: "planning-orch-inst",
		},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("planning-orch-inst")
	if result == nil {
		t.Fatal("GetStepInfo() returned nil, want StepInfo")
	}
	if result.Type != StepTypePlanning {
		t.Errorf("Type = %v, want %v", result.Type, StepTypePlanning)
	}
}

func TestResolver_GetStepInfo_MultiPassCoordinator(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			planCoordinatorIDs: []string{"multi-1", "multi-2", "multi-3"},
		},
		multiPassStrategyNames: []string{"conservative", "aggressive", "balanced"},
	}
	resolver := NewResolver(coord)

	tests := []struct {
		instanceID    string
		expectedLabel string
		expectedGroup int
	}{
		{"multi-1", "Plan Coordinator (conservative)", 0},
		{"multi-2", "Plan Coordinator (aggressive)", 1},
		{"multi-3", "Plan Coordinator (balanced)", 2},
	}

	for _, tt := range tests {
		t.Run(tt.instanceID, func(t *testing.T) {
			result := resolver.GetStepInfo(tt.instanceID)
			if result == nil {
				t.Fatal("GetStepInfo() returned nil")
			}
			if result.Type != StepTypePlanning {
				t.Errorf("Type = %v, want %v", result.Type, StepTypePlanning)
			}
			if result.Label != tt.expectedLabel {
				t.Errorf("Label = %v, want %v", result.Label, tt.expectedLabel)
			}
			if result.GroupIndex != tt.expectedGroup {
				t.Errorf("GroupIndex = %v, want %v", result.GroupIndex, tt.expectedGroup)
			}
		})
	}
}

func TestResolver_GetStepInfo_PlanManager(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			planManagerID: "plan-manager-inst",
		},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("plan-manager-inst")
	if result == nil {
		t.Fatal("GetStepInfo() returned nil, want StepInfo")
	}
	if result.Type != StepTypePlanManager {
		t.Errorf("Type = %v, want %v", result.Type, StepTypePlanManager)
	}
	if result.Label != "Plan Manager" {
		t.Errorf("Label = %v, want Plan Manager", result.Label)
	}
}

func TestResolver_GetStepInfo_TaskInstance(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			taskToInstance: map[string]string{
				"task-1": "task-inst-1",
			},
			tasks: map[string]*mockTask{
				"task-1": {id: "task-1", title: "Implement feature X"},
			},
		},
		taskGroupIndices: map[string]int{
			"task-1": 2,
		},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("task-inst-1")
	if result == nil {
		t.Fatal("GetStepInfo() returned nil, want StepInfo")
	}
	if result.Type != StepTypeTask {
		t.Errorf("Type = %v, want %v", result.Type, StepTypeTask)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %v, want task-1", result.TaskID)
	}
	if result.GroupIndex != 2 {
		t.Errorf("GroupIndex = %v, want 2", result.GroupIndex)
	}
	if result.Label != "Implement feature X" {
		t.Errorf("Label = %v, want Implement feature X", result.Label)
	}
}

func TestResolver_GetStepInfo_TaskInstanceFromExecutionOrchestrator(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			taskToInstance: map[string]string{},
			tasks: map[string]*mockTask{
				"task-2": {id: "task-2", title: "Running task"},
			},
		},
		executionOrchestrator: &mockExecutionOrchestrator{
			state: &mockExecutionState{
				runningTasks: map[string]string{
					"task-2": "exec-inst-2",
				},
			},
		},
		taskGroupIndices: map[string]int{
			"task-2": 1,
		},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("exec-inst-2")
	if result == nil {
		t.Fatal("GetStepInfo() returned nil, want StepInfo")
	}
	if result.Type != StepTypeTask {
		t.Errorf("Type = %v, want %v", result.Type, StepTypeTask)
	}
	if result.TaskID != "task-2" {
		t.Errorf("TaskID = %v, want task-2", result.TaskID)
	}
}

func TestResolver_GetStepInfo_GroupConsolidator(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			groupConsolidatorIDs: []string{"group-consol-0", "group-consol-1"},
		},
	}
	resolver := NewResolver(coord)

	tests := []struct {
		instanceID    string
		expectedGroup int
		expectedLabel string
	}{
		{"group-consol-0", 0, "Group 1 Consolidator"},
		{"group-consol-1", 1, "Group 2 Consolidator"},
	}

	for _, tt := range tests {
		t.Run(tt.instanceID, func(t *testing.T) {
			result := resolver.GetStepInfo(tt.instanceID)
			if result == nil {
				t.Fatal("GetStepInfo() returned nil")
			}
			if result.Type != StepTypeGroupConsolidator {
				t.Errorf("Type = %v, want %v", result.Type, StepTypeGroupConsolidator)
			}
			if result.GroupIndex != tt.expectedGroup {
				t.Errorf("GroupIndex = %v, want %v", result.GroupIndex, tt.expectedGroup)
			}
			if result.Label != tt.expectedLabel {
				t.Errorf("Label = %v, want %v", result.Label, tt.expectedLabel)
			}
		})
	}
}

func TestResolver_GetStepInfo_SynthesisInstance(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			synthesisID: "synthesis-inst",
		},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("synthesis-inst")
	if result == nil {
		t.Fatal("GetStepInfo() returned nil, want StepInfo")
	}
	if result.Type != StepTypeSynthesis {
		t.Errorf("Type = %v, want %v", result.Type, StepTypeSynthesis)
	}
	if result.Label != "Synthesis" {
		t.Errorf("Label = %v, want Synthesis", result.Label)
	}
}

func TestResolver_GetStepInfo_SynthesisOrchestratorFallback(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			synthesisID: "",
		},
		synthesisOrchestrator: &mockSynthesisOrchestrator{
			instanceID: "synth-orch-inst",
		},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("synth-orch-inst")
	if result == nil {
		t.Fatal("GetStepInfo() returned nil, want StepInfo")
	}
	if result.Type != StepTypeSynthesis {
		t.Errorf("Type = %v, want %v", result.Type, StepTypeSynthesis)
	}
}

func TestResolver_GetStepInfo_RevisionInstance(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			revisionID: "revision-inst",
		},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("revision-inst")
	if result == nil {
		t.Fatal("GetStepInfo() returned nil, want StepInfo")
	}
	if result.Type != StepTypeRevision {
		t.Errorf("Type = %v, want %v", result.Type, StepTypeRevision)
	}
	if result.Label != "Revision" {
		t.Errorf("Label = %v, want Revision", result.Label)
	}
}

func TestResolver_GetStepInfo_RevisionFromSynthesisState(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			revisionID: "",
		},
		synthesisOrchestrator: &mockSynthesisOrchestrator{
			state: &mockSynthesisState{
				runningRevisionTasks: map[string]string{
					"task-rev-1": "rev-task-inst",
				},
			},
		},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("rev-task-inst")
	if result == nil {
		t.Fatal("GetStepInfo() returned nil, want StepInfo")
	}
	if result.Type != StepTypeRevision {
		t.Errorf("Type = %v, want %v", result.Type, StepTypeRevision)
	}
}

func TestResolver_GetStepInfo_ConsolidationInstance(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			consolidationID: "consolidation-inst",
		},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("consolidation-inst")
	if result == nil {
		t.Fatal("GetStepInfo() returned nil, want StepInfo")
	}
	if result.Type != StepTypeConsolidation {
		t.Errorf("Type = %v, want %v", result.Type, StepTypeConsolidation)
	}
	if result.Label != "Consolidation" {
		t.Errorf("Label = %v, want Consolidation", result.Label)
	}
}

func TestResolver_GetStepInfo_ConsolidationOrchestratorFallback(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			consolidationID: "",
		},
		consolidationOrchestrator: &mockConsolidationOrchestrator{
			instanceID: "consol-orch-inst",
		},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("consol-orch-inst")
	if result == nil {
		t.Fatal("GetStepInfo() returned nil, want StepInfo")
	}
	if result.Type != StepTypeConsolidation {
		t.Errorf("Type = %v, want %v", result.Type, StepTypeConsolidation)
	}
}

func TestResolver_GetStepInfo_UnknownInstance(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{},
	}
	resolver := NewResolver(coord)

	result := resolver.GetStepInfo("unknown-inst")
	if result != nil {
		t.Errorf("GetStepInfo() for unknown instance = %v, want nil", result)
	}
}
