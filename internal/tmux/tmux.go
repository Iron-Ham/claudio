// Package tmux provides centralized configuration and helpers for tmux operations.
//
// Claudio uses a dedicated tmux socket to isolate its sessions from other tmux
// clients (like iTerm2's tmux integration). This prevents crashes caused by
// control-mode notification bugs in tmux when multiple clients share a server.
package tmux

import (
	"context"
	"os/exec"
)

// SocketName is the dedicated tmux socket name for Claudio.
// Using a dedicated socket isolates Claudio from other tmux clients
// that may use control mode (like iTerm2), preventing crashes from
// tmux control-mode notification bugs.
const SocketName = "claudio"

// Command creates an exec.Cmd for tmux with the Claudio socket.
// This ensures all Claudio tmux operations use the dedicated socket.
func Command(args ...string) *exec.Cmd {
	fullArgs := append([]string{"-L", SocketName}, args...)
	return exec.Command("tmux", fullArgs...)
}

// CommandContext creates a context-aware exec.Cmd for tmux with the Claudio socket.
// Use this when the command should be cancellable via context.
func CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	fullArgs := append([]string{"-L", SocketName}, args...)
	return exec.CommandContext(ctx, "tmux", fullArgs...)
}

// CommandArgs returns the arguments needed to run a tmux command
// with the Claudio socket. Use this when you need to build the
// command string differently (e.g., for display purposes).
func CommandArgs(args ...string) []string {
	return append([]string{"-L", SocketName}, args...)
}

// BaseArgs returns just the socket arguments [-L, claudio].
// Use this when you need to prepend socket args to existing argument slices.
func BaseArgs() []string {
	return []string{"-L", SocketName}
}
