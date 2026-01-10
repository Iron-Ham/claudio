// Package events provides a unified event system for the Claudio application.
// It defines event types and interfaces that will replace the current callback-based
// communication pattern between components, enabling better separation of concerns
// and more flexible event handling.
package events

import (
	"sync"
	"time"
)

// EventType represents the type identifier for events.
// Using string type allows for easy debugging and extensibility.
type EventType string

// Event types for instance-level events
const (
	// TypePRComplete indicates a PR workflow has completed
	TypePRComplete EventType = "pr.complete"
	// TypePROpened indicates a PR URL was detected in output
	TypePROpened EventType = "pr.opened"
	// TypeTimeout indicates a timeout condition was detected
	TypeTimeout EventType = "timeout"
	// TypeBell indicates a terminal bell was detected
	TypeBell EventType = "bell"
	// TypeStateChange indicates an instance's waiting state changed
	TypeStateChange EventType = "state.change"
	// TypeMetricsChange indicates instance metrics were updated
	TypeMetricsChange EventType = "metrics.change"
)

// Event types for coordinator/orchestration events
const (
	// TypePhaseChange indicates the ultra-plan phase changed
	TypePhaseChange EventType = "phase.change"
	// TypeTaskStart indicates a task execution has begun
	TypeTaskStart EventType = "task.start"
	// TypeTaskComplete indicates a task completed successfully
	TypeTaskComplete EventType = "task.complete"
	// TypeTaskFailed indicates a task failed
	TypeTaskFailed EventType = "task.failed"
	// TypeTaskBlocked indicates a task is blocked
	TypeTaskBlocked EventType = "task.blocked"
	// TypeGroupComplete indicates an execution group completed
	TypeGroupComplete EventType = "group.complete"
	// TypePlanReady indicates a plan is ready after planning phase
	TypePlanReady EventType = "plan.ready"
	// TypeProgress indicates progress update
	TypeProgress EventType = "progress"
	// TypeComplete indicates the entire operation completed
	TypeComplete EventType = "complete"
)

// Event types for multi-pass planning
const (
	// TypeMultiPassPlanGenerated indicates one coordinator finished planning
	TypeMultiPassPlanGenerated EventType = "multipass.plan_generated"
	// TypeAllPlansGenerated indicates all coordinators finished planning
	TypeAllPlansGenerated EventType = "multipass.all_plans_generated"
	// TypePlanSelectionStarted indicates plan selection has begun
	TypePlanSelectionStarted EventType = "multipass.plan_selection_started"
	// TypePlanSelected indicates a plan was selected
	TypePlanSelected EventType = "multipass.plan_selected"
)

// Event types for consolidation events
const (
	// TypeConsolidationStarted indicates consolidation phase has begun
	TypeConsolidationStarted EventType = "consolidation.started"
	// TypeConsolidationGroupStarted indicates a consolidation group has started
	TypeConsolidationGroupStarted EventType = "consolidation.group_started"
	// TypeConsolidationTaskMerging indicates a task is being merged
	TypeConsolidationTaskMerging EventType = "consolidation.task_merging"
	// TypeConsolidationTaskMerged indicates a task was merged successfully
	TypeConsolidationTaskMerged EventType = "consolidation.task_merged"
	// TypeConsolidationGroupComplete indicates a consolidation group completed
	TypeConsolidationGroupComplete EventType = "consolidation.group_complete"
	// TypeConsolidationPRCreating indicates a PR is being created
	TypeConsolidationPRCreating EventType = "consolidation.pr_creating"
	// TypeConsolidationPRCreated indicates a PR was created
	TypeConsolidationPRCreated EventType = "consolidation.pr_created"
	// TypeConsolidationComplete indicates consolidation completed successfully
	TypeConsolidationComplete EventType = "consolidation.complete"
	// TypeConsolidationFailed indicates consolidation failed
	TypeConsolidationFailed EventType = "consolidation.failed"
)

// Event types for conflict detection
const (
	// TypeConflictDetected indicates file conflicts were detected between instances
	TypeConflictDetected EventType = "conflict.detected"
)

