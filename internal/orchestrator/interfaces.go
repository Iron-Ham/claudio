// Package orchestrator provides interfaces and implementations for managing
// Claudio sessions, instances, and plan execution.
package orchestrator

import (
	"context"

	"github.com/Iron-Ham/claudio/internal/instance"
)

// SessionManager defines the interface for session lifecycle management.
// Sessions represent a working context that groups multiple Claude instances
// together for coordinated work.
type SessionManager interface {
	// Create starts a new session with the given name.
	// Returns the created session or an error if creation fails.
	Create(ctx context.Context, name string) (*Session, error)

	// Get retrieves an existing session by its ID.
	// Returns nil if the session does not exist.
	Get(ctx context.Context, sessionID string) (*Session, error)

	// List returns all available sessions.
	// May return an empty slice if no sessions exist.
	List(ctx context.Context) ([]*Session, error)

	// Delete removes a session and optionally cleans up associated resources.
	// If force is true, removes the session even if instances have uncommitted changes.
	Delete(ctx context.Context, sessionID string, force bool) error

	// Persist saves the current session state to disk.
	// This should be called after significant state changes.
	Persist(ctx context.Context, session *Session) error

	// Restore loads a previously saved session from disk.
	// Returns the restored session and a list of instance IDs that were reconnected.
	Restore(ctx context.Context, sessionID string) (*Session, []string, error)
}

// InstanceCoordinator defines the interface for managing Claude instances
// within a session. Instances are individual Claude processes running in
// isolated git worktrees.
type InstanceCoordinator interface {
	// AddInstance creates a new instance for the given task.
	// The instance is added to the session but not started.
	AddInstance(ctx context.Context, session *Session, task string) (*Instance, error)

	// AddInstanceFromBranch creates a new instance with a worktree based on
	// the specified branch. This is used for tasks that need to build on
	// previous work (e.g., dependent groups in an ultraplan).
	AddInstanceFromBranch(ctx context.Context, session *Session, task string, baseBranch string) (*Instance, error)

	// AddInstanceToWorktree creates a new instance that reuses an existing
	// worktree. This is used for revision tasks that modify existing work.
	AddInstanceToWorktree(ctx context.Context, session *Session, task string, worktreePath, branch string) (*Instance, error)

	// RemoveInstance stops and removes a specific instance from the session.
	// If force is true, removes even if the instance has uncommitted changes.
	RemoveInstance(ctx context.Context, session *Session, instanceID string, force bool) error

	// GetInstance retrieves an instance by its ID.
	// Returns nil if the instance does not exist.
	GetInstance(ctx context.Context, instanceID string) *Instance

	// GetInstanceManager returns the underlying instance manager for direct
	// control of the instance's process. Returns nil if not found.
	GetInstanceManager(ctx context.Context, instanceID string) *instance.Manager

	// ListInstances returns all instances in the given session.
	ListInstances(ctx context.Context, session *Session) []*Instance

	// StartAll starts all pending instances in the session.
	// Returns the number of instances started and any error encountered.
	StartAll(ctx context.Context, session *Session) (int, error)

	// StopAll stops all running instances in the session.
	// Returns the number of instances stopped and any error encountered.
	StopAll(ctx context.Context, session *Session) (int, error)

	// StartInstance starts a specific instance's Claude process.
	StartInstance(ctx context.Context, inst *Instance) error

	// StopInstance stops a specific instance's Claude process.
	StopInstance(ctx context.Context, inst *Instance) error

	// ReconnectInstance attempts to reconnect to a stopped or paused instance.
	// If the tmux session still exists, reconnects to it; otherwise restarts.
	ReconnectInstance(ctx context.Context, inst *Instance) error
}

