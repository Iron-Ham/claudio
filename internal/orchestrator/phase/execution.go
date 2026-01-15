// Package phase provides abstractions for ultra-plan phase execution.
package phase

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// TaskCompletionFileName is the sentinel file that tasks write to signal completion.
const TaskCompletionFileName = ".claudio-task-complete.json"

// TaskCompletion represents a task completion notification.
// It is sent by monitoring goroutines when a task finishes execution.
type TaskCompletion struct {
	TaskID      string // ID of the completed task
	InstanceID  string // ID of the instance that ran the task
	Success     bool   // Whether the task completed successfully
	Error       string // Error message if task failed
	NeedsRetry  bool   // Indicates task should be retried (no commits produced)
	CommitCount int    // Number of commits produced by this task
}

// ExecutionState tracks the current state of execution phase.
type ExecutionState struct {
	// RunningTasks maps task IDs to their instance IDs.
	RunningTasks map[string]string

	// RunningCount is the number of currently running tasks.
	RunningCount int

	// CompletedCount is the number of completed tasks.
	CompletedCount int

	// FailedCount is the number of failed tasks.
	FailedCount int

	// TotalTasks is the total number of tasks to execute.
	TotalTasks int
}

// PlannedTaskData provides access to task information needed for prompt building.
// This interface abstracts the PlannedTask struct from the orchestrator package.
type PlannedTaskData interface {
	GetID() string
	GetTitle() string
	GetDescription() string
	GetFiles() []string
	IsNoCode() bool
}

// GroupConsolidationContextData provides access to context from previous group consolidation.
type GroupConsolidationContextData interface {
	GetNotes() string
	GetIssuesForNextGroup() []string
	IsVerificationSuccess() bool
}

// ExecutionSessionInterface extends UltraPlanSessionInterface with execution-specific methods.
type ExecutionSessionInterface interface {
	UltraPlanSessionInterface

	// GetCurrentGroup returns the current execution group index.
	GetCurrentGroup() int

	// GetCompletedTaskCount returns the number of completed tasks.
	GetCompletedTaskCount() int

	// GetFailedTaskCount returns the number of failed tasks.
	GetFailedTaskCount() int

	// GetTotalTaskCount returns the total number of tasks.
	GetTotalTaskCount() int

	// GetMaxParallel returns the maximum parallel task limit.
	GetMaxParallel() int

	// IsMultiPass returns true if multi-pass mode is enabled.
	IsMultiPass() bool

	// GetPlanSummary returns the plan summary string.
	GetPlanSummary() string

	// GetGroupConsolidationContext returns the context from a previous group's consolidation.
	GetGroupConsolidationContext(groupIndex int) GroupConsolidationContextData
}

// ExecutionOrchestratorInterface extends OrchestratorInterface with execution-specific methods.
type ExecutionOrchestratorInterface interface {
	OrchestratorInterface

	// AddInstanceFromBranch creates a new instance from a specific base branch.
	AddInstanceFromBranch(session any, task string, baseBranch string) (any, error)

	// StopInstance stops a running instance.
	StopInstance(inst any) error

	// GetInstance returns an instance by ID.
	GetInstance(id string) any
}

// ExecutionCallbacksInterface extends CoordinatorCallbacksInterface with execution-specific callbacks.
type ExecutionCallbacksInterface interface {
	CoordinatorCallbacksInterface
}

