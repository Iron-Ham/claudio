package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// Manager handles git worktree operations
type Manager struct {
	repoDir            string
	logger             *logging.Logger
	sparseCheckoutDirs []string // Directories to include in sparse checkout (nil = disabled)
	coneMode           bool     // Whether to use cone mode for sparse checkout
}

// SetLogger sets the logger for the worktree manager.
// If not set, the manager will operate without logging.
func (m *Manager) SetLogger(logger *logging.Logger) {
	m.logger = logger
}

// SetSparseCheckoutConfig configures sparse checkout for new worktrees.
// When directories is non-empty, sparse checkout will be applied automatically
// when creating worktrees. Pass nil or empty to disable sparse checkout.
// If coneMode is true, git's faster cone mode is used (recommended).
func (m *Manager) SetSparseCheckoutConfig(directories []string, coneMode bool) {
	m.sparseCheckoutDirs = directories
	m.coneMode = coneMode
}

// truncateOutput truncates a string to maxLen characters, adding "..." if truncated.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FindGitRoot finds the root of the git repository by traversing up from startDir.
// It returns the directory containing .git (either a directory or a file for worktrees).
// Returns an error if no git repository is found.
func FindGitRoot(startDir string) (string, error) {
	dir := startDir
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			// .git can be a directory (normal repo) or a file (worktree)
			if info.IsDir() || info.Mode().IsRegular() {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding .git
			return "", fmt.Errorf("not a git repository (or any parent up to mount point)")
		}
		dir = parent
	}
}

// New creates a new worktree Manager
func New(repoDir string) (*Manager, error) {
	// Find the git repository root (may be in a parent directory)
	gitRoot, err := FindGitRoot(repoDir)
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %s", repoDir)
	}

	return &Manager{repoDir: gitRoot}, nil
}

// Create creates a new worktree at the given path with a new branch.
// If the repository has submodules, they are automatically initialized in the new worktree.
func (m *Manager) Create(path, branch string) error {
	// First, create the branch from current HEAD
	// Use git worktree add -b to create branch and worktree in one step
	args := []string{"worktree", "add", "-b", branch, path}
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoDir

	output, err := cmd.CombinedOutput()
	if m.logger != nil {
		m.logger.Debug("git command", "args", args, "output", truncateOutput(string(output), 500))
	}
	if err != nil {
		if m.logger != nil {
			m.logger.Error("git command failed", "args", args, "error", err, "stderr", string(output))
		}
		return fmt.Errorf("failed to create worktree: %w\n%s", err, string(output))
	}

	if m.logger != nil {
		m.logger.Info("worktree created", "path", path, "branch", branch)
	}

	// Initialize submodules if the repository has any
	if err := m.InitSubmodules(path); err != nil {
		// Log but don't fail - submodule init issues are non-fatal
		if m.logger != nil {
			m.logger.Warn("failed to initialize submodules in worktree",
				"path", path,
				"error", err)
		}
	}

	// Apply sparse checkout if configured
	m.applySparseCheckout(path)

	return nil
}

// CreateFromBranch creates a new worktree at the given path with a new branch based off a specific base branch.
// This is used when we want a task's branch to start from a consolidated branch rather than HEAD.
// If the repository has submodules, they are automatically initialized in the new worktree.
func (m *Manager) CreateFromBranch(path, newBranch, baseBranch string) error {
	// Use git worktree add -b <newBranch> <path> <baseBranch>
	// This creates a worktree at <path> with a new branch <newBranch> starting from <baseBranch>
	args := []string{"worktree", "add", "-b", newBranch, path, baseBranch}
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoDir

	output, err := cmd.CombinedOutput()
	if m.logger != nil {
		m.logger.Debug("git command", "args", args, "output", truncateOutput(string(output), 500))
	}
	if err != nil {
		if m.logger != nil {
			m.logger.Error("git command failed", "args", args, "error", err, "stderr", string(output))
		}
		return fmt.Errorf("failed to create worktree from branch %s: %w\n%s", baseBranch, err, string(output))
	}

	if m.logger != nil {
		m.logger.Info("worktree created", "path", path, "branch", newBranch)
	}

	// Initialize submodules if the repository has any
	if err := m.InitSubmodules(path); err != nil {
		// Log but don't fail - submodule init issues are non-fatal
		if m.logger != nil {
			m.logger.Warn("failed to initialize submodules in worktree",
				"path", path,
				"error", err)
		}
	}

	// Apply sparse checkout if configured
	m.applySparseCheckout(path)

	return nil
}

