package terminal

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/tmux"
)

// mockCommandRunner records all tmux commands for verification in tests.
type mockCommandRunner struct {
	commands              []mockCommand // recorded commands
	runErr                error         // error to return from Run
	outputResult          []byte        // result to return from Output
	outputErr             error         // error to return from Output
	blockUntilContextDone bool          // if true, OutputWithContext blocks until context is done
}

// mockCommand represents a recorded tmux command.
type mockCommand struct {
	socketName string
	args       []string
	hasEnv     bool // whether this was from CommandWithEnv
}

func (m *mockCommandRunner) Run(socketName string, args ...string) error {
	m.commands = append(m.commands, mockCommand{
		socketName: socketName,
		args:       args,
	})
	return m.runErr
}

func (m *mockCommandRunner) Output(socketName string, args ...string) ([]byte, error) {
	m.commands = append(m.commands, mockCommand{
		socketName: socketName,
		args:       args,
	})
	return m.outputResult, m.outputErr
}

func (m *mockCommandRunner) OutputWithContext(ctx context.Context, socketName string, args ...string) ([]byte, error) {
	m.commands = append(m.commands, mockCommand{
		socketName: socketName,
		args:       args,
	})

	// If blockUntilContextDone is set, wait for the context to be done.
	// This simulates a tmux command that hangs until the timeout expires,
	// which is the real-world scenario where ctx.Err() returns DeadlineExceeded.
	if m.blockUntilContextDone {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	return m.outputResult, m.outputErr
}

func (m *mockCommandRunner) CommandWithEnv(socketName string, args ...string) *exec.Cmd {
	// Record the command but return a real exec.Cmd that we can inspect
	// We use /bin/sh -c true which ignores Dir settings that might not exist
	cmd := exec.Command("/bin/sh", "-c", "true")
	m.commands = append(m.commands, mockCommand{
		socketName: socketName,
		args:       args,
		hasEnv:     true,
	})
	return cmd
}

func TestNewProcess(t *testing.T) {
	p := NewProcess("session123", "/tmp", 100, 50)

	if p == nil {
		t.Fatal("NewProcess returned nil")
	}

	// Should use the default socket
	if got := p.SocketName(); got != tmux.SocketName {
		t.Errorf("SocketName() = %q, want default %q", got, tmux.SocketName)
	}

	// Session name should be formatted correctly
	expectedSession := "claudio-term-session123"
	if got := p.SessionName(); got != expectedSession {
		t.Errorf("SessionName() = %q, want %q", got, expectedSession)
	}
}

func TestNewProcessWithSocket(t *testing.T) {
	customSocket := "claudio-custom456"
	p := NewProcessWithSocket("session123", customSocket, "/tmp", 100, 50)

	if p == nil {
		t.Fatal("NewProcessWithSocket returned nil")
	}

	// Should use the custom socket
	if got := p.SocketName(); got != customSocket {
		t.Errorf("SocketName() = %q, want %q", got, customSocket)
	}

	// Session name should still be formatted correctly
	expectedSession := "claudio-term-session123"
	if got := p.SessionName(); got != expectedSession {
		t.Errorf("SessionName() = %q, want %q", got, expectedSession)
	}
}

func TestProcess_SocketName(t *testing.T) {
	tests := []struct {
		name       string
		socketName string
	}{
		{
			name:       "default socket",
			socketName: tmux.SocketName,
		},
		{
			name:       "custom instance socket",
			socketName: "claudio-abc123",
		},
		{
			name:       "another custom socket",
			socketName: "claudio-xyz789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProcessWithSocket("test-session", tt.socketName, "/tmp", 100, 50)
			if got := p.SocketName(); got != tt.socketName {
				t.Errorf("SocketName() = %q, want %q", got, tt.socketName)
			}
		})
	}
}

func TestProcess_AttachCommand(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		socketName  string
		wantCommand string
	}{
		{
			name:        "default socket",
			sessionID:   "sess1",
			socketName:  tmux.SocketName,
			wantCommand: "tmux -L claudio attach -t claudio-term-sess1",
		},
		{
			name:        "custom socket",
			sessionID:   "sess2",
			socketName:  "claudio-custom",
			wantCommand: "tmux -L claudio-custom attach -t claudio-term-sess2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProcessWithSocket(tt.sessionID, tt.socketName, "/tmp", 100, 50)
			if got := p.AttachCommand(); got != tt.wantCommand {
				t.Errorf("AttachCommand() = %q, want %q", got, tt.wantCommand)
			}
		})
	}
}

func TestProcess_IsRunning_Initial(t *testing.T) {
	p := NewProcess("test", "/tmp", 100, 50)

	if p.IsRunning() {
		t.Error("New process should not be running")
	}
}

