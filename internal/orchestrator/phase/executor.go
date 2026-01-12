// Package phase provides abstractions for ultra-plan phase execution.
// It defines the PhaseExecutor interface that is implemented by each
// phase of the ultra-plan lifecycle: planning, execution, synthesis,
// revision, and consolidation.
package phase

import (
	"context"
	"errors"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// UltraPlanPhase represents the current phase of an ultra-plan session.
// This type is re-exported from the orchestrator package to avoid circular imports.
// Phase executors use this to identify which phase they implement.
type UltraPlanPhase string

// Phase constants match those defined in the orchestrator package.
const (
	PhasePlanning      UltraPlanPhase = "planning"
	PhasePlanSelection UltraPlanPhase = "plan_selection"
	PhaseRefresh       UltraPlanPhase = "context_refresh"
	PhaseExecuting     UltraPlanPhase = "executing"
	PhaseSynthesis     UltraPlanPhase = "synthesis"
	PhaseRevision      UltraPlanPhase = "revision"
	PhaseConsolidating UltraPlanPhase = "consolidating"
	PhaseComplete      UltraPlanPhase = "complete"
	PhaseFailed        UltraPlanPhase = "failed"
)

// PhaseExecutor defines the interface that all phase executors must implement.
// Each phase of the ultra-plan lifecycle (planning, execution, synthesis,
// revision, consolidation) has a dedicated executor that implements this interface.
//
// Executors are responsible for:
//   - Identifying their phase via Phase()
//   - Running their phase logic via Execute()
//   - Supporting graceful cancellation via Cancel()
//
// The Execute method receives a context for cancellation and should check
// ctx.Done() periodically for long-running operations.
type PhaseExecutor interface {
	// Phase returns the UltraPlanPhase that this executor handles.
	// This is used by the coordinator to verify the correct executor
	// is being invoked for the current session phase.
	Phase() UltraPlanPhase

	// Execute runs the phase logic. It should respect the provided context
	// for cancellation and return early if ctx.Done() is signaled.
	// Returns an error if the phase fails or is cancelled.
	Execute(ctx context.Context) error

	// Cancel signals the executor to stop any in-progress work.
	// This is used for immediate cancellation requests (e.g., user abort).
	// After Cancel is called, Execute should return promptly.
	// Cancel is safe to call multiple times.
	Cancel()
}

// PhaseContext holds the dependencies required by phase executors.
// It provides access to the ultra-plan manager, orchestrator, session state,
// logger, and callbacks. Each executor receives a PhaseContext when created.
//
// PhaseContext uses pointer types for its fields because these are shared
// resources that multiple executors need to coordinate through. The Validate
// method ensures all required fields are set before execution begins.
type PhaseContext struct {
	// Manager provides access to ultra-plan session management operations
	// such as setting phase, marking tasks complete, and emitting events.
	// Manager must not be nil.
	Manager UltraPlanManagerInterface

	// Orchestrator provides access to instance management, worktree operations,
	// and session persistence. Used to create/start instances and save state.
	// Orchestrator must not be nil.
	Orchestrator OrchestratorInterface

	// Session holds the current ultra-plan session state including the plan,
	// task status, and configuration. Executors read and update this state.
	// Session must not be nil.
	Session UltraPlanSessionInterface

	// Logger is used for structured logging throughout phase execution.
	// If nil, a NopLogger will be used (no logging).
	Logger *logging.Logger

	// Callbacks allows executors to notify the coordinator of events
	// such as phase changes, task completion, and progress updates.
	// May be nil if no callbacks are needed.
	Callbacks CoordinatorCallbacksInterface
}

// UltraPlanManagerInterface defines the subset of UltraPlanManager methods
// needed by phase executors. This interface allows for testing with mocks.
type UltraPlanManagerInterface interface {
	// Session returns the current ultra-plan session
	Session() UltraPlanSessionInterface

	// SetPhase updates the session phase and emits an event
	SetPhase(phase UltraPlanPhase)

	// SetPlan sets the plan on the session
	SetPlan(plan any)

	// MarkTaskComplete marks a task as completed
	MarkTaskComplete(taskID string)

	// MarkTaskFailed marks a task as failed
	MarkTaskFailed(taskID string, reason string)

	// AssignTaskToInstance records the mapping from task to instance
	AssignTaskToInstance(taskID, instanceID string)

	// Stop stops the ultra-plan execution
	Stop()
}

// OrchestratorInterface defines the subset of Orchestrator methods
// needed by phase executors. This interface allows for testing with mocks.
type OrchestratorInterface interface {
	// AddInstance adds a new instance to the session
	AddInstance(session any, task string) (any, error)

	// StartInstance starts a Claude process for an instance
	StartInstance(inst any) error

	// SaveSession persists the session state to disk
	SaveSession() error

	// GetInstanceManager returns the manager for an instance
	GetInstanceManager(id string) any

	// BranchPrefix returns the configured branch prefix
	BranchPrefix() string
}

// UltraPlanSessionInterface defines the session state methods needed by executors.
type UltraPlanSessionInterface interface {
	// GetTask returns a planned task by ID
	GetTask(taskID string) any

	// GetReadyTasks returns all tasks that are ready to execute
	GetReadyTasks() []string

	// IsCurrentGroupComplete returns true if all tasks in the current group are done
	IsCurrentGroupComplete() bool

	// AdvanceGroupIfComplete advances to the next group if current is complete
	AdvanceGroupIfComplete() (advanced bool, previousGroup int)

	// HasMoreGroups returns true if there are more groups to execute
	HasMoreGroups() bool

	// Progress returns the completion progress as a percentage
	Progress() float64
}

// CoordinatorCallbacksInterface defines the callback methods for phase events.
type CoordinatorCallbacksInterface interface {
	// OnPhaseChange is called when the ultra-plan phase changes
	OnPhaseChange(phase UltraPlanPhase)

	// OnTaskStart is called when a task begins execution
	OnTaskStart(taskID, instanceID string)

	// OnTaskComplete is called when a task completes successfully
	OnTaskComplete(taskID string)

	// OnTaskFailed is called when a task fails
	OnTaskFailed(taskID, reason string)

	// OnGroupComplete is called when an execution group completes
	OnGroupComplete(groupIndex int)

	// OnPlanReady is called when the plan is ready
	OnPlanReady(plan any)

	// OnProgress is called periodically with progress updates
	OnProgress(completed, total int, phase UltraPlanPhase)

	// OnComplete is called when the entire ultra-plan completes
	OnComplete(success bool, summary string)
}

// Validation errors returned by PhaseContext.Validate
var (
	// ErrNilManager is returned when PhaseContext.Manager is nil
	ErrNilManager = errors.New("phase context: manager is required")

	// ErrNilOrchestrator is returned when PhaseContext.Orchestrator is nil
	ErrNilOrchestrator = errors.New("phase context: orchestrator is required")

	// ErrNilSession is returned when PhaseContext.Session is nil
	ErrNilSession = errors.New("phase context: session is required")
)

// Validate checks that the PhaseContext has all required fields set.
// Returns an error describing the first missing required field, or nil if valid.
//
// Required fields:
//   - Manager: must not be nil
//   - Orchestrator: must not be nil
//   - Session: must not be nil
//
// Optional fields:
//   - Logger: if nil, executors should use logging.NopLogger()
//   - Callbacks: may be nil if no event notifications are needed
func (pc *PhaseContext) Validate() error {
	if pc.Manager == nil {
		return ErrNilManager
	}
	if pc.Orchestrator == nil {
		return ErrNilOrchestrator
	}
	if pc.Session == nil {
		return ErrNilSession
	}
	return nil
}

// GetLogger returns the Logger from the context, or a NopLogger if Logger is nil.
// This ensures executors always have a valid logger to use.
func (pc *PhaseContext) GetLogger() *logging.Logger {
	if pc.Logger != nil {
		return pc.Logger
	}
	return logging.NopLogger()
}
