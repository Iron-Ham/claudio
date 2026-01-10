package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// UltraPlanState holds ultra-plan specific UI state
type UltraPlanState struct {
	coordinator            *orchestrator.Coordinator
	showPlanView           bool                            // Toggle between plan view and normal output view
	selectedTaskIdx        int                             // Currently selected task index for navigation
	needsNotification      bool                            // Set when user input is needed (checked on tick)
	lastNotifiedPhase      orchestrator.UltraPlanPhase     // Prevent duplicate notifications for same phase
	lastConsolidationPhase orchestrator.ConsolidationPhase // Track consolidation phase for pause detection
	notifiedGroupDecision  bool                            // Prevent repeated notifications while awaiting group decision

	// Phase-aware navigation state
	navigableInstances []string // Ordered list of navigable instance IDs
	selectedNavIdx     int      // Index into navigableInstances
}

// checkForPhaseNotification checks if the ultraplan phase changed to one that needs user attention
// and sets the notification flag if needed. Call this from the tick handler.
func (m *Model) checkForPhaseNotification() {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return
	}

	// Check for synthesis phase (user may want to review)
	if session.Phase == orchestrator.PhaseSynthesis && m.ultraPlan.lastNotifiedPhase != orchestrator.PhaseSynthesis {
		m.ultraPlan.needsNotification = true
		m.ultraPlan.lastNotifiedPhase = orchestrator.PhaseSynthesis
		return
	}

	// Check for revision phase (issues were found, user may want to know)
	if session.Phase == orchestrator.PhaseRevision && m.ultraPlan.lastNotifiedPhase != orchestrator.PhaseRevision {
		m.ultraPlan.needsNotification = true
		m.ultraPlan.lastNotifiedPhase = orchestrator.PhaseRevision
		return
	}

	// Check for consolidation pause (conflict detected, needs user attention)
	if session.Phase == orchestrator.PhaseConsolidating && session.Consolidation != nil {
		if session.Consolidation.Phase == orchestrator.ConsolidationPaused &&
			m.ultraPlan.lastConsolidationPhase != orchestrator.ConsolidationPaused {
			m.ultraPlan.needsNotification = true
			m.ultraPlan.lastConsolidationPhase = orchestrator.ConsolidationPaused
			return
		}
		// Track consolidation phase changes
		m.ultraPlan.lastConsolidationPhase = session.Consolidation.Phase
	}

	// Check for group decision needed (partial success/failure)
	// Only notify once when we enter the awaiting decision state
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		if !m.ultraPlan.notifiedGroupDecision {
			m.ultraPlan.needsNotification = true
			m.ultraPlan.notifiedGroupDecision = true
		}
		return
	}

	// Reset group decision notification flag when no longer awaiting
	// (so we can notify again if another group decision occurs later)
	if m.ultraPlan.notifiedGroupDecision {
		m.ultraPlan.notifiedGroupDecision = false
	}
}

// renderUltraPlanHelp renders the help bar for ultra-plan mode
func (m Model) renderUltraPlanHelp() string {
	if m.ultraPlan == nil {
		return m.renderHelp()
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return m.renderHelp()
	}

	var keys []string

	// Group decision mode takes priority
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		keys = append(keys, "[c] continue partial")
		keys = append(keys, "[r] retry failed")
		keys = append(keys, "[q] cancel")
		keys = append(keys, "[↑↓] nav")
		return styles.HelpBar.Width(m.width).Render(strings.Join(keys, "  "))
	}

	// Common keys
	keys = append(keys, "[q] quit")
	keys = append(keys, "[↑↓] nav")

	// Phase-specific keys
	switch session.Phase {
	case orchestrator.PhasePlanning:
		keys = append(keys, "[p] parse plan")
		keys = append(keys, "[i] input mode")

	case orchestrator.PhasePlanSelection:
		// During plan selection/comparison, user may want to view progress
		keys = append(keys, "[v] toggle plan view")

	case orchestrator.PhaseRefresh:
		keys = append(keys, "[e] start execution")
		keys = append(keys, "[E] edit plan")

	case orchestrator.PhaseExecuting:
		keys = append(keys, "[tab] next task")
		keys = append(keys, "[1-9] select task")
		keys = append(keys, "[i] input mode")
		keys = append(keys, "[v] toggle plan view")
		keys = append(keys, "[c] cancel")

	case orchestrator.PhaseSynthesis:
		keys = append(keys, "[i] input mode")
		keys = append(keys, "[v] toggle plan view")
		if session.SynthesisAwaitingApproval {
			keys = append(keys, "[s] approve → proceed")
		} else {
			keys = append(keys, "[s] skip → consolidate")
		}

	case orchestrator.PhaseRevision:
		keys = append(keys, "[tab] next instance")
		keys = append(keys, "[v] toggle plan view")
		if session.Revision != nil {
			keys = append(keys, fmt.Sprintf("round %d/%d", session.Revision.RevisionRound, session.Revision.MaxRevisions))
		}

	case orchestrator.PhaseConsolidating:
		keys = append(keys, "[v] toggle plan view")
		if session.Consolidation != nil && session.Consolidation.Phase == orchestrator.ConsolidationPaused {
			keys = append(keys, "[r] resume")
		}

	case orchestrator.PhaseComplete, orchestrator.PhaseFailed:
		keys = append(keys, "[v] view plan")
		if len(session.PRUrls) > 0 {
			keys = append(keys, "[o] open PR")
		}
	}

	return styles.HelpBar.Width(m.width).Render(strings.Join(keys, "  "))
}

