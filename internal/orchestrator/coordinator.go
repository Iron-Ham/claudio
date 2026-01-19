package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator/group"
	"github.com/Iron-Ham/claudio/internal/orchestrator/phase"
	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
	"github.com/Iron-Ham/claudio/internal/orchestrator/retry"
	"github.com/Iron-Ham/claudio/internal/orchestrator/verify"
)

// Verifier provides task verification capabilities.
// This interface allows the Coordinator to delegate verification logic
// while maintaining control over its own state management.
type Verifier interface {
	// CheckCompletionFile checks if the task has written its completion sentinel file.
	CheckCompletionFile(worktreePath string) (bool, error)

	// VerifyTaskWork checks if a task produced actual commits and determines success/retry.
	// The opts parameter provides task-specific context (e.g., NoCode flag for verification tasks).
	VerifyTaskWork(taskID, instanceID, worktreePath, baseBranch string, opts *verify.TaskVerifyOptions) verify.TaskCompletionResult
}

// Coordinator orchestrates the execution of an ultra-plan
type Coordinator struct {
	manager      *UltraPlanManager
	orch         *Orchestrator
	baseSession  *Session
	callbacks    *CoordinatorCallbacks
	logger       *logging.Logger
	verifier     Verifier
	retryManager *retry.Manager
	groupTracker *group.Tracker

	// Phase orchestrators - each orchestrator owns one phase of ultra-plan execution
	planningOrchestrator      *phase.PlanningOrchestrator
	executionOrchestrator     *phase.ExecutionOrchestrator
	synthesisOrchestrator     *phase.SynthesisOrchestrator
	consolidationOrchestrator *phase.ConsolidationOrchestrator

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

	// Initialize retry manager and load existing state from session
	retryMgr := retry.NewManager()
	if ultraSession.TaskRetries != nil {
		retryMgr.LoadStates(ultraSession.TaskRetries)
	}

	// Create session adapter for group tracker
	sessionAdapter := group.NewSessionAdapter(
		func() group.PlanData {
			session := manager.Session()
			if session == nil || session.Plan == nil {
				return nil
			}
			return group.NewPlanAdapter(
				func() [][]string { return session.Plan.ExecutionOrder },
				func(taskID string) *group.Task {
					for i := range session.Plan.Tasks {
						if session.Plan.Tasks[i].ID == taskID {
							t := &session.Plan.Tasks[i]
							return &group.Task{
								ID:          t.ID,
								Title:       t.Title,
								Description: t.Description,
								Files:       t.Files,
								DependsOn:   t.DependsOn,
							}
						}
					}
					return nil
				},
			)
		},
		func() []string {
			session := manager.Session()
			if session == nil {
				return nil
			}
			return session.CompletedTasks
		},
		func() []string {
			session := manager.Session()
			if session == nil {
				return nil
			}
			return session.FailedTasks
		},
		func() map[string]int {
			session := manager.Session()
			if session == nil {
				return nil
			}
			return session.TaskCommitCounts
		},
		func() int {
			session := manager.Session()
			if session == nil {
				return 0
			}
			return session.CurrentGroup
		},
	)

	c := &Coordinator{
		manager:      manager,
		orch:         orch,
		baseSession:  baseSession,
		logger:       sessionLogger,
		retryManager: retryMgr,
		groupTracker: group.NewTracker(sessionAdapter),
		ctx:          ctx,
		cancelFunc:   cancel,
		runningTasks: make(map[string]string),
	}

	// Initialize the verifier with adapters that bridge to coordinator state
	retryTracker := &coordinatorRetryTracker{c: c}
	eventEmitter := &coordinatorEventEmitter{c: c}

	verifyConfig := verify.Config{
		RequireVerifiedCommits: ultraSession.Config.RequireVerifiedCommits,
		MaxTaskRetries:         ultraSession.Config.MaxTaskRetries,
	}
	if verifyConfig.MaxTaskRetries == 0 {
		verifyConfig.MaxTaskRetries = 3 // Default
	}

	c.verifier = verify.NewTaskVerifier(
		orch.wt,
		retryTracker,
		eventEmitter,
		verify.WithConfig(verifyConfig),
		verify.WithLogger(sessionLogger),
	)

	// Initialize phase orchestrators with shared dependencies
	// The orchestrators are created lazily via getter methods to avoid
	// issues during coordinator initialization when BuildPhaseContext
	// depends on the coordinator being fully constructed.
	// This is handled by the getter methods which call initializeOrchestrators().

	return c
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

// RetryManager returns the retry manager for task retry state management.
func (c *Coordinator) RetryManager() *retry.Manager {
	return c.retryManager
}

// syncRetryState syncs the retry manager's state back to the session.
// This should be called before saving the session to ensure retry state is persisted.
func (c *Coordinator) syncRetryState() {
	session := c.Session()
	c.mu.Lock()
	defer c.mu.Unlock()
	session.TaskRetries = c.retryManager.GetAllStates()
}

// GroupTracker returns the group tracker for execution group management
func (c *Coordinator) GroupTracker() *group.Tracker {
	return c.groupTracker
}

// Plan returns the current plan, if available
func (c *Coordinator) Plan() *PlanSpec {
	session := c.manager.Session()
	if session == nil {
		return nil
	}
	return session.Plan
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

	// Get PlanningOrchestrator - always delegate to it
	po := c.PlanningOrchestrator()
	if po == nil {
		return fmt.Errorf("planning orchestrator not initialized")
	}

	// Provide callbacks for group management and session state updates
	// Use SubgroupRouter to route planning instances to the Planning subgroup
	getGroup := func() any {
		// Use session.GroupID (set by TUI) for reliable group lookup
		var parentGroup *InstanceGroup
		if session.GroupID != "" {
			parentGroup = c.baseSession.GetGroup(session.GroupID)
		}
		if parentGroup == nil {
			parentGroup = c.baseSession.GetGroupBySessionType(SessionTypeUltraPlan)
		}
		if parentGroup == nil {
			return nil
		}
		return NewSubgroupRouter(parentGroup, session)
	}
	setCoordinatorID := func(id string) {
		session.CoordinatorID = id
	}

	return po.ExecuteWithPrompt(c.ctx, prompt, c.baseSession, getGroup, setCoordinatorID)
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
		// Update PlanningOrchestrator state on error
		if po := c.PlanningOrchestrator(); po != nil {
			po.SetError("no multi-pass planning strategies available")
		}
		return fmt.Errorf("no multi-pass planning strategies available")
	}

	// Initialize the PlanCoordinatorIDs slice
	session.PlanCoordinatorIDs = make([]string, 0, len(strategies))
	planCoordinatorIDs := make([]string, 0, len(strategies))

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
			// Update PlanningOrchestrator state on error
			if po := c.PlanningOrchestrator(); po != nil {
				po.SetError(err.Error())
			}
			return fmt.Errorf("failed to create planning instance for strategy %s: %w", strategy, err)
		}

		// Add planning instance to the multi-pass group for sidebar display
		// Use session.GroupID (set by TUI) for reliable group lookup, with fallback to type-based lookup
		var multiGroup *InstanceGroup
		if session.GroupID != "" {
			multiGroup = c.baseSession.GetGroup(session.GroupID)
		}
		if multiGroup == nil {
			multiGroup = c.baseSession.GetGroupBySessionType(SessionTypePlanMulti)
		}
		if multiGroup != nil {
			multiGroup.AddInstance(inst.ID)
		}

		// Store the instance ID
		session.PlanCoordinatorIDs = append(session.PlanCoordinatorIDs, inst.ID)
		planCoordinatorIDs = append(planCoordinatorIDs, inst.ID)

		// Start the instance
		if err := c.orch.StartInstance(inst); err != nil {
			c.logger.Error("planning failed",
				"error", err.Error(),
				"strategy", strategy,
				"stage", "start_instance",
			)
			// Update PlanningOrchestrator state on error
			if po := c.PlanningOrchestrator(); po != nil {
				po.SetError(err.Error())
			}
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

	// Update PlanningOrchestrator state with multi-pass configuration
	if po := c.PlanningOrchestrator(); po != nil {
		po.SetState(phase.PlanningState{
			MultiPass:          true,
			PlanCoordinatorIDs: planCoordinatorIDs,
			AwaitingCompletion: true,
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

	// Update PlanningOrchestrator state - planning coordinators are done, starting plan manager
	if po := c.PlanningOrchestrator(); po != nil {
		po.SetAwaitingCompletion(false)
	}

	// Build the plan manager prompt with all candidate plans
	prompt := BuildPlanManagerPrompt(c)

	// Create the plan manager instance
	inst, err := c.orch.AddInstance(c.baseSession, prompt)
	if err != nil {
		// Update PlanningOrchestrator state on error
		if po := c.PlanningOrchestrator(); po != nil {
			po.SetError(err.Error())
		}
		return fmt.Errorf("failed to create plan manager instance: %w", err)
	}

	// Add plan manager to the multi-pass group for sidebar display
	// Use session.GroupID (set by TUI) for reliable group lookup, with fallback to type-based lookup
	var multiGroup *InstanceGroup
	if session.GroupID != "" {
		multiGroup = c.baseSession.GetGroup(session.GroupID)
	}
	if multiGroup == nil {
		multiGroup = c.baseSession.GetGroupBySessionType(SessionTypePlanMulti)
	}
	if multiGroup != nil {
		multiGroup.AddInstance(inst.ID)
	}

	session.PlanManagerID = inst.ID

	// Persist the state
	_ = c.orch.SaveSession()

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		// Update PlanningOrchestrator state on error
		if po := c.PlanningOrchestrator(); po != nil {
			po.SetError(err.Error())
		}
		return fmt.Errorf("failed to start plan manager instance: %w", err)
	}

	// The TUI will handle monitoring and parsing the final plan from the manager
	return nil
}

// convertPlanSpecsToCandidatePlans converts []*PlanSpec to []prompt.CandidatePlanInfo.
// This bridges the orchestrator types to the prompt package types without requiring
// the prompt package to import orchestrator (avoiding circular dependencies).
// Nil plans in the input are skipped (not included in the output).
func convertPlanSpecsToCandidatePlans(plans []*PlanSpec, strategies []string) []prompt.CandidatePlanInfo {
	if len(plans) == 0 {
		return nil
	}

	result := make([]prompt.CandidatePlanInfo, 0, len(plans))
	for i, plan := range plans {
		// Skip nil plans to match original buildPlanComparisonSection behavior
		if plan == nil {
			continue
		}

		var strategy string
		if i < len(strategies) {
			strategy = strategies[i]
		}

		// Convert tasks
		taskInfos := make([]prompt.TaskInfo, len(plan.Tasks))
		for j, task := range plan.Tasks {
			taskInfos[j] = prompt.TaskInfo{
				ID:            task.ID,
				Title:         task.Title,
				Description:   task.Description,
				Files:         task.Files,
				DependsOn:     task.DependsOn,
				Priority:      task.Priority,
				EstComplexity: string(task.EstComplexity),
			}
		}

		result = append(result, prompt.CandidatePlanInfo{
			Strategy:       strategy,
			Summary:        plan.Summary,
			Tasks:          taskInfos,
			ExecutionOrder: plan.ExecutionOrder,
			Insights:       plan.Insights,
			Constraints:    plan.Constraints,
		})
	}

	return result
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

	// Get ExecutionOrchestrator - always delegate to it
	eo := c.ExecutionOrchestrator()
	if eo == nil {
		return fmt.Errorf("execution orchestrator not initialized")
	}

	// Reset ExecutionOrchestrator state for fresh execution
	eo.Reset()

	// Build execution context
	execCtx, err := c.BuildExecutionContext()
	if err != nil {
		return fmt.Errorf("failed to build execution context: %w", err)
	}

	// Start execution via the orchestrator
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if execErr := eo.ExecuteWithContext(c.ctx, execCtx); execErr != nil {
			c.logger.Error("execution phase failed", "error", execErr)
		}
	}()

	return nil
}

// getTaskGroupIndex returns the group index for a given task ID, or -1 if not found
func (c *Coordinator) getTaskGroupIndex(taskID string) int {
	return c.groupTracker.GetTaskGroupIndex(taskID)
}

// RunSynthesis executes the synthesis phase
func (c *Coordinator) RunSynthesis() error {
	c.notifyPhaseChange(PhaseSynthesis)

	// Get SynthesisOrchestrator - always delegate to it
	so := c.SynthesisOrchestrator()
	if so == nil {
		return fmt.Errorf("synthesis orchestrator not initialized")
	}

	// Reset SynthesisOrchestrator state for fresh synthesis
	so.Reset()

	// Start synthesis via the orchestrator in a goroutine
	// The orchestrator will handle monitoring and trigger consolidation
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if err := so.Execute(c.ctx); err != nil {
			c.logger.Error("synthesis phase failed", "error", err)
			// Update session state on failure
			session := c.Session()
			c.mu.Lock()
			session.Phase = PhaseFailed
			session.Error = fmt.Sprintf("synthesis failed: %v", err)
			c.mu.Unlock()
			_ = c.orch.SaveSession()
			c.notifyComplete(false, session.Error)
		}
	}()

	return nil
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

	// Update SynthesisOrchestrator state
	if so := c.SynthesisOrchestrator(); so != nil {
		so.SetAwaitingApproval(false)
	}

	// Stop the synthesis instance if it's still running
	if session.SynthesisID != "" {
		inst := c.orch.GetInstance(session.SynthesisID)
		if inst != nil {
			_ = c.orch.StopInstance(inst)
		}
	}

	// Proceed to consolidation (or completion if no consolidation configured)
	// Delegate to SynthesisOrchestrator which now owns this logic
	if so := c.SynthesisOrchestrator(); so != nil {
		so.OnSynthesisApproved()
	}
	return nil
}

