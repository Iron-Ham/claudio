package input

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// mockTmuxSender is a test implementation of TmuxSender that records all calls.
type mockTmuxSender struct {
	mu       sync.Mutex
	calls    []sendCall
	failNext bool
	failErr  error
}

type sendCall struct {
	sessionName string
	keys        string
	literal     bool
}

func (m *mockTmuxSender) SendKeys(sessionName string, keys string, literal bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failNext {
		m.failNext = false
		return m.failErr
	}

	m.calls = append(m.calls, sendCall{
		sessionName: sessionName,
		keys:        keys,
		literal:     literal,
	})
	return nil
}

func (m *mockTmuxSender) getCalls() []sendCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]sendCall, len(m.calls))
	copy(result, m.calls)
	return result
}

func (m *mockTmuxSender) setFailNext(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failNext = true
	m.failErr = err
}

func TestNewHandler(t *testing.T) {
	h := NewHandler()

	if h == nil {
		t.Fatal("NewHandler returned nil")
	}

	if h.sender == nil {
		t.Error("sender should not be nil")
	}

	if h.maxHistory != 100 {
		t.Errorf("maxHistory = %d, want 100", h.maxHistory)
	}
}

func TestNewHandler_WithOptions(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(
		WithTmuxSender(mock),
		WithMaxHistory(50),
	)

	if h.sender != mock {
		t.Error("custom sender not set")
	}

	if h.maxHistory != 50 {
		t.Errorf("maxHistory = %d, want 50", h.maxHistory)
	}
}

func TestHandler_SendInput(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	err := h.SendInput("test-session", "hello")
	if err != nil {
		t.Fatalf("SendInput failed: %v", err)
	}

	calls := mock.getCalls()
	// With batching, "hello" (all regular chars) should be sent in a single call
	if len(calls) != 1 {
		t.Errorf("got %d calls, want 1 (batched)", len(calls))
	}

	if calls[0].keys != "hello" {
		t.Errorf("call[0].keys = %q, want %q", calls[0].keys, "hello")
	}
	if calls[0].sessionName != "test-session" {
		t.Errorf("call[0].sessionName = %q, want %q", calls[0].sessionName, "test-session")
	}
	if !calls[0].literal {
		t.Errorf("call[0].literal = false, want true")
	}
}

func TestHandler_SendInput_SpecialCharacters(t *testing.T) {
	type expectedCall struct {
		keys    string
		literal bool
	}
	tests := []struct {
		name     string
		input    string
		expected []expectedCall
	}{
		{
			name:     "newline",
			input:    "\n",
			expected: []expectedCall{{keys: "Enter", literal: false}},
		},
		{
			name:     "carriage return",
			input:    "\r",
			expected: []expectedCall{{keys: "Enter", literal: false}},
		},
		{
			name:     "tab",
			input:    "\t",
			expected: []expectedCall{{keys: "Tab", literal: false}},
		},
		{
			name:     "backspace",
			input:    "\x7f",
			expected: []expectedCall{{keys: "BSpace", literal: false}},
		},
		{
			name:     "escape",
			input:    "\x1b",
			expected: []expectedCall{{keys: "Escape", literal: false}},
		},
		{
			name:     "space",
			input:    " ",
			expected: []expectedCall{{keys: "Space", literal: false}},
		},
		{
			name:     "control character (ctrl-a)",
			input:    "\x01",
			expected: []expectedCall{{keys: "C-a", literal: false}},
		},
		{
			// With batching: "a" is batched (literal), "\n" flushes and sends Enter
			// then "b" is batched (literal)
			name:  "mixed input",
			input: "a\nb",
			expected: []expectedCall{
				{keys: "a", literal: true},
				{keys: "Enter", literal: false},
				{keys: "b", literal: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTmuxSender{}
			h := NewHandler(WithTmuxSender(mock))

			err := h.SendInput("test-session", tt.input)
			if err != nil {
				t.Fatalf("SendInput failed: %v", err)
			}

			calls := mock.getCalls()
			if len(calls) != len(tt.expected) {
				t.Fatalf("got %d calls, want %d: %+v", len(calls), len(tt.expected), calls)
			}

			for i, exp := range tt.expected {
				if calls[i].keys != exp.keys {
					t.Errorf("call[%d].keys = %q, want %q", i, calls[i].keys, exp.keys)
				}
				if calls[i].literal != exp.literal {
					t.Errorf("call[%d].literal = %v, want %v", i, calls[i].literal, exp.literal)
				}
			}
		})
	}
}

func TestHandler_SendInput_Error(t *testing.T) {
	mock := &mockTmuxSender{}
	mock.setFailNext(fmt.Errorf("tmux error"))

	h := NewHandler(WithTmuxSender(mock))

	err := h.SendInput("test-session", "a")
	if err == nil {
		t.Error("SendInput should return error when sender fails")
	}
}

func TestHandler_SendKey(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	err := h.SendKey("test-session", "C-c")
	if err != nil {
		t.Fatalf("SendKey failed: %v", err)
	}

	// SendKey is async, so we need to wait a bit for the goroutine to execute
	// In a real test environment, we might use a channel or wait group
	// For simplicity, we'll just check the history was recorded
	history := h.History()
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}

	if history[0].Input != "C-c" {
		t.Errorf("history[0].Input = %q, want %q", history[0].Input, "C-c")
	}

	if history[0].Type != InputTypeKey {
		t.Errorf("history[0].Type = %v, want %v", history[0].Type, InputTypeKey)
	}
}

