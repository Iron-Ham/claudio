// Package panel provides interfaces and types for TUI panel rendering.
package panel

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DiffPanel renders a git diff with syntax highlighting and scrolling support.
type DiffPanel struct {
	height int
}

// NewDiffPanel creates a new DiffPanel.
func NewDiffPanel() *DiffPanel {
	return &DiffPanel{}
}

// Render produces the diff panel output.
func (p *DiffPanel) Render(state *RenderState) string {
	if err := state.ValidateBasic(); err != nil {
		return "[diff panel: render error]"
	}

	var b strings.Builder

	// Header with branch name if available
	title := "Diff Preview"
	if state.ActiveInstance != nil && state.ActiveInstance.Branch != "" {
		title = fmt.Sprintf("Diff Preview: %s", state.ActiveInstance.Branch)
	}
	if state.Theme != nil {
		title = state.Theme.Primary().Render(title)
	}
	b.WriteString(title)
	b.WriteString("\n")

	// Handle empty diff
	if state.DiffContent == "" {
		noChanges := "No changes to display"
		if state.Theme != nil {
			noChanges = state.Theme.Muted().Render(noChanges)
		}
		b.WriteString(noChanges)
		p.height = 3
		return b.String()
	}

	// Calculate available height for diff content
	maxLines := state.Height - 10
	if maxLines < 5 {
		maxLines = 5
	}

	// Split diff into lines
	lines := strings.Split(state.DiffContent, "\n")
	totalLines := len(lines)

	// Clamp scroll position
	maxScroll := totalLines - maxLines
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

	// Get visible lines
	startLine := scroll
	endLine := startLine + maxLines
	if endLine > totalLines {
		endLine = totalLines
	}

	visibleLines := lines[startLine:endLine]

	// Apply syntax highlighting to each visible line
	var highlighted []string
	for _, line := range visibleLines {
		highlighted = append(highlighted, p.highlightDiffLine(line, state.Theme))
	}

	// Show scroll indicator
	scrollInfo := fmt.Sprintf("Lines %d-%d of %d", startLine+1, endLine, totalLines)
	helpHint := "[j/k scroll, g/G top/bottom, d/Esc close]"
	if totalLines <= maxLines {
		helpHint = "[d/Esc close]"
	}
	if state.Theme != nil {
		scrollInfo = state.Theme.Muted().Render(scrollInfo)
		helpHint = state.Theme.Muted().Render(helpHint)
	}
	b.WriteString(scrollInfo + "  " + helpHint)
	b.WriteString("\n\n")

	// Add the diff content
	b.WriteString(strings.Join(highlighted, "\n"))

	p.height = len(visibleLines) + 4 // +4 for header and scroll info

	return b.String()
}

// highlightDiffLine applies syntax highlighting to a single diff line.
func (p *DiffPanel) highlightDiffLine(line string, theme Theme) string {
	if len(line) == 0 {
		return line
	}

	// Determine style based on line prefix
	var style lipgloss.Style
	var useDefault bool

	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		if theme != nil {
			style = theme.DiffHeader()
		} else {
			useDefault = true
		}
	case strings.HasPrefix(line, "@@"):
		if theme != nil {
			style = theme.DiffHunk()
		} else {
			useDefault = true
		}
	case strings.HasPrefix(line, "+"):
		if theme != nil {
			style = theme.DiffAdd()
		} else {
			useDefault = true
		}
	case strings.HasPrefix(line, "-"):
		if theme != nil {
			style = theme.DiffRemove()
		} else {
			useDefault = true
		}
	case strings.HasPrefix(line, "diff "):
		if theme != nil {
			style = theme.DiffHeader()
		} else {
			useDefault = true
		}
	case strings.HasPrefix(line, "index "):
		if theme != nil {
			style = theme.DiffHeader()
		} else {
			useDefault = true
		}
	default:
		if theme != nil {
			style = theme.DiffContext()
		} else {
			useDefault = true
		}
	}

	if useDefault {
		return p.defaultHighlight(line)
	}
	return style.Render(line)
}

// defaultHighlight provides basic highlighting without a theme.
func (p *DiffPanel) defaultHighlight(line string) string {
	switch {
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"),
		strings.HasPrefix(line, "diff "), strings.HasPrefix(line, "index "):
		return lipgloss.NewStyle().Bold(true).Render(line)
	case strings.HasPrefix(line, "@@"):
		return lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(line)
	case strings.HasPrefix(line, "+"):
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(line)
	case strings.HasPrefix(line, "-"):
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(line)
	default:
		return line
	}
}

// Height returns the rendered height of the panel.
func (p *DiffPanel) Height() int {
	return p.height
}
