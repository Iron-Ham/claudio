package ultraplan

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// HeaderRenderer handles rendering of the ultra-plan header with phase and progress.
type HeaderRenderer struct {
	ctx *RenderContext
}

// NewHeaderRenderer creates a new header renderer with the given context.
func NewHeaderRenderer(ctx *RenderContext) *HeaderRenderer {
	return &HeaderRenderer{ctx: ctx}
}

// Render renders the ultra-plan header with phase and progress.
func (h *HeaderRenderer) Render() string {
	if h.ctx.UltraPlan == nil || h.ctx.UltraPlan.Coordinator == nil {
		return ""
	}

	session := h.ctx.UltraPlan.Coordinator.Session()
	if session == nil {
		return ""
	}

	var b strings.Builder

	// Title with ultra-plan indicator
	title := fmt.Sprintf("Claudio Ultra-Plan: %s", truncate(session.Objective, 40))

	// Phase indicator
	phaseStr := PhaseToString(session.Phase)
	pStyle := PhaseStyle(session.Phase)

	// Build phase-appropriate progress display
	progressDisplay := h.buildProgressDisplay(session)

	// Combine
	header := fmt.Sprintf("%s  [%s]  %s",
		title,
		pStyle.Render(phaseStr),
		progressDisplay,
	)

	b.WriteString(styles.Header.Width(h.ctx.Width).Render(header))
	return b.String()
}

// buildProgressDisplay builds the progress display string based on current phase.
func (h *HeaderRenderer) buildProgressDisplay(session *orchestrator.UltraPlanSession) string {
	switch session.Phase {
	case orchestrator.PhasePlanning:
		return "analyzing codebase..."

	case orchestrator.PhasePlanSelection:
		return "comparing plans..."

	case orchestrator.PhaseRefresh:
		return "plan ready"

	case orchestrator.PhaseExecuting:
		progress := session.Progress()
		progressBar := RenderProgressBar(int(progress), 20)
		return fmt.Sprintf("%s %.0f%%", progressBar, progress)

	case orchestrator.PhaseSynthesis:
		return "reviewing..."

	case orchestrator.PhaseRevision:
		if session.Revision != nil {
			revised := len(session.Revision.RevisedTasks)
			total := len(session.Revision.TasksToRevise)
			if total > 0 {
				pct := float64(revised) / float64(total) * 100
				progressBar := RenderProgressBar(int(pct), 20)
				return fmt.Sprintf("%s round %d (%d/%d)", progressBar, session.Revision.RevisionRound, revised, total)
			}
			return fmt.Sprintf("round %d", session.Revision.RevisionRound)
		}
		return "addressing issues..."

	case orchestrator.PhaseConsolidating:
		if session.Consolidation != nil && session.Consolidation.TotalGroups > 0 {
			pct := float64(session.Consolidation.CurrentGroup) / float64(session.Consolidation.TotalGroups) * 100
			progressBar := RenderProgressBar(int(pct), 20)
			return fmt.Sprintf("%s %.0f%%", progressBar, pct)
		}
		return "consolidating..."

	case orchestrator.PhaseComplete:
		if len(session.PRUrls) > 0 {
			return fmt.Sprintf("%d PR(s) created", len(session.PRUrls))
		}
		progressBar := RenderProgressBar(100, 20)
		return fmt.Sprintf("%s 100%%", progressBar)

	case orchestrator.PhaseFailed:
		return "failed"

	default:
		progress := session.Progress()
		progressBar := RenderProgressBar(int(progress), 20)
		return fmt.Sprintf("%s %.0f%%", progressBar, progress)
	}
}
