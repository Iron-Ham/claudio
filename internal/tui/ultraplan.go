package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
)

// UltraPlanState is an alias to the view package's UltraPlanState.
// This maintains backwards compatibility with existing code.
type UltraPlanState = view.UltraPlanState

// checkForPhaseNotification checks if the ultraplan phase changed to one that needs user attention
// and sets the notification flag if needed. Call this from the tick handler.
func (m *Model) checkForPhaseNotification() {
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return
	}

	// Check for synthesis phase (user may want to review)
	if session.Phase == orchestrator.PhaseSynthesis && m.ultraPlan.LastNotifiedPhase != orchestrator.PhaseSynthesis {
		m.ultraPlan.NeedsNotification = true
		m.ultraPlan.LastNotifiedPhase = orchestrator.PhaseSynthesis
		return
	}

	// Check for revision phase (issues were found, user may want to know)
	if session.Phase == orchestrator.PhaseRevision && m.ultraPlan.LastNotifiedPhase != orchestrator.PhaseRevision {
		m.ultraPlan.NeedsNotification = true
		m.ultraPlan.LastNotifiedPhase = orchestrator.PhaseRevision
		return
	}

	// Check for consolidation pause (conflict detected, needs user attention)
	if session.Phase == orchestrator.PhaseConsolidating && session.Consolidation != nil {
		if session.Consolidation.Phase == orchestrator.ConsolidationPaused &&
			m.ultraPlan.LastConsolidationPhase != orchestrator.ConsolidationPaused {
			m.ultraPlan.NeedsNotification = true
			m.ultraPlan.LastConsolidationPhase = orchestrator.ConsolidationPaused
			return
		}
		// Track consolidation phase changes
		m.ultraPlan.LastConsolidationPhase = session.Consolidation.Phase
	}

	// Check for group decision needed (partial success/failure)
	// Only notify once when we enter the awaiting decision state
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		if !m.ultraPlan.NotifiedGroupDecision {
			m.ultraPlan.NeedsNotification = true
			m.ultraPlan.NotifiedGroupDecision = true
		}
		return
	}

	// Reset group decision notification flag when no longer awaiting
	// (so we can notify again if another group decision occurs later)
	if m.ultraPlan.NotifiedGroupDecision {
		m.ultraPlan.NotifiedGroupDecision = false
	}
}

// createUltraplanView creates a new ultraplan view with the current model state
func (m *Model) createUltraplanView() *view.UltraplanView {
	ctx := &view.RenderContext{
		Orchestrator: m.orchestrator,
		Session:      m.session,
		UltraPlan:    m.ultraPlan,
		ActiveTab:    m.activeTab,
		Width:        m.width,
		Height:       m.height,
		Outputs:      m.outputs,
		GetInstance: func(id string) *orchestrator.Instance {
			return m.orchestrator.GetInstance(id)
		},
		IsSelected: func(instanceID string) bool {
			return m.isInstanceSelected(instanceID)
		},
	}
	return view.NewUltraplanView(ctx)
}

// renderUltraPlanHeader renders the ultra-plan header with phase and progress
func (m Model) renderUltraPlanHeader() string {
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return m.renderHeader()
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return m.renderHeader()
	}

	v := m.createUltraplanView()
	return v.RenderHeader()
}

// renderUltraPlanSidebar renders a unified sidebar showing all phases with their instances
func (m Model) renderUltraPlanSidebar(width int, height int) string {
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		dashboardView := view.NewDashboardView()
		return dashboardView.RenderSidebar(m, width, height)
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		dashboardView := view.NewDashboardView()
		return dashboardView.RenderSidebar(m, width, height)
	}

	v := m.createUltraplanView()
	return v.RenderSidebar(width, height)
}

// isInstanceSelected checks if the given instance ID is currently selected in the TUI
func (m Model) isInstanceSelected(instanceID string) bool {
	if instanceID == "" {
		return false
	}
	if m.activeTab >= 0 && m.activeTab < len(m.session.Instances) {
		return m.session.Instances[m.activeTab].ID == instanceID
	}
	return false
}

