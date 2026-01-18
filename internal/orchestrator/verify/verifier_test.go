package verify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
)

// mockWorktreeOps is a mock implementation of WorktreeOperations.
type mockWorktreeOps struct {
	commitCount    int
	commitCountErr error
	mainBranch     string
}

func (m *mockWorktreeOps) CountCommitsBetween(_, _, _ string) (int, error) {
	return m.commitCount, m.commitCountErr
}

func (m *mockWorktreeOps) FindMainBranch() string {
	if m.mainBranch == "" {
		return "main"
	}
	return m.mainBranch
}

// mockRetryTracker is a mock implementation of RetryTracker.
type mockRetryTracker struct {
	retryCounts  map[string]int
	maxRetries   map[string]int
	commitCounts map[string][]int
}

func newMockRetryTracker() *mockRetryTracker {
	return &mockRetryTracker{
		retryCounts:  make(map[string]int),
		maxRetries:   make(map[string]int),
		commitCounts: make(map[string][]int),
	}
}

func (m *mockRetryTracker) GetRetryCount(taskID string) int {
	return m.retryCounts[taskID]
}

func (m *mockRetryTracker) IncrementRetry(taskID string) int {
	m.retryCounts[taskID]++
	return m.retryCounts[taskID]
}

func (m *mockRetryTracker) RecordCommitCount(taskID string, count int) {
	m.commitCounts[taskID] = append(m.commitCounts[taskID], count)
}

func (m *mockRetryTracker) GetMaxRetries(taskID string) int {
	return m.maxRetries[taskID]
}

// mockEventEmitter is a mock implementation of EventEmitter.
type mockEventEmitter struct {
	warnings []string
	retries  []retryEvent
	failures []string
}

type retryEvent struct {
	taskID     string
	attempt    int
	maxRetries int
	reason     string
}

func newMockEventEmitter() *mockEventEmitter {
	return &mockEventEmitter{
		warnings: make([]string, 0),
		retries:  make([]retryEvent, 0),
		failures: make([]string, 0),
	}
}

func (m *mockEventEmitter) EmitWarning(taskID, message string) {
	m.warnings = append(m.warnings, message)
}

func (m *mockEventEmitter) EmitRetry(taskID string, attempt, maxRetries int, reason string) {
	m.retries = append(m.retries, retryEvent{taskID, attempt, maxRetries, reason})
}

func (m *mockEventEmitter) EmitFailure(taskID, reason string) {
	m.failures = append(m.failures, reason)
}

func TestNewTaskVerifier(t *testing.T) {
	wt := &mockWorktreeOps{}
	rt := newMockRetryTracker()
	events := newMockEventEmitter()

	v := NewTaskVerifier(wt, rt, events)

	if v == nil {
		t.Fatal("NewTaskVerifier returned nil")
	}

	if v.wt != wt {
		t.Error("WorktreeOperations not set correctly")
	}

	if v.retryTracker != rt {
		t.Error("RetryTracker not set correctly")
	}

	if v.events != events {
		t.Error("EventEmitter not set correctly")
	}

	// Check default config
	if v.config.MaxTaskRetries != 3 {
		t.Errorf("expected default MaxTaskRetries=3, got %d", v.config.MaxTaskRetries)
	}
}

func TestNewTaskVerifier_WithOptions(t *testing.T) {
	wt := &mockWorktreeOps{}
	rt := newMockRetryTracker()
	events := newMockEventEmitter()

	cfg := Config{
		RequireVerifiedCommits: true,
		MaxTaskRetries:         5,
	}

	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	if v.config.RequireVerifiedCommits != true {
		t.Error("RequireVerifiedCommits not set correctly")
	}

	if v.config.MaxTaskRetries != 5 {
		t.Errorf("expected MaxTaskRetries=5, got %d", v.config.MaxTaskRetries)
	}
}

func TestNewTaskVerifier_NilWorktreeOps(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when wt is nil")
		}
	}()
	NewTaskVerifier(nil, newMockRetryTracker(), newMockEventEmitter())
}

func TestNewTaskVerifier_NilRetryTracker(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when retryTracker is nil")
		}
	}()
	NewTaskVerifier(&mockWorktreeOps{}, nil, newMockEventEmitter())
}

func TestNewTaskVerifier_NilEventEmitter(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when events is nil")
		}
	}()
	NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), nil)
}

