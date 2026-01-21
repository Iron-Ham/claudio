// Package terminal provides a persistent shell session for the TUI terminal pane.
package terminal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/tmux"
)

// CommandRunner abstracts tmux command execution for testability.
// In production, this uses real tmux commands. In tests, it can be mocked
// to verify the correct command sequences are executed.
type CommandRunner interface {
	// Run executes a tmux command and returns any error.
	Run(socketName string, args ...string) error
	// Output executes a tmux command and returns its output.
	Output(socketName string, args ...string) ([]byte, error)
	// OutputWithContext executes a tmux command with context support for cancellation/timeout.
	OutputWithContext(ctx context.Context, socketName string, args ...string) ([]byte, error)
	// CommandWithEnv returns an exec.Cmd for commands that need environment customization.
	CommandWithEnv(socketName string, args ...string) *exec.Cmd
}

// defaultCommandRunner is the production implementation that executes real tmux commands.
type defaultCommandRunner struct{}

func (r *defaultCommandRunner) Run(socketName string, args ...string) error {
	return tmux.CommandWithSocket(socketName, args...).Run()
}

func (r *defaultCommandRunner) Output(socketName string, args ...string) ([]byte, error) {
	return tmux.CommandWithSocket(socketName, args...).Output()
}

func (r *defaultCommandRunner) OutputWithContext(ctx context.Context, socketName string, args ...string) ([]byte, error) {
	return tmux.CommandContextWithSocket(ctx, socketName, args...).Output()
}

func (r *defaultCommandRunner) CommandWithEnv(socketName string, args ...string) *exec.Cmd {
	return tmux.CommandWithSocket(socketName, args...)
}

// Common errors for terminal process management.
var (
	ErrAlreadyRunning   = errors.New("terminal process is already running")
	ErrNotRunning       = errors.New("terminal process is not running")
	ErrCaptureTimeout   = errors.New("terminal output capture timed out")
	ErrCaptureKilled    = errors.New("terminal output capture was killed")
	ErrCaptureCancelled = errors.New("terminal output capture was cancelled")
)

// captureTimeout is the maximum time to wait for a tmux capture-pane command.
// This prevents the TUI from freezing if tmux becomes unresponsive.
const captureTimeout = 500 * time.Millisecond

// Process manages a persistent shell session in a tmux session for the terminal pane.
// Unlike the instance TmuxProcess, this runs a plain shell (no Claude command).
type Process struct {
	sessionName   string // tmux session name
	socketName    string // tmux socket for crash isolation
	invocationDir string // Directory where Claudio was invoked (never changes)
	currentDir    string // Current working directory
	width         int
	height        int
	cmdRunner     CommandRunner // abstraction for tmux command execution

	mu      sync.RWMutex
	running bool
}

// NewProcess creates a new terminal process manager.
// sessionID should be the Claudio session ID to ensure unique tmux session names.
// invocationDir is the directory where Claudio was launched.
// Uses the default "claudio" socket for the terminal pane, keeping it separate
// from per-instance sockets to isolate terminal from instance crashes.
func NewProcess(sessionID, invocationDir string, width, height int) *Process {
	return &Process{
		sessionName:   fmt.Sprintf("claudio-term-%s", sessionID),
		socketName:    tmux.SocketName, // Use shared socket for terminal pane
		invocationDir: invocationDir,
		currentDir:    invocationDir,
		width:         width,
		height:        height,
		cmdRunner:     &defaultCommandRunner{},
	}
}

// NewProcessWithSocket creates a new terminal process manager with a specific socket.
// This allows explicit control over socket isolation.
func NewProcessWithSocket(sessionID, socketName, invocationDir string, width, height int) *Process {
	return &Process{
		sessionName:   fmt.Sprintf("claudio-term-%s", sessionID),
		socketName:    socketName,
		invocationDir: invocationDir,
		currentDir:    invocationDir,
		width:         width,
		height:        height,
		cmdRunner:     &defaultCommandRunner{},
	}
}

// NewProcessWithRunner creates a new terminal process manager with a custom command runner.
// This is primarily used for testing to inject mock command execution.
func NewProcessWithRunner(sessionID, socketName, invocationDir string, width, height int, runner CommandRunner) *Process {
	return &Process{
		sessionName:   fmt.Sprintf("claudio-term-%s", sessionID),
		socketName:    socketName,
		invocationDir: invocationDir,
		currentDir:    invocationDir,
		width:         width,
		height:        height,
		cmdRunner:     runner,
	}
}

// Start launches the terminal shell in a tmux session.
func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.startLocked()
}

