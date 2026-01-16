package consolidation

import (
	"testing"
)

// mockSessionState implements SessionState for testing
type mockSessionState struct {
	planSummary                string
	executionOrder             [][]string
	taskCommitCounts           map[string]int
	tasks                      map[string]any
	groupConsolidatorIDs       []string
	groupConsolidatedBranches  []string
	groupConsolidationContexts []*GroupConsolidationCompletion
	branchPrefix               string
	sessionID                  string
	multiPass                  bool
}

func (m *mockSessionState) GetPlanSummary() string              { return m.planSummary }
func (m *mockSessionState) GetExecutionOrder() [][]string       { return m.executionOrder }
func (m *mockSessionState) GetTaskCommitCounts() map[string]int { return m.taskCommitCounts }
func (m *mockSessionState) GetTask(taskID string) any {
	if m.tasks == nil {
		return nil
	}
	return m.tasks[taskID]
}
func (m *mockSessionState) GetGroupConsolidatorIDs() []string { return m.groupConsolidatorIDs }
func (m *mockSessionState) SetGroupConsolidatorID(groupIndex int, instanceID string) {
	for len(m.groupConsolidatorIDs) <= groupIndex {
		m.groupConsolidatorIDs = append(m.groupConsolidatorIDs, "")
	}
	m.groupConsolidatorIDs[groupIndex] = instanceID
}
func (m *mockSessionState) GetGroupConsolidatedBranches() []string {
	return m.groupConsolidatedBranches
}
func (m *mockSessionState) SetGroupConsolidatedBranch(groupIndex int, branchName string) {
	for len(m.groupConsolidatedBranches) <= groupIndex {
		m.groupConsolidatedBranches = append(m.groupConsolidatedBranches, "")
	}
	m.groupConsolidatedBranches[groupIndex] = branchName
}
func (m *mockSessionState) GetGroupConsolidationContexts() []*GroupConsolidationCompletion {
	return m.groupConsolidationContexts
}
func (m *mockSessionState) SetGroupConsolidationContext(groupIndex int, context *GroupConsolidationCompletion) {
	for len(m.groupConsolidationContexts) <= groupIndex {
		m.groupConsolidationContexts = append(m.groupConsolidationContexts, nil)
	}
	m.groupConsolidationContexts[groupIndex] = context
}
func (m *mockSessionState) GetBranchPrefix() string { return m.branchPrefix }
func (m *mockSessionState) GetSessionID() string    { return m.sessionID }
func (m *mockSessionState) IsMultiPass() bool       { return m.multiPass }

// mockInstanceInfo implements InstanceInfo for testing
type mockInstanceInfo struct {
	id           string
	worktreePath string
	branch       string
	task         string
}

func (i *mockInstanceInfo) GetID() string           { return i.id }
func (i *mockInstanceInfo) GetWorktreePath() string { return i.worktreePath }
func (i *mockInstanceInfo) GetBranch() string       { return i.branch }
func (i *mockInstanceInfo) GetTask() string         { return i.task }

// mockInstanceStore implements InstanceStore for testing
type mockInstanceStore struct {
	instances []InstanceInfo
}

func (s *mockInstanceStore) GetInstances() []InstanceInfo { return s.instances }

// mockTask implements a task with title
type mockTask struct {
	id    string
	title string
}

func (t *mockTask) GetTitle() string { return t.title }

func TestNewGroupManager(t *testing.T) {
	t.Run("with nil context", func(t *testing.T) {
		m := NewGroupManager(nil)
		if m == nil {
			t.Fatal("NewGroupManager returned nil")
		}
	})

	t.Run("with context", func(t *testing.T) {
		ctx := &Context{
			Session: &mockSessionState{},
		}
		m := NewGroupManager(ctx)
		if m == nil {
			t.Fatal("NewGroupManager returned nil")
		}
	})
}

