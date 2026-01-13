// Package panel provides interfaces and types for TUI panel rendering.
package panel

import (
	"github.com/Iron-Ham/claudio/internal/tui/view"
)

// ConflictPanel renders file conflict information between instances.
// It adapts the view.ConflictsView to the PanelRenderer interface.
type ConflictPanel struct {
	height int
}

// NewConflictPanel creates a new ConflictPanel.
func NewConflictPanel() *ConflictPanel {
	return &ConflictPanel{}
}

// Render produces the conflict panel output.
func (p *ConflictPanel) Render(state *RenderState) string {
	if err := state.ValidateBasic(); err != nil {
		return "[conflict panel: render error]"
	}

	// No conflicts, return empty
	if len(state.Conflicts) == 0 {
		p.height = 0
		return ""
	}

	// Build instance info list from state instances
	instanceInfos := make([]view.InstanceInfo, len(state.Instances))
	for i, inst := range state.Instances {
		instanceInfos[i] = view.InstanceInfo{
			ID:   inst.ID,
			Task: inst.Task,
		}
	}

	// Use the existing ConflictsView for rendering
	conflictsView := view.NewConflictsView(state.Conflicts, instanceInfos)
	result := conflictsView.Render(state.Width)

	// Estimate height from result
	p.height = countNewlines(result) + 1

	return result
}

// Height returns the rendered height of the panel.
func (p *ConflictPanel) Height() int {
	return p.height
}

// countNewlines counts the number of newlines in a string.
func countNewlines(s string) int {
	count := 0
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	return count
}
