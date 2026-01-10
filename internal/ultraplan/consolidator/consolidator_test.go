package consolidator

import (
	"errors"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// mockWorktreeManager provides a test implementation of WorktreeManager.
type mockWorktreeManager struct {
	mainBranch        string
	createdBranches   map[string]string // branch -> baseBranch
	createdWorktrees  map[string]string // path -> branch
	cherryPickError   error
	conflictFiles     []string
	changedFiles      []string
	initialCommits    int
	finalCommits      int
	pushError         error
	removeError       error
	countCommitsCalls int
}

func newMockWorktreeManager() *mockWorktreeManager {
	return &mockWorktreeManager{
		mainBranch:       "main",
		createdBranches:  make(map[string]string),
		createdWorktrees: make(map[string]string),
		initialCommits:   0,
		finalCommits:     2,
	}
}

func (m *mockWorktreeManager) FindMainBranch() string {
	return m.mainBranch
}

func (m *mockWorktreeManager) CreateBranchFrom(newBranch, baseBranch string) error {
	m.createdBranches[newBranch] = baseBranch
	return nil
}

func (m *mockWorktreeManager) CreateWorktreeFromBranch(path, branch string) error {
	m.createdWorktrees[path] = branch
	return nil
}

func (m *mockWorktreeManager) CherryPickBranch(_, _ string) error {
	return m.cherryPickError
}

func (m *mockWorktreeManager) ContinueCherryPick(_ string) error {
	return nil
}

func (m *mockWorktreeManager) Push(_ string, _ bool) error {
	return m.pushError
}

func (m *mockWorktreeManager) Remove(_ string) error {
	return m.removeError
}

func (m *mockWorktreeManager) GetConflictingFiles(_ string) ([]string, error) {
	return m.conflictFiles, nil
}

func (m *mockWorktreeManager) GetChangedFiles(_ string) ([]string, error) {
	return m.changedFiles, nil
}

func (m *mockWorktreeManager) CountCommitsBetween(_, _, _ string) (int, error) {
	m.countCommitsCalls++
	// First call returns initial, subsequent calls return final
	if m.countCommitsCalls == 1 {
		return m.initialCommits, nil
	}
	return m.finalCommits, nil
}

// mockPRCreator provides a test implementation of PRCreator.
type mockPRCreator struct {
	createdPRs []createdPR
	prURL      string
	prError    error
}

type createdPR struct {
	title      string
	body       string
	branch     string
	baseBranch string
	draft      bool
	labels     []string
}

func newMockPRCreator() *mockPRCreator {
	return &mockPRCreator{
		createdPRs: make([]createdPR, 0),
		prURL:      "https://github.com/test/repo/pull/1",
	}
}

func (m *mockPRCreator) CreatePR(title, body, branch, baseBranch string, draft bool, labels []string) (string, error) {
	if m.prError != nil {
		return "", m.prError
	}
	m.createdPRs = append(m.createdPRs, createdPR{
		title:      title,
		body:       body,
		branch:     branch,
		baseBranch: baseBranch,
		draft:      draft,
		labels:     labels,
	})
	return m.prURL, nil
}

func TestMode_Constants(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModeStacked, "stacked"},
		{ModeSingle, "single"},
	}

	for _, tc := range tests {
		if string(tc.mode) != tc.want {
			t.Errorf("Mode %q != %q", tc.mode, tc.want)
		}
	}
}

func TestPhase_Constants(t *testing.T) {
	tests := []struct {
		phase Phase
		want  string
	}{
		{PhaseIdle, "idle"},
		{PhaseDetecting, "detecting_conflicts"},
		{PhaseCreatingBranches, "creating_branches"},
		{PhaseMergingTasks, "merging_tasks"},
		{PhasePushing, "pushing"},
		{PhaseCreatingPRs, "creating_prs"},
		{PhasePaused, "paused"},
		{PhaseComplete, "complete"},
		{PhaseFailed, "failed"},
	}

	for _, tc := range tests {
		if string(tc.phase) != tc.want {
			t.Errorf("Phase %q != %q", tc.phase, tc.want)
		}
	}
}

