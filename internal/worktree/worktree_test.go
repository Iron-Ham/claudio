//go:build integration

package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/testutil"
)

func TestFindGitRoot(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name    string
		setup   func(t *testing.T) (startDir string, wantRoot string)
		wantErr bool
	}{
		{
			name: "from repository root",
			setup: func(t *testing.T) (string, string) {
				repoDir := testutil.SetupTestRepo(t)
				return repoDir, repoDir
			},
			wantErr: false,
		},
		{
			name: "from subdirectory",
			setup: func(t *testing.T) (string, string) {
				repoDir := testutil.SetupTestRepo(t)
				// Create a nested subdirectory
				subDir := filepath.Join(repoDir, "web_app", "src", "components")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					t.Fatalf("failed to create subdirectory: %v", err)
				}
				return subDir, repoDir
			},
			wantErr: false,
		},
		{
			name: "from deeply nested subdirectory",
			setup: func(t *testing.T) (string, string) {
				repoDir := testutil.SetupTestRepo(t)
				// Create a very deep nested subdirectory
				subDir := filepath.Join(repoDir, "a", "b", "c", "d", "e", "f")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					t.Fatalf("failed to create subdirectory: %v", err)
				}
				return subDir, repoDir
			},
			wantErr: false,
		},
		{
			name: "non-git directory",
			setup: func(t *testing.T) (string, string) {
				return t.TempDir(), ""
			},
			wantErr: true,
		},
		{
			name: "non-existent directory",
			setup: func(t *testing.T) (string, string) {
				return "/non/existent/path", ""
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startDir, wantRoot := tt.setup(t)
			gotRoot, err := FindGitRoot(startDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindGitRoot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Resolve symlinks for comparison (macOS /var -> /private/var)
				resolvedWant, _ := filepath.EvalSymlinks(wantRoot)
				resolvedGot, _ := filepath.EvalSymlinks(gotRoot)
				if resolvedGot != resolvedWant {
					t.Errorf("FindGitRoot() = %v, want %v", gotRoot, wantRoot)
				}
			}
		})
	}
}

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
			name: "from subdirectory of git repository",
			setup: func(t *testing.T) string {
				repoDir := testutil.SetupTestRepo(t)
				subDir := filepath.Join(repoDir, "ios_app")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					t.Fatalf("failed to create subdirectory: %v", err)
				}
				return subDir
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

