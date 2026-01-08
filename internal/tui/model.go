package tui

import (
	"regexp"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// Model holds the TUI application state
type Model struct {
	// Core components
	orchestrator *orchestrator.Orchestrator
	session      *orchestrator.Session

	// UI state
	activeTab      int
	width          int
	height         int
	ready          bool
	quitting       bool
	showHelp       bool
	showConflicts  bool // When true, show detailed conflict view
	addingTask      bool
	taskInput       string
	taskInputCursor int // Cursor position within taskInput (0 = before first char)
	errorMessage string
	infoMessage  string // Non-error status message
	inputMode    bool   // When true, all keys are forwarded to the active instance's tmux session

	// Command mode state (vim-style ex commands with ':' prefix)
	commandMode   bool   // When true, we're typing a command after ':'
	commandBuffer string // The command being typed (without the ':' prefix)

	// Template dropdown state
	showTemplates    bool   // Whether the template dropdown is visible
	templateFilter   string // Current filter text (after the "/")
	templateSelected int    // Currently highlighted template index

	// File conflict tracking
	conflicts []conflict.FileConflict

	// Instance outputs (instance ID -> output string)
	outputs map[string]string

	// Diff preview state
	showDiff    bool   // Whether the diff panel is visible
	diffContent string // Cached diff content for the active instance
	diffScroll  int    // Scroll offset for navigating the diff

	// Output scroll state (per instance)
	outputScrolls    map[string]int  // Instance ID -> scroll offset
	outputAutoScroll map[string]bool // Instance ID -> auto-scroll enabled (follows new output)
	outputLineCount  map[string]int  // Instance ID -> previous line count (to detect new output)

	// Sidebar pagination
	sidebarScrollOffset int // Index of the first visible instance in sidebar

	// Resource metrics display
	showStats bool // When true, show the stats panel

	// Search state
	searchMode    bool           // Whether search mode is active (typing pattern)
	searchPattern string         // Current search pattern
	searchRegex   *regexp.Regexp // Compiled regex (nil for literal search)
	searchMatches []int          // Line numbers containing matches
	searchCurrent int            // Current match index (for n/N navigation)

	// Filter state
	filterMode       bool            // Whether filter mode is active
	filterCategories map[string]bool // Which categories are enabled
	filterCustom     string          // Custom filter pattern
	filterRegex      *regexp.Regexp  // Compiled custom filter regex
	outputScroll     int             // Scroll position in output (for search navigation)
}

// NewModel creates a new TUI model
func NewModel(orch *orchestrator.Orchestrator, session *orchestrator.Session) Model {
	return Model{
		orchestrator:     orch,
		session:          session,
		outputs:          make(map[string]string),
		outputScrolls:    make(map[string]int),
		outputAutoScroll: make(map[string]bool),
		outputLineCount:  make(map[string]int),
		filterCategories: map[string]bool{
			"errors":   true,
			"warnings": true,
			"tools":    true,
			"thinking": true,
			"progress": true,
		},
	}
}

// activeInstance returns the currently focused instance
func (m Model) activeInstance() *orchestrator.Instance {
	if m.session == nil || len(m.session.Instances) == 0 {
		return nil
	}

	if m.activeTab >= len(m.session.Instances) {
		return nil
	}

	return m.session.Instances[m.activeTab]
}

// instanceCount returns the number of instances
func (m Model) instanceCount() int {
	if m.session == nil {
		return 0
	}
	return len(m.session.Instances)
}

// ensureActiveVisible adjusts sidebarScrollOffset to keep activeTab visible
func (m *Model) ensureActiveVisible() {
	// Calculate visible slots (same calculation as in renderSidebar)
	// Reserve: 1 for title, 1 for blank line, 1 for add hint, 2 for scroll indicators, plus border padding
	reservedLines := 6
	mainAreaHeight := m.height - 6 // Same as in View()
	availableSlots := mainAreaHeight - reservedLines
	if availableSlots < 3 {
		availableSlots = 3
	}

	// Adjust scroll offset to keep active instance visible
	if m.activeTab < m.sidebarScrollOffset {
		// Active is above visible area, scroll up
		m.sidebarScrollOffset = m.activeTab
	} else if m.activeTab >= m.sidebarScrollOffset+availableSlots {
		// Active is below visible area, scroll down
		m.sidebarScrollOffset = m.activeTab - availableSlots + 1
	}

	// Ensure scroll offset is within valid bounds
	if m.sidebarScrollOffset < 0 {
		m.sidebarScrollOffset = 0
	}
	maxOffset := m.instanceCount() - availableSlots
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.sidebarScrollOffset > maxOffset {
		m.sidebarScrollOffset = maxOffset
	}
}

// Output scroll helper methods

// getOutputMaxLines returns the maximum number of lines visible in the output area
func (m Model) getOutputMaxLines() int {
	maxLines := m.height - 12
	if maxLines < 5 {
		maxLines = 5
	}
	return maxLines
}

// getOutputLineCount returns the total number of lines in the output for an instance
// This counts lines after filtering is applied to match what the user sees
func (m *Model) getOutputLineCount(instanceID string) int {
	output := m.outputs[instanceID]
	if output == "" {
		return 0
	}
	// Apply filters to match what's displayed
	output = m.filterOutput(output)
	if output == "" {
		return 0
	}
	// Count newlines + 1 for last line
	count := 1
	for _, c := range output {
		if c == '\n' {
			count++
		}
	}
	return count
}

// getOutputMaxScroll returns the maximum scroll offset for an instance
func (m *Model) getOutputMaxScroll(instanceID string) int {
	totalLines := m.getOutputLineCount(instanceID)
	maxLines := m.getOutputMaxLines()
	maxScroll := totalLines - maxLines
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

// isOutputAutoScroll returns whether auto-scroll is enabled for an instance (defaults to true)
func (m Model) isOutputAutoScroll(instanceID string) bool {
	if autoScroll, exists := m.outputAutoScroll[instanceID]; exists {
		return autoScroll
	}
	return true // Default to auto-scroll enabled
}

// scrollOutputUp scrolls the output up by n lines and disables auto-scroll
func (m *Model) scrollOutputUp(instanceID string, n int) {
	currentScroll := m.outputScrolls[instanceID]
	newScroll := currentScroll - n
	if newScroll < 0 {
		newScroll = 0
	}
	m.outputScrolls[instanceID] = newScroll
	// Disable auto-scroll when user scrolls up
	m.outputAutoScroll[instanceID] = false
}

// scrollOutputDown scrolls the output down by n lines
func (m *Model) scrollOutputDown(instanceID string, n int) {
	currentScroll := m.outputScrolls[instanceID]
	maxScroll := m.getOutputMaxScroll(instanceID)
	newScroll := currentScroll + n
	if newScroll > maxScroll {
		newScroll = maxScroll
	}
	m.outputScrolls[instanceID] = newScroll
	// If at bottom, re-enable auto-scroll
	if newScroll >= maxScroll {
		m.outputAutoScroll[instanceID] = true
	}
}

// scrollOutputToTop scrolls to the top of the output and disables auto-scroll
func (m *Model) scrollOutputToTop(instanceID string) {
	m.outputScrolls[instanceID] = 0
	m.outputAutoScroll[instanceID] = false
}

// scrollOutputToBottom scrolls to the bottom and re-enables auto-scroll
func (m *Model) scrollOutputToBottom(instanceID string) {
	m.outputScrolls[instanceID] = m.getOutputMaxScroll(instanceID)
	m.outputAutoScroll[instanceID] = true
}

// updateOutputScroll updates scroll position based on new output (if auto-scroll is enabled)
func (m *Model) updateOutputScroll(instanceID string) {
	if m.isOutputAutoScroll(instanceID) {
		// Auto-scroll to bottom
		m.outputScrolls[instanceID] = m.getOutputMaxScroll(instanceID)
	}
	// Update line count for detecting new output
	m.outputLineCount[instanceID] = m.getOutputLineCount(instanceID)
}

// hasNewOutput returns true if there's new output since last update
func (m Model) hasNewOutput(instanceID string) bool {
	currentLines := m.getOutputLineCount(instanceID)
	previousLines, exists := m.outputLineCount[instanceID]
	if !exists {
		return false
	}
	return currentLines > previousLines
}

// Task input cursor helper methods

// taskInputInsert inserts text at the current cursor position
func (m *Model) taskInputInsert(text string) {
	runes := []rune(m.taskInput)
	m.taskInput = string(runes[:m.taskInputCursor]) + text + string(runes[m.taskInputCursor:])
	m.taskInputCursor += len([]rune(text))
}

// taskInputDeleteBack deletes n runes before the cursor
func (m *Model) taskInputDeleteBack(n int) {
	if m.taskInputCursor == 0 {
		return
	}
	runes := []rune(m.taskInput)
	deleteCount := n
	if deleteCount > m.taskInputCursor {
		deleteCount = m.taskInputCursor
	}
	m.taskInput = string(runes[:m.taskInputCursor-deleteCount]) + string(runes[m.taskInputCursor:])
	m.taskInputCursor -= deleteCount
}

// taskInputDeleteForward deletes n runes after the cursor
func (m *Model) taskInputDeleteForward(n int) {
	runes := []rune(m.taskInput)
	if m.taskInputCursor >= len(runes) {
		return
	}
	deleteCount := n
	if m.taskInputCursor+deleteCount > len(runes) {
		deleteCount = len(runes) - m.taskInputCursor
	}
	m.taskInput = string(runes[:m.taskInputCursor]) + string(runes[m.taskInputCursor+deleteCount:])
}

// taskInputMoveCursor moves cursor by n runes (negative = left, positive = right)
func (m *Model) taskInputMoveCursor(n int) {
	runes := []rune(m.taskInput)
	newPos := m.taskInputCursor + n
	if newPos < 0 {
		newPos = 0
	}
	if newPos > len(runes) {
		newPos = len(runes)
	}
	m.taskInputCursor = newPos
}

// taskInputFindPrevWordBoundary finds the position of the previous word boundary
func (m *Model) taskInputFindPrevWordBoundary() int {
	if m.taskInputCursor == 0 {
		return 0
	}
	runes := []rune(m.taskInput)
	pos := m.taskInputCursor - 1

	// Skip any whitespace/punctuation immediately before cursor
	for pos > 0 && !isWordChar(runes[pos]) {
		pos--
	}
	// Move back through the word
	for pos > 0 && isWordChar(runes[pos-1]) {
		pos--
	}
	return pos
}

// taskInputFindNextWordBoundary finds the position of the next word boundary
func (m *Model) taskInputFindNextWordBoundary() int {
	runes := []rune(m.taskInput)
	if m.taskInputCursor >= len(runes) {
		return len(runes)
	}
	pos := m.taskInputCursor

	// Skip current word
	for pos < len(runes) && isWordChar(runes[pos]) {
		pos++
	}
	// Skip whitespace/punctuation to reach next word
	for pos < len(runes) && !isWordChar(runes[pos]) {
		pos++
	}
	return pos
}

// taskInputFindLineStart finds the start of the current line
func (m *Model) taskInputFindLineStart() int {
	runes := []rune(m.taskInput)
	pos := m.taskInputCursor
	for pos > 0 && runes[pos-1] != '\n' {
		pos--
	}
	return pos
}

// taskInputFindLineEnd finds the end of the current line
func (m *Model) taskInputFindLineEnd() int {
	runes := []rune(m.taskInput)
	pos := m.taskInputCursor
	for pos < len(runes) && runes[pos] != '\n' {
		pos++
	}
	return pos
}

// isWordChar returns true if the rune is considered part of a word
func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
