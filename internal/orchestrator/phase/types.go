// Package phase provides abstractions for managing the lifecycle phases of an ultra-plan session.
// It defines a formal state machine for phase transitions with validation, constraints,
// and event notification capabilities.
package phase

import (
	"errors"
	"slices"
	"time"
)

// Phase represents a discrete stage in the ultra-plan lifecycle.
// Each phase has specific responsibilities and valid transitions to other phases.
type Phase string

const (
	// PhasePlanning is the initial phase where the objective is analyzed
	// and a plan is created by decomposing work into tasks.
	PhasePlanning Phase = "planning"

	// PhasePlanSelection is used in multi-pass planning mode to compare
	// and select the best plan from multiple candidates.
	PhasePlanSelection Phase = "plan_selection"

	// PhaseRefresh signals a context refresh period before execution begins.
	// This allows the system to update any stale context before running tasks.
	PhaseRefresh Phase = "context_refresh"

	// PhaseExecution is the main work phase where tasks are executed
	// in parallel according to the dependency graph.
	PhaseExecution Phase = "executing"

	// PhaseSynthesis analyzes the results of all completed tasks,
	// identifying any issues or integration problems.
	PhaseSynthesis Phase = "synthesis"

	// PhaseRevision addresses issues identified during synthesis
	// by executing corrective tasks.
	PhaseRevision Phase = "revision"

	// PhaseConsolidation merges completed work from task worktrees
	// into consolidated branches and creates pull requests.
	PhaseConsolidation Phase = "consolidating"

	// PhaseComplete indicates successful completion of all phases.
	PhaseComplete Phase = "complete"

	// PhaseFailed indicates the session terminated due to an error.
	PhaseFailed Phase = "failed"
)

// AllPhases returns all defined phases in lifecycle order.
func AllPhases() []Phase {
	return []Phase{
		PhasePlanning,
		PhasePlanSelection,
		PhaseRefresh,
		PhaseExecution,
		PhaseSynthesis,
		PhaseRevision,
		PhaseConsolidation,
		PhaseComplete,
		PhaseFailed,
	}
}

// IsTerminal returns true if the phase is a terminal state (Complete or Failed).
func (p Phase) IsTerminal() bool {
	return p == PhaseComplete || p == PhaseFailed
}

// String returns the string representation of the phase.
func (p Phase) String() string {
	return string(p)
}

// PhaseChangeCallback is a function called when a phase transition occurs.
type PhaseChangeCallback func(from, to Phase)

// PhaseManager defines the interface for managing phase state and transitions.
// Implementations are responsible for maintaining the current phase,
// validating transitions, and notifying observers of changes.
type PhaseManager interface {
	// CurrentPhase returns the current phase of the session.
	CurrentPhase() Phase

	// CanTransitionTo checks whether a transition to the target phase is valid
	// from the current phase. This does not consider phase-specific constraints,
	// only the validity of the transition path.
	CanTransitionTo(phase Phase) bool

	// TransitionTo attempts to transition to the specified phase.
	// Returns an error if the transition is invalid or if phase-specific
	// constraints are not satisfied.
	TransitionTo(phase Phase) error

	// OnPhaseChange registers a callback to be invoked when phase transitions occur.
	// Multiple callbacks can be registered and will be called in registration order.
	// The callback receives both the source and destination phases.
	OnPhaseChange(callback PhaseChangeCallback)

	// PhaseHistory returns the ordered list of phase transitions that have occurred.
	PhaseHistory() []PhaseTransition

	// PhaseDuration returns the duration spent in a specific phase.
	// Returns zero duration if the phase has not been entered.
	PhaseDuration(phase Phase) time.Duration

	// Config returns the configuration for a specific phase.
	// Returns nil if no configuration is set for the phase.
	Config(phase Phase) *PhaseConfig
}