// StartConsolidation begins the consolidation phase
// This creates a Claude instance that performs branch consolidation and PR creation
func (c *Coordinator) StartConsolidation() error {
	session := c.Session()
	c.notifyPhaseChange(PhaseConsolidating)

	// Initialize consolidation state
	c.mu.Lock()
	session.Consolidation = &ConsolidatorState{
		Phase:       ConsolidationCreatingBranches,
		TotalGroups: len(session.Plan.ExecutionOrder),
	}
	c.mu.Unlock()

	// Reset ConsolidationOrchestrator state for fresh consolidation
	if co := c.ConsolidationOrchestrator(); co != nil {
		co.Reset()
	}

	// Build the consolidation prompt
	prompt := BuildConsolidationPrompt(c)

	// Create a consolidation instance
	inst, err := c.orch.AddInstance(c.baseSession, prompt)
	if err != nil {
		return fmt.Errorf("failed to create consolidation instance: %w", err)
	}

	// Add consolidation instance to the ultraplan group for sidebar display
	sessionType := SessionTypeUltraPlan
	if session.Config.MultiPass {
		sessionType = SessionTypePlanMulti
	}
	if ultraGroup := c.baseSession.GetGroupBySessionType(sessionType); ultraGroup != nil {
		ultraGroup.AddInstance(inst.ID)
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
		MonitorConsolidationInstance(c, inst.ID)
	}()

	return nil
}

