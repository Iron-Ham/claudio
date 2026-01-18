package view

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/spf13/viper"
)

// HelpBarState holds the state needed to render the help bar.
// This separates the render-time state from the Model struct.
type HelpBarState struct {
	// CommandMode indicates whether command mode is active
	CommandMode bool

	// CommandBuffer is the current command input
	CommandBuffer string

	// InputMode indicates whether input forwarding mode is active
	InputMode bool

	// TerminalFocused indicates whether the terminal pane has focus
	TerminalFocused bool

	// TerminalVisible indicates whether the terminal pane is visible
	TerminalVisible bool

	// TerminalDirMode is the current terminal directory mode ("invoke" or "worktree")
	TerminalDirMode string

	// ShowDiff indicates whether the diff panel is visible
	ShowDiff bool

	// FilterMode indicates whether filter mode is active
	FilterMode bool

	// SearchMode indicates whether search mode is active
	SearchMode bool

	// ConflictCount is the number of file conflicts detected
	ConflictCount int

	// SearchHasMatches indicates whether the search has matches
	SearchHasMatches bool

	// SearchCurrentIndex is the current search match index (0-based)
	SearchCurrentIndex int

	// SearchMatchCount is the total number of search matches
	SearchMatchCount int
}

// HelpBarView handles rendering of help bars for different modes.
type HelpBarView struct{}

// NewHelpBarView creates a new HelpBarView instance.
func NewHelpBarView() *HelpBarView {
	return &HelpBarView{}
}

// RenderCommandModeHelp renders the help bar when in command mode.
// This is separate so it can take priority in all modes (normal, ultra-plan, plan editor).
func (v *HelpBarView) RenderCommandModeHelp(state *HelpBarState) string {
	if state == nil {
		return ""
	}

	if viper.GetBool("tui.verbose_command_help") {
		return v.renderVerboseCommandHelp(state)
	}
	return v.renderCompactCommandHelp(state)
}

// renderCompactCommandHelp renders the compact single-line command help (for experts).
func (v *HelpBarView) renderCompactCommandHelp(state *HelpBarState) string {
	badge := styles.ModeBadgeCommand.Render("COMMAND")
	return styles.HelpBar.Render(
		badge + "  " +
			styles.Primary.Bold(true).Render(":") + styles.Primary.Render(state.CommandBuffer) +
			styles.Muted.Render("█") + "  " +
			styles.HelpKey.Render("[Enter]") + " execute  " +
			styles.HelpKey.Render("[Esc]") + " cancel  " +
			styles.Muted.Render("(:help for commands)"),
	)
}

// renderVerboseCommandHelp renders a multi-line command help panel with descriptions.
// Shows only the most commonly used commands, with a hint to use :help for more.
func (v *HelpBarView) renderVerboseCommandHelp(state *HelpBarState) string {
	var lines []string

	// Command input line with mode badge
	badge := styles.ModeBadgeCommand.Render("COMMAND")
	inputLine := badge + "  " +
		styles.Primary.Bold(true).Render(":") + styles.Primary.Render(state.CommandBuffer) +
		styles.Muted.Render("█") + "  " +
		styles.HelpKey.Render("[Enter]") + " execute  " +
		styles.HelpKey.Render("[Esc]") + " cancel"
	lines = append(lines, inputLine)

	// Show prioritized commands grouped by function
	// Line 1: Instance control (most common operations)
	line1 := styles.Secondary.Bold(true).Render("Control:") + " " +
		styles.HelpKey.Render("s/start") + " " + styles.Muted.Render("start") + "  " +
		styles.HelpKey.Render("x/stop") + " " + styles.Muted.Render("stop+PR") + "  " +
		styles.HelpKey.Render("p/pause") + " " + styles.Muted.Render("pause/resume") + "  " +
		styles.HelpKey.Render("a/add") + " " + styles.Muted.Render("new instance")
	lines = append(lines, line1)

	// Line 2: Views and navigation
	line2 := styles.Secondary.Bold(true).Render("View:") + " " +
		styles.HelpKey.Render("d/diff") + " " + styles.Muted.Render("changes") + "  " +
		styles.HelpKey.Render("m/stats") + " " + styles.Muted.Render("metrics") + "  " +
		styles.HelpKey.Render("t/term") + " " + styles.Muted.Render("terminal") + "  " +
		styles.HelpKey.Render("h/help") + " " + styles.Muted.Render("full help") + "  " +
		styles.HelpKey.Render("q/quit") + " " + styles.Muted.Render("exit")
	lines = append(lines, line2)

	return styles.HelpBar.Render(strings.Join(lines, "\n"))
}

