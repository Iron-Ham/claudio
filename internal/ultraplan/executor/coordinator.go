// Package executor defines interfaces and implementations for coordinating Ultra-Plan task execution.
package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// CoordinatorConfig holds configuration for the execution coordinator.
type CoordinatorConfig struct {
	// MaxParallel is the maximum number of tasks to execute concurrently.
	MaxParallel int

	// RequireVerifiedCommits specifies whether tasks must produce commits to be marked successful.
	RequireVerifiedCommits bool

	// MaxTaskRetries is the maximum number of retry attempts for failed tasks.
	MaxTaskRetries int
}

// DefaultCoordinatorConfig returns sensible defaults for coordinator configuration.
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		MaxParallel:            3,
		RequireVerifiedCommits: true,
		MaxTaskRetries:         3,
	}
}

// TaskExecutorFactory creates TaskExecutor instances for individual tasks.
// This allows the coordinator to remain decoupled from the specific executor implementation.
type TaskExecutorFactory interface {
	// Create creates a new TaskExecutor for the given task.
	// The worktreePath is the git worktree where the task will execute.
	Create(task *ultraplan.PlannedTask, worktreePath string) (TaskExecutor, error)
}

// EventHandler handles events emitted during execution.
type EventHandler interface {
	// OnTaskStarted is called when a task begins execution.
	OnTaskStarted(taskID, instanceID string)

	// OnTaskCompleted is called when a task completes successfully.
	OnTaskCompleted(taskID string, commitCount int)

	// OnTaskFailed is called when a task fails.
	OnTaskFailed(taskID string, err error)

	// OnGroupCompleted is called when all tasks in an execution group complete.
	OnGroupCompleted(groupIndex int, succeeded, failed []string)

	// OnProgress is called periodically with execution progress updates.
	OnProgress(progress ExecutionProgress)
}

// Coordinator implements the ExecutionCoordinator interface.
// It manages the parallel execution of Ultra-Plan tasks while respecting
// dependency ordering and concurrency limits.
type Coordinator struct {
	config         CoordinatorConfig
	plan           *ultraplan.PlanSpec
	factory        TaskExecutorFactory
	eventHandler   EventHandler
	worktreeGetter WorktreeGetter

	mu             sync.RWMutex
	progress       ExecutionProgress
	runningTasks   map[string]*runningTask
	taskExecutors  map[string]TaskExecutor
	currentGroup   int
	started        bool
	stopped        bool
	stopChan       chan struct{}
	wg             sync.WaitGroup
}

// WorktreeGetter provides worktree paths for tasks.
type WorktreeGetter interface {
	// GetWorktree returns the worktree path for a task.
	// Creates the worktree if it doesn't exist.
	GetWorktree(taskID string) (string, error)
}

// runningTask tracks a currently executing task.
type runningTask struct {
	taskID     string
	executor   TaskExecutor
	startTime  time.Time
	cancelFunc context.CancelFunc
}

// NewCoordinator creates a new execution coordinator.
func NewCoordinator(
	config CoordinatorConfig,
	factory TaskExecutorFactory,
	worktreeGetter WorktreeGetter,
	eventHandler EventHandler,
) *Coordinator {
	return &Coordinator{
		config:         config,
		factory:        factory,
		worktreeGetter: worktreeGetter,
		eventHandler:   eventHandler,
		runningTasks:   make(map[string]*runningTask),
		taskExecutors:  make(map[string]TaskExecutor),
		stopChan:       make(chan struct{}),
	}
}

// Start begins execution of the given plan.
// Returns immediately after initiating execution.
func (c *Coordinator) Start(plan *ultraplan.PlanSpec) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return fmt.Errorf("coordinator already started")
	}

	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	// Validate the plan
	if err := ultraplan.ValidatePlanSimple(plan); err != nil {
		return fmt.Errorf("invalid plan: %w", err)
	}

	c.plan = plan
	c.progress = NewExecutionProgress(plan)
	c.started = true

	// Start the execution loop
	c.wg.Add(1)
	go c.executionLoop()

	return nil
}

