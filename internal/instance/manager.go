package instance

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// StateChangeCallback is called when the detected waiting state changes
type StateChangeCallback func(instanceID string, state WaitingState)

// ManagerConfig holds configuration for instance management
type ManagerConfig struct {
	OutputBufferSize  int
	CaptureIntervalMs int
	TmuxWidth         int
	TmuxHeight        int
}

// DefaultManagerConfig returns the default manager configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		OutputBufferSize:  100000, // 100KB
		CaptureIntervalMs: 100,
		TmuxWidth:         200,
		TmuxHeight:        50,
	}
}

// Manager handles a single Claude Code instance running in a tmux session
type Manager struct {
	id          string
	workdir     string
	task        string
	sessionName string
	outputBuf   *RingBuffer
	mu          sync.RWMutex
	running     bool
	paused      bool
	doneChan    chan struct{}
	captureTick *time.Ticker
	config      ManagerConfig

	// State detection
	detector      *Detector
	currentState  WaitingState
	stateCallback StateChangeCallback
}

// NewManager creates a new instance manager with default configuration
func NewManager(id, workdir, task string) *Manager {
	return NewManagerWithConfig(id, workdir, task, DefaultManagerConfig())
}

// NewManagerWithConfig creates a new instance manager with the given configuration
func NewManagerWithConfig(id, workdir, task string, cfg ManagerConfig) *Manager {
	return &Manager{
		id:           id,
		workdir:      workdir,
		task:         task,
		sessionName:  fmt.Sprintf("claudio-%s", id),
		outputBuf:    NewRingBuffer(cfg.OutputBufferSize),
		doneChan:     make(chan struct{}),
		config:       cfg,
		detector:     NewDetector(),
		currentState: StateWorking,
	}
}

// SetStateCallback sets a callback that will be invoked when the detected state changes
func (m *Manager) SetStateCallback(cb StateChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateCallback = cb
}

// CurrentState returns the currently detected waiting state
func (m *Manager) CurrentState() WaitingState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentState
}

// Start launches the Claude Code process in a tmux session
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("instance already running")
	}

	// Kill any existing session with this name (cleanup from previous run)
	exec.Command("tmux", "kill-session", "-t", m.sessionName).Run()

	// Create a new detached tmux session with color support
	createCmd := exec.Command("tmux",
		"new-session",
		"-d",                                      // detached
		"-s", m.sessionName,                       // session name
		"-x", fmt.Sprintf("%d", m.config.TmuxWidth),  // width
		"-y", fmt.Sprintf("%d", m.config.TmuxHeight), // height
	)
	createCmd.Dir = m.workdir
	// Ensure TERM supports colors
	createCmd.Env = append(createCmd.Env, "TERM=xterm-256color")
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Set up the tmux session for color support and large history
	exec.Command("tmux", "set-option", "-t", m.sessionName, "history-limit", "10000").Run()
	exec.Command("tmux", "set-option", "-t", m.sessionName, "default-terminal", "xterm-256color").Run()

	// Send the claude command to the tmux session
	claudeCmd := fmt.Sprintf("claude --dangerously-skip-permissions %q", m.task)
	sendCmd := exec.Command("tmux",
		"send-keys",
		"-t", m.sessionName,
		claudeCmd,
		"Enter",
	)
	if err := sendCmd.Run(); err != nil {
		// Clean up the session if we failed to start claude
		exec.Command("tmux", "kill-session", "-t", m.sessionName).Run()
		return fmt.Errorf("failed to start claude in tmux session: %w", err)
	}

	m.running = true
	m.paused = false

	// Start background goroutine to capture output periodically
	m.captureTick = time.NewTicker(time.Duration(m.config.CaptureIntervalMs) * time.Millisecond)
	go m.captureLoop()

	return nil
}

// captureLoop periodically captures output from the tmux session
func (m *Manager) captureLoop() {
	// Track last output hash to detect changes
	var lastOutput string

	for {
		select {
		case <-m.doneChan:
			return
		case <-m.captureTick.C:
			m.mu.RLock()
			if !m.running || m.paused {
				m.mu.RUnlock()
				continue
			}
			sessionName := m.sessionName
			m.mu.RUnlock()

			// Capture the entire visible pane plus scrollback
			// -p prints to stdout, -S - starts from beginning of history
			// -e preserves ANSI escape sequences (colors)
			captureCmd := exec.Command("tmux",
				"capture-pane",
				"-t", sessionName,
				"-p",      // print to stdout
				"-e",      // preserve escape sequences (colors)
				"-S", "-", // start from beginning of scrollback
				"-E", "-", // end at bottom of scrollback
			)
			output, err := captureCmd.Output()
			if err != nil {
				continue
			}

			// Always update if content changed
			currentOutput := string(output)
			if currentOutput != lastOutput {
				m.outputBuf.Reset()
				m.outputBuf.Write(output)
				lastOutput = currentOutput

				// Detect waiting state from the new output
				m.detectAndNotifyState(output)
			}

			// Check if the session is still running
			checkCmd := exec.Command("tmux", "has-session", "-t", sessionName)
			if checkCmd.Run() != nil {
				// Session ended
				m.mu.Lock()
				m.running = false
				m.mu.Unlock()
				return
			}
		}
	}
}

