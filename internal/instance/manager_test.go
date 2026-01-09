package instance

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Test Helpers
// =============================================================================

// testManagerConfig returns a configuration suitable for unit testing
// with short intervals to speed up tests
func testManagerConfig() ManagerConfig {
	return ManagerConfig{
		OutputBufferSize:         1000,
		CaptureIntervalMs:        10, // Fast capture for tests
		TmuxWidth:                80,
		TmuxHeight:               24,
		ActivityTimeoutMinutes:   0, // Disabled for unit tests
		CompletionTimeoutMinutes: 0, // Disabled for unit tests
		StaleDetection:           false,
	}
}

// testManagerConfigWithTimeouts returns config with short timeouts for testing
func testManagerConfigWithTimeouts() ManagerConfig {
	cfg := testManagerConfig()
	cfg.ActivityTimeoutMinutes = 1    // 1 minute for testing
	cfg.CompletionTimeoutMinutes = 2  // 2 minutes for testing
	cfg.StaleDetection = true
	return cfg
}

// newTestManager creates a manager suitable for unit testing
func newTestManager(t *testing.T, id string) *Manager {
	t.Helper()
	return NewManagerWithConfig(id, t.TempDir(), "test task", testManagerConfig())
}

// newTestManagerWithTask creates a manager with a specific task
func newTestManagerWithTask(t *testing.T, id, task string) *Manager {
	t.Helper()
	return NewManagerWithConfig(id, t.TempDir(), task, testManagerConfig())
}

// stateRecorder helps track state changes in tests
type stateRecorder struct {
	mu      sync.Mutex
	changes []stateChange
}

type stateChange struct {
	instanceID string
	state      WaitingState
	timestamp  time.Time
}

func newStateRecorder() *stateRecorder {
	return &stateRecorder{
		changes: make([]stateChange, 0),
	}
}

func (r *stateRecorder) callback() StateChangeCallback {
	return func(instanceID string, state WaitingState) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.changes = append(r.changes, stateChange{
			instanceID: instanceID,
			state:      state,
			timestamp:  time.Now(),
		})
	}
}

func (r *stateRecorder) getChanges() []stateChange {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]stateChange, len(r.changes))
	copy(result, r.changes)
	return result
}

func (r *stateRecorder) lastState() (WaitingState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.changes) == 0 {
		return StateWorking, false
	}
	return r.changes[len(r.changes)-1].state, true
}

// metricsRecorder helps track metrics changes in tests
type metricsRecorder struct {
	mu      sync.Mutex
	changes []metricsChange
}

type metricsChange struct {
	instanceID string
	metrics    *ParsedMetrics
	timestamp  time.Time
}

func newMetricsRecorder() *metricsRecorder {
	return &metricsRecorder{
		changes: make([]metricsChange, 0),
	}
}

func (r *metricsRecorder) callback() MetricsChangeCallback {
	return func(instanceID string, metrics *ParsedMetrics) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.changes = append(r.changes, metricsChange{
			instanceID: instanceID,
			metrics:    metrics,
			timestamp:  time.Now(),
		})
	}
}

func (r *metricsRecorder) getChanges() []metricsChange {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]metricsChange, len(r.changes))
	copy(result, r.changes)
	return result
}

// timeoutRecorder helps track timeout callbacks in tests
type timeoutRecorder struct {
	mu       sync.Mutex
	timeouts []timeoutEvent
}

type timeoutEvent struct {
	instanceID  string
	timeoutType TimeoutType
	timestamp   time.Time
}

func newTimeoutRecorder() *timeoutRecorder {
	return &timeoutRecorder{
		timeouts: make([]timeoutEvent, 0),
	}
}

func (r *timeoutRecorder) callback() TimeoutCallback {
	return func(instanceID string, timeoutType TimeoutType) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.timeouts = append(r.timeouts, timeoutEvent{
			instanceID:  instanceID,
			timeoutType: timeoutType,
			timestamp:   time.Now(),
		})
	}
}

func (r *timeoutRecorder) getTimeouts() []timeoutEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]timeoutEvent, len(r.timeouts))
	copy(result, r.timeouts)
	return result
}

// bellRecorder helps track bell callbacks in tests
type bellRecorder struct {
	mu    sync.Mutex
	bells []bellEvent
}

type bellEvent struct {
	instanceID string
	timestamp  time.Time
}

func newBellRecorder() *bellRecorder {
	return &bellRecorder{
		bells: make([]bellEvent, 0),
	}
}

func (r *bellRecorder) callback() BellCallback {
	return func(instanceID string) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.bells = append(r.bells, bellEvent{
			instanceID: instanceID,
			timestamp:  time.Now(),
		})
	}
}

func (r *bellRecorder) getBells() []bellEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]bellEvent, len(r.bells))
	copy(result, r.bells)
	return result
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestExtractInstanceIDFromSession(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		expected    string
	}{
		{
			name:        "valid claudio session",
			sessionName: "claudio-abc123",
			expected:    "abc123",
		},
		{
			name:        "valid claudio session with longer ID",
			sessionName: "claudio-a1b2c3d4",
			expected:    "a1b2c3d4",
		},
		{
			name:        "new format with session ID",
			sessionName: "claudio-sess1234-inst5678",
			expected:    "inst5678",
		},
		{
			name:        "PR workflow session (legacy)",
			sessionName: "claudio-abc12345-pr",
			expected:    "abc12345",
		},
		{
			name:        "PR workflow session (new format)",
			sessionName: "claudio-sess1234-inst5678-pr",
			expected:    "inst5678",
		},
		{
			name:        "non-claudio session",
			sessionName: "other-session",
			expected:    "",
		},
		{
			name:        "empty string",
			sessionName: "",
			expected:    "",
		},
		{
			name:        "just prefix",
			sessionName: "claudio-",
			expected:    "",
		},
		{
			name:        "similar but not claudio prefix",
			sessionName: "claudio2-abc",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractInstanceIDFromSession(tt.sessionName)
			if result != tt.expected {
				t.Errorf("ExtractInstanceIDFromSession(%q) = %q, want %q", tt.sessionName, result, tt.expected)
			}
		})
	}
}

