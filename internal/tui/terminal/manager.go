// Package terminal provides terminal pane management for the TUI.
package terminal

// LayoutMode represents the terminal pane's visibility state.
type LayoutMode int

const (
	// LayoutHidden means the terminal pane is not visible.
	LayoutHidden LayoutMode = iota
	// LayoutVisible means the terminal pane is visible at the bottom of the screen.
	LayoutVisible
)

// Layout constants for pane dimension calculations.
const (
	// DefaultPaneHeight is the default height of the terminal pane in lines.
	DefaultPaneHeight = 15

	// MinPaneHeight is the minimum height of the terminal pane.
	MinPaneHeight = 5

	// MaxPaneHeightRatio is the maximum ratio of terminal height to total height.
	MaxPaneHeightRatio = 0.5

	// TerminalPaneSpacing is the vertical spacing between main content and terminal pane.
	TerminalPaneSpacing = 1
)

// PaneDimensions contains the calculated dimensions for all UI panes.
type PaneDimensions struct {
	// TerminalWidth is the full width of the terminal window.
	TerminalWidth int
	// TerminalHeight is the full height of the terminal window.
	TerminalHeight int

	// MainAreaHeight is the height available for the main content area
	// (sidebar + content), accounting for header, footer, and terminal pane.
	MainAreaHeight int

	// TerminalPaneHeight is the height of the terminal pane (0 if hidden).
	TerminalPaneHeight int

	// TerminalPaneContentHeight is the usable content height inside the terminal pane
	// (accounting for borders and header).
	TerminalPaneContentHeight int

	// TerminalPaneContentWidth is the usable content width inside the terminal pane
	// (accounting for borders and padding).
	TerminalPaneContentWidth int
}

// Manager tracks terminal dimensions and calculates pane layouts.
// It centralizes all terminal sizing and pane calculation logic.
type Manager struct {
	// Terminal window dimensions
	width  int
	height int

	// Terminal pane state
	paneHeight int        // Height of the terminal pane in lines
	layout     LayoutMode // Current layout mode (hidden/visible)
	focused    bool       // Whether the terminal pane has input focus
}

// NewManager creates a new TerminalManager with default settings.
func NewManager() *Manager {
	return &Manager{
		paneHeight: DefaultPaneHeight,
		layout:     LayoutHidden,
		focused:    false,
	}
}

// SetSize updates the terminal window dimensions.
// This should be called when the terminal is resized.
func (m *Manager) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Width returns the current terminal width.
func (m *Manager) Width() int {
	return m.width
}

// Height returns the current terminal height.
func (m *Manager) Height() int {
	return m.height
}

// GetPaneDimensions calculates and returns the dimensions for all UI panes
// based on the current terminal size and layout mode. The extraFooterLines
// parameter specifies additional lines to reserve for dynamic footer elements
// such as error messages, info messages, and conflict warnings.
func (m *Manager) GetPaneDimensions(extraFooterLines int) PaneDimensions {
	dims := PaneDimensions{
		TerminalWidth:  m.width,
		TerminalHeight: m.height,
	}

	// Calculate terminal pane height (0 if hidden)
	if m.layout == LayoutVisible {
		dims.TerminalPaneHeight = m.effectivePaneHeight()

		// Content dimensions account for border (2 lines: top/bottom) and header (1 line)
		dims.TerminalPaneContentHeight = max(dims.TerminalPaneHeight-3, 3)

		// Width accounts for border (2 chars) and padding (2 chars)
		dims.TerminalPaneContentWidth = max(m.width-4, 20)
	}

	// Calculate main area height
	// Base height minus header (2 lines), help bar (2 lines), and margins (2 lines) = 6
	// Plus any extra footer lines for dynamic elements (error messages, conflict warnings)
	const headerFooterReserved = 6
	dims.MainAreaHeight = m.height - headerFooterReserved - max(extraFooterLines, 0)

	// Reduce main area when terminal pane is visible
	if m.layout == LayoutVisible && dims.TerminalPaneHeight > 0 {
		dims.MainAreaHeight -= dims.TerminalPaneHeight + TerminalPaneSpacing
	}

	// Enforce minimum main area height
	const minMainAreaHeight = 10
	if dims.MainAreaHeight < minMainAreaHeight {
		dims.MainAreaHeight = minMainAreaHeight
	}

	return dims
}

// ToggleFocus toggles input focus between the terminal pane and main content.
// Returns true if the terminal pane now has focus, false otherwise.
func (m *Manager) ToggleFocus() bool {
	// Can only focus terminal pane if it's visible
	if m.layout != LayoutVisible {
		m.focused = false
		return false
	}
	m.focused = !m.focused
	return m.focused
}

// SetFocused explicitly sets the focus state of the terminal pane.
func (m *Manager) SetFocused(focused bool) {
	// Can only focus if visible
	if m.layout != LayoutVisible {
		m.focused = false
		return
	}
	m.focused = focused
}

// IsFocused returns true if the terminal pane has input focus.
func (m *Manager) IsFocused() bool {
	return m.focused && m.layout == LayoutVisible
}

// SetLayout sets the terminal pane layout mode.
func (m *Manager) SetLayout(layout LayoutMode) {
	m.layout = layout
	// Clear focus when hiding terminal pane
	if layout == LayoutHidden {
		m.focused = false
	}
}

// Layout returns the current layout mode.
func (m *Manager) Layout() LayoutMode {
	return m.layout
}

// IsVisible returns true if the terminal pane is visible.
func (m *Manager) IsVisible() bool {
	return m.layout == LayoutVisible
}

// ToggleVisibility toggles the terminal pane between visible and hidden.
// Returns true if the terminal pane is now visible, false otherwise.
func (m *Manager) ToggleVisibility() bool {
	if m.layout == LayoutVisible {
		m.layout = LayoutHidden
		m.focused = false
	} else {
		m.layout = LayoutVisible
	}
	return m.layout == LayoutVisible
}

// SetPaneHeight sets the terminal pane height.
// The height is clamped to valid bounds based on the current terminal size.
func (m *Manager) SetPaneHeight(height int) {
	m.paneHeight = height
}

// PaneHeight returns the configured terminal pane height.
// Note: Use GetPaneDimensions().TerminalPaneHeight for the effective height
// which accounts for visibility and clamping.
func (m *Manager) PaneHeight() int {
	return m.paneHeight
}

// effectivePaneHeight returns the actual pane height to use,
// applying defaults and clamping to valid bounds.
func (m *Manager) effectivePaneHeight() int {
	height := m.paneHeight
	if height == 0 {
		height = DefaultPaneHeight
	}

	// Clamp to minimum
	height = max(height, MinPaneHeight)

	// Clamp to maximum (based on terminal height)
	maxHeight := max(int(float64(m.height)*MaxPaneHeightRatio), MinPaneHeight)
	height = min(height, maxHeight)

	return height
}

// ResizePaneHeight adjusts the terminal pane height by delta lines.
// Positive delta increases height, negative decreases it.
func (m *Manager) ResizePaneHeight(delta int) {
	m.paneHeight = max(m.paneHeight+delta, MinPaneHeight)
}
