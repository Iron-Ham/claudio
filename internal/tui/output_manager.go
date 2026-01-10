package tui

// OutputManager consolidates all output-related state and operations for the TUI.
// It manages the outputs map (instance ID -> output string), scroll state,
// auto-scroll behavior, and provides methods for output manipulation.
//
// This extracts output management from the Model struct to improve separation
// of concerns and make the output handling logic more testable.
type OutputManager struct {
	// outputs stores the raw output content per instance ID
	outputs map[string]string

	// state manages scroll positions, auto-scroll, and line counts per instance
	state *OutputState
}

// NewOutputManager creates a new OutputManager with initialized storage.
func NewOutputManager() *OutputManager {
	return &OutputManager{
		outputs: make(map[string]string),
		state:   NewOutputState(),
	}
}

// GetOutput returns the output content for an instance.
func (om *OutputManager) GetOutput(instanceID string) string {
	return om.outputs[instanceID]
}

// SetOutput sets the output content for an instance.
func (om *OutputManager) SetOutput(instanceID string, content string) {
	om.outputs[instanceID] = content
}

// AppendOutput appends content to an instance's output.
func (om *OutputManager) AppendOutput(instanceID string, content string) {
	om.outputs[instanceID] += content
}

// ClearOutput clears the output for an instance.
func (om *OutputManager) ClearOutput(instanceID string) {
	om.outputs[instanceID] = ""
}

// HasOutput returns true if the instance has any output.
func (om *OutputManager) HasOutput(instanceID string) bool {
	return om.outputs[instanceID] != ""
}

// GetScroll returns the scroll offset for an instance.
func (om *OutputManager) GetScroll(instanceID string) int {
	return om.state.GetScroll(instanceID)
}

// SetScroll sets the scroll offset for an instance.
func (om *OutputManager) SetScroll(instanceID string, offset int) {
	om.state.SetScroll(instanceID, offset)
}

// IsAutoScroll returns whether auto-scroll is enabled for an instance.
// Defaults to true if not explicitly set.
func (om *OutputManager) IsAutoScroll(instanceID string) bool {
	return om.state.IsAutoScroll(instanceID)
}

// SetAutoScroll sets the auto-scroll flag for an instance.
func (om *OutputManager) SetAutoScroll(instanceID string, enabled bool) {
	om.state.SetAutoScroll(instanceID, enabled)
}

// ScrollUp scrolls up by n lines and disables auto-scroll.
// Returns the new scroll offset.
func (om *OutputManager) ScrollUp(instanceID string, n int) int {
	return om.state.ScrollUp(instanceID, n)
}

// ScrollDown scrolls down by n lines, capped at maxScroll.
// Re-enables auto-scroll if at bottom.
// Returns the new scroll offset.
func (om *OutputManager) ScrollDown(instanceID string, n int, maxScroll int) int {
	return om.state.ScrollDown(instanceID, n, maxScroll)
}

// ScrollToTop scrolls to the top and disables auto-scroll.
func (om *OutputManager) ScrollToTop(instanceID string) {
	om.state.ScrollToTop(instanceID)
}

// ScrollToBottom scrolls to the bottom (maxScroll) and re-enables auto-scroll.
func (om *OutputManager) ScrollToBottom(instanceID string, maxScroll int) {
	om.state.ScrollToBottom(instanceID, maxScroll)
}

// UpdateForNewOutput updates scroll position based on new output if auto-scroll is enabled.
// Also stores the current line count for future comparison.
func (om *OutputManager) UpdateForNewOutput(instanceID string, maxScroll int, currentLineCount int) {
	om.state.UpdateForNewOutput(instanceID, maxScroll, currentLineCount)
}

// HasNewOutput returns true if there's new output since the last recorded line count.
func (om *OutputManager) HasNewOutput(instanceID string, currentLineCount int) bool {
	return om.state.HasNewOutput(instanceID, currentLineCount)
}

// GetLineCount returns the stored line count for an instance.
func (om *OutputManager) GetLineCount(instanceID string) int {
	return om.state.GetLineCount(instanceID)
}

// SetLineCount sets the stored line count for an instance.
func (om *OutputManager) SetLineCount(instanceID string, count int) {
	om.state.SetLineCount(instanceID, count)
}

// State returns the underlying OutputState for direct access when needed.
// This is primarily for backward compatibility during refactoring.
func (om *OutputManager) State() *OutputState {
	return om.state
}

// Outputs returns the underlying outputs map for direct access when needed.
// This is primarily for backward compatibility during refactoring.
func (om *OutputManager) Outputs() map[string]string {
	return om.outputs
}
