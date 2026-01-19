package branch

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// mockWorktreeManager is a mock implementation of WorktreeManager for testing.
type mockWorktreeManager struct {
	mainBranch           string
	createBranchFromErr  error
	createWorktreeErr    error
	cherryPickErr        error
	changedFiles         []string
	changedFilesErr      error
	conflictingFiles     []string
	conflictingFilesErr  error
	commitCount          int
	commitCountErr       error
	pushErr              error
	removeErr            error
	deleteBranchErr      error
	continueCherryErr    error
	abortCherryErr       error
	createBranchFromCall struct {
		branchName string
		baseBranch string
	}
	createWorktreeCall struct {
		path   string
		branch string
	}
	cherryPickCall struct {
		path         string
		sourceBranch string
	}
}

func (m *mockWorktreeManager) FindMainBranch() string {
	if m.mainBranch == "" {
		return "main"
	}
	return m.mainBranch
}

func (m *mockWorktreeManager) CreateBranchFrom(branchName, baseBranch string) error {
	m.createBranchFromCall.branchName = branchName
	m.createBranchFromCall.baseBranch = baseBranch
	return m.createBranchFromErr
}

func (m *mockWorktreeManager) CreateWorktreeFromBranch(path, branch string) error {
	m.createWorktreeCall.path = path
	m.createWorktreeCall.branch = branch
	return m.createWorktreeErr
}

func (m *mockWorktreeManager) CherryPickBranch(path, sourceBranch string) error {
	m.cherryPickCall.path = path
	m.cherryPickCall.sourceBranch = sourceBranch
	return m.cherryPickErr
}

func (m *mockWorktreeManager) GetChangedFiles(_ string) ([]string, error) {
	return m.changedFiles, m.changedFilesErr
}

func (m *mockWorktreeManager) GetConflictingFiles(_ string) ([]string, error) {
	return m.conflictingFiles, m.conflictingFilesErr
}

func (m *mockWorktreeManager) CountCommitsBetween(_, _, _ string) (int, error) {
	return m.commitCount, m.commitCountErr
}

func (m *mockWorktreeManager) Push(_ string, _ bool) error {
	return m.pushErr
}

func (m *mockWorktreeManager) Remove(_ string) error {
	return m.removeErr
}

func (m *mockWorktreeManager) DeleteBranch(_ string) error {
	return m.deleteBranchErr
}

func (m *mockWorktreeManager) ContinueCherryPick(_ string) error {
	return m.continueCherryErr
}

func (m *mockWorktreeManager) AbortCherryPick(_ string) error {
	return m.abortCherryErr
}

func TestManager_FindMainBranch(t *testing.T) {
	mock := &mockWorktreeManager{mainBranch: "main"}
	mgr := New(mock)

	got := mgr.FindMainBranch(context.Background())
	if got != "main" {
		t.Errorf("FindMainBranch() = %q, want %q", got, "main")
	}
}

func TestManager_CreateGroupBranch(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		groupIdx   int
		baseBranch string
		wantBranch string
		mockErr    error
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "success first group",
			sessionID:  "abc12345",
			groupIdx:   0,
			baseBranch: "main",
			wantBranch: "Iron-Ham/ultraplan-abc12345-group-1",
		},
		{
			name:       "success second group",
			sessionID:  "abc12345",
			groupIdx:   1,
			baseBranch: "Iron-Ham/ultraplan-abc12345-group-1",
			wantBranch: "Iron-Ham/ultraplan-abc12345-group-2",
		},
		{
			name:       "error creating branch",
			sessionID:  "abc12345",
			groupIdx:   0,
			baseBranch: "main",
			mockErr:    errors.New("branch already exists"),
			wantErr:    true,
			wantErrMsg: "failed to create group branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockWorktreeManager{createBranchFromErr: tt.mockErr}
			ns := NewNamingStrategy("Iron-Ham", tt.sessionID)
			mgr := New(mock, WithNamingStrategy(ns))

			got, err := mgr.CreateGroupBranch(context.Background(), tt.groupIdx, tt.baseBranch)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateGroupBranch() expected error, got nil")
				} else if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("CreateGroupBranch() error = %v, want containing %q", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateGroupBranch() unexpected error: %v", err)
				return
			}

			if got != tt.wantBranch {
				t.Errorf("CreateGroupBranch() = %q, want %q", got, tt.wantBranch)
			}

			if mock.createBranchFromCall.branchName != tt.wantBranch {
				t.Errorf("CreateBranchFrom called with branchName = %q, want %q",
					mock.createBranchFromCall.branchName, tt.wantBranch)
			}

			if mock.createBranchFromCall.baseBranch != tt.baseBranch {
				t.Errorf("CreateBranchFrom called with baseBranch = %q, want %q",
					mock.createBranchFromCall.baseBranch, tt.baseBranch)
			}
		})
	}
}

func TestManager_CreateSingleBranch(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		baseBranch string
		wantBranch string
		mockErr    error
		wantErr    bool
	}{
		{
			name:       "success",
			sessionID:  "xyz98765",
			baseBranch: "main",
			wantBranch: "Iron-Ham/ultraplan-xyz98765",
		},
		{
			name:       "error",
			sessionID:  "xyz98765",
			baseBranch: "main",
			mockErr:    errors.New("failed"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockWorktreeManager{createBranchFromErr: tt.mockErr}
			ns := NewNamingStrategy("Iron-Ham", tt.sessionID)
			mgr := New(mock, WithNamingStrategy(ns))

			got, err := mgr.CreateSingleBranch(context.Background(), tt.baseBranch)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateSingleBranch() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CreateSingleBranch() unexpected error: %v", err)
				return
			}

			if got != tt.wantBranch {
				t.Errorf("CreateSingleBranch() = %q, want %q", got, tt.wantBranch)
			}
		})
	}
}

