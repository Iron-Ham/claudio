// Package phase provides abstractions for ultra-plan phase execution.
package phase

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
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

	// ProcessedTasks tracks tasks that have been processed (completed or failed).
	// This is used for duplicate detection when both monitor goroutine and poll
	// detect the same completion.
	ProcessedTasks map[string]bool

	// GroupDecision holds state about a partial group failure awaiting user decision.
	// May be nil if no partial failure is pending.
	GroupDecision *GroupDecisionState
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

	// GetInstanceByID returns an instance by ID with any return type.
	// This is separate from OrchestratorInterface.GetInstance which returns InstanceInterface.
	GetInstanceByID(id string) any
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

	// StartGroupConsolidation starts the group consolidation process.
	// Returns an error if consolidation fails.
	StartGroupConsolidation(groupIndex int) error

	// HandlePartialGroupFailure handles a group with mixed success/failure.
	// This typically pauses execution and awaits user decision.
	HandlePartialGroupFailure(groupIndex int)

	// ClearTaskFromInstance removes the task-to-instance mapping for retry.
	ClearTaskFromInstance(taskID string)

	// SaveSession persists the current session state.
	SaveSession() error

	// RunSynthesis starts the synthesis phase.
	RunSynthesis() error

	// NotifyComplete notifies callbacks of overall completion.
	NotifyComplete(success bool, summary string)

	// SetSessionPhase sets the session phase.
	SetSessionPhase(phase UltraPlanPhase)

	// SetSessionError sets the session error message.
	SetSessionError(err string)

	// GetNoSynthesis returns true if synthesis phase should be skipped.
	GetNoSynthesis() bool

	// RecordTaskCommitCount records the commit count for a completed task.
	RecordTaskCommitCount(taskID string, count int)

	// ConsolidateGroupWithVerification consolidates a group and verifies commits exist.
	// Used by ResumeWithPartialWork to consolidate only successful tasks.
	ConsolidateGroupWithVerification(groupIndex int) error

	// EmitEvent emits a coordinator event for UI notification.
	EmitEvent(eventType, message string)

	// StartExecutionLoop restarts the execution loop (used by RetriggerGroup).
	StartExecutionLoop()
}

// RetryManagerInterface defines the methods needed for retry state management.
// This interface abstracts the retry.Manager to avoid direct package dependencies.
type RetryManagerInterface interface {
	// Reset clears the retry state for a task.
	Reset(taskID string)

	// GetAllStates returns a copy of all task retry states.
	GetAllStates() map[string]*RetryTaskState
}

// RetryTaskState tracks retry attempts for a task.
// This mirrors the retry.TaskState struct.
type RetryTaskState struct {
	TaskID       string `json:"task_id"`
	RetryCount   int    `json:"retry_count"`
	MaxRetries   int    `json:"max_retries"`
	LastError    string `json:"last_error,omitempty"`
	CommitCounts []int  `json:"commit_counts,omitempty"`
	Succeeded    bool   `json:"succeeded,omitempty"`
}

// RetryRecoverySessionInterface defines session methods needed for retry/recovery operations.
// This interface abstracts the UltraPlanSession to allow independent testing.
type RetryRecoverySessionInterface interface {
	// GetGroupDecision returns the current group decision state.
	GetGroupDecision() *GroupDecisionState

	// SetGroupDecision sets the group decision state.
	SetGroupDecision(decision *GroupDecisionState)

	// GetCurrentGroup returns the current execution group index.
	GetCurrentGroup() int

	// SetCurrentGroup sets the current execution group index.
	SetCurrentGroup(group int)

	// GetCompletedTasks returns the list of completed task IDs.
	GetCompletedTasks() []string

	// SetCompletedTasks sets the completed tasks list.
	SetCompletedTasks(tasks []string)

	// GetFailedTasks returns the list of failed task IDs.
	GetFailedTasks() []string

	// SetFailedTasks sets the failed tasks list.
	SetFailedTasks(tasks []string)

	// GetTaskToInstance returns the task-to-instance mapping.
	GetTaskToInstance() map[string]string

	// DeleteTaskFromInstance removes a task from the task-to-instance mapping.
	DeleteTaskFromInstance(taskID string)

	// GetTaskCommitCounts returns the task commit counts map.
	GetTaskCommitCounts() map[string]int

	// DeleteTaskCommitCount removes a task from the commit counts map.
	DeleteTaskCommitCount(taskID string)

	// GetExecutionOrder returns the execution order (groups of task IDs).
	GetExecutionOrder() [][]string

	// GetGroupConsolidatedBranches returns the consolidated branch names per group.
	GetGroupConsolidatedBranches() []string

	// SetGroupConsolidatedBranches sets the consolidated branch names.
	SetGroupConsolidatedBranches(branches []string)

	// GetGroupConsolidatorIDs returns the consolidator instance IDs per group.
	GetGroupConsolidatorIDs() []string

	// SetGroupConsolidatorIDs sets the consolidator instance IDs.
	SetGroupConsolidatorIDs(ids []string)

	// GetGroupConsolidationContexts returns the consolidation contexts per group.
	GetGroupConsolidationContexts() []GroupConsolidationContextData

	// SetGroupConsolidationContextsLength truncates the contexts slice to the given length.
	SetGroupConsolidationContextsLength(length int)

	// SetTaskRetries sets the task retry states map.
	SetTaskRetries(retries map[string]*RetryTaskState)

	// ClearSynthesisState clears all synthesis-related state.
	ClearSynthesisState()

	// ClearRevisionState clears all revision-related state.
	ClearRevisionState()

	// ClearConsolidationState clears all consolidation-related state.
	ClearConsolidationState()

	// ClearPRUrls clears the PR URLs.
	ClearPRUrls()

	// SetError sets the session error message.
	SetError(err string)
}

