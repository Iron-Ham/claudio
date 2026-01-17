package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// Ralph Wiggum specific style aliases for better readability
var (
	rwHighlight = styles.Primary
	rwSuccess   = styles.Secondary
	rwWarning   = styles.Warning
	rwError     = styles.Error
	rwSubtle    = styles.Muted
)

// RalphWiggumState holds the state for Ralph Wiggum mode
type RalphWiggumState struct {
	// Coordinators maps group IDs to their Ralph Wiggum coordinators.
	// This enables multiple concurrent Ralph Wiggum sessions.
	Coordinators map[string]*orchestrator.RalphWiggumCoordinator

	NeedsNotification bool // Set when user input is needed (checked on tick)
}

// GetCoordinatorForGroup returns the coordinator for a specific group ID.
func (s *RalphWiggumState) GetCoordinatorForGroup(groupID string) *orchestrator.RalphWiggumCoordinator {
	if s == nil || s.Coordinators == nil {
		return nil
	}
	return s.Coordinators[groupID]
}

// GetAllCoordinators returns all active Ralph Wiggum coordinators.
// Results are sorted by group ID for deterministic ordering.
func (s *RalphWiggumState) GetAllCoordinators() []*orchestrator.RalphWiggumCoordinator {
	if s == nil || len(s.Coordinators) == 0 {
		return nil
	}

	// Sort keys for deterministic iteration order
	keys := make([]string, 0, len(s.Coordinators))
	for k := range s.Coordinators {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	coords := make([]*orchestrator.RalphWiggumCoordinator, 0, len(s.Coordinators))
	for _, k := range keys {
		coords = append(coords, s.Coordinators[k])
	}
	return coords
}

// HasActiveCoordinators returns true if there are any active Ralph Wiggum coordinators.
func (s *RalphWiggumState) HasActiveCoordinators() bool {
	if s == nil {
		return false
	}
	return len(s.Coordinators) > 0
}

// RalphWiggumRenderContext provides the necessary context for rendering Ralph Wiggum views
type RalphWiggumRenderContext struct {
	Orchestrator *orchestrator.Orchestrator
	Session      *orchestrator.Session
	RalphWiggum  *RalphWiggumState
	ActiveTab    int
	Width        int
	Height       int
}

// RenderRalphWiggumHeader renders a compact header showing Ralph Wiggum status.
// For multiple sessions, shows a summary count.
func RenderRalphWiggumHeader(ctx RalphWiggumRenderContext) string {
	if ctx.RalphWiggum == nil || !ctx.RalphWiggum.HasActiveCoordinators() {
		return ""
	}

	coordinators := ctx.RalphWiggum.GetAllCoordinators()
	if len(coordinators) == 0 {
		return ""
	}

	// For multiple sessions, show a summary
	if len(coordinators) > 1 {
		iterating := 0
		complete := 0
		failed := 0
		for _, coord := range coordinators {
			session := coord.Session()
			if session == nil {
				continue
			}
			switch session.Phase {
			case orchestrator.PhaseRalphWiggumIterating:
				iterating++
			case orchestrator.PhaseRalphWiggumComplete:
				complete++
			case orchestrator.PhaseRalphWiggumFailed, orchestrator.PhaseRalphWiggumMaxIterations:
				failed++
			}
		}
		summary := fmt.Sprintf("%d active", len(coordinators))
		if complete > 0 {
			summary += fmt.Sprintf(", %d complete", complete)
		}
		if failed > 0 {
			summary += fmt.Sprintf(", %d stopped", failed)
		}
		return rwSubtle.Render("Ralph Wiggum: ") + summary
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
	case orchestrator.PhaseRalphWiggumIterating:
		phaseIcon = "∞"
		phaseText = "Iterating"
		phaseStyle = rwHighlight
	case orchestrator.PhaseRalphWiggumPaused:
		phaseIcon = "⏸"
		phaseText = "Paused"
		phaseStyle = rwWarning
	case orchestrator.PhaseRalphWiggumComplete:
		phaseIcon = "✓"
		phaseText = "Complete"
		phaseStyle = rwSuccess
	case orchestrator.PhaseRalphWiggumMaxIterations:
		phaseIcon = "⚠"
		phaseText = "Max Iterations"
		phaseStyle = rwWarning
	case orchestrator.PhaseRalphWiggumFailed:
		phaseIcon = "✗"
		phaseText = "Failed"
		phaseStyle = rwError
	}

	// Build iteration info
	iterInfo := fmt.Sprintf("Iteration %d", session.CurrentIteration())
	if session.Config.MaxIterations > 0 {
		iterInfo += fmt.Sprintf("/%d", session.Config.MaxIterations)
	}

	// Build the header line
	header := fmt.Sprintf("%s %s | %s | Promise: %s",
		phaseIcon,
		phaseStyle.Render(phaseText),
		iterInfo,
		session.Config.CompletionPromise,
	)

	return rwSubtle.Render("Ralph Wiggum: ") + header
}

// RenderRalphWiggumSidebarSection renders a sidebar section for Ralph Wiggum mode.
func RenderRalphWiggumSidebarSection(ctx RalphWiggumRenderContext, width int) string {
	if ctx.RalphWiggum == nil || !ctx.RalphWiggum.HasActiveCoordinators() {
		return ""
	}

	var lines []string

	// Title - show count if multiple
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.BlueColor)
	coordinators := ctx.RalphWiggum.GetAllCoordinators()
	if len(coordinators) > 1 {
		lines = append(lines, titleStyle.Render(fmt.Sprintf("Ralph Wiggum (%d)", len(coordinators))))
	} else {
		lines = append(lines, titleStyle.Render("Ralph Wiggum"))
	}
	lines = append(lines, "")

	// Render each Ralph Wiggum session
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

		lines = append(lines, renderSingleRalphWiggumSection(ctx, session, coord, width, idx+1, len(coordinators) > 1)...)
	}

	return strings.Join(lines, "\n")
}

