package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/ai"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/tmux"
)

// Common errors returned by LifecycleManager operations.
var (
	// ErrAlreadyRunning is returned when Start is called on an already running instance.
	ErrAlreadyRunning = errors.New("instance already running")

	// ErrNotRunning is returned when an operation requires a running instance.
	ErrNotRunning = errors.New("instance not running")

	// ErrSessionNotFound is returned when reconnecting to a non-existent session.
	ErrSessionNotFound = errors.New("tmux session not found")

	// ErrReadyTimeout is returned when WaitForReady times out.
	ErrReadyTimeout = errors.New("timeout waiting for instance to be ready")

	// ErrInvalidInstance is returned when an operation is called with a nil instance.
	ErrInvalidInstance = errors.New("invalid instance: nil")
)

// State represents the lifecycle state of an instance.
type State int

const (
	// StateStopped indicates the instance is not running.
	StateStopped State = iota

	// StateStarting indicates the instance is being started.
	StateStarting

	// StateRunning indicates the instance is running but may not be ready.
	StateRunning

	// StateReady indicates the instance is running and ready to accept input.
	StateReady

	// StateStopping indicates the instance is being stopped.
	StateStopping
)

// String returns a human-readable string for the state.
func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateReady:
		return "ready"
	case StateStopping:
		return "stopping"
	default:
		return "unknown"
	}
}

// Instance defines the interface for an instance that can be managed by LifecycleManager.
// This interface allows the lifecycle manager to interact with instances without
// depending on the concrete instance implementation.
type Instance interface {
	// ID returns the unique identifier for this instance.
	ID() string

	// SessionName returns the tmux session name for this instance.
	SessionName() string

	// SocketName returns the tmux socket name for this instance.
	// Each instance uses a unique socket for crash isolation.
	SocketName() string

	// WorkDir returns the working directory for this instance.
	WorkDir() string

	// Task returns the task/prompt to execute.
	Task() string

	// Config returns the instance configuration.
	Config() InstanceConfig

	// SetRunning sets the running state of the instance.
	SetRunning(running bool)

	// IsRunning returns whether the instance is currently running.
	IsRunning() bool

	// SetStartTime sets the start time of the instance.
	SetStartTime(t time.Time)

	// OnStarted is called when the instance has been started successfully.
	// This allows the instance to perform any post-start initialization.
	OnStarted()

	// OnStopped is called when the instance has been stopped.
	// This allows the instance to perform any cleanup.
	OnStopped()
}

// InstanceConfig holds configuration needed for lifecycle operations.
type InstanceConfig struct {
	// TmuxWidth is the terminal width in columns.
	TmuxWidth int

	// TmuxHeight is the terminal height in rows.
	TmuxHeight int

	// TmuxHistoryLimit is the number of lines of scrollback to keep (default: 50000).
	TmuxHistoryLimit int
}

// ReadinessChecker is a function that checks if an instance is ready.
// Returns true if the instance is ready, false otherwise.
type ReadinessChecker func(inst Instance) bool

// Manager handles the lifecycle of AI backend instances.
// It manages starting, stopping, and restarting instances in tmux sessions.
type Manager struct {
	logger  *logging.Logger
	mu      sync.Mutex
	backend ai.Backend

	// instanceStates tracks the lifecycle state of each instance by ID.
	instanceStates map[string]State

	// readinessChecker is an optional custom readiness check function.
	// If nil, a default check based on tmux session existence is used.
	readinessChecker ReadinessChecker

	// gracefulStopTimeout is how long to wait after sending Ctrl+C before force killing.
	gracefulStopTimeout time.Duration
}

// NewManager creates a new lifecycle manager.
func NewManager(logger *logging.Logger) *Manager {
	return &Manager{
		logger:              logger,
		backend:             ai.DefaultBackend(),
		instanceStates:      make(map[string]State),
		gracefulStopTimeout: 500 * time.Millisecond,
	}
}

// SetBackend sets the AI backend for lifecycle operations.
func (m *Manager) SetBackend(backend ai.Backend) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if backend == nil {
		m.backend = ai.DefaultBackend()
		return
	}
	m.backend = backend
}

// SetReadinessChecker sets a custom readiness check function.
// This allows callers to define application-specific readiness criteria.
func (m *Manager) SetReadinessChecker(checker ReadinessChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readinessChecker = checker
}

// SetGracefulStopTimeout sets the timeout for graceful stop operations.
func (m *Manager) SetGracefulStopTimeout(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gracefulStopTimeout = timeout
}

