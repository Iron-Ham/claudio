package terminal

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/tmux"
)

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
