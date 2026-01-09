package orchestrator

import (
	"time"
)

// ReviewAgentType represents the type of review agent
type ReviewAgentType string

const (
	SecurityReview     ReviewAgentType = "security"
	PerformanceReview  ReviewAgentType = "performance"
	StyleReview        ReviewAgentType = "style"
	TestCoverageReview ReviewAgentType = "test_coverage"
	GeneralReview      ReviewAgentType = "general"
)

// ReviewPhase represents the current phase of a review session
type ReviewPhase string

const (
	ReviewPhaseInitializing ReviewPhase = "initializing"
	ReviewPhaseRunning      ReviewPhase = "running"
	ReviewPhasePaused       ReviewPhase = "paused"
	ReviewPhaseComplete     ReviewPhase = "complete"
)

// ReviewSeverity represents the severity level of a review issue
type ReviewSeverity string

const (
	SeverityCritical ReviewSeverity = "critical"
	SeverityMajor    ReviewSeverity = "major"
	SeverityMinor    ReviewSeverity = "minor"
	SeverityInfo     ReviewSeverity = "info"
)

// ReviewIssue represents a single issue found by a review agent
type ReviewIssue struct {
	ID          string          `json:"id"`
	Type        ReviewAgentType `json:"type"`
	Severity    string          `json:"severity"`
	File        string          `json:"file"`
	LineStart   int             `json:"line_start,omitempty"`
	LineEnd     int             `json:"line_end,omitempty"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Suggestion  string          `json:"suggestion,omitempty"`
	CodeSnippet string          `json:"code_snippet,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// ReviewAgent represents a specialized review agent instance
type ReviewAgent struct {
	ID          string          `json:"id"`
	Type        ReviewAgentType `json:"type"`
	InstanceID  string          `json:"instance_id"`
	Status      string          `json:"status"`
	IssuesFound int             `json:"issues_found"`
}

// ReviewConfig holds configuration for a review session
type ReviewConfig struct {
	WatchMode           bool              `json:"watch_mode"`
	EnabledAgents       []ReviewAgentType `json:"enabled_agents"`
	SeverityThreshold   string            `json:"severity_threshold"`
	AutoPauseImplementer bool             `json:"auto_pause_implementer"`
}

// ReviewSession represents a code review session
type ReviewSession struct {
	ID              string        `json:"id"`
	TargetSessionID string        `json:"target_session_id,omitempty"`
	Agents          []ReviewAgent `json:"agents"`
	Issues          []ReviewIssue `json:"issues"`
	Phase           ReviewPhase   `json:"phase"`
	StartedAt       time.Time     `json:"started_at"`
	Config          ReviewConfig  `json:"config"`
}

// NewReviewSession creates a new review session with default configuration
func NewReviewSession(targetSessionID string, config ReviewConfig) *ReviewSession {
	return &ReviewSession{
		ID:              GenerateID(),
		TargetSessionID: targetSessionID,
		Agents:          make([]ReviewAgent, 0),
		Issues:          make([]ReviewIssue, 0),
		Phase:           ReviewPhaseInitializing,
		StartedAt:       time.Now(),
		Config:          config,
	}
}

// NewReviewIssue creates a new review issue with a generated ID
func NewReviewIssue(agentType ReviewAgentType, severity, file, title, description string) *ReviewIssue {
	return &ReviewIssue{
		ID:          GenerateID(),
		Type:        agentType,
		Severity:    severity,
		File:        file,
		Title:       title,
		Description: description,
		CreatedAt:   time.Now(),
	}
}

// DefaultReviewConfig returns a sensible default configuration for review sessions
func DefaultReviewConfig() ReviewConfig {
	return ReviewConfig{
		WatchMode: false,
		EnabledAgents: []ReviewAgentType{
			SecurityReview,
			PerformanceReview,
			StyleReview,
			TestCoverageReview,
			GeneralReview,
		},
		SeverityThreshold:    string(SeverityMinor),
		AutoPauseImplementer: false,
	}
}

// GetIssuesBySeverity returns all issues with the specified severity
func (s *ReviewSession) GetIssuesBySeverity(severity string) []ReviewIssue {
	var result []ReviewIssue
	for _, issue := range s.Issues {
		if issue.Severity == severity {
			result = append(result, issue)
		}
	}
	return result
}

// GetIssuesByAgent returns all issues found by the specified agent type
func (s *ReviewSession) GetIssuesByAgent(agentType ReviewAgentType) []ReviewIssue {
	var result []ReviewIssue
	for _, issue := range s.Issues {
		if issue.Type == agentType {
			result = append(result, issue)
		}
	}
	return result
}

// GetIssuesByFile returns all issues for a specific file
func (s *ReviewSession) GetIssuesByFile(file string) []ReviewIssue {
	var result []ReviewIssue
	for _, issue := range s.Issues {
		if issue.File == file {
			result = append(result, issue)
		}
	}
	return result
}

// AddIssue adds a new issue to the review session
func (s *ReviewSession) AddIssue(issue ReviewIssue) {
	s.Issues = append(s.Issues, issue)
	// Update the agent's issue count
	for i := range s.Agents {
		if s.Agents[i].Type == issue.Type {
			s.Agents[i].IssuesFound++
			break
		}
	}
}

// GetAgent returns the agent of the specified type, or nil if not found
func (s *ReviewSession) GetAgent(agentType ReviewAgentType) *ReviewAgent {
	for i := range s.Agents {
		if s.Agents[i].Type == agentType {
			return &s.Agents[i]
		}
	}
	return nil
}

// HasCriticalIssues returns true if any critical issues have been found
func (s *ReviewSession) HasCriticalIssues() bool {
	for _, issue := range s.Issues {
		if issue.Severity == string(SeverityCritical) {
			return true
		}
	}
	return false
}

// IssueCount returns the total number of issues found
func (s *ReviewSession) IssueCount() int {
	return len(s.Issues)
}

// IssueCountBySeverity returns the count of issues for each severity level
func (s *ReviewSession) IssueCountBySeverity() map[string]int {
	counts := make(map[string]int)
	for _, issue := range s.Issues {
		counts[issue.Severity]++
	}
	return counts
}

// SeverityOrder returns the ordinal position of a severity (for comparison)
// Lower numbers are more severe
func SeverityOrder(severity string) int {
	switch severity {
	case string(SeverityCritical):
		return 0
	case string(SeverityMajor):
		return 1
	case string(SeverityMinor):
		return 2
	case string(SeverityInfo):
		return 3
	default:
		return 4
	}
}

// MeetsSeverityThreshold returns true if the issue severity meets or exceeds the threshold
func MeetsSeverityThreshold(issueSeverity, threshold string) bool {
	return SeverityOrder(issueSeverity) <= SeverityOrder(threshold)
}

// GetIssuesAboveThreshold returns issues that meet or exceed the configured severity threshold
func (s *ReviewSession) GetIssuesAboveThreshold() []ReviewIssue {
	var result []ReviewIssue
	for _, issue := range s.Issues {
		if MeetsSeverityThreshold(issue.Severity, s.Config.SeverityThreshold) {
			result = append(result, issue)
		}
	}
	return result
}
