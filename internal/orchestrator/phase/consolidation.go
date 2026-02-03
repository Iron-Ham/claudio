// Package phase provides abstractions for ultra-plan phase execution.
package phase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
)

// ConsolidatorState tracks the progress and status of consolidation.
// This mirrors the state tracked by the underlying Consolidator but is
// managed by the orchestrator for phase-level coordination.
type ConsolidatorState struct {
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
	state *ConsolidatorState

	// cancelled indicates whether Cancel() has been called
	cancelled bool

	// running indicates whether Execute() is currently running
	running bool

	// instanceID is the ID of the consolidation backend instance
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
		state:    &ConsolidatorState{},
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
func (o *ConsolidationOrchestrator) State() ConsolidatorState {
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
func (o *ConsolidationOrchestrator) SetState(state ConsolidatorState) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Deep copy slices to prevent external mutation
	o.state = &ConsolidatorState{
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
// This is parsed from the JSON file written by the backend instance performing consolidation.
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
	o.state = &ConsolidatorState{}
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
func (o *ConsolidationOrchestrator) GetConsolidation() *ConsolidatorState {
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

// ============================================================================
// Per-Group Consolidation Interfaces
// ============================================================================

// GroupConsolidationSessionInterface provides session state for per-group consolidation.
// This extends UltraPlanSessionInterface with group-specific methods.
type GroupConsolidationSessionInterface interface {
	UltraPlanSessionInterface

	// GetPlanExecutionOrder returns the execution order (task IDs per group)
	GetPlanExecutionOrder() [][]string

	// GetPlanSummary returns the plan summary
	GetPlanSummary() string

	// GetID returns the session ID
	GetID() string

	// GetBranchPrefix returns the configured branch prefix
	GetBranchPrefix() string

	// GetGroupConsolidatedBranches returns the consolidated branches for each group
	GetGroupConsolidatedBranches() []string

	// SetGroupConsolidatedBranch sets the consolidated branch for a group
	SetGroupConsolidatedBranch(groupIndex int, branch string)

	// GetGroupConsolidatorIDs returns the consolidator instance IDs for each group
	GetGroupConsolidatorIDs() []string

	// SetGroupConsolidatorID sets the consolidator instance ID for a group
	SetGroupConsolidatorID(groupIndex int, instanceID string)

	// GetGroupConsolidationContexts returns the consolidation contexts from each group
	GetGroupConsolidationContexts() []*types.GroupConsolidationCompletionFile

	// SetGroupConsolidationContext sets the consolidation context for a group
	SetGroupConsolidationContext(groupIndex int, context *types.GroupConsolidationCompletionFile)

	// IsMultiPass returns true if multi-pass planning is enabled
	IsMultiPass() bool
}

// GroupConsolidationOrchestratorInterface provides orchestrator methods for per-group consolidation.
// This extends OrchestratorInterface with group consolidation-specific operations.
type GroupConsolidationOrchestratorInterface interface {
	OrchestratorInterface

	// AddInstanceFromBranch creates a new instance from a specific branch
	AddInstanceFromBranch(session any, task string, baseBranch string) (InstanceInterface, error)

	// StopInstance stops a running instance
	StopInstance(inst any) error

	// FindMainBranch returns the name of the main branch
	FindMainBranch() string

	// CreateBranchFrom creates a new branch from a base branch
	CreateBranchFrom(newBranch, baseBranch string) error

	// CreateWorktreeFromBranch creates a worktree for a branch
	CreateWorktreeFromBranch(worktreePath, branch string) error

	// RemoveWorktree removes a worktree
	RemoveWorktree(worktreePath string) error

	// CherryPickBranch cherry-picks commits from a branch
	CherryPickBranch(worktreePath, branch string) error

	// AbortCherryPick aborts an in-progress cherry-pick
	AbortCherryPick(worktreePath string) error

	// CountCommitsBetween counts commits between two refs
	CountCommitsBetween(worktreePath, base, head string) (int, error)

	// Push pushes the current branch to remote
	Push(worktreePath string, force bool) error

	// GetClaudioDir returns the .claudio directory path
	GetClaudioDir() string

	// TmuxSessionExists checks if a tmux session exists for an instance
	TmuxSessionExists(instanceID string) bool
}

// GroupConsolidationBaseSessionInterface provides base session methods for per-group consolidation.
type GroupConsolidationBaseSessionInterface interface {
	BaseSessionInterface

	// GetInstanceByTask returns the instance executing a specific task
	GetInstanceByTask(taskID string) InstanceInterface
}

// TaskCompletionFileParser parses task completion files from worktrees.
type TaskCompletionFileParser interface {
	// ParseTaskCompletionFile reads and parses a task completion file
	ParseTaskCompletionFile(worktreePath string) (*types.TaskCompletionFile, error)

	// ParseGroupConsolidationCompletionFile reads and parses a group consolidation completion file
	ParseGroupConsolidationCompletionFile(worktreePath string) (*types.GroupConsolidationCompletionFile, error)

	// GroupConsolidationCompletionFilePath returns the full path to the completion file
	GroupConsolidationCompletionFilePath(worktreePath string) string
}

// GroupConsolidationEventEmitter emits events during consolidation.
type GroupConsolidationEventEmitter interface {
	// EmitGroupConsolidationEvent emits an event for group consolidation progress
	EmitGroupConsolidationEvent(eventType string, groupIndex int, message string)
}

// ============================================================================
// Per-Group Consolidation Methods
// ============================================================================

// GatherTaskCompletionContextForGroup reads completion files from all completed tasks
// in a group and aggregates the context for the group consolidator.
//
// This method iterates through all tasks in the specified group, finds their
// corresponding instances, and reads the completion files to extract:
//   - Task summaries
//   - Issues flagged by each task
//   - Suggestions for integration
//   - New dependencies added
//   - Implementation notes
//
// Parameters:
//   - groupIndex: The index of the execution group (0-based)
//   - session: The session providing execution order and task info
//   - baseSession: The base session providing instance info
//   - parser: The completion file parser
//
// Returns an types.AggregatedTaskContext containing all gathered information.
func (o *ConsolidationOrchestrator) GatherTaskCompletionContextForGroup(
	groupIndex int,
	session GroupConsolidationSessionInterface,
	baseSession GroupConsolidationBaseSessionInterface,
	parser TaskCompletionFileParser,
) *types.AggregatedTaskContext {
	executionOrder := session.GetPlanExecutionOrder()
	if groupIndex >= len(executionOrder) {
		return &types.AggregatedTaskContext{TaskSummaries: make(map[string]string)}
	}

	taskIDs := executionOrder[groupIndex]
	context := &types.AggregatedTaskContext{
		TaskSummaries:  make(map[string]string),
		AllIssues:      make([]string, 0),
		AllSuggestions: make([]string, 0),
		Dependencies:   make([]string, 0),
		Notes:          make([]string, 0),
	}

	seenDeps := make(map[string]bool)

	for _, taskID := range taskIDs {
		// Find the instance for this task
		inst := baseSession.GetInstanceByTask(taskID)
		if inst == nil {
			continue
		}

		worktreePath := inst.GetWorktreePath()
		if worktreePath == "" {
			continue
		}

		// Try to read the completion file
		completion, err := parser.ParseTaskCompletionFile(worktreePath)
		if err != nil {
			o.logger.Debug("failed to parse task completion file",
				"task_id", taskID,
				"worktree", worktreePath,
				"error", err.Error(),
			)
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

	o.logger.Debug("gathered task completion context",
		"group_index", groupIndex,
		"task_count", len(taskIDs),
		"summaries_collected", len(context.TaskSummaries),
		"issues_count", len(context.AllIssues),
	)

	return context
}

// GetTaskBranchesForGroup returns the branches and worktree info for all tasks in a group.
//
// This method finds the instance for each task in the group and collects:
//   - Task ID and title
//   - Worktree path
//   - Branch name
//
// Parameters:
//   - groupIndex: The index of the execution group (0-based)
//   - session: The session providing execution order and task info
//   - baseSession: The base session providing instance info
//
// Returns a slice of types.ConsolidationTaskWorktreeInfo for each task with an active instance.
func (o *ConsolidationOrchestrator) GetTaskBranchesForGroup(
	groupIndex int,
	session GroupConsolidationSessionInterface,
	baseSession GroupConsolidationBaseSessionInterface,
) []types.ConsolidationTaskWorktreeInfo {
	executionOrder := session.GetPlanExecutionOrder()
	if groupIndex >= len(executionOrder) {
		return nil
	}

	taskIDs := executionOrder[groupIndex]
	var branches []types.ConsolidationTaskWorktreeInfo

	for _, taskID := range taskIDs {
		task := session.GetTask(taskID)
		if task == nil {
			continue
		}

		// Extract task title from the task object
		taskTitle := extractTaskTitle(task)

		// Find the instance for this task
		inst := baseSession.GetInstanceByTask(taskID)
		if inst != nil {
			branches = append(branches, types.ConsolidationTaskWorktreeInfo{
				TaskID:       taskID,
				TaskTitle:    taskTitle,
				WorktreePath: inst.GetWorktreePath(),
				Branch:       inst.GetBranch(),
			})
		}
	}

	o.logger.Debug("retrieved task branches for group",
		"group_index", groupIndex,
		"task_count", len(taskIDs),
		"branches_found", len(branches),
	)

	return branches
}

// BuildGroupConsolidatorPrompt builds the prompt for a per-group consolidator session.
//
// The prompt includes:
//   - Group header and plan context
//   - List of tasks completed in this group with their branches
//   - Context from task completion files (notes, issues, suggestions)
//   - Context from previous group's consolidator (if applicable)
//   - Branch configuration (base branch, target branch)
//   - Detailed instructions for cherry-picking and verification
//   - Completion protocol with JSON schema
//
// Parameters:
//   - groupIndex: The index of the execution group (0-based)
//   - session: The session providing plan and branch info
//   - baseSession: The base session providing instance info
//   - orch: The orchestrator providing branch configuration
//   - parser: The completion file parser
//
// Returns the complete prompt string for the group consolidator.
func (o *ConsolidationOrchestrator) BuildGroupConsolidatorPrompt(
	groupIndex int,
	session GroupConsolidationSessionInterface,
	baseSession GroupConsolidationBaseSessionInterface,
	orch GroupConsolidationOrchestratorInterface,
	parser TaskCompletionFileParser,
) string {
	// Gather context from task completion files
	taskContext := o.GatherTaskCompletionContextForGroup(groupIndex, session, baseSession, parser)

	// Get task branches for this group
	taskBranches := o.GetTaskBranchesForGroup(groupIndex, session, baseSession)

	// Determine base branch
	baseBranch := o.determineBaseBranchForGroup(groupIndex, session, orch)

	// Generate consolidated branch name
	consolidatedBranch := o.generateGroupBranchName(groupIndex, session, orch)

	// Build the prompt
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Group %d Consolidation\n\n", groupIndex+1))
	sb.WriteString(fmt.Sprintf("## Part of Ultra-Plan: %s\n\n", session.GetPlanSummary()))

	sb.WriteString("## Objective\n\n")
	sb.WriteString("Consolidate all completed task branches from this group into a single stable branch.\n")
	sb.WriteString("You must resolve any merge conflicts, verify the consolidated code works, and pass context to the next group.\n\n")

	// Tasks completed in this group
	sb.WriteString("## Tasks Completed in This Group\n\n")
	for _, branch := range taskBranches {
		sb.WriteString(fmt.Sprintf("### %s: %s\n", branch.TaskID, branch.TaskTitle))
		sb.WriteString(fmt.Sprintf("- Branch: `%s`\n", branch.Branch))
		sb.WriteString(fmt.Sprintf("- Worktree: `%s`\n", branch.WorktreePath))
		if summary, ok := taskContext.TaskSummaries[branch.TaskID]; ok && summary != "" {
			sb.WriteString(fmt.Sprintf("- Summary: %s\n", summary))
		}
		sb.WriteString("\n")
	}

	// Context from task completion files
	if len(taskContext.Notes) > 0 {
		sb.WriteString("## Implementation Notes from Tasks\n\n")
		for _, note := range taskContext.Notes {
			sb.WriteString(fmt.Sprintf("- %s\n", note))
		}
		sb.WriteString("\n")
	}

	if len(taskContext.AllIssues) > 0 {
		sb.WriteString("## Issues Raised by Tasks\n\n")
		for _, issue := range taskContext.AllIssues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		sb.WriteString("\n")
	}

	if len(taskContext.AllSuggestions) > 0 {
		sb.WriteString("## Integration Suggestions from Tasks\n\n")
		for _, suggestion := range taskContext.AllSuggestions {
			sb.WriteString(fmt.Sprintf("- %s\n", suggestion))
		}
		sb.WriteString("\n")
	}

	// Context from previous group's consolidator
	if groupIndex > 0 {
		contexts := session.GetGroupConsolidationContexts()
		if groupIndex-1 < len(contexts) {
			prevContext := contexts[groupIndex-1]
			if prevContext != nil {
				sb.WriteString("## Context from Previous Group's Consolidator\n\n")
				if prevContext.Notes != "" {
					sb.WriteString(fmt.Sprintf("**Notes**: %s\n\n", prevContext.Notes))
				}
				if len(prevContext.IssuesForNextGroup) > 0 {
					sb.WriteString("**Issues/Warnings to Address**:\n")
					for _, issue := range prevContext.IssuesForNextGroup {
						sb.WriteString(fmt.Sprintf("- %s\n", issue))
					}
					sb.WriteString("\n")
				}
			}
		}
	}

	// Branch configuration
	sb.WriteString("## Branch Configuration\n\n")
	sb.WriteString(fmt.Sprintf("- **Base branch**: `%s`\n", baseBranch))
	sb.WriteString(fmt.Sprintf("- **Target consolidated branch**: `%s`\n", consolidatedBranch))
	sb.WriteString(fmt.Sprintf("- **Task branches to consolidate**: %d\n\n", len(taskBranches)))

	// Instructions
	o.writeGroupConsolidatorInstructions(&sb, consolidatedBranch, baseBranch)

	// Completion protocol
	o.writeGroupCompletionProtocol(&sb, groupIndex, consolidatedBranch)

	return sb.String()
}

// StartGroupConsolidatorSession creates and starts a backend session for consolidating a group.
//
// This method:
//  1. Validates the group has tasks with verified commits
//  2. Builds the consolidator prompt
//  3. Creates an instance from the appropriate base branch
//  4. Adds the instance to the ultraplan group for sidebar display
//  5. Records the instance ID in the session
//  6. Starts the instance
//  7. Monitors the instance synchronously until completion
//
// Parameters:
//   - groupIndex: The index of the execution group (0-based)
//   - session: The session providing plan and config info
//   - baseSession: The base session for instance management
//   - orch: The orchestrator for instance operations
//   - parser: The completion file parser
//   - eventEmitter: For emitting progress events (optional, may be nil)
//
// Returns an error if:
//   - The group index is invalid
//   - No tasks have verified commits
//   - Instance creation or startup fails
//   - The consolidator instance fails during execution
func (o *ConsolidationOrchestrator) StartGroupConsolidatorSession(
	groupIndex int,
	session GroupConsolidationSessionInterface,
	baseSession GroupConsolidationBaseSessionInterface,
	orch GroupConsolidationOrchestratorInterface,
	parser TaskCompletionFileParser,
	eventEmitter GroupConsolidationEventEmitter,
) error {
	executionOrder := session.GetPlanExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return fmt.Errorf("invalid group index: %d", groupIndex)
	}

	taskIDs := executionOrder[groupIndex]
	if len(taskIDs) == 0 {
		o.logger.Debug("skipping empty group", "group_index", groupIndex)
		return nil // Empty group, nothing to consolidate
	}

	// Check if there are any tasks with verified commits
	commitCounts := session.GetTaskCommitCounts()
	var activeTasks []string
	for _, taskID := range taskIDs {
		commitCount, ok := commitCounts[taskID]
		if ok && commitCount > 0 {
			activeTasks = append(activeTasks, taskID)
		}
	}

	if len(activeTasks) == 0 {
		return fmt.Errorf("no task branches with verified commits found for group %d", groupIndex)
	}

	o.logger.Info("starting group consolidator session",
		"group_index", groupIndex,
		"total_tasks", len(taskIDs),
		"active_tasks", len(activeTasks),
	)

	// Build the consolidator prompt
	prompt := o.BuildGroupConsolidatorPrompt(groupIndex, session, baseSession, orch, parser)

	// Determine base branch for the consolidator's worktree
	baseBranch := o.determineBaseBranchForGroup(groupIndex, session, orch)

	// Create the consolidator instance
	var inst InstanceInterface
	var err error
	if baseBranch != "" {
		inst, err = orch.AddInstanceFromBranch(baseSession, prompt, baseBranch)
	} else {
		instAny, addErr := orch.AddInstance(baseSession, prompt)
		if addErr == nil {
			inst, _ = instAny.(InstanceInterface)
		}
		err = addErr
	}
	if err != nil {
		return fmt.Errorf("failed to create group consolidator instance: %w", err)
	}

	// Add consolidator instance to the ultraplan group for sidebar display
	sessionType := "ultraplan"
	if session.IsMultiPass() {
		sessionType = "plan_multi"
	}
	if ultraGroup := baseSession.GetGroupBySessionType(sessionType); ultraGroup != nil {
		ultraGroup.AddInstance(inst.GetID())
	}

	// Store the consolidator instance ID
	session.SetGroupConsolidatorID(groupIndex, inst.GetID())

	// Save state
	_ = orch.SaveSession()

	// Emit event
	if eventEmitter != nil {
		eventEmitter.EmitGroupConsolidationEvent("group_consolidator_started", groupIndex,
			fmt.Sprintf("Starting group %d consolidator session", groupIndex+1))
	}

	// Start the instance
	if err := orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start group consolidator instance: %w", err)
	}

	// Monitor the consolidator synchronously (blocks until completion)
	return o.MonitorGroupConsolidator(groupIndex, inst.GetID(), session, orch, parser, eventEmitter)
}

// MonitorGroupConsolidator monitors the group consolidator instance and waits for completion.
//
// This method polls the instance status and completion file until:
//   - The completion file is written and indicates success
//   - The completion file indicates failure
//   - The instance terminates without a completion file
//   - The context is cancelled
//
// On success, it:
//   - Stores the consolidated branch name in the session
//   - Stores the consolidation context for the next group
//   - Stops the consolidator instance to free resources
//
// Parameters:
//   - groupIndex: The index of the execution group (0-based)
//   - instanceID: The ID of the consolidator instance
//   - session: The session for storing results
//   - orch: The orchestrator for instance management
//   - parser: The completion file parser
//   - eventEmitter: For emitting progress events (optional, may be nil)
//
// Returns an error if:
//   - The instance fails or terminates unexpectedly
//   - The completion file indicates failure
//   - The context is cancelled
func (o *ConsolidationOrchestrator) MonitorGroupConsolidator(
	groupIndex int,
	instanceID string,
	session GroupConsolidationSessionInterface,
	orch GroupConsolidationOrchestratorInterface,
	parser TaskCompletionFileParser,
	eventEmitter GroupConsolidationEventEmitter,
) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return fmt.Errorf("context cancelled")

		case <-ticker.C:
			inst := orch.GetInstance(instanceID)
			if inst == nil {
				return fmt.Errorf("consolidator instance not found")
			}

			// Check for the completion file
			worktreePath := inst.GetWorktreePath()
			if worktreePath != "" {
				completionPath := parser.GroupConsolidationCompletionFilePath(worktreePath)
				if fileExists(completionPath) {
					// Parse the completion file
					completion, err := parser.ParseGroupConsolidationCompletionFile(worktreePath)
					if err != nil {
						// File exists but is invalid/incomplete - might still be writing
						// Continue monitoring and try again on next tick
						o.logger.Debug("completion file exists but couldn't be parsed",
							"error", err.Error(),
						)
						continue
					}

					// Check status
					if completion.Status == "failed" {
						// Stop the consolidator instance even on failure
						_ = orch.StopInstance(inst)
						return fmt.Errorf("group %d consolidation failed: %s", groupIndex+1, completion.Notes)
					}

					// Store the consolidated branch
					session.SetGroupConsolidatedBranch(groupIndex, completion.BranchName)

					// Store the consolidation context for the next group
					session.SetGroupConsolidationContext(groupIndex, completion)

					// Persist state
					_ = orch.SaveSession()

					// Stop the consolidator instance to free up resources
					_ = orch.StopInstance(inst)

					// Emit success event
					if eventEmitter != nil {
						eventEmitter.EmitGroupConsolidationEvent("group_consolidation_complete", groupIndex,
							fmt.Sprintf("Group %d consolidated into %s (verification: %v)",
								groupIndex+1, completion.BranchName, completion.Verification.OverallSuccess))
					}

					o.logger.Info("group consolidation completed",
						"group_index", groupIndex,
						"branch", completion.BranchName,
						"verification_success", completion.Verification.OverallSuccess,
					)

					return nil
				}
			}

			// Check if instance has failed/exited without completion file
			status := inst.GetStatus()
			switch status {
			case StatusError:
				return fmt.Errorf("consolidator instance failed with error")
			case StatusCompleted:
				// Check if tmux session still exists
				if orch.TmuxSessionExists(instanceID) {
					// Still running, keep monitoring
					continue
				}
				// Instance completed without writing completion file
				return fmt.Errorf("consolidator completed without writing completion file")
			}
		}
	}
}

// ConsolidateGroupWithVerification consolidates a group and verifies commits exist.
// This is a direct consolidation method that performs cherry-picks programmatically
// rather than spawning a backend instance.
//
// This method:
//  1. Validates the group index and finds tasks with verified commits
//  2. Creates a consolidated branch from the appropriate base
//  3. Creates a temporary worktree for cherry-picking
//  4. Cherry-picks commits from each task branch
//  5. Verifies the consolidated branch has commits
//  6. Pushes the consolidated branch
//  7. Records the branch in the session
//
// Parameters:
//   - groupIndex: The index of the execution group (0-based)
//   - session: The session providing plan and branch info
//   - baseSession: The base session providing instance info
//   - orch: The orchestrator for git operations
//   - eventEmitter: For emitting progress events (optional, may be nil)
//
// Returns an error if:
//   - The group index is invalid
//   - No task branches with verified commits exist
//   - Branch creation fails
//   - Cherry-pick fails (blocking error)
//   - No commits were added to the consolidated branch
func (o *ConsolidationOrchestrator) ConsolidateGroupWithVerification(
	groupIndex int,
	session GroupConsolidationSessionInterface,
	baseSession GroupConsolidationBaseSessionInterface,
	orch GroupConsolidationOrchestratorInterface,
	eventEmitter GroupConsolidationEventEmitter,
) error {
	executionOrder := session.GetPlanExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return fmt.Errorf("invalid group index: %d", groupIndex)
	}

	taskIDs := executionOrder[groupIndex]
	if len(taskIDs) == 0 {
		return nil // Empty group, nothing to consolidate
	}

	// Collect task branches for this group, filtering to only those with verified commits
	commitCounts := session.GetTaskCommitCounts()
	var taskBranches []string
	var activeTasks []string

	for _, taskID := range taskIDs {
		// Skip tasks that failed or have no commits
		commitCount, ok := commitCounts[taskID]
		if !ok || commitCount == 0 {
			continue
		}

		// Find the instance for this task
		inst := baseSession.GetInstanceByTask(taskID)
		if inst != nil {
			taskBranches = append(taskBranches, inst.GetBranch())
			activeTasks = append(activeTasks, taskID)
		}
	}

	if len(taskBranches) == 0 {
		// No branches with work - this is an error now, not silent success
		return fmt.Errorf("no task branches with verified commits found for group %d", groupIndex)
	}

	o.logger.Info("consolidating group with verification",
		"group_index", groupIndex,
		"task_count", len(activeTasks),
	)

	// Generate consolidated branch name
	consolidatedBranch := o.generateGroupBranchName(groupIndex, session, orch)

	// Determine base branch
	baseBranch := o.determineBaseBranchForGroup(groupIndex, session, orch)

	// Create the consolidated branch from the base
	if err := orch.CreateBranchFrom(consolidatedBranch, baseBranch); err != nil {
		return fmt.Errorf("failed to create consolidated branch %s: %w", consolidatedBranch, err)
	}

	// Create a temporary worktree for cherry-picking
	worktreeBase := fmt.Sprintf("%s/consolidation-group-%d", orch.GetClaudioDir(), groupIndex)
	if err := orch.CreateWorktreeFromBranch(worktreeBase, consolidatedBranch); err != nil {
		return fmt.Errorf("failed to create consolidation worktree: %w", err)
	}
	defer func() {
		_ = orch.RemoveWorktree(worktreeBase)
	}()

	// Cherry-pick commits from each task branch - failures are now blocking
	for i, branch := range taskBranches {
		if err := orch.CherryPickBranch(worktreeBase, branch); err != nil {
			// Cherry-pick failed - this is now a blocking error
			_ = orch.AbortCherryPick(worktreeBase)
			return fmt.Errorf("failed to cherry-pick task %s (branch %s): %w", activeTasks[i], branch, err)
		}
	}

	// Verify the consolidated branch has commits
	consolidatedCommitCount, err := orch.CountCommitsBetween(worktreeBase, baseBranch, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to verify consolidated branch commits: %w", err)
	}

	if consolidatedCommitCount == 0 {
		return fmt.Errorf("consolidated branch has no commits after cherry-picking %d branches", len(taskBranches))
	}

	// Push the consolidated branch
	if err := orch.Push(worktreeBase, false); err != nil {
		if eventEmitter != nil {
			eventEmitter.EmitGroupConsolidationEvent("group_push_warning", groupIndex,
				fmt.Sprintf("Warning: failed to push consolidated branch %s: %v", consolidatedBranch, err))
		}
		// Not fatal - branch exists locally
	}

	// Store the consolidated branch
	session.SetGroupConsolidatedBranch(groupIndex, consolidatedBranch)

	// Emit success event
	if eventEmitter != nil {
		eventEmitter.EmitGroupConsolidationEvent("group_consolidation_complete", groupIndex,
			fmt.Sprintf("Group %d consolidated into %s (%d commits from %d tasks)",
				groupIndex+1, consolidatedBranch, consolidatedCommitCount, len(taskBranches)))
	}

	o.logger.Info("group consolidated with verification",
		"group_index", groupIndex,
		"branch", consolidatedBranch,
		"commit_count", consolidatedCommitCount,
		"task_count", len(activeTasks),
	)

	return nil
}

