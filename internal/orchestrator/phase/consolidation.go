// Package phase provides abstractions for ultra-plan phase execution.
package phase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// ConsolidationState tracks the progress and status of consolidation.
// This mirrors the state tracked by the underlying Consolidator but is
// managed by the orchestrator for phase-level coordination.
type ConsolidationState struct {
	// SubPhase tracks the current sub-phase within consolidation
	SubPhase string

	// CurrentGroup is the index of the group currently being consolidated
	CurrentGroup int

	// TotalGroups is the total number of groups to consolidate
	TotalGroups int

	// CurrentTask is the ID of the task currently being processed
	CurrentTask string

	// GroupBranches holds the branch names created for each group
	GroupBranches []string

	// PRUrls holds the URLs of created pull requests
	PRUrls []string

	// ConflictFiles lists files with merge conflicts (if paused)
	ConflictFiles []string

	// ConflictTaskID is the task ID that caused the conflict (if paused)
	ConflictTaskID string

	// ConflictWorktree is the worktree path where the conflict occurred (if paused)
	ConflictWorktree string

	// Error holds the error message if consolidation failed
	Error string
}

// ConsolidationOrchestrator manages the consolidation phase of an ultra-plan.
// It coordinates the merging of task branches into group branches and the
// creation of pull requests. The orchestrator delegates the actual consolidation
// work to a Consolidator instance while managing phase lifecycle and cancellation.
//
// The consolidation phase is responsible for:
//   - Building task-to-branch mappings from completed tasks
//   - Creating group branches (stacked or single mode)
//   - Cherry-picking task commits into group branches
//   - Handling merge conflicts (pausing for manual resolution)
//   - Creating pull requests for consolidated branches
type ConsolidationOrchestrator struct {
	// phaseCtx holds the shared dependencies for phase execution
	phaseCtx *PhaseContext

	// logger is a child logger with consolidation phase context
	logger *logging.Logger

	// ctx is the execution context, used for cancellation
	ctx context.Context

	// cancel cancels the execution context
	cancel context.CancelFunc

	// mu protects concurrent access to mutable state
	mu sync.Mutex

	// state tracks consolidation progress
	state *ConsolidationState

	// cancelled indicates whether Cancel() has been called
	cancelled bool

	// running indicates whether Execute() is currently running
	running bool

	// instanceID is the ID of the consolidation Claude instance
	instanceID string

	// completionFile holds the parsed completion data from the instance
	completionFile *ConsolidationCompletionFile

	// startedAt records when consolidation started
	startedAt *time.Time

	// completedAt records when consolidation completed
	completedAt *time.Time

	// completed indicates whether consolidation finished successfully
	completed bool
}

// NewConsolidationOrchestrator creates a new ConsolidationOrchestrator with the
// provided dependencies. The PhaseContext must be validated before use.
//
// The constructor creates a child context for cancellation and initializes
// the consolidation state to its default values.
func NewConsolidationOrchestrator(phaseCtx *PhaseContext) *ConsolidationOrchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a child logger with phase context
	logger := phaseCtx.GetLogger().WithPhase("consolidation-orchestrator")

	return &ConsolidationOrchestrator{
		phaseCtx: phaseCtx,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		state:    &ConsolidationState{},
	}
}

// Phase returns the UltraPlanPhase that this orchestrator handles.
// ConsolidationOrchestrator handles the PhaseConsolidating phase.
func (o *ConsolidationOrchestrator) Phase() UltraPlanPhase {
	return PhaseConsolidating
}

