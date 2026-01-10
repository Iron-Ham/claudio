package tui

// OutputState holds per-instance output scroll state management.
// It tracks scroll positions, auto-scroll preferences, and line counts
// for detecting new output in the output panel.
type OutputState struct {
	// scrolls maps instance ID to scroll offset (lines from top)
	scrolls map[string]int

	// autoScroll maps instance ID to auto-scroll enabled flag.
	// When true, the output automatically scrolls to show new content.
	autoScroll map[string]bool

	// lineCount maps instance ID to the previous line count.
	// Used to detect when new output has been added.
	lineCount map[string]int
}

// NewOutputState creates a new OutputState with initialized maps.
func NewOutputState() *OutputState {
	return &OutputState{
		scrolls:    make(map[string]int),
		autoScroll: make(map[string]bool),
		lineCount:  make(map[string]int),
	}
}

// GetScroll returns the scroll offset for an instance.
func (s *OutputState) GetScroll(instanceID string) int {
	return s.scrolls[instanceID]
}

// SetScroll sets the scroll offset for an instance.
func (s *OutputState) SetScroll(instanceID string, offset int) {
	s.scrolls[instanceID] = offset
}

// IsAutoScroll returns whether auto-scroll is enabled for an instance.
// Defaults to true if not explicitly set.
func (s *OutputState) IsAutoScroll(instanceID string) bool {
	if autoScroll, exists := s.autoScroll[instanceID]; exists {
		return autoScroll
	}
	return true // Default to auto-scroll enabled
}

// SetAutoScroll sets the auto-scroll flag for an instance.
func (s *OutputState) SetAutoScroll(instanceID string, enabled bool) {
	s.autoScroll[instanceID] = enabled
}

// GetLineCount returns the stored line count for an instance.
func (s *OutputState) GetLineCount(instanceID string) int {
	return s.lineCount[instanceID]
}

// SetLineCount sets the stored line count for an instance.
func (s *OutputState) SetLineCount(instanceID string, count int) {
	s.lineCount[instanceID] = count
}

// HasLineCount returns whether a line count has been recorded for an instance.
func (s *OutputState) HasLineCount(instanceID string) bool {
	_, exists := s.lineCount[instanceID]
	return exists
}

// ScrollUp scrolls up by n lines and disables auto-scroll.
// Returns the new scroll offset.
func (s *OutputState) ScrollUp(instanceID string, n int) int {
	currentScroll := s.scrolls[instanceID]
	newScroll := max(currentScroll-n, 0)
	s.scrolls[instanceID] = newScroll
	s.autoScroll[instanceID] = false
	return newScroll
}

// ScrollDown scrolls down by n lines, capped at maxScroll.
// Re-enables auto-scroll if at bottom.
// Returns the new scroll offset.
func (s *OutputState) ScrollDown(instanceID string, n int, maxScroll int) int {
	currentScroll := s.scrolls[instanceID]
	newScroll := min(currentScroll+n, maxScroll)
	s.scrolls[instanceID] = newScroll
	// If at bottom, re-enable auto-scroll
	if newScroll >= maxScroll {
		s.autoScroll[instanceID] = true
	}
	return newScroll
}

// ScrollToTop scrolls to the top and disables auto-scroll.
func (s *OutputState) ScrollToTop(instanceID string) {
	s.scrolls[instanceID] = 0
	s.autoScroll[instanceID] = false
}

// ScrollToBottom scrolls to the bottom (maxScroll) and re-enables auto-scroll.
func (s *OutputState) ScrollToBottom(instanceID string, maxScroll int) {
	s.scrolls[instanceID] = maxScroll
	s.autoScroll[instanceID] = true
}

// UpdateForNewOutput updates scroll position based on new output if auto-scroll is enabled.
// Also stores the current line count for future comparison.
func (s *OutputState) UpdateForNewOutput(instanceID string, maxScroll int, currentLineCount int) {
	if s.IsAutoScroll(instanceID) {
		s.scrolls[instanceID] = maxScroll
	}
	s.lineCount[instanceID] = currentLineCount
}

// HasNewOutput returns true if there's new output since the last recorded line count.
func (s *OutputState) HasNewOutput(instanceID string, currentLineCount int) bool {
	previousLines, exists := s.lineCount[instanceID]
	if !exists {
		return false
	}
	return currentLineCount > previousLines
}