func TestCheckCompletionFile_EmptyWorktreePath(t *testing.T) {
	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	found, err := v.CheckCompletionFile("")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected false for empty worktree path")
	}
}

func TestCheckCompletionFile_NoFile(t *testing.T) {
	tempDir := t.TempDir()

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected false when no completion file exists")
	}
}

func TestCheckCompletionFile_ValidTaskCompletion(t *testing.T) {
	tempDir := t.TempDir()

	completion := types.TaskCompletionFile{
		TaskID:        "task-1",
		Status:        "complete",
		Summary:       "Task completed successfully",
		FilesModified: []string{"file1.go", "file2.go"},
	}

	data, _ := json.Marshal(completion)
	err := os.WriteFile(filepath.Join(tempDir, TaskCompletionFileName), data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected true when valid task completion file exists")
	}
}

func TestCheckCompletionFile_ValidRevisionCompletion(t *testing.T) {
	tempDir := t.TempDir()

	completion := RevisionCompletionFile{
		TaskID:          "task-1",
		RevisionRound:   1,
		IssuesAddressed: []string{"issue-1"},
		Summary:         "Revision completed",
		FilesModified:   []string{"file1.go"},
	}

	data, _ := json.Marshal(completion)
	err := os.WriteFile(filepath.Join(tempDir, RevisionCompletionFileName), data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected true when valid revision completion file exists")
	}
}

func TestCheckCompletionFile_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()

	// Write invalid JSON
	err := os.WriteFile(filepath.Join(tempDir, TaskCompletionFileName), []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected false when completion file contains invalid JSON")
	}
}

func TestCheckCompletionFile_EmptyStatus(t *testing.T) {
	tempDir := t.TempDir()

	// Task completion with empty status should not be considered valid
	completion := types.TaskCompletionFile{
		TaskID:  "task-1",
		Status:  "",
		Summary: "No status",
	}

	data, _ := json.Marshal(completion)
	err := os.WriteFile(filepath.Join(tempDir, TaskCompletionFileName), data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected false when task completion has empty status")
	}
}

func TestParseTaskCompletionFile(t *testing.T) {
	tempDir := t.TempDir()

	expected := types.TaskCompletionFile{
		TaskID:        "task-123",
		Status:        "complete",
		Summary:       "Implemented feature X",
		FilesModified: []string{"a.go", "b.go"},
		Notes:         types.FlexibleString("Some notes"),
		Issues:        []string{"issue-1"},
		Suggestions:   []string{"suggestion-1"},
		Dependencies:  []string{"dep-1"},
	}

	data, _ := json.Marshal(expected)
	err := os.WriteFile(filepath.Join(tempDir, TaskCompletionFileName), data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	result, err := v.ParseTaskCompletionFile(tempDir)
	if err != nil {
		t.Fatalf("ParseTaskCompletionFile failed: %v", err)
	}

	if result.TaskID != expected.TaskID {
		t.Errorf("TaskID mismatch: got %q, want %q", result.TaskID, expected.TaskID)
	}
	if result.Status != expected.Status {
		t.Errorf("Status mismatch: got %q, want %q", result.Status, expected.Status)
	}
	if result.Summary != expected.Summary {
		t.Errorf("Summary mismatch: got %q, want %q", result.Summary, expected.Summary)
	}
	if len(result.FilesModified) != len(expected.FilesModified) {
		t.Errorf("FilesModified length mismatch: got %d, want %d", len(result.FilesModified), len(expected.FilesModified))
	}
}

func TestParseTaskCompletionFile_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	_, err := v.ParseTaskCompletionFile(tempDir)
	if err == nil {
		t.Error("expected error when file does not exist")
	}
}

