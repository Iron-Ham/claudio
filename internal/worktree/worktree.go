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

// GetDiffAgainstMain returns the diff of the branch against main/master
func (m *Manager) GetDiffAgainstMain(path string) (string, error) {
	// Try to find the main branch name (could be main or master)
	mainBranch := m.findMainBranch()

	cmd := exec.Command("git", "diff", mainBranch+"...HEAD")
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return string(output), nil
}

// GetCommitLog returns the commit log for the current branch since it diverged from main
func (m *Manager) GetCommitLog(path string) (string, error) {
	mainBranch := m.findMainBranch()

	cmd := exec.Command("git", "log", mainBranch+"..HEAD", "--pretty=format:%s%n%b---")
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit log: %w", err)
	}

	return string(output), nil
}

// GetChangedFiles returns a list of files changed compared to main
func (m *Manager) GetChangedFiles(path string) ([]string, error) {
	mainBranch := m.findMainBranch()

	cmd := exec.Command("git", "diff", "--name-only", mainBranch+"...HEAD")
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 1 && files[0] == "" {
		return []string{}, nil
	}

	return files, nil
}

// Push pushes the current branch to the remote
func (m *Manager) Push(path string, force bool) error {
	args := []string{"push", "-u", "origin", "HEAD"}
	if force {
		args = append(args, "--force-with-lease")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = path

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push: %w\n%s", err, string(output))
	}

	return nil
}

// RebaseOnMain rebases the current branch on main/master
func (m *Manager) RebaseOnMain(path string) error {
	mainBranch := m.findMainBranch()

	// First fetch the latest from origin
	fetchCmd := exec.Command("git", "fetch", "origin", mainBranch)
	fetchCmd.Dir = path
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to fetch origin/%s: %w\n%s", mainBranch, err, string(output))
	}

	// Rebase on origin/main
	rebaseCmd := exec.Command("git", "rebase", "origin/"+mainBranch)
	rebaseCmd.Dir = path
	if output, err := rebaseCmd.CombinedOutput(); err != nil {
		// Check if there are conflicts
		if strings.Contains(string(output), "CONFLICT") || strings.Contains(string(output), "could not apply") {
			// Abort the rebase so we don't leave things in a bad state
			abortCmd := exec.Command("git", "rebase", "--abort")
			abortCmd.Dir = path
			abortCmd.Run()
			return fmt.Errorf("rebase conflicts detected - manual resolution required:\n%s", string(output))
		}
		return fmt.Errorf("failed to rebase: %w\n%s", err, string(output))
	}

	return nil
}

// HasRebaseConflicts checks if rebasing on main would cause conflicts
// Returns true if conflicts would occur, false if rebase would be clean
func (m *Manager) HasRebaseConflicts(path string) (bool, error) {
	mainBranch := m.findMainBranch()

	// Fetch first
	fetchCmd := exec.Command("git", "fetch", "origin", mainBranch)
	fetchCmd.Dir = path
	if err := fetchCmd.Run(); err != nil {
		return false, fmt.Errorf("failed to fetch: %w", err)
	}

	// Check if we're already up to date
	behindCmd := exec.Command("git", "rev-list", "--count", "HEAD..origin/"+mainBranch)
	behindCmd.Dir = path
	behindOutput, err := behindCmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check commits behind: %w", err)
	}

	behindCount := strings.TrimSpace(string(behindOutput))
	if behindCount == "0" {
		// Already up to date, no rebase needed
		return false, nil
	}

	// Check for potential conflicts using merge-tree (dry run)
	// Get the merge base
	mergeBaseCmd := exec.Command("git", "merge-base", "HEAD", "origin/"+mainBranch)
	mergeBaseCmd.Dir = path
	mergeBaseOutput, err := mergeBaseCmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to get merge base: %w", err)
	}
	mergeBase := strings.TrimSpace(string(mergeBaseOutput))

	// Use merge-tree to check for conflicts
	mergeTreeCmd := exec.Command("git", "merge-tree", mergeBase, "HEAD", "origin/"+mainBranch)
	mergeTreeCmd.Dir = path
	mergeTreeOutput, err := mergeTreeCmd.Output()
	if err != nil {
		// merge-tree doesn't return error on conflicts, just outputs them
		return false, fmt.Errorf("failed to run merge-tree: %w", err)
	}

	// If output contains conflict markers, there would be conflicts
	output := string(mergeTreeOutput)
	hasConflicts := strings.Contains(output, "<<<<<<<") || strings.Contains(output, ">>>>>>>")

	return hasConflicts, nil
}

// GetBehindCount returns how many commits the branch is behind origin/main
func (m *Manager) GetBehindCount(path string) (int, error) {
	mainBranch := m.findMainBranch()

	// Fetch first
	fetchCmd := exec.Command("git", "fetch", "origin", mainBranch)
	fetchCmd.Dir = path
	fetchCmd.Run() // Ignore error, might be offline

	behindCmd := exec.Command("git", "rev-list", "--count", "HEAD..origin/"+mainBranch)
	behindCmd.Dir = path
	output, err := behindCmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to check commits behind: %w", err)
	}

	count := 0
	fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, nil
}

// findMainBranch returns the name of the main branch (main or master)
func (m *Manager) findMainBranch() string {
	// Check if 'main' exists
	cmd := exec.Command("git", "rev-parse", "--verify", "main")
	cmd.Dir = m.repoDir
	if err := cmd.Run(); err == nil {
		return "main"
	}

	// Fall back to 'master'
	return "master"
}
