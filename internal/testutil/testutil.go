// Package testutil provides testing utilities for Claudio tests.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// SetupTestRepo creates a temporary git repository for testing.
// Returns the path to the repository. The repository is automatically
// cleaned up when the test completes.
func SetupTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Initialize git repository
	if err := runGit(dir, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	if err := runGit(dir, "config", "user.email", "test@claudio.dev"); err != nil {
		t.Fatalf("failed to configure git email: %v", err)
	}
	if err := runGit(dir, "config", "user.name", "Claudio Test"); err != nil {
		t.Fatalf("failed to configure git name: %v", err)
	}

	// Create initial commit (git worktree requires at least one commit)
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test Repository\n"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}
	if err := runGit(dir, "add", "."); err != nil {
		t.Fatalf("failed to stage files: %v", err)
	}
	if err := runGit(dir, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	// Create main branch (some systems default to master)
	if err := runGit(dir, "branch", "-M", "main"); err != nil {
		t.Fatalf("failed to rename branch to main: %v", err)
	}

	return dir
}

// CommitFile creates or updates a file and commits it.
func CommitFile(t *testing.T, repoDir, path, content, message string) {
	t.Helper()

	fullPath := filepath.Join(repoDir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("failed to create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
	if err := runGit(repoDir, "add", path); err != nil {
		t.Fatalf("failed to stage file %s: %v", path, err)
	}
	if err := runGit(repoDir, "commit", "-m", message); err != nil {
		t.Fatalf("failed to commit file %s: %v", path, err)
	}
}

// CreateBranch creates a new branch in the repository.
func CreateBranch(t *testing.T, repoDir, branch string) {
	t.Helper()

	if err := runGit(repoDir, "branch", branch); err != nil {
		t.Fatalf("failed to create branch %s: %v", branch, err)
	}
}

// SkipIfNoGit skips the test if git is not installed.
func SkipIfNoGit(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}
}

// runGit runs a git command in the specified directory.
func runGit(dir string, args ...string) error {
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
		return &gitError{args: args, output: output, err: err}
	}
	return nil
}

type gitError struct {
	args   []string
	output []byte
	err    error
}

func (e *gitError) Error() string {
	return "git " + joinStrings(e.args, " ") + ": " + e.err.Error() + "\n" + string(e.output)
}

func (e *gitError) Unwrap() error {
	return e.err
}

// joinStrings joins strings with a separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// SetupTestRepoWithSubmodule creates a test repository that contains a git submodule.
// Returns paths to both the main repo and the submodule repo.
// The submodule is added at the path "vendor/submod" within the main repo.
func SetupTestRepoWithSubmodule(t *testing.T) (mainRepoDir, submoduleRepoDir string) {
	t.Helper()

	// Create the submodule repository first (as a "remote")
	submoduleRepoDir = t.TempDir()
	if err := runGit(submoduleRepoDir, "init"); err != nil {
		t.Fatalf("failed to init submodule repo: %v", err)
	}
	if err := runGit(submoduleRepoDir, "config", "user.email", "test@claudio.dev"); err != nil {
		t.Fatalf("failed to configure git email: %v", err)
	}
	if err := runGit(submoduleRepoDir, "config", "user.name", "Claudio Test"); err != nil {
		t.Fatalf("failed to configure git name: %v", err)
	}

	// Create a file in the submodule
	subFile := filepath.Join(submoduleRepoDir, "submodule-file.txt")
	if err := os.WriteFile(subFile, []byte("submodule content\n"), 0644); err != nil {
		t.Fatalf("failed to create submodule file: %v", err)
	}
	if err := runGit(submoduleRepoDir, "add", "."); err != nil {
		t.Fatalf("failed to stage submodule files: %v", err)
	}
	if err := runGit(submoduleRepoDir, "commit", "-m", "Initial submodule commit"); err != nil {
		t.Fatalf("failed to create submodule commit: %v", err)
	}
	if err := runGit(submoduleRepoDir, "branch", "-M", "main"); err != nil {
		t.Fatalf("failed to rename submodule branch to main: %v", err)
	}

	// Create the main repository
	mainRepoDir = SetupTestRepo(t)

	// Enable file protocol for submodule operations (required for git 2.38+)
	// This is safe for tests since we control both repositories
	if err := runGit(mainRepoDir, "-c", "protocol.file.allow=always", "submodule", "add", submoduleRepoDir, "vendor/submod"); err != nil {
		t.Fatalf("failed to add submodule: %v", err)
	}
	if err := runGit(mainRepoDir, "commit", "-m", "Add submodule"); err != nil {
		t.Fatalf("failed to commit submodule addition: %v", err)
	}

	return mainRepoDir, submoduleRepoDir
}
