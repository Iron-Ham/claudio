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

	// PlanGroupIDs tracks groups created by :plan commands while in triple-shot mode.
	// These appear as separate sections in the sidebar.
	// Note: :ultraplan is not allowed in triple-shot mode.
	PlanGroupIDs []string
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
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.PurpleColor)
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

	// Render plan groups if any exist
	planGroupLines := renderTripleShotPlanGroups(ctx, width)
	if planGroupLines != "" {
		lines = append(lines, "")
		lines = append(lines, planGroupLines)
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
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.PurpleColor)
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

// renderTripleShotPlanGroups renders the plan groups section for the tripleshot sidebar.
// This shows any plan or ultraplan groups that were started while in tripleshot mode.
func renderTripleShotPlanGroups(ctx TripleShotRenderContext, width int) string {
	if ctx.TripleShot == nil || len(ctx.TripleShot.PlanGroupIDs) == 0 {
		return ""
	}

	if ctx.Session == nil || len(ctx.Session.Groups) == 0 {
		return ""
	}

	var lines []string

	// Section header
	planTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.YellowColor)
	lines = append(lines, planTitleStyle.Render("Plans"))
	lines = append(lines, "")

	// Find and render each plan group
	for _, groupID := range ctx.TripleShot.PlanGroupIDs {
		group := findGroupByID(ctx.Session.Groups, groupID)
		if group == nil {
			continue
		}

		// Calculate group progress
		progress := CalculateGroupProgress(group, ctx.Session)

		// Render group header (simplified - not collapsible in tripleshot view)
		phaseColor := PhaseColor(group.Phase)
		phaseIndicator := PhaseIndicator(group.Phase)
		progressStr := fmt.Sprintf("[%d/%d]", progress.Completed, progress.Total)

		// Truncate name if needed (use rune-based truncation for Unicode safety)
		maxNameLen := width - len(progressStr) - 6
		displayName := truncateGroupName(group.Name, maxNameLen)

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(phaseColor)
		progressStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
		indicatorStyle := lipgloss.NewStyle().Foreground(phaseColor)

		lines = append(lines, headerStyle.Render(displayName)+" "+
			progressStyle.Render(progressStr)+" "+
			indicatorStyle.Render(phaseIndicator))

		// Render instances in this group
		for i, instID := range group.Instances {
			inst := ctx.Session.GetInstance(instID)
			if inst == nil {
				continue
			}

			// Check if this instance is active
			var prefix string
			if ctx.ActiveTab < len(ctx.Session.Instances) {
				activeInst := ctx.Session.Instances[ctx.ActiveTab]
				if activeInst != nil && activeInst.ID == inst.ID {
					prefix = "▶ "
				} else {
					prefix = "  "
				}
			} else {
				prefix = "  "
			}

			// Render instance line
			statusColor := styles.StatusColor(string(inst.Status))
			statusStyle := lipgloss.NewStyle().Foreground(statusColor)

			// Tree connector
			connector := "├"
			if i == len(group.Instances)-1 {
				connector = "└"
			}

			// Instance display name (use rune-based truncation for Unicode safety)
			instName := truncateGroupName(inst.EffectiveName(), width-12) // space for prefix, connector, status

			lines = append(lines, prefix+
				styles.Muted.Render(connector)+" "+
				instName+" "+
				statusStyle.Render(instanceStatusAbbrev(inst.Status)))
		}

		// Add blank line between groups
		lines = append(lines, "")
	}

	// Remove trailing blank line if present
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return strings.Join(lines, "\n")
}

// findGroupByID searches for a group by ID in the group list (including subgroups).
func findGroupByID(groups []*orchestrator.InstanceGroup, id string) *orchestrator.InstanceGroup {
	for _, g := range groups {
		if g.ID == id {
			return g
		}
		// Check subgroups
		if found := findGroupByID(g.SubGroups, id); found != nil {
			return found
		}
	}
	return nil
}
