//go:build integration

package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Ham/claudio/internal/testutil"
)

func TestManager_HasSubmodules(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name       string
		setupFunc  func(t *testing.T) string
		wantResult bool
	}{
		{
			name: "repo without submodules",
			setupFunc: func(t *testing.T) string {
				return testutil.SetupTestRepo(t)
			},
			wantResult: false,
		},
		{
			name: "repo with submodule",
			setupFunc: func(t *testing.T) string {
				mainRepo, _ := testutil.SetupTestRepoWithSubmodule(t)
				return mainRepo
			},
			wantResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := tt.setupFunc(t)

			mgr, err := New(repoDir)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			got := mgr.HasSubmodules()
			if got != tt.wantResult {
				t.Errorf("HasSubmodules() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestManager_GetSubmodules(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name      string
		setupFunc func(t *testing.T) string
		wantCount int
		wantPath  string
	}{
		{
			name: "repo without submodules",
			setupFunc: func(t *testing.T) string {
				return testutil.SetupTestRepo(t)
			},
			wantCount: 0,
		},
		{
			name: "repo with submodule",
			setupFunc: func(t *testing.T) string {
				mainRepo, _ := testutil.SetupTestRepoWithSubmodule(t)
				return mainRepo
			},
			wantCount: 1,
			wantPath:  "vendor/submod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := tt.setupFunc(t)

			mgr, err := New(repoDir)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			submodules, err := mgr.GetSubmodules()
			if err != nil {
				t.Fatalf("GetSubmodules() error = %v", err)
			}

			if len(submodules) != tt.wantCount {
				t.Errorf("GetSubmodules() returned %d submodules, want %d", len(submodules), tt.wantCount)
			}

			if tt.wantCount > 0 && tt.wantPath != "" {
				if submodules[0].Path != tt.wantPath {
					t.Errorf("GetSubmodules()[0].Path = %q, want %q", submodules[0].Path, tt.wantPath)
				}
			}
		})
	}
}

func TestManager_GetSubmodulePaths(t *testing.T) {
	testutil.SkipIfNoGit(t)

	mainRepo, _ := testutil.SetupTestRepoWithSubmodule(t)

	mgr, err := New(mainRepo)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	paths, err := mgr.GetSubmodulePaths()
	if err != nil {
		t.Fatalf("GetSubmodulePaths() error = %v", err)
	}

	if len(paths) != 1 {
		t.Fatalf("GetSubmodulePaths() returned %d paths, want 1", len(paths))
	}

	if paths[0] != "vendor/submod" {
		t.Errorf("GetSubmodulePaths()[0] = %q, want %q", paths[0], "vendor/submod")
	}
}

func TestManager_IsSubmodulePath(t *testing.T) {
	testutil.SkipIfNoGit(t)

	mainRepo, _ := testutil.SetupTestRepoWithSubmodule(t)

	mgr, err := New(mainRepo)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		path string
		want bool
	}{
		{"vendor/submod", true},
		{"vendor/submod/file.txt", true},
		{"vendor/submod/nested/deep/file.txt", true},
		{"vendor/other", false},
		{"vendor", false},
		{"README.md", false},
		{"src/main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := mgr.IsSubmodulePath(tt.path)
			if got != tt.want {
				t.Errorf("IsSubmodulePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsSubmoduleDir(t *testing.T) {
	testutil.SkipIfNoGit(t)

	mainRepo, _ := testutil.SetupTestRepoWithSubmodule(t)

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "submodule directory",
			path: filepath.Join(mainRepo, "vendor", "submod"),
			want: true,
		},
		{
			name: "normal directory",
			path: mainRepo,
			want: false,
		},
		{
			name: "non-existent directory",
			path: filepath.Join(mainRepo, "nonexistent"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSubmoduleDir(tt.path)
			if got != tt.want {
				t.Errorf("IsSubmoduleDir(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestManager_InitSubmodules(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name        string
		setupFunc   func(t *testing.T) (string, string)
		wantErr     bool
		checkResult func(t *testing.T, worktreePath string)
	}{
		{
			name: "repo without submodules",
			setupFunc: func(t *testing.T) (string, string) {
				repo := testutil.SetupTestRepo(t)
				return repo, repo // No submodules
			},
			wantErr: false,
		},
		{
			name: "repo with submodule",
			setupFunc: func(t *testing.T) (string, string) {
				mainRepo, _ := testutil.SetupTestRepoWithSubmodule(t)
				// Create a worktree to test submodule init
				mgr, _ := New(mainRepo)
				wtPath := filepath.Join(t.TempDir(), "worktree")
				_ = mgr.Create(wtPath, "test-branch")
				return mainRepo, wtPath
			},
			wantErr: false,
			checkResult: func(t *testing.T, worktreePath string) {
				// Verify submodule content exists in worktree
				subFile := filepath.Join(worktreePath, "vendor", "submod", "submodule-file.txt")
				if _, err := os.Stat(subFile); os.IsNotExist(err) {
					t.Errorf("submodule file should exist at %s after initialization", subFile)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mainRepo, worktreePath := tt.setupFunc(t)

			mgr, err := New(mainRepo)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			err = mgr.InitSubmodules(worktreePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitSubmodules() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.checkResult != nil {
				tt.checkResult(t, worktreePath)
			}
		})
	}
}

func TestManager_CreateWorktree_WithSubmodules(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name       string
		createFunc func(mgr *Manager, wtPath string) error
	}{
		{
			name: "Create initializes submodules",
			createFunc: func(mgr *Manager, wtPath string) error {
				return mgr.Create(wtPath, "test-create-branch")
			},
		},
		{
			name: "CreateFromBranch initializes submodules",
			createFunc: func(mgr *Manager, wtPath string) error {
				return mgr.CreateFromBranch(wtPath, "test-from-branch", "main")
			},
		},
		{
			name: "CreateWorktreeFromBranch initializes submodules",
			createFunc: func(mgr *Manager, wtPath string) error {
				// First create a branch without a worktree, then create worktree from it
				if err := mgr.CreateBranchFrom("test-existing-branch", "main"); err != nil {
					return err
				}
				return mgr.CreateWorktreeFromBranch(wtPath, "test-existing-branch")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mainRepo, _ := testutil.SetupTestRepoWithSubmodule(t)

			mgr, err := New(mainRepo)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			wtPath := filepath.Join(t.TempDir(), "worktree")
			if err := tt.createFunc(mgr, wtPath); err != nil {
				t.Fatalf("worktree creation error = %v", err)
			}

			// Verify submodule content exists in the new worktree
			subFile := filepath.Join(wtPath, "vendor", "submod", "submodule-file.txt")
			if _, err := os.Stat(subFile); os.IsNotExist(err) {
				t.Errorf("submodule file should exist at %s after worktree creation", subFile)
			}
		})
	}
}

func TestManager_GetSubmoduleStatus(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name       string
		setupFunc  func(t *testing.T) string
		wantNil    bool
		wantStatus SubmoduleStatus
	}{
		{
			name: "repository without submodules returns nil",
			setupFunc: func(t *testing.T) string {
				return testutil.SetupTestRepo(t)
			},
			wantNil: true,
		},
		{
			name: "initialized submodule shows up-to-date",
			setupFunc: func(t *testing.T) string {
				mainRepo, _ := testutil.SetupTestRepoWithSubmodule(t)
				return mainRepo
			},
			wantNil:    false,
			wantStatus: SubmoduleUpToDate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := tt.setupFunc(t)

			mgr, err := New(repoDir)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			status, err := mgr.GetSubmoduleStatus(repoDir)
			if err != nil {
				t.Fatalf("GetSubmoduleStatus() error = %v", err)
			}

			if tt.wantNil && status != nil {
				t.Errorf("GetSubmoduleStatus() = %v, want nil", status)
			}

			if !tt.wantNil {
				if status == nil || len(status) == 0 {
					t.Fatal("GetSubmoduleStatus() returned nil or empty, want non-empty")
				}
				if status[0].Status != tt.wantStatus {
					t.Errorf("GetSubmoduleStatus()[0].Status = %v, want %v", status[0].Status, tt.wantStatus)
				}
			}
		})
	}
}