// RenderHelp renders the main help bar based on current state.
// Always includes a mode badge at the start for clear mode visibility.
func (v *HelpBarView) RenderHelp(state *HelpBarState) string {
	if state == nil {
		return ""
	}

	if state.InputMode {
		badge := styles.ModeBadgeInput.Render("INPUT")
		help := styles.HelpKey.Render("[Ctrl+]]") + " exit  " +
			styles.Muted.Render("All keystrokes forwarded to Claude")
		return styles.HelpBar.Render(badge + "  " + help)
	}

	if state.TerminalFocused {
		badge := styles.ModeBadgeTerminal.Render("TERMINAL")
		dirMode := "invoke"
		if state.TerminalDirMode == "worktree" {
			dirMode = "worktree"
		}
		help := styles.HelpKey.Render("[Ctrl+]]") + " exit  " +
			styles.HelpKey.Render("[Ctrl+Shift+T]") + " switch dir  " +
			styles.Muted.Render("("+dirMode+")")
		return styles.HelpBar.Render(badge + "  " + help)
	}

	if state.ShowDiff {
		badge := styles.ModeBadgeDiff.Render("DIFF")
		help := styles.HelpKey.Render("[j/k]") + " scroll  " +
			styles.HelpKey.Render("[0/G]") + " top/bottom  " +
			styles.HelpKey.Render("[:d/Esc]") + " close"
		return styles.HelpBar.Render(badge + "  " + help)
	}

	if state.FilterMode {
		badge := styles.ModeBadgeFilter.Render("FILTER")
		help := styles.HelpKey.Render("[e/w/t/h/p]") + " toggle  " +
			styles.HelpKey.Render("[a]") + " all  " +
			styles.HelpKey.Render("[c]") + " clear  " +
			styles.HelpKey.Render("[Esc]") + " close"
		return styles.HelpBar.Render(badge + "  " + help)
	}

	if state.SearchMode {
		badge := styles.ModeBadgeSearch.Render("SEARCH")
		help := "Type pattern  " +
			styles.HelpKey.Render("[Enter]") + " confirm  " +
			styles.HelpKey.Render("[Esc]") + " cancel  " +
			styles.Muted.Render("r:pattern for regex")
		return styles.HelpBar.Render(badge + "  " + help)
	}

	// Normal mode - show NORMAL badge
	badge := styles.ModeBadgeNormal.Render("NORMAL")

	keys := []string{
		styles.HelpKey.Render("[:]") + " cmd",
		styles.HelpKey.Render("[j/k]") + " scroll",
		styles.HelpKey.Render("[Tab]") + " switch",
		styles.HelpKey.Render("[i]") + " input",
		styles.HelpKey.Render("[/]") + " search",
		styles.HelpKey.Render("[?]") + " help",
		styles.HelpKey.Render("[:q]") + " quit",
	}

	// Add terminal key based on visibility
	if state.TerminalVisible {
		keys = append(keys, styles.HelpKey.Render("[:t]")+" term "+styles.HelpKey.Render("[`]")+" hide")
	} else {
		keys = append(keys, styles.HelpKey.Render("[`]")+" term")
	}

	// Add conflict indicator when conflicts exist
	if state.ConflictCount > 0 {
		conflictKey := styles.Warning.Bold(true).Render("[:c]") + styles.Warning.Render(" conflicts")
		keys = append([]string{conflictKey}, keys...)
	}

	// Add search status indicator if search is active
	if state.SearchHasMatches {
		searchStatus := styles.Secondary.Render(fmt.Sprintf("[%d/%d]", state.SearchCurrentIndex+1, state.SearchMatchCount))
		keys = append(keys, searchStatus+" "+styles.HelpKey.Render("[n/N]")+" match")
	}

	return styles.HelpBar.Render(badge + "  " + strings.Join(keys, "  "))
}

// RenderTripleShotHelp renders the help bar for triple-shot mode.
// Accepts optional state to properly display INPUT mode when active.
func (v *HelpBarView) RenderTripleShotHelp(state *HelpBarState) string {
	// Check for input mode first - this takes priority
	if state != nil && state.InputMode {
		badge := styles.ModeBadgeInput.Render("INPUT")
		help := styles.HelpKey.Render("[Ctrl+]]") + " exit  " +
			styles.Muted.Render("All keystrokes forwarded to Claude")
		return styles.HelpBar.Render(badge + "  " + help)
	}

	// Check for terminal focused mode
	if state != nil && state.TerminalFocused {
		badge := styles.ModeBadgeTerminal.Render("TERMINAL")
		dirMode := "invoke"
		if state.TerminalDirMode == "worktree" {
			dirMode = "worktree"
		}
		help := styles.HelpKey.Render("[Ctrl+]]") + " exit  " +
			styles.HelpKey.Render("[Ctrl+Shift+T]") + " switch dir  " +
			styles.Muted.Render("("+dirMode+")")
		return styles.HelpBar.Render(badge + "  " + help)
	}

	// Normal triple-shot mode
	badge := styles.ModeBadgeNormal.Render("NORMAL")
	keys := []string{
		styles.HelpKey.Render("[:]") + " cmd",
		styles.HelpKey.Render("[j/k]") + " scroll",
		styles.HelpKey.Render("[Tab]") + " switch",
		styles.HelpKey.Render("[/]") + " search",
		styles.HelpKey.Render("[?]") + " help",
		styles.HelpKey.Render("[:q]") + " quit",
	}
	return styles.HelpBar.Render(badge + "  " + strings.Join(keys, "  "))
}

// Package-level convenience functions for backward compatibility and simpler usage

// helpBarView is the shared instance for package-level functions.
var helpBarView = NewHelpBarView()

// RenderCommandModeHelp renders the help bar when in command mode.
func RenderCommandModeHelp(state *HelpBarState) string {
	return helpBarView.RenderCommandModeHelp(state)
}

// RenderHelp renders the main help bar based on current state.
func RenderHelp(state *HelpBarState) string {
	return helpBarView.RenderHelp(state)
}

// RenderTripleShotHelp renders the help bar for triple-shot mode.
func RenderTripleShotHelp(state *HelpBarState) string {
	return helpBarView.RenderTripleShotHelp(state)
}