// ExecutionCoordinatorInterface defines methods the ExecutionOrchestrator needs from the Coordinator.
// This allows the orchestrator to interact with coordinator-level state and operations.
type ExecutionCoordinatorInterface interface {
	// GetBaseBranchForGroup returns the base branch for a given group index.
	GetBaseBranchForGroup(groupIndex int) string

	// AddRunningTask registers a task as running with the given instance ID.
	AddRunningTask(taskID, instanceID string)

	// RemoveRunningTask unregisters a task from the running state.
	RemoveRunningTask(taskID string) bool

	// GetRunningTaskCount returns the number of currently running tasks.
	GetRunningTaskCount() int

	// IsTaskRunning returns true if the given task is currently running.
	IsTaskRunning(taskID string) bool

	// GetBaseSession returns the base session for instance group management.
	GetBaseSession() any

	// GetTaskGroupIndex returns the group index for a given task ID.
	GetTaskGroupIndex(taskID string) int

	// VerifyTaskWork checks if a task produced actual commits and determines success/retry.
	VerifyTaskWork(taskID string, inst any) TaskCompletion

	// CheckForTaskCompletionFile checks if the task has written its completion sentinel file.
	CheckForTaskCompletionFile(inst any) bool

	// HandleTaskCompletion processes a task completion notification.
	HandleTaskCompletion(completion TaskCompletion)

	// PollTaskCompletions checks for task completions that monitoring goroutines may have missed.
	PollTaskCompletions(completionChan chan<- TaskCompletion)

	// NotifyTaskStart notifies callbacks that a task has started.
	NotifyTaskStart(taskID, instanceID string)

	// NotifyTaskFailed notifies callbacks that a task has failed.
	NotifyTaskFailed(taskID, reason string)

	// NotifyProgress notifies callbacks of progress updates.
	NotifyProgress()

	// FinishExecution performs cleanup after execution completes.
	FinishExecution()

	// AddInstanceToGroup adds an instance to the appropriate ultra-plan group.
	AddInstanceToGroup(instanceID string, isMultiPass bool)
}

// ExecutionContext holds the extended dependencies required by the ExecutionOrchestrator.
// It embeds PhaseContext and adds execution-specific dependencies.
type ExecutionContext struct {
	*PhaseContext

	// Coordinator provides access to coordinator-level operations.
	// May be nil if running in standalone mode (limited functionality).
	Coordinator ExecutionCoordinatorInterface

	// Session provides execution-specific session access (extended interface).
	ExecutionSession ExecutionSessionInterface

	// Orchestrator provides execution-specific orchestrator access (extended interface).
	ExecutionOrchestrator ExecutionOrchestratorInterface
}

// ExecutionOrchestrator manages the execution phase of ultra-plan execution.
// It is responsible for:
//   - Running the main execution loop with context cancellation
//   - Spawning task instances up to the MaxParallel limit
//   - Monitoring task completion via channels
//   - Handling completion notifications (success, failure, retry)
//   - Advancing through execution groups as they complete
//
// ExecutionOrchestrator implements the PhaseExecutor interface.
type ExecutionOrchestrator struct {
	// phaseCtx holds the shared dependencies for phase execution.
	phaseCtx *PhaseContext

	// execCtx holds the extended execution-specific context.
	// May be nil if only PhaseContext was provided.
	execCtx *ExecutionContext

	// logger is a convenience reference to phaseCtx.Logger for structured logging.
	logger *logging.Logger

	// state holds the current execution state.
	// Access must be protected by mu.
	state ExecutionState

	// ctx is the execution context, used for cancellation propagation.
	ctx context.Context

	// cancel is the cancel function for ctx.
	cancel context.CancelFunc

	// mu protects concurrent access to mutable state.
	mu sync.RWMutex

	// wg tracks background goroutines spawned by this orchestrator.
	wg sync.WaitGroup

	// cancelled indicates whether Cancel() has been called.
	cancelled bool

	// completionChan is used to receive task completion notifications.
	completionChan chan TaskCompletion
}

// NewExecutionOrchestrator creates a new ExecutionOrchestrator with the provided dependencies.
// The phaseCtx must be valid (non-nil Manager, Orchestrator, and Session).
// Returns an error if phaseCtx validation fails.
//
// For full functionality, use NewExecutionOrchestratorWithContext which accepts an ExecutionContext.
func NewExecutionOrchestrator(phaseCtx *PhaseContext) (*ExecutionOrchestrator, error) {
	if err := phaseCtx.Validate(); err != nil {
		return nil, err
	}

	return &ExecutionOrchestrator{
		phaseCtx: phaseCtx,
		logger:   phaseCtx.GetLogger().WithPhase("execution-orchestrator"),
		state: ExecutionState{
			RunningTasks: make(map[string]string),
		},
		completionChan: make(chan TaskCompletion, 100),
	}, nil
}