// Execute runs the consolidation phase logic. It respects the provided context
// for cancellation and returns early if the context is cancelled or Cancel()
// is called.
//
// Execute performs the following steps:
//  1. Validates the phase context
//  2. Sets the session phase to consolidating
//  3. Delegates to the underlying Consolidator
//  4. Updates phase to complete or failed based on result
//
// Returns an error if the phase context is invalid, consolidation fails,
// or the operation is cancelled.
func (o *ConsolidationOrchestrator) Execute(ctx context.Context) error {
	o.mu.Lock()
	if o.cancelled {
		o.mu.Unlock()
		return ErrCancelled
	}
	o.running = true
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.running = false
		o.mu.Unlock()
	}()

	// Validate the phase context
	if err := o.phaseCtx.Validate(); err != nil {
		return err
	}

	o.logger.Info("consolidation phase starting")

	// Set the session phase
	o.phaseCtx.Manager.SetPhase(PhaseConsolidating)

	// Notify callbacks of phase change
	if o.phaseCtx.Callbacks != nil {
		o.phaseCtx.Callbacks.OnPhaseChange(PhaseConsolidating)
	}

	// Check for cancellation before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-o.ctx.Done():
		return ErrCancelled
	default:
	}

	// The actual consolidation work will be delegated to the Consolidator
	// which is created by the main Coordinator. This orchestrator provides
	// the phase lifecycle management and cancellation handling.
	//
	// For now, we signal that consolidation has started. The integration
	// with the existing Consolidator will be completed when the Coordinator
	// is refactored to use phase orchestrators.

	o.logger.Info("consolidation phase ready for execution")

	return nil
}

// Cancel signals the orchestrator to stop any in-progress work.
// This is used for immediate cancellation requests (e.g., user abort).
// After Cancel is called, Execute should return promptly.
// Cancel is safe to call multiple times.
func (o *ConsolidationOrchestrator) Cancel() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cancelled {
		return
	}

	o.cancelled = true
	o.cancel()
	o.logger.Info("consolidation phase cancelled")
}

// State returns a copy of the current consolidation state.
// This is safe to call from any goroutine.
func (o *ConsolidationOrchestrator) State() ConsolidationState {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Return a copy to prevent external mutation
	stateCopy := *o.state
	if o.state.GroupBranches != nil {
		stateCopy.GroupBranches = make([]string, len(o.state.GroupBranches))
		copy(stateCopy.GroupBranches, o.state.GroupBranches)
	}
	if o.state.PRUrls != nil {
		stateCopy.PRUrls = make([]string, len(o.state.PRUrls))
		copy(stateCopy.PRUrls, o.state.PRUrls)
	}
	if o.state.ConflictFiles != nil {
		stateCopy.ConflictFiles = make([]string, len(o.state.ConflictFiles))
		copy(stateCopy.ConflictFiles, o.state.ConflictFiles)
	}
	return stateCopy
}

// SetState updates the consolidation state. This is primarily used for
// restoring state when resuming a paused consolidation.
func (o *ConsolidationOrchestrator) SetState(state ConsolidationState) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Deep copy slices to prevent external mutation
	o.state = &ConsolidationState{
		SubPhase:         state.SubPhase,
		CurrentGroup:     state.CurrentGroup,
		TotalGroups:      state.TotalGroups,
		CurrentTask:      state.CurrentTask,
		ConflictTaskID:   state.ConflictTaskID,
		ConflictWorktree: state.ConflictWorktree,
		Error:            state.Error,
	}
	if state.GroupBranches != nil {
		o.state.GroupBranches = make([]string, len(state.GroupBranches))
		copy(o.state.GroupBranches, state.GroupBranches)
	}
	if state.PRUrls != nil {
		o.state.PRUrls = make([]string, len(state.PRUrls))
		copy(o.state.PRUrls, state.PRUrls)
	}
	if state.ConflictFiles != nil {
		o.state.ConflictFiles = make([]string, len(state.ConflictFiles))
		copy(o.state.ConflictFiles, state.ConflictFiles)
	}
}

// IsRunning returns true if Execute() is currently running.
func (o *ConsolidationOrchestrator) IsRunning() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.running
}

// IsCancelled returns true if Cancel() has been called.
func (o *ConsolidationOrchestrator) IsCancelled() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.cancelled
}