func TestExtractSessionAndInstanceID(t *testing.T) {
	tests := []struct {
		name               string
		sessionName        string
		expectedSessionID  string
		expectedInstanceID string
	}{
		{
			name:               "legacy format",
			sessionName:        "claudio-abc12345",
			expectedSessionID:  "",
			expectedInstanceID: "abc12345",
		},
		{
			name:               "new format with session",
			sessionName:        "claudio-sess1234-inst5678",
			expectedSessionID:  "sess1234",
			expectedInstanceID: "inst5678",
		},
		{
			name:               "PR workflow legacy",
			sessionName:        "claudio-abc12345-pr",
			expectedSessionID:  "",
			expectedInstanceID: "abc12345",
		},
		{
			name:               "PR workflow new format",
			sessionName:        "claudio-sess1234-inst5678-pr",
			expectedSessionID:  "sess1234",
			expectedInstanceID: "inst5678",
		},
		{
			name:               "non-claudio session",
			sessionName:        "other-session",
			expectedSessionID:  "",
			expectedInstanceID: "",
		},
		{
			name:               "empty string",
			sessionName:        "",
			expectedSessionID:  "",
			expectedInstanceID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID, instanceID := ExtractSessionAndInstanceID(tt.sessionName)
			if sessionID != tt.expectedSessionID {
				t.Errorf("ExtractSessionAndInstanceID(%q) sessionID = %q, want %q", tt.sessionName, sessionID, tt.expectedSessionID)
			}
			if instanceID != tt.expectedInstanceID {
				t.Errorf("ExtractSessionAndInstanceID(%q) instanceID = %q, want %q", tt.sessionName, instanceID, tt.expectedInstanceID)
			}
		})
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	cfg := ManagerConfig{
		OutputBufferSize:  1000,
		CaptureIntervalMs: 50,
		TmuxWidth:         100,
		TmuxHeight:        30,
	}

	mgr := NewManagerWithConfig("test-id", "/tmp/test", "test task", cfg)

	if mgr == nil {
		t.Fatal("NewManagerWithConfig returned nil")
	}

	if mgr.id != "test-id" {
		t.Errorf("id = %q, want %q", mgr.id, "test-id")
	}

	if mgr.workdir != "/tmp/test" {
		t.Errorf("workdir = %q, want %q", mgr.workdir, "/tmp/test")
	}

	if mgr.task != "test task" {
		t.Errorf("task = %q, want %q", mgr.task, "test task")
	}

	expectedSession := "claudio-test-id"
	if mgr.sessionName != expectedSession {
		t.Errorf("sessionName = %q, want %q", mgr.sessionName, expectedSession)
	}

	if mgr.config.TmuxWidth != 100 {
		t.Errorf("config.TmuxWidth = %d, want %d", mgr.config.TmuxWidth, 100)
	}

	if mgr.config.TmuxHeight != 30 {
		t.Errorf("config.TmuxHeight = %d, want %d", mgr.config.TmuxHeight, 30)
	}
}

func TestNewManager(t *testing.T) {
	mgr := NewManager("test-id", "/tmp/test", "test task")

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	// Should use default config
	defaultCfg := DefaultManagerConfig()
	if mgr.config.OutputBufferSize != defaultCfg.OutputBufferSize {
		t.Errorf("config.OutputBufferSize = %d, want %d", mgr.config.OutputBufferSize, defaultCfg.OutputBufferSize)
	}
}

func TestNewManagerWithSession(t *testing.T) {
	cfg := testManagerConfig()

	tests := []struct {
		name            string
		sessionID       string
		instanceID      string
		expectedTmuxName string
	}{
		{
			name:            "with session ID",
			sessionID:       "sess1234",
			instanceID:      "inst5678",
			expectedTmuxName: "claudio-sess1234-inst5678",
		},
		{
			name:            "without session ID (legacy)",
			sessionID:       "",
			instanceID:      "abc12345",
			expectedTmuxName: "claudio-abc12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewManagerWithSession(tt.sessionID, tt.instanceID, "/tmp", "task", cfg)

			if mgr.sessionName != tt.expectedTmuxName {
				t.Errorf("sessionName = %q, want %q", mgr.sessionName, tt.expectedTmuxName)
			}

			if mgr.sessionID != tt.sessionID {
				t.Errorf("sessionID = %q, want %q", mgr.sessionID, tt.sessionID)
			}
		})
	}
}

func TestManager_SessionName(t *testing.T) {
	mgr := NewManager("abc123", "/tmp", "task")
	expected := "claudio-abc123"
	if mgr.SessionName() != expected {
		t.Errorf("SessionName() = %q, want %q", mgr.SessionName(), expected)
	}
}

func TestManager_ID(t *testing.T) {
	mgr := NewManager("test-id-123", "/tmp", "task")
	if mgr.ID() != "test-id-123" {
		t.Errorf("ID() = %q, want %q", mgr.ID(), "test-id-123")
	}
}

func TestManager_AttachCommand(t *testing.T) {
	mgr := NewManager("abc123", "/tmp", "task")
	cmd := mgr.AttachCommand()

	if !strings.Contains(cmd, "tmux attach") {
		t.Errorf("AttachCommand() should contain 'tmux attach', got %q", cmd)
	}

	if !strings.Contains(cmd, "claudio-abc123") {
		t.Errorf("AttachCommand() should contain session name, got %q", cmd)
	}
}