// PRManager defines the interface for PR workflow management.
// This handles the creation and tracking of pull requests from instance work.
type PRManager interface {
	// CreatePR starts the PR workflow for an instance.
	// This handles committing, pushing, and creating the pull request.
	CreatePR(ctx context.Context, inst *Instance) error

	// ListPRs returns all PR workflows currently in progress.
	ListPRs(ctx context.Context) map[string]*instance.PRWorkflow

	// GetPRWorkflow returns the PR workflow for a specific instance.
	// Returns nil if no workflow exists for the instance.
	GetPRWorkflow(ctx context.Context, instanceID string) *instance.PRWorkflow

	// GetPRStatus returns the current status of a PR workflow.
	GetPRStatus(ctx context.Context, instanceID string) (PRStatus, error)

	// StopPRWorkflow stops a running PR workflow.
	StopPRWorkflow(ctx context.Context, instanceID string) error

	// SetPRCompleteCallback sets the callback invoked when a PR workflow completes.
	SetPRCompleteCallback(callback func(instanceID string, success bool))

	// SetPROpenedCallback sets the callback invoked when a PR URL is detected.
	SetPROpenedCallback(callback func(instanceID string))
}

// PRStatus represents the status of a pull request workflow.
type PRStatus string

const (
	// PRStatusPending indicates the PR workflow has not started.
	PRStatusPending PRStatus = "pending"

	// PRStatusInProgress indicates the PR workflow is currently running.
	PRStatusInProgress PRStatus = "in_progress"

	// PRStatusComplete indicates the PR was successfully created.
	PRStatusComplete PRStatus = "complete"

	// PRStatusFailed indicates the PR workflow failed.
	PRStatusFailed PRStatus = "failed"
)

// PlanExecutor defines the interface for executing an ultraplan.
// The executor manages the lifecycle of plan execution including task
// scheduling, progress tracking, and state transitions.
type PlanExecutor interface {
	// Execute begins execution of the plan.
	// This is non-blocking; use GetProgress to monitor status.
	Execute(ctx context.Context) error

	// Pause temporarily halts plan execution.
	// Running tasks will complete but no new tasks will be started.
	Pause(ctx context.Context) error

	// Resume continues a paused plan execution.
	Resume(ctx context.Context) error

	// Cancel stops plan execution entirely.
	// All running tasks will be stopped.
	Cancel(ctx context.Context)

	// GetProgress returns the current execution progress.
	// Returns the number of completed tasks, total tasks, and current phase.
	GetProgress(ctx context.Context) (completed, total int, phase UltraPlanPhase)

	// GetRunningTasks returns the currently executing tasks and their instance IDs.
	GetRunningTasks(ctx context.Context) map[string]string

	// SetPlan sets the plan to execute. Must be called before Execute.
	SetPlan(ctx context.Context, plan *PlanSpec) error

	// GetPlan returns the current plan, or nil if not set.
	GetPlan(ctx context.Context) *PlanSpec

	// Wait blocks until plan execution completes or is cancelled.
	Wait()

	// SetCallbacks configures the callbacks for execution events.
	SetCallbacks(callbacks *CoordinatorCallbacks)
}

// PhaseHandler defines the interface for handling individual phases of
// ultraplan execution. Each phase (planning, execution, synthesis, etc.)
// has its own handler that manages that phase's specific logic.
type PhaseHandler interface {
	// CanExecute returns true if this handler can execute in the current state.
	// This allows handlers to define prerequisites for their phase.
	CanExecute(ctx context.Context, session *UltraPlanSession) bool

	// Execute runs the phase's logic.
	// This may be blocking or non-blocking depending on the phase.
	Execute(ctx context.Context, session *UltraPlanSession) error

	// GetResult returns the result of the phase execution.
	// Returns nil if the phase has not completed.
	GetResult(ctx context.Context) (PhaseResult, error)

	// GetProgress returns the progress within this phase.
	// The meaning of progress values is phase-specific.
	GetProgress(ctx context.Context) PhaseProgress

	// Name returns the name/type of this phase handler.
	Name() UltraPlanPhase
}