func TestFindAndParseTaskCompletionFile_AtRoot(t *testing.T) {
	tempDir := t.TempDir()

	expected := types.TaskCompletionFile{
		TaskID:  "task-root",
		Status:  "complete",
		Summary: "File at root",
	}

	data, _ := json.Marshal(expected)
	err := os.WriteFile(filepath.Join(tempDir, TaskCompletionFileName), data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	result, err := v.FindAndParseTaskCompletionFile(tempDir)
	if err != nil {
		t.Fatalf("FindAndParseTaskCompletionFile failed: %v", err)
	}

	if result.TaskID != expected.TaskID {
		t.Errorf("TaskID mismatch: got %q, want %q", result.TaskID, expected.TaskID)
	}
	if result.Status != expected.Status {
		t.Errorf("Status mismatch: got %q, want %q", result.Status, expected.Status)
	}
}

func TestFindAndParseTaskCompletionFile_InSubdirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Create a subdirectory to simulate Claude changing directories
	subDir := filepath.Join(tempDir, "project", "src")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	expected := types.TaskCompletionFile{
		TaskID:  "task-subdir",
		Status:  "complete",
		Summary: "File in subdirectory",
	}

	// Write completion file in subdirectory (simulating Claude wrote it after cd project/src)
	data, _ := json.Marshal(expected)
	err := os.WriteFile(filepath.Join(subDir, TaskCompletionFileName), data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	// Search from root should find file in subdirectory
	result, err := v.FindAndParseTaskCompletionFile(tempDir)
	if err != nil {
		t.Fatalf("FindAndParseTaskCompletionFile failed: %v", err)
	}

	if result.TaskID != expected.TaskID {
		t.Errorf("TaskID mismatch: got %q, want %q", result.TaskID, expected.TaskID)
	}
	if result.Status != expected.Status {
		t.Errorf("Status mismatch: got %q, want %q", result.Status, expected.Status)
	}
}

func TestFindAndParseTaskCompletionFile_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	_, err := v.FindAndParseTaskCompletionFile(tempDir)
	if err == nil {
		t.Error("expected error when file does not exist")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestParseRevisionCompletionFile(t *testing.T) {
	tempDir := t.TempDir()

	expected := RevisionCompletionFile{
		TaskID:          "task-456",
		RevisionRound:   2,
		IssuesAddressed: []string{"fix lint errors", "add missing tests"},
		Summary:         "Addressed review comments",
		FilesModified:   []string{"main.go", "main_test.go"},
		RemainingIssues: []string{"performance optimization deferred"},
	}

	data, _ := json.Marshal(expected)
	err := os.WriteFile(filepath.Join(tempDir, RevisionCompletionFileName), data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	result, err := v.ParseRevisionCompletionFile(tempDir)
	if err != nil {
		t.Fatalf("ParseRevisionCompletionFile failed: %v", err)
	}

	if result.TaskID != expected.TaskID {
		t.Errorf("TaskID mismatch: got %q, want %q", result.TaskID, expected.TaskID)
	}
	if result.RevisionRound != expected.RevisionRound {
		t.Errorf("RevisionRound mismatch: got %d, want %d", result.RevisionRound, expected.RevisionRound)
	}
	if len(result.IssuesAddressed) != len(expected.IssuesAddressed) {
		t.Errorf("IssuesAddressed length mismatch: got %d, want %d", len(result.IssuesAddressed), len(expected.IssuesAddressed))
	}
}

func TestVerifyTaskWork_VerificationDisabled(t *testing.T) {
	wt := &mockWorktreeOps{commitCount: 0}
	rt := newMockRetryTracker()
	events := newMockEventEmitter()

	// Default config has RequireVerifiedCommits=false
	v := NewTaskVerifier(wt, rt, events)

	result := v.VerifyTaskWork("task-1", "inst-1", "/tmp/worktree", "main", nil)

	if !result.Success {
		t.Error("expected success when verification is disabled")
	}
	if result.NeedsRetry {
		t.Error("expected no retry when verification is disabled")
	}
}

func TestVerifyTaskWork_WithCommits(t *testing.T) {
	wt := &mockWorktreeOps{commitCount: 3}
	rt := newMockRetryTracker()
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 3}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	result := v.VerifyTaskWork("task-1", "inst-1", "/tmp/worktree", "main", nil)

	if !result.Success {
		t.Error("expected success when commits were produced")
	}
	if result.CommitCount != 3 {
		t.Errorf("expected CommitCount=3, got %d", result.CommitCount)
	}
	if result.NeedsRetry {
		t.Error("expected no retry when commits were produced")
	}
}

func TestVerifyTaskWork_NoCommits_FirstRetry(t *testing.T) {
	wt := &mockWorktreeOps{commitCount: 0}
	rt := newMockRetryTracker()
	rt.maxRetries["task-1"] = 3
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 3}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	result := v.VerifyTaskWork("task-1", "inst-1", "/tmp/worktree", "main", nil)

	if result.Success {
		t.Error("expected failure when no commits and retries available")
	}
	if !result.NeedsRetry {
		t.Error("expected NeedsRetry=true when retries available")
	}
	if result.Error != "no_commits_retry" {
		t.Errorf("expected error='no_commits_retry', got %q", result.Error)
	}
	if rt.retryCounts["task-1"] != 1 {
		t.Errorf("expected retry count to be incremented to 1, got %d", rt.retryCounts["task-1"])
	}
	if len(events.retries) != 1 {
		t.Errorf("expected 1 retry event, got %d", len(events.retries))
	}
}

func TestVerifyTaskWork_NoCommits_MaxRetriesExhausted(t *testing.T) {
	wt := &mockWorktreeOps{commitCount: 0}
	rt := newMockRetryTracker()
	rt.retryCounts["task-1"] = 3 // Already at max
	rt.maxRetries["task-1"] = 3
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 3}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	result := v.VerifyTaskWork("task-1", "inst-1", "/tmp/worktree", "main", nil)

	if result.Success {
		t.Error("expected failure when max retries exhausted")
	}
	if result.NeedsRetry {
		t.Error("expected NeedsRetry=false when max retries exhausted")
	}
	if len(events.failures) != 1 {
		t.Errorf("expected 1 failure event, got %d", len(events.failures))
	}
}

