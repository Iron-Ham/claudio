package strategy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
)

// mockBranchOps is a mock implementation of BranchOps.
type mockBranchOps struct {
	mainBranch          string
	groupBranchName     string
	groupBranchNames    []string // For multi-group tests
	singleBranchName    string
	createGroupErr      error
	createSingleErr     error
	createWorktreeErr   error
	cherryPickErr       error
	changedFiles        []string
	changedFilesErr     error
	commitCount         int
	commitCountErr      error
	pushErr             error
	removeWorktreeErr   error
	createWorktreeCall  struct{ path, branch string }
	cherryPickCalls     []struct{ path, branch string }
	removeWorktreeCalls []string
	createGroupCalls    []struct {
		groupIdx   int
		baseBranch string
	}
}

func (m *mockBranchOps) FindMainBranch(_ context.Context) string {
	if m.mainBranch == "" {
		return "main"
	}
	return m.mainBranch
}

func (m *mockBranchOps) CreateGroupBranch(_ context.Context, groupIdx int, baseBranch string) (string, error) {
	m.createGroupCalls = append(m.createGroupCalls, struct {
		groupIdx   int
		baseBranch string
	}{groupIdx, baseBranch})
	if m.createGroupErr != nil {
		return "", m.createGroupErr
	}
	if len(m.groupBranchNames) > groupIdx {
		return m.groupBranchNames[groupIdx], nil
	}
	if m.groupBranchName != "" {
		return m.groupBranchName, nil
	}
	return fmt.Sprintf("Iron-Ham/ultraplan-abc-group-%d", groupIdx+1), nil
}

func (m *mockBranchOps) CreateSingleBranch(_ context.Context, _ string) (string, error) {
	if m.createSingleErr != nil {
		return "", m.createSingleErr
	}
	if m.singleBranchName != "" {
		return m.singleBranchName, nil
	}
	return "Iron-Ham/ultraplan-abc", nil
}

func (m *mockBranchOps) CreateWorktree(_ context.Context, path, branch string) error {
	m.createWorktreeCall = struct{ path, branch string }{path, branch}
	return m.createWorktreeErr
}

func (m *mockBranchOps) CherryPickBranch(_ context.Context, worktreePath, sourceBranch string) error {
	m.cherryPickCalls = append(m.cherryPickCalls, struct{ path, branch string }{worktreePath, sourceBranch})
	return m.cherryPickErr
}

func (m *mockBranchOps) GetChangedFiles(_ context.Context, _ string) ([]string, error) {
	return m.changedFiles, m.changedFilesErr
}

func (m *mockBranchOps) CountCommitsBetween(_ context.Context, _, _, _ string) (int, error) {
	return m.commitCount, m.commitCountErr
}

func (m *mockBranchOps) Push(_ context.Context, _ string, _ bool) error {
	return m.pushErr
}

func (m *mockBranchOps) RemoveWorktree(_ context.Context, path string) error {
	m.removeWorktreeCalls = append(m.removeWorktreeCalls, path)
	return m.removeWorktreeErr
}

// mockConflictOps is a mock implementation of ConflictOps.
type mockConflictOps struct {
	conflictInfo *ConflictInfo
	checkErr     error
	abortErr     error
	files        []string
	filesErr     error
}

func (m *mockConflictOps) CheckCherryPick(_ context.Context, _, _, _, _ string) (*ConflictInfo, error) {
	return m.conflictInfo, m.checkErr
}

func (m *mockConflictOps) AbortCherryPick(_ context.Context, _ string) error {
	return m.abortErr
}

func (m *mockConflictOps) GetConflictingFiles(_ context.Context, _ string) ([]string, error) {
	return m.files, m.filesErr
}

// mockPRBuilder is a mock implementation of PRBuilderOps.
type mockPRBuilder struct {
	content  *consolidation.PRContent
	buildErr error
}

func (m *mockPRBuilder) Build(_ []consolidation.CompletedTask, _ consolidation.PRBuildOptions) (*consolidation.PRContent, error) {
	if m.buildErr != nil {
		return nil, m.buildErr
	}
	if m.content != nil {
		return m.content, nil
	}
	return &consolidation.PRContent{
		Title:      "Test PR",
		Body:       "Test body",
		BaseBranch: "main",
		HeadBranch: "feature",
	}, nil
}

