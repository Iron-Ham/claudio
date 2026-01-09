package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Review issue parsing errors
var (
	ErrNoReviewIssuesTag = errors.New("no <review_issues> tag found in output")
	ErrInvalidJSON       = errors.New("invalid JSON in review issues")
	ErrEmptyTitle        = errors.New("review issue title is required")
	ErrEmptyDescription  = errors.New("review issue description is required")
	ErrEmptyFile         = errors.New("review issue file is required")
	ErrInvalidSeverity   = errors.New("review issue has invalid severity")
)

// reviewIssuesTagRegex matches <review_issues>...</review_issues> blocks
var reviewIssuesTagRegex = regexp.MustCompile(`(?s)<review_issues>\s*(.*?)\s*</review_issues>`)

// ParseReviewIssuesFromOutput extracts structured review issues from agent output.
// It looks for content within <review_issues>...</review_issues> tags and parses
// the JSON array inside. If multiple tags are found, all issues are combined.
// Malformed JSON is handled gracefully with partial extraction where possible.
func ParseReviewIssuesFromOutput(output string) ([]ReviewIssue, error) {
	matches := reviewIssuesTagRegex.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil, ErrNoReviewIssuesTag
	}

	var allIssues []ReviewIssue
	var parseErrors []string
	foundNonEmptyContent := false

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		jsonContent := strings.TrimSpace(match[1])
		if jsonContent == "" {
			continue
		}
		foundNonEmptyContent = true

		issues, err := parseJSONIssues(jsonContent)
		if err != nil {
			parseErrors = append(parseErrors, err.Error())
			// Try partial extraction for malformed JSON
			partialIssues := extractPartialIssues(jsonContent)
			allIssues = append(allIssues, partialIssues...)
			continue
		}
		allIssues = append(allIssues, issues...)
	}

	// If we found tags but all were empty, treat as no content found
	if !foundNonEmptyContent {
		return nil, ErrNoReviewIssuesTag
	}

	// Assign IDs and timestamps to issues that don't have them
	for i := range allIssues {
		if allIssues[i].ID == "" {
			allIssues[i].ID = GenerateID()
		}
		if allIssues[i].CreatedAt.IsZero() {
			allIssues[i].CreatedAt = time.Now()
		}
	}

	if len(allIssues) == 0 && len(parseErrors) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrInvalidJSON, strings.Join(parseErrors, "; "))
	}

	return allIssues, nil
}

// parseJSONIssues attempts to parse JSON as either an array or single issue
func parseJSONIssues(jsonContent string) ([]ReviewIssue, error) {
	// Try parsing as array first
	var issues []ReviewIssue
	if err := json.Unmarshal([]byte(jsonContent), &issues); err == nil {
		return issues, nil
	}

	// Try parsing as single issue
	var single ReviewIssue
	if err := json.Unmarshal([]byte(jsonContent), &single); err == nil {
		return []ReviewIssue{single}, nil
	}

	return nil, fmt.Errorf("failed to parse JSON content")
}

// extractPartialIssues attempts to extract valid issues from malformed JSON
// by finding individual JSON objects within the content
func extractPartialIssues(content string) []ReviewIssue {
	var issues []ReviewIssue

	// Find individual JSON objects by matching braces
	depth := 0
	start := -1
	for i, ch := range content {
		switch ch {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				objStr := content[start : i+1]
				var issue ReviewIssue
				if err := json.Unmarshal([]byte(objStr), &issue); err == nil {
					// Only add if it has meaningful content
					if issue.Title != "" || issue.Description != "" {
						issues = append(issues, issue)
					}
				}
				start = -1
			}
		}
	}

	return issues
}

// ValidateReviewIssue validates that a review issue has all required fields.
// Returns an error describing the validation failure, or nil if valid.
func ValidateReviewIssue(issue ReviewIssue) error {
	if strings.TrimSpace(issue.Title) == "" {
		return ErrEmptyTitle
	}
	if strings.TrimSpace(issue.Description) == "" {
		return ErrEmptyDescription
	}
	if strings.TrimSpace(issue.File) == "" {
		return ErrEmptyFile
	}
	if !isValidSeverity(issue.Severity) {
		return fmt.Errorf("%w: %q", ErrInvalidSeverity, issue.Severity)
	}
	return nil
}

