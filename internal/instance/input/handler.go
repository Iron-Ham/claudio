// Package input provides input handling for Claude Code instances.
//
// This package extracts input-related logic from the instance manager,
// providing a focused component for handling input encoding, buffering,
// and history tracking for tmux-based Claude sessions.
package input

import (
	"fmt"
	"io"
	"log"
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

// BatchConfig controls input batching behavior.
// When enabled, consecutive literal characters are coalesced into batches
// to reduce the number of tmux commands sent.
type BatchConfig struct {
	// Enabled controls whether batching is active
	Enabled bool
	// FlushInterval is how long to wait before flushing accumulated input
	FlushInterval time.Duration
	// MaxBatchSize is the maximum characters before forcing a flush
	MaxBatchSize int
}

// DefaultBatchConfig returns sensible defaults for input batching.
// The 8ms flush interval balances responsiveness with batching efficiency -
// it's imperceptible to users while allowing 5-10x reduction in tmux commands.
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		Enabled:       true,
		FlushInterval: 8 * time.Millisecond,
		MaxBatchSize:  100,
	}
}

// batchItem represents a single input item to be processed by the batcher.
type batchItem struct {
	text    string
	literal bool // true for literals, false for special keys
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

	// Batching state for coalescing keystrokes
	batchConfig   BatchConfig
	batchChan     chan batchItem
	batchStopChan chan struct{}
	batchWg       sync.WaitGroup
	batchOnce     sync.Once // Ensures batcher is stopped only once
	sessionName   string    // Cached session name for batching
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

// WithPersistentSender creates a persistent tmux sender for the given session.
// This uses tmux control mode to maintain a persistent connection, avoiding
// subprocess spawn overhead for each character sent.
// This option is mutually exclusive with WithTmuxSender.
func WithPersistentSender(sessionName string, opts ...PersistentOption) Option {
	return func(h *Handler) {
		h.sender = NewPersistentTmuxSender(sessionName, opts...)
	}
}

// WithBatching enables input batching with the given configuration.
// When enabled, consecutive literal characters are coalesced and sent together,
// dramatically reducing the number of tmux commands for fast typists.
// The sessionName is required for the batch goroutine to send to tmux.
// Panics if cfg.Enabled is true but FlushInterval or MaxBatchSize are not positive.
func WithBatching(sessionName string, cfg BatchConfig) Option {
	return func(h *Handler) {
		if cfg.Enabled {
			if cfg.FlushInterval <= 0 {
				panic("BatchConfig.FlushInterval must be positive when batching is enabled")
			}
			if cfg.MaxBatchSize <= 0 {
				panic("BatchConfig.MaxBatchSize must be positive when batching is enabled")
			}
		}
		h.sessionName = sessionName
		h.batchConfig = cfg
		if cfg.Enabled {
			h.batchChan = make(chan batchItem, 256)
			h.batchStopChan = make(chan struct{})
			h.startBatcher()
		}
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
// When batching is enabled, this flushes any pending literals first.
// This method is asynchronous and returns immediately.
func (h *Handler) SendKey(sessionName string, key string) error {
	h.recordHistory(key, InputTypeKey)

	if h.trySendToBatcher(batchItem{text: key, literal: false}) {
		return nil
	}

	h.sendAsync(sessionName, key, false)
	return nil
}

// SendInterrupt sends an interrupt signal (Ctrl+C) to the tmux session.
// This is a convenience wrapper around SendKey for the common interrupt case.
func (h *Handler) SendInterrupt(sessionName string) error {
	return h.SendKey(sessionName, "C-c")
}

// SendLiteral sends text to the tmux session without any interpretation.
// Unlike SendInput, this does not process special characters.
// When batching is enabled, literals are buffered and sent together.
// This method is asynchronous and returns immediately.
func (h *Handler) SendLiteral(sessionName string, text string) error {
	h.recordHistory(text, InputTypeLiteral)

	if h.trySendToBatcher(batchItem{text: text, literal: true}) {
		return nil
	}

	h.sendAsync(sessionName, text, true)
	return nil
}

// trySendToBatcher attempts to send an item to the batch channel.
// Returns true if the item was successfully queued, false if batching is
// disabled or the channel is full (caller should fall back to direct send).
func (h *Handler) trySendToBatcher(item batchItem) bool {
	if !h.batchConfig.Enabled || h.batchChan == nil {
		return false
	}

	select {
	case h.batchChan <- item:
		return true
	default:
		return false
	}
}

// sendAsync sends input to tmux asynchronously without blocking.
// Used as a fallback when batching is disabled or the batch channel is full.
func (h *Handler) sendAsync(sessionName string, text string, literal bool) {
	sender := h.getSender()
	go func() {
		if err := sender.SendKeys(sessionName, text, literal); err != nil {
			log.Printf("WARNING: async send failed: %v", err)
		}
	}()
}

// getSender returns the current sender with proper locking.
func (h *Handler) getSender() TmuxSender {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sender
}

// SendPaste sends pasted text with bracketed paste mode sequences.
// This preserves paste context for applications that support bracketed paste.
// The sequence is: ESC[200~ + text + ESC[201~
func (h *Handler) SendPaste(sessionName string, text string) error {
	h.recordHistory(text, InputTypePaste)

	sender := h.getSender()
	go func() {
		const pasteStart = "\x1b[200~"
		const pasteEnd = "\x1b[201~"
		_ = sender.SendKeys(sessionName, pasteStart, true)
		_ = sender.SendKeys(sessionName, text, true)
		_ = sender.SendKeys(sessionName, pasteEnd, true)
	}()

	return nil
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

// startBatcher launches the background goroutine that coalesces input.
// It accumulates literal characters in a buffer and flushes them either:
// - When the flush interval timer fires
// - When a special key is received (flush first, then send key)
// - When the batch reaches maximum size
func (h *Handler) startBatcher() {
	h.batchWg.Add(1)
	go func() {
		defer h.batchWg.Done()

		var literalBuf strings.Builder
		var timer *time.Timer
		var timerC <-chan time.Time

		stopTimer := func() {
			if timer != nil {
				timer.Stop()
				timer = nil
				timerC = nil
			}
		}

		flushLiterals := func() {
			if literalBuf.Len() > 0 {
				text := literalBuf.String()
				literalBuf.Reset()
				if err := h.getSender().SendKeys(h.sessionName, text, true); err != nil {
					log.Printf("WARNING: failed to send batched input (%d chars): %v", len(text), err)
				}
			}
			stopTimer()
		}

		processItem := func(item batchItem) {
			if item.literal {
				literalBuf.WriteString(item.text)
				if timer == nil {
					timer = time.NewTimer(h.batchConfig.FlushInterval)
					timerC = timer.C
				}
				if literalBuf.Len() >= h.batchConfig.MaxBatchSize {
					flushLiterals()
				}
				return
			}

			// Special key: flush pending literals first, then send key
			flushLiterals()
			if err := h.getSender().SendKeys(h.sessionName, item.text, false); err != nil {
				log.Printf("WARNING: failed to send key %q: %v", item.text, err)
			}
		}

		drainAndExit := func() {
			for {
				select {
				case item, ok := <-h.batchChan:
					if !ok {
						flushLiterals()
						return
					}
					processItem(item)
				default:
					flushLiterals()
					return
				}
			}
		}

		for {
			select {
			case <-h.batchStopChan:
				drainAndExit()
				return

			case item, ok := <-h.batchChan:
				if !ok {
					flushLiterals()
					return
				}
				processItem(item)

			case <-timerC:
				flushLiterals()
			}
		}
	}()
}

// Close releases resources held by the handler.
// If the handler uses a persistent sender, this closes the connection.
// This should be called when the handler is no longer needed.
// Safe to call multiple times; only the first call has any effect.
func (h *Handler) Close() error {
	h.stopBatcher()

	h.mu.Lock()
	defer h.mu.Unlock()

	if closer, ok := h.sender.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// stopBatcher safely stops the batcher goroutine if running.
// Uses sync.Once to ensure it only runs once, avoiding double-close panics.
func (h *Handler) stopBatcher() {
	h.batchOnce.Do(func() {
		h.mu.Lock()
		stopChan := h.batchStopChan
		h.mu.Unlock()

		if stopChan == nil {
			return
		}

		close(stopChan)
		h.batchWg.Wait()

		h.mu.Lock()
		h.batchStopChan = nil
		h.batchChan = nil
		h.mu.Unlock()
	})
}