// mockPRCreator is a mock implementation of PRCreatorOps.
type mockPRCreator struct {
	prURL     string
	createErr error
}

func (m *mockPRCreator) Create(_ context.Context, _ *consolidation.PRContent, _ bool, _ []string) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	if m.prURL != "" {
		return m.prURL, nil
	}
	return "https://github.com/org/repo/pull/1", nil
}

// mockEventEmitter is a mock implementation of EventEmitter.
type mockEventEmitter struct {
	events []consolidation.Event
}

func (m *mockEventEmitter) Emit(event consolidation.Event) {
	m.events = append(m.events, event)
}

func TestStacked_Name(t *testing.T) {
	s := NewStacked(Dependencies{}, Config{})
	if got := s.Name(); got != "stacked" {
		t.Errorf("Name() = %q, want %q", got, "stacked")
	}
}

func TestStacked_SupportsParallel(t *testing.T) {
	s := NewStacked(Dependencies{}, Config{})
	if got := s.SupportsParallel(); got != false {
		t.Errorf("SupportsParallel() = %v, want false", got)
	}
}

func TestSingle_Name(t *testing.T) {
	s := NewSingle(Dependencies{}, Config{})
	if got := s.Name(); got != "single" {
		t.Errorf("Name() = %q, want %q", got, "single")
	}
}

func TestSingle_SupportsParallel(t *testing.T) {
	s := NewSingle(Dependencies{}, Config{})
	if got := s.SupportsParallel(); got != true {
		t.Errorf("SupportsParallel() = %v, want true", got)
	}
}

func TestStacked_Execute_Success(t *testing.T) {
	events := &mockEventEmitter{}
	deps := Dependencies{
		Branch:    &mockBranchOps{commitCount: 3, changedFiles: []string{"a.go", "b.go"}},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{prURL: "https://github.com/org/repo/pull/1"},
		Events:    events,
	}
	config := Config{
		WorktreeDir: "/tmp",
		Objective:   "Test objective",
	}

	s := NewStacked(deps, config)

	groups := []TaskGroup{
		{
			Index: 0,
			Tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Task 1", Branch: "feature-1"},
				{ID: "task-2", Title: "Task 2", Branch: "feature-2"},
			},
		},
	}

	result, err := s.Execute(context.Background(), groups)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(result.PRs) != 1 {
		t.Errorf("Execute() created %d PRs, want 1", len(result.PRs))
	}

	if result.PRs[0].URL != "https://github.com/org/repo/pull/1" {
		t.Errorf("Execute() PR URL = %q, want %q", result.PRs[0].URL, "https://github.com/org/repo/pull/1")
	}

	if result.TotalCommits != 3 {
		t.Errorf("Execute() TotalCommits = %d, want 3", result.TotalCommits)
	}

	// Check events were emitted
	if len(events.events) == 0 {
		t.Error("Execute() should emit events")
	}
}

func TestStacked_Execute_Conflict(t *testing.T) {
	deps := Dependencies{
		Branch: &mockBranchOps{},
		Conflict: &mockConflictOps{
			conflictInfo: &ConflictInfo{
				TaskID: "task-1",
				Files:  []string{"conflict.go"},
			},
		},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{},
	}
	config := Config{
		WorktreeDir: "/tmp",
	}

	s := NewStacked(deps, config)

	groups := []TaskGroup{
		{
			Index: 0,
			Tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Task 1", Branch: "feature-1"},
			},
		},
	}

	result, _ := s.Execute(context.Background(), groups)

	// Should have one group result with failure
	if len(result.GroupResults) != 1 {
		t.Fatalf("Execute() returned %d group results, want 1", len(result.GroupResults))
	}

	if result.GroupResults[0].Success {
		t.Error("Execute() group should have failed due to conflict")
	}
}

