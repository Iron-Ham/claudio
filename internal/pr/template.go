package pr

import (
	"bytes"
	"regexp"
	"strings"
	"text/template"

	"github.com/gobwas/glob"
)

// TemplateData contains all data available to PR templates
type TemplateData struct {
	// AISummary is the AI-generated summary (if available)
	AISummary string
	// Task is the original task description
	Task string
	// Branch is the branch name
	Branch string
	// ChangedFiles is a list of modified file paths
	ChangedFiles []string
	// CommitLog is the git commit history
	CommitLog string
	// LinkedIssue is any detected issue reference (e.g., "#42")
	LinkedIssue string
	// InstanceID is the Claudio instance identifier
	InstanceID string
}

// RenderTemplate renders a custom PR body template with the given data
func RenderTemplate(tmplStr string, data TemplateData) (string, error) {
	tmpl, err := template.New("pr-template").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ExtractIssueReference extracts issue references from text
// Supports formats: #123, fixes #123, closes #123, resolves #123
func ExtractIssueReference(text string) string {
	// Pattern matches various issue reference formats
	patterns := []string{
		`(?i)(?:fixes|fix|closes|close|resolves|resolve)\s*#(\d+)`,
		`(?i)\((?:fixes|fix|closes|close|resolves|resolve)\s*#(\d+)\)`,
		`#(\d+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) >= 2 {
			return "#" + matches[1]
		}
	}

	return ""
}

// ResolveReviewers determines reviewers based on changed files and config
func ResolveReviewers(changedFiles []string, defaultReviewers []string, byPath map[string][]string) []string {
	reviewerSet := make(map[string]bool)

	// Add default reviewers
	for _, r := range defaultReviewers {
		reviewerSet[normalizeReviewer(r)] = true
	}

	// Check path-based rules
	for pattern, reviewers := range byPath {
		g, err := glob.Compile(pattern)
		if err != nil {
			continue
		}

		for _, file := range changedFiles {
			if g.Match(file) {
				for _, r := range reviewers {
					reviewerSet[normalizeReviewer(r)] = true
				}
				break
			}
		}
	}

	// Convert set to slice
	result := make([]string, 0, len(reviewerSet))
	for r := range reviewerSet {
		result = append(result, r)
	}

	return result
}

// normalizeReviewer removes @ prefix from reviewer handles
func normalizeReviewer(reviewer string) string {
	return strings.TrimPrefix(reviewer, "@")
}

// FormatClosesClause formats issue references for PR body
func FormatClosesClause(issues []string) string {
	if len(issues) == 0 {
		return ""
	}

	var clauses []string
	for _, issue := range issues {
		// Ensure issue has # prefix
		if !strings.HasPrefix(issue, "#") {
			issue = "#" + issue
		}
		clauses = append(clauses, "Closes "+issue)
	}

	return strings.Join(clauses, "\n")
}