func TestEventType_Constants(t *testing.T) {
	tests := []struct {
		eventType EventType
		want      string
	}{
		{EventStarted, "consolidation_started"},
		{EventGroupStarted, "consolidation_group_started"},
		{EventTaskMerging, "consolidation_task_merging"},
		{EventTaskMerged, "consolidation_task_merged"},
		{EventGroupComplete, "consolidation_group_complete"},
		{EventPRCreating, "consolidation_pr_creating"},
		{EventPRCreated, "consolidation_pr_created"},
		{EventConflict, "consolidation_conflict"},
		{EventComplete, "consolidation_complete"},
		{EventFailed, "consolidation_failed"},
	}

	for _, tc := range tests {
		if string(tc.eventType) != tc.want {
			t.Errorf("EventType %q != %q", tc.eventType, tc.want)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Mode != ModeStacked {
		t.Errorf("Expected Mode %q, got %q", ModeStacked, cfg.Mode)
	}
	if cfg.BranchPrefix != "" {
		t.Errorf("Expected empty BranchPrefix, got %q", cfg.BranchPrefix)
	}
	if !cfg.CreateDraftPRs {
		t.Error("Expected CreateDraftPRs to be true")
	}
	if len(cfg.PRLabels) != 1 || cfg.PRLabels[0] != "ultraplan" {
		t.Errorf("Expected PRLabels [\"ultraplan\"], got %v", cfg.PRLabels)
	}
}

func TestNewConsolidator(t *testing.T) {
	session := ultraplan.NewSession("Test objective", ultraplan.DefaultConfig())
	cfg := DefaultConfig()
	wt := newMockWorktreeManager()
	pr := newMockPRCreator()

	c := NewConsolidator(session, cfg, wt, pr, nil)

	if c == nil {
		t.Fatal("NewConsolidator returned nil")
	}

	state := c.State()
	if state.Phase != PhaseIdle {
		t.Errorf("Expected initial phase %q, got %q", PhaseIdle, state.Phase)
	}
}

func TestConsolidator_SetEventCallback(t *testing.T) {
	session := ultraplan.NewSession("Test", ultraplan.DefaultConfig())
	c := NewConsolidator(session, DefaultConfig(), newMockWorktreeManager(), newMockPRCreator(), nil)

	var receivedEvent *Event
	c.SetEventCallback(func(e Event) {
		receivedEvent = &e
	})

	// Emit an event internally
	c.emitEvent(Event{
		Type:    EventStarted,
		Message: "test message",
	})

	if receivedEvent == nil {
		t.Fatal("Event callback was not called")
	}
	if receivedEvent.Type != EventStarted {
		t.Errorf("Expected event type %q, got %q", EventStarted, receivedEvent.Type)
	}
	if receivedEvent.Message != "test message" {
		t.Errorf("Expected message %q, got %q", "test message", receivedEvent.Message)
	}
	if receivedEvent.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}

func TestConsolidator_SetTaskBranch(t *testing.T) {
	session := ultraplan.NewSession("Test", ultraplan.DefaultConfig())
	c := NewConsolidator(session, DefaultConfig(), newMockWorktreeManager(), newMockPRCreator(), nil)

	c.SetTaskBranch("task-1", "branch-1")
	c.SetTaskBranch("task-2", "branch-2")

	// Verify branches are set (internal state)
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.taskBranches["task-1"] != "branch-1" {
		t.Errorf("Expected task-1 -> branch-1, got %q", c.taskBranches["task-1"])
	}
	if c.taskBranches["task-2"] != "branch-2" {
		t.Errorf("Expected task-2 -> branch-2, got %q", c.taskBranches["task-2"])
	}
}

func TestConsolidator_State(t *testing.T) {
	session := ultraplan.NewSession("Test", ultraplan.DefaultConfig())
	c := NewConsolidator(session, DefaultConfig(), newMockWorktreeManager(), newMockPRCreator(), nil)

	// Get initial state
	state := c.State()
	if state.Phase != PhaseIdle {
		t.Errorf("Expected phase %q, got %q", PhaseIdle, state.Phase)
	}

	// Modify internal state
	c.mu.Lock()
	c.state.Phase = PhaseCreatingBranches
	c.state.CurrentGroup = 1
	c.mu.Unlock()

	// Verify State() returns updated values
	state = c.State()
	if state.Phase != PhaseCreatingBranches {
		t.Errorf("Expected phase %q, got %q", PhaseCreatingBranches, state.Phase)
	}
	if state.CurrentGroup != 1 {
		t.Errorf("Expected CurrentGroup 1, got %d", state.CurrentGroup)
	}
}

func TestConsolidator_Results(t *testing.T) {
	session := ultraplan.NewSession("Test", ultraplan.DefaultConfig())
	c := NewConsolidator(session, DefaultConfig(), newMockWorktreeManager(), newMockPRCreator(), nil)

	// Initially empty
	results := c.Results()
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}

	// Add a result
	c.mu.Lock()
	c.results = append(c.results, &GroupResult{
		GroupIndex: 0,
		TaskIDs:    []string{"task-1"},
		BranchName: "branch-1",
		Success:    true,
	})
	c.mu.Unlock()

	results = c.Results()
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].GroupIndex != 0 {
		t.Errorf("Expected GroupIndex 0, got %d", results[0].GroupIndex)
	}
}

