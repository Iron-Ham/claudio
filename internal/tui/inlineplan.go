package tui

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/group"
	"github.com/Iron-Ham/claudio/internal/tui/command"
	"github.com/Iron-Ham/claudio/internal/tui/view"
)

// initInlinePlanMode initializes inline plan mode when :plan command is executed.
// This prompts for an objective and will create a planning instance.
func (m *Model) initInlinePlanMode() {
	m.inlinePlan = &InlinePlanState{
		AwaitingObjective: true,
		TaskToInstance:    make(map[string]string),
	}

	// Prompt for objective using task input UI
	m.addingTask = true
	m.taskInput = ""
	m.taskInputCursor = 0
	m.infoMessage = "Enter plan objective:"
}

// initInlineUltraPlanMode initializes inline ultraplan mode when :ultraplan command is executed.
// This creates the UltraPlan coordinator and enters the planning workflow.
func (m *Model) initInlineUltraPlanMode(result command.Result) {
	// Create ultraplan session config
	cfg := orchestrator.UltraPlanConfig{
		AutoApprove: false, // Default to requiring approval in inline mode
		Review:      true,  // Always review in inline mode
	}

	if result.UltraPlanMultiPass != nil && *result.UltraPlanMultiPass {
		cfg.MultiPass = true
	}

	// If loading from file, handle that case
	if result.UltraPlanFromFile != nil && *result.UltraPlanFromFile != "" {
		planPath := *result.UltraPlanFromFile
		plan, err := orchestrator.ParsePlanFromFile(planPath, "")
		if err != nil {
			m.errorMessage = fmt.Sprintf("Failed to load plan: %v", err)
			return
		}

		// Create ultraplan session with loaded plan
		ultraSession := orchestrator.NewUltraPlanSession("", cfg)
		ultraSession.Plan = plan
		ultraSession.Phase = orchestrator.PhaseRefresh

		// Initialize coordinator with the session
		coordinator := orchestrator.NewCoordinator(m.orchestrator, m.session, ultraSession, m.logger)
		if err := coordinator.SetPlan(plan); err != nil {
			m.errorMessage = fmt.Sprintf("Invalid plan: %v", err)
			return
		}

		// Create a group for this ultraplan session
		sessionType := orchestrator.SessionTypeUltraPlan
		if cfg.MultiPass {
			sessionType = orchestrator.SessionTypePlanMulti
		}
		objective := plan.Objective
		if objective == "" {
			objective = "Loaded Plan"
		}
		ultraGroup := orchestrator.NewInstanceGroupWithType(
			truncateString(objective, 30),
			sessionType,
			objective,
		)
		m.session.AddGroup(ultraGroup)

		// Auto-enable grouped sidebar mode
		m.autoEnableGroupedMode()

		m.ultraPlan = &view.UltraPlanState{
			Coordinator:  coordinator,
			ShowPlanView: false,
		}

		// Enter plan editor for review
		m.enterPlanEditor()
		m.infoMessage = fmt.Sprintf("Plan loaded: %d tasks. Review and press [enter] to execute.", len(plan.Tasks))
		return
	}

	// If objective is provided, start planning immediately
	if result.UltraPlanObjective != nil && *result.UltraPlanObjective != "" {
		objective := *result.UltraPlanObjective

		// Create ultraplan session
		ultraSession := orchestrator.NewUltraPlanSession(objective, cfg)

		// Initialize coordinator
		coordinator := orchestrator.NewCoordinator(m.orchestrator, m.session, ultraSession, m.logger)

		// Create a group for this ultraplan session
		sessionType := orchestrator.SessionTypeUltraPlan
		if cfg.MultiPass {
			sessionType = orchestrator.SessionTypePlanMulti
		}
		ultraGroup := orchestrator.NewInstanceGroupWithType(
			truncateString(objective, 30),
			sessionType,
			objective,
		)
		m.session.AddGroup(ultraGroup)

		// Auto-enable grouped sidebar mode
		m.autoEnableGroupedMode()

		// Start planning phase - create the planning instance
		if err := coordinator.RunPlanning(); err != nil {
			m.errorMessage = fmt.Sprintf("Failed to start planning: %v", err)
			return
		}

		m.ultraPlan = &view.UltraPlanState{
			Coordinator:  coordinator,
			ShowPlanView: false,
		}
		m.infoMessage = "Planning started..."
		return
	}

	// No objective provided - prompt for one using inline plan state
	// Mark as ultraplan so the objective handler creates the ultraplan coordinator
	m.inlinePlan = &InlinePlanState{
		AwaitingObjective: true,
		TaskToInstance:    make(map[string]string),
		IsUltraPlan:       true,
		UltraPlanConfig:   &cfg,
	}

	m.addingTask = true
	m.taskInput = ""
	m.taskInputCursor = 0
	m.infoMessage = "Enter ultraplan objective:"
}

