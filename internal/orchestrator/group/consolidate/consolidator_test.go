package consolidate

import (
	"errors"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
)

// mockSession implements SessionInterface for testing.
type mockSession struct {
	id                         string
	plan                       *mockPlan
	config                     *mockConfig
	tasks                      map[string]*mockTask
	taskCommitCounts           map[string]int
	groupConsolidatedBranches  []string
	groupConsolidationContexts []*types.GroupConsolidationCompletionFile
	groupConsolidatorIDs       []string
}

func (m *mockSession) GetID() string { return m.id }
func (m *mockSession) GetPlan() PlanInterface {
	if m.plan == nil {
		return nil
	}
	return m.plan
}
func (m *mockSession) GetConfig() ConfigInterface {
	if m.config == nil {
		return &mockConfig{}
	}
	return m.config
}
func (m *mockSession) GetTask(taskID string) TaskInterface {
	if t, ok := m.tasks[taskID]; ok {
		return t
	}
	return nil
}
func (m *mockSession) GetTaskCommitCounts() map[string]int {
	if m.taskCommitCounts == nil {
		return make(map[string]int)
	}
	return m.taskCommitCounts
}
func (m *mockSession) GetGroupConsolidatedBranches() []string {
	return m.groupConsolidatedBranches
}
func (m *mockSession) GetGroupConsolidationContexts() []*types.GroupConsolidationCompletionFile {
	return m.groupConsolidationContexts
}
func (m *mockSession) GetGroupConsolidatorIDs() []string {
	return m.groupConsolidatorIDs
}
func (m *mockSession) SetGroupConsolidatorID(groupIndex int, id string) {
	if groupIndex >= 0 && groupIndex < len(m.groupConsolidatorIDs) {
		m.groupConsolidatorIDs[groupIndex] = id
	}
}
func (m *mockSession) SetGroupConsolidatedBranch(groupIndex int, branch string) {
	if groupIndex >= 0 && groupIndex < len(m.groupConsolidatedBranches) {
		m.groupConsolidatedBranches[groupIndex] = branch
	}
}
func (m *mockSession) SetGroupConsolidationContext(groupIndex int, ctx *types.GroupConsolidationCompletionFile) {
	if groupIndex >= 0 && groupIndex < len(m.groupConsolidationContexts) {
		m.groupConsolidationContexts[groupIndex] = ctx
	}
}
func (m *mockSession) EnsureGroupArraysCapacity(groupIndex int) {
	for len(m.groupConsolidatorIDs) <= groupIndex {
		m.groupConsolidatorIDs = append(m.groupConsolidatorIDs, "")
	}
	for len(m.groupConsolidatedBranches) <= groupIndex {
		m.groupConsolidatedBranches = append(m.groupConsolidatedBranches, "")
	}
	for len(m.groupConsolidationContexts) <= groupIndex {
		m.groupConsolidationContexts = append(m.groupConsolidationContexts, nil)
	}
}

// mockPlan implements PlanInterface.
type mockPlan struct {
	summary        string
	executionOrder [][]string
}

func (m *mockPlan) GetSummary() string            { return m.summary }
func (m *mockPlan) GetExecutionOrder() [][]string { return m.executionOrder }

// mockConfig implements ConfigInterface.
type mockConfig struct {
	branchPrefix string
	multiPass    bool
}

func (m *mockConfig) GetBranchPrefix() string { return m.branchPrefix }
func (m *mockConfig) IsMultiPass() bool       { return m.multiPass }

// mockTask implements TaskInterface.
type mockTask struct {
	id    string
	title string
}

func (m *mockTask) GetID() string    { return m.id }
func (m *mockTask) GetTitle() string { return m.title }

// mockWorktree implements WorktreeInterface.
type mockWorktree struct {
	mainBranch           string
	createBranchErr      error
	createWorktreeErr    error
	removeErr            error
	cherryPickErr        error
	abortCherryPickErr   error
	countCommitsResult   int
	countCommitsErr      error
	pushErr              error
	createdBranches      []string
	createdWorktrees     []string
	removedWorktrees     []string
	cherryPickedBranches []string
}

