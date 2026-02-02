// Package event defines event types for decoupling components in Claudio.
// These events enable communication between TUI, Orchestrator, and other components
// without requiring direct dependencies.
package event

import "time"

// Event is the interface that all events must implement.
// It provides a common way to identify and timestamp events.
type Event interface {
	// EventType returns a string identifier for this event type.
	// Convention: "category.action" (e.g., "instance.started", "pr.completed")
	EventType() string

	// Timestamp returns when the event occurred.
	Timestamp() time.Time
}

// baseEvent provides common fields for all events.
// Embed this in concrete event types to satisfy the Event interface.
type baseEvent struct {
	eventType string
	timestamp time.Time
}

func (e baseEvent) EventType() string    { return e.eventType }
func (e baseEvent) Timestamp() time.Time { return e.timestamp }

// newBaseEvent creates a baseEvent with the current time.
func newBaseEvent(eventType string) baseEvent {
	return baseEvent{
		eventType: eventType,
		timestamp: time.Now(),
	}
}

// -----------------------------------------------------------------------------
// Instance Lifecycle Events
// -----------------------------------------------------------------------------

// InstanceStartedEvent is emitted when a backend instance begins execution.
type InstanceStartedEvent struct {
	baseEvent
	InstanceID   string // Unique identifier for the instance
	WorktreePath string // Path to the git worktree
	Branch       string // Git branch name
	Task         string // Task description or prompt
}

// NewInstanceStartedEvent creates an InstanceStartedEvent.
func NewInstanceStartedEvent(instanceID, worktreePath, branch, task string) InstanceStartedEvent {
	return InstanceStartedEvent{
		baseEvent:    newBaseEvent("instance.started"),
		InstanceID:   instanceID,
		WorktreePath: worktreePath,
		Branch:       branch,
		Task:         task,
	}
}

// InstanceStoppedEvent is emitted when a backend instance stops execution.
type InstanceStoppedEvent struct {
	baseEvent
	InstanceID string // Unique identifier for the instance
	Success    bool   // Whether the instance completed successfully
	Reason     string // Reason for stopping (e.g., "completed", "error", "cancelled")
}

// NewInstanceStoppedEvent creates an InstanceStoppedEvent.
func NewInstanceStoppedEvent(instanceID string, success bool, reason string) InstanceStoppedEvent {
	return InstanceStoppedEvent{
		baseEvent:  newBaseEvent("instance.stopped"),
		InstanceID: instanceID,
		Success:    success,
		Reason:     reason,
	}
}

// -----------------------------------------------------------------------------
// PR Events
// -----------------------------------------------------------------------------

// PRCompleteEvent is emitted when a pull request operation completes.
type PRCompleteEvent struct {
	baseEvent
	InstanceID string // Instance that created/updated the PR
	Success    bool   // Whether the PR operation succeeded
	PRURL      string // URL of the pull request (if created)
	Error      string // Error message (if failed)
}

// NewPRCompleteEvent creates a PRCompleteEvent.
func NewPRCompleteEvent(instanceID string, success bool, prURL, errMsg string) PRCompleteEvent {
	return PRCompleteEvent{
		baseEvent:  newBaseEvent("pr.completed"),
		InstanceID: instanceID,
		Success:    success,
		PRURL:      prURL,
		Error:      errMsg,
	}
}

// -----------------------------------------------------------------------------
// Conflict Events
// -----------------------------------------------------------------------------

// ConflictDetectedEvent is emitted when a git conflict is detected.
type ConflictDetectedEvent struct {
	baseEvent
	InstanceID    string   // Instance that encountered the conflict
	Branch        string   // Branch with the conflict
	ConflictFiles []string // Files with conflicts
}

// NewConflictDetectedEvent creates a ConflictDetectedEvent.
func NewConflictDetectedEvent(instanceID, branch string, conflictFiles []string) ConflictDetectedEvent {
	return ConflictDetectedEvent{
		baseEvent:     newBaseEvent("conflict.detected"),
		InstanceID:    instanceID,
		Branch:        branch,
		ConflictFiles: conflictFiles,
	}
}

// -----------------------------------------------------------------------------
// Timeout Events
// -----------------------------------------------------------------------------

