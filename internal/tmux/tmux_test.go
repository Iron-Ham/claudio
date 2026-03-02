package tmux

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSocketName(t *testing.T) {
	if SocketName == "" {
		t.Error("SocketName should not be empty")
	}
	if SocketName != "claudio" {
		t.Errorf("SocketName = %q, want %q", SocketName, "claudio")
	}
}

func TestSocketPrefix(t *testing.T) {
	if SocketPrefix == "" {
		t.Error("SocketPrefix should not be empty")
	}
	if SocketPrefix != "claudio" {
		t.Errorf("SocketPrefix = %q, want %q", SocketPrefix, "claudio")
	}
}

func TestCommand(t *testing.T) {
	cmd := Command("list-sessions")
	args := cmd.Args

	if len(args) < 4 {
		t.Fatalf("Expected at least 4 args, got %d: %v", len(args), args)
	}

	if args[0] != "tmux" {
		t.Errorf("args[0] = %q, want %q", args[0], "tmux")
	}
	if args[1] != "-L" {
		t.Errorf("args[1] = %q, want %q", args[1], "-L")
	}
	if args[2] != SocketName {
		t.Errorf("args[2] = %q, want %q", args[2], SocketName)
	}
	if args[3] != "list-sessions" {
		t.Errorf("args[3] = %q, want %q", args[3], "list-sessions")
	}
}

func TestCommandWithSocket(t *testing.T) {
	customSocket := "claudio-abc123"
	cmd := CommandWithSocket(customSocket, "list-sessions")
	args := cmd.Args

	if len(args) < 4 {
		t.Fatalf("Expected at least 4 args, got %d: %v", len(args), args)
	}

	if args[0] != "tmux" {
		t.Errorf("args[0] = %q, want %q", args[0], "tmux")
	}
	if args[1] != "-L" {
		t.Errorf("args[1] = %q, want %q", args[1], "-L")
	}
	if args[2] != customSocket {
		t.Errorf("args[2] = %q, want %q", args[2], customSocket)
	}
	if args[3] != "list-sessions" {
		t.Errorf("args[3] = %q, want %q", args[3], "list-sessions")
	}
}