// Event is the interface that all events must implement.
// It provides a minimal contract for event identification and timing.
type Event interface {
	// Type returns the event type identifier
	Type() EventType
	// InstanceID returns the instance ID associated with this event, or empty string if not applicable
	InstanceID() string
	// Timestamp returns when the event occurred
	Timestamp() time.Time
}

// BaseEvent provides common fields for all events.
// Concrete event types should embed this struct.
type BaseEvent struct {
	eventType  EventType
	instanceID string
	timestamp  time.Time
}

// Type returns the event type identifier
func (e *BaseEvent) Type() EventType {
	return e.eventType
}

// InstanceID returns the instance ID associated with this event
func (e *BaseEvent) InstanceID() string {
	return e.instanceID
}

// Timestamp returns when the event occurred
func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

// NewBaseEvent creates a new BaseEvent with the current timestamp
func NewBaseEvent(eventType EventType, instanceID string) BaseEvent {
	return BaseEvent{
		eventType:  eventType,
		instanceID: instanceID,
		timestamp:  time.Now(),
	}
}

// -----------------------------------------------------------------------------
// Instance Events
// -----------------------------------------------------------------------------

// PRCompleteEvent is emitted when a PR workflow completes
type PRCompleteEvent struct {
	BaseEvent
	Success bool   // Whether the PR workflow succeeded
	Output  string // Output from the PR workflow
}

// NewPRCompleteEvent creates a new PRCompleteEvent
func NewPRCompleteEvent(instanceID string, success bool, output string) *PRCompleteEvent {
	return &PRCompleteEvent{
		BaseEvent: NewBaseEvent(TypePRComplete, instanceID),
		Success:   success,
		Output:    output,
	}
}

// PROpenedEvent is emitted when a PR URL is detected in instance output
type PROpenedEvent struct {
	BaseEvent
	PRURL string // The detected PR URL
}

// NewPROpenedEvent creates a new PROpenedEvent
func NewPROpenedEvent(instanceID string, prURL string) *PROpenedEvent {
	return &PROpenedEvent{
		BaseEvent: NewBaseEvent(TypePROpened, instanceID),
		PRURL:     prURL,
	}
}

// TimeoutType represents the type of timeout that occurred
type TimeoutType int

const (
	// TimeoutActivity indicates no activity for configured period
	TimeoutActivity TimeoutType = iota
	// TimeoutCompletion indicates total runtime exceeded limit
	TimeoutCompletion
	// TimeoutStale indicates repeated output detected (stuck in loop)
	TimeoutStale
)

// String returns a human-readable name for the timeout type
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

// TimeoutEvent is emitted when a timeout condition is detected
type TimeoutEvent struct {
	BaseEvent
	TimeoutType TimeoutType // The type of timeout that occurred
}

// NewTimeoutEvent creates a new TimeoutEvent
func NewTimeoutEvent(instanceID string, timeoutType TimeoutType) *TimeoutEvent {
	return &TimeoutEvent{
		BaseEvent:   NewBaseEvent(TypeTimeout, instanceID),
		TimeoutType: timeoutType,
	}
}

// BellEvent is emitted when a terminal bell is detected
type BellEvent struct {
	BaseEvent
}

// NewBellEvent creates a new BellEvent
func NewBellEvent(instanceID string) *BellEvent {
	return &BellEvent{
		BaseEvent: NewBaseEvent(TypeBell, instanceID),
	}
}

// WaitingState represents the detected waiting state of an instance
type WaitingState int

const (
	// StateWorking means Claude is actively working (not waiting)
	StateWorking WaitingState = iota
	// StateWaitingPermission means Claude is asking for permission
	StateWaitingPermission
	// StateWaitingQuestion means Claude is asking a question
	StateWaitingQuestion
	// StateWaitingInput means Claude is waiting for general input
	StateWaitingInput
)

// String returns a human-readable name for the waiting state
func (s WaitingState) String() string {
	switch s {
	case StateWorking:
		return "working"
	case StateWaitingPermission:
		return "waiting_permission"
	case StateWaitingQuestion:
		return "waiting_question"
	case StateWaitingInput:
		return "waiting_input"
	default:
		return "unknown"
	}
}