func TestGroupManager_GetBaseBranchForGroup(t *testing.T) {
	tests := []struct {
		name       string
		session    *mockSessionState
		groupIndex int
		want       string
	}{
		{
			name:       "group 0 returns empty (use default)",
			session:    &mockSessionState{},
			groupIndex: 0,
			want:       "",
		},
		{
			name: "group 1 with previous consolidated branch",
			session: &mockSessionState{
				groupConsolidatedBranches: []string{"consolidated-group-1"},
			},
			groupIndex: 1,
			want:       "consolidated-group-1",
		},
		{
			name: "group 2 with multiple consolidated branches",
			session: &mockSessionState{
				groupConsolidatedBranches: []string{"consolidated-group-1", "consolidated-group-2"},
			},
			groupIndex: 2,
			want:       "consolidated-group-2",
		},
		{
			name: "group 1 without previous branch returns empty",
			session: &mockSessionState{
				groupConsolidatedBranches: []string{},
			},
			groupIndex: 1,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{Session: tt.session}
			m := NewGroupManager(ctx)
			got := m.GetBaseBranchForGroup(tt.groupIndex)
			if got != tt.want {
				t.Errorf("GetBaseBranchForGroup(%d) = %q, want %q", tt.groupIndex, got, tt.want)
			}
		})
	}
}

func TestGroupManager_GetTaskBranchesForGroup(t *testing.T) {
	t.Run("returns task branches", func(t *testing.T) {
		session := &mockSessionState{
			executionOrder: [][]string{{"task-1", "task-2"}},
			tasks: map[string]any{
				"task-1": &mockTask{id: "task-1", title: "Task 1"},
				"task-2": &mockTask{id: "task-2", title: "Task 2"},
			},
		}
		instanceStore := &mockInstanceStore{
			instances: []InstanceInfo{
				&mockInstanceInfo{id: "inst-1", task: "task-1", worktreePath: "/wt1", branch: "branch-1"},
				&mockInstanceInfo{id: "inst-2", task: "task-2", worktreePath: "/wt2", branch: "branch-2"},
			},
		}

		ctx := &Context{
			Session:       session,
			InstanceStore: instanceStore,
		}
		m := NewGroupManager(ctx)
		branches := m.GetTaskBranchesForGroup(0)

		if len(branches) != 2 {
			t.Fatalf("expected 2 branches, got %d", len(branches))
		}

		if branches[0].TaskID != "task-1" {
			t.Errorf("expected task-1, got %s", branches[0].TaskID)
		}
		if branches[0].TaskTitle != "Task 1" {
			t.Errorf("expected 'Task 1', got %s", branches[0].TaskTitle)
		}
		if branches[0].Branch != "branch-1" {
			t.Errorf("expected branch-1, got %s", branches[0].Branch)
		}
	})

	t.Run("returns nil for invalid group index", func(t *testing.T) {
		session := &mockSessionState{
			executionOrder: [][]string{{"task-1"}},
		}
		ctx := &Context{Session: session}
		m := NewGroupManager(ctx)
		branches := m.GetTaskBranchesForGroup(5)

		if branches != nil {
			t.Error("expected nil for invalid group index")
		}
	})
}

func TestGroupManager_GatherTaskCompletionContextForGroup(t *testing.T) {
	t.Run("returns empty context when no instances", func(t *testing.T) {
		session := &mockSessionState{
			executionOrder: [][]string{{"task-1"}},
		}
		ctx := &Context{Session: session}
		m := NewGroupManager(ctx)

		taskCtx := m.GatherTaskCompletionContextForGroup(0)

		if taskCtx == nil {
			t.Fatal("expected non-nil context")
		}
		if taskCtx.TaskSummaries == nil {
			t.Error("expected TaskSummaries to be initialized")
		}
	})

	t.Run("returns empty context for invalid group", func(t *testing.T) {
		session := &mockSessionState{
			executionOrder: [][]string{{"task-1"}},
		}
		ctx := &Context{Session: session}
		m := NewGroupManager(ctx)

		taskCtx := m.GatherTaskCompletionContextForGroup(10)

		if taskCtx == nil {
			t.Fatal("expected non-nil context")
		}
		if len(taskCtx.TaskSummaries) != 0 {
			t.Error("expected empty TaskSummaries")
		}
	})
}

