package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// CoordinatorCallbacks holds callbacks for coordinator events
type CoordinatorCallbacks struct {
	// OnPhaseChange is called when the ultra-plan phase changes
	OnPhaseChange func(phase UltraPlanPhase)

	// OnTaskStart is called when a task begins execution
	OnTaskStart func(taskID, instanceID string)

	// OnTaskComplete is called when a task completes successfully
	OnTaskComplete func(taskID string)

	// OnTaskFailed is called when a task fails
	OnTaskFailed func(taskID, reason string)

	// OnGroupComplete is called when an execution group completes
	OnGroupComplete func(groupIndex int)

	// OnPlanReady is called when the plan is ready (after planning phase)
	OnPlanReady func(plan *PlanSpec)

	// OnProgress is called periodically with progress updates
	OnProgress func(completed, total int, phase UltraPlanPhase)

	// OnComplete is called when the entire ultra-plan completes
	OnComplete func(success bool, summary string)
}

// Coordinator orchestrates the execution of an ultra-plan
type Coordinator struct {
	manager     *UltraPlanManager
	orch        *Orchestrator
	baseSession *Session
	callbacks   *CoordinatorCallbacks

	// Running state
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex

	// Task tracking
	runningTasks   map[string]string // taskID -> instanceID
	runningCount   int
}

// NewCoordinator creates a new coordinator for an ultra-plan session
func NewCoordinator(orch *Orchestrator, baseSession *Session, ultraSession *UltraPlanSession) *Coordinator {
	manager := NewUltraPlanManager(orch, baseSession, ultraSession)

	ctx, cancel := context.WithCancel(context.Background())

	return &Coordinator{
		manager:      manager,
		orch:         orch,
		baseSession:  baseSession,
		ctx:          ctx,
		cancelFunc:   cancel,
		runningTasks: make(map[string]string),
	}
}

// SetCallbacks sets the coordinator callbacks
func (c *Coordinator) SetCallbacks(cb *CoordinatorCallbacks) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = cb
}

// Manager returns the underlying ultra-plan manager
func (c *Coordinator) Manager() *UltraPlanManager {
	return c.manager
}

// Session returns the ultra-plan session
func (c *Coordinator) Session() *UltraPlanSession {
	return c.manager.Session()
}

// Plan returns the current plan, if available
func (c *Coordinator) Plan() *PlanSpec {
	session := c.manager.Session()
	if session == nil {
		return nil
	}
	return session.Plan
}

// notifyPhaseChange notifies callbacks of phase change
func (c *Coordinator) notifyPhaseChange(phase UltraPlanPhase) {
	c.manager.SetPhase(phase)

	// Persist the phase change
	_ = c.orch.SaveSession()

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnPhaseChange != nil {
		cb.OnPhaseChange(phase)
	}
}

// notifyTaskStart notifies callbacks of task start
func (c *Coordinator) notifyTaskStart(taskID, instanceID string) {
	c.manager.AssignTaskToInstance(taskID, instanceID)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnTaskStart != nil {
		cb.OnTaskStart(taskID, instanceID)
	}
}

// notifyTaskComplete notifies callbacks of task completion
func (c *Coordinator) notifyTaskComplete(taskID string) {
	c.manager.MarkTaskComplete(taskID)

	// Persist the task completion
	_ = c.orch.SaveSession()

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnTaskComplete != nil {
		cb.OnTaskComplete(taskID)
	}
}

// notifyTaskFailed notifies callbacks of task failure
func (c *Coordinator) notifyTaskFailed(taskID, reason string) {
	c.manager.MarkTaskFailed(taskID, reason)

	// Persist the task failure
	_ = c.orch.SaveSession()

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnTaskFailed != nil {
		cb.OnTaskFailed(taskID, reason)
	}
}

// notifyPlanReady notifies callbacks that planning is complete
func (c *Coordinator) notifyPlanReady(plan *PlanSpec) {
	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnPlanReady != nil {
		cb.OnPlanReady(plan)
	}
}

// notifyProgress notifies callbacks of progress
func (c *Coordinator) notifyProgress() {
	session := c.Session()
	if session == nil || session.Plan == nil {
		return
	}

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnProgress != nil {
		cb.OnProgress(len(session.CompletedTasks), len(session.Plan.Tasks), session.Phase)
	}
}

// notifyComplete notifies callbacks of completion
func (c *Coordinator) notifyComplete(success bool, summary string) {
	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnComplete != nil {
		cb.OnComplete(success, summary)
	}
}

// RunPlanning executes the planning phase
// This creates a coordinator instance that explores the codebase and generates a plan
func (c *Coordinator) RunPlanning() error {
	session := c.Session()
	c.notifyPhaseChange(PhasePlanning)

	// Create the planning prompt
	prompt := fmt.Sprintf(PlanningPromptTemplate, session.Objective)

	// Create a coordinator instance for planning
	inst, err := c.orch.AddInstance(c.baseSession, prompt)
	if err != nil {
		return fmt.Errorf("failed to create planning instance: %w", err)
	}

	session.CoordinatorID = inst.ID

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start planning instance: %w", err)
	}

	// Wait for the instance to complete
	// The TUI will handle monitoring; here we just set up the session state
	return nil
}

// SetPlan sets the plan for this ultra-plan session (used after planning completes)
func (c *Coordinator) SetPlan(plan *PlanSpec) error {
	if err := ValidatePlan(plan); err != nil {
		return fmt.Errorf("invalid plan: %w", err)
	}

	c.mu.Lock()
	c.manager.session.Plan = plan
	c.mu.Unlock()

	// Persist the plan
	_ = c.orch.SaveSession()

	c.notifyPlanReady(plan)

	// Transition to refresh phase (plan ready, waiting for execution)
	c.notifyPhaseChange(PhaseRefresh)

	return nil
}

// StartExecution begins the execution phase
// This spawns child instances for each task group
func (c *Coordinator) StartExecution() error {
	session := c.Session()
	if session.Plan == nil {
		return fmt.Errorf("no plan available")
	}

	c.notifyPhaseChange(PhaseExecuting)

	now := time.Now()
	c.mu.Lock()
	session.StartedAt = &now
	c.mu.Unlock()

	// Start the execution loop in a goroutine
	c.wg.Add(1)
	go c.executionLoop()

	return nil
}

