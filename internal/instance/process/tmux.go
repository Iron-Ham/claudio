package process

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TmuxProcess implements the Process interface using tmux sessions.
// It manages the lifecycle of a Claude Code process running in a tmux session.
type TmuxProcess struct {
	config      Config
	sessionName string
	mu          sync.RWMutex

	// State
	running   bool
	started   bool // true if Start was ever called successfully
	waitDone  chan struct{}
	waitErr   error
	waitOnce  sync.Once
	doneChan  chan struct{}
	checkTick *time.Ticker
}

// NewTmuxProcess creates a new tmux-based process manager.
// The sessionName will be auto-generated if not provided in config.
func NewTmuxProcess(config Config) *TmuxProcess {
	sessionName := config.TmuxSession
	if sessionName == "" {
		sessionName = fmt.Sprintf("claudio-%d", time.Now().UnixNano())
	}

	return &TmuxProcess{
		config:      config,
		sessionName: sessionName,
	}
}

// Start launches the Claude Code process in a tmux session.
func (p *TmuxProcess) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return ErrAlreadyRunning
	}

	if err := p.config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Kill any existing session with this name (cleanup from previous run)
	if err := exec.Command("tmux", "kill-session", "-t", p.sessionName).Run(); err != nil {
		// Only log unexpected errors (not "session not found" which is expected)
		if !isSessionNotFoundError(err) {
			log.Printf("WARNING: failed to cleanup existing tmux session %s: %v", p.sessionName, err)
		}
	}

	// Determine terminal dimensions
	width := p.config.Width
	if width == 0 {
		width = 200
	}
	height := p.config.Height
	if height == 0 {
		height = 30
	}

	// Create a new detached tmux session
	createCmd := exec.CommandContext(ctx, "tmux",
		"new-session",
		"-d",
		"-s", p.sessionName,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
	)
	createCmd.Dir = p.config.WorkDir
	createCmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Set up tmux session options (log failures but don't abort - session will still work)
	if err := exec.Command("tmux", "set-option", "-t", p.sessionName, "history-limit", "10000").Run(); err != nil {
		log.Printf("WARNING: failed to set history-limit for tmux session %s: %v", p.sessionName, err)
	}
	if err := exec.Command("tmux", "set-option", "-t", p.sessionName, "default-terminal", "xterm-256color").Run(); err != nil {
		log.Printf("WARNING: failed to set default-terminal for tmux session %s: %v", p.sessionName, err)
	}
	if err := exec.Command("tmux", "set-option", "-t", p.sessionName, "-w", "monitor-bell", "on").Run(); err != nil {
		log.Printf("WARNING: failed to set monitor-bell for tmux session %s: %v", p.sessionName, err)
	}

	// Write prompt to file to avoid shell escaping issues
	promptFile := filepath.Join(p.config.WorkDir, ".claude-prompt")
	if err := os.WriteFile(promptFile, []byte(p.config.InitialPrompt), 0600); err != nil {
		_ = exec.Command("tmux", "kill-session", "-t", p.sessionName).Run()
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	// Send the claude command to the tmux session
	claudeCmd := fmt.Sprintf("claude --dangerously-skip-permissions \"$(cat %q)\" && rm %q", promptFile, promptFile)
	sendCmd := exec.CommandContext(ctx, "tmux",
		"send-keys",
		"-t", p.sessionName,
		claudeCmd,
		"Enter",
	)
	if err := sendCmd.Run(); err != nil {
		_ = exec.Command("tmux", "kill-session", "-t", p.sessionName).Run()
		_ = os.Remove(promptFile)
		return fmt.Errorf("failed to start claude in tmux session: %w", err)
	}

	p.running = true
	p.started = true
	p.doneChan = make(chan struct{})
	p.waitDone = make(chan struct{})
	p.waitOnce = sync.Once{}

	// Start session monitoring goroutine
	p.checkTick = time.NewTicker(100 * time.Millisecond)
	go p.monitorSession()

	return nil
}

// monitorSession periodically checks if the tmux session is still alive.
func (p *TmuxProcess) monitorSession() {
	for {
		select {
		case <-p.doneChan:
			return
		case <-p.checkTick.C:
			p.mu.RLock()
			sessionName := p.sessionName
			running := p.running
			p.mu.RUnlock()

			if !running {
				continue
			}

			// Check if session still exists
			checkCmd := exec.Command("tmux", "has-session", "-t", sessionName)
			if checkCmd.Run() != nil {
				// Session ended
				p.mu.Lock()
				p.running = false
				p.mu.Unlock()

				// Signal completion
				p.waitOnce.Do(func() {
					close(p.waitDone)
				})
				return
			}
		}
	}
}

// Stop terminates the process gracefully.
func (p *TmuxProcess) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	// Signal monitor to stop
	select {
	case <-p.doneChan:
	default:
		close(p.doneChan)
	}

	// Stop the ticker
	if p.checkTick != nil {
		p.checkTick.Stop()
	}

	// Send Ctrl+C to gracefully stop Claude first
	if err := exec.Command("tmux", "send-keys", "-t", p.sessionName, "C-c").Run(); err != nil {
		log.Printf("WARNING: failed to send Ctrl+C to tmux session %s, proceeding to force kill: %v", p.sessionName, err)
	}
	time.Sleep(500 * time.Millisecond)

	// Kill the tmux session
	if err := exec.Command("tmux", "kill-session", "-t", p.sessionName).Run(); err != nil {
		// Only log unexpected errors (not "session not found" which is expected if process exited)
		if !isSessionNotFoundError(err) {
			log.Printf("WARNING: unexpected error killing tmux session %s: %v", p.sessionName, err)
		}
	}

	p.running = false

	// Signal completion
	p.waitOnce.Do(func() {
		close(p.waitDone)
	})

	return nil
}