// ErrCancelled is returned when the orchestrator is cancelled.
var ErrCancelled = errors.New("consolidation phase cancelled")

// ErrConsolidationFailed is returned when consolidation fails.
var ErrConsolidationFailed = errors.New("consolidation phase failed")

// ConsolidationCompletionFile represents the completion data written by the consolidation instance.
// This is parsed from the JSON file written by the Claude instance performing consolidation.
type ConsolidationCompletionFile struct {
	// Status is "complete", "partial", or "failed"
	Status string `json:"status"`

	// Mode is the consolidation mode used ("stacked" or "single")
	Mode string `json:"mode"`

	// GroupResults contains results for each execution group
	GroupResults []GroupResult `json:"group_results"`

	// PRsCreated contains information about all created PRs
	PRsCreated []PRInfo `json:"prs_created"`

	// TotalCommits is the total number of commits consolidated
	TotalCommits int `json:"total_commits"`

	// FilesChanged lists all files modified during consolidation
	FilesChanged []string `json:"files_changed"`
}

// GroupResult represents the consolidation result for a single execution group.
type GroupResult struct {
	// GroupIndex is the 0-based index of the group
	GroupIndex int `json:"group_index"`

	// BranchName is the consolidated branch name
	BranchName string `json:"branch_name"`

	// TasksIncluded lists the task IDs consolidated in this group
	TasksIncluded []string `json:"tasks_included"`

	// CommitCount is the number of commits in the consolidated branch
	CommitCount int `json:"commit_count"`

	// Success indicates whether consolidation succeeded for this group
	Success bool `json:"success"`

	// Error contains error message if consolidation failed for this group
	Error string `json:"error,omitempty"`
}

// PRInfo represents information about a created pull request.
type PRInfo struct {
	// URL is the full URL of the PR
	URL string `json:"url"`

	// Title is the PR title
	Title string `json:"title"`

	// GroupIndex identifies which group this PR is for
	GroupIndex int `json:"group_index"`
}

// ConsolidationInstanceInfo provides methods to get instance status.
// This interface is used to check on the consolidation instance without
// depending on the orchestrator package's Instance type.
type ConsolidationInstanceInfo interface {
	// GetStatus returns the current instance status
	GetStatus() InstanceStatus

	// GetID returns the instance ID
	GetID() string
}

// ConsolidationOrchestratorExtended provides extended methods needed by the
// ConsolidationOrchestrator for full consolidation execution.
// This interface extends OrchestratorInterface with consolidation-specific operations.
type ConsolidationOrchestratorExtended interface {
	OrchestratorInterface

	// GetConsolidationInstance retrieves an instance by ID with consolidation-specific type.
	// This is separate from OrchestratorInterface.GetInstance to allow returning
	// the consolidation-specific ConsolidationInstanceInfo interface.
	GetConsolidationInstance(id string) ConsolidationInstanceInfo

	// GetBaseSession returns the base session for instance creation
	GetBaseSession() any

	// GetSessionType returns the session type for group management
	GetSessionType() string

	// GetGroupBySessionType returns the instance group for the session type
	GetGroupBySessionType(sessionType string) interface {
		AddInstance(id string)
	}
}

// ConsolidationPromptBuilder builds prompts for consolidation.
// This interface allows the orchestrator to delegate prompt building
// to the coordinator which has access to the full session state.
type ConsolidationPromptBuilder interface {
	// BuildConsolidationPrompt builds the prompt for the consolidation phase
	BuildConsolidationPrompt() string
}

// GetInstanceID returns the ID of the consolidation instance, or empty if not started.
func (o *ConsolidationOrchestrator) GetInstanceID() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.instanceID
}

// setInstanceID sets the consolidation instance ID.
func (o *ConsolidationOrchestrator) setInstanceID(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.instanceID = id
}

// GetCompletionFile returns the parsed completion file, or nil if not available.
func (o *ConsolidationOrchestrator) GetCompletionFile() *ConsolidationCompletionFile {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.completionFile == nil {
		return nil
	}
	// Return a copy to prevent external mutation
	copy := *o.completionFile
	return &copy
}

