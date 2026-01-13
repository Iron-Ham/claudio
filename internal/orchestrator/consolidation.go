package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/pr"
	"github.com/Iron-Ham/claudio/internal/worktree"
)

// ConsolidationMode defines how work is consolidated after ultraplan execution
type ConsolidationMode string

const (
	// ModeStackedPRs creates one PR per execution group, stacked on each other
	ModeStackedPRs ConsolidationMode = "stacked"
	// ModeSinglePR consolidates all work into a single PR
	ModeSinglePR ConsolidationMode = "single"
)

// ConsolidationPhase represents sub-phases within consolidation
type ConsolidationPhase string

const (
	ConsolidationIdle             ConsolidationPhase = "idle"
	ConsolidationDetecting        ConsolidationPhase = "detecting_conflicts"
	ConsolidationCreatingBranches ConsolidationPhase = "creating_branches"
	ConsolidationMergingTasks     ConsolidationPhase = "merging_tasks"
	ConsolidationPushing          ConsolidationPhase = "pushing"
	ConsolidationCreatingPRs      ConsolidationPhase = "creating_prs"
	ConsolidationPaused           ConsolidationPhase = "paused"
	ConsolidationComplete         ConsolidationPhase = "complete"
	ConsolidationFailed           ConsolidationPhase = "failed"
)

// ConsolidationState tracks the progress of consolidation
type ConsolidationState struct {
	Phase            ConsolidationPhase `json:"phase"`
	CurrentGroup     int                `json:"current_group"`
	TotalGroups      int                `json:"total_groups"`
	CurrentTask      string             `json:"current_task,omitempty"`
	GroupBranches    []string           `json:"group_branches"`
	PRUrls           []string           `json:"pr_urls"`
	ConflictFiles    []string           `json:"conflict_files,omitempty"`
	ConflictTaskID   string             `json:"conflict_task_id,omitempty"`
	ConflictWorktree string             `json:"conflict_worktree,omitempty"`
	Error            string             `json:"error,omitempty"`
	StartedAt        *time.Time         `json:"started_at,omitempty"`
	CompletedAt      *time.Time         `json:"completed_at,omitempty"`
}

// HasConflict returns true if consolidation is paused due to a conflict.
func (s *ConsolidationState) HasConflict() bool {
	return s.Phase == ConsolidationPaused && len(s.ConflictFiles) > 0
}

// GroupConsolidationResult holds the result of consolidating one group
type GroupConsolidationResult struct {
	GroupIndex   int      `json:"group_index"`
	TaskIDs      []string `json:"task_ids"`
	BranchName   string   `json:"branch_name"`
	CommitCount  int      `json:"commit_count"`
	FilesChanged []string `json:"files_changed"`
	PRUrl        string   `json:"pr_url,omitempty"`
	Success      bool     `json:"success"`
	Error        string   `json:"error,omitempty"`
}

// ConsolidationConfig holds configuration for branch consolidation
type ConsolidationConfig struct {
	Mode           ConsolidationMode
	BranchPrefix   string
	CreateDraftPRs bool
	PRLabels       []string
}

// ConsolidationEventType represents events during consolidation
type ConsolidationEventType string

const (
	EventConsolidationStarted       ConsolidationEventType = "consolidation_started"
	EventConsolidationGroupStarted  ConsolidationEventType = "consolidation_group_started"
	EventConsolidationTaskMerging   ConsolidationEventType = "consolidation_task_merging"
	EventConsolidationTaskMerged    ConsolidationEventType = "consolidation_task_merged"
	EventConsolidationGroupComplete ConsolidationEventType = "consolidation_group_complete"
	EventConsolidationPRCreating    ConsolidationEventType = "consolidation_pr_creating"
	EventConsolidationPRCreated     ConsolidationEventType = "consolidation_pr_created"
	EventConsolidationConflict      ConsolidationEventType = "consolidation_conflict"
	EventConsolidationComplete      ConsolidationEventType = "consolidation_complete"
	EventConsolidationFailed        ConsolidationEventType = "consolidation_failed"
)

