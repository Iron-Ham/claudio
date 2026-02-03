// Package input provides input handling for AI backend instances.
package input

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/tmux"
)

// writeTimeout is the maximum time to wait for a write to the tmux control mode
// connection before considering it stuck and attempting reconnection.
const writeTimeout = 500 * time.Millisecond

// errWriteTimeout is returned when a write operation times out.
var errWriteTimeout = errors.New("write timeout: tmux connection may be stuck")

// PersistentTmuxSender maintains a persistent control-mode connection to tmux.
// It implements the TmuxSender interface but uses a persistent pipe instead of
// spawning new subprocesses for each SendKeys call.
//
// This dramatically reduces latency for input operations by avoiding the
// ~50-200ms subprocess spawn overhead per character/batch.
type PersistentTmuxSender struct {
	mu          sync.Mutex
	sessionName string
	socketName  string // tmux socket for this connection

	// Process and pipes
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// State
	connected bool

	// Goroutine lifecycle management
	drainCancel  context.CancelFunc // Cancels drain goroutines
	drainWg      sync.WaitGroup     // Tracks drain goroutines
	activeWrites sync.WaitGroup     // Tracks in-flight write operations

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
// The socketName specifies which tmux socket to use for the control mode connection.
// The connection is established lazily on first use.
func NewPersistentTmuxSender(sessionName, socketName string, opts ...PersistentOption) *PersistentTmuxSender {
	p := &PersistentTmuxSender{
		sessionName: sessionName,
		socketName:  socketName,
		fallback:    NewDefaultTmuxSender(socketName),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// SendKeys implements TmuxSender interface.
// It writes send-keys commands to the persistent tmux control mode connection.
// If the connection fails or times out, it attempts to reconnect automatically.
// If reconnection fails, it falls back to subprocess spawning.
func (p *PersistentTmuxSender) SendKeys(sessionName string, keys string, literal bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If session doesn't match, use fallback (persistent sender is session-specific)
	if sessionName != p.sessionName {
		return p.fallback.SendKeys(sessionName, keys, literal)
	}

	// Ensure connected
	if !p.connected {
		if connErr := p.connectLocked(); connErr != nil {
			// Fall back to subprocess on connection failure
			log.Printf("WARNING: persistent tmux connection failed for session %s: %v, using subprocess fallback",
				p.sessionName, connErr)
			return p.fallback.SendKeys(sessionName, keys, literal)
		}
	}

	// Build the send-keys command for tmux control mode
	cmd := p.buildCommand(keys, literal)

	// Try to write with timeout
	if writeErr := p.writeWithTimeoutLocked([]byte(cmd)); writeErr != nil {
		// Connection is stuck or dead - disconnect and try to reconnect
		log.Printf("WARNING: tmux write failed for session %s: %v, attempting reconnect", p.sessionName, writeErr)
		p.disconnectLocked()

		// Attempt to reconnect
		if reconnErr := p.connectLocked(); reconnErr == nil {
			// Reconnected! Try sending again
			if retryErr := p.writeWithTimeoutLocked([]byte(cmd)); retryErr == nil {
				log.Printf("INFO: tmux reconnection successful for session %s", p.sessionName)
				return nil
			}
			// Still failing after reconnect, disconnect again
			log.Printf("WARNING: tmux write failed after reconnect for session %s, falling back to subprocess",
				p.sessionName)
			p.disconnectLocked()
		} else {
			log.Printf("WARNING: tmux reconnect failed for session %s: %v, falling back to subprocess",
				p.sessionName, reconnErr)
		}

		// Fall back to subprocess for this operation
		return p.fallback.SendKeys(sessionName, keys, literal)
	}

	return nil
}

// buildCommand constructs the tmux control mode command for send-keys.
func (p *PersistentTmuxSender) buildCommand(keys string, literal bool) string {
	if literal {
		escaped := escapeForControlMode(keys)
		return fmt.Sprintf("send-keys -t %s -l %s\n", p.sessionName, escaped)
	}
	return fmt.Sprintf("send-keys -t %s %s\n", p.sessionName, keys)
}

// writeWithTimeoutLocked writes data to stdin with a timeout.
// If the write doesn't complete within writeTimeout, it returns errWriteTimeout.
// Caller must hold p.mu. The timeout is implemented by closing the pipe if the
// write blocks too long, which unblocks the stuck write goroutine.
func (p *PersistentTmuxSender) writeWithTimeoutLocked(data []byte) error {
	if p.stdin == nil {
		return fmt.Errorf("stdin is nil")
	}

	// Capture stdin locally since we'll check it after timeout
	stdin := p.stdin

	done := make(chan error, 1)
	p.activeWrites.Add(1)
	go func() {
		defer p.activeWrites.Done()
		_, err := stdin.Write(data)
		// Use non-blocking send to prevent goroutine leak if timeout already fired.
		// If timeout fired, the caller will call disconnectLocked() which closes
		// stdin and unblocks any future writes.
		select {
		case done <- err:
		default:
			// Timeout already occurred, result is discarded
		}
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(writeTimeout):
		return errWriteTimeout
	}
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
	// Use the instance-specific socket for crash isolation
	cmd := tmux.CommandWithSocket(p.socketName, "-C", "attach-session", "-t", p.sessionName)

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
			// Kill the process before Wait() to prevent blocking indefinitely.
			// Without Kill(), Wait() can hang if the verification goroutine is
			// still reading from stdout when we closed the pipe.
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
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

	// Create cancellation context for drain goroutines
	ctx, cancel := context.WithCancel(context.Background())
	p.drainCancel = cancel

	// Start drain goroutines with proper lifecycle tracking
	p.drainWg.Add(2)
	go p.drainPipe(ctx, p.stdout)
	go p.drainPipe(ctx, p.stderr)

	return nil
}

// disconnectLocked closes the control-mode connection and waits for goroutines to exit.
// Caller must hold p.mu.
func (p *PersistentTmuxSender) disconnectLocked() {
	if !p.connected {
		return
	}

	p.connected = false

	// Cancel drain goroutines FIRST - this allows them to exit their loops
	if p.drainCancel != nil {
		p.drainCancel()
		p.drainCancel = nil
	}

	// Close pipes - this unblocks any Read/Write calls
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

	// Wait for drain goroutines to exit (with timeout to avoid blocking indefinitely)
	drainDone := make(chan struct{})
	go func() {
		p.drainWg.Wait()
		close(drainDone)
	}()
	select {
	case <-drainDone:
		// Drain goroutines exited cleanly
	case <-time.After(500 * time.Millisecond):
		log.Printf("WARNING: drain goroutines did not exit within timeout for session %s", p.sessionName)
	}

	// Wait for any active write operations (with timeout)
	writesDone := make(chan struct{})
	go func() {
		p.activeWrites.Wait()
		close(writesDone)
	}()
	select {
	case <-writesDone:
		// Active writes completed
	case <-time.After(100 * time.Millisecond):
		log.Printf("WARNING: active write operations did not complete within timeout for session %s", p.sessionName)
	}

	// Kill the process and wait for cleanup
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_ = p.cmd.Wait()
		p.cmd = nil
	}
}

// drainPipe reads from a pipe to prevent it from blocking.
// It exits when the context is cancelled or when the pipe is closed.
// Must be called after drainWg.Add(1) - this function calls drainWg.Done() on exit.
func (p *PersistentTmuxSender) drainPipe(ctx context.Context, pipe io.ReadCloser) {
	defer p.drainWg.Done()

	if pipe == nil {
		return
	}

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Read with a small buffer - this will block until data is available
			// or the pipe is closed. Note: context cancellation is only checked
			// between reads; the pipe.Close() call in disconnectLocked is what
			// actually unblocks this read when shutting down.
			_, err := pipe.Read(buf)
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
	//
	// Characters that must be quoted:
	// - space, tab, newline, carriage return: whitespace separates arguments
	// - ' " \: quote and escape characters
	// - ; : tmux command separator
	// - # : tmux format specifier prefix (e.g., #{window_name})
	// - ~ : tilde expansion in some contexts
	if strings.ContainsAny(s, " \t\n\r'\"\\;#~") {
		escaped := strings.ReplaceAll(s, "'", "'\\''")
		return "'" + escaped + "'"
	}
	return s
}
