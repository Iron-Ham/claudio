package instance

import (
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance/detect"
	"github.com/Iron-Ham/claudio/internal/instance/lifecycle"
	"github.com/Iron-Ham/claudio/internal/instance/state"
)

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

func TestNewManagerWithDeps_ExplicitMonitor(t *testing.T) {
	// Test with explicit StateMonitor
	monitor := state.NewMonitor(state.DefaultMonitorConfig())
	opts := ManagerOptions{
		ID:           "test-id",
		SessionID:    "session-123",
		WorkDir:      "/tmp/test",
		Task:         "test task",
		StateMonitor: monitor,
	}
	mgr := NewManagerWithDeps(opts)

	if mgr == nil {
		t.Fatal("NewManagerWithDeps returned nil")
	}
	if mgr.stateMonitor != monitor {
		t.Error("Should use provided StateMonitor")
	}
	if mgr.sessionName != "claudio-session-123-test-id" {
		t.Errorf("sessionName = %q, want claudio-session-123-test-id", mgr.sessionName)
	}
}

func TestNewManagerWithDeps_NilMonitor(t *testing.T) {
	// Test StateMonitor creation when nil
	opts := ManagerOptions{
		ID:      "test-id",
		WorkDir: "/tmp",
		Task:    "task",
		// StateMonitor: nil - should create one internally
	}
	mgr := NewManagerWithDeps(opts)

	if mgr == nil {
		t.Fatal("NewManagerWithDeps returned nil")
	}
	if mgr.stateMonitor == nil {
		t.Error("Should create StateMonitor when not provided")
	}
}

func TestNewManagerWithDeps_ConfigDefaults(t *testing.T) {
	// Test that zero config values get defaults
	opts := ManagerOptions{
		ID:      "test-id",
		WorkDir: "/tmp",
		Task:    "task",
		Config:  ManagerConfig{}, // All zeros
	}
	mgr := NewManagerWithDeps(opts)

	defaults := DefaultManagerConfig()
	if mgr.config.OutputBufferSize != defaults.OutputBufferSize {
		t.Errorf("Should apply default OutputBufferSize, got %d want %d",
			mgr.config.OutputBufferSize, defaults.OutputBufferSize)
	}
	if mgr.config.CaptureIntervalMs != defaults.CaptureIntervalMs {
		t.Errorf("Should apply default CaptureIntervalMs, got %d want %d",
			mgr.config.CaptureIntervalMs, defaults.CaptureIntervalMs)
	}
	if mgr.config.TmuxWidth != defaults.TmuxWidth {
		t.Errorf("Should apply default TmuxWidth, got %d want %d",
			mgr.config.TmuxWidth, defaults.TmuxWidth)
	}
}

func TestNewManagerWithDeps_SessionNameWithoutSessionID(t *testing.T) {
	// Test session name generation without SessionID
	opts := ManagerOptions{
		ID:      "test-id",
		WorkDir: "/tmp",
		Task:    "task",
	}
	mgr := NewManagerWithDeps(opts)

	if mgr.sessionName != "claudio-test-id" {
		t.Errorf("sessionName = %q, want claudio-test-id", mgr.sessionName)
	}
}

