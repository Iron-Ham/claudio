package review

import (
	"time"
)

// ReviewType represents the type of specialized code review
type ReviewType string

const (
	ReviewTypeSecurity    ReviewType = "security"
	ReviewTypePerformance ReviewType = "performance"
	ReviewTypeStyle       ReviewType = "style"
	ReviewTypeIntegration ReviewType = "integration"
)

// AllReviewTypes returns all available review types
func AllReviewTypes() []ReviewType {
	return []ReviewType{
		ReviewTypeSecurity,
		ReviewTypePerformance,
		ReviewTypeStyle,
		ReviewTypeIntegration,
	}
}

// String returns the string representation of the review type
func (r ReviewType) String() string {
	return string(r)
}

// IsValid returns true if the review type is a recognized value
func (r ReviewType) IsValid() bool {
	switch r {
	case ReviewTypeSecurity, ReviewTypePerformance, ReviewTypeStyle, ReviewTypeIntegration:
		return true
	}
	return false
}

// FindingSeverity represents the severity level of a review finding
type FindingSeverity string

const (
	SeverityCritical   FindingSeverity = "critical"
	SeverityMajor      FindingSeverity = "major"
	SeverityMinor      FindingSeverity = "minor"
	SeveritySuggestion FindingSeverity = "suggestion"
)

// String returns the string representation of the severity
func (s FindingSeverity) String() string {
	return string(s)
}

// IsValid returns true if the severity is a recognized value
func (s FindingSeverity) IsValid() bool {
	switch s {
	case SeverityCritical, SeverityMajor, SeverityMinor, SeveritySuggestion:
		return true
	}
	return false
}

// Weight returns a numeric weight for sorting/prioritizing findings
// Higher weight = more severe
func (s FindingSeverity) Weight() int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityMajor:
		return 3
	case SeverityMinor:
		return 2
	case SeveritySuggestion:
		return 1
	default:
		return 0
	}
}

// ReviewFinding represents a single issue found during code review
type ReviewFinding struct {
	ID          string          `json:"id"`
	Type        ReviewType      `json:"type"`
	Severity    FindingSeverity `json:"severity"`
	File        string          `json:"file"`
	Line        int             `json:"line,omitempty"`
	EndLine     int             `json:"end_line,omitempty"` // For multi-line findings
	Description string          `json:"description"`
	Suggestion  string          `json:"suggestion,omitempty"`
	Confidence  float64         `json:"confidence"` // 0.0 to 1.0

	// Additional context for implementer
	Category   string   `json:"category,omitempty"`   // e.g., "SQL injection", "N+1 query"
	References []string `json:"references,omitempty"` // Links to docs/best practices

	// Tracking fields
	Dismissed   bool      `json:"dismissed,omitempty"`
	DismissedBy string    `json:"dismissed_by,omitempty"` // Instance ID that dismissed
	DismissedAt *time.Time `json:"dismissed_at,omitempty"`
	Addressed   bool      `json:"addressed,omitempty"` // True if implementer fixed it
}

// IsCriticalOrMajor returns true for findings that typically need attention
func (f *ReviewFinding) IsCriticalOrMajor() bool {
	return f.Severity == SeverityCritical || f.Severity == SeverityMajor
}

// ReviewPhase represents the current phase of a review session
type ReviewPhase string

const (
	PhaseScanning  ReviewPhase = "scanning"  // Initial file discovery
	PhaseReviewing ReviewPhase = "reviewing" // Active review by agents
	PhaseReporting ReviewPhase = "reporting" // Aggregating and reporting findings
	PhaseComplete  ReviewPhase = "complete"
	PhaseFailed    ReviewPhase = "failed"
)

// String returns the string representation of the phase
func (p ReviewPhase) String() string {
	return string(p)
}