// GroupTrackerInterface defines the methods needed for group tracking.
// This interface abstracts the group.Tracker to avoid direct package dependencies.
type GroupTrackerInterface interface {
	// GetTaskGroupIndex returns the execution group index for a task.
	GetTaskGroupIndex(taskID string) int

	// IsGroupComplete returns true if all tasks in the group are done.
	IsGroupComplete(groupIndex int) bool

	// HasPartialFailure returns true if the group has mixed success/failure.
	HasPartialFailure(groupIndex int) bool

	// AdvanceGroup computes the next group index.
	AdvanceGroup(groupIndex int) (nextGroup int, done bool)

	// GetGroupTasks returns the tasks in a group.
	GetGroupTasks(groupIndex int) []GroupTaskInfo

	// TotalGroups returns the total number of groups.
	TotalGroups() int

	// HasMoreGroups returns true if there are more groups after the index.
	HasMoreGroups(groupIndex int) bool
}

// GroupTaskInfo provides minimal task information for group tracking.
type GroupTaskInfo struct {
	ID    string
	Title string
}

// GroupDecisionState holds state about a partial group failure awaiting decision.
type GroupDecisionState struct {
	GroupIndex       int
	SucceededTasks   []string
	FailedTasks      []string
	AwaitingDecision bool
}

// TaskVerifyOptions provides task-specific context for verification.
// This mirrors the verify.TaskVerifyOptions struct.
type TaskVerifyOptions struct {
	// NoCode indicates the task doesn't require code changes.
	// When true, the task succeeds even without commits.
	NoCode bool
}

// TaskVerifyResult represents the result of verifying a task's work.
// This mirrors the verify.TaskCompletionResult struct.
type TaskVerifyResult struct {
	TaskID      string
	InstanceID  string
	Success     bool
	Error       string
	NeedsRetry  bool
	CommitCount int
}

// TaskVerifierInterface defines the verification operations needed by ExecutionOrchestrator.
// This interface abstracts the verify.TaskVerifier to avoid direct package dependencies
// and enable testing with mocks.
type TaskVerifierInterface interface {
	// CheckCompletionFile checks if the task has written its completion sentinel file.
	// This checks for both regular task completion (.claudio-task-complete.json) and
	// revision task completion (.claudio-revision-complete.json).
	CheckCompletionFile(worktreePath string) (bool, error)

	// VerifyTaskWork checks if a task produced actual commits and determines success/retry.
	// The opts parameter provides task-specific context (e.g., NoCode flag for verification tasks).
	VerifyTaskWork(taskID, instanceID, worktreePath, baseBranch string, opts *TaskVerifyOptions) TaskVerifyResult
}

// InstanceManagerCheckerInterface provides methods for checking instance manager state.
// This allows the ExecutionOrchestrator to perform status-based fallback detection.
type InstanceManagerCheckerInterface interface {
	// TmuxSessionExists returns true if the tmux session for the instance is still running.
	TmuxSessionExists() bool
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

	// Verifier provides task verification operations.
	// If nil, verification will be delegated to the Coordinator.
	Verifier TaskVerifierInterface

	// GroupTracker provides group completion tracking operations.
	// If nil, group advancement will be delegated to the Coordinator.
	GroupTracker GroupTrackerInterface
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
			RunningTasks:   make(map[string]string),
			ProcessedTasks: make(map[string]bool),
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
			RunningTasks:   make(map[string]string),
			ProcessedTasks: make(map[string]bool),
		},
		completionChan: make(chan TaskCompletion, 100),
	}, nil
}

// Phase returns the UltraPlanPhase that this orchestrator handles.
// For ExecutionOrchestrator, this is always PhaseExecuting.
func (e *ExecutionOrchestrator) Phase() UltraPlanPhase {
	return PhaseExecuting
}

// ExecuteWithContext runs the execution phase with an extended execution context.
// This provides access to coordinator-level operations needed for full execution functionality.
// This is the preferred method for executing when the full Coordinator is available.
//
// The execCtx provides extended interfaces:
//   - ExecutionCoordinatorInterface: task verification, group consolidation, callbacks
//   - ExecutionSessionInterface: extended session access (optional)
//   - ExecutionOrchestratorInterface: extended orchestrator access (optional)
//
// Returns an error if execution fails or is cancelled.
func (e *ExecutionOrchestrator) ExecuteWithContext(ctx context.Context, execCtx *ExecutionContext) error {
	if execCtx == nil {
		return fmt.Errorf("execution context is required")
	}

	// Update the internal execution context
	e.mu.Lock()
	e.execCtx = execCtx
	e.mu.Unlock()

	// Delegate to the main Execute method
	return e.Execute(ctx)
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
			// This uses the local implementation which can work with or without a Coordinator
			e.pollTaskCompletions(e.completionChan)

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
// It delegates to prompt.TaskBuilder for the actual prompt generation,
// after converting the task data to the prompt package's types.
func (e *ExecutionOrchestrator) buildTaskPrompt(taskID string, task any) string {
	// Try to use the task as PlannedTaskData
	taskData, ok := task.(PlannedTaskData)
	if !ok {
		// Fallback to basic prompt if task doesn't implement interface
		e.logger.Warn("task does not implement PlannedTaskData interface, using basic prompt",
			"task_id", taskID,
			"task_type", fmt.Sprintf("%T", task),
		)
		return fmt.Sprintf("# Task: %s\n\nPlease complete this task.", taskID)
	}

	// Get plan summary
	planSummary := "Ultra-Plan Task"
	if e.execCtx != nil && e.execCtx.ExecutionSession != nil {
		planSummary = e.execCtx.ExecutionSession.GetPlanSummary()
	}

	// Determine group index for this task
	groupIndex := 0
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		groupIndex = e.execCtx.Coordinator.GetTaskGroupIndex(taskID)
	}

	// Build prompt context
	ctx := &prompt.Context{
		Phase: prompt.PhaseTask,
		Plan: &prompt.PlanInfo{
			Summary: planSummary,
		},
		Task:       convertPlannedTaskDataToTaskInfo(taskData),
		GroupIndex: groupIndex,
	}

	// Add previous group context if this task is not in group 0
	if groupIndex > 0 && e.execCtx != nil && e.execCtx.ExecutionSession != nil {
		prevGroupIdx := groupIndex - 1
		prevContext := e.execCtx.ExecutionSession.GetGroupConsolidationContext(prevGroupIdx)
		if prevContext != nil {
			ctx.PreviousGroup = convertGroupConsolidationContextToGroupContext(prevContext, prevGroupIdx)
		}
	}

	// Build prompt using TaskBuilder
	builder := prompt.NewTaskBuilder()
	result, err := builder.Build(ctx)
	if err != nil {
		// Fallback to basic prompt on error - include task details for context
		e.logger.Error("failed to build task prompt, using fallback",
			"task_id", taskID,
			"task_title", taskData.GetTitle(),
			"error", err.Error(),
		)
		return fmt.Sprintf("# Task: %s\n\n%s", taskData.GetTitle(), taskData.GetDescription())
	}

	return result
}