// NewExecutionOrchestratorWithContext creates a new ExecutionOrchestrator with full execution context.
// This provides access to extended interfaces needed for full execution loop functionality.
func NewExecutionOrchestratorWithContext(execCtx *ExecutionContext) (*ExecutionOrchestrator, error) {
	if execCtx == nil {
		return nil, fmt.Errorf("execution context is required")
	}
	if execCtx.PhaseContext == nil {
		return nil, fmt.Errorf("phase context is required")
	}
	if err := execCtx.Validate(); err != nil {
		return nil, err
	}

	return &ExecutionOrchestrator{
		phaseCtx: execCtx.PhaseContext,
		execCtx:  execCtx,
		logger:   execCtx.PhaseContext.GetLogger().WithPhase("execution-orchestrator"),
		state: ExecutionState{
			RunningTasks: make(map[string]string),
		},
		completionChan: make(chan TaskCompletion, 100),
	}, nil
}

// Phase returns the UltraPlanPhase that this orchestrator handles.
// For ExecutionOrchestrator, this is always PhaseExecuting.
func (e *ExecutionOrchestrator) Phase() UltraPlanPhase {
	return PhaseExecuting
}

// Execute runs the execution phase logic.
// It manages the parallel execution of tasks, respecting MaxParallel limits,
// monitoring task completion, and advancing through execution groups.
//
// Execute respects the provided context for cancellation. If ctx.Done() is
// signaled or Cancel() is called, Execute returns early.
//
// Returns an error if execution fails or is cancelled.
func (e *ExecutionOrchestrator) Execute(ctx context.Context) error {
	// Create a cancellable context derived from the provided context
	e.mu.Lock()
	if e.cancelled {
		e.mu.Unlock()
		return ErrExecutionCancelled
	}
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.mu.Unlock()

	e.logger.Info("execution phase starting")

	// Set the session phase to executing
	e.phaseCtx.Manager.SetPhase(PhaseExecuting)

	// Notify callbacks of phase change
	if e.phaseCtx.Callbacks != nil {
		e.phaseCtx.Callbacks.OnPhaseChange(PhaseExecuting)
	}

	// Run the main execution loop
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.executionLoop()
	}()

	// Wait for the execution loop to complete
	e.wg.Wait()

	e.logger.Info("execution phase completed")

	return nil
}

