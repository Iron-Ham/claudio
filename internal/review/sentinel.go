// Package review provides types and utilities for the parallel code review system.
// Specialized reviewers (security, performance, style) analyze code simultaneously
// and coordinate with active implementation sessions through shared sentinel files.
package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Sentinel file name constants for reviewer-implementer coordination.
// These files serve as both existence-based signals and context carriers
// for cross-session communication in tmux-based parallel workflows.
const (
	// ReviewFindingFileName is written by reviewers when they find an issue.
	// Multiple findings may be written over time, distinguished by unique FindingID.
	ReviewFindingFileName = ".claudio-review-finding.json"

	// ReviewSummaryFileName is written by each reviewer at the end of their review.
	// Contains aggregated statistics and final pass/fail determination.
	ReviewSummaryFileName = ".claudio-review-summary.json"

	// ImplementerAckFileName is written by the implementer to acknowledge findings.
	// Allows the implementer to indicate what action was taken for each finding.
	ImplementerAckFileName = ".claudio-review-ack.json"
)

// ReviewerType identifies the type of specialized reviewer
type ReviewerType string

const (
	ReviewerSecurity    ReviewerType = "security"
	ReviewerPerformance ReviewerType = "performance"
	ReviewerStyle       ReviewerType = "style"
)

// FindingPriority indicates the urgency/importance of a finding for display ordering
type FindingPriority int

const (
	PriorityCritical FindingPriority = 1
	PriorityMajor    FindingPriority = 2
	PriorityMinor    FindingPriority = 3
	PriorityInfo     FindingPriority = 4
)

// FindingSeverity is a string representation of severity for JSON compatibility
type FindingSeverity string

const (
	SeverityCritical FindingSeverity = "critical"
	SeverityMajor    FindingSeverity = "major"
	SeverityMinor    FindingSeverity = "minor"
	SeverityInfo     FindingSeverity = "info"
)

// ReviewFindingFile represents a single finding written by a reviewer.
// Reviewers write these files as they discover issues, allowing real-time
// communication with active implementation sessions.
type ReviewFindingFile struct {
	// FindingID uniquely identifies this finding (e.g., "security-001", "perf-sql-injection")
	FindingID string `json:"finding_id"`

	// ReviewerType identifies which reviewer found this issue
	ReviewerType ReviewerType `json:"reviewer_type"`

	// Finding contains the detailed finding information
	Finding ReviewFinding `json:"finding"`

	// Timestamp when this finding was written
	Timestamp time.Time `json:"timestamp"`

	// Priority for ordering display (lower = more important)
	Priority FindingPriority `json:"priority"`
}

// ReviewFinding contains the detailed information about a single code issue
type ReviewFinding struct {
	// Title is a short summary of the finding
	Title string `json:"title"`

	// Description provides detailed explanation of the issue
	Description string `json:"description"`

	// Severity indicates how serious the issue is
	Severity FindingSeverity `json:"severity"`

	// File is the path to the affected file
	File string `json:"file"`

	// Line is the primary line number (optional, 0 if not applicable)
	Line int `json:"line,omitempty"`

	// LineEnd marks the end of the affected range (optional)
	LineEnd int `json:"line_end,omitempty"`

	// CodeSnippet is an optional excerpt of the problematic code
	CodeSnippet string `json:"code_snippet,omitempty"`

	// Suggestion provides guidance on how to fix the issue
	Suggestion string `json:"suggestion,omitempty"`

	// References are links to documentation or resources
	References []string `json:"references,omitempty"`
}

// ReviewSummaryFile represents the final summary from each reviewer.
// Written at the end of a review to provide aggregated results and
// overall pass/fail determination.
type ReviewSummaryFile struct {
	// ReviewerType identifies which reviewer wrote this summary
	ReviewerType ReviewerType `json:"reviewer_type"`

	// TotalFindings is the total number of issues found
	TotalFindings int `json:"total_findings"`

	// CriticalCount is the number of critical severity findings
	CriticalCount int `json:"critical_count"`

	// MajorCount is the number of major severity findings
	MajorCount int `json:"major_count"`

	// MinorCount is the number of minor severity findings
	MinorCount int `json:"minor_count"`

	// InfoCount is the number of informational findings
	InfoCount int `json:"info_count"`

	// Passed indicates whether the code passed this reviewer's checks
	// Typically false if any critical or major issues were found
	Passed bool `json:"passed"`

	// Recommendations are general suggestions for improving the code
	Recommendations []string `json:"recommendations,omitempty"`

	// Duration is how long the review took
	Duration time.Duration `json:"duration,omitempty"`

	// Timestamp when this summary was written
	Timestamp time.Time `json:"timestamp"`
}