// ReviewSession tracks the state of a parallel review session
type ReviewSession struct {
	ID                string                 `json:"id"`
	TargetSession     string                 `json:"target_session"`     // Session ID being reviewed
	ReviewerInstances map[ReviewType]string  `json:"reviewer_instances"` // ReviewType -> Instance ID
	Phase             ReviewPhase            `json:"phase"`
	StartedAt         time.Time              `json:"started_at"`
	CompletedAt       *time.Time             `json:"completed_at,omitempty"`
	Findings          []ReviewFinding        `json:"findings"`
	Config            ReviewConfig           `json:"config"`
	Coordination      *ReviewCoordinationState `json:"coordination,omitempty"`

	// Progress tracking
	FilesScanned   int `json:"files_scanned"`
	FilesTotal     int `json:"files_total"`
	ReviewersReady int `json:"reviewers_ready"` // Count of reviewers that have completed

	// Error tracking
	Error string `json:"error,omitempty"`
}

// NewReviewSession creates a new review session
func NewReviewSession(id, targetSession string, config ReviewConfig) *ReviewSession {
	return &ReviewSession{
		ID:                id,
		TargetSession:     targetSession,
		ReviewerInstances: make(map[ReviewType]string),
		Phase:             PhaseScanning,
		StartedAt:         time.Now(),
		Findings:          make([]ReviewFinding, 0),
		Config:            config,
	}
}

// AddFinding adds a new finding to the session
func (s *ReviewSession) AddFinding(finding ReviewFinding) {
	s.Findings = append(s.Findings, finding)
}

// GetFindingsByType returns all findings of a specific type
func (s *ReviewSession) GetFindingsByType(t ReviewType) []ReviewFinding {
	var result []ReviewFinding
	for _, f := range s.Findings {
		if f.Type == t {
			result = append(result, f)
		}
	}
	return result
}

// GetFindingsBySeverity returns all findings of a specific severity
func (s *ReviewSession) GetFindingsBySeverity(sev FindingSeverity) []ReviewFinding {
	var result []ReviewFinding
	for _, f := range s.Findings {
		if f.Severity == sev {
			result = append(result, f)
		}
	}
	return result
}

// GetCriticalFindings returns all critical and major findings
func (s *ReviewSession) GetCriticalFindings() []ReviewFinding {
	var result []ReviewFinding
	for _, f := range s.Findings {
		if f.IsCriticalOrMajor() && !f.Dismissed {
			result = append(result, f)
		}
	}
	return result
}

// Progress returns the completion progress as a percentage (0-100)
func (s *ReviewSession) Progress() float64 {
	if len(s.Config.EnabledReviewers) == 0 {
		return 0
	}
	return float64(s.ReviewersReady) / float64(len(s.Config.EnabledReviewers)) * 100
}

// IsComplete returns true if the review session has finished
func (s *ReviewSession) IsComplete() bool {
	return s.Phase == PhaseComplete || s.Phase == PhaseFailed
}

// ReviewConfig holds configuration for a review session
type ReviewConfig struct {
	EnabledReviewers []ReviewType `json:"enabled_reviewers"`
	MaxParallel      int          `json:"max_parallel"`

	// Real-time streaming options
	RealTimeMode bool `json:"real_time_mode"` // Stream findings to implementer as found

	// Finding handling
	AutoDismissSuggestions bool `json:"auto_dismiss_suggestions"` // Auto-dismiss low-confidence suggestions
	MinConfidence          float64 `json:"min_confidence,omitempty"` // Minimum confidence threshold (0.0-1.0)

	// Scope control
	IncludePatterns []string `json:"include_patterns,omitempty"` // Glob patterns for files to review
	ExcludePatterns []string `json:"exclude_patterns,omitempty"` // Glob patterns for files to skip

	// Timeout settings
	ReviewerTimeout  int `json:"reviewer_timeout,omitempty"` // Seconds per reviewer (0 = no limit)
	SessionTimeout   int `json:"session_timeout,omitempty"`  // Total session timeout in seconds
}

