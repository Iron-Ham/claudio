// Package ralph provides the Ralph Wiggum iterative loop workflow coordinator.
// Ralph Wiggum loops iterate on a prompt until a completion promise is found
// in the output, allowing autonomous refinement of solutions.
package ralph

import (
	"strings"
	"time"
)

// Config holds configuration for a Ralph Wiggum loop session.
type Config struct {
	// MaxIterations is the safety limit for iterations (0 = no limit).
	MaxIterations int `json:"max_iterations"`

	// CompletionPromise is the phrase that signals completion.
	// When this phrase is detected in output, the loop stops.
	CompletionPromise string `json:"completion_promise"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		MaxIterations:     50, // Safety limit
		CompletionPromise: "", // User must specify
	}
}

// Phase represents the current phase of a Ralph loop session.
type Phase string

const (
	// PhaseWorking indicates the current iteration is running.
	PhaseWorking Phase = "working"

	// PhaseComplete indicates the completion promise was found.
	PhaseComplete Phase = "complete"

	// PhaseMaxIterations indicates the max iteration limit was reached.
	PhaseMaxIterations Phase = "max_iterations"

	// PhaseCancelled indicates the loop was manually cancelled.
	PhaseCancelled Phase = "cancelled"

	// PhaseError indicates an error occurred during the loop.
	PhaseError Phase = "error"
)

// Session holds the state for a Ralph Wiggum iterative loop.
type Session struct {
	// Prompt is the task description that gets repeated each iteration.
	Prompt string `json:"prompt"`

	// Config holds the loop configuration.
	Config *Config `json:"config"`

	// CurrentIteration is the current iteration number (1-indexed).
	CurrentIteration int `json:"current_iteration"`

	// Phase is the current phase of the ralph loop.
	Phase Phase `json:"phase"`

	// GroupID links this session to its InstanceGroup.
	GroupID string `json:"group_id,omitempty"`

	// InstanceID is the ID of the current active instance in the loop.
	InstanceID string `json:"instance_id,omitempty"`

	// InstanceIDs tracks all instance IDs created during this loop.
	InstanceIDs []string `json:"instance_ids,omitempty"`

	// Error holds the error message if Phase is PhaseError.
	Error string `json:"error,omitempty"`

	// StartedAt is when the ralph loop was started.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the ralph loop finished (if complete).
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// NewSession creates a new Ralph loop session.
func NewSession(prompt string, config *Config) *Session {
	if config == nil {
		config = DefaultConfig()
	}
	return &Session{
		Prompt:           prompt,
		Config:           config,
		CurrentIteration: 0, // Will be incremented when first instance starts
		Phase:            PhaseWorking,
		StartedAt:        time.Now(),
	}
}

// IsActive returns true if the ralph loop is still running.
func (s *Session) IsActive() bool {
	return s.Phase == PhaseWorking
}

// IsComplete returns true if the loop completed successfully (found promise).
func (s *Session) IsComplete() bool {
	return s.Phase == PhaseComplete
}

// ShouldContinue checks if another iteration should be started.
// Returns false if max iterations reached, cancelled, or completed.
func (s *Session) ShouldContinue() bool {
	if s.Phase != PhaseWorking {
		return false
	}
	if s.Config.MaxIterations > 0 && s.CurrentIteration >= s.Config.MaxIterations {
		return false
	}
	return true
}

// CheckCompletionPromise checks if the output contains the completion promise.
func (s *Session) CheckCompletionPromise(output string) bool {
	if s.Config.CompletionPromise == "" {
		return false
	}
	return strings.Contains(output, s.Config.CompletionPromise)
}

// MarkComplete marks the session as complete (promise found).
func (s *Session) MarkComplete() {
	s.Phase = PhaseComplete
	now := time.Now()
	s.CompletedAt = &now
}

// MarkMaxIterationsReached marks the session as stopped due to iteration limit.
func (s *Session) MarkMaxIterationsReached() {
	s.Phase = PhaseMaxIterations
	now := time.Now()
	s.CompletedAt = &now
}

// MarkCancelled marks the session as manually cancelled.
func (s *Session) MarkCancelled() {
	s.Phase = PhaseCancelled
	now := time.Now()
	s.CompletedAt = &now
}

// MarkError marks the session as having an error.
func (s *Session) MarkError(err error) {
	s.Phase = PhaseError
	s.Error = err.Error()
	now := time.Now()
	s.CompletedAt = &now
}

// IncrementIteration advances to the next iteration.
func (s *Session) IncrementIteration() {
	s.CurrentIteration++
}

// SetInstanceID sets the current active instance ID.
func (s *Session) SetInstanceID(id string) {
	s.InstanceID = id
	// Track all instance IDs created in this loop
	s.InstanceIDs = append(s.InstanceIDs, id)
}
