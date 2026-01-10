package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
		return m.renderSidebar(width, height)
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return m.renderSidebar(width, height)
	}

	v := m.createUltraplanView()
	return v.RenderSidebar(width, height)
}

// getGroupStatus returns a status indicator for a task group
func (m Model) getGroupStatus(session *orchestrator.UltraPlanSession, group []string) string {
	allComplete := true
	anyRunning := false
	anyFailed := false

	for _, taskID := range group {
		completed := false
		for _, ct := range session.CompletedTasks {
			if ct == taskID {
				completed = true
				break
			}
		}

		failed := false
		for _, ft := range session.FailedTasks {
			if ft == taskID {
				failed = true
				break
			}
		}

		if failed {
			anyFailed = true
		}

		if !completed && !failed {
			allComplete = false
			if _, running := session.TaskToInstance[taskID]; running {
				anyRunning = true
			}
		}
	}

	if allComplete && !anyFailed {
		return "✓"
	}
	if anyFailed {
		return "✗"
	}
	if anyRunning {
		return "⟳"
	}
	return "○"
}

// getPhaseSectionStatus returns a status indicator for a phase section header
func (m Model) getPhaseSectionStatus(phase orchestrator.UltraPlanPhase, session *orchestrator.UltraPlanSession) string {
	switch phase {
	case orchestrator.PhasePlanning:
		if session.Phase == orchestrator.PhasePlanning {
			return "[⟳]"
		}
		// Plan selection comes after planning but before refresh
		if session.Phase == orchestrator.PhasePlanSelection {
			return "[✓]"
		}
		if session.Plan != nil {
			return "[✓]"
		}
		return "[○]"

	case orchestrator.PhasePlanSelection:
		if session.Phase == orchestrator.PhasePlanSelection {
			return "[⟳]"
		}
		// Plan selection is complete once we move to refresh or later phases
		if session.Phase == orchestrator.PhaseRefresh ||
			session.Phase == orchestrator.PhaseExecuting ||
			session.Phase == orchestrator.PhaseSynthesis ||
			session.Phase == orchestrator.PhaseRevision ||
			session.Phase == orchestrator.PhaseConsolidating ||
			session.Phase == orchestrator.PhaseComplete {
			return "[✓]"
		}
		return "[○]"

	case orchestrator.PhaseExecuting:
		if session.Plan == nil {
			return "[○]"
		}
		completed := len(session.CompletedTasks)
		total := len(session.Plan.Tasks)
		if session.Phase == orchestrator.PhaseExecuting {
			return fmt.Sprintf("[%d/%d]", completed, total)
		}
		if completed == total && total > 0 {
			return "[✓]"
		}
		if completed > 0 {
			return fmt.Sprintf("[%d/%d]", completed, total)
		}
		return "[○]"

	case orchestrator.PhaseSynthesis:
		if session.Phase == orchestrator.PhaseSynthesis {
			return "[⟳]"
		}
		if session.Phase == orchestrator.PhaseRevision || session.Phase == orchestrator.PhaseConsolidating || session.Phase == orchestrator.PhaseComplete {
			return "[✓]"
		}
		return "[○]"

	case orchestrator.PhaseRevision:
		if session.Phase == orchestrator.PhaseRevision {
			if session.Revision != nil {
				revised := len(session.Revision.RevisedTasks)
				total := len(session.Revision.TasksToRevise)
				return fmt.Sprintf("[%d/%d]", revised, total)
			}
			return "[⟳]"
		}
		if session.Phase == orchestrator.PhaseConsolidating || session.Phase == orchestrator.PhaseComplete {
			if session.Revision != nil && len(session.Revision.Issues) > 0 {
				return "[✓]"
			}
			return "[○]" // No revision was needed
		}
		return "[○]"

	case orchestrator.PhaseConsolidating:
		if session.Phase == orchestrator.PhaseConsolidating {
			return "[⟳]"
		}
		if session.Phase == orchestrator.PhaseComplete && len(session.PRUrls) > 0 {
			return "[✓]"
		}
		if session.Phase == orchestrator.PhaseComplete {
			return "[✓]"
		}
		return "[○]"

	default:
		return ""
	}
}