// executionLoop manages the parallel execution of tasks
func (c *Coordinator) executionLoop() {
	defer c.wg.Done()

	session := c.Session()
	config := session.Config

	// Channel for task completion notifications
	completionChan := make(chan taskCompletion, 100)

	for {
		select {
		case <-c.ctx.Done():
			return

		case completion := <-completionChan:
			c.handleTaskCompletion(completion)
			c.notifyProgress()

		default:
			// Check if we're done
			c.mu.RLock()
			completedCount := len(session.CompletedTasks)
			failedCount := len(session.FailedTasks)
			totalTasks := len(session.Plan.Tasks)
			runningCount := c.runningCount
			c.mu.RUnlock()

			if completedCount+failedCount >= totalTasks {
				// All tasks done
				c.finishExecution()
				return
			}

			// Check if we can start more tasks
			if runningCount < config.MaxParallel {
				readyTasks := session.GetReadyTasks()
				for _, taskID := range readyTasks {
					c.mu.RLock()
					currentRunning := c.runningCount
					c.mu.RUnlock()

					if currentRunning >= config.MaxParallel {
						break
					}

					if err := c.startTask(taskID, completionChan); err != nil {
						c.notifyTaskFailed(taskID, err.Error())
					}
				}
			}

			// Small sleep to avoid busy-waiting
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// taskCompletion represents a task completion notification
type taskCompletion struct {
	taskID      string
	instanceID  string
	success     bool
	error       string
	needsRetry  bool // Indicates task should be retried (no commits produced)
	commitCount int  // Number of commits produced by this task
}

// startTask starts a single task as a new instance
func (c *Coordinator) startTask(taskID string, completionChan chan<- taskCompletion) error {
	session := c.Session()
	task := session.GetTask(taskID)
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Create the task prompt with context
	prompt := c.buildTaskPrompt(task)

	// Determine the base branch for this task
	// For tasks in group 0, use the default (HEAD/main)
	// For tasks in later groups, use the consolidated branch from the previous group
	baseBranch := c.getBaseBranchForGroup(session.CurrentGroup)

	// Create a new instance for this task
	var inst *Instance
	var err error
	if baseBranch != "" {
		// Use the consolidated branch from the previous group as the base
		inst, err = c.orch.AddInstanceFromBranch(c.baseSession, prompt, baseBranch)
	} else {
		// Use the default (HEAD/main)
		inst, err = c.orch.AddInstance(c.baseSession, prompt)
	}
	if err != nil {
		return fmt.Errorf("failed to create instance for task %s: %w", taskID, err)
	}

	// Track the running task
	c.mu.Lock()
	c.runningTasks[taskID] = inst.ID
	c.runningCount++
	c.mu.Unlock()

	c.notifyTaskStart(taskID, inst.ID)

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		c.mu.Lock()
		delete(c.runningTasks, taskID)
		c.runningCount--
		c.mu.Unlock()
		return fmt.Errorf("failed to start instance for task %s: %w", taskID, err)
	}

	// Monitor the instance for completion in a goroutine
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.monitorTaskInstance(taskID, inst.ID, completionChan)
	}()

	return nil
}

// buildTaskPrompt creates the prompt for a child task instance
func (c *Coordinator) buildTaskPrompt(task *PlannedTask) string {
	session := c.Session()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Task: %s\n\n", task.Title))
	sb.WriteString(fmt.Sprintf("## Part of Ultra-Plan: %s\n\n", session.Plan.Summary))
	sb.WriteString("## Your Task\n\n")
	sb.WriteString(task.Description)
	sb.WriteString("\n\n")

	if len(task.Files) > 0 {
		sb.WriteString("## Expected Files\n\n")
		sb.WriteString("You are expected to work with these files:\n")
		for _, f := range task.Files {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
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
	sb.WriteString(fmt.Sprintf("  \"task_id\": \"%s\",\n", task.ID))
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

// monitorTaskInstance monitors an instance and reports when it completes
func (c *Coordinator) monitorTaskInstance(taskID, instanceID string, completionChan chan<- taskCompletion) {
	// Poll for completion
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return

		case <-ticker.C:
			inst := c.orch.GetInstance(instanceID)
			if inst == nil {
				completionChan <- taskCompletion{
					taskID:     taskID,
					instanceID: instanceID,
					success:    false,
					error:      "instance not found",
				}
				return
			}

			// Primary completion detection: check for sentinel file
			// This is the preferred method as it's unambiguous - the task explicitly
			// signals completion by writing this file
			if c.checkForTaskCompletionFile(inst) {
				// Sentinel file exists - task has signaled completion
				// Stop the instance to free up resources
				_ = c.orch.StopInstance(inst)

				// Verify work was done before marking as success
				result := c.verifyTaskWork(taskID, inst)
				completionChan <- result
				return
			}

			// Fallback: status-based detection for tasks that don't write completion file
			// This handles legacy behavior and edge cases
			switch inst.Status {
			case StatusCompleted:
				// Instance process has exited - verify work was done
				result := c.verifyTaskWork(taskID, inst)
				completionChan <- result
				return

			case StatusWaitingInput:
				// Task is waiting at prompt without writing completion file.
				// This could be the old behavior or a task that got stuck.
				// Stop the instance to free up resources.
				_ = c.orch.StopInstance(inst)

				// Verify work was done before marking as success
				result := c.verifyTaskWork(taskID, inst)
				completionChan <- result
				return

			case StatusError, StatusTimeout, StatusStuck:
				completionChan <- taskCompletion{
					taskID:     taskID,
					instanceID: instanceID,
					success:    false,
					error:      string(inst.Status),
				}
				return
			}
		}
	}
}

// checkForTaskCompletionFile checks if the task has written its completion sentinel file
func (c *Coordinator) checkForTaskCompletionFile(inst *Instance) bool {
	if inst.WorktreePath == "" {
		return false
	}

	completionPath := TaskCompletionFilePath(inst.WorktreePath)
	if _, err := os.Stat(completionPath); err != nil {
		return false // File doesn't exist yet
	}

	// File exists - try to parse it to ensure it's valid
	completion, err := ParseTaskCompletionFile(inst.WorktreePath)
	if err != nil {
		// File exists but is invalid/incomplete - might still be writing
		return false
	}

	// File is valid - check status
	// Accept any status as "completion" - even "blocked" or "failed" means task is done
	return completion.Status != ""
}

// verifyTaskWork checks if a task produced actual commits and determines success/retry
func (c *Coordinator) verifyTaskWork(taskID string, inst *Instance) taskCompletion {
	session := c.Session()
	config := session.Config

	result := taskCompletion{
		taskID:     taskID,
		instanceID: inst.ID,
		success:    true,
	}

	// Skip verification if not required
	if !config.RequireVerifiedCommits {
		return result
	}

	// Determine the base branch for this task
	baseBranch := c.getBaseBranchForGroup(session.CurrentGroup)
	if baseBranch == "" {
		baseBranch = c.orch.wt.FindMainBranch()
	}

	// Count commits on the task branch beyond the base
	commitCount, err := c.orch.wt.CountCommitsBetween(inst.WorktreePath, baseBranch, "HEAD")
	if err != nil {
		// If we can't count commits, log warning but don't fail
		c.manager.emitEvent(CoordinatorEvent{
			Type:    EventConflict,
			TaskID:  taskID,
			Message: fmt.Sprintf("Warning: could not verify commits for task %s: %v", taskID, err),
		})
		return result
	}

	// Store commit count for later reference
	c.mu.Lock()
	if session.TaskCommitCounts == nil {
		session.TaskCommitCounts = make(map[string]int)
	}
	session.TaskCommitCounts[taskID] = commitCount
	c.mu.Unlock()

	result.commitCount = commitCount

	// Check if task produced any commits
	if commitCount == 0 {
		// No commits - check retry status
		maxRetries := config.MaxTaskRetries
		if maxRetries == 0 {
			maxRetries = 3 // Default if not set
		}
		retryState := c.getOrCreateRetryState(taskID, maxRetries)
		retryState.CommitCounts = append(retryState.CommitCounts, 0)

		if retryState.RetryCount < retryState.MaxRetries {
			// Trigger retry
			retryState.RetryCount++
			retryState.LastError = "task produced no commits"

			result.success = false
			result.needsRetry = true
			result.error = "no_commits_retry"

			c.manager.emitEvent(CoordinatorEvent{
				Type:    EventTaskStarted, // Reuse for retry notification
				TaskID:  taskID,
				Message: fmt.Sprintf("Task %s produced no commits, scheduling retry %d/%d", taskID, retryState.RetryCount, retryState.MaxRetries),
			})
		} else {
			// Max retries exhausted
			result.success = false
			result.needsRetry = false
			result.error = fmt.Sprintf("task produced no commits after %d attempts", retryState.MaxRetries)

			c.manager.emitEvent(CoordinatorEvent{
				Type:    EventTaskFailed,
				TaskID:  taskID,
				Message: fmt.Sprintf("Task %s failed: no commits after %d retry attempts", taskID, retryState.MaxRetries),
			})
		}
	}

	return result
}

// getOrCreateRetryState returns or creates retry state for a task
func (c *Coordinator) getOrCreateRetryState(taskID string, maxRetries int) *TaskRetryState {
	session := c.Session()

	c.mu.Lock()
	defer c.mu.Unlock()

	if session.TaskRetries == nil {
		session.TaskRetries = make(map[string]*TaskRetryState)
	}

	state, exists := session.TaskRetries[taskID]
	if !exists {
		state = &TaskRetryState{
			TaskID:       taskID,
			MaxRetries:   maxRetries,
			CommitCounts: make([]int, 0),
		}
		session.TaskRetries[taskID] = state
	}

	return state
}

// handleTaskCompletion handles a task completion notification
func (c *Coordinator) handleTaskCompletion(completion taskCompletion) {
	c.mu.Lock()
	delete(c.runningTasks, completion.taskID)
	c.runningCount--
	c.mu.Unlock()

	// Handle retry case - task needs to be re-run
	if completion.needsRetry {
		session := c.Session()

		// Remove from TaskToInstance so it becomes "ready" again for the execution loop
		c.mu.Lock()
		delete(session.TaskToInstance, completion.taskID)
		c.mu.Unlock()

		// Save state for persistence
		_ = c.orch.SaveSession()

		// Don't mark as complete or failed - execution loop will pick it up again
		return
	}

	if completion.success {
		c.notifyTaskComplete(completion.taskID)
	} else {
		c.notifyTaskFailed(completion.taskID, completion.error)
	}

	// Check if the current group is now complete and advance if so
	c.checkAndAdvanceGroup()
}

// checkAndAdvanceGroup checks if the current execution group is complete
// and advances to the next group, emitting EventGroupComplete.
// When a group completes, it consolidates all parallel task branches from that group
// into a single branch, which becomes the base for the next group's tasks.
// IMPORTANT: This now runs consolidation SYNCHRONOUSLY and blocks until it succeeds.
func (c *Coordinator) checkAndAdvanceGroup() {
	session := c.Session()
	if session == nil || session.Plan == nil {
		return
	}

	c.mu.Lock()
	advanced, previousGroup := session.AdvanceGroupIfComplete()
	c.mu.Unlock()

	if !advanced {
		return
	}

	// Check for partial group failure (some tasks succeeded, some failed)
	if c.hasPartialGroupFailure(previousGroup) {
		c.handlePartialGroupFailure(previousGroup)
		// Don't advance until user decides
		return
	}

	// Emit group complete event
	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventGroupComplete,
		Message: fmt.Sprintf("Group %d complete, consolidating before advancing to group %d", previousGroup+1, session.CurrentGroup+1),
	})

	// SYNCHRONOUSLY consolidate the completed group's parallel task branches
	// This blocks until consolidation succeeds with verified commits
	if err := c.consolidateGroupWithVerification(previousGroup); err != nil {
		c.manager.emitEvent(CoordinatorEvent{
			Type:    EventConflict,
			Message: fmt.Sprintf("Critical: failed to consolidate group %d: %v", previousGroup+1, err),
		})

		// Mark session as failed since we can't continue without consolidation
		c.mu.Lock()
		session.Phase = PhaseFailed
		session.Error = fmt.Sprintf("consolidation of group %d failed: %v", previousGroup+1, err)
		c.mu.Unlock()
		_ = c.orch.SaveSession()
		c.notifyComplete(false, session.Error)
		return
	}

	// Call the callback
	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()
	if cb != nil && cb.OnGroupComplete != nil {
		cb.OnGroupComplete(previousGroup)
	}

	// Persist the group advancement
	_ = c.orch.SaveSession()
}

