//go:build integration

package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Ham/claudio/internal/testutil"
)

func TestNew(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "valid git repository",
			setup: func(t *testing.T) string {
				return testutil.SetupTestRepo(t)
			},
			wantErr: false,
		},
		{
			name: "non-git directory",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: true,
		},
		{
			name: "non-existent directory",
			setup: func(t *testing.T) string {
				return "/non/existent/path"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			_, err := New(dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_Create(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	branchName := "test-branch"

	// Create worktree
	if err := mgr.Create(worktreePath, branchName); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// Verify branch was created in worktree
	branch, err := mgr.GetBranch(worktreePath)
	if err != nil {
		t.Fatalf("GetBranch() error = %v", err)
	}
	if branch != branchName {
		t.Errorf("GetBranch() = %v, want %v", branch, branchName)
	}

	// Verify worktree is listed
	worktrees, err := mgr.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	resolvedPath, _ := filepath.EvalSymlinks(worktreePath)
	found := false
	for _, wt := range worktrees {
		resolvedWt, _ := filepath.EvalSymlinks(wt)
		if resolvedWt == resolvedPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created worktree not found in list: %v", worktrees)
	}
}

func TestManager_Remove(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	branchName := "test-branch"

	// Create worktree
	if err := mgr.Create(worktreePath, branchName); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Remove worktree
	if err := mgr.Remove(worktreePath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Verify worktree directory is removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree directory still exists after removal")
	}

	// Verify worktree is not listed
	worktrees, err := mgr.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	for _, wt := range worktrees {
		if wt == worktreePath {
			t.Error("removed worktree still in list")
		}
	}
}

func TestManager_HasUncommittedChanges(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Initially no uncommitted changes
	hasChanges, err := mgr.HasUncommittedChanges(repoDir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error = %v", err)
	}
	if hasChanges {
		t.Error("HasUncommittedChanges() = true, want false for clean repo")
	}

	// Create a new file
	testFile := filepath.Join(repoDir, "new-file.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Now should have uncommitted changes
	hasChanges, err = mgr.HasUncommittedChanges(repoDir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error = %v", err)
	}
	if !hasChanges {
		t.Error("HasUncommittedChanges() = false, want true after creating file")
	}
}

func TestManager_CommitAll(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a new file
	testFile := filepath.Join(repoDir, "new-file.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Commit all changes
	if err := mgr.CommitAll(repoDir, "Test commit"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Should have no uncommitted changes now
	hasChanges, err := mgr.HasUncommittedChanges(repoDir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error = %v", err)
	}
	if hasChanges {
		t.Error("HasUncommittedChanges() = true after CommitAll()")
	}
}

func TestManager_CommitAll_NothingToCommit(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// CommitAll on clean repo should not error
	if err := mgr.CommitAll(repoDir, "Empty commit"); err != nil {
		t.Errorf("CommitAll() error = %v, want nil for clean repo", err)
	}
}

func TestManager_DeleteBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a branch
	testutil.CreateBranch(t, repoDir, "feature-branch")

	// Delete the branch
	if err := mgr.DeleteBranch("feature-branch"); err != nil {
		t.Fatalf("DeleteBranch() error = %v", err)
	}

	// Deleting non-existent branch should error
	if err := mgr.DeleteBranch("non-existent-branch"); err == nil {
		t.Error("DeleteBranch() should error for non-existent branch")
	}
}

func TestManager_GetDiffAgainstMain(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a worktree with changes
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "feature"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add a file in the worktree
	testFile := filepath.Join(worktreePath, "feature.txt")
	if err := os.WriteFile(testFile, []byte("feature content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := mgr.CommitAll(worktreePath, "Add feature"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Get diff against main
	diff, err := mgr.GetDiffAgainstMain(worktreePath)
	if err != nil {
		t.Fatalf("GetDiffAgainstMain() error = %v", err)
	}

	// Diff should contain the new file
	if len(diff) == 0 {
		t.Error("GetDiffAgainstMain() returned empty diff")
	}
}

func TestManager_GetCommitLog(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a worktree with commits
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "feature"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add multiple commits
	for i := 1; i <= 3; i++ {
		testFile := filepath.Join(worktreePath, "file.txt")
		content := []byte("content " + string(rune('0'+i)))
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		if err := mgr.CommitAll(worktreePath, "Commit "+string(rune('0'+i))); err != nil {
			t.Fatalf("CommitAll() error = %v", err)
		}
	}

	// Get commit log
	log, err := mgr.GetCommitLog(worktreePath)
	if err != nil {
		t.Fatalf("GetCommitLog() error = %v", err)
	}

	if len(log) == 0 {
		t.Error("GetCommitLog() returned empty log")
	}
}

func TestManager_GetChangedFiles(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a worktree with changes
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "feature"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add files
	files := []string{"a.txt", "b.txt", "dir/c.txt"}
	for _, f := range files {
		fullPath := filepath.Join(worktreePath, f)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}
	if err := mgr.CommitAll(worktreePath, "Add files"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Get changed files
	changed, err := mgr.GetChangedFiles(worktreePath)
	if err != nil {
		t.Fatalf("GetChangedFiles() error = %v", err)
	}

	if len(changed) != len(files) {
		t.Errorf("GetChangedFiles() returned %d files, want %d", len(changed), len(files))
	}
}

func TestManager_GetChangedFiles_Empty(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a worktree without any changes
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "feature"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get changed files (should be empty)
	changed, err := mgr.GetChangedFiles(worktreePath)
	if err != nil {
		t.Fatalf("GetChangedFiles() error = %v", err)
	}

	if len(changed) != 0 {
		t.Errorf("GetChangedFiles() returned %d files, want 0", len(changed))
	}
}

func TestManager_List(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Initially should have just the main worktree
	worktrees, err := mgr.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(worktrees) != 1 {
		t.Errorf("List() returned %d worktrees, want 1", len(worktrees))
	}

	// Create additional worktrees
	for i := 1; i <= 3; i++ {
		path := filepath.Join(t.TempDir(), "wt-"+string(rune('0'+i)))
		branch := "branch-" + string(rune('0'+i))
		if err := mgr.Create(path, branch); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	// Should now have 4 worktrees
	worktrees, err = mgr.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(worktrees) != 4 {
		t.Errorf("List() returned %d worktrees, want 4", len(worktrees))
	}
}

func TestManager_findMainBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	// Test with 'main' branch (created by SetupTestRepo)
	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	branch := mgr.findMainBranch()
	if branch != "main" {
		t.Errorf("findMainBranch() = %v, want 'main'", branch)
	}
}