// ConsolidationEvent represents an event during consolidation
type ConsolidationEvent struct {
	Type      ConsolidationEventType `json:"type"`
	GroupIdx  int                    `json:"group_idx,omitempty"`
	TaskID    string                 `json:"task_id,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// Consolidator handles the consolidation of task branches into group branches and PR creation
type Consolidator struct {
	orch        *Orchestrator
	session     *UltraPlanSession
	baseSession *Session
	config      ConsolidationConfig
	wt          *worktree.Manager
	state       *ConsolidationState
	results     []*GroupConsolidationResult
	logger      *logging.Logger

	// Task branch mapping: task ID -> branch name
	taskBranches map[string]string

	// Event handling
	eventCallback func(ConsolidationEvent)

	// Synchronization
	mu       sync.RWMutex
	stopChan chan struct{}
	stopped  bool
}

// NewConsolidator creates a new consolidator.
// If logger is nil, a NopLogger will be used.
func NewConsolidator(orch *Orchestrator, session *UltraPlanSession, baseSession *Session, config ConsolidationConfig, logger *logging.Logger) *Consolidator {
	if logger == nil {
		logger = logging.NopLogger()
	}
	// Create a child logger with the consolidation phase context
	logger = logger.WithPhase("consolidation")

	return &Consolidator{
		orch:         orch,
		session:      session,
		baseSession:  baseSession,
		config:       config,
		wt:           orch.wt,
		state:        &ConsolidationState{Phase: ConsolidationIdle},
		results:      make([]*GroupConsolidationResult, 0),
		taskBranches: make(map[string]string),
		stopChan:     make(chan struct{}),
		logger:       logger,
	}
}

// SetEventCallback sets the callback for consolidation events
func (c *Consolidator) SetEventCallback(cb func(ConsolidationEvent)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventCallback = cb
}

// State returns a copy of the current consolidation state
func (c *Consolidator) State() *ConsolidationState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Return a copy
	stateCopy := *c.state
	return &stateCopy
}

// setPhase updates the consolidation phase with logging
func (c *Consolidator) setPhase(phase ConsolidationPhase, groupIdx int) {
	c.mu.Lock()
	oldPhase := c.state.Phase
	c.state.Phase = phase
	c.state.CurrentGroup = groupIdx
	c.mu.Unlock()

	c.logger.Info("consolidation phase changed",
		"old_phase", string(oldPhase),
		"phase", string(phase),
		"group_index", groupIdx,
	)
}

// Results returns the consolidation results
func (c *Consolidator) Results() []*GroupConsolidationResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.results
}

// emitEvent sends an event to the callback
func (c *Consolidator) emitEvent(event ConsolidationEvent) {
	event.Timestamp = time.Now()

	c.mu.RLock()
	cb := c.eventCallback
	c.mu.RUnlock()

	if cb != nil {
		cb(event)
	}
}

// Run executes the full consolidation process
func (c *Consolidator) Run() error {
	c.mu.Lock()
	now := time.Now()
	c.state.StartedAt = &now
	c.state.TotalGroups = len(c.session.Plan.ExecutionOrder)
	c.mu.Unlock()

	c.logger.Info("consolidation started",
		"total_groups", c.state.TotalGroups,
		"mode", string(c.config.Mode),
		"session_id", c.session.ID,
	)

	c.emitEvent(ConsolidationEvent{
		Type:    EventConsolidationStarted,
		Message: fmt.Sprintf("Consolidating %d groups", c.state.TotalGroups),
	})

	// Build task branch mapping from instances
	c.logger.Debug("building task branch mapping",
		"completed_tasks", len(c.session.CompletedTasks),
	)
	if err := c.buildTaskBranchMapping(); err != nil {
		return c.fail(fmt.Errorf("failed to build task branch mapping: %w", err))
	}
	c.logger.Debug("task branch mapping built",
		"mapped_tasks", len(c.taskBranches),
	)

	// Run the appropriate mode
	var err error
	switch c.config.Mode {
	case ModeStackedPRs:
		err = c.runStackedMode()
	case ModeSinglePR:
		err = c.runSingleMode()
	default:
		err = c.runStackedMode() // Default to stacked
	}

	if err != nil {
		return err
	}

	// Create PRs
	if err := c.createPRs(); err != nil {
		return c.fail(fmt.Errorf("failed to create PRs: %w", err))
	}

	// Mark complete
	c.mu.Lock()
	completedAt := time.Now()
	c.state.CompletedAt = &completedAt
	c.state.Phase = ConsolidationComplete
	c.mu.Unlock()

	c.logger.Info("consolidation completed",
		"pr_count", len(c.state.PRUrls),
		"group_branches", len(c.state.GroupBranches),
	)

	c.emitEvent(ConsolidationEvent{
		Type:    EventConsolidationComplete,
		Message: fmt.Sprintf("Created %d PRs", len(c.state.PRUrls)),
	})

	return nil
}

// buildTaskBranchMapping builds a mapping from task IDs to their branch names
func (c *Consolidator) buildTaskBranchMapping() error {
	// Find instances for completed tasks
	for _, taskID := range c.session.CompletedTasks {
		// Find the instance that executed this task
		for _, inst := range c.baseSession.Instances {
			// Match instance to task via the task description or instance ID
			// The instance.Task contains the full task prompt which includes the task ID
			if strings.Contains(inst.Task, taskID) || c.matchTaskToInstance(taskID, inst) {
				c.taskBranches[taskID] = inst.Branch
				break
			}
		}
	}

	// Verify we found branches for all completed tasks
	for _, taskID := range c.session.CompletedTasks {
		if _, ok := c.taskBranches[taskID]; !ok {
			// Try to find by looking at task-instance mapping history in the session
			// The instance might have already been removed from TaskToInstance
			return fmt.Errorf("could not find branch for completed task %s", taskID)
		}
	}

	return nil
}

// matchTaskToInstance attempts to match a task ID to an instance
func (c *Consolidator) matchTaskToInstance(taskID string, inst *Instance) bool {
	task := c.session.GetTask(taskID)
	if task == nil {
		return false
	}

	// Check if the instance branch contains a slug version of the task title
	titleSlug := slugify(task.Title)
	return strings.Contains(inst.Branch, titleSlug)
}

// runStackedMode consolidates work into stacked branches (one per group)
// With per-group consolidators, branches are already created and stable.
// This function now just uses the pre-consolidated branches for PR creation.
func (c *Consolidator) runStackedMode() error {
	// Check if we have pre-consolidated branches from per-group consolidators
	if len(c.session.GroupConsolidatedBranches) > 0 {
		return c.runStackedModeFromPreconsolidated()
	}

	// Fallback to original behavior if no pre-consolidated branches
	return c.runStackedModeOriginal()
}

// runStackedModeFromPreconsolidated uses pre-consolidated branches from per-group consolidators
func (c *Consolidator) runStackedModeFromPreconsolidated() error {
	c.setPhase(ConsolidationCreatingBranches, 0)

	// Use the already-consolidated branches
	for groupIdx, branch := range c.session.GroupConsolidatedBranches {
		if branch == "" {
			c.logger.Debug("skipping empty consolidated branch",
				"group_index", groupIdx,
			)
			continue // Skip groups that weren't consolidated
		}

		c.logger.Info("using pre-consolidated branch",
			"branch_name", branch,
			"group_index", groupIdx,
		)

		c.emitEvent(ConsolidationEvent{
			Type:     EventConsolidationGroupStarted,
			GroupIdx: groupIdx,
			Message:  fmt.Sprintf("Using pre-consolidated branch %s", branch),
		})

		// Get task IDs for this group
		var taskIDs []string
		if groupIdx < len(c.session.Plan.ExecutionOrder) {
			taskIDs = c.session.Plan.ExecutionOrder[groupIdx]
		}

		// Build result from pre-consolidated context if available
		var filesChanged []string
		if groupIdx < len(c.session.GroupConsolidationContexts) && c.session.GroupConsolidationContexts[groupIdx] != nil {
			ctx := c.session.GroupConsolidationContexts[groupIdx]
			// Gather files from aggregated context
			if ctx.AggregatedContext != nil {
				for taskID := range ctx.AggregatedContext.TaskSummaries {
					// Find task worktree to get files
					for _, inst := range c.baseSession.Instances {
						if strings.Contains(inst.Task, taskID) {
							// Read completion file to get files modified
							if completion, err := ParseTaskCompletionFile(inst.WorktreePath); err == nil {
								filesChanged = append(filesChanged, completion.FilesModified...)
							}
							break
						}
					}
				}
			}
		}

		result := &GroupConsolidationResult{
			GroupIndex:   groupIdx,
			TaskIDs:      taskIDs,
			BranchName:   branch,
			CommitCount:  0, // Not tracked in preconsolidated mode
			FilesChanged: deduplicateStrings(filesChanged),
			Success:      true,
		}

		c.mu.Lock()
		c.results = append(c.results, result)
		c.state.GroupBranches = append(c.state.GroupBranches, branch)
		c.mu.Unlock()

		c.logger.Info("consolidated branch created",
			"branch_name", branch,
			"group_index", groupIdx,
			"task_count", len(taskIDs),
			"files_changed", len(result.FilesChanged),
		)

		c.emitEvent(ConsolidationEvent{
			Type:     EventConsolidationGroupComplete,
			GroupIdx: groupIdx,
			Message:  fmt.Sprintf("Group %d ready with branch %s", groupIdx+1, branch),
		})
	}

	return nil
}

// runStackedModeOriginal is the original implementation for when no pre-consolidated branches exist
func (c *Consolidator) runStackedModeOriginal() error {
	mainBranch := c.wt.FindMainBranch()
	worktreeBase := filepath.Join(c.orch.claudioDir, "consolidation")

	c.logger.Debug("git command executed",
		"command", "find main branch",
		"output", mainBranch,
	)

	// Ensure consolidation worktree directory exists
	if err := os.MkdirAll(worktreeBase, 0755); err != nil {
		return fmt.Errorf("failed to create consolidation directory: %w", err)
	}

	for groupIdx, taskIDs := range c.session.Plan.ExecutionOrder {
		c.setPhase(ConsolidationCreatingBranches, groupIdx)

		// Determine base branch
		var baseBranch string
		if groupIdx == 0 {
			baseBranch = mainBranch
		} else {
			baseBranch = c.state.GroupBranches[groupIdx-1]
		}

		// Generate group branch name
		groupBranch := c.generateGroupBranchName(groupIdx)

		c.logger.Debug("creating branch from base",
			"group_branch", groupBranch,
			"base_branch", baseBranch,
			"group_index", groupIdx,
		)

		c.emitEvent(ConsolidationEvent{
			Type:     EventConsolidationGroupStarted,
			GroupIdx: groupIdx,
			Message:  fmt.Sprintf("Creating branch %s from %s", groupBranch, baseBranch),
		})

		// Create branch from base
		if err := c.wt.CreateBranchFrom(groupBranch, baseBranch); err != nil {
			c.logger.Error("git command failed",
				"command", "create branch",
				"branch", groupBranch,
				"base", baseBranch,
				"error", err.Error(),
			)
			return c.fail(fmt.Errorf("failed to create branch %s: %w", groupBranch, err))
		}

		c.logger.Debug("git command executed",
			"command", "create branch",
			"output", groupBranch,
		)

		// Create worktree for this group
		groupWorktree := filepath.Join(worktreeBase, fmt.Sprintf("group-%d", groupIdx+1))
		if err := c.wt.CreateWorktreeFromBranch(groupWorktree, groupBranch); err != nil {
			c.logger.Error("git command failed",
				"command", "create worktree",
				"worktree", groupWorktree,
				"branch", groupBranch,
				"error", err.Error(),
			)
			return c.fail(fmt.Errorf("failed to create worktree for group %d: %w", groupIdx, err))
		}

		c.logger.Debug("git command executed",
			"command", "create worktree",
			"output", truncateString(groupWorktree, 100),
		)

		// Consolidate tasks into this group
		result, err := c.consolidateGroup(groupIdx, taskIDs, groupWorktree)
		if err != nil {
			// Check if it's a conflict error
			if conflictErr, ok := err.(*ConflictError); ok {
				c.handleConflict(groupIdx, conflictErr)
				return err
			}
			return c.fail(err)
		}

		c.mu.Lock()
		c.results = append(c.results, result)
		c.state.GroupBranches = append(c.state.GroupBranches, groupBranch)
		c.mu.Unlock()

		c.logger.Info("consolidated branch created",
			"branch_name", groupBranch,
			"group_index", groupIdx,
			"task_count", len(taskIDs),
			"commit_count", result.CommitCount,
			"files_changed", len(result.FilesChanged),
		)

		// Push the group branch
		c.setPhase(ConsolidationPushing, groupIdx)

		c.logger.Debug("pushing branch to remote",
			"branch", groupBranch,
			"group_index", groupIdx,
		)

		if err := c.wt.Push(groupWorktree, false); err != nil {
			c.logger.Error("git command failed",
				"command", "push",
				"branch", groupBranch,
				"error", err.Error(),
			)
			return c.fail(fmt.Errorf("failed to push branch %s: %w", groupBranch, err))
		}

		c.logger.Debug("git command executed",
			"command", "push",
			"output", "success",
		)

		// Clean up worktree (but keep the branch)
		_ = c.wt.Remove(groupWorktree)

		c.emitEvent(ConsolidationEvent{
			Type:     EventConsolidationGroupComplete,
			GroupIdx: groupIdx,
			Message:  fmt.Sprintf("Group %d consolidated with %d tasks", groupIdx+1, len(taskIDs)),
		})
	}

	return nil
}

// runSingleMode consolidates all work into a single branch
func (c *Consolidator) runSingleMode() error {
	mainBranch := c.wt.FindMainBranch()
	worktreeBase := filepath.Join(c.orch.claudioDir, "consolidation")

	c.logger.Debug("git command executed",
		"command", "find main branch",
		"output", mainBranch,
	)

	if err := os.MkdirAll(worktreeBase, 0755); err != nil {
		return fmt.Errorf("failed to create consolidation directory: %w", err)
	}

	// Create single consolidated branch
	consolidatedBranch := c.generateSingleBranchName()

	c.setPhase(ConsolidationCreatingBranches, 0)

	c.logger.Debug("creating consolidated branch",
		"branch", consolidatedBranch,
		"base", mainBranch,
	)

	c.emitEvent(ConsolidationEvent{
		Type:    EventConsolidationStarted,
		Message: fmt.Sprintf("Creating consolidated branch %s", consolidatedBranch),
	})

	if err := c.wt.CreateBranchFrom(consolidatedBranch, mainBranch); err != nil {
		c.logger.Error("git command failed",
			"command", "create branch",
			"branch", consolidatedBranch,
			"base", mainBranch,
			"error", err.Error(),
		)
		return c.fail(fmt.Errorf("failed to create branch %s: %w", consolidatedBranch, err))
	}

	c.logger.Debug("git command executed",
		"command", "create branch",
		"output", consolidatedBranch,
	)

	// Create worktree
	consolidatedWorktree := filepath.Join(worktreeBase, "consolidated")
	if err := c.wt.CreateWorktreeFromBranch(consolidatedWorktree, consolidatedBranch); err != nil {
		c.logger.Error("git command failed",
			"command", "create worktree",
			"worktree", consolidatedWorktree,
			"branch", consolidatedBranch,
			"error", err.Error(),
		)
		return c.fail(fmt.Errorf("failed to create worktree: %w", err))
	}

	c.logger.Debug("git command executed",
		"command", "create worktree",
		"output", truncateString(consolidatedWorktree, 100),
	)

	// Merge all tasks from all groups in order
	allTaskIDs := make([]string, 0)
	for _, group := range c.session.Plan.ExecutionOrder {
		allTaskIDs = append(allTaskIDs, group...)
	}

	c.logger.Debug("consolidating all tasks",
		"total_tasks", len(allTaskIDs),
	)

	result, err := c.consolidateGroup(0, allTaskIDs, consolidatedWorktree)
	if err != nil {
		if conflictErr, ok := err.(*ConflictError); ok {
			c.handleConflict(0, conflictErr)
			return err
		}
		return c.fail(err)
	}

	c.mu.Lock()
	c.results = append(c.results, result)
	c.state.GroupBranches = append(c.state.GroupBranches, consolidatedBranch)
	c.mu.Unlock()

	c.logger.Info("consolidated branch created",
		"branch_name", consolidatedBranch,
		"group_index", 0,
		"task_count", len(allTaskIDs),
		"commit_count", result.CommitCount,
		"files_changed", len(result.FilesChanged),
	)

	// Push
	c.setPhase(ConsolidationPushing, 0)

	c.logger.Debug("pushing branch to remote",
		"branch", consolidatedBranch,
	)

	if err := c.wt.Push(consolidatedWorktree, false); err != nil {
		c.logger.Error("git command failed",
			"command", "push",
			"branch", consolidatedBranch,
			"error", err.Error(),
		)
		return c.fail(fmt.Errorf("failed to push branch: %w", err))
	}

	c.logger.Debug("git command executed",
		"command", "push",
		"output", "success",
	)

	// Clean up
	_ = c.wt.Remove(consolidatedWorktree)

	return nil
}

// consolidateGroup merges all tasks in a group into the group worktree
func (c *Consolidator) consolidateGroup(groupIdx int, taskIDs []string, groupWorktree string) (*GroupConsolidationResult, error) {
	c.setPhase(ConsolidationMergingTasks, groupIdx)

	c.logger.Debug("starting group consolidation",
		"group_index", groupIdx,
		"task_count", len(taskIDs),
		"worktree", truncateString(groupWorktree, 80),
	)

	result := &GroupConsolidationResult{
		GroupIndex: groupIdx,
		TaskIDs:    taskIDs,
		Success:    false,
	}

	// Filter out tasks with no commits
	activeTasks := make([]string, 0)
	for _, taskID := range taskIDs {
		branch := c.taskBranches[taskID]
		if branch == "" {
			c.logger.Debug("skipping task without branch",
				"task_id", taskID,
				"group_index", groupIdx,
			)
			continue // Task has no branch (should not happen if buildTaskBranchMapping worked)
		}
		activeTasks = append(activeTasks, taskID)
	}

	// CHANGED: Return error when no active tasks instead of silent success
	if len(activeTasks) == 0 {
		c.logger.Error("no active tasks found",
			"group_index", groupIdx,
			"total_tasks", len(taskIDs),
		)
		return result, fmt.Errorf("no task branches with commits found for group %d", groupIdx)
	}

	// Get base branch for commit counting
	baseBranch := c.wt.FindMainBranch()

	// Track initial commit count
	initialCommits, _ := c.wt.CountCommitsBetween(groupWorktree, baseBranch, "HEAD")

	// Cherry-pick each task's commits
	for _, taskID := range activeTasks {
		c.mu.Lock()
		c.state.CurrentTask = taskID
		c.mu.Unlock()

		branch := c.taskBranches[taskID]

		c.logger.Debug("git command executed",
			"command", "cherry-pick start",
			"task_id", taskID,
			"branch", branch,
		)

		c.emitEvent(ConsolidationEvent{
			Type:     EventConsolidationTaskMerging,
			GroupIdx: groupIdx,
			TaskID:   taskID,
			Message:  fmt.Sprintf("Cherry-picking from %s", branch),
		})

		// Cherry-pick the task branch
		err := c.wt.CherryPickBranch(groupWorktree, branch)
		if err != nil {
			// Check for conflict
			if conflictErr, ok := err.(*worktree.CherryPickConflictError); ok {
				files, _ := c.wt.GetConflictingFiles(groupWorktree)
				c.logger.Debug("conflict detection check",
					"task_id", taskID,
					"branch", branch,
					"conflict_detected", true,
					"conflict_files", len(files),
				)
				return result, &ConflictError{
					TaskID:       taskID,
					Branch:       branch,
					Files:        files,
					WorktreePath: groupWorktree,
					Underlying:   conflictErr,
				}
			}
			c.logger.Error("git command failed",
				"command", "cherry-pick",
				"task_id", taskID,
				"branch", branch,
				"error", err.Error(),
			)
			return result, fmt.Errorf("failed to cherry-pick task %s: %w", taskID, err)
		}

		c.logger.Info("task branch merged",
			"task_id", taskID,
			"target_branch", groupWorktree,
			"source_branch", branch,
		)

		c.emitEvent(ConsolidationEvent{
			Type:     EventConsolidationTaskMerged,
			GroupIdx: groupIdx,
			TaskID:   taskID,
			Message:  fmt.Sprintf("Merged task %s", taskID),
		})
	}

	// Verify commits were actually added
	finalCommits, _ := c.wt.CountCommitsBetween(groupWorktree, baseBranch, "HEAD")
	addedCommits := finalCommits - initialCommits

	// CHANGED: Fail if no commits were added despite cherry-picking
	if addedCommits == 0 && len(activeTasks) > 0 {
		return result, fmt.Errorf("cherry-picked %d branches but no commits were added - task branches may have been empty", len(activeTasks))
	}

	// Get changed files for the result
	files, _ := c.wt.GetChangedFiles(groupWorktree)
	result.FilesChanged = files
	result.CommitCount = addedCommits
	result.Success = true

	c.mu.Lock()
	c.state.CurrentTask = ""
	c.mu.Unlock()

	return result, nil
}

// createPRs creates pull requests for all group branches
func (c *Consolidator) createPRs() error {
	c.setPhase(ConsolidationCreatingPRs, 0)

	mainBranch := c.wt.FindMainBranch()

	c.logger.Debug("creating PRs for group branches",
		"total_branches", len(c.state.GroupBranches),
		"main_branch", mainBranch,
	)

	// Create PRs in reverse order (so base branches exist)
	for i := len(c.state.GroupBranches) - 1; i >= 0; i-- {
		branch := c.state.GroupBranches[i]

		// Determine base branch for PR
		var baseBranch string
		if c.config.Mode == ModeSinglePR || i == 0 {
			baseBranch = mainBranch
		} else {
			baseBranch = c.state.GroupBranches[i-1]
		}

		c.logger.Debug("creating PR",
			"branch", branch,
			"base_branch", baseBranch,
			"group_index", i,
			"is_draft", c.config.CreateDraftPRs,
		)

		c.emitEvent(ConsolidationEvent{
			Type:     EventConsolidationPRCreating,
			GroupIdx: i,
			Message:  fmt.Sprintf("Creating PR for %s -> %s", branch, baseBranch),
		})

		// Build PR content
		content := c.buildPRContent(i)

		// Create the PR
		prURL, err := pr.CreateStackedPR(pr.PROptions{
			Title:  content.Title,
			Body:   content.Body,
			Branch: branch,
			Draft:  c.config.CreateDraftPRs,
			Labels: c.config.PRLabels,
		}, baseBranch)

		if err != nil {
			c.logger.Error("PR creation failed",
				"branch", branch,
				"base_branch", baseBranch,
				"group_index", i,
				"error", err.Error(),
			)
			return fmt.Errorf("failed to create PR for group %d: %w", i+1, err)
		}

		c.mu.Lock()
		c.state.PRUrls = append([]string{prURL}, c.state.PRUrls...) // Prepend to maintain order
		if i < len(c.results) {
			c.results[i].PRUrl = prURL
		}
		c.mu.Unlock()

		c.logger.Info("PR created",
			"pr_url", prURL,
			"group_index", i,
			"branch", branch,
			"base_branch", baseBranch,
		)

		c.emitEvent(ConsolidationEvent{
			Type:     EventConsolidationPRCreated,
			GroupIdx: i,
			Message:  prURL,
		})
	}

	return nil
}

// buildPRContent generates title and body for a group PR
func (c *Consolidator) buildPRContent(groupIdx int) *PRContent {
	var title string
	var body strings.Builder

	if c.config.Mode == ModeSinglePR {
		title = fmt.Sprintf("ultraplan: %s", truncateString(c.session.Objective, 50))
	} else {
		title = fmt.Sprintf("ultraplan: group %d - %s", groupIdx+1, truncateString(c.session.Objective, 40))
	}

	// Body
	body.WriteString("## Ultraplan Consolidation\n\n")
	body.WriteString(fmt.Sprintf("**Objective**: %s\n\n", c.session.Objective))

	if c.config.Mode == ModeStackedPRs {
		body.WriteString(fmt.Sprintf("**Group**: %d of %d\n\n", groupIdx+1, c.state.TotalGroups))

		if groupIdx > 0 {
			body.WriteString(fmt.Sprintf("**Base**: Group %d\n\n", groupIdx))
		}
		if groupIdx < c.state.TotalGroups-1 {
			body.WriteString(fmt.Sprintf("> **Note**: This PR must be merged before Group %d.\n\n", groupIdx+2))
		}
	}

	// Tasks included with summaries from completion files
	body.WriteString("## Tasks Included\n\n")
	var taskIDs []string
	if groupIdx < len(c.session.Plan.ExecutionOrder) {
		taskIDs = c.session.Plan.ExecutionOrder[groupIdx]
	} else if c.config.Mode == ModeSinglePR {
		// Single mode: all tasks
		for _, group := range c.session.Plan.ExecutionOrder {
			taskIDs = append(taskIDs, group...)
		}
	}

	// Gather completion context for these tasks
	taskContext := c.gatherTaskCompletionContext(taskIDs)

	for _, taskID := range taskIDs {
		task := c.session.GetTask(taskID)
		if task != nil {
			taskLine := fmt.Sprintf("- **%s**: %s", task.ID, task.Title)
			// Add summary from completion file if available
			if summary, ok := taskContext.TaskSummaries[taskID]; ok && summary != "" {
				taskLine += fmt.Sprintf("\n  - %s", summary)
			}
			body.WriteString(taskLine + "\n")
		}
	}

	// Files changed
	if groupIdx < len(c.results) && len(c.results[groupIdx].FilesChanged) > 0 {
		body.WriteString("\n## Files Changed\n\n")
		for _, f := range c.results[groupIdx].FilesChanged {
			body.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}

	// Add aggregated context from task completion files
	if taskContext.HasContent() {
		body.WriteString(taskContext.FormatForPR())
	}

	// Add synthesis review context if available
	if c.session.SynthesisCompletion != nil {
		synth := c.session.SynthesisCompletion
		if synth.IntegrationNotes != "" || len(synth.Recommendations) > 0 {
			body.WriteString("\n## Synthesis Review Notes\n\n")
			if synth.IntegrationNotes != "" {
				body.WriteString(fmt.Sprintf("**Integration Notes**: %s\n\n", synth.IntegrationNotes))
			}
			if len(synth.Recommendations) > 0 {
				body.WriteString("**Recommendations**:\n")
				for _, rec := range synth.Recommendations {
					body.WriteString(fmt.Sprintf("- %s\n", rec))
				}
			}
		}
	}

	return &PRContent{Title: title, Body: body.String()}
}

// PRContent holds PR title and body
type PRContent struct {
	Title string
	Body  string
}

// generateGroupBranchName generates a branch name for a group
func (c *Consolidator) generateGroupBranchName(groupIdx int) string {
	prefix := c.config.BranchPrefix
	if prefix == "" {
		prefix = "Iron-Ham"
	}
	planID := c.session.ID
	if len(planID) > 8 {
		planID = planID[:8]
	}
	return fmt.Sprintf("%s/ultraplan-%s-group-%d", prefix, planID, groupIdx+1)
}

// generateSingleBranchName generates a branch name for single PR mode
func (c *Consolidator) generateSingleBranchName() string {
	prefix := c.config.BranchPrefix
	if prefix == "" {
		prefix = "Iron-Ham"
	}
	planID := c.session.ID
	if len(planID) > 8 {
		planID = planID[:8]
	}
	return fmt.Sprintf("%s/ultraplan-%s", prefix, planID)
}

// handleConflict handles a conflict during consolidation
func (c *Consolidator) handleConflict(groupIdx int, conflictErr *ConflictError) {
	c.mu.Lock()
	c.state.Phase = ConsolidationPaused
	c.state.ConflictFiles = conflictErr.Files
	c.state.ConflictTaskID = conflictErr.TaskID
	c.state.ConflictWorktree = conflictErr.WorktreePath
	c.mu.Unlock()

	c.logger.Warn("merge conflict detected",
		"conflict_files", strings.Join(conflictErr.Files, ", "),
		"task_id", conflictErr.TaskID,
		"branch", conflictErr.Branch,
		"group_index", groupIdx,
	)

	c.logger.Warn("consolidation paused",
		"reason", "merge conflict requires manual resolution",
		"worktree", conflictErr.WorktreePath,
	)

	c.emitEvent(ConsolidationEvent{
		Type:     EventConsolidationConflict,
		GroupIdx: groupIdx,
		TaskID:   conflictErr.TaskID,
		Message:  fmt.Sprintf("Conflict in files: %s", strings.Join(conflictErr.Files, ", ")),
	})
}

// fail marks the consolidation as failed
func (c *Consolidator) fail(err error) error {
	c.mu.Lock()
	c.state.Phase = ConsolidationFailed
	c.state.Error = err.Error()
	currentGroup := c.state.CurrentGroup
	c.mu.Unlock()

	c.logger.Error("consolidation failed",
		"error", err.Error(),
		"group_index", currentGroup,
	)

	c.emitEvent(ConsolidationEvent{
		Type:    EventConsolidationFailed,
		Message: err.Error(),
	})

	return err
}

// Stop stops the consolidation process
func (c *Consolidator) Stop() {
	c.mu.Lock()
	if !c.stopped {
		c.stopped = true
		close(c.stopChan)
	}
	c.mu.Unlock()
}

// Resume resumes consolidation after conflict resolution
func (c *Consolidator) Resume() error {
	c.mu.Lock()
	if c.state.Phase != ConsolidationPaused {
		c.mu.Unlock()
		return fmt.Errorf("consolidation is not paused")
	}

	worktree := c.state.ConflictWorktree
	c.state.Phase = ConsolidationMergingTasks
	c.state.ConflictFiles = nil
	c.state.ConflictTaskID = ""
	c.mu.Unlock()

	// Continue the cherry-pick
	if err := c.wt.ContinueCherryPick(worktree); err != nil {
		return c.fail(fmt.Errorf("failed to continue cherry-pick: %w", err))
	}

	// Continue with remaining work
	// This is simplified - a full implementation would track where we left off
	return nil
}

// ConflictError represents a conflict during consolidation
type ConflictError struct {
	TaskID       string
	Branch       string
	Files        []string
	WorktreePath string
	Underlying   error
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict merging task %s from branch %s: %v", e.TaskID, e.Branch, e.Underlying)
}

// gatherTaskCompletionContext reads completion files from all completed task worktrees
// and aggregates the context for use in consolidation prompts and PR descriptions
func (c *Consolidator) gatherTaskCompletionContext(taskIDs []string) *AggregatedTaskContext {
	context := &AggregatedTaskContext{
		TaskSummaries:  make(map[string]string),
		AllIssues:      make([]string, 0),
		AllSuggestions: make([]string, 0),
		Dependencies:   make([]string, 0),
	}

	seenDeps := make(map[string]bool)

	for _, taskID := range taskIDs {
		// Find the instance for this task
		var inst *Instance
		for _, i := range c.baseSession.Instances {
			if strings.Contains(i.Task, taskID) || c.matchTaskToInstance(taskID, i) {
				inst = i
				break
			}
		}
		if inst == nil || inst.WorktreePath == "" {
			continue
		}

		// Try to read the completion file
		completion, err := ParseTaskCompletionFile(inst.WorktreePath)
		if err != nil {
			continue // No completion file or invalid
		}

		// Store task summary
		context.TaskSummaries[taskID] = completion.Summary

		// Aggregate issues (prefix with task ID for context)
		for _, issue := range completion.Issues {
			if issue != "" {
				context.AllIssues = append(context.AllIssues, fmt.Sprintf("[%s] %s", taskID, issue))
			}
		}

		// Aggregate suggestions
		for _, suggestion := range completion.Suggestions {
			if suggestion != "" {
				context.AllSuggestions = append(context.AllSuggestions, fmt.Sprintf("[%s] %s", taskID, suggestion))
			}
		}

		// Aggregate dependencies (deduplicated)
		for _, dep := range completion.Dependencies {
			if dep != "" && !seenDeps[dep] {
				seenDeps[dep] = true
				context.Dependencies = append(context.Dependencies, dep)
			}
		}

		// Collect notes
		if completion.Notes != "" {
			context.Notes = append(context.Notes, fmt.Sprintf("**%s**: %s", taskID, completion.Notes))
		}
	}

	return context
}

// AggregatedTaskContext holds the aggregated context from all task completion files
type AggregatedTaskContext struct {
	TaskSummaries  map[string]string // taskID -> summary
	AllIssues      []string          // All issues from all tasks
	AllSuggestions []string          // All suggestions from all tasks
	Dependencies   []string          // Deduplicated list of new dependencies
	Notes          []string          // Implementation notes from all tasks
}

// HasContent returns true if there is any aggregated context worth displaying
func (a *AggregatedTaskContext) HasContent() bool {
	return len(a.AllIssues) > 0 || len(a.AllSuggestions) > 0 || len(a.Dependencies) > 0 || len(a.Notes) > 0
}

// FormatForPR formats the aggregated context for inclusion in a PR description
func (a *AggregatedTaskContext) FormatForPR() string {
	var sb strings.Builder

	if len(a.Notes) > 0 {
		sb.WriteString("\n## Implementation Notes\n\n")
		for _, note := range a.Notes {
			sb.WriteString(fmt.Sprintf("- %s\n", note))
		}
	}

	if len(a.AllIssues) > 0 {
		sb.WriteString("\n## Issues/Concerns Flagged\n\n")
		for _, issue := range a.AllIssues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
	}

	if len(a.AllSuggestions) > 0 {
		sb.WriteString("\n## Integration Suggestions\n\n")
		for _, suggestion := range a.AllSuggestions {
			sb.WriteString(fmt.Sprintf("- %s\n", suggestion))
		}
	}

	if len(a.Dependencies) > 0 {
		sb.WriteString("\n## New Dependencies\n\n")
		for _, dep := range a.Dependencies {
			sb.WriteString(fmt.Sprintf("- `%s`\n", dep))
		}
	}

	return sb.String()
}

// Helper functions

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func deduplicateStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(strs))
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
