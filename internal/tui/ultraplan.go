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
	"github.com/charmbracelet/lipgloss"
)

// UltraPlanState holds ultra-plan specific UI state
type UltraPlanState struct {
	coordinator     *orchestrator.Coordinator
	showPlanView    bool // Toggle between plan view and normal output view
	selectedTaskIdx int  // Currently selected task index for navigation
}

// renderUltraPlanHeader renders the ultra-plan header with phase and progress
func (m Model) renderUltraPlanHeader() string {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return m.renderHeader()
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return m.renderHeader()
	}

	var b strings.Builder

	// Title with ultra-plan indicator
	title := fmt.Sprintf("Claudio Ultra-Plan: %s", truncate(session.Objective, 40))

	// Phase indicator
	phaseStr := phaseToString(session.Phase)
	phaseStyle := phaseStyle(session.Phase)

	// Progress bar
	progress := session.Progress()
	progressBar := renderProgressBar(int(progress), 20)

	// Combine
	header := fmt.Sprintf("%s  [%s]  %s %.0f%%",
		title,
		phaseStyle.Render(phaseStr),
		progressBar,
		progress,
	)

	b.WriteString(styles.Header.Width(m.width).Render(header))
	return b.String()
}

// renderUltraPlanSidebar renders the task-oriented sidebar for ultra-plan mode
func (m Model) renderUltraPlanSidebar(width int, height int) string {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return m.renderSidebar(width, height)
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return m.renderSidebar(width, height)
	}

	// During planning phase, show a planning-specific sidebar
	if session.Plan == nil {
		return m.renderPlanningSidebar(width, height, session)
	}

	// During consolidation phase, show consolidation-specific sidebar
	if session.Phase == orchestrator.PhaseConsolidating && session.Consolidation != nil {
		return m.renderConsolidationSidebar(width, height, session)
	}

	var b strings.Builder

	// Title
	b.WriteString(styles.SidebarTitle.Render("Execution Plan"))
	b.WriteString("\n")

	// Show execution groups
	plan := session.Plan
	reservedLines := 5
	availableLines := height - reservedLines

	lineCount := 0
	taskIdx := 0 // Track flat task index for selection highlighting
	for groupIdx, group := range plan.ExecutionOrder {
		if lineCount >= availableLines {
			break
		}

		// Group header
		groupStatus := m.getGroupStatus(session, group)
		groupHeader := fmt.Sprintf("Group %d %s", groupIdx+1, groupStatus)
		b.WriteString(styles.Muted.Render(groupHeader))
		b.WriteString("\n")
		lineCount++

		// Tasks in group
		for _, taskID := range group {
			if lineCount >= availableLines {
				break
			}

			task := session.GetTask(taskID)
			if task == nil {
				continue
			}

			// Find flat index for this task
			flatIdx := -1
			for i, t := range plan.Tasks {
				if t.ID == taskID {
					flatIdx = i
					break
				}
			}

			selected := m.ultraPlan != nil && flatIdx == m.ultraPlan.selectedTaskIdx
			taskLine := m.renderTaskLine(session, task, width-4, selected)
			b.WriteString(taskLine)
			b.WriteString("\n")
			lineCount++
			taskIdx++
		}
	}

	// Summary at bottom
	b.WriteString("\n")
	completed := len(session.CompletedTasks)
	total := len(session.Plan.Tasks)
	failed := len(session.FailedTasks)
	summary := fmt.Sprintf("Done: %d/%d", completed, total)
	if failed > 0 {
		summary += fmt.Sprintf(" Failed: %d", failed)
	}
	b.WriteString(styles.Muted.Render(summary))

	return styles.Sidebar.Width(width - 2).Render(b.String())
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
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return m.renderContent(width)
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return m.renderContent(width)
	}

	// If showing plan view, render the plan
	if m.ultraPlan.showPlanView && session.Plan != nil {
		return m.renderPlanView(width)
	}

	// Otherwise, show normal instance output with ultra-plan context
	return m.renderContent(width)
}

