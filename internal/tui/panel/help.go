// Package panel provides interfaces and types for TUI panel rendering.
package panel

import (
	"fmt"
	"strings"
)

// HelpPanel renders the help overlay with keybindings and scrolling support.
type HelpPanel struct {
	height int
}

// NewHelpPanel creates a new HelpPanel.
func NewHelpPanel() *HelpPanel {
	return &HelpPanel{}
}

// Render produces the help panel output.
func (p *HelpPanel) Render(state *RenderState) string {
	if err := state.ValidateBasic(); err != nil {
		return "[help panel: render error]"
	}

	// Use provided sections or fall back to defaults
	sections := state.HelpSections
	if len(sections) == 0 {
		sections = DefaultHelpSections()
	}

	var lines []string

	// Title
	title := "Claudio Help"
	subtitle := "Press : to enter command mode. Use j/k to scroll, ? or :h to close."
	if state.Theme != nil {
		title = state.Theme.Primary().Render(title)
		subtitle = state.Theme.Muted().Render(subtitle)
	}
	lines = append(lines, title)
	lines = append(lines, subtitle)
	lines = append(lines, "")

	// Build sections
	for _, section := range sections {
		sectionTitle := "▸ " + section.Title
		if state.Theme != nil {
			sectionTitle = state.Theme.Primary().Bold(true).Render(sectionTitle)
		}
		lines = append(lines, sectionTitle)

		for _, item := range section.Items {
			keyStr := item.Key
			descStr := item.Description
			if state.Theme != nil {
				keyStr = state.Theme.Secondary().Render(keyStr)
				descStr = state.Theme.Muted().Render(descStr)
			}
			lines = append(lines, fmt.Sprintf("    %s  %s", keyStr, descStr))
		}
		lines = append(lines, "")
		lines = append(lines, "")
	}

	// Calculate visible lines based on available height
	maxLines := state.Height - 6 // Leave room for borders and scroll indicator
	if maxLines < 10 {
		maxLines = 10
	}

	// Clamp scroll to valid range
	maxScroll := len(lines) - maxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := state.ScrollOffset
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	// Slice visible lines
	endLine := scroll + maxLines
	if endLine > len(lines) {
		endLine = len(lines)
	}
	visibleLines := lines[scroll:endLine]

	// Build content
	var content string
	if maxScroll > 0 {
		// Add scroll indicator
		scrollInfo := fmt.Sprintf(" [%d/%d] ", scroll+1, maxScroll+1)
		if state.Theme != nil {
			scrollInfo = state.Theme.Muted().Render(scrollInfo)
		}
		if scroll > 0 {
			upArrow := "▲ "
			if state.Theme != nil {
				upArrow = state.Theme.Warning().Render(upArrow)
			}
			scrollInfo = upArrow + scrollInfo
		}
		if scroll < maxScroll {
			downArrow := " ▼"
			if state.Theme != nil {
				downArrow = state.Theme.Warning().Render(downArrow)
			}
			scrollInfo = scrollInfo + downArrow
		}
		content = strings.Join(visibleLines, "\n") + "\n" + scrollInfo
	} else {
		content = strings.Join(visibleLines, "\n")
	}

	p.height = len(visibleLines) + 2 // +2 for scroll indicator line

	return content
}

// Height returns the rendered height of the panel.
func (p *HelpPanel) Height() int {
	return p.height
}