// GetConsolidation returns the current consolidation state
func (c *Coordinator) GetConsolidation() *ConsolidatorState {
	session := c.Session()
	if session == nil {
		return nil
	}
	return session.Consolidation
}

// ResumeConsolidation resumes consolidation after a conflict has been resolved.
// The user must first resolve the conflict manually in the conflict worktree,
// then call this method to continue the cherry-pick and restart consolidation.
// Delegates to ConsolidationOrchestrator.ResumeConsolidation for the actual implementation.
func (c *Coordinator) ResumeConsolidation() error {
	// Validate session first for proper error messages
	session := c.Session()
	if session == nil {
		return fmt.Errorf("no session")
	}
	if session.Consolidation == nil {
		return fmt.Errorf("no consolidation in progress")
	}
	if session.Consolidation.Phase != ConsolidationPaused {
		return fmt.Errorf("consolidation is not paused (current phase: %s)", session.Consolidation.Phase)
	}
	if session.Consolidation.ConflictWorktree == "" {
		return fmt.Errorf("no conflict worktree recorded")
	}

	co := c.ConsolidationOrchestrator()
	if co == nil {
		return fmt.Errorf("ConsolidationOrchestrator not initialized")
	}
	if c.orch == nil || c.orch.wt == nil {
		return fmt.Errorf("orchestrator not initialized")
	}

	// Create adapters for the interfaces the orchestrator needs
	worktreeOp := &coordinatorWorktreeAdapter{wt: c.orch.wt}
	sessionSaver := &coordinatorSessionSaver{orch: c.orch}

	return co.ResumeConsolidation(worktreeOp, sessionSaver, c.StartConsolidation)
}