func TestManager_CreateWorktree(t *testing.T) {
	mock := &mockWorktreeManager{}
	mgr := New(mock)

	err := mgr.CreateWorktree(context.Background(), "/path/to/worktree", "feature-branch")
	if err != nil {
		t.Errorf("CreateWorktree() unexpected error: %v", err)
	}

	if mock.createWorktreeCall.path != "/path/to/worktree" {
		t.Errorf("CreateWorktreeFromBranch called with path = %q, want %q",
			mock.createWorktreeCall.path, "/path/to/worktree")
	}

	if mock.createWorktreeCall.branch != "feature-branch" {
		t.Errorf("CreateWorktreeFromBranch called with branch = %q, want %q",
			mock.createWorktreeCall.branch, "feature-branch")
	}
}

func TestManager_CreateWorktree_Error(t *testing.T) {
	mock := &mockWorktreeManager{createWorktreeErr: errors.New("worktree exists")}
	mgr := New(mock)

	err := mgr.CreateWorktree(context.Background(), "/path", "branch")
	if err == nil {
		t.Error("CreateWorktree() expected error, got nil")
	}
}

func TestManager_CherryPickBranch(t *testing.T) {
	mock := &mockWorktreeManager{}
	mgr := New(mock)

	err := mgr.CherryPickBranch(context.Background(), "/worktree", "source-branch")
	if err != nil {
		t.Errorf("CherryPickBranch() unexpected error: %v", err)
	}

	if mock.cherryPickCall.path != "/worktree" {
		t.Errorf("CherryPickBranch called with path = %q, want %q",
			mock.cherryPickCall.path, "/worktree")
	}

	if mock.cherryPickCall.sourceBranch != "source-branch" {
		t.Errorf("CherryPickBranch called with sourceBranch = %q, want %q",
			mock.cherryPickCall.sourceBranch, "source-branch")
	}
}

func TestManager_GetChangedFiles(t *testing.T) {
	mock := &mockWorktreeManager{changedFiles: []string{"file1.go", "file2.go"}}
	mgr := New(mock)

	got, err := mgr.GetChangedFiles(context.Background(), "/worktree")
	if err != nil {
		t.Errorf("GetChangedFiles() unexpected error: %v", err)
	}

	if len(got) != 2 || got[0] != "file1.go" || got[1] != "file2.go" {
		t.Errorf("GetChangedFiles() = %v, want [file1.go file2.go]", got)
	}
}

func TestManager_GetConflictingFiles(t *testing.T) {
	mock := &mockWorktreeManager{conflictingFiles: []string{"conflict.go"}}
	mgr := New(mock)

	got, err := mgr.GetConflictingFiles(context.Background(), "/worktree")
	if err != nil {
		t.Errorf("GetConflictingFiles() unexpected error: %v", err)
	}

	if len(got) != 1 || got[0] != "conflict.go" {
		t.Errorf("GetConflictingFiles() = %v, want [conflict.go]", got)
	}
}

func TestManager_CountCommitsBetween(t *testing.T) {
	mock := &mockWorktreeManager{commitCount: 5}
	mgr := New(mock)

	got, err := mgr.CountCommitsBetween(context.Background(), "/worktree", "main", "feature")
	if err != nil {
		t.Errorf("CountCommitsBetween() unexpected error: %v", err)
	}

	if got != 5 {
		t.Errorf("CountCommitsBetween() = %d, want 5", got)
	}
}

func TestManager_Push(t *testing.T) {
	mock := &mockWorktreeManager{}
	mgr := New(mock)

	err := mgr.Push(context.Background(), "/worktree", false)
	if err != nil {
		t.Errorf("Push() unexpected error: %v", err)
	}
}

func TestManager_RemoveWorktree(t *testing.T) {
	mock := &mockWorktreeManager{}
	mgr := New(mock)

	err := mgr.RemoveWorktree(context.Background(), "/worktree")
	if err != nil {
		t.Errorf("RemoveWorktree() unexpected error: %v", err)
	}
}

func TestManager_DeleteBranch(t *testing.T) {
	mock := &mockWorktreeManager{}
	mgr := New(mock)

	err := mgr.DeleteBranch(context.Background(), "feature-branch")
	if err != nil {
		t.Errorf("DeleteBranch() unexpected error: %v", err)
	}
}

func TestManager_CherryPickOperations(t *testing.T) {
	t.Run("ContinueCherryPick", func(t *testing.T) {
		mock := &mockWorktreeManager{}
		mgr := New(mock)

		err := mgr.ContinueCherryPick(context.Background(), "/worktree")
		if err != nil {
			t.Errorf("ContinueCherryPick() unexpected error: %v", err)
		}
	})

	t.Run("AbortCherryPick", func(t *testing.T) {
		mock := &mockWorktreeManager{}
		mgr := New(mock)

		err := mgr.AbortCherryPick(context.Background(), "/worktree")
		if err != nil {
			t.Errorf("AbortCherryPick() unexpected error: %v", err)
		}
	})
}

func TestManager_NamingStrategy(t *testing.T) {
	ns := NewNamingStrategy("test", "session")
	mgr := New(&mockWorktreeManager{}, WithNamingStrategy(ns))

	got := mgr.NamingStrategy()
	if got != ns {
		t.Errorf("NamingStrategy() returned different instance")
	}
}
