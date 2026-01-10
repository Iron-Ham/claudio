package instance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// StateChangeCallback is called when the detected waiting state changes
type StateChangeCallback func(instanceID string, state WaitingState)

// TimeoutType represents the type of timeout that occurred
type TimeoutType int

const (
	TimeoutActivity   TimeoutType = iota // No activity for configured period
	TimeoutCompletion                    // Total runtime exceeded limit
	TimeoutStale                         // Repeated output detected (stuck in loop)
)

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
	bellCallback   BellCallback
	lastBellState  bool // Track last bell flag state to detect transitions
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
		outputBuf:     NewRingBuffer(cfg.OutputBufferSize),
		doneChan:      make(chan struct{}),
		config:        cfg,
		detector:      NewDetector(),
		currentState:  StateWorking,
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
		outputBuf:     NewRingBuffer(cfg.OutputBufferSize),
		doneChan:      make(chan struct{}),
		config:        cfg,
		detector:      NewDetector(),
		currentState:  StateWorking,
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
func (m *Manager) CurrentState() WaitingState {
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

	// Create a new detached tmux session
	tmuxCfg := TmuxConfig{
		Width:  m.config.TmuxWidth,
		Height: m.config.TmuxHeight,
	}
	if err := createTmuxSession(m.sessionName, m.workdir, tmuxCfg); err != nil {
		return err
	}

	// Write the task/prompt to a temporary file to avoid shell escaping issues
	// (prompts with <, >, |, etc. would otherwise be interpreted by the shell)
	promptFile := filepath.Join(m.workdir, ".claude-prompt")
	if err := os.WriteFile(promptFile, []byte(m.task), 0600); err != nil {
		_ = killTmuxSession(m.sessionName)
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	// Send the claude command to the tmux session, reading prompt from file
	claudeCmd := fmt.Sprintf("claude --dangerously-skip-permissions \"$(cat %q)\" && rm %q", promptFile, promptFile)
	if err := sendTmuxKeys(m.sessionName, claudeCmd, false); err != nil {
		// Clean up the session if we failed to start claude
		_ = killTmuxSession(m.sessionName)
		_ = os.Remove(promptFile)
		return fmt.Errorf("failed to start claude in tmux session: %w", err)
	}
	// Send Enter to execute the command
	if err := sendTmuxSpecialKey(m.sessionName, "Enter"); err != nil {
		_ = killTmuxSession(m.sessionName)
		_ = os.Remove(promptFile)
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

	return nil
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
	_ = sendTmuxSpecialKey(m.sessionName, "C-c")
	time.Sleep(500 * time.Millisecond)

	// Kill the tmux session
	_ = killTmuxSession(m.sessionName)

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
		key := mapRuneToTmuxKey(r)
		_ = sendTmuxKeys(m.sessionName, key, true)
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
		_ = sendTmuxSpecialKey(sessionName, key)
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
		_ = sendTmuxKeys(sessionName, text, true)
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
		_ = sendBracketedPaste(sessionName, text)
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
	pid, err := tmuxGetPanePID(m.sessionName)
	if err != nil {
		return 0
	}
	return pid
}

// AttachCommand returns the command to attach to this instance's tmux session
// This allows users to attach directly if needed
func (m *Manager) AttachCommand() string {
	return fmt.Sprintf("tmux attach -t %s", m.sessionName)
}

// TmuxSessionExists checks if the tmux session for this instance exists
func (m *Manager) TmuxSessionExists() bool {
	return tmuxSessionExists(m.sessionName)
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
	if !tmuxSessionExists(m.sessionName) {
		return fmt.Errorf("tmux session %s does not exist", m.sessionName)
	}

	// Ensure monitor-bell is enabled for bell detection (may not be set if session was created before this feature)
	_ = setTmuxWindowOption(m.sessionName, "monitor-bell", "on")

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

	return nil
}

// ListClaudioTmuxSessions returns a list of all tmux sessions with the claudio- prefix
func ListClaudioTmuxSessions() ([]string, error) {
	return listClaudioTmuxSessions()
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
	return resizeTmuxWindow(m.sessionName, width, height)
}
