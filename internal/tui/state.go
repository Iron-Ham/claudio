package tui

import (
	"regexp"

	"github.com/Iron-Ham/claudio/internal/conflict"
)

// InputState holds all user input-related state for the TUI.
// This includes task input, command mode, and cursor/editing state.
type InputState struct {
	// Task input state
	AddingTask      bool   // When true, the user is typing a new task
	TaskInput       string // The current task input text
	TaskInputCursor int    // Cursor position within TaskInput (0 = before first char)

	// Command mode state (vim-style ex commands with ':' prefix)
	CommandMode   bool   // When true, we're typing a command after ':'
	CommandBuffer string // The command being typed (without the ':' prefix)

	// Input mode state (forwarding keys to tmux)
	InputMode bool // When true, all keys are forwarded to the active instance's tmux session

	// Template dropdown state (shown when typing "/" in task input)
	ShowTemplates    bool   // Whether the template dropdown is visible
	TemplateFilter   string // Current filter text (after the "/")
	TemplateSelected int    // Currently highlighted template index
}

// NewInputState creates a new InputState with default values.
func NewInputState() InputState {
	return InputState{}
}

// Reset clears the input state to its default values.
func (s *InputState) Reset() {
	s.AddingTask = false
	s.TaskInput = ""
	s.TaskInputCursor = 0
	s.CommandMode = false
	s.CommandBuffer = ""
	s.InputMode = false
	s.ShowTemplates = false
	s.TemplateFilter = ""
	s.TemplateSelected = 0
}

// EnterCommandMode transitions into vim-style command mode.
func (s *InputState) EnterCommandMode() {
	s.CommandMode = true
	s.CommandBuffer = ""
}

// ExitCommandMode leaves command mode and clears the buffer.
func (s *InputState) ExitCommandMode() {
	s.CommandMode = false
	s.CommandBuffer = ""
}

// EnterTaskInput transitions into task input mode.
func (s *InputState) EnterTaskInput() {
	s.AddingTask = true
	s.TaskInput = ""
	s.TaskInputCursor = 0
}

// ExitTaskInput leaves task input mode and clears the input.
func (s *InputState) ExitTaskInput() {
	s.AddingTask = false
	s.TaskInput = ""
	s.TaskInputCursor = 0
	s.ShowTemplates = false
	s.TemplateFilter = ""
	s.TemplateSelected = 0
}

// EnterInputMode transitions into input forwarding mode (tmux passthrough).
func (s *InputState) EnterInputMode() {
	s.InputMode = true
}

// ExitInputMode leaves input forwarding mode.
func (s *InputState) ExitInputMode() {
	s.InputMode = false
}

// ShowTemplateDropdown shows the template dropdown menu.
func (s *InputState) ShowTemplateDropdown() {
	s.ShowTemplates = true
	s.TemplateFilter = ""
	s.TemplateSelected = 0
}

// HideTemplateDropdown hides the template dropdown menu.
func (s *InputState) HideTemplateDropdown() {
	s.ShowTemplates = false
	s.TemplateFilter = ""
	s.TemplateSelected = 0
}

// ViewState holds boolean flags that control what UI elements are visible.
// These represent view toggles and mode switches.
type ViewState struct {
	// Panel visibility flags
	ShowHelp      bool // When true, show the help panel
	ShowConflicts bool // When true, show detailed conflict view
	ShowDiff      bool // Whether the diff panel is visible
	ShowStats     bool // When true, show the stats panel

	// Mode flags
	SearchMode bool // Whether search mode is active (typing pattern)
	FilterMode bool // Whether filter mode is active
}

// NewViewState creates a new ViewState with default values.
func NewViewState() ViewState {
	return ViewState{}
}

// Reset clears the view state to its default values.
func (s *ViewState) Reset() {
	s.ShowHelp = false
	s.ShowConflicts = false
	s.ShowDiff = false
	s.ShowStats = false
	s.SearchMode = false
	s.FilterMode = false
}

// ToggleHelp toggles the help panel visibility.
func (s *ViewState) ToggleHelp() {
	s.ShowHelp = !s.ShowHelp
}

// ToggleConflicts toggles the conflict view visibility.
func (s *ViewState) ToggleConflicts() {
	s.ShowConflicts = !s.ShowConflicts
}

// ToggleDiff toggles the diff panel visibility.
func (s *ViewState) ToggleDiff() {
	s.ShowDiff = !s.ShowDiff
}