func TestProcess_CurrentDir(t *testing.T) {
	invocationDir := "/home/user/project"
	p := NewProcess("test", invocationDir, 100, 50)

	if got := p.CurrentDir(); got != invocationDir {
		t.Errorf("CurrentDir() = %q, want %q", got, invocationDir)
	}

	if got := p.InvocationDir(); got != invocationDir {
		t.Errorf("InvocationDir() = %q, want %q", got, invocationDir)
	}
}

func TestProcess_Start_TmuxCommandSequence(t *testing.T) {
	mock := &mockCommandRunner{}
	// Use /tmp as it's guaranteed to exist on all Unix systems
	p := NewProcessWithRunner("test-session", "test-socket", "/tmp", 100, 50, mock)

	err := p.Start()
	if err != nil {
		t.Fatalf("Start() returned unexpected error: %v", err)
	}

	// Verify the correct sequence of tmux commands was executed
	// Expected sequence:
	// 1. kill-session (cleanup any existing session)
	// 2. set-option -g history-limit 50000
	// 3. new-session (via CommandWithEnv for environment variable support)
	// 4. set-option -t <session> default-terminal xterm-256color

	if len(mock.commands) != 4 {
		t.Fatalf("Expected 4 tmux commands, got %d: %+v", len(mock.commands), mock.commands)
	}

	// Command 1: kill-session cleanup
	cmd := mock.commands[0]
	if cmd.socketName != "test-socket" {
		t.Errorf("Command 1: socket = %q, want %q", cmd.socketName, "test-socket")
	}
	if len(cmd.args) < 1 || cmd.args[0] != "kill-session" {
		t.Errorf("Command 1: expected kill-session, got %v", cmd.args)
	}

	// Command 2: set history-limit globally (before session creation)
	cmd = mock.commands[1]
	expectedArgs := []string{"set-option", "-g", "history-limit", "50000"}
	if !slices.Equal(cmd.args, expectedArgs) {
		t.Errorf("Command 2: args = %v, want %v", cmd.args, expectedArgs)
	}

	// Command 3: new-session with proper dimensions (via CommandWithEnv)
	cmd = mock.commands[2]
	if !cmd.hasEnv {
		t.Errorf("Command 3: expected CommandWithEnv to be used for new-session")
	}
	if len(cmd.args) < 1 || cmd.args[0] != "new-session" {
		t.Errorf("Command 3: expected new-session, got %v", cmd.args)
	}
	// Verify session name and dimensions are in args
	argsStr := strings.Join(cmd.args, " ")
	if !strings.Contains(argsStr, "claudio-term-test-session") {
		t.Errorf("Command 3: session name not found in args: %v", cmd.args)
	}
	if !strings.Contains(argsStr, "-x 100") {
		t.Errorf("Command 3: width not found in args: %v", cmd.args)
	}
	if !strings.Contains(argsStr, "-y 50") {
		t.Errorf("Command 3: height not found in args: %v", cmd.args)
	}

	// Command 4: set default-terminal per-session (not global)
	cmd = mock.commands[3]
	if cmd.args[0] != "set-option" {
		t.Errorf("Command 4: expected set-option, got %v", cmd.args)
	}
	// Should use -t (per-session) not -g (global)
	if !slices.Contains(cmd.args, "-t") {
		t.Errorf("Command 4: expected -t flag for per-session option, got %v", cmd.args)
	}
	if slices.Contains(cmd.args, "-g") {
		t.Errorf("Command 4: should not use -g (global) for default-terminal, got %v", cmd.args)
	}
	if !slices.Contains(cmd.args, "default-terminal") || !slices.Contains(cmd.args, "xterm-256color") {
		t.Errorf("Command 4: expected default-terminal xterm-256color, got %v", cmd.args)
	}

	// Verify running state
	if !p.IsRunning() {
		t.Error("Process should be running after Start()")
	}
}

func TestProcess_Start_DefaultDimensions(t *testing.T) {
	mock := &mockCommandRunner{}
	// Use 0 for width and height to test default values
	p := NewProcessWithRunner("test-session", "test-socket", "/tmp", 0, 0, mock)

	err := p.Start()
	if err != nil {
		t.Fatalf("Start() returned unexpected error: %v", err)
	}

	// Find the new-session command
	var newSessionCmd *mockCommand
	for i := range mock.commands {
		if len(mock.commands[i].args) > 0 && mock.commands[i].args[0] == "new-session" {
			newSessionCmd = &mock.commands[i]
			break
		}
	}

	if newSessionCmd == nil {
		t.Fatal("new-session command not found")
	}

	// Check for default dimensions (200x10)
	argsStr := strings.Join(newSessionCmd.args, " ")
	if !strings.Contains(argsStr, "-x 200") {
		t.Errorf("Expected default width 200, got args: %v", newSessionCmd.args)
	}
	if !strings.Contains(argsStr, "-y 10") {
		t.Errorf("Expected default height 10, got args: %v", newSessionCmd.args)
	}
}