// PhaseTransition captures metadata about a single phase transition.
// This provides an audit trail of the session's progression through phases.
type PhaseTransition struct {
	// From is the source phase of the transition.
	// Empty string indicates this is the initial phase.
	From Phase `json:"from,omitempty"`

	// To is the destination phase of the transition.
	To Phase `json:"to"`

	// Timestamp records when the transition occurred.
	Timestamp time.Time `json:"timestamp"`

	// Reason provides optional context for why the transition occurred.
	// This is particularly useful for transitions to Failed state.
	Reason string `json:"reason,omitempty"`

	// Metadata holds arbitrary key-value pairs associated with the transition.
	// This can store phase-specific context like error details or metrics.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Duration returns the time elapsed since this transition occurred.
func (t PhaseTransition) Duration() time.Duration {
	return time.Since(t.Timestamp)
}

// PhaseConfig holds configuration options for a specific phase.
// Different phases may have different requirements and behaviors.
type PhaseConfig struct {
	// Phase identifies which phase this configuration applies to.
	Phase Phase `json:"phase"`

	// Timeout specifies the maximum duration allowed for this phase.
	// Zero means no timeout.
	Timeout time.Duration `json:"timeout,omitempty"`

	// MaxRetries specifies how many times the phase can be retried on failure.
	// Zero means no retries.
	MaxRetries int `json:"max_retries,omitempty"`

	// AllowSkip indicates whether this phase can be skipped.
	// Some phases (like Synthesis) may be optional.
	AllowSkip bool `json:"allow_skip,omitempty"`

	// RequireConfirmation indicates whether user confirmation is needed
	// before entering this phase.
	RequireConfirmation bool `json:"require_confirmation,omitempty"`

	// Constraints holds phase-specific constraints that must be satisfied
	// before transitioning into this phase.
	Constraints []PhaseConstraint `json:"constraints,omitempty"`

	// Metadata holds arbitrary configuration data specific to this phase.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PhaseConstraint defines a condition that must be met for a phase transition.
type PhaseConstraint struct {
	// Name is a short identifier for this constraint.
	Name string `json:"name"`

	// Description explains what this constraint checks.
	Description string `json:"description"`

	// Required indicates whether this constraint must be satisfied.
	// Non-required constraints generate warnings but don't block transitions.
	Required bool `json:"required"`
}

// ValidTransitions defines which phase transitions are allowed.
// This is the canonical source of truth for the phase state machine.
var ValidTransitions = map[Phase][]Phase{
	// From Planning: can proceed to selection (multi-pass), refresh, or fail
	PhasePlanning: {
		PhasePlanSelection, // Multi-pass mode: go to plan selection
		PhaseRefresh,       // Single-pass mode: proceed to refresh
		PhaseFailed,        // Planning failed
	},

	// From PlanSelection: proceed to refresh after selecting best plan
	PhasePlanSelection: {
		PhaseRefresh, // Plan selected, proceed to refresh
		PhaseFailed,  // Selection failed
	},

	// From Refresh: proceed to execution
	PhaseRefresh: {
		PhaseExecution, // Context refreshed, start execution
		PhaseFailed,    // Refresh failed
	},

	// From Execution: proceed to synthesis, consolidation, or completion
	PhaseExecution: {
		PhaseSynthesis,     // Normal flow: analyze results
		PhaseConsolidation, // Skip synthesis: go directly to consolidation
		PhaseComplete,      // Dry run or no tasks: complete immediately
		PhaseFailed,        // Execution failed
	},

	// From Synthesis: proceed to revision, consolidation, or completion
	PhaseSynthesis: {
		PhaseRevision,      // Issues found: need revision
		PhaseConsolidation, // No issues: proceed to consolidation
		PhaseComplete,      // No consolidation needed
		PhaseFailed,        // Synthesis failed
	},

	// From Revision: loop back to synthesis or proceed to consolidation
	PhaseRevision: {
		PhaseSynthesis,     // Re-analyze after revision
		PhaseConsolidation, // Revision complete: consolidate
		PhaseFailed,        // Revision failed
	},

	// From Consolidation: complete or fail
	PhaseConsolidation: {
		PhaseComplete, // All PRs created
		PhaseFailed,   // Consolidation failed
	},

	// Terminal states: no transitions out
	PhaseComplete: {},
	PhaseFailed:   {},
}

// CanTransition checks whether a transition from one phase to another is valid
// according to the ValidTransitions map.
func CanTransition(from, to Phase) bool {
	validTargets, exists := ValidTransitions[from]
	if !exists {
		return false
	}
	return slices.Contains(validTargets, to)
}

// PhaseConstraints defines constraints that must be satisfied to enter each phase.
// These represent preconditions beyond simple transition validity.
var PhaseConstraints = map[Phase][]PhaseConstraint{
	PhaseExecution: {
		{
			Name:        "plan_required",
			Description: "A valid plan must exist before execution can begin",
			Required:    true,
		},
		{
			Name:        "tasks_exist",
			Description: "The plan must contain at least one task",
			Required:    true,
		},
	},
	PhaseSynthesis: {
		{
			Name:        "tasks_completed",
			Description: "All execution tasks must be completed or failed",
			Required:    true,
		},
	},
	PhaseRevision: {
		{
			Name:        "issues_identified",
			Description: "Synthesis must have identified issues requiring revision",
			Required:    true,
		},
	},
	PhaseConsolidation: {
		{
			Name:        "successful_tasks",
			Description: "At least one task must have completed successfully",
			Required:    true,
		},
	},
}

// GetConstraints returns the constraints for entering a phase.
// Returns an empty slice if no constraints are defined.
func GetConstraints(phase Phase) []PhaseConstraint {
	if constraints, exists := PhaseConstraints[phase]; exists {
		return constraints
	}
	return nil
}

// Common errors for phase transitions.
var (
	// ErrInvalidTransition indicates an attempted transition that is not allowed.
	ErrInvalidTransition = errors.New("invalid phase transition")

	// ErrAlreadyInPhase indicates an attempt to transition to the current phase.
	ErrAlreadyInPhase = errors.New("already in requested phase")

	// ErrTerminalPhase indicates an attempt to transition from a terminal phase.
	ErrTerminalPhase = errors.New("cannot transition from terminal phase")

	// ErrConstraintNotSatisfied indicates a phase constraint was not met.
	ErrConstraintNotSatisfied = errors.New("phase constraint not satisfied")

	// ErrPhaseTimeout indicates the phase exceeded its configured timeout.
	ErrPhaseTimeout = errors.New("phase timeout exceeded")
)

// TransitionError wraps transition failures with additional context.
type TransitionError struct {
	From       Phase
	To         Phase
	Constraint *PhaseConstraint // nil if not a constraint violation
	Err        error
}

func (e *TransitionError) Error() string {
	if e.Constraint != nil {
		return "phase transition from " + string(e.From) + " to " + string(e.To) +
			" blocked: constraint '" + e.Constraint.Name + "' not satisfied"
	}
	return "phase transition from " + string(e.From) + " to " + string(e.To) +
		" failed: " + e.Err.Error()
}

func (e *TransitionError) Unwrap() error {
	return e.Err
}

// NewTransitionError creates a new TransitionError.
func NewTransitionError(from, to Phase, err error) *TransitionError {
	return &TransitionError{From: from, To: to, Err: err}
}

// NewConstraintError creates a TransitionError for a constraint violation.
func NewConstraintError(from, to Phase, constraint PhaseConstraint) *TransitionError {
	return &TransitionError{
		From:       from,
		To:         to,
		Constraint: &constraint,
		Err:        ErrConstraintNotSatisfied,
	}
}
