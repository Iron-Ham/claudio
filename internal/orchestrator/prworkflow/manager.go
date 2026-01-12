// Package prworkflow provides PR workflow management for the orchestrator.
// It extracts PR workflow coordination from the main Orchestrator to maintain
// single responsibility and enable easier testing.
package prworkflow

import (
	"maps"
	"sync"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/logging"
)

// InstanceInfo provides the minimal information about an instance needed
// for PR workflow management. This interface decouples the Manager from
// the full orchestrator.Instance type.
type InstanceInfo interface {
	GetID() string
	GetWorktreePath() string
	GetBranch() string
	GetTask() string
}

// Config holds configuration for the PR workflow manager.
type Config struct {
	// UseAI enables AI-assisted PR creation via Claude
	UseAI bool
	// Draft creates PRs as drafts
	Draft bool
	// AutoRebase enables automatic rebasing before PR creation
	AutoRebase bool
	// TmuxWidth is the default tmux window width
	TmuxWidth int
	// TmuxHeight is the default tmux window height
	TmuxHeight int
}

// NewConfigFromConfig creates a PR workflow Config from the global config.
func NewConfigFromConfig(cfg *config.Config) Config {
	return Config{
		UseAI:      cfg.PR.UseAI,
		Draft:      cfg.PR.Draft,
		AutoRebase: cfg.PR.AutoRebase,
		TmuxWidth:  cfg.Instance.TmuxWidth,
		TmuxHeight: cfg.Instance.TmuxHeight,
	}
}

// Manager coordinates PR workflows for instances.
// It handles starting, tracking, and completing PR workflows, and notifies
// interested parties via callbacks and events.
type Manager struct {
	config    Config
	sessionID string // Claudio session ID for multi-session support
	eventBus  *event.Bus
	logger    *logging.Logger

	// Display dimensions (can be updated when TUI resizes)
	displayWidth  int
	displayHeight int

	// Callbacks for PR workflow events
	completeCallback func(instanceID string, success bool)
	openedCallback   func(instanceID string)

	mu        sync.RWMutex
	workflows map[string]*instance.PRWorkflow
}

// NewManager creates a new PR workflow manager.
func NewManager(cfg Config, sessionID string, eventBus *event.Bus) *Manager {
	return &Manager{
		config:    cfg,
		sessionID: sessionID,
		eventBus:  eventBus,
		workflows: make(map[string]*instance.PRWorkflow),
	}
}

// SetLogger sets the logger for the manager.
func (m *Manager) SetLogger(logger *logging.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger = logger
}

// SetDisplayDimensions sets the display dimensions for new PR workflows.
// This should be called when the TUI window resizes.
func (m *Manager) SetDisplayDimensions(width, height int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.displayWidth = width
	m.displayHeight = height
}

// SetCompleteCallback sets the callback invoked when a PR workflow completes.
func (m *Manager) SetCompleteCallback(cb func(instanceID string, success bool)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeCallback = cb
}

// SetOpenedCallback sets the callback invoked when a PR is opened.
func (m *Manager) SetOpenedCallback(cb func(instanceID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.openedCallback = cb
}

// Start begins a PR workflow for the given instance.
func (m *Manager) Start(inst InstanceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Build workflow configuration
	cfg := instance.PRWorkflowConfig{
		UseAI:      m.config.UseAI,
		Draft:      m.config.Draft,
		AutoRebase: m.config.AutoRebase,
		TmuxWidth:  m.displayWidth,
		TmuxHeight: m.displayHeight,
	}

	// Use config defaults if display dimensions not set
	if cfg.TmuxWidth == 0 {
		cfg.TmuxWidth = m.config.TmuxWidth
	}
	if cfg.TmuxHeight == 0 {
		cfg.TmuxHeight = m.config.TmuxHeight
	}

	// Create workflow with session-scoped naming if in multi-session mode
	var workflow *instance.PRWorkflow
	if m.sessionID != "" {
		workflow = instance.NewPRWorkflowWithSession(
			m.sessionID,
			inst.GetID(),
			inst.GetWorktreePath(),
			inst.GetBranch(),
			inst.GetTask(),
			cfg,
		)
	} else {
		workflow = instance.NewPRWorkflow(
			inst.GetID(),
			inst.GetWorktreePath(),
			inst.GetBranch(),
			inst.GetTask(),
			cfg,
		)
	}

	// Set logger if available
	if m.logger != nil {
		workflow.SetLogger(m.logger)
	}

	// Set completion callback
	workflow.SetCallback(m.handleComplete)

	// Start the workflow
	if err := workflow.Start(); err != nil {
		return err
	}

	m.workflows[inst.GetID()] = workflow
	return nil
}

// handleComplete handles PR workflow completion.
func (m *Manager) handleComplete(instanceID string, success bool, output string) {
	m.mu.Lock()
	// Clean up workflow
	delete(m.workflows, instanceID)

	// Get callbacks before unlocking
	completeCallback := m.completeCallback
	eventBus := m.eventBus
	m.mu.Unlock()

	// Publish event to event bus if available
	if eventBus != nil {
		eventBus.Publish(event.NewPRCompleteEvent(instanceID, success, "", ""))
	}

	// Notify via callback if set (for backwards compatibility)
	if completeCallback != nil {
		completeCallback(instanceID, success)
	}
}

// HandleComplete allows external completion handling (e.g., for testing or manual completion).
// In normal operation, completion is handled automatically via the workflow callback.
func (m *Manager) HandleComplete(instanceID string, success bool, output string) {
	m.handleComplete(instanceID, success, output)
}

// Get returns the PR workflow for an instance, or nil if none exists.
func (m *Manager) Get(instanceID string) *instance.PRWorkflow {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.workflows[instanceID]
}

// Stop terminates a PR workflow for the given instance.
func (m *Manager) Stop(instanceID string) error {
	m.mu.Lock()
	workflow, ok := m.workflows[instanceID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	delete(m.workflows, instanceID)
	m.mu.Unlock()

	return workflow.Stop()
}

// StopAll terminates all running PR workflows.
func (m *Manager) StopAll() {
	m.mu.Lock()
	workflows := make(map[string]*instance.PRWorkflow, len(m.workflows))
	maps.Copy(workflows, m.workflows)
	m.workflows = make(map[string]*instance.PRWorkflow)
	m.mu.Unlock()

	for _, workflow := range workflows {
		_ = workflow.Stop()
	}
}

// Running returns true if there's an active PR workflow for the instance.
func (m *Manager) Running(instanceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	workflow, ok := m.workflows[instanceID]
	return ok && workflow != nil && workflow.Running()
}

// Count returns the number of active PR workflows.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workflows)
}

// IDs returns the instance IDs of all active PR workflows.
func (m *Manager) IDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.workflows))
	for id := range m.workflows {
		ids = append(ids, id)
	}
	return ids
}
