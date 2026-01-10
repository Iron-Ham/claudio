package instance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance/capture"
	"github.com/Iron-Ham/claudio/internal/instance/detect"
	"github.com/Iron-Ham/claudio/internal/logging"
)

// StateChangeCallback is called when the detected waiting state changes
type StateChangeCallback func(instanceID string, state detect.WaitingState)

// TimeoutType represents the type of timeout that occurred
type TimeoutType int

const (
	TimeoutActivity   TimeoutType = iota // No activity for configured period
	TimeoutCompletion                    // Total runtime exceeded limit
	TimeoutStale                         // Repeated output detected (stuck in loop)
)

// timeoutTypeName returns a human-readable name for a timeout type
func timeoutTypeName(t TimeoutType) string {
	switch t {
	case TimeoutActivity:
		return "activity"
	case TimeoutCompletion:
		return "completion"
	case TimeoutStale:
		return "stale"
	default:
		return "unknown"
	}
}

// TimeoutCallback is called when a timeout condition is detected
type TimeoutCallback func(instanceID string, timeoutType TimeoutType)

// BellCallback is called when a terminal bell is detected in the tmux session
type BellCallback func(instanceID string)

// ManagerConfig holds configuration for instance management
type ManagerConfig struct {
	OutputBufferSize         int
	CaptureIntervalMs        int
	TmuxWidth                int
	TmuxHeight               int
	ActivityTimeoutMinutes   int  // 0 = disabled
	CompletionTimeoutMinutes int  // 0 = disabled
	StaleDetection           bool // Enable repeated output detection
}

// DefaultManagerConfig returns the default manager configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		OutputBufferSize:         100000, // 100KB
		CaptureIntervalMs:        100,
		TmuxWidth:                200,
		TmuxHeight:               30, // Shorter height so prompts scroll off and users see actual work
		ActivityTimeoutMinutes:   30, // 30 minutes of no activity
		CompletionTimeoutMinutes: 120, // 2 hours max runtime
		StaleDetection:           true,
	}
}

// MetricsChangeCallback is called when metrics are updated
type MetricsChangeCallback func(instanceID string, metrics *ParsedMetrics)

// Manager handles a single Claude Code instance running in a tmux session
type Manager struct {
	id          string
	sessionID   string // Claudio session ID (for multi-session support)
	workdir     string
	task        string
	sessionName string // tmux session name
	outputBuf   *capture.RingBuffer
	mu          sync.RWMutex
	running     bool
	paused      bool
	doneChan    chan struct{}
	captureTick *time.Ticker
	config      ManagerConfig

	// State detection
	detector      *detect.Detector
	currentState  detect.WaitingState
	stateCallback StateChangeCallback

	// Metrics tracking
	metricsParser   *MetricsParser
	currentMetrics  *ParsedMetrics
	metricsCallback MetricsChangeCallback
	startTime       *time.Time

	// Timeout tracking
	lastActivityTime    time.Time      // Last time output changed
	lastOutputHash      string         // Hash of last output for change detection
	repeatedOutputCount int            // Count of consecutive identical outputs (for stale detection)
	timeoutCallback     TimeoutCallback
	timedOut            bool           // Whether a timeout has been triggered
	timeoutType         TimeoutType    // Type of timeout that was triggered

	// Bell tracking
	bellCallback  BellCallback
	lastBellState bool // Track last bell flag state to detect transitions

	// Logger for structured logging
	logger *logging.Logger
}

// NewManager creates a new instance manager with default configuration
func NewManager(id, workdir, task string) *Manager {
	return NewManagerWithConfig(id, workdir, task, DefaultManagerConfig())
}

// NewManagerWithConfig creates a new instance manager with the given configuration.
// Uses legacy tmux naming (claudio-{instanceID}) for backwards compatibility.
func NewManagerWithConfig(id, workdir, task string, cfg ManagerConfig) *Manager {
	return &Manager{
		id:            id,
		workdir:       workdir,
		task:          task,
		sessionName:   fmt.Sprintf("claudio-%s", id),
		outputBuf:     capture.NewRingBuffer(cfg.OutputBufferSize),
		doneChan:      make(chan struct{}),
		config:        cfg,
		detector:      detect.NewDetector(),
		currentState:  detect.StateWorking,
		metricsParser: NewMetricsParser(),
	}
}