func TestDefaultManagerConfig(t *testing.T) {
	cfg := DefaultManagerConfig()

	if cfg.OutputBufferSize <= 0 {
		t.Errorf("OutputBufferSize should be positive, got %d", cfg.OutputBufferSize)
	}

	if cfg.CaptureIntervalMs <= 0 {
		t.Errorf("CaptureIntervalMs should be positive, got %d", cfg.CaptureIntervalMs)
	}

	if cfg.TmuxWidth <= 0 {
		t.Errorf("TmuxWidth should be positive, got %d", cfg.TmuxWidth)
	}

	if cfg.TmuxHeight <= 0 {
		t.Errorf("TmuxHeight should be positive, got %d", cfg.TmuxHeight)
	}

	// Verify default timeout values
	if cfg.ActivityTimeoutMinutes <= 0 {
		t.Errorf("ActivityTimeoutMinutes should be positive by default, got %d", cfg.ActivityTimeoutMinutes)
	}

	if cfg.CompletionTimeoutMinutes <= 0 {
		t.Errorf("CompletionTimeoutMinutes should be positive by default, got %d", cfg.CompletionTimeoutMinutes)
	}

	if !cfg.StaleDetection {
		t.Error("StaleDetection should be enabled by default")
	}
}

// =============================================================================
// State Tests (not running)
// =============================================================================

func TestManager_Running_NotStarted(t *testing.T) {
	mgr := newTestManager(t, "test")

	if mgr.Running() {
		t.Error("Running() should be false before Start()")
	}
}

func TestManager_Paused_NotStarted(t *testing.T) {
	mgr := newTestManager(t, "test")

	if mgr.Paused() {
		t.Error("Paused() should be false before Start()")
	}
}

func TestManager_GetOutput_Empty(t *testing.T) {
	mgr := newTestManager(t, "test")

	output := mgr.GetOutput()
	if len(output) != 0 {
		t.Errorf("GetOutput() should return empty slice before any output, got %d bytes", len(output))
	}
}

func TestManager_CurrentState_Initial(t *testing.T) {
	mgr := newTestManager(t, "test")

	if mgr.CurrentState() != StateWorking {
		t.Errorf("CurrentState() should be StateWorking initially, got %v", mgr.CurrentState())
	}
}

func TestManager_CurrentMetrics_Initial(t *testing.T) {
	mgr := newTestManager(t, "test")

	if mgr.CurrentMetrics() != nil {
		t.Error("CurrentMetrics() should be nil initially")
	}
}

func TestManager_StartTime_Initial(t *testing.T) {
	mgr := newTestManager(t, "test")

	if mgr.StartTime() != nil {
		t.Error("StartTime() should be nil before Start()")
	}
}

func TestManager_TimedOut_Initial(t *testing.T) {
	mgr := newTestManager(t, "test")

	timedOut, _ := mgr.TimedOut()
	if timedOut {
		t.Error("TimedOut() should be false initially")
	}
}

func TestManager_LastActivityTime_Initial(t *testing.T) {
	mgr := newTestManager(t, "test")

	// LastActivityTime should be zero before Start()
	if !mgr.LastActivityTime().IsZero() {
		t.Error("LastActivityTime() should be zero before Start()")
	}
}

func TestManager_PID_NotRunning(t *testing.T) {
	mgr := newTestManager(t, "test")

	if mgr.PID() != 0 {
		t.Errorf("PID() should return 0 when not running, got %d", mgr.PID())
	}
}

// =============================================================================
// Stop/Pause/Resume Tests (not running)
// =============================================================================

func TestManager_Stop_NotRunning(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Stop should not error when not running
	err := mgr.Stop()
	if err != nil {
		t.Errorf("Stop() should not error when not running, got: %v", err)
	}
}

func TestManager_Pause_NotRunning(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Pause should not error when not running
	err := mgr.Pause()
	if err != nil {
		t.Errorf("Pause() should not error when not running, got: %v", err)
	}
}

func TestManager_Resume_NotRunning(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Resume should not error when not running
	err := mgr.Resume()
	if err != nil {
		t.Errorf("Resume() should not error when not running, got: %v", err)
	}
}

// =============================================================================
// Input Handling Tests (not running)
// =============================================================================

func TestManager_SendInput_NotRunning(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Should not panic when not running
	mgr.SendInput([]byte("test input"))
}

func TestManager_SendKey_NotRunning(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Should not panic when not running
	mgr.SendKey("Enter")
}

func TestManager_SendLiteral_NotRunning(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Should not panic when not running
	mgr.SendLiteral("test text")
}

func TestManager_SendPaste_NotRunning(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Should not panic when not running
	mgr.SendPaste("pasted content")
}

// =============================================================================
// Tmux Session Tests (not requiring actual tmux)
// =============================================================================

func TestManager_TmuxSessionExists_NotCreated(t *testing.T) {
	mgr := newTestManager(t, "nonexistent-session-id-12345")

	// This session shouldn't exist since we never created it
	if mgr.TmuxSessionExists() {
		t.Error("TmuxSessionExists() should return false for non-existent session")
	}
}

func TestManager_Reconnect_NoSession(t *testing.T) {
	mgr := newTestManager(t, "nonexistent-reconnect-test")

	err := mgr.Reconnect()
	if err == nil {
		t.Error("Reconnect() should return error when tmux session doesn't exist")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Reconnect() error should mention session doesn't exist, got: %v", err)
	}
}

func TestManager_Reconnect_AlreadyRunning(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Manually set running flag
	mgr.mu.Lock()
	mgr.running = true
	mgr.mu.Unlock()

	err := mgr.Reconnect()
	if err == nil {
		t.Error("Reconnect() should return error when already running")
	}

	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("Reconnect() error should mention already running, got: %v", err)
	}
}