func TestVerifyTaskWork_CountCommitsError(t *testing.T) {
	wt := &mockWorktreeOps{
		commitCount:    0,
		commitCountErr: os.ErrNotExist,
	}
	rt := newMockRetryTracker()
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 3}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	result := v.VerifyTaskWork("task-1", "inst-1", "/tmp/worktree", "main", nil)

	// Should succeed (graceful degradation)
	if !result.Success {
		t.Error("expected success when commit count fails (graceful degradation)")
	}
	if len(events.warnings) != 1 {
		t.Errorf("expected 1 warning event, got %d", len(events.warnings))
	}
}

func TestVerifyTaskWork_EmptyBaseBranch(t *testing.T) {
	wt := &mockWorktreeOps{
		commitCount: 2,
		mainBranch:  "master",
	}
	rt := newMockRetryTracker()
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 3}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	// Empty base branch should use FindMainBranch
	result := v.VerifyTaskWork("task-1", "inst-1", "/tmp/worktree", "", nil)

	if !result.Success {
		t.Error("expected success")
	}
	if result.CommitCount != 2 {
		t.Errorf("expected CommitCount=2, got %d", result.CommitCount)
	}
}

func TestVerifyTaskWork_UseDefaultMaxRetries(t *testing.T) {
	wt := &mockWorktreeOps{commitCount: 0}
	rt := newMockRetryTracker()
	// Don't set maxRetries for task, should use config default
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 5}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	// First attempt
	result := v.VerifyTaskWork("task-1", "inst-1", "/tmp/worktree", "main", nil)

	if result.Success {
		t.Error("expected failure")
	}
	if !result.NeedsRetry {
		t.Error("expected NeedsRetry=true")
	}

	// Check retry event was emitted with config's max retries
	if len(events.retries) != 1 {
		t.Fatalf("expected 1 retry event, got %d", len(events.retries))
	}
	if events.retries[0].maxRetries != 5 {
		t.Errorf("expected maxRetries=5 in event, got %d", events.retries[0].maxRetries)
	}
}

func TestTaskCompletionFilePath(t *testing.T) {
	path := TaskCompletionFilePath("/tmp/worktree")
	expected := "/tmp/worktree/.claudio-task-complete.json"
	if path != expected {
		t.Errorf("TaskCompletionFilePath() = %q, want %q", path, expected)
	}
}

func TestRevisionCompletionFilePath(t *testing.T) {
	path := RevisionCompletionFilePath("/tmp/worktree")
	expected := "/tmp/worktree/.claudio-revision-complete.json"
	if path != expected {
		t.Errorf("RevisionCompletionFilePath() = %q, want %q", path, expected)
	}
}

