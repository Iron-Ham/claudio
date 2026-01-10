package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// renderContent renders the main content panel based on current state.
// It acts as a router, delegating to specialized renderers for each view mode.
func (m Model) renderContent(width int) string {
	if m.addingTask {
		return m.renderAddTask(width)
	}

	if m.showHelp {
		return m.renderHelpPanel(width)
	}

	if m.showDiff {
		return m.renderDiffPanel(width)
	}

	if m.showConflicts && len(m.conflicts) > 0 {
		return m.renderConflictPanel(width)
	}

	if m.showStats {
		return m.renderStatsPanel(width)
	}

	if m.filterMode {
		return m.renderFilterPanel(width)
	}

	inst := m.activeInstance()
	if inst == nil {
		return styles.ContentBox.Width(width - 4).Render(
			"No instance selected.\n\nPress [a] to add a new Claude instance.",
		)
	}

	return m.renderInstance(inst, width)
}

// renderInstance renders the active instance view with status, task, metrics, and scrollable output.
func (m Model) renderInstance(inst *orchestrator.Instance, width int) string {
	var b strings.Builder

	// Instance info
	statusColor := styles.StatusColor(string(inst.Status))
	statusBadge := styles.StatusBadge.Background(statusColor).Render(string(inst.Status))

	info := fmt.Sprintf("%s  Branch: %s", statusBadge, inst.Branch)
	b.WriteString(styles.InstanceInfo.Render(info))
	b.WriteString("\n")

	// Task (limit to 5 lines to prevent dominating the view)
	const maxTaskLines = 5
	taskDisplay := truncateLines(inst.Task, maxTaskLines)
	b.WriteString(styles.Subtitle.Render("Task: " + taskDisplay))
	b.WriteString("\n")

	// Resource metrics (if available and config enabled)
	cfg := config.Get()
	if cfg.Resources.ShowMetricsInSidebar && inst.Metrics != nil {
		metricsLine := m.formatInstanceMetrics(inst.Metrics)
		b.WriteString(styles.Muted.Render(metricsLine))
		b.WriteString("\n")
	}

	// Show running/input mode status
	mgr := m.orchestrator.GetInstanceManager(inst.ID)
	if mgr != nil && mgr.Running() {
		if m.inputMode {
			// Show active input mode indicator
			inputBanner := lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.TextColor).
				Background(styles.WarningColor).
				Padding(0, 1).
				Render("INPUT MODE")
			hint := inputBanner + "  " + styles.Muted.Render("Press ") +
				styles.HelpKey.Render("Ctrl+]") + styles.Muted.Render(" to exit")
			b.WriteString(hint)
		} else {
			// Show hint to enter input mode
			runningBanner := lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.TextColor).
				Background(styles.SecondaryColor).
				Padding(0, 1).
				Render("RUNNING")
			hint := runningBanner + "  " + styles.Muted.Render("Press ") +
				styles.HelpKey.Render("[i]") + styles.Muted.Render(" to interact  ") +
				styles.HelpKey.Render("[t]") + styles.Muted.Render(" for tmux attach cmd")
			b.WriteString(hint)
		}
	}
	b.WriteString("\n")

	// Output with scrolling support
	output := m.outputs[inst.ID]
	if output == "" {
		output = "No output yet. Press [s] to start this instance."
		outputBox := styles.OutputArea.
			Width(width - 4).
			Height(m.getOutputMaxLines()).
			Render(output)
		b.WriteString(outputBox)
		return b.String()
	}

	// Apply filters
	output = m.filterOutput(output)

	// Split output into lines and apply scroll
	lines := strings.Split(output, "\n")
	totalLines := len(lines)
	maxLines := m.getOutputMaxLines()

	// Get scroll position
	scrollOffset := m.outputState.GetScroll(inst.ID)
	maxScroll := m.getOutputMaxScroll(inst.ID)

	// Clamp scroll position
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}

	// Calculate visible range
	startLine := scrollOffset
	endLine := startLine + maxLines
	if endLine > totalLines {
		endLine = totalLines
	}

	// Get visible lines
	var visibleLines []string
	if totalLines <= maxLines {
		// No scrolling needed, show all
		visibleLines = lines
	} else {
		visibleLines = lines[startLine:endLine]
	}

	// Apply search highlighting
	if m.searchRegex != nil && m.searchPattern != "" {
		visibleLines = m.highlightSearchMatches(visibleLines, startLine)
	}

	visibleOutput := strings.Join(visibleLines, "\n")

	// Build scroll indicator
	var scrollIndicator string
	if totalLines > maxLines {
		// Show scroll position
		autoScrollEnabled := m.isOutputAutoScroll(inst.ID)
		hasNew := m.hasNewOutput(inst.ID) && !autoScrollEnabled

		if hasNew {
			// New output arrived while scrolled up
			scrollIndicator = styles.Warning.Render(fmt.Sprintf("▲ NEW OUTPUT - Line %d/%d", startLine+1, totalLines)) +
				"  " + styles.Muted.Render("[G] jump to latest")
		} else if scrollOffset == 0 && !autoScrollEnabled {
			// At top
			scrollIndicator = styles.Muted.Render(fmt.Sprintf("▲ TOP - Line 1/%d", totalLines)) +
				"  " + styles.Muted.Render("[j/↓] down  [G] bottom")
		} else if autoScrollEnabled {
			// Auto-scrolling (at bottom)
			scrollIndicator = styles.Secondary.Render(fmt.Sprintf("▼ FOLLOWING - Line %d/%d", endLine, totalLines)) +
				"  " + styles.Muted.Render("[k/↑] scroll up")
		} else {
			// Scrolled somewhere in the middle
			percent := 0
			if maxScroll > 0 {
				percent = scrollOffset * 100 / maxScroll
			}
			scrollIndicator = styles.Muted.Render(fmt.Sprintf("Line %d-%d/%d (%d%%)", startLine+1, endLine, totalLines, percent)) +
				"  " + styles.Muted.Render("[j/k] scroll  [g/G] top/bottom")
		}
		b.WriteString(scrollIndicator)
		b.WriteString("\n")
	}

	outputBox := styles.OutputArea.
		Width(width - 4).
		Height(maxLines).
		Render(visibleOutput)

	b.WriteString(outputBox)

	// Show search bar if in search mode or has active search
	if m.searchMode || m.searchPattern != "" {
		b.WriteString("\n")
		b.WriteString(m.renderSearchBar())
	}

	return b.String()
}