func TestListClaudioTmuxSessions_NoTmuxServer(t *testing.T) {
	// This test may return nil or an empty list depending on whether tmux is running
	// The important thing is it should not error in a way that causes a panic
	sessions, err := ListClaudioTmuxSessions()

	// If tmux isn't running or no sessions exist, we should get nil/empty result, not an error
	if err != nil {
		t.Logf("ListClaudioTmuxSessions returned error (expected if no tmux server): %v", err)
	}

	// Sessions should be nil or contain only claudio-prefixed sessions
	for _, sess := range sessions {
		if !strings.HasPrefix(sess, "claudio-") {
			t.Errorf("ListClaudioTmuxSessions returned non-claudio session: %q", sess)
		}
	}
}

func TestListSessionTmuxSessions_EmptySessionID(t *testing.T) {
	// With empty session ID, should return all claudio sessions
	sessions, err := ListSessionTmuxSessions("")

	if err != nil {
		t.Logf("ListSessionTmuxSessions returned error (expected if no tmux server): %v", err)
	}

	// Verify all returned sessions have claudio prefix
	for _, sess := range sessions {
		if !strings.HasPrefix(sess, "claudio-") {
			t.Errorf("ListSessionTmuxSessions returned non-claudio session: %q", sess)
		}
	}
}

// =============================================================================
// Callback Tests
// =============================================================================

func TestManager_SetStateCallback(t *testing.T) {
	mgr := newTestManager(t, "test")
	recorder := newStateRecorder()

	mgr.SetStateCallback(recorder.callback())

	// Verify callback was set by checking internal state
	mgr.mu.RLock()
	callbackSet := mgr.stateCallback != nil
	mgr.mu.RUnlock()

	if !callbackSet {
		t.Error("SetStateCallback should set the callback")
	}
}

func TestManager_SetMetricsCallback(t *testing.T) {
	mgr := newTestManager(t, "test")
	recorder := newMetricsRecorder()

	mgr.SetMetricsCallback(recorder.callback())

	// Verify callback was set
	mgr.mu.RLock()
	callbackSet := mgr.metricsCallback != nil
	mgr.mu.RUnlock()

	if !callbackSet {
		t.Error("SetMetricsCallback should set the callback")
	}
}

func TestManager_SetTimeoutCallback(t *testing.T) {
	mgr := newTestManager(t, "test")
	recorder := newTimeoutRecorder()

	mgr.SetTimeoutCallback(recorder.callback())

	// Verify callback was set
	mgr.mu.RLock()
	callbackSet := mgr.timeoutCallback != nil
	mgr.mu.RUnlock()

	if !callbackSet {
		t.Error("SetTimeoutCallback should set the callback")
	}
}

func TestManager_SetBellCallback(t *testing.T) {
	mgr := newTestManager(t, "test")
	recorder := newBellRecorder()

	mgr.SetBellCallback(recorder.callback())

	// Verify callback was set
	mgr.mu.RLock()
	callbackSet := mgr.bellCallback != nil
	mgr.mu.RUnlock()

	if !callbackSet {
		t.Error("SetBellCallback should set the callback")
	}
}

// =============================================================================
// State Detection and Notification Tests
// =============================================================================

func TestManager_DetectAndNotifyState(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		expectedState WaitingState
	}{
		{
			name:          "working spinner",
			output:        "Processing... ⠋",
			expectedState: StateWorking,
		},
		{
			name:          "question mark",
			output:        "What would you like me to do?",
			expectedState: StateWaitingQuestion,
		},
		{
			name:          "permission prompt",
			output:        "Do you want me to proceed? [Y/N]",
			expectedState: StateWaitingPermission,
		},
		{
			name:          "empty output",
			output:        "",
			expectedState: StateWorking,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestManager(t, "test")
			recorder := newStateRecorder()
			mgr.SetStateCallback(recorder.callback())

			// Call detectAndNotifyState directly
			mgr.detectAndNotifyState([]byte(tt.output))

			// Check current state
			if mgr.CurrentState() != tt.expectedState {
				t.Errorf("CurrentState() = %v, want %v", mgr.CurrentState(), tt.expectedState)
			}
		})
	}
}

func TestManager_DetectAndNotifyState_CallbackFired(t *testing.T) {
	mgr := newTestManager(t, "test-callback")
	recorder := newStateRecorder()
	mgr.SetStateCallback(recorder.callback())

	// Initial state is Working, so transition to Question should fire callback
	mgr.detectAndNotifyState([]byte("What should I do?"))

	changes := recorder.getChanges()
	if len(changes) != 1 {
		t.Fatalf("Expected 1 state change, got %d", len(changes))
	}

	if changes[0].state != StateWaitingQuestion {
		t.Errorf("Expected state change to StateWaitingQuestion, got %v", changes[0].state)
	}

	if changes[0].instanceID != "test-callback" {
		t.Errorf("Expected instanceID 'test-callback', got %q", changes[0].instanceID)
	}
}

func TestManager_DetectAndNotifyState_NoCallbackOnSameState(t *testing.T) {
	mgr := newTestManager(t, "test")
	recorder := newStateRecorder()
	mgr.SetStateCallback(recorder.callback())

	// Send output that results in Working state (initial state)
	mgr.detectAndNotifyState([]byte("Processing... ⠋"))

	changes := recorder.getChanges()
	if len(changes) != 0 {
		t.Errorf("Should not fire callback when state doesn't change, got %d changes", len(changes))
	}
}

// =============================================================================
// Metrics Parsing and Notification Tests
// =============================================================================

func TestManager_ParseAndNotifyMetrics(t *testing.T) {
	mgr := newTestManager(t, "test")
	recorder := newMetricsRecorder()
	mgr.SetMetricsCallback(recorder.callback())

	// Simulate output with metrics
	output := []byte("Total: 45.2K input, 12.8K output | Cost: $0.42")
	mgr.parseAndNotifyMetrics(output)

	changes := recorder.getChanges()
	if len(changes) != 1 {
		t.Fatalf("Expected 1 metrics change, got %d", len(changes))
	}

	metrics := changes[0].metrics
	if metrics.InputTokens != 45200 {
		t.Errorf("InputTokens = %d, want 45200", metrics.InputTokens)
	}

	if metrics.OutputTokens != 12800 {
		t.Errorf("OutputTokens = %d, want 12800", metrics.OutputTokens)
	}

	if metrics.Cost != 0.42 {
		t.Errorf("Cost = %f, want 0.42", metrics.Cost)
	}
}

