package step

import "fmt"

// PhasePlanning and other phase constants for session state management.
const (
	PhasePlanning      = "planning"
	PhasePlanSelection = "plan_selection"
	PhaseExecuting     = "executing"
	PhaseSynthesis     = "synthesis"
	PhaseRevision      = "revision"
	PhaseConsolidating = "consolidating"
)

// Restarter provides step restart functionality for ultra-plan workflows.
// It stops existing instances and starts fresh ones for the specified step.
type Restarter struct {
	coordinator StepCoordinatorInterface
}

// NewRestarter creates a new step restarter with the given coordinator interface.
func NewRestarter(coord StepCoordinatorInterface) *Restarter {
	return &Restarter{coordinator: coord}
}

// RestartStep restarts the specified step. This stops any existing instance for that step
// and starts a fresh one. Returns the new instance ID or an error.
func (r *Restarter) RestartStep(stepInfo *StepInfo) (string, error) {
	if stepInfo == nil {
		return "", fmt.Errorf("step info is nil")
	}

	session := r.coordinator.Session()
	if session == nil {
		return "", fmt.Errorf("no session")
	}

	// Stop the existing instance if it exists (best-effort)
	if stepInfo.InstanceID != "" {
		orch := r.coordinator.GetOrchestrator()
		if orch != nil {
			inst := orch.GetInstance(stepInfo.InstanceID)
			if inst != nil {
				if err := orch.StopInstance(inst); err != nil {
					r.coordinator.Logger().Warn("failed to stop existing instance before restart",
						"instance_id", stepInfo.InstanceID,
						"step_type", stepInfo.Type,
						"error", err)
					// Continue with restart - stopping is best-effort
				}
			}
		}
	}

	switch stepInfo.Type {
	case StepTypePlanning:
		return r.restartPlanning()

	case StepTypePlanManager:
		return r.restartPlanManager()

	case StepTypeTask:
		return r.restartTask(stepInfo.TaskID)

	case StepTypeSynthesis:
		return r.restartSynthesis()

	case StepTypeRevision:
		return r.restartRevision()

	case StepTypeConsolidation:
		return r.restartConsolidation()

	case StepTypeGroupConsolidator:
		return r.restartGroupConsolidator(stepInfo.GroupIndex)

	default:
		return "", fmt.Errorf("unknown step type: %s", stepInfo.Type)
	}
}

// restartPlanning restarts the planning phase.
func (r *Restarter) restartPlanning() (string, error) {
	session := r.coordinator.Session()

	// Reset planning orchestrator state first
	if planOrch := r.coordinator.PlanningOrchestrator(); planOrch != nil {
		planOrch.Reset()
	}

	// Reset planning-related state in session
	r.coordinator.Lock()
	session.SetCoordinatorID("")
	session.SetPlan(nil)
	session.SetPhase(PhasePlanning)
	r.coordinator.Unlock()

	// Run planning again
	if err := r.coordinator.RunPlanning(); err != nil {
		return "", fmt.Errorf("failed to restart planning: %w", err)
	}

	return session.GetCoordinatorID(), nil
}

// restartPlanManager restarts the plan manager in multi-pass mode.
func (r *Restarter) restartPlanManager() (string, error) {
	session := r.coordinator.Session()
	if !session.GetConfig().IsMultiPass() {
		return "", fmt.Errorf("plan manager only exists in multi-pass mode")
	}

	// Reset planning orchestrator state (includes multi-pass coordinator IDs)
	if planOrch := r.coordinator.PlanningOrchestrator(); planOrch != nil {
		planOrch.Reset()
	}

	// Reset plan manager state in session
	r.coordinator.Lock()
	session.SetPlanManagerID("")
	session.SetPlan(nil)
	session.SetPhase(PhasePlanSelection)
	r.coordinator.Unlock()

	// Run plan manager again
	if err := r.coordinator.RunPlanManager(); err != nil {
		return "", fmt.Errorf("failed to restart plan manager: %w", err)
	}

	return session.GetPlanManagerID(), nil
}

