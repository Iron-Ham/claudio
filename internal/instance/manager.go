package instance

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// Manager handles a single Claude Code instance
type Manager struct {
	id        string
	workdir   string
	task      string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	outputBuf *RingBuffer
	mu        sync.RWMutex
	running   bool
	doneChan  chan struct{}
}

// NewManager creates a new instance manager
func NewManager(id, workdir, task string) *Manager {
	return &Manager{
		id:        id,
		workdir:   workdir,
		task:      task,
		outputBuf: NewRingBuffer(100000), // 100KB output buffer
		doneChan:  make(chan struct{}),
	}
}

// Start launches the Claude Code process
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("instance already running")
	}

	// Create the claude command with --print for non-interactive mode
	// This gives us clean, isolated output without terminal interference
	m.cmd = exec.Command("claude",
		"--print",
		"--output-format", "text",
		"--dangerously-skip-permissions",
		m.task,
	)
	m.cmd.Dir = m.workdir

	// Set up environment - no TERM to prevent ANSI escape sequences
	env := os.Environ()
	m.cmd.Env = append(env, "NO_COLOR=1")

	// Create a new process group so it's isolated from our terminal
	m.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Set up pipes
	var err error
	m.stdin, err = m.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	m.stdout, err = m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	m.stderr, err = m.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}

	m.running = true

	// Start output readers
	go m.readPipe(m.stdout, "")
	go m.readPipe(m.stderr, "[stderr] ")

	// Wait for process in background
	go func() {
		m.cmd.Wait()
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()

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
	select {
	case <-m.doneChan:
	default:
		close(m.doneChan)
	}

	// Close stdin to signal EOF
	if m.stdin != nil {
		m.stdin.Close()
	}

	// Try graceful termination first
	if m.cmd.Process != nil {
		m.cmd.Process.Signal(syscall.SIGTERM)
	}

	// Wait for process (with timeout handled by caller if needed)
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

// SendInput sends input to the Claude process (limited in --print mode)
func (m *Manager) SendInput(data []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.stdin != nil {
		m.stdin.Write(data)
	}
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

// readPipe reads from a pipe and stores in buffer
func (m *Manager) readPipe(r io.Reader, prefix string) {
	scanner := bufio.NewScanner(r)
	// Increase buffer size for long lines
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-m.doneChan:
			return
		default:
			line := prefix + scanner.Text() + "\n"
			m.outputBuf.Write([]byte(line))
		}
	}
}