// startLocked is the internal start implementation that assumes the lock is already held.
func (p *Process) startLocked() error {
	if p.running {
		return ErrAlreadyRunning
	}

	// Kill any existing session with this name (cleanup from previous run)
	if err := p.cmdRunner.Run(p.socketName, "kill-session", "-t", p.sessionName); err != nil {
		if !isSessionNotFoundError(err) {
			log.Printf("WARNING: failed to cleanup existing terminal tmux session %s: %v", p.sessionName, err)
		}
	}

	// Determine dimensions
	width := p.width
	if width == 0 {
		width = 200
	}
	height := p.height
	if height == 0 {
		height = 10
	}

	// Set history-limit BEFORE creating session so the new pane inherits it.
	// tmux's history-limit only affects newly created panes, not existing ones.
	if err := p.cmdRunner.Run(p.socketName, "set-option", "-g", "history-limit", "50000"); err != nil {
		log.Printf("WARNING: failed to set global history-limit for tmux: %v", err)
	}

	// Create a new detached tmux session with proper environment setup.
	// We set TERM=xterm-256color in the environment so the shell inherits it directly,
	// which avoids modifying global tmux state that could affect other sessions.
	createCmd := p.cmdRunner.CommandWithEnv(p.socketName,
		"new-session",
		"-d",
		"-s", p.sessionName,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
		"-c", p.currentDir, // Start in the current directory
	)
	createCmd.Dir = p.currentDir
	createCmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create terminal tmux session: %w", err)
	}

	// Set default-terminal per-session (not globally) for any new panes/windows in this session.
	// This ensures consistent terminal emulation without affecting other tmux sessions.
	if err := p.cmdRunner.Run(p.socketName, "set-option", "-t", p.sessionName, "default-terminal", "xterm-256color"); err != nil {
		log.Printf("WARNING: failed to set default-terminal for terminal tmux session %s: %v", p.sessionName, err)
	}

	p.running = true

	return nil
}

// Stop terminates the terminal session.
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	// Kill the tmux session
	if err := p.cmdRunner.Run(p.socketName, "kill-session", "-t", p.sessionName); err != nil {
		if !isSessionNotFoundError(err) {
			log.Printf("WARNING: unexpected error killing terminal tmux session %s: %v", p.sessionName, err)
		}
	}

	p.running = false
	return nil
}

// IsRunning returns whether the terminal process is currently running.
func (p *Process) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// SessionExists checks if the tmux session still exists.
func (p *Process) SessionExists() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sessionExists()
}

func (p *Process) sessionExists() bool {
	return p.cmdRunner.Run(p.socketName, "has-session", "-t", p.sessionName) == nil
}

// EnsureRunning starts the process if it's not running, or restarts if the session died.
func (p *Process) EnsureRunning() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If we think it's running, verify the session exists
	if p.running && !p.sessionExists() {
		p.running = false
	}

	if p.running {
		return nil
	}

	return p.startLocked()
}

// ChangeDirectory changes the terminal's working directory.
func (p *Process) ChangeDirectory(dir string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		// Just update the stored directory; it will be used when we start
		p.currentDir = dir
		return nil
	}

	// Send cd command to the terminal
	cdCmd := fmt.Sprintf("cd %q", dir)
	if err := p.cmdRunner.Run(p.socketName, "send-keys", "-t", p.sessionName, cdCmd, "Enter"); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	p.currentDir = dir
	return nil
}

// CurrentDir returns the current working directory of the terminal.
func (p *Process) CurrentDir() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentDir
}

// InvocationDir returns the directory where Claudio was invoked.
func (p *Process) InvocationDir() string {
	return p.invocationDir
}

// SendKey sends a special key (like "Enter", "C-c", "Up") to the terminal.
func (p *Process) SendKey(key string) error {
	p.mu.RLock()
	running := p.running
	sessionName := p.sessionName
	p.mu.RUnlock()

	if !running {
		return ErrNotRunning
	}

	if err := p.cmdRunner.Run(p.socketName, "send-keys", "-t", sessionName, key); err != nil {
		return fmt.Errorf("failed to send key to terminal: %w", err)
	}
	return nil
}

// SendLiteral sends literal text to the terminal (characters sent as-is).
func (p *Process) SendLiteral(text string) error {
	p.mu.RLock()
	running := p.running
	sessionName := p.sessionName
	p.mu.RUnlock()

	if !running {
		return ErrNotRunning
	}

	if err := p.cmdRunner.Run(p.socketName, "send-keys", "-t", sessionName, "-l", text); err != nil {
		return fmt.Errorf("failed to send literal to terminal: %w", err)
	}
	return nil
}

// SendPaste sends pasted text with bracketed paste sequences.
func (p *Process) SendPaste(text string) error {
	p.mu.RLock()
	running := p.running
	sessionName := p.sessionName
	p.mu.RUnlock()

	if !running {
		return ErrNotRunning
	}

	// Send bracketed paste start sequence
	if err := p.cmdRunner.Run(p.socketName, "send-keys", "-t", sessionName, "-l", "\x1b[200~"); err != nil {
		return fmt.Errorf("failed to send paste start: %w", err)
	}

	// Send the pasted content
	if err := p.cmdRunner.Run(p.socketName, "send-keys", "-t", sessionName, "-l", text); err != nil {
		return fmt.Errorf("failed to send paste content: %w", err)
	}

	// Send bracketed paste end sequence
	if err := p.cmdRunner.Run(p.socketName, "send-keys", "-t", sessionName, "-l", "\x1b[201~"); err != nil {
		return fmt.Errorf("failed to send paste end: %w", err)
	}

	return nil
}