// isValidSeverity checks if the severity string is a known severity level
func isValidSeverity(severity string) bool {
	switch severity {
	case string(SeverityCritical), string(SeverityMajor), string(SeverityMinor), string(SeverityInfo):
		return true
	default:
		return false
	}
}

// DeduplicateIssues removes duplicate issues based on file+line+title combination.
// When duplicates are found, the first occurrence is kept.
func DeduplicateIssues(issues []ReviewIssue) []ReviewIssue {
	if len(issues) == 0 {
		return issues
	}

	seen := make(map[string]bool)
	result := make([]ReviewIssue, 0, len(issues))

	for _, issue := range issues {
		key := issueDedupeKey(issue)
		if !seen[key] {
			seen[key] = true
			result = append(result, issue)
		}
	}

	return result
}

// issueDedupeKey generates a unique key for deduplication based on file, line, and title
func issueDedupeKey(issue ReviewIssue) string {
	return fmt.Sprintf("%s:%d:%s", issue.File, issue.LineStart, issue.Title)
}

// MergeIssues combines two issue lists, handling duplicates.
// Issues from 'new' that don't exist in 'existing' are appended.
// Existing issues are not modified.
func MergeIssues(existing, new []ReviewIssue) []ReviewIssue {
	if len(new) == 0 {
		return existing
	}
	if len(existing) == 0 {
		return new
	}

	// Build set of existing issue keys
	existingKeys := make(map[string]bool)
	for _, issue := range existing {
		existingKeys[issueDedupeKey(issue)] = true
	}

	// Create result starting with existing issues
	result := make([]ReviewIssue, len(existing), len(existing)+len(new))
	copy(result, existing)

	// Add new issues that don't exist
	for _, issue := range new {
		key := issueDedupeKey(issue)
		if !existingKeys[key] {
			existingKeys[key] = true
			result = append(result, issue)
		}
	}

	return result
}

// FilterIssuesBySeverity returns issues that meet or exceed the minimum severity.
// Uses the existing SeverityOrder function for comparison.
func FilterIssuesBySeverity(issues []ReviewIssue, minSeverity string) []ReviewIssue {
	if len(issues) == 0 {
		return issues
	}

	result := make([]ReviewIssue, 0, len(issues))
	for _, issue := range issues {
		if MeetsSeverityThreshold(issue.Severity, minSeverity) {
			result = append(result, issue)
		}
	}

	return result
}

// GroupIssuesByFile groups issues by their file path.
// Returns a map where keys are file paths and values are slices of issues for that file.
func GroupIssuesByFile(issues []ReviewIssue) map[string][]ReviewIssue {
	result := make(map[string][]ReviewIssue)

	for _, issue := range issues {
		result[issue.File] = append(result[issue.File], issue)
	}

	return result
}

// GroupIssuesByAgent groups issues by the agent type that found them.
// Returns a map where keys are ReviewAgentType and values are slices of issues.
func GroupIssuesByAgent(issues []ReviewIssue) map[ReviewAgentType][]ReviewIssue {
	result := make(map[ReviewAgentType][]ReviewIssue)

	for _, issue := range issues {
		result[issue.Type] = append(result[issue.Type], issue)
	}

	return result
}

// CompareSeverity compares two severity levels.
// Returns -1 if a is more severe than b, 0 if equal, 1 if a is less severe than b.
// More severe = lower order number (critical=0, major=1, minor=2, info=3)
func CompareSeverity(a, b string) int {
	orderA := SeverityOrder(a)
	orderB := SeverityOrder(b)

	if orderA < orderB {
		return -1 // a is more severe
	}
	if orderA > orderB {
		return 1 // a is less severe
	}
	return 0 // equal
}
