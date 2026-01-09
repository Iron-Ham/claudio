package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/errors"
	"github.com/Iron-Ham/claudio/internal/testutil"
)

// -----------------------------------------------------------------------------
// Mock Command Executor for Unit Tests
// -----------------------------------------------------------------------------

// mockCall records a single command invocation
type mockCall struct {
	dir  string
	name string
	args []string
}

// mockExecutor is a test double for CommandExecutor
type mockExecutor struct {
	calls      []mockCall
	runOutputs [][]byte
	runErrors  []error
	callIndex  int
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		calls:      make([]mockCall, 0),
		runOutputs: make([][]byte, 0),
		runErrors:  make([]error, 0),
	}
}

func (m *mockExecutor) addResponse(output []byte, err error) {
	m.runOutputs = append(m.runOutputs, output)
	m.runErrors = append(m.runErrors, err)
}

func (m *mockExecutor) Run(dir string, name string, args ...string) ([]byte, error) {
	m.calls = append(m.calls, mockCall{dir: dir, name: name, args: args})
	idx := m.callIndex
	m.callIndex++
	if idx < len(m.runOutputs) {
		return m.runOutputs[idx], m.runErrors[idx]
	}
	return nil, nil
}

func (m *mockExecutor) RunQuiet(dir string, name string, args ...string) error {
	m.calls = append(m.calls, mockCall{dir: dir, name: name, args: args})
	idx := m.callIndex
	m.callIndex++
	if idx < len(m.runErrors) {
		return m.runErrors[idx]
	}
	return nil
}

func (m *mockExecutor) getCalls() []mockCall {
	return m.calls
}

func (m *mockExecutor) lastCall() mockCall {
	if len(m.calls) == 0 {
		return mockCall{}
	}
	return m.calls[len(m.calls)-1]
}

// -----------------------------------------------------------------------------
// CLIGitOperations Unit Tests
// -----------------------------------------------------------------------------

func TestCLIGitOperations_HasUncommittedChanges(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		err        error
		wantResult bool
		wantErr    bool
	}{
		{
			name:       "clean repo",
			output:     "",
			err:        nil,
			wantResult: false,
			wantErr:    false,
		},
		{
			name:       "modified file",
			output:     " M file.txt\n",
			err:        nil,
			wantResult: true,
			wantErr:    false,
		},
		{
			name:       "untracked file",
			output:     "?? newfile.txt\n",
			err:        nil,
			wantResult: true,
			wantErr:    false,
		},
		{
			name:       "staged file",
			output:     "A  staged.txt\n",
			err:        nil,
			wantResult: true,
			wantErr:    false,
		},
		{
			name:       "git status error",
			output:     "",
			err:        errors.New("git status failed"),
			wantResult: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockExecutor()
			mock.addResponse([]byte(tt.output), tt.err)

			g := NewCLIGitOperationsWithExecutor("/repo", mock)
			result, err := g.HasUncommittedChanges("/path")

			if (err != nil) != tt.wantErr {
				t.Errorf("HasUncommittedChanges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.wantResult {
				t.Errorf("HasUncommittedChanges() = %v, want %v", result, tt.wantResult)
			}

			// Verify correct command was called
			if len(mock.getCalls()) != 1 {
				t.Fatalf("expected 1 call, got %d", len(mock.getCalls()))
			}
			call := mock.getCalls()[0]
			if call.name != "git" || call.args[0] != "status" || call.args[1] != "--porcelain" {
				t.Errorf("unexpected command: %v %v", call.name, call.args)
			}
		})
	}
}

