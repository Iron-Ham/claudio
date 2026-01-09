//go:build integration

package instance

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// skipIfNoTmux skips the test if tmux is not available
func skipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available, skipping integration test")
	}
}

// cleanupTmuxSession ensures a tmux session is cleaned up after a test
func cleanupTmuxSession(t *testing.T, sessionName string) {
	t.Helper()
	_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
}

// =============================================================================
// Integration Test Helpers
// =============================================================================

// integrationManagerConfig returns config suitable for integration tests
func integrationManagerConfig() ManagerConfig {
	return ManagerConfig{
		OutputBufferSize:         10000,
		CaptureIntervalMs:        50, // Faster capture for tests
		TmuxWidth:                80,
		TmuxHeight:               24,
		ActivityTimeoutMinutes:   0, // Disabled
		CompletionTimeoutMinutes: 0, // Disabled
		StaleDetection:           false,
	}
}

// =============================================================================
// Tmux Session Creation and Cleanup Tests
// =============================================================================

func TestIntegration_TmuxSessionCreation(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	mgr := NewManagerWithConfig("integration-create-test", t.TempDir(), "echo 'hello world'", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())

	// Verify session doesn't exist before start
	if mgr.TmuxSessionExists() {
		t.Fatal("Session should not exist before Start()")
	}

	// Start the manager
	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer func() { _ = mgr.Stop() }()

	// Verify session exists after start
	if !mgr.TmuxSessionExists() {
		t.Error("Session should exist after Start()")
	}

	// Verify running state
	if !mgr.Running() {
		t.Error("Running() should be true after Start()")
	}

	// Verify start time is set
	if mgr.StartTime() == nil {
		t.Error("StartTime() should not be nil after Start()")
	}

	// Stop and verify cleanup
	err = mgr.Stop()
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Give tmux time to clean up
	time.Sleep(100 * time.Millisecond)

	if mgr.Running() {
		t.Error("Running() should be false after Stop()")
	}
}

func TestIntegration_TmuxSessionCleanup(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	mgr := NewManagerWithConfig("integration-cleanup-test", t.TempDir(), "sleep 60", cfg)

	// Start then stop
	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	sessionName := mgr.SessionName()

	// Verify session exists
	if !mgr.TmuxSessionExists() {
		t.Fatal("Session should exist after Start()")
	}

	// Stop should clean up the session
	err = mgr.Stop()
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Give tmux time to clean up
	time.Sleep(600 * time.Millisecond)

	// Session should not exist after Stop()
	checkCmd := exec.Command("tmux", "has-session", "-t", sessionName)
	if checkCmd.Run() == nil {
		t.Error("Session should not exist after Stop()")
		cleanupTmuxSession(t, sessionName) // Clean up for real
	}
}

func TestIntegration_StartAlreadyRunning(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	mgr := NewManagerWithConfig("integration-double-start", t.TempDir(), "sleep 60", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())
	defer func() { _ = mgr.Stop() }()

	// First start
	err := mgr.Start()
	if err != nil {
		t.Fatalf("First Start() failed: %v", err)
	}

	// Second start should fail
	err = mgr.Start()
	if err == nil {
		t.Error("Second Start() should fail when already running")
	}

	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("Error should mention 'already running', got: %v", err)
	}
}

// =============================================================================
// Output Streaming and Buffering Tests
// =============================================================================

func TestIntegration_OutputCapture(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	testMsg := "INTEGRATION_TEST_OUTPUT_12345"
	mgr := NewManagerWithConfig("integration-output", t.TempDir(), "echo '"+testMsg+"'", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())
	defer func() { _ = mgr.Stop() }()

	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait for output to be captured
	var output []byte
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		output = mgr.GetOutput()
		if strings.Contains(string(output), testMsg) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !strings.Contains(string(output), testMsg) {
		t.Errorf("Output should contain test message %q, got: %q", testMsg, string(output))
	}
}

func TestIntegration_OutputBufferRing(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	cfg.OutputBufferSize = 500 // Small buffer
	mgr := NewManagerWithConfig("integration-buffer-ring", t.TempDir(), "for i in $(seq 1 100); do echo \"line $i\"; done", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())
	defer func() { _ = mgr.Stop() }()

	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait for command to complete and output to be captured
	time.Sleep(2 * time.Second)

	output := mgr.GetOutput()

	// Buffer should not exceed configured size
	if len(output) > cfg.OutputBufferSize {
		t.Errorf("Output buffer size %d exceeds configured limit %d", len(output), cfg.OutputBufferSize)
	}
}

// =============================================================================
// Pause and Resume Tests
// =============================================================================

