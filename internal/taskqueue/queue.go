package taskqueue

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// Default maximum retries for failed tasks.
const defaultMaxRetries = 2

// Sentinel errors returned by queue operations.
var (
	ErrTaskNotFound      = errors.New("task not found")
	ErrInvalidTransition = errors.New("invalid status transition")
)

// TaskQueue manages a set of tasks with dependency-aware claiming.
// All methods are safe for concurrent use via an internal mutex.
type TaskQueue struct {
	mu     sync.Mutex
	tasks  map[string]*QueuedTask // taskID -> task
	claims map[string]string      // taskID -> instanceID
	order  []string               // task IDs in priority/topological order
}

// NewFromPlan creates a TaskQueue from an Ultra-Plan specification.
// Each PlannedTask is embedded into a QueuedTask, preserving all
// planning fields. Dependencies come from each task's DependsOn field.
func NewFromPlan(plan *ultraplan.PlanSpec) *TaskQueue {
	tasks := make(map[string]*QueuedTask, len(plan.Tasks))
	claims := make(map[string]string)
	for i := range plan.Tasks {
		pt := plan.Tasks[i]
		if pt.DependsOn == nil {
			pt.DependsOn = []string{}
		}
		tasks[pt.ID] = &QueuedTask{
			PlannedTask: pt,
			Status:      TaskPending,
			MaxRetries:  defaultMaxRetries,
		}
	}

	order := buildPriorityOrder(tasks)

	return &TaskQueue{
		tasks:  tasks,
		claims: claims,
		order:  order,
	}
}

// newFromTasks creates a TaskQueue from pre-built task maps and order.
// Used internally for loading persisted state.
func newFromTasks(tasks map[string]*QueuedTask, order []string) *TaskQueue {
	claims := make(map[string]string)
	for id, task := range tasks {
		if task.ClaimedBy != "" {
			claims[id] = task.ClaimedBy
		}
	}
	return &TaskQueue{
		tasks:  tasks,
		claims: claims,
		order:  order,
	}
}

// ClaimNext returns the next claimable task for the given instance.
// A task is claimable if it is pending and all its dependencies are completed.
// Returns nil with no error if no tasks are currently available.
func (q *TaskQueue) ClaimNext(instanceID string) (*QueuedTask, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if instanceID == "" {
		return nil, errors.New("instanceID must not be empty")
	}

	for _, id := range q.order {
		task := q.tasks[id]
		if q.isClaimable(task) {
			now := time.Now()
			task.Status = TaskClaimed
			task.ClaimedBy = instanceID
			task.ClaimedAt = &now
			q.claims[task.ID] = instanceID
			// Return a copy to avoid data races on the internal task pointer.
			cp := *task
			return &cp, nil
		}
	}
	return nil, nil
}

// MarkRunning transitions a claimed task to the running state.
func (q *TaskQueue) MarkRunning(taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.tasks[taskID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	if task.Status != TaskClaimed {
		return fmt.Errorf("%w: cannot transition %s from %s to running", ErrInvalidTransition, taskID, task.Status)
	}
	task.Status = TaskRunning
	return nil
}

// Complete marks a task as completed and returns the IDs of tasks
// that are newly claimable as a result.
func (q *TaskQueue) Complete(taskID string) ([]string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	if task.Status != TaskRunning && task.Status != TaskClaimed {
		return nil, fmt.Errorf("%w: cannot complete task %s in status %s", ErrInvalidTransition, taskID, task.Status)
	}
	now := time.Now()
	task.Status = TaskCompleted
	task.CompletedAt = &now

	unblocked := q.unblockedBy(taskID)
	return unblocked, nil
}

// Fail marks a task as failed. If retries remain, the task is returned
// to pending status for re-claiming. Otherwise it is permanently failed.
func (q *TaskQueue) Fail(taskID, failureContext string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.tasks[taskID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	if task.Status != TaskRunning && task.Status != TaskClaimed {
		return fmt.Errorf("%w: cannot fail task %s in status %s", ErrInvalidTransition, taskID, task.Status)
	}

	task.RetryCount++
	task.FailureContext = failureContext

	if task.RetryCount <= task.MaxRetries {
		// Return to pending for retry
		task.Status = TaskPending
		task.ClaimedBy = ""
		task.ClaimedAt = nil
		delete(q.claims, taskID)
	} else {
		// Permanently failed
		now := time.Now()
		task.Status = TaskFailed
		task.CompletedAt = &now
	}
	return nil
}

// Release returns a claimed or running task back to pending status.
// Used for stale claim cleanup when an instance dies.
func (q *TaskQueue) Release(taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.tasks[taskID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	if task.Status != TaskClaimed && task.Status != TaskRunning {
		return fmt.Errorf("%w: cannot release task %s in status %s", ErrInvalidTransition, taskID, task.Status)
	}

	task.Status = TaskPending
	task.ClaimedBy = ""
	task.ClaimedAt = nil
	delete(q.claims, taskID)
	return nil
}

// Status returns a snapshot of the current queue state counts.
func (q *TaskQueue) Status() QueueStatus {
	q.mu.Lock()
	defer q.mu.Unlock()

	var s QueueStatus
	s.Total = len(q.tasks)
	for _, task := range q.tasks {
		switch task.Status {
		case TaskPending:
			s.Pending++
		case TaskClaimed:
			s.Claimed++
		case TaskAwaitingApproval:
			s.AwaitingApproval++
		case TaskRunning:
			s.Running++
		case TaskCompleted:
			s.Completed++
		case TaskFailed:
			s.Failed++
		}
	}
	return s
}

// IsComplete returns true when all tasks are in a terminal state
// (completed or permanently failed).
func (q *TaskQueue) IsComplete() bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, task := range q.tasks {
		if !task.Status.IsTerminal() {
			return false
		}
	}
	return len(q.tasks) > 0
}

// GetTask returns the task with the given ID, or nil if not found.
func (q *TaskQueue) GetTask(taskID string) *QueuedTask {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.tasks[taskID]
	if !ok {
		return nil
	}
	// Return a copy to avoid data races
	cp := *task
	return &cp
}

// SetMaxRetries sets the maximum number of retries for the given task.
// The task must exist and be in a non-terminal state.
func (q *TaskQueue) SetMaxRetries(taskID string, maxRetries int) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.tasks[taskID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	if task.Status.IsTerminal() {
		return fmt.Errorf("%w: cannot set max retries on %s task %s", ErrInvalidTransition, task.Status, taskID)
	}
	task.MaxRetries = maxRetries
	return nil
}

// ReleaseStaleClaimed releases tasks that have been claimed but not marked
// running before the given cutoff time. Returns the IDs of released tasks.
// This is used for recovering from instances that died while holding a claim.
func (q *TaskQueue) ReleaseStaleClaimed(cutoff time.Time) []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	var released []string
	for _, task := range q.tasks {
		if task.Status == TaskClaimed && task.ClaimedAt != nil && task.ClaimedAt.Before(cutoff) {
			task.Status = TaskPending
			task.ClaimedBy = ""
			task.ClaimedAt = nil
			delete(q.claims, task.ID)
			released = append(released, task.ID)
		}
	}
	return released
}

// GetInstanceTasks returns all tasks claimed by or running on the given instance.
func (q *TaskQueue) GetInstanceTasks(instanceID string) []*QueuedTask {
	q.mu.Lock()
	defer q.mu.Unlock()

	var result []*QueuedTask
	for _, id := range q.order {
		task := q.tasks[id]
		if task.ClaimedBy == instanceID {
			cp := *task
			result = append(result, &cp)
		}
	}
	return result
}