// toggleGroupedView toggles the grouped instance view on/off
func (m *Model) toggleGroupedView() {
	// Toggle between flat and grouped sidebar modes
	if m.sidebarMode == view.SidebarModeFlat {
		// Switch to grouped mode
		m.sidebarMode = view.SidebarModeGrouped
		// Initialize group view state if needed
		if m.groupViewState == nil {
			m.groupViewState = view.NewGroupViewState()
		}
		m.infoMessage = "Grouped view enabled"
	} else {
		// Switch to flat mode
		m.sidebarMode = view.SidebarModeFlat
		m.infoMessage = "Grouped view disabled"
	}
}

// handleInlinePlanObjectiveSubmit handles submission of the plan objective.
// Called when user presses enter after typing an objective in inline plan mode.
// Called from the task input handler in app.go when m.inlinePlan.AwaitingObjective is true.
func (m *Model) handleInlinePlanObjectiveSubmit(objective string) {
	if m.inlinePlan == nil {
		if m.logger != nil {
			m.logger.Warn("handleInlinePlanObjectiveSubmit called with nil inlinePlan",
				"objective", objective)
		}
		m.errorMessage = "Unable to submit objective: plan state lost"
		return
	}

	m.inlinePlan.AwaitingObjective = false
	m.inlinePlan.Objective = objective
	m.inlinePlan.AwaitingPlanCreation = true

	// Create a planning instance to generate the plan
	inst, err := m.orchestrator.AddInstance(m.session, m.createPlanningPrompt(objective))
	if err != nil {
		m.errorMessage = fmt.Sprintf("Failed to create planning instance: %v", err)
		m.inlinePlan = nil
		return
	}

	m.inlinePlan.PlanningInstanceID = inst.ID

	// Start the planning instance
	if err := m.orchestrator.StartInstance(inst); err != nil {
		m.errorMessage = fmt.Sprintf("Failed to start planning: %v", err)
		m.inlinePlan = nil
		return
	}

	// Create a group for this plan's tasks
	gm := m.getGroupManager()
	if gm != nil {
		planGroup := gm.CreateGroup(fmt.Sprintf("Plan: %s", truncateString(objective, 30)), nil)
		m.inlinePlan.GroupID = planGroup.ID

		// Set session type on the group for proper icon display
		if orchGroup := m.session.GetGroup(planGroup.ID); orchGroup != nil {
			orchGroup.SessionType = orchestrator.SessionTypePlan
			orchGroup.Objective = objective
			// Add the planning instance to the group for sidebar display
			orchGroup.AddInstance(inst.ID)
		}

		// If in tripleshot mode, register this group for sidebar display
		if m.tripleShot != nil {
			m.tripleShot.PlanGroupIDs = append(m.tripleShot.PlanGroupIDs, planGroup.ID)
		}
		m.infoMessage = "Planning started. The plan will appear when ready..."
	} else {
		// Group manager unavailable - plan will still execute but won't be organized in a group
		if m.logger != nil {
			m.logger.Warn("group manager unavailable, plan will not appear in group view",
				"objective", objective)
		}
		m.infoMessage = "Planning started (group view unavailable)..."
	}
	// Pause the old active instance before switching
	if oldInst := m.activeInstance(); oldInst != nil {
		m.pauseInstance(oldInst.ID)
	}
	m.activeTab = m.findInstanceIndex(inst.ID)
	m.ensureActiveVisible()
	// Resume the new active instance's capture
	m.resumeActiveInstance()
}