// StateChangeEvent is emitted when an instance's waiting state changes
type StateChangeEvent struct {
	BaseEvent
	State         WaitingState // The new waiting state
	PreviousState WaitingState // The previous waiting state
}

// NewStateChangeEvent creates a new StateChangeEvent
func NewStateChangeEvent(instanceID string, state, previousState WaitingState) *StateChangeEvent {
	return &StateChangeEvent{
		BaseEvent:     NewBaseEvent(TypeStateChange, instanceID),
		State:         state,
		PreviousState: previousState,
	}
}

// -----------------------------------------------------------------------------
// Coordinator/Orchestration Events
// -----------------------------------------------------------------------------

// UltraPlanPhase represents the current phase of an ultra-plan session
type UltraPlanPhase string

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

// PhaseChangeEvent is emitted when the ultra-plan phase changes
type PhaseChangeEvent struct {
	BaseEvent
	Phase         UltraPlanPhase // The new phase
	PreviousPhase UltraPlanPhase // The previous phase
	Message       string         // Optional message about the phase change
}

// NewPhaseChangeEvent creates a new PhaseChangeEvent
func NewPhaseChangeEvent(phase, previousPhase UltraPlanPhase, message string) *PhaseChangeEvent {
	return &PhaseChangeEvent{
		BaseEvent:     NewBaseEvent(TypePhaseChange, ""),
		Phase:         phase,
		PreviousPhase: previousPhase,
		Message:       message,
	}
}

// TaskStartEvent is emitted when a task execution begins
type TaskStartEvent struct {
	BaseEvent
	TaskID string // The task being started
}

// NewTaskStartEvent creates a new TaskStartEvent
func NewTaskStartEvent(taskID, instanceID string) *TaskStartEvent {
	return &TaskStartEvent{
		BaseEvent: NewBaseEvent(TypeTaskStart, instanceID),
		TaskID:    taskID,
	}
}

// TaskCompleteEvent is emitted when a task completes
type TaskCompleteEvent struct {
	BaseEvent
	TaskID  string // The completed task ID
	Success bool   // Whether the task succeeded
	Message string // Optional message about completion
}

// NewTaskCompleteEvent creates a new TaskCompleteEvent
func NewTaskCompleteEvent(taskID, instanceID string, success bool, message string) *TaskCompleteEvent {
	return &TaskCompleteEvent{
		BaseEvent: NewBaseEvent(TypeTaskComplete, instanceID),
		TaskID:    taskID,
		Success:   success,
		Message:   message,
	}
}

// TaskFailedEvent is emitted when a task fails
type TaskFailedEvent struct {
	BaseEvent
	TaskID string // The failed task ID
	Reason string // Reason for failure
}

// NewTaskFailedEvent creates a new TaskFailedEvent
func NewTaskFailedEvent(taskID, instanceID, reason string) *TaskFailedEvent {
	return &TaskFailedEvent{
		BaseEvent: NewBaseEvent(TypeTaskFailed, instanceID),
		TaskID:    taskID,
		Reason:    reason,
	}
}

// TaskBlockedEvent is emitted when a task is blocked
type TaskBlockedEvent struct {
	BaseEvent
	TaskID     string   // The blocked task ID
	BlockedBy  []string // Task IDs blocking this task
	Message    string   // Description of blockage
}

// NewTaskBlockedEvent creates a new TaskBlockedEvent
func NewTaskBlockedEvent(taskID, instanceID string, blockedBy []string, message string) *TaskBlockedEvent {
	return &TaskBlockedEvent{
		BaseEvent: NewBaseEvent(TypeTaskBlocked, instanceID),
		TaskID:    taskID,
		BlockedBy: blockedBy,
		Message:   message,
	}
}

// GroupCompleteEvent is emitted when an execution group completes
type GroupCompleteEvent struct {
	BaseEvent
	GroupIndex int    // The index of the completed group
	Message    string // Optional message
}