func TestConsolidator_Stop(t *testing.T) {
	session := ultraplan.NewSession("Test", ultraplan.DefaultConfig())
	c := NewConsolidator(session, DefaultConfig(), newMockWorktreeManager(), newMockPRCreator(), nil)

	// Stop should be idempotent
	c.Stop()
	c.Stop()

	c.mu.RLock()
	stopped := c.stopped
	c.mu.RUnlock()

	if !stopped {
		t.Error("Expected consolidator to be stopped")
	}
}

func TestConsolidator_Run_MissingBranchMapping(t *testing.T) {
	session := ultraplan.NewSession("Test", ultraplan.DefaultConfig())
	session.CompletedTasks = []string{"task-1"}

	c := NewConsolidator(session, DefaultConfig(), newMockWorktreeManager(), newMockPRCreator(), nil)
	// Don't set task branch mapping

	err := c.Run("/tmp/base")
	if err == nil {
		t.Error("Expected error for missing branch mapping")
	}

	state := c.State()
	if state.Phase != PhaseFailed {
		t.Errorf("Expected phase %q after failure, got %q", PhaseFailed, state.Phase)
	}
}

func TestConsolidator_Run_SingleMode(t *testing.T) {
	session := ultraplan.NewSession("Test objective", ultraplan.DefaultConfig())
	session.Plan = &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
		},
		ExecutionOrder: [][]string{
			{"task-1", "task-2"},
		},
	}
	session.CompletedTasks = []string{"task-1", "task-2"}

	wt := newMockWorktreeManager()
	wt.initialCommits = 0
	wt.finalCommits = 2
	wt.changedFiles = []string{"file1.go", "file2.go"}

	pr := newMockPRCreator()

	cfg := DefaultConfig()
	cfg.Mode = ModeSingle

	c := NewConsolidator(session, cfg, wt, pr, nil)
	c.SetTaskBranch("task-1", "feature/task-1")
	c.SetTaskBranch("task-2", "feature/task-2")

	err := c.Run("/tmp/base")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	state := c.State()
	if state.Phase != PhaseComplete {
		t.Errorf("Expected phase %q, got %q", PhaseComplete, state.Phase)
	}

	// Verify PR was created
	if len(pr.createdPRs) != 1 {
		t.Errorf("Expected 1 PR, got %d", len(pr.createdPRs))
	}

	// Verify worktree was created
	if len(wt.createdWorktrees) != 1 {
		t.Errorf("Expected 1 worktree, got %d", len(wt.createdWorktrees))
	}
}

