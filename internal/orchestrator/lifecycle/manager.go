// Package lifecycle provides instance lifecycle management for the orchestrator.
//
// This package encapsulates the creation, starting, stopping, and monitoring
// of Claude Code instances within an orchestration session.
package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// InstanceStatus represents the current status of an instance.
type InstanceStatus string

const (
	StatusPending      InstanceStatus = "pending"
	StatusWorking      InstanceStatus = "working"
	StatusWaitingInput InstanceStatus = "waiting_input"
	StatusPaused       InstanceStatus = "paused"
	StatusCompleted    InstanceStatus = "completed"
	StatusError        InstanceStatus = "error"
	StatusCreatingPR   InstanceStatus = "creating_pr"
	StatusStuck        InstanceStatus = "stuck"
	StatusTimeout      InstanceStatus = "timeout"
)

// Instance represents a managed Claude Code instance.
type Instance struct {
	ID            string         `json:"id"`
	WorktreePath  string         `json:"worktree_path"`
	Branch        string         `json:"branch"`
	Task          string         `json:"task"`
	Status        InstanceStatus `json:"status"`
	PID           int            `json:"pid,omitempty"`
	FilesModified []string       `json:"files_modified,omitempty"`
	TmuxSession   string         `json:"tmux_session"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
}

// Callbacks holds callback functions for instance lifecycle events.
type Callbacks struct {
	// OnStatusChange is called when an instance's status changes.
	OnStatusChange func(instanceID string, oldStatus, newStatus InstanceStatus)

	// OnPRComplete is called when an instance completes creating a PR.
	OnPRComplete func(instanceID, prURL string)

	// OnTimeout is called when an instance times out.
	OnTimeout func(instanceID string)

	// OnBell is called when an instance triggers a terminal bell.
	OnBell func(instanceID string)

	// OnError is called when an instance encounters an error.
	OnError func(instanceID string, err error)
}

// Config holds configuration for the lifecycle manager.
type Config struct {
	// TmuxSessionPrefix is the prefix for tmux session names.
	TmuxSessionPrefix string

	// DefaultTermWidth is the default terminal width.
	DefaultTermWidth int

	// DefaultTermHeight is the default terminal height.
	DefaultTermHeight int

	// ActivityTimeout is how long to wait for activity before timing out.
	ActivityTimeout time.Duration

	// CompletionTimeout is how long to wait for completion detection.
	CompletionTimeout time.Duration
}

// DefaultConfig returns sensible defaults for lifecycle configuration.
func DefaultConfig() Config {
	return Config{
		TmuxSessionPrefix: "claudio",
		DefaultTermWidth:  200,
		DefaultTermHeight: 30,
		ActivityTimeout:   5 * time.Minute,
		CompletionTimeout: 30 * time.Second,
	}
}

// Manager manages the lifecycle of Claude Code instances.
type Manager struct {
	config    Config
	callbacks Callbacks
	logger    *logging.Logger

	mu        sync.RWMutex
	instances map[string]*Instance
	stopChan  chan struct{}
	stopped   bool
	wg        sync.WaitGroup
}

// NewManager creates a new lifecycle manager.
func NewManager(config Config, callbacks Callbacks, logger *logging.Logger) *Manager {
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &Manager{
		config:    config,
		callbacks: callbacks,
		logger:    logger.WithPhase("lifecycle"),
		instances: make(map[string]*Instance),
		stopChan:  make(chan struct{}),
	}
}

// CreateInstance creates a new instance but does not start it.
func (m *Manager) CreateInstance(id, worktreePath, branch, task string) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[id]; exists {
		return nil, fmt.Errorf("instance %s already exists", id)
	}

	inst := &Instance{
		ID:           id,
		WorktreePath: worktreePath,
		Branch:       branch,
		Task:         task,
		Status:       StatusPending,
		TmuxSession:  fmt.Sprintf("%s-%s", m.config.TmuxSessionPrefix, id),
	}

	m.instances[id] = inst

	m.logger.Info("instance created",
		"instance_id", id,
		"worktree", worktreePath,
		"branch", branch,
	)

	return inst, nil
}

// StartInstance starts a previously created instance.
func (m *Manager) StartInstance(ctx context.Context, id string) error {
	m.mu.Lock()
	inst, exists := m.instances[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("instance %s not found", id)
	}
	if inst.Status != StatusPending {
		m.mu.Unlock()
		return fmt.Errorf("instance %s is not in pending status", id)
	}
	m.mu.Unlock()

	// Start the instance (implementation depends on process management)
	// This is a placeholder - actual implementation would use process.TmuxProcess
	now := time.Now()

	m.mu.Lock()
	inst.Status = StatusWorking
	inst.StartedAt = &now
	m.mu.Unlock()

	if m.callbacks.OnStatusChange != nil {
		m.callbacks.OnStatusChange(id, StatusPending, StatusWorking)
	}

	m.logger.Info("instance started",
		"instance_id", id,
	)

	return nil
}

// StopInstance stops a running instance.
func (m *Manager) StopInstance(id string) error {
	m.mu.Lock()
	inst, exists := m.instances[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("instance %s not found", id)
	}
	oldStatus := inst.Status
	m.mu.Unlock()

	// Stop the instance (implementation depends on process management)
	// This is a placeholder - actual implementation would stop the tmux session

	now := time.Now()

	m.mu.Lock()
	inst.Status = StatusCompleted
	inst.CompletedAt = &now
	m.mu.Unlock()

	if m.callbacks.OnStatusChange != nil {
		m.callbacks.OnStatusChange(id, oldStatus, StatusCompleted)
	}

	m.logger.Info("instance stopped",
		"instance_id", id,
	)

	return nil
}

// GetInstance returns an instance by ID.
func (m *Manager) GetInstance(id string) (*Instance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, exists := m.instances[id]
	if !exists {
		return nil, false
	}
	// Return a copy
	instCopy := *inst
	return &instCopy, true
}

// ListInstances returns all instances.
func (m *Manager) ListInstances() []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		instCopy := *inst
		result = append(result, &instCopy)
	}
	return result
}

// UpdateStatus updates an instance's status.
func (m *Manager) UpdateStatus(id string, status InstanceStatus) error {
	m.mu.Lock()
	inst, exists := m.instances[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("instance %s not found", id)
	}
	oldStatus := inst.Status
	inst.Status = status

	if status == StatusCompleted || status == StatusError || status == StatusTimeout {
		now := time.Now()
		inst.CompletedAt = &now
	}
	m.mu.Unlock()

	if m.callbacks.OnStatusChange != nil && oldStatus != status {
		m.callbacks.OnStatusChange(id, oldStatus, status)
	}

	m.logger.Debug("instance status updated",
		"instance_id", id,
		"old_status", string(oldStatus),
		"new_status", string(status),
	)

	return nil
}

// RemoveInstance removes an instance from management.
func (m *Manager) RemoveInstance(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[id]; !exists {
		return fmt.Errorf("instance %s not found", id)
	}

	delete(m.instances, id)

	m.logger.Info("instance removed",
		"instance_id", id,
	)

	return nil
}

// GetRunningCount returns the number of currently running instances.
func (m *Manager) GetRunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, inst := range m.instances {
		if inst.Status == StatusWorking || inst.Status == StatusWaitingInput {
			count++
		}
	}
	return count
}

// Stop stops the lifecycle manager and all running instances.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	close(m.stopChan)

	// Collect IDs to stop while holding lock to avoid race condition
	var toStop []string
	for id, inst := range m.instances {
		if inst.Status == StatusWorking || inst.Status == StatusWaitingInput {
			toStop = append(toStop, id)
		}
	}
	m.mu.Unlock()

	// Stop all running instances (without holding lock)
	var stopErrors []error
	for _, id := range toStop {
		if err := m.StopInstance(id); err != nil {
			m.logger.Error("failed to stop instance during shutdown",
				"instance_id", id,
				"error", err.Error(),
			)
			stopErrors = append(stopErrors, err)
		}
	}

	if len(stopErrors) > 0 {
		m.logger.Warn("shutdown completed with errors",
			"failed_instances", len(stopErrors),
		)
	}

	m.wg.Wait()
}