// convertPlannedTaskDataToTaskInfo converts a PlannedTaskData interface to prompt.TaskInfo.
// This adapter function bridges the phase package's interface with the prompt package's type.
func convertPlannedTaskDataToTaskInfo(task PlannedTaskData) *prompt.TaskInfo {
	if task == nil {
		return nil
	}

	// Copy files slice to avoid aliasing
	files := task.GetFiles()
	var filesCopy []string
	if files != nil {
		filesCopy = make([]string, len(files))
		copy(filesCopy, files)
	}

	return &prompt.TaskInfo{
		ID:          task.GetID(),
		Title:       task.GetTitle(),
		Description: task.GetDescription(),
		Files:       filesCopy,
	}
}

// convertGroupConsolidationContextToGroupContext converts a GroupConsolidationContextData
// interface to prompt.GroupContext.
func convertGroupConsolidationContextToGroupContext(gc GroupConsolidationContextData, groupIndex int) *prompt.GroupContext {
	if gc == nil {
		return nil
	}

	// Copy issues slice to avoid aliasing
	issues := gc.GetIssuesForNextGroup()
	var issuesCopy []string
	if issues != nil {
		issuesCopy = make([]string, len(issues))
		copy(issuesCopy, issues)
	}

	return &prompt.GroupContext{
		GroupIndex:         groupIndex,
		Notes:              gc.GetNotes(),
		IssuesForNextGroup: issuesCopy,
		VerificationPassed: gc.IsVerificationSuccess(),
	}
}

// monitorTaskInstance monitors an instance and reports when it completes.
// It uses a two-tier detection approach:
//
//  1. Primary detection: Sentinel file (.claudio-task-complete.json)
//     This is the preferred method as it's unambiguous - the task explicitly
//     signals completion by writing this file.
//
//  2. Fallback detection: Status-based checks
//     For tasks that don't write completion files, this handles legacy behavior
//     and edge cases based on the instance's status (Completed, Error, Timeout, Stuck).
//
// The method polls at 1-second intervals until completion is detected or
// the context is cancelled.
func (e *ExecutionOrchestrator) monitorTaskInstance(taskID, instanceID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return

		case <-ticker.C:
			// Get the instance - try multiple methods
			var inst any
			if e.execCtx != nil && e.execCtx.ExecutionOrchestrator != nil {
				inst = e.execCtx.ExecutionOrchestrator.GetInstanceByID(instanceID)
			}
			if inst == nil && e.phaseCtx.Orchestrator != nil {
				// Try the base orchestrator interface
				instIface := e.phaseCtx.Orchestrator.GetInstance(instanceID)
				if instIface != nil {
					inst = instIface
				}
			}

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

			// Log instance status check at DEBUG level
			status := e.getInstanceStatus(inst)
			e.logger.Debug("instance status check",
				"task_id", taskID,
				"instance_id", instanceID,
				"status", string(status),
			)

			// Primary completion detection: check for sentinel file
			// This is the preferred method as it's unambiguous - the task explicitly
			// signals completion by writing this file
			if e.checkForTaskCompletionFile(inst) {
				// Sentinel file exists - task has signaled completion
				// Stop the instance to free up resources
				if e.execCtx != nil && e.execCtx.ExecutionOrchestrator != nil {
					_ = e.execCtx.ExecutionOrchestrator.StopInstance(inst)
				}

				// Verify work was done before marking as success
				result := e.verifyTaskWork(taskID, instanceID, inst)
				e.completionChan <- result
				return
			}

			// Fallback: status-based detection for tasks that don't write completion file
			// This handles legacy behavior and edge cases
			switch status {
			case StatusCompleted:
				// StatusCompleted can be triggered by false positive pattern detection
				// while the instance is still actively working. Only treat as actual
				// completion if the tmux session has truly exited.
				mgr := e.getInstanceManager(instanceID)
				if mgr != nil && mgr.TmuxSessionExists() {
					// Tmux session still running - this was a false positive completion detection
					// Reset status to working and continue monitoring for sentinel file
					e.setInstanceStatus(inst, StatusRunning)
					continue
				}
				// Tmux session has exited - verify work was done
				result := e.verifyTaskWork(taskID, instanceID, inst)
				e.completionChan <- result
				return

			// Note: StatusWaitingInput is intentionally NOT treated as completion.
			// The sentinel file (.claudio-task-complete.json) is the primary completion signal.
			// StatusWaitingInput can trigger too early from Claude Code's UI elements,
			// causing tasks to be marked failed before they complete their work.

			case StatusError, StatusTimeout, StatusStuck:
				e.completionChan <- TaskCompletion{
					TaskID:     taskID,
					InstanceID: instanceID,
					Success:    false,
					Error:      string(status),
				}
				return
			}
		}
	}
}