// finishExecution completes the execution phase
func (c *Coordinator) finishExecution() {
	session := c.Session()

	// Check for failures
	if len(session.FailedTasks) > 0 {
		c.mu.Lock()
		session.Phase = PhaseFailed
		session.Error = fmt.Sprintf("%d task(s) failed", len(session.FailedTasks))
		c.mu.Unlock()

		// Persist the failure state
		_ = c.orch.SaveSession()

		c.notifyComplete(false, session.Error)
		return
	}

	// Check if synthesis is disabled
	if session.Config.NoSynthesis {
		c.mu.Lock()
		session.Phase = PhaseComplete
		now := time.Now()
		session.CompletedAt = &now
		c.mu.Unlock()

		// Persist the completion state
		_ = c.orch.SaveSession()

		c.notifyComplete(true, "All tasks completed (synthesis skipped)")
		return
	}

	// Start synthesis phase
	_ = c.RunSynthesis()
}

// RunSynthesis executes the synthesis phase
func (c *Coordinator) RunSynthesis() error {
	c.notifyPhaseChange(PhaseSynthesis)

	// Build the synthesis prompt
	prompt := c.buildSynthesisPrompt()

	// Create a synthesis instance
	inst, err := c.orch.AddInstance(c.baseSession, prompt)
	if err != nil {
		return fmt.Errorf("failed to create synthesis instance: %w", err)
	}

	// Store the synthesis instance ID for TUI visibility
	session := c.Session()
	session.SynthesisID = inst.ID

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start synthesis instance: %w", err)
	}

	// Monitor the synthesis instance for completion
	// When it completes, automatically trigger consolidation
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.monitorSynthesisInstance(inst.ID)
	}()

	return nil
}

// monitorSynthesisInstance monitors the synthesis instance and triggers consolidation when complete
func (c *Coordinator) monitorSynthesisInstance(instanceID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return

		case <-ticker.C:
			inst := c.orch.GetInstance(instanceID)
			if inst == nil {
				// Instance gone, assume complete
				c.onSynthesisComplete()
				return
			}

			switch inst.Status {
			case StatusCompleted:
				// Synthesis fully completed - trigger consolidation or finish
				c.onSynthesisComplete()
				return

			// Note: StatusWaitingInput is intentionally NOT treated as completion.
			// Synthesis may need multiple user interactions. Use TriggerConsolidation()
			// or the [s] keybinding to manually signal synthesis is done.

			case StatusError, StatusTimeout, StatusStuck:
				// Synthesis failed
				session := c.Session()
				c.mu.Lock()
				session.Phase = PhaseFailed
				session.Error = fmt.Sprintf("synthesis failed: %s", inst.Status)
				c.mu.Unlock()
				_ = c.orch.SaveSession()
				c.notifyComplete(false, session.Error)
				return
			}
		}
	}
}