// ToggleStats toggles the stats panel visibility.
func (s *ViewState) ToggleStats() {
	s.ShowStats = !s.ShowStats
}

// EnterSearchMode transitions into search mode.
func (s *ViewState) EnterSearchMode() {
	s.SearchMode = true
}

// ExitSearchMode leaves search mode.
func (s *ViewState) ExitSearchMode() {
	s.SearchMode = false
}

// EnterFilterMode transitions into filter mode.
func (s *ViewState) EnterFilterMode() {
	s.FilterMode = true
}

// ExitFilterMode leaves filter mode.
func (s *ViewState) ExitFilterMode() {
	s.FilterMode = false
}

// NavigationState holds state related to UI navigation and scrolling.
type NavigationState struct {
	// Tab/instance navigation
	ActiveTab int // Index of the currently focused instance

	// Sidebar pagination
	SidebarScrollOffset int // Index of the first visible instance in sidebar

	// Output scroll state (per instance)
	OutputScrolls    map[string]int  // Instance ID -> scroll offset
	OutputAutoScroll map[string]bool // Instance ID -> auto-scroll enabled (follows new output)
	OutputLineCount  map[string]int  // Instance ID -> previous line count (to detect new output)

	// Diff scroll state
	DiffScroll int // Scroll offset for navigating the diff

	// Search navigation
	SearchPattern string         // Current search pattern
	SearchRegex   *regexp.Regexp // Compiled regex (nil for literal search)
	SearchMatches []int          // Line numbers containing matches
	SearchCurrent int            // Current match index (for n/N navigation)
	OutputScroll  int            // Scroll position in output (for search navigation)

	// Filter state
	FilterCategories map[string]bool // Which categories are enabled
	FilterCustom     string          // Custom filter pattern
	FilterRegex      *regexp.Regexp  // Compiled custom filter regex
}

// NewNavigationState creates a new NavigationState with default values.
func NewNavigationState() NavigationState {
	return NavigationState{
		OutputScrolls:    make(map[string]int),
		OutputAutoScroll: make(map[string]bool),
		OutputLineCount:  make(map[string]int),
		FilterCategories: map[string]bool{
			"errors":   true,
			"warnings": true,
			"tools":    true,
			"thinking": true,
			"progress": true,
		},
	}
}

// Reset clears the navigation state to its default values.
func (s *NavigationState) Reset() {
	s.ActiveTab = 0
	s.SidebarScrollOffset = 0
	s.OutputScrolls = make(map[string]int)
	s.OutputAutoScroll = make(map[string]bool)
	s.OutputLineCount = make(map[string]int)
	s.DiffScroll = 0
	s.ClearSearch()
	s.ResetFilter()
}

// ClearSearch resets all search-related state.
func (s *NavigationState) ClearSearch() {
	s.SearchPattern = ""
	s.SearchRegex = nil
	s.SearchMatches = nil
	s.SearchCurrent = 0
	s.OutputScroll = 0
}

// ResetFilter resets the filter to its default state.
func (s *NavigationState) ResetFilter() {
	s.FilterCategories = map[string]bool{
		"errors":   true,
		"warnings": true,
		"tools":    true,
		"thinking": true,
		"progress": true,
	}
	s.FilterCustom = ""
	s.FilterRegex = nil
}

// NextSearchMatch moves to the next search match.
func (s *NavigationState) NextSearchMatch() {
	if len(s.SearchMatches) == 0 {
		return
	}
	s.SearchCurrent = (s.SearchCurrent + 1) % len(s.SearchMatches)
}

// PrevSearchMatch moves to the previous search match.
func (s *NavigationState) PrevSearchMatch() {
	if len(s.SearchMatches) == 0 {
		return
	}
	s.SearchCurrent--
	if s.SearchCurrent < 0 {
		s.SearchCurrent = len(s.SearchMatches) - 1
	}
}

// SearchTotal returns the total number of search matches.
func (s *NavigationState) SearchTotal() int {
	return len(s.SearchMatches)
}

// IsOutputAutoScroll returns whether auto-scroll is enabled for an instance (defaults to true).
func (s *NavigationState) IsOutputAutoScroll(instanceID string) bool {
	if autoScroll, exists := s.OutputAutoScroll[instanceID]; exists {
		return autoScroll
	}
	return true // Default to auto-scroll enabled
}

// SetOutputScroll sets the scroll position for an instance's output.
func (s *NavigationState) SetOutputScroll(instanceID string, scroll int) {
	s.OutputScrolls[instanceID] = scroll
}