// handleTaskCompletion processes a task completion notification.
// This method implements duplicate detection, retry handling, and group advancement.
//
// Duplicate detection: Both monitorTaskInstance and pollTaskCompletions can detect
// the same completion file and send to completionChan, causing duplicate processing.
// We track processed tasks to skip duplicates.
//
// Retry handling: When a task needs retry (NeedsRetry=true), we clear its instance
// mapping so the execution loop will pick it up again.
//
// Group advancement: After each successful task, we check if the current group is
// complete and advance to the next group if so.
func (e *ExecutionOrchestrator) handleTaskCompletion(completion TaskCompletion) {
	// Check for duplicate processing (race between monitor goroutine and poll)
	e.mu.Lock()
	if e.state.ProcessedTasks[completion.TaskID] {
		e.mu.Unlock()
		e.logger.Debug("skipping duplicate task completion",
			"task_id", completion.TaskID,
			"instance_id", completion.InstanceID,
		)
		return
	}

	// Only decrement running count if task is still tracked as running
	if _, isRunning := e.state.RunningTasks[completion.TaskID]; isRunning {
		delete(e.state.RunningTasks, completion.TaskID)
		e.state.RunningCount--
	}
	e.mu.Unlock()

	// Also update coordinator state if available
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		e.execCtx.Coordinator.RemoveRunningTask(completion.TaskID)
	}

	// Handle retry case - task needs to be re-run
	if completion.NeedsRetry {
		e.logger.Debug("task needs retry",
			"task_id", completion.TaskID,
			"instance_id", completion.InstanceID,
		)

		// Clear task-to-instance mapping so it becomes "ready" again for the execution loop
		if e.execCtx != nil && e.execCtx.Coordinator != nil {
			e.execCtx.Coordinator.ClearTaskFromInstance(completion.TaskID)
			_ = e.execCtx.Coordinator.SaveSession()
		}

		// Don't mark as processed, completed, or failed - execution loop will pick it up again
		return
	}

	// Mark as processed AFTER we know it's not a retry
	e.mu.Lock()
	e.state.ProcessedTasks[completion.TaskID] = true
	if completion.Success {
		e.state.CompletedCount++
	} else {
		e.state.FailedCount++
	}
	e.mu.Unlock()

	// Record commit count for successful tasks
	if completion.Success && completion.CommitCount > 0 {
		if e.execCtx != nil && e.execCtx.Coordinator != nil {
			e.execCtx.Coordinator.RecordTaskCommitCount(completion.TaskID, completion.CommitCount)
		}
	}

	// Update manager state
	if completion.Success {
		e.phaseCtx.Manager.MarkTaskComplete(completion.TaskID)
		if e.phaseCtx.Callbacks != nil {
			e.phaseCtx.Callbacks.OnTaskComplete(completion.TaskID)
		}
	} else {
		e.phaseCtx.Manager.MarkTaskFailed(completion.TaskID, completion.Error)
		if e.phaseCtx.Callbacks != nil {
			e.phaseCtx.Callbacks.OnTaskFailed(completion.TaskID, completion.Error)
		}
	}

	// Also delegate to coordinator for any additional handling
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		e.execCtx.Coordinator.HandleTaskCompletion(completion)
	}

	// Check if the current group is now complete and advance if so
	e.checkAndAdvanceGroup()
}

// checkAndAdvanceGroup checks if the current execution group is complete
// and advances to the next group, triggering consolidation.
//
// When a group completes, it:
// 1. Checks for partial failure (some tasks succeeded, some failed)
// 2. If partial failure, pauses execution and awaits user decision
// 3. If all succeeded, consolidates all task branches into a single branch
// 4. Advances CurrentGroup to the next group
//
// IMPORTANT: Consolidation runs SYNCHRONOUSLY and blocks until it succeeds.
// This ensures the consolidated branch is ready before tasks from the next
// group start executing.
func (e *ExecutionOrchestrator) checkAndAdvanceGroup() {
	// Need GroupTracker to check completion status
	if e.execCtx == nil || e.execCtx.GroupTracker == nil {
		// No group tracker available - delegate to coordinator if available
		// (Coordinator has its own groupTracker)
		return
	}

	// Get current group from session
	currentGroup := 0
	if e.execCtx.ExecutionSession != nil {
		currentGroup = e.execCtx.ExecutionSession.GetCurrentGroup()
	}

	// Check if the group is complete
	if !e.execCtx.GroupTracker.IsGroupComplete(currentGroup) {
		return
	}

	// Check for partial group failure BEFORE advancing
	// This ensures CurrentGroup stays at the failed group index
	if e.execCtx.GroupTracker.HasPartialFailure(currentGroup) {
		e.handlePartialGroupFailure(currentGroup)
		// Don't advance until user decides - CurrentGroup remains unchanged
		return
	}

	e.logger.Info("group complete, starting consolidation",
		"group_index", currentGroup,
	)

	// Start the group consolidator Claude session
	// This blocks until the consolidator completes (writes completion file)
	if e.execCtx.Coordinator != nil {
		if err := e.execCtx.Coordinator.StartGroupConsolidation(currentGroup); err != nil {
			e.logger.Error("consolidation failed",
				"group_index", currentGroup,
				"error", err.Error(),
			)

			// Mark session as failed since we can't continue without consolidation
			if e.execCtx.Coordinator != nil {
				e.execCtx.Coordinator.SetSessionPhase(PhaseFailed)
				e.execCtx.Coordinator.SetSessionError(fmt.Sprintf("consolidation of group %d failed: %v", currentGroup+1, err))
				_ = e.execCtx.Coordinator.SaveSession()
				e.execCtx.Coordinator.NotifyComplete(false, fmt.Sprintf("consolidation of group %d failed", currentGroup+1))
			}
			return
		}
	}

	// Advance to the next group - only after consolidation succeeds
	nextGroup, _ := e.execCtx.GroupTracker.AdvanceGroup(currentGroup)

	// Log group completion
	groupTasks := e.execCtx.GroupTracker.GetGroupTasks(currentGroup)
	e.logger.Info("group completed",
		"group_index", currentGroup,
		"task_count", len(groupTasks),
		"next_group", nextGroup,
	)

	// Call the group complete callback
	if e.phaseCtx.Callbacks != nil {
		e.phaseCtx.Callbacks.OnGroupComplete(currentGroup)
	}

	// Persist the group advancement
	if e.execCtx.Coordinator != nil {
		_ = e.execCtx.Coordinator.SaveSession()
	}
}

