package instance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance/capture"
	"github.com/Iron-Ham/claudio/internal/instance/detect"
	"github.com/Iron-Ham/claudio/internal/instance/input"
	"github.com/Iron-Ham/claudio/internal/instance/lifecycle"
	"github.com/Iron-Ham/claudio/internal/instance/metrics"
	"github.com/Iron-Ham/claudio/internal/instance/state"
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

// fullRefreshInterval is the number of capture ticks between full scrollback captures.
// At 100ms per tick, 50 ticks = 5 seconds between full refreshes.
const fullRefreshInterval = 50

// tmuxCommandTimeout is the maximum time to wait for tmux subprocess commands.
// This prevents the capture loop from hanging indefinitely if tmux becomes unresponsive.
const tmuxCommandTimeout = 2 * time.Second

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
	TmuxHistoryLimit         int  // Number of lines of scrollback to keep (default: 50000)
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
		TmuxHeight:               30,    // Shorter height so prompts scroll off and users see actual work
		TmuxHistoryLimit:         50000, // 50k lines of scrollback
		ActivityTimeoutMinutes:   30,    // 30 minutes of no activity
		CompletionTimeoutMinutes: 0,     // Disabled by default (no max runtime limit)
		StaleDetection:           true,
	}
}

// MetricsChangeCallback is called when metrics are updated
type MetricsChangeCallback func(instanceID string, metrics *metrics.ParsedMetrics)

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
	metricsParser   *metrics.MetricsParser
	currentMetrics  *metrics.ParsedMetrics
	metricsCallback MetricsChangeCallback
	startTime       *time.Time

	// Timeout tracking
	lastActivityTime    time.Time // Last time output changed
	lastOutputHash      string    // Hash of last output for change detection
	repeatedOutputCount int       // Count of consecutive identical outputs (for stale detection)
	timeoutCallback     TimeoutCallback
	timedOut            bool        // Whether a timeout has been triggered
	timeoutType         TimeoutType // Type of timeout that was triggered

	// Differential capture optimization
	lastHistorySize    int // Last captured history size (for differential capture)
	fullRefreshCounter int // Counter for periodic full refresh

	// Bell tracking
	bellCallback  BellCallback
	lastBellState bool // Track last bell flag state to detect transitions

	// Input handling
	inputHandler *input.Handler

	// Logger for structured logging
	logger *logging.Logger

	// lifecycleManager is an optional lifecycle manager for coordinated lifecycle operations.
	// When set, certain operations may delegate to this manager for better separation of concerns.
	lifecycleManager *lifecycle.Manager

	// Optional external state monitor for centralized state tracking
	stateMonitor *state.Monitor
}

// NewManager creates a new instance manager with default configuration
func NewManager(id, workdir, task string) *Manager {
	return NewManagerWithConfig(id, workdir, task, DefaultManagerConfig())
}

