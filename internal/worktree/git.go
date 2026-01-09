// Package worktree provides git worktree operations and abstractions.
//
// This file provides concrete CLI implementations of the git interfaces
// defined in interfaces.go. These implementations wrap actual git commands
// and can be used for production code, while the interfaces allow for
// mock implementations in tests.
package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Iron-Ham/claudio/internal/errors"
)

// -----------------------------------------------------------------------------
// Command Executor
// -----------------------------------------------------------------------------

// CommandExecutor abstracts command execution for testability.
// This allows tests to mock git commands without executing them.
type CommandExecutor interface {
	// Run executes a command and returns combined output.
	Run(dir string, name string, args ...string) ([]byte, error)

	// RunQuiet executes a command and returns only the error.
	RunQuiet(dir string, name string, args ...string) error
}

// CLICommandExecutor executes commands using os/exec.
type CLICommandExecutor struct{}

// NewCLICommandExecutor creates a new CLI command executor.
func NewCLICommandExecutor() *CLICommandExecutor {
	return &CLICommandExecutor{}
}

// Run executes a command and returns combined output.
func (e *CLICommandExecutor) Run(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// RunQuiet executes a command and returns only the error.
func (e *CLICommandExecutor) RunQuiet(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

// -----------------------------------------------------------------------------
// CLIGitOperations - implements GitOperations interface
// -----------------------------------------------------------------------------

// CLIGitOperations implements GitOperations using git CLI commands.
// It provides basic git operations like commit, push, rebase, and cherry-pick.
type CLIGitOperations struct {
	repoDir  string
	executor CommandExecutor
}

// NewCLIGitOperations creates a new CLIGitOperations.
func NewCLIGitOperations(repoDir string) *CLIGitOperations {
	return &CLIGitOperations{
		repoDir:  repoDir,
		executor: NewCLICommandExecutor(),
	}
}

// NewCLIGitOperationsWithExecutor creates a CLIGitOperations with a custom executor.
// This is primarily useful for testing.
func NewCLIGitOperationsWithExecutor(repoDir string, executor CommandExecutor) *CLIGitOperations {
	return &CLIGitOperations{
		repoDir:  repoDir,
		executor: executor,
	}
}

// CommitAll stages and commits all changes with the given message.
// Returns nil if there are no changes to commit.
func (g *CLIGitOperations) CommitAll(path, message string) error {
	// Stage all changes
	output, err := g.executor.Run(path, "git", "add", "-A")
	if err != nil {
		return errors.NewGitError("failed to stage changes", err).
			WithRepository(path).
			WithGitOutput(string(output))
	}

	// Commit
	output, err = g.executor.Run(path, "git", "commit", "-m", message)
	if err != nil {
		// Check if there's nothing to commit
		if strings.Contains(string(output), "nothing to commit") {
			return nil
		}
		return errors.NewGitError("failed to commit changes", err).
			WithRepository(path).
			WithGitOutput(string(output))
	}

	return nil
}

// HasUncommittedChanges returns true if there are uncommitted changes.
func (g *CLIGitOperations) HasUncommittedChanges(path string) (bool, error) {
	output, err := g.executor.Run(path, "git", "status", "--porcelain")
	if err != nil {
		return false, errors.NewGitError("failed to check git status", err).
			WithRepository(path).
			WithGitOutput(string(output))
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// GetCommitLog returns the commit log for the current branch since it diverged from main.
func (g *CLIGitOperations) GetCommitLog(path string) (string, error) {
	mainBranch := g.findMainBranch()

	output, err := g.executor.Run(path, "git", "log", mainBranch+"..HEAD", "--pretty=format:%s%n%b---")
	if err != nil {
		return "", errors.NewGitError("failed to get commit log", err).
			WithRepository(path).
			WithBranch(mainBranch).
			WithGitOutput(string(output))
	}
	return string(output), nil
}

// GetCommitsBetween returns commit SHAs between base and head (exclusive of base).
func (g *CLIGitOperations) GetCommitsBetween(path, baseBranch, headBranch string) ([]string, error) {
	output, err := g.executor.Run(path, "git", "rev-list", "--reverse", baseBranch+".."+headBranch)
	if err != nil {
		return nil, errors.NewGitError("failed to get commits between branches", err).
			WithRepository(path).
			WithBranch(baseBranch + ".." + headBranch).
			WithGitOutput(string(output))
	}

	lines := strings.TrimSpace(string(output))
	if lines == "" {
		return []string{}, nil
	}

	return strings.Split(lines, "\n"), nil
}

// CountCommitsBetween returns the number of commits between base and head.
func (g *CLIGitOperations) CountCommitsBetween(path, baseBranch, headBranch string) (int, error) {
	output, err := g.executor.Run(path, "git", "rev-list", "--count", baseBranch+".."+headBranch)
	if err != nil {
		return 0, errors.NewGitError("failed to count commits between branches", err).
			WithRepository(path).
			WithBranch(baseBranch + ".." + headBranch).
			WithGitOutput(string(output))
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, errors.NewGitError("failed to parse commit count", err).
			WithRepository(path)
	}

	return count, nil
}

// HasCommitsBeyond returns true if there are commits beyond baseBranch.
func (g *CLIGitOperations) HasCommitsBeyond(path, baseBranch string) (bool, error) {
	count, err := g.CountCommitsBetween(path, baseBranch, "HEAD")
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Push pushes the current branch to the remote.
// If force is true, uses --force-with-lease for safety.
func (g *CLIGitOperations) Push(path string, force bool) error {
	args := []string{"push", "-u", "origin", "HEAD"}
	if force {
		args = append(args, "--force-with-lease")
	}

	output, err := g.executor.Run(path, "git", args...)
	if err != nil {
		return errors.NewGitError("failed to push", err).
			WithRepository(path).
			WithGitOutput(string(output))
	}

	return nil
}

// RebaseOnMain rebases the current branch on origin/main (or origin/master).
func (g *CLIGitOperations) RebaseOnMain(path string) error {
	mainBranch := g.findMainBranch()

	// First fetch the latest from origin
	output, err := g.executor.Run(path, "git", "fetch", "origin", mainBranch)
	if err != nil {
		return errors.NewGitError("failed to fetch origin/"+mainBranch, err).
			WithRepository(path).
			WithBranch(mainBranch).
			WithGitOutput(string(output))
	}

	// Rebase on origin/main
	output, err = g.executor.Run(path, "git", "rebase", "origin/"+mainBranch)
	if err != nil {
		outputStr := string(output)
		// Check if there are conflicts
		if strings.Contains(outputStr, "CONFLICT") || strings.Contains(outputStr, "could not apply") {
			// Abort the rebase so we don't leave things in a bad state
			_, _ = g.executor.Run(path, "git", "rebase", "--abort")
			return errors.NewGitError("rebase conflicts detected - manual resolution required", errors.ErrMergeConflict).
				WithRepository(path).
				WithBranch(mainBranch).
				WithGitOutput(outputStr)
		}
		return errors.NewGitError("failed to rebase", err).
			WithRepository(path).
			WithBranch(mainBranch).
			WithGitOutput(outputStr)
	}

	return nil
}

// HasRebaseConflicts checks if rebasing on main would cause conflicts.
// Returns true if conflicts would occur, false if rebase would be clean.
func (g *CLIGitOperations) HasRebaseConflicts(path string) (bool, error) {
	mainBranch := g.findMainBranch()

	// Fetch first
	_, err := g.executor.Run(path, "git", "fetch", "origin", mainBranch)
	if err != nil {
		return false, errors.NewGitError("failed to fetch", err).WithRepository(path)
	}

	// Check if we're already up to date
	behindOutput, err := g.executor.Run(path, "git", "rev-list", "--count", "HEAD..origin/"+mainBranch)
	if err != nil {
		return false, errors.NewGitError("failed to check commits behind", err).WithRepository(path)
	}

	behindCount := strings.TrimSpace(string(behindOutput))
	if behindCount == "0" {
		// Already up to date, no rebase needed
		return false, nil
	}

	// Get the merge base
	mergeBaseOutput, err := g.executor.Run(path, "git", "merge-base", "HEAD", "origin/"+mainBranch)
	if err != nil {
		return false, errors.NewGitError("failed to get merge base", err).
			WithRepository(path).
			WithGitOutput(string(mergeBaseOutput))
	}
	mergeBase := strings.TrimSpace(string(mergeBaseOutput))

	// Use merge-tree to check for conflicts
	mergeTreeOutput, err := g.executor.Run(path, "git", "merge-tree", mergeBase, "HEAD", "origin/"+mainBranch)
	if err != nil {
		// merge-tree doesn't return error on conflicts, just outputs them
		return false, errors.NewGitError("failed to run merge-tree", err).
			WithRepository(path).
			WithGitOutput(string(mergeTreeOutput))
	}

	// If output contains conflict markers, there would be conflicts
	output := string(mergeTreeOutput)
	hasConflicts := strings.Contains(output, "<<<<<<<") || strings.Contains(output, ">>>>>>>")

	return hasConflicts, nil
}

// GetBehindCount returns how many commits the branch is behind origin/main.
func (g *CLIGitOperations) GetBehindCount(path string) (int, error) {
	mainBranch := g.findMainBranch()

	// Try to fetch first (ignore errors, might be offline)
	_, _ = g.executor.Run(path, "git", "fetch", "origin", mainBranch)

	output, err := g.executor.Run(path, "git", "rev-list", "--count", "HEAD..origin/"+mainBranch)
	if err != nil {
		return 0, errors.NewGitError("failed to check commits behind", err).
			WithRepository(path).
			WithBranch(mainBranch).
			WithGitOutput(string(output))
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, errors.NewGitError("failed to parse behind count", err).
			WithRepository(path)
	}

	return count, nil
}

// CherryPickBranch cherry-picks all commits from sourceBranch that aren't in the current branch.
// It cherry-picks commits one by one in order (oldest first).
func (g *CLIGitOperations) CherryPickBranch(path, sourceBranch string) error {
	mainBranch := g.findMainBranch()

	// Get commits from source branch that are beyond main
	commits, err := g.GetCommitsBetween(path, mainBranch, sourceBranch)
	if err != nil {
		return errors.NewGitError("failed to get commits from source branch", err).
			WithRepository(path).
			WithBranch(sourceBranch)
	}

	if len(commits) == 0 {
		return nil // Nothing to cherry-pick
	}

	// Cherry-pick each commit
	for _, commit := range commits {
		output, err := g.executor.Run(path, "git", "cherry-pick", commit)
		if err != nil {
			outputStr := string(output)
			// Check for conflicts
			if strings.Contains(outputStr, "CONFLICT") || strings.Contains(outputStr, "could not apply") {
				return &CherryPickConflictError{
					Commit:       commit,
					SourceBranch: sourceBranch,
					Output:       outputStr,
				}
			}
			return errors.NewGitError("failed to cherry-pick commit "+commit, err).
				WithRepository(path).
				WithBranch(sourceBranch).
				WithGitOutput(outputStr)
		}
	}

	return nil
}

// CheckCherryPickConflicts checks if cherry-picking a branch would cause conflicts.
// Returns a list of files that would conflict, or empty if clean.
func (g *CLIGitOperations) CheckCherryPickConflicts(path, sourceBranch string) ([]string, error) {
	mainBranch := g.findMainBranch()

	// Get commits from source branch
	commits, err := g.GetCommitsBetween(path, mainBranch, sourceBranch)
	if err != nil {
		return nil, errors.NewGitError("failed to get commits", err).
			WithRepository(path).
			WithBranch(sourceBranch)
	}

	if len(commits) == 0 {
		return []string{}, nil
	}

	// Get current HEAD
	headOutput, err := g.executor.Run(path, "git", "rev-parse", "HEAD")
	if err != nil {
		return nil, errors.NewGitError("failed to get HEAD", err).
			WithRepository(path).
			WithGitOutput(string(headOutput))
	}
	currentHead := strings.TrimSpace(string(headOutput))

	// Try a dry-run by doing cherry-pick --no-commit and checking for conflicts
	var conflictFiles []string

	for _, commit := range commits {
		// Try cherry-pick with --no-commit
		output, err := g.executor.Run(path, "git", "cherry-pick", "--no-commit", commit)

		if err != nil {
			if strings.Contains(string(output), "CONFLICT") {
				// Get conflicting files
				statusOutput, _ := g.executor.Run(path, "git", "diff", "--name-only", "--diff-filter=U")
				files := strings.Split(strings.TrimSpace(string(statusOutput)), "\n")
				for _, f := range files {
					if f != "" {
						conflictFiles = append(conflictFiles, f)
					}
				}
			}
			// Abort and reset
			_, _ = g.executor.Run(path, "git", "cherry-pick", "--abort")
			break
		}
	}

	// Reset to original HEAD (clean up our dry run)
	_, _ = g.executor.Run(path, "git", "reset", "--hard", currentHead)

	return conflictFiles, nil
}

// AbortCherryPick aborts an in-progress cherry-pick.
func (g *CLIGitOperations) AbortCherryPick(path string) error {
	output, err := g.executor.Run(path, "git", "cherry-pick", "--abort")
	if err != nil {
		return errors.NewGitError("failed to abort cherry-pick", err).
			WithRepository(path).
			WithGitOutput(string(output))
	}
	return nil
}

// ContinueCherryPick continues cherry-pick after conflict resolution.
func (g *CLIGitOperations) ContinueCherryPick(path string) error {
	output, err := g.executor.Run(path, "git", "cherry-pick", "--continue")
	if err != nil {
		return errors.NewGitError("failed to continue cherry-pick", err).
			WithRepository(path).
			WithGitOutput(string(output))
	}
	return nil
}

// IsCherryPickInProgress returns true if a cherry-pick is in progress.
func (g *CLIGitOperations) IsCherryPickInProgress(path string) bool {
	cherryPickHead := filepath.Join(path, ".git", "CHERRY_PICK_HEAD")
	_, err := os.Stat(cherryPickHead)
	return err == nil
}

// GetConflictingFiles returns files with merge conflicts in a worktree.
func (g *CLIGitOperations) GetConflictingFiles(path string) ([]string, error) {
	output, err := g.executor.Run(path, "git", "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, errors.NewGitError("failed to get conflicting files", err).
			WithRepository(path).
			WithGitOutput(string(output))
	}

	lines := strings.TrimSpace(string(output))
	if lines == "" {
		return []string{}, nil
	}

	return strings.Split(lines, "\n"), nil
}

// findMainBranch returns the name of the main branch (main or master).
func (g *CLIGitOperations) findMainBranch() string {
	// Check if 'main' exists
	err := g.executor.RunQuiet(g.repoDir, "git", "rev-parse", "--verify", "main")
	if err == nil {
		return "main"
	}
	return "master"
}

// -----------------------------------------------------------------------------
// CLIWorktreeManager - implements WorktreeManager interface
// -----------------------------------------------------------------------------

// CLIWorktreeManager implements WorktreeManager using git CLI commands.
// It manages worktree creation, removal, and listing.
type CLIWorktreeManager struct {
	repoDir  string
	executor CommandExecutor
}

// NewCLIWorktreeManager creates a new CLIWorktreeManager.
func NewCLIWorktreeManager(repoDir string) *CLIWorktreeManager {
	return &CLIWorktreeManager{
		repoDir:  repoDir,
		executor: NewCLICommandExecutor(),
	}
}

// NewCLIWorktreeManagerWithExecutor creates a CLIWorktreeManager with a custom executor.
func NewCLIWorktreeManagerWithExecutor(repoDir string, executor CommandExecutor) *CLIWorktreeManager {
	return &CLIWorktreeManager{
		repoDir:  repoDir,
		executor: executor,
	}
}

// Create creates a new worktree with a new branch at the given path.
func (w *CLIWorktreeManager) Create(path, branch string) error {
	output, err := w.executor.Run(w.repoDir, "git", "worktree", "add", "-b", branch, path)
	if err != nil {
		return errors.NewGitError("failed to create worktree", err).
			WithRepository(w.repoDir).
			WithWorktree(path).
			WithBranch(branch).
			WithGitOutput(string(output))
	}
	return nil
}

// CreateFromBranch creates a worktree with a new branch based on an existing branch.
func (w *CLIWorktreeManager) CreateFromBranch(path, newBranch, baseBranch string) error {
	output, err := w.executor.Run(w.repoDir, "git", "worktree", "add", "-b", newBranch, path, baseBranch)
	if err != nil {
		return errors.NewGitError("failed to create worktree from branch "+baseBranch, err).
			WithRepository(w.repoDir).
			WithWorktree(path).
			WithBranch(newBranch).
			WithGitOutput(string(output))
	}
	return nil
}

// CreateWorktreeFromBranch creates a worktree from an existing branch (without creating a new branch).
func (w *CLIWorktreeManager) CreateWorktreeFromBranch(path, branch string) error {
	output, err := w.executor.Run(w.repoDir, "git", "worktree", "add", path, branch)
	if err != nil {
		return errors.NewGitError("failed to create worktree from existing branch "+branch, err).
			WithRepository(w.repoDir).
			WithWorktree(path).
			WithBranch(branch).
			WithGitOutput(string(output))
	}
	return nil
}

// Remove removes a worktree at the given path.
func (w *CLIWorktreeManager) Remove(path string) error {
	output, err := w.executor.Run(w.repoDir, "git", "worktree", "remove", "--force", path)
	if err != nil {
		// Try to clean up manually
		_ = os.RemoveAll(path)

		// Prune worktree references
		_, _ = w.executor.Run(w.repoDir, "git", "worktree", "prune")

		return errors.NewGitError("failed to remove worktree cleanly", err).
			WithRepository(w.repoDir).
			WithWorktree(path).
			WithGitOutput(string(output))
	}
	return nil
}

// List returns paths of all worktrees in the repository.
func (w *CLIWorktreeManager) List() ([]string, error) {
	output, err := w.executor.Run(w.repoDir, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, errors.NewGitError("failed to list worktrees", err).
			WithRepository(w.repoDir).
			WithGitOutput(string(output))
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

// GetPath returns the repository's root directory.
func (w *CLIWorktreeManager) GetPath() string {
	return w.repoDir
}

// -----------------------------------------------------------------------------
// CLIBranchManager - implements BranchManager interface
// -----------------------------------------------------------------------------

// CLIBranchManager implements BranchManager using git CLI commands.
// It handles branch creation, deletion, and querying.
type CLIBranchManager struct {
	repoDir  string
	executor CommandExecutor
}

// NewCLIBranchManager creates a new CLIBranchManager.
func NewCLIBranchManager(repoDir string) *CLIBranchManager {
	return &CLIBranchManager{
		repoDir:  repoDir,
		executor: NewCLICommandExecutor(),
	}
}

// NewCLIBranchManagerWithExecutor creates a CLIBranchManager with a custom executor.
func NewCLIBranchManagerWithExecutor(repoDir string, executor CommandExecutor) *CLIBranchManager {
	return &CLIBranchManager{
		repoDir:  repoDir,
		executor: executor,
	}
}

// CreateBranchFrom creates a new branch from a specified base branch (without creating a worktree).
func (b *CLIBranchManager) CreateBranchFrom(branchName, baseBranch string) error {
	output, err := b.executor.Run(b.repoDir, "git", "branch", branchName, baseBranch)
	if err != nil {
		// Check if branch already exists
		if strings.Contains(string(output), "already exists") {
			return errors.NewGitError("branch already exists", errors.ErrBranchExists).
				WithRepository(b.repoDir).
				WithBranch(branchName).
				WithGitOutput(string(output))
		}
		return errors.NewGitError("failed to create branch "+branchName+" from "+baseBranch, err).
			WithRepository(b.repoDir).
			WithBranch(branchName).
			WithGitOutput(string(output))
	}
	return nil
}

// DeleteBranch deletes a branch by name (force delete).
func (b *CLIBranchManager) DeleteBranch(branch string) error {
	output, err := b.executor.Run(b.repoDir, "git", "branch", "-D", branch)
	if err != nil {
		// Check if branch doesn't exist
		if strings.Contains(string(output), "not found") {
			return errors.NewGitError("branch not found", errors.ErrBranchNotFound).
				WithRepository(b.repoDir).
				WithBranch(branch).
				WithGitOutput(string(output))
		}
		return errors.NewGitError("failed to delete branch", err).
			WithRepository(b.repoDir).
			WithBranch(branch).
			WithGitOutput(string(output))
	}
	return nil
}

// GetBranch returns the current branch name for a given worktree path.
func (b *CLIBranchManager) GetBranch(path string) (string, error) {
	output, err := b.executor.Run(path, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", errors.NewGitError("failed to get branch", err).
			WithRepository(path).
			WithGitOutput(string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// FindMainBranch returns the name of the main branch (main or master).
func (b *CLIBranchManager) FindMainBranch() string {
	// Check if 'main' exists
	err := b.executor.RunQuiet(b.repoDir, "git", "rev-parse", "--verify", "main")
	if err == nil {
		return "main"
	}
	return "master"
}

// -----------------------------------------------------------------------------
// CLIDiffProvider - implements DiffProvider interface
// -----------------------------------------------------------------------------

// CLIDiffProvider implements DiffProvider using git CLI commands.
// It handles diff generation and change detection.
type CLIDiffProvider struct {
	repoDir  string
	executor CommandExecutor
}

// NewCLIDiffProvider creates a new CLIDiffProvider.
func NewCLIDiffProvider(repoDir string) *CLIDiffProvider {
	return &CLIDiffProvider{
		repoDir:  repoDir,
		executor: NewCLICommandExecutor(),
	}
}

// NewCLIDiffProviderWithExecutor creates a CLIDiffProvider with a custom executor.
func NewCLIDiffProviderWithExecutor(repoDir string, executor CommandExecutor) *CLIDiffProvider {
	return &CLIDiffProvider{
		repoDir:  repoDir,
		executor: executor,
	}
}

// GetDiffAgainstMain returns the diff of the branch against main/master.
// Uses three-dot syntax (main...HEAD) to show changes since divergence.
func (d *CLIDiffProvider) GetDiffAgainstMain(path string) (string, error) {
	mainBranch := d.findMainBranch()

	output, err := d.executor.Run(path, "git", "diff", mainBranch+"...HEAD")
	if err != nil {
		return "", errors.NewGitError("failed to get diff", err).
			WithRepository(path).
			WithBranch(mainBranch).
			WithGitOutput(string(output))
	}
	return string(output), nil
}

// GetChangedFiles returns a list of file paths that changed compared to main.
func (d *CLIDiffProvider) GetChangedFiles(path string) ([]string, error) {
	mainBranch := d.findMainBranch()

	output, err := d.executor.Run(path, "git", "diff", "--name-only", mainBranch+"...HEAD")
	if err != nil {
		return nil, errors.NewGitError("failed to get changed files", err).
			WithRepository(path).
			WithBranch(mainBranch).
			WithGitOutput(string(output))
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 1 && files[0] == "" {
		return []string{}, nil
	}

	return files, nil
}

// HasUncommittedChanges checks if a worktree has uncommitted changes.
func (d *CLIDiffProvider) HasUncommittedChanges(path string) (bool, error) {
	output, err := d.executor.Run(path, "git", "status", "--porcelain")
	if err != nil {
		return false, errors.NewGitError("failed to check status", err).
			WithRepository(path).
			WithGitOutput(string(output))
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// findMainBranch returns the name of the main branch (main or master).
func (d *CLIDiffProvider) findMainBranch() string {
	// Check if 'main' exists
	err := d.executor.RunQuiet(d.repoDir, "git", "rev-parse", "--verify", "main")
	if err == nil {
		return "main"
	}
	return "master"
}

// -----------------------------------------------------------------------------
// GitClient - Composite client combining all implementations
// -----------------------------------------------------------------------------

// GitClient provides a unified interface combining all git operations.
// It composes the individual interface implementations for convenience.
type GitClient struct {
	repoDir string
	GitOperations
	WorktreeManager
	BranchManager
	DiffProvider
}

// NewGitClient creates a new GitClient with all CLI implementations.
func NewGitClient(repoDir string) *GitClient {
	return &GitClient{
		repoDir:         repoDir,
		GitOperations:   NewCLIGitOperations(repoDir),
		WorktreeManager: NewCLIWorktreeManager(repoDir),
		BranchManager:   NewCLIBranchManager(repoDir),
		DiffProvider:    NewCLIDiffProvider(repoDir),
	}
}

// GetRepoDir returns the repository directory.
func (c *GitClient) GetRepoDir() string {
	return c.repoDir
}

// HasUncommittedChanges resolves the ambiguity between GitOperations and DiffProvider.
// Both interfaces define this method, so we need to explicitly implement it.
func (c *GitClient) HasUncommittedChanges(path string) (bool, error) {
	return c.GitOperations.HasUncommittedChanges(path)
}

// Ensure individual implementations satisfy their interfaces at compile time.
var (
	_ GitOperations   = (*CLIGitOperations)(nil)
	_ WorktreeManager = (*CLIWorktreeManager)(nil)
	_ BranchManager   = (*CLIBranchManager)(nil)
	_ DiffProvider    = (*CLIDiffProvider)(nil)
)
