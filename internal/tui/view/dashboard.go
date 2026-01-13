// Package view provides view components for the TUI application.
// Each view component is responsible for rendering a specific part of the UI,
// receiving model state and returning rendered strings.
package view

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// Layout constants for dashboard rendering
const (
	SidebarWidth    = 30 // Fixed sidebar width
	SidebarMinWidth = 20 // Minimum sidebar width for narrow terminals
)

// DashboardState provides the minimal state needed for dashboard rendering.
// This interface decouples the view from the full Model implementation.
type DashboardState interface {
	// Session returns the current orchestrator session
	Session() *orchestrator.Session
	// ActiveTab returns the index of the currently selected instance
	ActiveTab() int
	// SidebarScrollOffset returns the scroll offset for the sidebar
	SidebarScrollOffset() int
	// Conflicts returns the current file conflicts
	Conflicts() []conflict.FileConflict
	// TerminalWidth returns the terminal width
	TerminalWidth() int
	// TerminalHeight returns the terminal height
	TerminalHeight() int
	// IsAddingTask returns whether the user is currently adding a new task
	IsAddingTask() bool
}

// DashboardView handles rendering of the instance list/dashboard sidebar.
// It displays instance tabs, status indicators, and the instance list with
// pagination support.
type DashboardView struct{}

// NewDashboardView creates a new DashboardView instance.
func NewDashboardView() *DashboardView {
	return &DashboardView{}
}

// RenderSidebar renders the instance sidebar with pagination support.
// Returns the rendered sidebar string.
func (dv *DashboardView) RenderSidebar(state DashboardState, width, height int) string {
	var b strings.Builder

	// Sidebar title
	b.WriteString(styles.SidebarTitle.Render("Instances"))
	b.WriteString("\n")

	session := state.Session()
	instanceCount := 0
	if session != nil {
		instanceCount = len(session.Instances)
	}

	isAddingTask := state.IsAddingTask()

	if instanceCount == 0 && !isAddingTask {
		b.WriteString(styles.Muted.Render("No instances"))
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("Press [:a] to add"))
	} else {
		// Calculate available slots for instances
		// Reserve: 1 for title, 1 for blank line, 1 for add hint, 2 for scroll indicators, plus border padding
		reservedLines := 6
		if isAddingTask {
			reservedLines++
		}
		availableSlots := max(height-reservedLines, 3) // Minimum to show at least a few instances

		scrollOffset := state.SidebarScrollOffset()
		hasMoreAbove := scrollOffset > 0
		hasMoreBelow := scrollOffset+availableSlots < instanceCount

		// Show scroll up indicator if there are instances above
		if hasMoreAbove {
			scrollUp := styles.Muted.Render(fmt.Sprintf("▲ %d more above", scrollOffset))
			b.WriteString(scrollUp)
			b.WriteString("\n")
		}

		// Build a set of instance IDs that have conflicts
		conflictingInstances := dv.buildConflictMap(state.Conflicts())

		// Calculate the visible range
		startIdx := scrollOffset
		endIdx := min(scrollOffset+availableSlots, instanceCount)

		// Render visible instances
		activeTab := -1 // No instance highlighted when adding
		if !isAddingTask {
			activeTab = state.ActiveTab()
		}
		for i := startIdx; i < endIdx; i++ {
			inst := session.Instances[i]
			b.WriteString(dv.renderSidebarInstance(i, inst, conflictingInstances, activeTab, width))
			b.WriteString("\n")
		}

		// Show scroll down indicator if there are instances below
		if hasMoreBelow {
			remaining := instanceCount - endIdx
			scrollDown := styles.Muted.Render(fmt.Sprintf("▼ %d more below", remaining))
			b.WriteString(scrollDown)
			b.WriteString("\n")
		}

		if isAddingTask {
			newTaskLabel := fmt.Sprintf("%d New Task", instanceCount+1)
			dot := lipgloss.NewStyle().Foreground(styles.PrimaryColor).Render("●")
			b.WriteString(dot + " " + styles.SidebarItemActive.Render(newTaskLabel))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	// Add instance hint with navigation help when paginated
	if instanceCount > 0 {
		addHint := styles.Muted.Render("[:a]") + " " + styles.Muted.Render("add") + "  " +
			styles.Muted.Render("[Tab]") + " " + styles.Muted.Render("nav")
		b.WriteString(addHint)
	} else {
		addHint := styles.Muted.Render("[:a]") + " " + styles.Muted.Render("Add new")
		b.WriteString(addHint)
	}

	// Wrap in sidebar box
	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// renderSidebarInstance renders a single instance item in the sidebar.
func (dv *DashboardView) renderSidebarInstance(
	i int,
	inst *orchestrator.Instance,
	conflictingInstances map[string]bool,
	activeTab int,
	width int,
) string {
	// Status indicator (colored dot)
	statusColor := styles.StatusColor(string(inst.Status))
	dot := lipgloss.NewStyle().Foreground(statusColor).Render("●")

	// Build prefix icons
	prefix := ""
	prefixLen := 0

	// Add conflict indicator if instance has conflicts
	if conflictingInstances[inst.ID] {
		prefix += "⚠ "
		prefixLen += 2
	}

	// Add chain indicator if instance has dependencies (waiting for others)
	if len(inst.DependsOn) > 0 && inst.Status == orchestrator.StatusPending {
		prefix += "⛓ " // Chain icon to indicate waiting for dependencies
		prefixLen += 2
	}

	// Instance number and truncated task
	maxTaskLen := max(width-8-prefixLen, 10) // Account for number, dot, prefix, padding
	label := fmt.Sprintf("%d %s%s", i+1, prefix, truncate(inst.Task, maxTaskLen))

	// Choose style based on active state and status
	var itemStyle lipgloss.Style
	if i == activeTab {
		if conflictingInstances[inst.ID] {
			// Active item with conflict - use warning background
			itemStyle = styles.SidebarItemInputNeeded
		} else if inst.Status == orchestrator.StatusWaitingInput {
			itemStyle = styles.SidebarItemInputNeeded
		} else {
			itemStyle = styles.SidebarItemActive
		}
	} else {
		itemStyle = styles.SidebarItem
		if conflictingInstances[inst.ID] {
			// Inactive but has conflict - use warning color
			itemStyle = itemStyle.Foreground(styles.WarningColor)
		} else if inst.Status == orchestrator.StatusWaitingInput {
			itemStyle = itemStyle.Foreground(styles.WarningColor)
		} else {
			itemStyle = itemStyle.Foreground(styles.MutedColor)
		}
	}

	// Combine dot and label
	return dot + " " + itemStyle.Render(label)
}

// buildConflictMap creates a map of instance IDs that have conflicts.
func (dv *DashboardView) buildConflictMap(conflicts []conflict.FileConflict) map[string]bool {
	conflictingInstances := make(map[string]bool)
	for _, c := range conflicts {
		for _, instID := range c.Instances {
			conflictingInstances[instID] = true
		}
	}
	return conflictingInstances
}

// CalculateEffectiveSidebarWidth returns the appropriate sidebar width
// based on terminal width.
func CalculateEffectiveSidebarWidth(termWidth int) int {
	if termWidth < 80 {
		return SidebarMinWidth
	}
	return SidebarWidth
}

// CalculateMainAreaHeight returns the height available for the main content area.
func CalculateMainAreaHeight(termHeight int) int {
	return termHeight - 6 // Header + help bar + margins
}

// truncate truncates a string to max length, adding ellipsis if needed.
// Uses runes to properly handle Unicode characters.
func truncate(s string, max int) string {
	if max <= 3 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}
