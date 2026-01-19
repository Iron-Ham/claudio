// Package branch provides git branch operations for consolidation.
// It wraps the worktree.Manager to provide consolidation-specific
// branch management functionality.
package branch

import (
	"context"
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
	"github.com/Iron-Ham/claudio/internal/worktree"
)

// Compile-time check that Manager implements consolidation.BranchManager.
var _ consolidation.BranchManager = (*Manager)(nil)

// Manager handles git branch operations for consolidation.
type Manager struct {
	wt     WorktreeManager
	naming *NamingStrategy
}

// WorktreeManager defines the interface for git worktree operations.
// This interface allows for easy testing with mocks.
type WorktreeManager interface {
	FindMainBranch() string
	CreateBranchFrom(branchName, baseBranch string) error
	CreateWorktreeFromBranch(path, branch string) error
	CherryPickBranch(path, sourceBranch string) error
	GetChangedFiles(path string) ([]string, error)
	GetConflictingFiles(path string) ([]string, error)
	CountCommitsBetween(path, baseBranch, headBranch string) (int, error)
	Push(path string, force bool) error
	Remove(path string) error
	DeleteBranch(branch string) error
	ContinueCherryPick(path string) error
	AbortCherryPick(path string) error
}

// Option configures a Manager.
type Option func(*Manager)

// WithNamingStrategy sets a custom naming strategy.
func WithNamingStrategy(ns *NamingStrategy) Option {
	return func(m *Manager) {
		m.naming = ns
	}
}

// New creates a new branch Manager wrapping the given worktree manager.
func New(wt WorktreeManager, opts ...Option) *Manager {
	m := &Manager{
		wt:     wt,
		naming: NewNamingStrategy("", ""),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// NewFromWorktreeManager creates a Manager from a worktree.Manager.
// This is a convenience function for production use.
func NewFromWorktreeManager(wt *worktree.Manager, opts ...Option) *Manager {
	return New(wt, opts...)
}

// FindMainBranch returns the name of the main branch (main or master).
func (m *Manager) FindMainBranch(_ context.Context) string {
	return m.wt.FindMainBranch()
}

// CreateGroupBranch creates a branch for a consolidation group.
func (m *Manager) CreateGroupBranch(_ context.Context, groupIdx int, baseBranch string) (string, error) {
	branchName := m.naming.GroupBranchName(groupIdx)
	if err := m.wt.CreateBranchFrom(branchName, baseBranch); err != nil {
		return "", fmt.Errorf("failed to create group branch %s from %s: %w", branchName, baseBranch, err)
	}
	return branchName, nil
}

// CreateSingleBranch creates a branch for single PR mode consolidation.
func (m *Manager) CreateSingleBranch(_ context.Context, baseBranch string) (string, error) {
	branchName := m.naming.SingleBranchName()
	if err := m.wt.CreateBranchFrom(branchName, baseBranch); err != nil {
		return "", fmt.Errorf("failed to create consolidated branch %s from %s: %w", branchName, baseBranch, err)
	}
	return branchName, nil
}

// CreateWorktree creates a worktree for the given branch.
func (m *Manager) CreateWorktree(_ context.Context, path, branch string) error {
	if err := m.wt.CreateWorktreeFromBranch(path, branch); err != nil {
		return fmt.Errorf("failed to create worktree at %s for branch %s: %w", path, branch, err)
	}
	return nil
}

// CherryPickBranch cherry-picks commits from source branch into the worktree.
func (m *Manager) CherryPickBranch(_ context.Context, worktreePath, sourceBranch string) error {
	return m.wt.CherryPickBranch(worktreePath, sourceBranch)
}

// GetChangedFiles returns files changed in the worktree compared to main.
func (m *Manager) GetChangedFiles(_ context.Context, worktreePath string) ([]string, error) {
	return m.wt.GetChangedFiles(worktreePath)
}

// GetConflictingFiles returns files with merge conflicts.
func (m *Manager) GetConflictingFiles(_ context.Context, worktreePath string) ([]string, error) {
	return m.wt.GetConflictingFiles(worktreePath)
}

// CountCommitsBetween returns the number of commits between base and head.
func (m *Manager) CountCommitsBetween(_ context.Context, worktreePath, baseBranch, headBranch string) (int, error) {
	return m.wt.CountCommitsBetween(worktreePath, baseBranch, headBranch)
}

// Push pushes the branch to the remote.
func (m *Manager) Push(_ context.Context, worktreePath string, force bool) error {
	return m.wt.Push(worktreePath, force)
}

// RemoveWorktree removes a worktree.
func (m *Manager) RemoveWorktree(_ context.Context, path string) error {
	return m.wt.Remove(path)
}

// DeleteBranch deletes a branch.
func (m *Manager) DeleteBranch(_ context.Context, branch string) error {
	return m.wt.DeleteBranch(branch)
}

// ContinueCherryPick continues a cherry-pick after conflict resolution.
func (m *Manager) ContinueCherryPick(_ context.Context, worktreePath string) error {
	return m.wt.ContinueCherryPick(worktreePath)
}

// AbortCherryPick aborts an in-progress cherry-pick.
func (m *Manager) AbortCherryPick(_ context.Context, worktreePath string) error {
	return m.wt.AbortCherryPick(worktreePath)
}

// NamingStrategy returns the naming strategy used by this manager.
func (m *Manager) NamingStrategy() *NamingStrategy {
	return m.naming
}