func TestTaskCompletionResult_Fields(t *testing.T) {
	result := TaskCompletionResult{
		TaskID:      "task-1",
		InstanceID:  "inst-1",
		Success:     true,
		Error:       "some-error",
		NeedsRetry:  true,
		CommitCount: 5,
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if result.InstanceID != "inst-1" {
		t.Errorf("InstanceID = %q, want %q", result.InstanceID, "inst-1")
	}
	if result.CommitCount != 5 {
		t.Errorf("CommitCount = %d, want %d", result.CommitCount, 5)
	}
	if !result.Success {
		t.Error("Success should be true")
	}
	if result.Error != "some-error" {
		t.Errorf("Error = %q, want %q", result.Error, "some-error")
	}
	if !result.NeedsRetry {
		t.Error("NeedsRetry should be true")
	}
}

func TestCheckCompletionFile_BothFilesExist_TaskTakesPrecedence(t *testing.T) {
	tempDir := t.TempDir()

	// Write both task and revision completion files
	taskCompletion := types.TaskCompletionFile{
		TaskID: "task-1",
		Status: "complete",
	}
	revisionCompletion := RevisionCompletionFile{
		TaskID: "task-1",
	}

	taskData, _ := json.Marshal(taskCompletion)
	revisionData, _ := json.Marshal(revisionCompletion)

	if err := os.WriteFile(filepath.Join(tempDir, TaskCompletionFileName), taskData, 0644); err != nil {
		t.Fatalf("failed to write task completion file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, RevisionCompletionFileName), revisionData, 0644); err != nil {
		t.Fatalf("failed to write revision completion file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected true when completion file exists")
	}
}

func TestVerifyTaskWork_CommitCountRecorded(t *testing.T) {
	wt := &mockWorktreeOps{commitCount: 0}
	rt := newMockRetryTracker()
	rt.maxRetries["task-1"] = 3
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 3}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	v.VerifyTaskWork("task-1", "inst-1", "/tmp/worktree", "main", nil)

	if len(rt.commitCounts["task-1"]) != 1 {
		t.Errorf("expected 1 commit count recorded, got %d", len(rt.commitCounts["task-1"]))
	}
	if rt.commitCounts["task-1"][0] != 0 {
		t.Errorf("expected commit count 0, got %d", rt.commitCounts["task-1"][0])
	}
}

func TestVerifyTaskWork_NoCodeOption_SkipsCommitVerification(t *testing.T) {
	wt := &mockWorktreeOps{commitCount: 0} // No commits
	rt := newMockRetryTracker()
	rt.maxRetries["task-1"] = 3
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 3}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	// With NoCode option, task should succeed even without commits
	opts := &TaskVerifyOptions{NoCode: true}
	result := v.VerifyTaskWork("task-1", "inst-1", "/tmp/worktree", "main", opts)

	if !result.Success {
		t.Error("expected success for no-code task even without commits")
	}
	if result.NeedsRetry {
		t.Error("expected no retry for no-code task")
	}
	if len(events.retries) != 0 {
		t.Errorf("expected 0 retry events for no-code task, got %d", len(events.retries))
	}
	if len(events.failures) != 0 {
		t.Errorf("expected 0 failure events for no-code task, got %d", len(events.failures))
	}
}

func TestVerifyTaskWork_CompletionFileOverride_NoCommits(t *testing.T) {
	tempDir := t.TempDir()

	wt := &mockWorktreeOps{commitCount: 0} // No commits
	rt := newMockRetryTracker()
	rt.maxRetries["task-1"] = 3
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 3}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	// Write a completion file with status="complete"
	completion := types.TaskCompletionFile{
		TaskID:  "task-1",
		Status:  "complete",
		Summary: "Verification task completed successfully - no code changes needed",
	}
	data, _ := json.Marshal(completion)
	if err := os.WriteFile(filepath.Join(tempDir, TaskCompletionFileName), data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Should succeed because completion file has status="complete"
	result := v.VerifyTaskWork("task-1", "inst-1", tempDir, "main", nil)

	if !result.Success {
		t.Error("expected success when completion file has status='complete'")
	}
	if result.NeedsRetry {
		t.Error("expected no retry when completion file indicates success")
	}
	if len(events.retries) != 0 {
		t.Errorf("expected 0 retry events, got %d", len(events.retries))
	}
}