// setCompletionFile sets the consolidation completion file.
func (o *ConsolidationOrchestrator) setCompletionFile(cf *ConsolidationCompletionFile) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.completionFile = cf
}

// GetPRUrls returns the URLs of created pull requests.
func (o *ConsolidationOrchestrator) GetPRUrls() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.state.PRUrls == nil {
		return nil
	}
	urls := make([]string, len(o.state.PRUrls))
	copy(urls, o.state.PRUrls)
	return urls
}

// addPRUrl adds a PR URL to the state.
func (o *ConsolidationOrchestrator) addPRUrl(url string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.state.PRUrls = append(o.state.PRUrls, url)
}

// GetStartedAt returns when consolidation started, or nil if not started.
func (o *ConsolidationOrchestrator) GetStartedAt() *time.Time {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.startedAt
}

// setStartedAt sets the consolidation start time.
func (o *ConsolidationOrchestrator) setStartedAt(t time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.startedAt = &t
}

// GetCompletedAt returns when consolidation completed, or nil if not completed.
func (o *ConsolidationOrchestrator) GetCompletedAt() *time.Time {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.completedAt
}

// setCompletedAt sets the consolidation completion time.
func (o *ConsolidationOrchestrator) setCompletedAt(t time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.completedAt = &t
}

// IsComplete returns true if consolidation completed successfully.
func (o *ConsolidationOrchestrator) IsComplete() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.completed
}

// setCompleted marks the consolidation as completed.
func (o *ConsolidationOrchestrator) setCompleted(success bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.completed = success
}

// GetError returns the consolidation error message, or empty if no error.
func (o *ConsolidationOrchestrator) GetError() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.state.Error
}

// setError sets the error message in state.
func (o *ConsolidationOrchestrator) setError(err string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.state.Error = err
}

// MonitorInstance monitors the consolidation instance for completion.
// This method should be called in a goroutine after the instance is started.
// It will return when the instance completes, fails, or the orchestrator is cancelled.
//
// The method checks the instance status periodically and:
//   - Returns nil when the instance completes successfully
//   - Returns an error if the instance fails, times out, or gets stuck
//   - Returns ErrCancelled if the orchestrator is cancelled
//
// The pollInterval parameter controls how often to check instance status.
// A typical value is 1 second.
func (o *ConsolidationOrchestrator) MonitorInstance(
	getInstanceFn func(id string) ConsolidationInstanceInfo,
	pollInterval time.Duration,
) error {
	instanceID := o.GetInstanceID()
	if instanceID == "" {
		return fmt.Errorf("no consolidation instance to monitor")
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return ErrCancelled

		case <-ticker.C:
			inst := getInstanceFn(instanceID)
			if inst == nil {
				// Instance gone - assume complete
				o.logger.Info("consolidation instance no longer exists, assuming complete")
				return nil
			}

			status := inst.GetStatus()
			switch status {
			case StatusCompleted, StatusWaitingInput:
				o.logger.Info("consolidation instance completed",
					"instance_id", instanceID,
					"status", string(status),
				)
				return nil

			case StatusError, StatusTimeout, StatusStuck:
				errMsg := fmt.Sprintf("consolidation failed: instance status %s", status)
				o.setError(errMsg)
				o.logger.Error("consolidation instance failed",
					"instance_id", instanceID,
					"status", string(status),
				)
				return fmt.Errorf("%w: %s", ErrConsolidationFailed, errMsg)
			}
			// StatusRunning: continue waiting
		}
	}
}