// ============================================================================
// Per-Group Consolidation Helper Methods
// ============================================================================

// determineBaseBranchForGroup returns the base branch that the consolidated branch
// should be created from. For group 0, this is the main branch. For other groups,
// it's the consolidated branch from the previous group.
func (o *ConsolidationOrchestrator) determineBaseBranchForGroup(
	groupIndex int,
	session GroupConsolidationSessionInterface,
	orch GroupConsolidationOrchestratorInterface,
) string {
	if groupIndex == 0 {
		return orch.FindMainBranch()
	}

	// Check if we have a consolidated branch from the previous group
	consolidatedBranches := session.GetGroupConsolidatedBranches()
	previousGroupIndex := groupIndex - 1
	if previousGroupIndex < len(consolidatedBranches) {
		if branch := consolidatedBranches[previousGroupIndex]; branch != "" {
			return branch
		}
	}

	return orch.FindMainBranch()
}

// generateGroupBranchName generates the branch name for a consolidated group.
func (o *ConsolidationOrchestrator) generateGroupBranchName(
	groupIndex int,
	session GroupConsolidationSessionInterface,
	orch GroupConsolidationOrchestratorInterface,
) string {
	branchPrefix := session.GetBranchPrefix()
	if branchPrefix == "" {
		branchPrefix = orch.BranchPrefix()
	}
	if branchPrefix == "" {
		branchPrefix = "Iron-Ham"
	}

	planID := session.GetID()
	if len(planID) > 8 {
		planID = planID[:8]
	}

	return fmt.Sprintf("%s/ultraplan-%s-group-%d", branchPrefix, planID, groupIndex+1)
}