// handlePartialGroupFailure handles a group with mixed success/failure.
// It pauses execution and waits for user decision on how to proceed.
func (e *ExecutionOrchestrator) handlePartialGroupFailure(groupIndex int) {
	e.logger.Info("partial group failure detected",
		"group_index", groupIndex,
	)

	// Delegate to coordinator for the full handling (setting GroupDecision state,
	// emitting events, and persisting state)
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		e.execCtx.Coordinator.HandlePartialGroupFailure(groupIndex)
	}

	// Also track locally
	e.mu.Lock()
	e.state.GroupDecision = &GroupDecisionState{
		GroupIndex:       groupIndex,
		AwaitingDecision: true,
	}
	e.mu.Unlock()
}

// finishExecution completes the execution phase.
// It checks for task failures and either:
// - Marks the session as failed if any tasks failed
// - Skips to completion if synthesis is disabled
// - Starts the synthesis phase for review
func (e *ExecutionOrchestrator) finishExecution() {
	e.mu.RLock()
	completedCount := e.state.CompletedCount
	failedCount := e.state.FailedCount
	totalTasks := e.state.TotalTasks
	e.mu.RUnlock()

	e.logger.Info("execution phase finishing",
		"completed", completedCount,
		"failed", failedCount,
		"total", totalTasks,
	)

	// Check for failures
	if failedCount > 0 {
		errorMsg := fmt.Sprintf("%d task(s) failed", failedCount)
		e.logger.Error("execution phase failed with task failures",
			"failed_count", failedCount,
			"error", errorMsg,
		)

		// Update session phase to failed
		e.phaseCtx.Session.SetPhase(PhaseFailed)
		e.phaseCtx.Session.SetError(errorMsg)

		// Also update via coordinator for persistence
		if e.execCtx != nil && e.execCtx.Coordinator != nil {
			e.execCtx.Coordinator.SetSessionPhase(PhaseFailed)
			e.execCtx.Coordinator.SetSessionError(errorMsg)
			_ = e.execCtx.Coordinator.SaveSession()
			e.execCtx.Coordinator.NotifyComplete(false, errorMsg)
		} else if e.phaseCtx.Callbacks != nil {
			e.phaseCtx.Callbacks.OnComplete(false, errorMsg)
		}
		return
	}

	// Check if synthesis is disabled
	noSynthesis := false
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		noSynthesis = e.execCtx.Coordinator.GetNoSynthesis()
	}

	if noSynthesis {
		e.logger.Info("synthesis skipped per configuration")

		// Mark as complete
		e.phaseCtx.Session.SetPhase(PhaseComplete)

		// Persist completion
		if e.execCtx != nil && e.execCtx.Coordinator != nil {
			e.execCtx.Coordinator.SetSessionPhase(PhaseComplete)
			_ = e.execCtx.Coordinator.SaveSession()
			e.execCtx.Coordinator.NotifyComplete(true, "All tasks completed (synthesis skipped)")
		} else if e.phaseCtx.Callbacks != nil {
			e.phaseCtx.Callbacks.OnComplete(true, "All tasks completed (synthesis skipped)")
		}
		return
	}

	// Start synthesis phase
	e.logger.Info("starting synthesis phase")

	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		if err := e.execCtx.Coordinator.RunSynthesis(); err != nil {
			e.logger.Error("failed to start synthesis",
				"error", err.Error(),
			)
			// Let the error propagate through coordinator's error handling
		}
	} else {
		// No coordinator - notify phase change directly
		e.phaseCtx.Manager.SetPhase(PhaseSynthesis)
		if e.phaseCtx.Callbacks != nil {
			e.phaseCtx.Callbacks.OnPhaseChange(PhaseSynthesis)
		}
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

	if e.state.ProcessedTasks != nil {
		stateCopy.ProcessedTasks = make(map[string]bool, len(e.state.ProcessedTasks))
		maps.Copy(stateCopy.ProcessedTasks, e.state.ProcessedTasks)
	}

	if e.state.GroupDecision != nil {
		stateCopy.GroupDecision = &GroupDecisionState{
			GroupIndex:       e.state.GroupDecision.GroupIndex,
			SucceededTasks:   append([]string{}, e.state.GroupDecision.SucceededTasks...),
			FailedTasks:      append([]string{}, e.state.GroupDecision.FailedTasks...),
			AwaitingDecision: e.state.GroupDecision.AwaitingDecision,
		}
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
		RunningTasks:   make(map[string]string),
		ProcessedTasks: make(map[string]bool),
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

// checkForTaskCompletionFile checks if the task has written its completion sentinel file.
// This checks for both regular task completion (.claudio-task-complete.json) and
// revision task completion (.claudio-revision-complete.json) since both use this monitor.
//
// The method first tries to use the local Verifier if available, then falls back
// to the Coordinator's CheckForTaskCompletionFile method for backwards compatibility.
func (e *ExecutionOrchestrator) checkForTaskCompletionFile(inst any) bool {
	// Get worktree path from the instance
	worktreePath := e.getInstanceWorktreePath(inst)
	if worktreePath == "" {
		return false
	}

	// Try using local verifier first
	if e.execCtx != nil && e.execCtx.Verifier != nil {
		found, err := e.execCtx.Verifier.CheckCompletionFile(worktreePath)
		if err != nil {
			e.logger.Debug("error checking completion file",
				"worktree", worktreePath,
				"error", err)
		}
		return found
	}

	// Fallback to coordinator if available
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		return e.execCtx.Coordinator.CheckForTaskCompletionFile(inst)
	}

	return false
}

// verifyTaskWork checks if a task produced actual commits and determines success/retry.
// This method abstracts verification logic to work with either the local Verifier
// or the Coordinator's verification implementation.
//
// Parameters:
//   - taskID: The ID of the task being verified
//   - instanceID: The ID of the instance that ran the task
//   - inst: The instance object (for accessing worktree path and other metadata)
//
// Returns a TaskCompletion with success status, commit count, and retry information.
func (e *ExecutionOrchestrator) verifyTaskWork(taskID, instanceID string, inst any) TaskCompletion {
	worktreePath := e.getInstanceWorktreePath(inst)

	// Determine the base branch for this task
	baseBranch := ""
	currentGroup := 0
	if e.execCtx != nil && e.execCtx.ExecutionSession != nil {
		currentGroup = e.execCtx.ExecutionSession.GetCurrentGroup()
	}
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		baseBranch = e.execCtx.Coordinator.GetBaseBranchForGroup(currentGroup)
	}

	// Build verification options from task metadata
	var opts *TaskVerifyOptions
	session := e.phaseCtx.Session
	if task := session.GetTask(taskID); task != nil {
		taskData, ok := task.(PlannedTaskData)
		if ok && taskData.IsNoCode() {
			opts = &TaskVerifyOptions{NoCode: true}
		}
	}

	// Try using local verifier first
	if e.execCtx != nil && e.execCtx.Verifier != nil {
		result := e.execCtx.Verifier.VerifyTaskWork(taskID, instanceID, worktreePath, baseBranch, opts)
		// Convert directly since TaskVerifyResult and TaskCompletion have identical field layout
		return TaskCompletion(result)
	}

	// Fallback to coordinator if available
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		return e.execCtx.Coordinator.VerifyTaskWork(taskID, inst)
	}

	// If no verifier is available, assume success (lenient mode)
	return TaskCompletion{
		TaskID:     taskID,
		InstanceID: instanceID,
		Success:    true,
	}
}