func (m *mockWorktree) FindMainBranch() string { return m.mainBranch }
func (m *mockWorktree) CreateBranchFrom(branchName, baseBranch string) error {
	if m.createBranchErr != nil {
		return m.createBranchErr
	}
	m.createdBranches = append(m.createdBranches, branchName)
	return nil
}
func (m *mockWorktree) CreateWorktreeFromBranch(path, branch string) error {
	if m.createWorktreeErr != nil {
		return m.createWorktreeErr
	}
	m.createdWorktrees = append(m.createdWorktrees, path)
	return nil
}
func (m *mockWorktree) Remove(path string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	m.removedWorktrees = append(m.removedWorktrees, path)
	return nil
}
func (m *mockWorktree) CherryPickBranch(worktreePath, sourceBranch string) error {
	if m.cherryPickErr != nil {
		return m.cherryPickErr
	}
	m.cherryPickedBranches = append(m.cherryPickedBranches, sourceBranch)
	return nil
}
func (m *mockWorktree) AbortCherryPick(worktreePath string) error {
	return m.abortCherryPickErr
}
func (m *mockWorktree) CountCommitsBetween(worktreePath, baseBranch, head string) (int, error) {
	if m.countCommitsErr != nil {
		return 0, m.countCommitsErr
	}
	return m.countCommitsResult, nil
}
func (m *mockWorktree) Push(worktreePath string, force bool) error {
	return m.pushErr
}

// mockOrchestrator implements OrchestratorInterface.
type mockOrchestrator struct {
	worktree         *mockWorktree
	instances        map[string]*mockInstance
	claudioDir       string
	branchPrefix     string
	startErr         error
	stopErr          error
	saveErr          error
	addedInstances   []*mockInstance
	instanceManagers map[string]*mockInstanceManager
}

func (m *mockOrchestrator) Worktree() WorktreeInterface {
	if m.worktree == nil {
		return &mockWorktree{mainBranch: "main"}
	}
	return m.worktree
}
func (m *mockOrchestrator) AddInstance(baseSession BaseSessionInterface, prompt string) (InstanceInterface, error) {
	inst := &mockInstance{id: "new-inst-" + string(rune(len(m.addedInstances)+'0'))}
	m.addedInstances = append(m.addedInstances, inst)
	return inst, nil
}
func (m *mockOrchestrator) AddInstanceFromBranch(baseSession BaseSessionInterface, prompt, branch string) (InstanceInterface, error) {
	inst := &mockInstance{id: "new-inst-branch-" + string(rune(len(m.addedInstances)+'0'))}
	m.addedInstances = append(m.addedInstances, inst)
	return inst, nil
}
func (m *mockOrchestrator) GetInstance(id string) InstanceInterface {
	if inst, ok := m.instances[id]; ok {
		return inst
	}
	return nil
}
func (m *mockOrchestrator) StartInstance(inst InstanceInterface) error {
	return m.startErr
}
func (m *mockOrchestrator) StopInstance(inst InstanceInterface) error {
	return m.stopErr
}
func (m *mockOrchestrator) SaveSession() error {
	return m.saveErr
}
func (m *mockOrchestrator) GetClaudioDir() string {
	return m.claudioDir
}
func (m *mockOrchestrator) GetBranchPrefix() string {
	return m.branchPrefix
}
func (m *mockOrchestrator) GetInstanceManager(id string) InstanceManagerInterface {
	if mgr, ok := m.instanceManagers[id]; ok {
		return mgr
	}
	return nil
}

// mockInstance implements InstanceInterface.
type mockInstance struct {
	id           string
	task         string
	branch       string
	worktreePath string
	status       string
}

func (m *mockInstance) GetID() string           { return m.id }
func (m *mockInstance) GetTask() string         { return m.task }
func (m *mockInstance) GetBranch() string       { return m.branch }
func (m *mockInstance) GetWorktreePath() string { return m.worktreePath }
func (m *mockInstance) GetStatus() string       { return m.status }

// mockInstanceManager implements InstanceManagerInterface.
type mockInstanceManager struct {
	tmuxExists bool
}

func (m *mockInstanceManager) TmuxSessionExists() bool {
	return m.tmuxExists
}

// mockBaseSession implements BaseSessionInterface.
type mockBaseSession struct {
	instances []InstanceInterface
	groups    map[string]*mockGroup
}

