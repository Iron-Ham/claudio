package tui

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// renderSearchBar renders the search input bar at the bottom of the screen
func (m Model) renderSearchBar() string {
	var b strings.Builder

	// Search prompt
	b.WriteString(styles.SearchPrompt.Render("/"))
	b.WriteString(styles.SearchInput.Render(m.search.Pattern()))

	if m.search.IsActive() {
		b.WriteString("█") // Cursor
	}

	// Match info
	if m.search.HasPattern() {
		if m.search.HasMatches() {
			info := fmt.Sprintf(" [%d/%d]", m.search.CurrentMatchIndex()+1, m.search.MatchCount())
			b.WriteString(styles.SearchInfo.Render(info))
			b.WriteString(styles.Muted.Render("  n/N next/prev"))
		} else if !m.search.IsActive() {
			b.WriteString(styles.SearchInfo.Render(" No matches"))
		}
		if !m.search.IsActive() {
			b.WriteString(styles.Muted.Render("  Ctrl+/ clear"))
		}
	}

	return styles.SearchBar.Render(b.String())
}

// renderFilterPanel renders the filter configuration panel
func (m Model) renderFilterPanel(width int) string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Output Filters"))
	b.WriteString("\n\n")
	b.WriteString(styles.Muted.Render("Toggle categories to show/hide specific output types:"))
	b.WriteString("\n\n")

	// Category checkboxes
	categories := []struct {
		key      string
		label    string
		shortcut string
	}{
		{"errors", "Errors", "e/1"},
		{"warnings", "Warnings", "w/2"},
		{"tools", "Tool calls", "t/3"},
		{"thinking", "Thinking", "h/4"},
		{"progress", "Progress", "p/5"},
	}

	for _, cat := range categories {
		var checkbox string
		var labelStyle lipgloss.Style
		if m.filterCategories[cat.key] {
			checkbox = styles.FilterCheckbox.Render("[✓]")
			labelStyle = styles.FilterCategoryEnabled
		} else {
			checkbox = styles.FilterCheckboxEmpty.Render("[ ]")
			labelStyle = styles.FilterCategoryDisabled
		}

		line := fmt.Sprintf("%s %s %s",
			checkbox,
			labelStyle.Render(cat.label),
			styles.Muted.Render("("+cat.shortcut+")"))
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("[a] Toggle all  [c] Clear custom filter"))
	b.WriteString("\n\n")

	// Custom filter input
	b.WriteString(styles.Secondary.Render("Custom filter:"))
	b.WriteString(" ")
	if m.filterCustom != "" {
		b.WriteString(styles.SearchInput.Render(m.filterCustom))
	} else {
		b.WriteString(styles.Muted.Render("(type to filter by pattern)"))
	}
	b.WriteString("\n\n")

	// Help text
	b.WriteString(styles.Muted.Render("Category descriptions:"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Errors: Stack traces, error messages, failures"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Warnings: Warning indicators"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Tool calls: File operations, bash commands"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Thinking: Claude's reasoning phrases"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Progress: Progress indicators, spinners"))
	b.WriteString("\n\n")

	b.WriteString(styles.Muted.Render("Press [Esc] or [F] to close"))

	return styles.ContentBox.Width(width - 4).Render(b.String())
}

// renderHelpPanel renders the help overlay
func (m Model) renderHelpPanel(width int) string {
	help := `
Claudio uses vim-style commands. Press : to enter command mode.

Navigation (always available):
  Tab / l      Next instance
  Shift+Tab/h  Previous instance
  1-9          Select instance by number
  j / ↓        Scroll down one line
  k / ↑        Scroll up one line
  Ctrl+D/U     Scroll half page down/up
  Ctrl+F/B     Scroll full page down/up
  g / G        Jump to top / bottom

Commands (press : first, then type command):
  :s :start      Start selected instance
  :x :stop       Stop instance
  :p :pause      Pause/resume instance
  :a :add        Add new instance
  :R :reconnect  Reconnect to stopped instance
  :restart       Restart stuck/timed out instance
  :D :remove     Close/remove instance
  :kill          Kill and remove instance
  :C :clear      Clear completed instances

  :d :diff       Toggle diff preview
  :m :stats      Toggle metrics panel
  :c :conflicts  Toggle conflict view
  :f :filter     Open filter panel
  :t :tmux       Show tmux attach command
  :r :pr         Show PR creation command

  :h :help       Toggle this help
  :q :quit       Quit Claudio

Input Mode:
  i / Enter      Enter input mode (interact with Claude)
  Ctrl+]         Exit input mode back to navigation

Search (vim-style):
  /              Start search (type pattern, Enter to confirm)
  n / N          Next / previous match
  Ctrl+/         Clear search
  r:pattern      Use regex (prefix with r:)

Search Tips:
  • Search is case-insensitive by default
  • Use r: prefix for regex (e.g. /r:error.*file)
  • Matches highlighted in yellow, current in orange

Filter Categories (toggle in filter panel :f):
  • Errors: Stack traces, error messages
  • Warnings: Warning indicators
  • Tool calls: File operations, bash commands
  • Thinking: Claude's reasoning
  • Progress: Progress indicators

Input Mode Details:
  All keystrokes are forwarded to Claude, including:
  • Ctrl+key combinations (Ctrl+O, Ctrl+R, Ctrl+W, etc.)
  • Function keys (F1-F12)
  • Navigation keys (Page Up/Down, Home, End)
  • Pasted text with bracketed paste support
  Press Ctrl+] to return to navigation mode.

General:
  ?              Quick toggle help
  q              Quick quit
  Auto-scroll follows new output. Scroll up to pause,
  press G to resume. "NEW OUTPUT" appears when paused.
`
	return styles.ContentBox.Width(width - 4).Render(help)
}

// renderDiffPanel renders the diff preview panel with syntax highlighting
func (m Model) renderDiffPanel(width int) string {
	var b strings.Builder

	// Header
	inst := m.activeInstance()
	if inst != nil {
		b.WriteString(styles.Title.Render(fmt.Sprintf("Diff Preview: %s", inst.Branch)))
	} else {
		b.WriteString(styles.Title.Render("Diff Preview"))
	}
	b.WriteString("\n")

	if !m.getDiffState().HasContent() {
		b.WriteString(styles.Muted.Render("No changes to display"))
		return styles.ContentBox.Width(width - 4).Render(b.String())
	}

	// Calculate available height for diff content
	maxLines := m.height - 14
	if maxLines < 5 {
		maxLines = 5
	}

	// Get visible lines (this also clamps the scroll position)
	visibleLines, startLine, endLine := m.getDiffState().GetVisibleLines(maxLines)
	totalLines := m.getDiffState().LineCount()

	// Apply syntax highlighting to each visible line
	var highlighted []string
	for _, line := range visibleLines {
		highlighted = append(highlighted, m.highlightDiffLine(line))
	}

	// Show scroll indicator
	scrollInfo := fmt.Sprintf("Lines %d-%d of %d", startLine+1, endLine, totalLines)
	if totalLines > maxLines {
		scrollInfo += "  " + styles.Muted.Render("[j/k scroll, g/G top/bottom, d/Esc close]")
	} else {
		scrollInfo += "  " + styles.Muted.Render("[d/Esc close]")
	}
	b.WriteString(styles.Muted.Render(scrollInfo))
	b.WriteString("\n\n")

	// Add the diff content
	b.WriteString(strings.Join(highlighted, "\n"))

	return styles.ContentBox.Width(width - 4).Render(b.String())
}

// highlightDiffLine applies syntax highlighting to a single diff line
func (m Model) highlightDiffLine(line string) string {
	if len(line) == 0 {
		return line
	}

	// Check line prefix for diff syntax highlighting
	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		return styles.DiffHeader.Render(line)
	case strings.HasPrefix(line, "@@"):
		return styles.DiffHunk.Render(line)
	case strings.HasPrefix(line, "+"):
		return styles.DiffAdd.Render(line)
	case strings.HasPrefix(line, "-"):
		return styles.DiffRemove.Render(line)
	case strings.HasPrefix(line, "diff "):
		return styles.DiffHeader.Render(line)
	case strings.HasPrefix(line, "index "):
		return styles.DiffHeader.Render(line)
	default:
		return styles.DiffContext.Render(line)
	}
}

