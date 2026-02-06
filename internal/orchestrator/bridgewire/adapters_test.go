package bridgewire

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/bridge"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/verify"
)

// --- Mock Verifier ---

type mockVerifier struct {
	completionResult bool
	completionErr    error
	verifyResult     verify.TaskCompletionResult
}

func (v *mockVerifier) CheckCompletionFile(worktreePath string) (bool, error) {
	return v.completionResult, v.completionErr
}

func (v *mockVerifier) VerifyTaskWork(taskID, instanceID, worktreePath, baseBranch string, opts *verify.TaskVerifyOptions) verify.TaskCompletionResult {
	return v.verifyResult
}

// --- Tests ---

func TestNewCompletionChecker_CheckCompletion(t *testing.T) {
	v := &mockVerifier{completionResult: true}
	checker := NewCompletionChecker(v)

	done, err := checker.CheckCompletion("/tmp/wt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true")
	}
}

func TestNewCompletionChecker_VerifyWorkSuccess(t *testing.T) {
	v := &mockVerifier{
		verifyResult: verify.TaskCompletionResult{
			Success:     true,
			CommitCount: 3,
		},
	}
	checker := NewCompletionChecker(v)

	success, commits, err := checker.VerifyWork("t1", "inst-1", "/tmp/wt", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !success {
		t.Error("expected success=true")
	}
	if commits != 3 {
		t.Errorf("commits = %d, want 3", commits)
	}
}

func TestNewCompletionChecker_VerifyWorkFailure(t *testing.T) {
	v := &mockVerifier{
		verifyResult: verify.TaskCompletionResult{
			Success:     false,
			CommitCount: 0,
			Error:       "no commits produced",
		},
	}
	checker := NewCompletionChecker(v)

	success, commits, err := checker.VerifyWork("t1", "inst-1", "/tmp/wt", "main")
	if success {
		t.Error("expected success=false")
	}
	if commits != 0 {
		t.Errorf("commits = %d, want 0", commits)
	}
	if err == nil {
		t.Error("expected error for failed verification")
	}
}

func TestNewSessionRecorder(t *testing.T) {
	var assignedTask, assignedInst string
	var completedTask string
	var completedCommits int
	var failedTask, failedReason string

	recorder := NewSessionRecorder(SessionRecorderDeps{
		OnAssign: func(taskID, instanceID string) {
			assignedTask = taskID
			assignedInst = instanceID
		},
		OnComplete: func(taskID string, commitCount int) {
			completedTask = taskID
			completedCommits = commitCount
		},
		OnFailure: func(taskID, reason string) {
			failedTask = taskID
			failedReason = reason
		},
	})

	recorder.AssignTask("t1", "inst-1")
	if assignedTask != "t1" || assignedInst != "inst-1" {
		t.Errorf("AssignTask: got (%q, %q), want (%q, %q)", assignedTask, assignedInst, "t1", "inst-1")
	}

	recorder.RecordCompletion("t2", 5)
	if completedTask != "t2" || completedCommits != 5 {
		t.Errorf("RecordCompletion: got (%q, %d), want (%q, %d)", completedTask, completedCommits, "t2", 5)
	}

	recorder.RecordFailure("t3", "timeout")
	if failedTask != "t3" || failedReason != "timeout" {
		t.Errorf("RecordFailure: got (%q, %q), want (%q, %q)", failedTask, failedReason, "t3", "timeout")
	}
}

func TestNewSessionRecorder_NilCallbacks(t *testing.T) {
	recorder := NewSessionRecorder(SessionRecorderDeps{})

	// Should not panic with nil callbacks.
	recorder.AssignTask("t1", "inst-1")
	recorder.RecordCompletion("t1", 1)
	recorder.RecordFailure("t1", "reason")
}

func TestNewInstanceFactory(t *testing.T) {
	orch := &orchestrator.Orchestrator{}
	sess := &orchestrator.Session{}
	f := NewInstanceFactory(orch, sess)
	if f == nil {
		t.Fatal("NewInstanceFactory returned nil")
	}
}

func TestOrchInstance_Methods(t *testing.T) {
	inst := &orchInstance{inst: &orchestrator.Instance{
		ID:           "test-id",
		WorktreePath: "/tmp/wt",
		Branch:       "feature-branch",
	}}
	if inst.ID() != "test-id" {
		t.Errorf("ID() = %q, want %q", inst.ID(), "test-id")
	}
	if inst.WorktreePath() != "/tmp/wt" {
		t.Errorf("WorktreePath() = %q, want %q", inst.WorktreePath(), "/tmp/wt")
	}
	if inst.Branch() != "feature-branch" {
		t.Errorf("Branch() = %q, want %q", inst.Branch(), "feature-branch")
	}
}

// Verify interface compliance at compile time.
var (
	_ bridge.CompletionChecker = (*completionChecker)(nil)
	_ bridge.SessionRecorder   = (*sessionRecorder)(nil)
	_ bridge.InstanceFactory   = (*instanceFactory)(nil)
	_ bridge.Instance          = (*orchInstance)(nil)
)