func (m *mockBaseSession) GetInstances() []InstanceInterface {
	return m.instances
}
func (m *mockBaseSession) GetGroupBySessionType(sessionType string) GroupInterface {
	if g, ok := m.groups[sessionType]; ok {
		return g
	}
	return nil
}

// mockGroup implements GroupInterface.
type mockGroup struct {
	addedInstances []string
}

func (m *mockGroup) AddInstance(instanceID string) {
	m.addedInstances = append(m.addedInstances, instanceID)
}

// mockManager implements ManagerInterface.
type mockManager struct {
	emittedEvents []string
}

func (m *mockManager) EmitEvent(eventType, message string) {
	m.emittedEvents = append(m.emittedEvents, eventType+": "+message)
}

// mockContext implements ContextInterface.
type mockContext struct {
	done chan struct{}
}

func (m *mockContext) Done() <-chan struct{} {
	return m.done
}

// mockCoordinator implements CoordinatorInterface.
type mockCoordinator struct {
	session      *mockSession
	orchestrator *mockOrchestrator
	baseSession  *mockBaseSession
	manager      *mockManager
	ctx          *mockContext
	locked       bool
}

func (m *mockCoordinator) Session() SessionInterface {
	if m.session == nil {
		return nil
	}
	return m.session
}
func (m *mockCoordinator) Orchestrator() OrchestratorInterface {
	if m.orchestrator == nil {
		return &mockOrchestrator{}
	}
	return m.orchestrator
}
func (m *mockCoordinator) BaseSession() BaseSessionInterface {
	if m.baseSession == nil {
		return &mockBaseSession{}
	}
	return m.baseSession
}
func (m *mockCoordinator) Manager() ManagerInterface {
	if m.manager == nil {
		return &mockManager{}
	}
	return m.manager
}
func (m *mockCoordinator) Lock()   { m.locked = true }
func (m *mockCoordinator) Unlock() { m.locked = false }
func (m *mockCoordinator) Context() ContextInterface {
	if m.ctx == nil {
		return &mockContext{done: make(chan struct{})}
	}
	return m.ctx
}

func TestConsolidator_GetBaseBranchForGroup_Group0(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{},
	}
	consolidator := NewConsolidator(coord)

	result := consolidator.GetBaseBranchForGroup(0)
	if result != "" {
		t.Errorf("GetBaseBranchForGroup(0) = %v, want empty string", result)
	}
}

func TestConsolidator_GetBaseBranchForGroup_WithPreviousBranch(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			groupConsolidatedBranches: []string{"Iron-Ham/ultraplan-abc-group-1", "Iron-Ham/ultraplan-abc-group-2"},
		},
	}
	consolidator := NewConsolidator(coord)

	tests := []struct {
		groupIndex int
		want       string
	}{
		{0, ""},
		{1, "Iron-Ham/ultraplan-abc-group-1"},
		{2, "Iron-Ham/ultraplan-abc-group-2"},
		{3, ""}, // No previous branch exists
	}

	for _, tt := range tests {
		t.Run("group"+string(rune(tt.groupIndex+'0')), func(t *testing.T) {
			result := consolidator.GetBaseBranchForGroup(tt.groupIndex)
			if result != tt.want {
				t.Errorf("GetBaseBranchForGroup(%d) = %v, want %v", tt.groupIndex, result, tt.want)
			}
		})
	}
}

func TestConsolidator_ConsolidateWithVerification_NilSession(t *testing.T) {
	coord := &mockCoordinator{
		session: nil,
	}
	consolidator := NewConsolidator(coord)

	err := consolidator.ConsolidateWithVerification(0)
	if err == nil {
		t.Error("ConsolidateWithVerification with nil session should return error")
	}
}

func TestConsolidator_ConsolidateWithVerification_NilPlan(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{plan: nil},
	}
	consolidator := NewConsolidator(coord)

	err := consolidator.ConsolidateWithVerification(0)
	if err == nil {
		t.Error("ConsolidateWithVerification with nil plan should return error")
	}
}

func TestConsolidator_ConsolidateWithVerification_InvalidGroupIndex(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			plan: &mockPlan{
				executionOrder: [][]string{{"task-1"}},
			},
		},
	}
	consolidator := NewConsolidator(coord)

	tests := []int{-1, 5, 100}
	for _, idx := range tests {
		t.Run("index"+string(rune(idx+'0')), func(t *testing.T) {
			err := consolidator.ConsolidateWithVerification(idx)
			if err == nil {
				t.Errorf("ConsolidateWithVerification(%d) should return error for invalid index", idx)
			}
		})
	}
}

