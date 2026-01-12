// Package panel provides interfaces and types for TUI panel rendering.
// Each panel in the TUI (sidebar, main content, header, footer, etc.)
// can implement the PanelRenderer interface for consistent rendering behavior.
package panel

import (
	"errors"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/charmbracelet/lipgloss"
)

// Common errors returned by RenderState validation.
var (
	ErrInvalidWidth  = errors.New("width must be positive")
	ErrInvalidHeight = errors.New("height must be positive")
	ErrNilTheme      = errors.New("theme cannot be nil")
)

// PanelRenderer defines the interface for rendering UI panels.
// Each panel type (sidebar, main content area, header, etc.) implements
// this interface to provide consistent rendering behavior across the TUI.
type PanelRenderer interface {
	// Render produces the visual output for this panel given the current state.
	// The returned string contains the rendered content, potentially with
	// ANSI escape codes for styling.
	Render(state *RenderState) string

	// Height returns the rendered height of the panel in terminal rows.
	// This is useful for layout calculations and ensuring panels fit
	// within available space.
	Height() int
}

// Theme provides styling configuration for panel rendering.
// This interface abstracts the styling system, allowing panels to
// request styles without depending on concrete style implementations.
type Theme interface {
	// Primary returns the primary style for emphasis.
	Primary() lipgloss.Style
	// Secondary returns the secondary style for less prominent elements.
	Secondary() lipgloss.Style
	// Muted returns the muted style for de-emphasized elements.
	Muted() lipgloss.Style
	// Error returns the style for error states.
	Error() lipgloss.Style
	// Warning returns the style for warning states.
	Warning() lipgloss.Style
	// Surface returns the style for surface/background areas.
	Surface() lipgloss.Style
	// Border returns the style for borders.
	Border() lipgloss.Style
}

// RenderState holds the complete state needed for rendering a panel.
// It provides a snapshot of the TUI state at render time, decoupling
// panel renderers from the full application model.
type RenderState struct {
	// Width is the available width in terminal columns.
	Width int

	// Height is the available height in terminal rows.
	Height int

	// ActiveInstance is the currently selected/focused instance.
	// May be nil if no instance is selected.
	ActiveInstance *orchestrator.Instance

	// Instances is the list of all instances in the session.
	// May be empty but should not be nil.
	Instances []*orchestrator.Instance

	// Theme provides styling for the panel.
	// Required for rendering styled output.
	Theme Theme

	// ActiveIndex is the index of the active instance in the Instances slice.
	// Set to -1 when no instance is active or when ActiveInstance is nil.
	ActiveIndex int

	// ScrollOffset is the current scroll position for scrollable panels.
	// Interpretation varies by panel type.
	ScrollOffset int

	// Focused indicates whether this panel currently has focus.
	// Used to adjust border styling and visual emphasis.
	Focused bool
}

// Validate checks that the RenderState has valid values for rendering.
// Returns an error if any required fields are invalid.
func (rs *RenderState) Validate() error {
	if rs.Width <= 0 {
		return ErrInvalidWidth
	}
	if rs.Height <= 0 {
		return ErrInvalidHeight
	}
	if rs.Theme == nil {
		return ErrNilTheme
	}
	return nil
}

// ValidateBasic performs minimal validation checking only dimensions.
// Use this when theme may be optional (e.g., for tests with mock output).
func (rs *RenderState) ValidateBasic() error {
	if rs.Width <= 0 {
		return ErrInvalidWidth
	}
	if rs.Height <= 0 {
		return ErrInvalidHeight
	}
	return nil
}

// IsActiveInstance returns true if the given instance is the active one.
// Safe to call with nil values.
func (rs *RenderState) IsActiveInstance(inst *orchestrator.Instance) bool {
	if rs.ActiveInstance == nil || inst == nil {
		return false
	}
	return rs.ActiveInstance.ID == inst.ID
}

// InstanceCount returns the number of instances in the state.
func (rs *RenderState) InstanceCount() int {
	return len(rs.Instances)
}

// GetInstance returns the instance at the given index, or nil if out of bounds.
func (rs *RenderState) GetInstance(index int) *orchestrator.Instance {
	if index < 0 || index >= len(rs.Instances) {
		return nil
	}
	return rs.Instances[index]
}

// HasInstances returns true if there is at least one instance.
func (rs *RenderState) HasInstances() bool {
	return len(rs.Instances) > 0
}

// VisibleRange calculates the range of instances visible given the scroll offset
// and available slots. Returns start (inclusive) and end (exclusive) indices.
func (rs *RenderState) VisibleRange(availableSlots int) (start, end int) {
	count := rs.InstanceCount()
	if count == 0 || availableSlots <= 0 {
		return 0, 0
	}

	start = max(rs.ScrollOffset, 0)
	start = min(start, count-1)

	end = min(start+availableSlots, count)

	return start, end
}

// DefaultRenderState creates a new RenderState with sensible defaults.
// Width and Height are set to common terminal dimensions.
// Theme must still be set before rendering.
func DefaultRenderState() *RenderState {
	return &RenderState{
		Width:       80,
		Height:      24,
		Instances:   make([]*orchestrator.Instance, 0),
		ActiveIndex: -1,
	}
}

// NewRenderState creates a RenderState with the given dimensions.
// Instances slice is initialized to empty, ActiveIndex to -1.
func NewRenderState(width, height int) *RenderState {
	return &RenderState{
		Width:       width,
		Height:      height,
		Instances:   make([]*orchestrator.Instance, 0),
		ActiveIndex: -1,
	}
}
