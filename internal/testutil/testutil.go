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

// SetupTestRepoWithContent creates a test repository with specified files.
// The files map contains relative paths to file contents.
func SetupTestRepoWithContent(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := SetupTestRepo(t)

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file %s: %v", path, err)
		}
	}

	// Commit the additional files
	if err := runGit(dir, "add", "."); err != nil {
		t.Fatalf("failed to stage files: %v", err)
	}
	if err := runGit(dir, "commit", "-m", "Add test files"); err != nil {
		t.Fatalf("failed to commit test files: %v", err)
	}

	return dir
}

// SetupTestRepoWithRemote creates a test repository with a fake remote.
// This is useful for testing push/pull operations.
func SetupTestRepoWithRemote(t *testing.T) (repoDir, remoteDir string) {
	t.Helper()

	// Create the "remote" bare repository
	remoteDir = t.TempDir()
	if err := runGit(remoteDir, "init", "--bare"); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	// Create the local repository
	repoDir = SetupTestRepo(t)

	// Add the remote
	if err := runGit(repoDir, "remote", "add", "origin", remoteDir); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	// Push to remote
	if err := runGit(repoDir, "push", "-u", "origin", "main"); err != nil {
		t.Fatalf("failed to push to remote: %v", err)
	}

	return repoDir, remoteDir
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

// CheckoutBranch switches to a branch.
func CheckoutBranch(t *testing.T, repoDir, branch string) {
	t.Helper()

	if err := runGit(repoDir, "checkout", branch); err != nil {
		t.Fatalf("failed to checkout branch %s: %v", branch, err)
	}
}

// GetCurrentBranch returns the current branch name.
func GetCurrentBranch(t *testing.T, repoDir string) string {
	t.Helper()

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	return string(output[:len(output)-1]) // Remove trailing newline
}

// GetCommitCount returns the number of commits in the repository.
func GetCommitCount(t *testing.T, repoDir string) int {
	t.Helper()

	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to count commits: %v", err)
	}

	var count int
	if _, err := parsePositiveInt(string(output[:len(output)-1]), &count); err != nil {
		t.Fatalf("failed to parse commit count: %v", err)
	}
	return count
}

// HasUncommittedChanges returns true if the repository has uncommitted changes.
func HasUncommittedChanges(t *testing.T, repoDir string) bool {
	t.Helper()

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to check git status: %v", err)
	}
	return len(output) > 0
}

// ListWorktrees returns all worktrees in the repository.
func ListWorktrees(t *testing.T, repoDir string) []string {
	t.Helper()

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}

	var worktrees []string
	lines := splitLines(string(output))
	for _, line := range lines {
		if len(line) > 9 && line[:9] == "worktree " {
			worktrees = append(worktrees, line[9:])
		}
	}
	return worktrees
}

// SkipIfNoGit skips the test if git is not installed.
func SkipIfNoGit(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}
}

// SkipIfNoTmux skips the test if tmux is not installed.
func SkipIfNoTmux(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found in PATH, skipping test")
	}
}

// SkipIfNoGolangciLint skips the test if golangci-lint is not installed.
func SkipIfNoGolangciLint(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not found in PATH, skipping test")
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

// parsePositiveInt parses a string to a positive integer.
func parsePositiveInt(s string, result *int) (bool, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, nil
		}
		n = n*10 + int(c-'0')
	}
	*result = n
	return true, nil
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
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
