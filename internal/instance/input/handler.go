// Package input provides input handling for Claude Code instances.
//
// This package extracts input-related logic from the instance manager,
// providing a focused component for handling input encoding, buffering,
// and history tracking for tmux-based Claude sessions.
package input

import (
	"fmt"
	"os/exec"
	"sync"
)

// TmuxSender defines the interface for sending commands to tmux.
// This interface enables testing without requiring actual tmux sessions.
type TmuxSender interface {
	// SendKeys sends keys to the tmux session.
	// If literal is true, keys are sent without interpretation (-l flag).
	SendKeys(sessionName string, keys string, literal bool) error
}

// DefaultTmuxSender is the production implementation of TmuxSender.
type DefaultTmuxSender struct{}

// SendKeys sends keys to a tmux session using exec.Command.
func (d *DefaultTmuxSender) SendKeys(sessionName string, keys string, literal bool) error {
	args := []string{"send-keys", "-t", sessionName}
	if literal {
		args = append(args, "-l")
	}
	args = append(args, keys)
	return exec.Command("tmux", args...).Run()
}

// HistoryEntry represents a single entry in the input history.
type HistoryEntry struct {
	// Input is the actual input that was sent.
	Input string
	// Type indicates what kind of input this was (text, key, interrupt).
	Type InputType
}

// InputType represents the type of input sent.
type InputType int

const (
	// InputTypeText is regular text input.
	InputTypeText InputType = iota
	// InputTypeKey is a special key (like Enter, Tab, etc.).
	InputTypeKey
	// InputTypeLiteral is literal text sent without interpretation.
	InputTypeLiteral
	// InputTypePaste is pasted text with bracketed paste sequences.
	InputTypePaste
	// InputTypeInterrupt is an interrupt signal (Ctrl+C).
	InputTypeInterrupt
)

// String returns a human-readable string for the input type.
func (t InputType) String() string {
	switch t {
	case InputTypeText:
		return "text"
	case InputTypeKey:
		return "key"
	case InputTypeLiteral:
		return "literal"
	case InputTypePaste:
		return "paste"
	case InputTypeInterrupt:
		return "interrupt"
	default:
		return "unknown"
	}
}

// Handler manages input encoding, buffering, and history for tmux sessions.
type Handler struct {
	mu     sync.RWMutex
	sender TmuxSender

	// Input history for debugging and replay purposes
	history     []HistoryEntry
	maxHistory  int
	historyLock sync.RWMutex

	// Input buffer for batching small inputs
	buffer     []byte
	bufferLock sync.Mutex
}

// Option configures the Handler.
type Option func(*Handler)

// WithTmuxSender sets a custom tmux sender for the handler.
// Useful for testing with mock implementations.
func WithTmuxSender(sender TmuxSender) Option {
	return func(h *Handler) {
		h.sender = sender
	}
}

// WithMaxHistory sets the maximum number of history entries to keep.
// Default is 100 entries. Set to 0 to disable history tracking.
func WithMaxHistory(max int) Option {
	return func(h *Handler) {
		h.maxHistory = max
	}
}

