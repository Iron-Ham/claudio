package adversarial

import (
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// Session represents an adversarial review orchestration session
type Session struct {
	ID            string     `json:"id"`
	GroupID       string     `json:"group_id,omitempty"` // Link to InstanceGroup for display
	Task          string     `json:"task"`               // The original task/problem
	Phase         Phase      `json:"phase"`
	Config        Config     `json:"config"`
	ImplementerID string     `json:"implementer_id,omitempty"` // Instance ID of implementer
	ReviewerID    string     `json:"reviewer_id,omitempty"`    // Instance ID of reviewer
	WorktreePath  string     `json:"worktree_path,omitempty"`  // Worktree path for implementer/reviewer
	CurrentRound  int        `json:"current_round"`            // Current implement-review cycle (1-based)
	Created       time.Time  `json:"created"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	Error         string     `json:"error,omitempty"`      // Error message if failed
	StuckRole     string     `json:"stuck_role,omitempty"` // Role that got stuck (implementer/reviewer)

	// History tracks all increments and reviews
	History []Round `json:"history,omitempty"`
}

// NewSession creates a new adversarial review session
func NewSession(id string, task string, config Config) *Session {
	return &Session{
		ID:           id,
		Task:         task,
		Phase:        PhaseImplementing,
		Config:       config,
		CurrentRound: 1,
		Created:      time.Now(),
		History:      make([]Round, 0),
	}
}

// IsActive returns true if the session is in an active (working) state
func (s *Session) IsActive() bool {
	switch s.Phase {
	case PhaseImplementing, PhaseReviewing:
		return true
	default:
		return false
	}
}

// Manager manages the execution of an adversarial review session
type Manager struct {
	session *Session
	logger  *logging.Logger

	// Event handling
	eventCallback func(Event)

	// Synchronization
	mu sync.RWMutex
}

// NewManager creates a new adversarial manager
func NewManager(advSession *Session, logger *logging.Logger) *Manager {
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &Manager{
		session: advSession,
		logger:  logger.WithPhase("adversarial"),
	}
}

// SetEventCallback sets the callback for adversarial events
func (m *Manager) SetEventCallback(cb func(Event)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventCallback = cb
}

// Session returns the adversarial session
func (m *Manager) Session() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session
}

// emitEvent sends an event to the callback
func (m *Manager) emitEvent(event Event) {
	event.Timestamp = time.Now()

	m.mu.RLock()
	cb := m.eventCallback
	m.mu.RUnlock()

	if cb != nil {
		cb(event)
	}
}

// SetPhase updates the session phase and emits an event
func (m *Manager) SetPhase(phase Phase) {
	m.mu.Lock()
	m.session.Phase = phase
	m.mu.Unlock()

	m.emitEvent(Event{
		Type:    EventPhaseChange,
		Message: string(phase),
	})
}

// StartRound begins a new implement-review round
func (m *Manager) StartRound() {
	m.mu.Lock()
	round := Round{
		Round:     m.session.CurrentRound,
		StartedAt: time.Now(),
	}
	m.session.History = append(m.session.History, round)
	m.mu.Unlock()

	m.logger.Info("round started", "round", m.session.CurrentRound)
}

// RecordIncrement records an increment submission for the current round
func (m *Manager) RecordIncrement(increment *IncrementFile) {
	m.mu.Lock()
	if len(m.session.History) > 0 {
		m.session.History[len(m.session.History)-1].Increment = increment
	}
	m.mu.Unlock()

	m.logger.Info("increment recorded",
		"round", increment.Round,
		"files_modified", len(increment.FilesModified),
	)

	m.emitEvent(Event{
		Type:    EventIncrementReady,
		Round:   increment.Round,
		Message: increment.Summary,
	})
}

// RecordReview records a review for the current round
func (m *Manager) RecordReview(review *ReviewFile) {
	m.mu.Lock()
	now := time.Now()
	if len(m.session.History) > 0 {
		m.session.History[len(m.session.History)-1].Review = review
		m.session.History[len(m.session.History)-1].ReviewedAt = &now
	}
	m.mu.Unlock()

	m.logger.Info("review recorded",
		"round", review.Round,
		"approved", review.Approved,
		"score", review.Score,
	)

	if review.Approved {
		m.emitEvent(Event{
			Type:    EventApproved,
			Round:   review.Round,
			Message: review.Summary,
		})
	} else {
		m.emitEvent(Event{
			Type:    EventRejected,
			Round:   review.Round,
			Message: fmt.Sprintf("Issues: %d, Score: %d/10", len(review.Issues), review.Score),
		})
	}
}

// NextRound advances to the next round
func (m *Manager) NextRound() {
	m.mu.Lock()
	m.session.CurrentRound++
	m.mu.Unlock()

	m.logger.Info("advancing to next round", "round", m.session.CurrentRound)
}

// IsMaxIterationsReached checks if the max iterations limit has been reached
func (m *Manager) IsMaxIterationsReached() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session.Config.MaxIterations == 0 {
		return false // No limit
	}
	return m.session.CurrentRound > m.session.Config.MaxIterations
}