func TestHandler_SendInterrupt(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	err := h.SendInterrupt("test-session")
	if err != nil {
		t.Fatalf("SendInterrupt failed: %v", err)
	}

	// Check history was recorded as interrupt
	history := h.History()
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}

	if history[0].Input != "C-c" {
		t.Errorf("history[0].Input = %q, want %q", history[0].Input, "C-c")
	}

	if history[0].Type != InputTypeKey {
		t.Errorf("history[0].Type = %v, want %v", history[0].Type, InputTypeKey)
	}
}

func TestHandler_SendLiteral(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	err := h.SendLiteral("test-session", "literal text")
	if err != nil {
		t.Fatalf("SendLiteral failed: %v", err)
	}

	// Check history was recorded
	history := h.History()
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}

	if history[0].Input != "literal text" {
		t.Errorf("history[0].Input = %q, want %q", history[0].Input, "literal text")
	}

	if history[0].Type != InputTypeLiteral {
		t.Errorf("history[0].Type = %v, want %v", history[0].Type, InputTypeLiteral)
	}
}

func TestHandler_SendPaste(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	err := h.SendPaste("test-session", "pasted text")
	if err != nil {
		t.Fatalf("SendPaste failed: %v", err)
	}

	// Check history was recorded
	history := h.History()
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}

	if history[0].Input != "pasted text" {
		t.Errorf("history[0].Input = %q, want %q", history[0].Input, "pasted text")
	}

	if history[0].Type != InputTypePaste {
		t.Errorf("history[0].Type = %v, want %v", history[0].Type, InputTypePaste)
	}
}

func TestHandler_History(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock), WithMaxHistory(5))

	// Send multiple inputs
	_ = h.SendInput("s", "a")
	_ = h.SendInput("s", "b")
	_ = h.SendKey("s", "Enter")

	history := h.History()
	if len(history) != 3 {
		t.Errorf("history length = %d, want 3", len(history))
	}

	// Verify order
	if history[0].Input != "a" {
		t.Errorf("history[0].Input = %q, want %q", history[0].Input, "a")
	}
	if history[1].Input != "b" {
		t.Errorf("history[1].Input = %q, want %q", history[1].Input, "b")
	}
	if history[2].Input != "Enter" {
		t.Errorf("history[2].Input = %q, want %q", history[2].Input, "Enter")
	}
}

