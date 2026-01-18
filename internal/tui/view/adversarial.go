package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// Adversarial-specific style aliases for better readability
var (
	advHighlight = styles.Primary
	advSuccess   = styles.Secondary
	advWarning   = styles.Warning
	advError     = styles.Error
	advSubtle    = styles.Muted
)

// AdversarialState holds the state for adversarial review mode
type AdversarialState struct {
	// Coordinators maps group IDs to their adversarial coordinators.
	// This enables multiple concurrent adversarial sessions.
	Coordinators map[string]*adversarial.Coordinator

	NeedsNotification bool // Set when user input is needed (checked on tick)
}

// GetCoordinatorForGroup returns the coordinator for a specific group ID.
func (s *AdversarialState) GetCoordinatorForGroup(groupID string) *adversarial.Coordinator {
	if s == nil || s.Coordinators == nil {
		return nil
	}
	return s.Coordinators[groupID]
}

// GetAllCoordinators returns all active adversarial coordinators.
// Results are sorted by group ID for deterministic ordering.
func (s *AdversarialState) GetAllCoordinators() []*adversarial.Coordinator {
	if s == nil || len(s.Coordinators) == 0 {
		return nil
	}

	// Sort keys for deterministic iteration order
	keys := make([]string, 0, len(s.Coordinators))
	for k := range s.Coordinators {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	coords := make([]*adversarial.Coordinator, 0, len(s.Coordinators))
	for _, k := range keys {
		coords = append(coords, s.Coordinators[k])
	}
	return coords
}

// HasActiveCoordinators returns true if there are any active adversarial coordinators.
func (s *AdversarialState) HasActiveCoordinators() bool {
	if s == nil {
		return false
	}
	return len(s.Coordinators) > 0
}

// AdversarialRenderContext provides the necessary context for rendering adversarial views
type AdversarialRenderContext struct {
	Orchestrator *orchestrator.Orchestrator
	Session      *orchestrator.Session
	Adversarial  *AdversarialState
	ActiveTab    int
	Width        int
	Height       int
}

// RenderAdversarialHeader renders a compact header showing adversarial review status.
// For multiple sessions, shows a summary count.
func RenderAdversarialHeader(ctx AdversarialRenderContext) string {
	if ctx.Adversarial == nil || !ctx.Adversarial.HasActiveCoordinators() {
		return ""
	}

	coordinators := ctx.Adversarial.GetAllCoordinators()
	if len(coordinators) == 0 {
		return ""
	}

	// For multiple sessions, show a summary
	if len(coordinators) > 1 {
		implementing := 0
		reviewing := 0
		complete := 0
		failed := 0
		for _, coord := range coordinators {
			session := coord.Session()
			if session == nil {
				continue
			}
			switch session.Phase {
			case adversarial.PhaseImplementing:
				implementing++
			case adversarial.PhaseReviewing:
				reviewing++
			case adversarial.PhaseComplete, adversarial.PhaseApproved:
				complete++
			case adversarial.PhaseFailed:
				failed++
			}
		}
		summary := fmt.Sprintf("%d active", len(coordinators))
		if complete > 0 {
			summary += fmt.Sprintf(", %d approved", complete)
		}
		if failed > 0 {
			summary += fmt.Sprintf(", %d failed", failed)
		}
		return advSubtle.Render("Adversarial Reviews: ") + summary
	}

	// Single session - show detailed status
	session := coordinators[0].Session()
	if session == nil {
		return ""
	}

	// Build phase indicator
	var phaseIcon string
	var phaseText string
	var phaseStyle lipgloss.Style

	switch session.Phase {
	case adversarial.PhaseImplementing:
		phaseIcon = "🔨"
		phaseText = "Implementing"
		phaseStyle = advHighlight
	case adversarial.PhaseReviewing:
		phaseIcon = "🔍"
		phaseText = "Reviewing"
		phaseStyle = advWarning
	case adversarial.PhaseApproved, adversarial.PhaseComplete:
		phaseIcon = "✓"
		phaseText = "Approved"
		phaseStyle = advSuccess
	case adversarial.PhaseFailed:
		phaseIcon = "✗"
		phaseText = "Failed"
		phaseStyle = advError
	}

	// Build the header line
	header := fmt.Sprintf("%s %s | Round %d",
		phaseIcon,
		phaseStyle.Render(phaseText),
		session.CurrentRound,
	)

	// Add max iterations if set
	if session.Config.MaxIterations > 0 {
		header += fmt.Sprintf("/%d", session.Config.MaxIterations)
	}

	return advSubtle.Render("Adversarial: ") + header
}

// RenderAdversarialSidebarSection renders a sidebar section for adversarial mode.
// For multiple sessions, iterates through all adversarial groups.
func RenderAdversarialSidebarSection(ctx AdversarialRenderContext, width int) string {
	if ctx.Adversarial == nil || !ctx.Adversarial.HasActiveCoordinators() {
		return ""
	}

	var lines []string

	// Title - show count if multiple
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.PurpleColor)
	coordinators := ctx.Adversarial.GetAllCoordinators()
	if len(coordinators) > 1 {
		lines = append(lines, titleStyle.Render(fmt.Sprintf("Adversarial Reviews (%d)", len(coordinators))))
	} else {
		lines = append(lines, titleStyle.Render("Adversarial Review"))
	}
	lines = append(lines, "")

	// Render each adversarial session
	for idx, coord := range coordinators {
		session := coord.Session()
		if session == nil {
			continue
		}

		// Add separator between sessions
		if idx > 0 {
			lines = append(lines, "")
			lines = append(lines, strings.Repeat("─", width-4))
			lines = append(lines, "")
		}

		lines = append(lines, renderSingleAdversarialSection(ctx, session, width, idx+1, len(coordinators) > 1)...)
	}

	return strings.Join(lines, "\n")
}