// highlightSearchMatches highlights search matches in visible lines.
func (m Model) highlightSearchMatches(lines []string, startLine int) []string {
	if m.searchRegex == nil {
		return lines
	}

	result := make([]string, len(lines))
	for i, line := range lines {
		lineNum := startLine + i
		isCurrentMatchLine := false

		// Check if this line contains the current match
		if len(m.searchMatches) > 0 && m.searchCurrent < len(m.searchMatches) {
			if lineNum == m.searchMatches[m.searchCurrent] {
				isCurrentMatchLine = true
			}
		}

		// Find and highlight all matches in this line
		matches := m.searchRegex.FindAllStringIndex(line, -1)
		if len(matches) == 0 {
			result[i] = line
			continue
		}

		var highlighted strings.Builder
		lastEnd := 0
		for j, match := range matches {
			// Add text before match
			highlighted.WriteString(line[lastEnd:match[0]])

			// Highlight the match
			matchText := line[match[0]:match[1]]
			if isCurrentMatchLine && j == 0 {
				// Current match gets special highlighting
				highlighted.WriteString(styles.SearchCurrentMatch.Render(matchText))
			} else {
				highlighted.WriteString(styles.SearchMatch.Render(matchText))
			}
			lastEnd = match[1]
		}
		// Add remaining text after last match
		highlighted.WriteString(line[lastEnd:])
		result[i] = highlighted.String()
	}

	return result
}

// renderAddTask renders the add task input form.
func (m Model) renderAddTask(width int) string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Add New Instance"))
	b.WriteString("\n\n")
	b.WriteString("Enter task description:\n\n")

	// Render text with cursor at the correct position
	runes := []rune(m.taskInput)
	// Clamp cursor to valid bounds as a safety measure
	cursor := m.taskInputCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	beforeCursor := string(runes[:cursor])
	afterCursor := string(runes[cursor:])

	// Build the display with cursor indicator
	// The cursor line gets a ">" prefix, other lines get "  " prefix
	fullText := beforeCursor + "█" + afterCursor
	lines := strings.Split(fullText, "\n")

	// Find which line contains the cursor
	cursorLineIdx := strings.Count(beforeCursor, "\n")

	for i, line := range lines {
		if i == cursorLineIdx {
			b.WriteString("> " + line)
		} else {
			b.WriteString("  " + line)
		}
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}

	// Show template dropdown if active
	if m.showTemplates {
		b.WriteString("\n")
		b.WriteString(m.renderTemplateDropdown())
	}

	b.WriteString("\n\n")
	if m.showTemplates {
		b.WriteString(styles.Muted.Render("↑/↓") + " navigate  " +
			styles.Muted.Render("Enter/Tab") + " select  " +
			styles.Muted.Render("Esc") + " close  " +
			styles.Muted.Render("Type") + " filter")
	} else {
		b.WriteString(styles.Muted.Render("Enter") + " submit  " +
			styles.Muted.Render("Shift+Enter") + " newline  " +
			styles.Muted.Render("/") + " templates  " +
			styles.Muted.Render("Esc") + " cancel")
	}

	return styles.ContentBox.Width(width - 4).Render(b.String())
}