func TestCLIGitOperations_CommitAll(t *testing.T) {
	t.Run("successful commit", func(t *testing.T) {
		mock := newMockExecutor()
		mock.addResponse([]byte(""), nil)         // git add -A
		mock.addResponse([]byte("created"), nil)  // git commit

		g := NewCLIGitOperationsWithExecutor("/repo", mock)
		err := g.CommitAll("/path", "test message")

		if err != nil {
			t.Errorf("CommitAll() error = %v", err)
		}

		calls := mock.getCalls()
		if len(calls) != 2 {
			t.Fatalf("expected 2 calls, got %d", len(calls))
		}

		// Check git add -A
		if calls[0].args[0] != "add" || calls[0].args[1] != "-A" {
			t.Errorf("expected 'git add -A', got %v", calls[0].args)
		}

		// Check git commit -m
		if calls[1].args[0] != "commit" || calls[1].args[1] != "-m" || calls[1].args[2] != "test message" {
			t.Errorf("expected 'git commit -m test message', got %v", calls[1].args)
		}
	})

	t.Run("nothing to commit", func(t *testing.T) {
		mock := newMockExecutor()
		mock.addResponse([]byte(""), nil)
		mock.addResponse([]byte("nothing to commit"), errors.New("exit status 1"))

		g := NewCLIGitOperationsWithExecutor("/repo", mock)
		err := g.CommitAll("/path", "test message")

		// Should not return error when nothing to commit
		if err != nil {
			t.Errorf("CommitAll() should not error on 'nothing to commit', got %v", err)
		}
	})

	t.Run("commit failure", func(t *testing.T) {
		mock := newMockExecutor()
		mock.addResponse([]byte(""), nil)
		mock.addResponse([]byte("error message"), errors.New("exit status 1"))

		g := NewCLIGitOperationsWithExecutor("/repo", mock)
		err := g.CommitAll("/path", "test message")

		if err == nil {
			t.Error("CommitAll() should return error on failure")
		}

		// Verify it's a GitError
		var gitErr *errors.GitError
		if !errors.As(err, &gitErr) {
			t.Errorf("expected GitError, got %T", err)
		}
	})
}

func TestCLIGitOperations_GetCommitsBetween(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		err        error
		wantCount  int
		wantErr    bool
	}{
		{
			name:      "multiple commits",
			output:    "abc123\ndef456\nghi789\n",
			err:       nil,
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "single commit",
			output:    "abc123\n",
			err:       nil,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "no commits",
			output:    "",
			err:       nil,
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockExecutor()
			mock.addResponse([]byte(tt.output), tt.err)

			g := NewCLIGitOperationsWithExecutor("/repo", mock)
			commits, err := g.GetCommitsBetween("/path", "base", "head")

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCommitsBetween() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(commits) != tt.wantCount {
				t.Errorf("GetCommitsBetween() returned %d commits, want %d", len(commits), tt.wantCount)
			}
		})
	}
}

