package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/group"
	"github.com/Iron-Ham/claudio/internal/tui/command"
	tuimsg "github.com/Iron-Ham/claudio/internal/tui/msg"
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

// initInlineMultiPlanMode initializes inline multi-pass plan mode when :multiplan command is executed.
// This prompts for an objective and will create multiple planning instances (one per strategy)
// plus a plan manager instance that evaluates and merges the best plan.
func (m *Model) initInlineMultiPlanMode() {
	m.inlinePlan = &InlinePlanState{
		AwaitingObjective: true,
		TaskToInstance:    make(map[string]string),
		MultiPass:         true,
		ProcessedPlanners: make(map[int]bool),
	}

	// Prompt for objective using task input UI
	m.addingTask = true
	m.taskInput = ""
	m.taskInputCursor = 0
	m.infoMessage = "Enter multiplan objective (3 planners + 1 assessor):"
}

// initInlineUltraPlanMode initializes inline ultraplan mode when :ultraplan command is executed.
// This creates the UltraPlan coordinator and enters the planning workflow.
func (m *Model) initInlineUltraPlanMode(result command.Result) {
	// Start with default config to get proper defaults (e.g., RequireVerifiedCommits: true)
	cfg := orchestrator.DefaultUltraPlanConfig()
	cfg.AutoApprove = false // Default to requiring approval in inline mode
	cfg.Review = true       // Always review in inline mode

	if result.UltraPlanMultiPass != nil && *result.UltraPlanMultiPass {
		cfg.MultiPass = true
	}

	// If loading from file, handle that case
	if result.UltraPlanFromFile != nil && *result.UltraPlanFromFile != "" {
		planPath := expandTildePath(*result.UltraPlanFromFile)
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
		ultraSession.GroupID = ultraGroup.ID

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
		ultraSession.GroupID = ultraGroup.ID

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

// handleMultiPlanObjectiveSubmit handles submission of a multi-pass plan objective.
// Called when user presses enter after typing an objective in multiplan mode.
// This creates multiple planning instances (one per strategy) that run in parallel.
func (m *Model) handleMultiPlanObjectiveSubmit(objective string) {
	if m.inlinePlan == nil {
		if m.logger != nil {
			m.logger.Warn("handleMultiPlanObjectiveSubmit called with nil inlinePlan",
				"objective", objective)
		}
		m.errorMessage = "Unable to submit objective: plan state lost"
		return
	}

	if !m.inlinePlan.MultiPass {
		if m.logger != nil {
			m.logger.Warn("handleMultiPlanObjectiveSubmit called for non-multipass plan",
				"objective", objective)
		}
		m.errorMessage = "Unable to submit objective: not in multiplan mode"
		return
	}

	m.inlinePlan.AwaitingObjective = false
	m.inlinePlan.Objective = objective
	m.inlinePlan.AwaitingPlanCreation = true

	// Get the available strategy names
	strategies := orchestrator.GetMultiPassStrategyNames()
	if len(strategies) == 0 {
		m.errorMessage = "No planning strategies available"
		m.inlinePlan = nil
		return
	}

	// Initialize slices for tracking planner instances and their plans
	m.inlinePlan.PlanningInstanceIDs = make([]string, 0, len(strategies))
	m.inlinePlan.CandidatePlans = make([]*orchestrator.PlanSpec, len(strategies))
	m.inlinePlan.ProcessedPlanners = make(map[int]bool)

	// Create a group for this multiplan's instances
	gm := m.getGroupManager()
	var planGroup *group.InstanceGroup
	if gm != nil {
		planGroup = gm.CreateGroup(fmt.Sprintf("MultiPlan: %s", truncateString(objective, 25)), nil)
		m.inlinePlan.GroupID = planGroup.ID

		// Set session type on the group for proper icon display (use multi-pass icon)
		if orchGroup := m.session.GetGroup(planGroup.ID); orchGroup != nil {
			orchGroup.SessionType = orchestrator.SessionTypePlanMulti
			orchGroup.Objective = objective
		}

		// If in tripleshot mode, register this group for sidebar display
		if m.tripleShot != nil {
			m.tripleShot.PlanGroupIDs = append(m.tripleShot.PlanGroupIDs, planGroup.ID)
		}
	}

	// Create and start an instance for each planning strategy
	var firstInstanceID string
	for i, strategy := range strategies {
		// Build the strategy-specific prompt
		prompt := orchestrator.GetMultiPassPlanningPrompt(strategy, objective)

		// Create a planning instance for this strategy
		inst, err := m.orchestrator.AddInstance(m.session, prompt)
		if err != nil {
			m.errorMessage = fmt.Sprintf("Failed to create planning instance for strategy %s: %v", strategy, err)
			m.inlinePlan = nil
			return
		}

		// Set task name to identify the strategy
		inst.Task = fmt.Sprintf("Planning (%s)", strategy)

		// Store the instance ID
		m.inlinePlan.PlanningInstanceIDs = append(m.inlinePlan.PlanningInstanceIDs, inst.ID)

		// Track first instance for tab selection
		if i == 0 {
			firstInstanceID = inst.ID
		}

		// Add to group for sidebar display
		if gm != nil && planGroup != nil {
			if orchGroup := m.session.GetGroup(planGroup.ID); orchGroup != nil {
				orchGroup.AddInstance(inst.ID)
			}
		}

		// Start the instance
		if err := m.orchestrator.StartInstance(inst); err != nil {
			m.errorMessage = fmt.Sprintf("Failed to start planning instance for strategy %s: %v", strategy, err)
			m.inlinePlan = nil
			return
		}

		if m.logger != nil {
			m.logger.Info("started multiplan planner instance",
				"strategy", strategy,
				"instance_id", inst.ID,
				"objective", objective)
		}
	}

	// Auto-enable grouped sidebar mode for better visibility
	m.autoEnableGroupedMode()

	m.infoMessage = fmt.Sprintf("MultiPlan started: %d planners running in parallel...", len(strategies))

	// Pause the old active instance before switching
	if oldInst := m.activeInstance(); oldInst != nil {
		m.pauseInstance(oldInst.ID)
	}

	// Switch to the first planning instance
	if firstInstanceID != "" {
		m.activeTab = m.findInstanceIndex(firstInstanceID)
		m.ensureActiveVisible()
		m.resumeActiveInstance()
	}
}

// handleInlineMultiPlanCompletion handles completion of multiplan instances.
// Returns true if the instance was part of a multiplan workflow and was handled.
func (m *Model) handleInlineMultiPlanCompletion(inst *orchestrator.Instance) bool {
	if m.inlinePlan == nil || !m.inlinePlan.MultiPass {
		return false
	}

	// Check if this is one of the planner instances
	if m.inlinePlan.AwaitingPlanCreation {
		return m.handleInlineMultiPlanPlannerCompletion(inst)
	}

	// Check if this is the plan manager instance
	if m.inlinePlan.AwaitingPlanManager && m.inlinePlan.PlanManagerInstanceID == inst.ID {
		return m.handleInlineMultiPlanManagerCompletion(inst)
	}

	return false
}

// handleInlineMultiPlanPlannerCompletion handles completion of one of the multiplan planners.
func (m *Model) handleInlineMultiPlanPlannerCompletion(inst *orchestrator.Instance) bool {
	if m.inlinePlan == nil || !m.inlinePlan.MultiPass || !m.inlinePlan.AwaitingPlanCreation {
		return false
	}

	// Check if this instance is one of our planners
	plannerIndex := -1
	for i, plannerID := range m.inlinePlan.PlanningInstanceIDs {
		if plannerID == inst.ID {
			plannerIndex = i
			break
		}
	}

	if plannerIndex == -1 {
		return false // Not one of our planners
	}

	// Mark this planner as processed
	if m.inlinePlan.ProcessedPlanners == nil {
		m.inlinePlan.ProcessedPlanners = make(map[int]bool)
	}
	m.inlinePlan.ProcessedPlanners[plannerIndex] = true

	// Try to parse the plan from this planner
	strategyNames := orchestrator.GetMultiPassStrategyNames()
	strategyName := "unknown"
	if plannerIndex < len(strategyNames) {
		strategyName = strategyNames[plannerIndex]
	}

	planPath := orchestrator.PlanFilePath(inst.WorktreePath)
	plan, err := orchestrator.ParsePlanFromFile(planPath, m.inlinePlan.Objective)
	if err != nil {
		// Try output parsing as fallback
		output := m.outputManager.GetOutput(inst.ID)
		plan, err = orchestrator.ParsePlanFromOutput(output, m.inlinePlan.Objective)
	}

	if err != nil {
		if m.logger != nil {
			m.logger.Warn("multiplan planner failed to produce valid plan",
				"planner_index", plannerIndex,
				"strategy", strategyName,
				"error", err)
		}
		// Store nil to track completion even on failure
		m.inlinePlan.CandidatePlans[plannerIndex] = nil
	} else {
		m.inlinePlan.CandidatePlans[plannerIndex] = plan
		m.infoMessage = fmt.Sprintf("Plan %d/%d collected (%s): %d tasks",
			len(m.inlinePlan.ProcessedPlanners), len(m.inlinePlan.PlanningInstanceIDs),
			strategyName, len(plan.Tasks))
	}

	// Check if all planners have completed
	if len(m.inlinePlan.ProcessedPlanners) >= len(m.inlinePlan.PlanningInstanceIDs) {
		// Count valid plans
		validPlans := 0
		for _, p := range m.inlinePlan.CandidatePlans {
			if p != nil {
				validPlans++
			}
		}

		if validPlans == 0 {
			m.errorMessage = "All multiplan planners failed to produce valid plans"
			m.inlinePlan = nil
			return true
		}

		// Start the plan manager to evaluate and merge plans
		m.startInlineMultiPlanManager()
	}

	return true
}

// startInlineMultiPlanManager creates and starts the plan manager instance.
func (m *Model) startInlineMultiPlanManager() {
	if m.inlinePlan == nil || !m.inlinePlan.MultiPass {
		return
	}

	m.inlinePlan.AwaitingPlanCreation = false
	m.inlinePlan.AwaitingPlanManager = true

	// Build the plan manager prompt
	prompt := m.buildInlineMultiPlanManagerPrompt()

	// Create the plan manager instance
	inst, err := m.orchestrator.AddInstance(m.session, prompt)
	if err != nil {
		m.errorMessage = fmt.Sprintf("Failed to create plan manager: %v", err)
		m.inlinePlan = nil
		return
	}

	inst.Task = "Plan Manager (evaluating)"
	m.inlinePlan.PlanManagerInstanceID = inst.ID

	// Add to the multiplan group
	if m.inlinePlan.GroupID != "" {
		if orchGroup := m.session.GetGroup(m.inlinePlan.GroupID); orchGroup != nil {
			orchGroup.AddInstance(inst.ID)
		}
	}

	// Start the manager instance
	if err := m.orchestrator.StartInstance(inst); err != nil {
		m.errorMessage = fmt.Sprintf("Failed to start plan manager: %v", err)
		m.inlinePlan = nil
		return
	}

	m.infoMessage = "Plan manager started - comparing strategies to select the best plan..."

	if m.logger != nil {
		m.logger.Info("started multiplan manager instance",
			"instance_id", inst.ID,
			"candidate_plans", len(m.inlinePlan.CandidatePlans))
	}
}

// buildInlineMultiPlanManagerPrompt builds the prompt for the plan manager.
func (m *Model) buildInlineMultiPlanManagerPrompt() string {
	var plansSection strings.Builder

	strategyNames := orchestrator.GetMultiPassStrategyNames()
	for i, plan := range m.inlinePlan.CandidatePlans {
		if plan == nil {
			continue
		}

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
		for groupIdx, grp := range plan.ExecutionOrder {
			plansSection.WriteString(fmt.Sprintf("  - Group %d: %s\n", groupIdx+1, strings.Join(grp, ", ")))
		}
		plansSection.WriteString("\n---\n")
	}

	return fmt.Sprintf(orchestrator.PlanManagerPromptTemplate, m.inlinePlan.Objective, plansSection.String())
}

// handleInlineMultiPlanManagerCompletion handles completion of the plan manager.
func (m *Model) handleInlineMultiPlanManagerCompletion(inst *orchestrator.Instance) bool {
	if m.inlinePlan == nil || !m.inlinePlan.MultiPass || !m.inlinePlan.AwaitingPlanManager {
		return false
	}

	if m.inlinePlan.PlanManagerInstanceID != inst.ID {
		return false
	}

	// Parse the final plan from the manager
	planPath := orchestrator.PlanFilePath(inst.WorktreePath)
	plan, err := orchestrator.ParsePlanFromFile(planPath, m.inlinePlan.Objective)
	if err != nil {
		// Try output parsing as fallback
		output := m.outputManager.GetOutput(inst.ID)
		plan, err = orchestrator.ParsePlanFromOutput(output, m.inlinePlan.Objective)
	}

	if err != nil {
		m.errorMessage = fmt.Sprintf("Plan manager completed but failed to parse final plan: %v", err)
		m.inlinePlan = nil
		return true
	}

	// Store the final plan
	m.inlinePlan.Plan = plan
	m.inlinePlan.AwaitingPlanManager = false

	// Enter the plan editor for review
	m.enterInlinePlanEditor()
	m.infoMessage = fmt.Sprintf("MultiPlan complete: %d tasks. Edit and press [enter] to start execution.", len(plan.Tasks))

	if m.logger != nil {
		m.logger.Info("multiplan manager completed",
			"task_count", len(plan.Tasks),
			"objective", m.inlinePlan.Objective)
	}

	return true
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
	ultraSession.GroupID = ultraGroup.ID

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

// expandTildePath expands a tilde prefix (~/) to the user's home directory.
// Other path formats are returned unchanged.
func expandTildePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return home + path[1:]
		}
	}
	return path
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

// dispatchInlineMultiPlanFileChecks returns commands that check for plan files from inline multiplan planners.
// This enables plan file detection for the :multiplan command, similar to how checkMultiPassPlanFilesAsync
// works for :ultraplan. Each returned command checks one planner's plan file asynchronously.
func (m *Model) dispatchInlineMultiPlanFileChecks() []tea.Cmd {
	// Only check during inline multiplan mode when awaiting plan creation
	if m.inlinePlan == nil || !m.inlinePlan.MultiPass || !m.inlinePlan.AwaitingPlanCreation {
		return nil
	}

	// Skip if we don't have planner IDs yet
	numPlanners := len(m.inlinePlan.PlanningInstanceIDs)
	if numPlanners == 0 {
		return nil
	}

	// Capture needed data for async operations
	// (avoid accessing m.inlinePlan inside closures)
	plannerIDs := make([]string, len(m.inlinePlan.PlanningInstanceIDs))
	copy(plannerIDs, m.inlinePlan.PlanningInstanceIDs)

	processedPlanners := make(map[int]bool)
	for k, v := range m.inlinePlan.ProcessedPlanners {
		processedPlanners[k] = v
	}

	objective := m.inlinePlan.Objective
	strategyNames := orchestrator.GetMultiPassStrategyNames()
	orc := m.orchestrator

	var cmds []tea.Cmd

	// Create async check command for each planner that we haven't processed yet
	for i, plannerID := range plannerIDs {
		// Skip if we already processed this planner
		if processedPlanners[i] {
			continue
		}

		// Capture loop variables for closure
		idx := i
		instID := plannerID
		strategyName := "unknown"
		if idx < len(strategyNames) {
			strategyName = strategyNames[idx]
		}

		cmds = append(cmds, checkInlineMultiPlanFileAsync(orc, instID, idx, strategyName, objective))
	}

	return cmds
}

// checkInlineMultiPlanFileAsync returns a command that checks for a plan file from an inline multiplan planner.
// This runs async to avoid blocking the UI event loop.
func checkInlineMultiPlanFileAsync(
	orc *orchestrator.Orchestrator,
	instID string,
	idx int,
	strategyName string,
	objective string,
) tea.Cmd {
	return func() tea.Msg {
		// Get the planner instance
		inst := orc.GetInstance(instID)
		if inst == nil {
			return nil
		}

		// Check if plan file exists (async-safe)
		planPath := orchestrator.PlanFilePath(inst.WorktreePath)
		if _, err := statFile(planPath); err != nil {
			return nil
		}

		// Parse the plan
		plan, err := orchestrator.ParsePlanFromFile(planPath, objective)
		if err != nil {
			// File might be partially written, skip for now
			return nil
		}

		return tuimsg.InlineMultiPlanFileCheckResultMsg{
			Index:        idx,
			Plan:         plan,
			StrategyName: strategyName,
		}
	}
}

// statFile wraps os.Stat for testing purposes
var statFile = func(path string) (any, error) {
	return os.Stat(path)
}

// handleInlineMultiPlanFileCheckResult handles the result of async inline multiplan file checking.
// When a plan file is detected, this processes it and potentially triggers the evaluator.
func (m *Model) handleInlineMultiPlanFileCheckResult(msg tuimsg.InlineMultiPlanFileCheckResultMsg) (tea.Model, tea.Cmd) {
	// Verify we're still in inline multiplan mode
	if m.inlinePlan == nil || !m.inlinePlan.MultiPass || !m.inlinePlan.AwaitingPlanCreation {
		return m, nil
	}

	// Verify index is valid
	if msg.Index < 0 || msg.Index >= len(m.inlinePlan.PlanningInstanceIDs) {
		return m, nil
	}

	// Skip if already processed
	if m.inlinePlan.ProcessedPlanners[msg.Index] {
		return m, nil
	}

	// Mark this planner as processed
	m.inlinePlan.ProcessedPlanners[msg.Index] = true

	// Store the plan (may be nil if parsing failed)
	if msg.Index < len(m.inlinePlan.CandidatePlans) {
		m.inlinePlan.CandidatePlans[msg.Index] = msg.Plan
	}

	numPlanners := len(m.inlinePlan.PlanningInstanceIDs)
	taskCount := 0
	if msg.Plan != nil {
		taskCount = len(msg.Plan.Tasks)
	}
	m.infoMessage = fmt.Sprintf("Plan %d/%d collected (%s): %d tasks",
		len(m.inlinePlan.ProcessedPlanners), numPlanners,
		msg.StrategyName, taskCount)

	if m.logger != nil {
		m.logger.Info("inline multiplan: plan file detected",
			"planner_index", msg.Index,
			"strategy", msg.StrategyName,
			"task_count", taskCount)
	}

	// Check if all planners have completed
	if len(m.inlinePlan.ProcessedPlanners) >= numPlanners {
		// Count valid plans
		validPlans := 0
		for _, p := range m.inlinePlan.CandidatePlans {
			if p != nil {
				validPlans++
			}
		}

		if validPlans == 0 {
			m.errorMessage = "All multiplan planners failed to produce valid plans"
			m.inlinePlan = nil
			return m, nil
		}

		// Start the plan manager to evaluate and merge plans
		m.startInlineMultiPlanManager()
	}

	return m, nil
}