// NewManagerWithSession creates a new instance manager with session-scoped tmux naming.
// The tmux session will be named claudio-{sessionID}-{instanceID} to prevent collisions
// when multiple Claudio sessions are running simultaneously.
func NewManagerWithSession(sessionID, id, workdir, task string, cfg ManagerConfig) *Manager {
	// Use session-scoped naming if sessionID is provided
	var sessionName string
	if sessionID != "" {
		sessionName = fmt.Sprintf("claudio-%s-%s", sessionID, id)
	} else {
		sessionName = fmt.Sprintf("claudio-%s", id)
	}

	return &Manager{
		id:            id,
		sessionID:     sessionID,
		workdir:       workdir,
		task:          task,
		sessionName:   sessionName,
		outputBuf:     capture.NewRingBuffer(cfg.OutputBufferSize),
		doneChan:      make(chan struct{}),
		config:        cfg,
		detector:      detect.NewDetector(),
		currentState:  detect.StateWorking,
		metricsParser: NewMetricsParser(),
	}
}

// SetStateCallback sets a callback that will be invoked when the detected state changes
func (m *Manager) SetStateCallback(cb StateChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateCallback = cb
}

// SetMetricsCallback sets a callback that will be invoked when metrics are updated
func (m *Manager) SetMetricsCallback(cb MetricsChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metricsCallback = cb
}

// SetTimeoutCallback sets a callback that will be invoked when a timeout is detected
func (m *Manager) SetTimeoutCallback(cb TimeoutCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeoutCallback = cb
}

// SetBellCallback sets a callback that will be invoked when a terminal bell is detected
func (m *Manager) SetBellCallback(cb BellCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bellCallback = cb
}

// SetLogger sets the logger for the instance manager.
// If logger is nil, logging is disabled.
func (m *Manager) SetLogger(logger *logging.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger = logger
}

// CurrentMetrics returns the currently parsed metrics
func (m *Manager) CurrentMetrics() *ParsedMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentMetrics
}

// StartTime returns when the instance was started
func (m *Manager) StartTime() *time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.startTime
}

// CurrentState returns the currently detected waiting state
func (m *Manager) CurrentState() detect.WaitingState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentState
}

// TimedOut returns whether the instance has timed out and the type of timeout
func (m *Manager) TimedOut() (bool, TimeoutType) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.timedOut, m.timeoutType
}

// LastActivityTime returns when the instance last had output activity
func (m *Manager) LastActivityTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastActivityTime
}

// ClearTimeout resets the timeout state (for recovery/restart scenarios)
func (m *Manager) ClearTimeout() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timedOut = false
	m.repeatedOutputCount = 0
	m.lastActivityTime = time.Now()
}