// handleUltraPlanObjectiveSubmit handles submission of an ultraplan objective.
// Called when user presses enter after typing an objective for :ultraplan.
// This creates the full ultraplan coordinator and starts the planning phase.
func (m *Model) handleUltraPlanObjectiveSubmit(objective string) {
	if m.inlinePlan == nil {
		if m.logger != nil {
			m.logger.Warn("handleUltraPlanObjectiveSubmit called with nil inlinePlan",
				"objective", objective)
		}
		m.errorMessage = "Unable to submit objective: plan state lost"
		return
	}
	if !m.inlinePlan.IsUltraPlan {
		if m.logger != nil {
			m.logger.Warn("handleUltraPlanObjectiveSubmit called for non-ultraplan",
				"objective", objective,
				"isUltraPlan", m.inlinePlan.IsUltraPlan)
		}
		m.errorMessage = "Unable to submit objective: incorrect plan mode"
		return
	}

	// Get config from the inline plan state
	cfg := orchestrator.UltraPlanConfig{
		AutoApprove: false,
		Review:      true,
	}
	if m.inlinePlan.UltraPlanConfig != nil {
		cfg = *m.inlinePlan.UltraPlanConfig
	}

	// Clear the inline plan state - we're transitioning to ultraplan
	m.inlinePlan = nil

	// Create ultraplan session
	ultraSession := orchestrator.NewUltraPlanSession(objective, cfg)

	// Initialize coordinator
	coordinator := orchestrator.NewCoordinator(m.orchestrator, m.session, ultraSession, m.logger)

	// Start planning phase FIRST - before creating UI resources
	// This prevents orphaned groups if planning fails
	if err := coordinator.RunPlanning(); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to start ultraplan planning",
				"objective", objective,
				"multiPass", cfg.MultiPass,
				"error", err)
		}
		m.errorMessage = fmt.Sprintf("Failed to start planning: %v", err)
		return
	}

	// Create UI resources only after planning starts successfully
	sessionType := orchestrator.SessionTypeUltraPlan
	if cfg.MultiPass {
		sessionType = orchestrator.SessionTypePlanMulti
	}
	ultraGroup := orchestrator.NewInstanceGroupWithType(
		truncateString(objective, 30),
		sessionType,
		objective,
	)
	m.session.AddGroup(ultraGroup)

	// Auto-enable grouped sidebar mode
	m.autoEnableGroupedMode()

	m.ultraPlan = &view.UltraPlanState{
		Coordinator:  coordinator,
		ShowPlanView: false,
	}
	m.infoMessage = "Planning started..."
}

// createPlanningPrompt creates the prompt for the planning instance
func (m *Model) createPlanningPrompt(objective string) string {
	return fmt.Sprintf(`Create a detailed task plan for the following objective:

%s

Output a structured plan in the following YAML format:

---
objective: "%s"
summary: "Brief 1-2 sentence summary of the approach"
tasks:
  - id: "task-1"
    title: "Brief task title"
    description: "Detailed description of what this task accomplishes"
    files: ["file1.go", "file2.go"]  # Expected files to modify
    depends_on: []  # Task IDs this depends on (empty for first tasks)
    priority: 1  # 1-10 priority (1 = highest)
    complexity: "low"  # low, medium, or high

Write the plan to .claudio/plan.yaml

The plan should:
1. Break down the objective into independent, parallelizable tasks where possible
2. Identify clear dependencies between tasks
3. Estimate complexity for each task
4. List the files each task will likely modify`, objective, objective)
}

// handleInlinePlanCompletion handles when the planning instance completes.
// Parses the generated plan and enters the plan editor.
// Integration note: This should be called from the instance completion handler
// when an inline planning instance completes.
//
//nolint:unused // Integration with instance completion handler pending
func (m *Model) handleInlinePlanCompletion(inst *orchestrator.Instance) bool {
	if m.inlinePlan == nil || !m.inlinePlan.AwaitingPlanCreation {
		return false
	}

	if inst.ID != m.inlinePlan.PlanningInstanceID {
		return false
	}

	// Try to parse the plan from the instance's worktree
	planPath := orchestrator.PlanFilePath(inst.WorktreePath)
	plan, err := orchestrator.ParsePlanFromFile(planPath, m.inlinePlan.Objective)
	if err != nil {
		// Fall back to output parsing
		output := m.outputManager.GetOutput(inst.ID)
		plan, err = orchestrator.ParsePlanFromOutput(output, m.inlinePlan.Objective)
		if err != nil {
			m.errorMessage = fmt.Sprintf("Failed to parse plan: %v", err)
			return true
		}
	}

	m.inlinePlan.Plan = plan
	m.inlinePlan.AwaitingPlanCreation = false

	// Enter the inline plan editor
	m.enterInlinePlanEditor()
	m.infoMessage = fmt.Sprintf("Plan ready: %d tasks. Edit and press [enter] to start execution.", len(plan.Tasks))

	return true
}