// onSynthesisComplete handles synthesis completion and triggers revision or consolidation
func (c *Coordinator) onSynthesisComplete() {
	session := c.Session()

	// Try to parse revision issues from synthesis output
	issues := c.parseRevisionIssues()

	// Filter to only critical/major issues that need revision
	var issuesNeedingRevision []RevisionIssue
	for _, issue := range issues {
		if issue.Severity == "critical" || issue.Severity == "major" || issue.Severity == "" {
			issuesNeedingRevision = append(issuesNeedingRevision, issue)
		}
	}

	// If there are issues that need revision, start the revision phase
	if len(issuesNeedingRevision) > 0 {
		// Check if we've already had too many revision rounds
		if session.Revision != nil && session.Revision.RevisionRound >= session.Revision.MaxRevisions {
			// Max revisions reached, proceed to consolidation anyway
			c.captureTaskWorktreeInfo()
			c.proceedToConsolidationOrComplete()
			return
		}

		if err := c.StartRevision(issuesNeedingRevision); err != nil {
			c.mu.Lock()
			session.Phase = PhaseFailed
			session.Error = fmt.Sprintf("revision failed: %v", err)
			c.mu.Unlock()
			_ = c.orch.SaveSession()
			c.notifyComplete(false, session.Error)
		}
		return
	}

	// No issues - capture worktree info and proceed to consolidation or complete
	c.captureTaskWorktreeInfo()
	c.proceedToConsolidationOrComplete()
}

// parseRevisionIssues extracts revision issues from the synthesis instance output
func (c *Coordinator) parseRevisionIssues() []RevisionIssue {
	session := c.Session()
	if session.SynthesisID == "" {
		return nil
	}

	inst := c.orch.GetInstance(session.SynthesisID)
	if inst == nil {
		return nil
	}

	// Get the output from the instance manager
	mgr := c.orch.instances[inst.ID]
	if mgr == nil {
		return nil
	}

	outputBytes := mgr.GetOutput()
	if len(outputBytes) == 0 {
		return nil
	}

	issues, err := ParseRevisionIssuesFromOutput(string(outputBytes))
	if err != nil {
		// Log but don't fail - just proceed without revision
		return nil
	}

	return issues
}

// captureTaskWorktreeInfo captures worktree information for all completed tasks
func (c *Coordinator) captureTaskWorktreeInfo() {
	session := c.Session()
	if session.Plan == nil {
		return
	}

	var worktreeInfo []TaskWorktreeInfo
	for _, taskID := range session.CompletedTasks {
		task := session.GetTask(taskID)
		if task == nil {
			continue
		}

		// Find the instance for this task
		for _, inst := range c.baseSession.Instances {
			if strings.Contains(inst.Task, taskID) || strings.Contains(inst.Branch, slugify(task.Title)) {
				worktreeInfo = append(worktreeInfo, TaskWorktreeInfo{
					TaskID:       taskID,
					TaskTitle:    task.Title,
					WorktreePath: inst.WorktreePath,
					Branch:       inst.Branch,
				})
				break
			}
		}
	}

	c.mu.Lock()
	session.TaskWorktrees = worktreeInfo
	c.mu.Unlock()
}

// proceedToConsolidationOrComplete moves to consolidation if configured, otherwise completes
func (c *Coordinator) proceedToConsolidationOrComplete() {
	session := c.Session()

	// Check if consolidation is configured
	if session.Config.ConsolidationMode != "" {
		if err := c.StartConsolidation(); err != nil {
			// Consolidation failed to start
			c.mu.Lock()
			session.Phase = PhaseFailed
			session.Error = fmt.Sprintf("consolidation failed: %v", err)
			c.mu.Unlock()
			_ = c.orch.SaveSession()
			c.notifyComplete(false, session.Error)
		}
		return
	}

	// No consolidation - mark complete
	c.mu.Lock()
	session.Phase = PhaseComplete
	now := time.Now()
	session.CompletedAt = &now
	c.mu.Unlock()
	_ = c.orch.SaveSession()
	c.notifyComplete(true, "All tasks completed and synthesized")
}

// StartRevision begins the revision phase to address identified issues
func (c *Coordinator) StartRevision(issues []RevisionIssue) error {
	session := c.Session()
	c.notifyPhaseChange(PhaseRevision)

	// Initialize or update revision state
	c.mu.Lock()
	if session.Revision == nil {
		session.Revision = NewRevisionState(issues)
		now := time.Now()
		session.Revision.StartedAt = &now
	} else {
		// Increment revision round
		session.Revision.RevisionRound++
		session.Revision.Issues = issues
		session.Revision.TasksToRevise = extractTasksToRevise(issues)
		session.Revision.RevisedTasks = make([]string, 0)
	}
	c.mu.Unlock()

	// Start revision tasks for each affected task
	completionChan := make(chan taskCompletion, 100)

	for _, taskID := range session.Revision.TasksToRevise {
		if err := c.startRevisionTask(taskID, completionChan); err != nil {
			c.notifyTaskFailed(taskID, fmt.Sprintf("revision failed: %v", err))
		}
	}

	// Monitor revision tasks in a goroutine
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.monitorRevisionTasks(completionChan)
	}()

	return nil
}

// startRevisionTask starts a revision task for a specific task
func (c *Coordinator) startRevisionTask(taskID string, completionChan chan<- taskCompletion) error {
	session := c.Session()
	task := session.GetTask(taskID)
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Find the original instance for this task to get its worktree
	var originalInst *Instance
	for _, inst := range c.baseSession.Instances {
		if strings.Contains(inst.Task, taskID) || strings.Contains(inst.Branch, slugify(task.Title)) {
			originalInst = inst
			break
		}
	}

	if originalInst == nil {
		return fmt.Errorf("original instance for task %s not found", taskID)
	}

	// Build the revision prompt
	prompt := c.buildRevisionPrompt(task)

	// Create a new instance using the SAME worktree as the original task
	inst, err := c.orch.AddInstanceToWorktree(c.baseSession, prompt, originalInst.WorktreePath, originalInst.Branch)
	if err != nil {
		return fmt.Errorf("failed to create revision instance for task %s: %w", taskID, err)
	}

	c.mu.Lock()
	session.RevisionID = inst.ID
	c.mu.Unlock()

	// Track the running task
	c.mu.Lock()
	c.runningTasks[taskID] = inst.ID
	c.runningCount++
	c.mu.Unlock()

	c.notifyTaskStart(taskID, inst.ID)

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		c.mu.Lock()
		delete(c.runningTasks, taskID)
		c.runningCount--
		c.mu.Unlock()
		return fmt.Errorf("failed to start revision instance for task %s: %w", taskID, err)
	}

	// Monitor the instance for completion
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.monitorTaskInstance(taskID, inst.ID, completionChan)
	}()

	return nil
}

// buildRevisionPrompt creates the prompt for a revision task
func (c *Coordinator) buildRevisionPrompt(task *PlannedTask) string {
	session := c.Session()

	// Gather issues for this specific task
	var taskIssues []RevisionIssue
	for _, issue := range session.Revision.Issues {
		if issue.TaskID == task.ID || issue.TaskID == "" {
			taskIssues = append(taskIssues, issue)
		}
	}

	// Format issues as a readable list
	var issuesStr strings.Builder
	for i, issue := range taskIssues {
		issuesStr.WriteString(fmt.Sprintf("%d. **%s**: %s\n", i+1, issue.Severity, issue.Description))
		if len(issue.Files) > 0 {
			issuesStr.WriteString(fmt.Sprintf("   Files: %s\n", strings.Join(issue.Files, ", ")))
		}
		if issue.Suggestion != "" {
			issuesStr.WriteString(fmt.Sprintf("   Suggestion: %s\n", issue.Suggestion))
		}
		issuesStr.WriteString("\n")
	}

	return fmt.Sprintf(RevisionPromptTemplate,
		session.Objective,
		task.ID,
		task.Title,
		task.Description,
		issuesStr.String(),
	)
}