// FinishConsolidation completes the consolidation phase after the instance finishes.
// This updates state to reflect successful completion and notifies callbacks.
//
// Parameters:
//   - prUrls: The list of PR URLs that were created
//
// This method:
//  1. Updates internal state to complete
//  2. Records completion time
//  3. Stores PR URLs in state
//  4. Notifies the phase change callback
func (o *ConsolidationOrchestrator) FinishConsolidation(prUrls []string) {
	o.mu.Lock()
	now := time.Now()
	o.completedAt = &now
	o.completed = true
	o.state.SubPhase = "complete"
	if prUrls != nil {
		o.state.PRUrls = make([]string, len(prUrls))
		copy(o.state.PRUrls, prUrls)
	}
	o.mu.Unlock()

	o.logger.Info("consolidation phase finished",
		"pr_count", len(prUrls),
		"completed_at", now.Format(time.RFC3339),
	)
}

// MarkFailed marks the consolidation as failed with the given error.
// This updates state and logs the failure.
func (o *ConsolidationOrchestrator) MarkFailed(err error) {
	o.mu.Lock()
	o.state.SubPhase = "failed"
	o.state.Error = err.Error()
	o.completed = false
	now := time.Now()
	o.completedAt = &now
	o.mu.Unlock()

	o.logger.Error("consolidation phase failed",
		"error", err.Error(),
	)

	// Notify phase change if callbacks are configured
	if o.phaseCtx.Callbacks != nil {
		o.phaseCtx.Callbacks.OnPhaseChange(PhaseFailed)
	}
}

// UpdateProgress updates the consolidation progress state.
// This is called periodically to track which group is being consolidated.
func (o *ConsolidationOrchestrator) UpdateProgress(currentGroup, totalGroups int, subPhase string) {
	o.mu.Lock()
	o.state.CurrentGroup = currentGroup
	o.state.TotalGroups = totalGroups
	o.state.SubPhase = subPhase
	o.mu.Unlock()

	o.logger.Debug("consolidation progress updated",
		"current_group", currentGroup,
		"total_groups", totalGroups,
		"sub_phase", subPhase,
	)
}

// AddGroupBranch records a consolidated branch for a group.
func (o *ConsolidationOrchestrator) AddGroupBranch(branch string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.state.GroupBranches = append(o.state.GroupBranches, branch)
}

// SetConflict records that consolidation is paused due to a merge conflict.
// This allows the orchestrator to track conflicts for potential resumption.
//
// Parameters:
//   - taskID: The ID of the task that caused the conflict
//   - worktreePath: The path to the worktree where the conflict occurred
//   - files: The list of files with merge conflicts
func (o *ConsolidationOrchestrator) SetConflict(taskID, worktreePath string, files []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.state.SubPhase = "paused"
	o.state.CurrentTask = taskID
	o.state.ConflictTaskID = taskID
	o.state.ConflictWorktree = worktreePath
	o.state.ConflictFiles = make([]string, len(files))
	copy(o.state.ConflictFiles, files)
}

// ClearConflict clears the conflict state after resolution.
func (o *ConsolidationOrchestrator) ClearConflict() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.state.CurrentTask = ""
	o.state.ConflictTaskID = ""
	o.state.ConflictWorktree = ""
	o.state.ConflictFiles = nil
}

// HasConflict returns true if consolidation is paused due to a conflict.
func (o *ConsolidationOrchestrator) HasConflict() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.state.SubPhase == "paused" && len(o.state.ConflictFiles) > 0
}

// Reset resets the orchestrator to its initial state for a fresh execution.
// This is useful when restarting the consolidation phase.
func (o *ConsolidationOrchestrator) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Reset state
	o.state = &ConsolidationState{}
	o.instanceID = ""
	o.completionFile = nil
	o.startedAt = nil
	o.completedAt = nil
	o.completed = false
	o.cancelled = false
	o.running = false

	// Create a new cancellable context
	o.cancel()
	o.ctx, o.cancel = context.WithCancel(context.Background())
}