// getNavigableInstances returns an ordered list of instance IDs that can be navigated to.
// Only includes instances from phases that have started or completed.
// Order: Planning → Execution tasks (in order) → Synthesis → Consolidation
func (m *Model) getNavigableInstances() []string {
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return nil
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return nil
	}

	var instances []string

	// Planning - navigable once started (has instance)
	if session.CoordinatorID != "" {
		inst := m.orchestrator.GetInstance(session.CoordinatorID)
		if inst != nil && inst.Status != orchestrator.StatusPending {
			instances = append(instances, session.CoordinatorID)
		}
	}

	// Plan Selection (multi-pass) - plan coordinators and plan manager
	for _, coordID := range session.PlanCoordinatorIDs {
		if coordID != "" {
			inst := m.orchestrator.GetInstance(coordID)
			if inst != nil && inst.Status != orchestrator.StatusPending {
				instances = append(instances, coordID)
			}
		}
	}
	if session.PlanManagerID != "" {
		inst := m.orchestrator.GetInstance(session.PlanManagerID)
		if inst != nil && inst.Status != orchestrator.StatusPending {
			instances = append(instances, session.PlanManagerID)
		}
	}

	// Execution - navigable for tasks with instances (started or completed)
	if session.Plan != nil {
		// Add in execution order
		for groupIdx, group := range session.Plan.ExecutionOrder {
			for _, taskID := range group {
				// Check if task has an instance (either still in TaskToInstance or was completed)
				if instID, ok := session.TaskToInstance[taskID]; ok && instID != "" {
					instances = append(instances, instID)
				} else {
					// Task might be completed - find instance by checking completed tasks
					for _, completedTaskID := range session.CompletedTasks {
						if completedTaskID == taskID {
							// Find instance for this completed task
							for _, inst := range m.session.Instances {
								if strings.Contains(inst.Task, taskID) {
									instances = append(instances, inst.ID)
									break
								}
							}
							break
						}
					}
				}
			}

			// Add group consolidator instance if it exists
			if groupIdx < len(session.GroupConsolidatorIDs) && session.GroupConsolidatorIDs[groupIdx] != "" {
				instances = append(instances, session.GroupConsolidatorIDs[groupIdx])
			}
		}
	}

	// Synthesis - navigable once created
	if session.SynthesisID != "" {
		instances = append(instances, session.SynthesisID)
	}

	// Revision - navigable once created
	if session.RevisionID != "" {
		instances = append(instances, session.RevisionID)
	}

	// Consolidation - navigable once created
	if session.ConsolidationID != "" {
		instances = append(instances, session.ConsolidationID)
	}

	return instances
}

// updateNavigableInstances updates the list of navigable instances
func (m *Model) updateNavigableInstances() {
	if m.ultraPlan == nil {
		return
	}
	m.ultraPlan.NavigableInstances = m.getNavigableInstances()
}

// navigateToNextInstance navigates to the next navigable instance
// direction: +1 for next, -1 for previous
func (m *Model) navigateToNextInstance(direction int) bool {
	if m.ultraPlan == nil {
		return false
	}

	// Update the navigable instances list
	m.updateNavigableInstances()
	instances := m.ultraPlan.NavigableInstances

	if len(instances) == 0 {
		return false
	}

	// Find current position in the list
	currentIdx := -1
	if m.activeTab >= 0 && m.activeTab < len(m.session.Instances) {
		currentInstID := m.session.Instances[m.activeTab].ID
		for i, instID := range instances {
			if instID == currentInstID {
				currentIdx = i
				break
			}
		}
	}

	// Calculate next index with wrapping
	var nextIdx int
	if currentIdx < 0 {
		// Not currently on a navigable instance, start from beginning or end
		if direction > 0 {
			nextIdx = 0
		} else {
			nextIdx = len(instances) - 1
		}
	} else {
		nextIdx = (currentIdx + direction + len(instances)) % len(instances)
	}

	// Find the instance in session.Instances and switch to it
	targetInstID := instances[nextIdx]
	for i, inst := range m.session.Instances {
		if inst.ID == targetInstID {
			m.activeTab = i
			m.ultraPlan.SelectedNavIdx = nextIdx
			m.ensureActiveVisible()
			return true
		}
	}

	return false
}

// renderUltraPlanContent renders the content area for ultra-plan mode
func (m Model) renderUltraPlanContent(width int) string {
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return m.renderContent(width)
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return m.renderContent(width)
	}

	// If plan editor is active, render the plan editor view
	if m.IsPlanEditorActive() && session.Plan != nil {
		return m.renderPlanEditorView(width)
	}

	// If showing plan view, render the plan
	if m.ultraPlan.ShowPlanView && session.Plan != nil {
		return m.renderPlanView(width)
	}

	// Otherwise, show normal instance output with ultra-plan context
	return m.renderContent(width)
}

// renderPlanView renders the detailed plan view
func (m Model) renderPlanView(width int) string {
	v := m.createUltraplanView()
	return v.RenderPlanView(width)
}