// restartTask restarts a specific task by delegating to the ExecutionOrchestrator.
// The session state is reset first, then the task is started through the orchestrator.
func (r *Restarter) restartTask(taskID string) (string, error) {
	session := r.coordinator.Session()
	if taskID == "" {
		return "", fmt.Errorf("task ID is required")
	}

	task := session.GetTask(taskID)
	if task == nil {
		return "", fmt.Errorf("task %s not found", taskID)
	}

	// Check if any tasks are currently running
	runningCount := r.coordinator.GetRunningCount()
	if runningCount > 0 {
		return "", fmt.Errorf("cannot restart task while %d tasks are running", runningCount)
	}

	// Get ExecutionOrchestrator - required for proper task execution
	execOrch := r.coordinator.ExecutionOrchestrator()
	if execOrch == nil {
		return "", fmt.Errorf("ExecutionOrchestrator not initialized")
	}

	// Reset execution orchestrator state to clear ProcessedTasks and allow re-execution
	// Since no tasks are running (verified above), clearing all state is safe
	execOrch.Reset()

	// Reset task state in session
	r.coordinator.Lock()
	// Remove from completed tasks
	completedTasks := session.GetCompletedTasks()
	newCompleted := make([]string, 0, len(completedTasks))
	for _, t := range completedTasks {
		if t != taskID {
			newCompleted = append(newCompleted, t)
		}
	}
	session.SetCompletedTasks(newCompleted)

	// Remove from failed tasks
	failedTasks := session.GetFailedTasks()
	newFailed := make([]string, 0, len(failedTasks))
	for _, t := range failedTasks {
		if t != taskID {
			newFailed = append(newFailed, t)
		}
	}
	session.SetFailedTasks(newFailed)

	// Remove from TaskToInstance
	session.DeleteTaskToInstance(taskID)

	// Reset retry state
	retryManager := r.coordinator.GetRetryManager()
	retryManager.Reset(taskID)
	session.SetTaskRetries(retryManager.GetAllStates())

	// Reset commit count
	session.DeleteTaskCommitCount(taskID)

	// Ensure we're in executing phase
	session.SetPhase(PhaseExecuting)

	// Clear any group decision state
	session.SetGroupDecision(nil)
	r.coordinator.Unlock()

	// Start the task through ExecutionOrchestrator
	newInstanceID, err := execOrch.StartSingleTask(taskID)
	if err != nil {
		return "", fmt.Errorf("failed to restart task: %w", err)
	}

	if err := r.coordinator.SaveSession(); err != nil {
		r.coordinator.Logger().Error("failed to save session after task restart",
			"task_id", taskID,
			"new_instance_id", newInstanceID,
			"error", err)
	}

	return newInstanceID, nil
}

// restartSynthesis restarts the synthesis phase.
func (r *Restarter) restartSynthesis() (string, error) {
	session := r.coordinator.Session()

	// Reset synthesis orchestrator state
	if synthOrch := r.coordinator.SynthesisOrchestrator(); synthOrch != nil {
		synthOrch.Reset()
	}

	// Reset synthesis state in session
	r.coordinator.Lock()
	session.SetSynthesisID("")
	session.SetSynthesisCompletion(nil)
	session.SetSynthesisAwaitingApproval(false)
	session.SetPhase(PhaseSynthesis)
	r.coordinator.Unlock()

	// Run synthesis again
	if err := r.coordinator.RunSynthesis(); err != nil {
		return "", fmt.Errorf("failed to restart synthesis: %w", err)
	}

	return session.GetSynthesisID(), nil
}

