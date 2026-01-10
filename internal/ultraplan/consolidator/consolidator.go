// Package consolidator provides types and utilities for consolidating Ultra-Plan task branches.
//
// The consolidator is responsible for merging task branches into group branches
// and creating pull requests after Ultra-Plan execution completes.
package consolidator

import (
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// Mode defines how work is consolidated after ultraplan execution.
type Mode string

const (
	// ModeStacked creates one PR per execution group, stacked on each other.
	ModeStacked Mode = "stacked"
	// ModeSingle consolidates all work into a single PR.
	ModeSingle Mode = "single"
)

// Phase represents sub-phases within consolidation.
type Phase string

const (
	PhaseIdle             Phase = "idle"
	PhaseDetecting        Phase = "detecting_conflicts"
	PhaseCreatingBranches Phase = "creating_branches"
	PhaseMergingTasks     Phase = "merging_tasks"
	PhasePushing          Phase = "pushing"
	PhaseCreatingPRs      Phase = "creating_prs"
	PhasePaused           Phase = "paused"
	PhaseComplete         Phase = "complete"
	PhaseFailed           Phase = "failed"
)

// State tracks the progress of consolidation.
type State struct {
	Phase            Phase      `json:"phase"`
	CurrentGroup     int        `json:"current_group"`
	TotalGroups      int        `json:"total_groups"`
	CurrentTask      string     `json:"current_task,omitempty"`
	GroupBranches    []string   `json:"group_branches"`
	PRUrls           []string   `json:"pr_urls"`
	ConflictFiles    []string   `json:"conflict_files,omitempty"`
	ConflictTaskID   string     `json:"conflict_task_id,omitempty"`
	ConflictWorktree string     `json:"conflict_worktree,omitempty"`
	Error            string     `json:"error,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

// GroupResult holds the result of consolidating one group.
type GroupResult struct {
	GroupIndex   int      `json:"group_index"`
	TaskIDs      []string `json:"task_ids"`
	BranchName   string   `json:"branch_name"`
	CommitCount  int      `json:"commit_count"`
	FilesChanged []string `json:"files_changed"`
	PRUrl        string   `json:"pr_url,omitempty"`
	Success      bool     `json:"success"`
	Error        string   `json:"error,omitempty"`
}

// Config holds configuration for branch consolidation.
type Config struct {
	Mode           Mode
	BranchPrefix   string
	CreateDraftPRs bool
	PRLabels       []string
}

// DefaultConfig returns sensible defaults for consolidation configuration.
func DefaultConfig() Config {
	return Config{
		Mode:           ModeStacked,
		BranchPrefix:   "",
		CreateDraftPRs: true,
		PRLabels:       []string{"ultraplan"},
	}
}

// EventType represents events during consolidation.
type EventType string

const (
	EventStarted       EventType = "consolidation_started"
	EventGroupStarted  EventType = "consolidation_group_started"
	EventTaskMerging   EventType = "consolidation_task_merging"
	EventTaskMerged    EventType = "consolidation_task_merged"
	EventGroupComplete EventType = "consolidation_group_complete"
	EventPRCreating    EventType = "consolidation_pr_creating"
	EventPRCreated     EventType = "consolidation_pr_created"
	EventConflict      EventType = "consolidation_conflict"
	EventComplete      EventType = "consolidation_complete"
	EventFailed        EventType = "consolidation_failed"
)

// Event represents an event during consolidation.
type Event struct {
	Type      EventType `json:"type"`
	GroupIdx  int       `json:"group_idx,omitempty"`
	TaskID    string    `json:"task_id,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// WorktreeManager provides git worktree operations for consolidation.
type WorktreeManager interface {
	// FindMainBranch returns the name of the main/master branch.
	FindMainBranch() string

	// CreateBranchFrom creates a new branch from a base branch.
	CreateBranchFrom(newBranch, baseBranch string) error

	// CreateWorktreeFromBranch creates a worktree at path for a branch.
	CreateWorktreeFromBranch(path, branch string) error

	// CherryPickBranch cherry-picks all commits from a branch into the current worktree.
	CherryPickBranch(worktreePath, sourceBranch string) error

	// ContinueCherryPick continues a paused cherry-pick operation.
	ContinueCherryPick(worktreePath string) error

	// Push pushes the current branch to the remote.
	Push(worktreePath string, force bool) error

	// Remove removes a worktree.
	Remove(worktreePath string) error

	// GetConflictingFiles returns files with merge conflicts.
	GetConflictingFiles(worktreePath string) ([]string, error)

	// GetChangedFiles returns files changed in the worktree.
	GetChangedFiles(worktreePath string) ([]string, error)

	// CountCommitsBetween counts commits between two refs.
	CountCommitsBetween(worktreePath, base, head string) (int, error)
}

// PRCreator creates pull requests.
type PRCreator interface {
	// CreatePR creates a pull request.
	CreatePR(title, body, branch, baseBranch string, draft bool, labels []string) (string, error)
}

// Consolidator handles the consolidation of task branches into group branches and PR creation.
type Consolidator struct {
	session  *ultraplan.Session
	config   Config
	wt       WorktreeManager
	pr       PRCreator
	state    *State
	results  []*GroupResult
	logger   *logging.Logger

	// Task branch mapping: task ID -> branch name
	taskBranches map[string]string

	// Event handling
	eventCallback func(Event)

	// Synchronization
	mu       sync.RWMutex
	stopChan chan struct{}
	stopped  bool
}

// NewConsolidator creates a new consolidator.
func NewConsolidator(
	session *ultraplan.Session,
	config Config,
	wt WorktreeManager,
	pr PRCreator,
	logger *logging.Logger,
) *Consolidator {
	if logger == nil {
		logger = logging.NopLogger()
	}
	logger = logger.WithPhase("consolidation")

	return &Consolidator{
		session:      session,
		config:       config,
		wt:           wt,
		pr:           pr,
		state:        &State{Phase: PhaseIdle},
		results:      make([]*GroupResult, 0),
		taskBranches: make(map[string]string),
		stopChan:     make(chan struct{}),
		logger:       logger,
	}
}

// SetEventCallback sets the callback for consolidation events.
func (c *Consolidator) SetEventCallback(cb func(Event)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventCallback = cb
}

// State returns a copy of the current consolidation state.
func (c *Consolidator) State() *State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	stateCopy := *c.state
	return &stateCopy
}

// Results returns the consolidation results.
func (c *Consolidator) Results() []*GroupResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.results
}