// executionLoop manages the parallel execution of tasks.
// This is the main loop that polls for ready tasks, spawns instances,
// and handles completion notifications.
func (e *ExecutionOrchestrator) executionLoop() {
	session := e.phaseCtx.Session

	// Initialize state from session
	e.mu.Lock()
	e.state.TotalTasks = len(session.GetReadyTasks()) // Will be updated to actual total
	e.mu.Unlock()

	for {
		select {
		case <-e.ctx.Done():
			e.logger.Debug("execution loop cancelled via context")
			return

		case completion := <-e.completionChan:
			e.handleTaskCompletion(completion)
			e.notifyProgress()

		default:
			// Poll for task completions that monitoring goroutines may have missed
			if e.execCtx != nil && e.execCtx.Coordinator != nil {
				e.execCtx.Coordinator.PollTaskCompletions(e.completionChan)
			}

			// Check if we're done
			e.mu.RLock()
			completedCount := e.state.CompletedCount
			failedCount := e.state.FailedCount
			totalTasks := e.state.TotalTasks
			runningCount := e.state.RunningCount
			e.mu.RUnlock()

			// Update total tasks from session if we have extended access
			if e.execCtx != nil && e.execCtx.ExecutionSession != nil {
				e.mu.Lock()
				e.state.TotalTasks = e.execCtx.ExecutionSession.GetTotalTaskCount()
				totalTasks = e.state.TotalTasks
				e.mu.Unlock()
			}

			if completedCount+failedCount >= totalTasks && totalTasks > 0 {
				// All tasks done
				e.finishExecution()
				return
			}

			// Get MaxParallel configuration
			maxParallel := 3 // Default
			if e.execCtx != nil && e.execCtx.ExecutionSession != nil {
				maxParallel = e.execCtx.ExecutionSession.GetMaxParallel()
			}

			// Check if we can start more tasks (MaxParallel <= 0 means unlimited)
			if maxParallel <= 0 || runningCount < maxParallel {
				readyTasks := session.GetReadyTasks()
				for _, taskID := range readyTasks {
					e.mu.RLock()
					currentRunning := e.state.RunningCount
					e.mu.RUnlock()

					if maxParallel > 0 && currentRunning >= maxParallel {
						break
					}

					// Skip if already running
					if e.isTaskRunning(taskID) {
						continue
					}

					if err := e.startTask(taskID); err != nil {
						e.notifyTaskFailed(taskID, err.Error())
					}
				}
			}

			// Small sleep to avoid busy-waiting
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// startTask starts a single task as a new instance.
func (e *ExecutionOrchestrator) startTask(taskID string) error {
	session := e.phaseCtx.Session
	task := session.GetTask(taskID)
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Build the task prompt
	prompt := e.buildTaskPrompt(taskID, task)

	// Determine the base branch for this task
	baseBranch := ""
	currentGroup := 0
	if e.execCtx != nil && e.execCtx.ExecutionSession != nil {
		currentGroup = e.execCtx.ExecutionSession.GetCurrentGroup()
	}
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		baseBranch = e.execCtx.Coordinator.GetBaseBranchForGroup(currentGroup)
	}

	// Create a new instance for this task
	var inst any
	var err error
	if baseBranch != "" && e.execCtx != nil && e.execCtx.ExecutionOrchestrator != nil {
		// Use the consolidated branch from the previous group as the base
		inst, err = e.execCtx.ExecutionOrchestrator.AddInstanceFromBranch(nil, prompt, baseBranch)
	} else {
		// Use the default (HEAD/main)
		inst, err = e.phaseCtx.Orchestrator.AddInstance(nil, prompt)
	}
	if err != nil {
		e.logger.Error("task execution failed",
			"task_id", taskID,
			"error", err.Error(),
			"stage", "create_instance",
		)
		return fmt.Errorf("failed to create instance for task %s: %w", taskID, err)
	}

	// Get instance ID
	instanceID := e.getInstanceID(inst)

	// Add instance to the ultraplan group for sidebar display
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		isMultiPass := false
		if e.execCtx.ExecutionSession != nil {
			isMultiPass = e.execCtx.ExecutionSession.IsMultiPass()
		}
		e.execCtx.Coordinator.AddInstanceToGroup(instanceID, isMultiPass)
	}

	// Track the running task
	e.mu.Lock()
	e.state.RunningTasks[taskID] = instanceID
	e.state.RunningCount++
	e.mu.Unlock()

	// Also track in coordinator if available
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		e.execCtx.Coordinator.AddRunningTask(taskID, instanceID)
	}

	e.notifyTaskStart(taskID, instanceID)

	// Start the instance
	if err := e.phaseCtx.Orchestrator.StartInstance(inst); err != nil {
		e.mu.Lock()
		delete(e.state.RunningTasks, taskID)
		e.state.RunningCount--
		e.mu.Unlock()

		if e.execCtx != nil && e.execCtx.Coordinator != nil {
			e.execCtx.Coordinator.RemoveRunningTask(taskID)
		}

		e.logger.Error("task execution failed",
			"task_id", taskID,
			"error", err.Error(),
			"stage", "start_instance",
		)
		return fmt.Errorf("failed to start instance for task %s: %w", taskID, err)
	}

	// Monitor the instance for completion in a goroutine
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.monitorTaskInstance(taskID, instanceID)
	}()

	return nil
}

