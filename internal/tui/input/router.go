// Package input provides input routing for the TUI.
// It manages mode transitions and routes key events to appropriate handlers.
package input

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Mode represents the current input mode of the TUI.
// The mode determines how keyboard input is processed.
type Mode int

const (
	// ModeNormal is the default mode for navigating instances and views.
	ModeNormal Mode = iota

	// ModeCommand is vim-style ex command mode (triggered by ':').
	ModeCommand

	// ModeSearch is pattern search mode (triggered by '/').
	ModeSearch

	// ModeFilter is output filtering mode (triggered by 'f' or 'F').
	ModeFilter

	// ModeInput forwards keys to the active instance's tmux session.
	ModeInput

	// ModeTerminal forwards keys to the terminal pane's tmux session.
	ModeTerminal

	// ModeTaskInput handles task description entry.
	ModeTaskInput

	// ModePlanEditor handles interactive plan editing.
	ModePlanEditor

	// ModeUltraPlan handles ultra-plan specific controls.
	ModeUltraPlan
)

// String returns the string representation of the mode.
func (m Mode) String() string {
	switch m {
	case ModeNormal:
		return "normal"
	case ModeCommand:
		return "command"
	case ModeSearch:
		return "search"
	case ModeFilter:
		return "filter"
	case ModeInput:
		return "input"
	case ModeTerminal:
		return "terminal"
	case ModeTaskInput:
		return "task-input"
	case ModePlanEditor:
		return "plan-editor"
	case ModeUltraPlan:
		return "ultra-plan"
	default:
		return "unknown"
	}
}

// Result represents the result of routing a key event.
type Result struct {
	// Handled indicates whether the key was processed.
	Handled bool

	// Cmd is an optional tea.Cmd to execute.
	Cmd tea.Cmd

	// NextMode indicates a mode transition (nil means no change).
	NextMode *Mode

	// ClearBuffer indicates the input buffer should be cleared.
	ClearBuffer bool
}

// NewResult creates a result indicating the key was handled.
func NewResult() Result {
	return Result{Handled: true}
}

// WithCmd adds a command to the result.
func (r Result) WithCmd(cmd tea.Cmd) Result {
	r.Cmd = cmd
	return r
}

// WithModeChange indicates a mode transition.
func (r Result) WithModeChange(mode Mode) Result {
	r.NextMode = &mode
	return r
}

// WithBufferClear indicates the buffer should be cleared.
func (r Result) WithBufferClear() Result {
	r.ClearBuffer = true
	return r
}

// NotHandled returns a result indicating the key was not handled.
func NotHandled() Result {
	return Result{Handled: false}
}

// Handler processes key events for a specific mode.
type Handler interface {
	// HandleKey processes a key event and returns the result.
	// The handler should not modify any state directly; instead, it returns
	// a Result that indicates what action should be taken.
	HandleKey(msg tea.KeyMsg) Result
}

// HandlerFunc is an adapter to allow ordinary functions to be used as handlers.
type HandlerFunc func(tea.KeyMsg) Result

// HandleKey calls f(msg).
func (f HandlerFunc) HandleKey(msg tea.KeyMsg) Result {
	return f(msg)
}

// Router manages input mode and routes key events to appropriate handlers.
type Router struct {
	mode     Mode
	handlers map[Mode]Handler

	// Buffer holds text being typed in command/search modes.
	Buffer string

	// Submode tracking for complex modes
	ultraPlanActive   bool
	planEditorActive  bool
	templateDropdown  bool
	groupDecisionMode bool
	retriggerMode     bool
}

// NewRouter creates a new input router in normal mode.
func NewRouter() *Router {
	return &Router{
		mode:     ModeNormal,
		handlers: make(map[Mode]Handler),
	}
}

// Mode returns the current input mode.
func (r *Router) Mode() Mode {
	return r.mode
}

// SetMode changes the current input mode.
func (r *Router) SetMode(mode Mode) {
	r.mode = mode
}

// RegisterHandler registers a handler for a specific mode.
func (r *Router) RegisterHandler(mode Mode, handler Handler) {
	r.handlers[mode] = handler
}

// RegisterHandlerFunc registers a handler function for a specific mode.
func (r *Router) RegisterHandlerFunc(mode Mode, f func(tea.KeyMsg) Result) {
	r.handlers[mode] = HandlerFunc(f)
}

// SetUltraPlanActive sets whether ultra-plan mode is active.
func (r *Router) SetUltraPlanActive(active bool) {
	r.ultraPlanActive = active
}

// IsUltraPlanActive returns whether ultra-plan mode is active.
func (r *Router) IsUltraPlanActive() bool {
	return r.ultraPlanActive
}

// SetPlanEditorActive sets whether plan editor mode is active.
func (r *Router) SetPlanEditorActive(active bool) {
	r.planEditorActive = active
}

// IsPlanEditorActive returns whether plan editor mode is active.
func (r *Router) IsPlanEditorActive() bool {
	return r.planEditorActive
}