func TestNewManagerWithDeps_PreservesCustomConfig(t *testing.T) {
	// Test that custom config values are preserved (not overwritten by defaults)
	customCfg := ManagerConfig{
		OutputBufferSize:  5000,
		CaptureIntervalMs: 200,
		TmuxWidth:         150,
		TmuxHeight:        40,
		TmuxHistoryLimit:  50000,
	}
	opts := ManagerOptions{
		ID:      "test-id",
		WorkDir: "/tmp",
		Task:    "task",
		Config:  customCfg,
	}
	mgr := NewManagerWithDeps(opts)

	if mgr.config.OutputBufferSize != 5000 {
		t.Errorf("config.OutputBufferSize = %d, want 5000", mgr.config.OutputBufferSize)
	}
	if mgr.config.CaptureIntervalMs != 200 {
		t.Errorf("config.CaptureIntervalMs = %d, want 200", mgr.config.CaptureIntervalMs)
	}
	if mgr.config.TmuxWidth != 150 {
		t.Errorf("config.TmuxWidth = %d, want 150", mgr.config.TmuxWidth)
	}
	if mgr.config.TmuxHeight != 40 {
		t.Errorf("config.TmuxHeight = %d, want 40", mgr.config.TmuxHeight)
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

	if !strings.Contains(cmd, "tmux") || !strings.Contains(cmd, "attach") {
		t.Errorf("AttachCommand() should contain 'tmux' and 'attach', got %q", cmd)
	}

	if !strings.Contains(cmd, "-L claudio") {
		t.Errorf("AttachCommand() should specify claudio socket (-L claudio), got %q", cmd)
	}

	if !strings.Contains(cmd, "claudio-abc123") {
		t.Errorf("AttachCommand() should contain session name, got %q", cmd)
	}
}

func TestManager_Running_NotStarted(t *testing.T) {
	mgr := NewManager("test", "/tmp", "task")

	if mgr.Running() {
		t.Error("Running() should be false before Start()")
	}
}

func TestManager_Paused_NotStarted(t *testing.T) {
	mgr := NewManager("test", "/tmp", "task")

	if mgr.Paused() {
		t.Error("Paused() should be false before Start()")
	}
}

func TestManager_GetOutput_Empty(t *testing.T) {
	mgr := NewManager("test", "/tmp", "task")

	output := mgr.GetOutput()
	if len(output) != 0 {
		t.Errorf("GetOutput() should return empty slice before any output, got %d bytes", len(output))
	}
}

func TestManager_CurrentState_Initial(t *testing.T) {
	mgr := NewManager("test", "/tmp", "task")

	if mgr.CurrentState() != detect.StateWorking {
		t.Errorf("CurrentState() should be detect.StateWorking initially, got %v", mgr.CurrentState())
	}
}

func TestManager_TmuxSessionExists_NotCreated(t *testing.T) {
	mgr := NewManager("nonexistent-session-id-12345", "/tmp", "task")

	// This session shouldn't exist since we never created it
	if mgr.TmuxSessionExists() {
		t.Error("TmuxSessionExists() should return false for non-existent session")
	}
}

func TestManager_Reconnect_NoSession(t *testing.T) {
	mgr := NewManager("nonexistent-reconnect-test", "/tmp", "task")

	err := mgr.Reconnect()
	if err == nil {
		t.Error("Reconnect() should return error when tmux session doesn't exist")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Reconnect() error should mention session doesn't exist, got: %v", err)
	}
}

func TestManager_Stop_NotRunning(t *testing.T) {
	mgr := NewManager("test", "/tmp", "task")

	// Stop should not error when not running
	err := mgr.Stop()
	if err != nil {
		t.Errorf("Stop() should not error when not running, got: %v", err)
	}
}

func TestManager_Pause_NotRunning(t *testing.T) {
	mgr := NewManager("test", "/tmp", "task")

	// Pause should not error when not running
	err := mgr.Pause()
	if err != nil {
		t.Errorf("Pause() should not error when not running, got: %v", err)
	}
}

func TestManager_Resume_NotRunning(t *testing.T) {
	mgr := NewManager("test", "/tmp", "task")

	// Resume should not error when not running
	err := mgr.Resume()
	if err != nil {
		t.Errorf("Resume() should not error when not running, got: %v", err)
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

	if cfg.TmuxHistoryLimit <= 0 {
		t.Errorf("TmuxHistoryLimit should be positive, got %d", cfg.TmuxHistoryLimit)
	}
}

func TestManager_DifferentialCaptureFieldsInitialized(t *testing.T) {
	mgr := NewManager("test-diff-capture", "/tmp", "task")

	// Verify differential capture fields are initialized to zero values
	if mgr.lastHistorySize != 0 {
		t.Errorf("lastHistorySize should be 0 initially, got %d", mgr.lastHistorySize)
	}

	if mgr.fullRefreshCounter != 0 {
		t.Errorf("fullRefreshCounter should be 0 initially, got %d", mgr.fullRefreshCounter)
	}
}

func TestManager_GetHistorySize_NoSession(t *testing.T) {
	mgr := NewManager("nonexistent-hist-test", "/tmp", "task")

	// getHistorySize should return -1 for a non-existent session
	size := mgr.getHistorySize("nonexistent-session-xyz")
	if size != -1 {
		t.Errorf("getHistorySize for non-existent session should return -1, got %d", size)
	}
}

func TestManager_GetSessionStatus_NoSession(t *testing.T) {
	mgr := NewManager("nonexistent-status-test", "/tmp", "task")

	// getSessionStatus should indicate session doesn't exist for a non-existent session
	status := mgr.getSessionStatus("nonexistent-session-xyz")

	if status.sessionExists {
		t.Error("getSessionStatus for non-existent session should return sessionExists=false")
	}
	if status.historySize != -1 {
		t.Errorf("getSessionStatus for non-existent session should return historySize=-1, got %d", status.historySize)
	}
	if status.bellActive {
		t.Error("getSessionStatus for non-existent session should return bellActive=false")
	}
}

func TestParseSessionStatusOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantHistory int
		wantBell    bool
		wantOK      bool
	}{
		{
			name:        "valid with bell active",
			output:      "123|1",
			wantHistory: 123,
			wantBell:    true,
			wantOK:      true,
		},
		{
			name:        "valid with bell inactive",
			output:      "456|0",
			wantHistory: 456,
			wantBell:    false,
			wantOK:      true,
		},
		{
			name:        "zero history",
			output:      "0|1",
			wantHistory: 0,
			wantBell:    true,
			wantOK:      true,
		},
		{
			name:        "large history",
			output:      "50000|0",
			wantHistory: 50000,
			wantBell:    false,
			wantOK:      true,
		},
		{
			name:        "with leading/trailing whitespace",
			output:      "  123|0  ",
			wantHistory: 123,
			wantBell:    false,
			wantOK:      true,
		},
		{
			name:        "with newline",
			output:      "789|1\n",
			wantHistory: 789,
			wantBell:    true,
			wantOK:      true,
		},
		{
			name:        "missing delimiter",
			output:      "123",
			wantHistory: -1,
			wantBell:    false,
			wantOK:      false,
		},
		{
			name:        "empty string",
			output:      "",
			wantHistory: -1,
			wantBell:    false,
			wantOK:      false,
		},
		{
			name:        "only whitespace",
			output:      "   ",
			wantHistory: -1,
			wantBell:    false,
			wantOK:      false,
		},
		{
			name:        "non-numeric history",
			output:      "abc|1",
			wantHistory: -1,
			wantBell:    true,
			wantOK:      true,
		},
		{
			name:        "extra parts ignored",
			output:      "1|2|3",
			wantHistory: -1,
			wantBell:    false,
			wantOK:      false,
		},
		{
			name:        "bell value not 0 or 1 treated as false",
			output:      "100|2",
			wantHistory: 100,
			wantBell:    false,
			wantOK:      true,
		},
		{
			name:        "empty bell value treated as false",
			output:      "100|",
			wantHistory: 100,
			wantBell:    false,
			wantOK:      true,
		},
		{
			name:        "negative history parsed correctly",
			output:      "-1|0",
			wantHistory: -1,
			wantBell:    false,
			wantOK:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHistory, gotBell, gotOK := parseSessionStatusOutput(tt.output)
			if gotHistory != tt.wantHistory {
				t.Errorf("parseSessionStatusOutput(%q) historySize = %d, want %d", tt.output, gotHistory, tt.wantHistory)
			}
			if gotBell != tt.wantBell {
				t.Errorf("parseSessionStatusOutput(%q) bellActive = %v, want %v", tt.output, gotBell, tt.wantBell)
			}
			if gotOK != tt.wantOK {
				t.Errorf("parseSessionStatusOutput(%q) ok = %v, want %v", tt.output, gotOK, tt.wantOK)
			}
		})
	}
}