func TestGroupManager_ConsolidateGroupWithVerification(t *testing.T) {
	t.Run("fails for invalid group index", func(t *testing.T) {
		session := &mockSessionState{
			executionOrder: [][]string{{"task-1"}},
		}
		ctx := &Context{Session: session}
		m := NewGroupManager(ctx)

		err := m.ConsolidateGroupWithVerification(10)
		if err == nil {
			t.Error("expected error for invalid group index")
		}
	})

	t.Run("fails when no commits for tasks", func(t *testing.T) {
		session := &mockSessionState{
			executionOrder:   [][]string{{"task-1", "task-2"}},
			taskCommitCounts: map[string]int{}, // No commits
			tasks: map[string]any{
				"task-1": &mockTask{id: "task-1", title: "Task 1"},
				"task-2": &mockTask{id: "task-2", title: "Task 2"},
			},
		}
		instanceStore := &mockInstanceStore{
			instances: []InstanceInfo{
				&mockInstanceInfo{id: "inst-1", task: "task-1", worktreePath: "/wt1", branch: "branch-1"},
			},
		}

		ctx := &Context{
			Session:       session,
			InstanceStore: instanceStore,
		}
		m := NewGroupManager(ctx)

		err := m.ConsolidateGroupWithVerification(0)
		if err == nil {
			t.Error("expected error when no commits")
		}
	})
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Task 1: Add feature", "task-1-add-feature"},
		{"UPPERCASE", "uppercase"},
		{"with-dashes", "with-dashes"},
		{"with_underscores", "withunderscores"},
		{"special!@#$chars", "specialchars"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGroupManager_BuildGroupConsolidatorPrompt(t *testing.T) {
	t.Run("builds prompt with task info", func(t *testing.T) {
		session := &mockSessionState{
			planSummary:    "Test Plan",
			executionOrder: [][]string{{"task-1", "task-2"}},
			tasks: map[string]any{
				"task-1": &mockTask{id: "task-1", title: "Task 1"},
				"task-2": &mockTask{id: "task-2", title: "Task 2"},
			},
			taskCommitCounts: map[string]int{
				"task-1": 2,
				"task-2": 1,
			},
			sessionID:    "test-session-123",
			branchPrefix: "test-prefix",
		}
		instanceStore := &mockInstanceStore{
			instances: []InstanceInfo{
				&mockInstanceInfo{id: "inst-1", task: "task-1", worktreePath: "/wt1", branch: "branch-1"},
				&mockInstanceInfo{id: "inst-2", task: "task-2", worktreePath: "/wt2", branch: "branch-2"},
			},
		}

		// Simple mock worktree that just returns a value
		ctx := &Context{
			Session:       session,
			InstanceStore: instanceStore,
		}
		m := NewGroupManager(ctx)

		prompt := m.buildGroupConsolidatorPrompt(0)

		if prompt == "" {
			t.Error("expected non-empty prompt")
		}

		// Check for expected content
		if !containsString(prompt, "Group 1 Consolidation") {
			t.Error("expected 'Group 1 Consolidation' in prompt")
		}
		if !containsString(prompt, "Test Plan") {
			t.Error("expected plan summary in prompt")
		}
		if !containsString(prompt, "task-1") {
			t.Error("expected task-1 in prompt")
		}
		if !containsString(prompt, "Task 1") {
			t.Error("expected Task 1 title in prompt")
		}
	})

	t.Run("includes previous group context", func(t *testing.T) {
		session := &mockSessionState{
			planSummary:    "Test Plan",
			executionOrder: [][]string{{"task-1"}, {"task-2"}},
			tasks: map[string]any{
				"task-1": &mockTask{id: "task-1", title: "Task 1"},
				"task-2": &mockTask{id: "task-2", title: "Task 2"},
			},
			groupConsolidatedBranches: []string{"consolidated-1"},
			groupConsolidationContexts: []*GroupConsolidationCompletion{
				{
					GroupIndex: 0,
					Notes:      "Watch out for API changes",
					IssuesForNextGroup: []string{
						"Update imports",
						"Check tests",
					},
				},
			},
			sessionID:    "test-123",
			branchPrefix: "test",
		}
		instanceStore := &mockInstanceStore{
			instances: []InstanceInfo{
				&mockInstanceInfo{id: "inst-2", task: "task-2", worktreePath: "/wt2", branch: "branch-2"},
			},
		}

		ctx := &Context{
			Session:       session,
			InstanceStore: instanceStore,
		}
		m := NewGroupManager(ctx)

		prompt := m.buildGroupConsolidatorPrompt(1)

		if !containsString(prompt, "Context from Previous Group") {
			t.Error("expected previous group context section")
		}
		if !containsString(prompt, "Watch out for API changes") {
			t.Error("expected previous group notes")
		}
		if !containsString(prompt, "Update imports") {
			t.Error("expected previous group issues")
		}
	})
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
