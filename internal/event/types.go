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

// -----------------------------------------------------------------------------
// Mailbox Events (Inter-Instance Communication)
// -----------------------------------------------------------------------------

// MailboxMessageEvent is emitted when an inter-instance mailbox message is sent.
type MailboxMessageEvent struct {
	baseEvent
	From        string // Sender instance ID or "coordinator"
	To          string // Recipient instance ID or "broadcast"
	MessageType string // Message type (discovery, claim, warning, etc.)
	Body        string // Message content
}

// NewMailboxMessageEvent creates a MailboxMessageEvent.
func NewMailboxMessageEvent(from, to, messageType, body string) MailboxMessageEvent {
	return MailboxMessageEvent{
		baseEvent:   newBaseEvent("mailbox.message"),
		From:        from,
		To:          to,
		MessageType: messageType,
		Body:        body,
	}
}

// -----------------------------------------------------------------------------
// Task Queue Events (Dynamic Task Claiming)
// -----------------------------------------------------------------------------

// TaskClaimedEvent is emitted when an instance claims a task from the queue.
type TaskClaimedEvent struct {
	baseEvent
	TaskID     string // Task that was claimed
	InstanceID string // Instance that claimed it
}

// NewTaskClaimedEvent creates a TaskClaimedEvent.
func NewTaskClaimedEvent(taskID, instanceID string) TaskClaimedEvent {
	return TaskClaimedEvent{
		baseEvent:  newBaseEvent("queue.task_claimed"),
		TaskID:     taskID,
		InstanceID: instanceID,
	}
}

// TaskReleasedEvent is emitted when a task is returned to the queue.
type TaskReleasedEvent struct {
	baseEvent
	TaskID string // Task that was released
	Reason string // Why it was released (e.g., "stale_claim", "instance_died")
}

// NewTaskReleasedEvent creates a TaskReleasedEvent.
func NewTaskReleasedEvent(taskID, reason string) TaskReleasedEvent {
	return TaskReleasedEvent{
		baseEvent: newBaseEvent("queue.task_released"),
		TaskID:    taskID,
		Reason:    reason,
	}
}

// QueueDepthChangedEvent is emitted when the queue depth changes.
// Used by the TUI to display queue progress.
type QueueDepthChangedEvent struct {
	baseEvent
	Pending   int // Number of pending tasks
	Claimed   int // Number of claimed tasks
	Running   int // Number of running tasks
	Completed int // Number of completed tasks
	Failed    int // Number of permanently failed tasks
	Total     int // Total number of tasks
}

// NewQueueDepthChangedEvent creates a QueueDepthChangedEvent.
func NewQueueDepthChangedEvent(pending, claimed, running, completed, failed, total int) QueueDepthChangedEvent {
	return QueueDepthChangedEvent{
		baseEvent: newBaseEvent("queue.depth_changed"),
		Pending:   pending,
		Claimed:   claimed,
		Running:   running,
		Completed: completed,
		Failed:    failed,
		Total:     total,
	}
}

// TaskAwaitingApprovalEvent is emitted when a task enters the awaiting_approval state.
// This occurs when a task with RequiresApproval=true is claimed and the gate
// intercepts the transition to running.
type TaskAwaitingApprovalEvent struct {
	baseEvent
	TaskID     string // Task that is awaiting approval
	InstanceID string // Instance that claimed the task
}

// NewTaskAwaitingApprovalEvent creates a TaskAwaitingApprovalEvent.
func NewTaskAwaitingApprovalEvent(taskID, instanceID string) TaskAwaitingApprovalEvent {
	return TaskAwaitingApprovalEvent{
		baseEvent:  newBaseEvent("queue.task_awaiting_approval"),
		TaskID:     taskID,
		InstanceID: instanceID,
	}
}

// -----------------------------------------------------------------------------
// Scaling Events
// -----------------------------------------------------------------------------

