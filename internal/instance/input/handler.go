// Package input provides input handling for Claude Code instances.
//
// This package extracts input-related logic from the instance manager,
// providing a focused component for handling input encoding, buffering,
// and history tracking for tmux-based Claude sessions.
package input

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
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

// inputItem represents an item to be sent to tmux.
// It can be either literal text (batched) or a special key.
type inputItem struct {
	sessionName string
	text        string // for literal text
	key         string // for special keys (mutually exclusive with text)
	literal     bool   // true if text should be sent with -l flag
}

// workerBatchTimeout is how long the worker waits for more input before flushing.
// This allows rapid keystrokes to be batched while ensuring responsive feedback.
const workerBatchTimeout = 5 * time.Millisecond

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

	// Worker goroutine for batching keyboard input
	workerChan chan inputItem // channel for sending items to worker
	workerDone chan struct{}  // signal to stop worker
	workerOnce sync.Once      // ensures worker starts only once
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
		workerChan: make(chan inputItem, 256), // buffered to avoid blocking callers
		workerDone: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// SendInput sends text input to the tmux session, handling special characters.
// Characters are batched to minimize subprocess calls: consecutive regular characters
// are accumulated and sent in a single tmux command, while special characters
// (Enter, Tab, etc.) flush the batch and are sent individually.
// This method is synchronous and blocks until all input is sent.
func (h *Handler) SendInput(sessionName string, input string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var batch strings.Builder

	for _, r := range input {
		if h.isSpecialRune(r) {
			// Flush any accumulated regular characters first
			if batch.Len() > 0 {
				if err := h.sender.SendKeys(sessionName, batch.String(), true); err != nil {
					return fmt.Errorf("failed to send batch %q: %w", batch.String(), err)
				}
				batch.Reset()
			}
			// Send the special key
			key := h.encodeRune(r)
			if err := h.sender.SendKeys(sessionName, key, false); err != nil {
				return fmt.Errorf("failed to send key %q: %w", key, err)
			}
		} else {
			// Accumulate regular characters
			batch.WriteRune(r)
		}
	}

	// Flush any remaining regular characters
	if batch.Len() > 0 {
		if err := h.sender.SendKeys(sessionName, batch.String(), true); err != nil {
			return fmt.Errorf("failed to send batch %q: %w", batch.String(), err)
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

// ensureWorkerRunning starts the worker goroutine if not already running.
// The worker batches consecutive literal characters to reduce subprocess calls.
func (h *Handler) ensureWorkerRunning() {
	h.workerOnce.Do(func() {
		go h.workerLoop()
	})
}

// workerLoop is the main loop for the batching worker goroutine.
// It collects literal characters and batches them together, flushing when:
// - A special key arrives (non-literal)
// - The session changes
// - A timeout occurs (workerBatchTimeout)
// - The worker is shutting down
func (h *Handler) workerLoop() {
	var batch strings.Builder
	var currentSession string
	timer := time.NewTimer(workerBatchTimeout)
	timer.Stop() // Start stopped; we'll reset it when we have data

	flush := func() {
		if batch.Len() > 0 && currentSession != "" {
			h.mu.RLock()
			sender := h.sender
			h.mu.RUnlock()
			_ = sender.SendKeys(currentSession, batch.String(), true)
			batch.Reset()
		}
		timer.Stop()
	}

	for {
		select {
		case <-h.workerDone:
			flush()
			return

		case item := <-h.workerChan:
			// If session changed, flush previous batch
			if currentSession != "" && item.sessionName != currentSession {
				flush()
			}
			currentSession = item.sessionName

			if item.key != "" {
				// Special key - flush batch first, then send the key
				flush()
				h.mu.RLock()
				sender := h.sender
				h.mu.RUnlock()
				_ = sender.SendKeys(item.sessionName, item.key, false)
			} else if item.text != "" {
				// Literal text - add to batch
				batch.WriteString(item.text)
				// Reset timer to flush after timeout if no more input arrives
				timer.Reset(workerBatchTimeout)
			}

		case <-timer.C:
			// Timeout - flush whatever we have
			flush()
		}
	}
}

// QueueLiteral queues literal text to be sent to the tmux session.
// Unlike SendLiteral, this uses a worker goroutine to batch consecutive
// characters, reducing the number of tmux subprocess calls.
// This method returns immediately and is safe for use in keyboard handlers.
func (h *Handler) QueueLiteral(sessionName string, text string) {
	h.ensureWorkerRunning()

	// Non-blocking send - if channel is full, fall back to direct send
	select {
	case h.workerChan <- inputItem{sessionName: sessionName, text: text, literal: true}:
		// Successfully queued
	default:
		// Channel full - send directly to avoid blocking
		h.mu.RLock()
		sender := h.sender
		h.mu.RUnlock()
		go func() {
			_ = sender.SendKeys(sessionName, text, true)
		}()
	}

	h.recordHistory(text, InputTypeLiteral)
}

// QueueKey queues a special key to be sent to the tmux session.
// This causes any pending literal characters to be flushed first.
func (h *Handler) QueueKey(sessionName string, key string) {
	h.ensureWorkerRunning()

	// Non-blocking send - if channel is full, fall back to direct send
	select {
	case h.workerChan <- inputItem{sessionName: sessionName, key: key}:
		// Successfully queued
	default:
		// Channel full - send directly to avoid blocking
		h.mu.RLock()
		sender := h.sender
		h.mu.RUnlock()
		go func() {
			_ = sender.SendKeys(sessionName, key, false)
		}()
	}

	h.recordHistory(key, InputTypeKey)
}

// StopWorker stops the batching worker goroutine.
// This should be called when the handler is no longer needed.
func (h *Handler) StopWorker() {
	select {
	case <-h.workerDone:
		// Already closed
	default:
		close(h.workerDone)
	}
}

// isSpecialRune returns true if the rune requires special handling by tmux
// and cannot be batched with regular characters. Special runes include
// newlines, tabs, backspace, escape, space, and control characters.
func (h *Handler) isSpecialRune(r rune) bool {
	switch r {
	case '\r', '\n', '\t', '\x7f', '\b', '\x1b', ' ':
		return true
	default:
		// Control characters (0x00-0x1F) require special handling
		return r < 32
	}
}

// encodeRune converts a rune to the appropriate tmux key sequence.
// For special runes (newline, tab, etc.), returns the tmux key name.
// For regular characters, returns the character literally.
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