// NewManagerWithConfig creates a new instance manager with the given configuration.
// Uses legacy tmux naming (claudio-{instanceID}) for backwards compatibility.
func NewManagerWithConfig(id, workdir, task string, cfg ManagerConfig) *Manager {
	sessionName := fmt.Sprintf("claudio-%s", id)
	return &Manager{
		id:            id,
		workdir:       workdir,
		task:          task,
		sessionName:   sessionName,
		outputBuf:     capture.NewRingBuffer(cfg.OutputBufferSize),
		doneChan:      make(chan struct{}),
		config:        cfg,
		detector:      detect.NewDetector(),
		currentState:  detect.StateWorking,
		metricsParser: metrics.NewMetricsParser(),
		inputHandler: input.NewHandler(
			input.WithPersistentSender(sessionName),
			input.WithBatching(sessionName, input.DefaultBatchConfig()),
		),
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
		metricsParser: metrics.NewMetricsParser(),
		inputHandler: input.NewHandler(
			input.WithPersistentSender(sessionName),
			input.WithBatching(sessionName, input.DefaultBatchConfig()),
		),
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

// SetStateMonitor sets an external state monitor for centralized state tracking.
// When set, the manager delegates state detection, timeout checking, and bell detection
// to the monitor. This allows multiple managers to share a single monitor for
// coordinated state management.
//
// If monitor is nil, the manager uses its internal state tracking.
func (m *Manager) SetStateMonitor(monitor *state.Monitor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateMonitor = monitor
}

// StateMonitor returns the configured state monitor, or nil if not set.
func (m *Manager) StateMonitor() *state.Monitor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stateMonitor
}

// CurrentMetrics returns the currently parsed metrics
func (m *Manager) CurrentMetrics() *metrics.ParsedMetrics {
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
	m.timedOut = false
	m.repeatedOutputCount = 0
	m.lastActivityTime = time.Now()
	stateMonitor := m.stateMonitor
	instanceID := m.id
	m.mu.Unlock()

	// Also clear timeout in external state monitor if configured
	if stateMonitor != nil {
		stateMonitor.ClearTimeout(instanceID)
	}
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

	// Determine history limit from config (default to 50000 if not set)
	historyLimit := m.config.TmuxHistoryLimit
	if historyLimit == 0 {
		historyLimit = 50000
	}

	// Set history-limit BEFORE creating session so the new pane inherits it.
	// tmux's history-limit only affects newly created panes, not existing ones.
	if err := exec.Command("tmux", "set-option", "-g", "history-limit", fmt.Sprintf("%d", historyLimit)).Run(); err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to set global history-limit for tmux",
				"history_limit", historyLimit,
				"error", err.Error())
		}
	}

	// Create a new detached tmux session with color support
	createCmd := exec.Command("tmux",
		"new-session",
		"-d",                // detached
		"-s", m.sessionName, // session name
		"-x", fmt.Sprintf("%d", m.config.TmuxWidth), // width
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

	// Set up additional tmux session options for color support
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

	// Register with external state monitor if configured
	if m.stateMonitor != nil {
		m.stateMonitor.Start(m.id)
	}

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

			// Differential capture optimization:
			// - Check history size to detect new output
			// - Capture only visible pane when nothing changed (much faster)
			// - Do full capture when history grows or periodically (every 50 ticks = 5 seconds)
			currentHistorySize := m.getHistorySize(sessionName)
			if currentHistorySize == -1 {
				// Session may not exist, skip this tick
				continue
			}

			m.mu.Lock()
			lastHistorySize := m.lastHistorySize
			m.fullRefreshCounter++
			doFullCapture := m.fullRefreshCounter >= fullRefreshInterval || currentHistorySize > lastHistorySize
			if doFullCapture {
				m.fullRefreshCounter = 0
				m.lastHistorySize = currentHistorySize
			}
			m.mu.Unlock()

			var output []byte
			var err error
			if doFullCapture {
				output, err = m.captureFullPane(sessionName)
			} else {
				output, err = m.captureVisiblePane(sessionName)
			}
			if err != nil {
				m.mu.RLock()
				logger := m.logger
				m.mu.RUnlock()
				if logger != nil {
					logger.Debug("capture failed",
						"session_name", sessionName,
						"full_capture", doFullCapture,
						"error", err.Error())
				}
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
				stateMonitor := m.stateMonitor
				instanceID := m.id
				m.mu.Unlock()

				if logger != nil {
					logger.Debug("output captured",
						"byte_count", byteCount)
				}

				lastOutput = currentOutput

				// Detect waiting state from the new output
				if stateMonitor != nil {
					// Use external state monitor for state detection
					stateMonitor.ProcessOutput(instanceID, output, currentOutput)
				} else {
					// Fall back to internal state detection
					m.detectAndNotifyState(output)
				}
			} else {
				// Output hasn't changed - check for stale detection
				m.mu.Lock()
				if m.config.StaleDetection {
					m.repeatedOutputCount++
				}
				stateMonitor := m.stateMonitor
				instanceID := m.id
				m.mu.Unlock()

				// Also notify state monitor about unchanged output (for stale tracking)
				if stateMonitor != nil {
					stateMonitor.ProcessOutput(instanceID, output, currentOutput)
				}
			}

			// Check for timeout conditions
			m.mu.RLock()
			stateMonitor := m.stateMonitor
			instanceID := m.id
			m.mu.RUnlock()

			if stateMonitor != nil {
				stateMonitor.CheckTimeouts(instanceID)
			} else {
				m.checkTimeouts()
			}

			// Check for terminal bells and forward them
			m.checkAndForwardBellWithMonitor(sessionName, stateMonitor, instanceID)

			// Check if the session is still running
			ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
			checkCmd := exec.CommandContext(ctx, "tmux", "has-session", "-t", sessionName)
			sessionErr := checkCmd.Run()
			cancel()
			if sessionErr != nil {
				// Check if this was a timeout vs actual session end
				if ctx.Err() == context.DeadlineExceeded {
					m.mu.RLock()
					logger := m.logger
					m.mu.RUnlock()
					if logger != nil {
						logger.Warn("tmux has-session timed out, will retry next tick",
							"session_name", sessionName)
					}
					continue // Don't mark as completed on timeout, retry next tick
				}
				// Session actually ended - notify completion and stop
				m.mu.Lock()
				m.running = false
				callback := m.stateCallback
				localInstanceID := m.id
				localStateMonitor := m.stateMonitor
				m.currentState = detect.StateCompleted
				m.mu.Unlock()

				// Notify state monitor about completion
				if localStateMonitor != nil {
					localStateMonitor.SetState(localInstanceID, detect.StateCompleted)
					localStateMonitor.Stop(localInstanceID)
				}

				// Fire the completion callback so coordinator knows task is done
				if callback != nil {
					callback(localInstanceID, detect.StateCompleted)
				}
				return
			}
		}
	}
}

