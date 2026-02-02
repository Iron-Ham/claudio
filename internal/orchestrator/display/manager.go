// Package display provides display dimension management for the orchestrator.
//
// This package encapsulates the tracking of terminal display dimensions
// and coordinating resize events across all running backend instances.
package display

import (
	"sync"
)

// ResizeObserver is an interface for components that need to be notified of resize events.
// This is implemented by instance.Manager to receive size change notifications.
type ResizeObserver interface {
	// Running returns true if the observer is currently active and should receive resize events.
	Running() bool

	// Resize updates the observer to the new dimensions.
	Resize(width, height int) error
}

// Manager manages display dimensions for tmux sessions.
//
// It tracks the current display dimensions and notifies registered observers
// (typically running backend instances) when the dimensions change.
type Manager struct {
	mu            sync.RWMutex
	width         int
	height        int
	defaultWidth  int
	defaultHeight int
	observers     []ResizeObserver
}

// Config holds configuration options for the display Manager.
type Config struct {
	// DefaultWidth is the default terminal width when no explicit dimensions are set.
	DefaultWidth int

	// DefaultHeight is the default terminal height when no explicit dimensions are set.
	DefaultHeight int
}

// DefaultConfig returns sensible defaults for display configuration.
func DefaultConfig() Config {
	return Config{
		DefaultWidth:  200,
		DefaultHeight: 30,
	}
}

// NewManager creates a new display Manager with the given configuration.
func NewManager(config Config) *Manager {
	return &Manager{
		defaultWidth:  config.DefaultWidth,
		defaultHeight: config.DefaultHeight,
	}
}

// SetDimensions sets the current display dimensions.
// This should be called to set initial dimensions before starting instances.
// It does NOT notify observers - use NotifyResize for that.
func (m *Manager) SetDimensions(width, height int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.width = width
	m.height = height
}

// GetDimensions returns the current display dimensions.
// If dimensions have not been explicitly set, returns the default dimensions.
func (m *Manager) GetDimensions() (width, height int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	width = m.width
	height = m.height

	// Fall back to defaults if not set
	if width == 0 {
		width = m.defaultWidth
	}
	if height == 0 {
		height = m.defaultHeight
	}

	return width, height
}

// NotifyResize updates the stored dimensions and notifies all registered observers.
// This is typically called when the TUI window is resized.
func (m *Manager) NotifyResize(width, height int) {
	m.mu.Lock()
	m.width = width
	m.height = height
	observers := make([]ResizeObserver, len(m.observers))
	copy(observers, m.observers)
	m.mu.Unlock()

	// Notify observers outside the lock to avoid potential deadlocks
	for _, obs := range observers {
		if obs != nil && obs.Running() {
			// Errors are intentionally ignored since resize is best-effort
			// and we don't want one failed resize to affect others
			_ = obs.Resize(width, height)
		}
	}
}

// AddObserver registers an observer to receive resize notifications.
func (m *Manager) AddObserver(obs ResizeObserver) {
	if obs == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.observers = append(m.observers, obs)
}

// RemoveObserver unregisters an observer from resize notifications.
func (m *Manager) RemoveObserver(obs ResizeObserver) {
	if obs == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for i, o := range m.observers {
		if o == obs {
			// Remove by replacing with last element and truncating
			m.observers[i] = m.observers[len(m.observers)-1]
			m.observers = m.observers[:len(m.observers)-1]
			return
		}
	}
}

// ObserverCount returns the number of registered observers.
// This is primarily useful for testing.
func (m *Manager) ObserverCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.observers)
}