// monitorRevisionTasks monitors all revision tasks and triggers re-synthesis when complete
func (c *Coordinator) monitorRevisionTasks(completionChan <-chan taskCompletion) {
	session := c.Session()

	for {
		select {
		case <-c.ctx.Done():
			return

		case completion := <-completionChan:
			c.handleRevisionTaskCompletion(completion)

			// Check if all revision tasks are complete
			c.mu.RLock()
			allComplete := len(session.Revision.RevisedTasks) >= len(session.Revision.TasksToRevise)
			c.mu.RUnlock()

			if allComplete {
				c.onRevisionComplete()
				return
			}
		}
	}
}

// handleRevisionTaskCompletion handles a revision task completion
func (c *Coordinator) handleRevisionTaskCompletion(completion taskCompletion) {
	session := c.Session()

	c.mu.Lock()
	delete(c.runningTasks, completion.taskID)
	c.runningCount--

	if completion.success {
		session.Revision.RevisedTasks = append(session.Revision.RevisedTasks, completion.taskID)
	}
	c.mu.Unlock()

	if completion.success {
		c.notifyTaskComplete(completion.taskID)
	} else {
		c.notifyTaskFailed(completion.taskID, completion.error)
	}
}

// onRevisionComplete handles completion of all revision tasks
func (c *Coordinator) onRevisionComplete() {
	session := c.Session()

	c.mu.Lock()
	now := time.Now()
	session.Revision.CompletedAt = &now
	c.mu.Unlock()

	// Re-run synthesis to check if issues are resolved
	_ = c.RunSynthesis()
}

// TriggerConsolidation manually signals that synthesis is done and consolidation should proceed.
// This is called from the TUI when the user indicates they're done with synthesis review.
func (c *Coordinator) TriggerConsolidation() error {
	session := c.Session()
	if session == nil {
		return fmt.Errorf("no session")
	}

	// Only allow triggering from synthesis phase
	if session.Phase != PhaseSynthesis {
		return fmt.Errorf("can only trigger consolidation during synthesis phase (current: %s)", session.Phase)
	}

	// Stop the synthesis instance if it's still running
	if session.SynthesisID != "" {
		inst := c.orch.GetInstance(session.SynthesisID)
		if inst != nil {
			_ = c.orch.StopInstance(inst)
		}
	}

	// Proceed to consolidation (or completion if no consolidation configured)
	c.onSynthesisComplete()
	return nil
}


// StartConsolidation begins the consolidation phase
// This creates a Claude instance that performs branch consolidation and PR creation
func (c *Coordinator) StartConsolidation() error {
	session := c.Session()
	c.notifyPhaseChange(PhaseConsolidating)

	// Initialize consolidation state
	c.mu.Lock()
	session.Consolidation = &ConsolidationState{
		Phase:       ConsolidationCreatingBranches,
		TotalGroups: len(session.Plan.ExecutionOrder),
	}
	c.mu.Unlock()

	// Build the consolidation prompt
	prompt := c.buildConsolidationPrompt()

	// Create a consolidation instance
	inst, err := c.orch.AddInstance(c.baseSession, prompt)
	if err != nil {
		return fmt.Errorf("failed to create consolidation instance: %w", err)
	}

	// Store the consolidation instance ID for TUI visibility
	session.ConsolidationID = inst.ID

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start consolidation instance: %w", err)
	}

	// Monitor the consolidation instance for completion
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.monitorConsolidationInstance(inst.ID)
	}()

	return nil
}

// buildConsolidationPrompt creates the prompt for the consolidation phase
func (c *Coordinator) buildConsolidationPrompt() string {
	session := c.Session()

	// Get branch configuration
	branchPrefix := session.Config.BranchPrefix
	if branchPrefix == "" {
		branchPrefix = c.orch.config.Branch.Prefix
	}
	if branchPrefix == "" {
		branchPrefix = "Iron-Ham"
	}

	mainBranch := c.orch.wt.FindMainBranch()
	mode := string(session.Config.ConsolidationMode)
	createDrafts := session.Config.CreateDraftPRs

	// Check if we have pre-consolidated group branches (from incremental consolidation)
	hasPreConsolidatedBranches := len(session.GroupConsolidatedBranches) > 0

	// Build the execution groups and task branches information
	var groupsInfo strings.Builder
	for groupIdx, taskIDs := range session.Plan.ExecutionOrder {
		groupsInfo.WriteString(fmt.Sprintf("\n### Group %d\n", groupIdx+1))

		// If we have a pre-consolidated branch for this group, include it
		if hasPreConsolidatedBranches && groupIdx < len(session.GroupConsolidatedBranches) {
			consolidatedBranch := session.GroupConsolidatedBranches[groupIdx]
			if consolidatedBranch != "" {
				groupsInfo.WriteString(fmt.Sprintf("**CONSOLIDATED BRANCH (ALREADY MERGED)**: %s\n", consolidatedBranch))
				groupsInfo.WriteString("The tasks in this group have already been consolidated into this branch.\n")
			}
		}

		groupsInfo.WriteString("Tasks in this group:\n")
		for _, taskID := range taskIDs {
			task := session.GetTask(taskID)
			if task == nil {
				continue
			}

			// Find the branch for this task
			branchName := "unknown"
			for _, inst := range c.baseSession.Instances {
				if strings.Contains(inst.Task, taskID) || strings.Contains(inst.Branch, slugify(task.Title)) {
					branchName = inst.Branch
					break
				}
			}

			groupsInfo.WriteString(fmt.Sprintf("- Task: %s (%s)\n", task.Title, taskID))
			groupsInfo.WriteString(fmt.Sprintf("  Branch: %s\n", branchName))
		}
	}

	// Build worktree details from captured task worktree info
	var worktreeInfo strings.Builder
	if len(session.TaskWorktrees) > 0 {
		for _, twi := range session.TaskWorktrees {
			worktreeInfo.WriteString(fmt.Sprintf("- **%s** (%s)\n", twi.TaskTitle, twi.TaskID))
			worktreeInfo.WriteString(fmt.Sprintf("  - Worktree: %s\n", twi.WorktreePath))
			worktreeInfo.WriteString(fmt.Sprintf("  - Branch: %s\n", twi.Branch))
		}
	} else {
		// Fall back to building from instances if TaskWorktrees wasn't captured
		for _, taskID := range session.CompletedTasks {
			task := session.GetTask(taskID)
			if task == nil {
				continue
			}
			for _, inst := range c.baseSession.Instances {
				if strings.Contains(inst.Task, taskID) || strings.Contains(inst.Branch, slugify(task.Title)) {
					worktreeInfo.WriteString(fmt.Sprintf("- **%s** (%s)\n", task.Title, taskID))
					worktreeInfo.WriteString(fmt.Sprintf("  - Worktree: %s\n", inst.WorktreePath))
					worktreeInfo.WriteString(fmt.Sprintf("  - Branch: %s\n", inst.Branch))
					break
				}
			}
		}
	}

	// Add note about pre-consolidated branches if available
	if hasPreConsolidatedBranches {
		worktreeInfo.WriteString("\n## Pre-Consolidated Group Branches\n")
		worktreeInfo.WriteString("**IMPORTANT**: Groups have already been incrementally consolidated. Use these branches directly:\n")
		for groupIdx, branch := range session.GroupConsolidatedBranches {
			if branch != "" {
				worktreeInfo.WriteString(fmt.Sprintf("- Group %d: %s\n", groupIdx+1, branch))
			}
		}
		worktreeInfo.WriteString("\nYou do NOT need to cherry-pick individual task branches - just create PRs from these consolidated branches.\n")
	}

	return fmt.Sprintf(ConsolidationPromptTemplate,
		session.Objective,
		branchPrefix,
		mainBranch,
		mode,
		createDrafts,
		groupsInfo.String(),
		worktreeInfo.String(),
	)
}