func TestProcess_Start_AlreadyRunning(t *testing.T) {
	mock := &mockCommandRunner{}
	p := NewProcessWithRunner("test", "socket", "/tmp", 100, 50, mock)

	// Start the process
	if err := p.Start(); err != nil {
		t.Fatalf("First Start() failed: %v", err)
	}

	// Try to start again
	err := p.Start()
	if err != ErrAlreadyRunning {
		t.Errorf("Second Start() = %v, want ErrAlreadyRunning", err)
	}
}

func TestProcess_Stop_TmuxCommand(t *testing.T) {
	mock := &mockCommandRunner{}
	p := NewProcessWithRunner("test", "test-socket", "/tmp", 100, 50, mock)

	// Start the process first
	if err := p.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Clear recorded commands
	mock.commands = nil

	// Stop the process
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	// Verify kill-session command was sent
	if len(mock.commands) != 1 {
		t.Fatalf("Expected 1 command (kill-session), got %d", len(mock.commands))
	}

	cmd := mock.commands[0]
	if cmd.args[0] != "kill-session" {
		t.Errorf("Expected kill-session, got %v", cmd.args)
	}
	if !slices.Contains(cmd.args, "claudio-term-test") {
		t.Errorf("Expected session name in kill-session args, got %v", cmd.args)
	}

	// Verify not running
	if p.IsRunning() {
		t.Error("Process should not be running after Stop()")
	}
}

func TestProcess_NoMouseOrAggressiveResize(t *testing.T) {
	// This test ensures we don't add mouse support or aggressive-resize
	// as those were identified as scope creep in the review.
	mock := &mockCommandRunner{}
	p := NewProcessWithRunner("test", "socket", "/tmp", 100, 50, mock)

	if err := p.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify no mouse or aggressive-resize commands were sent
	for _, cmd := range mock.commands {
		argsStr := strings.Join(cmd.args, " ")
		if strings.Contains(argsStr, "mouse") {
			t.Errorf("Unexpected mouse command found: %v", cmd.args)
		}
		if strings.Contains(argsStr, "aggressive-resize") {
			t.Errorf("Unexpected aggressive-resize command found: %v", cmd.args)
		}
	}
}

func TestNewProcessWithRunner(t *testing.T) {
	mock := &mockCommandRunner{}
	p := NewProcessWithRunner("session123", "custom-socket", "/home/user", 80, 24, mock)

	if p == nil {
		t.Fatal("NewProcessWithRunner returned nil")
	}

	if got := p.SocketName(); got != "custom-socket" {
		t.Errorf("SocketName() = %q, want %q", got, "custom-socket")
	}

	expectedSession := "claudio-term-session123"
	if got := p.SessionName(); got != expectedSession {
		t.Errorf("SessionName() = %q, want %q", got, expectedSession)
	}
}

func TestProcess_CaptureOutput_UsesContextForTimeout(t *testing.T) {
	mock := &mockCommandRunner{
		outputResult: []byte("terminal output"),
	}
	p := NewProcessWithRunner("test", "socket", "/tmp", 100, 50, mock)

	// Mark as running so capture is attempted
	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	output, err := p.CaptureOutput()
	if err != nil {
		t.Fatalf("CaptureOutput() returned unexpected error: %v", err)
	}

	if output != "terminal output" {
		t.Errorf("CaptureOutput() = %q, want %q", output, "terminal output")
	}

	// Verify capture-pane command was called
	found := false
	for _, cmd := range mock.commands {
		if slices.Contains(cmd.args, "capture-pane") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected capture-pane command to be called")
	}
}

func TestProcess_CaptureOutput_ReturnsErrCaptureTimeout(t *testing.T) {
	// To properly test ErrCaptureTimeout, the mock must block until the context
	// times out. This causes ctx.Err() to return context.DeadlineExceeded, which
	// triggers the ErrCaptureTimeout code path.
	mock := &mockCommandRunner{
		blockUntilContextDone: true,
	}
	p := NewProcessWithRunner("test", "socket", "/tmp", 100, 50, mock)

	// Mark as running so capture is attempted
	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	// CaptureOutput has a 500ms timeout (captureTimeout constant).
	// The mock will block until that timeout expires, then return ctx.Err().
	_, err := p.CaptureOutput()
	if err != ErrCaptureTimeout {
		t.Errorf("CaptureOutput() error = %v, want ErrCaptureTimeout", err)
	}
}