// NewGroupCompleteEvent creates a new GroupCompleteEvent
func NewGroupCompleteEvent(groupIndex int, message string) *GroupCompleteEvent {
	return &GroupCompleteEvent{
		BaseEvent:  NewBaseEvent(TypeGroupComplete, ""),
		GroupIndex: groupIndex,
		Message:    message,
	}
}

// ProgressEvent is emitted to indicate progress
type ProgressEvent struct {
	BaseEvent
	Completed int            // Number of completed items
	Total     int            // Total number of items
	Phase     UltraPlanPhase // Current phase
	Message   string         // Optional progress message
}

// NewProgressEvent creates a new ProgressEvent
func NewProgressEvent(completed, total int, phase UltraPlanPhase, message string) *ProgressEvent {
	return &ProgressEvent{
		BaseEvent: NewBaseEvent(TypeProgress, ""),
		Completed: completed,
		Total:     total,
		Phase:     phase,
		Message:   message,
	}
}

// CompleteEvent is emitted when the entire operation completes
type CompleteEvent struct {
	BaseEvent
	Success bool   // Whether the operation succeeded
	Summary string // Summary of the operation
}

// NewCompleteEvent creates a new CompleteEvent
func NewCompleteEvent(success bool, summary string) *CompleteEvent {
	return &CompleteEvent{
		BaseEvent: NewBaseEvent(TypeComplete, ""),
		Success:   success,
		Summary:   summary,
	}
}

// -----------------------------------------------------------------------------
// Multi-Pass Planning Events
// -----------------------------------------------------------------------------

// MultiPassPlanGeneratedEvent is emitted when one coordinator finishes planning
type MultiPassPlanGeneratedEvent struct {
	BaseEvent
	PlanIndex int    // Which plan was generated (0-indexed)
	Strategy  string // Planning strategy name
	Message   string // Optional message
}

// NewMultiPassPlanGeneratedEvent creates a new MultiPassPlanGeneratedEvent
func NewMultiPassPlanGeneratedEvent(planIndex int, strategy, message string) *MultiPassPlanGeneratedEvent {
	return &MultiPassPlanGeneratedEvent{
		BaseEvent: NewBaseEvent(TypeMultiPassPlanGenerated, ""),
		PlanIndex: planIndex,
		Strategy:  strategy,
		Message:   message,
	}
}

// PlanSelectedEvent is emitted when a plan is selected
type PlanSelectedEvent struct {
	BaseEvent
	PlanIndex int    // Which plan was selected (0-indexed)
	Strategy  string // Strategy of the selected plan
	Reason    string // Reason for selection
}

// NewPlanSelectedEvent creates a new PlanSelectedEvent
func NewPlanSelectedEvent(planIndex int, strategy, reason string) *PlanSelectedEvent {
	return &PlanSelectedEvent{
		BaseEvent: NewBaseEvent(TypePlanSelected, ""),
		PlanIndex: planIndex,
		Strategy:  strategy,
		Reason:    reason,
	}
}

// -----------------------------------------------------------------------------
// Consolidation Events
// -----------------------------------------------------------------------------

// ConsolidationStartedEvent is emitted when consolidation begins
type ConsolidationStartedEvent struct {
	BaseEvent
	TotalGroups int // Total number of groups to consolidate
}

// NewConsolidationStartedEvent creates a new ConsolidationStartedEvent
func NewConsolidationStartedEvent(totalGroups int) *ConsolidationStartedEvent {
	return &ConsolidationStartedEvent{
		BaseEvent:   NewBaseEvent(TypeConsolidationStarted, ""),
		TotalGroups: totalGroups,
	}
}

// ConsolidationGroupStartedEvent is emitted when a consolidation group starts
type ConsolidationGroupStartedEvent struct {
	BaseEvent
	GroupIndex int // Index of the group starting
	TaskCount  int // Number of tasks in the group
}

// NewConsolidationGroupStartedEvent creates a new ConsolidationGroupStartedEvent
func NewConsolidationGroupStartedEvent(groupIndex, taskCount int) *ConsolidationGroupStartedEvent {
	return &ConsolidationGroupStartedEvent{
		BaseEvent:  NewBaseEvent(TypeConsolidationGroupStarted, ""),
		GroupIndex: groupIndex,
		TaskCount:  taskCount,
	}
}