// getHistorySize queries the current tmux pane history size.
// Returns -1 if the query fails (indicates session may not exist or tmux error).
// Note: Called without lock since it's a tmux query, and lastHistorySize is only
// modified within the captureLoop goroutine.
func (m *Manager) getHistorySize(sessionName string) int {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "display-message", "-t", sessionName, "-p", "#{history_size}")
	output, err := cmd.Output()
	if err != nil {
		// Session not found is expected when session ends - don't log
		// Other errors (tmux crash, etc.) are worth logging at debug level
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if !strings.Contains(stderr, "can't find") && !strings.Contains(stderr, "no server running") {
				m.mu.RLock()
				logger := m.logger
				m.mu.RUnlock()
				if logger != nil {
					logger.Debug("getHistorySize failed", "session_name", sessionName, "error", err.Error())
				}
			}
		}
		return -1
	}
	var size int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &size)
	if err != nil {
		m.mu.RLock()
		logger := m.logger
		m.mu.RUnlock()
		if logger != nil {
			logger.Debug("getHistorySize parse failed",
				"session_name", sessionName,
				"output", strings.TrimSpace(string(output)),
				"error", err.Error())
		}
		return -1
	}
	return size
}

// captureVisiblePane captures only the visible pane content (no scrollback history).
// This is much faster than capturing the full scrollback buffer.
func (m *Manager) captureVisiblePane(sessionName string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "tmux", "capture-pane", "-t", sessionName, "-p", "-e").Output()
}

// captureFullPane captures the full pane content including scrollback history.
func (m *Manager) captureFullPane(sessionName string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "tmux", "capture-pane", "-t", sessionName, "-p", "-e", "-S", "-", "-E", "-").Output()
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