func TestManager_ParseAndNotifyMetrics_NoMetrics(t *testing.T) {
	mgr := newTestManager(t, "test")
	recorder := newMetricsRecorder()
	mgr.SetMetricsCallback(recorder.callback())

	// Output without metrics
	output := []byte("Just some regular output without any token counts")
	mgr.parseAndNotifyMetrics(output)

	changes := recorder.getChanges()
	if len(changes) != 0 {
		t.Errorf("Should not fire callback when no metrics found, got %d changes", len(changes))
	}
}

func TestManager_ParseAndNotifyMetrics_SameMetrics(t *testing.T) {
	mgr := newTestManager(t, "test")
	recorder := newMetricsRecorder()
	mgr.SetMetricsCallback(recorder.callback())

	output := []byte("Total: 100 input, 50 output")

	// First call should fire callback
	mgr.parseAndNotifyMetrics(output)

	// Second call with same metrics should also fire (current implementation doesn't deduplicate)
	mgr.parseAndNotifyMetrics(output)

	// The current implementation fires callback on every change detection
	// This tests the actual behavior
	changes := recorder.getChanges()
	if len(changes) < 1 {
		t.Error("Expected at least one metrics callback")
	}
}

// =============================================================================
// Timeout Tests
// =============================================================================

func TestManager_CheckTimeouts_NotRunning(t *testing.T) {
	mgr := newTestManager(t, "test")
	recorder := newTimeoutRecorder()
	mgr.SetTimeoutCallback(recorder.callback())

	// Should not trigger timeout when not running
	mgr.checkTimeouts()

	timeouts := recorder.getTimeouts()
	if len(timeouts) != 0 {
		t.Errorf("Should not trigger timeout when not running, got %d timeouts", len(timeouts))
	}
}

func TestManager_CheckTimeouts_Paused(t *testing.T) {
	mgr := newTestManager(t, "test")
	recorder := newTimeoutRecorder()
	mgr.SetTimeoutCallback(recorder.callback())

	// Set as running and paused
	mgr.mu.Lock()
	mgr.running = true
	mgr.paused = true
	now := time.Now()
	mgr.startTime = &now
	mgr.lastActivityTime = now
	mgr.mu.Unlock()

	// Should not trigger timeout when paused
	mgr.checkTimeouts()

	timeouts := recorder.getTimeouts()
	if len(timeouts) != 0 {
		t.Errorf("Should not trigger timeout when paused, got %d timeouts", len(timeouts))
	}
}

func TestManager_CheckTimeouts_AlreadyTimedOut(t *testing.T) {
	mgr := newTestManager(t, "test")
	recorder := newTimeoutRecorder()
	mgr.SetTimeoutCallback(recorder.callback())

	// Set as running and already timed out
	mgr.mu.Lock()
	mgr.running = true
	mgr.timedOut = true
	now := time.Now()
	mgr.startTime = &now
	mgr.mu.Unlock()

	// Should not trigger another timeout
	mgr.checkTimeouts()

	timeouts := recorder.getTimeouts()
	if len(timeouts) != 0 {
		t.Errorf("Should not trigger timeout when already timed out, got %d timeouts", len(timeouts))
	}
}

func TestManager_CheckTimeouts_CompletionTimeout(t *testing.T) {
	cfg := testManagerConfig()
	cfg.CompletionTimeoutMinutes = 1 // 1 minute timeout
	mgr := NewManagerWithConfig("test", t.TempDir(), "task", cfg)

	recorder := newTimeoutRecorder()
	mgr.SetTimeoutCallback(recorder.callback())

	// Set as running with start time in the past
	mgr.mu.Lock()
	mgr.running = true
	pastTime := time.Now().Add(-2 * time.Minute) // 2 minutes ago
	mgr.startTime = &pastTime
	mgr.lastActivityTime = time.Now()
	mgr.mu.Unlock()

	mgr.checkTimeouts()

	timeouts := recorder.getTimeouts()
	if len(timeouts) != 1 {
		t.Fatalf("Expected 1 timeout, got %d", len(timeouts))
	}

	if timeouts[0].timeoutType != TimeoutCompletion {
		t.Errorf("Expected TimeoutCompletion, got %v", timeouts[0].timeoutType)
	}

	timedOut, timeoutType := mgr.TimedOut()
	if !timedOut {
		t.Error("TimedOut() should return true after completion timeout")
	}
	if timeoutType != TimeoutCompletion {
		t.Errorf("TimeoutType should be TimeoutCompletion, got %v", timeoutType)
	}
}

func TestManager_CheckTimeouts_ActivityTimeout(t *testing.T) {
	cfg := testManagerConfig()
	cfg.ActivityTimeoutMinutes = 1 // 1 minute timeout
	mgr := NewManagerWithConfig("test", t.TempDir(), "task", cfg)

	recorder := newTimeoutRecorder()
	mgr.SetTimeoutCallback(recorder.callback())

	// Set as running with last activity in the past
	mgr.mu.Lock()
	mgr.running = true
	now := time.Now()
	mgr.startTime = &now
	mgr.lastActivityTime = time.Now().Add(-2 * time.Minute) // 2 minutes ago
	mgr.mu.Unlock()

	mgr.checkTimeouts()

	timeouts := recorder.getTimeouts()
	if len(timeouts) != 1 {
		t.Fatalf("Expected 1 timeout, got %d", len(timeouts))
	}

	if timeouts[0].timeoutType != TimeoutActivity {
		t.Errorf("Expected TimeoutActivity, got %v", timeouts[0].timeoutType)
	}
}