func TestConsolidator_ConsolidateWithVerification_EmptyGroup(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			plan: &mockPlan{
				executionOrder: [][]string{{}, {"task-1"}},
			},
		},
	}
	consolidator := NewConsolidator(coord)

	err := consolidator.ConsolidateWithVerification(0)
	if err != nil {
		t.Errorf("ConsolidateWithVerification for empty group should return nil, got: %v", err)
	}
}

func TestConsolidator_ConsolidateWithVerification_NoCommits(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			plan: &mockPlan{
				executionOrder: [][]string{{"task-1", "task-2"}},
			},
			taskCommitCounts: map[string]int{
				"task-1": 0,
				"task-2": 0,
			},
		},
	}
	consolidator := NewConsolidator(coord)

	err := consolidator.ConsolidateWithVerification(0)
	if err == nil {
		t.Error("ConsolidateWithVerification with no commits should return error")
	}
}

func TestConsolidator_ConsolidateWithVerification_Success(t *testing.T) {
	wt := &mockWorktree{
		mainBranch:         "main",
		countCommitsResult: 3,
	}
	baseSession := &mockBaseSession{
		instances: []InstanceInterface{
			&mockInstance{id: "inst-1", task: "task-1", branch: "Iron-Ham/task-1"},
		},
	}
	session := &mockSession{
		id: "abc12345",
		plan: &mockPlan{
			executionOrder: [][]string{{"task-1"}},
		},
		taskCommitCounts: map[string]int{
			"task-1": 2,
		},
		tasks: map[string]*mockTask{
			"task-1": {id: "task-1", title: "Implement Feature"},
		},
		config: &mockConfig{branchPrefix: "Iron-Ham"},
	}
	orch := &mockOrchestrator{
		worktree:   wt,
		claudioDir: "/tmp/claudio",
	}
	manager := &mockManager{}

	coord := &mockCoordinator{
		session:      session,
		orchestrator: orch,
		baseSession:  baseSession,
		manager:      manager,
	}
	consolidator := NewConsolidator(coord)

	err := consolidator.ConsolidateWithVerification(0)
	if err != nil {
		t.Errorf("ConsolidateWithVerification error = %v", err)
	}

	// Verify branch was created
	if len(wt.createdBranches) != 1 {
		t.Errorf("Expected 1 branch created, got %d", len(wt.createdBranches))
	}

	// Verify worktree was created and removed
	if len(wt.createdWorktrees) != 1 {
		t.Errorf("Expected 1 worktree created, got %d", len(wt.createdWorktrees))
	}

	// Verify event was emitted
	if len(manager.emittedEvents) == 0 {
		t.Error("Expected event to be emitted")
	}
}

func TestConsolidator_ConsolidateWithVerification_CherryPickError(t *testing.T) {
	wt := &mockWorktree{
		mainBranch:    "main",
		cherryPickErr: errors.New("merge conflict"),
	}
	baseSession := &mockBaseSession{
		instances: []InstanceInterface{
			&mockInstance{id: "inst-1", task: "task-1", branch: "Iron-Ham/task-1"},
		},
	}
	session := &mockSession{
		id: "abc12345",
		plan: &mockPlan{
			executionOrder: [][]string{{"task-1"}},
		},
		taskCommitCounts: map[string]int{
			"task-1": 2,
		},
		tasks: map[string]*mockTask{
			"task-1": {id: "task-1", title: "Task 1"},
		},
		config: &mockConfig{},
	}
	orch := &mockOrchestrator{
		worktree:   wt,
		claudioDir: "/tmp/claudio",
	}

	coord := &mockCoordinator{
		session:      session,
		orchestrator: orch,
		baseSession:  baseSession,
	}
	consolidator := NewConsolidator(coord)

	err := consolidator.ConsolidateWithVerification(0)
	if err == nil {
		t.Error("ConsolidateWithVerification with cherry-pick error should return error")
	}
}

