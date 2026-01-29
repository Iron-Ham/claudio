// Package adversarial provides the adversarial review workflow coordinator.
// Adversarial review creates a feedback loop between an IMPLEMENTER and a REVIEWER,
// iterating until the work is approved.
package adversarial

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	Round         int            `json:"round"`
	Increment     *IncrementFile `json:"increment,omitempty"`
	Review        *ReviewFile    `json:"review,omitempty"`
	StartedAt     time.Time      `json:"started_at"`
	ReviewedAt    *time.Time     `json:"reviewed_at,omitempty"`
	SubGroupID    string         `json:"sub_group_id,omitempty"`   // Sub-group ID for this round's instances
	ImplementerID string         `json:"implementer_id,omitempty"` // Instance ID of implementer for this round
	ReviewerID    string         `json:"reviewer_id,omitempty"`    // Instance ID of reviewer for this round
}

// IncrementFileName is the sentinel file the implementer writes when ready for review
const IncrementFileName = ".claudio-adversarial-incremental.json"

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

// sanitizeJSONContent cleans up common LLM quirks in JSON output.
// This handles issues like:
// - Smart/curly quotes (" " ' ') instead of straight quotes (" ')
// - Markdown code blocks wrapping the JSON
// - Extra text before or after the JSON object
// - Various Unicode characters that look like standard ASCII
func sanitizeJSONContent(data []byte) []byte {
	content := string(data)

	// Step 1: Replace smart/curly quotes with straight quotes
	// These are commonly produced by LLMs and word processors
	replacements := map[string]string{
		"\u201C": `"`, // Left double quotation mark "
		"\u201D": `"`, // Right double quotation mark "
		"\u201E": `"`, // Double low-9 quotation mark „
		"\u201F": `"`, // Double high-reversed-9 quotation mark ‟
		"\u2018": `'`, // Left single quotation mark '
		"\u2019": `'`, // Right single quotation mark '
		"\u201A": `'`, // Single low-9 quotation mark ‚
		"\u201B": `'`, // Single high-reversed-9 quotation mark ‛
		"\u00AB": `"`, // Left-pointing double angle quotation mark «
		"\u00BB": `"`, // Right-pointing double angle quotation mark »
		"\u2039": `'`, // Single left-pointing angle quotation mark ‹
		"\u203A": `'`, // Single right-pointing angle quotation mark ›
		"\uFF02": `"`, // Fullwidth quotation mark ＂
	}
	for old, new := range replacements {
		content = strings.ReplaceAll(content, old, new)
	}

	// Step 2: Strip markdown code blocks
	// Match ```json ... ``` or ``` ... ``` (with or without language identifier)
	codeBlockPattern := regexp.MustCompile("(?s)```(?:json)?\\s*\n?(.*?)\n?```")
	if matches := codeBlockPattern.FindStringSubmatch(content); len(matches) > 1 {
		content = matches[1]
	}

	// Step 3: Try to extract JSON object if there's surrounding text
	// Look for the outermost { ... } that forms a valid JSON structure
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "{") {
		// Find the first { and try to extract from there
		startIdx := strings.Index(content, "{")
		if startIdx != -1 {
			content = content[startIdx:]
		}
	}
	if !strings.HasSuffix(content, "}") {
		// Find the last } and try to extract up to there
		endIdx := strings.LastIndex(content, "}")
		if endIdx != -1 {
			content = content[:endIdx+1]
		}
	}

	// Step 4: Final trim
	content = strings.TrimSpace(content)

	return []byte(content)
}