func TestSingle_Execute_Success(t *testing.T) {
	events := &mockEventEmitter{}
	deps := Dependencies{
		Branch:    &mockBranchOps{commitCount: 5, changedFiles: []string{"x.go", "y.go"}},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{prURL: "https://github.com/org/repo/pull/42"},
		Events:    events,
	}
	config := Config{
		WorktreeDir: "/tmp",
		Objective:   "Single PR test",
	}

	s := NewSingle(deps, config)

	groups := []TaskGroup{
		{
			Index: 0,
			Tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Task 1", Branch: "feature-1"},
			},
		},
		{
			Index: 1,
			Tasks: []consolidation.CompletedTask{
				{ID: "task-2", Title: "Task 2", Branch: "feature-2"},
			},
		},
	}

	result, err := s.Execute(context.Background(), groups)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(result.PRs) != 1 {
		t.Errorf("Execute() created %d PRs, want 1", len(result.PRs))
	}

	if result.PRs[0].URL != "https://github.com/org/repo/pull/42" {
		t.Errorf("Execute() PR URL = %q", result.PRs[0].URL)
	}

	if result.TotalCommits != 5 {
		t.Errorf("Execute() TotalCommits = %d, want 5", result.TotalCommits)
	}

	// Should have processed both groups
	if len(result.GroupResults) != 2 {
		t.Errorf("Execute() returned %d group results, want 2", len(result.GroupResults))
	}
}

func TestSingle_Execute_CreateBranchError(t *testing.T) {
	deps := Dependencies{
		Branch: &mockBranchOps{
			createSingleErr: errors.New("branch already exists"),
		},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{},
	}
	config := Config{
		WorktreeDir: "/tmp",
	}

	s := NewSingle(deps, config)

	groups := []TaskGroup{
		{
			Index: 0,
			Tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Task 1", Branch: "feature-1"},
			},
		},
	}

	_, err := s.Execute(context.Background(), groups)
	if err == nil {
		t.Error("Execute() expected error for branch creation failure")
	}
}

func TestSingle_Execute_Conflict(t *testing.T) {
	deps := Dependencies{
		Branch: &mockBranchOps{},
		Conflict: &mockConflictOps{
			conflictInfo: &ConflictInfo{
				TaskID: "task-1",
				Files:  []string{"conflict.go"},
			},
		},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{},
	}
	config := Config{
		WorktreeDir: "/tmp",
	}

	s := NewSingle(deps, config)

	groups := []TaskGroup{
		{
			Index: 0,
			Tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Task 1", Branch: "feature-1"},
			},
		},
	}

	result, err := s.Execute(context.Background(), groups)
	if err == nil {
		t.Error("Execute() expected error for conflict")
	}

	if result.Error == nil {
		t.Error("Execute() result.Error should be set on conflict")
	}
}

func TestBase_NilLogger(t *testing.T) {
	// Test that Base handles nil logger gracefully
	deps := Dependencies{
		Logger: nil,
	}
	base := NewBase(deps, Config{})

	// Should return nopLogger
	logger := base.log()
	if logger == nil {
		t.Error("log() should return nopLogger when Logger is nil")
	}

	// nopLogger methods should not panic
	logger.Debug("test")
	logger.Info("test")
	logger.Warn("test")
	logger.Error("test")
}

func TestBase_NilEventEmitter(t *testing.T) {
	// Test that Base handles nil event emitter gracefully
	deps := Dependencies{
		Events: nil,
	}
	base := NewBase(deps, Config{})

	// Should not panic
	base.emit(consolidation.Event{
		Type:    consolidation.EventStarted,
		Message: "test",
	})
}