func TestConsolidator_Run_StackedMode_WithPreconsolidatedBranches(t *testing.T) {
	session := ultraplan.NewSession("Test objective", ultraplan.DefaultConfig())
	session.Plan = &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
		},
		ExecutionOrder: [][]string{
			{"task-1"},
		},
	}
	session.CompletedTasks = []string{"task-1"}
	session.GroupConsolidatedBranches = []string{"preconsolidated-branch-1"}

	wt := newMockWorktreeManager()
	pr := newMockPRCreator()

	c := NewConsolidator(session, DefaultConfig(), wt, pr, nil)
	c.SetTaskBranch("task-1", "feature/task-1")

	err := c.Run("/tmp/base")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	state := c.State()
	if state.Phase != PhaseComplete {
		t.Errorf("Expected phase %q, got %q", PhaseComplete, state.Phase)
	}

	// Should use pre-consolidated branch, not create new ones
	if len(wt.createdBranches) != 0 {
		t.Errorf("Expected 0 created branches (using preconsolidated), got %d", len(wt.createdBranches))
	}
}

func TestConsolidator_Run_PRCreationFailure(t *testing.T) {
	session := ultraplan.NewSession("Test", ultraplan.DefaultConfig())
	session.Plan = &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
		},
		ExecutionOrder: [][]string{
			{"task-1"},
		},
	}
	session.CompletedTasks = []string{"task-1"}
	session.GroupConsolidatedBranches = []string{"branch-1"}

	wt := newMockWorktreeManager()
	pr := newMockPRCreator()
	pr.prError = errors.New("PR creation failed")

	c := NewConsolidator(session, DefaultConfig(), wt, pr, nil)
	c.SetTaskBranch("task-1", "feature/task-1")

	err := c.Run("/tmp/base")
	if err == nil {
		t.Error("Expected error for PR creation failure")
	}

	state := c.State()
	if state.Phase != PhaseFailed {
		t.Errorf("Expected phase %q, got %q", PhaseFailed, state.Phase)
	}
}

func TestConsolidator_GenerateBranchNames(t *testing.T) {
	session := ultraplan.NewSession("Test", ultraplan.DefaultConfig())
	session.ID = "12345678abcdef"

	cfg := DefaultConfig()
	cfg.BranchPrefix = "custom-prefix"

	c := NewConsolidator(session, cfg, newMockWorktreeManager(), newMockPRCreator(), nil)

	// Test group branch name
	groupBranch := c.generateGroupBranchName(0)
	expected := "custom-prefix/ultraplan-12345678-group-1"
	if groupBranch != expected {
		t.Errorf("Expected group branch %q, got %q", expected, groupBranch)
	}

	// Test single branch name
	singleBranch := c.generateSingleBranchName()
	expected = "custom-prefix/ultraplan-12345678"
	if singleBranch != expected {
		t.Errorf("Expected single branch %q, got %q", expected, singleBranch)
	}
}

func TestConsolidator_GenerateBranchNames_DefaultPrefix(t *testing.T) {
	session := ultraplan.NewSession("Test", ultraplan.DefaultConfig())
	session.ID = "abcdef12"

	cfg := DefaultConfig()
	cfg.BranchPrefix = "" // Empty, should use default

	c := NewConsolidator(session, cfg, newMockWorktreeManager(), newMockPRCreator(), nil)

	groupBranch := c.generateGroupBranchName(2)
	expected := "Iron-Ham/ultraplan-abcdef12-group-3"
	if groupBranch != expected {
		t.Errorf("Expected group branch %q, got %q", expected, groupBranch)
	}
}

func TestConsolidator_BuildPRContent(t *testing.T) {
	session := ultraplan.NewSession("Short objective", ultraplan.DefaultConfig())
	cfg := DefaultConfig()

	c := NewConsolidator(session, cfg, newMockWorktreeManager(), newMockPRCreator(), nil)
	c.mu.Lock()
	c.state.TotalGroups = 3
	c.mu.Unlock()

	// Test stacked mode PR content
	title, body := c.buildPRContent(0)
	if title != "ultraplan: group 1 - Short objective" {
		t.Errorf("Unexpected title: %q", title)
	}
	if body == "" {
		t.Error("Expected non-empty body")
	}

	// Test single mode PR content
	cfg.Mode = ModeSingle
	c2 := NewConsolidator(session, cfg, newMockWorktreeManager(), newMockPRCreator(), nil)
	title, _ = c2.buildPRContent(0)
	if title != "ultraplan: Short objective" {
		t.Errorf("Unexpected single mode title: %q", title)
	}
}