// buildTaskPrompt creates the prompt for a child task instance.
func (e *ExecutionOrchestrator) buildTaskPrompt(taskID string, task any) string {
	// Try to use the task as PlannedTaskData
	taskData, ok := task.(PlannedTaskData)
	if !ok {
		// Fallback to basic prompt if task doesn't implement interface
		return fmt.Sprintf("# Task: %s\n\nPlease complete this task.", taskID)
	}

	var sb strings.Builder

	// Get plan summary
	planSummary := "Ultra-Plan Task"
	if e.execCtx != nil && e.execCtx.ExecutionSession != nil {
		planSummary = e.execCtx.ExecutionSession.GetPlanSummary()
	}

	sb.WriteString(fmt.Sprintf("# Task: %s\n\n", taskData.GetTitle()))
	sb.WriteString(fmt.Sprintf("## Part of Ultra-Plan: %s\n\n", planSummary))
	sb.WriteString("## Your Task\n\n")
	sb.WriteString(taskData.GetDescription())
	sb.WriteString("\n\n")

	files := taskData.GetFiles()
	if len(files) > 0 {
		sb.WriteString("## Expected Files\n\n")
		sb.WriteString("You are expected to work with these files:\n")
		for _, f := range files {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}

	// Add context from previous group's consolidator if this task is not in group 0
	groupIndex := 0
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		groupIndex = e.execCtx.Coordinator.GetTaskGroupIndex(taskID)
	}
	if groupIndex > 0 && e.execCtx != nil && e.execCtx.ExecutionSession != nil {
		prevGroupIdx := groupIndex - 1
		prevContext := e.execCtx.ExecutionSession.GetGroupConsolidationContext(prevGroupIdx)
		if prevContext != nil {
			sb.WriteString("## Context from Previous Group\n\n")
			sb.WriteString(fmt.Sprintf("This task builds on work consolidated from Group %d.\n\n", prevGroupIdx+1))

			notes := prevContext.GetNotes()
			if notes != "" {
				sb.WriteString(fmt.Sprintf("**Consolidator Notes**: %s\n\n", notes))
			}

			issues := prevContext.GetIssuesForNextGroup()
			if len(issues) > 0 {
				sb.WriteString("**Important**: The previous group's consolidator flagged these issues:\n")
				for _, issue := range issues {
					sb.WriteString(fmt.Sprintf("- %s\n", issue))
				}
				sb.WriteString("\n")
			}

			if prevContext.IsVerificationSuccess() {
				sb.WriteString("The consolidated code from the previous group has been verified (build/lint/tests passed).\n\n")
			} else {
				sb.WriteString("**Warning**: The previous group's code verification may have issues. Check carefully.\n\n")
			}
		}
	}

	sb.WriteString("## Guidelines\n\n")
	sb.WriteString("- Focus only on this specific task\n")
	sb.WriteString("- Do not modify files outside of your assigned scope unless necessary\n")
	sb.WriteString("- Commit your changes before writing the completion file\n\n")

	// Add completion protocol instructions
	sb.WriteString("## Completion Protocol\n\n")
	sb.WriteString("When your task is complete, you MUST write a completion file to signal the orchestrator:\n\n")
	sb.WriteString(fmt.Sprintf("1. Use Write tool to create `%s` in your worktree root\n", TaskCompletionFileName))
	sb.WriteString("2. Include this JSON structure:\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString(fmt.Sprintf("  \"task_id\": \"%s\",\n", taskID))
	sb.WriteString("  \"status\": \"complete\",\n")
	sb.WriteString("  \"summary\": \"Brief description of what you accomplished\",\n")
	sb.WriteString("  \"files_modified\": [\"list\", \"of\", \"files\", \"you\", \"changed\"],\n")
	sb.WriteString("  \"notes\": \"Any implementation notes for the consolidation phase\",\n")
	sb.WriteString("  \"issues\": [\"Any concerns or blocking issues found\"],\n")
	sb.WriteString("  \"suggestions\": [\"Suggestions for integration with other tasks\"],\n")
	sb.WriteString("  \"dependencies\": [\"Any new runtime dependencies added\"]\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("3. Use status \"blocked\" if you cannot complete (explain in issues), or \"failed\" if something broke\n")
	sb.WriteString("4. This file signals that your work is done and provides context for consolidation\n")

	return sb.String()
}