// WorktreeOperator defines the worktree operations needed for conflict handling.
// This interface allows the orchestrator to check for and resolve merge conflicts
// without depending on the worktree package directly.
type WorktreeOperator interface {
	// GetConflictingFiles returns the list of files with merge conflicts in a worktree.
	// Returns an empty slice if there are no conflicts.
	GetConflictingFiles(worktreePath string) ([]string, error)

	// IsCherryPickInProgress returns true if a cherry-pick is in progress in the worktree.
	IsCherryPickInProgress(worktreePath string) bool

	// ContinueCherryPick continues a cherry-pick operation after conflict resolution.
	ContinueCherryPick(worktreePath string) error
}

// ConsolidationRestartCallback is called when consolidation needs to be restarted.
// This callback is provided by the Coordinator to trigger a fresh consolidation.
type ConsolidationRestartCallback func() error

// ConsolidationSessionSaver defines the interface for saving session state.
// This is used during resume to persist state changes.
type ConsolidationSessionSaver interface {
	// SaveSession persists the current session state to disk.
	SaveSession() error
}

// Error variables for resume operations.
var (
	// ErrNoConsolidation is returned when attempting to resume without an active consolidation.
	ErrNoConsolidation = errors.New("no consolidation in progress")

	// ErrNotPaused is returned when attempting to resume a consolidation that is not paused.
	ErrNotPaused = errors.New("consolidation is not paused")

	// ErrNoConflictWorktree is returned when resuming without a recorded conflict worktree.
	ErrNoConflictWorktree = errors.New("no conflict worktree recorded")

	// ErrUnresolvedConflicts is returned when attempting to resume with unresolved conflicts.
	ErrUnresolvedConflicts = errors.New("unresolved conflicts remain")
)

// ResumeConsolidation resumes consolidation after a conflict has been resolved.
// The user must first resolve the conflict manually in the conflict worktree,
// then call this method to continue the cherry-pick and restart consolidation.
//
// The method performs the following steps:
//  1. Validates that consolidation is in a paused state with a conflict
//  2. Verifies there are no remaining unresolved conflicts
//  3. Continues the cherry-pick operation if one is in progress
//  4. Clears the conflict state
//  5. Saves the session state
//  6. Restarts consolidation via the provided callback
//
// Note: This restarts the consolidation instance from scratch, replaying any
// already-completed group merges (which will be no-ops since commits exist).
//
// Parameters:
//   - worktreeOp: The worktree operator for checking/resolving conflicts
//   - sessionSaver: Interface for saving session state
//   - restartCallback: Callback to restart consolidation after clearing conflict state
//
// Returns an error if:
//   - Consolidation is not in progress or not paused
//   - No conflict worktree is recorded
//   - Unresolved conflicts remain
//   - The cherry-pick continuation fails
//   - Session save fails
//   - The restart callback fails
func (o *ConsolidationOrchestrator) ResumeConsolidation(
	worktreeOp WorktreeOperator,
	sessionSaver ConsolidationSessionSaver,
	restartCallback ConsolidationRestartCallback,
) error {
	// Get current state
	state := o.State()

	// Validate state
	if state.SubPhase != "paused" {
		return fmt.Errorf("%w (current sub-phase: %s)", ErrNotPaused, state.SubPhase)
	}

	conflictWorktree := state.ConflictWorktree
	if conflictWorktree == "" {
		return ErrNoConflictWorktree
	}

	// Log the resume attempt
	o.logger.Info("resuming consolidation",
		"conflict_worktree", conflictWorktree,
		"conflict_task_id", state.ConflictTaskID,
		"conflict_files", state.ConflictFiles,
	)

	// Check if there are still unresolved conflicts
	conflictFiles, err := worktreeOp.GetConflictingFiles(conflictWorktree)
	if err != nil {
		return fmt.Errorf("failed to check for conflicts in worktree %s: %w", conflictWorktree, err)
	}
	if len(conflictFiles) > 0 {
		return fmt.Errorf("%w in %d file(s): %v", ErrUnresolvedConflicts, len(conflictFiles), conflictFiles)
	}

	// Continue the cherry-pick if one is in progress
	if worktreeOp.IsCherryPickInProgress(conflictWorktree) {
		if err := worktreeOp.ContinueCherryPick(conflictWorktree); err != nil {
			return fmt.Errorf("failed to continue cherry-pick: %w", err)
		}
		o.logger.Info("continued cherry-pick operation", "worktree", conflictWorktree)
	} else {
		// No cherry-pick in progress - user may have resolved via abort/skip
		o.logger.Info("no cherry-pick in progress, assuming conflict was resolved via abort or skip",
			"worktree", conflictWorktree,
		)
	}

	// Clear the conflict state
	o.ClearConflict()

	// Also clear the instance ID so a new one will be created
	o.mu.Lock()
	o.instanceID = ""
	o.mu.Unlock()

	// Save session state before restarting
	if err := sessionSaver.SaveSession(); err != nil {
		return fmt.Errorf("failed to save session state: %w", err)
	}

	// Restart consolidation from scratch
	// The consolidation instance will replay all groups, but already-merged
	// commits will be detected and skipped
	o.logger.Info("restarting consolidation instance after conflict resolution")

	if err := restartCallback(); err != nil {
		return fmt.Errorf("failed to restart consolidation: %w", err)
	}

	return nil
}

