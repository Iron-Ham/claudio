package tui

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UltraPlanState holds ultra-plan specific UI state
type UltraPlanState struct {
	coordinator   *orchestrator.Coordinator
	planScrollPos int // Scroll position in the plan view
	showPlanView  bool // Toggle between plan view and normal output view
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
	if session == nil || session.Plan == nil {
		// Still in planning phase - show regular sidebar
		return m.renderSidebar(width, height)
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

			taskLine := m.renderTaskLine(session, task, width-4)
			b.WriteString(taskLine)
			b.WriteString("\n")
			lineCount++
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
func (m Model) renderTaskLine(session *orchestrator.UltraPlanSession, task *orchestrator.PlannedTask, maxWidth int) string {
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

	return fmt.Sprintf("  %s %s", statusStyle.Render(status), title)
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
		keys = append(keys, "[v] toggle plan view")
		keys = append(keys, "[c] cancel")

	case orchestrator.PhaseSynthesis:
		keys = append(keys, "[v] toggle plan view")

	case orchestrator.PhaseComplete, orchestrator.PhaseFailed:
		keys = append(keys, "[v] view plan")
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
		// Parse plan from coordinator output (only during planning phase)
		if session.Phase == orchestrator.PhasePlanning {
			// Get the coordinator instance output
			if session.CoordinatorID != "" {
				inst := m.orchestrator.GetInstance(session.CoordinatorID)
				if inst != nil {
					output := m.outputs[inst.ID]
					if output != "" {
						plan, err := orchestrator.ParsePlanFromOutput(output, session.Objective)
						if err != nil {
							m.errorMessage = fmt.Sprintf("Failed to parse plan: %v", err)
						} else {
							if err := m.ultraPlan.coordinator.SetPlan(plan); err != nil {
								m.errorMessage = fmt.Sprintf("Invalid plan: %v", err)
							} else {
								m.infoMessage = fmt.Sprintf("Plan parsed: %d tasks in %d groups", len(plan.Tasks), len(plan.ExecutionOrder))
							}
						}
					} else {
						m.infoMessage = "No output yet - wait for planning to complete"
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
	}

	return false, m, nil
}