// getGroupManager returns the group manager for the session, creating one if needed.
func (m *Model) getGroupManager() *group.Manager {
	if m.session == nil {
		return nil
	}
	// Create adapter that implements ManagerSessionData
	adapter := &sessionGroupAdapter{session: m.session}
	return group.NewManager(adapter)
}

// sessionGroupAdapter adapts orchestrator.Session to group.ManagerSessionData
type sessionGroupAdapter struct {
	session *orchestrator.Session
}

func (a *sessionGroupAdapter) GetGroups() []*group.InstanceGroup {
	if a.session == nil || a.session.Groups == nil {
		return nil
	}
	// Convert orchestrator.InstanceGroup to group.InstanceGroup
	result := make([]*group.InstanceGroup, len(a.session.Groups))
	for i, g := range a.session.Groups {
		result[i] = convertOrchestratorGroupToGroup(g)
	}
	return result
}

func (a *sessionGroupAdapter) SetGroups(groups []*group.InstanceGroup) {
	if a.session == nil {
		return
	}
	// Convert back to orchestrator.InstanceGroup
	result := make([]*orchestrator.InstanceGroup, len(groups))
	for i, g := range groups {
		result[i] = convertGroupToOrchestratorGroup(g)
	}
	a.session.Groups = result
}

func (a *sessionGroupAdapter) GenerateID() string {
	return orchestrator.GenerateID()
}

// convertOrchestratorGroupToGroup converts orchestrator.InstanceGroup to group.InstanceGroup
func convertOrchestratorGroupToGroup(og *orchestrator.InstanceGroup) *group.InstanceGroup {
	if og == nil {
		return nil
	}
	g := &group.InstanceGroup{
		ID:             og.ID,
		Name:           og.Name,
		Phase:          group.GroupPhase(og.Phase),
		Instances:      make([]string, len(og.Instances)),
		ParentID:       og.ParentID,
		ExecutionOrder: og.ExecutionOrder,
		DependsOn:      make([]string, len(og.DependsOn)),
		Created:        og.Created,
	}
	copy(g.Instances, og.Instances)
	copy(g.DependsOn, og.DependsOn)

	// Convert sub-groups recursively
	g.SubGroups = make([]*group.InstanceGroup, len(og.SubGroups))
	for i, sg := range og.SubGroups {
		g.SubGroups[i] = convertOrchestratorGroupToGroup(sg)
	}
	return g
}

// convertGroupToOrchestratorGroup converts group.InstanceGroup to orchestrator.InstanceGroup
func convertGroupToOrchestratorGroup(g *group.InstanceGroup) *orchestrator.InstanceGroup {
	if g == nil {
		return nil
	}
	og := &orchestrator.InstanceGroup{
		ID:             g.ID,
		Name:           g.Name,
		Phase:          orchestrator.GroupPhase(g.Phase),
		Instances:      make([]string, len(g.Instances)),
		ParentID:       g.ParentID,
		ExecutionOrder: g.ExecutionOrder,
		DependsOn:      make([]string, len(g.DependsOn)),
		Created:        g.Created,
	}
	copy(og.Instances, g.Instances)
	copy(og.DependsOn, g.DependsOn)

	// Convert sub-groups recursively
	og.SubGroups = make([]*orchestrator.InstanceGroup, len(g.SubGroups))
	for i, sg := range g.SubGroups {
		og.SubGroups[i] = convertGroupToOrchestratorGroup(sg)
	}
	return og
}