// findInstanceIDForTask finds the instance ID associated with a task.
// It first checks the active TaskToInstance mapping, then searches through
// all instances for completed/failed tasks that are no longer in the mapping.
func (m Model) findInstanceIDForTask(session *orchestrator.UltraPlanSession, taskID string) string {
	// First check the active TaskToInstance mapping
	if instID, ok := session.TaskToInstance[taskID]; ok && instID != "" {
		return instID
	}

	// For completed/failed tasks, search through all instances
	for _, inst := range m.session.Instances {
		if strings.Contains(inst.Task, taskID) {
			return inst.ID
		}
	}

	return ""
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

// renderPhaseInstanceLine renders a line for a phase instance (coordinator, synthesis, consolidation)
func (m Model) renderPhaseInstanceLine(inst *orchestrator.Instance, name string, selected, navigable bool, _ int) string {
	var statusIcon string
	var statusStyle lipgloss.Style

	if inst == nil {
		statusIcon = "○"
		statusStyle = styles.Muted
	} else {
		switch inst.Status {
		case orchestrator.StatusWorking:
			statusIcon = "⟳"
			statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
		case orchestrator.StatusCompleted, orchestrator.StatusWaitingInput:
			statusIcon = "✓"
			statusStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
		case orchestrator.StatusError, orchestrator.StatusStuck, orchestrator.StatusTimeout:
			statusIcon = "✗"
			statusStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
		case orchestrator.StatusPending:
			statusIcon = "○"
			statusStyle = styles.Muted
		default:
			statusIcon = "◌"
			statusStyle = styles.Muted
		}
	}

	line := fmt.Sprintf("  %s %s", statusStyle.Render(statusIcon), name)

	// Apply styling based on navigability and selection
	if selected {
		line = lipgloss.NewStyle().
			Background(styles.PrimaryColor).
			Foreground(styles.TextColor).
			Render(line)
	} else if !navigable {
		line = styles.Muted.Render(line)
	}

	return line
}

// renderExecutionTaskLine renders a task line in the execution section
func (m Model) renderExecutionTaskLine(session *orchestrator.UltraPlanSession, task *orchestrator.PlannedTask, instanceID string, selected, navigable bool, maxWidth int) string {
	// Determine task status
	var statusIcon string
	var statusStyle lipgloss.Style

	// Check if completed
	for _, ct := range session.CompletedTasks {
		if ct == task.ID {
			statusIcon = "✓"
			statusStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
			break
		}
	}

	// Check if failed
	if statusIcon == "" {
		for _, ft := range session.FailedTasks {
			if ft == task.ID {
				statusIcon = "✗"
				statusStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
				break
			}
		}
	}

	// Check if running
	if statusIcon == "" && instanceID != "" {
		statusIcon = "⟳"
		statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
	}

	// Default: pending
	if statusIcon == "" {
		statusIcon = "○"
		statusStyle = styles.Muted
	}

	// Build line with truncated title
	titleLen := maxWidth - 6 // status + spaces
	title := truncate(task.Title, titleLen)
	line := fmt.Sprintf("    %s %s", statusStyle.Render(statusIcon), title)

	// Apply styling
	if selected {
		line = lipgloss.NewStyle().
			Background(styles.PrimaryColor).
			Foreground(styles.TextColor).
			Render(line)
	} else if !navigable {
		line = styles.Muted.Render(line)
	}

	return line
}

// renderGroupConsolidatorLine renders a consolidator line in the execution section
func (m Model) renderGroupConsolidatorLine(inst *orchestrator.Instance, groupIndex int, selected, navigable bool, maxWidth int) string {
	var statusIcon string
	var statusStyle lipgloss.Style

	if inst == nil {
		statusIcon = "○"
		statusStyle = styles.Muted
	} else {
		switch inst.Status {
		case orchestrator.StatusCompleted:
			statusIcon = "✓"
			statusStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
		case orchestrator.StatusError:
			statusIcon = "✗"
			statusStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
		case orchestrator.StatusWorking, orchestrator.StatusWaitingInput:
			statusIcon = "⟳"
			statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
		default:
			statusIcon = "○"
			statusStyle = styles.Muted
		}
	}

	// Build line
	title := fmt.Sprintf("Consolidator (Group %d)", groupIndex+1)
	titleLen := maxWidth - 6
	title = truncate(title, titleLen)
	line := fmt.Sprintf("    %s %s", statusStyle.Render(statusIcon), title)

	// Apply styling
	if selected {
		line = lipgloss.NewStyle().
			Background(styles.PrimaryColor).
			Foreground(styles.TextColor).
			Render(line)
	} else if !navigable {
		line = styles.Muted.Render(line)
	}

	return line
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

// selectInstanceByID selects an instance by its ID, if it's navigable
func (m *Model) selectInstanceByID(instanceID string) bool {
	if m.ultraPlan == nil || instanceID == "" {
		return false
	}

	// Update the navigable instances list
	m.updateNavigableInstances()
	instances := m.ultraPlan.NavigableInstances

	// Check if this instance is navigable
	isNavigable := false
	navIdx := 0
	for i, instID := range instances {
		if instID == instanceID {
			isNavigable = true
			navIdx = i
			break
		}
	}

	if !isNavigable {
		return false
	}

	// Find and select the instance
	for i, inst := range m.session.Instances {
		if inst.ID == instanceID {
			m.activeTab = i
			m.ultraPlan.SelectedNavIdx = navIdx
			m.ensureActiveVisible()
			return true
		}
	}

	return false
}

// renderGroupDecisionSection renders the group decision dialog when user input is needed
func (m Model) renderGroupDecisionSection(decision *orchestrator.GroupDecisionState, maxWidth int) string {
	var b strings.Builder

	// Warning header
	warningStyle := lipgloss.NewStyle().
		Foreground(styles.YellowColor).
		Bold(true)
	b.WriteString(warningStyle.Render("⚠ PARTIAL GROUP FAILURE"))
	b.WriteString("\n\n")

	// Group info
	b.WriteString(fmt.Sprintf("Group %d has mixed results:\n", decision.GroupIndex+1))

	// Succeeded tasks
	if len(decision.SucceededTasks) > 0 {
		successStyle := lipgloss.NewStyle().Foreground(styles.GreenColor)
		b.WriteString(successStyle.Render(fmt.Sprintf("  ✓ %d task(s) succeeded", len(decision.SucceededTasks))))
		b.WriteString("\n")
	}

	// Failed tasks
	if len(decision.FailedTasks) > 0 {
		failStyle := lipgloss.NewStyle().Foreground(styles.RedColor)
		b.WriteString(failStyle.Render(fmt.Sprintf("  ✗ %d task(s) failed", len(decision.FailedTasks))))
		b.WriteString("\n")

		// List failed task IDs (truncated)
		maxToShow := 3
		for i, taskID := range decision.FailedTasks {
			if i >= maxToShow {
				remaining := len(decision.FailedTasks) - maxToShow
				b.WriteString(styles.Muted.Render(fmt.Sprintf("    ... +%d more", remaining)))
				b.WriteString("\n")
				break
			}
			taskDisplay := truncate(taskID, maxWidth-8)
			b.WriteString(styles.Muted.Render("    - " + taskDisplay))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Options
	b.WriteString(styles.SidebarTitle.Render("Choose action:"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  [c] Continue with successful tasks"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  [r] Retry failed tasks"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  [q] Cancel ultraplan"))
	b.WriteString("\n")

	return b.String()
}

// renderTaskLine renders a single task line in the sidebar
func (m Model) renderTaskLine(session *orchestrator.UltraPlanSession, task *orchestrator.PlannedTask, maxWidth int, selected bool) string {
	// Determine task status
	status := "○" // pending
	statusStyle := styles.Muted

	for _, ct := range session.CompletedTasks {
		if ct == task.ID {
			status = "✓"
			statusStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
			break
		}
	}

	for _, ft := range session.FailedTasks {
		if ft == task.ID {
			status = "✗"
			statusStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
			break
		}
	}

	if _, running := session.TaskToInstance[task.ID]; running {
		status = "⟳"
		statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
	}

	// Build task line
	titleLen := maxWidth - 4 // status + space + padding
	title := truncate(task.Title, titleLen)

	line := fmt.Sprintf("  %s %s", statusStyle.Render(status), title)

	// Highlight selected task
	if selected {
		line = lipgloss.NewStyle().
			Background(styles.PrimaryColor).
			Foreground(styles.TextColor).
			Render(line)
	}

	return line
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

// findNextRunnableTask finds the next or previous task that has a running instance.
// direction: +1 for next, -1 for previous
// Returns the task index or -1 if no runnable task is found.
func (m *Model) findNextRunnableTask(session *orchestrator.UltraPlanSession, direction int) int {
	if session.Plan == nil || len(session.Plan.Tasks) == 0 {
		return -1
	}

	numTasks := len(session.Plan.Tasks)
	startIdx := m.ultraPlan.SelectedTaskIdx

	// Search through all tasks in the given direction
	for i := 1; i <= numTasks; i++ {
		// Calculate next index with wrapping
		nextIdx := (startIdx + i*direction + numTasks) % numTasks

		task := &session.Plan.Tasks[nextIdx]
		if _, hasInstance := session.TaskToInstance[task.ID]; hasInstance {
			return nextIdx
		}
	}

	return -1
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

// renderPlanningSidebar renders a planning-specific sidebar during the planning phase
func (m Model) renderPlanningSidebar(width int, _ int, session *orchestrator.UltraPlanSession) string {
	var b strings.Builder

	// Title
	b.WriteString(styles.SidebarTitle.Render("Planning Phase"))
	b.WriteString("\n\n")

	// Check if multi-pass mode is enabled
	if session.Config.MultiPass {
		return m.renderMultiPassPlanningSidebar(width, 0, session)
	}

	// Show coordinator status (single-pass mode)
	if session.CoordinatorID != "" {
		inst := m.orchestrator.GetInstance(session.CoordinatorID)
		if inst != nil {
			// Status indicator
			var statusIcon string
			switch inst.Status {
			case orchestrator.StatusWorking:
				statusIcon = "⟳"
			case orchestrator.StatusCompleted:
				statusIcon = "✓"
			case orchestrator.StatusError, orchestrator.StatusStuck, orchestrator.StatusTimeout:
				statusIcon = "✗"
			case orchestrator.StatusPending:
				statusIcon = "○"
			default:
				statusIcon = "◌"
			}

			// Coordinator line
			coordLine := fmt.Sprintf("%s Coordinator", statusIcon)
			if m.activeTab == 0 { // Coordinator should be the first instance
				b.WriteString(styles.SidebarItemActive.Render(coordLine))
			} else {
				b.WriteString(styles.Muted.Render(coordLine))
			}
			b.WriteString("\n")

			// Status description
			var statusDesc string
			switch inst.Status {
			case orchestrator.StatusWorking:
				statusDesc = "Analyzing codebase..."
			case orchestrator.StatusCompleted:
				statusDesc = "Planning complete"
			case orchestrator.StatusError:
				statusDesc = "Planning failed"
			case orchestrator.StatusStuck:
				statusDesc = "Stuck - no activity"
			case orchestrator.StatusTimeout:
				statusDesc = "Timed out"
			case orchestrator.StatusPending:
				statusDesc = "Starting..."
			default:
				statusDesc = fmt.Sprintf("Status: %s", inst.Status)
			}
			b.WriteString(styles.Muted.Render("  " + statusDesc))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(styles.Muted.Render("Initializing..."))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Instructions
	b.WriteString(styles.Muted.Render("Claude is exploring the"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("codebase and creating"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("an execution plan."))

	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// renderMultiPassPlanningSection renders the multi-pass planning coordinators in the unified sidebar
// Returns the number of lines written
func (m Model) renderMultiPassPlanningSection(b *strings.Builder, session *orchestrator.UltraPlanSession, width int, availableLines int) int {
	lineCount := 0

	// Strategy names for display
	strategyNames := []string{
		"maximize-parallelism",
		"minimize-complexity",
		"balanced-approach",
	}

	// Count how many plans are ready
	plansReady := len(session.CandidatePlans)
	totalCoordinators := len(session.PlanCoordinatorIDs)
	if totalCoordinators == 0 {
		totalCoordinators = 3 // Expected count
	}

	// Show each coordinator with its strategy
	for i, strategy := range strategyNames {
		if lineCount >= availableLines {
			break
		}

		var statusIcon string
		var statusStyle lipgloss.Style

		// Determine status for this coordinator
		if i < len(session.PlanCoordinatorIDs) {
			instID := session.PlanCoordinatorIDs[i]
			inst := m.orchestrator.GetInstance(instID)
			if inst != nil {
				switch inst.Status {
				case orchestrator.StatusWorking:
					statusIcon = "⟳"
					statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
				case orchestrator.StatusCompleted:
					statusIcon = "✓"
					statusStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
				case orchestrator.StatusError, orchestrator.StatusStuck, orchestrator.StatusTimeout:
					statusIcon = "✗"
					statusStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
				case orchestrator.StatusPending:
					statusIcon = "○"
					statusStyle = styles.Muted
				default:
					statusIcon = "◌"
					statusStyle = styles.Muted
				}
			} else {
				statusIcon = "○"
				statusStyle = styles.Muted
			}
		} else {
			// Coordinator not yet spawned
			statusIcon = "○"
			statusStyle = styles.Muted
		}

		// Check if this coordinator is selected (for highlighting)
		isSelected := false
		if i < len(session.PlanCoordinatorIDs) {
			isSelected = m.isInstanceSelected(session.PlanCoordinatorIDs[i])
		}

		// Format: "  [icon] strategy-name"
		strategyLine := fmt.Sprintf("  %s %s", statusStyle.Render(statusIcon), strategy)

		if isSelected {
			b.WriteString(lipgloss.NewStyle().
				Background(styles.PrimaryColor).
				Foreground(styles.TextColor).
				Render(strategyLine))
		} else {
			b.WriteString(strategyLine)
		}
		b.WriteString("\n")
		lineCount++
	}

	// Show plans collected count
	if lineCount < availableLines {
		plansCountLine := fmt.Sprintf("  %d/%d plans ready", plansReady, totalCoordinators)
		if plansReady == totalCoordinators {
			b.WriteString(lipgloss.NewStyle().Foreground(styles.GreenColor).Render(plansCountLine))
		} else {
			b.WriteString(styles.Muted.Render(plansCountLine))
		}
		b.WriteString("\n")
		lineCount++
	}

	// Show manager status if in PlanSelection phase
	if session.Phase == orchestrator.PhasePlanSelection && lineCount < availableLines {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.YellowColor).Bold(true).Render("  Manager comparing plans..."))
		b.WriteString("\n")
		lineCount++

		// Show manager instance status if available
		if session.PlanManagerID != "" && lineCount < availableLines {
			inst := m.orchestrator.GetInstance(session.PlanManagerID)
			if inst != nil {
				var managerIcon string
				var managerStyle lipgloss.Style
				switch inst.Status {
				case orchestrator.StatusWorking:
					managerIcon = "⟳"
					managerStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
				case orchestrator.StatusCompleted:
					managerIcon = "✓"
					managerStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
				case orchestrator.StatusError:
					managerIcon = "✗"
					managerStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
				default:
					managerIcon = "○"
					managerStyle = styles.Muted
				}
				managerLine := fmt.Sprintf("    %s Manager", managerStyle.Render(managerIcon))
				b.WriteString(managerLine)
				b.WriteString("\n")
				lineCount++
			}
		}
	}

	return lineCount
}

// renderMultiPassPlanningSidebar renders the sidebar for multi-pass planning mode
func (m Model) renderMultiPassPlanningSidebar(width int, _ int, session *orchestrator.UltraPlanSession) string {
	var b strings.Builder

	// Title with multi-pass indicator
	b.WriteString(styles.SidebarTitle.Render("Multi-Pass Planning"))
	b.WriteString("\n\n")

	// Strategy names for display
	strategyNames := []string{
		"maximize-parallelism",
		"minimize-complexity",
		"balanced-approach",
	}

	// Count how many plans are ready
	plansReady := len(session.CandidatePlans)
	totalCoordinators := len(session.PlanCoordinatorIDs)
	if totalCoordinators == 0 {
		totalCoordinators = 3 // Expected count
	}

	// Show each coordinator with its strategy
	for i, strategy := range strategyNames {
		var statusIcon string
		var statusStyle lipgloss.Style

		// Determine status for this coordinator
		if i < len(session.PlanCoordinatorIDs) {
			instID := session.PlanCoordinatorIDs[i]
			inst := m.orchestrator.GetInstance(instID)
			if inst != nil {
				switch inst.Status {
				case orchestrator.StatusWorking:
					statusIcon = "⟳"
					statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
				case orchestrator.StatusCompleted:
					statusIcon = "✓"
					statusStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
				case orchestrator.StatusError, orchestrator.StatusStuck, orchestrator.StatusTimeout:
					statusIcon = "✗"
					statusStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
				case orchestrator.StatusPending:
					statusIcon = "○"
					statusStyle = styles.Muted
				default:
					statusIcon = "◌"
					statusStyle = styles.Muted
				}
			} else {
				statusIcon = "○"
				statusStyle = styles.Muted
			}
		} else {
			// Coordinator not yet spawned
			statusIcon = "○"
			statusStyle = styles.Muted
		}

		// Format: "Strategy: name [icon]"
		strategyLine := fmt.Sprintf("Strategy: %s [%s]", strategy, statusStyle.Render(statusIcon))

		// Check if this coordinator is selected (for highlighting)
		isSelected := false
		if i < len(session.PlanCoordinatorIDs) {
			instID := session.PlanCoordinatorIDs[i]
			for tabIdx, inst := range m.session.Instances {
				if inst.ID == instID && m.activeTab == tabIdx {
					isSelected = true
					break
				}
			}
		}

		if isSelected {
			b.WriteString(styles.SidebarItemActive.Render(strategyLine))
		} else {
			b.WriteString(strategyLine)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Show plans collected count
	plansCountLine := fmt.Sprintf("%d/%d plans ready", plansReady, totalCoordinators)
	if plansReady == totalCoordinators {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.GreenColor).Render(plansCountLine))
	} else {
		b.WriteString(styles.Muted.Render(plansCountLine))
	}
	b.WriteString("\n\n")

	// Show manager status if in PlanSelection phase
	if session.Phase == orchestrator.PhasePlanSelection {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.YellowColor).Bold(true).Render("Manager comparing plans..."))
		b.WriteString("\n")

		// Show manager instance status if available
		if session.PlanManagerID != "" {
			inst := m.orchestrator.GetInstance(session.PlanManagerID)
			if inst != nil {
				var managerStatus string
				switch inst.Status {
				case orchestrator.StatusWorking:
					managerStatus = "  ⟳ Evaluating..."
				case orchestrator.StatusCompleted:
					managerStatus = "  ✓ Decision made"
				case orchestrator.StatusError:
					managerStatus = "  ✗ Evaluation failed"
				default:
					managerStatus = fmt.Sprintf("  %s", inst.Status)
				}
				b.WriteString(styles.Muted.Render(managerStatus))
				b.WriteString("\n")
			}
		}
	} else if plansReady < totalCoordinators {
		// Still collecting plans
		b.WriteString(styles.Muted.Render("Coordinators exploring"))
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("with different strategies..."))
	}

	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// renderSynthesisSidebar renders a synthesis-specific sidebar during the synthesis phase
func (m Model) renderSynthesisSidebar(width int, _ int, session *orchestrator.UltraPlanSession) string {
	var b strings.Builder

	// Title
	b.WriteString(styles.SidebarTitle.Render("Synthesis Phase"))
	b.WriteString("\n\n")

	// Show synthesis instance status
	if session.SynthesisID != "" {
		inst := m.orchestrator.GetInstance(session.SynthesisID)
		if inst != nil {
			// Status indicator
			var statusIcon string
			switch inst.Status {
			case orchestrator.StatusWorking:
				statusIcon = "⟳"
			case orchestrator.StatusCompleted:
				statusIcon = "✓"
			case orchestrator.StatusError, orchestrator.StatusStuck, orchestrator.StatusTimeout:
				statusIcon = "✗"
			case orchestrator.StatusPending:
				statusIcon = "○"
			default:
				statusIcon = "◌"
			}

			// Synthesis reviewer line - highlight if active
			reviewerLine := fmt.Sprintf("%s Reviewer", statusIcon)
			// Find index of synthesis instance to check if it's active
			isActive := false
			for i, sessionInst := range m.session.Instances {
				if sessionInst.ID == session.SynthesisID && m.activeTab == i {
					isActive = true
					break
				}
			}
			if isActive {
				b.WriteString(styles.SidebarItemActive.Render(reviewerLine))
			} else {
				b.WriteString(styles.Muted.Render(reviewerLine))
			}
			b.WriteString("\n")

			// Status description
			var statusDesc string
			switch inst.Status {
			case orchestrator.StatusWorking:
				statusDesc = "Reviewing completed work..."
			case orchestrator.StatusCompleted:
				statusDesc = "Review complete"
			case orchestrator.StatusWaitingInput:
				statusDesc = "Review complete"
			case orchestrator.StatusError:
				statusDesc = "Review failed"
			case orchestrator.StatusStuck:
				statusDesc = "Stuck - no activity"
			case orchestrator.StatusTimeout:
				statusDesc = "Timed out"
			case orchestrator.StatusPending:
				statusDesc = "Starting..."
			default:
				statusDesc = fmt.Sprintf("Status: %s", inst.Status)
			}
			b.WriteString(styles.Muted.Render("  " + statusDesc))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(styles.Muted.Render("Initializing..."))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Completed tasks summary
	completedCount := len(session.CompletedTasks)
	totalTasks := 0
	if session.Plan != nil {
		totalTasks = len(session.Plan.Tasks)
	}
	b.WriteString(styles.Muted.Render(fmt.Sprintf("Tasks: %d/%d completed", completedCount, totalTasks)))
	b.WriteString("\n\n")

	// Instructions
	b.WriteString(styles.Muted.Render("Claude is reviewing all"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("completed work to ensure"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("the objective is met."))
	b.WriteString("\n\n")

	// Show what comes next
	if session.Config.ConsolidationMode != "" {
		b.WriteString(styles.Muted.Render("Next: Branch consolidation"))
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("      & PR creation"))
	}

	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// renderConsolidationSidebar renders the sidebar during the consolidation phase
func (m Model) renderConsolidationSidebar(width int, height int) string {
	v := m.createUltraplanView()
	return v.RenderConsolidationSidebar(width, height)
}

// consolidationPhaseIcon returns an icon for the consolidation phase
func consolidationPhaseIcon(phase orchestrator.ConsolidationPhase) string {
	return view.ConsolidationPhaseIcon(phase)
}

// consolidationPhaseDesc returns a human-readable description for the consolidation phase
func consolidationPhaseDesc(phase orchestrator.ConsolidationPhase) string {
	return view.ConsolidationPhaseDesc(phase)
}

// openURL opens the given URL in the default browser
func openURL(url string) error {
	return view.OpenURL(url)
}
