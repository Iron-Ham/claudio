package tui

import (
	"regexp"
	"time"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/command"
	"github.com/Iron-Ham/claudio/internal/tui/input"
	"github.com/Iron-Ham/claudio/internal/tui/output"
	"github.com/Iron-Ham/claudio/internal/tui/search"
	"github.com/Iron-Ham/claudio/internal/tui/terminal"
)

// TerminalDirMode indicates which directory the terminal pane is using.
// Deprecated: Use terminal.DirMode instead.
type TerminalDirMode = terminal.DirMode

// Terminal directory mode constants.
// Deprecated: Use terminal.DirInvocation and terminal.DirWorktree instead.
const (
	TerminalDirInvocation = terminal.DirInvocation
	TerminalDirWorktree   = terminal.DirWorktree
)

// modelInstanceProvider adapts the Model to the terminal.ActiveInstanceProvider interface.
type modelInstanceProvider struct {
	model *Model
}

// WorktreePath returns the worktree path of the active instance.
func (p modelInstanceProvider) WorktreePath() string {
	if inst := p.model.activeInstance(); inst != nil {
		return inst.WorktreePath
	}
	return ""
}

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
	orchestrator   *orchestrator.Orchestrator
	session        *orchestrator.Session
	logger         *logging.Logger
	startTime      time.Time        // Time when the TUI session started
	commandHandler *command.Handler // Handler for vim-style commands

	// Input routing
	inputRouter *input.Router

	// Terminal pane manager (owns dimensions and layout calculations)
	terminalManager *terminal.Manager

	// Ultra-plan mode (nil if not in ultra-plan mode)
	ultraPlan *UltraPlanState

	// Plan editor mode (nil if not in plan editor mode)
	planEditor *PlanEditorState

	// UI state
	activeTab       int
	ready           bool
	quitting        bool
	showHelp        bool
	helpScroll      int  // Scroll offset for help panel
	showConflicts   bool // When true, show detailed conflict view
	addingTask      bool
	taskInput       string
	taskInputCursor int // Cursor position within taskInput (0 = before first char)

	// Dependent task state (for :chain command - adding a task that depends on another)
	addingDependentTask   bool   // When true, taskInput will create a dependent task
	dependentOnInstanceID string // The instance ID that the new task will depend on
	errorMessage          string
	infoMessage           string // Non-error status message
	inputMode             bool   // When true, all keys are forwarded to the active instance's tmux session

	// Command mode state (vim-style ex commands with ':' prefix)
	commandMode   bool   // When true, we're typing a command after ':'
	commandBuffer string // The command being typed (without the ':' prefix)

	// Template dropdown state
	showTemplates    bool   // Whether the template dropdown is visible
	templateFilter   string // Current filter text (after the "/")
	templateSelected int    // Currently highlighted template index
	templateSuffix   string // Suffix to append on submission (from selected template)

	// Branch selection state (for selecting base branch when adding task)
	showBranchSelector bool     // Whether the branch selector is visible
	branchList         []string // Cached list of branch names
	branchSelected     int      // Currently highlighted branch index (0 = main branch)
	selectedBaseBranch string   // The selected base branch for the new instance

	// File conflict tracking
	conflicts []conflict.FileConflict

	// Output management (handles per-instance output buffers, scrolling, and auto-scroll)
	outputManager *output.Manager

	// Diff preview state
	showDiff    bool   // Whether the diff panel is visible
	diffContent string // Cached diff content for the active instance
	diffScroll  int    // Scroll offset for navigating the diff

	// Sidebar pagination
	sidebarScrollOffset int // Index of the first visible instance in sidebar

	// Resource metrics display
	showStats bool // When true, show the stats panel

	// Search state
	searchMode   bool           // Whether search mode is active (typing pattern)
	searchInput  string         // Current search input being typed (live updated)
	searchEngine *search.Engine // Search engine for output buffer searching

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
	if m.planEditor == nil || m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return
	}

	session := m.ultraPlan.Coordinator.Session()
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

	session := m.ultraPlan.Coordinator.Session()
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