// ScalingDecisionEvent is emitted when the scaling monitor makes a scaling decision.
type ScalingDecisionEvent struct {
	baseEvent
	Action           string // "scale_up", "scale_down", or "none"
	Delta            int    // Number of instances to add (positive) or remove (negative)
	Reason           string // Human-readable explanation of the decision
	CurrentInstances int    // Number of instances before the scaling action
}

// NewScalingDecisionEvent creates a ScalingDecisionEvent.
func NewScalingDecisionEvent(action string, delta int, reason string, currentInstances int) ScalingDecisionEvent {
	return ScalingDecisionEvent{
		baseEvent:        newBaseEvent("scaling.decision"),
		Action:           action,
		Delta:            delta,
		Reason:           reason,
		CurrentInstances: currentInstances,
	}
}

// -----------------------------------------------------------------------------
// Debate Events (Peer Debate Protocol)
// -----------------------------------------------------------------------------

// DebateStartedEvent is emitted when a structured debate begins between two instances.
type DebateStartedEvent struct {
	baseEvent
	DebateID  string // Unique identifier for the debate session
	InstanceA string // First participant
	InstanceB string // Second participant
	Topic     string // Subject of the debate
}

// NewDebateStartedEvent creates a DebateStartedEvent.
func NewDebateStartedEvent(debateID, instanceA, instanceB, topic string) DebateStartedEvent {
	return DebateStartedEvent{
		baseEvent: newBaseEvent("debate.started"),
		DebateID:  debateID,
		InstanceA: instanceA,
		InstanceB: instanceB,
		Topic:     topic,
	}
}

// DebateResolvedEvent is emitted when a debate reaches consensus.
type DebateResolvedEvent struct {
	baseEvent
	DebateID   string // Unique identifier for the debate session
	Resolution string // The consensus resolution
	Rounds     int    // Number of challenge-defense rounds
}

// NewDebateResolvedEvent creates a DebateResolvedEvent.
func NewDebateResolvedEvent(debateID, resolution string, rounds int) DebateResolvedEvent {
	return DebateResolvedEvent{
		baseEvent:  newBaseEvent("debate.resolved"),
		DebateID:   debateID,
		Resolution: resolution,
		Rounds:     rounds,
	}
}

// -----------------------------------------------------------------------------
// Context Propagation Events
// -----------------------------------------------------------------------------

// ContextPropagatedEvent is emitted when context is shared across instances.
type ContextPropagatedEvent struct {
	baseEvent
	From          string // Instance that shared the context
	InstanceCount int    // Number of instances that received the context
	MessageType   string // Type of message propagated (discovery, warning, etc.)
}

// NewContextPropagatedEvent creates a ContextPropagatedEvent.
func NewContextPropagatedEvent(from string, instanceCount int, messageType string) ContextPropagatedEvent {
	return ContextPropagatedEvent{
		baseEvent:     newBaseEvent("context.propagated"),
		From:          from,
		InstanceCount: instanceCount,
		MessageType:   messageType,
	}
}

// -----------------------------------------------------------------------------
// File Lock Events (File Conflict Prevention)
// -----------------------------------------------------------------------------

// FileClaimEvent is emitted when an instance claims ownership of a file.
type FileClaimEvent struct {
	baseEvent
	InstanceID string // Instance claiming the file
	FilePath   string // Path to the claimed file
}

// NewFileClaimEvent creates a FileClaimEvent.
func NewFileClaimEvent(instanceID, filePath string) FileClaimEvent {
	return FileClaimEvent{
		baseEvent:  newBaseEvent("filelock.claimed"),
		InstanceID: instanceID,
		FilePath:   filePath,
	}
}

// FileReleaseEvent is emitted when an instance releases ownership of a file.
type FileReleaseEvent struct {
	baseEvent
	InstanceID string // Instance releasing the file
	FilePath   string // Path to the released file
}

// NewFileReleaseEvent creates a FileReleaseEvent.
func NewFileReleaseEvent(instanceID, filePath string) FileReleaseEvent {
	return FileReleaseEvent{
		baseEvent:  newBaseEvent("filelock.released"),
		InstanceID: instanceID,
		FilePath:   filePath,
	}
}

