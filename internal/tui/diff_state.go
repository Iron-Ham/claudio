package tui

import "strings"

// DiffState holds the state for the diff preview panel.
// It tracks whether the panel is visible, the cached diff content,
// and the current scroll position for navigating large diffs.
type DiffState struct {
	// show indicates whether the diff panel is currently visible
	show bool

	// content holds the cached diff content for the active instance
	content string

	// scroll is the scroll offset (line number) for navigating the diff
	scroll int
}

// NewDiffState creates a new DiffState with default values.
func NewDiffState() *DiffState {
	return &DiffState{
		show:    false,
		content: "",
		scroll:  0,
	}
}

// IsVisible returns whether the diff panel is currently visible.
func (d *DiffState) IsVisible() bool {
	return d.show
}

// Content returns the cached diff content.
func (d *DiffState) Content() string {
	return d.content
}

// Scroll returns the current scroll offset.
func (d *DiffState) Scroll() int {
	return d.scroll
}

// Show displays the diff panel with the given content.
// It resets the scroll position to the top.
func (d *DiffState) Show(content string) {
	d.content = content
	d.show = true
	d.scroll = 0
}

// Hide hides the diff panel and clears its content.
func (d *DiffState) Hide() {
	d.show = false
	d.content = ""
	d.scroll = 0
}

// Toggle toggles the diff panel visibility.
// If hiding, it clears the content.
// Returns true if the panel is now visible, false if hidden.
func (d *DiffState) Toggle() bool {
	if d.show {
		d.Hide()
		return false
	}
	// Note: When toggling on, caller should set content via Show()
	return true
}

// ScrollDown scrolls down by n lines, respecting the maximum scroll limit.
// Returns the new scroll offset.
func (d *DiffState) ScrollDown(n int, maxScroll int) int {
	d.scroll += n
	if d.scroll > maxScroll {
		d.scroll = maxScroll
	}
	return d.scroll
}

// ScrollUp scrolls up by n lines, stopping at 0.
// Returns the new scroll offset.
func (d *DiffState) ScrollUp(n int) int {
	d.scroll -= n
	if d.scroll < 0 {
		d.scroll = 0
	}
	return d.scroll
}

// ScrollToTop scrolls to the top of the diff content.
func (d *DiffState) ScrollToTop() {
	d.scroll = 0
}

// ScrollToBottom scrolls to the bottom of the diff content.
// Requires the maximum scroll value as a parameter.
func (d *DiffState) ScrollToBottom(maxScroll int) {
	d.scroll = maxScroll
}

// ClampScroll ensures the scroll position is within valid bounds.
// Returns true if the scroll position was adjusted.
func (d *DiffState) ClampScroll(maxScroll int) bool {
	if d.scroll > maxScroll {
		d.scroll = maxScroll
		return true
	}
	return false
}

// LineCount returns the number of lines in the diff content.
func (d *DiffState) LineCount() int {
	if d.content == "" {
		return 0
	}
	return len(strings.Split(d.content, "\n"))
}

// CalculateMaxScroll calculates the maximum scroll offset for the given
// visible line count. Returns 0 if all content fits in the visible area.
func (d *DiffState) CalculateMaxScroll(visibleLines int) int {
	totalLines := d.LineCount()
	maxScroll := totalLines - visibleLines
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

// GetVisibleLines returns the lines that should be visible given the
// current scroll position and the number of visible lines.
// Returns the visible lines slice, start line number, and end line number.
func (d *DiffState) GetVisibleLines(visibleLineCount int) ([]string, int, int) {
	if d.content == "" {
		return nil, 0, 0
	}

	lines := strings.Split(d.content, "\n")
	totalLines := len(lines)

	// Clamp scroll position
	maxScroll := d.CalculateMaxScroll(visibleLineCount)
	if d.scroll > maxScroll {
		d.scroll = maxScroll
	}

	// Calculate visible range
	startLine := d.scroll
	endLine := min(startLine+visibleLineCount, totalLines)

	return lines[startLine:endLine], startLine, endLine
}

// HasContent returns true if there is diff content to display.
func (d *DiffState) HasContent() bool {
	return d.content != ""
}
