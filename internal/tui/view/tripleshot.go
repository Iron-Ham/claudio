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

	// Show phase and round info for adversarial mode
	if session.Config.Adversarial {
		lines = append(lines, renderAdversarialPhaseInfo(session)...)
		lines = append(lines, "")
	}

	// Render attempts - either as pairs (adversarial) or standalone
	if session.Config.Adversarial {
		lines = append(lines, renderAdversarialAttemptPairs(ctx, session)...)
	} else {
		lines = append(lines, renderStandardAttempts(ctx, session)...)
	}

	// Judge status - show for adversarial after review phase, or for standard mode
	showJudge := session.JudgeID != "" ||
		session.Phase == tripleshot.PhaseEvaluating ||
		session.Phase == tripleshot.PhaseComplete ||
		(session.Config.Adversarial && session.Phase == tripleshot.PhaseAdversarialReview)

	if showJudge {
		lines = append(lines, "")
		lines = append(lines, tsSubtle.Render("Judge:"))

		var judgeStatus string
		var judgeStyle lipgloss.Style
		switch session.Phase {
		case tripleshot.PhaseWorking:
			judgeStatus = "waiting"
			judgeStyle = tsSubtle
		case tripleshot.PhaseAdversarialReview:
			judgeStatus = "waiting for reviews"
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
		default:
			judgeStatus = "waiting"
			judgeStyle = tsSubtle
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

		// Check if active instance is one of the attempts or their reviewers
		for _, attempt := range session.Attempts {
			if attempt.InstanceID == activeInst.ID {
				return session
			}
			// Also check if active instance is the reviewer for this attempt
			if attempt.ReviewerID == activeInst.ID {
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

		// Render group header (simplified - not collapsible in tripleshot view)
		phaseColor := PhaseColor(group.Phase)
		phaseIndicator := PhaseIndicator(group.Phase)

		// Truncate name if needed (use rune-based truncation for Unicode safety)
		maxNameLen := width - 4 // Leave room for space + indicator
		displayName := truncateGroupName(group.Name, maxNameLen)

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(phaseColor)
		indicatorStyle := lipgloss.NewStyle().Foreground(phaseColor)

		lines = append(lines, headerStyle.Render(displayName)+" "+
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

// renderAdversarialPhaseInfo renders the current phase info for adversarial mode.
// This shows users which phase the tripleshot is in and the current round status.
func renderAdversarialPhaseInfo(session *tripleshot.Session) []string {
	var lines []string

	// Determine phase text and style
	var phaseText string
	var phaseStyle lipgloss.Style

	switch session.Phase {
	case tripleshot.PhaseWorking:
		phaseText = "Implementing"
		phaseStyle = tsHighlight
	case tripleshot.PhaseAdversarialReview:
		phaseText = "Under Review"
		phaseStyle = tsWarning
	case tripleshot.PhaseEvaluating:
		phaseText = "Judging"
		phaseStyle = tsHighlight
	case tripleshot.PhaseComplete:
		phaseText = "Complete"
		phaseStyle = tsSuccess
	case tripleshot.PhaseFailed:
		phaseText = "Failed"
		phaseStyle = tsError
	default:
		phaseText = "Starting"
		phaseStyle = tsSubtle
	}

	// Build phase line with round info if relevant
	phaseLine := tsSubtle.Render("Phase: ") + phaseStyle.Render(phaseText)

	// Add round info when in review phase
	if session.Phase == tripleshot.PhaseAdversarialReview {
		// Count how many attempts are under review or have reviewers
		reviewingCount := 0
		for _, attempt := range session.Attempts {
			if attempt.ReviewerID != "" {
				reviewingCount++
			}
		}
		if reviewingCount > 0 {
			phaseLine += tsSubtle.Render(fmt.Sprintf(" (%d/3 pairs active)", reviewingCount))
		}
	}

	lines = append(lines, phaseLine)

	return lines
}

// renderAdversarialAttemptPairs renders the attempts as implementer/reviewer pairs.
// Each pair is visually grouped to show the relationship between implementer and reviewer.
func renderAdversarialAttemptPairs(ctx TripleShotRenderContext, session *tripleshot.Session) []string {
	var lines []string

	lines = append(lines, tsSubtle.Render("Implementer/Reviewer Pairs:"))

	for i, attempt := range session.Attempts {
		// Add spacing between pairs
		if i > 0 {
			lines = append(lines, "")
		}

		// Pair header
		pairLabel := fmt.Sprintf("Pair %d", i+1)
		lines = append(lines, tsSubtle.Render("  "+pairLabel+":"))

		// Implementer line
		implStatus, implStyle := getAttemptStatusDisplay(attempt.Status)
		implPrefix := getInstancePrefix(ctx, attempt.InstanceID)
		lines = append(lines, implPrefix+"    Impl: "+implStyle.Render(implStatus))

		// Reviewer line - show status based on whether reviewer exists
		if attempt.ReviewerID != "" {
			revStatus, revStyle := getReviewerStatusDisplay(ctx, attempt, session.Phase)
			revPrefix := getInstancePrefix(ctx, attempt.ReviewerID)
			lines = append(lines, revPrefix+"    Rev:  "+revStyle.Render(revStatus))

			// Show review score if available
			if attempt.ReviewApproved || attempt.ReviewScore > 0 {
				scoreText := fmt.Sprintf("%d/10", attempt.ReviewScore)
				var scoreStyle lipgloss.Style
				if attempt.ReviewScore >= 8 {
					scoreStyle = tsSuccess
				} else if attempt.ReviewScore >= 5 {
					scoreStyle = tsWarning
				} else {
					scoreStyle = tsError
				}
				approvalIcon := "✗"
				if attempt.ReviewApproved {
					approvalIcon = "✓"
				}
				lines = append(lines, "          "+scoreStyle.Render(scoreText)+" "+tsSubtle.Render(approvalIcon))
			}
		} else if attempt.Status == tripleshot.AttemptStatusCompleted ||
			attempt.Status == tripleshot.AttemptStatusUnderReview {
			// Reviewer not yet assigned but implementation is ready
			lines = append(lines, "      Rev:  "+tsSubtle.Render("pending"))
		} else {
			// Implementation not ready yet, reviewer waiting
			lines = append(lines, "      Rev:  "+tsSubtle.Render("waiting"))
		}
	}

	return lines
}

// renderStandardAttempts renders the attempts in the standard (non-adversarial) format.
func renderStandardAttempts(ctx TripleShotRenderContext, session *tripleshot.Session) []string {
	var lines []string

	lines = append(lines, tsSubtle.Render("Attempts:"))
	for i, attempt := range session.Attempts {
		status, style := getAttemptStatusDisplay(attempt.Status)
		prefix := getInstancePrefix(ctx, attempt.InstanceID)
		lines = append(lines, prefix+fmt.Sprintf("%d: %s", i+1, style.Render(status)))
	}

	return lines
}

// getAttemptStatusDisplay returns the display text and style for an attempt status.
func getAttemptStatusDisplay(status tripleshot.AttemptStatus) (string, lipgloss.Style) {
	switch status {
	case tripleshot.AttemptStatusWorking:
		return "working", tsHighlight
	case tripleshot.AttemptStatusUnderReview:
		return "under review", tsWarning
	case tripleshot.AttemptStatusCompleted:
		return "done", tsSuccess
	case tripleshot.AttemptStatusFailed:
		return "failed", tsError
	default:
		return "pending", tsSubtle
	}
}

// getReviewerStatusDisplay returns the display text and style for a reviewer.
func getReviewerStatusDisplay(ctx TripleShotRenderContext, attempt tripleshot.Attempt, phase tripleshot.Phase) (string, lipgloss.Style) {
	// Check if attempt has been approved
	if attempt.ReviewApproved {
		return "approved", tsSuccess
	}

	// Check instance status if we have session context
	if ctx.Session != nil && attempt.ReviewerID != "" {
		inst := ctx.Session.GetInstance(attempt.ReviewerID)
		if inst != nil {
			switch inst.Status {
			case orchestrator.StatusCompleted:
				// Reviewer completed but didn't approve - rejected
				if attempt.ReviewScore > 0 && !attempt.ReviewApproved {
					return "rejected", tsError
				}
				return "done", tsSuccess
			case orchestrator.StatusWorking:
				return "reviewing", tsWarning
			case orchestrator.StatusError, orchestrator.StatusStuck:
				return "error", tsError
			}
		}
	}

	// Default based on phase
	if phase == tripleshot.PhaseAdversarialReview {
		return "reviewing", tsWarning
	}
	return "waiting", tsSubtle
}

// getInstancePrefix returns the prefix for an instance line based on selection state.
func getInstancePrefix(ctx TripleShotRenderContext, instanceID string) string {
	if ctx.Session != nil && ctx.ActiveTab < len(ctx.Session.Instances) && instanceID != "" {
		activeInst := ctx.Session.Instances[ctx.ActiveTab]
		if activeInst != nil && activeInst.ID == instanceID {
			return "▶ "
		}
	}
	return "  "
}