// DefaultReviewConfig returns the default review configuration
func DefaultReviewConfig() ReviewConfig {
	return ReviewConfig{
		EnabledReviewers: []ReviewType{
			ReviewTypeSecurity,
			ReviewTypePerformance,
			ReviewTypeStyle,
		},
		MaxParallel:            3,
		RealTimeMode:           true,
		AutoDismissSuggestions: false,
		MinConfidence:          0.5,
	}
}

// ReviewerPrompt maps a review type to its specialized prompt template
type ReviewerPrompt struct {
	Type           ReviewType `json:"type"`
	SystemPrompt   string     `json:"system_prompt"`
	FocusAreas     []string   `json:"focus_areas"`     // Specific areas to examine
	OutputFormat   string     `json:"output_format"`   // Expected output structure
	ExampleFinding string     `json:"example_finding"` // Example JSON for finding format
}

// DefaultReviewerPrompts returns the default prompts for each reviewer type
func DefaultReviewerPrompts() map[ReviewType]ReviewerPrompt {
	return map[ReviewType]ReviewerPrompt{
		ReviewTypeSecurity: {
			Type: ReviewTypeSecurity,
			SystemPrompt: `You are a security-focused code reviewer. Analyze code for:
- Injection vulnerabilities (SQL, command, XSS)
- Authentication and authorization issues
- Sensitive data exposure
- Insecure cryptographic practices
- OWASP Top 10 vulnerabilities`,
			FocusAreas: []string{
				"input validation",
				"authentication flows",
				"data sanitization",
				"secrets management",
				"access control",
			},
		},
		ReviewTypePerformance: {
			Type: ReviewTypePerformance,
			SystemPrompt: `You are a performance-focused code reviewer. Analyze code for:
- Algorithm complexity issues (O(n²) or worse)
- Memory leaks and excessive allocations
- N+1 query problems
- Missing caching opportunities
- Blocking operations in hot paths`,
			FocusAreas: []string{
				"database queries",
				"loop efficiency",
				"memory usage",
				"concurrency",
				"caching",
			},
		},
		ReviewTypeStyle: {
			Type: ReviewTypeStyle,
			SystemPrompt: `You are a code style and maintainability reviewer. Analyze code for:
- Code duplication
- Naming conventions
- Function length and complexity
- Error handling consistency
- Documentation gaps`,
			FocusAreas: []string{
				"naming conventions",
				"code organization",
				"error handling",
				"documentation",
				"test coverage",
			},
		},
		ReviewTypeIntegration: {
			Type: ReviewTypeIntegration,
			SystemPrompt: `You are an integration-focused code reviewer. Analyze code for:
- API contract violations
- Breaking changes to interfaces
- Missing error handling at boundaries
- Inconsistent data formats
- Cross-module dependencies`,
			FocusAreas: []string{
				"API contracts",
				"interface boundaries",
				"data consistency",
				"dependency management",
				"backwards compatibility",
			},
		},
	}
}

// ReviewCoordinationState tracks cross-session communication for reviews
type ReviewCoordinationState struct {
	// Channel for real-time finding streaming
	FindingsChannel string `json:"findings_channel,omitempty"` // Named pipe or channel identifier

	// Shared context files
	SharedContextPath string `json:"shared_context_path,omitempty"` // Path to shared context directory
	FindingsFilePath  string `json:"findings_file_path,omitempty"` // Path to aggregated findings file

	// Implementer coordination
	ImplementerInstanceID string `json:"implementer_instance_id,omitempty"` // Instance being reviewed
	ImplementerNotified   bool   `json:"implementer_notified"`              // Whether implementer knows about review

	// Cross-reviewer coordination
	ReviewerSyncPoints map[ReviewType]time.Time `json:"reviewer_sync_points,omitempty"` // Last sync time per reviewer
	ConflictingFindings []FindingConflict       `json:"conflicting_findings,omitempty"` // Findings that contradict

	// Event tracking
	LastEventAt *time.Time `json:"last_event_at,omitempty"`
	EventCount  int        `json:"event_count"`
}

