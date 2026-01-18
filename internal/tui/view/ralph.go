package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/ralph"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// Ralph-specific style aliases for better readability
var (
	ralphHighlight = styles.Primary
	ralphSuccess   = styles.Secondary
	ralphWarning   = styles.Warning
	ralphSubtle    = styles.Muted
)

// RalphState holds the state for ralph loop mode.
// Mirrors the TUI-level RalphState for view components.
type RalphState struct {
	// Coordinators maps group IDs to their ralph coordinators.
	Coordinators map[string]*ralph.Coordinator

	// NeedsNotification is set when user notification is needed.
	NeedsNotification bool
}

// GetCoordinatorForGroup returns the coordinator for a specific group ID.
func (s *RalphState) GetCoordinatorForGroup(groupID string) *ralph.Coordinator {
	if s == nil || s.Coordinators == nil {
		return nil
	}
	return s.Coordinators[groupID]
}

// GetAllCoordinators returns all active ralph coordinators.
// Results are sorted by group ID for deterministic ordering.
func (s *RalphState) GetAllCoordinators() []*ralph.Coordinator {
	if s == nil || len(s.Coordinators) == 0 {
		return nil
	}

	// Sort keys for deterministic iteration order
	keys := make([]string, 0, len(s.Coordinators))
	for k := range s.Coordinators {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	coords := make([]*ralph.Coordinator, 0, len(s.Coordinators))
	for _, k := range keys {
		coords = append(coords, s.Coordinators[k])
	}
	return coords
}

// HasActiveCoordinators returns true if there are any active ralph coordinators.
func (s *RalphState) HasActiveCoordinators() bool {
	if s == nil {
		return false
	}
	return len(s.Coordinators) > 0
}

// RalphRenderContext provides the necessary context for rendering ralph views.
type RalphRenderContext struct {
	Orchestrator *orchestrator.Orchestrator
	Session      *orchestrator.Session
	Ralph        *RalphState
	ActiveTab    int
	Width        int
	Height       int
}

// RenderRalphHeader renders a compact header showing ralph loop status.
// For multiple sessions, shows a summary count.
func RenderRalphHeader(ctx RalphRenderContext) string {
	if ctx.Ralph == nil || !ctx.Ralph.HasActiveCoordinators() {
		return ""
	}

	coordinators := ctx.Ralph.GetAllCoordinators()
	if len(coordinators) == 0 {
		return ""
	}

	// For multiple sessions, show a summary
	if len(coordinators) > 1 {
		working := 0
		complete := 0
		stopped := 0
		for _, coord := range coordinators {
			session := coord.Session()
			if session == nil {
				continue
			}
			switch session.Phase {
			case ralph.PhaseWorking:
				working++
			case ralph.PhaseComplete:
				complete++
			case ralph.PhaseMaxIterations, ralph.PhaseCancelled, ralph.PhaseError:
				stopped++
			}
		}
		summary := fmt.Sprintf("%d active", len(coordinators))
		if complete > 0 {
			summary += fmt.Sprintf(", %d complete", complete)
		}
		if stopped > 0 {
			summary += fmt.Sprintf(", %d stopped", stopped)
		}
		return ralphSubtle.Render("Ralph Loops: ") + summary
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
	case ralph.PhaseWorking:
		phaseIcon = "♻"
		phaseText = "Running"
		phaseStyle = ralphHighlight
	case ralph.PhaseComplete:
		phaseIcon = "✓"
		phaseText = "Complete"
		phaseStyle = ralphSuccess
	case ralph.PhaseMaxIterations:
		phaseIcon = "⚠"
		phaseText = "Max Iterations"
		phaseStyle = ralphWarning
	case ralph.PhaseCancelled:
		phaseIcon = "✗"
		phaseText = "Cancelled"
		phaseStyle = ralphWarning
	case ralph.PhaseError:
		phaseIcon = "✗"
		phaseText = "Error"
		phaseStyle = styles.Error
	}

	// Build the header line
	header := fmt.Sprintf("%s %s | Iteration %d",
		phaseIcon,
		phaseStyle.Render(phaseText),
		session.CurrentIteration,
	)

	// Add max iterations if set
	if session.Config.MaxIterations > 0 {
		header += fmt.Sprintf("/%d", session.Config.MaxIterations)
	}

	return ralphSubtle.Render("Ralph: ") + header
}

// RenderRalphSidebarSection renders a sidebar section for ralph mode.
// For multiple sessions, iterates through all ralph groups.
func RenderRalphSidebarSection(ctx RalphRenderContext, width int) string {
	if ctx.Ralph == nil || !ctx.Ralph.HasActiveCoordinators() {
		return ""
	}

	var lines []string

	// Title - show count if multiple
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.BlueColor)
	coordinators := ctx.Ralph.GetAllCoordinators()
	if len(coordinators) > 1 {
		lines = append(lines, titleStyle.Render(fmt.Sprintf("Ralph Loops (%d)", len(coordinators))))
	} else {
		lines = append(lines, titleStyle.Render("Ralph Loop"))
	}
	lines = append(lines, "")

	// Render each ralph session
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

		lines = append(lines, renderSingleRalphSection(ctx, session, width, idx+1, len(coordinators) > 1)...)
	}

	return strings.Join(lines, "\n")
}