// GetConsolidation returns a copy of the current consolidation state.
// This is an alias for State() that matches the naming convention used by the Coordinator.
// Returns nil if no consolidation is in progress (i.e., state is at initial values).
func (o *ConsolidationOrchestrator) GetConsolidation() *ConsolidationState {
	state := o.State()

	// Return nil if there's no active consolidation
	// (state is at initial values - no groups processed, no sub-phase set)
	if state.SubPhase == "" && state.TotalGroups == 0 && state.CurrentGroup == 0 {
		return nil
	}

	return &state
}

// ClearStateForRestart prepares the orchestrator for a fresh restart while
// preserving any progress that should be maintained (like completed groups).
// This is different from Reset() which clears everything.
//
// ClearStateForRestart:
//   - Clears conflict state
//   - Clears the instance ID (so a new instance will be created)
//   - Resets completion flags
//   - Does NOT clear group branches or PR URLs (progress tracking)
//   - Does NOT clear the error field (for debugging)
func (o *ConsolidationOrchestrator) ClearStateForRestart() {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Clear conflict-related state
	o.state.SubPhase = ""
	o.state.CurrentTask = ""
	o.state.ConflictTaskID = ""
	o.state.ConflictWorktree = ""
	o.state.ConflictFiles = nil

	// Clear instance tracking (will be set when new instance starts)
	o.instanceID = ""
	o.completionFile = nil

	// Reset completion flags (preserve times for debugging)
	o.completed = false

	// Reset cancellation state
	o.cancelled = false
	o.running = false

	// Create a new cancellable context
	o.cancel()
	o.ctx, o.cancel = context.WithCancel(context.Background())
}

// GetConflictInfo returns detailed information about the current conflict.
// Returns empty strings if there is no conflict.
func (o *ConsolidationOrchestrator) GetConflictInfo() (taskID, worktreePath string, files []string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.state.SubPhase != "paused" {
		return "", "", nil
	}

	// Return copies to prevent external mutation
	var filesCopy []string
	if o.state.ConflictFiles != nil {
		filesCopy = make([]string, len(o.state.ConflictFiles))
		copy(filesCopy, o.state.ConflictFiles)
	}

	return o.state.ConflictTaskID, o.state.ConflictWorktree, filesCopy
}

// IsPaused returns true if the consolidation is in a paused state.
// This includes conflicts and any other paused conditions.
func (o *ConsolidationOrchestrator) IsPaused() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.state.SubPhase == "paused"
}

// CanResume returns true if consolidation can be resumed.
// This requires being paused with a valid conflict worktree.
func (o *ConsolidationOrchestrator) CanResume() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.state.SubPhase == "paused" && o.state.ConflictWorktree != ""
}
