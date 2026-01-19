package strategy

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
)

// Compile-time check that Single implements consolidation.Strategy.
var _ consolidation.Strategy = (*Single)(nil)

// Single implements the single PR consolidation strategy.
// It consolidates all tasks from all groups into a single PR.
type Single struct {
	*Base
}

// NewSingle creates a new Single strategy.
func NewSingle(deps Dependencies, config Config) *Single {
	return &Single{
		Base: NewBase(deps, config),
	}
}

// Name returns the strategy name.
func (s *Single) Name() string {
	return "single"
}

// SupportsParallel returns whether this strategy supports parallel task processing.
func (s *Single) SupportsParallel() bool {
	return true // Single strategy can process tasks in parallel within a group
}

// Execute runs the single PR consolidation strategy.
func (s *Single) Execute(ctx context.Context, groups []TaskGroup) (*Result, error) {
	startTime := time.Now()
	result := &Result{
		PRs:          make([]consolidation.PRInfo, 0, 1),
		GroupResults: make([]GroupResult, 0, len(groups)),
	}

	// Flatten all tasks from all groups
	var allTasks []consolidation.CompletedTask
	for _, group := range groups {
		allTasks = append(allTasks, group.Tasks...)
	}

	s.emit(consolidation.Event{
		Type:    consolidation.EventStarted,
		Message: fmt.Sprintf("Starting single PR consolidation for %d tasks", len(allTasks)),
	})

	mainBranch := s.deps.Branch.FindMainBranch(ctx)

	// Create consolidated branch
	branchName, err := s.deps.Branch.CreateSingleBranch(ctx, mainBranch)
	if err != nil {
		result.Error = err
		return result, fmt.Errorf("failed to create consolidated branch: %w", err)
	}

	// Create temporary worktree for consolidation
	worktreePath := filepath.Join(s.config.WorktreeDir, "consolidate-single")
	if err := s.deps.Branch.CreateWorktree(ctx, worktreePath, branchName); err != nil {
		result.Error = err
		return result, fmt.Errorf("failed to create worktree: %w", err)
	}
	defer func() {
		if err := s.deps.Branch.RemoveWorktree(ctx, worktreePath); err != nil {
			s.log().Warn("failed to remove worktree during cleanup",
				"worktree_path", worktreePath,
				"error", err,
			)
		}
	}()

	// Process each group in order (to maintain dependency order)
	for _, group := range groups {
		s.emit(consolidation.Event{
			Type:     consolidation.EventGroupStarted,
			GroupIdx: group.Index,
			Message:  fmt.Sprintf("Processing group %d with %d tasks", group.Index+1, len(group.Tasks)),
		})

		groupResult := GroupResult{
			GroupIndex: group.Index,
			BranchName: branchName,
			TaskIDs:    make([]string, 0, len(group.Tasks)),
			Success:    true,
		}

		for _, task := range group.Tasks {
			s.emit(consolidation.Event{
				Type:     consolidation.EventTaskMerging,
				GroupIdx: group.Index,
				TaskID:   task.ID,
				Message:  fmt.Sprintf("Merging task %s", task.ID),
			})

			conflictInfo, err := s.deps.Conflict.CheckCherryPick(ctx, worktreePath, task.Branch, task.ID, task.Title)
			if err != nil {
				groupResult.Success = false
				groupResult.Error = err.Error()
				result.GroupResults = append(result.GroupResults, groupResult)
				result.Error = fmt.Errorf("cherry-pick check failed for task %s: %w", task.ID, err)
				return result, result.Error
			}

			if conflictInfo != nil {
				s.emit(consolidation.Event{
					Type:     consolidation.EventConflict,
					GroupIdx: group.Index,
					TaskID:   task.ID,
					Message:  fmt.Sprintf("Conflict in task %s: %v", task.ID, conflictInfo.Files),
				})

				if abortErr := s.deps.Conflict.AbortCherryPick(ctx, worktreePath); abortErr != nil {
					s.log().Warn("failed to abort cherry-pick after conflict detection",
						"worktree_path", worktreePath,
						"task_id", task.ID,
						"abort_error", abortErr,
					)
				}
				groupResult.Success = false
				groupResult.Error = fmt.Sprintf("merge conflict: files %v", conflictInfo.Files)
				result.GroupResults = append(result.GroupResults, groupResult)
				result.Error = fmt.Errorf("merge conflict in task %s", task.ID)
				return result, result.Error
			}

			groupResult.TaskIDs = append(groupResult.TaskIDs, task.ID)

			s.emit(consolidation.Event{
				Type:     consolidation.EventTaskMerged,
				GroupIdx: group.Index,
				TaskID:   task.ID,
				Message:  fmt.Sprintf("Task %s merged successfully", task.ID),
			})
		}

		result.GroupResults = append(result.GroupResults, groupResult)

		s.emit(consolidation.Event{
			Type:     consolidation.EventGroupComplete,
			GroupIdx: group.Index,
			Message:  fmt.Sprintf("Group %d processing complete", group.Index+1),
		})
	}

	// Get commit count and changed files
	commitCount, err := s.deps.Branch.CountCommitsBetween(ctx, worktreePath, mainBranch, "HEAD")
	if err != nil {
		s.log().Warn("failed to count commits",
			"worktree_path", worktreePath,
			"base_branch", mainBranch,
			"error", err,
		)
	} else {
		result.TotalCommits = commitCount
	}

	changedFiles, err := s.deps.Branch.GetChangedFiles(ctx, worktreePath)
	if err != nil {
		s.log().Warn("failed to get changed files",
			"worktree_path", worktreePath,
			"error", err,
		)
	} else {
		result.FilesChanged = changedFiles
	}

	// Push branch
	if err := s.deps.Branch.Push(ctx, worktreePath, false); err != nil {
		result.Error = err
		return result, fmt.Errorf("failed to push branch: %w", err)
	}

	s.emit(consolidation.Event{
		Type:    consolidation.EventPRCreating,
		Message: "Creating consolidated PR",
	})

	// Build PR content
	prOpts := consolidation.PRBuildOptions{
		Mode:            consolidation.ModeSingle,
		GroupIndex:      0,
		TotalGroups:     1,
		Objective:       s.config.Objective,
		SynthesisNotes:  s.config.SynthesisNotes,
		Recommendations: s.config.Recommendations,
		FilesChanged:    result.FilesChanged,
		BaseBranch:      mainBranch,
		HeadBranch:      branchName,
	}

	prContent, err := s.deps.PRBuilder.Build(allTasks, prOpts)
	if err != nil {
		result.Error = err
		return result, fmt.Errorf("failed to build PR content: %w", err)
	}

	// Create PR
	prURL, err := s.deps.PRCreator.Create(ctx, prContent, s.config.CreateDraftPRs, s.config.PRLabels)
	if err != nil {
		result.Error = err
		return result, fmt.Errorf("failed to create PR: %w", err)
	}

	result.PRs = append(result.PRs, consolidation.PRInfo{
		URL:        prURL,
		Title:      "Consolidated PR",
		GroupIndex: 0,
	})

	s.emit(consolidation.Event{
		Type:    consolidation.EventPRCreated,
		Message: fmt.Sprintf("PR created: %s", prURL),
	})

	result.Duration = time.Since(startTime)

	s.emit(consolidation.Event{
		Type:    consolidation.EventComplete,
		Message: "Single PR consolidation complete",
	})

	return result, nil
}