func TestVerifyTaskWork_CompletionFileInSubdirectory_NoCommits(t *testing.T) {
	tempDir := t.TempDir()

	// Create a subdirectory to simulate Claude changing directories
	subDir := filepath.Join(tempDir, "project", "src")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	wt := &mockWorktreeOps{commitCount: 0} // No commits
	rt := newMockRetryTracker()
	rt.maxRetries["task-1"] = 3
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 3}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	// Write a completion file in subdirectory (simulating Claude wrote it after cd project/src)
	completion := types.TaskCompletionFile{
		TaskID:  "task-1",
		Status:  "complete",
		Summary: "Verification task completed - file in subdirectory",
	}
	data, _ := json.Marshal(completion)
	// Write to subdirectory, NOT root
	if err := os.WriteFile(filepath.Join(subDir, TaskCompletionFileName), data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Should succeed because FindAndParseTaskCompletionFile uses recursive search
	result := v.VerifyTaskWork("task-1", "inst-1", tempDir, "main", nil)

	if !result.Success {
		t.Error("expected success when completion file with status='complete' is in subdirectory")
	}
	if result.NeedsRetry {
		t.Error("expected no retry when completion file indicates success")
	}
	if len(events.retries) != 0 {
		t.Errorf("expected 0 retry events, got %d", len(events.retries))
	}
}

func TestVerifyTaskWork_CompletionFileBlocked_StillFails(t *testing.T) {
	tempDir := t.TempDir()

	wt := &mockWorktreeOps{commitCount: 0} // No commits
	rt := newMockRetryTracker()
	rt.maxRetries["task-1"] = 3
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 3}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	// Write a completion file with status="blocked" (not "complete")
	completion := types.TaskCompletionFile{
		TaskID:  "task-1",
		Status:  "blocked",
		Summary: "Task is blocked by external dependency",
	}
	data, _ := json.Marshal(completion)
	if err := os.WriteFile(filepath.Join(tempDir, TaskCompletionFileName), data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Should fail because status is not "complete"
	result := v.VerifyTaskWork("task-1", "inst-1", tempDir, "main", nil)

	if result.Success {
		t.Error("expected failure when completion file has status='blocked'")
	}
	if !result.NeedsRetry {
		t.Error("expected retry when completion file doesn't indicate success")
	}
}

func TestVerifyTaskWork_NoCompletionFile_FailsNormally(t *testing.T) {
	tempDir := t.TempDir()

	wt := &mockWorktreeOps{commitCount: 0} // No commits
	rt := newMockRetryTracker()
	rt.maxRetries["task-1"] = 3
	events := newMockEventEmitter()

	cfg := Config{RequireVerifiedCommits: true, MaxTaskRetries: 3}
	v := NewTaskVerifier(wt, rt, events, WithConfig(cfg))

	// No completion file written - should fail as before
	result := v.VerifyTaskWork("task-1", "inst-1", tempDir, "main", nil)

	if result.Success {
		t.Error("expected failure when no commits and no completion file")
	}
	if !result.NeedsRetry {
		t.Error("expected retry when no completion file")
	}
}

// Tests for subdirectory completion file detection

func TestCheckCompletionFile_InSubdirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Create a subdirectory structure (simulating Claude cd-ing into a project dir)
	subdir := filepath.Join(tempDir, "mail-ios", "src")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdirs: %v", err)
	}

	// Write completion file in subdirectory (simulating the bug)
	completion := types.TaskCompletionFile{
		TaskID:  "task-1",
		Status:  "complete",
		Summary: "Task completed",
	}
	data, _ := json.Marshal(completion)
	completionPath := filepath.Join(tempDir, "mail-ios", TaskCompletionFileName)
	if err := os.WriteFile(completionPath, data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	// Should find the file even though it's not at the root
	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected to find completion file in subdirectory")
	}
}

func TestCheckCompletionFile_RevisionInSubdirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Create subdirectory
	subdir := filepath.Join(tempDir, "project")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Write revision completion file in subdirectory
	completion := RevisionCompletionFile{
		TaskID:        "task-1",
		RevisionRound: 1,
	}
	data, _ := json.Marshal(completion)
	completionPath := filepath.Join(subdir, RevisionCompletionFileName)
	if err := os.WriteFile(completionPath, data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected to find revision completion file in subdirectory")
	}
}

func TestCheckCompletionFile_SkipsNodeModules(t *testing.T) {
	tempDir := t.TempDir()

	// Create node_modules with a completion file (should be skipped)
	nodeModules := filepath.Join(tempDir, "node_modules", "some-package")
	if err := os.MkdirAll(nodeModules, 0755); err != nil {
		t.Fatalf("failed to create node_modules: %v", err)
	}

	completion := types.TaskCompletionFile{TaskID: "task-1", Status: "complete"}
	data, _ := json.Marshal(completion)
	if err := os.WriteFile(filepath.Join(nodeModules, TaskCompletionFileName), data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	// Should NOT find the file because it's in node_modules
	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("should not find completion file inside node_modules")
	}
}

