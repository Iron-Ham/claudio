package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
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
	logger      *logging.Logger

	// Running state
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex

	// Task tracking
	runningTasks map[string]string // taskID -> instanceID
	runningCount int
}

// NewCoordinator creates a new coordinator for an ultra-plan session.
// The logger parameter is optional; if nil, a no-op logger will be used.
func NewCoordinator(orch *Orchestrator, baseSession *Session, ultraSession *UltraPlanSession, logger *logging.Logger) *Coordinator {
	// Use NopLogger if no logger provided (needed before we create sessionLogger below)
	if logger == nil {
		logger = logging.NopLogger()
	}
	manager := NewUltraPlanManager(orch, baseSession, ultraSession, logger)

	ctx, cancel := context.WithCancel(context.Background())

	// Add session context to logger
	sessionLogger := logger.WithSession(ultraSession.ID).WithPhase("coordinator")

	return &Coordinator{
		manager:      manager,
		orch:         orch,
		baseSession:  baseSession,
		logger:       sessionLogger,
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

// StoreCandidatePlan stores a candidate plan at the given index with proper mutex protection.
// Returns the count of non-nil plans collected so far.
func (c *Coordinator) StoreCandidatePlan(planIndex int, plan *PlanSpec) int {
	return c.manager.StoreCandidatePlan(planIndex, plan)
}

// CountCandidatePlans returns the number of non-nil candidate plans collected.
func (c *Coordinator) CountCandidatePlans() int {
	return c.manager.CountCandidatePlans()
}

// CountCoordinatorsCompleted returns the number of coordinators that have completed
// (regardless of whether they produced a valid plan or not).
func (c *Coordinator) CountCoordinatorsCompleted() int {
	return c.manager.CountCoordinatorsCompleted()
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
	// Get the previous phase for logging
	session := c.Session()
	fromPhase := PhasePlanning
	if session != nil {
		fromPhase = session.Phase
	}

	c.manager.SetPhase(phase)

	// Log the phase transition
	c.logger.Info("phase changed",
		"from_phase", string(fromPhase),
		"to_phase", string(phase),
		"session_id", session.ID,
	)

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

	// Get task title for logging
	session := c.Session()
	taskTitle := ""
	if session != nil {
		if task := session.GetTask(taskID); task != nil {
			taskTitle = task.Title
		}
	}

	// Log task started
	c.logger.Info("task started",
		"task_id", taskID,
		"instance_id", instanceID,
		"task_title", taskTitle,
	)

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

	// Log task completed
	// Note: duration tracking requires instance start time, which could be added in the future
	c.logger.Info("task completed",
		"task_id", taskID,
	)

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

	// Log task failed
	c.logger.Info("task failed",
		"task_id", taskID,
		"reason", reason,
	)

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
	// Log plan ready
	taskCount := 0
	groupCount := 0
	if plan != nil {
		taskCount = len(plan.Tasks)
		groupCount = len(plan.ExecutionOrder)
	}
	c.logger.Info("plan ready",
		"task_count", taskCount,
		"group_count", groupCount,
	)

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

	completed := len(session.CompletedTasks)
	total := len(session.Plan.Tasks)

	// Log progress update at DEBUG level
	c.logger.Debug("progress update",
		"completed", completed,
		"total", total,
		"phase", string(session.Phase),
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnProgress != nil {
		cb.OnProgress(completed, total, session.Phase)
	}
}

// notifyComplete notifies callbacks of completion
func (c *Coordinator) notifyComplete(success bool, summary string) {
	// Log coordinator complete
	c.logger.Info("coordinator complete",
		"success", success,
		"summary", summary,
	)

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

	// Check if multi-pass planning is enabled
	if session.Config.MultiPass {
		return c.RunMultiPassPlanning()
	}

	c.notifyPhaseChange(PhasePlanning)

	// Create the planning prompt
	prompt := fmt.Sprintf(PlanningPromptTemplate, session.Objective)

	// Create a coordinator instance for planning
	inst, err := c.orch.AddInstance(c.baseSession, prompt)
	if err != nil {
		c.logger.Error("planning failed",
			"error", err.Error(),
			"stage", "create_instance",
		)
		return fmt.Errorf("failed to create planning instance: %w", err)
	}

	session.CoordinatorID = inst.ID

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		c.logger.Error("planning failed",
			"error", err.Error(),
			"stage", "start_instance",
		)
		return fmt.Errorf("failed to start planning instance: %w", err)
	}

	// Wait for the instance to complete
	// The TUI will handle monitoring; here we just set up the session state
	return nil
}

// RunMultiPassPlanning executes the multi-pass planning phase
// This creates three coordinator instances in parallel, each using a different
// planning strategy. The TUI monitors these instances and the coordinator-manager
// will later select or merge the best plan.
func (c *Coordinator) RunMultiPassPlanning() error {
	session := c.Session()
	c.notifyPhaseChange(PhasePlanning)

	// Get the available strategy names
	strategies := GetMultiPassStrategyNames()
	if len(strategies) == 0 {
		c.logger.Error("planning failed",
			"error", "no multi-pass planning strategies available",
			"stage", "get_strategies",
		)
		return fmt.Errorf("no multi-pass planning strategies available")
	}

	// Initialize the PlanCoordinatorIDs slice
	session.PlanCoordinatorIDs = make([]string, 0, len(strategies))

	// Create and start an instance for each strategy in parallel
	for i, strategy := range strategies {
		// Build the strategy-specific prompt
		prompt := GetMultiPassPlanningPrompt(strategy, session.Objective)

		// Create a coordinator instance for this strategy
		inst, err := c.orch.AddInstance(c.baseSession, prompt)
		if err != nil {
			c.logger.Error("planning failed",
				"error", err.Error(),
				"strategy", strategy,
				"stage", "create_instance",
			)
			return fmt.Errorf("failed to create planning instance for strategy %s: %w", strategy, err)
		}

		// Store the instance ID
		session.PlanCoordinatorIDs = append(session.PlanCoordinatorIDs, inst.ID)

		// Start the instance
		if err := c.orch.StartInstance(inst); err != nil {
			c.logger.Error("planning failed",
				"error", err.Error(),
				"strategy", strategy,
				"stage", "start_instance",
			)
			return fmt.Errorf("failed to start planning instance for strategy %s: %w", strategy, err)
		}

		// Emit event for this planning instance
		c.manager.emitEvent(CoordinatorEvent{
			Type:      EventMultiPassPlanGenerated,
			Message:   fmt.Sprintf("Started planning with strategy: %s", strategy),
			PlanIndex: i,
			Strategy:  strategy,
		})
	}

	// Persist the session state with all coordinator IDs
	_ = c.orch.SaveSession()

	// Return without blocking - TUI will monitor completion of all instances
	return nil
}

// RunPlanManager starts the plan manager (coordinator-manager) for multi-pass planning.
// This is called after all candidate plans have been collected from the parallel planning coordinators.
// The plan manager evaluates all plans and either selects the best one or merges them.
func (c *Coordinator) RunPlanManager() error {
	session := c.Session()

	// Validate multi-pass mode and collected plans
	if !session.Config.MultiPass {
		return fmt.Errorf("RunPlanManager called but MultiPass mode is not enabled")
	}

	if len(session.CandidatePlans) < len(session.PlanCoordinatorIDs) {
		return fmt.Errorf("not all candidate plans collected: have %d, need %d",
			len(session.CandidatePlans), len(session.PlanCoordinatorIDs))
	}

	// Verify all plans are non-nil
	for i, plan := range session.CandidatePlans {
		if plan == nil {
			return fmt.Errorf("candidate plan at index %d is nil", i)
		}
	}

	// Transition to plan selection phase
	c.notifyPhaseChange(PhasePlanSelection)

	// Build the plan manager prompt with all candidate plans
	prompt := c.buildPlanManagerPrompt()

	// Create the plan manager instance
	inst, err := c.orch.AddInstance(c.baseSession, prompt)
	if err != nil {
		return fmt.Errorf("failed to create plan manager instance: %w", err)
	}

	session.PlanManagerID = inst.ID

	// Persist the state
	_ = c.orch.SaveSession()

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start plan manager instance: %w", err)
	}

	// The TUI will handle monitoring and parsing the final plan from the manager
	return nil
}

// buildPlanManagerPrompt constructs the prompt for the plan manager
// It includes all candidate plans formatted for comparison
func (c *Coordinator) buildPlanManagerPrompt() string {
	session := c.Session()
	var plansSection strings.Builder

	strategyNames := GetMultiPassStrategyNames()
	for i, plan := range session.CandidatePlans {
		strategyName := "unknown"
		if i < len(strategyNames) {
			strategyName = strategyNames[i]
		}

		plansSection.WriteString(fmt.Sprintf("\n### Plan %d: %s Strategy\n\n", i+1, strategyName))
		plansSection.WriteString(fmt.Sprintf("**Summary:** %s\n\n", plan.Summary))
		plansSection.WriteString(fmt.Sprintf("**Tasks (%d total):**\n", len(plan.Tasks)))
		for _, task := range plan.Tasks {
			deps := "none"
			if len(task.DependsOn) > 0 {
				deps = strings.Join(task.DependsOn, ", ")
			}
			plansSection.WriteString(fmt.Sprintf("- [%s] %s (complexity: %s, depends: %s)\n",
				task.ID, task.Title, task.EstComplexity, deps))
		}
		plansSection.WriteString(fmt.Sprintf("\n**Execution Groups:** %d parallel groups\n", len(plan.ExecutionOrder)))
		for groupIdx, group := range plan.ExecutionOrder {
			plansSection.WriteString(fmt.Sprintf("  - Group %d: %s\n", groupIdx+1, strings.Join(group, ", ")))
		}

		if len(plan.Insights) > 0 {
			plansSection.WriteString("\n**Insights:**\n")
			for _, insight := range plan.Insights {
				plansSection.WriteString(fmt.Sprintf("- %s\n", insight))
			}
		}

		if len(plan.Constraints) > 0 {
			plansSection.WriteString("\n**Constraints:**\n")
			for _, constraint := range plan.Constraints {
				plansSection.WriteString(fmt.Sprintf("- %s\n", constraint))
			}
		}
		plansSection.WriteString("\n---\n")
	}

	return fmt.Sprintf(PlanManagerPromptTemplate, session.Objective, plansSection.String())
}

// buildPlanComparisonSection formats all candidate plans for comparison by the plan manager.
// Each plan includes its strategy name, summary, and full task list in JSON format.
func (c *Coordinator) buildPlanComparisonSection() string {
	session := c.Session()
	var sb strings.Builder

	strategies := GetMultiPassStrategyNames()

	for i, plan := range session.CandidatePlans {
		if plan == nil {
			continue
		}

		// Determine strategy name (use index if out of bounds)
		strategyName := fmt.Sprintf("strategy-%d", i+1)
		if i < len(strategies) {
			strategyName = strategies[i]
		}

		sb.WriteString(fmt.Sprintf("### Plan %d: %s\n\n", i+1, strategyName))
		sb.WriteString(fmt.Sprintf("**Summary**: %s\n\n", plan.Summary))

		// Task count and parallelism stats
		sb.WriteString(fmt.Sprintf("**Task Count**: %d tasks\n", len(plan.Tasks)))
		if len(plan.ExecutionOrder) > 0 {
			sb.WriteString(fmt.Sprintf("**Execution Groups**: %d groups\n", len(plan.ExecutionOrder)))
			// Calculate max parallelism (largest group)
			maxParallel := 0
			for _, group := range plan.ExecutionOrder {
				if len(group) > maxParallel {
					maxParallel = len(group)
				}
			}
			sb.WriteString(fmt.Sprintf("**Max Parallelism**: %d concurrent tasks\n", maxParallel))
		}
		sb.WriteString("\n")

		// Insights
		if len(plan.Insights) > 0 {
			sb.WriteString("**Insights**:\n")
			for _, insight := range plan.Insights {
				sb.WriteString(fmt.Sprintf("- %s\n", insight))
			}
			sb.WriteString("\n")
		}

		// Constraints
		if len(plan.Constraints) > 0 {
			sb.WriteString("**Constraints**:\n")
			for _, constraint := range plan.Constraints {
				sb.WriteString(fmt.Sprintf("- %s\n", constraint))
			}
			sb.WriteString("\n")
		}

		// Full task list in JSON format for detailed comparison
		sb.WriteString("**Tasks (JSON)**:\n```json\n")
		tasksJSON, err := json.MarshalIndent(plan.Tasks, "", "  ")
		if err != nil {
			sb.WriteString(fmt.Sprintf("Error marshaling tasks: %v\n", err))
		} else {
			sb.WriteString(string(tasksJSON))
		}
		sb.WriteString("\n```\n\n")

		// Execution order visualization
		if len(plan.ExecutionOrder) > 0 {
			sb.WriteString("**Execution Order**:\n")
			for groupIdx, group := range plan.ExecutionOrder {
				sb.WriteString(fmt.Sprintf("- Group %d: %s\n", groupIdx+1, strings.Join(group, ", ")))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---\n\n")
	}

	return sb.String()
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
			// Fallback: poll for task completions that monitoring goroutines may have missed
			// This catches cases where goroutines exit early (context cancellation) or fail to send
			c.pollTaskCompletions(completionChan)

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

			// Check if we can start more tasks (MaxParallel <= 0 means unlimited)
			if config.MaxParallel <= 0 || runningCount < config.MaxParallel {
				readyTasks := session.GetReadyTasks()
				for _, taskID := range readyTasks {
					c.mu.RLock()
					currentRunning := c.runningCount
					c.mu.RUnlock()

					if config.MaxParallel > 0 && currentRunning >= config.MaxParallel {
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
		c.logger.Error("task execution failed",
			"task_id", taskID,
			"error", err.Error(),
			"stage", "create_instance",
		)
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
		c.logger.Error("task execution failed",
			"task_id", taskID,
			"error", err.Error(),
			"stage", "start_instance",
		)
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

	// Add context from previous group's consolidator if this task is not in group 0
	groupIndex := c.getTaskGroupIndex(task.ID)
	if groupIndex > 0 {
		prevGroupIdx := groupIndex - 1
		if prevGroupIdx < len(session.GroupConsolidationContexts) && session.GroupConsolidationContexts[prevGroupIdx] != nil {
			prevContext := session.GroupConsolidationContexts[prevGroupIdx]
			sb.WriteString("## Context from Previous Group\n\n")
			sb.WriteString(fmt.Sprintf("This task builds on work consolidated from Group %d.\n\n", prevGroupIdx+1))

			if prevContext.Notes != "" {
				sb.WriteString(fmt.Sprintf("**Consolidator Notes**: %s\n\n", prevContext.Notes))
			}

			if len(prevContext.IssuesForNextGroup) > 0 {
				sb.WriteString("**Important**: The previous group's consolidator flagged these issues:\n")
				for _, issue := range prevContext.IssuesForNextGroup {
					sb.WriteString(fmt.Sprintf("- %s\n", issue))
				}
				sb.WriteString("\n")
			}

			if prevContext.Verification.OverallSuccess {
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

// getTaskGroupIndex returns the group index for a given task ID, or -1 if not found
func (c *Coordinator) getTaskGroupIndex(taskID string) int {
	session := c.Session()
	if session == nil || session.Plan == nil {
		return -1
	}

	for groupIdx, group := range session.Plan.ExecutionOrder {
		for _, tid := range group {
			if tid == taskID {
				return groupIdx
			}
		}
	}
	return -1
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
				c.logger.Debug("instance status check",
					"task_id", taskID,
					"instance_id", instanceID,
					"status", "not_found",
				)
				completionChan <- taskCompletion{
					taskID:     taskID,
					instanceID: instanceID,
					success:    false,
					error:      "instance not found",
				}
				return
			}

			// Log instance status check at DEBUG level (only when status changes would be interesting)
			c.logger.Debug("instance status check",
				"task_id", taskID,
				"instance_id", instanceID,
				"status", string(inst.Status),
			)

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
				// StatusCompleted can be triggered by false positive pattern detection
				// while the instance is still actively working. Only treat as actual
				// completion if the tmux session has truly exited.
				mgr := c.orch.GetInstanceManager(instanceID)
				if mgr != nil && mgr.TmuxSessionExists() {
					// Tmux session still running - this was a false positive completion detection
					// Reset status to working and continue monitoring for sentinel file
					inst.Status = StatusWorking
					continue
				}
				// Tmux session has exited - verify work was done
				result := c.verifyTaskWork(taskID, inst)
				completionChan <- result
				return

			// Note: StatusWaitingInput is intentionally NOT treated as completion.
			// The sentinel file (.claudio-task-complete.json) is the primary completion signal.
			// StatusWaitingInput can trigger too early from Claude Code's UI elements,
			// causing tasks to be marked failed before they complete their work.

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
// This checks for both regular task completion (.claudio-task-complete.json) and
// revision task completion (.claudio-revision-complete.json) since both use this monitor
func (c *Coordinator) checkForTaskCompletionFile(inst *Instance) bool {
	if inst.WorktreePath == "" {
		return false
	}

	// First check for regular task completion file
	taskCompletionPath := TaskCompletionFilePath(inst.WorktreePath)
	if _, err := os.Stat(taskCompletionPath); err == nil {
		// File exists - try to parse it to ensure it's valid
		completion, err := ParseTaskCompletionFile(inst.WorktreePath)
		if err == nil && completion.Status != "" {
			return true
		}
	}

	// Also check for revision completion file (revision tasks write this instead)
	revisionCompletionPath := RevisionCompletionFilePath(inst.WorktreePath)
	if _, err := os.Stat(revisionCompletionPath); err == nil {
		// File exists - try to parse it to ensure it's valid
		completion, err := ParseRevisionCompletionFile(inst.WorktreePath)
		if err == nil && completion.TaskID != "" {
			return true
		}
	}

	return false
}

// pollTaskCompletions scans all started tasks for completion files.
// This is a fallback mechanism to detect completions when monitoring goroutines
// exit early (e.g., due to context cancellation) or fail to send to the channel.
func (c *Coordinator) pollTaskCompletions(completionChan chan<- taskCompletion) {
	session := c.Session()
	if session == nil {
		return
	}

	c.mu.RLock()
	taskToInstance := session.TaskToInstance
	completedTasks := session.CompletedTasks
	failedTasks := session.FailedTasks
	c.mu.RUnlock()

	// Build set of already-finished tasks
	finished := make(map[string]bool)
	for _, t := range completedTasks {
		finished[t] = true
	}
	for _, t := range failedTasks {
		finished[t] = true
	}

	// Check each started task for completion
	for taskID, instanceID := range taskToInstance {
		if finished[taskID] {
			continue
		}

		inst := c.orch.GetInstance(instanceID)
		if inst == nil {
			continue
		}

		// Check for completion file
		if c.checkForTaskCompletionFile(inst) {
			// Stop instance to free resources
			_ = c.orch.StopInstance(inst)

			// Verify and report
			result := c.verifyTaskWork(taskID, inst)

			// Non-blocking send (skip if channel full, will retry next iteration)
			select {
			case completionChan <- result:
			default:
			}
		}
	}
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

	// CRITICAL FIX: Check if group is complete WITHOUT advancing CurrentGroup yet.
	// We must check for partial failure and complete consolidation BEFORE
	// incrementing CurrentGroup, otherwise GetReadyTasks() will return tasks
	// from the next group prematurely.
	c.mu.RLock()
	isComplete := session.IsCurrentGroupComplete()
	currentGroup := session.CurrentGroup
	c.mu.RUnlock()

	if !isComplete {
		return
	}

	// Check for partial group failure BEFORE advancing
	// This ensures CurrentGroup stays at the failed group index
	if c.hasPartialGroupFailure(currentGroup) {
		c.handlePartialGroupFailure(currentGroup)
		// Don't advance until user decides - CurrentGroup remains unchanged
		return
	}

	// Emit group complete event
	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventGroupComplete,
		Message: fmt.Sprintf("Group %d complete, consolidating before advancing to group %d", currentGroup+1, currentGroup+2),
	})

	// Start the group consolidator Claude session
	// This blocks until the consolidator completes (writes completion file)
	if err := c.startGroupConsolidatorSession(currentGroup); err != nil {
		c.manager.emitEvent(CoordinatorEvent{
			Type:    EventConflict,
			Message: fmt.Sprintf("Critical: failed to consolidate group %d: %v", currentGroup+1, err),
		})

		// Mark session as failed since we can't continue without consolidation
		c.mu.Lock()
		session.Phase = PhaseFailed
		session.Error = fmt.Sprintf("consolidation of group %d failed: %v", currentGroup+1, err)
		c.mu.Unlock()
		_ = c.orch.SaveSession()
		c.notifyComplete(false, session.Error)
		return
	}

	// NOW advance to the next group - only after consolidation succeeds
	c.mu.Lock()
	session.CurrentGroup++
	c.mu.Unlock()

	// Log group completion
	taskCount := 0
	if currentGroup < len(session.Plan.ExecutionOrder) {
		taskCount = len(session.Plan.ExecutionOrder[currentGroup])
	}
	c.logger.Info("group completed",
		"group_index", currentGroup,
		"task_count", taskCount,
	)

	// Call the callback
	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()
	if cb != nil && cb.OnGroupComplete != nil {
		cb.OnGroupComplete(currentGroup)
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
		c.logger.Error("synthesis failed",
			"error", err.Error(),
			"stage", "create_instance",
		)
		return fmt.Errorf("failed to create synthesis instance: %w", err)
	}

	// Store the synthesis instance ID for TUI visibility
	session := c.Session()
	session.SynthesisID = inst.ID

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		c.logger.Error("synthesis failed",
			"error", err.Error(),
			"stage", "start_instance",
		)
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

			// Check for sentinel file first - this is the most reliable completion signal
			// The synthesis agent writes .claudio-synthesis-complete.json when done
			if c.checkForSynthesisCompletionFile(inst) {
				// Don't auto-advance - set flag and wait for user approval
				c.onSynthesisReady()
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

// checkForSynthesisCompletionFile checks if the synthesis completion sentinel file exists and is valid
func (c *Coordinator) checkForSynthesisCompletionFile(inst *Instance) bool {
	if inst.WorktreePath == "" {
		return false
	}

	completionPath := SynthesisCompletionFilePath(inst.WorktreePath)
	if _, err := os.Stat(completionPath); err != nil {
		return false // File doesn't exist yet
	}

	// File exists - try to parse it to ensure it's valid
	completion, err := ParseSynthesisCompletionFile(inst.WorktreePath)
	if err != nil {
		// File exists but is invalid/incomplete - might still be writing
		return false
	}

	// File is valid - check status is set
	return completion.Status != ""
}

// onSynthesisReady is called when synthesis writes its completion file
// Instead of auto-advancing, it sets a flag and waits for user approval
func (c *Coordinator) onSynthesisReady() {
	session := c.Session()

	// Parse and store synthesis completion data
	synthesisCompletion, _ := c.parseRevisionIssues()
	c.mu.Lock()
	if synthesisCompletion != nil {
		session.SynthesisCompletion = synthesisCompletion
	}
	session.SynthesisAwaitingApproval = true
	c.mu.Unlock()

	// Save session state
	_ = c.orch.SaveSession()

	// Notify that user input is needed (but don't advance)
	c.notifyPhaseChange(PhaseSynthesis)
}

// onSynthesisComplete handles synthesis completion and triggers revision or consolidation
func (c *Coordinator) onSynthesisComplete() {
	session := c.Session()

	// Try to parse synthesis completion from sentinel file (preferred) or stdout (fallback)
	synthesisCompletion, issues := c.parseRevisionIssues()

	// Store synthesis completion for later use in consolidation
	if synthesisCompletion != nil {
		c.mu.Lock()
		session.SynthesisCompletion = synthesisCompletion
		c.mu.Unlock()
	}

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

// parseRevisionIssues extracts revision issues from the synthesis completion file (preferred)
// or falls back to parsing stdout output. Returns the full completion struct (if available) and issues.
func (c *Coordinator) parseRevisionIssues() (*SynthesisCompletionFile, []RevisionIssue) {
	session := c.Session()
	if session.SynthesisID == "" {
		return nil, nil
	}

	inst := c.orch.GetInstance(session.SynthesisID)
	if inst == nil {
		return nil, nil
	}

	// First, try to read from the sentinel file (preferred method)
	if inst.WorktreePath != "" {
		completion, err := ParseSynthesisCompletionFile(inst.WorktreePath)
		if err == nil && completion != nil {
			// Successfully parsed sentinel file - return the full completion and issues
			return completion, completion.IssuesFound
		}
	}

	// Fallback: parse revision issues from stdout (legacy method)
	mgr := c.orch.instances[inst.ID]
	if mgr == nil {
		return nil, nil
	}

	outputBytes := mgr.GetOutput()
	if len(outputBytes) == 0 {
		return nil, nil
	}

	issues, err := ParseRevisionIssuesFromOutput(string(outputBytes))
	if err != nil {
		// Log but don't fail - just proceed without revision
		return nil, nil
	}

	return nil, issues
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
		c.logger.Error("revision failed",
			"task_id", taskID,
			"error", err.Error(),
			"stage", "create_instance",
		)
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
		c.logger.Error("revision failed",
			"task_id", taskID,
			"error", err.Error(),
			"stage", "start_instance",
		)
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

	// Get current revision round (default to 1 if not set)
	revisionRound := 1
	if session.Revision != nil {
		revisionRound = session.Revision.RevisionRound + 1
	}

	return fmt.Sprintf(RevisionPromptTemplate,
		session.Objective,
		task.ID,
		task.Title,
		task.Description,
		revisionRound,
		issuesStr.String(),
		task.ID,       // For completion file JSON
		revisionRound, // For completion file JSON
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

	// Clear the awaiting approval flag
	c.mu.Lock()
	session.SynthesisAwaitingApproval = false
	c.mu.Unlock()

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

	// Build synthesis context from the synthesis completion file
	var synthesisContext strings.Builder
	if session.SynthesisCompletion != nil {
		synthesisContext.WriteString(fmt.Sprintf("Status: %s\n", session.SynthesisCompletion.Status))
		if session.SynthesisCompletion.IntegrationNotes != "" {
			synthesisContext.WriteString(fmt.Sprintf("Integration Notes: %s\n", session.SynthesisCompletion.IntegrationNotes))
		}
		if len(session.SynthesisCompletion.Recommendations) > 0 {
			synthesisContext.WriteString("Recommendations:\n")
			for _, rec := range session.SynthesisCompletion.Recommendations {
				synthesisContext.WriteString(fmt.Sprintf("- %s\n", rec))
			}
		}
		if len(session.SynthesisCompletion.IssuesFound) > 0 {
			synthesisContext.WriteString(fmt.Sprintf("Issues Found: %d\n", len(session.SynthesisCompletion.IssuesFound)))
			for _, issue := range session.SynthesisCompletion.IssuesFound {
				synthesisContext.WriteString(fmt.Sprintf("- [%s] %s\n", issue.Severity, issue.Description))
			}
		}
	} else {
		synthesisContext.WriteString("No synthesis context available (synthesis may have used legacy mode)\n")
	}

	return fmt.Sprintf(ConsolidationPromptTemplate,
		session.Objective,
		branchPrefix,
		mainBranch,
		mode,
		createDrafts,
		groupsInfo.String(),
		worktreeInfo.String(),
		synthesisContext.String(),
		mode, // For completion file JSON
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

	// Get current revision round (0 for first synthesis)
	revisionRound := 0
	if session.Revision != nil {
		revisionRound = session.Revision.RevisionRound
	}

	return fmt.Sprintf(SynthesisPromptTemplate, session.Objective, taskList.String(), resultsSummary.String(), revisionRound)
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

	// Advance to the next group AFTER consolidation succeeds
	// This is critical - without this, checkAndAdvanceGroup() would detect
	// the partial failure again and re-prompt the user
	c.mu.Lock()
	session.CurrentGroup++
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

	// Ensure we stay at the current group (should already be at groupIdx)
	// and clear the decision state so tasks can be retried
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

// RetriggerGroup resets execution state to the specified group index and restarts execution.
// All state from groups >= targetGroup is cleared, since subsequent groups depend on the
// re-triggered group's consolidated branch.
//
// Preconditions:
//   - targetGroup must be >= 0 and < number of execution groups
//   - No tasks currently running (runningCount == 0)
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
func (c *Coordinator) RetriggerGroup(targetGroup int) error {
	session := c.Session()
	if session == nil || session.Plan == nil {
		return fmt.Errorf("no plan available")
	}

	// Validate target group
	numGroups := len(session.Plan.ExecutionOrder)
	if targetGroup < 0 || targetGroup >= numGroups {
		return fmt.Errorf("invalid target group %d (must be 0-%d)", targetGroup, numGroups-1)
	}

	// Check we're not currently executing tasks
	c.mu.RLock()
	runningCount := c.runningCount
	awaitingDecision := session.GroupDecision != nil && session.GroupDecision.AwaitingDecision
	c.mu.RUnlock()

	if runningCount > 0 {
		return fmt.Errorf("cannot retrigger while %d tasks are running", runningCount)
	}

	if awaitingDecision {
		return fmt.Errorf("cannot retrigger while awaiting group decision")
	}

	// Build set of tasks in groups >= targetGroup
	tasksToReset := make(map[string]bool)
	for groupIdx := targetGroup; groupIdx < numGroups; groupIdx++ {
		for _, taskID := range session.Plan.ExecutionOrder[groupIdx] {
			tasksToReset[taskID] = true
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear CompletedTasks for affected tasks
	newCompleted := make([]string, 0)
	for _, taskID := range session.CompletedTasks {
		if !tasksToReset[taskID] {
			newCompleted = append(newCompleted, taskID)
		}
	}
	session.CompletedTasks = newCompleted

	// Clear FailedTasks for affected tasks
	newFailed := make([]string, 0)
	for _, taskID := range session.FailedTasks {
		if !tasksToReset[taskID] {
			newFailed = append(newFailed, taskID)
		}
	}
	session.FailedTasks = newFailed

	// Clear task-related maps for affected tasks
	for taskID := range tasksToReset {
		delete(session.TaskToInstance, taskID)
		delete(session.TaskRetries, taskID)
		delete(session.TaskCommitCounts, taskID)
	}

	// Truncate group-related slices
	if targetGroup < len(session.GroupConsolidatedBranches) {
		session.GroupConsolidatedBranches = session.GroupConsolidatedBranches[:targetGroup]
	}
	if targetGroup < len(session.GroupConsolidatorIDs) {
		session.GroupConsolidatorIDs = session.GroupConsolidatorIDs[:targetGroup]
	}
	if targetGroup < len(session.GroupConsolidationContexts) {
		session.GroupConsolidationContexts = session.GroupConsolidationContexts[:targetGroup]
	}

	// Reset CurrentGroup
	session.CurrentGroup = targetGroup

	// Reset phase to executing
	session.Phase = PhaseExecuting
	session.GroupDecision = nil
	session.Error = ""

	// Clear synthesis/revision/consolidation state
	session.SynthesisID = ""
	session.SynthesisCompletion = nil
	session.SynthesisAwaitingApproval = false
	session.Revision = nil
	session.RevisionID = ""
	session.Consolidation = nil
	session.ConsolidationID = ""
	session.PRUrls = nil

	// Log the retrigger
	c.logger.Info("group retriggered",
		"target_group", targetGroup,
		"tasks_reset", len(tasksToReset),
	)

	// Persist the state - log error but don't fail the operation since state is already modified
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist retrigger state",
			"target_group", targetGroup,
			"error", err.Error(),
		)
	}

	// Emit event
	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventPhaseChange,
		Message: fmt.Sprintf("Retriggered from group %d", targetGroup),
	})

	// Restart execution loop
	c.wg.Add(1)
	go c.executionLoop()

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

// ============================================================================
// Per-Group Consolidator Session Management
// ============================================================================

// gatherTaskCompletionContextForGroup reads completion files from all completed tasks in a group
// and aggregates the context for the group consolidator
func (c *Coordinator) gatherTaskCompletionContextForGroup(groupIndex int) *AggregatedTaskContext {
	session := c.Session()
	if session == nil || session.Plan == nil || groupIndex >= len(session.Plan.ExecutionOrder) {
		return &AggregatedTaskContext{TaskSummaries: make(map[string]string)}
	}

	taskIDs := session.Plan.ExecutionOrder[groupIndex]
	context := &AggregatedTaskContext{
		TaskSummaries:  make(map[string]string),
		AllIssues:      make([]string, 0),
		AllSuggestions: make([]string, 0),
		Dependencies:   make([]string, 0),
		Notes:          make([]string, 0),
	}

	seenDeps := make(map[string]bool)

	for _, taskID := range taskIDs {
		// Find the instance for this task
		var inst *Instance
		for _, i := range c.baseSession.Instances {
			if strings.Contains(i.Task, taskID) {
				inst = i
				break
			}
		}
		if inst == nil || inst.WorktreePath == "" {
			continue
		}

		// Try to read the completion file
		completion, err := ParseTaskCompletionFile(inst.WorktreePath)
		if err != nil {
			continue // No completion file or invalid
		}

		// Store task summary
		context.TaskSummaries[taskID] = completion.Summary

		// Aggregate issues (prefix with task ID for context)
		for _, issue := range completion.Issues {
			if issue != "" {
				context.AllIssues = append(context.AllIssues, fmt.Sprintf("[%s] %s", taskID, issue))
			}
		}

		// Aggregate suggestions
		for _, suggestion := range completion.Suggestions {
			if suggestion != "" {
				context.AllSuggestions = append(context.AllSuggestions, fmt.Sprintf("[%s] %s", taskID, suggestion))
			}
		}

		// Aggregate dependencies (deduplicated)
		for _, dep := range completion.Dependencies {
			if dep != "" && !seenDeps[dep] {
				seenDeps[dep] = true
				context.Dependencies = append(context.Dependencies, dep)
			}
		}

		// Collect notes
		if completion.Notes != "" {
			context.Notes = append(context.Notes, fmt.Sprintf("**%s**: %s", taskID, completion.Notes))
		}
	}

	return context
}

// getTaskBranchesForGroup returns the branches and commit counts for all tasks in a group
func (c *Coordinator) getTaskBranchesForGroup(groupIndex int) []TaskWorktreeInfo {
	session := c.Session()
	if session == nil || session.Plan == nil || groupIndex >= len(session.Plan.ExecutionOrder) {
		return nil
	}

	taskIDs := session.Plan.ExecutionOrder[groupIndex]
	var branches []TaskWorktreeInfo

	for _, taskID := range taskIDs {
		task := session.GetTask(taskID)
		if task == nil {
			continue
		}

		// Find the instance for this task
		for _, inst := range c.baseSession.Instances {
			if strings.Contains(inst.Task, taskID) {
				branches = append(branches, TaskWorktreeInfo{
					TaskID:       taskID,
					TaskTitle:    task.Title,
					WorktreePath: inst.WorktreePath,
					Branch:       inst.Branch,
				})
				break
			}
		}
	}

	return branches
}

// buildGroupConsolidatorPrompt builds the prompt for a per-group consolidator session
func (c *Coordinator) buildGroupConsolidatorPrompt(groupIndex int) string {
	session := c.Session()
	if session == nil || session.Plan == nil {
		return ""
	}

	taskContext := c.gatherTaskCompletionContextForGroup(groupIndex)
	taskBranches := c.getTaskBranchesForGroup(groupIndex)

	// Determine base branch
	var baseBranch string
	if groupIndex == 0 {
		baseBranch = c.orch.wt.FindMainBranch()
	} else if groupIndex-1 < len(session.GroupConsolidatedBranches) {
		baseBranch = session.GroupConsolidatedBranches[groupIndex-1]
	} else {
		baseBranch = c.orch.wt.FindMainBranch()
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

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Group %d Consolidation\n\n", groupIndex+1))
	sb.WriteString(fmt.Sprintf("## Part of Ultra-Plan: %s\n\n", session.Plan.Summary))

	sb.WriteString("## Objective\n\n")
	sb.WriteString("Consolidate all completed task branches from this group into a single stable branch.\n")
	sb.WriteString("You must resolve any merge conflicts, verify the consolidated code works, and pass context to the next group.\n\n")

	// Tasks completed in this group
	sb.WriteString("## Tasks Completed in This Group\n\n")
	for _, branch := range taskBranches {
		sb.WriteString(fmt.Sprintf("### %s: %s\n", branch.TaskID, branch.TaskTitle))
		sb.WriteString(fmt.Sprintf("- Branch: `%s`\n", branch.Branch))
		sb.WriteString(fmt.Sprintf("- Worktree: `%s`\n", branch.WorktreePath))
		if summary, ok := taskContext.TaskSummaries[branch.TaskID]; ok && summary != "" {
			sb.WriteString(fmt.Sprintf("- Summary: %s\n", summary))
		}
		sb.WriteString("\n")
	}

	// Context from task completion files
	if len(taskContext.Notes) > 0 {
		sb.WriteString("## Implementation Notes from Tasks\n\n")
		for _, note := range taskContext.Notes {
			sb.WriteString(fmt.Sprintf("- %s\n", note))
		}
		sb.WriteString("\n")
	}

	if len(taskContext.AllIssues) > 0 {
		sb.WriteString("## Issues Raised by Tasks\n\n")
		for _, issue := range taskContext.AllIssues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		sb.WriteString("\n")
	}

	if len(taskContext.AllSuggestions) > 0 {
		sb.WriteString("## Integration Suggestions from Tasks\n\n")
		for _, suggestion := range taskContext.AllSuggestions {
			sb.WriteString(fmt.Sprintf("- %s\n", suggestion))
		}
		sb.WriteString("\n")
	}

	// Context from previous group's consolidator
	if groupIndex > 0 && groupIndex-1 < len(session.GroupConsolidationContexts) {
		prevContext := session.GroupConsolidationContexts[groupIndex-1]
		if prevContext != nil {
			sb.WriteString("## Context from Previous Group's Consolidator\n\n")
			if prevContext.Notes != "" {
				sb.WriteString(fmt.Sprintf("**Notes**: %s\n\n", prevContext.Notes))
			}
			if len(prevContext.IssuesForNextGroup) > 0 {
				sb.WriteString("**Issues/Warnings to Address**:\n")
				for _, issue := range prevContext.IssuesForNextGroup {
					sb.WriteString(fmt.Sprintf("- %s\n", issue))
				}
				sb.WriteString("\n")
			}
		}
	}

	// Branch configuration
	sb.WriteString("## Branch Configuration\n\n")
	sb.WriteString(fmt.Sprintf("- **Base branch**: `%s`\n", baseBranch))
	sb.WriteString(fmt.Sprintf("- **Target consolidated branch**: `%s`\n", consolidatedBranch))
	sb.WriteString(fmt.Sprintf("- **Task branches to consolidate**: %d\n\n", len(taskBranches)))

	// Instructions
	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. **Create the consolidated branch** from the base branch:\n")
	sb.WriteString(fmt.Sprintf("   ```bash\n   git checkout -b %s %s\n   ```\n\n", consolidatedBranch, baseBranch))

	sb.WriteString("2. **Cherry-pick commits** from each task branch in order. For each branch:\n")
	sb.WriteString("   - Review the commits on the branch\n")
	sb.WriteString("   - Cherry-pick them onto the consolidated branch\n")
	sb.WriteString("   - Resolve any conflicts intelligently using your understanding of the code\n\n")

	sb.WriteString("3. **Run verification** to ensure the consolidated code is stable:\n")
	sb.WriteString("   - Detect the project type (Go, Node, Python, iOS, etc.)\n")
	sb.WriteString("   - Run appropriate build/compile commands\n")
	sb.WriteString("   - Run linting if available\n")
	sb.WriteString("   - Run tests if available\n")
	sb.WriteString("   - Fix any issues that arise\n\n")

	sb.WriteString("4. **Push the consolidated branch** to the remote\n\n")

	sb.WriteString("5. **Write the completion file** to signal success\n\n")

	// Conflict resolution guidelines
	sb.WriteString("## Conflict Resolution Guidelines\n\n")
	sb.WriteString("- Prefer changes that preserve functionality from all tasks\n")
	sb.WriteString("- If there are conflicting approaches, choose the more robust one\n")
	sb.WriteString("- Document your resolution reasoning in the completion file\n")
	sb.WriteString("- If you cannot resolve a conflict, document it as an issue\n\n")

	// Completion protocol
	sb.WriteString("## Completion Protocol\n\n")
	sb.WriteString(fmt.Sprintf("When consolidation is complete, write `%s` in your worktree root:\n\n", GroupConsolidationCompletionFileName))
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString(fmt.Sprintf("  \"group_index\": %d,\n", groupIndex))
	sb.WriteString("  \"status\": \"complete\",\n")
	sb.WriteString(fmt.Sprintf("  \"branch_name\": \"%s\",\n", consolidatedBranch))
	sb.WriteString("  \"tasks_consolidated\": [\"task-id-1\", \"task-id-2\"],\n")
	sb.WriteString("  \"conflicts_resolved\": [\n")
	sb.WriteString("    {\"file\": \"path/to/file.go\", \"resolution\": \"Kept both changes, merged logic\"}\n")
	sb.WriteString("  ],\n")
	sb.WriteString("  \"verification\": {\n")
	sb.WriteString("    \"project_type\": \"go\",\n")
	sb.WriteString("    \"commands_run\": [\n")
	sb.WriteString("      {\"name\": \"build\", \"command\": \"go build ./...\", \"success\": true},\n")
	sb.WriteString("      {\"name\": \"lint\", \"command\": \"golangci-lint run\", \"success\": true},\n")
	sb.WriteString("      {\"name\": \"test\", \"command\": \"go test ./...\", \"success\": true}\n")
	sb.WriteString("    ],\n")
	sb.WriteString("    \"overall_success\": true,\n")
	sb.WriteString("    \"summary\": \"All checks passed\"\n")
	sb.WriteString("  },\n")
	sb.WriteString("  \"notes\": \"Any observations about the consolidated code\",\n")
	sb.WriteString("  \"issues_for_next_group\": [\"Any warnings or concerns for the next group\"]\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("Use status \"failed\" if consolidation cannot be completed.\n")

	return sb.String()
}

// startGroupConsolidatorSession creates and starts a Claude session for consolidating a group
func (c *Coordinator) startGroupConsolidatorSession(groupIndex int) error {
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

	// Check if there are any tasks with verified commits
	var activeTasks []string
	for _, taskID := range taskIDs {
		commitCount, ok := session.TaskCommitCounts[taskID]
		if ok && commitCount > 0 {
			activeTasks = append(activeTasks, taskID)
		}
	}

	if len(activeTasks) == 0 {
		return fmt.Errorf("no task branches with verified commits found for group %d", groupIndex)
	}

	// Build the consolidator prompt
	prompt := c.buildGroupConsolidatorPrompt(groupIndex)

	// Determine base branch for the consolidator's worktree
	baseBranch := c.getBaseBranchForGroup(groupIndex)

	// Create the consolidator instance
	var inst *Instance
	var err error
	if baseBranch != "" {
		inst, err = c.orch.AddInstanceFromBranch(c.baseSession, prompt, baseBranch)
	} else {
		inst, err = c.orch.AddInstance(c.baseSession, prompt)
	}
	if err != nil {
		return fmt.Errorf("failed to create group consolidator instance: %w", err)
	}

	// Store the consolidator instance ID
	c.mu.Lock()
	for len(session.GroupConsolidatorIDs) <= groupIndex {
		session.GroupConsolidatorIDs = append(session.GroupConsolidatorIDs, "")
	}
	session.GroupConsolidatorIDs[groupIndex] = inst.ID
	c.mu.Unlock()

	// Save state
	_ = c.orch.SaveSession()

	// Emit event
	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventGroupComplete,
		Message: fmt.Sprintf("Starting group %d consolidator session", groupIndex+1),
	})

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start group consolidator instance: %w", err)
	}

	// Monitor the consolidator synchronously (blocks until completion)
	return c.monitorGroupConsolidator(groupIndex, inst.ID)
}

// monitorGroupConsolidator monitors the group consolidator instance and waits for completion
func (c *Coordinator) monitorGroupConsolidator(groupIndex int, instanceID string) error {
	session := c.Session()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return fmt.Errorf("context cancelled")

		case <-ticker.C:
			inst := c.orch.GetInstance(instanceID)
			if inst == nil {
				return fmt.Errorf("consolidator instance not found")
			}

			// Check for the completion file
			if inst.WorktreePath != "" {
				completionPath := GroupConsolidationCompletionFilePath(inst.WorktreePath)
				if _, err := os.Stat(completionPath); err == nil {
					// Parse the completion file
					completion, err := ParseGroupConsolidationCompletionFile(inst.WorktreePath)
					if err != nil {
						// File exists but is invalid/incomplete - might still be writing
						// Continue monitoring and try again on next tick
						continue
					}

					// Check status
					if completion.Status == "failed" {
						// Stop the consolidator instance even on failure
						_ = c.orch.StopInstance(inst)
						return fmt.Errorf("group %d consolidation failed: %s", groupIndex+1, completion.Notes)
					}

					// Store the consolidated branch
					c.mu.Lock()
					for len(session.GroupConsolidatedBranches) <= groupIndex {
						session.GroupConsolidatedBranches = append(session.GroupConsolidatedBranches, "")
					}
					session.GroupConsolidatedBranches[groupIndex] = completion.BranchName

					// Store the consolidation context for the next group
					for len(session.GroupConsolidationContexts) <= groupIndex {
						session.GroupConsolidationContexts = append(session.GroupConsolidationContexts, nil)
					}
					session.GroupConsolidationContexts[groupIndex] = completion
					c.mu.Unlock()

					// Persist state
					_ = c.orch.SaveSession()

					// Stop the consolidator instance to free up resources
					_ = c.orch.StopInstance(inst)

					// Emit success event
					c.manager.emitEvent(CoordinatorEvent{
						Type:    EventGroupComplete,
						Message: fmt.Sprintf("Group %d consolidated into %s (verification: %v)", groupIndex+1, completion.BranchName, completion.Verification.OverallSuccess),
					})

					return nil
				}
			}

			// Check if instance has failed/exited without completion file
			switch inst.Status {
			case StatusError:
				return fmt.Errorf("consolidator instance failed with error")
			case StatusCompleted:
				// Check if tmux session still exists
				mgr := c.orch.GetInstanceManager(instanceID)
				if mgr != nil && mgr.TmuxSessionExists() {
					// Still running, keep monitoring
					continue
				}
				// Instance completed without writing completion file
				return fmt.Errorf("consolidator completed without writing completion file")
			}
		}
	}
}

// formatCandidatePlansForManager formats candidate plans for the PlanManagerPromptTemplate.
// Each plan is formatted with its strategy name (from MultiPassPlanningPrompts) and full JSON content.
func formatCandidatePlansForManager(plans []*PlanSpec) string {
	if len(plans) == 0 {
		return "No candidate plans available."
	}

	var sb strings.Builder

	for i, plan := range plans {
		// Get the strategy name from the corresponding MultiPassPlanningPrompts entry
		strategyName := "unknown"
		if i < len(MultiPassPlanningPrompts) {
			strategyName = MultiPassPlanningPrompts[i].Strategy
		}

		// Write the plan header
		sb.WriteString(fmt.Sprintf("### Plan %d: %s\n", i+1, strategyName))

		// Handle nil plan
		if plan == nil {
			sb.WriteString("<plan>\n")
			sb.WriteString("null\n")
			sb.WriteString("</plan>\n\n")
			continue
		}

		// Marshal the plan to JSON with indentation for readability
		planJSON, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			sb.WriteString("<plan>\n")
			sb.WriteString(fmt.Sprintf("Error marshaling plan: %v\n", err))
			sb.WriteString("</plan>\n\n")
			continue
		}

		sb.WriteString("<plan>\n")
		sb.WriteString(string(planJSON))
		sb.WriteString("\n</plan>\n\n")
	}

	return strings.TrimSuffix(sb.String(), "\n")
}
