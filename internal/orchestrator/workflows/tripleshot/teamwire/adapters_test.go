package teamwire

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	ts "github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
)

// --- Mock implementations for adapter tests ---

type mockInstance struct {
	id           string
	worktreePath string
	branch       string
}

func (m *mockInstance) GetID() string           { return m.id }
func (m *mockInstance) GetWorktreePath() string { return m.worktreePath }
func (m *mockInstance) GetBranch() string       { return m.branch }

type mockOrch struct {
	addErr   error
	startErr error
	inst     *mockInstance
}

func (m *mockOrch) AddInstance(_ ts.SessionInterface, _ string) (ts.InstanceInterface, error) {
	if m.addErr != nil {
		return nil, m.addErr
	}
	return m.inst, nil
}

func (m *mockOrch) AddInstanceToWorktree(_ ts.SessionInterface, _, _, _ string) (ts.InstanceInterface, error) {
	return m.inst, nil
}

func (m *mockOrch) StartInstance(_ ts.InstanceInterface) error {
	return m.startErr
}

func (m *mockOrch) SaveSession() error { return nil }

func (m *mockOrch) AddInstanceStub(_ ts.SessionInterface, _ string) (ts.InstanceInterface, error) {
	return m.inst, nil
}

func (m *mockOrch) CompleteInstanceSetupByID(_ ts.SessionInterface, _ string) error {
	return nil
}

type mockSession struct {
	instances map[string]*mockInstance
}

func (m *mockSession) GetGroup(_ string) ts.GroupInterface              { return nil }
func (m *mockSession) GetGroupBySessionType(_ string) ts.GroupInterface { return nil }
func (m *mockSession) GetInstance(id string) ts.InstanceInterface {
	inst := m.instances[id]
	if inst == nil {
		return nil
	}
	return inst
}

// --- attemptFactory tests ---

func TestAttemptFactory_CreateInstance(t *testing.T) {
	inst := &mockInstance{id: "inst-1", worktreePath: "/tmp/wt-1", branch: "branch-1"}
	orch := &mockOrch{inst: inst}
	session := &mockSession{}

	factory := newAttemptFactory(orch, session)
	result, err := factory.CreateInstance("test prompt")
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	if result.ID() != "inst-1" {
		t.Errorf("ID = %q, want %q", result.ID(), "inst-1")
	}
	if result.WorktreePath() != "/tmp/wt-1" {
		t.Errorf("WorktreePath = %q, want %q", result.WorktreePath(), "/tmp/wt-1")
	}
	if result.Branch() != "branch-1" {
		t.Errorf("Branch = %q, want %q", result.Branch(), "branch-1")
	}
}

func TestAttemptFactory_CreateInstance_Error(t *testing.T) {
	orch := &mockOrch{addErr: errors.New("disk full")}
	session := &mockSession{}

	factory := newAttemptFactory(orch, session)
	_, err := factory.CreateInstance("test prompt")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, orch.addErr) {
		t.Errorf("error = %v, want wrapped %v", err, orch.addErr)
	}
}

func TestAttemptFactory_StartInstance(t *testing.T) {
	inst := &mockInstance{id: "inst-1"}
	orch := &mockOrch{inst: inst}
	session := &mockSession{
		instances: map[string]*mockInstance{"inst-1": inst},
	}

	factory := newAttemptFactory(orch, session)
	ai := &attemptInstance{inst: inst}
	if err := factory.StartInstance(ai); err != nil {
		t.Fatalf("StartInstance: %v", err)
	}
}

func TestAttemptFactory_StartInstance_NotFound(t *testing.T) {
	orch := &mockOrch{}
	session := &mockSession{instances: map[string]*mockInstance{}}

	factory := newAttemptFactory(orch, session)
	ai := &attemptInstance{inst: &mockInstance{id: "missing"}}
	err := factory.StartInstance(ai)
	if err == nil {
		t.Fatal("expected error for missing instance")
	}
}