// renderUltraPlanHelp renders the help bar for ultra-plan mode
func (m Model) renderUltraPlanHelp() string {
	if m.ultraPlan == nil {
		return m.renderHelp()
	}

	v := m.createUltraplanView()
	return v.RenderHelp()
}

// handleUltraPlanKeypress handles ultra-plan specific key presses
// Returns (handled, model, cmd) where handled indicates if the key was processed
func (m Model) handleUltraPlanKeypress(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return false, m, nil
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return false, m, nil
	}

	// Group decision handling takes priority
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		switch msg.String() {
		case "c":
			// Continue with partial work (successful tasks only)
			if err := m.ultraPlan.Coordinator.ResumeWithPartialWork(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to continue: %v", err)
			} else {
				m.infoMessage = "Continuing with successful tasks..."
				// Log user decision
				if m.logger != nil {
					m.logger.Info("user decision",
						"decision_type", "group_partial_failure",
						"choice", "continue_partial")
				}
			}
			return true, m, nil

		case "r":
			// Retry failed tasks
			if err := m.ultraPlan.Coordinator.RetryFailedTasks(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to retry: %v", err)
			} else {
				m.infoMessage = "Retrying failed tasks..."
				// Log user decision
				if m.logger != nil {
					m.logger.Info("user decision",
						"decision_type", "group_partial_failure",
						"choice", "retry_failed")
				}
			}
			return true, m, nil

		case "q":
			// Cancel the ultraplan
			m.ultraPlan.Coordinator.Cancel()
			m.infoMessage = "Ultraplan cancelled"
			// Log user decision
			if m.logger != nil {
				m.logger.Info("user decision",
					"decision_type", "group_partial_failure",
					"choice", "cancel")
			}
			return true, m, nil
		}
	}

	switch msg.String() {
	case "v":
		// Toggle plan view (only when plan is available)
		if session.Plan != nil {
			m.ultraPlan.ShowPlanView = !m.ultraPlan.ShowPlanView
		}
		return true, m, nil

	case "p":
		// Parse plan from file or coordinator output (only during planning phase)
		if session.Phase == orchestrator.PhasePlanning {
			if session.CoordinatorID != "" {
				inst := m.orchestrator.GetInstance(session.CoordinatorID)
				if inst != nil {
					plan, err := m.tryParsePlan(inst, session)
					if err != nil {
						m.errorMessage = fmt.Sprintf("Failed to parse plan: %v", err)
					} else {
						if err := m.ultraPlan.Coordinator.SetPlan(plan); err != nil {
							m.errorMessage = fmt.Sprintf("Invalid plan: %v", err)
						} else {
							m.infoMessage = fmt.Sprintf("Plan parsed: %d tasks in %d groups", len(plan.Tasks), len(plan.ExecutionOrder))
						}
					}
				}
			}
		}
		return true, m, nil

	case "e":
		// Start execution (only during refresh phase when plan is ready)
		if session.Phase == orchestrator.PhaseRefresh && session.Plan != nil {
			// Validate plan before starting execution
			validation := orchestrator.ValidatePlanForEditor(session.Plan)
			if validation.HasErrors() {
				m.errorMessage = fmt.Sprintf("Cannot execute: plan has %d validation error(s). Press [E] to review.", validation.ErrorCount)
				return true, m, nil
			}
			if err := m.ultraPlan.Coordinator.StartExecution(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to start execution: %v", err)
			} else {
				m.infoMessage = "Execution started"
			}
		}
		return true, m, nil

	case "E":
		// Enter plan editor (only during refresh phase when plan is ready)
		if session.Phase == orchestrator.PhaseRefresh && session.Plan != nil {
			m.enterPlanEditor()
			m.infoMessage = "Plan editor opened. Use [enter] to confirm or [esc] to cancel."
		}
		return true, m, nil

	case "c":
		// Cancel execution (only during executing phase)
		if session.Phase == orchestrator.PhaseExecuting {
			m.ultraPlan.Coordinator.Cancel()
			m.infoMessage = "Execution cancelled"
		}
		return true, m, nil

	case "tab", "l":
		// Navigate to next navigable instance across all phases
		if m.navigateToNextInstance(1) {
			m.infoMessage = ""
		}
		return true, m, nil

	case "shift+tab", "h":
		// Navigate to previous navigable instance across all phases
		if m.navigateToNextInstance(-1) {
			m.infoMessage = ""
		}
		return true, m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Jump to task by number during execution (only if task has instance)
		if session.Phase == orchestrator.PhaseExecuting && session.Plan != nil {
			idx := int(msg.String()[0] - '1')
			if idx < len(session.Plan.Tasks) {
				task := &session.Plan.Tasks[idx]
				if _, hasInstance := session.TaskToInstance[task.ID]; hasInstance {
					m.ultraPlan.SelectedTaskIdx = idx
					m.selectTaskInstance(session)
				} else {
					m.infoMessage = fmt.Sprintf("Task %d not yet started (blocked)", idx+1)
				}
			}
		}
		return true, m, nil

	case "o":
		// Open first PR URL in browser (when PRs have been created)
		if len(session.PRUrls) > 0 {
			prURL := session.PRUrls[0]
			if err := view.OpenURL(prURL); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to open PR: %v", err)
			} else {
				m.infoMessage = "Opened PR in browser"
			}
		}
		return true, m, nil

	case "r":
		// Resume paused consolidation
		if session.Phase == orchestrator.PhaseConsolidating &&
			session.Consolidation != nil &&
			session.Consolidation.Phase == orchestrator.ConsolidationPaused {
			// TODO: Implement resume functionality when coordinator exposes it
			m.infoMessage = "Resuming consolidation..."
		}
		return true, m, nil

	case "s":
		// Signal synthesis is done, proceed to consolidation
		if session.Phase == orchestrator.PhaseSynthesis {
			if err := m.ultraPlan.Coordinator.TriggerConsolidation(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to proceed: %v", err)
			} else {
				m.infoMessage = "Proceeding to consolidation..."
				// Log synthesis approval
				if m.logger != nil {
					m.logger.Info("user approved synthesis")
				}
			}
		}
		return true, m, nil
	}

	return false, m, nil
}

