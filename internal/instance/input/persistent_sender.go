// Package input provides input handling for Claude Code instances.
package input

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// PersistentTmuxSender maintains a persistent control-mode connection to tmux.
// It implements the TmuxSender interface but uses a persistent pipe instead of
// spawning new subprocesses for each SendKeys call.
//
// This dramatically reduces latency for input operations by avoiding the
// ~50-200ms subprocess spawn overhead per character/batch.
type PersistentTmuxSender struct {
	mu          sync.Mutex
	sessionName string

	// Process and pipes
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// State
	connected bool

	// Fallback sender for error recovery
	fallback TmuxSender
}

// PersistentOption configures the PersistentTmuxSender.
type PersistentOption func(*PersistentTmuxSender)

// WithFallbackSender sets a custom fallback sender for error recovery.
// If not set, DefaultTmuxSender is used as fallback.
func WithFallbackSender(sender TmuxSender) PersistentOption {
	return func(p *PersistentTmuxSender) {
		p.fallback = sender
	}
}

// NewPersistentTmuxSender creates a new persistent sender for the given session.
// The connection is established lazily on first use.
func NewPersistentTmuxSender(sessionName string, opts ...PersistentOption) *PersistentTmuxSender {
	p := &PersistentTmuxSender{
		sessionName: sessionName,
		fallback:    &DefaultTmuxSender{},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// SendKeys implements TmuxSender interface.
// It writes send-keys commands to the persistent tmux control mode connection.
// If the connection fails, it falls back to subprocess spawning.
func (p *PersistentTmuxSender) SendKeys(sessionName string, keys string, literal bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If session doesn't match, use fallback (persistent sender is session-specific)
	if sessionName != p.sessionName {
		return p.fallback.SendKeys(sessionName, keys, literal)
	}

	// Ensure connected
	if !p.connected {
		if err := p.connectLocked(); err != nil {
			// Fall back to subprocess on connection failure
			return p.fallback.SendKeys(sessionName, keys, literal)
		}
	}

	// Build the send-keys command for tmux control mode
	var cmd string
	if literal {
		// Escape the keys for tmux control mode protocol
		escaped := escapeForControlMode(keys)
		cmd = fmt.Sprintf("send-keys -t %s -l %s\n", p.sessionName, escaped)
	} else {
		cmd = fmt.Sprintf("send-keys -t %s %s\n", p.sessionName, keys)
	}

	// Write to stdin
	if _, err := p.stdin.Write([]byte(cmd)); err != nil {
		// Connection lost, mark as disconnected and fall back
		p.disconnectLocked()
		return p.fallback.SendKeys(sessionName, keys, literal)
	}

	return nil
}

// Close shuts down the persistent connection and releases resources.
func (p *PersistentTmuxSender) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.disconnectLocked()
	return nil
}

// Connected returns whether the persistent connection is currently established.
func (p *PersistentTmuxSender) Connected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.connected
}

// connectLocked establishes the control-mode connection to tmux.
// Caller must hold p.mu.
// Coverage: Success path requires a real tmux session; failure paths are tested
// via nonexistent session names which trigger the verification error path.
func (p *PersistentTmuxSender) connectLocked() error {
	if p.connected {
		return nil
	}

	// Start tmux in control mode, attached to the session
	// Control mode (-C) keeps stdin open for commands and writes responses to stdout
	cmd := exec.Command("tmux", "-C", "attach-session", "-t", p.sessionName)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return fmt.Errorf("failed to start tmux control mode: %w", err)
	}

	// Verify the connection is actually working by reading tmux's response
	// On success: %begin, %end, %session-changed
	// On failure: %begin, error message, %error, %exit
	verified := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(stdout)
		// Read up to 5 lines to detect success or failure
		for i := 0; i < 5; i++ {
			line, err := reader.ReadString('\n')
			if err != nil {
				verified <- fmt.Errorf("failed to read tmux response: %w", err)
				return
			}
			line = strings.TrimSpace(line)
			// Check for error indicator
			if strings.HasPrefix(line, "%error") || strings.HasPrefix(line, "%exit") {
				verified <- fmt.Errorf("tmux session not found")
				return
			}
			// Check for success indicator
			if strings.HasPrefix(line, "%session-changed") {
				verified <- nil
				return
			}
		}
		// If we get here without %session-changed or %error, assume success
		verified <- nil
	}()

	// Wait for verification with timeout
	select {
	case err := <-verified:
		if err != nil {
			_ = stdin.Close()
			_ = stdout.Close()
			_ = stderr.Close()
			_ = cmd.Wait()
			return err
		}
	case <-time.After(1 * time.Second):
		// Timeout - check if process exited
		if cmd.ProcessState != nil {
			_ = stdin.Close()
			_ = stdout.Close()
			_ = stderr.Close()
			return fmt.Errorf("tmux control mode exited unexpectedly")
		}
		// Process still running but no response - likely okay, proceed
	}

	p.cmd = cmd
	p.stdin = stdin
	p.stdout = stdout
	p.stderr = stderr
	p.connected = true

	// Start a goroutine to drain remaining stdout/stderr to prevent blocking
	go p.drainOutput()

	return nil
}

// disconnectLocked closes the control-mode connection.
// Caller must hold p.mu.
func (p *PersistentTmuxSender) disconnectLocked() {
	if !p.connected {
		return
	}

	p.connected = false

	if p.stdin != nil {
		_ = p.stdin.Close()
		p.stdin = nil
	}
	if p.stdout != nil {
		_ = p.stdout.Close()
		p.stdout = nil
	}
	if p.stderr != nil {
		_ = p.stderr.Close()
		p.stderr = nil
	}
	if p.cmd != nil && p.cmd.Process != nil {
		// Kill the process (sends SIGKILL) and wait for cleanup
		_ = p.cmd.Process.Kill()
		_ = p.cmd.Wait()
		p.cmd = nil
	}
}

// drainOutput reads from stdout and stderr to prevent the pipes from blocking.
// This runs in a goroutine and exits when the pipes are closed.
// Coverage: This method runs as a background goroutine reading from pipe file
// descriptors. Testing requires a real tmux control mode connection.
func (p *PersistentTmuxSender) drainOutput() {
	// We need local copies since we can't hold the lock while reading
	p.mu.Lock()
	stdout := p.stdout
	stderr := p.stderr
	p.mu.Unlock()

	// Drain stdout in one goroutine - each goroutine needs its own buffer
	// to avoid data races
	go func() {
		if stdout != nil {
			buf := make([]byte, 4096)
			for {
				_, err := stdout.Read(buf)
				if err != nil {
					return
				}
			}
		}
	}()

	// Drain stderr with its own buffer
	if stderr != nil {
		buf := make([]byte, 4096)
		for {
			_, err := stderr.Read(buf)
			if err != nil {
				return
			}
		}
	}
}

// escapeForControlMode escapes a string for use in tmux control mode commands.
// In control mode, certain characters need escaping to be sent literally.
func escapeForControlMode(s string) string {
	// In tmux control mode, the argument to send-keys -l needs to be quoted
	// if it contains special characters. We use single quotes and escape
	// any existing single quotes by ending the quote, adding escaped quote,
	// and restarting the quote: ' -> '\''
	if strings.ContainsAny(s, " \t\n\r'\"\\") {
		escaped := strings.ReplaceAll(s, "'", "'\\''")
		return "'" + escaped + "'"
	}
	return s
}
