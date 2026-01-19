// Package conflict provides merge conflict detection and handling for consolidation.
// It wraps git operations to provide a clean interface for detecting and reporting
// conflicts during branch consolidation.
package conflict

import (
	"context"
	"errors"
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
	"github.com/Iron-Ham/claudio/internal/worktree"
)

// Info is an alias to consolidation.ConflictInfo for convenience.
type Info = consolidation.ConflictInfo

// Compile-time check that Detector implements consolidation.ConflictManager.
var _ consolidation.ConflictManager = (*Detector)(nil)

// BranchOperator defines the operations needed for conflict detection.
type BranchOperator interface {
	GetConflictingFiles(ctx context.Context, worktreePath string) ([]string, error)
	CherryPickBranch(ctx context.Context, worktreePath, sourceBranch string) error
	AbortCherryPick(ctx context.Context, worktreePath string) error
	ContinueCherryPick(ctx context.Context, worktreePath string) error
}

// Detector detects merge conflicts during consolidation.
type Detector struct {
	branch BranchOperator
}

// NewDetector creates a new conflict Detector.
func NewDetector(branch BranchOperator) *Detector {
	return &Detector{branch: branch}
}

// CheckCherryPick attempts a cherry-pick and reports any conflicts.
// If the cherry-pick succeeds, it returns nil.
// If there are conflicts, it returns an Info describing the conflict.
// The caller is responsible for handling or aborting the cherry-pick.
func (d *Detector) CheckCherryPick(ctx context.Context, worktreePath, sourceBranch, taskID, taskTitle string) (*Info, error) {
	err := d.branch.CherryPickBranch(ctx, worktreePath, sourceBranch)
	if err == nil {
		return nil, nil // No conflict
	}

	// Check if it's a cherry-pick conflict error
	var cpErr *worktree.CherryPickConflictError
	if !errors.As(err, &cpErr) {
		// Not a conflict error, something else went wrong
		return nil, fmt.Errorf("cherry-pick failed: %w", err)
	}

	// Get the conflicting files
	files, filesErr := d.branch.GetConflictingFiles(ctx, worktreePath)
	if filesErr != nil {
		files = []string{} // Use empty slice if we can't get the files
	}

	return &Info{
		TaskID:       taskID,
		TaskTitle:    taskTitle,
		Branch:       sourceBranch,
		Files:        files,
		WorktreePath: worktreePath,
	}, nil
}

// AbortCherryPick aborts an in-progress cherry-pick operation.
func (d *Detector) AbortCherryPick(ctx context.Context, worktreePath string) error {
	return d.branch.AbortCherryPick(ctx, worktreePath)
}

// ContinueCherryPick continues a cherry-pick after conflicts have been resolved.
func (d *Detector) ContinueCherryPick(ctx context.Context, worktreePath string) error {
	return d.branch.ContinueCherryPick(ctx, worktreePath)
}

// GetConflictingFiles returns the files with conflicts in the given worktree.
func (d *Detector) GetConflictingFiles(ctx context.Context, worktreePath string) ([]string, error) {
	return d.branch.GetConflictingFiles(ctx, worktreePath)
}