// selectTaskInstance switches to the instance associated with the currently selected task
func (m *Model) selectTaskInstance(session *orchestrator.UltraPlanSession) {
	if session.Plan == nil || m.ultraPlan.SelectedTaskIdx >= len(session.Plan.Tasks) {
		return
	}

	task := &session.Plan.Tasks[m.ultraPlan.SelectedTaskIdx]
	instanceID, ok := session.TaskToInstance[task.ID]
	if !ok {
		m.infoMessage = fmt.Sprintf("Task %s not yet started", task.Title)
		return
	}

	// Find the instance index in session.Instances
	for i, inst := range m.session.Instances {
		if inst.ID == instanceID {
			m.activeTab = i
			m.ensureActiveVisible()
			m.infoMessage = fmt.Sprintf("Viewing: %s", task.Title)
			return
		}
	}

	m.infoMessage = fmt.Sprintf("Instance for task %s not found", task.Title)
}

// handlePlanManagerCompletion handles the plan manager instance completing in multi-pass mode.
// The plan manager evaluates multiple candidate plans and selects or merges the best one.
// Returns true if this was a plan manager completion that was handled.
func (m *Model) handlePlanManagerCompletion(inst *orchestrator.Instance) bool {
	// Not in ultra-plan mode
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return false
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return false
	}

	// Only handle during plan selection phase
	if session.Phase != orchestrator.PhasePlanSelection {
		return false
	}

	// Check if this is the plan manager instance
	if session.PlanManagerID != inst.ID {
		return false
	}

	// Parse the plan decision from the output
	output := m.outputs[inst.ID]
	decision, err := orchestrator.ParsePlanDecisionFromOutput(output)
	if err != nil {
		m.errorMessage = fmt.Sprintf("Plan selection completed but failed to parse decision: %v", err)
		m.ultraPlan.Coordinator.Manager().SetPhase(orchestrator.PhaseFailed)
		return true
	}

	// Store the decision info in the session
	session.SelectedPlanIndex = decision.SelectedIndex

	// Parse the final plan from the plan manager's worktree
	// The plan manager writes its final choice (selected or merged) to the plan file
	plan, err := m.tryParsePlan(inst, session)
	if err != nil {
		m.errorMessage = fmt.Sprintf("Plan selection completed but failed to parse final plan: %v", err)
		m.ultraPlan.Coordinator.Manager().SetPhase(orchestrator.PhaseFailed)
		return true
	}

	// Set the plan using the coordinator (validates and transitions to PhaseRefresh)
	if err := m.ultraPlan.Coordinator.SetPlan(plan); err != nil {
		m.errorMessage = fmt.Sprintf("Plan selection completed but plan is invalid: %v", err)
		m.ultraPlan.Coordinator.Manager().SetPhase(orchestrator.PhaseFailed)
		return true
	}

	// Build info message based on decision type
	var decisionDesc string
	if decision.Action == "select" {
		strategyNames := orchestrator.GetMultiPassStrategyNames()
		if decision.SelectedIndex >= 0 && decision.SelectedIndex < len(strategyNames) {
			decisionDesc = fmt.Sprintf("Selected '%s' plan", strategyNames[decision.SelectedIndex])
		} else {
			decisionDesc = fmt.Sprintf("Selected plan %d", decision.SelectedIndex)
		}
	} else {
		decisionDesc = "Merged best elements from multiple plans"
	}

	// Determine whether to open plan editor or auto-start execution
	// This follows the same logic as single-pass planning
	if session.Config.Review || !session.Config.AutoApprove {
		// Enter plan editor for interactive review
		m.enterPlanEditor()
		m.infoMessage = fmt.Sprintf("%s: %d tasks in %d groups. Review and press [enter] to execute, or [esc] to cancel.",
			decisionDesc, len(plan.Tasks), len(plan.ExecutionOrder))
		// Notify user that input is needed
		m.ultraPlan.NeedsNotification = true
		m.ultraPlan.LastNotifiedPhase = orchestrator.PhaseRefresh
	} else {
		// Auto-start execution (AutoApprove is true and Review is false)
		if err := m.ultraPlan.Coordinator.StartExecution(); err != nil {
			m.errorMessage = fmt.Sprintf("%s but failed to auto-start: %v", decisionDesc, err)
		} else {
			m.infoMessage = fmt.Sprintf("%s: %d tasks in %d groups. Auto-starting execution...",
				decisionDesc, len(plan.Tasks), len(plan.ExecutionOrder))
		}
	}

	return true
}