// monitorConsolidationInstance monitors the consolidation instance and completes when done
func (c *Coordinator) monitorConsolidationInstance(instanceID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return

		case <-ticker.C:
			inst := c.orch.GetInstance(instanceID)
			if inst == nil {
				// Instance gone, assume complete
				c.finishConsolidation()
				return
			}

			switch inst.Status {
			case StatusCompleted, StatusWaitingInput:
				// Consolidation complete
				c.finishConsolidation()
				return

			case StatusError, StatusTimeout, StatusStuck:
				// Consolidation failed
				session := c.Session()
				c.mu.Lock()
				session.Phase = PhaseFailed
				session.Error = fmt.Sprintf("consolidation failed: %s", inst.Status)
				if session.Consolidation != nil {
					session.Consolidation.Phase = ConsolidationFailed
					session.Consolidation.Error = string(inst.Status)
				}
				c.mu.Unlock()
				_ = c.orch.SaveSession()
				c.notifyComplete(false, session.Error)
				return
			}
		}
	}
}

// handleConsolidationError handles a consolidation error (legacy, kept for compatibility)
func (c *Coordinator) handleConsolidationError(err error, consolidator *Consolidator) {
	session := c.Session()

	// Check if it's a conflict error (consolidation is paused)
	if _, ok := err.(*ConflictError); ok {
		// Consolidation is paused waiting for resolution
		c.mu.Lock()
		session.Consolidation = consolidator.State()
		c.mu.Unlock()
		_ = c.orch.SaveSession()
		// Don't mark as failed - it's paused
		return
	}

	// Actual error
	c.mu.Lock()
	session.Phase = PhaseFailed
	session.Error = fmt.Sprintf("consolidation failed: %v", err)
	session.Consolidation = consolidator.State()
	c.mu.Unlock()
	_ = c.orch.SaveSession()
	c.notifyComplete(false, session.Error)
}

// finishConsolidation completes the ultraplan after successful consolidation
func (c *Coordinator) finishConsolidation() {
	session := c.Session()

	c.mu.Lock()
	session.Phase = PhaseComplete
	now := time.Now()
	session.CompletedAt = &now
	if session.Consolidation != nil {
		session.Consolidation.Phase = ConsolidationComplete
		completedAt := time.Now()
		session.Consolidation.CompletedAt = &completedAt
	}
	c.mu.Unlock()
	_ = c.orch.SaveSession()

	prCount := len(session.PRUrls)
	c.notifyComplete(true, fmt.Sprintf("Completed: %d PR(s) created", prCount))
}

// GetConsolidation returns the current consolidation state
func (c *Coordinator) GetConsolidation() *ConsolidationState {
	session := c.Session()
	if session == nil {
		return nil
	}
	return session.Consolidation
}

// buildSynthesisPrompt creates the prompt for the synthesis phase
func (c *Coordinator) buildSynthesisPrompt() string {
	session := c.Session()

	var taskList strings.Builder
	var resultsSummary strings.Builder

	for _, taskID := range session.CompletedTasks {
		task := session.GetTask(taskID)
		if task == nil {
			continue
		}

		// Include commit count in task list
		commitCount := 0
		if count, ok := session.TaskCommitCounts[taskID]; ok {
			commitCount = count
		}

		if commitCount > 0 {
			taskList.WriteString(fmt.Sprintf("- [%s] %s (%d commits)\n", task.ID, task.Title, commitCount))
		} else {
			taskList.WriteString(fmt.Sprintf("- [%s] %s (NO COMMITS - verify this task)\n", task.ID, task.Title))
		}
	}

	// Get summaries from completed instances
	for taskID, instanceID := range session.TaskToInstance {
		task := session.GetTask(taskID)
		inst := c.orch.GetInstance(instanceID)
		if task != nil && inst != nil {
			resultsSummary.WriteString(fmt.Sprintf("### %s\n", task.Title))
			resultsSummary.WriteString(fmt.Sprintf("Status: %s\n", inst.Status))

			// Add commit count
			if count, ok := session.TaskCommitCounts[taskID]; ok {
				resultsSummary.WriteString(fmt.Sprintf("Commits: %d\n", count))
			}

			if len(inst.FilesModified) > 0 {
				resultsSummary.WriteString(fmt.Sprintf("Files modified: %s\n", strings.Join(inst.FilesModified, ", ")))
			}
			resultsSummary.WriteString("\n")
		}
	}

	// Also include tasks that completed but are no longer in TaskToInstance
	for _, taskID := range session.CompletedTasks {
		if _, inMap := session.TaskToInstance[taskID]; inMap {
			continue // Already processed above
		}
		task := session.GetTask(taskID)
		if task == nil {
			continue
		}
		resultsSummary.WriteString(fmt.Sprintf("### %s\n", task.Title))
		resultsSummary.WriteString("Status: completed\n")
		if count, ok := session.TaskCommitCounts[taskID]; ok {
			resultsSummary.WriteString(fmt.Sprintf("Commits: %d\n", count))
		}
		resultsSummary.WriteString("\n")
	}

	return fmt.Sprintf(SynthesisPromptTemplate, session.Objective, taskList.String(), resultsSummary.String())
}

// Cancel cancels the ultra-plan execution
func (c *Coordinator) Cancel() {
	c.cancelFunc()

	// Stop all running task instances
	c.mu.RLock()
	runningTasks := make(map[string]string, len(c.runningTasks))
	for k, v := range c.runningTasks {
		runningTasks[k] = v
	}
	c.mu.RUnlock()

	for _, instanceID := range runningTasks {
		inst := c.orch.GetInstance(instanceID)
		if inst != nil {
			_ = c.orch.StopInstance(inst)
		}
	}

	c.manager.Stop()
	c.wg.Wait()

	c.mu.Lock()
	session := c.Session()
	session.Phase = PhaseFailed
	session.Error = "cancelled by user"
	c.mu.Unlock()

	// Persist the cancellation state
	_ = c.orch.SaveSession()
}

// Wait waits for the ultra-plan to complete
func (c *Coordinator) Wait() {
	c.wg.Wait()
}

// GetProgress returns the current progress
func (c *Coordinator) GetProgress() (completed, total int, phase UltraPlanPhase) {
	session := c.Session()
	if session == nil {
		return 0, 0, PhasePlanning
	}

	if session.Plan == nil {
		return 0, 0, session.Phase
	}

	return len(session.CompletedTasks), len(session.Plan.Tasks), session.Phase
}

// GetRunningTasks returns the currently running tasks and their instance IDs
func (c *Coordinator) GetRunningTasks() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]string, len(c.runningTasks))
	for k, v := range c.runningTasks {
		result[k] = v
	}
	return result
}