// TimeoutType represents the type of timeout that occurred.
type TimeoutType int

const (
	TimeoutActivity   TimeoutType = iota // No activity for configured period
	TimeoutCompletion                    // Total runtime exceeded limit
	TimeoutStale                         // Repeated output detected (stuck in loop)
)

// String returns a human-readable name for a timeout type.
func (t TimeoutType) String() string {
	switch t {
	case TimeoutActivity:
		return "activity"
	case TimeoutCompletion:
		return "completion"
	case TimeoutStale:
		return "stale"
	default:
		return "unknown"
	}
}

// TimeoutEvent is emitted when an instance times out.
type TimeoutEvent struct {
	baseEvent
	InstanceID  string      // Instance that timed out
	TimeoutType TimeoutType // Type of timeout
	Duration    string      // How long since last activity or start
}

// NewTimeoutEvent creates a TimeoutEvent.
func NewTimeoutEvent(instanceID string, timeoutType TimeoutType, duration string) TimeoutEvent {
	return TimeoutEvent{
		baseEvent:   newBaseEvent("instance.timeout"),
		InstanceID:  instanceID,
		TimeoutType: timeoutType,
		Duration:    duration,
	}
}

// -----------------------------------------------------------------------------
// Task Events (Ultra-Plan)
// -----------------------------------------------------------------------------

// TaskCompletedEvent is emitted when an ultra-plan task completes.
type TaskCompletedEvent struct {
	baseEvent
	TaskID     string // Task identifier from the plan
	InstanceID string // Instance that executed the task (empty if not yet started)
	Success    bool   // Whether the task completed successfully
	Reason     string // Additional context (error message if failed)
}

// NewTaskCompletedEvent creates a TaskCompletedEvent.
func NewTaskCompletedEvent(taskID, instanceID string, success bool, reason string) TaskCompletedEvent {
	return TaskCompletedEvent{
		baseEvent:  newBaseEvent("task.completed"),
		TaskID:     taskID,
		InstanceID: instanceID,
		Success:    success,
		Reason:     reason,
	}
}

// -----------------------------------------------------------------------------
// Phase Events (Ultra-Plan)
// -----------------------------------------------------------------------------

// Phase represents the current phase of an ultra-plan session.
// Mirrors orchestrator.UltraPlanPhase for decoupling.
type Phase string

const (
	PhasePlanning      Phase = "planning"
	PhasePlanSelection Phase = "plan_selection"
	PhaseRefresh       Phase = "context_refresh"
	PhaseExecuting     Phase = "executing"
	PhaseSynthesis     Phase = "synthesis"
	PhaseRevision      Phase = "revision"
	PhaseConsolidating Phase = "consolidating"
	PhaseComplete      Phase = "complete"
	PhaseFailed        Phase = "failed"
)

// PhaseChangeEvent is emitted when the ultra-plan phase changes.
type PhaseChangeEvent struct {
	baseEvent
	PreviousPhase Phase  // Previous phase (empty if first transition)
	CurrentPhase  Phase  // New current phase
	SessionID     string // Ultra-plan session ID
}

// NewPhaseChangeEvent creates a PhaseChangeEvent.
func NewPhaseChangeEvent(sessionID string, previousPhase, currentPhase Phase) PhaseChangeEvent {
	return PhaseChangeEvent{
		baseEvent:     newBaseEvent("phase.changed"),
		PreviousPhase: previousPhase,
		CurrentPhase:  currentPhase,
		SessionID:     sessionID,
	}
}

// -----------------------------------------------------------------------------
// Metrics Events
// -----------------------------------------------------------------------------

// MetricsUpdateEvent is emitted when instance metrics are updated.
type MetricsUpdateEvent struct {
	baseEvent
	InstanceID   string  // Instance the metrics belong to
	InputTokens  int64   // Total input tokens used
	OutputTokens int64   // Total output tokens used
	CacheRead    int64   // Tokens read from cache
	CacheWrite   int64   // Tokens written to cache
	Cost         float64 // Estimated cost in USD
	APICalls     int     // Number of API calls made
}