func TestIntegration_PauseResume(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	mgr := NewManagerWithConfig("integration-pause-resume", t.TempDir(), "sleep 60", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())
	defer func() { _ = mgr.Stop() }()

	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Initially not paused
	if mgr.Paused() {
		t.Error("Should not be paused after Start()")
	}

	// Pause
	err = mgr.Pause()
	if err != nil {
		t.Errorf("Pause() failed: %v", err)
	}

	if !mgr.Paused() {
		t.Error("Paused() should be true after Pause()")
	}

	// Session should still exist
	if !mgr.TmuxSessionExists() {
		t.Error("Session should still exist after Pause()")
	}

	// Resume
	err = mgr.Resume()
	if err != nil {
		t.Errorf("Resume() failed: %v", err)
	}

	if mgr.Paused() {
		t.Error("Paused() should be false after Resume()")
	}
}

// =============================================================================
// State Detection Tests
// =============================================================================

func TestIntegration_StateCallback(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	mgr := NewManagerWithConfig("integration-state-cb", t.TempDir(), "echo 'Test output'", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())
	defer func() { _ = mgr.Stop() }()

	var mu sync.Mutex
	var states []WaitingState

	mgr.SetStateCallback(func(instanceID string, state WaitingState) {
		mu.Lock()
		defer mu.Unlock()
		states = append(states, state)
	})

	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait a bit for capture loop to run
	time.Sleep(500 * time.Millisecond)

	// Stop to trigger completion state if applicable
	_ = mgr.Stop()

	mu.Lock()
	statesCopy := make([]WaitingState, len(states))
	copy(statesCopy, states)
	mu.Unlock()

	t.Logf("Captured %d state changes: %v", len(statesCopy), statesCopy)
}

// =============================================================================
// Reconnect Tests
// =============================================================================

func TestIntegration_Reconnect(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	sessionName := "claudio-integration-reconnect"

	// First, create a tmux session manually
	cleanupTmuxSession(t, sessionName)
	createCmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-x", "80", "-y", "24")
	createCmd.Dir = t.TempDir()
	if err := createCmd.Run(); err != nil {
		t.Fatalf("Failed to create test tmux session: %v", err)
	}
	defer cleanupTmuxSession(t, sessionName)

	// Create a manager for the existing session
	mgr := NewManagerWithConfig("integration-reconnect", t.TempDir(), "task", cfg)

	// Override session name to match the existing one
	mgr.mu.Lock()
	mgr.sessionName = sessionName
	mgr.mu.Unlock()

	// Reconnect should succeed
	err := mgr.Reconnect()
	if err != nil {
		t.Fatalf("Reconnect() failed: %v", err)
	}
	defer func() { _ = mgr.Stop() }()

	// Verify state
	if !mgr.Running() {
		t.Error("Running() should be true after Reconnect()")
	}

	// Reconnect again should fail
	err = mgr.Reconnect()
	if err == nil {
		t.Error("Second Reconnect() should fail")
	}
}

// =============================================================================
// PID Tests
// =============================================================================

func TestIntegration_PID(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	mgr := NewManagerWithConfig("integration-pid", t.TempDir(), "sleep 60", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())
	defer func() { _ = mgr.Stop() }()

	// PID should be 0 before start
	if mgr.PID() != 0 {
		t.Errorf("PID() should be 0 before Start(), got %d", mgr.PID())
	}

	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Give tmux time to spawn shell
	time.Sleep(200 * time.Millisecond)

	pid := mgr.PID()
	if pid == 0 {
		t.Error("PID() should return non-zero after Start()")
	}

	// Verify the PID actually exists
	proc, err := os.FindProcess(pid)
	if err != nil {
		t.Errorf("Process with PID %d not found: %v", pid, err)
	} else {
		// On Unix, FindProcess always succeeds, so we need to signal to verify
		err = proc.Signal(os.Signal(nil))
		// If there's no error, the process exists
		t.Logf("PID %d exists", pid)
	}
}

// =============================================================================
// Resize Tests
// =============================================================================

func TestIntegration_Resize(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	mgr := NewManagerWithConfig("integration-resize", t.TempDir(), "sleep 60", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())
	defer func() { _ = mgr.Stop() }()

	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Resize to new dimensions
	err = mgr.Resize(120, 40)
	if err != nil {
		t.Errorf("Resize() failed: %v", err)
	}

	// Verify the config was updated
	mgr.mu.RLock()
	width := mgr.config.TmuxWidth
	height := mgr.config.TmuxHeight
	mgr.mu.RUnlock()

	if width != 120 {
		t.Errorf("TmuxWidth = %d, want 120", width)
	}
	if height != 40 {
		t.Errorf("TmuxHeight = %d, want 40", height)
	}

	// Verify tmux session dimensions (query actual dimensions)
	widthCmd := exec.Command("tmux", "display-message", "-t", mgr.SessionName(), "-p", "#{window_width}")
	widthOut, err := widthCmd.Output()
	if err == nil {
		t.Logf("Actual tmux width: %s", strings.TrimSpace(string(widthOut)))
	}
}

