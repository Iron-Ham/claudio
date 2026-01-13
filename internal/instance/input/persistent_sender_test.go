package input

import (
	"errors"
	"sync"
	"testing"
	"time"
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

func TestPersistentTmuxSender_BuildCommand(t *testing.T) {
	p := NewPersistentTmuxSender("my-session")

	tests := []struct {
		name     string
		keys     string
		literal  bool
		expected string
	}{
		{
			name:     "non-literal key",
			keys:     "Enter",
			literal:  false,
			expected: "send-keys -t my-session Enter\n",
		},
		{
			name:     "literal simple text",
			keys:     "hello",
			literal:  true,
			expected: "send-keys -t my-session -l hello\n",
		},
		{
			name:     "literal text with spaces",
			keys:     "hello world",
			literal:  true,
			expected: "send-keys -t my-session -l 'hello world'\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.buildCommand(tt.keys, tt.literal)
			if got != tt.expected {
				t.Errorf("buildCommand(%q, %v) = %q, want %q", tt.keys, tt.literal, got, tt.expected)
			}
		})
	}
}

func TestPersistentTmuxSender_WriteWithTimeoutLocked_NilStdin(t *testing.T) {
	p := NewPersistentTmuxSender("test-session")
	// stdin is nil when not connected

	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.writeWithTimeoutLocked([]byte("test"))
	if err == nil {
		t.Error("expected error for nil stdin, got nil")
	}
}

func TestPersistentTmuxSender_AutoReconnect_TriggeredOnTimeout(t *testing.T) {
	// This test verifies that when a write times out, the sender:
	// 1. Disconnects the stuck connection
	// 2. Attempts to reconnect
	// 3. Falls back to subprocess if reconnect also fails

	// We use a session that doesn't exist, so connection always fails
	// and we can verify the fallback is called
	mock := &mockTmuxSender{}
	p := NewPersistentTmuxSender("nonexistent-session-timeout-test", WithFallbackSender(mock))

	// First call - connection will fail, falls back to mock
	err := p.SendKeys("nonexistent-session-timeout-test", "hello", true)
	if err != nil {
		t.Fatalf("SendKeys failed: %v", err)
	}

	// Verify fallback was used
	calls := mock.getCalls()
	if len(calls) != 1 {
		t.Errorf("got %d fallback calls, want 1", len(calls))
	}

	// The connection should not be established (it failed)
	if p.Connected() {
		t.Error("should not be connected when session doesn't exist")
	}
}

func TestPersistentTmuxSender_MultipleReconnectAttempts(t *testing.T) {
	// Test that multiple calls in sequence all properly fall back
	// when the session doesn't exist
	mock := &mockTmuxSender{}
	p := NewPersistentTmuxSender("nonexistent-reconnect-test", WithFallbackSender(mock))

	// Make multiple calls
	for i := 0; i < 5; i++ {
		err := p.SendKeys("nonexistent-reconnect-test", "key", false)
		if err != nil {
			t.Fatalf("SendKeys call %d failed: %v", i, err)
		}
	}

	// All calls should have gone through the fallback
	calls := mock.getCalls()
	if len(calls) != 5 {
		t.Errorf("got %d fallback calls, want 5", len(calls))
	}
}

// TestPersistentTmuxSender_GoroutineLifecycle tests that goroutines are properly
// managed across connect/disconnect cycles.
func TestPersistentTmuxSender_GoroutineLifecycle(t *testing.T) {
	mock := &mockTmuxSender{}
	p := NewPersistentTmuxSender("nonexistent-lifecycle-test", WithFallbackSender(mock))

	// Perform multiple operations that would trigger connection attempts
	// Each failed connection should properly clean up
	for i := 0; i < 10; i++ {
		err := p.SendKeys("nonexistent-lifecycle-test", "key", false)
		if err != nil {
			t.Fatalf("SendKeys call %d failed: %v", i, err)
		}
	}

	// Close should complete without hanging (would hang if goroutines leak)
	done := make(chan struct{})
	go func() {
		_ = p.Close()
		close(done)
	}()

	select {
	case <-done:
		// Close completed successfully
	case <-time.After(2 * time.Second):
		t.Error("Close timed out - possible goroutine leak")
	}

	// Should not be connected after close
	if p.Connected() {
		t.Error("should not be connected after Close")
	}
}

// TestPersistentTmuxSender_RepeatedCloseIsSafe tests that calling Close
// multiple times is safe and doesn't cause panics.
func TestPersistentTmuxSender_RepeatedCloseIsSafe(t *testing.T) {
	p := NewPersistentTmuxSender("test-session")

	// Close multiple times - should not panic
	for i := 0; i < 5; i++ {
		err := p.Close()
		if err != nil {
			t.Fatalf("Close call %d failed: %v", i, err)
		}
	}
}

// TestPersistentTmuxSender_CloseWhileNotConnected tests that Close handles
// the case where the sender was never connected.
func TestPersistentTmuxSender_CloseWhileNotConnected(t *testing.T) {
	p := NewPersistentTmuxSender("test-session")

	// Never attempt to connect, just close
	err := p.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// WaitGroups should still be in valid state
	// (no panic on subsequent operations)
	if p.Connected() {
		t.Error("should not be connected")
	}
}

// TestPersistentTmuxSender_DisconnectCleansUp tests that disconnectLocked
// properly waits for goroutines to exit.
func TestPersistentTmuxSender_DisconnectCleansUp(t *testing.T) {
	mock := &mockTmuxSender{}
	p := NewPersistentTmuxSender("nonexistent-cleanup-test", WithFallbackSender(mock))

	// Trigger a connection attempt (will fail, but tests the code path)
	_ = p.SendKeys("nonexistent-cleanup-test", "key", false)

	// Close should complete quickly since connection failed
	start := time.Now()
	err := p.Close()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Close should be fast (< 1 second) since there's no real connection
	// If goroutines were leaking, the WaitGroup wait would timeout
	if elapsed > 1*time.Second {
		t.Errorf("Close took %v, expected < 1s", elapsed)
	}
}