// restartRevision restarts the revision phase.
// Revision is a sub-phase of synthesis, so we reset the synthesis orchestrator's
// revision-related state while preserving the identified issues.
func (r *Restarter) restartRevision() (string, error) {
	session := r.coordinator.Session()

	revision := session.GetRevision()
	if revision == nil || len(revision.GetIssues()) == 0 {
		return "", fmt.Errorf("no revision issues to address")
	}

	// Reset synthesis orchestrator state (which handles revision as a sub-phase)
	// Note: Reset() clears all state including revision state, but the session's
	// Revision.Issues are preserved below
	if synthOrch := r.coordinator.SynthesisOrchestrator(); synthOrch != nil {
		synthOrch.Reset()
	}

	// Reset revision state in session (keep issues but reset progress)
	r.coordinator.Lock()
	session.SetRevisionID("")
	session.SetPhase(PhaseRevision)
	revision.SetRevisedTasks(make([]string, 0))
	revision.SetTasksToRevise(extractTasksToRevise(revision.GetIssues()))
	r.coordinator.Unlock()

	// Run revision again via SynthesisOrchestrator
	if synthOrch := r.coordinator.SynthesisOrchestrator(); synthOrch != nil {
		if err := synthOrch.StartRevision(revision.GetIssues()); err != nil {
			return "", fmt.Errorf("failed to restart revision: %w", err)
		}
	}

	return session.GetRevisionID(), nil
}

// restartConsolidation restarts the consolidation phase.
func (r *Restarter) restartConsolidation() (string, error) {
	session := r.coordinator.Session()

	// Reset consolidation orchestrator state
	if consolOrch := r.coordinator.ConsolidationOrchestrator(); consolOrch != nil {
		consolOrch.Reset()
	}

	// Reset consolidation state in session
	r.coordinator.Lock()
	session.SetConsolidationID("")
	session.SetConsolidation(nil)
	session.SetPRUrls(nil)
	session.SetPhase(PhaseConsolidating)
	r.coordinator.Unlock()

	// Start consolidation again
	if err := r.coordinator.StartConsolidation(); err != nil {
		return "", fmt.Errorf("failed to restart consolidation: %w", err)
	}

	return session.GetConsolidationID(), nil
}

// restartGroupConsolidator restarts a specific group consolidator.
func (r *Restarter) restartGroupConsolidator(groupIndex int) (string, error) {
	session := r.coordinator.Session()

	plan := session.GetPlan()
	if plan == nil {
		return "", fmt.Errorf("no plan")
	}

	executionOrder := plan.GetExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return "", fmt.Errorf("invalid group index: %d", groupIndex)
	}

	// Reset consolidation orchestrator state for restart
	// This clears conflict-related state and instance tracking
	if consolOrch := r.coordinator.ConsolidationOrchestrator(); consolOrch != nil {
		consolOrch.ClearStateForRestart()
	}

	// Reset group consolidator state in session
	r.coordinator.Lock()
	session.SetGroupConsolidatorID(groupIndex, "")
	session.SetGroupConsolidatedBranch(groupIndex, "")
	session.SetGroupConsolidationContext(groupIndex, nil)
	r.coordinator.Unlock()

	// Start group consolidation again
	if err := r.coordinator.StartGroupConsolidatorSession(groupIndex); err != nil {
		return "", fmt.Errorf("failed to restart group consolidator: %w", err)
	}

	// Get the new instance ID
	groupConsolidatorIDs := session.GetGroupConsolidatorIDs()
	var newInstanceID string
	if groupIndex < len(groupConsolidatorIDs) {
		newInstanceID = groupConsolidatorIDs[groupIndex]
	}

	if err := r.coordinator.SaveSession(); err != nil {
		r.coordinator.Logger().Error("failed to save session after group consolidator restart",
			"group_index", groupIndex,
			"new_instance_id", newInstanceID,
			"error", err)
	}

	return newInstanceID, nil
}

// extractTasksToRevise extracts unique task IDs from revision issues.
func extractTasksToRevise(issues []RevisionIssue) []string {
	seen := make(map[string]bool)
	var tasks []string
	for _, issue := range issues {
		if !seen[issue.TaskID] {
			seen[issue.TaskID] = true
			tasks = append(tasks, issue.TaskID)
		}
	}
	return tasks
}