// Start launches the Claude Code process in a tmux session
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("instance already running")
	}

	// Kill any existing session with this name (cleanup from previous run)
	_ = exec.Command("tmux", "kill-session", "-t", m.sessionName).Run()

	// Create a new detached tmux session with color support
	createCmd := exec.Command("tmux",
		"new-session",
		"-d",                                      // detached
		"-s", m.sessionName,                       // session name
		"-x", fmt.Sprintf("%d", m.config.TmuxWidth),  // width
		"-y", fmt.Sprintf("%d", m.config.TmuxHeight), // height
	)
	createCmd.Dir = m.workdir
	// Inherit full environment (required for Claude credentials) and ensure TERM supports colors
	createCmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if err := createCmd.Run(); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to create tmux session",
				"session_name", m.sessionName,
				"workdir", m.workdir,
				"error", err.Error())
		}
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Set up the tmux session for color support and large history
	_ = exec.Command("tmux", "set-option", "-t", m.sessionName, "history-limit", "10000").Run()
	_ = exec.Command("tmux", "set-option", "-t", m.sessionName, "default-terminal", "xterm-256color").Run()
	// Enable bell monitoring so we can detect and forward terminal bells
	_ = exec.Command("tmux", "set-option", "-t", m.sessionName, "-w", "monitor-bell", "on").Run()

	// Write the task/prompt to a temporary file to avoid shell escaping issues
	// (prompts with <, >, |, etc. would otherwise be interpreted by the shell)
	promptFile := filepath.Join(m.workdir, ".claude-prompt")
	if err := os.WriteFile(promptFile, []byte(m.task), 0600); err != nil {
		_ = exec.Command("tmux", "kill-session", "-t", m.sessionName).Run()
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	// Send the claude command to the tmux session, reading prompt from file
	claudeCmd := fmt.Sprintf("claude --dangerously-skip-permissions \"$(cat %q)\" && rm %q", promptFile, promptFile)
	sendCmd := exec.Command("tmux",
		"send-keys",
		"-t", m.sessionName,
		claudeCmd,
		"Enter",
	)
	if err := sendCmd.Run(); err != nil {
		// Clean up the session if we failed to start claude
		_ = exec.Command("tmux", "kill-session", "-t", m.sessionName).Run()
		_ = os.Remove(promptFile)
		if m.logger != nil {
			m.logger.Error("failed to start claude in tmux session",
				"session_name", m.sessionName,
				"error", err.Error())
		}
		return fmt.Errorf("failed to start claude in tmux session: %w", err)
	}

	m.running = true
	m.paused = false
	m.timedOut = false
	m.repeatedOutputCount = 0

	// Record start time for duration tracking
	now := time.Now()
	m.startTime = &now
	m.lastActivityTime = now

	// Start background goroutine to capture output periodically
	m.captureTick = time.NewTicker(time.Duration(m.config.CaptureIntervalMs) * time.Millisecond)
	go m.captureLoop()

	if m.logger != nil {
		m.logger.Info("tmux session created",
			"session_name", m.sessionName,
			"workdir", m.workdir)
	}

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
			timedOut := m.timedOut
			m.mu.RUnlock()

			// Skip processing if already timed out
			if timedOut {
				continue
			}

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
				byteCount := len(output)
				m.outputBuf.Reset()
				_, _ = m.outputBuf.Write(output)

				// Update activity tracking
				m.mu.Lock()
				m.lastActivityTime = time.Now()
				m.lastOutputHash = lastOutput
				m.repeatedOutputCount = 0
				logger := m.logger
				m.mu.Unlock()

				if logger != nil {
					logger.Debug("output captured",
						"byte_count", byteCount)
				}

				lastOutput = currentOutput

				// Detect waiting state from the new output
				m.detectAndNotifyState(output)
			} else {
				// Output hasn't changed - check for stale detection
				m.mu.Lock()
				if m.config.StaleDetection {
					m.repeatedOutputCount++
				}
				m.mu.Unlock()
			}

			// Check for timeout conditions
			m.checkTimeouts()

			// Check for terminal bells and forward them
			m.checkAndForwardBell(sessionName)

			// Check if the session is still running
			checkCmd := exec.Command("tmux", "has-session", "-t", sessionName)
			if checkCmd.Run() != nil {
				// Session ended - notify completion and stop
				m.mu.Lock()
				m.running = false
				callback := m.stateCallback
				instanceID := m.id
				m.currentState = detect.StateCompleted
				m.mu.Unlock()

				// Fire the completion callback so coordinator knows task is done
				if callback != nil {
					callback(instanceID, detect.StateCompleted)
				}
				return
			}
		}
	}
}

// checkTimeouts checks for various timeout conditions and triggers callbacks
func (m *Manager) checkTimeouts() {
	m.mu.Lock()
	if m.timedOut || !m.running || m.paused {
		m.mu.Unlock()
		return
	}

	now := time.Now()
	callback := m.timeoutCallback
	instanceID := m.id
	logger := m.logger
	var triggeredTimeout *TimeoutType
	repeatCount := m.repeatedOutputCount

	// Check completion timeout (total runtime)
	if m.config.CompletionTimeoutMinutes > 0 && m.startTime != nil {
		completionTimeout := time.Duration(m.config.CompletionTimeoutMinutes) * time.Minute
		if now.Sub(*m.startTime) > completionTimeout {
			t := TimeoutCompletion
			triggeredTimeout = &t
			m.timedOut = true
			m.timeoutType = TimeoutCompletion
		}
	}

	// Check activity timeout (no output changes)
	if triggeredTimeout == nil && m.config.ActivityTimeoutMinutes > 0 {
		activityTimeout := time.Duration(m.config.ActivityTimeoutMinutes) * time.Minute
		if now.Sub(m.lastActivityTime) > activityTimeout {
			t := TimeoutActivity
			triggeredTimeout = &t
			m.timedOut = true
			m.timeoutType = TimeoutActivity
		}
	}

	// Check for stale detection (repeated identical output)
	// Trigger if we've seen the same output 3000 times (5 minutes at 100ms interval)
	// This catches stuck loops producing identical output while allowing time for
	// legitimate long-running operations like planning and exploration
	if triggeredTimeout == nil && m.config.StaleDetection && repeatCount > 3000 {
		t := TimeoutStale
		triggeredTimeout = &t
		m.timedOut = true
		m.timeoutType = TimeoutStale
	}

	m.mu.Unlock()

	// Log and invoke callback outside of lock to prevent deadlocks
	if triggeredTimeout != nil {
		if logger != nil {
			if *triggeredTimeout == TimeoutStale {
				logger.Warn("stale output detected",
					"instance_id", instanceID,
					"repeat_count", repeatCount)
			} else {
				logger.Warn("timeout triggered",
					"instance_id", instanceID,
					"timeout_type", timeoutTypeName(*triggeredTimeout))
			}
		}
		if callback != nil {
			callback(instanceID, *triggeredTimeout)
		}
	}
}