// pollTaskCompletions scans all started tasks for completion files.
// This is a fallback mechanism to detect completions when monitoring goroutines
// exit early (e.g., due to context cancellation) or fail to send to the channel.
//
// The method iterates through all tasks that have been assigned to instances
// but haven't been marked as completed or failed yet. For each such task, it
// checks for the presence of a completion file and, if found, stops the instance
// and verifies the work.
//
// Non-blocking sends are used to avoid deadlocks if the completion channel is full.
func (e *ExecutionOrchestrator) pollTaskCompletions(completionChan chan<- TaskCompletion) {
	session := e.phaseCtx.Session
	if session == nil {
		return
	}

	// Get task-to-instance mapping
	taskToInstance := session.GetTaskToInstance()
	completedTasks := session.GetCompletedTasks()

	// Build set of already-finished tasks
	// Note: Failed tasks are tracked in the local state, not the session directly
	finished := make(map[string]bool)
	for _, t := range completedTasks {
		finished[t] = true
	}

	// Also include tasks tracked as completed/failed in local state
	e.mu.RLock()
	localRunning := make(map[string]string)
	maps.Copy(localRunning, e.state.RunningTasks)
	e.mu.RUnlock()

	// Check each started task for completion
	for taskID, instanceID := range taskToInstance {
		if finished[taskID] {
			continue
		}

		// Skip if not currently tracked as running (already processed)
		if _, isRunning := localRunning[taskID]; !isRunning {
			continue
		}

		// Get the instance
		var inst any
		if e.execCtx != nil && e.execCtx.ExecutionOrchestrator != nil {
			inst = e.execCtx.ExecutionOrchestrator.GetInstanceByID(instanceID)
		}
		if inst == nil {
			continue
		}

		// Check for completion file
		if e.checkForTaskCompletionFile(inst) {
			// Stop instance to free resources
			if e.execCtx != nil && e.execCtx.ExecutionOrchestrator != nil {
				_ = e.execCtx.ExecutionOrchestrator.StopInstance(inst)
			}

			// Verify and report
			result := e.verifyTaskWork(taskID, instanceID, inst)

			// Non-blocking send (skip if channel full, will retry next iteration)
			select {
			case completionChan <- result:
			default:
			}
		}
	}
}

// getInstanceWorktreePath extracts the worktree path from an instance.
// The instance may implement various interfaces depending on its source.
func (e *ExecutionOrchestrator) getInstanceWorktreePath(inst any) string {
	if inst == nil {
		return ""
	}

	// Try the InstanceInterface
	if iface, ok := inst.(InstanceInterface); ok {
		return iface.GetWorktreePath()
	}

	// Try common interface patterns
	if getter, ok := inst.(interface{ GetWorktreePath() string }); ok {
		return getter.GetWorktreePath()
	}
	if getter, ok := inst.(interface{ WorktreePath() string }); ok {
		return getter.WorktreePath()
	}

	return ""
}

// getInstanceStatus extracts the status from an instance.
// The instance may implement various interfaces depending on its source.
func (e *ExecutionOrchestrator) getInstanceStatus(inst any) InstanceStatus {
	if inst == nil {
		return ""
	}

	// Try the InstanceInterface
	if iface, ok := inst.(InstanceInterface); ok {
		return iface.GetStatus()
	}

	// Try common interface patterns
	if getter, ok := inst.(interface{ GetStatus() InstanceStatus }); ok {
		return getter.GetStatus()
	}
	if getter, ok := inst.(interface{ Status() InstanceStatus }); ok {
		return getter.Status()
	}

	return ""
}

// setInstanceStatus sets the status on an instance if it supports mutation.
// Returns true if the status was successfully set.
func (e *ExecutionOrchestrator) setInstanceStatus(inst any, status InstanceStatus) bool {
	if inst == nil {
		return false
	}

	// Try common setter patterns
	if setter, ok := inst.(interface{ SetStatus(InstanceStatus) }); ok {
		setter.SetStatus(status)
		return true
	}

	return false
}