// renderSingleRalphSection renders a single ralph session's status.
func renderSingleRalphSection(ctx RalphRenderContext, session *ralph.Session, width int, index int, showIndex bool) []string {
	var lines []string

	// Prompt preview with optional index
	prompt := session.Prompt
	maxPromptLen := width - 10
	if showIndex {
		maxPromptLen -= 4 // Account for "#N: " prefix
	}
	if len(prompt) > maxPromptLen {
		prompt = prompt[:maxPromptLen-3] + "..."
	}
	if showIndex {
		lines = append(lines, ralphSubtle.Render(fmt.Sprintf("#%d: ", index))+prompt)
	} else {
		lines = append(lines, ralphSubtle.Render("Prompt: ")+prompt)
	}
	lines = append(lines, "")

	// Phase and iteration status
	var phaseStyle lipgloss.Style
	var phaseText string

	switch session.Phase {
	case ralph.PhaseWorking:
		phaseStyle = ralphHighlight
		phaseText = "running"
	case ralph.PhaseComplete:
		phaseStyle = ralphSuccess
		phaseText = "complete"
	case ralph.PhaseMaxIterations:
		phaseStyle = ralphWarning
		phaseText = "max iterations"
	case ralph.PhaseCancelled:
		phaseStyle = ralphWarning
		phaseText = "cancelled"
	case ralph.PhaseError:
		phaseStyle = styles.Error
		phaseText = "error"
	default:
		phaseStyle = ralphSubtle
		phaseText = "pending"
	}

	iterInfo := fmt.Sprintf("Iteration %d", session.CurrentIteration)
	if session.Config.MaxIterations > 0 {
		iterInfo += fmt.Sprintf("/%d", session.Config.MaxIterations)
	}
	lines = append(lines, ralphSubtle.Render("Status: ")+phaseStyle.Render(phaseText)+" "+ralphSubtle.Render("("+iterInfo+")"))

	// Completion promise
	if session.Config.CompletionPromise != "" {
		promiseText := session.Config.CompletionPromise
		maxLen := width - 12
		if len(promiseText) > maxLen {
			promiseText = promiseText[:maxLen-3] + "..."
		}
		lines = append(lines, "")
		lines = append(lines, ralphSubtle.Render("Looking for: ")+ralphWarning.Render("\""+promiseText+"\""))
	}

	// Current instance status
	if session.InstanceID != "" {
		lines = append(lines, "")
		lines = append(lines, ralphSubtle.Render("Current Instance:"))
		var prefix string
		if ctx.Session != nil && ctx.ActiveTab < len(ctx.Session.Instances) {
			activeInst := ctx.Session.Instances[ctx.ActiveTab]
			if activeInst != nil && activeInst.ID == session.InstanceID {
				prefix = "▶ "
			} else {
				prefix = "  "
			}
		} else {
			prefix = "  "
		}

		instStatus := "working"
		instStyle := ralphHighlight
		if session.Phase != ralph.PhaseWorking {
			instStatus = "done"
			instStyle = ralphSuccess
		}
		lines = append(lines, prefix+instStyle.Render(instStatus))
	}

	// Iteration history summary
	if len(session.InstanceIDs) > 1 {
		lines = append(lines, "")
		lines = append(lines, ralphSubtle.Render(fmt.Sprintf("Total instances: %d", len(session.InstanceIDs))))
	}

	return lines
}

// RenderRalphHelp renders help text for ralph mode.
func RenderRalphHelp(state *HelpBarState) string {
	var items []string

	items = append(items, styles.HelpKey.Render("[j/k]")+" "+ralphSubtle.Render("nav"))
	items = append(items, styles.HelpKey.Render("[:]")+" "+ralphSubtle.Render("cmd"))
	items = append(items, styles.HelpKey.Render("[:cancel-ralph]")+" "+ralphSubtle.Render("stop loop"))
	items = append(items, styles.HelpKey.Render("[?]")+" "+ralphSubtle.Render("help"))
	items = append(items, styles.HelpKey.Render("[q]")+" "+ralphSubtle.Render("quit"))

	return strings.Join(items, "  ")
}

// FindRalphForActiveInstance finds the ralph session that contains
// the currently active instance (based on activeTab).
func FindRalphForActiveInstance(ctx RalphRenderContext) *ralph.Session {
	if ctx.Session == nil || ctx.ActiveTab >= len(ctx.Session.Instances) {
		return nil
	}

	if ctx.Ralph == nil {
		return nil
	}

	activeInst := ctx.Session.Instances[ctx.ActiveTab]
	if activeInst == nil {
		return nil
	}

	// Search through all coordinators to find which session owns this instance
	for _, coord := range ctx.Ralph.GetAllCoordinators() {
		session := coord.Session()
		if session == nil {
			continue
		}

		// Check current instance
		if session.InstanceID == activeInst.ID {
			return session
		}

		// Check all instance IDs in the session
		for _, id := range session.InstanceIDs {
			if id == activeInst.ID {
				return session
			}
		}
	}

	return nil
}
