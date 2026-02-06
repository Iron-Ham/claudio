package taskqueue

import (
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
)

// EventQueue wraps a TaskQueue and publishes events to an event bus
// whenever queue operations occur.
type EventQueue struct {
	mu  sync.Mutex
	q   *TaskQueue
	bus *event.Bus
}

// NewEventQueue creates an EventQueue that publishes events on the given bus.
func NewEventQueue(q *TaskQueue, bus *event.Bus) *EventQueue {
	return &EventQueue{q: q, bus: bus}
}

// ClaimNext claims the next available task and publishes a TaskClaimedEvent
// and a QueueDepthChangedEvent.
func (eq *EventQueue) ClaimNext(instanceID string) (*QueuedTask, error) {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	task, err := eq.q.ClaimNext(instanceID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, nil
	}

	eq.bus.Publish(event.NewTaskClaimedEvent(task.ID, instanceID))
	eq.publishDepth()
	return task, nil
}

// MarkRunning transitions a task to running and publishes a QueueDepthChangedEvent.
func (eq *EventQueue) MarkRunning(taskID string) error {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if err := eq.q.MarkRunning(taskID); err != nil {
		return err
	}
	eq.publishDepth()
	return nil
}

// Complete marks a task completed and publishes a QueueDepthChangedEvent.
// Returns the list of newly unblocked task IDs.
func (eq *EventQueue) Complete(taskID string) ([]string, error) {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	unblocked, err := eq.q.Complete(taskID)
	if err != nil {
		return nil, err
	}
	eq.publishDepth()
	return unblocked, nil
}

// Fail marks a task as failed and publishes a QueueDepthChangedEvent.
func (eq *EventQueue) Fail(taskID, failureContext string) error {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if err := eq.q.Fail(taskID, failureContext); err != nil {
		return err
	}
	eq.publishDepth()
	return nil
}

// Release returns a task to the queue and publishes TaskReleasedEvent
// and QueueDepthChangedEvent.
func (eq *EventQueue) Release(taskID, reason string) error {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if err := eq.q.Release(taskID); err != nil {
		return err
	}
	eq.bus.Publish(event.NewTaskReleasedEvent(taskID, reason))
	eq.publishDepth()
	return nil
}

// Status returns the current queue status snapshot.
func (eq *EventQueue) Status() QueueStatus {
	return eq.q.Status()
}

// IsComplete returns true when all tasks are in a terminal state.
func (eq *EventQueue) IsComplete() bool {
	return eq.q.IsComplete()
}

// GetTask returns the task with the given ID.
func (eq *EventQueue) GetTask(taskID string) *QueuedTask {
	return eq.q.GetTask(taskID)
}

// GetInstanceTasks returns all tasks for the given instance.
func (eq *EventQueue) GetInstanceTasks(instanceID string) []*QueuedTask {
	return eq.q.GetInstanceTasks(instanceID)
}

// SaveState persists the queue state to disk.
func (eq *EventQueue) SaveState(dir string) error {
	return eq.q.SaveState(dir)
}

// publishDepth publishes a QueueDepthChangedEvent with current counts.
// Must be called while eq.mu is held.
func (eq *EventQueue) publishDepth() {
	s := eq.q.Status()
	eq.bus.Publish(event.NewQueueDepthChangedEvent(
		s.Pending, s.Claimed, s.Running, s.Completed, s.Failed, s.Total,
	))
}

// Ensure the new event types satisfy the Event interface at compile time.
var (
	_ event.Event = event.TaskClaimedEvent{}
	_ event.Event = event.TaskReleasedEvent{}
	_ event.Event = event.QueueDepthChangedEvent{}
)

// Ensure error sentinels are usable with errors.Is.
var (
	_ error = ErrTaskNotFound
	_ error = ErrInvalidTransition
)

// ClaimStaleBefore releases tasks that have been claimed but not marked
// running before the given cutoff time. This is useful for recovering from
// instances that died while holding a claim.
func (eq *EventQueue) ClaimStaleBefore(cutoff time.Time) []string {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	released := eq.q.ReleaseStaleClaimed(cutoff)

	for _, id := range released {
		eq.bus.Publish(event.NewTaskReleasedEvent(id, "stale_claim"))
	}
	if len(released) > 0 {
		eq.publishDepth()
	}
	return released
}