// FindingConflict represents conflicting findings from different reviewers
type FindingConflict struct {
	FindingIDs  []string   `json:"finding_ids"`  // IDs of conflicting findings
	Description string     `json:"description"`  // Description of the conflict
	Resolution  string     `json:"resolution,omitempty"` // How it was resolved
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// NewReviewCoordinationState creates a new coordination state
func NewReviewCoordinationState() *ReviewCoordinationState {
	return &ReviewCoordinationState{
		ReviewerSyncPoints:  make(map[ReviewType]time.Time),
		ConflictingFindings: make([]FindingConflict, 0),
	}
}

// UpdateSyncPoint records the last sync time for a reviewer
func (c *ReviewCoordinationState) UpdateSyncPoint(reviewerType ReviewType) {
	if c.ReviewerSyncPoints == nil {
		c.ReviewerSyncPoints = make(map[ReviewType]time.Time)
	}
	c.ReviewerSyncPoints[reviewerType] = time.Now()
}

// AddConflict records a conflict between findings
func (c *ReviewCoordinationState) AddConflict(conflict FindingConflict) {
	c.ConflictingFindings = append(c.ConflictingFindings, conflict)
}

// ReviewEvent represents an event in the review process (for real-time streaming)
type ReviewEvent struct {
	Type       ReviewEventType `json:"type"`
	ReviewerID ReviewType      `json:"reviewer_id,omitempty"`
	Finding    *ReviewFinding  `json:"finding,omitempty"`
	Message    string          `json:"message,omitempty"`
	Timestamp  time.Time       `json:"timestamp"`
}

// ReviewEventType represents the type of review event
type ReviewEventType string

const (
	EventReviewerStarted   ReviewEventType = "reviewer_started"
	EventReviewerCompleted ReviewEventType = "reviewer_completed"
	EventFindingDiscovered ReviewEventType = "finding_discovered"
	EventFindingDismissed  ReviewEventType = "finding_dismissed"
	EventFindingAddressed  ReviewEventType = "finding_addressed"
	EventConflictDetected  ReviewEventType = "conflict_detected"
	EventPhaseChange       ReviewEventType = "phase_change"
)

// ReviewCompletionFile is the sentinel file written when a review session completes
const ReviewCompletionFileName = ".claudio-review-complete.json"

// ReviewCompletionFile represents the completion report from a review session
type ReviewCompletionFile struct {
	SessionID        string          `json:"session_id"`
	TargetSession    string          `json:"target_session"`
	Status           string          `json:"status"` // "complete", "partial", "failed"
	FindingsSummary  FindingsSummary `json:"findings_summary"`
	Findings         []ReviewFinding `json:"findings"`
	ReviewersRun     []ReviewType    `json:"reviewers_run"`
	Duration         string          `json:"duration"` // Human-readable duration
	Recommendations  []string        `json:"recommendations,omitempty"`
}

// FindingsSummary provides aggregate statistics about findings
type FindingsSummary struct {
	Total       int            `json:"total"`
	BySeverity  map[string]int `json:"by_severity"`
	ByType      map[string]int `json:"by_type"`
	Dismissed   int            `json:"dismissed"`
	Addressed   int            `json:"addressed"`
	Outstanding int            `json:"outstanding"` // Total - Dismissed - Addressed
}

// NewFindingsSummary creates a summary from a list of findings
func NewFindingsSummary(findings []ReviewFinding) FindingsSummary {
	summary := FindingsSummary{
		Total:      len(findings),
		BySeverity: make(map[string]int),
		ByType:     make(map[string]int),
	}

	for _, f := range findings {
		summary.BySeverity[f.Severity.String()]++
		summary.ByType[f.Type.String()]++

		if f.Dismissed {
			summary.Dismissed++
		} else if f.Addressed {
			summary.Addressed++
		}
	}

	summary.Outstanding = summary.Total - summary.Dismissed - summary.Addressed
	return summary
}