// detectAndNotifyState analyzes output and notifies if state changed
func (m *Manager) detectAndNotifyState(output []byte) {
	newState := m.detector.Detect(output)

	m.mu.Lock()
	oldState := m.currentState
	callback := m.stateCallback
	instanceID := m.id

	if newState != oldState {
		m.currentState = newState
	}
	m.mu.Unlock()

	// Invoke callback outside of lock to prevent deadlocks
	if newState != oldState && callback != nil {
		callback(instanceID, newState)
	}
}

// Stop terminates the tmux session
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	// Signal stop to capture loop
	select {
	case <-m.doneChan:
	default:
		close(m.doneChan)
	}

	// Stop the ticker
	if m.captureTick != nil {
		m.captureTick.Stop()
	}

	// Send Ctrl+C to gracefully stop Claude first
	exec.Command("tmux", "send-keys", "-t", m.sessionName, "C-c").Run()
	time.Sleep(500 * time.Millisecond)

	// Kill the tmux session
	exec.Command("tmux", "kill-session", "-t", m.sessionName).Run()

	m.running = false
	return nil
}

// Pause pauses output capture (tmux session continues running)
func (m *Manager) Pause() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.paused = true
	return nil
}

// Resume resumes output capture
func (m *Manager) Resume() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.paused = false
	return nil
}

// SendInput sends input to the tmux session
func (m *Manager) SendInput(data []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running {
		return
	}

	// Convert bytes to string for send-keys
	input := string(data)

	// Handle special characters
	// tmux send-keys interprets certain prefixes specially
	// We need to handle newlines, control characters, etc.
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

		exec.Command("tmux", "send-keys", "-t", m.sessionName, "-l", key).Run()
	}
}

// SendKey sends a special key to the tmux session
func (m *Manager) SendKey(key string) {
	m.mu.RLock()
	sessionName := m.sessionName
	running := m.running
	m.mu.RUnlock()

	if !running {
		return
	}

	// Use CombinedOutput to capture any error
	exec.Command("tmux", "send-keys", "-t", sessionName, key).Run()
}

// SendLiteral sends literal text to the tmux session (no interpretation)
func (m *Manager) SendLiteral(text string) {
	m.mu.RLock()
	sessionName := m.sessionName
	running := m.running
	m.mu.RUnlock()

	if !running {
		return
	}

	// -l flag sends keys literally without interpretation
	exec.Command("tmux", "send-keys", "-t", sessionName, "-l", text).Run()
}

// SendPaste sends pasted text to the tmux session with bracketed paste sequences
// This preserves the paste context for applications that support bracketed paste mode
func (m *Manager) SendPaste(text string) {
	m.mu.RLock()
	sessionName := m.sessionName
	running := m.running
	m.mu.RUnlock()

	if !running {
		return
	}

	// Bracketed paste mode escape sequences
	// Start: ESC[200~ End: ESC[201~
	// This tells the receiving application that the following text is pasted
	pasteStart := "\x1b[200~"
	pasteEnd := "\x1b[201~"

	// Send bracketed paste start
	exec.Command("tmux", "send-keys", "-t", sessionName, "-l", pasteStart).Run()
	// Send the pasted content
	exec.Command("tmux", "send-keys", "-t", sessionName, "-l", text).Run()
	// Send bracketed paste end
	exec.Command("tmux", "send-keys", "-t", sessionName, "-l", pasteEnd).Run()
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

// Paused returns whether the instance is paused
func (m *Manager) Paused() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.paused
}

// SessionName returns the tmux session name
func (m *Manager) SessionName() string {
	return m.sessionName
}

// ID returns the instance ID
func (m *Manager) ID() string {
	return m.id
}

// PID returns the process ID of the shell in the tmux session
func (m *Manager) PID() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running {
		return 0
	}

	// Get the PID from tmux
	cmd := exec.Command("tmux", "display-message", "-t", m.sessionName, "-p", "#{pane_pid}")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	var pid int
	fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &pid)
	return pid
}

// AttachCommand returns the command to attach to this instance's tmux session
// This allows users to attach directly if needed
func (m *Manager) AttachCommand() string {
	return fmt.Sprintf("tmux attach -t %s", m.sessionName)
}

// Resize changes the tmux pane dimensions
// This is useful when the display area changes (e.g., sidebar added/removed)
func (m *Manager) Resize(width, height int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	// Update stored config
	m.config.TmuxWidth = width
	m.config.TmuxHeight = height

	// Resize the tmux window
	// Note: We resize the window (not pane) since each session has one window
	resizeCmd := exec.Command("tmux",
		"resize-window",
		"-t", m.sessionName,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
	)
	if err := resizeCmd.Run(); err != nil {
		return fmt.Errorf("failed to resize tmux session: %w", err)
	}

	return nil
}