func TestManager_CheckSessionExists_NoSession(t *testing.T) {
	mgr := NewManager("nonexistent-check-test", "/tmp", "task")

	// checkSessionExists should return false for a non-existent session
	exists := mgr.checkSessionExists("nonexistent-session-xyz")
	if exists {
		t.Error("checkSessionExists for non-existent session should return false")
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

// Tests for lifecycle.Instance interface implementation

func TestManager_WorkDir(t *testing.T) {
	mgr := NewManager("test-id", "/custom/workdir", "task")
	if mgr.WorkDir() != "/custom/workdir" {
		t.Errorf("WorkDir() = %q, want %q", mgr.WorkDir(), "/custom/workdir")
	}
}

func TestManager_Task(t *testing.T) {
	mgr := NewManager("test-id", "/tmp", "custom task prompt")
	if mgr.Task() != "custom task prompt" {
		t.Errorf("Task() = %q, want %q", mgr.Task(), "custom task prompt")
	}
}

func TestManager_Config_ReturnsLifecycleConfig(t *testing.T) {
	cfg := ManagerConfig{
		TmuxWidth:  150,
		TmuxHeight: 40,
	}
	mgr := NewManagerWithConfig("test", "/tmp", "task", cfg)

	lcConfig := mgr.Config()
	if lcConfig.TmuxWidth != 150 {
		t.Errorf("Config().TmuxWidth = %d, want %d", lcConfig.TmuxWidth, 150)
	}
	if lcConfig.TmuxHeight != 40 {
		t.Errorf("Config().TmuxHeight = %d, want %d", lcConfig.TmuxHeight, 40)
	}
}

func TestManager_SetRunning(t *testing.T) {
	mgr := NewManager("test", "/tmp", "task")

	if mgr.Running() {
		t.Error("Running() should initially be false")
	}

	mgr.SetRunning(true)
	if !mgr.Running() {
		t.Error("Running() should be true after SetRunning(true)")
	}

	mgr.SetRunning(false)
	if mgr.Running() {
		t.Error("Running() should be false after SetRunning(false)")
	}
}

func TestManager_IsRunning(t *testing.T) {
	mgr := NewManager("test", "/tmp", "task")

	// IsRunning and Running should return the same value
	if mgr.IsRunning() != mgr.Running() {
		t.Error("IsRunning() and Running() should return the same value")
	}

	mgr.SetRunning(true)
	if mgr.IsRunning() != mgr.Running() {
		t.Error("IsRunning() and Running() should return the same value after SetRunning")
	}
}

func TestManager_SetStartTime(t *testing.T) {
	mgr := NewManager("test", "/tmp", "task")

	if mgr.StartTime() != nil {
		t.Error("StartTime() should be nil initially")
	}

	now := time.Now()
	mgr.SetStartTime(now)

	startTime := mgr.StartTime()
	if startTime == nil {
		t.Fatal("StartTime() should not be nil after SetStartTime")
	}
	if !startTime.Equal(now) {
		t.Errorf("StartTime() = %v, want %v", *startTime, now)
	}
}

func TestManager_SetLifecycleManager(t *testing.T) {
	mgr := NewManager("test", "/tmp", "task")

	if mgr.LifecycleManager() != nil {
		t.Error("LifecycleManager() should initially be nil")
	}

	// Create a lifecycle manager and set it
	// Note: We pass nil logger since we're just testing the setter
	lm := lifecycle.NewManager(nil)
	mgr.SetLifecycleManager(lm)

	if mgr.LifecycleManager() != lm {
		t.Error("LifecycleManager() should return the set manager")
	}

	// Clearing it
	mgr.SetLifecycleManager(nil)
	if mgr.LifecycleManager() != nil {
		t.Error("LifecycleManager() should be nil after setting nil")
	}
}

func TestManager_LifecycleConfig(t *testing.T) {
	cfg := ManagerConfig{
		OutputBufferSize:  1000,
		CaptureIntervalMs: 50,
		TmuxWidth:         180,
		TmuxHeight:        25,
		TmuxHistoryLimit:  75000,
	}
	mgr := NewManagerWithConfig("test", "/tmp", "task", cfg)

	lcConfig := mgr.LifecycleConfig()
	if lcConfig.TmuxWidth != 180 {
		t.Errorf("LifecycleConfig().TmuxWidth = %d, want %d", lcConfig.TmuxWidth, 180)
	}
	if lcConfig.TmuxHeight != 25 {
		t.Errorf("LifecycleConfig().TmuxHeight = %d, want %d", lcConfig.TmuxHeight, 25)
	}
	if lcConfig.TmuxHistoryLimit != 75000 {
		t.Errorf("LifecycleConfig().TmuxHistoryLimit = %d, want %d", lcConfig.TmuxHistoryLimit, 75000)
	}
}

func TestManager_SocketName(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		wantSocket string
	}{
		{
			name:       "standard instance ID",
			instanceID: "abc123",
			wantSocket: "claudio-abc123",
		},
		{
			name:       "longer instance ID",
			instanceID: "instance-12345678",
			wantSocket: "claudio-instance-12345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewManager(tt.instanceID, "/tmp", "test task")
			got := mgr.SocketName()
			if got != tt.wantSocket {
				t.Errorf("SocketName() = %q, want %q", got, tt.wantSocket)
			}
		})
	}
}