func TestConsolidator_BuildPrompt(t *testing.T) {
	wt := &mockWorktree{mainBranch: "main"}
	baseSession := &mockBaseSession{
		instances: []InstanceInterface{
			&mockInstance{id: "inst-1", task: "task-1", branch: "Iron-Ham/task-1", worktreePath: "/tmp/wt1"},
		},
	}
	session := &mockSession{
		id: "abc12345",
		plan: &mockPlan{
			summary:        "Implement feature X",
			executionOrder: [][]string{{"task-1"}},
		},
		tasks: map[string]*mockTask{
			"task-1": {id: "task-1", title: "Implement Feature"},
		},
		taskCommitCounts: map[string]int{"task-1": 2},
		config:           &mockConfig{branchPrefix: "Iron-Ham"},
	}
	orch := &mockOrchestrator{
		worktree:     wt,
		branchPrefix: "Iron-Ham",
	}

	coord := &mockCoordinator{
		session:      session,
		orchestrator: orch,
		baseSession:  baseSession,
	}
	consolidator := NewConsolidator(coord)

	prompt := consolidator.BuildPrompt(0)

	if prompt == "" {
		t.Error("BuildPrompt returned empty string")
	}

	// Verify prompt contains expected sections
	expectedParts := []string{
		"Group 1 Consolidation",
		"Implement feature X",
		"Objective",
		"Tasks Completed",
		"task-1",
		"Branch Configuration",
	}
	for _, part := range expectedParts {
		if !contains(prompt, part) {
			t.Errorf("BuildPrompt missing expected part: %s", part)
		}
	}
}

func TestConsolidator_BuildPrompt_NilSession(t *testing.T) {
	coord := &mockCoordinator{session: nil}
	consolidator := NewConsolidator(coord)

	prompt := consolidator.BuildPrompt(0)
	if prompt != "" {
		t.Errorf("BuildPrompt with nil session should return empty, got: %s", prompt)
	}
}

func TestConsolidator_BuildPrompt_WithPreviousGroupContext(t *testing.T) {
	wt := &mockWorktree{mainBranch: "main"}
	baseSession := &mockBaseSession{
		instances: []InstanceInterface{
			&mockInstance{id: "inst-1", task: "task-2", branch: "Iron-Ham/task-2", worktreePath: "/tmp/wt2"},
		},
	}
	session := &mockSession{
		id: "abc12345",
		plan: &mockPlan{
			summary:        "Multi-group plan",
			executionOrder: [][]string{{"task-1"}, {"task-2"}},
		},
		tasks: map[string]*mockTask{
			"task-2": {id: "task-2", title: "Task 2"},
		},
		taskCommitCounts:          map[string]int{"task-2": 1},
		config:                    &mockConfig{branchPrefix: "Iron-Ham"},
		groupConsolidatedBranches: []string{"Iron-Ham/ultraplan-abc-group-1"},
		groupConsolidationContexts: []*types.GroupConsolidationCompletionFile{
			{
				Notes:              "Group 1 completed successfully",
				IssuesForNextGroup: []string{"Watch for API compatibility"},
			},
		},
	}
	orch := &mockOrchestrator{
		worktree:     wt,
		branchPrefix: "Iron-Ham",
	}

	coord := &mockCoordinator{
		session:      session,
		orchestrator: orch,
		baseSession:  baseSession,
	}
	consolidator := NewConsolidator(coord)

	prompt := consolidator.BuildPrompt(1)

	// Verify prompt contains previous group context
	if !contains(prompt, "Context from Previous Group") {
		t.Error("BuildPrompt should include previous group context")
	}
	if !contains(prompt, "Group 1 completed successfully") {
		t.Error("BuildPrompt should include previous group notes")
	}
	if !contains(prompt, "Watch for API compatibility") {
		t.Error("BuildPrompt should include previous group issues")
	}
}

func TestConsolidator_GatherTaskCompletionContext_EmptyGroup(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			plan: &mockPlan{
				executionOrder: [][]string{{}},
			},
		},
		baseSession: &mockBaseSession{},
	}
	consolidator := NewConsolidator(coord)

	ctx := consolidator.GatherTaskCompletionContext(0)
	if ctx == nil {
		t.Fatal("GatherTaskCompletionContext should return non-nil context")
	}
	if len(ctx.TaskSummaries) != 0 {
		t.Errorf("Expected empty TaskSummaries, got %d", len(ctx.TaskSummaries))
	}
}

