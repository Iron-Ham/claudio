package instance

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
)

// Manager handles a single Claude Code instance
type Manager struct {
	id          string
	workdir     string
	task        string
	cmd         *exec.Cmd
	pty         *os.File
	outputBuf   *RingBuffer
	mu          sync.RWMutex
	running     bool
	inputChan   chan []byte
	outputChan  chan []byte
	doneChan    chan struct{}
}

// NewManager creates a new instance manager
func NewManager(id, workdir, task string) *Manager {
	return &Manager{
		id:         id,
		workdir:    workdir,
		task:       task,
		outputBuf:  NewRingBuffer(100000), // 100KB output buffer
		inputChan:  make(chan []byte, 100),
		outputChan: make(chan []byte, 1000),
		doneChan:   make(chan struct{}),
	}
}

// Start launches the Claude Code process
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("instance already running")
	}

	// Create the claude command with the task as the initial prompt
	m.cmd = exec.Command("claude", "--dangerously-skip-permissions", m.task)
	m.cmd.Dir = m.workdir
	m.cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Start with PTY
	ptmx, err := pty.Start(m.cmd)
	if err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}
	m.pty = ptmx
	m.running = true

	// Start output reader goroutine
	go m.readOutput()

	// Start input writer goroutine
	go m.writeInput()

	return nil
}

// Stop terminates the Claude process
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	// Signal stop
	close(m.doneChan)

	// Try graceful termination first
	if m.cmd.Process != nil {
		m.cmd.Process.Signal(syscall.SIGTERM)
	}

	// Close PTY
	if m.pty != nil {
		m.pty.Close()
	}

	// Wait for process
	if m.cmd != nil {
		m.cmd.Wait()
	}

	m.running = false
	return nil
}

// Pause sends SIGSTOP to pause the process
func (m *Manager) Pause() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running || m.cmd.Process == nil {
		return nil
	}

	return m.cmd.Process.Signal(syscall.SIGSTOP)
}

// Resume sends SIGCONT to resume the process
func (m *Manager) Resume() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running || m.cmd.Process == nil {
		return nil
	}

	return m.cmd.Process.Signal(syscall.SIGCONT)
}

// SendInput sends input to the Claude process
func (m *Manager) SendInput(data []byte) {
	select {
	case m.inputChan <- data:
	default:
		// Channel full, drop input
	}
}

// Output returns the output channel
func (m *Manager) Output() <-chan []byte {
	return m.outputChan
}

// GetOutput returns all buffered output
func (m *Manager) GetOutput() []byte {
	return m.outputBuf.Bytes()
}

// Running returns whether the instance is running
func (m *Manager) Running() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// PID returns the process ID
func (m *Manager) PID() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Pid
	}
	return 0
}

// ID returns the instance ID
func (m *Manager) ID() string {
	return m.id
}

// readOutput reads from PTY and broadcasts to channels
func (m *Manager) readOutput() {
	buf := make([]byte, 4096)

	for {
		select {
		case <-m.doneChan:
			return
		default:
			n, err := m.pty.Read(buf)
			if err != nil {
				if err != io.EOF {
					// Log error but don't stop
					fmt.Fprintf(os.Stderr, "read error: %v\n", err)
				}
				m.mu.Lock()
				m.running = false
				m.mu.Unlock()
				return
			}

			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])

				// Write to buffer
				m.outputBuf.Write(data)

				// Send to output channel (non-blocking)
				select {
				case m.outputChan <- data:
				default:
					// Channel full, drop this chunk
				}
			}
		}
	}
}

// writeInput reads from input channel and writes to PTY
func (m *Manager) writeInput() {
	for {
		select {
		case <-m.doneChan:
			return
		case data := <-m.inputChan:
			m.mu.RLock()
			if m.pty != nil {
				m.pty.Write(data)
			}
			m.mu.RUnlock()
		}
	}
}

// Resize resizes the PTY
func (m *Manager) Resize(rows, cols int) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.pty == nil {
		return nil
	}

	return pty.Setsize(m.pty, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}
