package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// RalphWiggumPhase represents the current phase of a Ralph Wiggum session
type RalphWiggumPhase string

const (
	// PhaseRalphWiggumIterating - instance is working through iterations
	PhaseRalphWiggumIterating RalphWiggumPhase = "iterating"
	// PhaseRalphWiggumComplete - completion promise was found
	PhaseRalphWiggumComplete RalphWiggumPhase = "complete"
	// PhaseRalphWiggumMaxIterations - maximum iterations reached without completion
	PhaseRalphWiggumMaxIterations RalphWiggumPhase = "max_iterations"
	// PhaseRalphWiggumFailed - something went wrong
	PhaseRalphWiggumFailed RalphWiggumPhase = "failed"
	// PhaseRalphWiggumPaused - iteration paused by user
	PhaseRalphWiggumPaused RalphWiggumPhase = "paused"
)

// RalphWiggumConfig holds configuration for a Ralph Wiggum session
type RalphWiggumConfig struct {
	// CompletionPromise is the text that signals task completion (e.g., "DONE", "COMPLETE")
	// The instance should output <promise>COMPLETION_TEXT</promise> when done
	CompletionPromise string `json:"completion_promise"`

	// MaxIterations is the maximum number of iterations before stopping (0 = unlimited)
	MaxIterations int `json:"max_iterations"`

	// AutoContinue determines whether to automatically continue after each iteration
	// When false, the user will be prompted between iterations
	AutoContinue bool `json:"auto_continue"`
}

// DefaultRalphWiggumConfig returns the default configuration
func DefaultRalphWiggumConfig() RalphWiggumConfig {
	return RalphWiggumConfig{
		CompletionPromise: "DONE",
		MaxIterations:     50,
		AutoContinue:      true,
	}
}

// RalphWiggumIteration represents a single iteration of the loop
type RalphWiggumIteration struct {
	Index       int        `json:"index"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	HasCommits  bool       `json:"has_commits"` // Whether this iteration produced commits
}

// RalphWiggumSession represents a Ralph Wiggum iterative loop session
type RalphWiggumSession struct {
	ID          string                 `json:"id"`
	GroupID     string                 `json:"group_id,omitempty"`
	Task        string                 `json:"task"`  // The original task/problem
	Phase       RalphWiggumPhase       `json:"phase"` // Current phase
	Config      RalphWiggumConfig      `json:"config"`
	InstanceID  string                 `json:"instance_id,omitempty"` // The single instance doing the work
	Created     time.Time              `json:"created"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Iterations  []RalphWiggumIteration `json:"iterations"` // Track each iteration
	Error       string                 `json:"error,omitempty"`
}

// NewRalphWiggumSession creates a new Ralph Wiggum session
func NewRalphWiggumSession(task string, config RalphWiggumConfig) *RalphWiggumSession {
	return &RalphWiggumSession{
		ID:         generateID(),
		Task:       task,
		Phase:      PhaseRalphWiggumIterating,
		Config:     config,
		Created:    time.Now(),
		Iterations: make([]RalphWiggumIteration, 0),
	}
}

// CurrentIteration returns the current iteration number (1-indexed for display)
func (s *RalphWiggumSession) CurrentIteration() int {
	return len(s.Iterations)
}

// IsComplete returns true if the session has completed (success or max iterations)
func (s *RalphWiggumSession) IsComplete() bool {
	return s.Phase == PhaseRalphWiggumComplete ||
		s.Phase == PhaseRalphWiggumMaxIterations ||
		s.Phase == PhaseRalphWiggumFailed
}

// ShouldContinue returns true if another iteration should be started
func (s *RalphWiggumSession) ShouldContinue() bool {
	if s.Phase != PhaseRalphWiggumIterating {
		return false
	}
	if s.Config.MaxIterations > 0 && len(s.Iterations) >= s.Config.MaxIterations {
		return false
	}
	return true
}

// RalphWiggumStatusFileName is the file that tracks iteration status
const RalphWiggumStatusFileName = ".claudio-ralph-status.json"

// RalphWiggumStatusFile represents the status file written/read during iterations
type RalphWiggumStatusFile struct {
	Iteration     int       `json:"iteration"`
	Phase         string    `json:"phase"`
	PromiseFound  bool      `json:"promise_found"`
	LastActivity  time.Time `json:"last_activity"`
	CommitCount   int       `json:"commit_count"`
	FilesModified []string  `json:"files_modified,omitempty"`
}

// RalphWiggumStatusFilePath returns the full path to the status file
func RalphWiggumStatusFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, RalphWiggumStatusFileName)
}

// ParseRalphWiggumStatusFile reads and parses the status file
func ParseRalphWiggumStatusFile(worktreePath string) (*RalphWiggumStatusFile, error) {
	statusPath := RalphWiggumStatusFilePath(worktreePath)
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, err
	}

	var status RalphWiggumStatusFile
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("failed to parse Ralph Wiggum status JSON: %w", err)
	}

	return &status, nil
}

// CheckOutputForCompletionPromise checks if the output contains the completion promise
func CheckOutputForCompletionPromise(output, promise string) bool {
	if promise == "" {
		return false
	}
	// Look for <promise>PROMISE_TEXT</promise> pattern
	pattern := fmt.Sprintf(`<promise>\s*%s\s*</promise>`, regexp.QuoteMeta(promise))
	re := regexp.MustCompile("(?i)" + pattern)
	return re.MatchString(output)
}

