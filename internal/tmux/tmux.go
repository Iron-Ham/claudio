// Package tmux provides centralized configuration and helpers for tmux operations.
//
// Claudio uses per-instance tmux sockets to isolate each instance's sessions.
// This prevents a crash in one instance's tmux server from affecting other instances.
// Each instance uses a socket named "claudio-{instanceID}", providing complete
// isolation between instances.
//
// The default "claudio" socket is used for global operations like listing all
// sessions or cleanup operations that need to work across multiple instances.
package tmux

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

// SocketName is the base tmux socket name for Claudio global operations.
// Individual instances use sockets named "claudio-{instanceID}" for isolation.
const SocketName = "claudio"

// SocketPrefix is the prefix used for all Claudio tmux sockets.
// Instance sockets are named "{SocketPrefix}-{instanceID}".
const SocketPrefix = "claudio"

// Command creates an exec.Cmd for tmux with the default Claudio socket.
// Use this for global operations like listing all sessions or cleanup.
// For instance-specific operations, use CommandWithSocket instead.
func Command(args ...string) *exec.Cmd {
	return CommandWithSocket(SocketName, args...)
}

// CommandContext creates a context-aware exec.Cmd for tmux with the default socket.
// Use this for global operations. For instance-specific operations, use
// CommandContextWithSocket instead.
func CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	return CommandContextWithSocket(ctx, SocketName, args...)
}

// CommandWithSocket creates an exec.Cmd for tmux with a custom socket name.
// Use this for instance-specific operations to ensure socket isolation.
// Sets TMUX_TMPDIR so sockets are placed in SocketDir() instead of /tmp.
func CommandWithSocket(socket string, args ...string) *exec.Cmd {
	fullArgs := append([]string{"-L", socket}, args...)
	cmd := exec.Command("tmux", fullArgs...)
	cmd.Env = append(os.Environ(), "TMUX_TMPDIR="+SocketDir())
	return cmd
}

// CommandContextWithSocket creates a context-aware exec.Cmd with a custom socket.
// Use this for instance-specific operations that need context cancellation.
// Sets TMUX_TMPDIR so sockets are placed in SocketDir() instead of /tmp.
func CommandContextWithSocket(ctx context.Context, socket string, args ...string) *exec.Cmd {
	fullArgs := append([]string{"-L", socket}, args...)
	cmd := exec.CommandContext(ctx, "tmux", fullArgs...)
	cmd.Env = append(os.Environ(), "TMUX_TMPDIR="+SocketDir())
	return cmd
}

// CommandArgs returns the arguments needed to run a tmux command
// with the default Claudio socket. Use this when you need to build the
// command string differently (e.g., for display purposes).
func CommandArgs(args ...string) []string {
	return CommandArgsWithSocket(SocketName, args...)
}

// CommandArgsWithSocket returns tmux arguments with a custom socket name.
func CommandArgsWithSocket(socket string, args ...string) []string {
	return append([]string{"-L", socket}, args...)
}

// BaseArgs returns just the socket arguments [-L, claudio].
// Use this when you need to prepend socket args to existing argument slices.
func BaseArgs() []string {
	return BaseArgsWithSocket(SocketName)
}

// BaseArgsWithSocket returns socket arguments for a custom socket name.
func BaseArgsWithSocket(socket string) []string {
	return []string{"-L", socket}
}

// SocketDir returns the directory where Claudio tmux sockets are stored.
// Uses ~/.claudio/sockets/ instead of /tmp/tmux-{uid}/ for stability
// (macOS periodically cleans /tmp which can kill active tmux servers).
// Falls back to os.TempDir()/claudio-sockets/ if HOME cannot be determined.
func SocketDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "claudio-sockets")
	}
	return filepath.Join(home, ".claudio", "sockets")
}

// EnsureSocketDir creates the socket directory if it doesn't exist.
func EnsureSocketDir() error {
	return os.MkdirAll(SocketDir(), 0700)
}

// InstanceSocketName returns the socket name for a specific instance.
// Socket names follow the format "claudio-{instanceID}".
func InstanceSocketName(instanceID string) string {
	return SocketPrefix + "-" + instanceID
}

// ListClaudioSockets returns all tmux sockets that belong to Claudio instances.
// It searches both the new stable socket directory (~/.claudio/sockets/) and
// the legacy tmux directory (/tmp/tmux-{uid}/) for backward compatibility.
func ListClaudioSockets() ([]string, error) {
	seen := make(map[string]struct{})
	var sockets []string

	// Search the primary (stable) socket directory
	collectSockets(SocketDir(), seen, &sockets)

	// Search the legacy tmux socket directory for backward compatibility
	if legacyDir, err := legacySocketDir(); err == nil {
		collectSockets(legacyDir, seen, &sockets)
	}

	return sockets, nil
}

// collectSockets searches a directory for Claudio sockets and appends unique names.
func collectSockets(dir string, seen map[string]struct{}, sockets *[]string) {
	pattern := filepath.Join(dir, SocketPrefix+"-*")
	matches, _ := filepath.Glob(pattern)

	// Also include the default socket if it exists
	defaultSocket := filepath.Join(dir, SocketName)
	if _, err := os.Stat(defaultSocket); err == nil {
		matches = append(matches, defaultSocket)
	}

	for _, match := range matches {
		name := filepath.Base(match)
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			*sockets = append(*sockets, name)
		}
	}
}

// legacySocketDir returns the legacy tmux socket directory (/tmp/tmux-{uid}/).
// Used for backward compatibility when searching for existing sockets.
func legacySocketDir() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join("/tmp", "tmux-"+u.Uid), nil
}

// IsInstanceSocket returns true if the socket name is an instance-specific socket.
func IsInstanceSocket(socket string) bool {
	return strings.HasPrefix(socket, SocketPrefix+"-") && socket != SocketName
}

// ExtractInstanceID extracts the instance ID from an instance socket name.
// Returns empty string if the socket is not an instance socket.
func ExtractInstanceID(socket string) string {
	prefix := SocketPrefix + "-"
	if id, found := strings.CutPrefix(socket, prefix); found {
		return id
	}
	return ""
}

// MapKeyToTmux converts Bubble Tea key names to tmux key names.
// Bubble Tea uses lowercase names like "left", "backspace" while
// tmux expects capitalized names like "Left", "BSpace".
func MapKeyToTmux(key string) string {
	switch key {
	case "up":
		return "Up"
	case "down":
		return "Down"
	case "left":
		return "Left"
	case "right":
		return "Right"
	case "home":
		return "Home"
	case "end":
		return "End"
	case "backspace":
		return "BSpace"
	case "delete":
		return "DC"
	case "insert":
		return "IC"
	case "pgup":
		return "PageUp"
	case "pgdown":
		return "PageDown"
	case "tab":
		return "Tab"
	case "enter":
		return "Enter"
	case "esc", "escape":
		return "Escape"
	default:
		return key
	}
}
