// Package view provides the ultraplan view components.
//
// This file provides backward-compatible wrappers around the ultraplan subpackage.
// The actual implementation has been refactored into focused components in
// internal/tui/view/ultraplan/ for better maintainability and testability.
package view

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/view/ultraplan"
)

// UltraPlanState holds ultra-plan specific UI state.
// This is an alias to the implementation in the ultraplan subpackage.
type UltraPlanState = ultraplan.State

// RenderContext provides the necessary context for rendering ultraplan views.
// This is an alias to the implementation in the ultraplan subpackage.
type RenderContext = ultraplan.RenderContext

// UltraplanView handles rendering of ultra-plan UI components including
// phase display, task graph visualization, and progress indicators.
type UltraplanView struct {
	view *ultraplan.View
	ctx  *RenderContext
}

// NewUltraplanView creates a new ultraplan view with the given render context.
func NewUltraplanView(ctx *RenderContext) *UltraplanView {
	return &UltraplanView{
		view: ultraplan.NewView(ctx),
		ctx:  ctx,
	}
}

// Render renders the main ultraplan view based on the current state.
// Returns an empty string if not in ultraplan mode.
func (v *UltraplanView) Render() string {
	return v.view.Render()
}

// RenderHeader renders the ultra-plan header with phase and progress.
func (v *UltraplanView) RenderHeader() string {
	return v.view.RenderHeader()
}

// RenderSidebar renders a unified sidebar showing all phases with their instances.
func (v *UltraplanView) RenderSidebar(width int, height int) string {
	return v.view.RenderSidebar(width, height)
}

// RenderInlineContent renders the ultraplan phase content for display inline within a group.
func (v *UltraplanView) RenderInlineContent(width int, maxLines int) string {
	return v.view.RenderInlineContent(width, maxLines)
}

// RenderPlanView renders the detailed plan view.
func (v *UltraplanView) RenderPlanView(width int) string {
	return v.view.RenderPlanView(width)
}

// RenderHelp renders the help bar for ultra-plan mode.
func (v *UltraplanView) RenderHelp() string {
	return v.view.RenderHelp()
}

// RenderConsolidationSidebar renders the sidebar during the consolidation phase.
func (v *UltraplanView) RenderConsolidationSidebar(width int, height int) string {
	return v.view.RenderConsolidationSidebar(width, height)
}

// ExecutionTaskResult holds the rendered task line(s) and the number of lines used.
type ExecutionTaskResult = ultraplan.ExecutionTaskResult

// GroupStats holds statistics about a task group for collapsed display.
type GroupStats = ultraplan.GroupStats

// Legacy methods for backward compatibility with existing tests.
// These delegate to the appropriate subpackage components.

// renderExecutionTaskLine renders a task line in the execution section.
func (v *UltraplanView) renderExecutionTaskLine(session *orchestrator.UltraPlanSession, task *orchestrator.PlannedTask, instanceID string, selected, navigable bool, maxWidth int) ExecutionTaskResult {
	return v.view.Tasks.RenderExecutionTaskLine(session, task, instanceID, selected, navigable, maxWidth)
}

// calculateGroupStats calculates statistics for a task group.
func (v *UltraplanView) calculateGroupStats(session *orchestrator.UltraPlanSession, group []string) GroupStats {
	return v.view.Tasks.CalculateGroupStats(session, group)
}

// formatGroupSummary formats the summary statistics for a collapsed group.
func (v *UltraplanView) formatGroupSummary(stats GroupStats) string {
	return v.view.Tasks.FormatGroupSummary(stats)
}

// findInstanceIDForTask finds the instance ID associated with a task.
func (v *UltraplanView) findInstanceIDForTask(session *orchestrator.UltraPlanSession, taskID string) string {
	return v.view.Tasks.FindInstanceIDForTask(session, taskID)
}

// Package-level functions for backward compatibility

// RenderProgressBar renders a simple ASCII progress bar.
func RenderProgressBar(percent int, width int) string {
	return ultraplan.RenderProgressBar(percent, width)
}

// PhaseToString converts a phase to a display string.
func PhaseToString(phase orchestrator.UltraPlanPhase) string {
	return ultraplan.PhaseToString(phase)
}

// PhaseStyle returns the style for a phase indicator.
func PhaseStyle(phase orchestrator.UltraPlanPhase) interface{} {
	return ultraplan.PhaseStyle(phase)
}

// ComplexityIndicator returns a visual indicator for task complexity.
func ComplexityIndicator(complexity orchestrator.TaskComplexity) string {
	return ultraplan.ComplexityIndicator(complexity)
}

// ConsolidationPhaseIcon returns an icon for the consolidation phase.
func ConsolidationPhaseIcon(phase orchestrator.ConsolidationPhase) string {
	return ultraplan.ConsolidationPhaseIcon(phase)
}

// ConsolidationPhaseDesc returns a human-readable description for the consolidation phase.
func ConsolidationPhaseDesc(phase orchestrator.ConsolidationPhase) string {
	return ultraplan.ConsolidationPhaseDesc(phase)
}

// OpenURL opens the given URL in the default browser.
func OpenURL(url string) error {
	return ultraplan.OpenURL(url)
}

// trimLeadingSpaces removes leading space characters from a rune slice.
// This is exported for test compatibility with the original implementation.
func trimLeadingSpaces(runes []rune) []rune {
	for len(runes) > 0 && runes[0] == ' ' {
		runes = runes[1:]
	}
	return runes
}

// padToWidth pads a string with spaces to reach the target width.
// This is exported for test compatibility with the original implementation.
func padToWidth(s string, width int) string {
	return ultraplan.PadToWidth(s, width)
}