func TestTmuxSessionInfo_KillCommand(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		socketName  string
	}{
		{
			name:        "standard session",
			sessionName: "claudio-abc123",
			socketName:  "claudio-abc123",
		},
		{
			name:        "default socket",
			sessionName: "claudio-term-xyz",
			socketName:  "claudio",
		},
		{
			name:        "session with session ID",
			sessionName: "claudio-sess1-inst1",
			socketName:  "claudio-inst1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := TmuxSessionInfo{
				SessionName: tt.sessionName,
				SocketName:  tt.socketName,
			}

			cmd := info.KillCommand()
			args := cmd.Args

			// Verify command structure: tmux -L <socket> kill-session -t <session>
			if len(args) < 6 {
				t.Fatalf("Expected at least 6 args, got %d: %v", len(args), args)
			}

			if args[0] != "tmux" {
				t.Errorf("args[0] = %q, want %q", args[0], "tmux")
			}
			if args[1] != "-L" {
				t.Errorf("args[1] = %q, want %q", args[1], "-L")
			}
			if args[2] != tt.socketName {
				t.Errorf("args[2] (socket) = %q, want %q", args[2], tt.socketName)
			}
			if args[3] != "kill-session" {
				t.Errorf("args[3] = %q, want %q", args[3], "kill-session")
			}
			if args[4] != "-t" {
				t.Errorf("args[4] = %q, want %q", args[4], "-t")
			}
			if args[5] != tt.sessionName {
				t.Errorf("args[5] (session) = %q, want %q", args[5], tt.sessionName)
			}
		})
	}
}