// writeGroupConsolidatorInstructions writes the detailed instructions section
// for the group consolidator prompt.
func (o *ConsolidationOrchestrator) writeGroupConsolidatorInstructions(
	sb *strings.Builder,
	consolidatedBranch, baseBranch string,
) {
	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. **Create the consolidated branch** from the base branch:\n")
	fmt.Fprintf(sb, "   ```bash\n   git checkout -b %s %s\n   ```\n\n", consolidatedBranch, baseBranch)

	sb.WriteString("2. **Cherry-pick commits** from each task branch in order. For each branch:\n")
	sb.WriteString("   - Review the commits on the branch\n")
	sb.WriteString("   - Cherry-pick them onto the consolidated branch\n")
	sb.WriteString("   - Resolve any conflicts intelligently using your understanding of the code\n\n")

	sb.WriteString("3. **Run verification** to ensure the consolidated code is stable:\n")
	sb.WriteString("   - Detect the project type (Go, Node, Python, iOS, etc.)\n")
	sb.WriteString("   - Run appropriate build/compile commands\n")
	sb.WriteString("   - Run linting if available\n")
	sb.WriteString("   - Run tests if available\n")
	sb.WriteString("   - Fix any issues that arise\n\n")

	sb.WriteString("4. **Push the consolidated branch** to the remote\n\n")

	sb.WriteString("5. **Write the completion file** to signal success\n\n")

	// Conflict resolution guidelines
	sb.WriteString("## Conflict Resolution Guidelines\n\n")
	sb.WriteString("- Prefer changes that preserve functionality from all tasks\n")
	sb.WriteString("- If there are conflicting approaches, choose the more robust one\n")
	sb.WriteString("- Document your resolution reasoning in the completion file\n")
	sb.WriteString("- If you cannot resolve a conflict, document it as an issue\n\n")
}

