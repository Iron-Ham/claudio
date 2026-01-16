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
	"github.com/Iron-Ham/claudio/internal/tmux"
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

// pausedHeartbeatInterval is the number of ticks between session existence checks
// when an instance is paused. This allows detecting completion of background instances
// without the overhead of full capture. At 100ms per tick, 50 ticks = 5 seconds.
const pausedHeartbeatInterval = 50

// tmuxCommandTimeout is the maximum time to wait for tmux subprocess commands.
// This prevents the capture loop from hanging indefinitely if tmux becomes unresponsive.
const tmuxCommandTimeout = 2 * time.Second

// String returns a human-readable name for a timeout type
func (t TimeoutType) String() string {
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

// ManagerOptions holds explicit dependencies for creating a Manager.
// Use NewManagerWithDeps to create a Manager with these options.
type ManagerOptions struct {
	ID               string
	SessionID        string
	WorkDir          string
	Task             string
	Config           ManagerConfig
	StateMonitor     *state.Monitor     // Optional - if nil, an internal monitor is created
	LifecycleManager *lifecycle.Manager // Optional - if set, delegates Start/Stop/Reconnect
	ClaudeSessionID  string             // Optional - Claude's internal session UUID for resume capability
}

// Manager handles a single Claude Code instance running in a tmux session.
// The Manager is a facade that delegates state tracking to its StateMonitor.
type Manager struct {
	id              string
	sessionID       string // Claudio session ID (for multi-session support)
	workdir         string
	task            string
	sessionName     string // tmux session name
	socketName      string // tmux socket name for isolation (claudio-{instanceID})
	claudeSessionID string // Claude's internal session UUID for resume capability
	outputBuf       *capture.RingBuffer
	mu              sync.RWMutex
	running         bool
	paused          bool
	doneChan        chan struct{}
	captureTick     *time.Ticker
	config          ManagerConfig

	// State detection - delegated to stateMonitor
	stateCallback StateChangeCallback

	// Metrics tracking
	metricsParser   *metrics.MetricsParser
	currentMetrics  *metrics.ParsedMetrics
	metricsCallback MetricsChangeCallback
	startTime       *time.Time

	// Timeout tracking - delegated to stateMonitor
	timeoutCallback TimeoutCallback

	// Differential capture optimization
	lastHistorySize    int  // Last captured history size (for differential capture)
	fullRefreshCounter int  // Counter for periodic full refresh
	forceFullCapture   bool // Force full capture on next tick (set when visible content changes)

	// Paused heartbeat - tracks ticks while paused to do periodic session checks
	pausedHeartbeatCounter int

	// Bell tracking - delegated to stateMonitor
	bellCallback BellCallback

	// Input handling
	inputHandler *input.Handler

	// Logger for structured logging
	logger *logging.Logger

	// lifecycleManager is an optional lifecycle manager for coordinated lifecycle operations.
	// When set, certain operations may delegate to this manager for better separation of concerns.
	lifecycleManager *lifecycle.Manager

	// stateMonitor handles centralized state tracking (state detection, timeouts, bells).
	// This is always set - either provided explicitly or created internally.
	stateMonitor *state.Monitor
}

// NewManagerWithDeps creates a new instance manager with explicit dependencies.
// This is the preferred constructor for new code, enabling proper dependency injection.
//
// The StateMonitor handles all state detection, timeout checking, and bell detection.
// If not provided, an internal monitor is created. Providing a shared monitor allows
// multiple managers to share state tracking for coordinated management.
func NewManagerWithDeps(opts ManagerOptions) *Manager {
	// Determine session name based on session ID
	var sessionName string
	if opts.SessionID != "" {
		sessionName = fmt.Sprintf("claudio-%s-%s", opts.SessionID, opts.ID)
	} else {
		sessionName = fmt.Sprintf("claudio-%s", opts.ID)
	}

	// Merge with defaults for any unset fields
	cfg := opts.Config
	defaults := DefaultManagerConfig()
	if cfg.OutputBufferSize == 0 {
		cfg.OutputBufferSize = defaults.OutputBufferSize
	}
	if cfg.CaptureIntervalMs == 0 {
		cfg.CaptureIntervalMs = defaults.CaptureIntervalMs
	}
	if cfg.TmuxWidth == 0 {
		cfg.TmuxWidth = defaults.TmuxWidth
	}
	if cfg.TmuxHeight == 0 {
		cfg.TmuxHeight = defaults.TmuxHeight
	}
	if cfg.TmuxHistoryLimit == 0 {
		cfg.TmuxHistoryLimit = defaults.TmuxHistoryLimit
	}

	// Use provided StateMonitor or create an internal one
	monitor := opts.StateMonitor
	if monitor == nil {
		monitorCfg := state.MonitorConfig{
			ActivityTimeoutMinutes:   cfg.ActivityTimeoutMinutes,
			CompletionTimeoutMinutes: cfg.CompletionTimeoutMinutes,
			StaleDetection:           cfg.StaleDetection,
		}
		monitor = state.NewMonitor(monitorCfg)
	}

	// Each instance gets its own tmux socket for crash isolation
	socketName := tmux.InstanceSocketName(opts.ID)

	return &Manager{
		id:              opts.ID,
		sessionID:       opts.SessionID,
		workdir:         opts.WorkDir,
		task:            opts.Task,
		sessionName:     sessionName,
		socketName:      socketName,
		claudeSessionID: opts.ClaudeSessionID,
		outputBuf:       capture.NewRingBuffer(cfg.OutputBufferSize),
		doneChan:        make(chan struct{}),
		config:          cfg,
		metricsParser:   metrics.NewMetricsParser(),
		inputHandler: input.NewHandler(
			input.WithPersistentSender(sessionName, socketName),
			input.WithBatching(sessionName, input.DefaultBatchConfig()),
		),
		stateMonitor:     monitor,
		lifecycleManager: opts.LifecycleManager,
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

// StateMonitor returns the configured state monitor.
// A state monitor is always set - never returns nil.
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

// CurrentState returns the currently detected waiting state.
// Delegates to the StateMonitor for centralized state tracking.
func (m *Manager) CurrentState() detect.WaitingState {
	return m.stateMonitor.GetState(m.id)
}

// TimedOut returns whether the instance has timed out and the type of timeout.
// Delegates to the StateMonitor for centralized timeout tracking.
func (m *Manager) TimedOut() (bool, TimeoutType) {
	timedOut, timeoutType := m.stateMonitor.GetTimedOut(m.id)
	// Convert state.TimeoutType to instance.TimeoutType
	return timedOut, TimeoutType(timeoutType)
}

// LastActivityTime returns when the instance last had output activity.
// Delegates to the StateMonitor for centralized activity tracking.
func (m *Manager) LastActivityTime() time.Time {
	return m.stateMonitor.GetLastActivityTime(m.id)
}

// ClearTimeout resets the timeout state (for recovery/restart scenarios).
// Delegates to the StateMonitor.
func (m *Manager) ClearTimeout() {
	m.stateMonitor.ClearTimeout(m.id)
}

// Start launches the Claude Code process in a tmux session.
// If a LifecycleManager is configured, delegates to it for tmux session management.
func (m *Manager) Start() error {
	// Delegate to lifecycle manager if available
	if m.lifecycleManager != nil {
		return m.lifecycleManager.Start(m)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("instance already running")
	}

	// Kill any existing session with this name (cleanup from previous run)
	_ = m.tmuxCmd("kill-session", "-t", m.sessionName).Run()

	// Determine history limit from config (default to 50000 if not set)
	historyLimit := m.config.TmuxHistoryLimit
	if historyLimit == 0 {
		historyLimit = 50000
	}

	// Set history-limit BEFORE creating session so the new pane inherits it.
	// tmux's history-limit only affects newly created panes, not existing ones.
	if err := m.tmuxCmd("set-option", "-g", "history-limit", fmt.Sprintf("%d", historyLimit)).Run(); err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to set global history-limit for tmux",
				"history_limit", historyLimit,
				"error", err.Error())
		}
	}

	// Create a new detached tmux session with color support
	createCmd := m.tmuxCmd(
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
	_ = m.tmuxCmd("set-option", "-t", m.sessionName, "default-terminal", "xterm-256color").Run()
	// Enable bell monitoring so we can detect and forward terminal bells
	_ = m.tmuxCmd("set-option", "-t", m.sessionName, "-w", "monitor-bell", "on").Run()

	// Write the task/prompt to a temporary file to avoid shell escaping issues
	// (prompts with <, >, |, etc. would otherwise be interpreted by the shell)
	promptFile := filepath.Join(m.workdir, ".claude-prompt")
	if err := os.WriteFile(promptFile, []byte(m.task), 0600); err != nil {
		_ = m.tmuxCmd("kill-session", "-t", m.sessionName).Run()
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	// Build the claude command with optional session-id for resume capability
	// If claudeSessionID is set, use it to enable later resumption with --resume
	var claudeCmd string
	if m.claudeSessionID != "" {
		claudeCmd = fmt.Sprintf("claude --dangerously-skip-permissions --session-id %q \"$(cat %q)\" && rm %q",
			m.claudeSessionID, promptFile, promptFile)
	} else {
		claudeCmd = fmt.Sprintf("claude --dangerously-skip-permissions \"$(cat %q)\" && rm %q",
			promptFile, promptFile)
	}

	sendCmd := m.tmuxCmd(
		"send-keys",
		"-t", m.sessionName,
		claudeCmd,
		"Enter",
	)
	if err := sendCmd.Run(); err != nil {
		// Clean up the session if we failed to start claude
		_ = m.tmuxCmd("kill-session", "-t", m.sessionName).Run()
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

	// Record start time for duration tracking
	now := time.Now()
	m.startTime = &now

	// Register with state monitor for state/timeout/bell tracking
	m.stateMonitor.Start(m.id)

	// Start background goroutine to capture output periodically
	m.captureTick = time.NewTicker(time.Duration(m.config.CaptureIntervalMs) * time.Millisecond)
	go m.captureLoop()

	if m.logger != nil {
		m.logger.Info("tmux session created",
			"session_name", m.sessionName,
			"workdir", m.workdir,
			"claude_session_id", m.claudeSessionID)
	}

	return nil
}

// StartWithResume launches Claude Code with --resume to continue a previous session.
// This requires a valid ClaudeSessionID to be set (either via ManagerOptions or SetClaudeSessionID).
// The resumed session will continue from where it left off.
func (m *Manager) StartWithResume() error {
	// Delegate to lifecycle manager if available
	if m.lifecycleManager != nil {
		return m.lifecycleManager.Start(m)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("instance already running")
	}

	if m.claudeSessionID == "" {
		return fmt.Errorf("cannot resume: no Claude session ID set")
	}

	// Kill any existing tmux session with this name (cleanup from previous run)
	_ = m.tmuxCmd("kill-session", "-t", m.sessionName).Run()

	// Determine history limit from config (default to 50000 if not set)
	historyLimit := m.config.TmuxHistoryLimit
	if historyLimit == 0 {
		historyLimit = 50000
	}

	// Set history-limit BEFORE creating session so the new pane inherits it.
	if err := m.tmuxCmd("set-option", "-g", "history-limit", fmt.Sprintf("%d", historyLimit)).Run(); err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to set global history-limit for tmux",
				"history_limit", historyLimit,
				"error", err.Error())
		}
	}

	// Create a new detached tmux session with color support
	createCmd := m.tmuxCmd(
		"new-session",
		"-d",                // detached
		"-s", m.sessionName, // session name
		"-x", fmt.Sprintf("%d", m.config.TmuxWidth), // width
		"-y", fmt.Sprintf("%d", m.config.TmuxHeight), // height
	)
	createCmd.Dir = m.workdir
	createCmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if err := createCmd.Run(); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to create tmux session for resume",
				"session_name", m.sessionName,
				"workdir", m.workdir,
				"error", err.Error())
		}
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Set up additional tmux session options
	_ = m.tmuxCmd("set-option", "-t", m.sessionName, "default-terminal", "xterm-256color").Run()
	_ = m.tmuxCmd("set-option", "-t", m.sessionName, "-w", "monitor-bell", "on").Run()

	// Build the claude command with --resume to continue the previous session
	claudeCmd := fmt.Sprintf("claude --dangerously-skip-permissions --resume %q", m.claudeSessionID)

	sendCmd := m.tmuxCmd(
		"send-keys",
		"-t", m.sessionName,
		claudeCmd,
		"Enter",
	)
	if err := sendCmd.Run(); err != nil {
		_ = m.tmuxCmd("kill-session", "-t", m.sessionName).Run()
		if m.logger != nil {
			m.logger.Error("failed to start claude with resume in tmux session",
				"session_name", m.sessionName,
				"claude_session_id", m.claudeSessionID,
				"error", err.Error())
		}
		return fmt.Errorf("failed to start claude with resume: %w", err)
	}

	m.running = true
	m.paused = false

	// Record start time for duration tracking
	now := time.Now()
	m.startTime = &now

	// Register with state monitor for state/timeout/bell tracking
	m.stateMonitor.Start(m.id)

	// Start background goroutine to capture output periodically
	m.captureTick = time.NewTicker(time.Duration(m.config.CaptureIntervalMs) * time.Millisecond)
	go m.captureLoop()

	if m.logger != nil {
		m.logger.Info("tmux session created with resume",
			"session_name", m.sessionName,
			"workdir", m.workdir,
			"claude_session_id", m.claudeSessionID)
	}

	return nil
}