func TestConsolidator_GetTaskBranches(t *testing.T) {
	baseSession := &mockBaseSession{
		instances: []InstanceInterface{
			&mockInstance{id: "inst-1", task: "task-1", branch: "Iron-Ham/task-1", worktreePath: "/tmp/wt1"},
			&mockInstance{id: "inst-2", task: "task-2", branch: "Iron-Ham/task-2", worktreePath: "/tmp/wt2"},
		},
	}
	session := &mockSession{
		plan: &mockPlan{
			executionOrder: [][]string{{"task-1", "task-2"}},
		},
		tasks: map[string]*mockTask{
			"task-1": {id: "task-1", title: "Task 1"},
			"task-2": {id: "task-2", title: "Task 2"},
		},
	}

	coord := &mockCoordinator{
		session:     session,
		baseSession: baseSession,
	}
	consolidator := NewConsolidator(coord)

	branches := consolidator.GetTaskBranches(0)
	if len(branches) != 2 {
		t.Errorf("GetTaskBranches returned %d branches, want 2", len(branches))
	}
}

func TestConsolidator_GetTaskBranches_InvalidGroup(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			plan: &mockPlan{
				executionOrder: [][]string{{"task-1"}},
			},
		},
	}
	consolidator := NewConsolidator(coord)

	branches := consolidator.GetTaskBranches(5)
	if branches != nil {
		t.Errorf("GetTaskBranches for invalid group should return nil, got %v", branches)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Implement Feature", "implement-feature"},
		{"UPPERCASE", "uppercase"},
		{"already-slugified", "already-slugified"},
		{"Multiple   Spaces", "multiple---spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := slugify(tt.input)
			if result != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestConsolidator_StartSession_NilSession(t *testing.T) {
	coord := &mockCoordinator{session: nil}
	consolidator := NewConsolidator(coord)

	err := consolidator.StartSession(0)
	if err == nil {
		t.Error("StartSession with nil session should return error")
	}
}

func TestConsolidator_StartSession_InvalidGroup(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			plan: &mockPlan{
				executionOrder: [][]string{{"task-1"}},
			},
		},
	}
	consolidator := NewConsolidator(coord)

	err := consolidator.StartSession(5)
	if err == nil {
		t.Error("StartSession with invalid group index should return error")
	}
}

func TestConsolidator_StartSession_NoVerifiedCommits(t *testing.T) {
	coord := &mockCoordinator{
		session: &mockSession{
			plan: &mockPlan{
				executionOrder: [][]string{{"task-1"}},
			},
			taskCommitCounts: map[string]int{
				"task-1": 0, // No commits
			},
		},
	}
	consolidator := NewConsolidator(coord)

	err := consolidator.StartSession(0)
	if err == nil {
		t.Error("StartSession with no verified commits should return error")
	}
}

func TestConsolidator_Monitor_ContextCancelled(t *testing.T) {
	done := make(chan struct{})
	close(done) // Already cancelled

	coord := &mockCoordinator{
		session: &mockSession{},
		orchestrator: &mockOrchestrator{
			instances: map[string]*mockInstance{
				"inst-1": {id: "inst-1", status: "running"},
			},
		},
		ctx: &mockContext{done: done},
	}
	consolidator := NewConsolidator(coord)

	err := consolidator.Monitor(0, "inst-1")
	if err == nil {
		t.Error("Monitor with cancelled context should return error")
	}
}

func TestConsolidator_Monitor_InstanceNotFound(t *testing.T) {
	coord := &mockCoordinator{
		session:      &mockSession{},
		orchestrator: &mockOrchestrator{instances: map[string]*mockInstance{}},
		ctx:          &mockContext{done: make(chan struct{})},
	}
	consolidator := NewConsolidator(coord)

	// This should return quickly with an error since instance is not found
	go func() {
		// Close context after a short delay to prevent infinite loop
		close(coord.ctx.done)
	}()

	err := consolidator.Monitor(0, "nonexistent")
	if err == nil {
		t.Error("Monitor with nonexistent instance should return error")
	}
}

func TestNewConsolidator(t *testing.T) {
	coord := &mockCoordinator{}
	consolidator := NewConsolidator(coord)

	if consolidator == nil {
		t.Error("NewConsolidator returned nil")
	}
}