// renderSingleAdversarialSection renders a single adversarial session's status.
func renderSingleAdversarialSection(ctx AdversarialRenderContext, session *adversarial.Session, width int, index int, showIndex bool) []string {
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
		lines = append(lines, advSubtle.Render(fmt.Sprintf("#%d: ", index))+task)
	} else {
		lines = append(lines, advSubtle.Render("Task: ")+task)
	}
	lines = append(lines, "")

	// Phase and round status
	var phaseStyle lipgloss.Style
	var phaseText string

	switch session.Phase {
	case adversarial.PhaseImplementing:
		phaseStyle = advHighlight
		phaseText = "implementing"
	case adversarial.PhaseReviewing:
		phaseStyle = advWarning
		phaseText = "reviewing"
	case adversarial.PhaseApproved, adversarial.PhaseComplete:
		phaseStyle = advSuccess
		phaseText = "approved"
	case adversarial.PhaseFailed:
		phaseStyle = advError
		phaseText = "failed"
	default:
		phaseStyle = advSubtle
		phaseText = "pending"
	}

	roundInfo := fmt.Sprintf("Round %d", session.CurrentRound)
	if session.Config.MaxIterations > 0 {
		roundInfo += fmt.Sprintf("/%d", session.Config.MaxIterations)
	}
	lines = append(lines, advSubtle.Render("Status: ")+phaseStyle.Render(phaseText)+" "+advSubtle.Render("("+roundInfo+")"))
	lines = append(lines, "")

	// Implementer status
	lines = append(lines, advSubtle.Render("Implementer:"))
	if session.ImplementerID != "" {
		var prefix string
		if ctx.Session != nil && ctx.ActiveTab < len(ctx.Session.Instances) {
			activeInst := ctx.Session.Instances[ctx.ActiveTab]
			if activeInst != nil && activeInst.ID == session.ImplementerID {
				prefix = "▶ "
			} else {
				prefix = "  "
			}
		} else {
			prefix = "  "
		}

		implStatus := "working"
		implStyle := advHighlight
		if session.Phase == adversarial.PhaseReviewing ||
			session.Phase == adversarial.PhaseApproved ||
			session.Phase == adversarial.PhaseComplete {
			implStatus = "done"
			implStyle = advSuccess
		}
		lines = append(lines, prefix+implStyle.Render(implStatus))
	} else {
		lines = append(lines, "  "+advSubtle.Render("pending"))
	}

	// Reviewer status
	lines = append(lines, "")
	lines = append(lines, advSubtle.Render("Reviewer:"))
	if session.ReviewerID != "" {
		var prefix string
		if ctx.Session != nil && ctx.ActiveTab < len(ctx.Session.Instances) {
			activeInst := ctx.Session.Instances[ctx.ActiveTab]
			if activeInst != nil && activeInst.ID == session.ReviewerID {
				prefix = "▶ "
			} else {
				prefix = "  "
			}
		} else {
			prefix = "  "
		}

		var revStatus string
		var revStyle lipgloss.Style
		switch session.Phase {
		case adversarial.PhaseApproved, adversarial.PhaseComplete:
			revStatus = "approved"
			revStyle = advSuccess
		case adversarial.PhaseImplementing:
			revStatus = "waiting"
			revStyle = advSubtle
		default:
			revStatus = "reviewing"
			revStyle = advWarning
		}
		lines = append(lines, prefix+revStyle.Render(revStatus))
	} else if session.Phase != adversarial.PhaseImplementing {
		lines = append(lines, "  "+advSubtle.Render("pending"))
	} else {
		lines = append(lines, "  "+advSubtle.Render("waiting for implementation"))
	}

	// History summary if any rounds completed
	if len(session.History) > 0 {
		lines = append(lines, "")
		lines = append(lines, advSubtle.Render("History:"))
		for _, round := range session.History {
			if round.Review != nil {
				var scoreStyle lipgloss.Style
				switch {
				case round.Review.Score >= 8:
					scoreStyle = advSuccess
				case round.Review.Score >= 5:
					scoreStyle = advWarning
				default:
					scoreStyle = advError
				}
				status := "✗"
				if round.Review.Approved {
					status = "✓"
				}
				lines = append(lines, fmt.Sprintf("  R%d: %s %s",
					round.Round,
					scoreStyle.Render(fmt.Sprintf("%d/10", round.Review.Score)),
					advSubtle.Render(status),
				))
			}
		}
	}

	return lines
}

