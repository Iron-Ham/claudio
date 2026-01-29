package tripleshot

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// Session represents a triple-shot orchestration session
type Session struct {
	ID                  string      `json:"id"`
	GroupID             string      `json:"group_id,omitempty"`              // Link to InstanceGroup for multi-tripleshot support
	ImplementersGroupID string      `json:"implementers_group_id,omitempty"` // ID of the Implementers sub-group (for TUI collapse)
	Task                string      `json:"task"`                            // The original task/problem
	Phase               Phase       `json:"phase"`
	Config              Config      `json:"config"`
	Attempts            [3]Attempt  `json:"attempts"`           // Exactly 3 attempts
	JudgeID             string      `json:"judge_id,omitempty"` // Instance ID of the judge
	Created             time.Time   `json:"created"`
	StartedAt           *time.Time  `json:"started_at,omitempty"`
	CompletedAt         *time.Time  `json:"completed_at,omitempty"`
	Evaluation          *Evaluation `json:"evaluation,omitempty"` // Judge's evaluation
	Error               string      `json:"error,omitempty"`      // Error message if failed
}

// generateID creates a unique identifier for the session
func generateID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// NewSession creates a new triple-shot session
func NewSession(task string, config Config) *Session {
	return &Session{
		ID:      generateID(),
		Task:    task,
		Phase:   PhaseWorking,
		Config:  config,
		Created: time.Now(),
	}
}

// AllAttemptsComplete returns true if all three attempts have completed (success or failure)
// Note: AttemptStatusUnderReview is NOT considered complete - the attempt must either
// pass review (AttemptStatusCompleted) or fail review (AttemptStatusFailed).
func (s *Session) AllAttemptsComplete() bool {
	for _, attempt := range s.Attempts {
		switch attempt.Status {
		case AttemptStatusPending, AttemptStatusWorking, AttemptStatusUnderReview:
			return false
		}
	}
	return true
}

// SuccessfulAttemptCount returns the number of attempts that completed successfully
func (s *Session) SuccessfulAttemptCount() int {
	count := 0
	for _, attempt := range s.Attempts {
		if attempt.Status == AttemptStatusCompleted {
			count++
		}
	}
	return count
}

// Manager manages the execution of a triple-shot session
type Manager struct {
	session *Session
	logger  *logging.Logger

	// Event handling
	eventCallback func(Event)

	// Synchronization
	mu sync.RWMutex
}

// NewManager creates a new triple-shot manager
func NewManager(tripleSession *Session, logger *logging.Logger) *Manager {
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &Manager{
		session: tripleSession,
		logger:  logger.WithPhase("tripleshot"),
	}
}

// SetEventCallback sets the callback for triple-shot events
func (m *Manager) SetEventCallback(cb func(Event)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventCallback = cb
}

// Session returns the triple-shot session
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

// MarkAttemptComplete marks an attempt as completed
func (m *Manager) MarkAttemptComplete(attemptIndex int) {
	m.mu.Lock()
	if attemptIndex >= 0 && attemptIndex < 3 {
		m.session.Attempts[attemptIndex].Status = AttemptStatusCompleted
		now := time.Now()
		m.session.Attempts[attemptIndex].CompletedAt = &now
	}
	m.mu.Unlock()

	m.logger.Info("attempt completed", "attempt_index", attemptIndex)

	m.emitEvent(Event{
		Type:         EventAttemptComplete,
		AttemptIndex: attemptIndex,
	})
}

// MarkAttemptFailed marks an attempt as failed
func (m *Manager) MarkAttemptFailed(attemptIndex int, reason string) {
	m.mu.Lock()
	if attemptIndex >= 0 && attemptIndex < 3 {
		m.session.Attempts[attemptIndex].Status = AttemptStatusFailed
		now := time.Now()
		m.session.Attempts[attemptIndex].CompletedAt = &now
	}
	m.mu.Unlock()

	m.logger.Info("attempt failed", "attempt_index", attemptIndex, "reason", reason)

	m.emitEvent(Event{
		Type:         EventAttemptFailed,
		AttemptIndex: attemptIndex,
		Message:      reason,
	})
}

// SetEvaluation sets the evaluation on the session
func (m *Manager) SetEvaluation(eval *Evaluation) {
	m.mu.Lock()
	m.session.Evaluation = eval
	m.mu.Unlock()

	m.logger.Info("evaluation set",
		"winner_index", eval.WinnerIndex,
		"strategy", eval.MergeStrategy,
	)

	m.emitEvent(Event{
		Type:    EventEvaluationReady,
		Message: eval.Reasoning,
	})
}

// EmitAllAttemptsReady emits the all attempts ready event
func (m *Manager) EmitAllAttemptsReady() {
	m.emitEvent(Event{
		Type:    EventAllAttemptsReady,
		Message: "All attempts complete, ready for evaluation",
	})
}
