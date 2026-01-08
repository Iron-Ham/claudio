package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager handles git worktree operations
type Manager struct {
	repoDir string
}

// New creates a new worktree Manager
func New(repoDir string) (*Manager, error) {
	// Verify it's a git repository
	gitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a git repository: %s", repoDir)
	}

	return &Manager{repoDir: repoDir}, nil
}

// Create creates a new worktree at the given path with a new branch
func (m *Manager) Create(path, branch string) error {
	// First, create the branch from current HEAD
	// Use git worktree add -b to create branch and worktree in one step
	cmd := exec.Command("git", "worktree", "add", "-b", branch, path)
	cmd.Dir = m.repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree: %w\n%s", err, string(output))
	}

	return nil
}

// Remove removes a worktree
func (m *Manager) Remove(path string) error {
	// First, try to remove the worktree
	cmd := exec.Command("git", "worktree", "remove", "--force", path)
	cmd.Dir = m.repoDir

	if output, err := cmd.CombinedOutput(); err != nil {
		// If worktree remove fails, try to clean up manually
		os.RemoveAll(path)

		// Prune worktree references
		pruneCmd := exec.Command("git", "worktree", "prune")
		pruneCmd.Dir = m.repoDir
		pruneCmd.Run()

		return fmt.Errorf("failed to remove worktree cleanly: %w\n%s", err, string(output))
	}

	return nil
}

// List returns all worktrees
func (m *Manager) List() ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			worktrees = append(worktrees, path)
		}
	}

	return worktrees, nil
}

// GetBranch returns the branch for a worktree
func (m *Manager) GetBranch(path string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// DeleteBranch deletes a branch
func (m *Manager) DeleteBranch(branch string) error {
	cmd := exec.Command("git", "branch", "-D", branch)
	cmd.Dir = m.repoDir

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete branch: %w\n%s", err, string(output))
	}

	return nil
}

// HasUncommittedChanges checks if a worktree has uncommitted changes
func (m *Manager) HasUncommittedChanges(path string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check status: %w", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

// CommitAll commits all changes in a worktree
func (m *Manager) CommitAll(path, message string) error {
	// Add all changes
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = path
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add changes: %w\n%s", err, string(output))
	}

	// Commit
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = path
	if output, err := commitCmd.CombinedOutput(); err != nil {
		// Check if there's nothing to commit
		if strings.Contains(string(output), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("failed to commit: %w\n%s", err, string(output))
	}

	return nil
}