// handleUltraPlanCoordinatorCompletion handles auto-parsing when the planning coordinator completes
// Returns true if this was an ultra-plan coordinator completion that was handled
func (m *Model) handleUltraPlanCoordinatorCompletion(inst *orchestrator.Instance) bool {
	// Not in ultra-plan mode
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return false
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return false
	}

	// Multi-pass mode: dispatch to specialized handlers based on phase
	if session.Config.MultiPass {
		// During planning phase, check if this is one of the multi-pass coordinators
		if session.Phase == orchestrator.PhasePlanning {
			if m.handleMultiPassCoordinatorCompletion(inst) {
				return true
			}
		}
		// During plan selection phase, check if this is the plan manager
		if session.Phase == orchestrator.PhasePlanSelection {
			if m.handlePlanManagerCompletion(inst) {
				return true
			}
		}
		// Fall through to single-coordinator logic for other instances/phases
	}

	// Single-pass mode (or fall-through from multi-pass for non-matching instances):
	// Only auto-parse during planning phase
	if session.Phase != orchestrator.PhasePlanning {
		return false
	}

	// Check if this is the coordinator instance
	if session.CoordinatorID != inst.ID {
		return false
	}

	// Try to parse the plan (file-based first, then output fallback)
	plan, err := m.tryParsePlan(inst, session)
	if err != nil {
		m.errorMessage = fmt.Sprintf("Planning completed but failed to parse plan: %v", err)
		return true
	}

	if err := m.ultraPlan.Coordinator.SetPlan(plan); err != nil {
		m.errorMessage = fmt.Sprintf("Planning completed but plan is invalid: %v", err)
		return true
	}

	// Determine whether to open plan editor or auto-start execution
	// Review flag forces plan editor open, even if AutoApprove is also set
	if session.Config.Review || !session.Config.AutoApprove {
		// Enter plan editor for interactive review
		m.enterPlanEditor()
		m.infoMessage = fmt.Sprintf("Plan ready: %d tasks in %d groups. Review and press [enter] to execute, or [esc] to cancel.",
			len(plan.Tasks), len(plan.ExecutionOrder))
		// Notify user that input is needed
		m.ultraPlan.NeedsNotification = true
		m.ultraPlan.LastNotifiedPhase = orchestrator.PhaseRefresh
	} else {
		// Auto-start execution (AutoApprove is true and Review is false)
		if err := m.ultraPlan.Coordinator.StartExecution(); err != nil {
			m.errorMessage = fmt.Sprintf("Plan ready but failed to auto-start: %v", err)
		} else {
			m.infoMessage = fmt.Sprintf("Plan ready: %d tasks in %d groups. Auto-starting execution...",
				len(plan.Tasks), len(plan.ExecutionOrder))
		}
	}

	return true
}

