// Package consolidation provides interfaces and implementations for consolidating
// work from multiple task branches into unified branches for PR creation.
package consolidation

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitOperations defines the interface for git operations needed during consolidation.
// This abstraction enables testing without actual git repositories and allows
// alternative implementations (e.g., using libgit2 or GitHub API).
type GitOperations interface {
	// CreateBranch creates a new branch from a base branch.
	// The new branch is created at the same commit as base.
	CreateBranch(name, base string) error

	// Merge merges the specified branch into the current branch.
	// Returns an error if merge fails (including conflicts).
	Merge(branch string) error

	// CherryPick applies the specified commits to the current branch.
	// Commits are applied in order. Returns an error on first failure.
	CherryPick(commits []string) error

	// HasConflicts returns true if there are unresolved merge conflicts.
	HasConflicts() bool

	// GetConflicts returns a list of files with unresolved conflicts.
	GetConflicts() []string

	// AbortMerge aborts an in-progress merge operation.
	// This also works for aborting cherry-picks.
	AbortMerge() error

	// CommitMerge commits the current merge with the given message.
	// Used after manually resolving conflicts.
	CommitMerge(message string) error

	// Push pushes the specified branch to the remote origin.
	Push(branch string) error
}

// MergeConflictError represents a conflict during merge or cherry-pick.
type MergeConflictError struct {
	Operation    string   // "merge" or "cherry-pick"
	Branch       string   // The branch being merged/picked
	Commit       string   // The specific commit (for cherry-pick)
	Files        []string // Files with conflicts
	GitOutput    string   // Raw git output
}

func (e *MergeConflictError) Error() string {
	if e.Commit != "" {
		return fmt.Sprintf("%s conflict on commit %s from %s: %d file(s) affected",
			e.Operation, e.Commit, e.Branch, len(e.Files))
	}
	return fmt.Sprintf("%s conflict on branch %s: %d file(s) affected",
		e.Operation, e.Branch, len(e.Files))
}

// DefaultGitOperations implements GitOperations using exec to run git commands.
type DefaultGitOperations struct {
	// WorkDir is the working directory for git commands.
	// This should be a git worktree or repository root.
	WorkDir string

	// RepoDir is the main repository directory (for operations that need it).
	// If empty, WorkDir is used.
	RepoDir string
}

// NewDefaultGitOperations creates a new DefaultGitOperations instance.
func NewDefaultGitOperations(workDir string) *DefaultGitOperations {
	return &DefaultGitOperations{
		WorkDir: workDir,
		RepoDir: workDir,
	}
}

// NewDefaultGitOperationsWithRepo creates a new DefaultGitOperations instance
// with separate working directory and repository root.
func NewDefaultGitOperationsWithRepo(workDir, repoDir string) *DefaultGitOperations {
	return &DefaultGitOperations{
		WorkDir: workDir,
		RepoDir: repoDir,
	}
}

// CreateBranch creates a new branch from a base branch.
func (g *DefaultGitOperations) CreateBranch(name, base string) error {
	cmd := exec.Command("git", "branch", name, base)
	cmd.Dir = g.repoDir()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create branch %s from %s: %w\n%s",
			name, base, err, string(output))
	}

	return nil
}

// Merge merges the specified branch into the current branch.
func (g *DefaultGitOperations) Merge(branch string) error {
	cmd := exec.Command("git", "merge", "--no-ff", branch)
	cmd.Dir = g.WorkDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		// Check for conflicts
		if strings.Contains(outputStr, "CONFLICT") || strings.Contains(outputStr, "Automatic merge failed") {
			files := g.GetConflicts()
			return &MergeConflictError{
				Operation: "merge",
				Branch:    branch,
				Files:     files,
				GitOutput: outputStr,
			}
		}
		return fmt.Errorf("failed to merge branch %s: %w\n%s", branch, err, outputStr)
	}

	return nil
}

// CherryPick applies the specified commits to the current branch.
func (g *DefaultGitOperations) CherryPick(commits []string) error {
	if len(commits) == 0 {
		return nil
	}

	for _, commit := range commits {
		cmd := exec.Command("git", "cherry-pick", commit)
		cmd.Dir = g.WorkDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			outputStr := string(output)
			// Check for conflicts
			if strings.Contains(outputStr, "CONFLICT") || strings.Contains(outputStr, "could not apply") {
				files := g.GetConflicts()
				return &MergeConflictError{
					Operation: "cherry-pick",
					Commit:    commit,
					Files:     files,
					GitOutput: outputStr,
				}
			}
			return fmt.Errorf("failed to cherry-pick commit %s: %w\n%s", commit, err, outputStr)
		}
	}

	return nil
}

// HasConflicts returns true if there are unresolved merge conflicts.
func (g *DefaultGitOperations) HasConflicts() bool {
	return len(g.GetConflicts()) > 0
}

// GetConflicts returns a list of files with unresolved conflicts.
func (g *DefaultGitOperations) GetConflicts() []string {
	// Use git diff with --diff-filter=U to find unmerged files
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = g.WorkDir

	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.TrimSpace(string(output))
	if lines == "" {
		return nil
	}

	return strings.Split(lines, "\n")
}

// AbortMerge aborts an in-progress merge or cherry-pick operation.
func (g *DefaultGitOperations) AbortMerge() error {
	// Try aborting merge first
	mergeCmd := exec.Command("git", "merge", "--abort")
	mergeCmd.Dir = g.WorkDir
	if err := mergeCmd.Run(); err == nil {
		return nil
	}

	// Try aborting cherry-pick
	cherryCmd := exec.Command("git", "cherry-pick", "--abort")
	cherryCmd.Dir = g.WorkDir
	if output, err := cherryCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to abort operation: %w\n%s", err, string(output))
	}

	return nil
}

// CommitMerge commits the current merge with the given message.
func (g *DefaultGitOperations) CommitMerge(message string) error {
	// Stage all changes first
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = g.WorkDir
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage changes: %w\n%s", err, string(output))
	}

	// Commit
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = g.WorkDir
	if output, err := commitCmd.CombinedOutput(); err != nil {
		outputStr := string(output)
		// "nothing to commit" is not an error
		if strings.Contains(outputStr, "nothing to commit") {
			return nil
		}
		return fmt.Errorf("failed to commit merge: %w\n%s", err, outputStr)
	}

	return nil
}

// Push pushes the specified branch to the remote origin.
func (g *DefaultGitOperations) Push(branch string) error {
	cmd := exec.Command("git", "push", "-u", "origin", branch)
	cmd.Dir = g.WorkDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push branch %s: %w\n%s", branch, err, string(output))
	}

	return nil
}

// repoDir returns the repository directory to use for operations.
func (g *DefaultGitOperations) repoDir() string {
	if g.RepoDir != "" {
		return g.RepoDir
	}
	return g.WorkDir
}

// Ensure DefaultGitOperations implements GitOperations
var _ GitOperations = (*DefaultGitOperations)(nil)