// ConsolidationTaskMergingEvent is emitted when a task is being merged
type ConsolidationTaskMergingEvent struct {
	BaseEvent
	GroupIndex int    // Group index
	TaskID     string // Task being merged
}

// NewConsolidationTaskMergingEvent creates a new ConsolidationTaskMergingEvent
func NewConsolidationTaskMergingEvent(groupIndex int, taskID string) *ConsolidationTaskMergingEvent {
	return &ConsolidationTaskMergingEvent{
		BaseEvent:  NewBaseEvent(TypeConsolidationTaskMerging, ""),
		GroupIndex: groupIndex,
		TaskID:     taskID,
	}
}

// ConsolidationTaskMergedEvent is emitted when a task was merged successfully
type ConsolidationTaskMergedEvent struct {
	BaseEvent
	GroupIndex int    // Group index
	TaskID     string // Task that was merged
}

// NewConsolidationTaskMergedEvent creates a new ConsolidationTaskMergedEvent
func NewConsolidationTaskMergedEvent(groupIndex int, taskID string) *ConsolidationTaskMergedEvent {
	return &ConsolidationTaskMergedEvent{
		BaseEvent:  NewBaseEvent(TypeConsolidationTaskMerged, ""),
		GroupIndex: groupIndex,
		TaskID:     taskID,
	}
}

// ConsolidationGroupCompleteEvent is emitted when a consolidation group completes
type ConsolidationGroupCompleteEvent struct {
	BaseEvent
	GroupIndex int    // Index of the completed group
	Message    string // Optional message
}

// NewConsolidationGroupCompleteEvent creates a new ConsolidationGroupCompleteEvent
func NewConsolidationGroupCompleteEvent(groupIndex int, message string) *ConsolidationGroupCompleteEvent {
	return &ConsolidationGroupCompleteEvent{
		BaseEvent:  NewBaseEvent(TypeConsolidationGroupComplete, ""),
		GroupIndex: groupIndex,
		Message:    message,
	}
}

// ConsolidationPRCreatedEvent is emitted when a PR is created during consolidation
type ConsolidationPRCreatedEvent struct {
	BaseEvent
	GroupIndex int    // Group index
	PRURL      string // URL of the created PR
	PRNumber   int    // PR number
}

// NewConsolidationPRCreatedEvent creates a new ConsolidationPRCreatedEvent
func NewConsolidationPRCreatedEvent(groupIndex int, prURL string, prNumber int) *ConsolidationPRCreatedEvent {
	return &ConsolidationPRCreatedEvent{
		BaseEvent:  NewBaseEvent(TypeConsolidationPRCreated, ""),
		GroupIndex: groupIndex,
		PRURL:      prURL,
		PRNumber:   prNumber,
	}
}

// ConsolidationCompleteEvent is emitted when consolidation completes
type ConsolidationCompleteEvent struct {
	BaseEvent
	Success     bool   // Whether consolidation succeeded
	Summary     string // Summary of consolidation
	PRsCreated  int    // Number of PRs created
}

// NewConsolidationCompleteEvent creates a new ConsolidationCompleteEvent
func NewConsolidationCompleteEvent(success bool, summary string, prsCreated int) *ConsolidationCompleteEvent {
	return &ConsolidationCompleteEvent{
		BaseEvent:  NewBaseEvent(TypeConsolidationComplete, ""),
		Success:    success,
		Summary:    summary,
		PRsCreated: prsCreated,
	}
}

// ConsolidationFailedEvent is emitted when consolidation fails
type ConsolidationFailedEvent struct {
	BaseEvent
	Reason     string // Reason for failure
	GroupIndex int    // Group index where failure occurred (-1 if not group-specific)
}

// NewConsolidationFailedEvent creates a new ConsolidationFailedEvent
func NewConsolidationFailedEvent(reason string, groupIndex int) *ConsolidationFailedEvent {
	return &ConsolidationFailedEvent{
		BaseEvent:  NewBaseEvent(TypeConsolidationFailed, ""),
		Reason:     reason,
		GroupIndex: groupIndex,
	}
}