// -----------------------------------------------------------------------------
// Adaptive Lead Events (Dynamic Coordination)
// -----------------------------------------------------------------------------

// ScalingSignalEvent is emitted when the adaptive lead detects a scaling need.
type ScalingSignalEvent struct {
	baseEvent
	Pending        int    // Number of pending tasks
	Running        int    // Number of running tasks
	Recommendation string // Human-readable recommendation
}

// NewScalingSignalEvent creates a ScalingSignalEvent.
func NewScalingSignalEvent(pending, running int, recommendation string) ScalingSignalEvent {
	return ScalingSignalEvent{
		baseEvent:      newBaseEvent("adaptive.scaling_signal"),
		Pending:        pending,
		Running:        running,
		Recommendation: recommendation,
	}
}

// TaskReassignedEvent is emitted when the adaptive lead reassigns a task.
type TaskReassignedEvent struct {
	baseEvent
	TaskID       string // Task that was reassigned
	FromInstance string // Instance the task was taken from
	ToInstance   string // Instance the task was given to
	Reason       string // Why the reassignment happened
}

// NewTaskReassignedEvent creates a TaskReassignedEvent.
func NewTaskReassignedEvent(taskID, fromInstance, toInstance, reason string) TaskReassignedEvent {
	return TaskReassignedEvent{
		baseEvent:    newBaseEvent("adaptive.task_reassigned"),
		TaskID:       taskID,
		FromInstance: fromInstance,
		ToInstance:   toInstance,
		Reason:       reason,
	}
}

// -----------------------------------------------------------------------------
// Team Lifecycle Events (Multi-Team Orchestration)
// -----------------------------------------------------------------------------

// TeamCreatedEvent is emitted when a new team is added to the manager.
type TeamCreatedEvent struct {
	baseEvent
	TeamID   string // Unique identifier for the team
	TeamName string // Human-readable team name
	TeamRole string // Team's role (execution, planning, review, consolidation)
}

// NewTeamCreatedEvent creates a TeamCreatedEvent.
func NewTeamCreatedEvent(teamID, teamName, teamRole string) TeamCreatedEvent {
	return TeamCreatedEvent{
		baseEvent: newBaseEvent("team.created"),
		TeamID:    teamID,
		TeamName:  teamName,
		TeamRole:  teamRole,
	}
}

// TeamPhaseChangedEvent is emitted when a team transitions between phases.
type TeamPhaseChangedEvent struct {
	baseEvent
	TeamID        string // Unique identifier for the team
	TeamName      string // Human-readable team name
	PreviousPhase string // Previous phase (e.g., "forming", "blocked")
	CurrentPhase  string // New phase (e.g., "working", "done")
}

// NewTeamPhaseChangedEvent creates a TeamPhaseChangedEvent.
func NewTeamPhaseChangedEvent(teamID, teamName, previousPhase, currentPhase string) TeamPhaseChangedEvent {
	return TeamPhaseChangedEvent{
		baseEvent:     newBaseEvent("team.phase_changed"),
		TeamID:        teamID,
		TeamName:      teamName,
		PreviousPhase: previousPhase,
		CurrentPhase:  currentPhase,
	}
}

// TeamCompletedEvent is emitted when a team finishes all its work.
type TeamCompletedEvent struct {
	baseEvent
	TeamID      string // Unique identifier for the team
	TeamName    string // Human-readable team name
	Success     bool   // True if the team completed without failures
	TasksDone   int    // Number of tasks completed successfully
	TasksFailed int    // Number of tasks that failed
}

// NewTeamCompletedEvent creates a TeamCompletedEvent.
func NewTeamCompletedEvent(teamID, teamName string, success bool, tasksDone, tasksFailed int) TeamCompletedEvent {
	return TeamCompletedEvent{
		baseEvent:   newBaseEvent("team.completed"),
		TeamID:      teamID,
		TeamName:    teamName,
		Success:     success,
		TasksDone:   tasksDone,
		TasksFailed: tasksFailed,
	}
}