// DefaultHelpSections returns the default Claudio help sections.
func DefaultHelpSections() []HelpSection {
	return []HelpSection{
		{
			Title: "Navigation",
			Items: []HelpItem{
				{Key: "Tab/l  Shift+Tab/h", Description: "Next / Previous instance"},
				{Key: "j/↓  k/↑", Description: "Scroll down / up one line"},
				{Key: "Ctrl+D/U  Ctrl+F/B", Description: "Scroll half / full page"},
				{Key: "gg  G", Description: "Jump to top / bottom"},
			},
		},
		{
			Title: "Group Commands (g prefix)",
			Items: []HelpItem{
				{Key: "gc", Description: "Collapse/expand current group"},
				{Key: "gC", Description: "Collapse/expand all groups"},
				{Key: "gn", Description: "Jump to next group"},
				{Key: "gp", Description: "Jump to previous group"},
				{Key: "gs", Description: "Skip current group (mark pending as skipped)"},
				{Key: "gr", Description: "Retry failed tasks in current group"},
				{Key: "gf", Description: "Force-start next group (ignore dependencies)"},
				{Key: "gq", Description: "Dismiss all instances in current group"},
			},
		},
		{
			Title: "Instance Control",
			Items: []HelpItem{
				{Key: ":s  :start", Description: "Start a stopped/new instance"},
				{Key: ":x  :stop", Description: "Stop instance + auto-PR workflow"},
				{Key: ":e  :exit", Description: "Stop instance (no auto-PR)"},
				{Key: ":p  :pause", Description: "Pause/resume instance"},
				{Key: ":R  :reconnect", Description: "Reattach to stopped tmux session"},
				{Key: ":restart", Description: "Restart stuck/timeout instance"},
			},
		},
		{
			Title: "Instance Management",
			Items: []HelpItem{
				{Key: ":a  :add", Description: "Create and add new instance"},
				{Key: ":chain [N]  :dep  :depends", Description: "Add dependent task (N = sidebar #, or #N)"},
				{Key: ":D  :remove", Description: "Remove instance (keeps branch)"},
				{Key: ":kill", Description: "Force kill and remove instance"},
				{Key: ":C  :clear", Description: "Remove all completed instances"},
			},
		},
		{
			Title: "Triple-Shot Mode",
			Items: []HelpItem{
				{Key: ":tripleshot", Description: "Run 3 parallel attempts + judge"},
				{Key: ":accept", Description: "Accept winning triple-shot solution"},
			},
		},
		{
			Title: "Adversarial Mode",
			Items: []HelpItem{
				{Key: ":adversarial  :adv", Description: "Start implementer + reviewer feedback loop"},
			},
		},
		{
			Title: "Planning Modes (experimental)",
			Items: []HelpItem{
				{Key: ":plan", Description: "Start inline plan mode"},
				{Key: ":multiplan  :mp", Description: "Start multi-pass plan (3 planners + assessor)"},
				{Key: ":ultraplan  :up", Description: "Start ultraplan mode"},
				{Key: ":up --multi-pass", Description: "Ultraplan with multi-pass planning (3 strategies)"},
				{Key: ":up --plan <file>", Description: "Load ultraplan from existing plan file"},
				{Key: ":cancel", Description: "Cancel ultraplan execution"},
			},
		},
		{
			Title: "Group Management",
			Items: []HelpItem{
				{Key: ":group create", Description: "Create a new empty group"},
				{Key: ":group add", Description: "Add instance to a group"},
				{Key: ":group remove", Description: "Remove instance from its group"},
				{Key: ":group move", Description: "Move instance to a different group"},
				{Key: ":group order", Description: "Reorder group execution sequence"},
				{Key: ":group delete", Description: "Delete an empty group"},
				{Key: ":group show", Description: "Toggle grouped instance view"},
			},
		},
		{
			Title: "View Commands",
			Items: []HelpItem{
				{Key: ":d  :diff", Description: "Toggle diff preview panel"},
				{Key: ":m  :stats", Description: "Toggle metrics panel"},
				{Key: ":c  :conflicts", Description: "Toggle conflict view"},
				{Key: ":f  :filter", Description: "Open filter panel"},
				{Key: ":tmux", Description: "Show tmux attach command"},
				{Key: ":r  :pr", Description: "Show PR creation command"},
				{Key: ":pr --group", Description: "Create stacked PRs for all groups"},
				{Key: ":pr --group=all", Description: "Create consolidated PR from all groups"},
				{Key: ":pr --group=single", Description: "Create PR for current group only"},
			},
		},
		{
			Title: "Terminal Pane",
			Items: []HelpItem{
				{Key: "`  :term", Description: "Toggle terminal pane"},
				{Key: ":t", Description: "Focus terminal for typing"},
				{Key: "Ctrl+]", Description: "Exit terminal mode"},
				{Key: "Ctrl+Shift+T", Description: "Switch terminal directory"},
			},
		},
		{
			Title: "Input Mode",
			Items: []HelpItem{
				{Key: "i  Enter", Description: "Enter input mode (talk to Claude)"},
				{Key: "Ctrl+]", Description: "Exit input mode"},
			},
		},
		{
			Title: "Search",
			Items: []HelpItem{
				{Key: "/", Description: "Start search"},
				{Key: "n  N", Description: "Next / previous match"},
				{Key: "Ctrl+/", Description: "Clear search"},
				{Key: "r:pattern", Description: "Use regex search"},
			},
		},
		{
			Title: "Session",
			Items: []HelpItem{
				{Key: ":h  :help", Description: "Toggle this help panel"},
				{Key: ":q  :quit", Description: "Quit (instances continue in tmux)"},
				{Key: ":q!  :quit!", Description: "Force quit: stop all, cleanup worktrees, exit"},
				{Key: "?", Description: "Quick toggle help"},
			},
		},
	}
}