// ExtractPromiseFromOutput extracts any promise text from the output
func ExtractPromiseFromOutput(output string) string {
	re := regexp.MustCompile(`(?i)<promise>\s*(.*?)\s*</promise>`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// RalphWiggumManager manages the execution of a Ralph Wiggum session
type RalphWiggumManager struct {
	session     *RalphWiggumSession
	orch        *Orchestrator
	baseSession *Session
	logger      *logging.Logger

	// Event handling
	eventCallback func(RalphWiggumEvent)

	// Synchronization
	mu sync.RWMutex
}

// RalphWiggumEventType represents the type of Ralph Wiggum event
type RalphWiggumEventType string

const (
	EventRalphWiggumIterationStart    RalphWiggumEventType = "iteration_start"
	EventRalphWiggumIterationComplete RalphWiggumEventType = "iteration_complete"
	EventRalphWiggumPromiseFound      RalphWiggumEventType = "promise_found"
	EventRalphWiggumMaxIterations     RalphWiggumEventType = "max_iterations"
	EventRalphWiggumPhaseChange       RalphWiggumEventType = "phase_change"
	EventRalphWiggumError             RalphWiggumEventType = "error"
)

// RalphWiggumEvent represents an event from the Ralph Wiggum manager
type RalphWiggumEvent struct {
	Type       RalphWiggumEventType `json:"type"`
	Iteration  int                  `json:"iteration,omitempty"`
	InstanceID string               `json:"instance_id,omitempty"`
	Message    string               `json:"message,omitempty"`
	Timestamp  time.Time            `json:"timestamp"`
}

// NewRalphWiggumManager creates a new Ralph Wiggum manager
func NewRalphWiggumManager(orch *Orchestrator, baseSession *Session, ralphSession *RalphWiggumSession, logger *logging.Logger) *RalphWiggumManager {
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &RalphWiggumManager{
		session:     ralphSession,
		orch:        orch,
		baseSession: baseSession,
		logger:      logger.WithPhase("ralphwiggum"),
	}
}

// SetEventCallback sets the callback for Ralph Wiggum events
func (m *RalphWiggumManager) SetEventCallback(cb func(RalphWiggumEvent)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventCallback = cb
}

// Session returns the Ralph Wiggum session
func (m *RalphWiggumManager) Session() *RalphWiggumSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session
}

// emitEvent sends an event to the callback
func (m *RalphWiggumManager) emitEvent(event RalphWiggumEvent) {
	event.Timestamp = time.Now()

	m.mu.RLock()
	cb := m.eventCallback
	m.mu.RUnlock()

	if cb != nil {
		cb(event)
	}
}

// SetPhase updates the session phase and emits an event
func (m *RalphWiggumManager) SetPhase(phase RalphWiggumPhase) {
	m.mu.Lock()
	m.session.Phase = phase
	m.mu.Unlock()

	m.emitEvent(RalphWiggumEvent{
		Type:    EventRalphWiggumPhaseChange,
		Message: string(phase),
	})
}

// StartIteration marks the start of a new iteration
func (m *RalphWiggumManager) StartIteration() {
	m.mu.Lock()
	iteration := RalphWiggumIteration{
		Index:     len(m.session.Iterations),
		StartedAt: time.Now(),
	}
	m.session.Iterations = append(m.session.Iterations, iteration)
	iterationNum := len(m.session.Iterations)
	m.mu.Unlock()

	m.logger.Info("iteration started", "iteration", iterationNum)

	m.emitEvent(RalphWiggumEvent{
		Type:      EventRalphWiggumIterationStart,
		Iteration: iterationNum,
	})
}

// CompleteIteration marks the current iteration as complete
func (m *RalphWiggumManager) CompleteIteration(hasCommits bool) {
	m.mu.Lock()
	if len(m.session.Iterations) > 0 {
		idx := len(m.session.Iterations) - 1
		now := time.Now()
		m.session.Iterations[idx].CompletedAt = &now
		m.session.Iterations[idx].HasCommits = hasCommits
	}
	iterationNum := len(m.session.Iterations)
	m.mu.Unlock()

	m.logger.Info("iteration completed",
		"iteration", iterationNum,
		"has_commits", hasCommits,
	)

	m.emitEvent(RalphWiggumEvent{
		Type:      EventRalphWiggumIterationComplete,
		Iteration: iterationNum,
	})
}

// RalphWiggumPromptTemplate is the prompt template for Ralph Wiggum iterations
const RalphWiggumPromptTemplate = `## Task
%s

## Completion Promise

When you have FULLY completed the task, output the following to signal completion:

<promise>%s</promise>

**Rules:**
- Only output the promise when the task is COMPLETELY done
- If tests exist, they must pass before outputting the promise
- If you encounter errors, fix them before outputting the promise
- Each iteration, you can see your previous work in the files and git history

## Current Status

This is iteration %d of your work on this task.
%s

Continue working on the task. Review your previous changes, check for issues, and make progress.
When the task is fully complete with all requirements met, output the completion promise.`

// RalphWiggumFirstIterationExtra is appended to the first iteration
const RalphWiggumFirstIterationExtra = `
This is the FIRST iteration. Start by understanding the task and beginning implementation.`

// RalphWiggumContinueIterationExtra is appended to subsequent iterations
const RalphWiggumContinueIterationExtra = `
Review your previous work:
- Check git log for recent commits
- Review any test failures or errors
- Continue making progress toward completion`
