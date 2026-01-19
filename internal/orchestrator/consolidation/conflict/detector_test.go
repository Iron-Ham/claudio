package conflict

import (
	"context"
	"errors"
	"testing"

	"github.com/Iron-Ham/claudio/internal/worktree"
)

// mockBranchOperator is a mock implementation of BranchOperator for testing.
type mockBranchOperator struct {
	conflictingFiles    []string
	conflictingFilesErr error
	cherryPickErr       error
	abortErr            error
	continueErr         error
	cherryPickCall      struct {
		worktreePath string
		sourceBranch string
	}
	abortCall    string
	continueCall string
}

func (m *mockBranchOperator) GetConflictingFiles(_ context.Context, worktreePath string) ([]string, error) {
	return m.conflictingFiles, m.conflictingFilesErr
}

func (m *mockBranchOperator) CherryPickBranch(_ context.Context, worktreePath, sourceBranch string) error {
	m.cherryPickCall.worktreePath = worktreePath
	m.cherryPickCall.sourceBranch = sourceBranch
	return m.cherryPickErr
}

func (m *mockBranchOperator) AbortCherryPick(_ context.Context, worktreePath string) error {
	m.abortCall = worktreePath
	return m.abortErr
}

func (m *mockBranchOperator) ContinueCherryPick(_ context.Context, worktreePath string) error {
	m.continueCall = worktreePath
	return m.continueErr
}

func TestDetector_CheckCherryPick_Success(t *testing.T) {
	mock := &mockBranchOperator{}
	detector := NewDetector(mock)

	info, err := detector.CheckCherryPick(context.Background(), "/worktree", "feature", "task-1", "Task One")
	if err != nil {
		t.Errorf("CheckCherryPick() unexpected error: %v", err)
	}
	if info != nil {
		t.Errorf("CheckCherryPick() expected nil info for successful cherry-pick, got %+v", info)
	}

	if mock.cherryPickCall.worktreePath != "/worktree" {
		t.Errorf("CherryPickBranch called with worktreePath = %q, want %q",
			mock.cherryPickCall.worktreePath, "/worktree")
	}
	if mock.cherryPickCall.sourceBranch != "feature" {
		t.Errorf("CherryPickBranch called with sourceBranch = %q, want %q",
			mock.cherryPickCall.sourceBranch, "feature")
	}
}