// getInstanceManager returns the instance manager for an instance ID.
// The manager provides access to tmux session checking for status-based fallback.
func (e *ExecutionOrchestrator) getInstanceManager(instanceID string) InstanceManagerCheckerInterface {
	if e.phaseCtx.Orchestrator == nil {
		return nil
	}

	mgr := e.phaseCtx.Orchestrator.GetInstanceManager(instanceID)
	if mgr == nil {
		return nil
	}

	// Try to cast to InstanceManagerCheckerInterface
	if checker, ok := mgr.(InstanceManagerCheckerInterface); ok {
		return checker
	}

	return nil
}

// =============================================================================
// Retry, Recovery, and Partial Failure Handling
// =============================================================================

// RetryRecoveryContext holds the extended dependencies required for retry/recovery operations.
// It embeds ExecutionContext and adds retry-specific dependencies.
type RetryRecoveryContext struct {
	*ExecutionContext

	// RetryManager provides retry state management.
	// If nil, retry operations will not be available.
	RetryManager RetryManagerInterface

	// RetryRecoverySession provides extended session access for retry operations.
	// If nil, retry operations will be delegated to the Coordinator.
	RetryRecoverySession RetryRecoverySessionInterface
}

// SetRetryRecoveryContext sets the retry/recovery context after construction.
// This allows adding retry capabilities to an existing orchestrator.
func (e *ExecutionOrchestrator) SetRetryRecoveryContext(retryCtx *RetryRecoveryContext) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if retryCtx != nil {
		e.execCtx = retryCtx.ExecutionContext
	}
}

// HasPartialGroupFailure checks if the specified group has a mix of successful and failed tasks.
// This is used to determine if user intervention is needed before proceeding.
//
// A partial failure occurs when:
// - At least one task in the group completed successfully
// - At least one task in the group failed
//
// Returns true if the group has both successes and failures.
func (e *ExecutionOrchestrator) HasPartialGroupFailure(groupIndex int) bool {
	// Delegate to GroupTracker if available
	if e.execCtx != nil && e.execCtx.GroupTracker != nil {
		return e.execCtx.GroupTracker.HasPartialFailure(groupIndex)
	}

	e.logger.Debug("HasPartialGroupFailure called without GroupTracker",
		"group_index", groupIndex,
	)
	return false
}

// GetGroupDecision returns the current group decision state.
// Returns nil if no partial failure is pending.
func (e *ExecutionOrchestrator) GetGroupDecision() *GroupDecisionState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.state.GroupDecision == nil {
		return nil
	}

	// Return a copy to prevent external mutation
	return &GroupDecisionState{
		GroupIndex:       e.state.GroupDecision.GroupIndex,
		SucceededTasks:   append([]string{}, e.state.GroupDecision.SucceededTasks...),
		FailedTasks:      append([]string{}, e.state.GroupDecision.FailedTasks...),
		AwaitingDecision: e.state.GroupDecision.AwaitingDecision,
	}
}

// IsAwaitingDecision returns true if the orchestrator is paused waiting for user decision.
func (e *ExecutionOrchestrator) IsAwaitingDecision() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.GroupDecision != nil && e.state.GroupDecision.AwaitingDecision
}

// ResumeWithPartialWork continues execution with only the successful tasks from a partial failure.
// Failed tasks are skipped, and the group is consolidated with only the successful work.
//
// Preconditions:
//   - A partial failure must be pending (IsAwaitingDecision returns true)
//
// This method:
//  1. Marks the decision as resolved
//  2. Consolidates only the successful tasks
//  3. Advances to the next group
//  4. Persists state and emits events
//
// Returns an error if no decision is pending or consolidation fails.
func (e *ExecutionOrchestrator) ResumeWithPartialWork() error {
	// Check for pending decision
	e.mu.RLock()
	decision := e.state.GroupDecision
	if decision == nil || !decision.AwaitingDecision {
		e.mu.RUnlock()
		return fmt.Errorf("no pending group decision")
	}
	groupIdx := decision.GroupIndex
	succeededCount := len(decision.SucceededTasks)
	e.mu.RUnlock()

	e.logger.Info("resuming with partial work",
		"group_index", groupIdx,
		"succeeded_count", succeededCount,
	)

	// Mark decision as resolved
	e.mu.Lock()
	e.state.GroupDecision.AwaitingDecision = false
	e.mu.Unlock()

	// Emit event
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		e.execCtx.Coordinator.EmitEvent("group_complete",
			fmt.Sprintf("Continuing group %d with partial work (%d tasks)", groupIdx+1, succeededCount))
	}

	// Consolidate only the successful tasks
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		if err := e.execCtx.Coordinator.ConsolidateGroupWithVerification(groupIdx); err != nil {
			return fmt.Errorf("failed to consolidate partial group: %w", err)
		}
	}

	// Advance to the next group AFTER consolidation succeeds
	// This is critical - without this, checkAndAdvanceGroup() would detect
	// the partial failure again and re-prompt the user
	e.mu.Lock()
	e.state.GroupDecision = nil
	e.mu.Unlock()

	// Persist state
	if e.execCtx != nil && e.execCtx.Coordinator != nil {
		_ = e.execCtx.Coordinator.SaveSession()
	}

	return nil
}

