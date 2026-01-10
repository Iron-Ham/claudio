package tui

// InputHandler manages text input state including the buffer content and cursor position.
// It provides methods for inserting, deleting, and navigating text with proper
// cursor management and word/line boundary detection.
type InputHandler struct {
	// buffer holds the current input text
	buffer string
	// cursor is the position within the buffer (0 = before first char)
	cursor int
}

// NewInputHandler creates a new InputHandler with empty buffer
func NewInputHandler() *InputHandler {
	return &InputHandler{
		buffer: "",
		cursor: 0,
	}
}

// Buffer returns the current input buffer content
func (h *InputHandler) Buffer() string {
	return h.buffer
}

// SetBuffer sets the buffer content and optionally updates cursor position
func (h *InputHandler) SetBuffer(s string) {
	h.buffer = s
	// Ensure cursor doesn't exceed buffer length
	runes := []rune(h.buffer)
	if h.cursor > len(runes) {
		h.cursor = len(runes)
	}
}

// Cursor returns the current cursor position
func (h *InputHandler) Cursor() int {
	return h.cursor
}

// SetCursor sets the cursor position, clamping to valid bounds
func (h *InputHandler) SetCursor(pos int) {
	runes := []rune(h.buffer)
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	h.cursor = pos
}

// Clear resets both buffer and cursor to initial state
func (h *InputHandler) Clear() {
	h.buffer = ""
	h.cursor = 0
}

// IsEmpty returns true if the buffer is empty
func (h *InputHandler) IsEmpty() bool {
	return h.buffer == ""
}

// Len returns the length of the buffer in runes
func (h *InputHandler) Len() int {
	return len([]rune(h.buffer))
}

// Insert inserts text at the current cursor position
func (h *InputHandler) Insert(text string) {
	runes := []rune(h.buffer)
	h.buffer = string(runes[:h.cursor]) + text + string(runes[h.cursor:])
	h.cursor += len([]rune(text))
}

// DeleteBack deletes n runes before the cursor
func (h *InputHandler) DeleteBack(n int) {
	if h.cursor == 0 {
		return
	}
	runes := []rune(h.buffer)
	deleteCount := n
	if deleteCount > h.cursor {
		deleteCount = h.cursor
	}
	h.buffer = string(runes[:h.cursor-deleteCount]) + string(runes[h.cursor:])
	h.cursor -= deleteCount
}

// DeleteForward deletes n runes after the cursor
func (h *InputHandler) DeleteForward(n int) {
	runes := []rune(h.buffer)
	if h.cursor >= len(runes) {
		return
	}
	deleteCount := n
	if h.cursor+deleteCount > len(runes) {
		deleteCount = len(runes) - h.cursor
	}
	h.buffer = string(runes[:h.cursor]) + string(runes[h.cursor+deleteCount:])
}

// MoveCursor moves cursor by n runes (negative = left, positive = right)
func (h *InputHandler) MoveCursor(n int) {
	runes := []rune(h.buffer)
	newPos := h.cursor + n
	if newPos < 0 {
		newPos = 0
	}
	if newPos > len(runes) {
		newPos = len(runes)
	}
	h.cursor = newPos
}

// MoveToStart moves cursor to the beginning of the buffer
func (h *InputHandler) MoveToStart() {
	h.cursor = 0
}

// MoveToEnd moves cursor to the end of the buffer
func (h *InputHandler) MoveToEnd() {
	h.cursor = len([]rune(h.buffer))
}

// FindPrevWordBoundary finds the position of the previous word boundary
func (h *InputHandler) FindPrevWordBoundary() int {
	if h.cursor == 0 {
		return 0
	}
	runes := []rune(h.buffer)
	pos := h.cursor - 1

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

// FindNextWordBoundary finds the position of the next word boundary
func (h *InputHandler) FindNextWordBoundary() int {
	runes := []rune(h.buffer)
	if h.cursor >= len(runes) {
		return len(runes)
	}
	pos := h.cursor

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

// FindLineStart finds the start of the current line
func (h *InputHandler) FindLineStart() int {
	runes := []rune(h.buffer)
	pos := h.cursor
	for pos > 0 && runes[pos-1] != '\n' {
		pos--
	}
	return pos
}

// FindLineEnd finds the end of the current line
func (h *InputHandler) FindLineEnd() int {
	runes := []rune(h.buffer)
	pos := h.cursor
	for pos < len(runes) && runes[pos] != '\n' {
		pos++
	}
	return pos
}

// DeleteToLineStart deletes from cursor to start of current line
func (h *InputHandler) DeleteToLineStart() {
	lineStart := h.FindLineStart()
	h.DeleteBack(h.cursor - lineStart)
}

// DeleteToLineEnd deletes from cursor to end of current line
func (h *InputHandler) DeleteToLineEnd() {
	lineEnd := h.FindLineEnd()
	h.DeleteForward(lineEnd - h.cursor)
}

// DeleteWord deletes the word before the cursor (for Ctrl+W)
func (h *InputHandler) DeleteWord() {
	prevWord := h.FindPrevWordBoundary()
	h.DeleteBack(h.cursor - prevWord)
}

// MoveToPrevWord moves cursor to the previous word boundary
func (h *InputHandler) MoveToPrevWord() {
	h.cursor = h.FindPrevWordBoundary()
}

// MoveToNextWord moves cursor to the next word boundary
func (h *InputHandler) MoveToNextWord() {
	h.cursor = h.FindNextWordBoundary()
}

// MoveToLineStart moves cursor to the start of the current line
func (h *InputHandler) MoveToLineStart() {
	h.cursor = h.FindLineStart()
}

// MoveToLineEnd moves cursor to the end of the current line
func (h *InputHandler) MoveToLineEnd() {
	h.cursor = h.FindLineEnd()
}

// IsAtLineStart returns true if cursor is at the start of a line
func (h *InputHandler) IsAtLineStart() bool {
	if h.cursor == 0 {
		return true
	}
	runes := []rune(h.buffer)
	return h.cursor > 0 && runes[h.cursor-1] == '\n'
}

// isWordChar returns true if the rune is considered part of a word
func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