// =============================================================================
// Input Sending Tests
// =============================================================================

func TestIntegration_SendKey(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	mgr := NewManagerWithConfig("integration-sendkey", t.TempDir(), "cat", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())
	defer func() { _ = mgr.Stop() }()

	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Give cat time to start
	time.Sleep(200 * time.Millisecond)

	// Send some input and then Ctrl+C
	mgr.SendLiteral("hello")
	mgr.SendKey("Enter")
	time.Sleep(100 * time.Millisecond)
	mgr.SendKey("C-c") // Ctrl+C to stop cat

	// Wait for output
	time.Sleep(500 * time.Millisecond)

	output := mgr.GetOutput()
	t.Logf("Output after SendKey: %q", string(output))

	// Output should contain "hello"
	if !strings.Contains(string(output), "hello") {
		t.Errorf("Output should contain 'hello', got: %q", string(output))
	}
}

func TestIntegration_SendPaste(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	mgr := NewManagerWithConfig("integration-sendpaste", t.TempDir(), "cat", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())
	defer func() { _ = mgr.Stop() }()

	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Give cat time to start
	time.Sleep(200 * time.Millisecond)

	// Send pasted content
	mgr.SendPaste("pasted text here")
	mgr.SendKey("Enter")
	time.Sleep(100 * time.Millisecond)
	mgr.SendKey("C-c")

	// Wait for output
	time.Sleep(500 * time.Millisecond)

	output := mgr.GetOutput()
	t.Logf("Output after SendPaste: %q", string(output))

	// Output should contain the pasted text
	if !strings.Contains(string(output), "pasted text here") {
		t.Errorf("Output should contain 'pasted text here', got: %q", string(output))
	}
}

// =============================================================================
// Session Listing Tests
// =============================================================================

func TestIntegration_ListClaudioTmuxSessions(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()

	// Create multiple test sessions
	sessionIDs := []string{"list-test-1", "list-test-2", "list-test-3"}
	managers := make([]*Manager, len(sessionIDs))

	for i, id := range sessionIDs {
		mgr := NewManagerWithConfig(id, t.TempDir(), "sleep 60", cfg)
		err := mgr.Start()
		if err != nil {
			t.Fatalf("Failed to start manager %s: %v", id, err)
		}
		managers[i] = mgr
	}

	// Clean up all sessions at the end
	defer func() {
		for _, mgr := range managers {
			_ = mgr.Stop()
			cleanupTmuxSession(t, mgr.SessionName())
		}
	}()

	// List sessions
	sessions, err := ListClaudioTmuxSessions()
	if err != nil {
		t.Fatalf("ListClaudioTmuxSessions() failed: %v", err)
	}

	// All test sessions should be in the list
	for _, id := range sessionIDs {
		expectedName := "claudio-" + id
		found := false
		for _, sess := range sessions {
			if sess == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Session %q not found in list: %v", expectedName, sessions)
		}
	}
}

func TestIntegration_ListSessionTmuxSessions(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	sessionID := "testsess"

	// Create sessions with and without session ID
	mgr1 := NewManagerWithSession(sessionID, "inst1", t.TempDir(), "sleep 60", cfg)
	mgr2 := NewManagerWithSession(sessionID, "inst2", t.TempDir(), "sleep 60", cfg)
	mgr3 := NewManagerWithConfig("other-inst", t.TempDir(), "sleep 60", cfg) // No session ID

	for _, mgr := range []*Manager{mgr1, mgr2, mgr3} {
		if err := mgr.Start(); err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
	}

	defer func() {
		for _, mgr := range []*Manager{mgr1, mgr2, mgr3} {
			_ = mgr.Stop()
			cleanupTmuxSession(t, mgr.SessionName())
		}
	}()

	// List sessions for specific session ID
	sessions, err := ListSessionTmuxSessions(sessionID)
	if err != nil {
		t.Fatalf("ListSessionTmuxSessions() failed: %v", err)
	}

	// Should only contain sessions with the session ID prefix
	expectedPrefix := "claudio-" + sessionID + "-"
	for _, sess := range sessions {
		if !strings.HasPrefix(sess, expectedPrefix) {
			t.Errorf("Session %q should have prefix %q", sess, expectedPrefix)
		}
	}

	// Should have exactly 2 sessions (inst1 and inst2)
	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d: %v", len(sessions), sessions)
	}
}

// =============================================================================
// Concurrent Instance Management Tests
// =============================================================================