// renderConflictWarning renders the file conflict warning banner
func (m Model) renderConflictWarning() string {
	if len(m.conflicts) == 0 {
		return ""
	}

	var b strings.Builder

	// Banner header with hint that it's interactive
	banner := styles.ConflictBanner.Render("⚠ FILE CONFLICT DETECTED")
	b.WriteString(banner)
	b.WriteString("  ")
	b.WriteString(styles.Muted.Render("(press [c] for details)"))
	b.WriteString("  ")

	// Build conflict details
	var conflictDetails []string
	for _, c := range m.conflicts {
		// Find instance names/numbers for the conflicting instances
		var instanceLabels []string
		for _, instID := range c.Instances {
			// Find the instance index
			for i, inst := range m.session.Instances {
				if inst.ID == instID {
					instanceLabels = append(instanceLabels, fmt.Sprintf("[%d]", i+1))
					break
				}
			}
		}
		detail := fmt.Sprintf("%s (instances %s)", c.RelativePath, strings.Join(instanceLabels, ", "))
		conflictDetails = append(conflictDetails, detail)
	}

	// Show conflict files
	if len(conflictDetails) <= 2 {
		b.WriteString(styles.Warning.Render(strings.Join(conflictDetails, "; ")))
	} else {
		// Show count and first file
		b.WriteString(styles.Warning.Render(fmt.Sprintf("%d files: %s, ...", len(conflictDetails), conflictDetails[0])))
	}

	return b.String()
}