func TestConsolidator_BuildPRContent_LongObjective(t *testing.T) {
	longObjective := "This is a very long objective that exceeds fifty characters and should be truncated"
	session := ultraplan.NewSession(longObjective, ultraplan.DefaultConfig())

	cfg := DefaultConfig()
	cfg.Mode = ModeSingle

	c := NewConsolidator(session, cfg, newMockWorktreeManager(), newMockPRCreator(), nil)
	title, _ := c.buildPRContent(0)

	// Objective should be truncated to 47 chars + "..." when > 50 chars
	// "This is a very long objective that exceeds fift" = 47 chars
	expectedTitle := "ultraplan: This is a very long objective that exceeds fift..."
	if title != expectedTitle {
		t.Errorf("Expected title %q, got: %q", expectedTitle, title)
	}
}

func TestConflictError(t *testing.T) {
	err := &ConflictError{
		TaskID:       "task-1",
		Branch:       "feature/task-1",
		Files:        []string{"file1.go", "file2.go"},
		WorktreePath: "/tmp/worktree",
		Underlying:   errors.New("cherry-pick failed"),
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Expected non-empty error message")
	}

	if err.TaskID != "task-1" {
		t.Errorf("Expected TaskID %q, got %q", "task-1", err.TaskID)
	}
	if len(err.Files) != 2 {
		t.Errorf("Expected 2 conflict files, got %d", len(err.Files))
	}
}

func TestState_Fields(t *testing.T) {
	now := time.Now()
	state := State{
		Phase:            PhaseComplete,
		CurrentGroup:     2,
		TotalGroups:      3,
		CurrentTask:      "task-1",
		GroupBranches:    []string{"branch-1", "branch-2"},
		PRUrls:           []string{"https://github.com/test/repo/pull/1"},
		ConflictFiles:    []string{"conflict.go"},
		ConflictTaskID:   "task-conflict",
		ConflictWorktree: "/tmp/conflict",
		Error:            "some error",
		StartedAt:        &now,
		CompletedAt:      &now,
	}

	if state.Phase != PhaseComplete {
		t.Errorf("Expected Phase %q, got %q", PhaseComplete, state.Phase)
	}
	if state.CurrentGroup != 2 {
		t.Errorf("Expected CurrentGroup 2, got %d", state.CurrentGroup)
	}
	if len(state.GroupBranches) != 2 {
		t.Errorf("Expected 2 GroupBranches, got %d", len(state.GroupBranches))
	}
	if state.StartedAt == nil || state.CompletedAt == nil {
		t.Error("Expected timestamps to be set")
	}
}

func TestGroupResult_Fields(t *testing.T) {
	result := GroupResult{
		GroupIndex:   1,
		TaskIDs:      []string{"task-1", "task-2"},
		BranchName:   "group-branch",
		CommitCount:  5,
		FilesChanged: []string{"file1.go", "file2.go"},
		PRUrl:        "https://github.com/test/repo/pull/1",
		Success:      true,
		Error:        "",
	}

	if result.GroupIndex != 1 {
		t.Errorf("Expected GroupIndex 1, got %d", result.GroupIndex)
	}
	if len(result.TaskIDs) != 2 {
		t.Errorf("Expected 2 TaskIDs, got %d", len(result.TaskIDs))
	}
	if result.CommitCount != 5 {
		t.Errorf("Expected CommitCount 5, got %d", result.CommitCount)
	}
	if !result.Success {
		t.Error("Expected Success to be true")
	}
}

func TestEvent_Fields(t *testing.T) {
	now := time.Now()
	event := Event{
		Type:      EventTaskMerged,
		GroupIdx:  1,
		TaskID:    "task-1",
		Message:   "Task merged successfully",
		Timestamp: now,
	}

	if event.Type != EventTaskMerged {
		t.Errorf("Expected Type %q, got %q", EventTaskMerged, event.Type)
	}
	if event.GroupIdx != 1 {
		t.Errorf("Expected GroupIdx 1, got %d", event.GroupIdx)
	}
	if event.TaskID != "task-1" {
		t.Errorf("Expected TaskID %q, got %q", "task-1", event.TaskID)
	}
	if event.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}
}
