// Package search provides search functionality for the TUI output buffers.
// This file contains UI handlers that work with the search Engine.
package search

// Context provides the interface for handler dependencies.
// This allows the search handlers to interact with the TUI state without
// importing the tui package directly, avoiding circular dependencies.
type Context interface {
	// GetSearchInput returns the current search input string being typed.
	GetSearchInput() string

	// SetSearchInput sets the search input string.
	SetSearchInput(input string)

	// GetSearchEngine returns the search engine for pattern matching.
	GetSearchEngine() *Engine

	// GetOutputForActiveInstance returns the output content for the active instance.
	// Returns empty string if no active instance or no output.
	GetOutputForActiveInstance() string

	// GetViewportHeight returns the height of the output viewport (visible lines).
	GetViewportHeight() int

	// GetOutputScroll returns the current output scroll position (line number).
	GetOutputScroll() int

	// SetOutputScroll sets the output scroll position (line number).
	SetOutputScroll(scroll int)
}

// Handler provides search-related UI handling methods.
// It coordinates between user input, the search Engine, and the TUI state.
type Handler struct {
	ctx Context
}

// NewHandler creates a new search Handler with the given context.
func NewHandler(ctx Context) *Handler {
	return &Handler{ctx: ctx}
}

// HandleInput processes keyboard input during search mode.
// Returns true if the input was handled, false otherwise.
type InputAction int

const (
	// ActionNone indicates no special action is needed.
	ActionNone InputAction = iota
	// ActionEscape indicates search mode should be exited (without clearing).
	ActionEscape
	// ActionConfirm indicates search was confirmed and mode should be exited.
	ActionConfirm
)

// HandleBackspace handles backspace key press during search input.
// It removes the last character and triggers live search.
func (h *Handler) HandleBackspace() {
	input := h.ctx.GetSearchInput()
	if len(input) > 0 {
		h.ctx.SetSearchInput(input[:len(input)-1])
		h.Execute()
	}
}

// HandleRunes handles character input during search.
// It appends the runes to the search input and triggers live search.
func (h *Handler) HandleRunes(runes string) {
	h.ctx.SetSearchInput(h.ctx.GetSearchInput() + runes)
	h.Execute()
}

// Execute performs the search using the current input.
// It searches the active instance's output and scrolls to the first match.
func (h *Handler) Execute() {
	input := h.ctx.GetSearchInput()
	engine := h.ctx.GetSearchEngine()

	if input == "" {
		engine.Clear()
		return
	}

	output := h.ctx.GetOutputForActiveInstance()
	if output == "" {
		return
	}

	// Execute search using the search engine
	engine.Search(input, output)

	// Scroll to first match if any
	if engine.HasMatches() {
		h.ScrollToMatch()
	}
}

// Clear resets all search state including input and engine state.
func (h *Handler) Clear() {
	h.ctx.SetSearchInput("")
	h.ctx.GetSearchEngine().Clear()
	h.ctx.SetOutputScroll(0)
}

// ScrollToMatch adjusts output scroll to center the current match in the viewport.
func (h *Handler) ScrollToMatch() {
	engine := h.ctx.GetSearchEngine()
	current := engine.Current()
	if current == nil {
		return
	}

	matchLine := current.LineNumber
	maxLines := max(h.ctx.GetViewportHeight(), 5)

	// Center the match in the visible area
	scroll := max(matchLine-maxLines/2, 0)
	h.ctx.SetOutputScroll(scroll)
}

// NextMatch moves to the next search result and scrolls to it.
// Returns true if there was a match to navigate to.
func (h *Handler) NextMatch() bool {
	engine := h.ctx.GetSearchEngine()
	if !engine.HasMatches() {
		return false
	}
	engine.Next()
	h.ScrollToMatch()
	return true
}

// PreviousMatch moves to the previous search result and scrolls to it.
// Returns true if there was a match to navigate to.
func (h *Handler) PreviousMatch() bool {
	engine := h.ctx.GetSearchEngine()
	if !engine.HasMatches() {
		return false
	}
	engine.Previous()
	h.ScrollToMatch()
	return true
}
