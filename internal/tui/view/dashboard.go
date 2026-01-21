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
	ExpandedNameMaxLen = 50 // Maximum length for expanded instance names
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
	// IntelligentNamingEnabled returns whether intelligent naming is enabled
	IntelligentNamingEnabled() bool
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
		// Calculate available lines for content (not slots - actual lines!)
		// Reserve: 1 for title, 1 for blank line, 1 for add hint, 2 for scroll indicators, plus border padding
		reservedLines := 6
		if isAddingTask {
			reservedLines += 2 // "New Task" entry takes 2 lines
		}
		availableLines := max(height-reservedLines, 3) // Minimum to show at least a few lines

		scrollOffset := state.SidebarScrollOffset()
		hasMoreAbove := scrollOffset > 0

		// Show scroll up indicator if there are instances above (counts as 1 line)
		if hasMoreAbove {
			scrollUp := styles.Muted.Render(fmt.Sprintf("▲ %d more above", scrollOffset))
			b.WriteString(scrollUp)
			b.WriteString("\n")
			availableLines-- // Account for scroll indicator line
		}

		// Reserve space for scroll down indicator (1 line)
		availableLines--

		// Build a set of instance IDs that have conflicts
		conflictingInstances := dv.buildConflictMap(state.Conflicts())

		// Track actual lines used and where we stop rendering
		linesUsed := 0
		lastRenderedIdx := scrollOffset - 1 // Will be updated as we render

		// Render visible instances, tracking actual line usage
		activeTab := -1 // No instance highlighted when adding
		if !isAddingTask {
			activeTab = state.ActiveTab()
		}
		intelligentNaming := state.IntelligentNamingEnabled()

		for i := scrollOffset; i < instanceCount; i++ {
			inst := session.Instances[i]
			renderedContent := dv.renderSidebarInstance(i, inst, conflictingInstances, activeTab, width, intelligentNaming)

			// Calculate how many lines this item will take
			itemLines := strings.Count(renderedContent, "\n") + 1

			// Check if adding this item would exceed available lines
			if linesUsed+itemLines > availableLines {
				break
			}

			// Render the item
			b.WriteString(renderedContent)
			b.WriteString("\n")
			linesUsed += itemLines
			lastRenderedIdx = i
		}

		// Show scroll down indicator if there are more instances
		hasMoreBelow := lastRenderedIdx < instanceCount-1
		if hasMoreBelow {
			remaining := instanceCount - lastRenderedIdx - 1
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

// renderStatusLine renders a status indicator line with proper indentation.
// The indent parameter specifies how many spaces to add before the status dot.
func renderStatusLine(status orchestrator.InstanceStatus, statusColor lipgloss.Color, indent int) string {
	return strings.Repeat(" ", indent) +
		lipgloss.NewStyle().Foreground(statusColor).Render("●"+instanceStatusAbbrev(status))
}

// renderSidebarInstance renders a single instance item in the sidebar.
// When intelligentNaming is enabled and this instance is selected (i == activeTab),
// the instance name is expanded to show more characters (up to ExpandedNameMaxLen),
// potentially wrapping to multiple lines.
func (dv *DashboardView) renderSidebarInstance(
	i int,
	inst *orchestrator.Instance,
	conflictingInstances map[string]bool,
	activeTab int,
	width int,
	intelligentNaming bool,
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

	// Choose style - only differentiate active (selected) vs inactive
	var itemStyle lipgloss.Style
	if i == activeTab {
		itemStyle = styles.SidebarItemActive
	} else {
		itemStyle = styles.SidebarItem.Foreground(styles.MutedColor)
	}

	// Calculate maximum task length based on context
	effectiveName := inst.EffectiveName()
	normalMaxLen := max(width-8-prefixLen, 10) // Account for number, dot, prefix, padding
	isSelected := i == activeTab

	// Calculate status line indent: "● " (2 chars) + number + " " + prefix
	statusIndent := 2 + len(fmt.Sprintf("%d ", i+1)) + prefixLen

	if intelligentNaming && isSelected && len([]rune(effectiveName)) > normalMaxLen {
		// Expand the selected instance name, potentially wrapping to multiple lines
		return dv.renderExpandedInstance(i, inst, effectiveName, prefix, dot, statusColor, itemStyle, statusIndent, width)
	}

	// Standard rendering with status on second line
	label := fmt.Sprintf("%d %s%s", i+1, prefix, truncate(effectiveName, normalMaxLen))
	firstLine := dot + " " + itemStyle.Render(label)
	statusLine := renderStatusLine(inst.Status, statusColor, statusIndent)

	return firstLine + "\n" + statusLine
}

// renderExpandedInstance renders an instance with an expanded name that may wrap to multiple lines.
func (dv *DashboardView) renderExpandedInstance(
	i int,
	inst *orchestrator.Instance,
	effectiveName string,
	prefix string,
	dot string,
	statusColor lipgloss.Color,
	itemStyle lipgloss.Style,
	statusIndent int,
	width int,
) string {
	// Cap at maximum expanded length
	nameRunes := []rune(effectiveName)
	if len(nameRunes) > ExpandedNameMaxLen {
		nameRunes = append(nameRunes[:ExpandedNameMaxLen-3], '.', '.', '.')
	}
	displayName := string(nameRunes)

	// Calculate how much fits on the first line
	// Format: "● N prefix<name>" where N is the instance number
	// Width deductions:
	// - 2 chars: sidebar Padding(1, 1) horizontal
	// - 2 chars: itemStyle Padding(0, 1) horizontal (from SidebarItemActive)
	// - 2 chars: safety buffer for border/edge alignment
	firstLineAvailable := max(width-statusIndent-6, 10)

	if len(nameRunes) <= firstLineAvailable {
		// Fits on one line even when expanded
		label := fmt.Sprintf("%d %s%s", i+1, prefix, displayName)
		firstLine := dot + " " + itemStyle.Render(label)
		statusLine := renderStatusLine(inst.Status, statusColor, statusIndent)
		return firstLine + "\n" + statusLine
	}

	// Need to wrap to multiple lines with word-boundary awareness
	var lines []string

	// First line: "● N prefix<part of name>"
	firstPart := wrapAtWordBoundary(nameRunes, firstLineAvailable)
	firstLabel := fmt.Sprintf("%d %s%s", i+1, prefix, firstPart)
	lines = append(lines, dot+" "+itemStyle.Render(firstLabel))

	// Continuation lines: indented with remaining text
	remaining := nameRunes[len([]rune(firstPart)):]
	// Trim leading space from remaining text after a word break
	for len(remaining) > 0 && remaining[0] == ' ' {
		remaining = remaining[1:]
	}
	// Same width deductions as first line: sidebar padding (2) + item padding (2) + buffer (2)
	continuationAvailable := max(width-statusIndent-6, 10)

	for len(remaining) > 0 {
		chunk := wrapAtWordBoundary(remaining, continuationAvailable)
		if len(chunk) == 0 {
			// Safety: prevent infinite loop if wrapAtWordBoundary returns empty
			break
		}
		remaining = remaining[len([]rune(chunk)):]
		// Trim leading space from remaining text after a word break
		for len(remaining) > 0 && remaining[0] == ' ' {
			remaining = remaining[1:]
		}

		// Indent continuation lines to align under the name
		// Pad the chunk to fill available width so background styling is consistent
		paddedChunk := chunk
		chunkLen := len([]rune(chunk))
		if chunkLen < continuationAvailable {
			paddedChunk = chunk + strings.Repeat(" ", continuationAvailable-chunkLen)
		}
		indent := strings.Repeat(" ", statusIndent)
		lines = append(lines, indent+itemStyle.Render(paddedChunk))
	}

	// Add status line at the end
	lines = append(lines, renderStatusLine(inst.Status, statusColor, statusIndent))

	return strings.Join(lines, "\n")
}

// wrapAtWordBoundary returns a substring of runes that fits within maxLen,
// breaking at the last space if possible to avoid splitting words. If no
// space is found, or if the last space is within the first 1/3 of maxLen
// (to avoid very short lines), it falls back to character-based breaking.
func wrapAtWordBoundary(runes []rune, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(runes) <= maxLen {
		return string(runes)
	}

	// Look for the last space within the available length
	lastSpace := -1
	for i := maxLen - 1; i >= 0; i-- {
		if runes[i] == ' ' {
			lastSpace = i
			break
		}
	}

	// If we found a space and it's not too early in the string (at least 1/3 of available space),
	// break at the word boundary
	if lastSpace > maxLen/3 {
		return string(runes[:lastSpace])
	}

	// No suitable word boundary found, fall back to character-based breaking
	return string(runes[:maxLen])
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