// complexityIndicator returns a visual indicator for task complexity
func complexityIndicator(complexity orchestrator.TaskComplexity) string {
	switch complexity {
	case orchestrator.ComplexityLow:
		return "◦"
	case orchestrator.ComplexityMedium:
		return "◎"
	case orchestrator.ComplexityHigh:
		return "●"
	default:
		return "○"
	}
}

// handleUltraPlanKeypress handles ultra-plan specific key presses
// Returns (handled, model, cmd) where handled indicates if the key was processed
func (m Model) handleUltraPlanKeypress(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return false, m, nil
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return false, m, nil
	}

	// Group decision handling takes priority
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		switch msg.String() {
		case "c":
			// Continue with partial work (successful tasks only)
			if err := m.ultraPlan.coordinator.ResumeWithPartialWork(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to continue: %v", err)
			} else {
				m.infoMessage = "Continuing with successful tasks..."
			}
			return true, m, nil

		case "r":
			// Retry failed tasks
			if err := m.ultraPlan.coordinator.RetryFailedTasks(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to retry: %v", err)
			} else {
				m.infoMessage = "Retrying failed tasks..."
			}
			return true, m, nil

		case "q":
			// Cancel the ultraplan
			m.ultraPlan.coordinator.Cancel()
			m.infoMessage = "Ultraplan cancelled"
			return true, m, nil
		}
	}

	switch msg.String() {
	case "v":
		// Toggle plan view (only when plan is available)
		if session.Plan != nil {
			m.ultraPlan.showPlanView = !m.ultraPlan.showPlanView
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
						if err := m.ultraPlan.coordinator.SetPlan(plan); err != nil {
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
			if err := m.ultraPlan.coordinator.StartExecution(); err != nil {
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
			m.ultraPlan.coordinator.Cancel()
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
					m.ultraPlan.selectedTaskIdx = idx
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
			if err := openURL(prURL); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to open PR: %v", err)
			} else {
				m.infoMessage = fmt.Sprintf("Opened PR in browser")
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
			if err := m.ultraPlan.coordinator.TriggerConsolidation(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to proceed: %v", err)
			} else {
				m.infoMessage = "Proceeding to consolidation..."
			}
		}
		return true, m, nil
	}

	return false, m, nil
}

// tryParsePlan attempts to parse a plan from file or output
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
	output := m.outputManager.GetOutput(inst.ID)
	if output == "" {
		return nil, fmt.Errorf("no plan file found and no output available")
	}

	return orchestrator.ParsePlanFromOutput(output, session.Objective)
}

// checkForPlanFile checks if the plan file exists and parses it (called during tick updates)
// Returns true if a plan was found and successfully parsed
func (m *Model) checkForPlanFile() bool {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return false
	}

	session := m.ultraPlan.coordinator.Session()
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

	if err := m.ultraPlan.coordinator.SetPlan(plan); err != nil {
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
		m.ultraPlan.needsNotification = true
		m.ultraPlan.lastNotifiedPhase = orchestrator.PhaseRefresh
	} else {
		// Auto-start execution (AutoApprove is true and Review is false)
		if err := m.ultraPlan.coordinator.StartExecution(); err != nil {
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
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return false
	}

	session := m.ultraPlan.coordinator.Session()
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
		collectedCount := m.ultraPlan.coordinator.StoreCandidatePlan(i, plan)
		newPlansFound = true

		strategyName := "unknown"
		if i < len(strategyNames) {
			strategyName = strategyNames[i]
		}
		m.infoMessage = fmt.Sprintf("Plan file detected %d/%d (%s): %d tasks",
			collectedCount, numCoordinators, strategyName, len(plan.Tasks))
	}

	// Check if all plans are now collected
	validCount := m.ultraPlan.coordinator.CountCandidatePlans()
	if validCount >= numCoordinators {
		// All plans collected - start plan manager
		m.infoMessage = "All candidate plans collected via file detection. Starting plan evaluation..."
		if err := m.ultraPlan.coordinator.RunPlanManager(); err != nil {
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
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return false
	}

	session := m.ultraPlan.coordinator.Session()
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
	output := m.outputManager.GetOutput(inst.ID)
	decision, _ := orchestrator.ParsePlanDecisionFromOutput(output)
	if decision != nil {
		session.SelectedPlanIndex = decision.SelectedIndex
	}

	// Set the plan using the coordinator (validates and transitions to PhaseRefresh)
	if err := m.ultraPlan.coordinator.SetPlan(plan); err != nil {
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

// openURL opens the given URL in the default browser
func openURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
