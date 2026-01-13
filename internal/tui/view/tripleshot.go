package view

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// Triple-shot specific style aliases for better readability
var (
	tsHighlight = styles.Primary
	tsSuccess   = styles.Secondary
	tsWarning   = styles.Warning
	tsError     = styles.Error
	tsSubtle    = styles.Muted
)

// TripleShotState holds the state for triple-shot mode
type TripleShotState struct {
	Coordinator       *orchestrator.TripleShotCoordinator
	NeedsNotification bool // Set when user input is needed (checked on tick)
}

// TripleShotRenderContext provides the necessary context for rendering triple-shot views
type TripleShotRenderContext struct {
	Orchestrator *orchestrator.Orchestrator
	Session      *orchestrator.Session
	TripleShot   *TripleShotState
	ActiveTab    int
	Width        int
	Height       int
}

// RenderTripleShotHeader renders a compact header showing triple-shot status
func RenderTripleShotHeader(ctx TripleShotRenderContext) string {
	if ctx.TripleShot == nil || ctx.TripleShot.Coordinator == nil {
		return ""
	}

	session := ctx.TripleShot.Coordinator.Session()
	if session == nil {
		return ""
	}

	// Build phase indicator
	var phaseIcon string
	var phaseText string
	var phaseStyle lipgloss.Style

	switch session.Phase {
	case orchestrator.PhaseTripleShotWorking:
		phaseIcon = "⚡"
		phaseText = "Working"
		phaseStyle = tsHighlight
	case orchestrator.PhaseTripleShotEvaluating:
		phaseIcon = "🔍"
		phaseText = "Evaluating"
		phaseStyle = tsWarning
	case orchestrator.PhaseTripleShotComplete:
		phaseIcon = "✓"
		phaseText = "Complete"
		phaseStyle = tsSuccess
	case orchestrator.PhaseTripleShotFailed:
		phaseIcon = "✗"
		phaseText = "Failed"
		phaseStyle = tsError
	}

	// Build attempt status
	var attemptStatuses []string
	for i, attempt := range session.Attempts {
		var statusIcon string
		switch attempt.Status {
		case orchestrator.AttemptStatusWorking:
			statusIcon = "⏳"
		case orchestrator.AttemptStatusCompleted:
			statusIcon = "✓"
		case orchestrator.AttemptStatusFailed:
			statusIcon = "✗"
		default:
			statusIcon = "○"
		}
		attemptStatuses = append(attemptStatuses, fmt.Sprintf("%s%d", statusIcon, i+1))
	}

	// Build the header line
	header := fmt.Sprintf("%s %s | Attempts: %s",
		phaseIcon,
		phaseStyle.Render(phaseText),
		strings.Join(attemptStatuses, " "),
	)

	// Add judge status if in evaluating phase
	if session.Phase == orchestrator.PhaseTripleShotEvaluating && session.JudgeID != "" {
		header += " | Judge: ⏳"
	} else if session.Phase == orchestrator.PhaseTripleShotComplete {
		header += " | Judge: ✓"
	}

	return tsSubtle.Render("Triple-Shot: ") + header
}