// Remove removes a worktree
func (m *Manager) Remove(path string) error {
	// First, try to remove the worktree
	args := []string{"worktree", "remove", "--force", path}
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoDir

	output, err := cmd.CombinedOutput()
	if m.logger != nil {
		m.logger.Debug("git command", "args", args, "output", truncateOutput(string(output), 500))
	}
	if err != nil {
		if m.logger != nil {
			m.logger.Error("git command failed", "args", args, "error", err, "stderr", string(output))
		}
		// If worktree remove fails, try to clean up manually
		_ = os.RemoveAll(path)

		// Prune worktree references
		pruneCmd := exec.Command("git", "worktree", "prune")
		pruneCmd.Dir = m.repoDir
		_ = pruneCmd.Run()

		return fmt.Errorf("failed to remove worktree cleanly: %w\n%s", err, string(output))
	}

	if m.logger != nil {
		m.logger.Info("worktree removed", "path", path)
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
	args := []string{"branch", "-D", branch}
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoDir

	output, err := cmd.CombinedOutput()
	if m.logger != nil {
		m.logger.Debug("git command", "args", args, "output", truncateOutput(string(output), 500))
	}
	if err != nil {
		if m.logger != nil {
			m.logger.Error("git command failed", "args", args, "error", err, "stderr", string(output))
		}
		return fmt.Errorf("failed to delete branch: %w\n%s", err, string(output))
	}

	if m.logger != nil {
		m.logger.Info("branch deleted", "branch_name", branch)
	}

	return nil
}

// HasUncommittedChanges checks if a worktree has uncommitted changes
func (m *Manager) HasUncommittedChanges(path string) (bool, error) {
	args := []string{"status", "--porcelain"}
	cmd := exec.Command("git", args...)
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		if m.logger != nil {
			m.logger.Error("git command failed", "args", args, "error", err)
		}
		return false, fmt.Errorf("failed to check status: %w", err)
	}

	hasChanges := len(strings.TrimSpace(string(output))) > 0
	if m.logger != nil {
		m.logger.Debug("checking uncommitted changes", "path", path, "has_changes", hasChanges)
	}

	return hasChanges, nil
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
			_ = abortCmd.Run()
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
	_ = fetchCmd.Run() // Ignore error, might be offline

	behindCmd := exec.Command("git", "rev-list", "--count", "HEAD..origin/"+mainBranch)
	behindCmd.Dir = path
	output, err := behindCmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to check commits behind: %w", err)
	}

	count := 0
	_, _ = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
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

// FindMainBranch is the exported version of findMainBranch
func (m *Manager) FindMainBranch() string {
	return m.findMainBranch()
}

// BranchInfo contains information about a git branch
type BranchInfo struct {
	Name      string // Branch name (without refs/heads/ prefix)
	IsCurrent bool   // Whether this is the currently checked out branch
	IsMain    bool   // Whether this is the main/master branch
}