// checkAndForwardBellWithMonitor checks for terminal bells using either the state monitor
// or internal tracking, depending on whether a monitor is configured.
func (m *Manager) checkAndForwardBellWithMonitor(sessionName string, stateMonitor *state.Monitor, instanceID string) {
	// Query the window_bell_flag from tmux
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	bellCmd := exec.CommandContext(ctx, "tmux", "display-message", "-t", sessionName, "-p", "#{window_bell_flag}")
	output, err := bellCmd.Output()
	if err != nil {
		return
	}

	bellActive := strings.TrimSpace(string(output)) == "1"

	if stateMonitor != nil {
		// Use external state monitor for bell detection
		stateMonitor.CheckBell(instanceID, bellActive)
	} else {
		// Fall back to internal bell detection
		m.mu.Lock()
		lastBellState := m.lastBellState
		callback := m.bellCallback
		logger := m.logger
		m.lastBellState = bellActive
		m.mu.Unlock()

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
	newMetrics, err := m.metricsParser.Parse(output)
	if err != nil || newMetrics == nil {
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

	// Close the input handler to release persistent tmux connection
	if m.inputHandler != nil {
		_ = m.inputHandler.Close()
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

	// Unregister from external state monitor if configured
	if m.stateMonitor != nil {
		m.stateMonitor.Stop(m.id)
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
	running := m.running
	sessionName := m.sessionName
	handler := m.inputHandler
	m.mu.RUnlock()

	if !running {
		return
	}

	// Delegate to InputHandler for encoding and sending
	_ = handler.SendInput(sessionName, string(data))
}

// SendKey sends a special key to the tmux session
func (m *Manager) SendKey(key string) {
	m.mu.RLock()
	running := m.running
	sessionName := m.sessionName
	handler := m.inputHandler
	m.mu.RUnlock()

	if !running {
		return
	}

	// Delegate to InputHandler (already async)
	_ = handler.SendKey(sessionName, key)
}

// SendLiteral sends literal text to the tmux session (no interpretation)
func (m *Manager) SendLiteral(text string) {
	m.mu.RLock()
	running := m.running
	sessionName := m.sessionName
	handler := m.inputHandler
	m.mu.RUnlock()

	if !running {
		return
	}

	// Delegate to InputHandler (already async)
	_ = handler.SendLiteral(sessionName, text)
}

// SendPaste sends pasted text to the tmux session with bracketed paste sequences
// This preserves the paste context for applications that support bracketed paste mode
func (m *Manager) SendPaste(text string) {
	m.mu.RLock()
	running := m.running
	sessionName := m.sessionName
	handler := m.inputHandler
	m.mu.RUnlock()

	if !running {
		return
	}

	// Delegate to InputHandler (already async with bracketed paste)
	_ = handler.SendPaste(sessionName, text)
}

// InputHandler returns the input handler for this manager.
// This allows access to input history and buffering features.
func (m *Manager) InputHandler() *input.Handler {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.inputHandler
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
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "display-message", "-t", m.sessionName, "-p", "#{pane_pid}")
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
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "has-session", "-t", m.sessionName)
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
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	_ = exec.CommandContext(ctx, "tmux", "set-option", "-t", m.sessionName, "-w", "monitor-bell", "on").Run()
	cancel()

	m.running = true
	m.paused = false
	m.timedOut = false
	m.repeatedOutputCount = 0
	m.lastActivityTime = time.Now()
	m.lastBellState = false // Reset bell state on reconnect
	m.doneChan = make(chan struct{})

	// Register with external state monitor if configured
	if m.stateMonitor != nil {
		m.stateMonitor.Start(m.id)
	}

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
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "list-sessions", "-F", "#{session_name}")
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

// SetLifecycleManager sets an optional lifecycle manager for coordinated operations.
// When set, the Manager can participate in lifecycle operations coordinated by the
// LifecycleManager. This is optional for backward compatibility.
func (m *Manager) SetLifecycleManager(lm *lifecycle.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lifecycleManager = lm
}

// LifecycleManager returns the configured lifecycle manager, if any.
func (m *Manager) LifecycleManager() *lifecycle.Manager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lifecycleManager
}

// WorkDir returns the working directory for this instance.
// Implements lifecycle.Instance interface.
func (m *Manager) WorkDir() string {
	return m.workdir
}

// Task returns the task/prompt to execute.
// Implements lifecycle.Instance interface.
func (m *Manager) Task() string {
	return m.task
}

// LifecycleConfig returns the configuration needed for lifecycle operations.
// Implements lifecycle.Instance interface.
func (m *Manager) LifecycleConfig() lifecycle.InstanceConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return lifecycle.InstanceConfig{
		TmuxWidth:        m.config.TmuxWidth,
		TmuxHeight:       m.config.TmuxHeight,
		TmuxHistoryLimit: m.config.TmuxHistoryLimit,
	}
}

// Config returns the lifecycle instance configuration.
// Implements lifecycle.Instance interface.
func (m *Manager) Config() lifecycle.InstanceConfig {
	return m.LifecycleConfig()
}

// SetRunning sets the running state of the instance.
// Implements lifecycle.Instance interface.
func (m *Manager) SetRunning(running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = running
}

// IsRunning returns whether the instance is currently running.
// Implements lifecycle.Instance interface.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// SetStartTime sets the start time of the instance.
// Implements lifecycle.Instance interface.
func (m *Manager) SetStartTime(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startTime = &t
}

// OnStarted is called when the instance has been started by the lifecycle manager.
// Implements lifecycle.Instance interface.
func (m *Manager) OnStarted() {
	m.mu.Lock()
	m.paused = false
	m.timedOut = false
	m.repeatedOutputCount = 0
	m.lastActivityTime = time.Now()

	// Start background goroutine to capture output periodically
	m.doneChan = make(chan struct{})
	m.captureTick = time.NewTicker(time.Duration(m.config.CaptureIntervalMs) * time.Millisecond)
	m.mu.Unlock()

	go m.captureLoop()
}

// OnStopped is called when the instance has been stopped by the lifecycle manager.
// Implements lifecycle.Instance interface.
func (m *Manager) OnStopped() {
	m.mu.Lock()
	defer m.mu.Unlock()

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
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	resizeCmd := exec.CommandContext(ctx, "tmux",
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

// Verify Manager implements lifecycle.Instance at compile time.
var _ lifecycle.Instance = (*Manager)(nil)