// hasPartialGroupFailure checks if a group has a mix of successful and failed tasks
func (c *Coordinator) hasPartialGroupFailure(groupIndex int) bool {
	session := c.Session()
	if session == nil || session.Plan == nil {
		return false
	}

	if groupIndex >= len(session.Plan.ExecutionOrder) {
		return false
	}

	taskIDs := session.Plan.ExecutionOrder[groupIndex]
	successCount := 0
	failureCount := 0

	for _, taskID := range taskIDs {
		// Check if in completed with verified commits
		isCompleted := false
		for _, ct := range session.CompletedTasks {
			if ct == taskID {
				isCompleted = true
				break
			}
		}

		if isCompleted {
			// Verify it has commits
			if count, ok := session.TaskCommitCounts[taskID]; ok && count > 0 {
				successCount++
			} else {
				failureCount++
			}
			continue
		}

		// Check if in failed
		for _, ft := range session.FailedTasks {
			if ft == taskID {
				failureCount++
				break
			}
		}
	}

	// Partial failure = at least one success AND at least one failure
	return successCount > 0 && failureCount > 0
}

// handlePartialGroupFailure pauses execution and waits for user decision
func (c *Coordinator) handlePartialGroupFailure(groupIndex int) {
	session := c.Session()

	taskIDs := session.Plan.ExecutionOrder[groupIndex]
	var succeeded, failed []string

	for _, taskID := range taskIDs {
		isCompleted := false
		for _, ct := range session.CompletedTasks {
			if ct == taskID {
				isCompleted = true
				break
			}
		}

		if isCompleted {
			if count, ok := session.TaskCommitCounts[taskID]; ok && count > 0 {
				succeeded = append(succeeded, taskID)
			} else {
				failed = append(failed, taskID)
			}
		} else {
			// Check if failed
			for _, ft := range session.FailedTasks {
				if ft == taskID {
					failed = append(failed, taskID)
					break
				}
			}
		}
	}

	c.mu.Lock()
	session.GroupDecision = &GroupDecisionState{
		GroupIndex:       groupIndex,
		SucceededTasks:   succeeded,
		FailedTasks:      failed,
		AwaitingDecision: true,
	}
	c.mu.Unlock()

	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventGroupComplete,
		Message: fmt.Sprintf("Group %d has partial success (%d/%d tasks succeeded). Awaiting user decision.", groupIndex+1, len(succeeded), len(taskIDs)),
	})

	_ = c.orch.SaveSession()
}

// ResumeWithPartialWork continues execution with only the successful tasks
func (c *Coordinator) ResumeWithPartialWork() error {
	session := c.Session()
	if session.GroupDecision == nil || !session.GroupDecision.AwaitingDecision {
		return fmt.Errorf("no pending group decision")
	}

	groupIdx := session.GroupDecision.GroupIndex

	c.mu.Lock()
	session.GroupDecision.AwaitingDecision = false
	c.mu.Unlock()

	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventGroupComplete,
		Message: fmt.Sprintf("Continuing group %d with partial work (%d tasks)", groupIdx+1, len(session.GroupDecision.SucceededTasks)),
	})

	// Consolidate only the successful tasks
	if err := c.consolidateGroupWithVerification(groupIdx); err != nil {
		return fmt.Errorf("failed to consolidate partial group: %w", err)
	}

	// Clear the decision state
	c.mu.Lock()
	session.GroupDecision = nil
	c.mu.Unlock()

	// Continue execution
	_ = c.orch.SaveSession()
	return nil
}

// RetryFailedTasks retries the failed tasks in the current group
func (c *Coordinator) RetryFailedTasks() error {
	session := c.Session()
	if session.GroupDecision == nil || !session.GroupDecision.AwaitingDecision {
		return fmt.Errorf("no pending group decision")
	}

	failedTasks := session.GroupDecision.FailedTasks
	groupIdx := session.GroupDecision.GroupIndex

	// Reset retry states and remove from failed list
	c.mu.Lock()
	for _, taskID := range failedTasks {
		// Reset retry counter
		if state, ok := session.TaskRetries[taskID]; ok {
			state.RetryCount = 0
		}
		// Remove from failed list
		newFailed := make([]string, 0)
		for _, ft := range session.FailedTasks {
			if ft != taskID {
				newFailed = append(newFailed, ft)
			}
		}
		session.FailedTasks = newFailed
		// Remove from completed list (in case it was there with 0 commits)
		newCompleted := make([]string, 0)
		for _, ct := range session.CompletedTasks {
			if ct != taskID {
				newCompleted = append(newCompleted, ct)
			}
		}
		session.CompletedTasks = newCompleted
		// Remove from TaskToInstance so they become ready again
		delete(session.TaskToInstance, taskID)
	}

	// Roll back group advancement
	session.CurrentGroup = groupIdx
	session.GroupDecision = nil
	c.mu.Unlock()

	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventGroupComplete,
		Message: fmt.Sprintf("Retrying %d failed tasks in group %d", len(failedTasks), groupIdx+1),
	})

	_ = c.orch.SaveSession()
	return nil
}

// consolidateGroupWithVerification consolidates a group and verifies commits exist
func (c *Coordinator) consolidateGroupWithVerification(groupIndex int) error {
	session := c.Session()
	if session == nil || session.Plan == nil {
		return fmt.Errorf("no session or plan")
	}

	if groupIndex < 0 || groupIndex >= len(session.Plan.ExecutionOrder) {
		return fmt.Errorf("invalid group index: %d", groupIndex)
	}

	taskIDs := session.Plan.ExecutionOrder[groupIndex]
	if len(taskIDs) == 0 {
		return nil // Empty group, nothing to consolidate
	}

	// Collect task branches for this group, filtering to only those with verified commits
	var taskBranches []string
	var activeTasks []string

	for _, taskID := range taskIDs {
		// Skip tasks that failed or have no commits
		commitCount, ok := session.TaskCommitCounts[taskID]
		if !ok || commitCount == 0 {
			continue
		}

		task := session.GetTask(taskID)
		if task == nil {
			continue
		}

		// Find the instance that executed this task
		for _, inst := range c.baseSession.Instances {
			if strings.Contains(inst.Task, taskID) || strings.Contains(inst.Branch, slugify(task.Title)) {
				taskBranches = append(taskBranches, inst.Branch)
				activeTasks = append(activeTasks, taskID)
				break
			}
		}
	}

	if len(taskBranches) == 0 {
		// No branches with work - this is an error now, not silent success
		return fmt.Errorf("no task branches with verified commits found for group %d", groupIndex)
	}

	// Generate consolidated branch name
	branchPrefix := session.Config.BranchPrefix
	if branchPrefix == "" {
		branchPrefix = c.orch.config.Branch.Prefix
	}
	if branchPrefix == "" {
		branchPrefix = "Iron-Ham"
	}
	planID := session.ID
	if len(planID) > 8 {
		planID = planID[:8]
	}
	consolidatedBranch := fmt.Sprintf("%s/ultraplan-%s-group-%d", branchPrefix, planID, groupIndex+1)

	// Determine base branch
	var baseBranch string
	if groupIndex == 0 {
		baseBranch = c.orch.wt.FindMainBranch()
	} else if groupIndex-1 < len(session.GroupConsolidatedBranches) {
		baseBranch = session.GroupConsolidatedBranches[groupIndex-1]
	} else {
		baseBranch = c.orch.wt.FindMainBranch()
	}

	// Create the consolidated branch from the base
	if err := c.orch.wt.CreateBranchFrom(consolidatedBranch, baseBranch); err != nil {
		return fmt.Errorf("failed to create consolidated branch %s: %w", consolidatedBranch, err)
	}

	// Create a temporary worktree for cherry-picking
	worktreeBase := fmt.Sprintf("%s/consolidation-group-%d", c.orch.claudioDir, groupIndex)
	if err := c.orch.wt.CreateWorktreeFromBranch(worktreeBase, consolidatedBranch); err != nil {
		return fmt.Errorf("failed to create consolidation worktree: %w", err)
	}
	defer func() {
		_ = c.orch.wt.Remove(worktreeBase)
	}()

	// Cherry-pick commits from each task branch - failures are now blocking
	for i, branch := range taskBranches {
		if err := c.orch.wt.CherryPickBranch(worktreeBase, branch); err != nil {
			// Cherry-pick failed - this is now a blocking error
			_ = c.orch.wt.AbortCherryPick(worktreeBase)
			return fmt.Errorf("failed to cherry-pick task %s (branch %s): %w", activeTasks[i], branch, err)
		}
	}

	// Verify the consolidated branch has commits
	consolidatedCommitCount, err := c.orch.wt.CountCommitsBetween(worktreeBase, baseBranch, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to verify consolidated branch commits: %w", err)
	}

	if consolidatedCommitCount == 0 {
		return fmt.Errorf("consolidated branch has no commits after cherry-picking %d branches", len(taskBranches))
	}

	// Push the consolidated branch
	if err := c.orch.wt.Push(worktreeBase, false); err != nil {
		c.manager.emitEvent(CoordinatorEvent{
			Type:    EventGroupComplete,
			Message: fmt.Sprintf("Warning: failed to push consolidated branch %s: %v", consolidatedBranch, err),
		})
		// Not fatal - branch exists locally
	}

	// Store the consolidated branch
	c.mu.Lock()
	for len(session.GroupConsolidatedBranches) <= groupIndex {
		session.GroupConsolidatedBranches = append(session.GroupConsolidatedBranches, "")
	}
	session.GroupConsolidatedBranches[groupIndex] = consolidatedBranch
	c.mu.Unlock()

	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventGroupComplete,
		Message: fmt.Sprintf("Group %d consolidated into %s (%d commits from %d tasks)", groupIndex+1, consolidatedBranch, consolidatedCommitCount, len(taskBranches)),
	})

	return nil
}

