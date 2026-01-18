// Package adversarial provides the adversarial review workflow coordinator.
// Adversarial review creates a feedback loop between an IMPLEMENTER and a REVIEWER,
// iterating until the work is approved.
package adversarial

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Phase represents the current phase of an adversarial review session
type Phase string

const (
	// PhaseImplementing - the implementer is working on the task
	PhaseImplementing Phase = "implementing"
	// PhaseReviewing - the reviewer is critically examining the work
	PhaseReviewing Phase = "reviewing"
	// PhaseApproved - the reviewer has approved the implementation
	PhaseApproved Phase = "approved"
	// PhaseComplete - the session is complete
	PhaseComplete Phase = "complete"
	// PhaseFailed - something went wrong
	PhaseFailed Phase = "failed"
)

// Config holds configuration for an adversarial review session.
// Note: This struct is used at runtime for orchestration. There is a corresponding
// config.AdversarialConfig struct used for file persistence and viper loading
// which should be kept in sync with this one when adding new fields.
type Config struct {
	// MaxIterations limits the number of implement-review cycles (0 = unlimited)
	MaxIterations int `json:"max_iterations"`
	// MinPassingScore is the minimum score required for approval (1-10, default: 8)
	MinPassingScore int `json:"min_passing_score"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		MaxIterations:   10, // Reasonable default to prevent infinite loops
		MinPassingScore: 8,  // Score >= 8 required for approval
	}
}

// Round represents one implement-review cycle
type Round struct {
	Round      int            `json:"round"`
	Increment  *IncrementFile `json:"increment,omitempty"`
	Review     *ReviewFile    `json:"review,omitempty"`
	StartedAt  time.Time      `json:"started_at"`
	ReviewedAt *time.Time     `json:"reviewed_at,omitempty"`
}

// IncrementFileName is the sentinel file the implementer writes when ready for review
const IncrementFileName = ".claudio-adversarial-increment.json"

// IncrementFile represents the implementer's work submission
type IncrementFile struct {
	Round         int      `json:"round"`          // Which iteration this is
	Status        string   `json:"status"`         // "ready_for_review" or "failed"
	Summary       string   `json:"summary"`        // Brief summary of changes made
	FilesModified []string `json:"files_modified"` // Files changed in this increment
	Approach      string   `json:"approach"`       // Description of the approach taken
	Notes         string   `json:"notes"`          // Any concerns or questions for reviewer
}

// IncrementFilePath returns the full path to the increment file
func IncrementFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, IncrementFileName)
}

// ParseIncrementFile reads and parses an increment file
func ParseIncrementFile(worktreePath string) (*IncrementFile, error) {
	path := IncrementFilePath(worktreePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var increment IncrementFile
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

// ReviewFileName is the sentinel file the reviewer writes after review
const ReviewFileName = ".claudio-adversarial-review.json"

// ReviewFile represents the reviewer's feedback
type ReviewFile struct {
	Round           int      `json:"round"`            // Which iteration this review is for
	Approved        bool     `json:"approved"`         // True if work is satisfactory
	Score           int      `json:"score"`            // Quality score 1-10
	Strengths       []string `json:"strengths"`        // What was done well
	Issues          []string `json:"issues"`           // Critical problems that must be fixed
	Suggestions     []string `json:"suggestions"`      // Optional improvements
	Summary         string   `json:"summary"`          // Overall assessment
	RequiredChanges []string `json:"required_changes"` // Specific changes needed (if not approved)
}

// ReviewFilePath returns the full path to the review file
func ReviewFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, ReviewFileName)
}

// ParseReviewFile reads and parses a review file
func ParseReviewFile(worktreePath string) (*ReviewFile, error) {
	path := ReviewFilePath(worktreePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var review ReviewFile
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

// EventType represents the type of adversarial event
type EventType string

const (
	EventImplementerStarted EventType = "implementer_started"
	EventIncrementReady     EventType = "increment_ready"
	EventReviewerStarted    EventType = "reviewer_started"
	EventReviewReady        EventType = "review_ready"
	EventApproved           EventType = "approved"
	EventRejected           EventType = "rejected"
	EventPhaseChange        EventType = "phase_change"
	EventComplete           EventType = "complete"
	EventFailed             EventType = "failed"
)

// Event represents an event from the adversarial manager
type Event struct {
	Type       EventType `json:"type"`
	Round      int       `json:"round,omitempty"`
	InstanceID string    `json:"instance_id,omitempty"`
	Message    string    `json:"message,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}