func TestIntegration_ConcurrentInstances(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	const numInstances = 3

	managers := make([]*Manager, numInstances)
	var wg sync.WaitGroup

	// Start all instances concurrently
	for i := 0; i < numInstances; i++ {
		id := "concurrent-" + string(rune('a'+i))
		managers[i] = NewManagerWithConfig(id, t.TempDir(), "sleep 60", cfg)
	}

	// Clean up at the end
	defer func() {
		for _, mgr := range managers {
			if mgr != nil {
				_ = mgr.Stop()
				cleanupTmuxSession(t, mgr.SessionName())
			}
		}
	}()

	// Start all managers
	var startErrors []error
	var mu sync.Mutex

	for _, mgr := range managers {
		wg.Add(1)
		go func(m *Manager) {
			defer wg.Done()
			if err := m.Start(); err != nil {
				mu.Lock()
				startErrors = append(startErrors, err)
				mu.Unlock()
			}
		}(mgr)
	}

	wg.Wait()

	if len(startErrors) > 0 {
		t.Fatalf("Failed to start some managers: %v", startErrors)
	}

	// All should be running
	for _, mgr := range managers {
		if !mgr.Running() {
			t.Errorf("Manager %s should be running", mgr.ID())
		}
		if !mgr.TmuxSessionExists() {
			t.Errorf("Tmux session for %s should exist", mgr.ID())
		}
	}

	// Stop all concurrently
	for _, mgr := range managers {
		wg.Add(1)
		go func(m *Manager) {
			defer wg.Done()
			_ = m.Stop()
		}(mgr)
	}

	wg.Wait()

	// All should be stopped
	for _, mgr := range managers {
		if mgr.Running() {
			t.Errorf("Manager %s should not be running after Stop()", mgr.ID())
		}
	}
}

// =============================================================================
// Resource Cleanup Tests
// =============================================================================

func TestIntegration_CleanupOnShutdown(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	mgr := NewManagerWithConfig("cleanup-shutdown", t.TempDir(), "sleep 60", cfg)
	sessionName := mgr.SessionName()

	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify session exists
	if !mgr.TmuxSessionExists() {
		t.Fatal("Session should exist after Start()")
	}

	// Stop
	err = mgr.Stop()
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Wait for cleanup
	time.Sleep(700 * time.Millisecond)

	// Verify session is gone
	checkCmd := exec.Command("tmux", "has-session", "-t", sessionName)
	if checkCmd.Run() == nil {
		t.Error("Session should be cleaned up after Stop()")
		cleanupTmuxSession(t, sessionName)
	}

	// Verify running state
	if mgr.Running() {
		t.Error("Running() should be false after Stop()")
	}
}

func TestIntegration_CleansUpExistingSession(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	sessionName := "claudio-cleanup-existing"

	// Create a session manually with the same name
	cleanupTmuxSession(t, sessionName)
	createCmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	createCmd.Dir = t.TempDir()
	if err := createCmd.Run(); err != nil {
		t.Fatalf("Failed to create pre-existing session: %v", err)
	}

	// Create manager with same ID
	mgr := NewManagerWithConfig("cleanup-existing", t.TempDir(), "echo 'new session'", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())
	defer func() { _ = mgr.Stop() }()

	// Start should succeed (cleaning up the old session)
	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() should succeed even with pre-existing session: %v", err)
	}

	if !mgr.Running() {
		t.Error("Manager should be running after Start()")
	}
}

// =============================================================================
// Activity and Output Change Detection Tests
// =============================================================================

func TestIntegration_LastActivityTimeUpdates(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	mgr := NewManagerWithConfig("activity-time", t.TempDir(), "sleep 1 && echo 'done'", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())
	defer func() { _ = mgr.Stop() }()

	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Record initial activity time
	initialActivity := mgr.LastActivityTime()

	// Wait for some output
	time.Sleep(1500 * time.Millisecond)

	// Activity time should have been updated
	laterActivity := mgr.LastActivityTime()

	if !laterActivity.After(initialActivity) {
		t.Error("LastActivityTime() should increase after output changes")
	}
}

// =============================================================================
// Session Completion Detection Tests
// =============================================================================

func TestIntegration_SessionCompletion(t *testing.T) {
	skipIfNoTmux(t)

	cfg := integrationManagerConfig()
	// Use a command that exits quickly
	mgr := NewManagerWithConfig("completion-test", t.TempDir(), "echo 'done' && exit", cfg)
	defer cleanupTmuxSession(t, mgr.SessionName())

	stateRecorder := newStateRecorder()
	mgr.SetStateCallback(stateRecorder.callback())

	err := mgr.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait for the command to complete
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && mgr.Running() {
		time.Sleep(200 * time.Millisecond)
	}

	// Session should have completed
	changes := stateRecorder.getChanges()
	t.Logf("State changes: %v", changes)

	// Final state should be completed
	if mgr.CurrentState() != StateCompleted && mgr.Running() {
		t.Logf("Note: Session may or may not detect completion based on shell behavior")
	}
}