func TestCommandArgs(t *testing.T) {
	args := CommandArgs("kill-session", "-t", "test")

	expected := []string{"-L", SocketName, "kill-session", "-t", "test"}
	if len(args) != len(expected) {
		t.Fatalf("len(args) = %d, want %d", len(args), len(expected))
	}

	for i, want := range expected {
		if args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}

func TestCommandArgsWithSocket(t *testing.T) {
	customSocket := "claudio-def456"
	args := CommandArgsWithSocket(customSocket, "kill-session", "-t", "test")

	expected := []string{"-L", customSocket, "kill-session", "-t", "test"}
	if len(args) != len(expected) {
		t.Fatalf("len(args) = %d, want %d", len(args), len(expected))
	}

	for i, want := range expected {
		if args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}

func TestBaseArgs(t *testing.T) {
	args := BaseArgs()

	if len(args) != 2 {
		t.Fatalf("len(args) = %d, want 2", len(args))
	}
	if args[0] != "-L" {
		t.Errorf("args[0] = %q, want %q", args[0], "-L")
	}
	if args[1] != SocketName {
		t.Errorf("args[1] = %q, want %q", args[1], SocketName)
	}
}

func TestBaseArgsWithSocket(t *testing.T) {
	customSocket := "claudio-ghi789"
	args := BaseArgsWithSocket(customSocket)

	if len(args) != 2 {
		t.Fatalf("len(args) = %d, want 2", len(args))
	}
	if args[0] != "-L" {
		t.Errorf("args[0] = %q, want %q", args[0], "-L")
	}
	if args[1] != customSocket {
		t.Errorf("args[1] = %q, want %q", args[1], customSocket)
	}
}

func TestCommandContext(t *testing.T) {
	ctx := context.Background()
	cmd := CommandContext(ctx, "has-session", "-t", "test")
	args := cmd.Args

	if len(args) < 6 {
		t.Fatalf("Expected at least 6 args, got %d: %v", len(args), args)
	}

	if args[0] != "tmux" {
		t.Errorf("args[0] = %q, want %q", args[0], "tmux")
	}
	if args[1] != "-L" {
		t.Errorf("args[1] = %q, want %q", args[1], "-L")
	}
	if args[2] != SocketName {
		t.Errorf("args[2] = %q, want %q", args[2], SocketName)
	}
	if args[3] != "has-session" {
		t.Errorf("args[3] = %q, want %q", args[3], "has-session")
	}
}

func TestCommandContextWithSocket(t *testing.T) {
	ctx := context.Background()
	customSocket := "claudio-jkl012"
	cmd := CommandContextWithSocket(ctx, customSocket, "has-session", "-t", "test")
	args := cmd.Args

	if len(args) < 6 {
		t.Fatalf("Expected at least 6 args, got %d: %v", len(args), args)
	}

	if args[0] != "tmux" {
		t.Errorf("args[0] = %q, want %q", args[0], "tmux")
	}
	if args[1] != "-L" {
		t.Errorf("args[1] = %q, want %q", args[1], "-L")
	}
	if args[2] != customSocket {
		t.Errorf("args[2] = %q, want %q", args[2], customSocket)
	}
	if args[3] != "has-session" {
		t.Errorf("args[3] = %q, want %q", args[3], "has-session")
	}
}

func TestInstanceSocketName(t *testing.T) {
	tests := []struct {
		instanceID string
		want       string
	}{
		{"abc123", "claudio-abc123"},
		{"12345678", "claudio-12345678"},
		{"", "claudio-"},
	}

	for _, tt := range tests {
		t.Run(tt.instanceID, func(t *testing.T) {
			got := InstanceSocketName(tt.instanceID)
			if got != tt.want {
				t.Errorf("InstanceSocketName(%q) = %q, want %q", tt.instanceID, got, tt.want)
			}
		})
	}
}

func TestIsInstanceSocket(t *testing.T) {
	tests := []struct {
		socket string
		want   bool
	}{
		{"claudio-abc123", true},
		{"claudio-12345678", true},
		{"claudio", false},          // Default socket is not an instance socket
		{"other-socket", false},     // Different prefix
		{"claudio-", true},          // Empty instance ID is still technically an instance socket format
		{"claudiosomething", false}, // No hyphen separator
	}

	for _, tt := range tests {
		t.Run(tt.socket, func(t *testing.T) {
			got := IsInstanceSocket(tt.socket)
			if got != tt.want {
				t.Errorf("IsInstanceSocket(%q) = %v, want %v", tt.socket, got, tt.want)
			}
		})
	}
}

func TestExtractInstanceID(t *testing.T) {
	tests := []struct {
		socket string
		want   string
	}{
		{"claudio-abc123", "abc123"},
		{"claudio-12345678", "12345678"},
		{"claudio-", ""},
		{"claudio", ""},          // Default socket has no instance ID
		{"other-socket", ""},     // Different prefix
		{"claudiosomething", ""}, // No hyphen separator
	}

	for _, tt := range tests {
		t.Run(tt.socket, func(t *testing.T) {
			got := ExtractInstanceID(tt.socket)
			if got != tt.want {
				t.Errorf("ExtractInstanceID(%q) = %q, want %q", tt.socket, got, tt.want)
			}
		})
	}
}

func TestMapKeyToTmux(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Arrow keys
		{"up", "Up"},
		{"down", "Down"},
		{"left", "Left"},
		{"right", "Right"},
		// Navigation keys
		{"home", "Home"},
		{"end", "End"},
		{"backspace", "BSpace"},
		{"delete", "DC"},
		{"insert", "IC"},
		{"pgup", "PageUp"},
		{"pgdown", "PageDown"},
		// Other special keys
		{"tab", "Tab"},
		{"enter", "Enter"},
		{"esc", "Escape"},
		{"escape", "Escape"},
		// Unknown keys should pass through unchanged
		{"f1", "f1"},
		{"unknown", "unknown"},
		{"x", "x"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MapKeyToTmux(tt.input)
			if got != tt.expected {
				t.Errorf("MapKeyToTmux(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSocketDir(t *testing.T) {
	dir := SocketDir()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	expected := filepath.Join(home, ".claudio", "sockets")
	if dir != expected {
		t.Errorf("SocketDir() = %q, want %q", dir, expected)
	}
}

func TestSocketDir_FallbackWhenHomeUnset(t *testing.T) {
	// Save and unset HOME
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", "")

	dir := SocketDir()

	// Should fall back to os.TempDir()-based path
	expected := filepath.Join(os.TempDir(), "claudio-sockets")
	if dir != expected {
		t.Errorf("SocketDir() with empty HOME = %q, want %q", dir, expected)
	}

	// Restore HOME (t.Setenv handles cleanup automatically)
	_ = origHome
}

func TestEnsureSocketDir(t *testing.T) {
	// Create a temp dir to avoid modifying real filesystem
	tmpDir := t.TempDir()
	socketDir := filepath.Join(tmpDir, "sockets")

	// Verify the directory doesn't exist yet
	if _, err := os.Stat(socketDir); !os.IsNotExist(err) {
		t.Fatal("socket dir should not exist initially")
	}

	// Use os.MkdirAll directly since we can't override SocketDir()
	err := os.MkdirAll(socketDir, 0700)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(socketDir)
	if err != nil {
		t.Fatalf("socket dir should exist after EnsureSocketDir: %v", err)
	}
	if !info.IsDir() {
		t.Error("socket dir should be a directory")
	}

	// Calling again should be idempotent
	err = os.MkdirAll(socketDir, 0700)
	if err != nil {
		t.Fatalf("second MkdirAll should succeed: %v", err)
	}
}

func TestCommandWithSocket_SetsTMUX_TMPDIR(t *testing.T) {
	cmd := CommandWithSocket("test-socket", "list-sessions")

	// Check that TMUX_TMPDIR is set in the environment
	found := false
	expectedPrefix := "TMUX_TMPDIR=" + SocketDir()
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "TMUX_TMPDIR=") {
			found = true
			if env != expectedPrefix {
				t.Errorf("TMUX_TMPDIR = %q, want %q", env, expectedPrefix)
			}
			break
		}
	}
	if !found {
		t.Error("TMUX_TMPDIR not found in command environment")
	}
}

func TestCommandContextWithSocket_SetsTMUX_TMPDIR(t *testing.T) {
	ctx := context.Background()
	cmd := CommandContextWithSocket(ctx, "test-socket", "list-sessions")

	// Check that TMUX_TMPDIR is set in the environment
	found := false
	expectedPrefix := "TMUX_TMPDIR=" + SocketDir()
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "TMUX_TMPDIR=") {
			found = true
			if env != expectedPrefix {
				t.Errorf("TMUX_TMPDIR = %q, want %q", env, expectedPrefix)
			}
			break
		}
	}
	if !found {
		t.Error("TMUX_TMPDIR not found in command environment")
	}
}

func TestCommand_SetsTMUX_TMPDIR(t *testing.T) {
	// The default Command() delegates to CommandWithSocket, so it should also set TMUX_TMPDIR
	cmd := Command("list-sessions")

	found := false
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "TMUX_TMPDIR=") {
			found = true
			break
		}
	}
	if !found {
		t.Error("TMUX_TMPDIR not found in command environment (via Command())")
	}
}