// NewMetricsUpdateEvent creates a MetricsUpdateEvent.
func NewMetricsUpdateEvent(instanceID string, inputTokens, outputTokens, cacheRead, cacheWrite int64, cost float64, apiCalls int) MetricsUpdateEvent {
	return MetricsUpdateEvent{
		baseEvent:    newBaseEvent("metrics.updated"),
		InstanceID:   instanceID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CacheRead:    cacheRead,
		CacheWrite:   cacheWrite,
		Cost:         cost,
		APICalls:     apiCalls,
	}
}

// TotalTokens returns the sum of input and output tokens.
func (e MetricsUpdateEvent) TotalTokens() int64 {
	return e.InputTokens + e.OutputTokens
}

// -----------------------------------------------------------------------------
// Bell Events (Terminal Notification)
// -----------------------------------------------------------------------------

// BellEvent is emitted when a terminal bell is detected in an instance.
// Used to forward audio notifications from tmux sessions to the parent terminal.
type BellEvent struct {
	baseEvent
	InstanceID string // Instance that triggered the bell
}

// NewBellEvent creates a BellEvent.
func NewBellEvent(instanceID string) BellEvent {
	return BellEvent{
		baseEvent:  newBaseEvent("instance.bell"),
		InstanceID: instanceID,
	}
}

// -----------------------------------------------------------------------------
// PR Opened Events (Inline PR Detection)
// -----------------------------------------------------------------------------

// PROpenedEvent is emitted when a PR URL is detected in instance output.
// This indicates an inline PR was created during task execution (via gh pr create).
type PROpenedEvent struct {
	baseEvent
	InstanceID string // Instance that opened the PR
	PRURL      string // Reserved for future use - currently not populated
}

// NewPROpenedEvent creates a PROpenedEvent.
func NewPROpenedEvent(instanceID string, prURL string) PROpenedEvent {
	return PROpenedEvent{
		baseEvent:  newBaseEvent("pr.opened"),
		InstanceID: instanceID,
		PRURL:      prURL,
	}
}

// -----------------------------------------------------------------------------
// Group Phase Events (Group-Aware Lifecycle)
// -----------------------------------------------------------------------------

// GroupPhase represents the phase of an instance group.
// Mirrors orchestrator.GroupPhase for decoupling.
type GroupPhase string

const (
	GroupPhasePending   GroupPhase = "pending"
	GroupPhaseExecuting GroupPhase = "executing"
	GroupPhaseCompleted GroupPhase = "completed"
	GroupPhaseFailed    GroupPhase = "failed"
)

// GroupPhaseChangeEvent is emitted when a group's phase changes.
// This enables TUI reactivity to group state transitions.
type GroupPhaseChangeEvent struct {
	baseEvent
	GroupID       string     // Unique identifier for the group
	GroupName     string     // Human-readable group name
	PreviousPhase GroupPhase // Previous phase
	CurrentPhase  GroupPhase // New current phase
}

// NewGroupPhaseChangeEvent creates a GroupPhaseChangeEvent.
func NewGroupPhaseChangeEvent(groupID, groupName string, previousPhase, currentPhase GroupPhase) GroupPhaseChangeEvent {
	return GroupPhaseChangeEvent{
		baseEvent:     newBaseEvent("group.phase_changed"),
		GroupID:       groupID,
		GroupName:     groupName,
		PreviousPhase: previousPhase,
		CurrentPhase:  currentPhase,
	}
}

// GroupCompletionEvent is emitted when a group completes (all instances finished).
type GroupCompletionEvent struct {
	baseEvent
	GroupID      string // Unique identifier for the group
	GroupName    string // Human-readable group name
	Success      bool   // True if all instances completed successfully
	FailedCount  int    // Number of instances that failed
	SuccessCount int    // Number of instances that succeeded
}

// NewGroupCompletionEvent creates a GroupCompletionEvent.
func NewGroupCompletionEvent(groupID, groupName string, success bool, failedCount, successCount int) GroupCompletionEvent {
	return GroupCompletionEvent{
		baseEvent:    newBaseEvent("group.completed"),
		GroupID:      groupID,
		GroupName:    groupName,
		Success:      success,
		FailedCount:  failedCount,
		SuccessCount: successCount,
	}
}
