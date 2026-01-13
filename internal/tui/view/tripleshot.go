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
	tsPurple    = styles.PrimaryColor
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

// RenderTripleShotSidebarSection renders a sidebar section for triple-shot mode
func RenderTripleShotSidebarSection(ctx TripleShotRenderContext, width int) string {
	if ctx.TripleShot == nil || ctx.TripleShot.Coordinator == nil {
		return ""
	}

	session := ctx.TripleShot.Coordinator.Session()
	if session == nil {
		return ""
	}

	var lines []string

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(tsPurple)
	lines = append(lines, titleStyle.Render("Triple-Shot"))
	lines = append(lines, "")

	// Task preview
	task := session.Task
	if len(task) > width-4 {
		task = task[:width-7] + "..."
	}
	lines = append(lines, tsSubtle.Render("Task: ")+task)
	lines = append(lines, "")

	// Attempt status
	lines = append(lines, tsSubtle.Render("Attempts:"))
	for i, attempt := range session.Attempts {
		var statusStyle lipgloss.Style
		var statusText string

		switch attempt.Status {
		case orchestrator.AttemptStatusWorking:
			statusStyle = tsHighlight
			statusText = "working"
		case orchestrator.AttemptStatusCompleted:
			statusStyle = tsSuccess
			statusText = "done"
		case orchestrator.AttemptStatusFailed:
			statusStyle = tsError
			statusText = "failed"
		default:
			statusStyle = tsSubtle
			statusText = "pending"
		}

		// Highlight if this attempt's instance is currently selected
		var prefix string
		if ctx.Session != nil && ctx.ActiveTab < len(ctx.Session.Instances) {
			activeInst := ctx.Session.Instances[ctx.ActiveTab]
			if activeInst != nil && activeInst.ID == attempt.InstanceID {
				prefix = "▶ "
			} else {
				prefix = "  "
			}
		} else {
			prefix = "  "
		}

		lines = append(lines, prefix+fmt.Sprintf("%d: %s", i+1, statusStyle.Render(statusText)))
	}

	// Judge status
	if session.JudgeID != "" || session.Phase == orchestrator.PhaseTripleShotEvaluating || session.Phase == orchestrator.PhaseTripleShotComplete {
		lines = append(lines, "")
		lines = append(lines, tsSubtle.Render("Judge:"))

		var judgeStatus string
		var judgeStyle lipgloss.Style
		switch session.Phase {
		case orchestrator.PhaseTripleShotWorking:
			judgeStatus = "waiting"
			judgeStyle = tsSubtle
		case orchestrator.PhaseTripleShotEvaluating:
			judgeStatus = "evaluating"
			judgeStyle = tsWarning
		case orchestrator.PhaseTripleShotComplete:
			judgeStatus = "done"
			judgeStyle = tsSuccess
		case orchestrator.PhaseTripleShotFailed:
			judgeStatus = "failed"
			judgeStyle = tsError
		}

		var prefix string
		if ctx.Session != nil && ctx.ActiveTab < len(ctx.Session.Instances) {
			activeInst := ctx.Session.Instances[ctx.ActiveTab]
			if activeInst != nil && activeInst.ID == session.JudgeID {
				prefix = "▶ "
			} else {
				prefix = "  "
			}
		} else {
			prefix = "  "
		}

		lines = append(lines, prefix+judgeStyle.Render(judgeStatus))
	}

	// Evaluation result preview (if complete)
	if session.Phase == orchestrator.PhaseTripleShotComplete && session.Evaluation != nil {
		lines = append(lines, "")
		lines = append(lines, tsSubtle.Render("Result:"))

		eval := session.Evaluation
		if eval.MergeStrategy == orchestrator.MergeStrategySelect && eval.WinnerIndex >= 0 && eval.WinnerIndex < 3 {
			lines = append(lines, tsSuccess.Render(fmt.Sprintf("Winner: Attempt %d", eval.WinnerIndex+1)))
		} else {
			lines = append(lines, tsHighlight.Render(fmt.Sprintf("Strategy: %s", eval.MergeStrategy)))
		}
	}

	return strings.Join(lines, "\n")
}

// RenderTripleShotEvaluation renders the full evaluation results
func RenderTripleShotEvaluation(ctx TripleShotRenderContext) string {
	if ctx.TripleShot == nil || ctx.TripleShot.Coordinator == nil {
		return ""
	}

	session := ctx.TripleShot.Coordinator.Session()
	if session == nil || session.Evaluation == nil {
		return ""
	}

	eval := session.Evaluation
	var lines []string

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(tsPurple)
	lines = append(lines, titleStyle.Render("Evaluation Results"))
	lines = append(lines, strings.Repeat("─", 40))
	lines = append(lines, "")

	// Strategy and winner
	if eval.MergeStrategy == orchestrator.MergeStrategySelect && eval.WinnerIndex >= 0 && eval.WinnerIndex < 3 {
		lines = append(lines, tsSuccess.Render(fmt.Sprintf("Winner: Attempt %d", eval.WinnerIndex+1)))
	} else {
		lines = append(lines, tsHighlight.Render(fmt.Sprintf("Strategy: %s", eval.MergeStrategy)))
	}
	lines = append(lines, "")

	// Reasoning
	lines = append(lines, tsSubtle.Render("Reasoning:"))
	// Word wrap reasoning
	words := strings.Fields(eval.Reasoning)
	var line string
	for _, word := range words {
		if len(line)+len(word)+1 > ctx.Width-4 {
			lines = append(lines, "  "+line)
			line = word
		} else if line == "" {
			line = word
		} else {
			line += " " + word
		}
	}
	if line != "" {
		lines = append(lines, "  "+line)
	}
	lines = append(lines, "")

	// Individual attempt scores
	lines = append(lines, tsSubtle.Render("Attempt Scores:"))
	for _, attemptEval := range eval.AttemptEvaluation {
		var scoreStyle lipgloss.Style
		switch {
		case attemptEval.Score >= 8:
			scoreStyle = tsSuccess
		case attemptEval.Score >= 6:
			scoreStyle = tsWarning
		default:
			scoreStyle = tsError
		}

		lines = append(lines, fmt.Sprintf("  Attempt %d: %s",
			attemptEval.AttemptIndex+1,
			scoreStyle.Render(fmt.Sprintf("%d/10", attemptEval.Score)),
		))

		if len(attemptEval.Strengths) > 0 {
			lines = append(lines, tsSuccess.Render("    Strengths: ")+strings.Join(attemptEval.Strengths, ", "))
		}
		if len(attemptEval.Weaknesses) > 0 {
			lines = append(lines, tsError.Render("    Weaknesses: ")+strings.Join(attemptEval.Weaknesses, ", "))
		}
	}

	// Suggested changes if merging
	if len(eval.SuggestedChanges) > 0 {
		lines = append(lines, "")
		lines = append(lines, tsSubtle.Render("Suggested Changes:"))
		for _, change := range eval.SuggestedChanges {
			lines = append(lines, "  • "+change)
		}
	}

	return strings.Join(lines, "\n")
}