// ListBranches returns all local branches in the repository, sorted with main/master first
func (m *Manager) ListBranches() ([]BranchInfo, error) {
	// Use git branch to list all local branches
	// --format gives us control over output: %(refname:short) for name, %(HEAD) for current indicator
	cmd := exec.Command("git", "branch", "--format=%(HEAD)|%(refname:short)")
	cmd.Dir = m.repoDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	mainBranch := m.findMainBranch()
	var branches []BranchInfo
	var mainBranchInfo *BranchInfo

	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			// Skip malformed lines - this shouldn't happen with git's --format output,
			// but we gracefully skip rather than fail the entire operation
			continue
		}

		isCurrent := strings.TrimSpace(parts[0]) == "*"
		name := strings.TrimSpace(parts[1])

		info := BranchInfo{
			Name:      name,
			IsCurrent: isCurrent,
			IsMain:    name == mainBranch,
		}

		// Collect main branch separately to put it first
		if info.IsMain {
			mainBranchInfo = &info
		} else {
			branches = append(branches, info)
		}
	}

	// Put main branch first
	var result []BranchInfo
	if mainBranchInfo != nil {
		result = append(result, *mainBranchInfo)
	}
	result = append(result, branches...)

	return result, nil
}

// CreateBranchFrom creates a new branch from a specified base branch (without creating a worktree)
func (m *Manager) CreateBranchFrom(branchName, baseBranch string) error {
	cmd := exec.Command("git", "branch", branchName, baseBranch)
	cmd.Dir = m.repoDir

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch %s from %s: %w\n%s", branchName, baseBranch, err, string(output))
	}

	return nil
}

// CreateWorktreeFromBranch creates a worktree from an existing branch.
// If the repository has submodules, they are automatically initialized in the new worktree.
func (m *Manager) CreateWorktreeFromBranch(path, branch string) error {
	cmd := exec.Command("git", "worktree", "add", path, branch)
	cmd.Dir = m.repoDir

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create worktree from branch %s: %w\n%s", branch, err, string(output))
	}

	// Initialize submodules if the repository has any
	if err := m.InitSubmodules(path); err != nil {
		// Log but don't fail - submodule init issues are non-fatal
		if m.logger != nil {
			m.logger.Warn("failed to initialize submodules in worktree",
				"path", path,
				"error", err)
		}
	}

	return nil
}

// HasCommitsBeyond returns true if the branch has commits beyond the base branch
func (m *Manager) HasCommitsBeyond(path, baseBranch string) (bool, error) {
	cmd := exec.Command("git", "rev-list", "--count", baseBranch+"..HEAD")
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to count commits: %w", err)
	}

	count := 0
	_, _ = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count > 0, nil
}

// GetCommitsBetween returns the commit SHAs between base and head (exclusive of base)
func (m *Manager) GetCommitsBetween(path, baseBranch, headBranch string) ([]string, error) {
	cmd := exec.Command("git", "rev-list", "--reverse", baseBranch+".."+headBranch)
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	lines := strings.TrimSpace(string(output))
	if lines == "" {
		return []string{}, nil
	}

	return strings.Split(lines, "\n"), nil
}

// CountCommitsBetween returns the number of commits between base and head branches.
// This is more efficient than GetCommitsBetween when you only need the count.
func (m *Manager) CountCommitsBetween(path, baseBranch, headBranch string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", baseBranch+".."+headBranch)
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to count commits between %s and %s: %w", baseBranch, headBranch, err)
	}

	count := 0
	_, _ = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, nil
}

// CherryPickBranch cherry-picks all commits from sourceBranch that aren't in the current branch
// It cherry-picks commits one by one in order (oldest first)
func (m *Manager) CherryPickBranch(path, sourceBranch string) error {
	mainBranch := m.findMainBranch()

	// Get commits from source branch that are beyond main
	commits, err := m.GetCommitsBetween(path, mainBranch, sourceBranch)
	if err != nil {
		return fmt.Errorf("failed to get commits from %s: %w", sourceBranch, err)
	}

	if len(commits) == 0 {
		return nil // Nothing to cherry-pick
	}

	// Cherry-pick each commit
	for _, commit := range commits {
		cmd := exec.Command("git", "cherry-pick", commit)
		cmd.Dir = path

		if output, err := cmd.CombinedOutput(); err != nil {
			// Check for conflicts
			if strings.Contains(string(output), "CONFLICT") || strings.Contains(string(output), "could not apply") {
				return &CherryPickConflictError{
					Commit:       commit,
					SourceBranch: sourceBranch,
					Output:       string(output),
				}
			}
			return fmt.Errorf("failed to cherry-pick commit %s: %w\n%s", commit, err, string(output))
		}
	}

	return nil
}