// renderConflictPanel renders a detailed conflict view showing all files and instances
func (m Model) renderConflictPanel(width int) string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("⚠ File Conflicts"))
	b.WriteString("\n\n")

	b.WriteString(styles.Muted.Render("The following files have been modified by multiple instances:"))
	b.WriteString("\n\n")

	// Build instance ID to number mapping
	instanceNum := make(map[string]int)
	instanceTask := make(map[string]string)
	for i, inst := range m.session.Instances {
		instanceNum[inst.ID] = i + 1
		instanceTask[inst.ID] = inst.Task
	}

	// Render each conflict
	for i, c := range m.conflicts {
		// File path in warning color
		fileLine := styles.Warning.Bold(true).Render(c.RelativePath)
		b.WriteString(fileLine)
		b.WriteString("\n")

		// List the instances that modified this file
		b.WriteString(styles.Muted.Render("  Modified by:"))
		b.WriteString("\n")
		for _, instID := range c.Instances {
			num := instanceNum[instID]
			task := instanceTask[instID]
			// Truncate task if too long
			maxTaskLen := width - 15
			if maxTaskLen < 20 {
				maxTaskLen = 20
			}
			if len(task) > maxTaskLen {
				task = task[:maxTaskLen-3] + "..."
			}
			instanceLine := fmt.Sprintf("    [%d] %s", num, task)
			b.WriteString(styles.Text.Render(instanceLine))
			b.WriteString("\n")
		}

		// Add spacing between conflicts except for the last one
		if i < len(m.conflicts)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("Press [c] to close this view"))

	return styles.ContentBox.Width(width - 4).Render(b.String())
}

