package tui

// Layout constants define the fixed dimensions and offsets used throughout the TUI.
// These values ensure consistent spacing and sizing across all views.
const (
	// SidebarWidth is the standard width for the sidebar panel.
	SidebarWidth = 30

	// SidebarMinWidth is the minimum sidebar width used when terminal is narrow (< 80 columns).
	SidebarMinWidth = 20

	// ContentWidthOffset accounts for the gap between sidebar and content area.
	// Calculated as: sidebar gap (3) + output area margin (4) = 7
	ContentWidthOffset = 7

	// ContentHeightOffset accounts for vertical UI elements that reduce content area height.
	// Includes: header + help bar + instance info + task + status + scroll indicator = 12
	ContentHeightOffset = 12
)

// CalculateContentDimensions returns the effective content area dimensions
// given the terminal width and height. This accounts for the sidebar and other UI elements.
// The sidebar width is reduced for narrow terminals (< 80 columns) to preserve content space.
func CalculateContentDimensions(termWidth, termHeight int) (contentWidth, contentHeight int) {
	effectiveSidebarWidth := SidebarWidth
	if termWidth < 80 {
		effectiveSidebarWidth = SidebarMinWidth
	}
	contentWidth = termWidth - effectiveSidebarWidth - ContentWidthOffset
	contentHeight = termHeight - ContentHeightOffset
	return contentWidth, contentHeight
}

// CalculateEffectiveSidebarWidth returns the appropriate sidebar width based on terminal width.
// Returns SidebarMinWidth for terminals narrower than 80 columns, otherwise SidebarWidth.
func CalculateEffectiveSidebarWidth(termWidth int) int {
	if termWidth < 80 {
		return SidebarMinWidth
	}
	return SidebarWidth
}