// CherryPickConflictError represents a conflict during cherry-pick
type CherryPickConflictError struct {
	Commit       string
	SourceBranch string
	Output       string
}

func (e *CherryPickConflictError) Error() string {
	return fmt.Sprintf("cherry-pick conflict on commit %s from %s", e.Commit, e.SourceBranch)
}

// AbortCherryPick aborts an in-progress cherry-pick
func (m *Manager) AbortCherryPick(path string) error {
	cmd := exec.Command("git", "cherry-pick", "--abort")
	cmd.Dir = path

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to abort cherry-pick: %w\n%s", err, string(output))
	}

	return nil
}

// ContinueCherryPick continues cherry-pick after conflict resolution
func (m *Manager) ContinueCherryPick(path string) error {
	cmd := exec.Command("git", "cherry-pick", "--continue")
	cmd.Dir = path

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to continue cherry-pick: %w\n%s", err, string(output))
	}

	return nil
}

// CheckCherryPickConflicts checks if cherry-picking a branch would cause conflicts
// Returns a list of files that would conflict, or empty if clean
func (m *Manager) CheckCherryPickConflicts(path, sourceBranch string) ([]string, error) {
	mainBranch := m.findMainBranch()

	// Get commits from source branch
	commits, err := m.GetCommitsBetween(path, mainBranch, sourceBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	if len(commits) == 0 {
		return []string{}, nil
	}

	// Get current HEAD
	headCmd := exec.Command("git", "rev-parse", "HEAD")
	headCmd.Dir = path
	headOutput, err := headCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}
	currentHead := strings.TrimSpace(string(headOutput))

	// Try a dry-run by doing cherry-pick --no-commit and checking for conflicts
	// We'll need to reset afterwards
	var conflictFiles []string

	for _, commit := range commits {
		// Try cherry-pick with --no-commit
		cpCmd := exec.Command("git", "cherry-pick", "--no-commit", commit)
		cpCmd.Dir = path
		output, err := cpCmd.CombinedOutput()

		if err != nil {
			if strings.Contains(string(output), "CONFLICT") {
				// Get conflicting files
				statusCmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
				statusCmd.Dir = path
				statusOutput, _ := statusCmd.Output()
				files := strings.Split(strings.TrimSpace(string(statusOutput)), "\n")
				for _, f := range files {
					if f != "" {
						conflictFiles = append(conflictFiles, f)
					}
				}
			}
			// Abort and reset
			_ = m.AbortCherryPick(path)
			break
		}
	}

	// Reset to original HEAD (clean up our dry run)
	resetCmd := exec.Command("git", "reset", "--hard", currentHead)
	resetCmd.Dir = path
	_ = resetCmd.Run()

	return conflictFiles, nil
}

// GetConflictingFiles returns files with merge conflicts in a worktree
func (m *Manager) GetConflictingFiles(path string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get conflicting files: %w", err)
	}

	lines := strings.TrimSpace(string(output))
	if lines == "" {
		return []string{}, nil
	}

	return strings.Split(lines, "\n"), nil
}

// IsCherryPickInProgress returns true if a cherry-pick is in progress
func (m *Manager) IsCherryPickInProgress(path string) bool {
	cherryPickHead := filepath.Join(path, ".git", "CHERRY_PICK_HEAD")
	_, err := os.Stat(cherryPickHead)
	return err == nil
}

