package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// AdversarialPhase represents the current phase of an adversarial review session
type AdversarialPhase string

const (
	// PhaseAdversarialImplementing - the implementer is working on the task
	PhaseAdversarialImplementing AdversarialPhase = "implementing"
	// PhaseAdversarialReviewing - the reviewer is critically examining the work
	PhaseAdversarialReviewing AdversarialPhase = "reviewing"
	// PhaseAdversarialApproved - the reviewer has approved the implementation
	PhaseAdversarialApproved AdversarialPhase = "approved"
	// PhaseAdversarialComplete - the session is complete
	PhaseAdversarialComplete AdversarialPhase = "complete"
	// PhaseAdversarialFailed - something went wrong
	PhaseAdversarialFailed AdversarialPhase = "failed"
)

// AdversarialConfig holds configuration for an adversarial review session
type AdversarialConfig struct {
	// MaxIterations limits the number of implement-review cycles (0 = unlimited)
	MaxIterations int `json:"max_iterations"`
}

// DefaultAdversarialConfig returns the default configuration
func DefaultAdversarialConfig() AdversarialConfig {
	return AdversarialConfig{
		MaxIterations: 10, // Reasonable default to prevent infinite loops
	}
}

// AdversarialSession represents an adversarial review orchestration session
type AdversarialSession struct {
	ID            string            `json:"id"`
	GroupID       string            `json:"group_id,omitempty"` // Link to InstanceGroup for display
	Task          string            `json:"task"`               // The original task/problem
	Phase         AdversarialPhase  `json:"phase"`
	Config        AdversarialConfig `json:"config"`
	ImplementerID string            `json:"implementer_id,omitempty"` // Instance ID of implementer
	ReviewerID    string            `json:"reviewer_id,omitempty"`    // Instance ID of reviewer
	CurrentRound  int               `json:"current_round"`            // Current implement-review cycle (1-based)
	Created       time.Time         `json:"created"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	Error         string            `json:"error,omitempty"` // Error message if failed

	// History tracks all increments and reviews
	History []AdversarialRound `json:"history,omitempty"`
}

// AdversarialRound represents one implement-review cycle
type AdversarialRound struct {
	Round      int                       `json:"round"`
	Increment  *AdversarialIncrementFile `json:"increment,omitempty"`
	Review     *AdversarialReviewFile    `json:"review,omitempty"`
	StartedAt  time.Time                 `json:"started_at"`
	ReviewedAt *time.Time                `json:"reviewed_at,omitempty"`
}

// NewAdversarialSession creates a new adversarial review session
func NewAdversarialSession(task string, config AdversarialConfig) *AdversarialSession {
	return &AdversarialSession{
		ID:           generateID(),
		Task:         task,
		Phase:        PhaseAdversarialImplementing,
		Config:       config,
		CurrentRound: 1,
		Created:      time.Now(),
		History:      make([]AdversarialRound, 0),
	}
}

// AdversarialIncrementFileName is the sentinel file the implementer writes when ready for review
const AdversarialIncrementFileName = ".claudio-adversarial-increment.json"

// AdversarialIncrementFile represents the implementer's work submission
type AdversarialIncrementFile struct {
	Round         int      `json:"round"`          // Which iteration this is
	Status        string   `json:"status"`         // "ready_for_review" or "failed"
	Summary       string   `json:"summary"`        // Brief summary of changes made
	FilesModified []string `json:"files_modified"` // Files changed in this increment
	Approach      string   `json:"approach"`       // Description of the approach taken
	Notes         string   `json:"notes"`          // Any concerns or questions for reviewer
}

// AdversarialIncrementFilePath returns the full path to the increment file
func AdversarialIncrementFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, AdversarialIncrementFileName)
}

// ParseAdversarialIncrementFile reads and parses an increment file
func ParseAdversarialIncrementFile(worktreePath string) (*AdversarialIncrementFile, error) {
	path := AdversarialIncrementFilePath(worktreePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var increment AdversarialIncrementFile
	if err := json.Unmarshal(data, &increment); err != nil {
		return nil, fmt.Errorf("failed to parse adversarial increment JSON: %w", err)
	}

	// Validate required fields
	if increment.Round < 1 {
		return nil, fmt.Errorf("invalid round number in increment file: %d (must be >= 1)", increment.Round)
	}
	if increment.Status != "ready_for_review" && increment.Status != "failed" {
		return nil, fmt.Errorf("invalid status in increment file: %q (expected 'ready_for_review' or 'failed')", increment.Status)
	}

	return &increment, nil
}

// AdversarialReviewFileName is the sentinel file the reviewer writes after review
const AdversarialReviewFileName = ".claudio-adversarial-review.json"

// AdversarialReviewFile represents the reviewer's feedback
type AdversarialReviewFile struct {
	Round           int      `json:"round"`            // Which iteration this review is for
	Approved        bool     `json:"approved"`         // True if work is satisfactory
	Score           int      `json:"score"`            // Quality score 1-10
	Strengths       []string `json:"strengths"`        // What was done well
	Issues          []string `json:"issues"`           // Critical problems that must be fixed
	Suggestions     []string `json:"suggestions"`      // Optional improvements
	Summary         string   `json:"summary"`          // Overall assessment
	RequiredChanges []string `json:"required_changes"` // Specific changes needed (if not approved)
}

// AdversarialReviewFilePath returns the full path to the review file
func AdversarialReviewFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, AdversarialReviewFileName)
}

// ParseAdversarialReviewFile reads and parses a review file
func ParseAdversarialReviewFile(worktreePath string) (*AdversarialReviewFile, error) {
	path := AdversarialReviewFilePath(worktreePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var review AdversarialReviewFile
	if err := json.Unmarshal(data, &review); err != nil {
		return nil, fmt.Errorf("failed to parse adversarial review JSON: %w", err)
	}

	// Validate required fields
	if review.Round < 1 {
		return nil, fmt.Errorf("invalid round number in review file: %d (must be >= 1)", review.Round)
	}
	if review.Score < 1 || review.Score > 10 {
		return nil, fmt.Errorf("invalid score in review file: %d (must be 1-10)", review.Score)
	}

	return &review, nil
}

// AdversarialManager manages the execution of an adversarial review session
type AdversarialManager struct {
	session     *AdversarialSession
	orch        *Orchestrator
	baseSession *Session
	logger      *logging.Logger

	// Event handling
	eventCallback func(AdversarialEvent)

	// Synchronization
	mu sync.RWMutex
}

// AdversarialEventType represents the type of adversarial event
type AdversarialEventType string

const (
	EventAdversarialImplementerStarted AdversarialEventType = "implementer_started"
	EventAdversarialIncrementReady     AdversarialEventType = "increment_ready"
	EventAdversarialReviewerStarted    AdversarialEventType = "reviewer_started"
	EventAdversarialReviewReady        AdversarialEventType = "review_ready"
	EventAdversarialApproved           AdversarialEventType = "approved"
	EventAdversarialRejected           AdversarialEventType = "rejected"
	EventAdversarialPhaseChange        AdversarialEventType = "phase_change"
	EventAdversarialComplete           AdversarialEventType = "complete"
	EventAdversarialFailed             AdversarialEventType = "failed"
)

// AdversarialEvent represents an event from the adversarial manager
type AdversarialEvent struct {
	Type       AdversarialEventType `json:"type"`
	Round      int                  `json:"round,omitempty"`
	InstanceID string               `json:"instance_id,omitempty"`
	Message    string               `json:"message,omitempty"`
	Timestamp  time.Time            `json:"timestamp"`
}

// NewAdversarialManager creates a new adversarial manager
func NewAdversarialManager(orch *Orchestrator, baseSession *Session, advSession *AdversarialSession, logger *logging.Logger) *AdversarialManager {
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &AdversarialManager{
		session:     advSession,
		orch:        orch,
		baseSession: baseSession,
		logger:      logger.WithPhase("adversarial"),
	}
}

// SetEventCallback sets the callback for adversarial events
func (m *AdversarialManager) SetEventCallback(cb func(AdversarialEvent)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventCallback = cb
}

// Session returns the adversarial session
func (m *AdversarialManager) Session() *AdversarialSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session
}

// emitEvent sends an event to the callback
func (m *AdversarialManager) emitEvent(event AdversarialEvent) {
	event.Timestamp = time.Now()

	m.mu.RLock()
	cb := m.eventCallback
	m.mu.RUnlock()

	if cb != nil {
		cb(event)
	}
}

// SetPhase updates the session phase and emits an event
func (m *AdversarialManager) SetPhase(phase AdversarialPhase) {
	m.mu.Lock()
	m.session.Phase = phase
	m.mu.Unlock()

	m.emitEvent(AdversarialEvent{
		Type:    EventAdversarialPhaseChange,
		Message: string(phase),
	})
}

// StartRound begins a new implement-review round
func (m *AdversarialManager) StartRound() {
	m.mu.Lock()
	round := AdversarialRound{
		Round:     m.session.CurrentRound,
		StartedAt: time.Now(),
	}
	m.session.History = append(m.session.History, round)
	m.mu.Unlock()

	m.logger.Info("round started", "round", m.session.CurrentRound)
}

// RecordIncrement records an increment submission for the current round
func (m *AdversarialManager) RecordIncrement(increment *AdversarialIncrementFile) {
	m.mu.Lock()
	if len(m.session.History) > 0 {
		m.session.History[len(m.session.History)-1].Increment = increment
	}
	m.mu.Unlock()

	m.logger.Info("increment recorded",
		"round", increment.Round,
		"files_modified", len(increment.FilesModified),
	)

	m.emitEvent(AdversarialEvent{
		Type:    EventAdversarialIncrementReady,
		Round:   increment.Round,
		Message: increment.Summary,
	})
}

// RecordReview records a review for the current round
func (m *AdversarialManager) RecordReview(review *AdversarialReviewFile) {
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
		m.emitEvent(AdversarialEvent{
			Type:    EventAdversarialApproved,
			Round:   review.Round,
			Message: review.Summary,
		})
	} else {
		m.emitEvent(AdversarialEvent{
			Type:    EventAdversarialRejected,
			Round:   review.Round,
			Message: fmt.Sprintf("Issues: %d, Score: %d/10", len(review.Issues), review.Score),
		})
	}
}

// NextRound advances to the next round
func (m *AdversarialManager) NextRound() {
	m.mu.Lock()
	m.session.CurrentRound++
	m.mu.Unlock()

	m.logger.Info("advancing to next round", "round", m.session.CurrentRound)
}

// IsMaxIterationsReached checks if the max iterations limit has been reached
func (m *AdversarialManager) IsMaxIterationsReached() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session.Config.MaxIterations == 0 {
		return false // No limit
	}
	return m.session.CurrentRound > m.session.Config.MaxIterations
}

// AdversarialImplementerPromptTemplate is the prompt for the implementer instance
const AdversarialImplementerPromptTemplate = `You are the IMPLEMENTER in an adversarial review workflow.

## Task
%s

## Your Role
You are responsible for implementing the solution. A critical REVIEWER will examine your work thoroughly after each increment. Your goal is to produce high-quality, well-tested code that can withstand rigorous scrutiny.

## Current Round: %d

%s

## Process
1. Implement the required changes
2. Ensure your code is complete, tested, and follows best practices
3. When ready for review, write the increment file (details below)
4. Wait for reviewer feedback (if not approved, you'll receive specific issues to fix)

## CRITICAL: Increment File Requirement

**YOUR WORK IS NOT READY FOR REVIEW UNTIL YOU WRITE THE INCREMENT FILE.**

The reviewer is waiting for this file. You MUST write it when your implementation is ready.

**File:** ` + "`" + AdversarialIncrementFileName + "`" + ` (in your worktree root)

**Required JSON structure:**
` + "```json" + `
{
  "round": %d,
  "status": "ready_for_review",
  "summary": "Brief summary of what you implemented",
  "files_modified": ["file1.go", "file2.go"],
  "approach": "Description of the approach you took and why",
  "notes": "Any concerns or questions for the reviewer"
}
` + "```" + `

**Rules:**
- Set status to "ready_for_review" when your implementation is complete
- Set status to "failed" if you cannot complete the task
- Be thorough in your summary - the reviewer will read it before examining code
- List ALL files you modified

**REMINDER: Write ` + "`" + AdversarialIncrementFileName + "`" + ` when ready for review.**`

// AdversarialReviewerPromptTemplate is the prompt for the reviewer instance
const AdversarialReviewerPromptTemplate = `You are a CRITICAL REVIEWER in an adversarial review workflow.

## Original Task
%s

## Your Role
You must thoroughly and critically examine the implementer's work. Be demanding - your job is to find problems, not to approve work prematurely. Only approve when the implementation truly meets all requirements with high quality.

## Current Round: %d

## Implementer's Submission
%s

## Review Guidelines
1. **Examine the code thoroughly** - Read all modified files, understand the approach
2. **Be critical** - Look for bugs, edge cases, security issues, performance problems
3. **Check completeness** - Does it fully solve the task? Are there missing pieces?
4. **Verify quality** - Is the code clean, well-structured, properly tested?
5. **Consider maintainability** - Will this code be easy to understand and modify?

## What to Look For
- Logic errors and bugs
- Missing error handling
- Security vulnerabilities
- Performance issues
- Code style violations
- Missing or inadequate tests
- Incomplete implementations
- Edge cases not handled

## CRITICAL: Review File Requirement

**YOUR REVIEW IS NOT COMPLETE UNTIL YOU WRITE THE REVIEW FILE.**

The system is waiting for this file to continue the workflow.

**File:** ` + "`" + AdversarialReviewFileName + "`" + ` (in your worktree root)

**Required JSON structure:**
` + "```json" + `
{
  "round": %d,
  "approved": false,
  "score": 7,
  "strengths": ["Good error handling", "Clean code structure"],
  "issues": ["Missing null check in line 42", "No tests for edge case X"],
  "suggestions": ["Consider adding logging", "Could optimize the loop"],
  "summary": "Overall assessment of the implementation",
  "required_changes": ["Fix the null check", "Add tests for edge case X"]
}
` + "```" + `

**Rules:**
- Set approved to true ONLY if the implementation is truly ready (score >= 8, no critical issues)
- Score from 1-10: 1-4 = major problems, 5-6 = needs work, 7-8 = good, 9-10 = excellent
- Issues should list specific problems that MUST be fixed
- Suggestions are optional improvements (not required for approval)
- required_changes should be specific and actionable

**IMPORTANT:** Do NOT approve work that has any significant issues. The implementer can iterate.

**REMINDER: Write ` + "`" + AdversarialReviewFileName + "`" + ` when your review is complete.**`

// AdversarialPreviousFeedbackTemplate is appended to show previous review feedback
const AdversarialPreviousFeedbackTemplate = `

## Previous Review Feedback (Round %d)
The reviewer found the following issues that must be addressed:

**Score:** %d/10

**Issues to Fix:**
%s

**Required Changes:**
%s

**Reviewer's Summary:**
%s

Please address ALL the issues above in this iteration.`

// FormatAdversarialImplementerPrompt creates the full prompt for the implementer
func FormatAdversarialImplementerPrompt(task string, round int, previousReview *AdversarialReviewFile) string {
	var previousFeedback string
	if previousReview != nil {
		issues := ""
		for i, issue := range previousReview.Issues {
			issues += fmt.Sprintf("  %d. %s\n", i+1, issue)
		}
		if issues == "" {
			issues = "  (none specified)\n"
		}

		changes := ""
		for i, change := range previousReview.RequiredChanges {
			changes += fmt.Sprintf("  %d. %s\n", i+1, change)
		}
		if changes == "" {
			changes = "  (none specified)\n"
		}

		previousFeedback = fmt.Sprintf(AdversarialPreviousFeedbackTemplate,
			previousReview.Round,
			previousReview.Score,
			issues,
			changes,
			previousReview.Summary,
		)
	}

	return fmt.Sprintf(AdversarialImplementerPromptTemplate, task, round, previousFeedback, round)
}

// FormatAdversarialReviewerPrompt creates the full prompt for the reviewer
func FormatAdversarialReviewerPrompt(task string, round int, increment *AdversarialIncrementFile) string {
	submission := fmt.Sprintf(`**Summary:** %s

**Approach:** %s

**Files Modified:** %v

**Notes from Implementer:** %s`,
		increment.Summary,
		increment.Approach,
		increment.FilesModified,
		increment.Notes,
	)

	return fmt.Sprintf(AdversarialReviewerPromptTemplate, task, round, submission, round)
}