func TestDetector_CheckCherryPick_Conflict(t *testing.T) {
	conflictErr := &worktree.CherryPickConflictError{
		Commit:       "abc123",
		SourceBranch: "feature",
		Output:       "CONFLICT (content): Merge conflict in file.go",
	}
	mock := &mockBranchOperator{
		cherryPickErr:    conflictErr,
		conflictingFiles: []string{"file.go", "other.go"},
	}
	detector := NewDetector(mock)

	info, err := detector.CheckCherryPick(context.Background(), "/worktree", "feature", "task-1", "Task One")
	if err != nil {
		t.Errorf("CheckCherryPick() unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("CheckCherryPick() expected conflict info, got nil")
	}

	if info.TaskID != "task-1" {
		t.Errorf("info.TaskID = %q, want %q", info.TaskID, "task-1")
	}
	if info.TaskTitle != "Task One" {
		t.Errorf("info.TaskTitle = %q, want %q", info.TaskTitle, "Task One")
	}
	if info.Branch != "feature" {
		t.Errorf("info.Branch = %q, want %q", info.Branch, "feature")
	}
	if len(info.Files) != 2 || info.Files[0] != "file.go" || info.Files[1] != "other.go" {
		t.Errorf("info.Files = %v, want [file.go other.go]", info.Files)
	}
	if info.WorktreePath != "/worktree" {
		t.Errorf("info.WorktreePath = %q, want %q", info.WorktreePath, "/worktree")
	}
}

func TestDetector_CheckCherryPick_ConflictFilesError(t *testing.T) {
	conflictErr := &worktree.CherryPickConflictError{
		Commit:       "abc123",
		SourceBranch: "feature",
	}
	mock := &mockBranchOperator{
		cherryPickErr:       conflictErr,
		conflictingFilesErr: errors.New("failed to get files"),
	}
	detector := NewDetector(mock)

	info, err := detector.CheckCherryPick(context.Background(), "/worktree", "feature", "task-1", "Task One")
	if err != nil {
		t.Errorf("CheckCherryPick() unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("CheckCherryPick() expected conflict info, got nil")
	}

	// Files should be empty slice when we can't get them
	if info.Files == nil || len(info.Files) != 0 {
		t.Errorf("info.Files = %v, want empty slice", info.Files)
	}
}

func TestDetector_CheckCherryPick_OtherError(t *testing.T) {
	mock := &mockBranchOperator{
		cherryPickErr: errors.New("some other error"),
	}
	detector := NewDetector(mock)

	info, err := detector.CheckCherryPick(context.Background(), "/worktree", "feature", "task-1", "Task One")
	if err == nil {
		t.Error("CheckCherryPick() expected error, got nil")
	}
	if info != nil {
		t.Errorf("CheckCherryPick() expected nil info on non-conflict error, got %+v", info)
	}
}

func TestDetector_AbortCherryPick(t *testing.T) {
	mock := &mockBranchOperator{}
	detector := NewDetector(mock)

	err := detector.AbortCherryPick(context.Background(), "/worktree")
	if err != nil {
		t.Errorf("AbortCherryPick() unexpected error: %v", err)
	}

	if mock.abortCall != "/worktree" {
		t.Errorf("AbortCherryPick called with path = %q, want %q", mock.abortCall, "/worktree")
	}
}

func TestDetector_AbortCherryPick_Error(t *testing.T) {
	mock := &mockBranchOperator{abortErr: errors.New("abort failed")}
	detector := NewDetector(mock)

	err := detector.AbortCherryPick(context.Background(), "/worktree")
	if err == nil {
		t.Error("AbortCherryPick() expected error, got nil")
	}
}

func TestDetector_ContinueCherryPick(t *testing.T) {
	mock := &mockBranchOperator{}
	detector := NewDetector(mock)

	err := detector.ContinueCherryPick(context.Background(), "/worktree")
	if err != nil {
		t.Errorf("ContinueCherryPick() unexpected error: %v", err)
	}

	if mock.continueCall != "/worktree" {
		t.Errorf("ContinueCherryPick called with path = %q, want %q", mock.continueCall, "/worktree")
	}
}

func TestDetector_ContinueCherryPick_Error(t *testing.T) {
	mock := &mockBranchOperator{continueErr: errors.New("continue failed")}
	detector := NewDetector(mock)

	err := detector.ContinueCherryPick(context.Background(), "/worktree")
	if err == nil {
		t.Error("ContinueCherryPick() expected error, got nil")
	}
}

func TestDetector_GetConflictingFiles(t *testing.T) {
	mock := &mockBranchOperator{conflictingFiles: []string{"a.go", "b.go"}}
	detector := NewDetector(mock)

	files, err := detector.GetConflictingFiles(context.Background(), "/worktree")
	if err != nil {
		t.Errorf("GetConflictingFiles() unexpected error: %v", err)
	}

	if len(files) != 2 || files[0] != "a.go" || files[1] != "b.go" {
		t.Errorf("GetConflictingFiles() = %v, want [a.go b.go]", files)
	}
}

func TestDetector_GetConflictingFiles_Error(t *testing.T) {
	mock := &mockBranchOperator{conflictingFilesErr: errors.New("failed")}
	detector := NewDetector(mock)

	_, err := detector.GetConflictingFiles(context.Background(), "/worktree")
	if err == nil {
		t.Error("GetConflictingFiles() expected error, got nil")
	}
}