// checkAndForwardBell checks for terminal bells and triggers the callback if detected
func (m *Manager) checkAndForwardBell(sessionName string) {
	// Query the window_bell_flag from tmux
	bellCmd := exec.Command("tmux", "display-message", "-t", sessionName, "-p", "#{window_bell_flag}")
	output, err := bellCmd.Output()
	if err != nil {
		return
	}

	bellActive := strings.TrimSpace(string(output)) == "1"

	m.mu.Lock()
	lastBellState := m.lastBellState
	callback := m.bellCallback
	instanceID := m.id
	logger := m.logger
	m.lastBellState = bellActive
	m.mu.Unlock()

	// Trigger callback on transition from no-bell to bell (edge detection)
	// This ensures we only fire once per bell, not continuously while the flag is set
	if bellActive && !lastBellState {
		if logger != nil {
			logger.Debug("bell detected",
				"instance_id", instanceID)
		}
		if callback != nil {
			callback(instanceID)
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
	logger := m.logger

	if newState != oldState {
		m.currentState = newState
	}
	m.mu.Unlock()

	// Log and invoke callback outside of lock to prevent deadlocks
	if newState != oldState {
		if logger != nil {
			logger.Info("instance state changed",
				"instance_id", instanceID,
				"old_state", oldState.String(),
				"new_state", newState.String())
		}
		if callback != nil {
			callback(instanceID, newState)
		}
	}

	// Parse and notify about metrics changes
	m.parseAndNotifyMetrics(output)
}

// parseAndNotifyMetrics parses metrics from output and notifies if changed
func (m *Manager) parseAndNotifyMetrics(output []byte) {
	newMetrics := m.metricsParser.Parse(output)
	if newMetrics == nil {
		return
	}

	m.mu.Lock()
	oldMetrics := m.currentMetrics
	callback := m.metricsCallback
	instanceID := m.id
	logger := m.logger

	// Check if metrics changed (simple comparison)
	metricsChanged := oldMetrics == nil ||
		newMetrics.InputTokens != oldMetrics.InputTokens ||
		newMetrics.OutputTokens != oldMetrics.OutputTokens ||
		newMetrics.Cost != oldMetrics.Cost

	if metricsChanged {
		m.currentMetrics = newMetrics
	}
	m.mu.Unlock()

	// Log and invoke callback outside of lock to prevent deadlocks
	if metricsChanged {
		if logger != nil {
			logger.Debug("metrics updated",
				"instance_id", instanceID,
				"tokens", newMetrics.InputTokens+newMetrics.OutputTokens,
				"cost", newMetrics.Cost)
		}
		if callback != nil {
			callback(instanceID, newMetrics)
		}
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
	_ = exec.Command("tmux", "send-keys", "-t", m.sessionName, "C-c").Run()
	time.Sleep(500 * time.Millisecond)

	// Kill the tmux session
	if err := exec.Command("tmux", "kill-session", "-t", m.sessionName).Run(); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to kill tmux session",
				"session_name", m.sessionName,
				"error", err.Error())
		}
	} else {
		if m.logger != nil {
			m.logger.Info("tmux session stopped",
				"session_name", m.sessionName)
		}
	}

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

		_ = exec.Command("tmux", "send-keys", "-t", m.sessionName, "-l", key).Run()
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

	// Run async to avoid blocking the UI thread
	go func() {
		_ = exec.Command("tmux", "send-keys", "-t", sessionName, key).Run()
	}()
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

	// Run async to avoid blocking the UI thread
	// -l flag sends keys literally without interpretation
	go func() {
		_ = exec.Command("tmux", "send-keys", "-t", sessionName, "-l", text).Run()
	}()
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

	// Run async to avoid blocking the UI thread
	// Commands run sequentially within the goroutine to maintain paste order
	go func() {
		// Bracketed paste mode escape sequences
		// Start: ESC[200~ End: ESC[201~
		// This tells the receiving application that the following text is pasted
		pasteStart := "\x1b[200~"
		pasteEnd := "\x1b[201~"

		// Send bracketed paste start
		_ = exec.Command("tmux", "send-keys", "-t", sessionName, "-l", pasteStart).Run()
		// Send the pasted content
		_ = exec.Command("tmux", "send-keys", "-t", sessionName, "-l", text).Run()
		// Send bracketed paste end
		_ = exec.Command("tmux", "send-keys", "-t", sessionName, "-l", pasteEnd).Run()
	}()
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
	_, _ = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &pid)
	return pid
}

