package tui

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// handlePlanManagerCompletion handles the plan manager instance completing in multi-pass mode.
// The plan manager evaluates multiple candidate plans and selects or merges the best one.
// Returns true if this was a plan manager completion that was handled.
func (m *Model) handlePlanManagerCompletion(inst *orchestrator.Instance) bool {
	// Not in ultra-plan mode
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return false
	}

	session := m.ultraPlan.coordinator.Session()
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
		m.ultraPlan.coordinator.Manager().SetPhase(orchestrator.PhaseFailed)
		return true
	}

	// Store the decision info in the session
	session.SelectedPlanIndex = decision.SelectedIndex

	// Parse the final plan from the plan manager's worktree
	// The plan manager writes its final choice (selected or merged) to the plan file
	plan, err := m.tryParsePlan(inst, session)
	if err != nil {
		m.errorMessage = fmt.Sprintf("Plan selection completed but failed to parse final plan: %v", err)
		m.ultraPlan.coordinator.Manager().SetPhase(orchestrator.PhaseFailed)
		return true
	}

	// Set the plan using the coordinator (validates and transitions to PhaseRefresh)
	if err := m.ultraPlan.coordinator.SetPlan(plan); err != nil {
		m.errorMessage = fmt.Sprintf("Plan selection completed but plan is invalid: %v", err)
		m.ultraPlan.coordinator.Manager().SetPhase(orchestrator.PhaseFailed)
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
		m.ultraPlan.needsNotification = true
		m.ultraPlan.lastNotifiedPhase = orchestrator.PhaseRefresh
	} else {
		// Auto-start execution (AutoApprove is true and Review is false)
		if err := m.ultraPlan.coordinator.StartExecution(); err != nil {
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
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return false
	}

	session := m.ultraPlan.coordinator.Session()
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

	if err := m.ultraPlan.coordinator.SetPlan(plan); err != nil {
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
		m.ultraPlan.needsNotification = true
		m.ultraPlan.lastNotifiedPhase = orchestrator.PhaseRefresh
	} else {
		// Auto-start execution (AutoApprove is true and Review is false)
		if err := m.ultraPlan.coordinator.StartExecution(); err != nil {
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
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return false
	}

	session := m.ultraPlan.coordinator.Session()
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
		m.ultraPlan.coordinator.StoreCandidatePlan(planIndex, nil)
		m.errorMessage = fmt.Sprintf("Multi-pass coordinator %d (%s) failed to parse plan: %v", planIndex+1, strategyName, parseErr)

		// Check if all coordinators have completed (even if some failed)
		completedCount := m.ultraPlan.coordinator.CountCoordinatorsCompleted()
		if completedCount >= numCoordinators {
			// All coordinators completed - proceed with valid plans if we have any
			validCount := m.ultraPlan.coordinator.CountCandidatePlans()
			if validCount > 0 {
				m.infoMessage = fmt.Sprintf("%d/%d plans collected. Starting plan evaluation with available plans...",
					validCount, numCoordinators)
				if err := m.ultraPlan.coordinator.RunPlanManager(); err != nil {
					m.errorMessage = fmt.Sprintf("Failed to start plan manager: %v", err)
				} else {
					m.infoMessage = fmt.Sprintf("Plan manager started with %d valid plans...", validCount)
				}
			} else {
				// All coordinators failed - transition to failed state
				m.errorMessage = "All multi-pass coordinators failed to produce valid plans"
				m.ultraPlan.coordinator.Manager().SetPhase(orchestrator.PhaseFailed)
			}
		}
		return true
	}

	// Store the plan using mutex-protected method to avoid race conditions
	collectedCount := m.ultraPlan.coordinator.StoreCandidatePlan(planIndex, plan)

	m.infoMessage = fmt.Sprintf("Multi-pass plan %d/%d collected (%s): %d tasks",
		collectedCount, numCoordinators, strategyName, len(plan.Tasks))

	// Check if all coordinators have completed
	completedCount := m.ultraPlan.coordinator.CountCoordinatorsCompleted()
	if completedCount >= numCoordinators {
		validCount := m.ultraPlan.coordinator.CountCandidatePlans()
		if validCount > 0 {
			// Start plan evaluation with whatever valid plans we have
			if validCount < numCoordinators {
				m.infoMessage = fmt.Sprintf("%d/%d plans collected. Starting plan evaluation with available plans...",
					validCount, numCoordinators)
			} else {
				m.infoMessage = "All candidate plans collected. Starting plan evaluation..."
			}
			if err := m.ultraPlan.coordinator.RunPlanManager(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to start plan manager: %v", err)
			} else {
				m.infoMessage = "Plan manager started - comparing strategies to select the best plan..."
			}
		} else {
			// All coordinators completed but no valid plans - this shouldn't happen in the success path
			// but handle defensively to avoid getting stuck
			m.errorMessage = "All coordinators completed but no valid plans were collected"
			m.ultraPlan.coordinator.Manager().SetPhase(orchestrator.PhaseFailed)
		}
	}

	return true
}