// GetOutputScroll returns the scroll position for an instance's output.
func (s *NavigationState) GetOutputScroll(instanceID string) int {
	return s.OutputScrolls[instanceID]
}

// EnableAutoScroll enables auto-scroll for an instance.
func (s *NavigationState) EnableAutoScroll(instanceID string) {
	s.OutputAutoScroll[instanceID] = true
}

// DisableAutoScroll disables auto-scroll for an instance.
func (s *NavigationState) DisableAutoScroll(instanceID string) {
	s.OutputAutoScroll[instanceID] = false
}

// DataState holds the data being displayed in the TUI.
// This includes instance outputs and file conflicts.
type DataState struct {
	// Instance outputs (instance ID -> output string)
	Outputs map[string]string

	// File conflict tracking
	Conflicts []conflict.FileConflict

	// Diff content cache
	DiffContent string // Cached diff content for the active instance
}

// NewDataState creates a new DataState with default values.
func NewDataState() DataState {
	return DataState{
		Outputs: make(map[string]string),
	}
}

// Reset clears the data state to its default values.
func (s *DataState) Reset() {
	s.Outputs = make(map[string]string)
	s.Conflicts = nil
	s.DiffContent = ""
}

// GetOutput returns the output for an instance.
func (s *DataState) GetOutput(instanceID string) string {
	return s.Outputs[instanceID]
}

// SetOutput sets the output for an instance.
func (s *DataState) SetOutput(instanceID string, output string) {
	s.Outputs[instanceID] = output
}

// AppendOutput appends data to an instance's output.
func (s *DataState) AppendOutput(instanceID string, data string) {
	s.Outputs[instanceID] += data
}

// ClearOutput clears the output for an instance.
func (s *DataState) ClearOutput(instanceID string) {
	delete(s.Outputs, instanceID)
}

// HasConflicts returns true if there are any file conflicts.
func (s *DataState) HasConflicts() bool {
	return len(s.Conflicts) > 0
}

// ConflictCount returns the number of file conflicts.
func (s *DataState) ConflictCount() int {
	return len(s.Conflicts)
}

// SetConflicts sets the file conflicts.
func (s *DataState) SetConflicts(conflicts []conflict.FileConflict) {
	s.Conflicts = conflicts
}

// ClearConflicts removes all file conflicts.
func (s *DataState) ClearConflicts() {
	s.Conflicts = nil
}

// UIState holds layout and dimension state for the TUI.
// This is separated from other state as it's primarily about rendering concerns.
type UIState struct {
	Width    int  // Terminal width
	Height   int  // Terminal height
	Ready    bool // Whether the TUI has received initial dimensions
	Quitting bool // Whether the TUI is quitting
}

// NewUIState creates a new UIState with default values.
func NewUIState() UIState {
	return UIState{}
}

// SetDimensions updates the terminal dimensions and marks the UI as ready.
func (s *UIState) SetDimensions(width, height int) {
	s.Width = width
	s.Height = height
	s.Ready = true
}

// IsReady returns whether the UI has been initialized with dimensions.
func (s *UIState) IsReady() bool {
	return s.Ready
}

// StartQuitting marks the UI as quitting.
func (s *UIState) StartQuitting() {
	s.Quitting = true
}

// MessageState holds transient message state for displaying errors and info.
type MessageState struct {
	ErrorMessage string // Error message to display
	InfoMessage  string // Non-error status message
}

// NewMessageState creates a new MessageState with default values.
func NewMessageState() MessageState {
	return MessageState{}
}

// SetError sets an error message.
func (s *MessageState) SetError(msg string) {
	s.ErrorMessage = msg
}

// SetInfo sets an info message.
func (s *MessageState) SetInfo(msg string) {
	s.InfoMessage = msg
}

// ClearError clears the error message.
func (s *MessageState) ClearError() {
	s.ErrorMessage = ""
}

// ClearInfo clears the info message.
func (s *MessageState) ClearInfo() {
	s.InfoMessage = ""
}

// Clear clears both error and info messages.
func (s *MessageState) Clear() {
	s.ErrorMessage = ""
	s.InfoMessage = ""
}

// HasError returns true if there is an error message.
func (s *MessageState) HasError() bool {
	return s.ErrorMessage != ""
}

// HasInfo returns true if there is an info message.
func (s *MessageState) HasInfo() bool {
	return s.InfoMessage != ""
}