// NewModel creates a new TUI model
func NewModel(orch *orchestrator.Orchestrator, session *orchestrator.Session, logger *logging.Logger) Model {
	// Create a TUI-specific logger with phase context
	var tuiLogger *logging.Logger
	if logger != nil {
		tuiLogger = logger.WithPhase("tui")
	}

	// Get invocation directory from orchestrator
	invocationDir := ""
	if orch != nil {
		invocationDir = orch.BaseDir()
	}

	// Create terminal manager with configuration
	termMgr := terminal.NewManagerWithConfig(terminal.ManagerConfig{
		InvocationDir: invocationDir,
		Logger:        tuiLogger,
	})

	return Model{
		orchestrator:    orch,
		session:         session,
		logger:          tuiLogger,
		startTime:       time.Now(),
		commandHandler:  command.New(),
		inputRouter:     input.NewRouter(),
		outputManager:   output.NewManager(),
		searchEngine:    search.NewEngine(),
		terminalManager: termMgr,
		filterCategories: map[string]bool{
			"errors":   true,
			"warnings": true,
			"tools":    true,
			"thinking": true,
			"progress": true,
		},
	}
}

// InputRouter returns the input router for this model.
func (m Model) InputRouter() *input.Router {
	return m.inputRouter
}

// syncRouterState synchronizes the InputRouter state with the Model's mode flags.
// This ensures the router reflects the current mode based on the existing boolean flags.
func (m *Model) syncRouterState() {
	if m.inputRouter == nil {
		return
	}

	// Sync mode based on priority order (matching handleKeypress)
	switch {
	case m.searchMode:
		m.inputRouter.SetMode(input.ModeSearch)
	case m.filterMode:
		m.inputRouter.SetMode(input.ModeFilter)
	case m.inputMode:
		m.inputRouter.SetMode(input.ModeInput)
	case m.terminalManager.IsFocused():
		m.inputRouter.SetMode(input.ModeTerminal)
	case m.addingTask:
		m.inputRouter.SetMode(input.ModeTaskInput)
	case m.commandMode:
		m.inputRouter.SetMode(input.ModeCommand)
	default:
		m.inputRouter.SetMode(input.ModeNormal)
	}

	// Sync submode states
	m.inputRouter.SetUltraPlanActive(m.ultraPlan != nil)
	m.inputRouter.SetPlanEditorActive(m.IsPlanEditorActive())
	m.inputRouter.SetTemplateDropdown(m.showTemplates)

	// Sync group decision and retrigger modes from ultra-plan state
	if m.ultraPlan != nil && m.ultraPlan.Coordinator != nil {
		session := m.ultraPlan.Coordinator.Session()
		if session != nil && session.GroupDecision != nil {
			m.inputRouter.SetGroupDecisionMode(session.GroupDecision.AwaitingDecision)
		} else {
			m.inputRouter.SetGroupDecisionMode(false)
		}
		m.inputRouter.SetRetriggerMode(m.ultraPlan.RetriggerMode)
	} else {
		m.inputRouter.SetGroupDecisionMode(false)
		m.inputRouter.SetRetriggerMode(false)
	}

	// Sync command buffer
	m.inputRouter.Buffer = m.commandBuffer
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
	dims := m.terminalManager.GetPaneDimensions(m.calculateExtraFooterLines())
	availableSlots := dims.MainAreaHeight - reservedLines
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
// These methods delegate to the OutputManager for output buffer management.

// getOutputMaxLines returns the maximum number of lines visible in the output area
func (m Model) getOutputMaxLines() int {
	dims := m.terminalManager.GetPaneDimensions(m.calculateExtraFooterLines())
	// Output area is within main area, minus some reserved lines for header/status
	maxLines := dims.MainAreaHeight - 6
	if maxLines < 5 {
		maxLines = 5
	}
	return maxLines
}

// isOutputAutoScroll returns whether auto-scroll is enabled for an instance (defaults to true)
func (m Model) isOutputAutoScroll(instanceID string) bool {
	return m.outputManager.IsAutoScroll(instanceID)
}

// scrollOutputUp scrolls the output up by n lines and disables auto-scroll
func (m *Model) scrollOutputUp(instanceID string, n int) {
	m.outputManager.SetFilterFunc(m.filterOutput)
	m.outputManager.Scroll(instanceID, -n, m.getOutputMaxLines())
}

// scrollOutputDown scrolls the output down by n lines
func (m *Model) scrollOutputDown(instanceID string, n int) {
	m.outputManager.SetFilterFunc(m.filterOutput)
	m.outputManager.Scroll(instanceID, n, m.getOutputMaxLines())
}

// scrollOutputToTop scrolls to the top of the output and disables auto-scroll
func (m *Model) scrollOutputToTop(instanceID string) {
	m.outputManager.ScrollToTop(instanceID)
}

// scrollOutputToBottom scrolls to the bottom and re-enables auto-scroll
func (m *Model) scrollOutputToBottom(instanceID string) {
	m.outputManager.SetFilterFunc(m.filterOutput)
	m.outputManager.ScrollToBottom(instanceID, m.getOutputMaxLines())
}

// updateOutputScroll updates scroll position based on new output (if auto-scroll is enabled)
func (m *Model) updateOutputScroll(instanceID string) {
	m.outputManager.SetFilterFunc(m.filterOutput)
	m.outputManager.UpdateScroll(instanceID, m.getOutputMaxLines())
}

// hasNewOutput returns true if there's new output since last update
func (m Model) hasNewOutput(instanceID string) bool {
	return m.outputManager.HasNewOutput(instanceID)
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

// -----------------------------------------------------------------------------
// Terminal pane helper methods
// -----------------------------------------------------------------------------

// DefaultTerminalHeight is the default height of the terminal pane in lines.
// Set to 15 to provide a more useful terminal display showing adequate
// command output and shell history.
// Deprecated: Use terminal.DefaultPaneHeight instead.
const DefaultTerminalHeight = terminal.DefaultPaneHeight

// MinTerminalHeight is the minimum height of the terminal pane.
// Deprecated: Use terminal.MinPaneHeight instead.
const MinTerminalHeight = terminal.MinPaneHeight

// MaxTerminalHeightRatio is the maximum ratio of terminal height to total height.
// Deprecated: Use terminal.MaxPaneHeightRatio instead.
const MaxTerminalHeightRatio = terminal.MaxPaneHeightRatio

// IsTerminalMode returns true if the terminal pane has input focus.
func (m Model) IsTerminalMode() bool {
	return m.terminalManager.IsFocused()
}

// IsTerminalVisible returns true if the terminal pane is visible.
func (m Model) IsTerminalVisible() bool {
	return m.terminalManager.IsVisible()
}

// TerminalPaneHeight returns the current terminal pane height (0 if hidden).
func (m Model) TerminalPaneHeight() int {
	dims := m.terminalManager.GetPaneDimensions(0)
	return dims.TerminalPaneHeight
}

// getTerminalDir returns the directory path for the terminal based on current mode.
func (m Model) getTerminalDir() string {
	return m.terminalManager.GetDir(modelInstanceProvider{model: &m})
}

// toggleTerminalVisibility toggles the terminal pane on or off.
// If turning on and process doesn't exist, it will be created lazily.
func (m *Model) toggleTerminalVisibility(sessionID string) {
	errMsg, warnMsg := m.terminalManager.Toggle(sessionID)
	if errMsg != "" {
		m.errorMessage = errMsg
	} else if warnMsg != "" {
		m.infoMessage = warnMsg
	}
}

// enterTerminalMode enters terminal input mode (keys go to terminal).
func (m *Model) enterTerminalMode() {
	m.terminalManager.EnterMode()
}

// exitTerminalMode exits terminal input mode.
func (m *Model) exitTerminalMode() {
	m.terminalManager.ExitMode()
}

// switchTerminalDir toggles between worktree and invocation directory modes.
func (m *Model) switchTerminalDir() {
	infoMsg, errMsg := m.terminalManager.SwitchDir(modelInstanceProvider{model: m})
	if errMsg != "" {
		m.errorMessage = errMsg
	} else if infoMsg != "" {
		m.infoMessage = infoMsg
	}
}

// updateTerminalOutput captures current terminal output.
func (m *Model) updateTerminalOutput() {
	m.terminalManager.UpdateOutput()
}

// resizeTerminal updates the terminal dimensions.
func (m *Model) resizeTerminal() {
	m.terminalManager.Resize()
}

// cleanupTerminal stops the terminal process (called on quit).
func (m *Model) cleanupTerminal() {
	m.terminalManager.Cleanup()
}

// updateTerminalOnInstanceChange updates terminal directory if in worktree mode.
// Called when the active instance changes.
func (m *Model) updateTerminalOnInstanceChange() {
	if errMsg := m.terminalManager.UpdateOnInstanceChange(modelInstanceProvider{model: m}); errMsg != "" {
		m.errorMessage = errMsg
	}
}

// -----------------------------------------------------------------------------
// DashboardState interface implementation
// These methods implement the view.DashboardState interface, allowing the Model
// to be passed to view components for rendering.
// -----------------------------------------------------------------------------

// Session returns the current orchestrator session.
func (m Model) Session() *orchestrator.Session {
	return m.session
}

// ActiveTab returns the index of the currently selected instance.
func (m Model) ActiveTab() int {
	return m.activeTab
}

// SidebarScrollOffset returns the scroll offset for the sidebar.
func (m Model) SidebarScrollOffset() int {
	return m.sidebarScrollOffset
}

// Conflicts returns the current file conflicts.
func (m Model) Conflicts() []conflict.FileConflict {
	return m.conflicts
}

// TerminalWidth returns the terminal width.
func (m Model) TerminalWidth() int {
	return m.terminalManager.Width()
}

// TerminalHeight returns the terminal height.
func (m Model) TerminalHeight() int {
	return m.terminalManager.Height()
}

// IsAddingTask returns whether the user is currently adding a new task
func (m Model) IsAddingTask() bool {
	return m.addingTask
}

// -----------------------------------------------------------------------------
// command.Dependencies interface implementation
// These methods implement the command.Dependencies interface, allowing the Model
// to be passed to the CommandHandler for command execution.
// -----------------------------------------------------------------------------

// GetOrchestrator returns the orchestrator instance.
func (m Model) GetOrchestrator() *orchestrator.Orchestrator {
	return m.orchestrator
}

// GetSession returns the current session.
func (m Model) GetSession() *orchestrator.Session {
	return m.session
}

// ActiveInstance returns the currently focused instance.
func (m Model) ActiveInstance() *orchestrator.Instance {
	return m.activeInstance()
}

// InstanceCount returns the number of instances.
func (m Model) InstanceCount() int {
	return m.instanceCount()
}

// GetConflicts returns the number of file conflicts.
func (m Model) GetConflicts() int {
	return len(m.conflicts)
}

// IsDiffVisible returns true if the diff panel is visible.
func (m Model) IsDiffVisible() bool {
	return m.showDiff
}

// GetDiffContent returns the current diff content.
func (m Model) GetDiffContent() string {
	return m.diffContent
}

// GetUltraPlanCoordinator returns the ultraplan coordinator if in ultraplan mode.
func (m Model) GetUltraPlanCoordinator() *orchestrator.Coordinator {
	if m.ultraPlan == nil {
		return nil
	}
	return m.ultraPlan.Coordinator
}

// GetLogger returns the logger instance.
func (m Model) GetLogger() *logging.Logger {
	return m.logger
}

// GetStartTime returns the TUI session start time.
func (m Model) GetStartTime() time.Time {
	return m.startTime
}