// RenderAdversarialHelp renders help text for adversarial mode.
func RenderAdversarialHelp(state *HelpBarState) string {
	var items []string

	items = append(items, styles.HelpKey.Render("[j/k]")+" "+advSubtle.Render("nav"))
	items = append(items, styles.HelpKey.Render("[:]")+" "+advSubtle.Render("cmd"))
	items = append(items, styles.HelpKey.Render("[?]")+" "+advSubtle.Render("help"))
	items = append(items, styles.HelpKey.Render("[q]")+" "+advSubtle.Render("quit"))

	return strings.Join(items, "  ")
}

// FindAdversarialForActiveInstance finds the adversarial session that contains
// the currently active instance (based on activeTab).
func FindAdversarialForActiveInstance(ctx AdversarialRenderContext) *adversarial.Session {
	if ctx.Session == nil || ctx.ActiveTab >= len(ctx.Session.Instances) {
		return nil
	}

	if ctx.Adversarial == nil {
		return nil
	}

	activeInst := ctx.Session.Instances[ctx.ActiveTab]
	if activeInst == nil {
		return nil
	}

	// Search through all coordinators to find which session owns this instance
	for _, coord := range ctx.Adversarial.GetAllCoordinators() {
		session := coord.Session()
		if session == nil {
			continue
		}

		// Check if active instance is the implementer
		if session.ImplementerID == activeInst.ID {
			return session
		}

		// Check if active instance is the reviewer
		if session.ReviewerID == activeInst.ID {
			return session
		}
	}

	return nil
}
