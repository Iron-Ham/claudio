package taskqueue

import (
	"time"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// TaskStatus represents the current state of a queued task.
type TaskStatus string

const (
	// TaskPending indicates the task is waiting to be claimed.
	TaskPending TaskStatus = "pending"

	// TaskClaimed indicates the task has been claimed by an instance
	// but has not yet started running.
	TaskClaimed TaskStatus = "claimed"

	// TaskRunning indicates the task is actively being executed.
	TaskRunning TaskStatus = "running"

	// TaskCompleted indicates the task finished successfully.
	TaskCompleted TaskStatus = "completed"

	// TaskFailed indicates the task failed and exhausted all retries.
	TaskFailed TaskStatus = "failed"
)

// String returns the string representation of the task status.
func (s TaskStatus) String() string {
	return string(s)
}

// IsTerminal returns true if this status represents a final state.
func (s TaskStatus) IsTerminal() bool {
	return s == TaskCompleted || s == TaskFailed
}

// QueuedTask wraps an ultraplan.PlannedTask with execution state for the
// dynamic task queue. It embeds the planned task so all planning fields
// (ID, Title, Description, Files, DependsOn, Priority, EstComplexity)
// are directly accessible.
type QueuedTask struct {
	// PlannedTask is the underlying task from the plan.
	ultraplan.PlannedTask

	// Status is the current execution state.
	Status TaskStatus `json:"status"`

	// ClaimedBy is the instance ID that claimed this task.
	ClaimedBy string `json:"claimed_by,omitempty"`

	// ClaimedAt is when the task was claimed.
	ClaimedAt *time.Time `json:"claimed_at,omitempty"`

	// CompletedAt is when the task reached a terminal state.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// RetryCount is the number of retry attempts so far.
	RetryCount int `json:"retry_count"`

	// MaxRetries is the maximum number of retry attempts allowed.
	MaxRetries int `json:"max_retries"`

	// FailureContext contains error context from the most recent failure.
	FailureContext string `json:"failure_context,omitempty"`
}

// QueueStatus is a snapshot of the queue's current state counts.
type QueueStatus struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	Claimed   int `json:"claimed"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}