func TestProcess_CaptureOutput_NotRunning(t *testing.T) {
	mock := &mockCommandRunner{}
	p := NewProcessWithRunner("test", "socket", "/tmp", 100, 50, mock)

	// Process not running - should return ErrNotRunning
	output, err := p.CaptureOutput()
	if err != ErrNotRunning {
		t.Errorf("CaptureOutput() error = %v, want ErrNotRunning", err)
	}
	if output != "" {
		t.Errorf("CaptureOutput() = %q, want empty string", output)
	}
}

func TestProcess_CaptureOutputWithHistory_UsesContextForTimeout(t *testing.T) {
	mock := &mockCommandRunner{
		outputResult: []byte("terminal output with history"),
	}
	p := NewProcessWithRunner("test", "socket", "/tmp", 100, 50, mock)

	// Mark as running so capture is attempted
	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	output, err := p.CaptureOutputWithHistory(100)
	if err != nil {
		t.Fatalf("CaptureOutputWithHistory() returned unexpected error: %v", err)
	}

	if output != "terminal output with history" {
		t.Errorf("CaptureOutputWithHistory() = %q, want %q", output, "terminal output with history")
	}

	// Verify capture-pane command was called with history flag
	found := false
	for _, cmd := range mock.commands {
		if slices.Contains(cmd.args, "capture-pane") && slices.Contains(cmd.args, "-S") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected capture-pane command with -S flag to be called")
	}
}

func TestProcess_CaptureOutput_ReturnsErrCaptureKilled(t *testing.T) {
	// Create a real exec.ExitError with exit code -1 by running a process
	// that gets killed by a signal. This simulates what happens when tmux
	// capture-pane is killed due to context cancellation.
	killedErr := createSignalKilledError(t)
	if killedErr == nil {
		t.Skip("Could not create signal-killed error on this platform")
	}

	mock := &mockCommandRunner{
		outputErr: killedErr,
	}
	p := NewProcessWithRunner("test", "socket", "/tmp", 100, 50, mock)

	// Mark as running so capture is attempted
	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	_, err := p.CaptureOutput()
	if err != ErrCaptureKilled {
		t.Errorf("CaptureOutput() error = %v, want ErrCaptureKilled", err)
	}
}

func TestProcess_CaptureOutputWithHistory_ReturnsErrCaptureTimeout(t *testing.T) {
	// Test that CaptureOutputWithHistory also returns ErrCaptureTimeout when
	// the context times out.
	mock := &mockCommandRunner{
		blockUntilContextDone: true,
	}
	p := NewProcessWithRunner("test", "socket", "/tmp", 100, 50, mock)

	// Mark as running so capture is attempted
	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	_, err := p.CaptureOutputWithHistory(100)
	if err != ErrCaptureTimeout {
		t.Errorf("CaptureOutputWithHistory() error = %v, want ErrCaptureTimeout", err)
	}
}

func TestProcess_CaptureOutputWithHistory_ReturnsErrCaptureKilled(t *testing.T) {
	killedErr := createSignalKilledError(t)
	if killedErr == nil {
		t.Skip("Could not create signal-killed error on this platform")
	}

	mock := &mockCommandRunner{
		outputErr: killedErr,
	}
	p := NewProcessWithRunner("test", "socket", "/tmp", 100, 50, mock)

	// Mark as running so capture is attempted
	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	_, err := p.CaptureOutputWithHistory(100)
	if err != ErrCaptureKilled {
		t.Errorf("CaptureOutputWithHistory() error = %v, want ErrCaptureKilled", err)
	}
}

func TestProcess_CaptureOutputWithHistory_NotRunning(t *testing.T) {
	mock := &mockCommandRunner{}
	p := NewProcessWithRunner("test", "socket", "/tmp", 100, 50, mock)

	// Process not running - should return ErrNotRunning
	output, err := p.CaptureOutputWithHistory(100)
	if err != ErrNotRunning {
		t.Errorf("CaptureOutputWithHistory() error = %v, want ErrNotRunning", err)
	}
	if output != "" {
		t.Errorf("CaptureOutputWithHistory() = %q, want empty string", output)
	}
}

// createSignalKilledError creates an exec.ExitError with exit code -1 by running
// a process that kills itself with a signal. This is used to test the ErrCaptureKilled
// error path which handles processes killed by context cancellation.
func createSignalKilledError(t *testing.T) *exec.ExitError {
	t.Helper()

	// Run a shell command that immediately kills itself with SIGKILL.
	// This produces an ExitError with ExitCode() == -1.
	cmd := exec.Command("/bin/sh", "-c", "kill -9 $$")
	err := cmd.Run()
	if err == nil {
		return nil
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return nil
	}

	// Verify it has the expected exit code
	if exitErr.ExitCode() != -1 {
		t.Logf("Warning: expected exit code -1, got %d", exitErr.ExitCode())
		return nil
	}

	return exitErr
}
