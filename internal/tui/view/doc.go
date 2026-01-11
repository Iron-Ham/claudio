// Package view provides reusable view components for the Claudio TUI.
//
// This package extracts rendering logic from the main TUI model into
// focused, testable components. It uses the Bubble Tea framework's
// patterns for building terminal user interfaces with lipgloss styling.
//
// # Main Types
//
//   - [InstanceView]: Renders a single instance's detail view with output, status, and metrics
//   - [RenderState]: Dynamic state needed for rendering (output, scroll position, search)
//
// # InstanceView Components
//
// The [InstanceView] renders several components:
//   - Header: Status badge and branch information
//   - Task: Task description with truncation
//   - Metrics: Token usage, cost, and duration (when enabled)
//   - Status Banner: Running/input mode indicators
//   - Output Area: Scrollable output with search highlighting
//   - Search Bar: Search input with match navigation
//
// # Render State
//
// [RenderState] separates render-time state from persistent instance data:
//   - Output: Current terminal output text
//   - IsRunning: Whether the instance manager is active
//   - InputMode: Whether TUI is capturing input for this instance
//   - ScrollOffset: Current scroll position in output
//   - AutoScrollEnabled: Whether to follow new output
//   - HasNewOutput: New output arrived while scrolled up
//   - Search*: Search pattern, regex, matches, and current match index
//
// # Scrolling
//
// The output area supports vim-style scrolling:
//   - j/k or arrows: Line-by-line scrolling
//   - g/G: Jump to top/bottom
//   - Auto-scroll: Follows output when at bottom
//   - New output indicator: Shows when scrolled up and new output arrives
//
// # Search
//
// Output search with regex support:
//   - / to start search
//   - n/N to navigate matches
//   - Highlights current match differently from other matches
//   - Shows match count and current position
//
// # Basic Usage
//
//	view := view.NewInstanceView(80, 20) // width, maxOutputLines
//
//	state := view.RenderState{
//	    Output:            capturedOutput,
//	    IsRunning:         true,
//	    AutoScrollEnabled: true,
//	}
//
//	rendered := view.Render(instance, state)
//	fmt.Print(rendered)
//
// # Helpers
//
// The package provides formatting utilities:
//   - [FormatDuration]: Formats time.Duration for display (e.g., "5m 30s")
package view