// -----------------------------------------------------------------------------
// Conflict Detection Events
// -----------------------------------------------------------------------------

// FileConflict represents a detected file conflict between instances
type FileConflict struct {
	RelativePath string    // Path relative to worktree root
	Instances    []string  // Instance IDs that modified this file
	LastModified time.Time // When the conflict was last detected
}

// ConflictDetectedEvent is emitted when file conflicts are detected
type ConflictDetectedEvent struct {
	BaseEvent
	Conflicts []FileConflict // The detected conflicts
}

// NewConflictDetectedEvent creates a new ConflictDetectedEvent
func NewConflictDetectedEvent(instanceID string, conflicts []FileConflict) *ConflictDetectedEvent {
	return &ConflictDetectedEvent{
		BaseEvent: NewBaseEvent(TypeConflictDetected, instanceID),
		Conflicts: conflicts,
	}
}

// -----------------------------------------------------------------------------
// EventBus Interface
// -----------------------------------------------------------------------------

// Subscription represents an active event subscription.
// It provides a way to identify and manage subscriptions.
type Subscription interface {
	// ID returns a unique identifier for this subscription
	ID() string
	// EventType returns the event type this subscription is listening for
	EventType() EventType
	// Unsubscribe removes this subscription from the event bus
	Unsubscribe()
}

// Handler is a function that handles an event
type Handler func(Event)

// EventBus defines the interface for publishing and subscribing to events.
// Implementations should be thread-safe.
type EventBus interface {
	// Publish sends an event to all subscribers of that event type.
	// This method should be non-blocking; events may be queued for async delivery.
	Publish(event Event)

	// Subscribe registers a handler for a specific event type.
	// Returns a Subscription that can be used to unsubscribe.
	// The handler will be called for each event of the specified type.
	Subscribe(eventType EventType, handler Handler) Subscription

	// SubscribeAll registers a handler for all event types.
	// Returns a Subscription that can be used to unsubscribe.
	// The handler will be called for every event published.
	SubscribeAll(handler Handler) Subscription

	// Unsubscribe removes a subscription.
	// After calling this, the handler will no longer receive events.
	Unsubscribe(subscription Subscription)

	// Close shuts down the event bus and releases resources.
	// After Close is called, Publish and Subscribe will have no effect.
	Close()
}

// -----------------------------------------------------------------------------
// Basic Subscription Implementation
// -----------------------------------------------------------------------------

// subscription implements the Subscription interface
type subscription struct {
	id        string
	eventType EventType
	handler   Handler
	bus       EventBus
	mu        sync.Mutex
	active    bool
}

// ID returns the subscription's unique identifier
func (s *subscription) ID() string {
	return s.id
}

// EventType returns the event type this subscription is for
func (s *subscription) EventType() EventType {
	return s.eventType
}

// Unsubscribe removes this subscription from the event bus
func (s *subscription) Unsubscribe() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active && s.bus != nil {
		s.bus.Unsubscribe(s)
		s.active = false
	}
}

// NewSubscription creates a new subscription (for use by EventBus implementations)
func NewSubscription(id string, eventType EventType, handler Handler, bus EventBus) Subscription {
	return &subscription{
		id:        id,
		eventType: eventType,
		handler:   handler,
		bus:       bus,
		active:    true,
	}
}

// GetHandler returns the handler function (for use by EventBus implementations)
func GetHandler(s Subscription) Handler {
	if sub, ok := s.(*subscription); ok {
		return sub.handler
	}
	return nil
}

// IsActive returns whether the subscription is still active
func IsActive(s Subscription) bool {
	if sub, ok := s.(*subscription); ok {
		sub.mu.Lock()
		defer sub.mu.Unlock()
		return sub.active
	}
	return false
}

// Deactivate marks a subscription as inactive (for use by EventBus implementations)
func Deactivate(s Subscription) {
	if sub, ok := s.(*subscription); ok {
		sub.mu.Lock()
		defer sub.mu.Unlock()
		sub.active = false
	}
}
