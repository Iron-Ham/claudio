package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/group"
	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
	"github.com/Iron-Ham/claudio/internal/tui/command"
	tuimsg "github.com/Iron-Ham/claudio/internal/tui/msg"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	"github.com/Iron-Ham/claudio/internal/util"
)

// initInlinePlanMode initializes inline plan mode when :plan command is executed.
// This prompts for an objective and will create a planning instance.
// Multiple plan sessions are supported - each gets its own group.
func (m *Model) initInlinePlanMode() {
	// Initialize the container if needed
	if m.inlinePlan == nil {
		m.inlinePlan = NewInlinePlanState()
	}

	// Create a new session with a temporary ID (will be updated when group is created)
	tempID := orchestrator.GenerateID()
	session := &InlinePlanSession{
		AwaitingObjective: true,
		TaskToInstance:    make(map[string]string),
	}
	m.inlinePlan.AddSession(tempID, session)

	// Prompt for objective using task input UI
	m.addingTask = true
	m.taskInput = ""
	m.taskInputCursor = 0

	// Show different message if multiple plans are active
	if m.inlinePlan.GetSessionCount() > 1 {
		m.infoMessage = "Enter objective for additional plan:"
	} else {
		m.infoMessage = "Enter plan objective:"
	}
}

// initInlineMultiPlanMode initializes inline multi-pass plan mode when :multiplan command is executed.
// This prompts for an objective and will create multiple planning instances (one per strategy)
// plus a plan manager instance that evaluates and merges the best plan.
// Multiple multiplan sessions are supported.
func (m *Model) initInlineMultiPlanMode() {
	// Initialize the container if needed
	if m.inlinePlan == nil {
		m.inlinePlan = NewInlinePlanState()
	}

	// Create a new session with a temporary ID (will be updated when group is created)
	tempID := orchestrator.GenerateID()
	session := &InlinePlanSession{
		AwaitingObjective: true,
		TaskToInstance:    make(map[string]string),
		MultiPass:         true,
		ProcessedPlanners: make(map[int]bool),
	}
	m.inlinePlan.AddSession(tempID, session)

	// Prompt for objective using task input UI
	m.addingTask = true
	m.taskInput = ""
	m.taskInputCursor = 0

	// Show different message if multiple plans are active
	if m.inlinePlan.GetSessionCount() > 1 {
		m.infoMessage = "Enter objective for additional multiplan (3 planners + 1 assessor):"
	} else {
		m.infoMessage = "Enter multiplan objective (3 planners + 1 assessor):"
	}
}

