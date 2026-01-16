// Package callback provides centralized callback dispatch for coordinator events.
// It encapsulates the notification logic that was previously scattered throughout
// the Coordinator, making it easier to test and maintain.
package callback

import (
	"sync"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// UltraPlanPhase represents the current phase of an ultra-plan session.
// Defined locally to avoid circular imports with the orchestrator package.
type UltraPlanPhase string

// Phase constants.
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

// PlanSpec is a type alias allowing the dispatcher to accept any plan type,
// avoiding circular imports with the orchestrator package.
type PlanSpec = any

// Callbacks holds all callback functions for coordinator events.
// Each field is optional - nil callbacks are safely skipped.
type Callbacks struct {
	// OnPhaseChange is called when the ultra-plan phase changes
	OnPhaseChange func(phase UltraPlanPhase)

	// OnTaskStart is called when a task begins execution
	OnTaskStart func(taskID, instanceID string)

	// OnTaskComplete is called when a task completes successfully
	OnTaskComplete func(taskID string)

	// OnTaskFailed is called when a task fails
	OnTaskFailed func(taskID, reason string)

	// OnGroupComplete is called when an execution group completes
	OnGroupComplete func(groupIndex int)

	// OnPlanReady is called when the plan is ready (after planning phase)
	OnPlanReady func(plan PlanSpec)

	// OnProgress is called periodically with progress updates
	OnProgress func(completed, total int, phase UltraPlanPhase)

	// OnComplete is called when the entire ultra-plan completes
	OnComplete func(success bool, summary string)
}

// SessionPersister provides session persistence operations.
// This interface allows the Dispatcher to save session state after
// certain notifications without creating circular dependencies.
type SessionPersister interface {
	// SaveSession persists the current session state
	SaveSession() error
}

// Dispatcher provides centralized callback dispatch for coordinator events.
// It handles thread-safe callback invocation, logging, and optional session
// persistence after significant events.
//
// The Dispatcher is designed to be a thin layer that:
//   - Provides thread-safe callback access
//   - Logs events before dispatch for observability
//   - Optionally persists session state after key events
//   - Safely handles nil callbacks
type Dispatcher struct {
	callbacks *Callbacks
	persister SessionPersister
	logger    *logging.Logger
	mu        sync.RWMutex
}

// NewDispatcher creates a new Dispatcher with the given dependencies.
// The persister parameter is optional - if nil, no session persistence occurs.
// The logger parameter is optional - if nil, a no-op logger is used.
func NewDispatcher(persister SessionPersister, logger *logging.Logger) *Dispatcher {
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &Dispatcher{
		persister: persister,
		logger:    logger.WithPhase("callback-dispatcher"),
	}
}

// SetCallbacks sets or updates the callback functions.
// This is thread-safe and can be called at any time.
func (d *Dispatcher) SetCallbacks(cb *Callbacks) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.callbacks = cb
}

// GetCallbacks returns the current callback configuration.
// Returns nil if no callbacks have been set.
func (d *Dispatcher) GetCallbacks() *Callbacks {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.callbacks
}

// NotifyPhaseChange notifies callbacks that the phase has changed.
// It logs the transition and optionally persists session state.
func (d *Dispatcher) NotifyPhaseChange(fromPhase, toPhase UltraPlanPhase, sessionID string) {
	d.logger.Info("phase changed",
		"from_phase", string(fromPhase),
		"to_phase", string(toPhase),
		"session_id", sessionID,
	)

	// Persist phase change
	if d.persister != nil {
		if err := d.persister.SaveSession(); err != nil {
			d.logger.Warn("failed to persist session after phase change",
				"error", err.Error(),
				"from_phase", string(fromPhase),
				"to_phase", string(toPhase),
			)
		}
	}

	d.mu.RLock()
	cb := d.callbacks
	d.mu.RUnlock()

	if cb != nil && cb.OnPhaseChange != nil {
		cb.OnPhaseChange(toPhase)
	}
}