// validateIncrementJSON performs structural validation of the increment file JSON.
// It checks that the file is valid JSON with the expected field types.
func validateIncrementJSON(data []byte) error {
	// Check if it's valid JSON at all
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		// Try to provide a helpful message for common non-JSON errors
		content := string(data)
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		return fmt.Errorf("increment file is not valid JSON. Content starts with: %q. JSON parse error: %w", content, err)
	}

	// Check for required fields existence
	requiredFields := []string{"round", "status", "summary", "files_modified", "approach"}
	var missingFields []string
	for _, field := range requiredFields {
		if _, exists := raw[field]; !exists {
			missingFields = append(missingFields, field)
		}
	}
	if len(missingFields) > 0 {
		return fmt.Errorf("increment file is missing required fields: %v. Expected JSON structure:\n"+
			`{"round": <number>, "status": "ready_for_review"|"failed", "summary": "<string>", `+
			`"files_modified": ["<file1>", "<file2>"], "approach": "<string>", "notes": "<string>"}`,
			missingFields)
	}

	// Validate field types
	if _, ok := raw["round"].(float64); !ok {
		return fmt.Errorf("increment file field 'round' must be a number, got %T", raw["round"])
	}
	if _, ok := raw["status"].(string); !ok {
		return fmt.Errorf("increment file field 'status' must be a string, got %T", raw["status"])
	}
	if _, ok := raw["summary"].(string); !ok {
		return fmt.Errorf("increment file field 'summary' must be a string, got %T", raw["summary"])
	}
	if _, ok := raw["approach"].(string); !ok {
		return fmt.Errorf("increment file field 'approach' must be a string, got %T", raw["approach"])
	}

	// Validate files_modified is an array
	filesModified, ok := raw["files_modified"].([]any)
	if !ok {
		return fmt.Errorf("increment file field 'files_modified' must be an array of strings, got %T", raw["files_modified"])
	}
	// Validate each element in files_modified is a string
	for i, f := range filesModified {
		if _, ok := f.(string); !ok {
			return fmt.Errorf("increment file field 'files_modified[%d]' must be a string, got %T", i, f)
		}
	}

	return nil
}

// validateIncrementContent validates the semantic content of a parsed IncrementFile.
func validateIncrementContent(increment *IncrementFile) error {
	var errors []string

	// Validate round number
	if increment.Round < 1 {
		errors = append(errors, fmt.Sprintf("round must be >= 1, got %d", increment.Round))
	}

	// Validate status
	if increment.Status != "ready_for_review" && increment.Status != "failed" {
		errors = append(errors, fmt.Sprintf("status must be 'ready_for_review' or 'failed', got %q", increment.Status))
	}

	// For ready_for_review status, require non-empty content fields
	if increment.Status == "ready_for_review" {
		if strings.TrimSpace(increment.Summary) == "" {
			errors = append(errors, "summary cannot be empty when status is 'ready_for_review'")
		}
		if strings.TrimSpace(increment.Approach) == "" {
			errors = append(errors, "approach cannot be empty when status is 'ready_for_review'")
		}
		if len(increment.FilesModified) == 0 {
			errors = append(errors, "files_modified cannot be empty when status is 'ready_for_review'")
		} else {
			// Validate individual file entries are not empty
			for i, f := range increment.FilesModified {
				if strings.TrimSpace(f) == "" {
					errors = append(errors, fmt.Sprintf("files_modified[%d] cannot be empty or whitespace", i))
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("increment file validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// ParseIncrementFile reads and parses an increment file with comprehensive validation.
// It sanitizes the input to handle common LLM quirks, validates the JSON structure,
// and checks semantic content to catch malformed files with actionable error messages.
func ParseIncrementFile(worktreePath string) (*IncrementFile, error) {
	path := IncrementFilePath(worktreePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err // Return unwrapped for existence checks
		}
		return nil, fmt.Errorf("failed to read adversarial increment file: %w", err)
	}

	// Sanitize the content to handle common LLM output issues
	sanitized := sanitizeJSONContent(data)

	// Validate that the file contains valid JSON and has expected structure
	if err := validateIncrementJSON(sanitized); err != nil {
		return nil, err
	}

	var increment IncrementFile
	if err := json.Unmarshal(sanitized, &increment); err != nil {
		return nil, fmt.Errorf("failed to parse adversarial increment JSON: %w", err)
	}

	// Validate the parsed content
	if err := validateIncrementContent(&increment); err != nil {
		return nil, err
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

// ParseReviewFile reads and parses a review file.
// It applies sanitization to handle common LLM quirks like smart quotes,
// markdown code blocks, and surrounding text.
func ParseReviewFile(worktreePath string) (*ReviewFile, error) {
	path := ReviewFilePath(worktreePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err // Return unwrapped for existence checks
		}
		return nil, fmt.Errorf("failed to read adversarial review file: %w", err)
	}

	// Sanitize the content to handle common LLM output issues
	sanitized := sanitizeJSONContent(data)

	var review ReviewFile
	if err := json.Unmarshal(sanitized, &review); err != nil {
		// Provide additional context about what we tried to parse
		preview := string(sanitized)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("failed to parse adversarial review JSON: %w (content preview: %q)", err, preview)
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
