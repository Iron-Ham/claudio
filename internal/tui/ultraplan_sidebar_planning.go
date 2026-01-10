package tui

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// renderPlanningSidebar renders the planning phase sidebar (standalone view)
func (m Model) renderPlanningSidebar(width int, height int, session *orchestrator.UltraPlanSession) string {
	var b strings.Builder

	// Title
	b.WriteString(styles.SidebarTitle.Render("Planning Phase"))
	b.WriteString("\n\n")

	// Check if multi-pass mode is enabled
	if session.Config.MultiPass {
		return m.renderMultiPassPlanningSidebar(width, height, session)
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

// renderMultiPassPlanningSidebar renders the sidebar for multi-pass planning mode (standalone view)
func (m Model) renderMultiPassPlanningSidebar(width int, height int, session *orchestrator.UltraPlanSession) string {
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