func TestManager_CheckTimeouts_StaleTimeout(t *testing.T) {
	cfg := testManagerConfig()
	cfg.StaleDetection = true
	mgr := NewManagerWithConfig("test", t.TempDir(), "task", cfg)

	recorder := newTimeoutRecorder()
	mgr.SetTimeoutCallback(recorder.callback())

	// Set as running with high repeated output count
	mgr.mu.Lock()
	mgr.running = true
	now := time.Now()
	mgr.startTime = &now
	mgr.lastActivityTime = now
	mgr.repeatedOutputCount = 3001 // Over threshold of 3000
	mgr.mu.Unlock()

	mgr.checkTimeouts()

	timeouts := recorder.getTimeouts()
	if len(timeouts) != 1 {
		t.Fatalf("Expected 1 timeout, got %d", len(timeouts))
	}

	if timeouts[0].timeoutType != TimeoutStale {
		t.Errorf("Expected TimeoutStale, got %v", timeouts[0].timeoutType)
	}
}

func TestManager_CheckTimeouts_CompletionTakesPrecedence(t *testing.T) {
	cfg := testManagerConfig()
	cfg.CompletionTimeoutMinutes = 1
	cfg.ActivityTimeoutMinutes = 1
	cfg.StaleDetection = true
	mgr := NewManagerWithConfig("test", t.TempDir(), "task", cfg)

	recorder := newTimeoutRecorder()
	mgr.SetTimeoutCallback(recorder.callback())

	// Set all timeout conditions
	mgr.mu.Lock()
	mgr.running = true
	pastTime := time.Now().Add(-2 * time.Minute)
	mgr.startTime = &pastTime
	mgr.lastActivityTime = pastTime
	mgr.repeatedOutputCount = 3001
	mgr.mu.Unlock()

	mgr.checkTimeouts()

	timeouts := recorder.getTimeouts()
	if len(timeouts) != 1 {
		t.Fatalf("Expected 1 timeout, got %d", len(timeouts))
	}

	// Completion timeout should take precedence
	if timeouts[0].timeoutType != TimeoutCompletion {
		t.Errorf("Expected TimeoutCompletion (takes precedence), got %v", timeouts[0].timeoutType)
	}
}

func TestManager_ClearTimeout(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Set timeout state
	mgr.mu.Lock()
	mgr.timedOut = true
	mgr.timeoutType = TimeoutActivity
	mgr.repeatedOutputCount = 100
	oldActivityTime := time.Now().Add(-time.Hour)
	mgr.lastActivityTime = oldActivityTime
	mgr.mu.Unlock()

	// Clear timeout
	mgr.ClearTimeout()

	timedOut, _ := mgr.TimedOut()
	if timedOut {
		t.Error("TimedOut() should be false after ClearTimeout()")
	}

	mgr.mu.RLock()
	if mgr.repeatedOutputCount != 0 {
		t.Errorf("repeatedOutputCount should be 0 after ClearTimeout(), got %d", mgr.repeatedOutputCount)
	}
	if !mgr.lastActivityTime.After(oldActivityTime) {
		t.Error("lastActivityTime should be updated after ClearTimeout()")
	}
	mgr.mu.RUnlock()
}

// =============================================================================
// Timeout Type Tests
// =============================================================================

func TestTimeoutType_Values(t *testing.T) {
	// Verify timeout type constants
	if TimeoutActivity != 0 {
		t.Errorf("TimeoutActivity should be 0, got %d", TimeoutActivity)
	}
	if TimeoutCompletion != 1 {
		t.Errorf("TimeoutCompletion should be 1, got %d", TimeoutCompletion)
	}
	if TimeoutStale != 2 {
		t.Errorf("TimeoutStale should be 2, got %d", TimeoutStale)
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestManager_ConcurrentStateAccess(t *testing.T) {
	mgr := newTestManager(t, "test")

	var wg sync.WaitGroup
	const numGoroutines = 10
	const numIterations = 100

	// Multiple goroutines reading state
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				_ = mgr.CurrentState()
				_ = mgr.Running()
				_ = mgr.Paused()
				_, _ = mgr.TimedOut()
				_ = mgr.CurrentMetrics()
				_ = mgr.StartTime()
				_ = mgr.LastActivityTime()
			}
		}()
	}

	wg.Wait()
}

func TestManager_ConcurrentCallbackSetting(t *testing.T) {
	mgr := newTestManager(t, "test")

	var wg sync.WaitGroup
	const numGoroutines = 10

	// Multiple goroutines setting callbacks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(4)
		go func() {
			defer wg.Done()
			mgr.SetStateCallback(func(string, WaitingState) {})
		}()
		go func() {
			defer wg.Done()
			mgr.SetMetricsCallback(func(string, *ParsedMetrics) {})
		}()
		go func() {
			defer wg.Done()
			mgr.SetTimeoutCallback(func(string, TimeoutType) {})
		}()
		go func() {
			defer wg.Done()
			mgr.SetBellCallback(func(string) {})
		}()
	}

	wg.Wait()
}

func TestManager_ConcurrentDetectAndNotify(t *testing.T) {
	mgr := newTestManager(t, "test")

	var callCount atomic.Int32
	mgr.SetStateCallback(func(string, WaitingState) {
		callCount.Add(1)
	})

	var wg sync.WaitGroup
	const numGoroutines = 10

	outputs := []string{
		"Processing... ⠋",
		"What should I do?",
		"Do you want me to proceed? [Y/N]",
		"Regular output",
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			output := outputs[idx%len(outputs)]
			mgr.detectAndNotifyState([]byte(output))
		}(i)
	}

	wg.Wait()

	// Should complete without race conditions
	t.Logf("Callback invoked %d times", callCount.Load())
}