// IsRunning returns whether the process is currently running.
func (p *TmuxProcess) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Wait blocks until the process exits.
func (p *TmuxProcess) Wait() error {
	p.mu.RLock()
	started := p.started
	waitDone := p.waitDone
	p.mu.RUnlock()

	if !started {
		return ErrNotRunning
	}

	<-waitDone

	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.waitErr
}

// SendInput sends input to the running process.
func (p *TmuxProcess) SendInput(input string) error {
	p.mu.RLock()
	running := p.running
	sessionName := p.sessionName
	p.mu.RUnlock()

	if !running {
		return ErrNotRunning
	}

	// Process input character by character, handling special keys
	for _, r := range input {
		var key string
		switch r {
		case '\r', '\n':
			key = "Enter"
		case '\t':
			key = "Tab"
		case '\x7f', '\b': // backspace
			key = "BSpace"
		case '\x1b': // escape
			key = "Escape"
		case ' ':
			key = "Space"
		default:
			if r < 32 {
				// Control character: Ctrl+letter
				key = fmt.Sprintf("C-%c", r+'a'-1)
			} else {
				// Regular character - send literally
				key = string(r)
			}
		}

		if err := exec.Command("tmux", "send-keys", "-t", sessionName, "-l", key).Run(); err != nil {
			return fmt.Errorf("failed to send key: %w", err)
		}
	}

	return nil
}

// SessionName returns the tmux session name.
func (p *TmuxProcess) SessionName() string {
	return p.sessionName
}

// Resize implements Resizable interface.
func (p *TmuxProcess) Resize(width, height int) error {
	p.mu.RLock()
	running := p.running
	sessionName := p.sessionName
	p.mu.RUnlock()

	if !running {
		return nil
	}

	resizeCmd := exec.Command("tmux",
		"resize-window",
		"-t", sessionName,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
	)
	if err := resizeCmd.Run(); err != nil {
		return fmt.Errorf("failed to resize tmux session: %w", err)
	}

	return nil
}

// Reconnect implements Reconnectable interface.
func (p *TmuxProcess) Reconnect() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return ErrAlreadyRunning
	}

	if !p.sessionExists() {
		return ErrSessionNotFound
	}

	// Ensure monitor-bell is enabled
	if err := exec.Command("tmux", "set-option", "-t", p.sessionName, "-w", "monitor-bell", "on").Run(); err != nil {
		log.Printf("WARNING: failed to set monitor-bell on reconnect for tmux session %s: %v", p.sessionName, err)
	}

	p.running = true
	p.doneChan = make(chan struct{})
	p.waitDone = make(chan struct{})
	p.waitOnce = sync.Once{}

	// Start session monitoring
	p.checkTick = time.NewTicker(100 * time.Millisecond)
	go p.monitorSession()

	return nil
}

// SessionExists implements Reconnectable interface.
func (p *TmuxProcess) SessionExists() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sessionExists()
}

func (p *TmuxProcess) sessionExists() bool {
	cmd := exec.Command("tmux", "has-session", "-t", p.sessionName)
	return cmd.Run() == nil
}

// AttachCommand returns the command to attach to this process's tmux session.
func (p *TmuxProcess) AttachCommand() string {
	return fmt.Sprintf("tmux attach -t %s", p.sessionName)
}

// PID returns the process ID of the shell in the tmux session.
func (p *TmuxProcess) PID() int {
	p.mu.RLock()
	running := p.running
	sessionName := p.sessionName
	p.mu.RUnlock()

	if !running {
		return 0
	}

	cmd := exec.Command("tmux", "display-message", "-t", sessionName, "-p", "#{pane_pid}")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	var pid int
	_, _ = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &pid)
	return pid
}

// isSessionNotFoundError checks if the error indicates a tmux session was not found.
// This is expected when cleaning up sessions that may not exist.
func isSessionNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "session not found") ||
		strings.Contains(errStr, "no server running") ||
		strings.Contains(errStr, "can't find session")
}

// Verify interface implementations at compile time.
var (
	_ Process       = (*TmuxProcess)(nil)
	_ Resizable     = (*TmuxProcess)(nil)
	_ Reconnectable = (*TmuxProcess)(nil)
)