// coordinatorWorktreeAdapter adapts the coordinator's worktree manager to the WorktreeOperator interface.
type coordinatorWorktreeAdapter struct {
	wt interface {
		GetConflictingFiles(worktreePath string) ([]string, error)
		IsCherryPickInProgress(worktreePath string) bool
		ContinueCherryPick(worktreePath string) error
	}
}

func (a *coordinatorWorktreeAdapter) GetConflictingFiles(worktreePath string) ([]string, error) {
	return a.wt.GetConflictingFiles(worktreePath)
}

func (a *coordinatorWorktreeAdapter) IsCherryPickInProgress(worktreePath string) bool {
	return a.wt.IsCherryPickInProgress(worktreePath)
}

func (a *coordinatorWorktreeAdapter) ContinueCherryPick(worktreePath string) error {
	return a.wt.ContinueCherryPick(worktreePath)
}

// coordinatorSessionSaver adapts the orchestrator's SaveSession to the ConsolidationSessionSaver interface.
type coordinatorSessionSaver struct {
	orch *Orchestrator
}

func (s *coordinatorSessionSaver) SaveSession() error {
	return s.orch.SaveSession()
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

// ResumeWithPartialWork continues execution with only the successful tasks.
// Delegates core work to ExecutionOrchestrator, then advances the group state.
func (c *Coordinator) ResumeWithPartialWork() error {
	eo := c.ExecutionOrchestrator()
	if eo == nil {
		return fmt.Errorf("ExecutionOrchestrator not initialized")
	}

	// Delegate consolidation and event emission to the orchestrator
	if err := eo.ResumeWithPartialWork(); err != nil {
		return err
	}

	// Advance to the next group AFTER consolidation succeeds
	// This is critical - without this, checkAndAdvanceGroup() would detect
	// the partial failure again and re-prompt the user
	session := c.Session()
	c.mu.Lock()
	session.CurrentGroup++
	c.mu.Unlock()

	// Reset ExecutionOrchestrator state for the next group
	eo.Reset()

	return nil
}

// RetryFailedTasks retries the failed tasks in the current group.
// Delegates to ExecutionOrchestrator.RetryFailedTasks for the actual implementation.
func (c *Coordinator) RetryFailedTasks() error {
	eo := c.ExecutionOrchestrator()
	if eo == nil {
		return fmt.Errorf("ExecutionOrchestrator not initialized")
	}
	return eo.RetryFailedTasks()
}

// RetriggerGroup resets execution state to the specified group index and restarts execution.
// All state from groups >= targetGroup is cleared, since subsequent groups depend on the
// re-triggered group's consolidated branch.
// Delegates to ExecutionOrchestrator.RetriggerGroup for the actual implementation.
func (c *Coordinator) RetriggerGroup(targetGroup int) error {
	eo := c.ExecutionOrchestrator()
	if eo == nil {
		return fmt.Errorf("ExecutionOrchestrator not initialized")
	}
	return eo.RetriggerGroup(targetGroup)
}
