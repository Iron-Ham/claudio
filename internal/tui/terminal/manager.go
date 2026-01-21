// Package terminal provides terminal pane management for the TUI.
package terminal

import (
	"errors"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/tmux"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// DirMode indicates which directory the terminal pane is using.
type DirMode int

const (
	// DirInvocation means the terminal is in the directory where Claudio was invoked.
	DirInvocation DirMode = iota
	// DirWorktree means the terminal is in the active instance's worktree directory.
	DirWorktree
)

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

// ActiveInstanceProvider returns the current active instance's worktree path.
// This interface decouples the terminal manager from the orchestrator.
type ActiveInstanceProvider interface {
	// WorktreePath returns the worktree path of the active instance, or empty string if none.
	WorktreePath() string
}

// Manager tracks terminal dimensions, manages the terminal process, and calculates pane layouts.
// It centralizes all terminal pane concerns including process lifecycle, directory mode,
// output capture, and key forwarding.
type Manager struct {
	// Terminal window dimensions
	width  int
	height int

	// Terminal pane state
	paneHeight int        // Height of the terminal pane in lines
	layout     LayoutMode // Current layout mode (hidden/visible)
	focused    bool       // Whether the terminal pane has input focus

	// Terminal process management
	process       *Process // Manages the terminal tmux session (nil until first toggle)
	invocationDir string   // Directory where Claudio was invoked (for terminal)
	dirMode       DirMode  // Which directory the terminal is in
	output        string   // Cached terminal output

	// Logger for error reporting
	logger *logging.Logger
}

// NewManager creates a new TerminalManager with default settings.
func NewManager() *Manager {
	return &Manager{
		paneHeight: DefaultPaneHeight,
		layout:     LayoutHidden,
		focused:    false,
		dirMode:    DirInvocation,
	}
}

// ManagerConfig contains configuration options for creating a Manager.
type ManagerConfig struct {
	InvocationDir string
	Logger        *logging.Logger
}

// NewManagerWithConfig creates a new Manager with the given configuration.
func NewManagerWithConfig(cfg ManagerConfig) *Manager {
	return &Manager{
		paneHeight:    DefaultPaneHeight,
		layout:        LayoutHidden,
		focused:       false,
		dirMode:       DirInvocation,
		invocationDir: cfg.InvocationDir,
		logger:        cfg.Logger,
	}
}

// SetInvocationDir sets the invocation directory for the terminal.
// This should be called before the first Toggle if not using NewManagerWithConfig.
func (m *Manager) SetInvocationDir(dir string) {
	m.invocationDir = dir
}

// SetLogger sets the logger for the terminal manager.
func (m *Manager) SetLogger(logger *logging.Logger) {
	m.logger = logger
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
	// Use centralized constant from styles package to stay in sync with style definitions.
	// Plus any extra footer lines for dynamic elements (error messages, conflict warnings)
	dims.MainAreaHeight = m.height - styles.HeaderFooterReserved - max(extraFooterLines, 0)

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

// -----------------------------------------------------------------------------
// Process management methods
// These methods manage the terminal's tmux process lifecycle.
// -----------------------------------------------------------------------------

// Toggle toggles the terminal pane visibility and manages the process lifecycle.
// If turning on and the process doesn't exist, it will be created lazily.
// Returns an error message for fatal errors, and a warning message for non-fatal issues.
// The sessionID is used to create a unique tmux session name.
func (m *Manager) Toggle(sessionID string) (errMsg, warnMsg string) {
	nowVisible := m.ToggleVisibility()

	if nowVisible {
		// Initialize terminal process if needed (lazy initialization)
		if m.process == nil {
			dims := m.GetPaneDimensions(0)
			m.process = NewProcess(sessionID, m.invocationDir, dims.TerminalPaneContentWidth, dims.TerminalPaneContentHeight)
		}

		// Start the process if not running
		if !m.process.IsRunning() {
			if err := m.process.Start(); err != nil {
				m.SetLayout(LayoutHidden)
				return "Failed to start terminal: " + err.Error(), ""
			}
		}

		// Set initial directory based on mode
		targetDir := m.GetDir(nil)
		if m.process.CurrentDir() != targetDir {
			if err := m.process.ChangeDirectory(targetDir); err != nil {
				if m.logger != nil {
					m.logger.Warn("failed to set initial terminal directory", "target", targetDir, "error", err)
				}
				// Terminal is open and functional, just in wrong directory - return as warning
				return "", "Terminal opened but could not change to target directory"
			}
		}
	}

	return "", ""
}

// EnterMode enters terminal input mode (keys go to terminal).
// This is a no-op if the terminal is not visible or no process is running.
func (m *Manager) EnterMode() {
	if !m.IsVisible() || m.process == nil || !m.process.IsRunning() {
		return
	}
	m.SetFocused(true)
}

// ExitMode exits terminal input mode.
func (m *Manager) ExitMode() {
	m.SetFocused(false)
}

// GetDir returns the directory path for the terminal based on current mode.
// If the mode is DirWorktree and a provider is given, it returns the active instance's worktree path.
// Falls back to invocation directory if no worktree is available.
func (m *Manager) GetDir(provider ActiveInstanceProvider) string {
	if m.dirMode == DirWorktree && provider != nil {
		if path := provider.WorktreePath(); path != "" {
			return path
		}
	}
	return m.invocationDir
}

// SwitchDir toggles between worktree and invocation directory modes.
// Returns an info message describing the result, or an error message if the directory change failed.
func (m *Manager) SwitchDir(provider ActiveInstanceProvider) (infoMsg, errMsg string) {
	if m.dirMode == DirInvocation {
		m.dirMode = DirWorktree
	} else {
		m.dirMode = DirInvocation
	}

	// Change directory if terminal is running
	if m.process != nil && m.process.IsRunning() {
		targetDir := m.GetDir(provider)
		if err := m.process.ChangeDirectory(targetDir); err != nil {
			return "", "Failed to change directory: " + err.Error()
		}
		if m.dirMode == DirWorktree {
			return "Terminal: switched to worktree", ""
		}
		return "Terminal: switched to invocation directory", ""
	}

	// Provide feedback even when terminal is not running
	if m.dirMode == DirWorktree {
		return "Terminal will use worktree when opened", ""
	}
	return "Terminal will use invocation directory when opened", ""
}

// DirMode returns the current directory mode.
func (m *Manager) DirMode() DirMode {
	return m.dirMode
}

// SetDirMode sets the directory mode.
func (m *Manager) SetDirMode(mode DirMode) {
	m.dirMode = mode
}

// UpdateOutput captures current terminal output.
// If the capture times out (e.g., tmux is unresponsive), the previous output is preserved.
func (m *Manager) UpdateOutput() {
	if m.process == nil || !m.process.IsRunning() {
		return
	}

	output, err := m.process.CaptureOutput()
	if err != nil {
		// Log at debug level for expected interruptions (timeout, cancelled, killed)
		// since they don't require user attention. Other errors are logged as warnings.
		if errors.Is(err, ErrCaptureTimeout) || errors.Is(err, ErrCaptureCancelled) || errors.Is(err, ErrCaptureKilled) {
			if m.logger != nil {
				m.logger.Debug("terminal output capture interrupted", "error", err)
			}
		} else {
			// Unexpected errors should always be logged, even without a configured logger
			if m.logger != nil {
				m.logger.Warn("failed to capture terminal output", "error", err)
			} else {
				log.Printf("WARNING: failed to capture terminal output: %v", err)
			}
		}
		// On error, preserve the previous output so the display doesn't go blank
		return
	}
	m.output = output
}

// Output returns the cached terminal output.
func (m *Manager) Output() string {
	return m.output
}

// Resize updates the terminal dimensions.
func (m *Manager) Resize() {
	if m.process == nil {
		return
	}

	// Get content dimensions from manager (accounts for borders, padding, header)
	dims := m.GetPaneDimensions(0)

	if err := m.process.Resize(dims.TerminalPaneContentWidth, dims.TerminalPaneContentHeight); err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to resize terminal", "width", dims.TerminalPaneContentWidth, "height", dims.TerminalPaneContentHeight, "error", err)
		}
	}
}