// consolidateGroup consolidates all task branches from a completed group into a single branch.
// This runs after each group completes, allowing the next group's tasks to build on the consolidated work.
// If the group has only one task, the task's branch becomes the consolidated branch directly.
func (c *Coordinator) consolidateGroup(groupIndex int) error {
	session := c.Session()
	if session == nil || session.Plan == nil {
		return fmt.Errorf("no session or plan")
	}

	if groupIndex < 0 || groupIndex >= len(session.Plan.ExecutionOrder) {
		return fmt.Errorf("invalid group index: %d", groupIndex)
	}

	taskIDs := session.Plan.ExecutionOrder[groupIndex]
	if len(taskIDs) == 0 {
		return nil // Empty group, nothing to consolidate
	}

	// Collect task branches for this group
	var taskBranches []string
	for _, taskID := range taskIDs {
		task := session.GetTask(taskID)
		if task == nil {
			continue
		}

		// Find the instance that executed this task
		for _, inst := range c.baseSession.Instances {
			if strings.Contains(inst.Task, taskID) || strings.Contains(inst.Branch, slugify(task.Title)) {
				taskBranches = append(taskBranches, inst.Branch)
				break
			}
		}
	}

	if len(taskBranches) == 0 {
		return fmt.Errorf("no task branches found for group %d", groupIndex)
	}

	// Generate consolidated branch name
	branchPrefix := session.Config.BranchPrefix
	if branchPrefix == "" {
		branchPrefix = c.orch.config.Branch.Prefix
	}
	if branchPrefix == "" {
		branchPrefix = "Iron-Ham"
	}
	planID := session.ID
	if len(planID) > 8 {
		planID = planID[:8]
	}
	consolidatedBranch := fmt.Sprintf("%s/ultraplan-%s-group-%d", branchPrefix, planID, groupIndex+1)

	// If there's only one task in the group, we can just use its branch as the base for the next group
	// But we still create a consolidated branch to maintain consistent naming
	var baseBranch string
	if groupIndex == 0 {
		baseBranch = c.orch.wt.FindMainBranch()
	} else if groupIndex-1 < len(session.GroupConsolidatedBranches) {
		baseBranch = session.GroupConsolidatedBranches[groupIndex-1]
	} else {
		baseBranch = c.orch.wt.FindMainBranch()
	}

	// Create the consolidated branch from the base
	if err := c.orch.wt.CreateBranchFrom(consolidatedBranch, baseBranch); err != nil {
		return fmt.Errorf("failed to create consolidated branch %s: %w", consolidatedBranch, err)
	}

	// Create a temporary worktree for cherry-picking
	worktreeBase := fmt.Sprintf("%s/consolidation-group-%d", c.orch.claudioDir, groupIndex)
	if err := c.orch.wt.CreateWorktreeFromBranch(worktreeBase, consolidatedBranch); err != nil {
		return fmt.Errorf("failed to create consolidation worktree: %w", err)
	}
	defer func() {
		// Clean up the worktree when done
		_ = c.orch.wt.Remove(worktreeBase)
	}()

	// Cherry-pick commits from each task branch
	for _, branch := range taskBranches {
		if err := c.orch.wt.CherryPickBranch(worktreeBase, branch); err != nil {
			// If cherry-pick fails, log but continue - the user may need to resolve conflicts later
			c.manager.emitEvent(CoordinatorEvent{
				Type:    EventConflict,
				Message: fmt.Sprintf("Failed to cherry-pick from %s: %v", branch, err),
			})
			// Try to abort the cherry-pick and continue
			_ = c.orch.wt.AbortCherryPick(worktreeBase)
			continue
		}
	}

	// Push the consolidated branch so it's available for remote operations
	if err := c.orch.wt.Push(worktreeBase, false); err != nil {
		// Push failure is not fatal - the branch exists locally
		c.manager.emitEvent(CoordinatorEvent{
			Type:    EventGroupComplete,
			Message: fmt.Sprintf("Warning: failed to push consolidated branch %s: %v", consolidatedBranch, err),
		})
	}

	// Store the consolidated branch
	c.mu.Lock()
	// Ensure the slice is large enough
	for len(session.GroupConsolidatedBranches) <= groupIndex {
		session.GroupConsolidatedBranches = append(session.GroupConsolidatedBranches, "")
	}
	session.GroupConsolidatedBranches[groupIndex] = consolidatedBranch
	c.mu.Unlock()

	// Persist the state
	_ = c.orch.SaveSession()

	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventGroupComplete,
		Message: fmt.Sprintf("Group %d consolidated into branch %s", groupIndex+1, consolidatedBranch),
	})

	return nil
}

// getBaseBranchForGroup returns the base branch that new tasks in a group should use.
// For group 0, this is the main branch. For other groups, it's the consolidated branch from the previous group.
func (c *Coordinator) getBaseBranchForGroup(groupIndex int) string {
	session := c.Session()

	if groupIndex == 0 {
		return "" // Use default (HEAD/main)
	}

	// Check if we have a consolidated branch from the previous group
	previousGroupIndex := groupIndex - 1
	if session != nil && previousGroupIndex < len(session.GroupConsolidatedBranches) {
		consolidatedBranch := session.GroupConsolidatedBranches[previousGroupIndex]
		if consolidatedBranch != "" {
			return consolidatedBranch
		}
	}

	return "" // Use default
}