// renderSingleRalphWiggumSection renders a single Ralph Wiggum session's status.
func renderSingleRalphWiggumSection(ctx RalphWiggumRenderContext, session *orchestrator.RalphWiggumSession, coord *orchestrator.RalphWiggumCoordinator, width int, index int, showIndex bool) []string {
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
		lines = append(lines, rwSubtle.Render(fmt.Sprintf("#%d: ", index))+task)
	} else {
		lines = append(lines, rwSubtle.Render("Task: ")+task)
	}
	lines = append(lines, "")

	// Phase status
	var phaseStyle lipgloss.Style
	var phaseText string
	switch session.Phase {
	case orchestrator.PhaseRalphWiggumIterating:
		phaseStyle = rwHighlight
		phaseText = "∞ Iterating"
	case orchestrator.PhaseRalphWiggumPaused:
		phaseStyle = rwWarning
		phaseText = "⏸ Paused"
	case orchestrator.PhaseRalphWiggumComplete:
		phaseStyle = rwSuccess
		phaseText = "✓ Complete"
	case orchestrator.PhaseRalphWiggumMaxIterations:
		phaseStyle = rwWarning
		phaseText = "⚠ Max Iterations"
	case orchestrator.PhaseRalphWiggumFailed:
		phaseStyle = rwError
		phaseText = "✗ Failed"
	}
	lines = append(lines, rwSubtle.Render("Status: ")+phaseStyle.Render(phaseText))
	lines = append(lines, "")

	// Iteration info
	iterInfo := fmt.Sprintf("Iteration: %d", session.CurrentIteration())
	if session.Config.MaxIterations > 0 {
		iterInfo += fmt.Sprintf(" of %d", session.Config.MaxIterations)
	}
	lines = append(lines, rwSubtle.Render(iterInfo))

	// Promise info
	lines = append(lines, rwSubtle.Render("Promise: ")+session.Config.CompletionPromise)

	// Instance indicator
	if session.InstanceID != "" {
		// Check if this instance is active
		prefix := "  "
		if ctx.Session != nil && ctx.ActiveTab < len(ctx.Session.Instances) {
			activeInst := ctx.Session.Instances[ctx.ActiveTab]
			if activeInst != nil && activeInst.ID == session.InstanceID {
				prefix = "▶ "
			}
		}
		lines = append(lines, "")
		lines = append(lines, prefix+rwSubtle.Render("Instance: ")+session.InstanceID[:8])
	}

	// Pending continue indicator
	if coord.IsPendingContinue() {
		lines = append(lines, "")
		lines = append(lines, rwWarning.Render("  Press [c] to continue"))
	}

	return lines
}

// RenderRalphWiggumHelp renders the help bar for Ralph Wiggum mode.
func RenderRalphWiggumHelp(state *HelpBarState) string {
	var parts []string

	// Mode-specific keys
	parts = append(parts, styles.HelpKey.Render("[c]")+" continue")
	parts = append(parts, styles.HelpKey.Render("[p]")+" pause")
	parts = append(parts, styles.HelpKey.Render("[x]")+" stop")
	parts = append(parts, styles.HelpKey.Render("[:]")+" command")
	parts = append(parts, styles.HelpKey.Render("[?]")+" help")
	parts = append(parts, styles.HelpKey.Render("[q]")+" quit")

	return strings.Join(parts, "  ")
}

// FindRalphWiggumForActiveInstance finds the Ralph Wiggum session that contains
// the currently active instance (based on activeTab).
// Exported for potential use by other view components.
func FindRalphWiggumForActiveInstance(ctx RalphWiggumRenderContext) *orchestrator.RalphWiggumSession {
	if ctx.Session == nil || ctx.ActiveTab >= len(ctx.Session.Instances) {
		return nil
	}

	activeInst := ctx.Session.Instances[ctx.ActiveTab]
	if activeInst == nil {
		return nil
	}

	// Search through all coordinators to find which session owns this instance
	for _, coord := range ctx.RalphWiggum.GetAllCoordinators() {
		session := coord.Session()
		if session == nil {
			continue
		}

		if session.InstanceID == activeInst.ID {
			return session
		}
	}

	return nil
}