// renderStatsPanel renders the session statistics/metrics panel
func (m Model) renderStatsPanel(width int) string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("📊 Session Statistics"))
	b.WriteString("\n\n")

	if m.session == nil {
		b.WriteString(styles.Muted.Render("No active session"))
		return styles.ContentBox.Width(width - 4).Render(b.String())
	}

	// Get aggregated session metrics
	sessionMetrics := m.orchestrator.GetSessionMetrics()

	// Session summary
	b.WriteString(styles.Subtitle.Render("Session Summary"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Total Instances: %d (%d active)\n",
		sessionMetrics.InstanceCount, sessionMetrics.ActiveCount))
	b.WriteString(fmt.Sprintf("  Session Started: %s\n",
		m.session.Created.Format("2006-01-02 15:04:05")))
	b.WriteString("\n")

	// Token usage
	b.WriteString(styles.Subtitle.Render("Token Usage"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Input:  %s\n", instance.FormatTokens(sessionMetrics.TotalInputTokens)))
	b.WriteString(fmt.Sprintf("  Output: %s\n", instance.FormatTokens(sessionMetrics.TotalOutputTokens)))
	totalTokens := sessionMetrics.TotalInputTokens + sessionMetrics.TotalOutputTokens
	b.WriteString(fmt.Sprintf("  Total:  %s\n", instance.FormatTokens(totalTokens)))
	if sessionMetrics.TotalCacheRead > 0 || sessionMetrics.TotalCacheWrite > 0 {
		b.WriteString(fmt.Sprintf("  Cache:  %s read / %s write\n",
			instance.FormatTokens(sessionMetrics.TotalCacheRead),
			instance.FormatTokens(sessionMetrics.TotalCacheWrite)))
	}
	b.WriteString("\n")

	// Cost summary
	b.WriteString(styles.Subtitle.Render("Estimated Cost"))
	b.WriteString("\n")
	costStr := instance.FormatCost(sessionMetrics.TotalCost)
	cfg := config.Get()
	if cfg.Resources.CostWarningThreshold > 0 && sessionMetrics.TotalCost >= cfg.Resources.CostWarningThreshold {
		b.WriteString(styles.Warning.Render(fmt.Sprintf("  Total: %s (⚠ exceeds warning threshold)", costStr)))
	} else {
		b.WriteString(fmt.Sprintf("  Total: %s\n", costStr))
	}
	if cfg.Resources.CostLimit > 0 {
		b.WriteString(fmt.Sprintf("  Limit: %s\n", instance.FormatCost(cfg.Resources.CostLimit)))
	}
	b.WriteString("\n")

	// Per-instance breakdown
	b.WriteString(styles.Subtitle.Render("Top Instances by Cost"))
	b.WriteString("\n")

	// Sort instances by cost (simple bubble for small lists)
	type instCost struct {
		id   string
		num  int
		task string
		cost float64
	}
	var costList []instCost
	for i, inst := range m.session.Instances {
		cost := 0.0
		if inst.Metrics != nil {
			cost = inst.Metrics.Cost
		}
		costList = append(costList, instCost{
			id:   inst.ID,
			num:  i + 1,
			task: inst.Task,
			cost: cost,
		})
	}
	// Sort descending by cost
	for i := 0; i < len(costList)-1; i++ {
		for j := i + 1; j < len(costList); j++ {
			if costList[j].cost > costList[i].cost {
				costList[i], costList[j] = costList[j], costList[i]
			}
		}
	}

	// Show top 5
	shown := 0
	for _, ic := range costList {
		if shown >= 5 {
			break
		}
		if ic.cost > 0 {
			taskTrunc := truncate(ic.task, width-25)
			b.WriteString(fmt.Sprintf("  %d. [%d] %s: %s\n",
				shown+1, ic.num, taskTrunc, instance.FormatCost(ic.cost)))
			shown++
		}
	}
	if shown == 0 {
		b.WriteString(styles.Muted.Render("  No cost data available yet"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("Press [m] to close this view"))

	return styles.ContentBox.Width(width - 4).Render(b.String())
}