// Cleanup stops the terminal process (called on quit).
func (m *Manager) Cleanup() {
	if m.process != nil {
		if err := m.process.Stop(); err != nil {
			if m.logger != nil {
				m.logger.Warn("failed to cleanup terminal session", "error", err)
			}
		}
	}
}

// UpdateOnInstanceChange updates terminal directory if in worktree mode.
// Called when the active instance changes.
func (m *Manager) UpdateOnInstanceChange(provider ActiveInstanceProvider) string {
	if m.dirMode != DirWorktree {
		return ""
	}
	if m.process == nil || !m.process.IsRunning() {
		return ""
	}

	targetDir := m.GetDir(provider)
	if m.process.CurrentDir() != targetDir {
		if err := m.process.ChangeDirectory(targetDir); err != nil {
			return "Failed to change terminal directory: " + err.Error()
		}
	}
	return ""
}

// Process returns the underlying terminal process, or nil if not initialized.
// This is provided for cases where direct process access is needed.
func (m *Manager) Process() *Process {
	return m.process
}

// -----------------------------------------------------------------------------
// Key sending methods
// These methods forward key events to the terminal's tmux session.
// -----------------------------------------------------------------------------

// SendKey sends a key event to the terminal pane's tmux session.
// This translates tea.KeyMsg to tmux key names.
func (m *Manager) SendKey(msg tea.KeyMsg) {
	if m.process == nil || !m.process.IsRunning() {
		return
	}

	// Helper to log terminal key send errors
	logKeyErr := func(op, key string, err error) {
		if err != nil && m.logger != nil {
			m.logger.Warn("failed to send key to terminal", "op", op, "key", key, "error", err)
		}
	}

	var key string
	literal := false

	switch msg.Type {
	// Basic keys
	case tea.KeyEnter:
		key = "Enter"
	case tea.KeyBackspace:
		key = "BSpace"
	case tea.KeyTab:
		key = "Tab"
	case tea.KeyShiftTab:
		key = "BTab"
	case tea.KeySpace:
		key = " "
		literal = true
	case tea.KeyEsc:
		key = "Escape"

	// Arrow keys
	case tea.KeyUp:
		key = "Up"
	case tea.KeyDown:
		key = "Down"
	case tea.KeyRight:
		key = "Right"
	case tea.KeyLeft:
		key = "Left"

	// Navigation keys
	case tea.KeyPgUp:
		key = "PageUp"
	case tea.KeyPgDown:
		key = "PageDown"
	case tea.KeyHome:
		key = "Home"
	case tea.KeyEnd:
		key = "End"
	case tea.KeyDelete:
		key = "DC"
	case tea.KeyInsert:
		key = "IC"

	// Ctrl+letter combinations
	case tea.KeyCtrlA:
		key = "C-a"
	case tea.KeyCtrlB:
		key = "C-b"
	case tea.KeyCtrlC:
		key = "C-c"
	case tea.KeyCtrlD:
		key = "C-d"
	case tea.KeyCtrlE:
		key = "C-e"
	case tea.KeyCtrlF:
		key = "C-f"
	case tea.KeyCtrlG:
		key = "C-g"
	case tea.KeyCtrlH:
		key = "C-h"
	case tea.KeyCtrlJ:
		key = "C-j"
	case tea.KeyCtrlK:
		key = "C-k"
	case tea.KeyCtrlL:
		key = "C-l"
	case tea.KeyCtrlN:
		key = "C-n"
	case tea.KeyCtrlO:
		key = "C-o"
	case tea.KeyCtrlP:
		key = "C-p"
	case tea.KeyCtrlQ:
		key = "C-q"
	case tea.KeyCtrlR:
		key = "C-r"
	case tea.KeyCtrlS:
		key = "C-s"
	case tea.KeyCtrlT:
		key = "C-t"
	case tea.KeyCtrlU:
		key = "C-u"
	case tea.KeyCtrlV:
		key = "C-v"
	case tea.KeyCtrlW:
		key = "C-w"
	case tea.KeyCtrlX:
		key = "C-x"
	case tea.KeyCtrlY:
		key = "C-y"
	case tea.KeyCtrlZ:
		key = "C-z"

	// Function keys
	case tea.KeyF1:
		key = "F1"
	case tea.KeyF2:
		key = "F2"
	case tea.KeyF3:
		key = "F3"
	case tea.KeyF4:
		key = "F4"
	case tea.KeyF5:
		key = "F5"
	case tea.KeyF6:
		key = "F6"
	case tea.KeyF7:
		key = "F7"
	case tea.KeyF8:
		key = "F8"
	case tea.KeyF9:
		key = "F9"
	case tea.KeyF10:
		key = "F10"
	case tea.KeyF11:
		key = "F11"
	case tea.KeyF12:
		key = "F12"

	// Runes (regular characters)
	case tea.KeyRunes:
		if len(msg.Runes) > 0 {
			// Handle Alt+key combinations by sending Escape followed by the key
			if msg.Alt {
				key = string(msg.Runes)
				logKeyErr("SendKey", "Escape", m.process.SendKey("Escape"))
				logKeyErr("SendLiteral", key, m.process.SendLiteral(key))
				return
			}
			key = string(msg.Runes)
			literal = true
		}

	default:
		// Handle complex key strings (shift+, alt+, ctrl+ combinations)
		keyStr := msg.String()
		switch {
		case strings.HasPrefix(keyStr, "shift+"):
			baseKey := strings.TrimPrefix(keyStr, "shift+")
			switch baseKey {
			case "up":
				key = "S-Up"
			case "down":
				key = "S-Down"
			case "left":
				key = "S-Left"
			case "right":
				key = "S-Right"
			case "home":
				key = "S-Home"
			case "end":
				key = "S-End"
			default:
				key = keyStr
			}
		case strings.HasPrefix(keyStr, "alt+"):
			baseKey := strings.TrimPrefix(keyStr, "alt+")
			logKeyErr("SendKey", "Escape", m.process.SendKey("Escape"))
			if len(baseKey) == 1 {
				logKeyErr("SendLiteral", baseKey, m.process.SendLiteral(baseKey))
			} else {
				// Map Bubble Tea key names to tmux key names
				tmuxKey := tmux.MapKeyToTmux(baseKey)
				logKeyErr("SendKey", tmuxKey, m.process.SendKey(tmuxKey))
			}
			return
		case strings.HasPrefix(keyStr, "ctrl+"):
			baseKey := strings.TrimPrefix(keyStr, "ctrl+")
			if len(baseKey) == 1 {
				key = "C-" + baseKey
			} else {
				key = keyStr
			}
		default:
			key = keyStr
			if len(key) == 1 {
				literal = true
			}
		}
	}

	if key == "" {
		return
	}

	var err error
	if literal {
		err = m.process.SendLiteral(key)
	} else {
		err = m.process.SendKey(key)
	}
	logKeyErr("send", key, err)
}

// SendPaste sends pasted text with bracketed paste sequences.
func (m *Manager) SendPaste(text string) error {
	if m.process == nil || !m.process.IsRunning() {
		return ErrNotRunning
	}
	return m.process.SendPaste(text)
}