// EnableSparseCheckout configures sparse checkout for an existing worktree.
// This must be called after the worktree is created.
// If coneMode is true, git's cone mode is used (faster, uses directory paths).
// If coneMode is false, gitignore-style patterns are used.
func (m *Manager) EnableSparseCheckout(path string, directories []string, coneMode bool) error {
	if len(directories) == 0 {
		return fmt.Errorf("at least one directory is required for sparse checkout")
	}

	// Initialize sparse checkout
	var initArgs []string
	if coneMode {
		initArgs = []string{"sparse-checkout", "init", "--cone"}
	} else {
		initArgs = []string{"sparse-checkout", "init"}
	}

	initCmd := exec.Command("git", initArgs...)
	initCmd.Dir = path

	output, err := initCmd.CombinedOutput()
	if m.logger != nil {
		m.logger.Debug("git command", "args", initArgs, "output", truncateOutput(string(output), 500))
	}
	if err != nil {
		if m.logger != nil {
			m.logger.Error("git sparse-checkout init failed", "args", initArgs, "error", err, "stderr", string(output))
		}
		return fmt.Errorf("failed to initialize sparse checkout: %w\n%s", err, string(output))
	}

	// Set the sparse checkout directories
	setArgs := append([]string{"sparse-checkout", "set"}, directories...)
	setCmd := exec.Command("git", setArgs...)
	setCmd.Dir = path

	output, err = setCmd.CombinedOutput()
	if m.logger != nil {
		m.logger.Debug("git command", "args", setArgs, "output", truncateOutput(string(output), 500))
	}
	if err != nil {
		if m.logger != nil {
			m.logger.Error("git sparse-checkout set failed", "args", setArgs, "error", err, "stderr", string(output))
		}
		return fmt.Errorf("failed to set sparse checkout directories: %w\n%s", err, string(output))
	}

	if m.logger != nil {
		m.logger.Info("sparse checkout enabled", "path", path, "directories", directories, "cone_mode", coneMode)
	}

	return nil
}

// DisableSparseCheckout disables sparse checkout for a worktree, restoring full checkout.
func (m *Manager) DisableSparseCheckout(path string) error {
	args := []string{"sparse-checkout", "disable"}
	cmd := exec.Command("git", args...)
	cmd.Dir = path

	output, err := cmd.CombinedOutput()
	if m.logger != nil {
		m.logger.Debug("git command", "args", args, "output", truncateOutput(string(output), 500))
	}
	if err != nil {
		if m.logger != nil {
			m.logger.Error("git sparse-checkout disable failed", "args", args, "error", err, "stderr", string(output))
		}
		return fmt.Errorf("failed to disable sparse checkout: %w\n%s", err, string(output))
	}

	if m.logger != nil {
		m.logger.Info("sparse checkout disabled", "path", path)
	}

	return nil
}

// IsSparseCheckoutEnabled checks if sparse checkout is enabled for a worktree.
func (m *Manager) IsSparseCheckoutEnabled(path string) (bool, error) {
	// With extensions.worktreeConfig=true (used when sparse-checkout is initialized with --cone),
	// the config is stored in the worktree-specific config file.
	// We need to use --worktree flag to check the correct config location.
	args := []string{"config", "--worktree", "--get", "core.sparseCheckout"}
	cmd := exec.Command("git", args...)
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		// Try fallback to --local if --worktree fails
		// (older git versions or repos without worktreeConfig extension)
		args = []string{"config", "--local", "--get", "core.sparseCheckout"}
		cmd = exec.Command("git", args...)
		cmd.Dir = path
		output, err = cmd.Output()
		if err != nil {
			// Config key doesn't exist means sparse checkout is not enabled
			return false, nil
		}
	}

	value := strings.TrimSpace(string(output))
	return value == "true", nil
}

