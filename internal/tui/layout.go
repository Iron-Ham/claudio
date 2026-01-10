// Package tui provides the terminal user interface for Claudio.
// This file contains layout-related constants and dimension calculation functions.
package tui

// Sidebar dimensions
const (
	// SidebarWidth is the default width of the sidebar panel.
	SidebarWidth = 30

	// SidebarMinWidth is the minimum sidebar width used on narrow terminals (< 80 cols).
	SidebarMinWidth = 20

	// NarrowTerminalThreshold is the terminal width below which the sidebar uses minimum width.
	NarrowTerminalThreshold = 80
)

// Layout offsets - these represent the space taken by fixed UI elements
const (
	// ContentWidthOffset accounts for sidebar gap (3) + output area margin (4).
	// Used when calculating how much width is available for content rendering.
	ContentWidthOffset = 7

	// ContentHeightOffset accounts for header + help bar + instance info + task + status + scroll indicator.
	// Used when calculating how much height is available for content rendering.
	ContentHeightOffset = 12

	// MainAreaHeightOffset accounts for header + help bar + margins.
	// Used when calculating the main area height for sidebar and content panels.
	MainAreaHeightOffset = 6

	// PanelGap is the gap between sidebar and content panels.
	PanelGap = 3

	// ContentBoxPadding is the horizontal padding inside content boxes.
	// Content boxes render with Width(width - ContentBoxPadding).
	ContentBoxPadding = 4

	// SidebarPadding is the horizontal padding inside the sidebar.
	// Sidebar renders with Width(width - SidebarPadding).
	SidebarPadding = 2
)

// Output area dimensions
const (
	// OutputHeightOffset is subtracted from terminal height to get output area max lines.
	// This is the same as ContentHeightOffset but named specifically for output context.
	OutputHeightOffset = 12

	// OutputMinLines is the minimum number of visible lines in the output area.
	OutputMinLines = 5

	// DiffPanelHeightOffset is subtracted from terminal height for diff panel content.
	// Accounts for panel header, borders, and status information.
	DiffPanelHeightOffset = 14
)

// Sidebar content dimensions
const (
	// SidebarTaskNumberWidth accounts for number, dot, and padding in task list.
	// Used as: maxTaskLen := width - SidebarTaskNumberWidth
	SidebarTaskNumberWidth = 8

	// SidebarReservedLines accounts for title, blank line, add hint, scroll indicators, and border padding.
	SidebarReservedLines = 6

	// ConflictTaskPadding is used in conflict panel for task truncation.
	// Used as: maxTaskLen := width - ConflictTaskPadding
	ConflictTaskPadding = 15

	// StatsTaskPadding is used in stats panel for task truncation.
	// Used as: taskTrunc := truncate(ic.task, width - StatsTaskPadding)
	StatsTaskPadding = 25
)

// Task display dimensions
const (
	// MaxTaskLines is the maximum number of lines to display for a task description.
	MaxTaskLines = 5
)

// GetEffectiveSidebarWidth returns the sidebar width based on terminal width.
// Uses SidebarMinWidth for narrow terminals, SidebarWidth otherwise.
func GetEffectiveSidebarWidth(termWidth int) int {
	if termWidth < NarrowTerminalThreshold {
		return SidebarMinWidth
	}
	return SidebarWidth
}

// CalculateContentDimensions returns the effective content area dimensions
// given the terminal width and height. This accounts for the sidebar and other UI elements.
// This is primarily used for resizing Claude instances to fit their available space.
func CalculateContentDimensions(termWidth, termHeight int) (contentWidth, contentHeight int) {
	effectiveSidebarWidth := GetEffectiveSidebarWidth(termWidth)
	contentWidth = termWidth - effectiveSidebarWidth - ContentWidthOffset
	contentHeight = termHeight - ContentHeightOffset
	return contentWidth, contentHeight
}

// CalculateMainAreaDimensions returns the dimensions for the main content area
// (sidebar and content panels) given the terminal dimensions.
func CalculateMainAreaDimensions(termWidth, termHeight int) (sidebarWidth, contentWidth, mainHeight int) {
	sidebarWidth = GetEffectiveSidebarWidth(termWidth)
	contentWidth = termWidth - sidebarWidth - PanelGap
	mainHeight = termHeight - MainAreaHeightOffset
	return sidebarWidth, contentWidth, mainHeight
}

// CalculateOutputMaxLines returns the maximum number of visible lines in the output area.
// This ensures a minimum of OutputMinLines even on small terminals.
func CalculateOutputMaxLines(termHeight int) int {
	maxLines := termHeight - OutputHeightOffset
	if maxLines < OutputMinLines {
		return OutputMinLines
	}
	return maxLines
}

// CalculateDiffMaxLines returns the maximum number of visible lines in the diff panel.
// This ensures a minimum of OutputMinLines even on small terminals.
func CalculateDiffMaxLines(termHeight int) int {
	maxLines := termHeight - DiffPanelHeightOffset
	if maxLines < OutputMinLines {
		return OutputMinLines
	}
	return maxLines
}

// CalculateContentBoxWidth returns the width for content box rendering.
func CalculateContentBoxWidth(availableWidth int) int {
	return availableWidth - ContentBoxPadding
}

// CalculateSidebarContentWidth returns the width for sidebar content rendering.
func CalculateSidebarContentWidth(sidebarWidth int) int {
	return sidebarWidth - SidebarPadding
}

// CalculateAvailableSidebarSlots returns the number of instance slots available
// in the sidebar given the main area height.
func CalculateAvailableSidebarSlots(mainAreaHeight int) int {
	return mainAreaHeight - SidebarReservedLines
}

// CalculateMaxTaskLength returns the maximum task name length for sidebar display.
func CalculateMaxTaskLength(sidebarWidth int) int {
	return sidebarWidth - SidebarTaskNumberWidth
}
