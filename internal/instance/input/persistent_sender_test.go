package input

import (
	"errors"
	"sync"
	"testing"
)

func TestNewPersistentTmuxSender(t *testing.T) {
	p := NewPersistentTmuxSender("test-session")

	if p == nil {
		t.Fatal("NewPersistentTmuxSender returned nil")
	}

	if p.sessionName != "test-session" {
		t.Errorf("sessionName = %q, want %q", p.sessionName, "test-session")
	}

	if p.fallback == nil {
		t.Error("fallback should not be nil")
	}

	if p.connected {
		t.Error("should not be connected initially")
	}
}

func TestNewPersistentTmuxSender_WithFallbackSender(t *testing.T) {
	mock := &mockTmuxSender{}
	p := NewPersistentTmuxSender("test-session", WithFallbackSender(mock))

	if p.fallback != mock {
		t.Error("custom fallback sender not set")
	}
}

func TestPersistentTmuxSender_SendKeys_SessionMismatch(t *testing.T) {
	// When called with a different session name, should use fallback
	mock := &mockTmuxSender{}
	p := NewPersistentTmuxSender("session-a", WithFallbackSender(mock))

	err := p.SendKeys("session-b", "hello", true)
	if err != nil {
		t.Fatalf("SendKeys failed: %v", err)
	}

	calls := mock.getCalls()
	if len(calls) != 1 {
		t.Fatalf("got %d fallback calls, want 1", len(calls))
	}

	if calls[0].sessionName != "session-b" {
		t.Errorf("fallback session = %q, want %q", calls[0].sessionName, "session-b")
	}
	if calls[0].keys != "hello" {
		t.Errorf("fallback keys = %q, want %q", calls[0].keys, "hello")
	}
	if !calls[0].literal {
		t.Error("fallback literal = false, want true")
	}
}

func TestPersistentTmuxSender_SendKeys_FallbackOnConnectionError(t *testing.T) {
	// When connection fails, should fall back to subprocess sender
	mock := &mockTmuxSender{}
	p := NewPersistentTmuxSender("nonexistent-session-12345", WithFallbackSender(mock))

	// This should fail to connect (session doesn't exist) and use fallback
	err := p.SendKeys("nonexistent-session-12345", "hello", true)
	if err != nil {
		t.Fatalf("SendKeys failed: %v", err)
	}

	// Should have fallen back to mock
	calls := mock.getCalls()
	if len(calls) != 1 {
		t.Fatalf("got %d fallback calls, want 1", len(calls))
	}

	// Should not be marked as connected after failed connection
	if p.Connected() {
		t.Error("should not be connected after failed connection")
	}
}

func TestPersistentTmuxSender_SendKeys_FallbackPropagatesError(t *testing.T) {
	// When fallback fails, error should be propagated
	mock := &mockTmuxSender{}
	expectedErr := errors.New("mock error")
	mock.setFailNext(expectedErr)

	p := NewPersistentTmuxSender("nonexistent-session-12345", WithFallbackSender(mock))

	err := p.SendKeys("nonexistent-session-12345", "hello", true)
	if err == nil {
		t.Fatal("expected error from fallback, got nil")
	}
	if err != expectedErr {
		t.Errorf("got error %v, want %v", err, expectedErr)
	}
}

func TestPersistentTmuxSender_Close(t *testing.T) {
	p := NewPersistentTmuxSender("test-session")

	// Close should be safe to call even when not connected
	err := p.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Should not be connected after close
	if p.Connected() {
		t.Error("should not be connected after Close")
	}
}

func TestPersistentTmuxSender_Close_AfterConnect(t *testing.T) {
	// This test requires tmux to be available, so we just test the code path
	// The actual connection will fail without a real session, but Close should
	// handle that gracefully
	p := NewPersistentTmuxSender("test-session")

	// Simulate a connected state by attempting connection (which will fail)
	// and then closing
	_ = p.SendKeys("test-session", "hello", true)

	err := p.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if p.Connected() {
		t.Error("should not be connected after Close")
	}
}

func TestPersistentTmuxSender_ConcurrentAccess(t *testing.T) {
	mock := &mockTmuxSender{}
	p := NewPersistentTmuxSender("nonexistent-session", WithFallbackSender(mock))

	// Run multiple goroutines calling SendKeys concurrently
	var wg sync.WaitGroup
	numGoroutines := 10
	callsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				_ = p.SendKeys("nonexistent-session", "x", true)
			}
		}(i)
	}

	wg.Wait()

	// All calls should have been made (via fallback since connection fails)
	calls := mock.getCalls()
	expectedCalls := numGoroutines * callsPerGoroutine
	if len(calls) != expectedCalls {
		t.Errorf("got %d calls, want %d", len(calls), expectedCalls)
	}
}

func TestEscapeForControlMode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "string with space",
			input:    "hello world",
			expected: "'hello world'",
		},
		{
			name:     "string with single quote",
			input:    "it's",
			expected: "'it'\\''s'",
		},
		{
			name:     "string with double quote",
			input:    `say "hello"`,
			expected: `'say "hello"'`,
		},
		{
			name:     "string with newline",
			input:    "hello\nworld",
			expected: "'hello\nworld'",
		},
		{
			name:     "string with tab",
			input:    "hello\tworld",
			expected: "'hello\tworld'",
		},
		{
			name:     "string with backslash",
			input:    "path\\to\\file",
			expected: "'path\\to\\file'",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "complex string",
			input:    "echo 'hello' | grep 'world'",
			expected: "'echo '\\''hello'\\'' | grep '\\''world'\\'''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeForControlMode(tt.input)
			if got != tt.expected {
				t.Errorf("escapeForControlMode(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPersistentTmuxSender_Connected(t *testing.T) {
	p := NewPersistentTmuxSender("test-session")

	// Initially not connected
	if p.Connected() {
		t.Error("should not be connected initially")
	}

	// After failed connection attempt (no real tmux session), should still be false
	_ = p.SendKeys("test-session", "hello", true)
	if p.Connected() {
		t.Error("should not be connected after failed connection")
	}

	// After close, should still be false
	_ = p.Close()
	if p.Connected() {
		t.Error("should not be connected after Close")
	}
}

// BenchmarkPersistentTmuxSender_SendKeys_Fallback benchmarks the fallback path
// which is used when connection fails. This measures the overhead of the
// persistent sender's locking and connection checking.
func BenchmarkPersistentTmuxSender_SendKeys_Fallback(b *testing.B) {
	mock := &mockTmuxSender{}
	p := NewPersistentTmuxSender("nonexistent-session", WithFallbackSender(mock))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.SendKeys("nonexistent-session", "x", true)
	}
}

// BenchmarkDefaultTmuxSender_SendKeys_Mock benchmarks the mock sender directly
// for comparison with the persistent sender overhead.
func BenchmarkDefaultTmuxSender_SendKeys_Mock(b *testing.B) {
	mock := &mockTmuxSender{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mock.SendKeys("test-session", "x", true)
	}
}
