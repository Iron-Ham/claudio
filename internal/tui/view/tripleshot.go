package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
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
	// Coordinators maps group IDs to their tripleshot coordinators.
	// This enables multiple concurrent tripleshot sessions.
	Coordinators map[string]*tripleshot.Coordinator

	NeedsNotification bool // Set when user input is needed (checked on tick)

	// PlanGroupIDs tracks groups created by :plan commands while in triple-shot mode.
	// These appear as separate sections in the sidebar.
	// Note: :ultraplan is not allowed in triple-shot mode.
	PlanGroupIDs []string
}

// GetCoordinatorForGroup returns the coordinator for a specific group ID.
func (s *TripleShotState) GetCoordinatorForGroup(groupID string) *tripleshot.Coordinator {
	if s == nil || s.Coordinators == nil {
		return nil
	}
	return s.Coordinators[groupID]
}

// GetAllCoordinators returns all active tripleshot coordinators.
// Results are sorted by group ID for deterministic ordering.
func (s *TripleShotState) GetAllCoordinators() []*tripleshot.Coordinator {
	if s == nil || len(s.Coordinators) == 0 {
		return nil
	}

	// Sort keys for deterministic iteration order
	keys := make([]string, 0, len(s.Coordinators))
	for k := range s.Coordinators {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	coords := make([]*tripleshot.Coordinator, 0, len(s.Coordinators))
	for _, k := range keys {
		coords = append(coords, s.Coordinators[k])
	}
	return coords
}

// HasActiveCoordinators returns true if there are any active tripleshot coordinators.
func (s *TripleShotState) HasActiveCoordinators() bool {
	if s == nil {
		return false
	}
	return len(s.Coordinators) > 0
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

// RenderTripleShotHeader renders a compact header showing triple-shot status.
// For multiple tripleshots, shows a summary count.
func RenderTripleShotHeader(ctx TripleShotRenderContext) string {
	if ctx.TripleShot == nil || !ctx.TripleShot.HasActiveCoordinators() {
		return ""
	}

	coordinators := ctx.TripleShot.GetAllCoordinators()
	if len(coordinators) == 0 {
		return ""
	}

	// For multiple tripleshots, show a summary
	if len(coordinators) > 1 {
		working := 0
		complete := 0
		failed := 0
		for _, coord := range coordinators {
			session := coord.Session()
			if session == nil {
				continue
			}
			switch session.Phase {
			case tripleshot.PhaseWorking, tripleshot.PhaseEvaluating:
				working++
			case tripleshot.PhaseComplete:
				complete++
			case tripleshot.PhaseFailed:
				failed++
			}
		}
		summary := fmt.Sprintf("%d active", len(coordinators))
		if complete > 0 {
			summary += fmt.Sprintf(", %d complete", complete)
		}
		if failed > 0 {
			summary += fmt.Sprintf(", %d failed", failed)
		}
		return tsSubtle.Render("Triple-Shots: ") + summary
	}

	// Single tripleshot - show detailed status
	session := coordinators[0].Session()
	if session == nil {
		return ""
	}

	// Build phase indicator
	var phaseIcon string
	var phaseText string
	var phaseStyle lipgloss.Style

	switch session.Phase {
	case tripleshot.PhaseWorking:
		phaseIcon = "⚡"
		phaseText = "Working"
		phaseStyle = tsHighlight
	case tripleshot.PhaseEvaluating:
		phaseIcon = "🔍"
		phaseText = "Evaluating"
		phaseStyle = tsWarning
	case tripleshot.PhaseComplete:
		phaseIcon = "✓"
		phaseText = "Complete"
		phaseStyle = tsSuccess
	case tripleshot.PhaseFailed:
		phaseIcon = "✗"
		phaseText = "Failed"
		phaseStyle = tsError
	}

	// Build attempt status
	var attemptStatuses []string
	for i, attempt := range session.Attempts {
		var statusIcon string
		switch attempt.Status {
		case tripleshot.AttemptStatusWorking:
			statusIcon = "⏳"
		case tripleshot.AttemptStatusCompleted:
			statusIcon = "✓"
		case tripleshot.AttemptStatusFailed:
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
	if session.Phase == tripleshot.PhaseEvaluating && session.JudgeID != "" {
		header += " | Judge: ⏳"
	} else if session.Phase == tripleshot.PhaseComplete {
		header += " | Judge: ✓"
	}

	return tsSubtle.Render("Triple-Shot: ") + header
}

// RenderTripleShotSidebarSection renders a sidebar section for triple-shot mode.
// For multiple tripleshots, iterates through all tripleshot groups.
func RenderTripleShotSidebarSection(ctx TripleShotRenderContext, width int) string {
	if ctx.TripleShot == nil || !ctx.TripleShot.HasActiveCoordinators() {
		return ""
	}

	var lines []string

	// Title - show count if multiple
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.PurpleColor)
	coordinators := ctx.TripleShot.GetAllCoordinators()
	if len(coordinators) > 1 {
		lines = append(lines, titleStyle.Render(fmt.Sprintf("Triple-Shots (%d)", len(coordinators))))
	} else {
		lines = append(lines, titleStyle.Render("Triple-Shot"))
	}
	lines = append(lines, "")

	// Render each tripleshot session
	for idx, coord := range coordinators {
		session := coord.Session()
		if session == nil {
			continue
		}

		// Add separator between tripleshots
		if idx > 0 {
			lines = append(lines, "")
			lines = append(lines, strings.Repeat("─", width-4))
			lines = append(lines, "")
		}

		lines = append(lines, renderSingleTripleShotSection(ctx, session, width, idx+1, len(coordinators) > 1)...)
	}

	// Render plan groups if any exist
	planGroupLines := renderTripleShotPlanGroups(ctx, width)
	if planGroupLines != "" {
		lines = append(lines, "")
		lines = append(lines, planGroupLines)
	}

	return strings.Join(lines, "\n")
}

// renderSingleTripleShotSection renders a single tripleshot session's status.
func renderSingleTripleShotSection(ctx TripleShotRenderContext, session *tripleshot.Session, width int, index int, showIndex bool) []string {
	var lines []string

	// Task preview with optional index
	task := session.Task
	maxTaskLen := width - 10
	if showIndex {
		maxTaskLen -= 4 // Account for "#N: " prefix
	}
	if len(task) > maxTaskLen {
		task = task[:maxTaskLen-3] + "..."
	}
	if showIndex {
		lines = append(lines, tsSubtle.Render(fmt.Sprintf("#%d: ", index))+task)
	} else {
		lines = append(lines, tsSubtle.Render("Task: ")+task)
	}
	lines = append(lines, "")

	// Attempt status
	lines = append(lines, tsSubtle.Render("Attempts:"))
	for i, attempt := range session.Attempts {
		var statusStyle lipgloss.Style
		var statusText string

		switch attempt.Status {
		case tripleshot.AttemptStatusWorking:
			statusStyle = tsHighlight
			statusText = "working"
		case tripleshot.AttemptStatusCompleted:
			statusStyle = tsSuccess
			statusText = "done"
		case tripleshot.AttemptStatusFailed:
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
	if session.JudgeID != "" || session.Phase == tripleshot.PhaseEvaluating || session.Phase == tripleshot.PhaseComplete {
		lines = append(lines, "")
		lines = append(lines, tsSubtle.Render("Judge:"))

		var judgeStatus string
		var judgeStyle lipgloss.Style
		switch session.Phase {
		case tripleshot.PhaseWorking:
			judgeStatus = "waiting"
			judgeStyle = tsSubtle
		case tripleshot.PhaseEvaluating:
			judgeStatus = "evaluating"
			judgeStyle = tsWarning
		case tripleshot.PhaseComplete:
			judgeStatus = "done"
			judgeStyle = tsSuccess
		case tripleshot.PhaseFailed:
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
	if session.Phase == tripleshot.PhaseComplete && session.Evaluation != nil {
		lines = append(lines, "")
		lines = append(lines, tsSubtle.Render("Result:"))

		eval := session.Evaluation
		if eval.MergeStrategy == tripleshot.MergeStrategySelect && eval.WinnerIndex >= 0 && eval.WinnerIndex < 3 {
			lines = append(lines, tsSuccess.Render(fmt.Sprintf("Winner: Attempt %d", eval.WinnerIndex+1)))
		} else {
			lines = append(lines, tsHighlight.Render(fmt.Sprintf("Strategy: %s", eval.MergeStrategy)))
		}
	}

	return lines
}

// findTripleShotForActiveInstance finds the tripleshot session that contains
// the currently active instance (based on activeTab).
func findTripleShotForActiveInstance(ctx TripleShotRenderContext) *tripleshot.Session {
	if ctx.Session == nil || ctx.ActiveTab >= len(ctx.Session.Instances) {
		return nil
	}

	activeInst := ctx.Session.Instances[ctx.ActiveTab]
	if activeInst == nil {
		return nil
	}

	// Search through all coordinators to find which tripleshot owns this instance
	for _, coord := range ctx.TripleShot.GetAllCoordinators() {
		session := coord.Session()
		if session == nil {
			continue
		}

		// Check if active instance is one of the attempts
		for _, attempt := range session.Attempts {
			if attempt.InstanceID == activeInst.ID {
				return session
			}
		}

		// Check if active instance is the judge
		if session.JudgeID == activeInst.ID {
			return session
		}
	}

	return nil
}

// RenderTripleShotEvaluation renders the full evaluation results.
// For multiple tripleshots, renders the evaluation for the tripleshot
// whose instance is currently selected (based on activeTab).
func RenderTripleShotEvaluation(ctx TripleShotRenderContext) string {
	if ctx.TripleShot == nil || !ctx.TripleShot.HasActiveCoordinators() {
		return ""
	}

	// Find the tripleshot session for the currently active instance
	session := findTripleShotForActiveInstance(ctx)
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
	if eval.MergeStrategy == tripleshot.MergeStrategySelect && eval.WinnerIndex >= 0 && eval.WinnerIndex < 3 {
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
			prefix := "  "
			if ctx.ActiveTab < len(ctx.Session.Instances) {
				if activeInst := ctx.Session.Instances[ctx.ActiveTab]; activeInst != nil && activeInst.ID == inst.ID {
					prefix = "▶ "
				}
			}

			// Tree connector
			connector := "├"
			if i == len(group.Instances)-1 {
				connector = "└"
			}

			// Instance display name (use rune-based truncation for Unicode safety)
			instName := truncateGroupName(inst.EffectiveName(), width-8)

			// First line: prefix + connector + name
			lines = append(lines, prefix+styles.Muted.Render(connector)+" "+instName)

			// Second line: status aligned under the instance name
			statusColor := styles.StatusColor(string(inst.Status))
			lines = append(lines, "    "+lipgloss.NewStyle().Foreground(statusColor).Render("●"+instanceStatusAbbrev(inst.Status)))
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
