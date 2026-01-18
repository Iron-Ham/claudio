// Package view provides reusable view components for the TUI.
package view

import (
	"path/filepath"
	"strings"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/Iron-Ham/claudio/internal/util"
	"github.com/charmbracelet/lipgloss"
)

// TerminalView handles rendering of the terminal pane at the bottom of the screen.
type TerminalView struct {
	Width  int
	Height int
}

// NewTerminalView creates a new TerminalView with the given dimensions.
func NewTerminalView(width, height int) *TerminalView {
	return &TerminalView{
		Width:  width,
		Height: height,
	}
}

// TerminalState holds the state needed for rendering the terminal pane.
type TerminalState struct {
	// Output is the current terminal output
	Output string
	// IsWorktreeMode indicates whether we're in worktree mode (true) or invocation dir mode (false)
	IsWorktreeMode bool
	// CurrentDir is the current working directory of the terminal
	CurrentDir string
	// InvocationDir is the directory where Claudio was invoked
	InvocationDir string
	// TerminalMode indicates if the terminal has input focus
	TerminalMode bool
	// InstanceID is the active instance ID (for worktree mode display)
	InstanceID string
}

// Render renders the complete terminal pane.
func (v *TerminalView) Render(state TerminalState) string {
	if v.Height < 2 {
		return ""
	}

	var b strings.Builder

	// Header line with directory info and mode indicator
	header := v.renderHeader(state)
	b.WriteString(header)
	b.WriteString("\n")

	// Output area: total height minus border (2 lines) and header (1 line)
	// The border style adds 2 lines (top + bottom), and we have 1 header line
	outputHeight := v.Height - 3
	if outputHeight < 1 {
		outputHeight = 1
	}
	output := v.renderOutput(state.Output, outputHeight)
	b.WriteString(output)

	// Apply border style
	borderStyle := styles.TerminalPaneBorder
	if state.TerminalMode {
		borderStyle = styles.TerminalPaneBorderFocused
	}

	return borderStyle.
		Width(v.Width). // Full width - lipgloss Width() sets outer width including border/padding
		Height(v.Height).
		MaxHeight(v.Height).
		Render(b.String())
}

// renderHeader renders the terminal pane header line.
func (v *TerminalView) renderHeader(state TerminalState) string {
	// Mode indicator
	var modeStr string
	if state.IsWorktreeMode {
		if state.InstanceID != "" {
			modeStr = "[worktree:" + state.InstanceID + "]"
		} else {
			modeStr = "[worktree]"
		}
	} else {
		modeStr = "[invoke]"
	}

	// Shorten the directory path for display
	displayDir := state.CurrentDir
	if state.InvocationDir != "" && strings.HasPrefix(displayDir, state.InvocationDir) {
		relPath, err := filepath.Rel(state.InvocationDir, displayDir)
		if err == nil && relPath != "." {
			displayDir = "./" + relPath
		} else if relPath == "." {
			displayDir = "."
		}
	}

	// Focus indicator
	focusIndicator := ""
	if state.TerminalMode {
		focusIndicator = styles.TerminalFocusIndicator.Render(" TERMINAL ")
	}

	// Build header
	header := styles.TerminalHeader.Render(modeStr + " " + displayDir)
	if focusIndicator != "" {
		header = focusIndicator + " " + header
	}

	// Truncate if too long
	maxWidth := v.Width - 4 // Account for borders and padding
	if lipgloss.Width(header) > maxWidth {
		header = util.TruncateANSI(header, maxWidth)
	}

	return header
}

// renderOutput renders the terminal output area.
func (v *TerminalView) renderOutput(output string, height int) string {
	if output == "" {
		// Show placeholder text when terminal is empty
		placeholder := styles.Muted.Render("(shell ready)")
		return placeholder
	}

	// Trim trailing whitespace before splitting to prevent capture-pane's trailing
	// newline from creating an extra empty element. Without this, when we "take last
	// N lines", we could drop content from the beginning (like the shell prompt) while
	// keeping empty lines from the end.
	output = strings.TrimRight(output, "\r\n")

	lines := strings.Split(output, "\n")

	// Trim trailing empty lines from the end of the content
	// (e.g., empty lines before the cursor position in tmux)
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	// Take only the last 'height' lines (most recent output)
	// This is done AFTER trimming empty lines so we prioritize showing actual content
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}

	// Join and return
	return strings.Join(lines, "\n")
}