func TestStacked_Execute_MultipleGroups(t *testing.T) {
	// Test that stacked strategy correctly chains baseBranch through multiple groups
	branchOps := &mockBranchOps{
		commitCount:      2,
		changedFiles:     []string{"file.go"},
		groupBranchNames: []string{"group-1-branch", "group-2-branch", "group-3-branch"},
	}
	deps := Dependencies{
		Branch:    branchOps,
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{},
	}
	config := Config{
		WorktreeDir: "/tmp",
		Objective:   "Multi-group test",
	}

	s := NewStacked(deps, config)

	groups := []TaskGroup{
		{Index: 0, Tasks: []consolidation.CompletedTask{{ID: "task-1", Branch: "feature-1"}}},
		{Index: 1, Tasks: []consolidation.CompletedTask{{ID: "task-2", Branch: "feature-2"}}},
		{Index: 2, Tasks: []consolidation.CompletedTask{{ID: "task-3", Branch: "feature-3"}}},
	}

	result, err := s.Execute(context.Background(), groups)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Should have 3 PRs
	if len(result.PRs) != 3 {
		t.Errorf("Execute() created %d PRs, want 3", len(result.PRs))
	}

	// Verify baseBranch chaining: each group should use previous group's branch as base
	if len(branchOps.createGroupCalls) != 3 {
		t.Fatalf("Expected 3 CreateGroupBranch calls, got %d", len(branchOps.createGroupCalls))
	}

	// First group should use "main" as base
	if branchOps.createGroupCalls[0].baseBranch != "main" {
		t.Errorf("Group 0 baseBranch = %q, want %q", branchOps.createGroupCalls[0].baseBranch, "main")
	}

	// Second group should use first group's branch as base
	if branchOps.createGroupCalls[1].baseBranch != "group-1-branch" {
		t.Errorf("Group 1 baseBranch = %q, want %q", branchOps.createGroupCalls[1].baseBranch, "group-1-branch")
	}

	// Third group should use second group's branch as base
	if branchOps.createGroupCalls[2].baseBranch != "group-2-branch" {
		t.Errorf("Group 2 baseBranch = %q, want %q", branchOps.createGroupCalls[2].baseBranch, "group-2-branch")
	}
}

func TestStacked_Execute_WorktreeError(t *testing.T) {
	deps := Dependencies{
		Branch: &mockBranchOps{
			createWorktreeErr: errors.New("failed to create worktree"),
		},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{},
	}
	config := Config{WorktreeDir: "/tmp"}

	s := NewStacked(deps, config)

	groups := []TaskGroup{
		{Index: 0, Tasks: []consolidation.CompletedTask{{ID: "task-1", Branch: "feature-1"}}},
	}

	result, _ := s.Execute(context.Background(), groups)

	if len(result.GroupResults) != 1 {
		t.Fatalf("Expected 1 group result, got %d", len(result.GroupResults))
	}

	if result.GroupResults[0].Success {
		t.Error("Group should have failed due to worktree error")
	}
}

func TestStacked_Execute_PushError(t *testing.T) {
	deps := Dependencies{
		Branch: &mockBranchOps{
			pushErr: errors.New("failed to push"),
		},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{},
	}
	config := Config{WorktreeDir: "/tmp"}

	s := NewStacked(deps, config)

	groups := []TaskGroup{
		{Index: 0, Tasks: []consolidation.CompletedTask{{ID: "task-1", Branch: "feature-1"}}},
	}

	result, _ := s.Execute(context.Background(), groups)

	if len(result.GroupResults) != 1 {
		t.Fatalf("Expected 1 group result, got %d", len(result.GroupResults))
	}

	if result.GroupResults[0].Success {
		t.Error("Group should have failed due to push error")
	}
}

func TestStacked_Execute_PRBuildError(t *testing.T) {
	deps := Dependencies{
		Branch:    &mockBranchOps{},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{buildErr: errors.New("failed to build PR content")},
		PRCreator: &mockPRCreator{},
	}
	config := Config{WorktreeDir: "/tmp"}

	s := NewStacked(deps, config)

	groups := []TaskGroup{
		{Index: 0, Tasks: []consolidation.CompletedTask{{ID: "task-1", Branch: "feature-1"}}},
	}

	result, _ := s.Execute(context.Background(), groups)

	if len(result.GroupResults) != 1 {
		t.Fatalf("Expected 1 group result, got %d", len(result.GroupResults))
	}

	if result.GroupResults[0].Success {
		t.Error("Group should have failed due to PR build error")
	}
}

func TestStacked_Execute_PRCreateError(t *testing.T) {
	deps := Dependencies{
		Branch:    &mockBranchOps{},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{createErr: errors.New("failed to create PR")},
	}
	config := Config{WorktreeDir: "/tmp"}

	s := NewStacked(deps, config)

	groups := []TaskGroup{
		{Index: 0, Tasks: []consolidation.CompletedTask{{ID: "task-1", Branch: "feature-1"}}},
	}

	result, _ := s.Execute(context.Background(), groups)

	if len(result.GroupResults) != 1 {
		t.Fatalf("Expected 1 group result, got %d", len(result.GroupResults))
	}

	if result.GroupResults[0].Success {
		t.Error("Group should have failed due to PR creation error")
	}
}

