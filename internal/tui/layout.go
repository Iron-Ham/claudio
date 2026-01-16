package tui

// Layout constants define the fixed dimensions and offsets used throughout the TUI.
// These values ensure consistent spacing and sizing across all views.
const (
	// DefaultSidebarWidth is the default width for the sidebar panel.
	// This is used when no user configuration is provided.
	DefaultSidebarWidth = 36

	// SidebarMinWidth is the minimum sidebar width used when terminal is narrow (< 80 columns).
	// Values below this are rejected during configuration validation.
	SidebarMinWidth = 20

	// SidebarMaxWidth is the maximum allowed sidebar width to prevent the sidebar
	// from consuming too much screen space.
	SidebarMaxWidth = 60

	// ContentWidthOffset accounts for the gap between sidebar and content area.
	// Calculated as: sidebar gap (3) + output area margin (4) = 7
	ContentWidthOffset = 7

	// ContentHeightOffset accounts for vertical UI elements that reduce content area height.
	// Includes: header + help bar + instance info + task + status + scroll indicator = 12
	ContentHeightOffset = 12
)

// ClampSidebarWidth ensures the sidebar width is within valid bounds
// (SidebarMinWidth to SidebarMaxWidth).
func ClampSidebarWidth(width int) int {
	if width < SidebarMinWidth {
		return SidebarMinWidth
	}
	if width > SidebarMaxWidth {
		return SidebarMaxWidth
	}
	return width
}

// CalculateContentDimensions returns the effective content area dimensions
// given the terminal width and height. This accounts for the sidebar and other UI elements.
// The sidebar width is reduced for narrow terminals (< 80 columns) to preserve content space.
// Uses the default sidebar width; for custom widths, use CalculateContentDimensionsWithSidebarWidth.
func CalculateContentDimensions(termWidth, termHeight int) (contentWidth, contentHeight int) {
	return CalculateContentDimensionsWithSidebarWidth(termWidth, termHeight, DefaultSidebarWidth)
}

// CalculateContentDimensionsWithSidebarWidth returns the effective content area dimensions
// given the terminal dimensions and a configured sidebar width.
// The sidebar width is reduced for narrow terminals (< 80 columns) to preserve content space.
func CalculateContentDimensionsWithSidebarWidth(termWidth, termHeight, configuredSidebarWidth int) (contentWidth, contentHeight int) {
	effectiveSidebarWidth := CalculateEffectiveSidebarWidthWithConfig(termWidth, configuredSidebarWidth)
	contentWidth = termWidth - effectiveSidebarWidth - ContentWidthOffset
	contentHeight = termHeight - ContentHeightOffset
	return contentWidth, contentHeight
}

// CalculateEffectiveSidebarWidth returns the appropriate sidebar width based on terminal width.
// Returns SidebarMinWidth for terminals narrower than 80 columns, otherwise DefaultSidebarWidth.
// For custom sidebar widths, use CalculateEffectiveSidebarWidthWithConfig.
func CalculateEffectiveSidebarWidth(termWidth int) int {
	return CalculateEffectiveSidebarWidthWithConfig(termWidth, DefaultSidebarWidth)
}

// CalculateEffectiveSidebarWidthWithConfig returns the appropriate sidebar width based on
// terminal width and user-configured sidebar width.
// Returns SidebarMinWidth for terminals narrower than 80 columns, otherwise the clamped
// configured width.
func CalculateEffectiveSidebarWidthWithConfig(termWidth, configuredSidebarWidth int) int {
	if termWidth < 80 {
		return SidebarMinWidth
	}
	return ClampSidebarWidth(configuredSidebarWidth)
}