// writeGroupCompletionProtocol writes the completion protocol section
// for the group consolidator prompt.
func (o *ConsolidationOrchestrator) writeGroupCompletionProtocol(
	sb *strings.Builder,
	groupIndex int,
	consolidatedBranch string,
) {
	sb.WriteString("## Completion Protocol\n\n")
	fmt.Fprintf(sb, "When consolidation is complete, write `%s` in your worktree root:\n\n", types.GroupConsolidationCompletionFileName)
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	fmt.Fprintf(sb, "  \"group_index\": %d,\n", groupIndex)
	sb.WriteString("  \"status\": \"complete\",\n")
	fmt.Fprintf(sb, "  \"branch_name\": \"%s\",\n", consolidatedBranch)
	sb.WriteString("  \"tasks_consolidated\": [\"task-id-1\", \"task-id-2\"],\n")
	sb.WriteString("  \"conflicts_resolved\": [\n")
	sb.WriteString("    {\"file\": \"path/to/file.go\", \"resolution\": \"Kept both changes, merged logic\"}\n")
	sb.WriteString("  ],\n")
	sb.WriteString("  \"verification\": {\n")
	sb.WriteString("    \"project_type\": \"go\",\n")
	sb.WriteString("    \"commands_run\": [\n")
	sb.WriteString("      {\"name\": \"build\", \"command\": \"go build ./...\", \"success\": true},\n")
	sb.WriteString("      {\"name\": \"lint\", \"command\": \"golangci-lint run\", \"success\": true},\n")
	sb.WriteString("      {\"name\": \"test\", \"command\": \"go test ./...\", \"success\": true}\n")
	sb.WriteString("    ],\n")
	sb.WriteString("    \"overall_success\": true,\n")
	sb.WriteString("    \"summary\": \"All checks passed\"\n")
	sb.WriteString("  },\n")
	sb.WriteString("  \"notes\": \"Any observations about the consolidated code\",\n")
	sb.WriteString("  \"issues_for_next_group\": [\"Any warnings or concerns for the next group\"]\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("Use status \"failed\" if consolidation cannot be completed.\n")
}

// extractTaskTitle extracts the title from a task object.
// This is a helper function that handles different task representations.
func extractTaskTitle(task any) string {
	// Try type assertion to a map
	if taskMap, ok := task.(map[string]any); ok {
		if title, ok := taskMap["title"].(string); ok {
			return title
		}
		if title, ok := taskMap["Title"].(string); ok {
			return title
		}
	}

	// Try to use reflection for struct types
	// This handles the case where task is a pointer to a struct
	type titleGetter interface {
		GetTitle() string
	}
	if tg, ok := task.(titleGetter); ok {
		return tg.GetTitle()
	}

	// Fallback
	return "Unknown Task"
}

// fileExists checks if a file exists at the given path.
func fileExists(path string) bool {
	_, err := statFile(path)
	return err == nil
}

// statFile is a variable to allow mocking in tests.
var statFile = defaultStatFile

func defaultStatFile(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
