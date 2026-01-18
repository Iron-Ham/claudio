package orchestrator

import (
	"strings"
	"time"
)

// RalphConfig holds configuration for a Ralph Wiggum loop session.
type RalphConfig struct {
	// MaxIterations is the safety limit for iterations (0 = no limit).
	MaxIterations int `json:"max_iterations"`

	// CompletionPromise is the phrase that signals completion.
	// When this phrase is detected in output, the loop stops.
	CompletionPromise string `json:"completion_promise"`
}

// DefaultRalphConfig returns a RalphConfig with sensible defaults.
func DefaultRalphConfig() *RalphConfig {
	return &RalphConfig{
		MaxIterations:     50, // Safety limit
		CompletionPromise: "", // User must specify
	}
}

// RalphPhase represents the current phase of a Ralph loop session.
type RalphPhase string

const (
	// PhaseRalphWorking indicates the current iteration is running.
	PhaseRalphWorking RalphPhase = "working"

	// PhaseRalphComplete indicates the completion promise was found.
	PhaseRalphComplete RalphPhase = "complete"

	// PhaseRalphMaxIterations indicates the max iteration limit was reached.
	PhaseRalphMaxIterations RalphPhase = "max_iterations"

	// PhaseRalphCancelled indicates the loop was manually cancelled.
	PhaseRalphCancelled RalphPhase = "cancelled"

	// PhaseRalphError indicates an error occurred during the loop.
	PhaseRalphError RalphPhase = "error"
)

// RalphSession holds the state for a Ralph Wiggum iterative loop.
type RalphSession struct {
	// Prompt is the task description that gets repeated each iteration.
	Prompt string `json:"prompt"`

	// Config holds the loop configuration.
	Config *RalphConfig `json:"config"`

	// CurrentIteration is the current iteration number (1-indexed).
	CurrentIteration int `json:"current_iteration"`

	// Phase is the current phase of the ralph loop.
	Phase RalphPhase `json:"phase"`

	// GroupID links this session to its InstanceGroup.
	GroupID string `json:"group_id,omitempty"`

	// InstanceID is the ID of the current active instance in the loop.
	InstanceID string `json:"instance_id,omitempty"`

	// InstanceIDs tracks all instance IDs created during this loop.
	InstanceIDs []string `json:"instance_ids,omitempty"`

	// Error holds the error message if Phase is PhaseRalphError.
	Error string `json:"error,omitempty"`

	// StartedAt is when the ralph loop was started.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the ralph loop finished (if complete).
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// NewRalphSession creates a new Ralph loop session.
func NewRalphSession(prompt string, config *RalphConfig) *RalphSession {
	if config == nil {
		config = DefaultRalphConfig()
	}
	return &RalphSession{
		Prompt:           prompt,
		Config:           config,
		CurrentIteration: 0, // Will be incremented when first instance starts
		Phase:            PhaseRalphWorking,
		StartedAt:        time.Now(),
	}
}

// IsActive returns true if the ralph loop is still running.
func (s *RalphSession) IsActive() bool {
	return s.Phase == PhaseRalphWorking
}

// IsComplete returns true if the loop completed successfully (found promise).
func (s *RalphSession) IsComplete() bool {
	return s.Phase == PhaseRalphComplete
}

// ShouldContinue checks if another iteration should be started.
// Returns false if max iterations reached, cancelled, or completed.
func (s *RalphSession) ShouldContinue() bool {
	if s.Phase != PhaseRalphWorking {
		return false
	}
	if s.Config.MaxIterations > 0 && s.CurrentIteration >= s.Config.MaxIterations {
		return false
	}
	return true
}

// CheckCompletionPromise checks if the output contains the completion promise.
func (s *RalphSession) CheckCompletionPromise(output string) bool {
	if s.Config.CompletionPromise == "" {
		return false
	}
	return strings.Contains(output, s.Config.CompletionPromise)
}

// MarkComplete marks the session as complete (promise found).
func (s *RalphSession) MarkComplete() {
	s.Phase = PhaseRalphComplete
	now := time.Now()
	s.CompletedAt = &now
}

// MarkMaxIterationsReached marks the session as stopped due to iteration limit.
func (s *RalphSession) MarkMaxIterationsReached() {
	s.Phase = PhaseRalphMaxIterations
	now := time.Now()
	s.CompletedAt = &now
}

// MarkCancelled marks the session as manually cancelled.
func (s *RalphSession) MarkCancelled() {
	s.Phase = PhaseRalphCancelled
	now := time.Now()
	s.CompletedAt = &now
}

// MarkError marks the session as having an error.
func (s *RalphSession) MarkError(err error) {
	s.Phase = PhaseRalphError
	s.Error = err.Error()
	now := time.Now()
	s.CompletedAt = &now
}

// IncrementIteration advances to the next iteration.
func (s *RalphSession) IncrementIteration() {
	s.CurrentIteration++
}

// SetInstanceID sets the current active instance ID.
func (s *RalphSession) SetInstanceID(id string) {
	s.InstanceID = id
	// Track all instance IDs created in this loop
	s.InstanceIDs = append(s.InstanceIDs, id)
}