// Stop halts execution of all currently running tasks.
func (c *Coordinator) Stop() error {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return nil
	}
	c.stopped = true

	// Close stop channel to signal shutdown
	close(c.stopChan)

	// Cancel all running tasks
	for _, rt := range c.runningTasks {
		if rt.cancelFunc != nil {
			rt.cancelFunc()
		}
	}
	c.mu.Unlock()

	// Wait for all goroutines to finish
	c.wg.Wait()
	return nil
}

// GetProgress returns the current execution progress.
func (c *Coordinator) GetProgress() ExecutionProgress {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.progress
}

// Wait blocks until all tasks complete or an error occurs.
func (c *Coordinator) Wait() error {
	c.wg.Wait()

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.progress.FailedTasks > 0 {
		return fmt.Errorf("%d tasks failed", c.progress.FailedTasks)
	}
	return nil
}

// executionLoop is the main execution loop that schedules and monitors tasks.
func (c *Coordinator) executionLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			// Mark remaining tasks as cancelled
			c.mu.Lock()
			for taskID, status := range c.progress.TaskStatuses {
				if status.Status == StatusPending {
					status.Status = StatusCancelled
					c.progress.TaskStatuses[taskID] = status
					c.progress.CancelledTasks++
					c.progress.PendingTasks--
				}
			}
			c.progress.EndTime = time.Now()
			c.mu.Unlock()
			return

		default:
			// Check if execution is complete
			c.mu.RLock()
			isComplete := c.progress.IsComplete()
			c.mu.RUnlock()

			if isComplete {
				c.mu.Lock()
				c.progress.EndTime = time.Now()
				c.mu.Unlock()
				return
			}

			// Try to start new tasks
			c.scheduleReadyTasks()

			// Check for group completion
			c.checkGroupCompletion()

			// Brief sleep to prevent busy-waiting
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// scheduleReadyTasks starts execution of tasks that are ready to run.
func (c *Coordinator) scheduleReadyTasks() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.plan == nil || len(c.plan.ExecutionOrder) == 0 {
		return
	}

	// Check if current group is valid
	if c.currentGroup >= len(c.plan.ExecutionOrder) {
		return
	}

	// Count currently running tasks
	runningCount := len(c.runningTasks)

	// Get tasks from the current group
	currentGroupTasks := c.plan.ExecutionOrder[c.currentGroup]

	for _, taskID := range currentGroupTasks {
		// Check concurrency limit
		if runningCount >= c.config.MaxParallel {
			break
		}

		// Skip if already started, completed, or failed
		status, exists := c.progress.TaskStatuses[taskID]
		if !exists || status.Status != StatusPending {
			continue
		}

		// Check if dependencies are satisfied
		if !c.areDependenciesSatisfied(taskID) {
			continue
		}

		// Start the task
		c.startTask(taskID)
		runningCount++
	}
}

// areDependenciesSatisfied checks if all dependencies for a task have completed.
// Must be called with mu held.
func (c *Coordinator) areDependenciesSatisfied(taskID string) bool {
	task := c.plan.GetTask(taskID)
	if task == nil {
		return false
	}

	for _, depID := range task.DependsOn {
		status, exists := c.progress.TaskStatuses[depID]
		if !exists || status.Status != StatusCompleted {
			return false
		}
	}
	return true
}