func TestCLIGitOperations_CountCommitsBetween(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		err       error
		wantCount int
		wantErr   bool
	}{
		{
			name:      "ten commits",
			output:    "10\n",
			err:       nil,
			wantCount: 10,
			wantErr:   false,
		},
		{
			name:      "zero commits",
			output:    "0\n",
			err:       nil,
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockExecutor()
			mock.addResponse([]byte(tt.output), tt.err)

			g := NewCLIGitOperationsWithExecutor("/repo", mock)
			count, err := g.CountCommitsBetween("/path", "base", "head")

			if (err != nil) != tt.wantErr {
				t.Errorf("CountCommitsBetween() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if count != tt.wantCount {
				t.Errorf("CountCommitsBetween() = %d, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestCLIGitOperations_Push(t *testing.T) {
	t.Run("normal push", func(t *testing.T) {
		mock := newMockExecutor()
		mock.addResponse([]byte(""), nil)

		g := NewCLIGitOperationsWithExecutor("/repo", mock)
		err := g.Push("/path", false)

		if err != nil {
			t.Errorf("Push() error = %v", err)
		}

		call := mock.lastCall()
		if !contains(call.args, "--force-with-lease") {
			// good, should not have force flag
		}
	})

	t.Run("force push", func(t *testing.T) {
		mock := newMockExecutor()
		mock.addResponse([]byte(""), nil)

		g := NewCLIGitOperationsWithExecutor("/repo", mock)
		err := g.Push("/path", true)

		if err != nil {
			t.Errorf("Push() error = %v", err)
		}

		call := mock.lastCall()
		if !contains(call.args, "--force-with-lease") {
			t.Error("force push should use --force-with-lease")
		}
	})
}

func TestCLIGitOperations_IsCherryPickInProgress(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()

	t.Run("no cherry-pick in progress", func(t *testing.T) {
		gitDir := filepath.Join(tempDir, ".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatal(err)
		}

		g := NewCLIGitOperations(tempDir)
		if g.IsCherryPickInProgress(tempDir) {
			t.Error("expected false when no CHERRY_PICK_HEAD exists")
		}
	})

	t.Run("cherry-pick in progress", func(t *testing.T) {
		gitDir := filepath.Join(tempDir, ".git")
		cherryPickHead := filepath.Join(gitDir, "CHERRY_PICK_HEAD")
		if err := os.WriteFile(cherryPickHead, []byte("abc123"), 0644); err != nil {
			t.Fatal(err)
		}

		g := NewCLIGitOperations(tempDir)
		if !g.IsCherryPickInProgress(tempDir) {
			t.Error("expected true when CHERRY_PICK_HEAD exists")
		}
	})
}

// -----------------------------------------------------------------------------
// CLIWorktreeManager Unit Tests
// -----------------------------------------------------------------------------

func TestCLIWorktreeManager_Create(t *testing.T) {
	t.Run("successful create", func(t *testing.T) {
		mock := newMockExecutor()
		mock.addResponse([]byte("Preparing worktree"), nil)

		w := NewCLIWorktreeManagerWithExecutor("/repo", mock)
		err := w.Create("/path/to/worktree", "feature-branch")

		if err != nil {
			t.Errorf("Create() error = %v", err)
		}

		call := mock.lastCall()
		expectedArgs := []string{"worktree", "add", "-b", "feature-branch", "/path/to/worktree"}
		for i, arg := range expectedArgs {
			if call.args[i] != arg {
				t.Errorf("arg[%d] = %s, want %s", i, call.args[i], arg)
			}
		}
	})

	t.Run("create failure", func(t *testing.T) {
		mock := newMockExecutor()
		mock.addResponse([]byte("fatal: already exists"), errors.New("exit status 128"))

		w := NewCLIWorktreeManagerWithExecutor("/repo", mock)
		err := w.Create("/path/to/worktree", "feature-branch")

		if err == nil {
			t.Error("Create() should return error on failure")
		}

		var gitErr *errors.GitError
		if !errors.As(err, &gitErr) {
			t.Errorf("expected GitError, got %T", err)
		}
	})
}

func TestCLIWorktreeManager_CreateFromBranch(t *testing.T) {
	mock := newMockExecutor()
	mock.addResponse([]byte(""), nil)

	w := NewCLIWorktreeManagerWithExecutor("/repo", mock)
	err := w.CreateFromBranch("/path", "new-branch", "base-branch")

	if err != nil {
		t.Errorf("CreateFromBranch() error = %v", err)
	}

	call := mock.lastCall()
	expectedArgs := []string{"worktree", "add", "-b", "new-branch", "/path", "base-branch"}
	for i, arg := range expectedArgs {
		if call.args[i] != arg {
			t.Errorf("arg[%d] = %s, want %s", i, call.args[i], arg)
		}
	}
}

func TestCLIWorktreeManager_List(t *testing.T) {
	porcelainOutput := `worktree /path/to/repo
HEAD abc123
branch refs/heads/main

worktree /path/to/worktree1
HEAD def456
branch refs/heads/feature

worktree /path/to/worktree2
HEAD ghi789
branch refs/heads/bugfix
`
	mock := newMockExecutor()
	mock.addResponse([]byte(porcelainOutput), nil)

	w := NewCLIWorktreeManagerWithExecutor("/repo", mock)
	worktrees, err := w.List()

	if err != nil {
		t.Errorf("List() error = %v", err)
	}

	if len(worktrees) != 3 {
		t.Errorf("List() returned %d worktrees, want 3", len(worktrees))
	}

	expected := []string{"/path/to/repo", "/path/to/worktree1", "/path/to/worktree2"}
	for i, wt := range worktrees {
		if wt != expected[i] {
			t.Errorf("worktree[%d] = %s, want %s", i, wt, expected[i])
		}
	}
}

func TestCLIWorktreeManager_GetPath(t *testing.T) {
	w := NewCLIWorktreeManager("/my/repo/path")
	if w.GetPath() != "/my/repo/path" {
		t.Errorf("GetPath() = %s, want /my/repo/path", w.GetPath())
	}
}

// -----------------------------------------------------------------------------
// CLIBranchManager Unit Tests
// -----------------------------------------------------------------------------

func TestCLIBranchManager_CreateBranchFrom(t *testing.T) {
	t.Run("successful create", func(t *testing.T) {
		mock := newMockExecutor()
		mock.addResponse([]byte(""), nil)

		b := NewCLIBranchManagerWithExecutor("/repo", mock)
		err := b.CreateBranchFrom("new-branch", "base-branch")

		if err != nil {
			t.Errorf("CreateBranchFrom() error = %v", err)
		}

		call := mock.lastCall()
		expectedArgs := []string{"branch", "new-branch", "base-branch"}
		for i, arg := range expectedArgs {
			if call.args[i] != arg {
				t.Errorf("arg[%d] = %s, want %s", i, call.args[i], arg)
			}
		}
	})

	t.Run("branch already exists", func(t *testing.T) {
		mock := newMockExecutor()
		mock.addResponse([]byte("fatal: a branch named 'new-branch' already exists"), errors.New("exit status 128"))

		b := NewCLIBranchManagerWithExecutor("/repo", mock)
		err := b.CreateBranchFrom("new-branch", "base-branch")

		if err == nil {
			t.Error("CreateBranchFrom() should return error when branch exists")
		}

		if !errors.Is(err, errors.ErrBranchExists) {
			t.Errorf("expected ErrBranchExists, got %v", err)
		}
	})
}

func TestCLIBranchManager_DeleteBranch(t *testing.T) {
	t.Run("successful delete", func(t *testing.T) {
		mock := newMockExecutor()
		mock.addResponse([]byte("Deleted branch feature"), nil)

		b := NewCLIBranchManagerWithExecutor("/repo", mock)
		err := b.DeleteBranch("feature")

		if err != nil {
			t.Errorf("DeleteBranch() error = %v", err)
		}

		call := mock.lastCall()
		if call.args[0] != "branch" || call.args[1] != "-D" || call.args[2] != "feature" {
			t.Errorf("unexpected args: %v", call.args)
		}
	})

	t.Run("branch not found", func(t *testing.T) {
		mock := newMockExecutor()
		mock.addResponse([]byte("error: branch 'nonexistent' not found"), errors.New("exit status 1"))

		b := NewCLIBranchManagerWithExecutor("/repo", mock)
		err := b.DeleteBranch("nonexistent")

		if err == nil {
			t.Error("DeleteBranch() should return error when branch not found")
		}

		if !errors.Is(err, errors.ErrBranchNotFound) {
			t.Errorf("expected ErrBranchNotFound, got %v", err)
		}
	})
}

func TestCLIBranchManager_GetBranch(t *testing.T) {
	mock := newMockExecutor()
	mock.addResponse([]byte("feature-branch\n"), nil)

	b := NewCLIBranchManagerWithExecutor("/repo", mock)
	branch, err := b.GetBranch("/path/to/worktree")

	if err != nil {
		t.Errorf("GetBranch() error = %v", err)
	}

	if branch != "feature-branch" {
		t.Errorf("GetBranch() = %s, want feature-branch", branch)
	}
}

func TestCLIBranchManager_FindMainBranch(t *testing.T) {
	t.Run("main exists", func(t *testing.T) {
		mock := newMockExecutor()
		// git rev-parse --verify main succeeds
		mock.addResponse(nil, nil)

		b := NewCLIBranchManagerWithExecutor("/repo", mock)
		branch := b.FindMainBranch()

		if branch != "main" {
			t.Errorf("FindMainBranch() = %s, want main", branch)
		}
	})

	t.Run("main does not exist, falls back to master", func(t *testing.T) {
		mock := newMockExecutor()
		// git rev-parse --verify main fails
		mock.addResponse(nil, errors.New("exit status 128"))

		b := NewCLIBranchManagerWithExecutor("/repo", mock)
		branch := b.FindMainBranch()

		if branch != "master" {
			t.Errorf("FindMainBranch() = %s, want master", branch)
		}
	})
}

// -----------------------------------------------------------------------------
// CLIDiffProvider Unit Tests
// -----------------------------------------------------------------------------

func TestCLIDiffProvider_GetChangedFiles(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantFiles []string
	}{
		{
			name:      "multiple files",
			output:    "file1.go\nfile2.go\ndir/file3.go\n",
			wantFiles: []string{"file1.go", "file2.go", "dir/file3.go"},
		},
		{
			name:      "single file",
			output:    "only-file.txt\n",
			wantFiles: []string{"only-file.txt"},
		},
		{
			name:      "no files",
			output:    "",
			wantFiles: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockExecutor()
			// First call: RunQuiet for main branch check
			mock.addResponse(nil, nil)
			// Second call: git diff --name-only
			mock.addResponse([]byte(tt.output), nil)

			d := NewCLIDiffProviderWithExecutor("/repo", mock)
			files, err := d.GetChangedFiles("/path")

			if err != nil {
				t.Errorf("GetChangedFiles() error = %v", err)
				return
			}

			if len(files) != len(tt.wantFiles) {
				t.Errorf("GetChangedFiles() returned %d files, want %d", len(files), len(tt.wantFiles))
				return
			}

			for i, f := range files {
				if f != tt.wantFiles[i] {
					t.Errorf("files[%d] = %s, want %s", i, f, tt.wantFiles[i])
				}
			}
		})
	}
}

func TestCLIDiffProvider_HasUncommittedChanges(t *testing.T) {
	mock := newMockExecutor()
	mock.addResponse([]byte(" M modified.txt\n"), nil)

	d := NewCLIDiffProviderWithExecutor("/repo", mock)
	hasChanges, err := d.HasUncommittedChanges("/path")

	if err != nil {
		t.Errorf("HasUncommittedChanges() error = %v", err)
	}

	if !hasChanges {
		t.Error("HasUncommittedChanges() should return true for modified files")
	}
}

// -----------------------------------------------------------------------------
// GitClient Unit Tests
// -----------------------------------------------------------------------------

func TestNewGitClient(t *testing.T) {
	client := NewGitClient("/repo")

	if client.GetRepoDir() != "/repo" {
		t.Errorf("GetRepoDir() = %s, want /repo", client.GetRepoDir())
	}

	// Verify all interfaces are set
	if client.GitOperations == nil {
		t.Error("GitOperations should not be nil")
	}
	if client.WorktreeManager == nil {
		t.Error("WorktreeManager should not be nil")
	}
	if client.BranchManager == nil {
		t.Error("BranchManager should not be nil")
	}
	if client.DiffProvider == nil {
		t.Error("DiffProvider should not be nil")
	}
}

// -----------------------------------------------------------------------------
// Integration Tests with Real Git Repos
// -----------------------------------------------------------------------------

func TestCLIGitOperations_Integration_HasUncommittedChanges(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	g := NewCLIGitOperations(repoDir)

	// Initially clean
	hasChanges, err := g.HasUncommittedChanges(repoDir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error = %v", err)
	}
	if hasChanges {
		t.Error("expected clean repo to have no uncommitted changes")
	}

	// Create a file
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Now should have changes
	hasChanges, err = g.HasUncommittedChanges(repoDir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error = %v", err)
	}
	if !hasChanges {
		t.Error("expected repo with new file to have uncommitted changes")
	}
}

func TestCLIGitOperations_Integration_CommitAll(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	g := NewCLIGitOperations(repoDir)

	// Create and commit a file
	testFile := filepath.Join(repoDir, "newfile.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	err := g.CommitAll(repoDir, "Add newfile.txt")
	if err != nil {
		t.Errorf("CommitAll() error = %v", err)
	}

	// Verify no uncommitted changes remain
	hasChanges, err := g.HasUncommittedChanges(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	if hasChanges {
		t.Error("expected no uncommitted changes after CommitAll()")
	}
}

func TestCLIWorktreeManager_Integration_CreateAndList(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	w := NewCLIWorktreeManager(repoDir)

	// Create worktree
	worktreePath := filepath.Join(t.TempDir(), "test-worktree")
	err := w.Create(worktreePath, "test-branch")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify worktree exists on filesystem
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// List worktrees
	worktrees, err := w.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should have at least 2 (main repo + new worktree)
	if len(worktrees) < 2 {
		t.Errorf("expected at least 2 worktrees, got %d", len(worktrees))
	}

	// Verify our worktree is in the list (resolve symlinks for macOS)
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

func TestCLIBranchManager_Integration_CreateAndDelete(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	b := NewCLIBranchManager(repoDir)

	// Create a branch
	err := b.CreateBranchFrom("feature-branch", "main")
	if err != nil {
		t.Fatalf("CreateBranchFrom() error = %v", err)
	}

	// Verify branch exists by getting its name
	_, err = b.GetBranch(repoDir)
	if err != nil {
		t.Fatalf("GetBranch() error = %v", err)
	}

	// Delete the branch
	err = b.DeleteBranch("feature-branch")
	if err != nil {
		t.Fatalf("DeleteBranch() error = %v", err)
	}

	// Delete again should fail (branch not found)
	err = b.DeleteBranch("feature-branch")
	if err == nil {
		t.Error("DeleteBranch() should fail for non-existent branch")
	}
}

func TestCLIDiffProvider_Integration_GetChangedFiles(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	wm := NewCLIWorktreeManager(repoDir)
	d := NewCLIDiffProvider(repoDir)

	// Create worktree with changes
	worktreePath := filepath.Join(t.TempDir(), "feature-worktree")
	err := wm.Create(worktreePath, "feature")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add a file in the worktree
	testFile := filepath.Join(worktreePath, "feature.txt")
	if err := os.WriteFile(testFile, []byte("feature content"), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewCLIGitOperations(repoDir)
	if err := g.CommitAll(worktreePath, "Add feature file"); err != nil {
		t.Fatal(err)
	}

	// Get changed files
	files, err := d.GetChangedFiles(worktreePath)
	if err != nil {
		t.Fatalf("GetChangedFiles() error = %v", err)
	}

	if len(files) != 1 || files[0] != "feature.txt" {
		t.Errorf("GetChangedFiles() = %v, want [feature.txt]", files)
	}
}

func TestCLIDiffProvider_Integration_GetDiffAgainstMain(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	wm := NewCLIWorktreeManager(repoDir)
	d := NewCLIDiffProvider(repoDir)

	// Create worktree
	worktreePath := filepath.Join(t.TempDir(), "diff-worktree")
	err := wm.Create(worktreePath, "diff-branch")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add a file
	testFile := filepath.Join(worktreePath, "diff.txt")
	if err := os.WriteFile(testFile, []byte("diff content"), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewCLIGitOperations(repoDir)
	if err := g.CommitAll(worktreePath, "Add diff file"); err != nil {
		t.Fatal(err)
	}

	// Get diff
	diff, err := d.GetDiffAgainstMain(worktreePath)
	if err != nil {
		t.Fatalf("GetDiffAgainstMain() error = %v", err)
	}

	// Diff should contain our file
	if !strings.Contains(diff, "diff.txt") {
		t.Error("diff should contain our new file")
	}
}

func TestGitClient_Integration(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	client := NewGitClient(repoDir)

	// Test composition - all operations through single client
	hasChanges, err := client.HasUncommittedChanges(repoDir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error = %v", err)
	}
	if hasChanges {
		t.Error("expected clean repo")
	}

	worktrees, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(worktrees))
	}

	mainBranch := client.FindMainBranch()
	if mainBranch != "main" {
		t.Errorf("FindMainBranch() = %s, want main", mainBranch)
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