// monitorTaskInstance monitors an instance and reports when it completes.
func (e *ExecutionOrchestrator) monitorTaskInstance(taskID, instanceID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return

		case <-ticker.C:
			// Check for completion using coordinator if available
			if e.execCtx != nil && e.execCtx.Coordinator != nil && e.execCtx.ExecutionOrchestrator != nil {
				inst := e.execCtx.ExecutionOrchestrator.GetInstance(instanceID)
				if inst == nil {
					e.logger.Debug("instance status check",
						"task_id", taskID,
						"instance_id", instanceID,
						"status", "not_found",
					)
					e.completionChan <- TaskCompletion{
						TaskID:     taskID,
						InstanceID: instanceID,
						Success:    false,
						Error:      "instance not found",
					}
					return
				}

				// Check for sentinel file
				if e.execCtx.Coordinator.CheckForTaskCompletionFile(inst) {
					// Stop the instance
					if e.execCtx.ExecutionOrchestrator != nil {
						_ = e.execCtx.ExecutionOrchestrator.StopInstance(inst)
					}

					// Verify work was done
					result := e.execCtx.Coordinator.VerifyTaskWork(taskID, inst)
					e.completionChan <- result
					return
				}

				// Check instance status for other completion conditions
				// This is handled by the coordinator's monitoring logic
			}
		}
	}
}

// handleTaskCompletion processes a task completion notification.
func (e *ExecutionOrchestrator) handleTaskCompletion(completion TaskCompletion) {
	e.mu.Lock()
	delete(e.state.RunningTasks, completion.TaskID)
	e.state.RunningCount--
	if completion.Success {
		e.state.CompletedCount++
	} else {
		e.state.FailedCount++
	}
	e.mu.Unlock()

	// Also update coordinator state if available
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		e.execCtx.Coordinator.RemoveRunningTask(completion.TaskID)
		e.execCtx.Coordinator.HandleTaskCompletion(completion)
	}

	// Update manager state
	if completion.Success {
		e.phaseCtx.Manager.MarkTaskComplete(completion.TaskID)
		if e.phaseCtx.Callbacks != nil {
			e.phaseCtx.Callbacks.OnTaskComplete(completion.TaskID)
		}
	} else if !completion.NeedsRetry {
		e.phaseCtx.Manager.MarkTaskFailed(completion.TaskID, completion.Error)
		if e.phaseCtx.Callbacks != nil {
			e.phaseCtx.Callbacks.OnTaskFailed(completion.TaskID, completion.Error)
		}
	}
}

// finishExecution performs cleanup after execution completes.
func (e *ExecutionOrchestrator) finishExecution() {
	e.logger.Info("execution phase finishing",
		"completed", e.state.CompletedCount,
		"failed", e.state.FailedCount,
		"total", e.state.TotalTasks,
	)

	// Delegate to coordinator if available
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		e.execCtx.Coordinator.FinishExecution()
	}
}

// notifyTaskStart notifies callbacks that a task has started.
func (e *ExecutionOrchestrator) notifyTaskStart(taskID, instanceID string) {
	e.logger.Debug("task started",
		"task_id", taskID,
		"instance_id", instanceID,
	)

	// Assign task to instance in manager
	e.phaseCtx.Manager.AssignTaskToInstance(taskID, instanceID)

	// Notify via coordinator if available
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		e.execCtx.Coordinator.NotifyTaskStart(taskID, instanceID)
	} else if e.phaseCtx.Callbacks != nil {
		e.phaseCtx.Callbacks.OnTaskStart(taskID, instanceID)
	}
}