// NotifyTaskStart notifies callbacks that a task has started.
func (d *Dispatcher) NotifyTaskStart(taskID, instanceID, taskTitle string) {
	d.logger.Info("task started",
		"task_id", taskID,
		"instance_id", instanceID,
		"task_title", taskTitle,
	)

	d.mu.RLock()
	cb := d.callbacks
	d.mu.RUnlock()

	if cb != nil && cb.OnTaskStart != nil {
		cb.OnTaskStart(taskID, instanceID)
	}
}

// NotifyTaskComplete notifies callbacks that a task has completed successfully.
// It persists session state to record the completion.
func (d *Dispatcher) NotifyTaskComplete(taskID string) {
	d.logger.Info("task completed",
		"task_id", taskID,
	)

	// Persist task completion
	if d.persister != nil {
		if err := d.persister.SaveSession(); err != nil {
			d.logger.Warn("failed to persist session after task completion",
				"error", err.Error(),
				"task_id", taskID,
			)
		}
	}

	d.mu.RLock()
	cb := d.callbacks
	d.mu.RUnlock()

	if cb != nil && cb.OnTaskComplete != nil {
		cb.OnTaskComplete(taskID)
	}
}

// NotifyTaskFailed notifies callbacks that a task has failed.
// It persists session state to record the failure.
func (d *Dispatcher) NotifyTaskFailed(taskID, reason string) {
	d.logger.Info("task failed",
		"task_id", taskID,
		"reason", reason,
	)

	// Persist task failure
	if d.persister != nil {
		if err := d.persister.SaveSession(); err != nil {
			d.logger.Warn("failed to persist session after task failure",
				"error", err.Error(),
				"task_id", taskID,
			)
		}
	}

	d.mu.RLock()
	cb := d.callbacks
	d.mu.RUnlock()

	if cb != nil && cb.OnTaskFailed != nil {
		cb.OnTaskFailed(taskID, reason)
	}
}

// NotifyGroupComplete notifies callbacks that an execution group has completed.
// It persists session state to record the group advancement.
func (d *Dispatcher) NotifyGroupComplete(groupIndex, taskCount int) {
	d.logger.Info("group completed",
		"group_index", groupIndex,
		"task_count", taskCount,
	)

	d.mu.RLock()
	cb := d.callbacks
	d.mu.RUnlock()

	if cb != nil && cb.OnGroupComplete != nil {
		cb.OnGroupComplete(groupIndex)
	}

	// Persist group advancement
	if d.persister != nil {
		if err := d.persister.SaveSession(); err != nil {
			d.logger.Warn("failed to persist session after group completion",
				"error", err.Error(),
				"group_index", groupIndex,
			)
		}
	}
}

// NotifyPlanReady notifies callbacks that the plan is ready.
func (d *Dispatcher) NotifyPlanReady(plan PlanSpec, taskCount, groupCount int) {
	d.logger.Info("plan ready",
		"task_count", taskCount,
		"group_count", groupCount,
	)

	d.mu.RLock()
	cb := d.callbacks
	d.mu.RUnlock()

	if cb != nil && cb.OnPlanReady != nil {
		cb.OnPlanReady(plan)
	}
}

// NotifyProgress notifies callbacks of progress updates.
// This is called at DEBUG level to avoid log spam.
func (d *Dispatcher) NotifyProgress(completed, total int, phase UltraPlanPhase) {
	d.logger.Debug("progress update",
		"completed", completed,
		"total", total,
		"phase", string(phase),
	)

	d.mu.RLock()
	cb := d.callbacks
	d.mu.RUnlock()

	if cb != nil && cb.OnProgress != nil {
		cb.OnProgress(completed, total, phase)
	}
}

// NotifyComplete notifies callbacks that the ultra-plan has completed.
func (d *Dispatcher) NotifyComplete(success bool, summary string) {
	d.logger.Info("coordinator complete",
		"success", success,
		"summary", summary,
	)

	d.mu.RLock()
	cb := d.callbacks
	d.mu.RUnlock()

	if cb != nil && cb.OnComplete != nil {
		cb.OnComplete(success, summary)
	}
}