// TeamBudgetExhaustedEvent is emitted when a team exhausts its token budget.
type TeamBudgetExhaustedEvent struct {
	baseEvent
	TeamID          string  // Unique identifier for the team
	MaxInputTokens  int64   // Configured input token limit
	MaxOutputTokens int64   // Configured output token limit
	MaxTotalCost    float64 // Configured cost limit (USD)
	UsedInput       int64   // Actual input tokens consumed
	UsedOutput      int64   // Actual output tokens consumed
	UsedCost        float64 // Actual cost consumed (USD)
}

// NewTeamBudgetExhaustedEvent creates a TeamBudgetExhaustedEvent.
func NewTeamBudgetExhaustedEvent(teamID string, maxIn, maxOut, usedIn, usedOut int64, maxCost, usedCost float64) TeamBudgetExhaustedEvent {
	return TeamBudgetExhaustedEvent{
		baseEvent:       newBaseEvent("team.budget_exhausted"),
		TeamID:          teamID,
		MaxInputTokens:  maxIn,
		MaxOutputTokens: maxOut,
		MaxTotalCost:    maxCost,
		UsedInput:       usedIn,
		UsedOutput:      usedOut,
		UsedCost:        usedCost,
	}
}

// -----------------------------------------------------------------------------
// Team Dynamic Management Events
// -----------------------------------------------------------------------------

// TeamDynamicAddedEvent is emitted when a team is dynamically added to a
// running manager (after Start).
type TeamDynamicAddedEvent struct {
	baseEvent
	TeamID   string // Unique identifier for the team
	TeamName string // Human-readable team name
	Phase    string // Team's initial phase (working or blocked)
}

// NewTeamDynamicAddedEvent creates a TeamDynamicAddedEvent.
func NewTeamDynamicAddedEvent(teamID, teamName, phase string) TeamDynamicAddedEvent {
	return TeamDynamicAddedEvent{
		baseEvent: newBaseEvent("team.dynamic_added"),
		TeamID:    teamID,
		TeamName:  teamName,
		Phase:     phase,
	}
}

// -----------------------------------------------------------------------------
// Pipeline Lifecycle Events
// -----------------------------------------------------------------------------

// PipelinePhaseChangedEvent is emitted when the pipeline transitions between phases.
type PipelinePhaseChangedEvent struct {
	baseEvent
	PipelineID    string // Unique identifier for the pipeline
	PreviousPhase string // Previous phase (e.g., "planning", "execution")
	CurrentPhase  string // New phase (e.g., "execution", "review")
}

// NewPipelinePhaseChangedEvent creates a PipelinePhaseChangedEvent.
func NewPipelinePhaseChangedEvent(pipelineID, previousPhase, currentPhase string) PipelinePhaseChangedEvent {
	return PipelinePhaseChangedEvent{
		baseEvent:     newBaseEvent("pipeline.phase_changed"),
		PipelineID:    pipelineID,
		PreviousPhase: previousPhase,
		CurrentPhase:  currentPhase,
	}
}

// PipelineCompletedEvent is emitted when a pipeline finishes.
type PipelineCompletedEvent struct {
	baseEvent
	PipelineID string // Unique identifier for the pipeline
	Success    bool   // True if the pipeline completed without failures
	PhasesRun  int    // Number of phases that were executed
}

// NewPipelineCompletedEvent creates a PipelineCompletedEvent.
func NewPipelineCompletedEvent(pipelineID string, success bool, phasesRun int) PipelineCompletedEvent {
	return PipelineCompletedEvent{
		baseEvent:  newBaseEvent("pipeline.completed"),
		PipelineID: pipelineID,
		Success:    success,
		PhasesRun:  phasesRun,
	}
}

// -----------------------------------------------------------------------------
// Bridge Events (Pipeline → Instance Execution)
// -----------------------------------------------------------------------------

// BridgeTaskStartedEvent is emitted when a bridge starts executing a task
// by spawning a Claude Code instance.
type BridgeTaskStartedEvent struct {
	baseEvent
	TeamID     string // Team the task belongs to
	TaskID     string // Task being executed
	InstanceID string // Instance created for the task
}