// initInlineUltraPlanMode initializes inline ultraplan mode when :ultraplan command is executed.
// This creates the UltraPlan coordinator and enters the planning workflow.
func (m *Model) initInlineUltraPlanMode(result command.Result) {
	// Start with default config to get proper defaults (e.g., RequireVerifiedCommits: true)
	ultraCfg := orchestrator.DefaultUltraPlanConfig()
	ultraCfg.AutoApprove = false // Default to requiring approval in inline mode
	ultraCfg.Review = true       // Always review in inline mode

	// Apply config file settings (same as CLI ultraplan does)
	// Use Load() instead of Get() so we can warn users if config fails to load
	appCfg, err := config.Load()
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to load config file, using defaults for ultraplan",
				"error", err)
		}
		appCfg = config.Default()
	}
	ultraCfg.MaxParallel = appCfg.Ultraplan.MaxParallel
	ultraCfg.MultiPass = appCfg.Ultraplan.MultiPass
	if appCfg.Ultraplan.ConsolidationMode != "" {
		ultraCfg.ConsolidationMode = orchestrator.ConsolidationMode(appCfg.Ultraplan.ConsolidationMode)
	}
	ultraCfg.CreateDraftPRs = appCfg.Ultraplan.CreateDraftPRs
	if len(appCfg.Ultraplan.PRLabels) > 0 {
		ultraCfg.PRLabels = appCfg.Ultraplan.PRLabels
	}
	ultraCfg.BranchPrefix = appCfg.Ultraplan.BranchPrefix
	ultraCfg.MaxTaskRetries = appCfg.Ultraplan.MaxTaskRetries
	ultraCfg.RequireVerifiedCommits = appCfg.Ultraplan.RequireVerifiedCommits

	// Command flags override config file settings
	if result.UltraPlanMultiPass != nil && *result.UltraPlanMultiPass {
		ultraCfg.MultiPass = true
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
		ultraSession := orchestrator.NewUltraPlanSession("", ultraCfg)
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
		if ultraCfg.MultiPass {
			sessionType = orchestrator.SessionTypePlanMulti
		}
		objective := plan.Objective
		if objective == "" {
			objective = "Loaded Plan"
		}
		ultraGroup := orchestrator.NewInstanceGroupWithType(
			util.TruncateString(objective, 30),
			sessionType,
			objective,
		)
		m.session.AddGroup(ultraGroup)
		ultraSession.GroupID = ultraGroup.ID

		// Auto-enable grouped sidebar mode
		m.autoEnableGroupedMode()

		m.ultraPlan = &view.UltraPlanState{
			Coordinator:           coordinator,
			ShowPlanView:          false,
			LastAutoExpandedGroup: -1,
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
		ultraSession := orchestrator.NewUltraPlanSession(objective, ultraCfg)

		// Initialize coordinator
		coordinator := orchestrator.NewCoordinator(m.orchestrator, m.session, ultraSession, m.logger)

		// Create a group for this ultraplan session
		sessionType := orchestrator.SessionTypeUltraPlan
		if ultraCfg.MultiPass {
			sessionType = orchestrator.SessionTypePlanMulti
		}
		ultraGroup := orchestrator.NewInstanceGroupWithType(
			util.TruncateString(objective, 30),
			sessionType,
			objective,
		)
		m.session.AddGroup(ultraGroup)
		ultraSession.GroupID = ultraGroup.ID

		// Auto-enable grouped sidebar mode
		m.autoEnableGroupedMode()

		// Start planning phase - instances will be added to the group we just created
		if err := coordinator.RunPlanning(); err != nil {
			// Remove the group on failure to avoid orphaned empty groups
			m.session.RemoveGroup(ultraGroup.ID)
			m.errorMessage = fmt.Sprintf("Failed to start planning: %v", err)
			return
		}

		m.ultraPlan = &view.UltraPlanState{
			Coordinator:           coordinator,
			ShowPlanView:          false,
			LastAutoExpandedGroup: -1,
		}
		m.infoMessage = "Planning started..."
		return
	}

	// No objective provided - prompt for one using inline plan state
	// Mark as ultraplan so the objective handler creates the ultraplan coordinator
	if m.inlinePlan == nil {
		m.inlinePlan = NewInlinePlanState()
	}

	tempID := orchestrator.GenerateID()
	session := &InlinePlanSession{
		AwaitingObjective: true,
		TaskToInstance:    make(map[string]string),
		IsUltraPlan:       true,
		UltraPlanConfig:   &ultraCfg,
	}
	m.inlinePlan.AddSession(tempID, session)

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

// toggleGraphView toggles between the dependency graph view and the previous view mode.
// The graph view displays instances organized by their dependency levels.
// When exiting graph view, it restores the previous mode (flat or grouped).
func (m *Model) toggleGraphView() {
	switch m.sidebarMode {
	case view.SidebarModeGraph:
		// Switch back to previous mode (flat or grouped)
		m.sidebarMode = m.previousSidebarMode
		if m.sidebarMode == view.SidebarModeGrouped {
			m.infoMessage = "Grouped view enabled"
		} else {
			m.infoMessage = "List view enabled"
		}
	default:
		// Save current mode and switch to graph mode
		m.previousSidebarMode = m.sidebarMode
		m.sidebarMode = view.SidebarModeGraph
		m.infoMessage = "Dependency graph view enabled"
	}
}

// handleInlinePlanObjectiveSubmit handles submission of the plan objective.
// Called when user presses enter after typing an objective in inline plan mode.
// Called from the task input handler in app.go when a session has AwaitingObjective true.
func (m *Model) handleInlinePlanObjectiveSubmit(objective string) {
	if m.inlinePlan == nil {
		if m.logger != nil {
			m.logger.Warn("handleInlinePlanObjectiveSubmit called with nil inlinePlan",
				"objective", objective)
		}
		m.errorMessage = "Unable to submit objective: plan state lost"
		return
	}

	// Get the session awaiting objective input
	session := m.inlinePlan.GetAwaitingObjectiveSession()
	if session == nil {
		if m.logger != nil {
			m.logger.Warn("handleInlinePlanObjectiveSubmit called but no session awaiting objective",
				"objective", objective)
		}
		m.errorMessage = "Unable to submit objective: no active session"
		return
	}

	// Store old session ID before updating (it may be a temp ID)
	oldSessionID := session.GroupID

	session.AwaitingObjective = false
	session.Objective = objective
	session.AwaitingPlanCreation = true

	// Create a planning instance to generate the plan
	inst, err := m.orchestrator.AddInstance(m.session, m.createPlanningPrompt(objective))
	if err != nil {
		m.errorMessage = fmt.Sprintf("Failed to create planning instance: %v", err)
		m.inlinePlan.RemoveSession(oldSessionID)
		return
	}

	session.PlanningInstanceID = inst.ID

	// Start the planning instance
	if err := m.orchestrator.StartInstance(inst); err != nil {
		m.errorMessage = fmt.Sprintf("Failed to start planning: %v", err)
		m.inlinePlan.RemoveSession(oldSessionID)
		return
	}

	// Create a group for this plan's tasks
	gm := m.getGroupManager()
	if gm != nil {
		planGroup := gm.CreateGroup(fmt.Sprintf("Plan: %s", util.TruncateString(objective, 30)), nil)

		// Update session to use the new group ID
		delete(m.inlinePlan.Sessions, oldSessionID)
		session.GroupID = planGroup.ID
		m.inlinePlan.Sessions[planGroup.ID] = session
		m.inlinePlan.CurrentSessionID = planGroup.ID

		// Set session type on the group for proper icon display
		if orchGroup := m.session.GetGroup(planGroup.ID); orchGroup != nil {
			orchestrator.SetSessionType(orchGroup, orchestrator.SessionTypePlan)
			orchGroup.Objective = objective
			// Add the planning instance to the group for sidebar display
			orchGroup.AddInstance(inst.ID)
		}

		// Initialize sidebar state tracking for plan sessions
		m.initPlanSidebarSession(planGroup.ID, objective, inst.ID)

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
	if idx := m.findInstanceIndexByID(inst.ID); idx >= 0 {
		m.activeTab = idx
	}
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

	// Get the session awaiting objective input
	session := m.inlinePlan.GetAwaitingObjectiveSession()
	if session == nil || !session.MultiPass {
		if m.logger != nil {
			m.logger.Warn("handleMultiPlanObjectiveSubmit called but no multipass session awaiting objective",
				"objective", objective)
		}
		m.errorMessage = "Unable to submit objective: no multipass session active"
		return
	}

	// Store old session ID before updating (it may be a temp ID)
	oldSessionID := session.GroupID

	session.AwaitingObjective = false
	session.Objective = objective
	session.AwaitingPlanCreation = true

	// Get the available strategy names
	strategies := orchestrator.GetMultiPassStrategyNames()
	if len(strategies) == 0 {
		m.errorMessage = "No planning strategies available"
		m.inlinePlan.RemoveSession(oldSessionID)
		return
	}

	// Initialize slices for tracking planner instances and their plans
	session.PlanningInstanceIDs = make([]string, 0, len(strategies))
	session.CandidatePlans = make([]*orchestrator.PlanSpec, len(strategies))
	session.ProcessedPlanners = make(map[int]bool)

	// Create a group for this multiplan's instances
	gm := m.getGroupManager()
	var planGroup *group.InstanceGroup
	if gm != nil {
		planGroup = gm.CreateGroup(fmt.Sprintf("MultiPlan: %s", util.TruncateString(objective, 25)), nil)

		// Update session to use the new group ID
		delete(m.inlinePlan.Sessions, oldSessionID)
		session.GroupID = planGroup.ID
		m.inlinePlan.Sessions[planGroup.ID] = session
		m.inlinePlan.CurrentSessionID = planGroup.ID

		// Set session type on the group for proper icon display (use multi-pass icon)
		if orchGroup := m.session.GetGroup(planGroup.ID); orchGroup != nil {
			orchestrator.SetSessionType(orchGroup, orchestrator.SessionTypePlanMulti)
			orchGroup.Objective = objective
		}

		// Initialize sidebar state tracking for multiplan sessions
		m.initMultiPlanSidebarSession(planGroup.ID, objective)

		// If in tripleshot mode, register this group for sidebar display
		if m.tripleShot != nil {
			m.tripleShot.PlanGroupIDs = append(m.tripleShot.PlanGroupIDs, planGroup.ID)
		}
	}

	// Create and start an instance for each planning strategy
	var firstInstanceID string
	for i, strategy := range strategies {
		// Build the strategy-specific prompt
		stratPrompt := orchestrator.GetMultiPassPlanningPrompt(strategy, objective)

		// Create a planning instance for this strategy
		inst, err := m.orchestrator.AddInstance(m.session, stratPrompt)
		if err != nil {
			m.errorMessage = fmt.Sprintf("Failed to create planning instance for strategy %s: %v", strategy, err)
			m.inlinePlan.RemoveSession(session.GroupID)
			return
		}

		// Set task name to identify the strategy
		inst.Task = fmt.Sprintf("Planning (%s)", strategy)

		// Store the instance ID
		session.PlanningInstanceIDs = append(session.PlanningInstanceIDs, inst.ID)

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
			m.inlinePlan.RemoveSession(session.GroupID)
			return
		}

		// Update sidebar state to show planner as working
		if planGroup != nil {
			m.updateMultiPlanSidebarPlannerStatus(planGroup.ID, i, view.PlannerStatusWorking, inst.ID)
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
		if idx := m.findInstanceIndexByID(firstInstanceID); idx >= 0 {
			m.activeTab = idx
		}
		m.ensureActiveVisible()
		m.resumeActiveInstance()
	}
}

// handleInlineMultiPlanCompletion handles completion of multiplan instances.
// Returns true if the instance was part of a multiplan workflow and was handled.
func (m *Model) handleInlineMultiPlanCompletion(inst *orchestrator.Instance) bool {
	if m.inlinePlan == nil {
		return false
	}

	// Find the multiplan session that owns this instance
	for _, session := range m.inlinePlan.Sessions {
		if !session.MultiPass {
			continue
		}

		// Check if this is one of the planner instances
		if session.AwaitingPlanCreation {
			for _, plannerID := range session.PlanningInstanceIDs {
				if plannerID == inst.ID {
					return m.handleInlineMultiPlanPlannerCompletionForSession(inst, session)
				}
			}
		}

		// Check if this is the plan manager instance
		if session.AwaitingPlanManager && session.PlanManagerInstanceID == inst.ID {
			return m.handleInlineMultiPlanManagerCompletionForSession(inst, session)
		}
	}

	return false
}

// handleInlineMultiPlanPlannerCompletionForSession handles completion of one multiplan planner for a specific session.
func (m *Model) handleInlineMultiPlanPlannerCompletionForSession(inst *orchestrator.Instance, session *InlinePlanSession) bool {
	if session == nil || !session.MultiPass || !session.AwaitingPlanCreation {
		return false
	}

	// Check if this instance is one of our planners
	plannerIndex := -1
	for i, plannerID := range session.PlanningInstanceIDs {
		if plannerID == inst.ID {
			plannerIndex = i
			break
		}
	}

	if plannerIndex == -1 {
		return false // Not one of our planners
	}

	// Mark this planner as processed
	if session.ProcessedPlanners == nil {
		session.ProcessedPlanners = make(map[int]bool)
	}
	session.ProcessedPlanners[plannerIndex] = true

	// Try to parse the plan from this planner
	strategyNames := orchestrator.GetMultiPassStrategyNames()
	strategyName := "unknown"
	if plannerIndex < len(strategyNames) {
		strategyName = strategyNames[plannerIndex]
	}

	planPath := orchestrator.PlanFilePath(inst.WorktreePath)
	plan, err := orchestrator.ParsePlanFromFile(planPath, session.Objective)
	if err != nil {
		// Try output parsing as fallback
		output := m.outputManager.GetOutput(inst.ID)
		plan, err = orchestrator.ParsePlanFromOutput(output, session.Objective)
	}

	if err != nil {
		if m.logger != nil {
			m.logger.Warn("multiplan planner failed to produce valid plan",
				"planner_index", plannerIndex,
				"strategy", strategyName,
				"error", err)
		}
		// Store nil to track completion even on failure
		session.CandidatePlans[plannerIndex] = nil
	} else {
		session.CandidatePlans[plannerIndex] = plan
		m.infoMessage = fmt.Sprintf("Plan %d/%d collected (%s): %d tasks",
			len(session.ProcessedPlanners), len(session.PlanningInstanceIDs),
			strategyName, len(plan.Tasks))
	}

	// Check if all planners have completed
	if len(session.ProcessedPlanners) >= len(session.PlanningInstanceIDs) {
		// Count valid plans
		validPlans := 0
		for _, p := range session.CandidatePlans {
			if p != nil {
				validPlans++
			}
		}

		if validPlans == 0 {
			m.errorMessage = "All multiplan planners failed to produce valid plans"
			m.inlinePlan.RemoveSession(session.GroupID)
			return true
		}

		// Start the plan manager to evaluate and merge plans
		m.startInlineMultiPlanManagerForSession(session)
	}

	return true
}

// startInlineMultiPlanManagerForSession creates and starts the plan manager instance for a session.
func (m *Model) startInlineMultiPlanManagerForSession(session *InlinePlanSession) {
	if session == nil || !session.MultiPass {
		return
	}

	session.AwaitingPlanCreation = false
	session.AwaitingPlanManager = true

	// Build the plan manager prompt
	managerPrompt := m.buildInlineMultiPlanManagerPromptForSession(session)

	// Create the plan manager instance
	inst, err := m.orchestrator.AddInstance(m.session, managerPrompt)
	if err != nil {
		m.errorMessage = fmt.Sprintf("Failed to create plan manager: %v", err)
		m.inlinePlan.RemoveSession(session.GroupID)
		return
	}

	inst.Task = "Plan Manager (evaluating)"
	session.PlanManagerInstanceID = inst.ID

	// Add to the multiplan group
	if session.GroupID != "" {
		if orchGroup := m.session.GetGroup(session.GroupID); orchGroup != nil {
			orchGroup.AddInstance(inst.ID)
		}
	}

	// Start the manager instance
	if err := m.orchestrator.StartInstance(inst); err != nil {
		m.errorMessage = fmt.Sprintf("Failed to start plan manager: %v", err)
		m.inlinePlan.RemoveSession(session.GroupID)
		return
	}

	m.infoMessage = "Plan manager started - comparing strategies to select the best plan..."

	if m.logger != nil {
		m.logger.Info("started multiplan manager instance",
			"instance_id", inst.ID,
			"candidate_plans", len(session.CandidatePlans))
	}
}

// buildInlineMultiPlanManagerPromptForSession builds the prompt for the plan manager for a session.
func (m *Model) buildInlineMultiPlanManagerPromptForSession(session *InlinePlanSession) string {
	var plansSection strings.Builder

	strategyNames := orchestrator.GetMultiPassStrategyNames()
	for i, plan := range session.CandidatePlans {
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

	return fmt.Sprintf(prompt.PlanManagerPromptTemplate, session.Objective, plansSection.String())
}

// handleInlineMultiPlanManagerCompletionForSession handles completion of the plan manager for a session.
func (m *Model) handleInlineMultiPlanManagerCompletionForSession(inst *orchestrator.Instance, session *InlinePlanSession) bool {
	if session == nil || !session.MultiPass || !session.AwaitingPlanManager {
		return false
	}

	if session.PlanManagerInstanceID != inst.ID {
		return false
	}

	// Parse the final plan from the manager
	planPath := orchestrator.PlanFilePath(inst.WorktreePath)
	plan, err := orchestrator.ParsePlanFromFile(planPath, session.Objective)
	if err != nil {
		// Try output parsing as fallback
		output := m.outputManager.GetOutput(inst.ID)
		plan, err = orchestrator.ParsePlanFromOutput(output, session.Objective)
	}

	if err != nil {
		m.errorMessage = fmt.Sprintf("Plan manager completed but failed to parse final plan: %v", err)
		m.inlinePlan.RemoveSession(session.GroupID)
		return true
	}

	// Store the final plan
	session.Plan = plan
	session.AwaitingPlanManager = false

	// Set this as the current session for the plan editor
	m.inlinePlan.CurrentSessionID = session.GroupID

	// Enter the plan editor for review
	m.enterInlinePlanEditor()
	m.infoMessage = fmt.Sprintf("MultiPlan complete: %d tasks. Edit and press [enter] to start execution.", len(plan.Tasks))

	if m.logger != nil {
		m.logger.Info("multiplan manager completed",
			"task_count", len(plan.Tasks),
			"objective", session.Objective)
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

	// Get the session awaiting objective input
	session := m.inlinePlan.GetAwaitingObjectiveSession()
	if session == nil || !session.IsUltraPlan {
		if m.logger != nil {
			m.logger.Warn("handleUltraPlanObjectiveSubmit called but no ultraplan session awaiting objective",
				"objective", objective)
		}
		m.errorMessage = "Unable to submit objective: no ultraplan session active"
		return
	}

	// Get config from the session
	cfg := orchestrator.UltraPlanConfig{
		AutoApprove: false,
		Review:      true,
	}
	if session.UltraPlanConfig != nil {
		cfg = *session.UltraPlanConfig
	}

	// Remove the session - we're transitioning to ultraplan mode
	m.inlinePlan.RemoveSession(session.GroupID)

	// Create ultraplan session
	ultraSession := orchestrator.NewUltraPlanSession(objective, cfg)

	// Initialize coordinator
	coordinator := orchestrator.NewCoordinator(m.orchestrator, m.session, ultraSession, m.logger)

	// Create the group BEFORE starting planning so that instances created
	// during RunPlanning() get added to the group for proper sidebar display
	sessionType := orchestrator.SessionTypeUltraPlan
	if cfg.MultiPass {
		sessionType = orchestrator.SessionTypePlanMulti
	}
	ultraGroup := orchestrator.NewInstanceGroupWithType(
		util.TruncateString(objective, 30),
		sessionType,
		objective,
	)
	m.session.AddGroup(ultraGroup)
	ultraSession.GroupID = ultraGroup.ID

	// Auto-enable grouped sidebar mode
	m.autoEnableGroupedMode()

	// Start planning phase - instances will be added to the group we just created
	if err := coordinator.RunPlanning(); err != nil {
		// Remove the group on failure to avoid orphaned empty groups
		m.session.RemoveGroup(ultraGroup.ID)
		if m.logger != nil {
			m.logger.Error("failed to start ultraplan planning",
				"objective", objective,
				"multiPass", cfg.MultiPass,
				"error", err)
		}
		m.errorMessage = fmt.Sprintf("Failed to start planning: %v", err)
		return
	}

	m.ultraPlan = &view.UltraPlanState{
		Coordinator:           coordinator,
		ShowPlanView:          false,
		LastAutoExpandedGroup: -1,
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

// getGroupManager returns the group manager for the session, creating one if needed.
func (m *Model) getGroupManager() *group.Manager {
	if m.session == nil {
		return nil
	}
	// Create adapter that implements ManagerSessionData.
	// Since orchestrator.InstanceGroup and group.InstanceGroup are both type aliases
	// to grouptypes.InstanceGroup, no conversion is needed - they're the same type.
	adapter := &sessionGroupAdapter{session: m.session}
	return group.NewManager(adapter)
}

// sessionGroupAdapter adapts orchestrator.Session to group.ManagerSessionData.
// No type conversion is needed since both packages use grouptypes.InstanceGroup.
type sessionGroupAdapter struct {
	session *orchestrator.Session
}

func (a *sessionGroupAdapter) GetGroups() []*group.InstanceGroup {
	if a.session == nil {
		return nil
	}
	return a.session.GetGroups()
}

func (a *sessionGroupAdapter) SetGroups(groups []*group.InstanceGroup) {
	if a.session == nil {
		return
	}
	a.session.SetGroups(groups)
}

func (a *sessionGroupAdapter) GenerateID() string {
	return orchestrator.GenerateID()
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

// syncPlanTasksToInstances synchronizes plan tasks to instances in the group.
// Creates pending instances for new tasks, removes instances for deleted tasks.
func (m *Model) syncPlanTasksToInstances() error {
	session := m.getCurrentPlanSession()
	if session == nil || session.Plan == nil {
		return fmt.Errorf("no inline plan available")
	}

	plan := session.Plan
	gm := m.getGroupManager()
	if gm == nil {
		return fmt.Errorf("no group manager available")
	}

	// Get or create the plan group
	var planGroup *group.InstanceGroup
	if session.GroupID != "" {
		planGroup = gm.GetGroup(session.GroupID)
	}
	if planGroup == nil {
		planGroup = gm.CreateGroup(fmt.Sprintf("Plan: %s", util.TruncateString(session.Objective, 30)), nil)
		session.GroupID = planGroup.ID
	}

	// Track which task IDs currently have instances
	existingTasks := make(map[string]bool)
	for taskID := range session.TaskToInstance {
		existingTasks[taskID] = true
	}

	// Create instances for new tasks
	for _, task := range plan.Tasks {
		if _, exists := session.TaskToInstance[task.ID]; !exists {
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
			session.TaskToInstance[task.ID] = inst.ID
		}
		delete(existingTasks, task.ID)
	}

	// Remove instances for deleted tasks (only if not started)
	for taskID := range existingTasks {
		instanceID := session.TaskToInstance[taskID]
		inst := m.orchestrator.GetInstance(instanceID)
		if inst != nil && inst.Status == orchestrator.StatusPending {
			_ = m.orchestrator.RemoveInstance(m.session, instanceID, true)
		}
		delete(session.TaskToInstance, taskID)
	}

	return nil
}

// startInlinePlanExecution starts execution of the inline plan.
// Creates instances for all tasks and organizes them into groups based on dependencies.
func (m *Model) startInlinePlanExecution() error {
	session := m.getCurrentPlanSession()
	if session == nil || session.Plan == nil {
		return fmt.Errorf("no inline plan available")
	}

	plan := session.Plan

	// Sync tasks to instances first
	if err := m.syncPlanTasksToInstances(); err != nil {
		return err
	}

	// Start instances in execution order
	for _, taskGroup := range plan.ExecutionOrder {
		for _, taskID := range taskGroup {
			instanceID, ok := session.TaskToInstance[taskID]
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
					if depInstID, exists := session.TaskToInstance[depTaskID]; exists {
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
	session := m.getCurrentPlanSession()
	if session == nil || session.Plan == nil {
		return fmt.Errorf("no inline plan available")
	}

	// Validate plan before executing
	validation := orchestrator.ValidatePlanForEditor(session.Plan)
	if validation.HasErrors() {
		return fmt.Errorf("plan has %d validation errors", validation.ErrorCount)
	}

	// Start execution
	if err := m.startInlinePlanExecution(); err != nil {
		return err
	}

	// Exit plan editor
	m.exitPlanEditor()
	m.infoMessage = fmt.Sprintf("Execution started: %d tasks", len(session.Plan.Tasks))

	// Log plan execution
	if m.logger != nil {
		m.logger.Info("inline plan execution started",
			"task_count", len(session.Plan.Tasks),
			"objective", session.Objective)
	}

	return nil
}

// dispatchInlineMultiPlanFileChecks returns commands that check for plan files from inline multiplan planners.
// This enables plan file detection for the :multiplan command, similar to how checkMultiPassPlanFilesAsync
// works for :ultraplan. Each returned command checks one planner's plan file asynchronously.
func (m *Model) dispatchInlineMultiPlanFileChecks() []tea.Cmd {
	if m.inlinePlan == nil {
		return nil
	}

	var cmds []tea.Cmd

	// Check all multiplan sessions that are awaiting plan creation
	for _, session := range m.inlinePlan.Sessions {
		if !session.MultiPass || !session.AwaitingPlanCreation {
			continue
		}

		// Skip if we don't have planner IDs yet
		numPlanners := len(session.PlanningInstanceIDs)
		if numPlanners == 0 {
			continue
		}

		// Capture needed data for async operations
		plannerIDs := make([]string, len(session.PlanningInstanceIDs))
		copy(plannerIDs, session.PlanningInstanceIDs)

		processedPlanners := make(map[int]bool)
		for k, v := range session.ProcessedPlanners {
			processedPlanners[k] = v
		}

		objective := session.Objective
		strategyNames := orchestrator.GetMultiPassStrategyNames()
		orc := m.orchestrator

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

			cmds = append(cmds, checkInlineMultiPlanFileAsync(orc, instID, idx, strategyName, objective, session.GroupID))
		}
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
	groupID string,
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
			GroupID:      groupID,
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
	if m.inlinePlan == nil {
		return m, nil
	}

	// Find the session by group ID
	session := m.inlinePlan.GetSession(msg.GroupID)
	if session == nil || !session.MultiPass || !session.AwaitingPlanCreation {
		return m, nil
	}

	// Verify index is valid
	if msg.Index < 0 || msg.Index >= len(session.PlanningInstanceIDs) {
		return m, nil
	}

	// Skip if already processed
	if session.ProcessedPlanners[msg.Index] {
		return m, nil
	}

	// Mark this planner as processed
	session.ProcessedPlanners[msg.Index] = true

	// Store the plan (may be nil if parsing failed)
	if msg.Index < len(session.CandidatePlans) {
		session.CandidatePlans[msg.Index] = msg.Plan
	}

	numPlanners := len(session.PlanningInstanceIDs)
	taskCount := 0
	if msg.Plan != nil {
		taskCount = len(msg.Plan.Tasks)
	}
	m.infoMessage = fmt.Sprintf("Plan %d/%d collected (%s): %d tasks",
		len(session.ProcessedPlanners), numPlanners,
		msg.StrategyName, taskCount)

	if m.logger != nil {
		m.logger.Info("inline multiplan: plan file detected",
			"planner_index", msg.Index,
			"strategy", msg.StrategyName,
			"task_count", taskCount)
	}

	// Check if all planners have completed
	if len(session.ProcessedPlanners) >= numPlanners {
		// Count valid plans
		validPlans := 0
		for _, p := range session.CandidatePlans {
			if p != nil {
				validPlans++
			}
		}

		if validPlans == 0 {
			m.errorMessage = "All multiplan planners failed to produce valid plans"
			m.inlinePlan.RemoveSession(session.GroupID)
			return m, nil
		}

		// Start the plan manager to evaluate and merge plans
		m.startInlineMultiPlanManagerForSession(session)
	}

	return m, nil
}

// initPlanSidebarSession initializes sidebar state tracking for a plan session.
// This creates a view.PlanSession that enables sidebar rendering of the plan's progress.
func (m *Model) initPlanSidebarSession(groupID, objective, plannerID string) {
	if m.planSessions == nil {
		m.planSessions = view.NewPlanState()
	}

	session := &view.PlanSession{
		GroupID:       groupID,
		Objective:     objective,
		Phase:         view.PlanPhasePlanning,
		PlannerID:     plannerID,
		PlannerStatus: view.PlannerStatusWorking,
	}
	m.planSessions.AddSession(session)
}

// initMultiPlanSidebarSession initializes sidebar state tracking for a multiplan session.
// This creates a view.MultiPlanSession that enables sidebar rendering of the multiplan's progress.
func (m *Model) initMultiPlanSidebarSession(groupID, objective string) {
	if m.multiPlanSessions == nil {
		m.multiPlanSessions = view.NewMultiPlanState()
	}

	session := &view.MultiPlanSession{
		GroupID:         groupID,
		Objective:       objective,
		Phase:           view.MultiPlanPhasePlanning,
		PlannerStatuses: make([]view.PlannerStatus, 3), // 3 strategies
	}
	// Initialize all planners as pending (will be updated to working when started)
	for i := range session.PlannerStatuses {
		session.PlannerStatuses[i] = view.PlannerStatusPending
	}
	m.multiPlanSessions.AddSession(session)
}

// updateMultiPlanSidebarPlannerStatus updates a planner's status in the sidebar state.
func (m *Model) updateMultiPlanSidebarPlannerStatus(groupID string, plannerIndex int, status view.PlannerStatus, plannerID string) {
	if m.multiPlanSessions == nil {
		return
	}
	session := m.multiPlanSessions.GetSession(groupID)
	if session == nil {
		return
	}
	if plannerIndex >= 0 && plannerIndex < len(session.PlannerStatuses) {
		session.PlannerStatuses[plannerIndex] = status
	}
	// Track planner ID if provided
	if plannerID != "" {
		if session.PlannerIDs == nil {
			session.PlannerIDs = make([]string, 3)
		}
		if plannerIndex >= 0 && plannerIndex < len(session.PlannerIDs) {
			session.PlannerIDs[plannerIndex] = plannerID
		}
	}
}