// AcknowledgmentAction represents the action taken by the implementer for a finding
type AcknowledgmentAction string

const (
	// ActionFixed indicates the issue was fixed in the code
	ActionFixed AcknowledgmentAction = "fixed"

	// ActionAcknowledged indicates the issue is known and accepted as-is
	ActionAcknowledged AcknowledgmentAction = "acknowledged"

	// ActionDismissed indicates the finding was dismissed (e.g., false positive)
	ActionDismissed AcknowledgmentAction = "dismissed"

	// ActionDeferred indicates the issue will be addressed later
	ActionDeferred AcknowledgmentAction = "deferred"
)

// FindingAcknowledgment represents the implementer's response to a single finding
type FindingAcknowledgment struct {
	// FindingID matches the FindingID from ReviewFindingFile
	FindingID string `json:"finding_id"`

	// Action indicates what the implementer did about the finding
	Action AcknowledgmentAction `json:"action"`

	// Response is an optional explanation of the action taken
	Response string `json:"response,omitempty"`

	// CommitSHA is the commit that addressed this finding (if ActionFixed)
	CommitSHA string `json:"commit_sha,omitempty"`

	// Timestamp when this acknowledgment was written
	Timestamp time.Time `json:"timestamp"`
}

// ImplementerAckFile represents the implementer's acknowledgment of review findings.
// Written by the implementer to communicate their response to review findings,
// enabling a feedback loop between reviewers and implementers.
type ImplementerAckFile struct {
	// Acknowledgments contains responses to individual findings
	Acknowledgments []FindingAcknowledgment `json:"acknowledgments"`

	// OverallResponse is an optional summary of how findings were addressed
	OverallResponse string `json:"overall_response,omitempty"`

	// Timestamp when this acknowledgment file was written
	Timestamp time.Time `json:"timestamp"`
}

// ReviewFindingFilePath returns the full path to the review finding file for a given worktree.
// Since multiple findings may exist, this returns a path with the finding ID embedded.
func ReviewFindingFilePath(worktreePath, findingID string) string {
	// Use a directory to store multiple findings
	return filepath.Join(worktreePath, ".claudio-review-findings", findingID+".json")
}

// ReviewFindingsDir returns the directory path where finding files are stored
func ReviewFindingsDir(worktreePath string) string {
	return filepath.Join(worktreePath, ".claudio-review-findings")
}

// ReviewSummaryFilePath returns the full path to the review summary file for a given worktree and reviewer.
// Each reviewer type writes its own summary file.
func ReviewSummaryFilePath(worktreePath string, reviewerType ReviewerType) string {
	return filepath.Join(worktreePath, fmt.Sprintf(".claudio-review-summary-%s.json", reviewerType))
}

// ImplementerAckFilePath returns the full path to the implementer acknowledgment file.
func ImplementerAckFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, ImplementerAckFileName)
}

// ParseReviewFindingFile reads and parses a review finding file
func ParseReviewFindingFile(filePath string) (*ReviewFindingFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var finding ReviewFindingFile
	if err := json.Unmarshal(data, &finding); err != nil {
		return nil, fmt.Errorf("failed to parse review finding JSON: %w", err)
	}

	return &finding, nil
}

// ParseReviewFindingFileFromWorktree reads and parses a review finding file by worktree and finding ID
func ParseReviewFindingFileFromWorktree(worktreePath, findingID string) (*ReviewFindingFile, error) {
	return ParseReviewFindingFile(ReviewFindingFilePath(worktreePath, findingID))
}