// PhaseResult represents the outcome of a phase execution.
type PhaseResult struct {
	// Success indicates whether the phase completed successfully.
	Success bool

	// Error contains any error message if Success is false.
	Error string

	// Data contains phase-specific result data.
	// For planning: *PlanSpec
	// For synthesis: *SynthesisCompletionFile
	// For consolidation: *ConsolidationState
	Data any
}

// PhaseProgress represents progress within a phase.
type PhaseProgress struct {
	// Phase is the current phase.
	Phase UltraPlanPhase

	// Completed is the number of completed units of work.
	Completed int

	// Total is the total number of units of work.
	Total int

	// Message is an optional human-readable status message.
	Message string
}

// EventEmitter defines the interface for emitting coordinator events.
// This is used by phase handlers and the plan executor to communicate
// state changes to the UI layer.
type EventEmitter interface {
	// EmitEvent sends an event to registered listeners.
	EmitEvent(event CoordinatorEvent)

	// SetEventCallback sets the callback for receiving events.
	SetEventCallback(callback func(CoordinatorEvent))
}

// WorktreeManager defines the interface for git worktree operations.
// This abstracts the git operations needed by the orchestrator.
type WorktreeManager interface {
	// Create creates a new worktree at the given path with a new branch.
	Create(path, branch string) error

	// CreateFromBranch creates a worktree based on an existing branch.
	CreateFromBranch(path, newBranch, baseBranch string) error

	// Remove removes a worktree.
	Remove(path string) error

	// DeleteBranch deletes a git branch.
	DeleteBranch(branch string) error

	// HasUncommittedChanges checks if a worktree has uncommitted changes.
	HasUncommittedChanges(path string) (bool, error)

	// GetDiffAgainstMain returns the diff between a worktree and the main branch.
	GetDiffAgainstMain(path string) (string, error)

	// FindMainBranch returns the name of the main/master branch.
	FindMainBranch() string

	// CountCommitsBetween counts commits between two references.
	CountCommitsBetween(workdir, base, head string) (int, error)

	// CherryPickBranch cherry-picks all commits from a branch into the worktree.
	CherryPickBranch(workdir, branch string) error

	// AbortCherryPick aborts a failed cherry-pick operation.
	AbortCherryPick(workdir string) error

	// CreateBranchFrom creates a new branch from an existing branch.
	CreateBranchFrom(newBranch, baseBranch string) error

	// CreateWorktreeFromBranch creates a worktree from an existing branch.
	CreateWorktreeFromBranch(path, branch string) error
}

// ConflictDetector defines the interface for detecting conflicts between
// instances working on the same files.
type ConflictDetector interface {
	// Start begins monitoring for conflicts.
	Start()

	// Stop stops monitoring for conflicts.
	Stop()

	// AddInstance registers an instance for conflict monitoring.
	AddInstance(instanceID, worktreePath string) error

	// RemoveInstance unregisters an instance from conflict monitoring.
	RemoveInstance(instanceID string) error

	// GetConflicts returns any detected conflicts.
	GetConflicts() []Conflict
}

// Conflict represents a detected file conflict between instances.
type Conflict struct {
	// File is the path to the conflicting file.
	File string

	// InstanceIDs lists the instances that modified this file.
	InstanceIDs []string

	// DetectedAt is when the conflict was detected.
	DetectedAt string
}

// InstanceStateCallback is the callback signature for instance state changes.
type InstanceStateCallback func(instanceID string, state instance.WaitingState)

// InstanceMetricsCallback is the callback signature for instance metrics updates.
type InstanceMetricsCallback func(instanceID string, metrics *instance.ParsedMetrics)

// InstanceTimeoutCallback is the callback signature for instance timeout events.
type InstanceTimeoutCallback func(instanceID string, timeoutType instance.TimeoutType)

// InstanceBellCallback is the callback signature for terminal bell events.
type InstanceBellCallback func(instanceID string)