// handleMultiPassCoordinatorCompletion handles completion of one of the multi-pass planning coordinators.
// In multi-pass mode, 3 parallel coordinators generate plans with different strategies.
// This method collects each plan and triggers the plan manager when all 3 are ready.
// Returns true if this was a multi-pass coordinator completion that was handled.
func (m *Model) handleMultiPassCoordinatorCompletion(inst *orchestrator.Instance) bool {
	// Not in ultra-plan mode
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return false
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return false
	}

	// Check if multi-pass mode is enabled
	if !session.Config.MultiPass {
		return false
	}

	// Only process during planning phase
	if session.Phase != orchestrator.PhasePlanning {
		return false
	}

	// Check if this instance is one of the plan coordinators
	planIndex := -1
	for i, coordID := range session.PlanCoordinatorIDs {
		if coordID == inst.ID {
			planIndex = i
			break
		}
	}

	if planIndex == -1 {
		// Not a multi-pass planning coordinator
		return false
	}

	// Parse the plan from the completed instance's worktree
	plan, parseErr := m.tryParsePlan(inst, session)

	// Determine the strategy name for messages
	strategyNames := orchestrator.GetMultiPassStrategyNames()
	strategyName := "unknown"
	if planIndex < len(strategyNames) {
		strategyName = strategyNames[planIndex]
	}

	numCoordinators := len(session.PlanCoordinatorIDs)

	if parseErr != nil {
		// Store nil for failed coordinator to track completion
		// This prevents the system from hanging forever waiting for a plan that will never arrive
		m.ultraPlan.Coordinator.StoreCandidatePlan(planIndex, nil)
		m.errorMessage = fmt.Sprintf("Multi-pass coordinator %d (%s) failed to parse plan: %v", planIndex+1, strategyName, parseErr)

		// Check if all coordinators have completed (even if some failed)
		completedCount := m.ultraPlan.Coordinator.CountCoordinatorsCompleted()
		if completedCount >= numCoordinators {
			// All coordinators completed - proceed with valid plans if we have any
			validCount := m.ultraPlan.Coordinator.CountCandidatePlans()
			if validCount > 0 {
				m.infoMessage = fmt.Sprintf("%d/%d plans collected. Starting plan evaluation with available plans...",
					validCount, numCoordinators)
				if err := m.ultraPlan.Coordinator.RunPlanManager(); err != nil {
					m.errorMessage = fmt.Sprintf("Failed to start plan manager: %v", err)
				} else {
					m.infoMessage = fmt.Sprintf("Plan manager started with %d valid plans...", validCount)
				}
			} else {
				// All coordinators failed - transition to failed state
				m.errorMessage = "All multi-pass coordinators failed to produce valid plans"
				m.ultraPlan.Coordinator.Manager().SetPhase(orchestrator.PhaseFailed)
			}
		}
		return true
	}

	// Store the plan using mutex-protected method to avoid race conditions
	collectedCount := m.ultraPlan.Coordinator.StoreCandidatePlan(planIndex, plan)

	m.infoMessage = fmt.Sprintf("Multi-pass plan %d/%d collected (%s): %d tasks",
		collectedCount, numCoordinators, strategyName, len(plan.Tasks))

	// Check if all coordinators have completed
	completedCount := m.ultraPlan.Coordinator.CountCoordinatorsCompleted()
	if completedCount >= numCoordinators {
		validCount := m.ultraPlan.Coordinator.CountCandidatePlans()
		if validCount > 0 {
			// Start plan evaluation with whatever valid plans we have
			if validCount < numCoordinators {
				m.infoMessage = fmt.Sprintf("%d/%d plans collected. Starting plan evaluation with available plans...",
					validCount, numCoordinators)
			} else {
				m.infoMessage = "All candidate plans collected. Starting plan evaluation..."
			}
			if err := m.ultraPlan.Coordinator.RunPlanManager(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to start plan manager: %v", err)
			} else {
				m.infoMessage = "Plan manager started - comparing strategies to select the best plan..."
			}
		} else {
			// All coordinators completed but no valid plans - this shouldn't happen in the success path
			// but handle defensively to avoid getting stuck
			m.errorMessage = "All coordinators completed but no valid plans were collected"
			m.ultraPlan.Coordinator.Manager().SetPhase(orchestrator.PhaseFailed)
		}
	}

	return true
}

