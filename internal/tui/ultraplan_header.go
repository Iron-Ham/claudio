package tui

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

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
	pStyle := phaseStyle(session.Phase)

	// Build phase-appropriate progress display
	var progressDisplay string
	switch session.Phase {
	case orchestrator.PhasePlanning:
		// During planning, show activity indicator instead of progress
		progressDisplay = "analyzing codebase..."

	case orchestrator.PhasePlanSelection:
		// During plan selection, show that comparison is in progress
		progressDisplay = "comparing plans..."

	case orchestrator.PhaseRefresh:
		// Plan ready, waiting for user to start execution
		progressDisplay = "plan ready"

	case orchestrator.PhaseExecuting:
		// During execution, show task progress
		progress := session.Progress()
		progressBar := renderProgressBar(int(progress), 20)
		progressDisplay = fmt.Sprintf("%s %.0f%%", progressBar, progress)

	case orchestrator.PhaseSynthesis:
		// During synthesis, show that review is in progress
		progressDisplay = "reviewing..."

	case orchestrator.PhaseRevision:
		// During revision, show revision progress
		if session.Revision != nil {
			revised := len(session.Revision.RevisedTasks)
			total := len(session.Revision.TasksToRevise)
			if total > 0 {
				pct := float64(revised) / float64(total) * 100
				progressBar := renderProgressBar(int(pct), 20)
				progressDisplay = fmt.Sprintf("%s round %d (%d/%d)", progressBar, session.Revision.RevisionRound, revised, total)
			} else {
				progressDisplay = fmt.Sprintf("round %d", session.Revision.RevisionRound)
			}
		} else {
			progressDisplay = "addressing issues..."
		}

	case orchestrator.PhaseConsolidating:
		// During consolidation, show consolidation progress if available
		if session.Consolidation != nil && session.Consolidation.TotalGroups > 0 {
			pct := float64(session.Consolidation.CurrentGroup) / float64(session.Consolidation.TotalGroups) * 100
			progressBar := renderProgressBar(int(pct), 20)
			progressDisplay = fmt.Sprintf("%s %.0f%%", progressBar, pct)
		} else {
			progressDisplay = "consolidating..."
		}

	case orchestrator.PhaseComplete:
		// Show completion with PR count if available
		if len(session.PRUrls) > 0 {
			progressDisplay = fmt.Sprintf("%d PR(s) created", len(session.PRUrls))
		} else {
			progressBar := renderProgressBar(100, 20)
			progressDisplay = fmt.Sprintf("%s 100%%", progressBar)
		}

	case orchestrator.PhaseFailed:
		progressDisplay = "failed"

	default:
		progress := session.Progress()
		progressBar := renderProgressBar(int(progress), 20)
		progressDisplay = fmt.Sprintf("%s %.0f%%", progressBar, progress)
	}

	// Combine
	header := fmt.Sprintf("%s  [%s]  %s",
		title,
		pStyle.Render(phaseStr),
		progressDisplay,
	)

	b.WriteString(styles.Header.Width(m.width).Render(header))
	return b.String()
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
	case orchestrator.PhasePlanSelection:
		return "SELECTING PLAN"
	case orchestrator.PhaseRefresh:
		return "READY"
	case orchestrator.PhaseExecuting:
		return "EXECUTING"
	case orchestrator.PhaseSynthesis:
		return "SYNTHESIS"
	case orchestrator.PhaseRevision:
		return "REVISION"
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
	case orchestrator.PhasePlanSelection:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true) // Orange for plan selection (intermediate state)
	case orchestrator.PhaseRefresh:
		return lipgloss.NewStyle().Foreground(styles.YellowColor)
	case orchestrator.PhaseExecuting:
		return lipgloss.NewStyle().Foreground(styles.BlueColor).Bold(true)
	case orchestrator.PhaseSynthesis:
		return lipgloss.NewStyle().Foreground(styles.PurpleColor)
	case orchestrator.PhaseRevision:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true) // Orange for revision
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
