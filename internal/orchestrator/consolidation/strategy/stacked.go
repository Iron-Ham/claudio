package strategy

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
)

// Compile-time check that Stacked implements consolidation.Strategy.
var _ consolidation.Strategy = (*Stacked)(nil)

// Stacked implements the stacked PRs consolidation strategy.
// It creates one PR per execution group, where each group's PR
// is based on the previous group's branch.
type Stacked struct {
	*Base
}

// NewStacked creates a new Stacked strategy.
func NewStacked(deps Dependencies, config Config) *Stacked {
	return &Stacked{
		Base: NewBase(deps, config),
	}
}

// Name returns the strategy name.
func (s *Stacked) Name() string {
	return "stacked"
}

// SupportsParallel returns whether this strategy supports parallel task processing.
func (s *Stacked) SupportsParallel() bool {
	return false // Stacked strategy processes groups sequentially
}

// Execute runs the stacked consolidation strategy.
func (s *Stacked) Execute(ctx context.Context, groups []TaskGroup) (*Result, error) {
	startTime := time.Now()
	result := &Result{
		PRs:          make([]consolidation.PRInfo, 0, len(groups)),
		GroupResults: make([]GroupResult, 0, len(groups)),
	}

	s.emit(consolidation.Event{
		Type:    consolidation.EventStarted,
		Message: fmt.Sprintf("Starting stacked consolidation for %d groups", len(groups)),
	})

	mainBranch := s.deps.Branch.FindMainBranch(ctx)
	baseBranch := mainBranch
	allFiles := make(map[string]bool)

	for _, group := range groups {
		s.emit(consolidation.Event{
			Type:     consolidation.EventGroupStarted,
			GroupIdx: group.Index,
			Message:  fmt.Sprintf("Consolidating group %d with %d tasks", group.Index+1, len(group.Tasks)),
		})

		groupResult, err := s.consolidateGroup(ctx, group, baseBranch, mainBranch, len(groups))
		if err != nil {
			s.log().Error("failed to consolidate group",
				"group_index", group.Index,
				"error", err,
			)
			groupResult.Error = err.Error()
			groupResult.Success = false
		}

		result.GroupResults = append(result.GroupResults, *groupResult)
		result.TotalCommits += groupResult.CommitCount

		for _, f := range groupResult.FilesChanged {
			allFiles[f] = true
		}

		if groupResult.PRUrl != "" {
			result.PRs = append(result.PRs, consolidation.PRInfo{
				URL:        groupResult.PRUrl,
				Title:      fmt.Sprintf("Group %d", group.Index+1),
				GroupIndex: group.Index,
			})
		}

		// Next group will be based on this group's branch
		if groupResult.Success && groupResult.BranchName != "" {
			baseBranch = groupResult.BranchName
		}

		s.emit(consolidation.Event{
			Type:     consolidation.EventGroupComplete,
			GroupIdx: group.Index,
			Message:  fmt.Sprintf("Group %d consolidation complete", group.Index+1),
		})
	}

	// Collect all changed files
	for f := range allFiles {
		result.FilesChanged = append(result.FilesChanged, f)
	}

	result.Duration = time.Since(startTime)

	s.emit(consolidation.Event{
		Type:    consolidation.EventComplete,
		Message: fmt.Sprintf("Stacked consolidation complete: %d PRs created", len(result.PRs)),
	})

	return result, nil
}

// consolidateGroup consolidates a single execution group.
// The mainBranch parameter is reserved for future use when computing diffs.
func (s *Stacked) consolidateGroup(ctx context.Context, group TaskGroup, baseBranch, _ string, totalGroups int) (*GroupResult, error) {
	result := &GroupResult{
		GroupIndex: group.Index,
		TaskIDs:    make([]string, 0, len(group.Tasks)),
		Success:    true,
	}

	// Create branch for this group
	branchName, err := s.deps.Branch.CreateGroupBranch(ctx, group.Index, baseBranch)
	if err != nil {
		return result, fmt.Errorf("failed to create group branch: %w", err)
	}
	result.BranchName = branchName

	// Create temporary worktree for consolidation
	worktreePath := filepath.Join(s.config.WorktreeDir, fmt.Sprintf("consolidate-group-%d", group.Index))
	if err := s.deps.Branch.CreateWorktree(ctx, worktreePath, branchName); err != nil {
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

	// Cherry-pick commits from each task
	for _, task := range group.Tasks {
		s.emit(consolidation.Event{
			Type:     consolidation.EventTaskMerging,
			GroupIdx: group.Index,
			TaskID:   task.ID,
			Message:  fmt.Sprintf("Merging task %s", task.ID),
		})

		conflictInfo, err := s.deps.Conflict.CheckCherryPick(ctx, worktreePath, task.Branch, task.ID, task.Title)
		if err != nil {
			return result, fmt.Errorf("cherry-pick check failed for task %s: %w", task.ID, err)
		}

		if conflictInfo != nil {
			// Conflict detected
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
			return result, fmt.Errorf("merge conflict in task %s: files %v", task.ID, conflictInfo.Files)
		}

		result.TaskIDs = append(result.TaskIDs, task.ID)

		s.emit(consolidation.Event{
			Type:     consolidation.EventTaskMerged,
			GroupIdx: group.Index,
			TaskID:   task.ID,
			Message:  fmt.Sprintf("Task %s merged successfully", task.ID),
		})
	}

	// Get commit count and changed files
	commitCount, err := s.deps.Branch.CountCommitsBetween(ctx, worktreePath, baseBranch, "HEAD")
	if err != nil {
		s.log().Warn("failed to count commits",
			"worktree_path", worktreePath,
			"base_branch", baseBranch,
			"error", err,
		)
	} else {
		result.CommitCount = commitCount
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
		return result, fmt.Errorf("failed to push branch: %w", err)
	}

	s.emit(consolidation.Event{
		Type:     consolidation.EventPRCreating,
		GroupIdx: group.Index,
		Message:  fmt.Sprintf("Creating PR for group %d", group.Index+1),
	})

	// Build PR content
	prOpts := consolidation.PRBuildOptions{
		Mode:            consolidation.ModeStacked,
		GroupIndex:      group.Index,
		TotalGroups:     totalGroups,
		Objective:       s.config.Objective,
		SynthesisNotes:  s.config.SynthesisNotes,
		Recommendations: s.config.Recommendations,
		FilesChanged:    result.FilesChanged,
		BaseBranch:      baseBranch,
		HeadBranch:      branchName,
	}

	prContent, err := s.deps.PRBuilder.Build(group.Tasks, prOpts)
	if err != nil {
		return result, fmt.Errorf("failed to build PR content: %w", err)
	}

	// Create PR
	prURL, err := s.deps.PRCreator.Create(ctx, prContent, s.config.CreateDraftPRs, s.config.PRLabels)
	if err != nil {
		return result, fmt.Errorf("failed to create PR: %w", err)
	}

	result.PRUrl = prURL

	s.emit(consolidation.Event{
		Type:     consolidation.EventPRCreated,
		GroupIdx: group.Index,
		Message:  fmt.Sprintf("PR created: %s", prURL),
	})

	return result, nil
}