// CaptureOutput captures the current visible content of the terminal pane.
// Uses a timeout to prevent blocking the TUI if tmux is unresponsive.
func (p *Process) CaptureOutput() (string, error) {
	p.mu.RLock()
	running := p.running
	sessionName := p.sessionName
	p.mu.RUnlock()

	if !running {
		return "", ErrNotRunning
	}

	// Create a context with timeout to prevent blocking the TUI event loop
	// if tmux becomes unresponsive (e.g., due to socket issues, session not found)
	ctx, cancel := context.WithTimeout(context.Background(), captureTimeout)
	defer cancel()

	// Capture visible pane content with escape sequences preserved (-e flag)
	// The -e flag preserves ANSI color codes and other escape sequences
	output, err := p.cmdRunner.OutputWithContext(ctx, p.socketName, "capture-pane", "-t", sessionName, "-p", "-e")
	if err != nil {
		// Distinguish between timeout and other errors for better diagnostics
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", ErrCaptureTimeout
		}
		// Coverage: ErrCaptureCancelled is defensive code. Currently unreachable because
		// the context is created from context.Background() (which cannot be cancelled)
		// and context.WithTimeout only triggers context.DeadlineExceeded on expiry.
		// Kept for future-proofing if this function is refactored to accept an external context.
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", ErrCaptureCancelled
		}
		// Check if the process was killed (happens when context times out)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == -1 {
			return "", ErrCaptureKilled
		}
		return "", fmt.Errorf("failed to capture terminal output: %w", err)
	}

	return string(output), nil
}

// CaptureOutputWithHistory captures terminal content including scrollback history.
// Uses a timeout to prevent blocking the TUI if tmux is unresponsive.
func (p *Process) CaptureOutputWithHistory(lines int) (string, error) {
	p.mu.RLock()
	running := p.running
	sessionName := p.sessionName
	p.mu.RUnlock()

	if !running {
		return "", ErrNotRunning
	}

	// Create a context with timeout to prevent blocking the TUI event loop
	ctx, cancel := context.WithTimeout(context.Background(), captureTimeout)
	defer cancel()

	// Capture with history (-S for start line, negative means history)
	// The -e flag preserves ANSI color codes and other escape sequences
	output, err := p.cmdRunner.OutputWithContext(ctx, p.socketName, "capture-pane", "-t", sessionName, "-p", "-e", "-S", fmt.Sprintf("-%d", lines))
	if err != nil {
		// Distinguish between timeout and other errors for better diagnostics
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", ErrCaptureTimeout
		}
		// Coverage: ErrCaptureCancelled is defensive code. Currently unreachable because
		// the context is created from context.Background() (which cannot be cancelled)
		// and context.WithTimeout only triggers context.DeadlineExceeded on expiry.
		// Kept for future-proofing if this function is refactored to accept an external context.
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", ErrCaptureCancelled
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == -1 {
			return "", ErrCaptureKilled
		}
		return "", fmt.Errorf("failed to capture terminal output with history: %w", err)
	}

	return string(output), nil
}

// Resize adjusts the terminal dimensions.
func (p *Process) Resize(width, height int) error {
	p.mu.Lock()
	p.width = width
	p.height = height
	running := p.running
	sessionName := p.sessionName
	p.mu.Unlock()

	if !running {
		return nil
	}

	if err := p.cmdRunner.Run(p.socketName, "resize-window", "-t", sessionName, "-x", fmt.Sprintf("%d", width), "-y", fmt.Sprintf("%d", height)); err != nil {
		return fmt.Errorf("failed to resize terminal: %w", err)
	}

	return nil
}

// AttachCommand returns the command to attach to this terminal's tmux session.
func (p *Process) AttachCommand() string {
	return fmt.Sprintf("tmux -L %s attach -t %s", p.socketName, p.sessionName)
}

// SocketName returns the tmux socket name used for this terminal.
func (p *Process) SocketName() string {
	return p.socketName
}

// SessionName returns the tmux session name.
func (p *Process) SessionName() string {
	return p.sessionName
}

// WaitForPrompt waits for the shell prompt to appear (basic readiness check).
func (p *Process) WaitForPrompt(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output, err := p.CaptureOutput()
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		// Check for common prompt indicators
		if strings.Contains(output, "$") || strings.Contains(output, "%") || strings.Contains(output, ">") {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for shell prompt")
}

// isSessionNotFoundError checks if the error indicates a tmux session was not found.
func isSessionNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "session not found") ||
		strings.Contains(errStr, "no server running") ||
		strings.Contains(errStr, "can't find session")
}