// GetState returns the current lifecycle state of an instance.
func (m *Manager) GetState(inst Instance) State {
	if inst == nil {
		return StateStopped
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.instanceStates[inst.ID()]
}

// Start launches an AI instance in a tmux session.
// It creates a new tmux session, configures it, and starts the backend command.
func (m *Manager) Start(inst Instance) error {
	if inst == nil {
		return ErrInvalidInstance
	}

	m.mu.Lock()
	currentState := m.instanceStates[inst.ID()]
	if currentState == StateRunning || currentState == StateReady || currentState == StateStarting {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}
	m.instanceStates[inst.ID()] = StateStarting
	m.mu.Unlock()

	sessionName := inst.SessionName()
	socketName := inst.SocketName()
	workDir := inst.WorkDir()
	task := inst.Task()
	cfg := inst.Config()

	// Kill any existing session with this name (cleanup from previous run)
	_ = tmux.CommandWithSocket(socketName, "kill-session", "-t", sessionName).Run()

	// Determine terminal dimensions
	width := cfg.TmuxWidth
	if width == 0 {
		width = 200
	}
	height := cfg.TmuxHeight
	if height == 0 {
		height = 30
	}
	historyLimit := cfg.TmuxHistoryLimit
	if historyLimit == 0 {
		historyLimit = 50000
	}

	// Set history-limit BEFORE creating session so the new pane inherits it.
	// tmux's history-limit only affects newly created panes, not existing ones.
	// Using the instance socket ensures this applies to the instance's isolated tmux server.
	if err := tmux.CommandWithSocket(socketName, "set-option", "-g", "history-limit", fmt.Sprintf("%d", historyLimit)).Run(); err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to set global history-limit for tmux",
				"history_limit", historyLimit,
				"error", err.Error())
		}
	}

	// Create a new detached tmux session with color support
	createCmd := tmux.CommandWithSocket(
		socketName,
		"new-session",
		"-d",
		"-s", sessionName,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
	)
	createCmd.Dir = workDir
	createCmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if err := createCmd.Run(); err != nil {
		m.setStateStopped(inst)
		if m.logger != nil {
			m.logger.Error("failed to create tmux session",
				"session_name", sessionName,
				"workdir", workDir,
				"error", err.Error())
		}
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Set up additional tmux session options for color support
	_ = tmux.CommandWithSocket(socketName, "set-option", "-t", sessionName, "default-terminal", "xterm-256color").Run()
	// Enable bell monitoring for detecting terminal bells
	_ = tmux.CommandWithSocket(socketName, "set-option", "-t", sessionName, "-w", "monitor-bell", "on").Run()

	// Write the task/prompt to a temporary file to avoid shell escaping issues
	promptFile := filepath.Join(workDir, m.backend.PromptFileName())
	if err := os.WriteFile(promptFile, []byte(task), 0600); err != nil {
		_ = tmux.CommandWithSocket(socketName, "kill-session", "-t", sessionName).Run()
		m.setStateStopped(inst)
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	// Send the backend command to the tmux session, reading prompt from file
	backendCmd, err := m.backend.BuildStartCommand(ai.StartOptions{
		PromptFile: promptFile,
		Mode:       ai.StartModeInteractive,
	})
	if err != nil {
		_ = tmux.CommandWithSocket(socketName, "kill-session", "-t", sessionName).Run()
		_ = os.Remove(promptFile)
		m.setStateStopped(inst)
		return fmt.Errorf("failed to build backend command: %w", err)
	}
	sendCmd := tmux.CommandWithSocket(
		socketName,
		"send-keys",
		"-t", sessionName,
		backendCmd,
		"Enter",
	)
	if err := sendCmd.Run(); err != nil {
		_ = tmux.CommandWithSocket(socketName, "kill-session", "-t", sessionName).Run()
		_ = os.Remove(promptFile)
		m.setStateStopped(inst)
		if m.logger != nil {
			m.logger.Error("failed to start backend in tmux session",
				"session_name", sessionName,
				"error", err.Error())
		}
		return fmt.Errorf("failed to start backend in tmux session: %w", err)
	}

	// Update instance and state
	inst.SetRunning(true)
	inst.SetStartTime(time.Now())

	m.mu.Lock()
	m.instanceStates[inst.ID()] = StateRunning
	m.mu.Unlock()

	// Notify instance that it has been started
	inst.OnStarted()

	if m.logger != nil {
		m.logger.Info("instance started",
			"instance_id", inst.ID(),
			"session_name", sessionName,
			"workdir", workDir)
	}

	return nil
}

// Stop terminates a running instance.
// It sends Ctrl+C for graceful shutdown, then kills the tmux session.
func (m *Manager) Stop(inst Instance) error {
	if inst == nil {
		return ErrInvalidInstance
	}

	m.mu.Lock()
	currentState := m.instanceStates[inst.ID()]
	if currentState == StateStopped || currentState == StateStopping {
		m.mu.Unlock()
		return nil
	}
	m.instanceStates[inst.ID()] = StateStopping
	gracefulTimeout := m.gracefulStopTimeout
	m.mu.Unlock()

	sessionName := inst.SessionName()
	socketName := inst.SocketName()

	// Send Ctrl+C to gracefully stop the backend session first
	_ = tmux.CommandWithSocket(socketName, "send-keys", "-t", sessionName, "C-c").Run()
	time.Sleep(gracefulTimeout)

	// Kill the tmux session
	if err := tmux.CommandWithSocket(socketName, "kill-session", "-t", sessionName).Run(); err != nil {
		// Log but don't fail - session may have already exited
		if m.logger != nil {
			m.logger.Debug("tmux kill-session error (may be expected)",
				"session_name", sessionName,
				"error", err.Error())
		}
	}

	// Update instance state
	inst.SetRunning(false)
	m.setStateStopped(inst)

	// Notify instance that it has been stopped
	inst.OnStopped()

	if m.logger != nil {
		m.logger.Info("instance stopped",
			"instance_id", inst.ID(),
			"session_name", sessionName)
	}

	return nil
}

// Restart stops and then starts an instance.
// If the instance is not running, it simply starts it.
func (m *Manager) Restart(inst Instance) error {
	if inst == nil {
		return ErrInvalidInstance
	}

	m.mu.Lock()
	currentState := m.instanceStates[inst.ID()]
	m.mu.Unlock()

	// Stop if currently running
	if currentState == StateRunning || currentState == StateReady {
		if err := m.Stop(inst); err != nil {
			return fmt.Errorf("failed to stop instance for restart: %w", err)
		}
	}

	// Start the instance
	if err := m.Start(inst); err != nil {
		return fmt.Errorf("failed to start instance for restart: %w", err)
	}

	if m.logger != nil {
		m.logger.Info("instance restarted",
			"instance_id", inst.ID())
	}

	return nil
}

// WaitForReady waits for an instance to become ready within the given timeout.
// It polls the instance's readiness status until ready or timeout.
func (m *Manager) WaitForReady(inst Instance, timeout time.Duration) error {
	if inst == nil {
		return ErrInvalidInstance
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Poll interval starts small and backs off
	pollInterval := 50 * time.Millisecond
	maxPollInterval := 500 * time.Millisecond

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ErrReadyTimeout
		case <-ticker.C:
			if m.isReady(inst) {
				m.mu.Lock()
				m.instanceStates[inst.ID()] = StateReady
				m.mu.Unlock()

				if m.logger != nil {
					m.logger.Debug("instance ready",
						"instance_id", inst.ID())
				}
				return nil
			}

			// Check if instance stopped unexpectedly
			if !inst.IsRunning() {
				return ErrNotRunning
			}

			// Back off the poll interval
			if pollInterval < maxPollInterval {
				pollInterval *= 2
				if pollInterval > maxPollInterval {
					pollInterval = maxPollInterval
				}
				ticker.Reset(pollInterval)
			}
		}
	}
}

