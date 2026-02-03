package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Ham/claudio/internal/testutil"
)

// TestManagerCreate tests the Manager.Create method
func TestManagerCreate(t *testing.T) {
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

// TestManagerCreateFromBranch tests creating a worktree from a specific base branch
func TestManagerCreateFromBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a base branch with some commits
	testutil.CommitFile(t, repoDir, "base-file.txt", "base content", "Add base file")

	worktreePath := filepath.Join(t.TempDir(), "from-branch-worktree")
	newBranch := "feature-from-main"

	// Create worktree from main branch
	if err := mgr.CreateFromBranch(worktreePath, newBranch, "main"); err != nil {
		t.Fatalf("CreateFromBranch() error = %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// Verify branch was created
	branch, err := mgr.GetBranch(worktreePath)
	if err != nil {
		t.Fatalf("GetBranch() error = %v", err)
	}
	if branch != newBranch {
		t.Errorf("GetBranch() = %v, want %v", branch, newBranch)
	}

	// Verify base file exists (inherited from main)
	baseFile := filepath.Join(worktreePath, "base-file.txt")
	if _, err := os.Stat(baseFile); os.IsNotExist(err) {
		t.Error("base file not found in worktree - branch didn't inherit from main")
	}
}

// TestManagerRemove tests the Manager.Remove method
func TestManagerRemove(t *testing.T) {
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

// TestManagerHasUncommittedChanges tests the HasUncommittedChanges method
func TestManagerHasUncommittedChanges(t *testing.T) {
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

// TestManagerCommitAll tests the CommitAll method
func TestManagerCommitAll(t *testing.T) {
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

// TestManagerCommitAllNothingToCommit tests CommitAll on a clean repo
func TestManagerCommitAllNothingToCommit(t *testing.T) {
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

// TestManagerDeleteBranch tests the DeleteBranch method
func TestManagerDeleteBranch(t *testing.T) {
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

// TestManagerList tests the List method
func TestManagerList(t *testing.T) {
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

// TestManagerGetDiffAgainstMain tests the GetDiffAgainstMain method
func TestManagerGetDiffAgainstMain(t *testing.T) {
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

// TestManagerGetCommitLog tests the GetCommitLog method
func TestManagerGetCommitLog(t *testing.T) {
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

// TestManagerGetChangedFiles tests the GetChangedFiles method
func TestManagerGetChangedFiles(t *testing.T) {
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

// TestManagerGetChangedFilesEmpty tests GetChangedFiles when there are no changes
func TestManagerGetChangedFilesEmpty(t *testing.T) {
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

// TestManagerFindMainBranch tests the findMainBranch method
func TestManagerFindMainBranch(t *testing.T) {
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

	// Also test the exported version
	exportedBranch := mgr.FindMainBranch()
	if exportedBranch != "main" {
		t.Errorf("FindMainBranch() = %v, want 'main'", exportedBranch)
	}
}

// TestManagerListBranches tests the ListBranches method
func TestManagerListBranches(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Initially should have just 'main' branch
	branches, err := mgr.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	if len(branches) != 1 {
		t.Errorf("ListBranches() returned %d branches, want 1", len(branches))
	}
	if branches[0].Name != "main" {
		t.Errorf("ListBranches()[0].Name = %v, want 'main'", branches[0].Name)
	}
	if !branches[0].IsMain {
		t.Error("ListBranches()[0].IsMain = false, want true")
	}

	// Create additional branches
	testutil.CreateBranch(t, repoDir, "feature-1")
	testutil.CreateBranch(t, repoDir, "feature-2")

	branches, err = mgr.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	if len(branches) != 3 {
		t.Errorf("ListBranches() returned %d branches, want 3", len(branches))
	}

	// First branch should be main
	if branches[0].Name != "main" || !branches[0].IsMain {
		t.Errorf("ListBranches() first branch should be main, got %v", branches[0])
	}

	// Other branches should not be marked as main
	for _, b := range branches[1:] {
		if b.IsMain {
			t.Errorf("ListBranches() branch %v.IsMain = true, want false", b.Name)
		}
	}
}

// TestManagerListBranchesCurrentBranch tests that IsCurrent is set correctly
func TestManagerListBranchesCurrentBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a feature branch
	testutil.CreateBranch(t, repoDir, "feature-branch")

	branches, err := mgr.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}

	// Find the current branch and verify IsCurrent is set correctly
	foundCurrent := false
	for _, b := range branches {
		if b.IsCurrent {
			foundCurrent = true
			// After CreateBranch, we should still be on main
			if b.Name != "main" {
				t.Errorf("IsCurrent branch should be 'main', got %v", b.Name)
			}
		}
	}
	if !foundCurrent {
		t.Error("ListBranches() did not mark any branch as current")
	}
}

// TestManagerCreateBranchFrom tests creating a branch from a base branch
func TestManagerCreateBranchFrom(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a new branch from main
	if err := mgr.CreateBranchFrom("new-feature", "main"); err != nil {
		t.Fatalf("CreateBranchFrom() error = %v", err)
	}

	// Verify branch was created
	branches, err := mgr.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}

	found := false
	for _, b := range branches {
		if b.Name == "new-feature" {
			found = true
			break
		}
	}
	if !found {
		t.Error("CreateBranchFrom() did not create the branch")
	}
}

// TestManagerCreateWorktreeFromBranch tests creating a worktree from an existing branch
func TestManagerCreateWorktreeFromBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// First create a branch
	if err := mgr.CreateBranchFrom("existing-branch", "main"); err != nil {
		t.Fatalf("CreateBranchFrom() error = %v", err)
	}

	// Now create a worktree from that branch
	worktreePath := filepath.Join(t.TempDir(), "wt-from-branch")
	if err := mgr.CreateWorktreeFromBranch(worktreePath, "existing-branch"); err != nil {
		t.Fatalf("CreateWorktreeFromBranch() error = %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// Verify it's on the right branch
	branch, err := mgr.GetBranch(worktreePath)
	if err != nil {
		t.Fatalf("GetBranch() error = %v", err)
	}
	if branch != "existing-branch" {
		t.Errorf("GetBranch() = %v, want 'existing-branch'", branch)
	}
}

// TestManagerHasCommitsBeyond tests the HasCommitsBeyond method
func TestManagerHasCommitsBeyond(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a worktree
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "feature"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Initially no commits beyond main
	hasCommits, err := mgr.HasCommitsBeyond(worktreePath, "main")
	if err != nil {
		t.Fatalf("HasCommitsBeyond() error = %v", err)
	}
	if hasCommits {
		t.Error("HasCommitsBeyond() = true, want false for new branch")
	}

	// Add a commit
	testFile := filepath.Join(worktreePath, "new-file.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := mgr.CommitAll(worktreePath, "Add file"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Now should have commits beyond main
	hasCommits, err = mgr.HasCommitsBeyond(worktreePath, "main")
	if err != nil {
		t.Fatalf("HasCommitsBeyond() error = %v", err)
	}
	if !hasCommits {
		t.Error("HasCommitsBeyond() = false, want true after adding commit")
	}
}

// TestManagerGetCommitsBetween tests the GetCommitsBetween method
func TestManagerGetCommitsBetween(t *testing.T) {
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

	// Get commits between main and HEAD
	commits, err := mgr.GetCommitsBetween(worktreePath, "main", "HEAD")
	if err != nil {
		t.Fatalf("GetCommitsBetween() error = %v", err)
	}

	if len(commits) != 3 {
		t.Errorf("GetCommitsBetween() returned %d commits, want 3", len(commits))
	}
}

// TestManagerCountCommitsBetween tests the CountCommitsBetween method
func TestManagerCountCommitsBetween(t *testing.T) {
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

	// Initially zero commits
	count, err := mgr.CountCommitsBetween(worktreePath, "main", "HEAD")
	if err != nil {
		t.Fatalf("CountCommitsBetween() error = %v", err)
	}
	if count != 0 {
		t.Errorf("CountCommitsBetween() = %d, want 0", count)
	}

	// Add commits
	for i := 1; i <= 5; i++ {
		testFile := filepath.Join(worktreePath, "file.txt")
		content := []byte("content " + string(rune('0'+i)))
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		if err := mgr.CommitAll(worktreePath, "Commit "+string(rune('0'+i))); err != nil {
			t.Fatalf("CommitAll() error = %v", err)
		}
	}

	// Count commits
	count, err = mgr.CountCommitsBetween(worktreePath, "main", "HEAD")
	if err != nil {
		t.Fatalf("CountCommitsBetween() error = %v", err)
	}
	if count != 5 {
		t.Errorf("CountCommitsBetween() = %d, want 5", count)
	}
}

// TestManagerCopyLocalClaudeFiles tests the CopyLocalClaudeFiles method
func TestManagerCopyLocalClaudeFiles(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name        string
		setupSource func(t *testing.T, repoDir string) // Setup source files in repo
		wantCopied  []string                           // Files expected to be copied
		wantErr     bool
	}{
		{
			name: "copies CLAUDE.local.md when present",
			setupSource: func(t *testing.T, repoDir string) {
				content := []byte("# Local Claude Settings\ntest content")
				if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.local.md"), content, 0644); err != nil {
					t.Fatalf("failed to create source file: %v", err)
				}
			},
			wantCopied: []string{"CLAUDE.local.md"},
			wantErr:    false,
		},
		{
			name:        "no error when CLAUDE.local.md does not exist",
			setupSource: func(t *testing.T, repoDir string) {},
			wantErr:     false,
		},
		{
			name: "preserves unicode content",
			setupSource: func(t *testing.T, repoDir string) {
				content := []byte("special test content with unicode: éàü 中文 🎉")
				if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.local.md"), content, 0644); err != nil {
					t.Fatalf("failed to create source file: %v", err)
				}
			},
			wantCopied: []string{"CLAUDE.local.md"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := testutil.SetupTestRepo(t)
			mgr, err := New(repoDir)
			if err != nil {
				t.Fatalf("failed to create manager: %v", err)
			}

			// Setup source files
			tt.setupSource(t, repoDir)

			// Create worktree
			worktreePath := filepath.Join(t.TempDir(), "test-worktree")
			if err := mgr.Create(worktreePath, "test-branch"); err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			// Copy local claude files
			err = mgr.CopyLocalClaudeFiles(worktreePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("CopyLocalClaudeFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify expected files were copied
			for _, filename := range tt.wantCopied {
				srcPath := filepath.Join(repoDir, filename)
				dstPath := filepath.Join(worktreePath, filename)

				// Check destination exists
				if _, err := os.Stat(dstPath); os.IsNotExist(err) {
					t.Errorf("expected file %s to be copied but it doesn't exist", filename)
					continue
				}

				// Verify content matches
				srcContent, err := os.ReadFile(srcPath)
				if err != nil {
					t.Fatalf("failed to read source file: %v", err)
				}
				dstContent, err := os.ReadFile(dstPath)
				if err != nil {
					t.Fatalf("failed to read destination file: %v", err)
				}
				if string(srcContent) != string(dstContent) {
					t.Errorf("file content mismatch for %s: got %q, want %q", filename, dstContent, srcContent)
				}
			}
		})
	}
}

func TestManagerCopyLocalConfigFiles(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name        string
		setupSource func(t *testing.T, repoDir string)
		files       []string
		wantCopied  []string
		wantErr     bool
	}{
		{
			name: "copies codex config file when present",
			setupSource: func(t *testing.T, repoDir string) {
				content := []byte("# Local Codex Settings\nconfig")
				if err := os.WriteFile(filepath.Join(repoDir, "CODEX.local.md"), content, 0644); err != nil {
					t.Fatalf("failed to create source file: %v", err)
				}
			},
			files:      []string{"CODEX.local.md"},
			wantCopied: []string{"CODEX.local.md"},
			wantErr:    false,
		},
		{
			name:        "no error when files do not exist",
			setupSource: func(t *testing.T, repoDir string) {},
			files:       []string{"MISSING.local.md"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := testutil.SetupTestRepo(t)
			mgr, err := New(repoDir)
			if err != nil {
				t.Fatalf("failed to create manager: %v", err)
			}

			tt.setupSource(t, repoDir)

			worktreePath := filepath.Join(t.TempDir(), "test-worktree")
			if err := mgr.Create(worktreePath, "test-branch"); err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			err = mgr.CopyLocalConfigFiles(worktreePath, tt.files, "Codex")
			if (err != nil) != tt.wantErr {
				t.Errorf("CopyLocalConfigFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for _, filename := range tt.wantCopied {
				dstPath := filepath.Join(worktreePath, filename)
				if _, err := os.Stat(dstPath); os.IsNotExist(err) {
					t.Errorf("expected file %s to be copied but it doesn't exist", filename)
				}
			}
		})
	}
}

// TestManagerCopyLocalClaudeFilesPreservesPermissions tests permission preservation
func TestManagerCopyLocalClaudeFilesPreservesPermissions(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create source file with specific permissions
	srcPath := filepath.Join(repoDir, "CLAUDE.local.md")
	if err := os.WriteFile(srcPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create worktree
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "test-branch"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Copy files
	if err := mgr.CopyLocalClaudeFiles(worktreePath); err != nil {
		t.Fatalf("CopyLocalClaudeFiles() error = %v", err)
	}

	// Check permissions are preserved
	dstPath := filepath.Join(worktreePath, "CLAUDE.local.md")
	srcInfo, _ := os.Stat(srcPath)
	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("failed to stat destination file: %v", err)
	}

	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("permissions not preserved: got %v, want %v", dstInfo.Mode(), srcInfo.Mode())
	}
}

// TestNewFromSubdirectory tests creating a Manager from a subdirectory
func TestNewFromSubdirectory(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)

	// Create a subdirectory
	subDir := filepath.Join(repoDir, "sub", "nested", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Create Manager from subdirectory
	mgr, err := New(subDir)
	if err != nil {
		t.Fatalf("New() from subdirectory error = %v", err)
	}

	// Should still work correctly
	branches, err := mgr.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	if len(branches) != 1 {
		t.Errorf("ListBranches() returned %d branches, want 1", len(branches))
	}
}

// TestNewNonGitDirectory tests creating a Manager from a non-git directory
func TestNewNonGitDirectory(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tmpDir := t.TempDir()
	_, err := New(tmpDir)
	if err == nil {
		t.Error("New() should error for non-git directory")
	}
}

// TestFindGitRootFunction tests the FindGitRoot function
func TestFindGitRootFunction(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)

	// Test from repository root
	root, err := FindGitRoot(repoDir)
	if err != nil {
		t.Fatalf("FindGitRoot() from root error = %v", err)
	}
	resolvedRepo, _ := filepath.EvalSymlinks(repoDir)
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	if resolvedRoot != resolvedRepo {
		t.Errorf("FindGitRoot() = %v, want %v", root, repoDir)
	}

	// Test from subdirectory
	subDir := filepath.Join(repoDir, "deep", "nested", "path")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	root, err = FindGitRoot(subDir)
	if err != nil {
		t.Fatalf("FindGitRoot() from subdirectory error = %v", err)
	}
	resolvedRoot, _ = filepath.EvalSymlinks(root)
	if resolvedRoot != resolvedRepo {
		t.Errorf("FindGitRoot() from subdirectory = %v, want %v", root, repoDir)
	}

	// Test from non-git directory
	nonGitDir := t.TempDir()
	_, err = FindGitRoot(nonGitDir)
	if err == nil {
		t.Error("FindGitRoot() should error for non-git directory")
	}
}

// TestIsCherryPickInProgress tests the IsCherryPickInProgress method
func TestIsCherryPickInProgress(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// No cherry-pick in progress
	if mgr.IsCherryPickInProgress(repoDir) {
		t.Error("IsCherryPickInProgress() = true, want false for clean repo")
	}
}

// TestGetConflictingFiles tests the GetConflictingFiles method
func TestGetConflictingFiles(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// No conflicts in clean repo
	files, err := mgr.GetConflictingFiles(repoDir)
	if err != nil {
		t.Fatalf("GetConflictingFiles() error = %v", err)
	}
	if len(files) != 0 {
		t.Errorf("GetConflictingFiles() returned %d files, want 0", len(files))
	}
}