func TestManager_ConcurrentClearTimeout(t *testing.T) {
	mgr := newTestManager(t, "test")

	var wg sync.WaitGroup
	const numGoroutines = 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			mgr.ClearTimeout()
		}()
		go func() {
			defer wg.Done()
			_, _ = mgr.TimedOut()
		}()
	}

	wg.Wait()
}

// =============================================================================
// Multiple Instance Management Tests
// =============================================================================

func TestMultipleManagers_IndependentState(t *testing.T) {
	mgr1 := newTestManager(t, "instance-1")
	mgr2 := newTestManager(t, "instance-2")
	mgr3 := newTestManager(t, "instance-3")

	// Each should have independent state
	if mgr1.SessionName() == mgr2.SessionName() {
		t.Error("Different managers should have different session names")
	}
	if mgr2.SessionName() == mgr3.SessionName() {
		t.Error("Different managers should have different session names")
	}

	// Set different states on each
	mgr1.detectAndNotifyState([]byte("What should I do?"))
	mgr2.detectAndNotifyState([]byte("Do you want to proceed? [Y/N]"))
	mgr3.detectAndNotifyState([]byte("Processing... ⠋"))

	if mgr1.CurrentState() != StateWaitingQuestion {
		t.Errorf("mgr1 state = %v, want StateWaitingQuestion", mgr1.CurrentState())
	}
	if mgr2.CurrentState() != StateWaitingPermission {
		t.Errorf("mgr2 state = %v, want StateWaitingPermission", mgr2.CurrentState())
	}
	if mgr3.CurrentState() != StateWorking {
		t.Errorf("mgr3 state = %v, want StateWorking", mgr3.CurrentState())
	}
}

func TestMultipleManagers_IndependentCallbacks(t *testing.T) {
	mgr1 := newTestManager(t, "instance-1")
	mgr2 := newTestManager(t, "instance-2")

	recorder1 := newStateRecorder()
	recorder2 := newStateRecorder()

	mgr1.SetStateCallback(recorder1.callback())
	mgr2.SetStateCallback(recorder2.callback())

	// Trigger state change on mgr1 only
	mgr1.detectAndNotifyState([]byte("What should I do?"))

	changes1 := recorder1.getChanges()
	changes2 := recorder2.getChanges()

	if len(changes1) != 1 {
		t.Errorf("recorder1 should have 1 change, got %d", len(changes1))
	}
	if len(changes2) != 0 {
		t.Errorf("recorder2 should have 0 changes, got %d", len(changes2))
	}

	if len(changes1) > 0 && changes1[0].instanceID != "instance-1" {
		t.Errorf("callback should receive correct instance ID, got %q", changes1[0].instanceID)
	}
}

func TestMultipleManagers_ConcurrentOperations(t *testing.T) {
	const numManagers = 5
	managers := make([]*Manager, numManagers)

	for i := 0; i < numManagers; i++ {
		managers[i] = newTestManager(t, "instance-"+string(rune('a'+i)))
		managers[i].SetStateCallback(func(string, WaitingState) {})
	}

	var wg sync.WaitGroup

	// Concurrent operations on multiple managers
	for _, mgr := range managers {
		wg.Add(1)
		go func(m *Manager) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.detectAndNotifyState([]byte("What?"))
				_ = m.CurrentState()
				_ = m.Running()
				m.detectAndNotifyState([]byte("Processing... ⠋"))
			}
		}(mgr)
	}

	wg.Wait()
}

// =============================================================================
// Resource Cleanup Tests
// =============================================================================

func TestManager_StopCleansUpDoneChan(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Manually set up as if running
	mgr.mu.Lock()
	mgr.running = true
	mgr.doneChan = make(chan struct{})
	mgr.mu.Unlock()

	// Stop should close doneChan
	_ = mgr.Stop()

	// Verify doneChan is closed
	select {
	case <-mgr.doneChan:
		// Expected - channel is closed
	default:
		t.Error("doneChan should be closed after Stop()")
	}
}

func TestManager_StopIdempotent(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Multiple stops should not panic
	for i := 0; i < 3; i++ {
		err := mgr.Stop()
		if err != nil {
			t.Errorf("Stop() iteration %d returned error: %v", i, err)
		}
	}
}

func TestManager_PauseResumeIdempotent(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Multiple pause/resume when not running should not error
	for i := 0; i < 3; i++ {
		if err := mgr.Pause(); err != nil {
			t.Errorf("Pause() iteration %d returned error: %v", i, err)
		}
		if err := mgr.Resume(); err != nil {
			t.Errorf("Resume() iteration %d returned error: %v", i, err)
		}
	}
}

// =============================================================================
// Resize Tests
// =============================================================================

func TestManager_Resize_NotRunning(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Should not error when not running
	err := mgr.Resize(100, 50)
	if err != nil {
		t.Errorf("Resize() should not error when not running, got: %v", err)
	}
}

func TestManager_Resize_UpdatesConfig(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Simulate running state
	mgr.mu.Lock()
	mgr.running = true
	mgr.mu.Unlock()

	// Resize will fail without real tmux session, but config should still update
	_ = mgr.Resize(150, 40)

	mgr.mu.RLock()
	width := mgr.config.TmuxWidth
	height := mgr.config.TmuxHeight
	mgr.mu.RUnlock()

	if width != 150 {
		t.Errorf("TmuxWidth = %d, want 150", width)
	}
	if height != 40 {
		t.Errorf("TmuxHeight = %d, want 40", height)
	}
}

// =============================================================================
// Output Buffer Tests
// =============================================================================

func TestManager_OutputBuffer_DirectWrite(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Write directly to buffer (simulating capture loop behavior)
	testData := []byte("test output data")
	mgr.outputBuf.Reset()
	_, _ = mgr.outputBuf.Write(testData)

	output := mgr.GetOutput()
	if string(output) != string(testData) {
		t.Errorf("GetOutput() = %q, want %q", output, testData)
	}
}

