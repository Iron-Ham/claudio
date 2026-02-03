// Package filter provides output filtering functionality for the Claudio TUI.
//
// This package encapsulates category-based and regex-based filtering of
// backend output. It allows users to show/hide specific types of output
// such as errors, warnings, tool calls, thinking, and progress indicators.
//
// # Main Types
//
//   - [Filter]: The main filter engine that manages categories and custom patterns
//   - [Category]: Predefined filter categories with keyword patterns
//   - [Categories]: Standard set of filter categories
//
// # Filtering Modes
//
// The filter supports two filtering modes:
//
//  1. Category filtering: Toggle visibility of predefined categories
//     (errors, warnings, tools, thinking, progress)
//
//  2. Custom regex filtering: When a custom pattern is set, it takes
//     precedence over category filters, showing only matching lines
//
// # Usage
//
//	f := filter.New()
//
//	// Toggle category visibility
//	f.ToggleCategory("errors")
//	f.ToggleCategory("warnings")
//
//	// Apply filter to output
//	filtered := f.Apply(rawOutput)
//
//	// Set custom regex pattern
//	f.SetCustomPattern("TODO|FIXME")
//	filtered = f.Apply(rawOutput)
//
// # Keyboard Input Handling
//
// The [InputResult] type captures the result of handling a key press:
//
//	result := f.HandleKey(keyMsg)
//	if result.ExitMode {
//	    // User pressed Esc/F/q to exit filter mode
//	}
//
// # Panel Rendering
//
// The package provides a panel renderer for the filter configuration UI:
//
//	panel := filter.RenderPanel(f, width)
package filter