// GetSparseCheckoutPatterns returns the current sparse checkout patterns for a worktree.
// Returns an empty slice if sparse checkout is not enabled.
func (m *Manager) GetSparseCheckoutPatterns(path string) ([]string, error) {
	args := []string{"sparse-checkout", "list"}
	cmd := exec.Command("git", args...)
	cmd.Dir = path

	output, err := cmd.CombinedOutput()
	if err != nil {
		// If sparse-checkout is not enabled, the command may fail or return empty
		// This is not an error condition - just means no sparse checkout
		if m.logger != nil {
			m.logger.Debug("sparse-checkout list returned error (may not be enabled)", "error", err)
		}
		return []string{}, nil
	}

	lines := strings.TrimSpace(string(output))
	if lines == "" {
		return []string{}, nil
	}

	return strings.Split(lines, "\n"), nil
}

// applySparseCheckout applies sparse checkout to a worktree if configured.
// This is called automatically after worktree creation when sparse checkout is configured.
//
// IMPORTANT: Failures are logged but do not cause an error - sparse checkout is treated
// as an optimization, not a requirement. If sparse checkout fails:
//   - The worktree is still fully functional with ALL files checked out
//   - A warning is logged (if logger is configured)
//   - Users who explicitly configured sparse checkout should check logs if disk usage
//     is higher than expected
//
// This design choice prioritizes worktree usability over strict sparse checkout enforcement.
// For critical sparse checkout requirements, callers should use EnableSparseCheckout directly
// and handle errors appropriately.
func (m *Manager) applySparseCheckout(path string) {
	if len(m.sparseCheckoutDirs) == 0 {
		return
	}

	if err := m.EnableSparseCheckout(path, m.sparseCheckoutDirs, m.coneMode); err != nil {
		// Always log the failure - this is an important operational warning
		// Users who configured sparse checkout expect it to work
		if m.logger != nil {
			m.logger.Warn("sparse checkout failed - worktree created with full checkout instead",
				"path", path,
				"configured_directories", m.sparseCheckoutDirs,
				"cone_mode", m.coneMode,
				"error", err,
			)
		}
		// We don't return an error - sparse checkout is an optimization
		// The worktree is still fully functional, just with all files
	}
}

// localClaudeFiles lists the gitignored Claude configuration files that should be
// copied from the main repo to worktrees. These files are typically used for local
// settings that users want available in all worktrees.
var localClaudeFiles = []string{
	"CLAUDE.local.md",
}

// CopyLocalClaudeFiles copies gitignored Claude configuration files from the main
// repository to the specified worktree. This ensures that local settings like
// CLAUDE.local.md are available in worktrees even though they're not tracked by git.
//
// Files that don't exist in the source are silently skipped.
// Errors during individual file copies are logged but don't fail the operation.
func (m *Manager) CopyLocalClaudeFiles(worktreePath string) error {
	var lastErr error

	for _, filename := range localClaudeFiles {
		srcPath := filepath.Join(m.repoDir, filename)
		dstPath := filepath.Join(worktreePath, filename)

		if err := copyFile(srcPath, dstPath); err != nil {
			if !os.IsNotExist(err) {
				// Log non-existence errors but continue with other files
				if m.logger != nil {
					m.logger.Warn("failed to copy local Claude file",
						"file", filename,
						"src", srcPath,
						"dst", dstPath,
						"error", err,
					)
				}
				lastErr = err
			}
			// File doesn't exist - that's expected and fine
			continue
		}

		if m.logger != nil {
			m.logger.Debug("copied local Claude file to worktree",
				"file", filename,
				"worktree", worktreePath,
			)
		}
	}

	return lastErr
}

// copyFile copies a file from src to dst, preserving permissions.
// Returns os.ErrNotExist if the source file doesn't exist.
// If copying fails partway through, the incomplete destination file is removed.
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}

	// Track success to determine if cleanup is needed
	success := false
	defer func() {
		_ = dstFile.Close()
		if !success {
			_ = os.Remove(dst) // Clean up incomplete file on failure
		}
	}()

	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		return err
	}

	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		return err
	}

	success = true
	return nil
}