// renderTemplateDropdown renders the template selection dropdown.
func (m Model) renderTemplateDropdown() string {
	templates := FilterTemplates(m.templateFilter)
	if len(templates) == 0 {
		return styles.Muted.Render("  No matching templates")
	}

	var items []string
	for i, t := range templates {
		cmd := "/" + t.Command
		name := " - " + t.Name

		var item string
		if i == m.templateSelected {
			// Selected item - highlight the whole row
			item = styles.DropdownItemSelected.Render(cmd + name)
		} else {
			// Normal item - color command and name differently
			item = styles.DropdownItem.Render(
				styles.DropdownCommand.Render(cmd) +
					styles.DropdownName.Render(name),
			)
		}
		items = append(items, item)
	}

	content := strings.Join(items, "\n")
	return styles.DropdownContainer.Render(content)
}

// renderHelp renders the help bar at the bottom of the screen.
func (m Model) renderHelp() string {
	if m.inputMode {
		return styles.HelpBar.Render(
			styles.Warning.Bold(true).Render("INPUT MODE") + "  " +
				styles.HelpKey.Render("[Ctrl+]]") + " exit input mode  " +
				"All keystrokes forwarded to Claude",
		)
	}

	if m.commandMode {
		return styles.HelpBar.Render(
			styles.Primary.Bold(true).Render(":") + styles.Primary.Render(m.commandBuffer) +
				styles.Muted.Render("█") + "  " +
				styles.HelpKey.Render("[Enter]") + " execute  " +
				styles.HelpKey.Render("[Esc]") + " cancel  " +
				styles.Muted.Render("Type command (e.g., :s :a :d :h)"),
		)
	}

	if m.showDiff {
		return styles.HelpBar.Render(
			styles.Primary.Bold(true).Render("DIFF VIEW") + "  " +
				styles.HelpKey.Render("[j/k]") + " scroll  " +
				styles.HelpKey.Render("[g/G]") + " top/bottom  " +
				styles.HelpKey.Render("[:d/Esc]") + " close",
		)
	}

	if m.filterMode {
		return styles.HelpBar.Render(
			styles.Primary.Bold(true).Render("FILTER MODE") + "  " +
				styles.HelpKey.Render("[e/w/t/h/p]") + " toggle categories  " +
				styles.HelpKey.Render("[a]") + " all  " +
				styles.HelpKey.Render("[c]") + " clear  " +
				styles.HelpKey.Render("[Esc]") + " close",
		)
	}

	if m.searchMode {
		return styles.HelpBar.Render(
			styles.Primary.Bold(true).Render("SEARCH") + "  " +
				"Type pattern  " +
				styles.HelpKey.Render("[Enter]") + " confirm  " +
				styles.HelpKey.Render("[Esc]") + " cancel  " +
				styles.Muted.Render("r:pattern for regex"),
		)
	}

	keys := []string{
		styles.HelpKey.Render("[:]") + " cmd",
		styles.HelpKey.Render("[j/k]") + " scroll",
		styles.HelpKey.Render("[Tab]") + " switch",
		styles.HelpKey.Render("[i]") + " input",
		styles.HelpKey.Render("[/]") + " search",
		styles.HelpKey.Render("[?]") + " help",
		styles.HelpKey.Render("[q]") + " quit",
	}

	// Add conflict indicator when conflicts exist
	if len(m.conflicts) > 0 {
		conflictKey := styles.Warning.Bold(true).Render("[:c]") + styles.Warning.Render(" conflicts")
		keys = append([]string{conflictKey}, keys...)
	}

	// Add search status indicator if search is active
	if m.searchPattern != "" && len(m.searchMatches) > 0 {
		searchStatus := styles.Secondary.Render(fmt.Sprintf("[%d/%d]", m.searchCurrent+1, len(m.searchMatches)))
		keys = append(keys, searchStatus+" "+styles.HelpKey.Render("[n/N]")+" match")
	}

	return styles.HelpBar.Render(strings.Join(keys, "  "))
}

// Helper functions for content rendering

// truncate limits a string to max length, adding ellipsis if truncated.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// truncateLines limits text to maxLines, adding ellipsis if truncated.
func truncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n") + "\n..."
}

// formatInstanceMetrics formats metrics for a single instance.
func (m Model) formatInstanceMetrics(metrics *orchestrator.Metrics) string {
	if metrics == nil {
		return ""
	}

	parts := []string{}

	// Token usage
	if metrics.InputTokens > 0 || metrics.OutputTokens > 0 {
		parts = append(parts, fmt.Sprintf("Tokens: %s in / %s out",
			instance.FormatTokens(metrics.InputTokens),
			instance.FormatTokens(metrics.OutputTokens)))
	}

	// Cost
	if metrics.Cost > 0 {
		parts = append(parts, instance.FormatCost(metrics.Cost))
	}

	// Duration
	if duration := metrics.Duration(); duration > 0 {
		parts = append(parts, formatDuration(duration))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "  │  ")
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