// findInstanceIndex finds the index of an instance in the session's instance list
//
//nolint:unused // Used by handleInlinePlanObjectiveSubmit
func (m *Model) findInstanceIndex(instanceID string) int {
	if m.session == nil {
		return 0
	}
	for i, inst := range m.session.Instances {
		if inst.ID == instanceID {
			return i
		}
	}
	return 0
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// getPlanForInlineEditor returns the plan for inline plan editing
//
//nolint:unused // Superseded by getPlanForEditor with inline mode check
func (m *Model) getPlanForInlineEditor() *orchestrator.PlanSpec {
	if m.inlinePlan == nil {
		return nil
	}
	return m.inlinePlan.Plan
}

// syncPlanTasksToInstances synchronizes plan tasks to instances in the group.
// Creates pending instances for new tasks, removes instances for deleted tasks.
func (m *Model) syncPlanTasksToInstances() error {
	if m.inlinePlan == nil || m.inlinePlan.Plan == nil {
		return fmt.Errorf("no inline plan available")
	}

	plan := m.inlinePlan.Plan
	gm := m.getGroupManager()
	if gm == nil {
		return fmt.Errorf("no group manager available")
	}

	// Get or create the plan group
	var planGroup *group.InstanceGroup
	if m.inlinePlan.GroupID != "" {
		planGroup = gm.GetGroup(m.inlinePlan.GroupID)
	}
	if planGroup == nil {
		planGroup = gm.CreateGroup(fmt.Sprintf("Plan: %s", truncateString(m.inlinePlan.Objective, 30)), nil)
		m.inlinePlan.GroupID = planGroup.ID
	}

	// Track which task IDs currently have instances
	existingTasks := make(map[string]bool)
	for taskID := range m.inlinePlan.TaskToInstance {
		existingTasks[taskID] = true
	}

	// Create instances for new tasks
	for _, task := range plan.Tasks {
		if _, exists := m.inlinePlan.TaskToInstance[task.ID]; !exists {
			// Create a pending instance for this task
			inst, err := m.orchestrator.AddInstance(m.session, task.Description)
			if err != nil {
				return fmt.Errorf("failed to create instance for task %s: %w", task.ID, err)
			}

			// Update instance metadata
			inst.Task = task.Title

			// Add to group
			gm.MoveInstanceToGroup(inst.ID, planGroup.ID)

			// Track mapping
			m.inlinePlan.TaskToInstance[task.ID] = inst.ID
		}
		delete(existingTasks, task.ID)
	}

	// Remove instances for deleted tasks (only if not started)
	for taskID := range existingTasks {
		instanceID := m.inlinePlan.TaskToInstance[taskID]
		inst := m.orchestrator.GetInstance(instanceID)
		if inst != nil && inst.Status == orchestrator.StatusPending {
			_ = m.orchestrator.RemoveInstance(m.session, instanceID, true)
		}
		delete(m.inlinePlan.TaskToInstance, taskID)
	}

	return nil
}

// startInlinePlanExecution starts execution of the inline plan.
// Creates instances for all tasks and organizes them into groups based on dependencies.
func (m *Model) startInlinePlanExecution() error {
	if m.inlinePlan == nil || m.inlinePlan.Plan == nil {
		return fmt.Errorf("no inline plan available")
	}

	plan := m.inlinePlan.Plan

	// Sync tasks to instances first
	if err := m.syncPlanTasksToInstances(); err != nil {
		return err
	}

	// Start instances in execution order
	for _, taskGroup := range plan.ExecutionOrder {
		for _, taskID := range taskGroup {
			instanceID, ok := m.inlinePlan.TaskToInstance[taskID]
			if !ok {
				continue
			}

			inst := m.orchestrator.GetInstance(instanceID)
			if inst == nil || inst.Status != orchestrator.StatusPending {
				continue
			}

			// Set up dependencies
			task := orchestrator.GetTaskByID(plan, taskID)
			if task != nil {
				for _, depTaskID := range task.DependsOn {
					if depInstID, exists := m.inlinePlan.TaskToInstance[depTaskID]; exists {
						inst.DependsOn = append(inst.DependsOn, depInstID)
					}
				}
			}

			// Start the instance (orchestrator will handle dependency waiting)
			if err := m.orchestrator.StartInstance(inst); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to start task %s: %v", taskID, err)
			}
		}
	}

	return nil
}