func TestHandler_HistoryLimit(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock), WithMaxHistory(3))

	// Send more inputs than the limit
	for i := range 5 {
		_ = h.SendInput("s", fmt.Sprintf("%d", i))
	}

	history := h.History()
	if len(history) != 3 {
		t.Errorf("history length = %d, want 3", len(history))
	}

	// Should have the most recent entries
	if history[0].Input != "2" {
		t.Errorf("history[0].Input = %q, want %q", history[0].Input, "2")
	}
	if history[1].Input != "3" {
		t.Errorf("history[1].Input = %q, want %q", history[1].Input, "3")
	}
	if history[2].Input != "4" {
		t.Errorf("history[2].Input = %q, want %q", history[2].Input, "4")
	}
}

func TestHandler_HistoryDisabled(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock), WithMaxHistory(0))

	_ = h.SendInput("s", "test")
	_ = h.SendKey("s", "Enter")

	history := h.History()
	if len(history) != 0 {
		t.Errorf("history length = %d, want 0 (disabled)", len(history))
	}
}

func TestHandler_ClearHistory(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	_ = h.SendInput("s", "test")

	history := h.History()
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}

	h.ClearHistory()

	history = h.History()
	if len(history) != 0 {
		t.Errorf("history length = %d, want 0 after clear", len(history))
	}
}

func TestHandler_Buffer(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	// Initially buffer should be empty
	if h.BufferSize() != 0 {
		t.Errorf("initial buffer size = %d, want 0", h.BufferSize())
	}

	// Append to buffer
	h.AppendToBuffer([]byte("hello"))
	if h.BufferSize() != 5 {
		t.Errorf("buffer size = %d, want 5", h.BufferSize())
	}

	// Append more
	h.AppendToBuffer([]byte(" world"))
	if h.BufferSize() != 11 {
		t.Errorf("buffer size = %d, want 11", h.BufferSize())
	}
}

func TestHandler_FlushBuffer(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	h.AppendToBuffer([]byte("abc"))

	n, err := h.FlushBuffer("test-session")
	if err != nil {
		t.Fatalf("FlushBuffer failed: %v", err)
	}

	if n != 3 {
		t.Errorf("flushed %d bytes, want 3", n)
	}

	if h.BufferSize() != 0 {
		t.Errorf("buffer size = %d, want 0 after flush", h.BufferSize())
	}

	calls := mock.getCalls()
	// With batching, "abc" (all regular chars) is sent as a single call
	if len(calls) != 1 {
		t.Errorf("got %d calls, want 1 (batched)", len(calls))
	}

	if calls[0].keys != "abc" {
		t.Errorf("call[0].keys = %q, want %q", calls[0].keys, "abc")
	}
}

func TestHandler_FlushBuffer_Empty(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	n, err := h.FlushBuffer("test-session")
	if err != nil {
		t.Fatalf("FlushBuffer failed: %v", err)
	}

	if n != 0 {
		t.Errorf("flushed %d bytes, want 0", n)
	}

	calls := mock.getCalls()
	if len(calls) != 0 {
		t.Errorf("got %d calls, want 0 for empty buffer", len(calls))
	}
}

func TestHandler_ClearBuffer(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	h.AppendToBuffer([]byte("test"))
	if h.BufferSize() != 4 {
		t.Fatalf("buffer size = %d, want 4", h.BufferSize())
	}

	h.ClearBuffer()
	if h.BufferSize() != 0 {
		t.Errorf("buffer size = %d, want 0 after clear", h.BufferSize())
	}
}

func TestHandler_ConcurrentAccess(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 100

	// Test concurrent SendInput
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range numOps {
				_ = h.SendInput("s", "x")
			}
		}()
	}

	// Test concurrent history access
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range numOps {
				_ = h.History()
			}
		}()
	}

	// Test concurrent buffer access
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range numOps {
				h.AppendToBuffer([]byte("x"))
				_ = h.BufferSize()
			}
		}()
	}

	wg.Wait()

	// Test shouldn't panic - if we get here, concurrent access is working
}

func TestHandler_HistoryIsCopy(t *testing.T) {
	mock := &mockTmuxSender{}
	h := NewHandler(WithTmuxSender(mock))

	_ = h.SendInput("s", "test")

	history1 := h.History()
	history2 := h.History()

	// Modify the first slice
	history1[0].Input = "modified"

	// The second slice should be unaffected
	if history2[0].Input != "test" {
		t.Errorf("History() returned aliased slice, got %q want %q", history2[0].Input, "test")
	}
}