// notifyTaskFailed notifies callbacks that a task has failed.
func (e *ExecutionOrchestrator) notifyTaskFailed(taskID, reason string) {
	e.logger.Error("task failed",
		"task_id", taskID,
		"reason", reason,
	)

	e.mu.Lock()
	e.state.FailedCount++
	e.mu.Unlock()

	// Update manager state
	e.phaseCtx.Manager.MarkTaskFailed(taskID, reason)

	// Notify via coordinator if available
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		e.execCtx.Coordinator.NotifyTaskFailed(taskID, reason)
	} else if e.phaseCtx.Callbacks != nil {
		e.phaseCtx.Callbacks.OnTaskFailed(taskID, reason)
	}
}

// notifyProgress notifies callbacks of progress updates.
func (e *ExecutionOrchestrator) notifyProgress() {
	e.mu.RLock()
	completed := e.state.CompletedCount
	total := e.state.TotalTasks
	e.mu.RUnlock()

	// Notify via coordinator if available
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		e.execCtx.Coordinator.NotifyProgress()
	} else if e.phaseCtx.Callbacks != nil {
		e.phaseCtx.Callbacks.OnProgress(completed, total, PhaseExecuting)
	}
}

// Cancel signals the orchestrator to stop any in-progress work.
// This is used for immediate cancellation requests (e.g., user abort).
// After Cancel is called, Execute should return promptly.
// Cancel is safe to call multiple times (idempotent).
func (e *ExecutionOrchestrator) Cancel() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cancelled {
		return
	}
	e.cancelled = true

	if e.cancel != nil {
		e.cancel()
	}

	e.logger.Info("execution phase cancelled")
}

// State returns a copy of the current execution state.
// This is safe for concurrent access.
func (e *ExecutionOrchestrator) State() ExecutionState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stateCopy := ExecutionState{
		RunningCount:   e.state.RunningCount,
		CompletedCount: e.state.CompletedCount,
		FailedCount:    e.state.FailedCount,
		TotalTasks:     e.state.TotalTasks,
	}

	if e.state.RunningTasks != nil {
		stateCopy.RunningTasks = make(map[string]string, len(e.state.RunningTasks))
		maps.Copy(stateCopy.RunningTasks, e.state.RunningTasks)
	}

	return stateCopy
}

// GetRunningCount returns the number of currently running tasks.
func (e *ExecutionOrchestrator) GetRunningCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.RunningCount
}

// GetCompletedCount returns the number of completed tasks.
func (e *ExecutionOrchestrator) GetCompletedCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.CompletedCount
}

// GetFailedCount returns the number of failed tasks.
func (e *ExecutionOrchestrator) GetFailedCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.FailedCount
}

// isTaskRunning returns true if the given task is currently running.
func (e *ExecutionOrchestrator) isTaskRunning(taskID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.state.RunningTasks[taskID]
	return exists
}

// getInstanceID extracts the instance ID from an instance.
// The instance type varies depending on whether we're using the extended interface.
func (e *ExecutionOrchestrator) getInstanceID(inst any) string {
	// Try common interface patterns
	if getter, ok := inst.(interface{ GetID() string }); ok {
		return getter.GetID()
	}
	if getter, ok := inst.(interface{ ID() string }); ok {
		return getter.ID()
	}
	// Fallback
	return fmt.Sprintf("%v", inst)
}

// Reset clears the orchestrator state for a fresh execution.
// This is useful when restarting the execution phase.
func (e *ExecutionOrchestrator) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state = ExecutionState{
		RunningTasks: make(map[string]string),
	}
	e.cancelled = false
	e.cancel = nil
	e.ctx = nil

	// Drain completion channel
	for len(e.completionChan) > 0 {
		<-e.completionChan
	}
}

// ErrExecutionCancelled is returned when the orchestrator is cancelled.
var ErrExecutionCancelled = fmt.Errorf("execution phase cancelled")