// startTask initiates execution of a task.
// Must be called with mu held.
func (c *Coordinator) startTask(taskID string) {
	task := c.plan.GetTask(taskID)
	if task == nil {
		return
	}

	// Get worktree for the task
	worktreePath, err := c.worktreeGetter.GetWorktree(taskID)
	if err != nil {
		c.markTaskFailed(taskID, err)
		return
	}

	// Create executor for the task
	executor, err := c.factory.Create(task, worktreePath)
	if err != nil {
		c.markTaskFailed(taskID, err)
		return
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Update status
	status := c.progress.TaskStatuses[taskID]
	status.Status = StatusRunning
	status.StartTime = time.Now()
	status.WorktreePath = worktreePath
	c.progress.TaskStatuses[taskID] = status
	c.progress.RunningTasks++
	c.progress.PendingTasks--

	// Track running task
	rt := &runningTask{
		taskID:     taskID,
		executor:   executor,
		startTime:  time.Now(),
		cancelFunc: cancel,
	}
	c.runningTasks[taskID] = rt
	c.taskExecutors[taskID] = executor

	// Notify event handler
	if c.eventHandler != nil {
		c.eventHandler.OnTaskStarted(taskID, status.InstanceID)
	}

	// Start execution in a goroutine
	c.wg.Add(1)
	go c.executeTask(ctx, taskID, executor)
}

// executeTask runs a task and handles its completion.
func (c *Coordinator) executeTask(ctx context.Context, taskID string, executor TaskExecutor) {
	defer c.wg.Done()

	// Get the task
	task := c.plan.GetTask(taskID)
	if task == nil {
		c.mu.Lock()
		c.markTaskFailed(taskID, fmt.Errorf("task not found"))
		c.mu.Unlock()
		return
	}

	// Execute the task
	err := executor.Execute(ctx, task)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove from running tasks
	delete(c.runningTasks, taskID)

	if err != nil {
		c.markTaskFailed(taskID, err)
	} else {
		c.markTaskCompleted(taskID)
	}

	// Notify progress
	if c.eventHandler != nil {
		c.eventHandler.OnProgress(c.progress)
	}
}

// markTaskCompleted marks a task as successfully completed.
// Must be called with mu held.
func (c *Coordinator) markTaskCompleted(taskID string) {
	status := c.progress.TaskStatuses[taskID]
	status.Status = StatusCompleted
	status.EndTime = time.Now()
	c.progress.TaskStatuses[taskID] = status
	c.progress.CompletedTasks++
	c.progress.RunningTasks--

	if c.eventHandler != nil {
		c.eventHandler.OnTaskCompleted(taskID, 0) // commitCount would come from actual execution
	}
}

// markTaskFailed marks a task as failed.
// Must be called with mu held.
func (c *Coordinator) markTaskFailed(taskID string, err error) {
	status := c.progress.TaskStatuses[taskID]
	status.Status = StatusFailed
	status.EndTime = time.Now()
	status.Error = err.Error()
	c.progress.TaskStatuses[taskID] = status
	c.progress.FailedTasks++

	// Decrement running if it was running, otherwise pending
	if _, running := c.runningTasks[taskID]; running {
		c.progress.RunningTasks--
	} else {
		c.progress.PendingTasks--
	}

	if c.eventHandler != nil {
		c.eventHandler.OnTaskFailed(taskID, err)
	}
}

// checkGroupCompletion checks if the current group is complete and advances to the next.
func (c *Coordinator) checkGroupCompletion() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.plan == nil || c.currentGroup >= len(c.plan.ExecutionOrder) {
		return
	}

	currentGroupTasks := c.plan.ExecutionOrder[c.currentGroup]

	// Check if all tasks in the current group are done
	var succeeded, failed []string
	allDone := true
	for _, taskID := range currentGroupTasks {
		status := c.progress.TaskStatuses[taskID]
		switch status.Status {
		case StatusCompleted:
			succeeded = append(succeeded, taskID)
		case StatusFailed, StatusCancelled:
			failed = append(failed, taskID)
		default:
			allDone = false
		}
	}

	if allDone {
		// Notify of group completion
		if c.eventHandler != nil {
			c.eventHandler.OnGroupCompleted(c.currentGroup, succeeded, failed)
		}

		// Advance to next group
		c.currentGroup++
	}
}

// GetCurrentGroup returns the current execution group index.
func (c *Coordinator) GetCurrentGroup() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentGroup
}

// GetRunningTaskCount returns the number of currently running tasks.
func (c *Coordinator) GetRunningTaskCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.runningTasks)
}

// Verify interface implementation at compile time.
var _ ExecutionCoordinator = (*Coordinator)(nil)