func TestInputType_String(t *testing.T) {
	tests := []struct {
		inputType InputType
		expected  string
	}{
		{InputTypeText, "text"},
		{InputTypeKey, "key"},
		{InputTypeLiteral, "literal"},
		{InputTypePaste, "paste"},
		{InputTypeInterrupt, "interrupt"},
		{InputType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.inputType.String(); got != tt.expected {
				t.Errorf("InputType.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestHandler_encodeRune(t *testing.T) {
	h := NewHandler()

	tests := []struct {
		name     string
		r        rune
		expected string
	}{
		{"regular char", 'a', "a"},
		{"uppercase", 'A', "A"},
		{"digit", '5', "5"},
		{"newline", '\n', "Enter"},
		{"carriage return", '\r', "Enter"},
		{"tab", '\t', "Tab"},
		{"backspace 0x7f", '\x7f', "BSpace"},
		{"backspace 0x08", '\b', "BSpace"},
		{"escape", '\x1b', "Escape"},
		{"space", ' ', "Space"},
		{"ctrl-a", '\x01', "C-a"},
		{"ctrl-c", '\x03', "C-c"},
		{"ctrl-z", '\x1a', "C-z"},
		{"unicode", '\u4e2d', "\u4e2d"}, // Chinese character
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.encodeRune(tt.r)
			if got != tt.expected {
				t.Errorf("encodeRune(%q) = %q, want %q", tt.r, got, tt.expected)
			}
		})
	}
}

func TestDefaultTmuxSender_SendKeys(t *testing.T) {
	// Skip if tmux isn't available
	// This is an integration test that requires tmux
	t.Skip("Integration test - requires tmux")

	sender := &DefaultTmuxSender{}

	// This would fail since we don't have a real session, but we can verify
	// the method doesn't panic
	err := sender.SendKeys("nonexistent-session", "test", true)
	if err == nil {
		t.Error("Expected error for nonexistent session")
	}
}

// TestHandler_SendInput_Batching tests the batching optimization that reduces
// subprocess calls by grouping consecutive regular characters.
func TestHandler_SendInput_Batching(t *testing.T) {
	type expectedCall struct {
		keys    string
		literal bool
	}
	tests := []struct {
		name     string
		input    string
		expected []expectedCall
	}{
		{
			name:  "single character",
			input: "a",
			expected: []expectedCall{
				{keys: "a", literal: true},
			},
		},
		{
			name:  "multiple regular characters batched",
			input: "hello world!",
			expected: []expectedCall{
				{keys: "hello", literal: true},
				{keys: "Space", literal: false},
				{keys: "world!", literal: true},
			},
		},
		{
			name:  "unicode characters batched",
			input: "hello中文world",
			expected: []expectedCall{
				{keys: "hello中文world", literal: true},
			},
		},
		{
			name:  "multiple special characters in a row",
			input: "\n\n\t",
			expected: []expectedCall{
				{keys: "Enter", literal: false},
				{keys: "Enter", literal: false},
				{keys: "Tab", literal: false},
			},
		},
		{
			name:  "special at start",
			input: "\nhello",
			expected: []expectedCall{
				{keys: "Enter", literal: false},
				{keys: "hello", literal: true},
			},
		},
		{
			name:  "special at end",
			input: "hello\n",
			expected: []expectedCall{
				{keys: "hello", literal: true},
				{keys: "Enter", literal: false},
			},
		},
		{
			name:  "alternating regular and special",
			input: "a b c",
			expected: []expectedCall{
				{keys: "a", literal: true},
				{keys: "Space", literal: false},
				{keys: "b", literal: true},
				{keys: "Space", literal: false},
				{keys: "c", literal: true},
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []expectedCall{
				// No calls for empty input
			},
		},
		{
			name:  "typical sentence",
			input: "Hello, World!",
			expected: []expectedCall{
				{keys: "Hello,", literal: true},
				{keys: "Space", literal: false},
				{keys: "World!", literal: true},
			},
		},
		{
			name:  "command with enter",
			input: "ls -la\n",
			expected: []expectedCall{
				{keys: "ls", literal: true},
				{keys: "Space", literal: false},
				{keys: "-la", literal: true},
				{keys: "Enter", literal: false},
			},
		},
		{
			name:  "only special characters",
			input: " \t\n",
			expected: []expectedCall{
				{keys: "Space", literal: false},
				{keys: "Tab", literal: false},
				{keys: "Enter", literal: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTmuxSender{}
			h := NewHandler(WithTmuxSender(mock))

			err := h.SendInput("test-session", tt.input)
			if err != nil {
				t.Fatalf("SendInput failed: %v", err)
			}

			calls := mock.getCalls()
			if len(calls) != len(tt.expected) {
				t.Fatalf("got %d calls, want %d\ncalls: %+v\nexpected: %+v",
					len(calls), len(tt.expected), calls, tt.expected)
			}

			for i, exp := range tt.expected {
				if calls[i].keys != exp.keys {
					t.Errorf("call[%d].keys = %q, want %q", i, calls[i].keys, exp.keys)
				}
				if calls[i].literal != exp.literal {
					t.Errorf("call[%d].literal = %v, want %v", i, calls[i].literal, exp.literal)
				}
			}
		})
	}
}

// TestHandler_SendInput_BatchingEfficiency verifies that batching reduces
// the number of subprocess calls compared to character-by-character sending.
func TestHandler_SendInput_BatchingEfficiency(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		maxExpectedCalls  int
		unbatchedCalls    int // How many calls would be made without batching
		reductionExpected float64
	}{
		{
			name:              "typical typing",
			input:             "Hello, World!",
			maxExpectedCalls:  3,  // "Hello,", Space, "World!"
			unbatchedCalls:    13, // 13 characters
			reductionExpected: 0.76,
		},
		{
			name:              "long text no specials",
			input:             "abcdefghijklmnopqrstuvwxyz",
			maxExpectedCalls:  1,  // One batched call
			unbatchedCalls:    26, // 26 characters
			reductionExpected: 0.96,
		},
		{
			name:              "many words",
			input:             "the quick brown fox jumps",
			maxExpectedCalls:  9,  // 5 words + 4 spaces
			unbatchedCalls:    25, // 25 characters
			reductionExpected: 0.64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTmuxSender{}
			h := NewHandler(WithTmuxSender(mock))

			err := h.SendInput("test-session", tt.input)
			if err != nil {
				t.Fatalf("SendInput failed: %v", err)
			}

			calls := mock.getCalls()
			if len(calls) > tt.maxExpectedCalls {
				t.Errorf("got %d calls, want at most %d (batching should reduce calls)",
					len(calls), tt.maxExpectedCalls)
			}

			reduction := 1.0 - float64(len(calls))/float64(tt.unbatchedCalls)
			if reduction < tt.reductionExpected {
				t.Errorf("reduction = %.2f, want at least %.2f (got %d calls, unbatched would be %d)",
					reduction, tt.reductionExpected, len(calls), tt.unbatchedCalls)
			}
		})
	}
}

func TestHandler_isSpecialRune(t *testing.T) {
	h := NewHandler()

	tests := []struct {
		name     string
		r        rune
		expected bool
	}{
		{"regular lowercase", 'a', false},
		{"regular uppercase", 'Z', false},
		{"regular digit", '5', false},
		{"regular punctuation", '!', false},
		{"regular unicode", '中', false},
		{"newline", '\n', true},
		{"carriage return", '\r', true},
		{"tab", '\t', true},
		{"space", ' ', true},
		{"backspace 0x7f", '\x7f', true},
		{"backspace 0x08", '\b', true},
		{"escape", '\x1b', true},
		{"ctrl-a", '\x01', true},
		{"ctrl-c", '\x03', true},
		{"null", '\x00', true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.isSpecialRune(tt.r)
			if got != tt.expected {
				t.Errorf("isSpecialRune(%q) = %v, want %v", tt.r, got, tt.expected)
			}
		})
	}
}

// TestHandler_SendInput_ErrorInBatch verifies error handling during batched sends.
func TestHandler_SendInput_ErrorInBatch(t *testing.T) {
	t.Run("error on batch send", func(t *testing.T) {
		mock := &mockTmuxSender{}
		h := NewHandler(WithTmuxSender(mock))

		// Fail on first call (the batched regular chars)
		mock.setFailNext(fmt.Errorf("batch send error"))

		err := h.SendInput("test-session", "hello")
		if err == nil {
			t.Error("expected error when batch send fails")
		}
		if !strings.Contains(err.Error(), "batch send error") {
			t.Errorf("error should contain original error, got: %v", err)
		}
	})

	t.Run("error on special key send", func(t *testing.T) {
		mock := &mockTmuxSender{}
		h := NewHandler(WithTmuxSender(mock))

		// First call (batch) succeeds, second call (special key) fails
		err := h.SendInput("test-session", "a")
		if err != nil {
			t.Fatalf("first send failed: %v", err)
		}

		// Now set up to fail on special key
		mock.setFailNext(fmt.Errorf("special key error"))
		err = h.SendInput("test-session", "\n")
		if err == nil {
			t.Error("expected error when special key send fails")
		}
	})
}

// countingTmuxSender counts calls and fails at a specific call number.
type countingTmuxSender struct {
	mu        sync.Mutex
	calls     []sendCall
	callCount int
	failAt    int // Fail on this call number (1-indexed)
}

func (c *countingTmuxSender) SendKeys(sessionName string, keys string, literal bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callCount++
	c.calls = append(c.calls, sendCall{sessionName: sessionName, keys: keys, literal: literal})
	if c.callCount == c.failAt {
		return fmt.Errorf("mid-batch error at call %d", c.callCount)
	}
	return nil
}

// TestHandler_SendInput_ErrorMidBatch tests error handling when a batch is partially sent.
func TestHandler_SendInput_ErrorMidBatch(t *testing.T) {
	// First call succeeds (batch "hello"), then fail on special key (Enter)
	// The input "hello\nworld" should: send "hello", fail on Enter
	counter := &countingTmuxSender{failAt: 2}
	h := NewHandler(WithTmuxSender(counter))

	err := h.SendInput("test-session", "hello\nworld")
	if err == nil {
		t.Error("expected error when send fails mid-batch")
	}
	if !strings.Contains(err.Error(), "mid-batch error") {
		t.Errorf("error should contain 'mid-batch error', got: %v", err)
	}
}

func TestNewHandler_WithPersistentSender(t *testing.T) {
	// WithPersistentSender should set up a persistent sender
	h := NewHandler(WithPersistentSender("test-session"))

	if h == nil {
		t.Fatal("NewHandler returned nil")
	}

	if h.sender == nil {
		t.Error("sender should not be nil")
	}

	// The sender should be a PersistentTmuxSender
	_, ok := h.sender.(*PersistentTmuxSender)
	if !ok {
		t.Errorf("sender should be *PersistentTmuxSender, got %T", h.sender)
	}

	// Clean up
	_ = h.Close()
}

func TestHandler_Close(t *testing.T) {
	t.Run("with default sender", func(t *testing.T) {
		h := NewHandler()

		// Close should be safe to call with default sender (not a Closer)
		err := h.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})

	t.Run("with mock sender", func(t *testing.T) {
		mock := &mockTmuxSender{}
		h := NewHandler(WithTmuxSender(mock))

		// Close should be safe to call with mock (not a Closer)
		err := h.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})

	t.Run("with persistent sender", func(t *testing.T) {
		h := NewHandler(WithPersistentSender("test-session"))

		// Close should work with persistent sender
		err := h.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})
}

func TestHandler_Close_Multiple(t *testing.T) {
	h := NewHandler(WithPersistentSender("test-session"))

	// Multiple closes should be safe
	err := h.Close()
	if err != nil {
		t.Fatalf("First Close failed: %v", err)
	}

	err = h.Close()
	if err != nil {
		t.Fatalf("Second Close failed: %v", err)
	}
}