// NewBridgeTaskStartedEvent creates a BridgeTaskStartedEvent.
func NewBridgeTaskStartedEvent(teamID, taskID, instanceID string) BridgeTaskStartedEvent {
	return BridgeTaskStartedEvent{
		baseEvent:  newBaseEvent("bridge.task_started"),
		TeamID:     teamID,
		TaskID:     taskID,
		InstanceID: instanceID,
	}
}

// BridgeTaskCompletedEvent is emitted when a bridge-managed task finishes
// (either successfully or with failure).
type BridgeTaskCompletedEvent struct {
	baseEvent
	TeamID      string // Team the task belongs to
	TaskID      string // Task that completed
	InstanceID  string // Instance that executed the task
	Success     bool   // Whether the task completed successfully
	CommitCount int    // Number of commits produced (0 if failed)
	Error       string // Error description (empty on success)
}

// NewBridgeTaskCompletedEvent creates a BridgeTaskCompletedEvent.
func NewBridgeTaskCompletedEvent(teamID, taskID, instanceID string, success bool, commitCount int, errMsg string) BridgeTaskCompletedEvent {
	return BridgeTaskCompletedEvent{
		baseEvent:   newBaseEvent("bridge.task_completed"),
		TeamID:      teamID,
		TaskID:      taskID,
		InstanceID:  instanceID,
		Success:     success,
		CommitCount: commitCount,
		Error:       errMsg,
	}
}

// -----------------------------------------------------------------------------
// Inter-Team Communication Events
// -----------------------------------------------------------------------------

// InterTeamMessageEvent is emitted when a message is routed between teams.
type InterTeamMessageEvent struct {
	baseEvent
	FromTeam    string // Source team ID
	ToTeam      string // Destination team ID or "broadcast"
	MessageType string // Message category (discovery, dependency, warning, request)
	Content     string // Message content
	Priority    string // Message priority (info, important, urgent)
}

// NewInterTeamMessageEvent creates an InterTeamMessageEvent.
func NewInterTeamMessageEvent(fromTeam, toTeam, messageType, content, priority string) InterTeamMessageEvent {
	return InterTeamMessageEvent{
		baseEvent:   newBaseEvent("team.message"),
		FromTeam:    fromTeam,
		ToTeam:      toTeam,
		MessageType: messageType,
		Content:     content,
		Priority:    priority,
	}
}

// -----------------------------------------------------------------------------
// TripleShot Team Events
// -----------------------------------------------------------------------------

// TripleShotAttemptCompletedEvent is emitted when a tripleshot attempt finishes
// (either successfully or with failure) in the team-based coordinator.
type TripleShotAttemptCompletedEvent struct {
	baseEvent
	AttemptIndex int    // 0, 1, or 2
	TeamID       string // Team that ran this attempt
	Success      bool   // Whether the attempt completed successfully
}

// NewTripleShotAttemptCompletedEvent creates a TripleShotAttemptCompletedEvent.
func NewTripleShotAttemptCompletedEvent(attemptIndex int, teamID string, success bool) TripleShotAttemptCompletedEvent {
	return TripleShotAttemptCompletedEvent{
		baseEvent:    newBaseEvent("tripleshot.attempt_completed"),
		AttemptIndex: attemptIndex,
		TeamID:       teamID,
		Success:      success,
	}
}

// TripleShotJudgeCompletedEvent is emitted when the tripleshot judge finishes
// its evaluation in the team-based coordinator.
type TripleShotJudgeCompletedEvent struct {
	baseEvent
	TeamID  string // Team that ran the judge
	Success bool   // Whether the evaluation completed successfully
}

// NewTripleShotJudgeCompletedEvent creates a TripleShotJudgeCompletedEvent.
func NewTripleShotJudgeCompletedEvent(teamID string, success bool) TripleShotJudgeCompletedEvent {
	return TripleShotJudgeCompletedEvent{
		baseEvent: newBaseEvent("tripleshot.judge_completed"),
		TeamID:    teamID,
		Success:   success,
	}
}
