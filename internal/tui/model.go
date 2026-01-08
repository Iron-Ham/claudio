package tui

import (
	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// Model holds the TUI application state
type Model struct {
	// Core components
	orchestrator *orchestrator.Orchestrator
	session      *orchestrator.Session

	// UI state
	activeTab      int
	width          int
	height         int
	ready          bool
	quitting       bool
	showHelp       bool
	showConflicts  bool // When true, show detailed conflict view
	addingTask     bool
	taskInput      string
	errorMessage   string
	infoMessage    string // Non-error status message
	warningMessage string // Warning message (e.g., file conflicts)
	inputMode      bool   // When true, all keys are forwarded to the active instance's tmux session

	// Template dropdown state
	showTemplates    bool   // Whether the template dropdown is visible
	templateFilter   string // Current filter text (after the "/")
	templateSelected int    // Currently highlighted template index

	// File conflict tracking
	conflicts []conflict.FileConflict

	// Instance outputs (instance ID -> output string)
	outputs map[string]string

	// Diff preview state
	showDiff    bool   // Whether the diff panel is visible
	diffContent string // Cached diff content for the active instance
	diffScroll  int    // Scroll offset for navigating the diff

	// Sidebar pagination
	sidebarScrollOffset int // Index of the first visible instance in sidebar
}

// NewModel creates a new TUI model
func NewModel(orch *orchestrator.Orchestrator, session *orchestrator.Session) Model {
	return Model{
		orchestrator: orch,
		session:      session,
		outputs:      make(map[string]string),
	}
}

// activeInstance returns the currently focused instance
func (m Model) activeInstance() *orchestrator.Instance {
	if m.session == nil || len(m.session.Instances) == 0 {
		return nil
	}

	if m.activeTab >= len(m.session.Instances) {
		return nil
	}

	return m.session.Instances[m.activeTab]
}

// instanceCount returns the number of instances
func (m Model) instanceCount() int {
	if m.session == nil {
		return 0
	}
	return len(m.session.Instances)
}

// ensureActiveVisible adjusts sidebarScrollOffset to keep activeTab visible
func (m *Model) ensureActiveVisible() {
	// Calculate visible slots (same calculation as in renderSidebar)
	// Reserve: 1 for title, 1 for blank line, 1 for add hint, 2 for scroll indicators, plus border padding
	reservedLines := 6
	mainAreaHeight := m.height - 6 // Same as in View()
	availableSlots := mainAreaHeight - reservedLines
	if availableSlots < 3 {
		availableSlots = 3
	}

	// Adjust scroll offset to keep active instance visible
	if m.activeTab < m.sidebarScrollOffset {
		// Active is above visible area, scroll up
		m.sidebarScrollOffset = m.activeTab
	} else if m.activeTab >= m.sidebarScrollOffset+availableSlots {
		// Active is below visible area, scroll down
		m.sidebarScrollOffset = m.activeTab - availableSlots + 1
	}

	// Ensure scroll offset is within valid bounds
	if m.sidebarScrollOffset < 0 {
		m.sidebarScrollOffset = 0
	}
	maxOffset := m.instanceCount() - availableSlots
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.sidebarScrollOffset > maxOffset {
		m.sidebarScrollOffset = maxOffset
	}
}
