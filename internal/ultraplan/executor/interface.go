// Package executor defines interfaces for coordinating Ultra-Plan task execution.
//
// This package provides the abstraction layer between Ultra-Plan's planning phase
// and the actual execution of tasks. It defines how individual tasks are executed
// and how the overall execution is coordinated.
package executor

import (
	"context"
	"time"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// TaskExecutor defines the interface for executing a single Ultra-Plan task.
// Implementations handle the mechanics of running a Claude Code instance
// with the task's configuration and monitoring its progress.
type TaskExecutor interface {
	// Execute runs the given task and blocks until completion or error.
	//
	// The context can be used to cancel execution early. If cancelled,
	// the implementation should attempt graceful shutdown of the underlying
	// process before returning.
	//
	// Returns nil on successful completion, or an error describing what went wrong.
	Execute(ctx context.Context, task *ultraplan.PlannedTask) error

	// GetStatus returns the current execution status of this executor.
	// This can be called during execution to monitor progress.
	GetStatus() ExecutionStatus
}

// ExecutionCoordinator defines the interface for orchestrating parallel task execution.
// It manages the execution of an entire Ultra-Plan, handling dependencies,
// parallelism, and progress tracking.
type ExecutionCoordinator interface {
	// Start begins execution of the given plan.
	//
	// This returns immediately after initiating execution. Use GetProgress()
	// to monitor progress and Wait() to block until completion.
	//
	// Returns an error if the plan cannot be started (e.g., validation failure).
	Start(plan *ultraplan.PlanSpec) error

	// Stop halts execution of all currently running tasks.
	//
	// This attempts graceful shutdown of all running executors. Tasks that
	// haven't started yet will not be started. Already completed tasks
	// are not affected.
	//
	// Returns an error if stopping fails.
	Stop() error

	// GetProgress returns the current execution progress.
	// Safe to call concurrently during execution.
	GetProgress() ExecutionProgress

	// Wait blocks until all tasks complete or an error occurs.
	// Returns nil if all tasks completed successfully.
	Wait() error
}

// ExecutionStatus represents the current status of a single task executor.
type ExecutionStatus int

const (
	// StatusPending indicates the task has not yet started execution.
	StatusPending ExecutionStatus = iota

	// StatusRunning indicates the task is currently executing.
	StatusRunning

	// StatusCompleted indicates the task finished successfully.
	StatusCompleted

	// StatusFailed indicates the task failed with an error.
	StatusFailed

	// StatusCancelled indicates the task was cancelled before completion.
	StatusCancelled
)

// String returns a human-readable string for the status.
func (s ExecutionStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// IsTerminal returns true if this status represents a final state.
func (s ExecutionStatus) IsTerminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled
}

// ExecutionProgress contains the overall progress of plan execution.
type ExecutionProgress struct {
	// TotalTasks is the total number of tasks in the plan.
	TotalTasks int

	// CompletedTasks is the number of tasks that have completed successfully.
	CompletedTasks int

	// RunningTasks is the number of tasks currently executing.
	RunningTasks int

	// FailedTasks is the number of tasks that have failed.
	FailedTasks int

	// PendingTasks is the number of tasks waiting to be executed.
	PendingTasks int

	// CancelledTasks is the number of tasks that were cancelled.
	CancelledTasks int

	// TaskStatuses maps task ID to its current status.
	TaskStatuses map[string]TaskStatus

	// StartTime is when execution began.
	StartTime time.Time

	// EndTime is when execution completed (zero if still running).
	EndTime time.Time
}

// TaskStatus contains the execution status and metadata for a single task.
type TaskStatus struct {
	// TaskID is the identifier of the task.
	TaskID string

	// Status is the current execution status.
	Status ExecutionStatus

	// StartTime is when the task started executing (zero if pending).
	StartTime time.Time

	// EndTime is when the task finished (zero if still running or pending).
	EndTime time.Time

	// Error contains the error message if the task failed.
	Error string

	// InstanceID is the ID of the Claude instance executing this task.
	// Empty if the task hasn't started yet.
	InstanceID string

	// WorktreePath is the path to the git worktree for this task.
	// Empty if not yet assigned.
	WorktreePath string
}

// Duration returns how long the task has been or was running.
// Returns zero for pending tasks.
func (s *TaskStatus) Duration() time.Duration {
	if s.StartTime.IsZero() {
		return 0
	}
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// IsComplete returns true if all tasks have reached a terminal state.
func (p *ExecutionProgress) IsComplete() bool {
	return p.CompletedTasks+p.FailedTasks+p.CancelledTasks == p.TotalTasks
}

// SuccessRate returns the percentage of tasks that completed successfully.
// Returns 0 if no tasks have finished yet.
func (p *ExecutionProgress) SuccessRate() float64 {
	finished := p.CompletedTasks + p.FailedTasks + p.CancelledTasks
	if finished == 0 {
		return 0
	}
	return float64(p.CompletedTasks) / float64(finished) * 100
}

// Duration returns the total execution time so far.
func (p *ExecutionProgress) Duration() time.Duration {
	if p.StartTime.IsZero() {
		return 0
	}
	if p.EndTime.IsZero() {
		return time.Since(p.StartTime)
	}
	return p.EndTime.Sub(p.StartTime)
}

// RemainingEstimate estimates time remaining based on current progress.
// Returns zero if no tasks have completed or if progress cannot be estimated.
func (p *ExecutionProgress) RemainingEstimate() time.Duration {
	if p.CompletedTasks == 0 || p.IsComplete() {
		return 0
	}

	elapsed := p.Duration()
	avgPerTask := elapsed / time.Duration(p.CompletedTasks)
	remaining := p.TotalTasks - p.CompletedTasks - p.FailedTasks - p.CancelledTasks
	return avgPerTask * time.Duration(remaining)
}

// NewExecutionProgress creates a new ExecutionProgress for a plan.
func NewExecutionProgress(plan *ultraplan.PlanSpec) ExecutionProgress {
	progress := ExecutionProgress{
		TotalTasks:   len(plan.Tasks),
		PendingTasks: len(plan.Tasks),
		TaskStatuses: make(map[string]TaskStatus),
		StartTime:    time.Now(),
	}

	for _, task := range plan.Tasks {
		progress.TaskStatuses[task.ID] = TaskStatus{
			TaskID: task.ID,
			Status: StatusPending,
		}
	}

	return progress
}