// Reconnect attempts to reconnect to an existing tmux session.
// This is used for session recovery after a restart.
func (m *Manager) Reconnect(inst Instance) error {
	if inst == nil {
		return ErrInvalidInstance
	}

	m.mu.Lock()
	currentState := m.instanceStates[inst.ID()]
	if currentState == StateRunning || currentState == StateReady {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}
	m.mu.Unlock()

	sessionName := inst.SessionName()
	socketName := inst.SocketName()

	// Check if the tmux session exists
	if !m.SessionExistsWithSocket(sessionName, socketName) {
		return ErrSessionNotFound
	}

	// Ensure monitor-bell is enabled
	_ = tmux.CommandWithSocket(socketName, "set-option", "-t", sessionName, "-w", "monitor-bell", "on").Run()

	// Update instance state
	inst.SetRunning(true)

	m.mu.Lock()
	m.instanceStates[inst.ID()] = StateRunning
	m.mu.Unlock()

	// Notify instance that it has been started (reconnected)
	inst.OnStarted()

	if m.logger != nil {
		m.logger.Info("instance reconnected",
			"instance_id", inst.ID(),
			"session_name", sessionName)
	}

	return nil
}

// SessionExistsWithSocket checks if a tmux session exists on a specific socket.
func (m *Manager) SessionExistsWithSocket(sessionName, socketName string) bool {
	cmd := tmux.CommandWithSocket(socketName, "has-session", "-t", sessionName)
	return cmd.Run() == nil
}

// isReady checks if an instance is ready using the configured readiness checker.
func (m *Manager) isReady(inst Instance) bool {
	m.mu.Lock()
	checker := m.readinessChecker
	m.mu.Unlock()

	if checker != nil {
		return checker(inst)
	}

	// Default readiness check: tmux session exists and instance is marked as running
	return inst.IsRunning() && m.SessionExistsWithSocket(inst.SessionName(), inst.SocketName())
}

// setStateStopped sets the instance state to stopped.
func (m *Manager) setStateStopped(inst Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instanceStates[inst.ID()] = StateStopped
}

// ClearState removes the state tracking for an instance.
// This should be called when an instance is being destroyed.
func (m *Manager) ClearState(inst Instance) {
	if inst == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.instanceStates, inst.ID())
}
