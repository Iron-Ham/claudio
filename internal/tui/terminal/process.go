// Package terminal provides a persistent shell session for the TUI terminal pane.
package terminal

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/tmux"
)

// Common errors for terminal process management.
var (
	ErrAlreadyRunning = errors.New("terminal process is already running")
	ErrNotRunning     = errors.New("terminal process is not running")
)

// Process manages a persistent shell session in a tmux session for the terminal pane.
// Unlike the instance TmuxProcess, this runs a plain shell (no Claude command).
type Process struct {
	sessionName   string // tmux session name
	socketName    string // tmux socket for crash isolation
	invocationDir string // Directory where Claudio was invoked (never changes)
	currentDir    string // Current working directory
	width         int
	height        int

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
	if err := tmux.CommandWithSocket(p.socketName, "kill-session", "-t", p.sessionName).Run(); err != nil {
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
	// Use 50000 lines for generous scrollback in the terminal pane.
	if err := tmux.CommandWithSocket(p.socketName, "set-option", "-g", "history-limit", "50000").Run(); err != nil {
		log.Printf("WARNING: failed to set global history-limit for tmux: %v", err)
	}

	// Create a new detached tmux session
	createCmd := tmux.CommandWithSocket(p.socketName,
		"new-session",
		"-d",
		"-s", p.sessionName,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
		"-c", p.currentDir, // Start in the current directory
	)
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create terminal tmux session: %w", err)
	}

	// Set up additional tmux session options
	if err := tmux.CommandWithSocket(p.socketName, "set-option", "-t", p.sessionName, "default-terminal", "xterm-256color").Run(); err != nil {
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
	if err := tmux.CommandWithSocket(p.socketName, "kill-session", "-t", p.sessionName).Run(); err != nil {
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
	cmd := tmux.CommandWithSocket(p.socketName, "has-session", "-t", p.sessionName)
	return cmd.Run() == nil
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
	if err := tmux.CommandWithSocket(p.socketName, "send-keys", "-t", p.sessionName, cdCmd, "Enter").Run(); err != nil {
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

	if err := tmux.CommandWithSocket(p.socketName, "send-keys", "-t", sessionName, key).Run(); err != nil {
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

	if err := tmux.CommandWithSocket(p.socketName, "send-keys", "-t", sessionName, "-l", text).Run(); err != nil {
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
	if err := tmux.CommandWithSocket(p.socketName, "send-keys", "-t", sessionName, "-l", "\x1b[200~").Run(); err != nil {
		return fmt.Errorf("failed to send paste start: %w", err)
	}

	// Send the pasted content
	if err := tmux.CommandWithSocket(p.socketName, "send-keys", "-t", sessionName, "-l", text).Run(); err != nil {
		return fmt.Errorf("failed to send paste content: %w", err)
	}

	// Send bracketed paste end sequence
	if err := tmux.CommandWithSocket(p.socketName, "send-keys", "-t", sessionName, "-l", "\x1b[201~").Run(); err != nil {
		return fmt.Errorf("failed to send paste end: %w", err)
	}

	return nil
}

// CaptureOutput captures the current visible content of the terminal pane.
func (p *Process) CaptureOutput() (string, error) {
	p.mu.RLock()
	running := p.running
	sessionName := p.sessionName
	p.mu.RUnlock()

	if !running {
		return "", ErrNotRunning
	}

	// Capture visible pane content with escape sequences preserved (-e flag)
	// The -e flag preserves ANSI color codes and other escape sequences
	cmd := tmux.CommandWithSocket(p.socketName, "capture-pane", "-t", sessionName, "-p", "-e")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to capture terminal output: %w", err)
	}

	return string(output), nil
}

// CaptureOutputWithHistory captures terminal content including scrollback history.
func (p *Process) CaptureOutputWithHistory(lines int) (string, error) {
	p.mu.RLock()
	running := p.running
	sessionName := p.sessionName
	p.mu.RUnlock()

	if !running {
		return "", ErrNotRunning
	}

	// Capture with history (-S for start line, negative means history)
	// The -e flag preserves ANSI color codes and other escape sequences
	cmd := tmux.CommandWithSocket(p.socketName, "capture-pane", "-t", sessionName, "-p", "-e", "-S", fmt.Sprintf("-%d", lines))
	output, err := cmd.Output()
	if err != nil {
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

	resizeCmd := tmux.CommandWithSocket(p.socketName,
		"resize-window",
		"-t", sessionName,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
	)
	if err := resizeCmd.Run(); err != nil {
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