// emitEvent sends an event to the callback.
func (c *Consolidator) emitEvent(event Event) {
	event.Timestamp = time.Now()

	c.mu.RLock()
	cb := c.eventCallback
	c.mu.RUnlock()

	if cb != nil {
		cb(event)
	}
}

// setPhase updates the consolidation phase with logging.
func (c *Consolidator) setPhase(phase Phase, groupIdx int) {
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

// SetTaskBranch maps a task ID to its branch name.
// This must be called for all completed tasks before Run().
func (c *Consolidator) SetTaskBranch(taskID, branch string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taskBranches[taskID] = branch
}

// Run executes the full consolidation process.
func (c *Consolidator) Run(baseDir string) error {
	c.mu.Lock()
	now := time.Now()
	c.state.StartedAt = &now
	if c.session.Plan != nil {
		c.state.TotalGroups = len(c.session.Plan.ExecutionOrder)
	}
	c.mu.Unlock()

	c.logger.Info("consolidation started",
		"total_groups", c.state.TotalGroups,
		"mode", string(c.config.Mode),
		"session_id", c.session.ID,
	)

	c.emitEvent(Event{
		Type:    EventStarted,
		Message: fmt.Sprintf("Consolidating %d groups", c.state.TotalGroups),
	})

	// Verify all completed tasks have branches mapped
	for _, taskID := range c.session.CompletedTasks {
		if _, ok := c.taskBranches[taskID]; !ok {
			return c.fail(fmt.Errorf("no branch mapping for completed task %s", taskID))
		}
	}

	// Run the appropriate mode
	var err error
	switch c.config.Mode {
	case ModeStacked:
		err = c.runStackedMode(baseDir)
	case ModeSingle:
		err = c.runSingleMode(baseDir)
	default:
		err = c.runStackedMode(baseDir) // Default to stacked
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
	c.state.Phase = PhaseComplete
	c.mu.Unlock()

	c.logger.Info("consolidation completed",
		"pr_count", len(c.state.PRUrls),
		"group_branches", len(c.state.GroupBranches),
	)

	c.emitEvent(Event{
		Type:    EventComplete,
		Message: fmt.Sprintf("Created %d PRs", len(c.state.PRUrls)),
	})

	return nil
}

// runStackedMode consolidates work into stacked branches (one per group).
func (c *Consolidator) runStackedMode(baseDir string) error {
	// Check if we have pre-consolidated branches
	if len(c.session.GroupConsolidatedBranches) > 0 {
		return c.usePreconsolidatedBranches()
	}

	return c.consolidateGroups(baseDir)
}

// runSingleMode consolidates all work into a single branch.
func (c *Consolidator) runSingleMode(baseDir string) error {
	mainBranch := c.wt.FindMainBranch()

	c.setPhase(PhaseCreatingBranches, 0)

	// Create single consolidated branch
	consolidatedBranch := c.generateSingleBranchName()

	c.logger.Debug("creating consolidated branch",
		"branch", consolidatedBranch,
		"base", mainBranch,
	)

	if err := c.wt.CreateBranchFrom(consolidatedBranch, mainBranch); err != nil {
		return c.fail(fmt.Errorf("failed to create branch %s: %w", consolidatedBranch, err))
	}

	// Create worktree
	worktreePath := fmt.Sprintf("%s/consolidated", baseDir)
	if err := c.wt.CreateWorktreeFromBranch(worktreePath, consolidatedBranch); err != nil {
		return c.fail(fmt.Errorf("failed to create worktree: %w", err))
	}

	// Collect all task IDs from all groups
	var allTaskIDs []string
	if c.session.Plan != nil {
		for _, group := range c.session.Plan.ExecutionOrder {
			allTaskIDs = append(allTaskIDs, group...)
		}
	}

	// Consolidate all tasks
	result, err := c.consolidateGroup(0, allTaskIDs, worktreePath, mainBranch)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.results = append(c.results, result)
	c.state.GroupBranches = append(c.state.GroupBranches, consolidatedBranch)
	c.mu.Unlock()

	// Push
	c.setPhase(PhasePushing, 0)
	if err := c.wt.Push(worktreePath, false); err != nil {
		return c.fail(fmt.Errorf("failed to push branch: %w", err))
	}

	// Clean up
	_ = c.wt.Remove(worktreePath)

	return nil
}

// usePreconsolidatedBranches uses branches that were already consolidated by per-group consolidators.
func (c *Consolidator) usePreconsolidatedBranches() error {
	c.setPhase(PhaseCreatingBranches, 0)

	for groupIdx, branch := range c.session.GroupConsolidatedBranches {
		if branch == "" {
			continue // Skip groups that weren't consolidated
		}

		c.logger.Info("using pre-consolidated branch",
			"branch_name", branch,
			"group_index", groupIdx,
		)

		c.emitEvent(Event{
			Type:     EventGroupStarted,
			GroupIdx: groupIdx,
			Message:  fmt.Sprintf("Using pre-consolidated branch %s", branch),
		})

		var taskIDs []string
		if c.session.Plan != nil && groupIdx < len(c.session.Plan.ExecutionOrder) {
			taskIDs = c.session.Plan.ExecutionOrder[groupIdx]
		}

		result := &GroupResult{
			GroupIndex:  groupIdx,
			TaskIDs:     taskIDs,
			BranchName:  branch,
			CommitCount: 0,
			Success:     true,
		}

		c.mu.Lock()
		c.results = append(c.results, result)
		c.state.GroupBranches = append(c.state.GroupBranches, branch)
		c.mu.Unlock()

		c.emitEvent(Event{
			Type:     EventGroupComplete,
			GroupIdx: groupIdx,
			Message:  fmt.Sprintf("Group %d ready with branch %s", groupIdx+1, branch),
		})
	}

	return nil
}

// consolidateGroups creates stacked branches by consolidating groups sequentially.
func (c *Consolidator) consolidateGroups(baseDir string) error {
	if c.session.Plan == nil {
		return fmt.Errorf("no plan in session")
	}

	mainBranch := c.wt.FindMainBranch()

	for groupIdx, taskIDs := range c.session.Plan.ExecutionOrder {
		c.setPhase(PhaseCreatingBranches, groupIdx)

		// Determine base branch
		var baseBranch string
		if groupIdx == 0 {
			baseBranch = mainBranch
		} else {
			baseBranch = c.state.GroupBranches[groupIdx-1]
		}

		// Generate group branch name
		groupBranch := c.generateGroupBranchName(groupIdx)

		c.emitEvent(Event{
			Type:     EventGroupStarted,
			GroupIdx: groupIdx,
			Message:  fmt.Sprintf("Creating branch %s from %s", groupBranch, baseBranch),
		})

		// Create branch from base
		if err := c.wt.CreateBranchFrom(groupBranch, baseBranch); err != nil {
			return c.fail(fmt.Errorf("failed to create branch %s: %w", groupBranch, err))
		}

		// Create worktree for this group
		worktreePath := fmt.Sprintf("%s/group-%d", baseDir, groupIdx+1)
		if err := c.wt.CreateWorktreeFromBranch(worktreePath, groupBranch); err != nil {
			return c.fail(fmt.Errorf("failed to create worktree for group %d: %w", groupIdx, err))
		}

		// Consolidate tasks into this group
		result, err := c.consolidateGroup(groupIdx, taskIDs, worktreePath, baseBranch)
		if err != nil {
			return err
		}

		c.mu.Lock()
		c.results = append(c.results, result)
		c.state.GroupBranches = append(c.state.GroupBranches, groupBranch)
		c.mu.Unlock()

		// Push the group branch
		c.setPhase(PhasePushing, groupIdx)
		if err := c.wt.Push(worktreePath, false); err != nil {
			return c.fail(fmt.Errorf("failed to push branch %s: %w", groupBranch, err))
		}

		// Clean up worktree
		_ = c.wt.Remove(worktreePath)

		c.emitEvent(Event{
			Type:     EventGroupComplete,
			GroupIdx: groupIdx,
			Message:  fmt.Sprintf("Group %d consolidated with %d tasks", groupIdx+1, len(taskIDs)),
		})
	}

	return nil
}

// consolidateGroup merges all tasks in a group into the group worktree.
func (c *Consolidator) consolidateGroup(groupIdx int, taskIDs []string, worktreePath, baseBranch string) (*GroupResult, error) {
	c.setPhase(PhaseMergingTasks, groupIdx)

	result := &GroupResult{
		GroupIndex: groupIdx,
		TaskIDs:    taskIDs,
		Success:    false,
	}

	// Filter to tasks with branches
	var activeTasks []string
	for _, taskID := range taskIDs {
		if branch := c.taskBranches[taskID]; branch != "" {
			activeTasks = append(activeTasks, taskID)
		}
	}

	if len(activeTasks) == 0 {
		return result, fmt.Errorf("no task branches found for group %d", groupIdx)
	}

	// Track initial commit count
	initialCommits, _ := c.wt.CountCommitsBetween(worktreePath, baseBranch, "HEAD")

	// Cherry-pick each task's commits
	for _, taskID := range activeTasks {
		c.mu.Lock()
		c.state.CurrentTask = taskID
		c.mu.Unlock()

		branch := c.taskBranches[taskID]

		c.emitEvent(Event{
			Type:     EventTaskMerging,
			GroupIdx: groupIdx,
			TaskID:   taskID,
			Message:  fmt.Sprintf("Cherry-picking from %s", branch),
		})

		err := c.wt.CherryPickBranch(worktreePath, branch)
		if err != nil {
			// Check for conflict
			files, _ := c.wt.GetConflictingFiles(worktreePath)
			if len(files) > 0 {
				return result, &ConflictError{
					TaskID:       taskID,
					Branch:       branch,
					Files:        files,
					WorktreePath: worktreePath,
					Underlying:   err,
				}
			}
			return result, fmt.Errorf("failed to cherry-pick task %s: %w", taskID, err)
		}

		c.emitEvent(Event{
			Type:     EventTaskMerged,
			GroupIdx: groupIdx,
			TaskID:   taskID,
			Message:  fmt.Sprintf("Merged task %s", taskID),
		})
	}

	// Verify commits were added
	finalCommits, _ := c.wt.CountCommitsBetween(worktreePath, baseBranch, "HEAD")
	addedCommits := finalCommits - initialCommits

	if addedCommits == 0 && len(activeTasks) > 0 {
		return result, fmt.Errorf("cherry-picked %d branches but no commits were added", len(activeTasks))
	}

	// Get changed files
	files, _ := c.wt.GetChangedFiles(worktreePath)
	result.FilesChanged = files
	result.CommitCount = addedCommits
	result.Success = true

	c.mu.Lock()
	c.state.CurrentTask = ""
	c.mu.Unlock()

	return result, nil
}

// createPRs creates pull requests for all group branches.
func (c *Consolidator) createPRs() error {
	c.setPhase(PhaseCreatingPRs, 0)

	mainBranch := c.wt.FindMainBranch()

	// Create PRs in reverse order
	for i := len(c.state.GroupBranches) - 1; i >= 0; i-- {
		branch := c.state.GroupBranches[i]

		var baseBranch string
		if c.config.Mode == ModeSingle || i == 0 {
			baseBranch = mainBranch
		} else {
			baseBranch = c.state.GroupBranches[i-1]
		}

		c.emitEvent(Event{
			Type:     EventPRCreating,
			GroupIdx: i,
			Message:  fmt.Sprintf("Creating PR for %s -> %s", branch, baseBranch),
		})

		title, body := c.buildPRContent(i)

		prURL, err := c.pr.CreatePR(title, body, branch, baseBranch, c.config.CreateDraftPRs, c.config.PRLabels)
		if err != nil {
			return fmt.Errorf("failed to create PR for group %d: %w", i+1, err)
		}

		c.mu.Lock()
		c.state.PRUrls = append([]string{prURL}, c.state.PRUrls...)
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

		c.emitEvent(Event{
			Type:     EventPRCreated,
			GroupIdx: i,
			Message:  prURL,
		})
	}

	return nil
}

// buildPRContent generates title and body for a group PR.
func (c *Consolidator) buildPRContent(groupIdx int) (string, string) {
	var title string
	objective := c.session.Objective
	if len(objective) > 50 {
		objective = objective[:47] + "..."
	}

	if c.config.Mode == ModeSingle {
		title = fmt.Sprintf("ultraplan: %s", objective)
	} else {
		title = fmt.Sprintf("ultraplan: group %d - %s", groupIdx+1, objective)
	}

	body := fmt.Sprintf("## Ultraplan Consolidation\n\n**Objective**: %s\n", c.session.Objective)

	if c.config.Mode == ModeStacked {
		body += fmt.Sprintf("\n**Group**: %d of %d\n", groupIdx+1, c.state.TotalGroups)
	}

	return title, body
}

// generateGroupBranchName generates a branch name for a group.
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

// generateSingleBranchName generates a branch name for single PR mode.
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

// fail marks the consolidation as failed.
func (c *Consolidator) fail(err error) error {
	c.mu.Lock()
	c.state.Phase = PhaseFailed
	c.state.Error = err.Error()
	c.mu.Unlock()

	c.logger.Error("consolidation failed", "error", err.Error())

	c.emitEvent(Event{
		Type:    EventFailed,
		Message: err.Error(),
	})

	return err
}

// Stop stops the consolidation process.
func (c *Consolidator) Stop() {
	c.mu.Lock()
	if !c.stopped {
		c.stopped = true
		close(c.stopChan)
	}
	c.mu.Unlock()
}

// ConflictError represents a conflict during consolidation.
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
