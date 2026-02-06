package approval

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/taskqueue"
)

// Sentinel errors returned by gate operations.
var (
	ErrTaskNotFound        = errors.New("task not found")
	ErrNotAwaitingApproval = errors.New("task is not awaiting approval")
)

// TaskLookup returns whether a task with the given ID requires approval.
// This is typically backed by the planned task's RequiresApproval field.
type TaskLookup func(taskID string) (requiresApproval bool, exists bool)

// Gate wraps an EventQueue to intercept MarkRunning transitions for tasks
// that require human approval. Tasks with RequiresApproval=true are held
// in an "awaiting_approval" state until explicitly approved or rejected.
//
// For tasks that do not require approval, all operations pass through
// to the underlying EventQueue unchanged.
type Gate struct {
	mu      sync.Mutex
	eq      *taskqueue.EventQueue
	bus     *event.Bus
	lookup  TaskLookup
	pending map[string]string // taskID -> instanceID for tasks awaiting approval
}

// NewGate creates a Gate that wraps the given EventQueue.
// The lookup function determines whether a given task requires approval.
func NewGate(eq *taskqueue.EventQueue, bus *event.Bus, lookup TaskLookup) *Gate {
	return &Gate{
		eq:      eq,
		bus:     bus,
		lookup:  lookup,
		pending: make(map[string]string),
	}
}

// MarkRunning transitions a task to running. If the task requires approval,
// it is instead placed into the awaiting_approval state and a
// TaskAwaitingApprovalEvent is published. For tasks that do not require
// approval, the call is passed through to the underlying EventQueue.
func (g *Gate) MarkRunning(taskID string) error {
	g.mu.Lock()

	requiresApproval, exists := g.lookup(taskID)
	if !exists {
		g.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}

	if !requiresApproval {
		g.mu.Unlock()
		return g.eq.MarkRunning(taskID)
	}

	// Look up the task to get the instance ID and verify it's claimed.
	task := g.eq.GetTask(taskID)
	if task == nil {
		g.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	if task.Status != taskqueue.TaskClaimed {
		g.mu.Unlock()
		return fmt.Errorf("%w: cannot hold task %s in status %s for approval",
			taskqueue.ErrInvalidTransition, taskID, task.Status)
	}

	g.pending[taskID] = task.ClaimedBy
	claimedBy := task.ClaimedBy
	g.mu.Unlock()

	// Publish events outside the mutex to avoid deadlock with event bus handlers.
	g.bus.Publish(event.NewTaskAwaitingApprovalEvent(taskID, claimedBy))
	g.publishDepth()
	return nil
}

// Approve resumes a task that is awaiting approval, transitioning it to running.
func (g *Gate) Approve(taskID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.pending[taskID]; !ok {
		return fmt.Errorf("%w: %s", ErrNotAwaitingApproval, taskID)
	}

	if err := g.eq.MarkRunning(taskID); err != nil {
		return fmt.Errorf("approve task: %w", err)
	}

	delete(g.pending, taskID)
	return nil
}

// Reject fails a task that is awaiting approval with the given reason.
func (g *Gate) Reject(taskID, reason string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.pending[taskID]; !ok {
		return fmt.Errorf("%w: %s", ErrNotAwaitingApproval, taskID)
	}

	if err := g.eq.Fail(taskID, reason); err != nil {
		return fmt.Errorf("reject task: %w", err)
	}

	delete(g.pending, taskID)
	return nil
}

// PendingApprovals returns the task IDs currently awaiting approval.
// The returned slice is a copy and safe to modify.
func (g *Gate) PendingApprovals() []string {
	g.mu.Lock()
	defer g.mu.Unlock()

	ids := make([]string, 0, len(g.pending))
	for id := range g.pending {
		ids = append(ids, id)
	}
	return ids
}

// IsAwaitingApproval returns true if the given task is currently awaiting approval.
func (g *Gate) IsAwaitingApproval(taskID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	_, ok := g.pending[taskID]
	return ok
}

// ClaimNext delegates to the underlying EventQueue.
func (g *Gate) ClaimNext(instanceID string) (*taskqueue.QueuedTask, error) {
	return g.eq.ClaimNext(instanceID)
}

// Complete delegates to the underlying EventQueue.
func (g *Gate) Complete(taskID string) ([]string, error) {
	return g.eq.Complete(taskID)
}

// Fail delegates to the underlying EventQueue.
func (g *Gate) Fail(taskID, failureContext string) error {
	return g.eq.Fail(taskID, failureContext)
}

// Release delegates to the underlying EventQueue and cleans up pending approvals.
func (g *Gate) Release(taskID, reason string) error {
	g.mu.Lock()
	delete(g.pending, taskID)
	g.mu.Unlock()

	return g.eq.Release(taskID, reason)
}

// Status delegates to the underlying EventQueue and adjusts counts for
// tasks awaiting approval. Tasks tracked in the gate's pending map are
// counted as AwaitingApproval rather than Claimed.
func (g *Gate) Status() taskqueue.QueueStatus {
	g.mu.Lock()
	pendingCount := len(g.pending)
	g.mu.Unlock()

	s := g.eq.Status()
	s.AwaitingApproval += pendingCount
	s.Claimed -= pendingCount
	if s.Claimed < 0 {
		s.Claimed = 0
	}
	return s
}

// IsComplete delegates to the underlying EventQueue.
func (g *Gate) IsComplete() bool {
	return g.eq.IsComplete()
}

// GetTask delegates to the underlying EventQueue. If the task is awaiting
// approval, its status is overridden to TaskAwaitingApproval.
func (g *Gate) GetTask(taskID string) *taskqueue.QueuedTask {
	task := g.eq.GetTask(taskID)
	if task == nil {
		return nil
	}

	g.mu.Lock()
	_, awaiting := g.pending[taskID]
	g.mu.Unlock()

	if awaiting {
		task.Status = taskqueue.TaskAwaitingApproval
	}
	return task
}

// GetInstanceTasks delegates to the underlying EventQueue.
func (g *Gate) GetInstanceTasks(instanceID string) []*taskqueue.QueuedTask {
	return g.eq.GetInstanceTasks(instanceID)
}

// SaveState delegates to the underlying EventQueue.
func (g *Gate) SaveState(dir string) error {
	return g.eq.SaveState(dir)
}

// ClaimStaleBefore delegates to the underlying EventQueue and cleans up
// any pending approvals for released tasks.
func (g *Gate) ClaimStaleBefore(cutoff time.Time) []string {
	released := g.eq.ClaimStaleBefore(cutoff)

	g.mu.Lock()
	for _, id := range released {
		delete(g.pending, id)
	}
	g.mu.Unlock()

	return released
}

// publishDepth publishes a QueueDepthChangedEvent with adjusted counts.
// It reads g.pending under the lock to get the count, then publishes outside
// the lock to avoid deadlock with event bus handlers.
func (g *Gate) publishDepth() {
	g.mu.Lock()
	pendingCount := len(g.pending)
	g.mu.Unlock()

	s := g.eq.Status()
	claimed := s.Claimed - pendingCount
	if claimed < 0 {
		claimed = 0
	}
	g.bus.Publish(event.NewQueueDepthChangedEvent(
		s.Pending, claimed, s.Running, s.Completed, s.Failed, s.Total,
	))
}
