// Package executor provides interfaces and types for task execution abstraction.
// This package decouples task execution logic from coordination, enabling
// flexible execution strategies and better testability.
package executor

import (
	"time"
)

// TaskStatus represents the current execution state of a task
type TaskStatus string

const (
	// TaskStatusPending indicates the task has been created but not yet started
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusRunning indicates the task is currently being executed
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusComplete indicates the task finished successfully
	TaskStatusComplete TaskStatus = "complete"
	// TaskStatusFailed indicates the task encountered an error and could not complete
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusCancelled indicates the task was stopped before completion
	TaskStatusCancelled TaskStatus = "cancelled"
)

// IsTerminal returns true if the status represents a final state
func (s TaskStatus) IsTerminal() bool {
	return s == TaskStatusComplete || s == TaskStatusFailed || s == TaskStatusCancelled
}

// TaskComplexity represents the estimated complexity of a task
type TaskComplexity string

const (
	ComplexityLow    TaskComplexity = "low"
	ComplexityMedium TaskComplexity = "medium"
	ComplexityHigh   TaskComplexity = "high"
)

// Task represents a unit of work to be executed by a TaskExecutor.
// Tasks are immutable once created and contain all information needed
// for execution.
type Task struct {
	// ID uniquely identifies the task
	ID string `json:"id"`

	// Title is a short human-readable name for the task
	Title string `json:"title"`

	// Description contains the detailed prompt or instructions for execution
	Description string `json:"description"`

	// Files lists the expected files to be modified by this task
	Files []string `json:"files,omitempty"`

	// DependsOn contains IDs of tasks that must complete before this one can start
	DependsOn []string `json:"depends_on,omitempty"`

	// Priority determines execution order (lower values = higher priority)
	Priority int `json:"priority"`

	// Complexity is the estimated difficulty of the task
	Complexity TaskComplexity `json:"complexity,omitempty"`

	// WorktreePath is the path to the git worktree where the task should execute
	WorktreePath string `json:"worktree_path,omitempty"`

	// Branch is the git branch name for this task's work
	Branch string `json:"branch,omitempty"`

	// Context provides additional context passed from the coordinator
	Context map[string]string `json:"context,omitempty"`

	// Timeout specifies the maximum duration for task execution (0 = no timeout)
	Timeout time.Duration `json:"timeout,omitempty"`

	// RetryCount tracks how many times this task has been retried
	RetryCount int `json:"retry_count,omitempty"`

	// MaxRetries specifies the maximum number of retry attempts
	MaxRetries int `json:"max_retries,omitempty"`
}

// CompletionResult contains the outcome of a completed task execution
type CompletionResult struct {
	// Success indicates whether the task completed successfully
	Success bool `json:"success"`

	// Status is the final task status
	Status TaskStatus `json:"status"`

	// Output contains any output produced by the task
	Output string `json:"output,omitempty"`

	// Error contains the error message if the task failed
	Error string `json:"error,omitempty"`

	// FilesModified lists files that were actually modified during execution
	FilesModified []string `json:"files_modified,omitempty"`

	// CommitSHA is the git commit SHA if the task produced a commit
	CommitSHA string `json:"commit_sha,omitempty"`

	// CommitCount is the number of commits produced by this task
	CommitCount int `json:"commit_count,omitempty"`

	// Metrics contains resource usage information
	Metrics *ExecutionMetrics `json:"metrics,omitempty"`

	// StartedAt records when execution began
	StartedAt time.Time `json:"started_at"`

	// CompletedAt records when execution finished
	CompletedAt time.Time `json:"completed_at"`

	// Duration is the total execution time
	Duration time.Duration `json:"duration"`
}

// ExecutionMetrics tracks resource usage during task execution
type ExecutionMetrics struct {
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CacheRead    int64   `json:"cache_read,omitempty"`
	CacheWrite   int64   `json:"cache_write,omitempty"`
	Cost         float64 `json:"cost"`
	APICalls     int     `json:"api_calls"`
}

// TotalTokens returns the sum of input and output tokens
func (m *ExecutionMetrics) TotalTokens() int64 {
	if m == nil {
		return 0
	}
	return m.InputTokens + m.OutputTokens
}

// TaskExecutorConfig holds configuration for a TaskExecutor
type TaskExecutorConfig struct {
	// MaxConcurrent is the maximum number of tasks that can run simultaneously
	MaxConcurrent int `json:"max_concurrent"`

	// DefaultTimeout is the default execution timeout for tasks without one specified
	DefaultTimeout time.Duration `json:"default_timeout,omitempty"`

	// ActivityTimeout is the maximum time a task can be idle before being marked stuck
	ActivityTimeout time.Duration `json:"activity_timeout,omitempty"`

	// RetryFailedTasks enables automatic retry of failed tasks
	RetryFailedTasks bool `json:"retry_failed_tasks"`

	// MaxRetries is the default maximum retry count for tasks
	MaxRetries int `json:"max_retries"`

	// RequireCommits requires tasks to produce commits to be considered successful
	RequireCommits bool `json:"require_commits"`

	// WorktreeBasePath is the base directory for creating worktrees
	WorktreeBasePath string `json:"worktree_base_path,omitempty"`

	// BranchPrefix is the prefix for branch names created by the executor
	BranchPrefix string `json:"branch_prefix,omitempty"`
}

// DefaultTaskExecutorConfig returns the default executor configuration
func DefaultTaskExecutorConfig() TaskExecutorConfig {
	return TaskExecutorConfig{
		MaxConcurrent:    3,
		DefaultTimeout:   30 * time.Minute,
		ActivityTimeout:  5 * time.Minute,
		RetryFailedTasks: true,
		MaxRetries:       3,
		RequireCommits:   true,
	}
}

