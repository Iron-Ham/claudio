package tui

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// Model holds the TUI application state
type Model struct {
	// Core components
	orchestrator *orchestrator.Orchestrator
	session      *orchestrator.Session

	// UI state
	activeTab    int
	width        int
	height       int
	ready        bool
	quitting     bool
	showHelp     bool
	addingTask   bool
	taskInput    string
	errorMessage string

	// Instance outputs (instance ID -> output string)
	outputs map[string]string
}

// NewModel creates a new TUI model
func NewModel(orch *orchestrator.Orchestrator, session *orchestrator.Session) Model {
	return Model{
		orchestrator: orch,
		session:      session,
		outputs:      make(map[string]string),
	}
}

// activeInstance returns the currently focused instance
func (m Model) activeInstance() *orchestrator.Instance {
	if m.session == nil || len(m.session.Instances) == 0 {
		return nil
	}

	if m.activeTab >= len(m.session.Instances) {
		return nil
	}

	return m.session.Instances[m.activeTab]
}

// instanceCount returns the number of instances
func (m Model) instanceCount() int {
	if m.session == nil {
		return 0
	}
	return len(m.session.Instances)
}
