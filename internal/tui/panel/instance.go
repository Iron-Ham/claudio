// Package panel provides interfaces and types for TUI panel rendering.
package panel

import (
	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/view"
)

// InstancePanel renders the instance sidebar showing all instances and their status.
// It adapts the view.DashboardView to the PanelRenderer interface.
type InstancePanel struct {
	height int
	view   *view.DashboardView
}

// NewInstancePanel creates a new InstancePanel.
func NewInstancePanel() *InstancePanel {
	return &InstancePanel{
		view: view.NewDashboardView(),
	}
}

// Render produces the instance panel output.
func (p *InstancePanel) Render(state *RenderState) string {
	if err := state.ValidateBasic(); err != nil {
		return "[instance panel: render error]"
	}

	// Create adapter state from RenderState
	adapterState := &instancePanelState{
		session:             state.Session,
		activeTab:           state.ActiveIndex,
		sidebarScrollOffset: state.ScrollOffset,
		conflicts:           state.Conflicts,
		terminalWidth:       state.Width,
		terminalHeight:      state.Height,
		isAddingTask:        state.IsAddingTask,
	}

	result := p.view.RenderSidebar(adapterState, state.Width, state.Height)

	// Estimate height from result
	p.height = countNewlines(result) + 1

	return result
}

// Height returns the rendered height of the panel.
func (p *InstancePanel) Height() int {
	return p.height
}

// instancePanelState adapts RenderState to the view.DashboardState interface.
type instancePanelState struct {
	session             *orchestrator.Session
	activeTab           int
	sidebarScrollOffset int
	conflicts           []conflict.FileConflict
	terminalWidth       int
	terminalHeight      int
	isAddingTask        bool
}

// Session implements view.DashboardState.
func (s *instancePanelState) Session() *orchestrator.Session {
	return s.session
}

// ActiveTab implements view.DashboardState.
func (s *instancePanelState) ActiveTab() int {
	return s.activeTab
}

// SidebarScrollOffset implements view.DashboardState.
func (s *instancePanelState) SidebarScrollOffset() int {
	return s.sidebarScrollOffset
}

// Conflicts implements view.DashboardState.
func (s *instancePanelState) Conflicts() []conflict.FileConflict {
	return s.conflicts
}

// TerminalWidth implements view.DashboardState.
func (s *instancePanelState) TerminalWidth() int {
	return s.terminalWidth
}

// TerminalHeight implements view.DashboardState.
func (s *instancePanelState) TerminalHeight() int {
	return s.terminalHeight
}

// IsAddingTask implements view.DashboardState.
func (s *instancePanelState) IsAddingTask() bool {
	return s.isAddingTask
}