func TestManager_ListBranches(t *testing.T) {
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

func TestManager_ListBranches_CurrentBranch(t *testing.T) {
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

func TestManager_CopyLocalClaudeFiles(t *testing.T) {
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

func TestManager_CopyLocalClaudeFiles_PreservesPermissions(t *testing.T) {
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

func TestManager_CreateFromBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a base branch with some commits
	testutil.CreateBranch(t, repoDir, "base-branch")
	testutil.CheckoutBranch(t, repoDir, "base-branch")
	testutil.CommitFile(t, repoDir, "base.txt", "base content", "Add base file")
	testutil.CheckoutBranch(t, repoDir, "main")

	// Create a worktree from the base branch
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.CreateFromBranch(worktreePath, "feature-branch", "base-branch"); err != nil {
		t.Fatalf("CreateFromBranch() error = %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// Verify branch is checked out
	branch, err := mgr.GetBranch(worktreePath)
	if err != nil {
		t.Fatalf("GetBranch() error = %v", err)
	}
	if branch != "feature-branch" {
		t.Errorf("GetBranch() = %v, want feature-branch", branch)
	}

	// Verify the worktree contains the file from base-branch
	baseFile := filepath.Join(worktreePath, "base.txt")
	if _, err := os.Stat(baseFile); os.IsNotExist(err) {
		t.Error("worktree should contain base.txt from base-branch")
	}
}

func TestManager_CreateBranchFrom(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a base branch with some commits
	testutil.CreateBranch(t, repoDir, "base-branch")
	testutil.CheckoutBranch(t, repoDir, "base-branch")
	testutil.CommitFile(t, repoDir, "base.txt", "base content", "Add base file")
	testutil.CheckoutBranch(t, repoDir, "main")

	// Create a branch from the base branch
	if err := mgr.CreateBranchFrom("derived-branch", "base-branch"); err != nil {
		t.Fatalf("CreateBranchFrom() error = %v", err)
	}

	// Verify branch was created by listing branches
	branches, err := mgr.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}

	found := false
	for _, b := range branches {
		if b.Name == "derived-branch" {
			found = true
			break
		}
	}
	if !found {
		t.Error("CreateBranchFrom() did not create the branch")
	}
}

func TestManager_CreateWorktreeFromBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a branch with some commits
	testutil.CreateBranch(t, repoDir, "feature-branch")
	testutil.CheckoutBranch(t, repoDir, "feature-branch")
	testutil.CommitFile(t, repoDir, "feature.txt", "feature content", "Add feature file")
	testutil.CheckoutBranch(t, repoDir, "main")

	// Create a worktree from the existing branch
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.CreateWorktreeFromBranch(worktreePath, "feature-branch"); err != nil {
		t.Fatalf("CreateWorktreeFromBranch() error = %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// Verify branch is checked out
	branch, err := mgr.GetBranch(worktreePath)
	if err != nil {
		t.Fatalf("GetBranch() error = %v", err)
	}
	if branch != "feature-branch" {
		t.Errorf("GetBranch() = %v, want feature-branch", branch)
	}

	// Verify the worktree contains the file from the branch
	featureFile := filepath.Join(worktreePath, "feature.txt")
	if _, err := os.Stat(featureFile); os.IsNotExist(err) {
		t.Error("worktree should contain feature.txt from feature-branch")
	}
}

func TestManager_Push(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir, _ := testutil.SetupTestRepoWithRemote(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a new branch with commits
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "push-test-branch"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add a commit
	testFile := filepath.Join(worktreePath, "pushed.txt")
	if err := os.WriteFile(testFile, []byte("push content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := mgr.CommitAll(worktreePath, "Add pushed file"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Push without force
	if err := mgr.Push(worktreePath, false); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	// Push with force (should succeed as no one else has pushed)
	testFile2 := filepath.Join(worktreePath, "pushed2.txt")
	if err := os.WriteFile(testFile2, []byte("more content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := mgr.CommitAll(worktreePath, "Add another file"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	if err := mgr.Push(worktreePath, true); err != nil {
		t.Fatalf("Push(force=true) error = %v", err)
	}
}

func TestManager_HasCommitsBeyond(t *testing.T) {
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
	testFile := filepath.Join(worktreePath, "feature.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := mgr.CommitAll(worktreePath, "Add feature"); err != nil {
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

func TestManager_GetCommitsBetween(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a branch with multiple commits
	testutil.CreateBranch(t, repoDir, "feature")
	testutil.CheckoutBranch(t, repoDir, "feature")

	for i := 1; i <= 3; i++ {
		testutil.CommitFile(t, repoDir, "file.txt", "content "+string(rune('0'+i)), "Commit "+string(rune('0'+i)))
	}

	// Get commits between main and feature
	commits, err := mgr.GetCommitsBetween(repoDir, "main", "feature")
	if err != nil {
		t.Fatalf("GetCommitsBetween() error = %v", err)
	}

	if len(commits) != 3 {
		t.Errorf("GetCommitsBetween() returned %d commits, want 3", len(commits))
	}
}

func TestManager_GetCommitsBetween_Empty(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a branch without any additional commits
	testutil.CreateBranch(t, repoDir, "feature")

	// Get commits between main and feature (should be empty)
	commits, err := mgr.GetCommitsBetween(repoDir, "main", "feature")
	if err != nil {
		t.Fatalf("GetCommitsBetween() error = %v", err)
	}

	if len(commits) != 0 {
		t.Errorf("GetCommitsBetween() returned %d commits, want 0", len(commits))
	}
}

func TestManager_CountCommitsBetween(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a branch with multiple commits
	testutil.CreateBranch(t, repoDir, "feature")
	testutil.CheckoutBranch(t, repoDir, "feature")

	for i := 1; i <= 5; i++ {
		testutil.CommitFile(t, repoDir, "file.txt", "content "+string(rune('0'+i)), "Commit "+string(rune('0'+i)))
	}

	// Count commits between main and feature
	count, err := mgr.CountCommitsBetween(repoDir, "main", "feature")
	if err != nil {
		t.Fatalf("CountCommitsBetween() error = %v", err)
	}

	if count != 5 {
		t.Errorf("CountCommitsBetween() = %d, want 5", count)
	}
}

func TestManager_FindMainBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	// Test with 'main' branch (created by SetupTestRepo)
	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	branch := mgr.FindMainBranch()
	if branch != "main" {
		t.Errorf("FindMainBranch() = %v, want 'main'", branch)
	}
}

func TestManager_IsCherryPickInProgress(t *testing.T) {
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

	// Initially no cherry-pick in progress
	if mgr.IsCherryPickInProgress(worktreePath) {
		t.Error("IsCherryPickInProgress() = true, want false")
	}
}

func TestManager_GetConflictingFiles(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Initially no conflicting files
	files, err := mgr.GetConflictingFiles(repoDir)
	if err != nil {
		t.Fatalf("GetConflictingFiles() error = %v", err)
	}

	if len(files) != 0 {
		t.Errorf("GetConflictingFiles() returned %d files, want 0", len(files))
	}
}

func TestManager_CherryPickBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a source branch with commits
	testutil.CreateBranch(t, repoDir, "source-branch")
	testutil.CheckoutBranch(t, repoDir, "source-branch")
	testutil.CommitFile(t, repoDir, "source1.txt", "source content 1", "Add source1")
	testutil.CommitFile(t, repoDir, "source2.txt", "source content 2", "Add source2")

	// Create a target worktree from main
	testutil.CheckoutBranch(t, repoDir, "main")
	worktreePath := filepath.Join(t.TempDir(), "target-worktree")
	if err := mgr.Create(worktreePath, "target-branch"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Cherry-pick from source branch
	if err := mgr.CherryPickBranch(worktreePath, "source-branch"); err != nil {
		t.Fatalf("CherryPickBranch() error = %v", err)
	}

	// Verify the files from source branch are now in target
	if _, err := os.Stat(filepath.Join(worktreePath, "source1.txt")); os.IsNotExist(err) {
		t.Error("source1.txt should exist after cherry-pick")
	}
	if _, err := os.Stat(filepath.Join(worktreePath, "source2.txt")); os.IsNotExist(err) {
		t.Error("source2.txt should exist after cherry-pick")
	}
}

func TestManager_CherryPickBranch_Empty(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a source branch without extra commits
	testutil.CreateBranch(t, repoDir, "source-branch")

	// Create a target worktree
	worktreePath := filepath.Join(t.TempDir(), "target-worktree")
	if err := mgr.Create(worktreePath, "target-branch"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Cherry-pick from source branch (should be no-op)
	if err := mgr.CherryPickBranch(worktreePath, "source-branch"); err != nil {
		t.Fatalf("CherryPickBranch() error = %v for empty branch", err)
	}
}

func TestManager_CherryPickBranch_Conflict(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a source branch with a commit that modifies a file
	testutil.CreateBranch(t, repoDir, "source-branch")
	testutil.CheckoutBranch(t, repoDir, "source-branch")
	testutil.CommitFile(t, repoDir, "conflict.txt", "source content", "Add conflict file from source")

	// Create a target worktree with a conflicting change
	testutil.CheckoutBranch(t, repoDir, "main")
	worktreePath := filepath.Join(t.TempDir(), "target-worktree")
	if err := mgr.Create(worktreePath, "target-branch"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add conflicting change in target
	conflictFile := filepath.Join(worktreePath, "conflict.txt")
	if err := os.WriteFile(conflictFile, []byte("target content"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}
	if err := mgr.CommitAll(worktreePath, "Add conflict file from target"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Cherry-pick should fail with conflict
	err = mgr.CherryPickBranch(worktreePath, "source-branch")
	if err == nil {
		t.Fatal("CherryPickBranch() should fail with conflict")
	}

	// Should be a CherryPickConflictError
	var conflictErr *CherryPickConflictError
	if _, ok := err.(*CherryPickConflictError); !ok {
		// Check if it's any error containing conflict info
		if !strings.Contains(err.Error(), "conflict") && !strings.Contains(err.Error(), "CONFLICT") {
			t.Errorf("CherryPickBranch() error should be CherryPickConflictError, got %T: %v", err, err)
		}
	} else {
		conflictErr = err.(*CherryPickConflictError)
		if conflictErr.SourceBranch != "source-branch" {
			t.Errorf("CherryPickConflictError.SourceBranch = %v, want source-branch", conflictErr.SourceBranch)
		}
	}

	// Abort the cherry-pick
	_ = mgr.AbortCherryPick(worktreePath)
}

func TestManager_CheckCherryPickConflicts(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a source branch with a commit
	testutil.CreateBranch(t, repoDir, "source-branch")
	testutil.CheckoutBranch(t, repoDir, "source-branch")
	testutil.CommitFile(t, repoDir, "file.txt", "source content", "Add file from source")

	// Create a target worktree without conflicts
	testutil.CheckoutBranch(t, repoDir, "main")
	worktreePath := filepath.Join(t.TempDir(), "target-worktree")
	if err := mgr.Create(worktreePath, "target-branch"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Check for conflicts (should be none)
	conflicts, err := mgr.CheckCherryPickConflicts(worktreePath, "source-branch")
	if err != nil {
		t.Fatalf("CheckCherryPickConflicts() error = %v", err)
	}

	if len(conflicts) != 0 {
		t.Errorf("CheckCherryPickConflicts() returned %d conflicts, want 0", len(conflicts))
	}
}

func TestManager_CheckCherryPickConflicts_WithConflicts(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a source branch with a commit that modifies a file
	testutil.CreateBranch(t, repoDir, "source-branch")
	testutil.CheckoutBranch(t, repoDir, "source-branch")
	testutil.CommitFile(t, repoDir, "conflict.txt", "source version", "Add from source")

	// Create a target worktree with a conflicting change
	testutil.CheckoutBranch(t, repoDir, "main")
	worktreePath := filepath.Join(t.TempDir(), "target-worktree")
	if err := mgr.Create(worktreePath, "target-branch"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add conflicting change in target
	conflictFile := filepath.Join(worktreePath, "conflict.txt")
	if err := os.WriteFile(conflictFile, []byte("target version"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}
	if err := mgr.CommitAll(worktreePath, "Add from target"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Check for conflicts
	conflicts, err := mgr.CheckCherryPickConflicts(worktreePath, "source-branch")
	if err != nil {
		t.Fatalf("CheckCherryPickConflicts() error = %v", err)
	}

	// Should report conflict in conflict.txt
	if len(conflicts) == 0 {
		t.Error("CheckCherryPickConflicts() should report conflicts")
	}
}

func TestManager_AbortCherryPick(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// AbortCherryPick on a repo without cherry-pick in progress should error
	err = mgr.AbortCherryPick(repoDir)
	if err == nil {
		t.Error("AbortCherryPick() should error when no cherry-pick in progress")
	}
}

func TestManager_ContinueCherryPick(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// ContinueCherryPick on a repo without cherry-pick in progress should error
	err = mgr.ContinueCherryPick(repoDir)
	if err == nil {
		t.Error("ContinueCherryPick() should error when no cherry-pick in progress")
	}
}

func TestManager_SetLogger_Integration(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Set a logger and verify operations still work
	logger, err := logging.NewLogger(t.TempDir(), "DEBUG")
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	mgr.SetLogger(logger)

	// Create a worktree (this should log)
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "logged-branch"); err != nil {
		t.Fatalf("Create() with logger error = %v", err)
	}

	// Remove the worktree (this should also log)
	if err := mgr.Remove(worktreePath); err != nil {
		t.Fatalf("Remove() with logger error = %v", err)
	}

	// Delete the branch (this should log)
	if err := mgr.DeleteBranch("logged-branch"); err != nil {
		t.Fatalf("DeleteBranch() with logger error = %v", err)
	}
}

func TestManager_GetBehindCount(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir, remoteDir := testutil.SetupTestRepoWithRemote(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a worktree
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "feature"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Initially should not be behind
	behindCount, err := mgr.GetBehindCount(worktreePath)
	if err != nil {
		t.Fatalf("GetBehindCount() error = %v", err)
	}
	if behindCount != 0 {
		t.Errorf("GetBehindCount() = %d, want 0", behindCount)
	}

	// Add commits to main on the remote
	// First we need to clone the bare repo, add commits, and push
	cloneDir := t.TempDir()
	runGit(t, cloneDir, "clone", remoteDir, ".")
	runGit(t, cloneDir, "config", "user.email", "test@claudio.dev")
	runGit(t, cloneDir, "config", "user.name", "Claudio Test")

	// Add commits to main
	for i := 1; i <= 3; i++ {
		testFile := filepath.Join(cloneDir, "remote_file.txt")
		if err := os.WriteFile(testFile, []byte("content "+string(rune('0'+i))), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		runGit(t, cloneDir, "add", "-A")
		runGit(t, cloneDir, "commit", "-m", "Remote commit "+string(rune('0'+i)))
	}
	runGit(t, cloneDir, "push", "origin", "main")

	// Now check behind count
	behindCount, err = mgr.GetBehindCount(worktreePath)
	if err != nil {
		t.Fatalf("GetBehindCount() error = %v", err)
	}
	if behindCount != 3 {
		t.Errorf("GetBehindCount() = %d, want 3", behindCount)
	}
}

func TestManager_RebaseOnMain(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir, _ := testutil.SetupTestRepoWithRemote(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a worktree with commits
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "feature"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add a commit to the feature branch
	testFile := filepath.Join(worktreePath, "feature.txt")
	if err := os.WriteFile(testFile, []byte("feature content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := mgr.CommitAll(worktreePath, "Add feature"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Rebase on main (should succeed since we're up to date)
	if err := mgr.RebaseOnMain(worktreePath); err != nil {
		t.Fatalf("RebaseOnMain() error = %v", err)
	}

	// Verify feature file still exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("feature.txt should still exist after rebase")
	}
}

func TestManager_RebaseOnMain_WithConflict(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir, remoteDir := testutil.SetupTestRepoWithRemote(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a worktree with a commit that will conflict
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "feature"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add a file in the worktree
	testFile := filepath.Join(worktreePath, "conflict.txt")
	if err := os.WriteFile(testFile, []byte("feature version"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := mgr.CommitAll(worktreePath, "Add conflict file from feature"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Add conflicting commit to main on the remote
	cloneDir := t.TempDir()
	runGit(t, cloneDir, "clone", remoteDir, ".")
	runGit(t, cloneDir, "config", "user.email", "test@claudio.dev")
	runGit(t, cloneDir, "config", "user.name", "Claudio Test")

	conflictFile := filepath.Join(cloneDir, "conflict.txt")
	if err := os.WriteFile(conflictFile, []byte("main version"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}
	runGit(t, cloneDir, "add", "-A")
	runGit(t, cloneDir, "commit", "-m", "Add conflict file from main")
	runGit(t, cloneDir, "push", "origin", "main")

	// Rebase on main should fail with conflict
	err = mgr.RebaseOnMain(worktreePath)
	if err == nil {
		t.Fatal("RebaseOnMain() should fail with conflict")
	}

	// Error should mention conflict
	if !strings.Contains(err.Error(), "conflict") && !strings.Contains(err.Error(), "CONFLICT") {
		t.Errorf("RebaseOnMain() error should mention conflict, got: %v", err)
	}
}

func TestManager_HasRebaseConflicts(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir, _ := testutil.SetupTestRepoWithRemote(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a worktree
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "feature"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Initially no conflicts (branch is up to date with main)
	hasConflicts, err := mgr.HasRebaseConflicts(worktreePath)
	if err != nil {
		t.Fatalf("HasRebaseConflicts() error = %v", err)
	}
	if hasConflicts {
		t.Error("HasRebaseConflicts() = true, want false when up to date")
	}
}

func TestManager_HasRebaseConflicts_WithConflicts(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir, remoteDir := testutil.SetupTestRepoWithRemote(t)
	mgr, err := New(repoDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a worktree with a file that will conflict
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	if err := mgr.Create(worktreePath, "feature"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add a file in the worktree
	testFile := filepath.Join(worktreePath, "conflict.txt")
	if err := os.WriteFile(testFile, []byte("feature version"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := mgr.CommitAll(worktreePath, "Add from feature"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Add conflicting commit to main on the remote
	cloneDir := t.TempDir()
	runGit(t, cloneDir, "clone", remoteDir, ".")
	runGit(t, cloneDir, "config", "user.email", "test@claudio.dev")
	runGit(t, cloneDir, "config", "user.name", "Claudio Test")

	conflictFile := filepath.Join(cloneDir, "conflict.txt")
	if err := os.WriteFile(conflictFile, []byte("main version"), 0644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}
	runGit(t, cloneDir, "add", "-A")
	runGit(t, cloneDir, "commit", "-m", "Add from main")
	runGit(t, cloneDir, "push", "origin", "main")

	// Check for conflicts
	hasConflicts, err := mgr.HasRebaseConflicts(worktreePath)
	if err != nil {
		t.Fatalf("HasRebaseConflicts() error = %v", err)
	}
	if !hasConflicts {
		t.Error("HasRebaseConflicts() = false, want true when conflicts exist")
	}
}

// runGit is a helper to run git commands in tests
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Claudio Test",
		"GIT_AUTHOR_EMAIL=test@claudio.dev",
		"GIT_COMMITTER_NAME=Claudio Test",
		"GIT_COMMITTER_EMAIL=test@claudio.dev",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