func TestCheckCompletionFile_SkipsVendor(t *testing.T) {
	tempDir := t.TempDir()

	// Create vendor directory with a completion file (should be skipped)
	vendorDir := filepath.Join(tempDir, "vendor", "github.com", "some-pkg")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("failed to create vendor dir: %v", err)
	}

	completion := types.TaskCompletionFile{TaskID: "task-1", Status: "complete"}
	data, _ := json.Marshal(completion)
	if err := os.WriteFile(filepath.Join(vendorDir, TaskCompletionFileName), data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("should not find completion file inside vendor")
	}
}

func TestCheckCompletionFile_SkipsPods(t *testing.T) {
	tempDir := t.TempDir()

	// Create Pods directory (iOS CocoaPods) with a completion file (should be skipped)
	podsDir := filepath.Join(tempDir, "Pods", "SomePod")
	if err := os.MkdirAll(podsDir, 0755); err != nil {
		t.Fatalf("failed to create Pods dir: %v", err)
	}

	completion := types.TaskCompletionFile{TaskID: "task-1", Status: "complete"}
	data, _ := json.Marshal(completion)
	if err := os.WriteFile(filepath.Join(podsDir, TaskCompletionFileName), data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("should not find completion file inside Pods")
	}
}

func TestCheckCompletionFile_RootTakesPrecedence(t *testing.T) {
	tempDir := t.TempDir()

	// Create subdirectory with completion file
	subdir := filepath.Join(tempDir, "project")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Write completion files in both root and subdirectory
	rootCompletion := types.TaskCompletionFile{TaskID: "root-task", Status: "complete"}
	subCompletion := types.TaskCompletionFile{TaskID: "sub-task", Status: "complete"}

	rootData, _ := json.Marshal(rootCompletion)
	subData, _ := json.Marshal(subCompletion)

	if err := os.WriteFile(filepath.Join(tempDir, TaskCompletionFileName), rootData, 0644); err != nil {
		t.Fatalf("failed to write root completion file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, TaskCompletionFileName), subData, 0644); err != nil {
		t.Fatalf("failed to write sub completion file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	// Fast path should find root file first
	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected to find completion file")
	}

	// Verify it found the root one (by parsing directly)
	completion, err := v.ParseTaskCompletionFile(tempDir)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if completion.TaskID != "root-task" {
		t.Errorf("expected root completion to take precedence, got TaskID=%q", completion.TaskID)
	}
}

func TestFindCompletionFile_DepthLimit(t *testing.T) {
	tempDir := t.TempDir()

	// Create deeply nested directory structure beyond maxSearchDepth (5)
	deepDir := filepath.Join(tempDir, "a", "b", "c", "d", "e", "f", "g") // 7 levels deep
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatalf("failed to create deep dirs: %v", err)
	}

	// Write completion file at depth > maxSearchDepth
	completion := types.TaskCompletionFile{TaskID: "deep-task", Status: "complete"}
	data, _ := json.Marshal(completion)
	if err := os.WriteFile(filepath.Join(deepDir, TaskCompletionFileName), data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	// Should NOT find the file because it's too deep
	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("should not find completion file beyond depth limit")
	}
}

func TestFindCompletionFile_WithinDepthLimit(t *testing.T) {
	tempDir := t.TempDir()

	// Create directory at exactly maxSearchDepth (5)
	atLimitDir := filepath.Join(tempDir, "a", "b", "c", "d", "e") // 5 levels deep
	if err := os.MkdirAll(atLimitDir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	// Write completion file at depth = maxSearchDepth
	completion := types.TaskCompletionFile{TaskID: "limit-task", Status: "complete"}
	data, _ := json.Marshal(completion)
	if err := os.WriteFile(filepath.Join(atLimitDir, TaskCompletionFileName), data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	v := NewTaskVerifier(&mockWorktreeOps{}, newMockRetryTracker(), newMockEventEmitter())

	// Should find the file at the depth limit
	found, err := v.CheckCompletionFile(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected to find completion file at depth limit")
	}
}