// tryParsePlan attempts to parse a plan, trying file-based parsing first, then output parsing
func (m *Model) tryParsePlan(inst *orchestrator.Instance, session *orchestrator.UltraPlanSession) (*orchestrator.PlanSpec, error) {
	// Try file-based parsing first (preferred)
	planPath := orchestrator.PlanFilePath(inst.WorktreePath)
	if _, err := os.Stat(planPath); err == nil {
		plan, err := orchestrator.ParsePlanFromFile(planPath, session.Objective)
		if err == nil {
			return plan, nil
		}
		// File exists but parsing failed - report this specific error
		return nil, fmt.Errorf("plan file exists but is invalid: %w", err)
	}

	// Fall back to output parsing (for backwards compatibility)
	output := m.outputs[inst.ID]
	if output == "" {
		return nil, fmt.Errorf("no plan file found and no output available")
	}

	return orchestrator.ParsePlanFromOutput(output, session.Objective)
}

// checkForPlanFile checks if the plan file exists and parses it (called during tick updates)
// Returns true if a plan was found and successfully parsed
func (m *Model) checkForPlanFile() bool {
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return false
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil || session.Phase != orchestrator.PhasePlanning || session.Plan != nil {
		return false
	}

	// Get the coordinator instance
	inst := m.orchestrator.GetInstance(session.CoordinatorID)
	if inst == nil {
		return false
	}

	// Check if plan file exists
	planPath := orchestrator.PlanFilePath(inst.WorktreePath)
	if _, err := os.Stat(planPath); err != nil {
		return false
	}

	// Parse the plan
	plan, err := orchestrator.ParsePlanFromFile(planPath, session.Objective)
	if err != nil {
		// Don't show error yet - file might be partially written
		return false
	}

	if err := m.ultraPlan.Coordinator.SetPlan(plan); err != nil {
		return false
	}

	// Plan detected - stop the coordinator instance (it's done its job)
	_ = m.orchestrator.StopInstance(inst)

	// Determine whether to open plan editor or auto-start execution
	// Review flag forces plan editor open, even if AutoApprove is also set
	if session.Config.Review || !session.Config.AutoApprove {
		// Enter plan editor for interactive review
		m.enterPlanEditor()
		m.infoMessage = fmt.Sprintf("Plan detected: %d tasks in %d groups. Review and press [enter] to execute, or [esc] to cancel.",
			len(plan.Tasks), len(plan.ExecutionOrder))
		// Notify user that input is needed
		m.ultraPlan.NeedsNotification = true
		m.ultraPlan.LastNotifiedPhase = orchestrator.PhaseRefresh
	} else {
		// Auto-start execution (AutoApprove is true and Review is false)
		if err := m.ultraPlan.Coordinator.StartExecution(); err != nil {
			m.errorMessage = fmt.Sprintf("Plan detected but failed to auto-start: %v", err)
		} else {
			m.infoMessage = fmt.Sprintf("Plan detected: %d tasks in %d groups. Auto-starting execution...",
				len(plan.Tasks), len(plan.ExecutionOrder))
		}
	}

	return true
}

// checkForMultiPassPlanFiles checks for plan files from all multi-pass coordinators (called during tick updates).
// This is the most reliable method for detecting plan completion in multi-pass mode,
// as it polls for the actual plan files rather than relying on instance state transitions.
// Returns true if all plans were found and the plan manager was triggered.
func (m *Model) checkForMultiPassPlanFiles() bool {
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return false
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return false
	}

	// Only check during planning phase in multi-pass mode
	if session.Phase != orchestrator.PhasePlanning || !session.Config.MultiPass {
		return false
	}

	// Skip if we don't have coordinator IDs yet
	numCoordinators := len(session.PlanCoordinatorIDs)
	if numCoordinators == 0 {
		return false
	}

	// Skip if plan manager is already running
	if session.PlanManagerID != "" {
		return false
	}

	strategyNames := orchestrator.GetMultiPassStrategyNames()
	newPlansFound := false

	// Check each coordinator for a plan file
	for i, coordID := range session.PlanCoordinatorIDs {
		// Skip if we already have a plan for this coordinator
		if i < len(session.CandidatePlans) && session.CandidatePlans[i] != nil {
			continue
		}

		// Get the coordinator instance
		inst := m.orchestrator.GetInstance(coordID)
		if inst == nil {
			continue
		}

		// Check if plan file exists
		planPath := orchestrator.PlanFilePath(inst.WorktreePath)
		if _, err := os.Stat(planPath); err != nil {
			continue
		}

		// Parse the plan
		plan, err := orchestrator.ParsePlanFromFile(planPath, session.Objective)
		if err != nil {
			// File might be partially written, skip for now
			continue
		}

		// Store the plan
		collectedCount := m.ultraPlan.Coordinator.StoreCandidatePlan(i, plan)
		newPlansFound = true

		strategyName := "unknown"
		if i < len(strategyNames) {
			strategyName = strategyNames[i]
		}
		m.infoMessage = fmt.Sprintf("Plan file detected %d/%d (%s): %d tasks",
			collectedCount, numCoordinators, strategyName, len(plan.Tasks))
	}

	// Check if all plans are now collected
	validCount := m.ultraPlan.Coordinator.CountCandidatePlans()
	if validCount >= numCoordinators {
		// All plans collected - start plan manager
		m.infoMessage = "All candidate plans collected via file detection. Starting plan evaluation..."
		if err := m.ultraPlan.Coordinator.RunPlanManager(); err != nil {
			m.errorMessage = fmt.Sprintf("Failed to start plan manager: %v", err)
			return false
		}
		m.infoMessage = "Plan manager started - comparing strategies to select the best plan..."
		return true
	}

	return newPlansFound
}