func TestManager_OutputBuffer_Overflow(t *testing.T) {
	cfg := testManagerConfig()
	cfg.OutputBufferSize = 100 // Small buffer
	mgr := NewManagerWithConfig("test", t.TempDir(), "task", cfg)

	// Write more than buffer size
	largeData := make([]byte, 200)
	for i := range largeData {
		largeData[i] = byte('a' + (i % 26))
	}

	mgr.outputBuf.Reset()
	_, _ = mgr.outputBuf.Write(largeData)

	output := mgr.GetOutput()
	// Ring buffer should only keep the last OutputBufferSize bytes
	if len(output) > cfg.OutputBufferSize {
		t.Errorf("Output buffer should respect size limit, got %d bytes", len(output))
	}
}

// =============================================================================
// Edge Cases and Error Handling
// =============================================================================

func TestManager_EmptyInstanceID(t *testing.T) {
	mgr := NewManager("", "/tmp", "task")

	// Should handle empty ID gracefully
	if mgr.ID() != "" {
		t.Errorf("ID() should return empty string for empty ID, got %q", mgr.ID())
	}

	expectedSession := "claudio-"
	if mgr.SessionName() != expectedSession {
		t.Errorf("SessionName() = %q, want %q", mgr.SessionName(), expectedSession)
	}
}

func TestManager_EmptyWorkdir(t *testing.T) {
	// Using "" as workdir
	mgr := NewManager("test", "", "task")

	if mgr == nil {
		t.Fatal("NewManager should not return nil for empty workdir")
	}
}

func TestManager_EmptyTask(t *testing.T) {
	mgr := NewManager("test", "/tmp", "")

	if mgr == nil {
		t.Fatal("NewManager should not return nil for empty task")
	}
}

func TestManager_SpecialCharactersInID(t *testing.T) {
	// Test with various special characters in ID
	specialIDs := []string{
		"test-with-dashes",
		"test_with_underscores",
		"test.with.dots",
		"test123numeric",
	}

	for _, id := range specialIDs {
		t.Run(id, func(t *testing.T) {
			mgr := NewManager(id, "/tmp", "task")

			if mgr.ID() != id {
				t.Errorf("ID() = %q, want %q", mgr.ID(), id)
			}

			expectedSession := "claudio-" + id
			if mgr.SessionName() != expectedSession {
				t.Errorf("SessionName() = %q, want %q", mgr.SessionName(), expectedSession)
			}
		})
	}
}

func TestManager_NilCallbackHandling(t *testing.T) {
	mgr := newTestManager(t, "test")

	// With nil callbacks, state changes should not panic
	mgr.detectAndNotifyState([]byte("What should I do?"))
	mgr.parseAndNotifyMetrics([]byte("Total: 100 input, 50 output"))
	mgr.checkTimeouts()
}

// =============================================================================
// Bell Detection Tests (unit test - no tmux)
// =============================================================================

func TestManager_BellStateTracking(t *testing.T) {
	mgr := newTestManager(t, "test")

	// Initial bell state should be false
	mgr.mu.RLock()
	initialState := mgr.lastBellState
	mgr.mu.RUnlock()

	if initialState {
		t.Error("Initial lastBellState should be false")
	}

	// Setting bell callback should not change state
	recorder := newBellRecorder()
	mgr.SetBellCallback(recorder.callback())

	mgr.mu.RLock()
	afterCallback := mgr.lastBellState
	mgr.mu.RUnlock()

	if afterCallback {
		t.Error("lastBellState should remain false after setting callback")
	}
}

// =============================================================================
// Test Helper Function Tests
// =============================================================================

func TestStateRecorder(t *testing.T) {
	recorder := newStateRecorder()

	// Initially empty
	if len(recorder.getChanges()) != 0 {
		t.Error("New recorder should have no changes")
	}

	// Record some changes
	cb := recorder.callback()
	cb("inst1", StateWaitingQuestion)
	cb("inst2", StateWaitingPermission)
	cb("inst1", StateCompleted)

	changes := recorder.getChanges()
	if len(changes) != 3 {
		t.Fatalf("Expected 3 changes, got %d", len(changes))
	}

	// Verify last state
	lastState, ok := recorder.lastState()
	if !ok {
		t.Error("lastState should return ok=true when changes exist")
	}
	if lastState != StateCompleted {
		t.Errorf("lastState = %v, want StateCompleted", lastState)
	}
}

func TestMetricsRecorder(t *testing.T) {
	recorder := newMetricsRecorder()

	if len(recorder.getChanges()) != 0 {
		t.Error("New recorder should have no changes")
	}

	cb := recorder.callback()
	cb("inst1", &ParsedMetrics{InputTokens: 100})
	cb("inst2", &ParsedMetrics{OutputTokens: 50})

	changes := recorder.getChanges()
	if len(changes) != 2 {
		t.Fatalf("Expected 2 changes, got %d", len(changes))
	}
}

func TestTimeoutRecorder(t *testing.T) {
	recorder := newTimeoutRecorder()

	if len(recorder.getTimeouts()) != 0 {
		t.Error("New recorder should have no timeouts")
	}

	cb := recorder.callback()
	cb("inst1", TimeoutActivity)
	cb("inst2", TimeoutCompletion)

	timeouts := recorder.getTimeouts()
	if len(timeouts) != 2 {
		t.Fatalf("Expected 2 timeouts, got %d", len(timeouts))
	}
}

func TestBellRecorder(t *testing.T) {
	recorder := newBellRecorder()

	if len(recorder.getBells()) != 0 {
		t.Error("New recorder should have no bells")
	}

	cb := recorder.callback()
	cb("inst1")
	cb("inst2")
	cb("inst1")

	bells := recorder.getBells()
	if len(bells) != 3 {
		t.Fatalf("Expected 3 bells, got %d", len(bells))
	}
}