// captureLoop periodically captures output from the tmux session.
// All state detection, timeout checking, and bell detection is delegated to stateMonitor.
//
// IMPORTANT: Output capture continues even after timeouts (activity, completion, or stale)
// are detected. This ensures the TUI always displays up-to-date output. Only state detection
// and timeout checking are skipped after a timeout, not the actual capture.
func (m *Manager) captureLoop() {
	// Track last output hash to detect changes
	var lastOutput string

	for {
		select {
		case <-m.doneChan:
			return
		case <-m.captureTick.C:
			m.mu.RLock()
			running := m.running
			paused := m.paused
			sessionName := m.sessionName
			instanceID := m.id
			m.mu.RUnlock()

			if !running {
				continue
			}

			// When paused, do lightweight heartbeat checks to detect completion
			if paused {
				m.mu.Lock()
				m.pausedHeartbeatCounter++
				doHeartbeat := m.pausedHeartbeatCounter >= pausedHeartbeatInterval
				if doHeartbeat {
					m.pausedHeartbeatCounter = 0
				}
				m.mu.Unlock()

				if doHeartbeat {
					// Lightweight session check - only verify session exists
					if !m.checkSessionExists(sessionName) {
						// Session ended while paused - notify completion
						m.mu.Lock()
						m.running = false
						callback := m.stateCallback
						m.mu.Unlock()

						m.stateMonitor.SetState(instanceID, detect.StateCompleted)
						m.stateMonitor.Stop(instanceID)

						if callback != nil {
							callback(instanceID, detect.StateCompleted)
						}
						return
					}
				}
				continue
			}

			// Check if already timed out - we still capture output for display,
			// but skip state detection and timeout checking to avoid redundant work.
			timedOut, _ := m.stateMonitor.GetTimedOut(instanceID)

			// Batched tmux query: get history_size, bell_flag, and session existence
			// in a single subprocess call to reduce overhead (was 4 calls, now 2).
			status := m.getSessionStatus(sessionName)

			// Check if session ended
			if !status.sessionExists {
				// Session actually ended - notify completion and stop
				m.mu.Lock()
				m.running = false
				callback := m.stateCallback
				m.mu.Unlock()

				// Notify state monitor about completion
				m.stateMonitor.SetState(instanceID, detect.StateCompleted)
				m.stateMonitor.Stop(instanceID)

				// Fire the completion callback so coordinator knows task is done
				if callback != nil {
					callback(instanceID, detect.StateCompleted)
				}
				return
			}

			// When status query fails (historySize == -1) but session still exists,
			// we can't use differential capture optimization but MUST still capture
			// output to prevent frozen displays. Force a full capture in this case.
			statusQueryFailed := status.historySize == -1

			// Differential capture optimization:
			// - Check history size to detect new output
			// - Capture only visible pane when nothing changed (much faster)
			// - Do full capture when history grows or periodically (every 50 ticks = 5 seconds)
			// - Always do full capture when status query failed (can't determine changes)
			m.mu.Lock()
			lastHistorySize := m.lastHistorySize
			m.fullRefreshCounter++
			// Do full capture when:
			// 1. Status query failed (can't do differential capture without history_size)
			// 2. Periodic refresh interval reached (every 5 seconds)
			// 3. History size increased (new scrollback lines)
			// 4. Visible content changed in previous tick (user typing)
			doFullCapture := statusQueryFailed ||
				m.fullRefreshCounter >= fullRefreshInterval ||
				status.historySize > lastHistorySize ||
				m.forceFullCapture
			if doFullCapture {
				m.fullRefreshCounter = 0
				// Only update lastHistorySize when we have a valid value.
				// When status query failed (historySize == -1), preserve the previous value.
				if status.historySize >= 0 {
					m.lastHistorySize = status.historySize
				}
				m.forceFullCapture = false
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

			// Check if content changed
			currentOutput := string(output)
			if currentOutput != lastOutput {
				m.mu.RLock()
				logger := m.logger
				m.mu.RUnlock()

				if doFullCapture {
					// Only update the output buffer with full captures to preserve scrollback.
					// Visible-only captures don't include scrollback history, so writing them
					// to the buffer would cause the output to flash between short (visible) and
					// long (full) content, breaking scroll position.
					byteCount := len(output)
					// Use ReplaceWith for atomic reset+write to prevent race condition where
					// concurrent GetOutput() calls could see an empty buffer between Reset and Write.
					m.outputBuf.ReplaceWith(output)

					if logger != nil {
						logger.Debug("output captured",
							"byte_count", byteCount)
					}
				} else {
					// Visible-only capture detected content change (e.g., user typing).
					// Schedule a full capture on the next tick to update the output buffer.
					// Do NOT write visible-only content to the buffer - it lacks scrollback
					// and would cause display flashing.
					m.mu.Lock()
					m.forceFullCapture = true
					m.mu.Unlock()

					if logger != nil {
						logger.Debug("visible content changed, scheduling full capture")
					}
				}

				lastOutput = currentOutput
			}

			// Skip state detection and timeout checks if already timed out.
			// We still captured output above so the display stays up-to-date,
			// but there's no need to re-detect state or check timeouts again.
			if !timedOut {
				// Process output through state monitor for state detection and activity tracking.
				// Always call this even if output hasn't changed (for stale detection).
				m.stateMonitor.ProcessOutput(instanceID, output, currentOutput)

				// Parse metrics from output (separate from state detection)
				m.parseAndNotifyMetrics(output)

				// Check for timeout conditions
				m.stateMonitor.CheckTimeouts(instanceID)

				// Forward bell state from batched query
				m.stateMonitor.CheckBell(instanceID, status.bellActive)
			}
		}
	}
}

// sessionStatus holds the result of a batched tmux query for session state.
// Using a single tmux command to query multiple values reduces subprocess overhead.
type sessionStatus struct {
	historySize   int  // Current scrollback line count, -1 if unknown
	bellActive    bool // Whether window_bell_flag is set
	sessionExists bool // Whether the session is still running
}

// parseSessionStatusOutput parses the "historySize|bellFlag" output from tmux.
// Returns the parsed values. If parsing fails, historySize will be -1.
// This function is separated for testability.
func parseSessionStatusOutput(output string) (historySize int, bellActive bool, ok bool) {
	parts := strings.Split(strings.TrimSpace(output), "|")
	if len(parts) != 2 {
		return -1, false, false
	}

	historySize = -1
	if _, err := fmt.Sscanf(parts[0], "%d", &historySize); err != nil {
		// Leave historySize as -1 on parse error
		historySize = -1
	}

	bellActive = parts[1] == "1"
	return historySize, bellActive, true
}

// getSessionStatus queries multiple tmux values in a single command to reduce subprocess overhead.
// This batches history_size and window_bell_flag into one display-message call.
// If the command fails with a known "session not found" error, sessionExists is false.
// For unknown errors or transient failures, sessionExists is true to allow retry.
func (m *Manager) getSessionStatus(sessionName string) sessionStatus {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()

	// Batch query: history_size|window_bell_flag
	// Using pipe as delimiter since it won't appear in these numeric values
	cmd := m.tmuxCmdCtx(ctx, "display-message", "-t", sessionName, "-p", "#{history_size}|#{window_bell_flag}")
	output, err := cmd.Output()
	if err != nil {
		// Check if this was a timeout - session may still exist
		if ctx.Err() == context.DeadlineExceeded {
			m.mu.RLock()
			logger := m.logger
			m.mu.RUnlock()
			if logger != nil {
				logger.Warn("tmux display-message timed out, will retry next tick",
					"session_name", sessionName)
			}
			return sessionStatus{historySize: -1, bellActive: false, sessionExists: true}
		}

		// Any non-timeout error means session doesn't exist or can't be verified
		return sessionStatus{historySize: -1, bellActive: false, sessionExists: false}
	}

	// Parse the response using the extracted helper
	historySize, bellActive, ok := parseSessionStatusOutput(string(output))
	if !ok {
		m.mu.RLock()
		logger := m.logger
		m.mu.RUnlock()
		if logger != nil {
			logger.Debug("getSessionStatus parse failed: unexpected format",
				"session_name", sessionName,
				"output", strings.TrimSpace(string(output)))
		}
		return sessionStatus{historySize: -1, bellActive: false, sessionExists: true}
	}

	// Log if history size parsing failed (but format was valid)
	if historySize == -1 {
		m.mu.RLock()
		logger := m.logger
		m.mu.RUnlock()
		if logger != nil {
			logger.Debug("getSessionStatus: history_size parse failed",
				"session_name", sessionName,
				"output", strings.TrimSpace(string(output)))
		}
	}

	return sessionStatus{
		historySize:   historySize,
		bellActive:    bellActive,
		sessionExists: true,
	}
}

// checkSessionExists performs a lightweight check to verify the tmux session exists.
// This is used for heartbeat checks on paused instances - it avoids the overhead
// of querying history_size and bell_flag when we only care about session existence.
func (m *Manager) checkSessionExists(sessionName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()

	cmd := m.tmuxCmdCtx(ctx, "has-session", "-t", sessionName)
	err := cmd.Run()

	// If timeout, assume session still exists (retry on next heartbeat)
	if ctx.Err() == context.DeadlineExceeded {
		m.mu.RLock()
		logger := m.logger
		m.mu.RUnlock()
		if logger != nil {
			logger.Warn("tmux has-session timed out during heartbeat, will retry",
				"session_name", sessionName)
		}
		return true
	}

	if err != nil {
		// Any error means session doesn't exist or can't be verified
		// This is consistent with TmuxSessionExists() which uses `cmd.Run() == nil`
		return false
	}

	return true
}

// captureVisiblePane captures only the visible pane content (no scrollback history).
// This is much faster than capturing the full scrollback buffer.
func (m *Manager) captureVisiblePane(sessionName string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	return m.tmuxCmdCtx(ctx, "capture-pane", "-t", sessionName, "-p", "-e").Output()
}

// captureFullPane captures the full pane content including scrollback history.
func (m *Manager) captureFullPane(sessionName string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	return m.tmuxCmdCtx(ctx, "capture-pane", "-t", sessionName, "-p", "-e", "-S", "-", "-E", "-").Output()
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
	// Delegate to lifecycle manager if available
	if m.lifecycleManager != nil {
		// Close input handler before delegating (Manager-specific cleanup)
		if m.inputHandler != nil {
			_ = m.inputHandler.Close()
		}
		return m.lifecycleManager.Stop(m)
	}

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
	_ = m.tmuxCmd("send-keys", "-t", m.sessionName, "C-c").Run()
	time.Sleep(500 * time.Millisecond)

	// Kill the tmux session
	if err := m.tmuxCmd("kill-session", "-t", m.sessionName).Run(); err != nil {
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

	// Unregister from state monitor
	m.stateMonitor.Stop(m.id)

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
	m.pausedHeartbeatCounter = 0 // Reset so next pause starts fresh
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

// SocketName returns the tmux socket name for this instance.
// Each instance uses its own socket for crash isolation.
func (m *Manager) SocketName() string {
	return m.socketName
}

// ID returns the instance ID
func (m *Manager) ID() string {
	return m.id
}

// ClaudeSessionID returns the Claude session UUID used for resume capability.
// This is the UUID passed to `claude --session-id` when starting the instance.
func (m *Manager) ClaudeSessionID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.claudeSessionID
}

// SetClaudeSessionID sets the Claude session UUID for resume capability.
// This should be set before Start() or StartWithResume() is called.
func (m *Manager) SetClaudeSessionID(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.claudeSessionID = sessionID
}

// tmuxCmd creates a tmux command using this instance's isolated socket.
func (m *Manager) tmuxCmd(args ...string) *exec.Cmd {
	return tmux.CommandWithSocket(m.socketName, args...)
}

// tmuxCmdCtx creates a context-aware tmux command using this instance's isolated socket.
func (m *Manager) tmuxCmdCtx(ctx context.Context, args ...string) *exec.Cmd {
	return tmux.CommandContextWithSocket(ctx, m.socketName, args...)
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
	cmd := m.tmuxCmdCtx(ctx, "display-message", "-t", m.sessionName, "-p", "#{pane_pid}")
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
	return fmt.Sprintf("tmux -L %s attach -t %s", m.socketName, m.sessionName)
}

// TmuxSessionExists checks if the tmux session for this instance exists
func (m *Manager) TmuxSessionExists() bool {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	cmd := m.tmuxCmdCtx(ctx, "has-session", "-t", m.sessionName)
	return cmd.Run() == nil
}

// Reconnect attempts to reconnect to an existing tmux session
// This is used for session recovery after a restart
func (m *Manager) Reconnect() error {
	// Delegate to lifecycle manager if available
	if m.lifecycleManager != nil {
		return m.lifecycleManager.Reconnect(m)
	}

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
	_ = m.tmuxCmdCtx(ctx, "set-option", "-t", m.sessionName, "-w", "monitor-bell", "on").Run()
	cancel()

	m.running = true
	m.paused = false
	m.doneChan = make(chan struct{})

	// Register with state monitor for state/timeout/bell tracking
	m.stateMonitor.Start(m.id)

	// Start background goroutine to capture output periodically
	m.captureTick = time.NewTicker(time.Duration(m.config.CaptureIntervalMs) * time.Millisecond)
	go m.captureLoop()

	if m.logger != nil {
		m.logger.Info("instance reconnected",
			"session_name", m.sessionName)
	}

	return nil
}

// TmuxSessionInfo contains information about a tmux session including its socket.
type TmuxSessionInfo struct {
	SessionName string
	SocketName  string
}

// KillCommand returns an exec.Cmd that will kill this tmux session.
// This encapsulates the socket-aware kill-session command construction.
func (t TmuxSessionInfo) KillCommand() *exec.Cmd {
	return tmux.CommandWithSocket(t.SocketName, "kill-session", "-t", t.SessionName)
}

// ListClaudioTmuxSessions returns a list of all tmux session names with the claudio- prefix.
// This aggregates sessions from all claudio-* sockets.
func ListClaudioTmuxSessions() ([]string, error) {
	infos, err := ListClaudioTmuxSessionsWithSocket()
	if err != nil {
		return nil, err
	}
	sessions := make([]string, len(infos))
	for i, info := range infos {
		sessions[i] = info.SessionName
	}
	return sessions, nil
}

// ListClaudioTmuxSessionsWithSocket returns session information including socket names.
// This is needed for cleanup operations that must target specific sockets.
func ListClaudioTmuxSessionsWithSocket() ([]TmuxSessionInfo, error) {
	// Find all claudio sockets
	sockets, err := tmux.ListClaudioSockets()
	if err != nil {
		return nil, err
	}

	// Collect sessions from all sockets
	var allSessions []TmuxSessionInfo
	for _, socketName := range sockets {
		ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
		cmd := tmux.CommandContextWithSocket(ctx, socketName, "list-sessions", "-F", "#{session_name}")
		output, err := cmd.Output()
		cancel()
		if err != nil {
			// No sessions on this socket, skip
			continue
		}

		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if strings.HasPrefix(line, "claudio-") {
				allSessions = append(allSessions, TmuxSessionInfo{
					SessionName: line,
					SocketName:  socketName,
				})
			}
		}
	}
	return allSessions, nil
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

	// Start background goroutine to capture output periodically
	m.doneChan = make(chan struct{})
	m.captureTick = time.NewTicker(time.Duration(m.config.CaptureIntervalMs) * time.Millisecond)
	instanceID := m.id
	m.mu.Unlock()

	// Register with state monitor for state/timeout/bell tracking
	m.stateMonitor.Start(instanceID)

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
	resizeCmd := m.tmuxCmdCtx(ctx,
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