// ParseReviewSummaryFile reads and parses a review summary file
func ParseReviewSummaryFile(filePath string) (*ReviewSummaryFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var summary ReviewSummaryFile
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("failed to parse review summary JSON: %w", err)
	}

	return &summary, nil
}

// ParseReviewSummaryFileFromWorktree reads and parses a review summary file by worktree and reviewer type
func ParseReviewSummaryFileFromWorktree(worktreePath string, reviewerType ReviewerType) (*ReviewSummaryFile, error) {
	return ParseReviewSummaryFile(ReviewSummaryFilePath(worktreePath, reviewerType))
}

// ParseImplementerAckFile reads and parses an implementer acknowledgment file
func ParseImplementerAckFile(filePath string) (*ImplementerAckFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var ack ImplementerAckFile
	if err := json.Unmarshal(data, &ack); err != nil {
		return nil, fmt.Errorf("failed to parse implementer acknowledgment JSON: %w", err)
	}

	return &ack, nil
}

// ParseImplementerAckFileFromWorktree reads and parses an implementer acknowledgment file by worktree
func ParseImplementerAckFileFromWorktree(worktreePath string) (*ImplementerAckFile, error) {
	return ParseImplementerAckFile(ImplementerAckFilePath(worktreePath))
}

// ListReviewFindings returns all finding files in a worktree's findings directory
func ListReviewFindings(worktreePath string) ([]*ReviewFindingFile, error) {
	findingsDir := ReviewFindingsDir(worktreePath)
	entries, err := os.ReadDir(findingsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No findings directory = no findings
		}
		return nil, err
	}

	var findings []*ReviewFindingFile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		finding, err := ParseReviewFindingFile(filepath.Join(findingsDir, entry.Name()))
		if err != nil {
			continue // Skip malformed files
		}
		findings = append(findings, finding)
	}

	return findings, nil
}

// WriteReviewFindingFile writes a review finding to the appropriate path
func WriteReviewFindingFile(worktreePath string, finding *ReviewFindingFile) error {
	findingsDir := ReviewFindingsDir(worktreePath)
	if err := os.MkdirAll(findingsDir, 0755); err != nil {
		return fmt.Errorf("failed to create findings directory: %w", err)
	}

	filePath := ReviewFindingFilePath(worktreePath, finding.FindingID)
	data, err := json.MarshalIndent(finding, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal finding: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// WriteReviewSummaryFile writes a review summary to the appropriate path
func WriteReviewSummaryFile(worktreePath string, summary *ReviewSummaryFile) error {
	filePath := ReviewSummaryFilePath(worktreePath, summary.ReviewerType)
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// WriteImplementerAckFile writes an implementer acknowledgment to the appropriate path
func WriteImplementerAckFile(worktreePath string, ack *ImplementerAckFile) error {
	filePath := ImplementerAckFilePath(worktreePath)
	data, err := json.MarshalIndent(ack, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal acknowledgment: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// SeverityToPriority converts a FindingSeverity to a FindingPriority
func SeverityToPriority(severity FindingSeverity) FindingPriority {
	switch severity {
	case SeverityCritical:
		return PriorityCritical
	case SeverityMajor:
		return PriorityMajor
	case SeverityMinor:
		return PriorityMinor
	case SeverityInfo:
		return PriorityInfo
	default:
		return PriorityInfo
	}
}

// AllReviewerTypes returns all available reviewer types
func AllReviewerTypes() []ReviewerType {
	return []ReviewerType{
		ReviewerSecurity,
		ReviewerPerformance,
		ReviewerStyle,
	}
}

// CollectAllSummaries collects review summaries from all reviewer types in a worktree
func CollectAllSummaries(worktreePath string) (map[ReviewerType]*ReviewSummaryFile, error) {
	summaries := make(map[ReviewerType]*ReviewSummaryFile)
	for _, reviewerType := range AllReviewerTypes() {
		summary, err := ParseReviewSummaryFileFromWorktree(worktreePath, reviewerType)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Reviewer hasn't finished yet
			}
			return nil, fmt.Errorf("failed to parse %s summary: %w", reviewerType, err)
		}
		summaries[reviewerType] = summary
	}
	return summaries, nil
}