// TaskExecutor defines the interface for executing tasks.
// Implementations are responsible for managing the lifecycle of task execution,
// including starting, stopping, and monitoring task progress.
type TaskExecutor interface {
	// StartTask begins execution of a task in the specified instance.
	// The instanceID identifies the execution context (e.g., tmux session, worktree).
	// Returns an error if the task cannot be started.
	StartTask(task Task, instanceID string) error

	// StopTask cancels execution of a task running in the specified instance.
	// This should perform a graceful shutdown when possible.
	// Returns an error if the task cannot be stopped.
	StopTask(instanceID string) error

	// GetTaskStatus returns the current status of a task in the specified instance.
	// Returns TaskStatusPending if the instance is unknown.
	GetTaskStatus(instanceID string) TaskStatus

	// WaitForCompletion blocks until the task completes or the timeout expires.
	// A zero timeout means wait indefinitely.
	// Returns the completion result or an error if the wait fails (e.g., timeout).
	WaitForCompletion(instanceID string, timeout time.Duration) (CompletionResult, error)
}

// ExecutorEventType identifies the type of event emitted by a TaskExecutor
type ExecutorEventType string

const (
	// EventTaskStarted is emitted when a task begins execution
	EventTaskStarted ExecutorEventType = "task_started"
	// EventTaskComplete is emitted when a task finishes successfully
	EventTaskComplete ExecutorEventType = "task_complete"
	// EventTaskFailed is emitted when a task fails
	EventTaskFailed ExecutorEventType = "task_failed"
	// EventTaskCancelled is emitted when a task is cancelled
	EventTaskCancelled ExecutorEventType = "task_cancelled"
	// EventTaskProgress is emitted to report incremental task progress
	EventTaskProgress ExecutorEventType = "task_progress"
	// EventTaskRetry is emitted when a failed task is being retried
	EventTaskRetry ExecutorEventType = "task_retry"
	// EventTaskStuck is emitted when a task appears to be making no progress
	EventTaskStuck ExecutorEventType = "task_stuck"
)

// ExecutorEvent represents an event emitted during task execution.
// Events provide real-time visibility into execution progress and can be
// used by coordinators and UIs to update state.
type ExecutorEvent struct {
	// Type identifies what kind of event this is
	Type ExecutorEventType `json:"type"`

	// TaskID is the ID of the task this event relates to
	TaskID string `json:"task_id"`

	// InstanceID is the execution context identifier
	InstanceID string `json:"instance_id"`

	// Message provides human-readable details about the event
	Message string `json:"message,omitempty"`

	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Error contains error details for failure events
	Error string `json:"error,omitempty"`

	// Progress is a 0-100 value for progress events
	Progress int `json:"progress,omitempty"`

	// Result contains completion details for complete/failed events
	Result *CompletionResult `json:"result,omitempty"`

	// RetryCount indicates which retry attempt this is (for retry events)
	RetryCount int `json:"retry_count,omitempty"`
}

// NewTaskStartedEvent creates a new task started event
func NewTaskStartedEvent(taskID, instanceID string) ExecutorEvent {
	return ExecutorEvent{
		Type:       EventTaskStarted,
		TaskID:     taskID,
		InstanceID: instanceID,
		Message:    "Task execution started",
		Timestamp:  time.Now(),
	}
}

// NewTaskCompleteEvent creates a new task completion event
func NewTaskCompleteEvent(taskID, instanceID string, result CompletionResult) ExecutorEvent {
	return ExecutorEvent{
		Type:       EventTaskComplete,
		TaskID:     taskID,
		InstanceID: instanceID,
		Message:    "Task completed successfully",
		Timestamp:  time.Now(),
		Result:     &result,
	}
}

// NewTaskFailedEvent creates a new task failure event
func NewTaskFailedEvent(taskID, instanceID string, err error, result *CompletionResult) ExecutorEvent {
	event := ExecutorEvent{
		Type:       EventTaskFailed,
		TaskID:     taskID,
		InstanceID: instanceID,
		Message:    "Task execution failed",
		Timestamp:  time.Now(),
		Result:     result,
	}
	if err != nil {
		event.Error = err.Error()
	}
	return event
}

// NewTaskCancelledEvent creates a new task cancellation event
func NewTaskCancelledEvent(taskID, instanceID string) ExecutorEvent {
	return ExecutorEvent{
		Type:       EventTaskCancelled,
		TaskID:     taskID,
		InstanceID: instanceID,
		Message:    "Task execution cancelled",
		Timestamp:  time.Now(),
	}
}

// NewTaskRetryEvent creates a new task retry event
func NewTaskRetryEvent(taskID, instanceID string, retryCount int, lastError string) ExecutorEvent {
	return ExecutorEvent{
		Type:       EventTaskRetry,
		TaskID:     taskID,
		InstanceID: instanceID,
		Message:    "Retrying failed task",
		Timestamp:  time.Now(),
		Error:      lastError,
		RetryCount: retryCount,
	}
}

// EventCallback is a function type for receiving executor events
type EventCallback func(event ExecutorEvent)

// EventEmitter is an optional interface for executors that support event callbacks
type EventEmitter interface {
	// SetEventCallback sets the callback function for receiving events
	SetEventCallback(callback EventCallback)

	// Events returns a channel for receiving events (alternative to callbacks)
	Events() <-chan ExecutorEvent
}