// checkForPlanManagerPlanFile checks for the plan manager's output file during plan selection phase.
// This is the most reliable method for detecting plan manager completion, as it polls for the actual
// plan file rather than relying on instance state transitions (which can fail when the instance
// is in StatusWaitingInput state).
// Returns true if the plan was found and successfully processed.
func (m *Model) checkForPlanManagerPlanFile() bool {
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return false
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return false
	}

	// Only check during plan selection phase in multi-pass mode
	if session.Phase != orchestrator.PhasePlanSelection || !session.Config.MultiPass {
		return false
	}

	// Need a plan manager ID to check
	if session.PlanManagerID == "" {
		return false
	}

	// Skip if we already have a plan set
	if session.Plan != nil {
		return false
	}

	// Get the plan manager instance
	inst := m.orchestrator.GetInstance(session.PlanManagerID)
	if inst == nil {
		return false
	}

	// Check if plan file exists
	planPath := orchestrator.PlanFilePath(inst.WorktreePath)
	if _, err := os.Stat(planPath); err != nil {
		return false
	}

	// Parse the plan from the file
	plan, err := orchestrator.ParsePlanFromFile(planPath, session.Objective)
	if err != nil {
		// Show error if file exists but can't be parsed (helps debug)
		m.errorMessage = fmt.Sprintf("Plan file found but parse error: %v", err)
		return false
	}

	// Try to parse the plan decision from the output (for display purposes)
	// This is optional - the plan file is the ground truth
	output := m.outputs[inst.ID]
	decision, _ := orchestrator.ParsePlanDecisionFromOutput(output)
	if decision != nil {
		session.SelectedPlanIndex = decision.SelectedIndex
	}

	// Set the plan using the coordinator (validates and transitions to PhaseRefresh)
	if err := m.ultraPlan.Coordinator.SetPlan(plan); err != nil {
		m.errorMessage = fmt.Sprintf("Plan manager plan file found but invalid: %v", err)
		return false
	}

	// Build info message based on decision type
	var decisionDesc string
	if decision != nil {
		if decision.Action == "select" {
			strategyNames := orchestrator.GetMultiPassStrategyNames()
			if decision.SelectedIndex >= 0 && decision.SelectedIndex < len(strategyNames) {
				decisionDesc = fmt.Sprintf("Selected '%s' plan", strategyNames[decision.SelectedIndex])
			} else {
				decisionDesc = fmt.Sprintf("Selected plan %d", decision.SelectedIndex)
			}
		} else {
			decisionDesc = "Merged best elements from multiple plans"
		}
	} else {
		decisionDesc = "Plan manager completed"
	}

	// Determine whether to open plan editor or auto-start execution
	if session.Config.Review || !session.Config.AutoApprove {
		// Enter plan editor for interactive review
		m.enterPlanEditor()
		m.infoMessage = fmt.Sprintf("%s: %d tasks in %d groups. Review and press [enter] to execute, or [esc] to cancel.",
			decisionDesc, len(plan.Tasks), len(plan.ExecutionOrder))
		// Notify user that input is needed
		m.ultraPlan.NeedsNotification = true
		m.ultraPlan.LastNotifiedPhase = orchestrator.PhaseRefresh
	} else {
		// Auto-start execution (AutoApprove is true and Review is false)
		if err := m.ultraPlan.Coordinator.StartExecution(); err != nil {
			m.errorMessage = fmt.Sprintf("%s but failed to auto-start: %v", decisionDesc, err)
		} else {
			m.infoMessage = fmt.Sprintf("%s: %d tasks in %d groups. Auto-starting execution...",
				decisionDesc, len(plan.Tasks), len(plan.ExecutionOrder))
		}
	}

	return true
}
