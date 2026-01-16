package view

import (
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// ModeIndicatorState holds the state needed to render the mode indicator.
type ModeIndicatorState struct {
	// CommandMode indicates command mode is active (typing after ':')
	CommandMode bool

	// SearchMode indicates search mode is active
	SearchMode bool

	// FilterMode indicates filter mode is active
	FilterMode bool

	// InputMode indicates input forwarding mode is active
	InputMode bool

	// TerminalFocused indicates the terminal pane has focus
	TerminalFocused bool

	// AddingTask indicates task input mode is active
	AddingTask bool
}

// ModeInfo contains display information for a mode.
type ModeInfo struct {
	// Label is the text shown in the indicator
	Label string

	// Style is the lipgloss style to apply
	Style lipgloss.Style

	// IsHighPriority indicates this mode should be prominently displayed
	// because it changes how keyboard input is processed
	IsHighPriority bool
}

// ModeIndicatorView renders the mode indicator component.
type ModeIndicatorView struct{}

// NewModeIndicatorView creates a new mode indicator view.
func NewModeIndicatorView() *ModeIndicatorView {
	return &ModeIndicatorView{}
}

// GetModeInfo returns display information for the current mode.
// Returns nil if in normal mode (no indicator needed).
func (v *ModeIndicatorView) GetModeInfo(state *ModeIndicatorState) *ModeInfo {
	if state == nil {
		return nil
	}

	// Priority order matches the keyhandler's mode processing priority
	// High-priority modes that change keyboard behavior come first

	if state.InputMode {
		return &ModeInfo{
			Label: "INPUT",
			Style: lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.TextColor).
				Background(styles.WarningColor).
				Padding(0, 1),
			IsHighPriority: true,
		}
	}

	if state.TerminalFocused {
		return &ModeInfo{
			Label: "TERMINAL",
			Style: lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.TextColor).
				Background(styles.SecondaryColor).
				Padding(0, 1),
			IsHighPriority: true,
		}
	}

	if state.SearchMode {
		return &ModeInfo{
			Label: "SEARCH",
			Style: lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.TextColor).
				Background(styles.PrimaryColor).
				Padding(0, 1),
			IsHighPriority: false,
		}
	}

	if state.FilterMode {
		return &ModeInfo{
			Label: "FILTER",
			Style: lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.TextColor).
				Background(styles.PrimaryColor).
				Padding(0, 1),
			IsHighPriority: false,
		}
	}

	if state.CommandMode {
		return &ModeInfo{
			Label: "COMMAND",
			Style: lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.TextColor).
				Background(styles.PrimaryColor).
				Padding(0, 1),
			IsHighPriority: false,
		}
	}

	if state.AddingTask {
		return &ModeInfo{
			Label: "NEW TASK",
			Style: lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.TextColor).
				Background(styles.BlueColor).
				Padding(0, 1),
			IsHighPriority: false,
		}
	}

	// Normal mode - no indicator needed
	return nil
}

// Render returns the rendered mode indicator string.
// Returns empty string if in normal mode.
func (v *ModeIndicatorView) Render(state *ModeIndicatorState) string {
	info := v.GetModeInfo(state)
	if info == nil {
		return ""
	}

	return info.Style.Render(info.Label)
}

// RenderWithExitHint returns the mode indicator with an exit hint.
// For high-priority modes, includes the key to exit.
func (v *ModeIndicatorView) RenderWithExitHint(state *ModeIndicatorState) string {
	info := v.GetModeInfo(state)
	if info == nil {
		return ""
	}

	indicator := info.Style.Render(info.Label)

	// Add exit hint for high-priority modes
	if info.IsHighPriority {
		hint := styles.Muted.Render(" Ctrl+] to exit")
		return indicator + hint
	}

	return indicator
}

// Package-level convenience function
var modeIndicatorView = NewModeIndicatorView()

// RenderModeIndicator renders the mode indicator for the given state.
func RenderModeIndicator(state *ModeIndicatorState) string {
	return modeIndicatorView.Render(state)
}

// RenderModeIndicatorWithHint renders the mode indicator with exit hint.
func RenderModeIndicatorWithHint(state *ModeIndicatorState) string {
	return modeIndicatorView.RenderWithExitHint(state)
}

// GetCurrentModeInfo returns the mode info for the given state.
func GetCurrentModeInfo(state *ModeIndicatorState) *ModeInfo {
	return modeIndicatorView.GetModeInfo(state)
}