// renderPlanView renders the detailed plan view
func (m Model) renderPlanView(width int) string {
	session := m.ultraPlan.coordinator.Session()
	if session == nil || session.Plan == nil {
		return "No plan available"
	}

	plan := session.Plan
	var b strings.Builder

	// Plan summary
	b.WriteString(styles.SidebarTitle.Render("Plan Summary"))
	b.WriteString("\n\n")
	b.WriteString(plan.Summary)
	b.WriteString("\n\n")

	// Insights
	if len(plan.Insights) > 0 {
		b.WriteString(styles.SidebarTitle.Render("Key Insights"))
		b.WriteString("\n")
		for _, insight := range plan.Insights {
			b.WriteString(fmt.Sprintf("• %s\n", insight))
		}
		b.WriteString("\n")
	}

	// Constraints
	if len(plan.Constraints) > 0 {
		b.WriteString(styles.SidebarTitle.Render("Constraints/Risks"))
		b.WriteString("\n")
		for _, constraint := range plan.Constraints {
			b.WriteString(fmt.Sprintf("⚠ %s\n", constraint))
		}
		b.WriteString("\n")
	}

	// Tasks by execution order
	b.WriteString(styles.SidebarTitle.Render("Execution Order"))
	b.WriteString("\n")
	for groupIdx, group := range plan.ExecutionOrder {
		b.WriteString(fmt.Sprintf("\nGroup %d (parallel):\n", groupIdx+1))
		for _, taskID := range group {
			task := session.GetTask(taskID)
			if task != nil {
				complexity := complexityIndicator(task.EstComplexity)
				b.WriteString(fmt.Sprintf("  [%s] %s %s\n", task.ID, complexity, task.Title))
				if len(task.Files) > 0 {
					b.WriteString(fmt.Sprintf("      Files: %s\n", strings.Join(task.Files, ", ")))
				}
			}
		}
	}

	return styles.OutputArea.Width(width - 2).Render(b.String())
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

	// Common keys
	keys = append(keys, "[q] quit")
	keys = append(keys, "[↑↓] nav")

	// Phase-specific keys
	switch session.Phase {
	case orchestrator.PhasePlanning:
		keys = append(keys, "[p] parse plan")
		keys = append(keys, "[i] input mode")

	case orchestrator.PhaseRefresh:
		keys = append(keys, "[e] start execution")

	case orchestrator.PhaseExecuting:
		keys = append(keys, "[tab] next task")
		keys = append(keys, "[1-9] select task")
		keys = append(keys, "[i] input mode")
		keys = append(keys, "[v] toggle plan view")
		keys = append(keys, "[c] cancel")

	case orchestrator.PhaseSynthesis:
		keys = append(keys, "[v] toggle plan view")

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

// renderProgressBar renders a simple ASCII progress bar
func renderProgressBar(percent int, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := (percent * width) / 100
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("[%s]", bar)
}

// phaseToString converts a phase to a display string
func phaseToString(phase orchestrator.UltraPlanPhase) string {
	switch phase {
	case orchestrator.PhasePlanning:
		return "PLANNING"
	case orchestrator.PhaseRefresh:
		return "READY"
	case orchestrator.PhaseExecuting:
		return "EXECUTING"
	case orchestrator.PhaseSynthesis:
		return "SYNTHESIS"
	case orchestrator.PhaseConsolidating:
		return "CONSOLIDATING"
	case orchestrator.PhaseComplete:
		return "COMPLETE"
	case orchestrator.PhaseFailed:
		return "FAILED"
	default:
		return string(phase)
	}
}

// phaseStyle returns the style for a phase indicator
func phaseStyle(phase orchestrator.UltraPlanPhase) lipgloss.Style {
	switch phase {
	case orchestrator.PhasePlanning:
		return lipgloss.NewStyle().Foreground(styles.BlueColor)
	case orchestrator.PhaseRefresh:
		return lipgloss.NewStyle().Foreground(styles.YellowColor)
	case orchestrator.PhaseExecuting:
		return lipgloss.NewStyle().Foreground(styles.BlueColor).Bold(true)
	case orchestrator.PhaseSynthesis:
		return lipgloss.NewStyle().Foreground(styles.PurpleColor)
	case orchestrator.PhaseConsolidating:
		return lipgloss.NewStyle().Foreground(styles.YellowColor).Bold(true)
	case orchestrator.PhaseComplete:
		return lipgloss.NewStyle().Foreground(styles.GreenColor)
	case orchestrator.PhaseFailed:
		return lipgloss.NewStyle().Foreground(styles.RedColor)
	default:
		return lipgloss.NewStyle()
	}
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
			if err := m.ultraPlan.coordinator.StartExecution(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to start execution: %v", err)
			} else {
				m.infoMessage = "Execution started"
			}
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
		// Navigate to next runnable task during execution (skip blocked tasks)
		if session.Phase == orchestrator.PhaseExecuting && session.Plan != nil {
			if nextIdx := m.findNextRunnableTask(session, 1); nextIdx >= 0 {
				m.ultraPlan.selectedTaskIdx = nextIdx
				m.selectTaskInstance(session)
			}
		}
		return true, m, nil

	case "shift+tab", "h":
		// Navigate to previous runnable task during execution (skip blocked tasks)
		if session.Phase == orchestrator.PhaseExecuting && session.Plan != nil {
			if prevIdx := m.findNextRunnableTask(session, -1); prevIdx >= 0 {
				m.ultraPlan.selectedTaskIdx = prevIdx
				m.selectTaskInstance(session)
			}
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
	startIdx := m.ultraPlan.selectedTaskIdx

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
	if session.Plan == nil || m.ultraPlan.selectedTaskIdx >= len(session.Plan.Tasks) {
		return
	}

	task := &session.Plan.Tasks[m.ultraPlan.selectedTaskIdx]
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

	// Auto-start execution if configured
	if session.Config.AutoApprove {
		if err := m.ultraPlan.coordinator.StartExecution(); err != nil {
			m.errorMessage = fmt.Sprintf("Plan ready but failed to auto-start: %v", err)
		} else {
			m.infoMessage = fmt.Sprintf("Plan ready: %d tasks in %d groups. Auto-starting execution...",
				len(plan.Tasks), len(plan.ExecutionOrder))
		}
	} else {
		m.infoMessage = fmt.Sprintf("Plan ready: %d tasks in %d groups. Press [e] to execute.",
			len(plan.Tasks), len(plan.ExecutionOrder))
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

	// Auto-start execution if configured
	if session.Config.AutoApprove {
		if err := m.ultraPlan.coordinator.StartExecution(); err != nil {
			m.errorMessage = fmt.Sprintf("Plan detected but failed to auto-start: %v", err)
		} else {
			m.infoMessage = fmt.Sprintf("Plan detected: %d tasks in %d groups. Auto-starting execution...",
				len(plan.Tasks), len(plan.ExecutionOrder))
		}
	} else {
		m.infoMessage = fmt.Sprintf("Plan detected: %d tasks in %d groups. Press [e] to execute.",
			len(plan.Tasks), len(plan.ExecutionOrder))
	}

	return true
}

// renderPlanningSidebar renders a planning-specific sidebar during the planning phase
func (m Model) renderPlanningSidebar(width int, height int, session *orchestrator.UltraPlanSession) string {
	var b strings.Builder

	// Title
	b.WriteString(styles.SidebarTitle.Render("Planning Phase"))
	b.WriteString("\n\n")

	// Show coordinator status
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

// renderConsolidationSidebar renders the sidebar during the consolidation phase
func (m Model) renderConsolidationSidebar(width int, height int, session *orchestrator.UltraPlanSession) string {
	var b strings.Builder
	state := session.Consolidation

	// Title with phase indicator
	b.WriteString(styles.SidebarTitle.Render("Consolidation"))
	b.WriteString("\n\n")

	// Phase status
	phaseIcon := consolidationPhaseIcon(state.Phase)
	phaseDesc := consolidationPhaseDesc(state.Phase)
	statusLine := fmt.Sprintf("%s %s", phaseIcon, phaseDesc)
	b.WriteString(statusLine)
	b.WriteString("\n\n")

	// Progress: Groups
	if state.TotalGroups > 0 {
		groupProgress := fmt.Sprintf("Groups: %d/%d", state.CurrentGroup, state.TotalGroups)
		b.WriteString(styles.Muted.Render(groupProgress))
		b.WriteString("\n")

		// Show group branches created
		for i, branch := range state.GroupBranches {
			prefix := "  ✓ "
			if i == state.CurrentGroup-1 && state.Phase != orchestrator.ConsolidationComplete {
				prefix = "  ⟳ "
			}
			// Truncate branch name to fit sidebar
			branchDisplay := truncate(branch, width-8)
			b.WriteString(styles.Muted.Render(prefix + branchDisplay))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Current task being merged
	if state.CurrentTask != "" {
		b.WriteString(styles.Muted.Render("Merging:"))
		b.WriteString("\n")
		taskDisplay := truncate(state.CurrentTask, width-6)
		b.WriteString(styles.Muted.Render("  " + taskDisplay))
		b.WriteString("\n\n")
	}

	// PRs created
	if len(state.PRUrls) > 0 {
		b.WriteString(styles.SidebarTitle.Render("Pull Requests"))
		b.WriteString("\n")
		for i, prURL := range state.PRUrls {
			prefix := "  ✓ "
			// Extract PR number from URL (e.g., ".../pull/123")
			prDisplay := fmt.Sprintf("PR #%d", i+1)
			if idx := strings.LastIndex(prURL, "/"); idx >= 0 {
				prDisplay = "PR " + prURL[idx+1:]
			}
			b.WriteString(styles.Muted.Render(prefix + prDisplay))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Conflict info (if paused)
	if state.Phase == orchestrator.ConsolidationPaused && len(state.ConflictFiles) > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.RedColor).Render("⚠ Conflict Detected"))
		b.WriteString("\n\n")
		b.WriteString(styles.Muted.Render("Files:"))
		b.WriteString("\n")
		maxFiles := 5
		for i, file := range state.ConflictFiles {
			if i >= maxFiles {
				remaining := len(state.ConflictFiles) - maxFiles
				b.WriteString(styles.Muted.Render(fmt.Sprintf("  ... +%d more", remaining)))
				b.WriteString("\n")
				break
			}
			fileDisplay := truncate(file, width-6)
			b.WriteString(styles.Muted.Render("  " + fileDisplay))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("Press [r] to resume"))
		b.WriteString("\n")
	}

	// Error message
	if state.Error != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.RedColor).Render("Error:"))
		b.WriteString("\n")
		errDisplay := truncate(state.Error, width-4)
		b.WriteString(styles.Muted.Render(errDisplay))
		b.WriteString("\n")
	}

	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// consolidationPhaseIcon returns an icon for the consolidation phase
func consolidationPhaseIcon(phase orchestrator.ConsolidationPhase) string {
	switch phase {
	case orchestrator.ConsolidationIdle:
		return "○"
	case orchestrator.ConsolidationDetecting:
		return "⟳"
	case orchestrator.ConsolidationCreatingBranches:
		return "⟳"
	case orchestrator.ConsolidationMergingTasks:
		return "⟳"
	case orchestrator.ConsolidationPushing:
		return "⟳"
	case orchestrator.ConsolidationCreatingPRs:
		return "⟳"
	case orchestrator.ConsolidationPaused:
		return "⏸"
	case orchestrator.ConsolidationComplete:
		return "✓"
	case orchestrator.ConsolidationFailed:
		return "✗"
	default:
		return "○"
	}
}

// consolidationPhaseDesc returns a human-readable description for the consolidation phase
func consolidationPhaseDesc(phase orchestrator.ConsolidationPhase) string {
	switch phase {
	case orchestrator.ConsolidationIdle:
		return "Waiting..."
	case orchestrator.ConsolidationDetecting:
		return "Detecting conflicts..."
	case orchestrator.ConsolidationCreatingBranches:
		return "Creating branches..."
	case orchestrator.ConsolidationMergingTasks:
		return "Merging tasks..."
	case orchestrator.ConsolidationPushing:
		return "Pushing to remote..."
	case orchestrator.ConsolidationCreatingPRs:
		return "Creating PRs..."
	case orchestrator.ConsolidationPaused:
		return "Paused (conflict)"
	case orchestrator.ConsolidationComplete:
		return "Complete"
	case orchestrator.ConsolidationFailed:
		return "Failed"
	default:
		return string(phase)
	}
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