// confirmInlinePlanAndExecute confirms the inline plan and starts execution.
// Called when user presses enter in the plan editor to confirm the plan.
func (m *Model) confirmInlinePlanAndExecute() error {
	if m.inlinePlan == nil || m.inlinePlan.Plan == nil {
		return fmt.Errorf("no inline plan available")
	}

	// Validate plan before executing
	validation := orchestrator.ValidatePlanForEditor(m.inlinePlan.Plan)
	if validation.HasErrors() {
		return fmt.Errorf("plan has %d validation errors", validation.ErrorCount)
	}

	// Start execution
	if err := m.startInlinePlanExecution(); err != nil {
		return err
	}

	// Exit plan editor
	m.exitPlanEditor()
	m.infoMessage = fmt.Sprintf("Execution started: %d tasks", len(m.inlinePlan.Plan.Tasks))

	// Log plan execution
	if m.logger != nil {
		m.logger.Info("inline plan execution started",
			"task_count", len(m.inlinePlan.Plan.Tasks),
			"objective", m.inlinePlan.Objective)
	}

	return nil
}

// handleInlinePlanTaskDelete handles deleting a task from the inline plan.
// If the task has a started instance, requires confirmation.
// Integration note: This should be called from plan editor when deleting
// a task in inline mode (D key).
//
//nolint:unused // Integration with plan editor pending
func (m *Model) handleInlinePlanTaskDelete(plan *orchestrator.PlanSpec, taskID string) error {
	if m.inlinePlan == nil {
		return fmt.Errorf("not in inline plan mode")
	}

	// Check if this task has an instance
	instanceID, hasInstance := m.inlinePlan.TaskToInstance[taskID]
	if hasInstance {
		inst := m.orchestrator.GetInstance(instanceID)
		if inst != nil && inst.Status != orchestrator.StatusPending {
			// Instance has started - require confirmation
			m.planEditor.pendingConfirmDelete = taskID
			m.infoMessage = "Task has started instance. Press 'D' again to confirm deletion."
			return nil
		}

		// Remove pending instance
		_ = m.orchestrator.RemoveInstance(m.session, instanceID, true)
		delete(m.inlinePlan.TaskToInstance, taskID)
	}

	// Delete from plan
	return orchestrator.DeleteTask(plan, taskID)
}

// handleInlinePlanTaskAdd handles adding a task to the inline plan.
// Creates a pending instance in the plan's group.
// Integration note: This should be called from plan editor when adding
// a new task in inline mode (n key).
//
//nolint:unused // Integration with plan editor pending
func (m *Model) handleInlinePlanTaskAdd(plan *orchestrator.PlanSpec, afterTaskID string, newTask orchestrator.PlannedTask) error {
	if m.inlinePlan == nil {
		return fmt.Errorf("not in inline plan mode")
	}

	// Add to plan
	if err := orchestrator.AddTask(plan, afterTaskID, newTask); err != nil {
		return err
	}

	// Create a pending instance for the new task
	inst, err := m.orchestrator.AddInstance(m.session, newTask.Description)
	if err != nil {
		// Rollback plan change
		_ = orchestrator.DeleteTask(plan, newTask.ID)
		return fmt.Errorf("failed to create instance: %w", err)
	}

	inst.Task = newTask.Title
	m.inlinePlan.TaskToInstance[newTask.ID] = inst.ID

	// Add to group
	gm := m.getGroupManager()
	if gm != nil && m.inlinePlan.GroupID != "" {
		gm.MoveInstanceToGroup(inst.ID, m.inlinePlan.GroupID)
	}

	return nil
}

// handleInlinePlanTaskReorder handles reordering tasks in the inline plan.
// Updates instance dependencies based on new task order.
// Integration note: This should be called from plan editor when reordering
// tasks in inline mode (J/K keys).
//
//nolint:unused // Integration with plan editor pending
func (m *Model) handleInlinePlanTaskReorder(plan *orchestrator.PlanSpec) error {
	if m.inlinePlan == nil {
		return fmt.Errorf("not in inline plan mode")
	}

	// Recalculate execution order based on dependencies
	// EnsurePlanComputed will rebuild ExecutionOrder if needed
	orchestrator.EnsurePlanComputed(plan)

	// Update instance dependencies to match new order
	for _, task := range plan.Tasks {
		instanceID, ok := m.inlinePlan.TaskToInstance[task.ID]
		if !ok {
			continue
		}

		inst := m.orchestrator.GetInstance(instanceID)
		if inst == nil {
			continue
		}

		// Clear and rebuild dependencies
		inst.DependsOn = nil
		for _, depTaskID := range task.DependsOn {
			if depInstID, exists := m.inlinePlan.TaskToInstance[depTaskID]; exists {
				inst.DependsOn = append(inst.DependsOn, depInstID)
			}
		}
	}

	return nil
}