// SetTemplateDropdown sets whether the template dropdown is visible.
func (r *Router) SetTemplateDropdown(visible bool) {
	r.templateDropdown = visible
}

// IsTemplateDropdownVisible returns whether the template dropdown is visible.
func (r *Router) IsTemplateDropdownVisible() bool {
	return r.templateDropdown
}

// SetGroupDecisionMode sets whether we're awaiting a group decision.
func (r *Router) SetGroupDecisionMode(active bool) {
	r.groupDecisionMode = active
}

// IsGroupDecisionMode returns whether we're awaiting a group decision.
func (r *Router) IsGroupDecisionMode() bool {
	return r.groupDecisionMode
}

// SetRetriggerMode sets whether we're in retrigger mode.
func (r *Router) SetRetriggerMode(active bool) {
	r.retriggerMode = active
}

// IsRetriggerMode returns whether we're in retrigger mode.
func (r *Router) IsRetriggerMode() bool {
	return r.retriggerMode
}

// ClearBuffer clears the input buffer.
func (r *Router) ClearBuffer() {
	r.Buffer = ""
}

// AppendToBuffer appends text to the input buffer.
func (r *Router) AppendToBuffer(text string) {
	r.Buffer += text
}

// DeleteFromBuffer removes the last character from the buffer.
// Returns true if a character was deleted.
func (r *Router) DeleteFromBuffer() bool {
	if len(r.Buffer) > 0 {
		r.Buffer = r.Buffer[:len(r.Buffer)-1]
		return true
	}
	return false
}

// Route processes a key event and returns the routing result.
// It delegates to the appropriate handler based on the current mode.
func (r *Router) Route(msg tea.KeyMsg) Result {
	// Determine effective mode based on state
	effectiveMode := r.effectiveMode()

	// Get handler for the effective mode
	handler, exists := r.handlers[effectiveMode]
	if !exists {
		return NotHandled()
	}

	// Delegate to the handler
	result := handler.HandleKey(msg)

	// Apply mode transitions
	if result.NextMode != nil {
		r.mode = *result.NextMode
	}

	// Clear buffer if requested
	if result.ClearBuffer {
		r.Buffer = ""
	}

	return result
}

// effectiveMode determines the actual mode to use for routing.
// This handles the priority ordering of modes and submodes.
func (r *Router) effectiveMode() Mode {
	// Priority order matches app.go handleKeypress
	switch r.mode {
	case ModeSearch:
		return ModeSearch
	case ModeFilter:
		return ModeFilter
	case ModeInput:
		return ModeInput
	case ModeTerminal:
		return ModeTerminal
	case ModeTaskInput:
		return ModeTaskInput
	case ModeCommand:
		return ModeCommand
	default:
		// For normal mode, check for active special modes
		if r.planEditorActive {
			return ModePlanEditor
		}
		if r.ultraPlanActive {
			return ModeUltraPlan
		}
		return ModeNormal
	}
}

// ShouldExitModeOnEscape returns true if the current mode should exit on Escape.
func (r *Router) ShouldExitModeOnEscape() bool {
	switch r.mode {
	case ModeCommand, ModeSearch, ModeFilter, ModeTaskInput:
		return true
	default:
		return false
	}
}

// ShouldExitModeOnCtrlBracket returns true if the current mode exits on Ctrl+].
func (r *Router) ShouldExitModeOnCtrlBracket() bool {
	switch r.mode {
	case ModeInput, ModeTerminal:
		return true
	default:
		return false
	}
}

// IsBufferedMode returns true if the current mode uses a text buffer.
func (r *Router) IsBufferedMode() bool {
	switch r.mode {
	case ModeCommand, ModeSearch, ModeFilter, ModeTaskInput:
		return true
	default:
		return false
	}
}

// IsForwardingMode returns true if keys should be forwarded to tmux.
func (r *Router) IsForwardingMode() bool {
	switch r.mode {
	case ModeInput, ModeTerminal:
		return true
	default:
		return false
	}
}

// TransitionToNormal transitions back to normal mode.
func (r *Router) TransitionToNormal() {
	r.mode = ModeNormal
	r.Buffer = ""
}

// TransitionToCommand enters command mode.
func (r *Router) TransitionToCommand() {
	r.mode = ModeCommand
	r.Buffer = ""
}

// TransitionToSearch enters search mode.
func (r *Router) TransitionToSearch() {
	r.mode = ModeSearch
	r.Buffer = ""
}

// TransitionToFilter enters filter mode.
func (r *Router) TransitionToFilter() {
	r.mode = ModeFilter
}

// TransitionToInput enters input mode (tmux forwarding).
func (r *Router) TransitionToInput() {
	r.mode = ModeInput
}

// TransitionToTerminal enters terminal mode (terminal pane forwarding).
func (r *Router) TransitionToTerminal() {
	r.mode = ModeTerminal
}

// TransitionToTaskInput enters task input mode.
func (r *Router) TransitionToTaskInput() {
	r.mode = ModeTaskInput
	r.Buffer = ""
}