// NewHandler creates a new input handler with the given options.
func NewHandler(opts ...Option) *Handler {
	h := &Handler{
		sender:     &DefaultTmuxSender{},
		maxHistory: 100,
		history:    make([]HistoryEntry, 0),
		buffer:     make([]byte, 0),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// SendInput sends text input to the tmux session, handling special characters.
// Each character is processed and converted to the appropriate tmux key sequence.
// This method is synchronous and blocks until all input is sent.
func (h *Handler) SendInput(sessionName string, input string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, r := range input {
		key := h.encodeRune(r)
		if err := h.sender.SendKeys(sessionName, key, true); err != nil {
			return fmt.Errorf("failed to send key %q: %w", key, err)
		}
	}

	h.recordHistory(input, InputTypeText)
	return nil
}

// SendKey sends a special key to the tmux session.
// Common keys: "Enter", "Tab", "Escape", "BSpace", "C-c" (Ctrl+C), etc.
// This method is asynchronous and returns immediately.
func (h *Handler) SendKey(sessionName string, key string) error {
	h.mu.RLock()
	sender := h.sender
	h.mu.RUnlock()

	// Run async to avoid blocking the caller
	go func() {
		_ = sender.SendKeys(sessionName, key, false)
	}()

	h.recordHistory(key, InputTypeKey)
	return nil
}

// SendInterrupt sends an interrupt signal (Ctrl+C) to the tmux session.
// This is a convenience wrapper around SendKey for the common interrupt case.
func (h *Handler) SendInterrupt(sessionName string) error {
	return h.SendKey(sessionName, "C-c")
}

// SendLiteral sends text to the tmux session without any interpretation.
// Unlike SendInput, this does not process special characters.
// This method is asynchronous and returns immediately.
func (h *Handler) SendLiteral(sessionName string, text string) error {
	h.mu.RLock()
	sender := h.sender
	h.mu.RUnlock()

	// Run async to avoid blocking the caller
	go func() {
		_ = sender.SendKeys(sessionName, text, true)
	}()

	h.recordHistory(text, InputTypeLiteral)
	return nil
}

// SendPaste sends pasted text with bracketed paste mode sequences.
// This preserves paste context for applications that support bracketed paste.
// The sequence is: ESC[200~ + text + ESC[201~
func (h *Handler) SendPaste(sessionName string, text string) error {
	h.mu.RLock()
	sender := h.sender
	h.mu.RUnlock()

	// Run async to avoid blocking the caller
	go func() {
		// Bracketed paste mode escape sequences
		pasteStart := "\x1b[200~"
		pasteEnd := "\x1b[201~"

		// Send in sequence: start marker, content, end marker
		_ = sender.SendKeys(sessionName, pasteStart, true)
		_ = sender.SendKeys(sessionName, text, true)
		_ = sender.SendKeys(sessionName, pasteEnd, true)
	}()

	h.recordHistory(text, InputTypePaste)
	return nil
}

// encodeRune converts a rune to the appropriate tmux key sequence.
func (h *Handler) encodeRune(r rune) string {
	switch r {
	case '\r', '\n':
		return "Enter"
	case '\t':
		return "Tab"
	case '\x7f', '\b': // backspace
		return "BSpace"
	case '\x1b': // escape
		return "Escape"
	case ' ':
		return "Space"
	default:
		if r < 32 {
			// Control character: Ctrl+letter
			return fmt.Sprintf("C-%c", r+'a'-1)
		}
		// Regular character - send literally
		return string(r)
	}
}

// recordHistory adds an entry to the input history.
func (h *Handler) recordHistory(input string, inputType InputType) {
	if h.maxHistory <= 0 {
		return
	}

	h.historyLock.Lock()
	defer h.historyLock.Unlock()

	entry := HistoryEntry{
		Input: input,
		Type:  inputType,
	}

	h.history = append(h.history, entry)

	// Trim history if it exceeds the maximum
	if len(h.history) > h.maxHistory {
		// Remove oldest entries, keeping the most recent maxHistory entries
		h.history = h.history[len(h.history)-h.maxHistory:]
	}
}

// History returns a copy of the input history.
// The returned slice can be safely modified without affecting the handler.
func (h *Handler) History() []HistoryEntry {
	h.historyLock.RLock()
	defer h.historyLock.RUnlock()

	result := make([]HistoryEntry, len(h.history))
	copy(result, h.history)
	return result
}

// ClearHistory clears the input history.
func (h *Handler) ClearHistory() {
	h.historyLock.Lock()
	defer h.historyLock.Unlock()
	h.history = h.history[:0]
}

// AppendToBuffer adds data to the input buffer.
// This can be used to batch multiple small inputs before sending.
func (h *Handler) AppendToBuffer(data []byte) {
	h.bufferLock.Lock()
	defer h.bufferLock.Unlock()
	h.buffer = append(h.buffer, data...)
}

// FlushBuffer sends all buffered input to the session and clears the buffer.
// Returns the number of bytes flushed.
func (h *Handler) FlushBuffer(sessionName string) (int, error) {
	h.bufferLock.Lock()
	if len(h.buffer) == 0 {
		h.bufferLock.Unlock()
		return 0, nil
	}

	data := make([]byte, len(h.buffer))
	copy(data, h.buffer)
	h.buffer = h.buffer[:0]
	h.bufferLock.Unlock()

	if err := h.SendInput(sessionName, string(data)); err != nil {
		return 0, err
	}

	return len(data), nil
}

// BufferSize returns the current size of the input buffer.
func (h *Handler) BufferSize() int {
	h.bufferLock.Lock()
	defer h.bufferLock.Unlock()
	return len(h.buffer)
}

// ClearBuffer clears the input buffer without sending.
func (h *Handler) ClearBuffer() {
	h.bufferLock.Lock()
	defer h.bufferLock.Unlock()
	h.buffer = h.buffer[:0]
}