func TestSingle_Execute_WorktreeError(t *testing.T) {
	deps := Dependencies{
		Branch: &mockBranchOps{
			createWorktreeErr: errors.New("failed to create worktree"),
		},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{},
	}
	config := Config{WorktreeDir: "/tmp"}

	s := NewSingle(deps, config)

	groups := []TaskGroup{
		{Index: 0, Tasks: []consolidation.CompletedTask{{ID: "task-1", Branch: "feature-1"}}},
	}

	_, err := s.Execute(context.Background(), groups)
	if err == nil {
		t.Error("Execute() expected error for worktree creation failure")
	}
}

func TestSingle_Execute_PushError(t *testing.T) {
	deps := Dependencies{
		Branch: &mockBranchOps{
			pushErr: errors.New("failed to push"),
		},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{},
	}
	config := Config{WorktreeDir: "/tmp"}

	s := NewSingle(deps, config)

	groups := []TaskGroup{
		{Index: 0, Tasks: []consolidation.CompletedTask{{ID: "task-1", Branch: "feature-1"}}},
	}

	_, err := s.Execute(context.Background(), groups)
	if err == nil {
		t.Error("Execute() expected error for push failure")
	}
}

func TestSingle_Execute_PRBuildError(t *testing.T) {
	deps := Dependencies{
		Branch:    &mockBranchOps{},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{buildErr: errors.New("failed to build PR content")},
		PRCreator: &mockPRCreator{},
	}
	config := Config{WorktreeDir: "/tmp"}

	s := NewSingle(deps, config)

	groups := []TaskGroup{
		{Index: 0, Tasks: []consolidation.CompletedTask{{ID: "task-1", Branch: "feature-1"}}},
	}

	_, err := s.Execute(context.Background(), groups)
	if err == nil {
		t.Error("Execute() expected error for PR build failure")
	}
}

func TestSingle_Execute_PRCreateError(t *testing.T) {
	deps := Dependencies{
		Branch:    &mockBranchOps{},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{createErr: errors.New("failed to create PR")},
	}
	config := Config{WorktreeDir: "/tmp"}

	s := NewSingle(deps, config)

	groups := []TaskGroup{
		{Index: 0, Tasks: []consolidation.CompletedTask{{ID: "task-1", Branch: "feature-1"}}},
	}

	_, err := s.Execute(context.Background(), groups)
	if err == nil {
		t.Error("Execute() expected error for PR creation failure")
	}
}

func TestStacked_Execute_CreateGroupBranchError(t *testing.T) {
	deps := Dependencies{
		Branch: &mockBranchOps{
			createGroupErr: errors.New("branch already exists"),
		},
		Conflict:  &mockConflictOps{},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{},
	}
	config := Config{WorktreeDir: "/tmp"}

	s := NewStacked(deps, config)

	groups := []TaskGroup{
		{Index: 0, Tasks: []consolidation.CompletedTask{{ID: "task-1", Branch: "feature-1"}}},
	}

	result, _ := s.Execute(context.Background(), groups)

	if len(result.GroupResults) != 1 {
		t.Fatalf("Expected 1 group result, got %d", len(result.GroupResults))
	}

	if result.GroupResults[0].Success {
		t.Error("Group should have failed due to branch creation error")
	}
}

func TestSingle_Execute_CherryPickCheckError(t *testing.T) {
	deps := Dependencies{
		Branch: &mockBranchOps{},
		Conflict: &mockConflictOps{
			checkErr: errors.New("git error during cherry-pick check"),
		},
		PRBuilder: &mockPRBuilder{},
		PRCreator: &mockPRCreator{},
	}
	config := Config{WorktreeDir: "/tmp"}

	s := NewSingle(deps, config)

	groups := []TaskGroup{
		{Index: 0, Tasks: []consolidation.CompletedTask{{ID: "task-1", Branch: "feature-1"}}},
	}

	_, err := s.Execute(context.Background(), groups)
	if err == nil {
		t.Error("Execute() expected error for cherry-pick check failure")
	}
}