func TestAttemptFactory_StartInstance_Error(t *testing.T) {
	inst := &mockInstance{id: "inst-1"}
	orch := &mockOrch{startErr: errors.New("tmux failure"), inst: inst}
	session := &mockSession{
		instances: map[string]*mockInstance{"inst-1": inst},
	}

	factory := newAttemptFactory(orch, session)
	ai := &attemptInstance{inst: inst}
	err := factory.StartInstance(ai)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- attemptCompletionChecker tests ---

func TestAttemptCompletionChecker_NoFile(t *testing.T) {
	checker := newAttemptCompletionChecker()
	done, err := checker.CheckCompletion(t.TempDir())
	if err != nil {
		t.Fatalf("CheckCompletion: %v", err)
	}
	if done {
		t.Error("expected done=false for missing file")
	}
}

func TestAttemptCompletionChecker_WithFile(t *testing.T) {
	dir := t.TempDir()
	completion := ts.CompletionFile{
		AttemptIndex:  0,
		Status:        "complete",
		Summary:       "done",
		FilesModified: []string{"a.go"},
		Approach:      "direct",
	}
	data, _ := json.Marshal(completion)
	if err := os.WriteFile(filepath.Join(dir, ts.CompletionFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	checker := newAttemptCompletionChecker()
	done, err := checker.CheckCompletion(dir)
	if err != nil {
		t.Fatalf("CheckCompletion: %v", err)
	}
	if !done {
		t.Error("expected done=true for existing file")
	}
}

func TestAttemptCompletionChecker_VerifyWork(t *testing.T) {
	dir := t.TempDir()
	completion := ts.CompletionFile{
		AttemptIndex:  0,
		Status:        "complete",
		Summary:       "implemented feature",
		FilesModified: []string{"main.go"},
		Approach:      "test-driven",
	}
	data, _ := json.Marshal(completion)
	if err := os.WriteFile(filepath.Join(dir, ts.CompletionFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	checker := newAttemptCompletionChecker()
	success, commitCount, err := checker.VerifyWork("t1", "i1", dir, "main")
	if err != nil {
		t.Fatalf("VerifyWork: %v", err)
	}
	if !success {
		t.Error("expected success=true for complete status")
	}
	if commitCount != 0 {
		t.Errorf("commitCount = %d, want 0", commitCount)
	}
}

func TestAttemptCompletionChecker_VerifyWork_FailedStatus(t *testing.T) {
	dir := t.TempDir()
	completion := ts.CompletionFile{Status: "failed"}
	data, _ := json.Marshal(completion)
	if err := os.WriteFile(filepath.Join(dir, ts.CompletionFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	checker := newAttemptCompletionChecker()
	success, _, err := checker.VerifyWork("t1", "i1", dir, "main")
	if err != nil {
		t.Fatalf("VerifyWork: %v", err)
	}
	if success {
		t.Error("expected success=false for failed status")
	}
}

func TestAttemptCompletionChecker_VerifyWork_NoFile(t *testing.T) {
	checker := newAttemptCompletionChecker()
	_, _, err := checker.VerifyWork("t1", "i1", t.TempDir(), "main")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// --- judgeCompletionChecker tests ---

func TestJudgeCompletionChecker_NoFile(t *testing.T) {
	checker := newJudgeCompletionChecker()
	done, err := checker.CheckCompletion(t.TempDir())
	if err != nil {
		t.Fatalf("CheckCompletion: %v", err)
	}
	if done {
		t.Error("expected done=false for missing file")
	}
}

func TestJudgeCompletionChecker_WithFile(t *testing.T) {
	dir := t.TempDir()
	evaluation := ts.Evaluation{
		WinnerIndex:   1,
		MergeStrategy: ts.MergeStrategySelect,
		Reasoning:     "best solution",
	}
	data, _ := json.Marshal(evaluation)
	if err := os.WriteFile(filepath.Join(dir, ts.EvaluationFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	checker := newJudgeCompletionChecker()
	done, err := checker.CheckCompletion(dir)
	if err != nil {
		t.Fatalf("CheckCompletion: %v", err)
	}
	if !done {
		t.Error("expected done=true for existing file")
	}
}

func TestJudgeCompletionChecker_VerifyWork(t *testing.T) {
	dir := t.TempDir()
	evaluation := ts.Evaluation{
		WinnerIndex:   0,
		MergeStrategy: ts.MergeStrategySelect,
		Reasoning:     "solid",
	}
	data, _ := json.Marshal(evaluation)
	if err := os.WriteFile(filepath.Join(dir, ts.EvaluationFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	checker := newJudgeCompletionChecker()
	success, _, err := checker.VerifyWork("j1", "ji1", dir, "main")
	if err != nil {
		t.Fatalf("VerifyWork: %v", err)
	}
	if !success {
		t.Error("expected success=true")
	}
}

func TestJudgeCompletionChecker_VerifyWork_NoFile(t *testing.T) {
	checker := newJudgeCompletionChecker()
	_, _, err := checker.VerifyWork("j1", "ji1", t.TempDir(), "main")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// --- sessionRecorder tests ---

func TestSessionRecorder_AllCallbacks(t *testing.T) {
	var assigned, completed, failed bool

	recorder := newSessionRecorder(sessionRecorderDeps{
		OnAssign:   func(_, _ string) { assigned = true },
		OnComplete: func(_ string, _ int) { completed = true },
		OnFailure:  func(_, _ string) { failed = true },
	})

	recorder.AssignTask("t1", "i1")
	if !assigned {
		t.Error("OnAssign not called")
	}

	recorder.RecordCompletion("t1", 1)
	if !completed {
		t.Error("OnComplete not called")
	}

	recorder.RecordFailure("t1", "reason")
	if !failed {
		t.Error("OnFailure not called")
	}
}

func TestSessionRecorder_NilCallbacks(t *testing.T) {
	recorder := newSessionRecorder(sessionRecorderDeps{})

	// Should not panic with nil callbacks.
	recorder.AssignTask("t1", "i1")
	recorder.RecordCompletion("t1", 1)
	recorder.RecordFailure("t1", "reason")
}
