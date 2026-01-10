package tui

import (
	"regexp"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// PlanEditorState holds the state for the interactive plan editor
type PlanEditorState struct {
	// active indicates whether the plan editor is currently shown
	active bool

	// selectedTaskIdx is the index of the currently selected task in the task list
	selectedTaskIdx int

	// editingField indicates which field is being edited (empty if not editing)
	// Valid values: 'title', 'description', 'files', 'depends_on', 'priority', 'complexity'
	editingField string

	// editBuffer holds the current edit buffer content when editing a field
	editBuffer string

	// editCursor is the cursor position within the edit buffer (0 = before first char)
	editCursor int

	// scrollOffset is the vertical scroll offset for the task list
	scrollOffset int

	// validation holds the current validation results for the plan
	validation *orchestrator.ValidationResult

	// showValidationPanel controls whether the validation panel is visible
	showValidationPanel bool

	// validationScrollOffset is the scroll offset for the validation panel
	validationScrollOffset int

	// tasksInCycle contains task IDs that are part of a dependency cycle (for highlighting)
	tasksInCycle map[string]bool
}

// Model holds the TUI application state
type Model struct {
	// Core components
	orchestrator *orchestrator.Orchestrator
	session      *orchestrator.Session

	// Ultra-plan mode (nil if not in ultra-plan mode)
	ultraPlan *UltraPlanState

	// Plan editor mode (nil if not in plan editor mode)
	planEditor *PlanEditorState

	// UI state
	activeTab      int
	width          int
	height         int
	ready          bool
	quitting       bool
	showHelp       bool
	showConflicts  bool // When true, show detailed conflict view
	addingTask     bool
	taskInput      *InputHandler // Text input handler for task description
	errorMessage   string
	infoMessage  string // Non-error status message
	inputMode    bool   // When true, all keys are forwarded to the active instance's tmux session

	// Command mode state (vim-style ex commands with ':' prefix)
	commandHandler *CommandHandler // Encapsulates command mode logic
	commandMode    bool            // When true, we're typing a command after ':' (legacy, synced from handler)
	commandBuffer  string          // The command being typed (without the ':' prefix) (legacy, synced from handler)

	// Template dropdown handler
	templateHandler *TemplateHandler

	// File conflict tracking
	conflicts []conflict.FileConflict

	// Output management (consolidates outputs map and scroll state)
	outputManager *OutputManager

	// Diff preview state
	diffState *DiffState

	// Sidebar pagination
	sidebarScrollOffset int // Index of the first visible instance in sidebar

	// Resource metrics display
	showStats bool // When true, show the stats panel

	// Search state (encapsulates search mode, pattern, matches, navigation)
	search *SearchState

	// Filter state
	filterMode       bool            // Whether filter mode is active
	filterCategories map[string]bool // Which categories are enabled
	filterCustom     string          // Custom filter pattern
	filterRegex      *regexp.Regexp  // Compiled custom filter regex
	outputScroll     int             // Scroll position in output (for search navigation)
}

// IsUltraPlanMode returns true if the model is in ultra-plan mode
func (m Model) IsUltraPlanMode() bool {
	return m.ultraPlan != nil
}

// IsPlanEditorActive returns true if the plan editor is currently active and visible
func (m Model) IsPlanEditorActive() bool {
	return m.planEditor != nil && m.planEditor.active
}

// enterPlanEditor initializes the plan editor state when entering edit mode
func (m *Model) enterPlanEditor() {
	m.planEditor = &PlanEditorState{
		active:              true,
		selectedTaskIdx:     0,
		editingField:        "",
		editBuffer:          "",
		editCursor:          0,
		scrollOffset:        0,
		showValidationPanel: true, // Show validation by default
		tasksInCycle:        make(map[string]bool),
	}
	// Run initial validation
	m.updatePlanValidation()
}

// updatePlanValidation runs validation on the current plan and updates the editor state
func (m *Model) updatePlanValidation() {
	if m.planEditor == nil || m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil || session.Plan == nil {
		return
	}

	// Run validation
	m.planEditor.validation = orchestrator.ValidatePlanForEditor(session.Plan)

	// Update tasks in cycle map for highlighting
	m.planEditor.tasksInCycle = make(map[string]bool)
	cycleTasks := orchestrator.GetTasksInCycle(session.Plan)
	for _, taskID := range cycleTasks {
		m.planEditor.tasksInCycle[taskID] = true
	}
}

// isTaskInCycle returns true if the given task is part of a dependency cycle
func (m *Model) isTaskInCycle(taskID string) bool {
	if m.planEditor == nil || m.planEditor.tasksInCycle == nil {
		return false
	}
	return m.planEditor.tasksInCycle[taskID]
}

// canConfirmPlan returns true if the plan can be confirmed (no validation errors)
func (m *Model) canConfirmPlan() bool {
	if m.planEditor == nil || m.planEditor.validation == nil {
		return false
	}
	return !m.planEditor.validation.HasErrors()
}

// getValidationMessagesForSelectedTask returns validation messages for the currently selected task
func (m *Model) getValidationMessagesForSelectedTask() []orchestrator.ValidationMessage {
	if m.planEditor == nil || m.planEditor.validation == nil || m.ultraPlan == nil {
		return nil
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil || session.Plan == nil || m.planEditor.selectedTaskIdx >= len(session.Plan.Tasks) {
		return nil
	}

	taskID := session.Plan.Tasks[m.planEditor.selectedTaskIdx].ID
	return m.planEditor.validation.GetMessagesForTask(taskID)
}

// exitPlanEditor cleans up the plan editor state when exiting edit mode
func (m *Model) exitPlanEditor() {
	m.planEditor = nil
}

// getDiffState returns the diff state, initializing it if nil.
// This provides safe access to diff state even for test models created without NewModel.
func (m *Model) getDiffState() *DiffState {
	if m.diffState == nil {
		m.diffState = NewDiffState()
	}
	return m.diffState
}

// NewModel creates a new TUI model
func NewModel(orch *orchestrator.Orchestrator, session *orchestrator.Session) Model {
	return Model{
		orchestrator:     orch,
		session:          session,
		outputManager:    NewOutputManager(),
		taskInput:        NewInputHandler(),
		search:           NewSearchState(),
		commandHandler:   NewCommandHandler(),
		templateHandler:  NewTemplateHandler(),
		diffState:        NewDiffState(),
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
// These methods delegate to OutputManager but provide Model-aware calculations
// (e.g., applying filters, calculating available height based on UI dimensions)

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
	output := m.outputManager.GetOutput(instanceID)
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
	return m.outputManager.IsAutoScroll(instanceID)
}

// scrollOutputUp scrolls the output up by n lines and disables auto-scroll
func (m *Model) scrollOutputUp(instanceID string, n int) {
	m.outputManager.ScrollUp(instanceID, n)
}

// scrollOutputDown scrolls the output down by n lines
func (m *Model) scrollOutputDown(instanceID string, n int) {
	maxScroll := m.getOutputMaxScroll(instanceID)
	m.outputManager.ScrollDown(instanceID, n, maxScroll)
}

// scrollOutputToTop scrolls to the top of the output and disables auto-scroll
func (m *Model) scrollOutputToTop(instanceID string) {
	m.outputManager.ScrollToTop(instanceID)
}

// scrollOutputToBottom scrolls to the bottom and re-enables auto-scroll
func (m *Model) scrollOutputToBottom(instanceID string) {
	maxScroll := m.getOutputMaxScroll(instanceID)
	m.outputManager.ScrollToBottom(instanceID, maxScroll)
}

// updateOutputScroll updates scroll position based on new output (if auto-scroll is enabled)
func (m *Model) updateOutputScroll(instanceID string) {
	maxScroll := m.getOutputMaxScroll(instanceID)
	lineCount := m.getOutputLineCount(instanceID)
	m.outputManager.UpdateForNewOutput(instanceID, maxScroll, lineCount)
}

// hasNewOutput returns true if there's new output since last update
func (m Model) hasNewOutput(instanceID string) bool {
	currentLines := m.getOutputLineCount(instanceID)
	return m.outputManager.HasNewOutput(instanceID, currentLines)
}

