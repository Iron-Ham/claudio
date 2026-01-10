package tui

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

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

	// If plan editor is active, render the plan editor view
	if m.IsPlanEditorActive() && session.Plan != nil {
		return m.renderPlanEditorView(width)
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

	// Multi-pass planning source header (if applicable)
	if session.Config.MultiPass && len(session.CandidatePlans) > 0 {
		b.WriteString(styles.SidebarTitle.Render("Plan Source"))
		b.WriteString("\n")

		if session.SelectedPlanIndex == -1 {
			// Merged plan
			mergedStyle := lipgloss.NewStyle().Foreground(styles.PurpleColor)
			b.WriteString(mergedStyle.Render("⚡ Merged from multiple strategies"))
			b.WriteString("\n")
			// List the strategies that contributed
			strategyNames := orchestrator.GetMultiPassStrategyNames()
			contributingStrategies := []string{}
			for i := range session.CandidatePlans {
				if i < len(strategyNames) {
					contributingStrategies = append(contributingStrategies, strategyNames[i])
				}
			}
			if len(contributingStrategies) > 0 {
				b.WriteString(styles.Muted.Render("  Combined: " + strings.Join(contributingStrategies, ", ")))
				b.WriteString("\n")
			}
		} else if session.SelectedPlanIndex >= 0 {
			// Selected a specific strategy's plan
			strategyNames := orchestrator.GetMultiPassStrategyNames()
			strategyName := "unknown"
			if session.SelectedPlanIndex < len(strategyNames) {
				strategyName = strategyNames[session.SelectedPlanIndex]
			}
			selectedStyle := lipgloss.NewStyle().Foreground(styles.GreenColor)
			b.WriteString(selectedStyle.Render(fmt.Sprintf("✓ Strategy: %s (selected)", strategyName)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

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