// AttachCommand returns the command to attach to this instance's tmux session
// This allows users to attach directly if needed
func (m *Manager) AttachCommand() string {
	return fmt.Sprintf("tmux attach -t %s", m.sessionName)
}

// TmuxSessionExists checks if the tmux session for this instance exists
func (m *Manager) TmuxSessionExists() bool {
	cmd := exec.Command("tmux", "has-session", "-t", m.sessionName)
	return cmd.Run() == nil
}

// Reconnect attempts to reconnect to an existing tmux session
// This is used for session recovery after a restart
func (m *Manager) Reconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("instance already running")
	}

	// Check if the tmux session exists
	if !m.TmuxSessionExists() {
		return fmt.Errorf("tmux session %s does not exist", m.sessionName)
	}

	// Ensure monitor-bell is enabled for bell detection (may not be set if session was created before this feature)
	_ = exec.Command("tmux", "set-option", "-t", m.sessionName, "-w", "monitor-bell", "on").Run()

	m.running = true
	m.paused = false
	m.timedOut = false
	m.repeatedOutputCount = 0
	m.lastActivityTime = time.Now()
	m.lastBellState = false // Reset bell state on reconnect
	m.doneChan = make(chan struct{})

	// Start background goroutine to capture output periodically
	m.captureTick = time.NewTicker(time.Duration(m.config.CaptureIntervalMs) * time.Millisecond)
	go m.captureLoop()

	if m.logger != nil {
		m.logger.Info("instance reconnected",
			"session_name", m.sessionName)
	}

	return nil
}

// ListClaudioTmuxSessions returns a list of all tmux sessions with the claudio- prefix
func ListClaudioTmuxSessions() ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// No sessions or tmux not running
		return nil, nil
	}

	var sessions []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.HasPrefix(line, "claudio-") {
			sessions = append(sessions, line)
		}
	}
	return sessions, nil
}

// ExtractInstanceIDFromSession extracts the instance ID from a claudio tmux session name.
// Supports both legacy format (claudio-{instanceID}) and new format (claudio-{sessionID}-{instanceID}).
// For PR workflow sessions (claudio-{id}-pr or claudio-{sessionID}-{id}-pr), removes the -pr suffix first.
func ExtractInstanceIDFromSession(sessionName string) string {
	if !strings.HasPrefix(sessionName, "claudio-") {
		return ""
	}

	// Remove "claudio-" prefix
	rest := strings.TrimPrefix(sessionName, "claudio-")

	// Remove -pr suffix if present (PR workflow sessions)
	rest = strings.TrimSuffix(rest, "-pr")

	// Check if this is new format (claudio-{sessionID}-{instanceID})
	// by looking for a second hyphen after the session ID
	parts := strings.SplitN(rest, "-", 2)
	if len(parts) == 2 && len(parts[0]) == 8 && len(parts[1]) >= 8 {
		// Likely new format: first part is sessionID (8 chars), second is instanceID
		return parts[1]
	}

	// Legacy format or instance ID only
	return rest
}

// ExtractSessionAndInstanceID extracts both session ID and instance ID from a tmux session name.
// Returns (sessionID, instanceID). For legacy format, sessionID will be empty.
// For PR workflow sessions, removes the -pr suffix first.
func ExtractSessionAndInstanceID(sessionName string) (sessionID, instanceID string) {
	if !strings.HasPrefix(sessionName, "claudio-") {
		return "", ""
	}

	// Remove "claudio-" prefix
	rest := strings.TrimPrefix(sessionName, "claudio-")

	// Remove -pr suffix if present (PR workflow sessions)
	rest = strings.TrimSuffix(rest, "-pr")

	// Check if this is new format (claudio-{sessionID}-{instanceID})
	parts := strings.SplitN(rest, "-", 2)
	if len(parts) == 2 && len(parts[0]) == 8 && len(parts[1]) >= 8 {
		// New format: first part is sessionID, second is instanceID
		return parts[0], parts[1]
	}

	// Legacy format: no session ID, just instance ID
	return "", rest
}

// ListSessionTmuxSessions returns tmux sessions for a specific Claudio session.
// Filters by session ID prefix in the tmux session name.
func ListSessionTmuxSessions(sessionID string) ([]string, error) {
	allSessions, err := ListClaudioTmuxSessions()
	if err != nil {
		return nil, err
	}

	if sessionID == "" {
		return allSessions, nil
	}

	prefix := fmt.Sprintf("claudio-%s-", sessionID)
	var sessions []string
	for _, s := range allSessions {
		if strings.HasPrefix(s, prefix) {
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
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
