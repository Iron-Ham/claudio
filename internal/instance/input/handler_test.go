package input

import (
	"fmt"
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
	if len(calls) != 5 { // 5 characters in "hello"
		t.Errorf("got %d calls, want 5", len(calls))
	}

	expected := []string{"h", "e", "l", "l", "o"}
	for i, exp := range expected {
		if calls[i].keys != exp {
			t.Errorf("call[%d].keys = %q, want %q", i, calls[i].keys, exp)
		}
		if calls[i].sessionName != "test-session" {
			t.Errorf("call[%d].sessionName = %q, want %q", i, calls[i].sessionName, "test-session")
		}
		if !calls[i].literal {
			t.Errorf("call[%d].literal = false, want true", i)
		}
	}
}

func TestHandler_SendInput_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "newline",
			input:    "\n",
			expected: []string{"Enter"},
		},
		{
			name:     "carriage return",
			input:    "\r",
			expected: []string{"Enter"},
		},
		{
			name:     "tab",
			input:    "\t",
			expected: []string{"Tab"},
		},
		{
			name:     "backspace",
			input:    "\x7f",
			expected: []string{"BSpace"},
		},
		{
			name:     "escape",
			input:    "\x1b",
			expected: []string{"Escape"},
		},
		{
			name:     "space",
			input:    " ",
			expected: []string{"Space"},
		},
		{
			name:     "control character (ctrl-a)",
			input:    "\x01",
			expected: []string{"C-a"},
		},
		{
			name:     "mixed input",
			input:    "a\nb",
			expected: []string{"a", "Enter", "b"},
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
				t.Fatalf("got %d calls, want %d", len(calls), len(tt.expected))
			}

			for i, exp := range tt.expected {
				if calls[i].keys != exp {
					t.Errorf("call[%d].keys = %q, want %q", i, calls[i].keys, exp)
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
	if len(calls) != 3 { // 3 characters
		t.Errorf("got %d calls, want 3", len(calls))
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