// RetryFailedTasks retries all failed tasks in the current group.
// This resets retry counters and removes tasks from the failed/completed lists,
// allowing the execution loop to pick them up again.
//
// Preconditions:
//   - A partial failure must be pending (IsAwaitingDecision returns true)
//
// This method:
//  1. Resets retry state for each failed task
//  2. Removes tasks from failed/completed lists
//  3. Removes task-to-instance mappings (making them "ready" again)
//  4. Clears the decision state
//  5. Persists state and emits events
//
// Returns an error if no decision is pending.
func (e *ExecutionOrchestrator) RetryFailedTasks() error {
	// Check for pending decision
	e.mu.RLock()
	decision := e.state.GroupDecision
	if decision == nil || !decision.AwaitingDecision {
		e.mu.RUnlock()
		return fmt.Errorf("no pending group decision")
	}
	failedTasks := append([]string{}, decision.FailedTasks...)
	groupIdx := decision.GroupIndex
	e.mu.RUnlock()

	e.logger.Info("retrying failed tasks",
		"group_index", groupIdx,
		"failed_count", len(failedTasks),
	)

	// This operation primarily affects session state, delegate to coordinator
	// The coordinator has direct access to the RetryManager and UltraPlanSession
	if e.execCtx == nil || e.execCtx.Coordinator == nil {
		return fmt.Errorf("coordinator required for RetryFailedTasks")
	}

	// Clear the local decision state
	e.mu.Lock()
	e.state.GroupDecision = nil

	// Reset local state for the failed tasks
	for _, taskID := range failedTasks {
		delete(e.state.ProcessedTasks, taskID)
	}

	// Adjust counts (failed tasks will become ready again)
	e.state.FailedCount -= len(failedTasks)
	if e.state.FailedCount < 0 {
		e.state.FailedCount = 0
	}
	e.mu.Unlock()

	// Emit event
	e.execCtx.Coordinator.EmitEvent("group_complete",
		fmt.Sprintf("Retrying %d failed tasks in group %d", len(failedTasks), groupIdx+1))

	// Delegate actual retry state management to coordinator
	// The coordinator will:
	// - Reset retry counters via RetryManager
	// - Remove from session.FailedTasks and session.CompletedTasks
	// - Remove from session.TaskToInstance
	// - Persist state

	// Clear task mappings so execution loop picks them up
	for _, taskID := range failedTasks {
		e.execCtx.Coordinator.ClearTaskFromInstance(taskID)
	}

	_ = e.execCtx.Coordinator.SaveSession()

	return nil
}

// RetriggerGroup resets execution state to the specified group index and restarts execution.
// All state from groups >= targetGroup is cleared, since subsequent groups depend on the
// re-triggered group's consolidated branch.
//
// Preconditions:
//   - targetGroup must be >= 0 and < number of execution groups
//   - No tasks currently running (GetRunningCount() == 0)
//   - Not awaiting a group decision
//
// This clears:
//   - CompletedTasks for tasks in groups >= targetGroup
//   - FailedTasks for tasks in groups >= targetGroup
//   - TaskToInstance for tasks in groups >= targetGroup
//   - GroupConsolidatedBranches[>= targetGroup]
//   - GroupConsolidatorIDs[>= targetGroup]
//   - GroupConsolidationContexts[>= targetGroup]
//   - TaskRetries for tasks in groups >= targetGroup
//   - TaskCommitCounts for tasks in groups >= targetGroup
//   - Synthesis, Revision, and Consolidation state
//
// Note: This method returns nil upon successfully STARTING the retrigger operation.
// The actual execution happens asynchronously in executionLoop. Errors during execution
// are communicated via CoordinatorEvent callbacks, not through the return value.
func (e *ExecutionOrchestrator) RetriggerGroup(targetGroup int) error {
	// Validate coordinator is available (required for full retrigger functionality)
	if e.execCtx == nil || e.execCtx.Coordinator == nil {
		return fmt.Errorf("coordinator required for RetriggerGroup")
	}

	// Get execution order to validate target group
	executionOrder := e.getExecutionOrder()
	if executionOrder == nil {
		return fmt.Errorf("no plan available")
	}

	numGroups := len(executionOrder)
	if targetGroup < 0 || targetGroup >= numGroups {
		return fmt.Errorf("invalid target group %d (must be 0-%d)", targetGroup, numGroups-1)
	}

	// Check we're not currently executing tasks
	e.mu.RLock()
	runningCount := e.state.RunningCount
	awaitingDecision := e.state.GroupDecision != nil && e.state.GroupDecision.AwaitingDecision
	e.mu.RUnlock()

	if runningCount > 0 {
		return fmt.Errorf("cannot retrigger while %d tasks are running", runningCount)
	}

	if awaitingDecision {
		return fmt.Errorf("cannot retrigger while awaiting group decision")
	}

	// Build set of tasks in groups >= targetGroup
	tasksToReset := make(map[string]bool)
	for groupIdx := targetGroup; groupIdx < numGroups; groupIdx++ {
		for _, taskID := range executionOrder[groupIdx] {
			tasksToReset[taskID] = true
		}
	}

	e.logger.Info("retriggering group",
		"target_group", targetGroup,
		"tasks_to_reset", len(tasksToReset),
	)

	// Reset local state
	e.mu.Lock()
	for taskID := range tasksToReset {
		delete(e.state.ProcessedTasks, taskID)
		delete(e.state.RunningTasks, taskID)
	}
	e.state.GroupDecision = nil

	// Reset counts (will be recalculated from session state)
	e.state.CompletedCount = 0
	e.state.FailedCount = 0
	e.mu.Unlock()

	// Set phase back to executing
	e.phaseCtx.Manager.SetPhase(PhaseExecuting)

	// Emit event
	e.execCtx.Coordinator.EmitEvent("phase_change",
		fmt.Sprintf("Retriggered from group %d", targetGroup))

	// Persist and restart via coordinator
	if err := e.execCtx.Coordinator.SaveSession(); err != nil {
		e.logger.Error("failed to persist retrigger state",
			"target_group", targetGroup,
			"error", err.Error(),
		)
	}

	// Restart execution loop
	e.execCtx.Coordinator.StartExecutionLoop()

	return nil
}

// getExecutionOrder returns the execution order from the session.
// Returns nil if not available.
func (e *ExecutionOrchestrator) getExecutionOrder() [][]string {
	session := e.phaseCtx.Session
	if session == nil {
		return nil
	}

	// Try to get execution order via interface
	if getter, ok := session.(interface{ GetExecutionOrder() [][]string }); ok {
		return getter.GetExecutionOrder()
	}

	return nil
}
