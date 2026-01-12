package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// GitHubIssue represents a GitHub issue with the fields needed for plan ingestion.
// This struct is populated from the `gh` CLI or GitHub API response.
type GitHubIssue struct {
	Number int      `json:"number"` // Issue number (e.g., 42)
	Title  string   `json:"title"`  // Issue title
	Body   string   `json:"body"`   // Issue body (markdown content)
	Labels []string `json:"labels"` // Issue labels (used by gh CLI)
	URL    string   `json:"url"`    // Full GitHub issue URL (e.g., https://github.com/owner/repo/issues/42)
	State  string   `json:"state"`  // Issue state ("open", "closed")
}

// labelJSON is used to unmarshal the nested label objects from gh CLI JSON output.
type labelJSON struct {
	Name string `json:"name"`
}

// ghIssueResponse is the raw JSON structure returned by gh issue view --json.
type ghIssueResponse struct {
	Number int         `json:"number"`
	Title  string      `json:"title"`
	Body   string      `json:"body"`
	Labels []labelJSON `json:"labels"`
	URL    string      `json:"url"`
	State  string      `json:"state"`
}

// CommandExecutor is a function type that executes a command and returns its output.
// This allows for dependency injection in tests.
type CommandExecutor func(name string, args ...string) ([]byte, error)

// defaultExecutor runs commands using os/exec.
var defaultExecutor CommandExecutor = func(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// =============================================================================
// Error Types for GitHub Issue Ingestion
// =============================================================================

// IngestErrorKind categorizes the type of ingestion error.
type IngestErrorKind string

const (
	// ErrKindGHNotInstalled indicates gh CLI is not installed or not in PATH.
	ErrKindGHNotInstalled IngestErrorKind = "gh_not_installed"
	// ErrKindAuthRequired indicates GitHub authentication is required.
	ErrKindAuthRequired IngestErrorKind = "auth_required"
	// ErrKindIssueNotFound indicates the requested issue does not exist (404).
	ErrKindIssueNotFound IngestErrorKind = "issue_not_found"
	// ErrKindRateLimited indicates the request was rate limited by GitHub.
	ErrKindRateLimited IngestErrorKind = "rate_limited"
	// ErrKindNoSubIssues indicates the parent issue has no sub-issues.
	ErrKindNoSubIssues IngestErrorKind = "no_sub_issues"
	// ErrKindParsingFailed indicates parsing the issue content failed.
	ErrKindParsingFailed IngestErrorKind = "parsing_failed"
	// ErrKindCircularDependency indicates a circular dependency was detected.
	ErrKindCircularDependency IngestErrorKind = "circular_dependency"
	// ErrKindUnsupportedProvider indicates the URL provider is not supported.
	ErrKindUnsupportedProvider IngestErrorKind = "unsupported_provider"
	// ErrKindRepoNotFound indicates the repository was not found or not accessible.
	ErrKindRepoNotFound IngestErrorKind = "repo_not_found"
)

// IngestError is a structured error type for issue ingestion failures.
// It provides context about which issue failed and suggestions for resolution.
type IngestError struct {
	// Kind categorizes the error type for programmatic handling.
	Kind IngestErrorKind

	// Message is the human-readable error description.
	Message string

	// IssueNum is the issue number that caused the error (0 if not applicable).
	IssueNum int

	// Owner is the repository owner (empty if not applicable).
	Owner string

	// Repo is the repository name (empty if not applicable).
	Repo string

	// Suggestion provides actionable advice for resolving the error.
	Suggestion string

	// Cause is the underlying error, if any.
	Cause error
}

// Error implements the error interface.
func (e *IngestError) Error() string {
	var sb strings.Builder

	sb.WriteString(e.Message)

	if e.IssueNum > 0 {
		sb.WriteString(fmt.Sprintf(" (issue #%d)", e.IssueNum))
	}

	if e.Owner != "" && e.Repo != "" {
		sb.WriteString(fmt.Sprintf(" in %s/%s", e.Owner, e.Repo))
	}

	if e.Cause != nil {
		sb.WriteString(fmt.Sprintf(": %v", e.Cause))
	}

	return sb.String()
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *IngestError) Unwrap() error {
	return e.Cause
}

// Is implements error matching for errors.Is().
// It matches based on the Kind field when comparing with sentinel errors.
func (e *IngestError) Is(target error) bool {
	switch target {
	case ErrGHNotInstalled:
		return e.Kind == ErrKindGHNotInstalled
	case ErrGHAuthRequired:
		return e.Kind == ErrKindAuthRequired
	case ErrIssueNotFound:
		return e.Kind == ErrKindIssueNotFound
	case ErrRateLimited:
		return e.Kind == ErrKindRateLimited
	case ErrNoSubIssues:
		return e.Kind == ErrKindNoSubIssues
	case ErrParsingFailed:
		return e.Kind == ErrKindParsingFailed
	case ErrCircularDependency:
		return e.Kind == ErrKindCircularDependency
	case ErrUnsupportedProvider:
		return e.Kind == ErrKindUnsupportedProvider
	case ErrRepoNotFound:
		return e.Kind == ErrKindRepoNotFound
	}
	return false
}

// FormatForTerminal returns a user-friendly formatted string suitable for terminal output.
// It includes the error message and suggestion (if any) formatted for CLI display.
func (e *IngestError) FormatForTerminal() string {
	var sb strings.Builder

	// Error message with context
	sb.WriteString("Error: ")
	sb.WriteString(e.Message)

	if e.IssueNum > 0 {
		sb.WriteString(fmt.Sprintf(" (issue #%d)", e.IssueNum))
	}

	if e.Owner != "" && e.Repo != "" {
		sb.WriteString(fmt.Sprintf(" in %s/%s", e.Owner, e.Repo))
	}

	// Add suggestion if available
	if e.Suggestion != "" {
		sb.WriteString("\n\nSuggestion: ")
		sb.WriteString(e.Suggestion)
	}

	return sb.String()
}

// NewIngestError creates a new IngestError with the given parameters.
func NewIngestError(kind IngestErrorKind, message string) *IngestError {
	return &IngestError{
		Kind:    kind,
		Message: message,
	}
}

// WithIssue adds issue context to the error.
func (e *IngestError) WithIssue(issueNum int) *IngestError {
	e.IssueNum = issueNum
	return e
}

// WithRepo adds repository context to the error.
func (e *IngestError) WithRepo(owner, repo string) *IngestError {
	e.Owner = owner
	e.Repo = repo
	return e
}

// WithSuggestion adds a suggestion for resolving the error.
func (e *IngestError) WithSuggestion(suggestion string) *IngestError {
	e.Suggestion = suggestion
	return e
}

// WithCause adds an underlying error.
func (e *IngestError) WithCause(cause error) *IngestError {
	e.Cause = cause
	return e
}

// Sentinel errors for backward compatibility and errors.Is() matching.
// These are kept for compatibility with existing code that uses errors.Is().

// ErrGHNotInstalled indicates that the gh CLI tool is not installed or not in PATH.
var ErrGHNotInstalled = errors.New("gh CLI is not installed or not in PATH")

// ErrGHAuthRequired indicates that gh CLI requires authentication.
var ErrGHAuthRequired = errors.New("gh CLI requires authentication (run 'gh auth login')")

// ErrIssueNotFound indicates that the requested issue does not exist.
var ErrIssueNotFound = errors.New("issue not found")

// ErrRateLimited indicates that the request was rate limited by GitHub.
var ErrRateLimited = errors.New("rate limited by GitHub")

// ErrCircularDependency indicates that a circular dependency was detected in sub-issues.
var ErrCircularDependency = errors.New("circular dependency detected")

// ErrRepoNotFound indicates that the repository was not found or not accessible.
var ErrRepoNotFound = errors.New("repository not found")

// FetchIssue fetches a GitHub issue by owner, repo, and issue number using the gh CLI.
// It returns a GitHubIssue struct containing the issue data, or an error if the fetch fails.
//
// Common errors:
//   - ErrGHNotInstalled: gh CLI is not installed or not in PATH
//   - ErrGHAuthRequired: gh CLI requires authentication
//   - ErrIssueNotFound: the specified issue does not exist
func FetchIssue(owner, repo string, issueNum int) (*GitHubIssue, error) {
	return fetchIssueWithExecutor(owner, repo, issueNum, defaultExecutor)
}

// fetchIssueWithExecutor is the internal implementation that accepts a command executor
// for testability.
func fetchIssueWithExecutor(owner, repo string, issueNum int, executor CommandExecutor) (*GitHubIssue, error) {
	if owner == "" {
		return nil, fmt.Errorf("owner cannot be empty")
	}
	if repo == "" {
		return nil, fmt.Errorf("repo cannot be empty")
	}
	if issueNum <= 0 {
		return nil, fmt.Errorf("issue number must be positive")
	}

	repoArg := fmt.Sprintf("%s/%s", owner, repo)
	args := []string{
		"issue", "view",
		strconv.Itoa(issueNum),
		"--repo", repoArg,
		"--json", "number,title,body,labels,url,state",
	}

	output, err := executor("gh", args...)
	if err != nil {
		return nil, classifyGHError(err, output, issueNum)
	}

	var response ghIssueResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse gh output: %w", err)
	}

	// Convert labels from nested JSON structure to simple string slice
	labels := make([]string, len(response.Labels))
	for i, label := range response.Labels {
		labels[i] = label.Name
	}

	return &GitHubIssue{
		Number: response.Number,
		Title:  response.Title,
		Body:   response.Body,
		Labels: labels,
		URL:    response.URL,
		State:  response.State,
	}, nil
}

// classifyGHError analyzes the error and output from a gh command
// and returns a more specific error type when possible.
// It returns *IngestError with appropriate context and suggestions.
func classifyGHError(err error, output []byte, issueNum int) error {
	outStr := strings.ToLower(string(output))

	// Check for "executable file not found" which indicates gh is not installed
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return NewIngestError(ErrKindGHNotInstalled, "GitHub CLI (gh) is not installed or not in PATH").
			WithSuggestion("Install the GitHub CLI: https://cli.github.com/")
	}

	// Check for common error patterns in output
	switch {
	case strings.Contains(outStr, "not logged in") ||
		strings.Contains(outStr, "authentication required") ||
		strings.Contains(outStr, "gh auth login"):
		return NewIngestError(ErrKindAuthRequired, "GitHub authentication required").
			WithIssue(issueNum).
			WithSuggestion("Run 'gh auth login' to authenticate with GitHub")

	case strings.Contains(outStr, "rate limit") ||
		strings.Contains(outStr, "api rate limit") ||
		strings.Contains(outStr, "secondary rate limit") ||
		strings.Contains(outStr, "abuse detection"):
		return NewIngestError(ErrKindRateLimited, "GitHub API rate limit exceeded").
			WithIssue(issueNum).
			WithSuggestion("Wait a few minutes and try again. If using a token, ensure it has sufficient rate limits.")

	case strings.Contains(outStr, "could not find issue") ||
		strings.Contains(outStr, "issue not found"):
		return NewIngestError(ErrKindIssueNotFound, "issue not found").
			WithIssue(issueNum).
			WithSuggestion("Verify the issue number exists and you have access to the repository")

	case strings.Contains(outStr, "could not resolve to a repository"):
		return NewIngestError(ErrKindRepoNotFound, "repository not found or not accessible").
			WithIssue(issueNum).
			WithSuggestion("Check the repository name and ensure you have access. For private repos, run 'gh auth login'")

	case strings.Contains(outStr, "not found"):
		// Generic "not found" - could be issue or repo
		return NewIngestError(ErrKindIssueNotFound, "resource not found").
			WithIssue(issueNum).
			WithSuggestion("Verify the issue number and repository are correct")
	}

	// Return the original error with output for debugging
	return NewIngestError(ErrKindParsingFailed, "gh command failed").
		WithIssue(issueNum).
		WithCause(fmt.Errorf("%w\n%s", err, string(output)))
}

// GitHub issue URL patterns
var (
	// githubFullURLRegex matches full GitHub issue URLs like:
	// https://github.com/owner/repo/issues/123
	// https://github.com/owner/repo/issues/123?query=param
	// https://github.com/owner/repo/issues/123#anchor
	// http://github.com/owner/repo/issues/123 (both http and https)
	// Note: Does NOT match trailing slashes or extra path segments
	githubFullURLRegex = regexp.MustCompile(
		`^https?://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/issues/(\d+)(?:[?#].*)?$`,
	)

	// githubShorthandRegex matches shorthand format like:
	// owner/repo#123
	githubShorthandRegex = regexp.MustCompile(
		`^([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)#(\d+)$`,
	)
)

// ParseGitHubIssueURL extracts owner, repo, and issue number from a GitHub issue URL.
// It supports both full URLs and shorthand format:
//   - Full URL: https://github.com/owner/repo/issues/123
//   - Shorthand: owner/repo#123
//
// Returns an error if the URL is not a valid GitHub issue reference.
func ParseGitHubIssueURL(url string) (owner, repo string, issueNum int, err error) {
	if url == "" {
		return "", "", 0, fmt.Errorf("empty URL")
	}

	// Try full URL pattern first
	if matches := githubFullURLRegex.FindStringSubmatch(url); len(matches) == 4 {
		issueNum, err = strconv.Atoi(matches[3])
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid issue number: %w", err)
		}
		return matches[1], matches[2], issueNum, nil
	}

	// Try shorthand pattern
	if matches := githubShorthandRegex.FindStringSubmatch(url); len(matches) == 4 {
		issueNum, err = strconv.Atoi(matches[3])
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid issue number: %w", err)
		}
		return matches[1], matches[2], issueNum, nil
	}

	return "", "", 0, fmt.Errorf("invalid GitHub issue URL: %s", url)
}

// IsGitHubIssueURL returns true if the provided string is a valid GitHub issue URL.
// It accepts both full URLs and shorthand format.
func IsGitHubIssueURL(url string) bool {
	if url == "" {
		return false
	}
	return githubFullURLRegex.MatchString(url) || githubShorthandRegex.MatchString(url)
}

// ParentIssueContent holds the parsed content from a parent issue's body.
// This is used when ingesting GitHub issues back into a PlanSpec.
type ParentIssueContent struct {
	Summary         string
	Insights        []string
	Constraints     []string
	ExecutionGroups [][]int // Issue numbers grouped by execution order
}

// Section header patterns for parsing the parent issue body.
// These patterns match the sections defined in parentIssueBodyTemplate.
var (
	// Section headers (case-insensitive for robustness)
	summaryHeaderRe     = regexp.MustCompile(`(?i)^##\s+Summary\s*$`)
	analysisHeaderRe    = regexp.MustCompile(`(?i)^##\s+Analysis\s*$`)
	constraintsHeaderRe = regexp.MustCompile(`(?i)^##\s+Constraints\s*$`)
	subIssuesHeaderRe   = regexp.MustCompile(`(?i)^##\s+Sub-Issues\s*$`)
	executionHeaderRe   = regexp.MustCompile(`(?i)^##\s+Execution Order\s*$`)
	acceptanceHeaderRe  = regexp.MustCompile(`(?i)^##\s+Acceptance Criteria\s*$`)

	// Group header pattern: "### Group N" with optional suffix
	groupHeaderRe = regexp.MustCompile(`(?i)^###\s+Group\s+(\d+)`)

	// Sub-issue reference pattern: "- [ ] #42 - **Title**" or "- [x] #42 - **Title**"
	subIssueRefRe = regexp.MustCompile(`^\s*-\s*\[[x\s]\]\s*#(\d+)`)

	// Bullet point pattern for insights/constraints
	bulletPointRe = regexp.MustCompile(`^\s*-\s+(.+)$`)
)

// ParseParentIssueBody parses the markdown body of a parent issue
// and extracts the structured content.
//
// The parent issue template has these sections:
//   - ## Summary - plan summary text
//   - ## Analysis - insights as bullet list
//   - ## Constraints - constraints as bullet list
//   - ## Sub-Issues - grouped task references
//   - ## Execution Order - static text (ignored)
//   - ## Acceptance Criteria - checklist (ignored)
func ParseParentIssueBody(body string) (*ParentIssueContent, error) {
	content := &ParentIssueContent{
		Insights:        []string{},
		Constraints:     []string{},
		ExecutionGroups: [][]int{},
	}

	lines := strings.Split(body, "\n")

	// Track which section we're currently in
	type section int
	const (
		sectionNone section = iota
		sectionSummary
		sectionAnalysis
		sectionConstraints
		sectionSubIssues
		sectionExecution
		sectionAcceptance
	)

	currentSection := sectionNone
	var summaryLines []string
	var currentGroup []int
	currentGroupNum := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for section headers
		switch {
		case summaryHeaderRe.MatchString(trimmed):
			currentSection = sectionSummary
			continue
		case analysisHeaderRe.MatchString(trimmed):
			currentSection = sectionAnalysis
			continue
		case constraintsHeaderRe.MatchString(trimmed):
			currentSection = sectionConstraints
			continue
		case subIssuesHeaderRe.MatchString(trimmed):
			currentSection = sectionSubIssues
			continue
		case executionHeaderRe.MatchString(trimmed):
			// Save any pending group before changing sections
			if len(currentGroup) > 0 {
				content.ExecutionGroups = append(content.ExecutionGroups, currentGroup)
				currentGroup = nil
			}
			currentSection = sectionExecution
			continue
		case acceptanceHeaderRe.MatchString(trimmed):
			// Save any pending group before changing sections
			if len(currentGroup) > 0 {
				content.ExecutionGroups = append(content.ExecutionGroups, currentGroup)
				currentGroup = nil
			}
			currentSection = sectionAcceptance
			continue
		}

		// Check if this is a new main section header (## Something)
		// that we don't recognize - this ends the current section
		if strings.HasPrefix(trimmed, "## ") {
			if len(currentGroup) > 0 {
				content.ExecutionGroups = append(content.ExecutionGroups, currentGroup)
				currentGroup = nil
			}
			currentSection = sectionNone
			continue
		}

		// Process content based on current section
		switch currentSection {
		case sectionSummary:
			// Collect all non-empty lines until the next section
			if trimmed != "" {
				summaryLines = append(summaryLines, line)
			}

		case sectionAnalysis:
			// Extract bullet points
			if matches := bulletPointRe.FindStringSubmatch(line); len(matches) > 1 {
				insight := strings.TrimSpace(matches[1])
				if insight != "" {
					content.Insights = append(content.Insights, insight)
				}
			}

		case sectionConstraints:
			// Extract bullet points
			if matches := bulletPointRe.FindStringSubmatch(line); len(matches) > 1 {
				constraint := strings.TrimSpace(matches[1])
				if constraint != "" {
					content.Constraints = append(content.Constraints, constraint)
				}
			}

		case sectionSubIssues:
			// Check for group header
			if matches := groupHeaderRe.FindStringSubmatch(trimmed); len(matches) > 1 {
				// Save previous group if any
				if len(currentGroup) > 0 {
					content.ExecutionGroups = append(content.ExecutionGroups, currentGroup)
				}
				currentGroup = []int{}
				currentGroupNum, _ = strconv.Atoi(matches[1])
				_ = currentGroupNum // Track for potential validation
				continue
			}

			// Check for sub-issue reference
			if matches := subIssueRefRe.FindStringSubmatch(line); len(matches) > 1 {
				issueNum, err := strconv.Atoi(matches[1])
				if err == nil {
					currentGroup = append(currentGroup, issueNum)
				}
			}
		}
	}

	// Save any remaining group
	if len(currentGroup) > 0 {
		content.ExecutionGroups = append(content.ExecutionGroups, currentGroup)
	}

	// Join summary lines, preserving paragraph structure
	content.Summary = strings.TrimSpace(strings.Join(summaryLines, "\n"))

	return content, nil
}

// SubIssueContent holds the parsed content from a sub-issue body
type SubIssueContent struct {
	// Description is the task description from the "## Task" section
	Description string

	// Files is the list of file paths from the "## Files to Modify" section
	Files []string

	// DependencyIssueNums contains the issue numbers from the "## Dependencies" section
	DependencyIssueNums []int

	// Complexity is the estimated complexity (low, medium, high)
	Complexity string

	// ParentIssueNum is the parent issue number from "*Part of #<num>*"
	ParentIssueNum int
}

// ParseSubIssueBody parses a sub-issue body markdown and extracts structured content.
// The body should follow the template defined in template.go with sections:
// - ## Task (required)
// - ## Files to Modify (optional)
// - ## Dependencies (optional)
// - ## Complexity (required)
// - *Part of #<num>* (required)
func ParseSubIssueBody(body string) (*SubIssueContent, error) {
	if body == "" {
		return nil, fmt.Errorf("sub-issue body is empty")
	}

	content := &SubIssueContent{}

	// Parse the Task section (required)
	description, err := parseTaskSection(body)
	if err != nil {
		return nil, err
	}
	content.Description = description

	// Parse the Files to Modify section (optional)
	content.Files = parseFilesSection(body)

	// Parse the Dependencies section (optional)
	content.DependencyIssueNums = parseDependenciesSection(body)

	// Parse the Complexity section (required)
	complexity, err := parseComplexitySection(body)
	if err != nil {
		return nil, err
	}
	content.Complexity = complexity

	// Parse the parent issue reference (required)
	parentNum, err := parseParentReference(body)
	if err != nil {
		return nil, err
	}
	content.ParentIssueNum = parentNum

	return content, nil
}

// parseTaskSection extracts the description from the "## Task" section.
// The section starts with "## Task" and ends at the next "##" heading or end of relevant content.
func parseTaskSection(body string) (string, error) {
	// Match "## Task" at start of line, then capture everything until next section header
	// (?sm) enables multiline mode (^ matches line start) and dot-all mode
	re := regexp.MustCompile(`(?sm)^##\s*Task\s*$(.*?)(?:^##|^---|\z)`)
	matches := re.FindStringSubmatch(body)

	if len(matches) < 2 {
		return "", fmt.Errorf("missing required '## Task' section")
	}

	description := strings.TrimSpace(matches[1])
	if description == "" {
		return "", fmt.Errorf("'## Task' section is empty")
	}

	return description, nil
}

// parseFilesSection extracts file paths from the "## Files to Modify" section.
// Files are listed as bullet points with paths in backticks: - `path/to/file.go`
func parseFilesSection(body string) []string {
	// Match the entire Files to Modify section
	// (?sm) enables multiline mode (^ matches line start) and dot-all mode
	sectionRe := regexp.MustCompile(`(?sm)^##\s*Files\s+to\s+Modify\s*$(.*?)(?:^##|^---|\z)`)
	sectionMatches := sectionRe.FindStringSubmatch(body)

	if len(sectionMatches) < 2 {
		return nil
	}

	sectionContent := sectionMatches[1]

	// Extract file paths from backticks in bullet points
	// Pattern matches: - `filepath` or * `filepath`
	fileRe := regexp.MustCompile("(?m)^\\s*[-*]\\s*`([^`]+)`")
	fileMatches := fileRe.FindAllStringSubmatch(sectionContent, -1)

	var files []string
	for _, match := range fileMatches {
		if len(match) >= 2 {
			file := strings.TrimSpace(match[1])
			if file != "" {
				files = append(files, file)
			}
		}
	}

	return files
}

// parseDependenciesSection extracts issue numbers from the "## Dependencies" section.
// Dependencies are listed as references to other issues: #42, #123
func parseDependenciesSection(body string) []int {
	// Match the entire Dependencies section
	// (?sm) enables multiline mode (^ matches line start) and dot-all mode
	sectionRe := regexp.MustCompile(`(?sm)^##\s*Dependencies\s*$(.*?)(?:^##|^---|\z)`)
	sectionMatches := sectionRe.FindStringSubmatch(body)

	if len(sectionMatches) < 2 {
		return nil
	}

	sectionContent := sectionMatches[1]

	// Extract issue numbers from #N references
	issueRe := regexp.MustCompile(`#(\d+)`)
	issueMatches := issueRe.FindAllStringSubmatch(sectionContent, -1)

	var issueNums []int
	seen := make(map[int]bool)
	for _, match := range issueMatches {
		if len(match) >= 2 {
			num, err := strconv.Atoi(match[1])
			if err == nil && !seen[num] {
				issueNums = append(issueNums, num)
				seen[num] = true
			}
		}
	}

	return issueNums
}

// parseComplexitySection extracts the complexity value from the "## Complexity" section.
// Format: "Estimated: **low**" or "Estimated: **medium**" or "Estimated: **high**"
func parseComplexitySection(body string) (string, error) {
	// Match "Estimated: **value**" pattern
	re := regexp.MustCompile(`(?i)Estimated:\s*\*\*(\w+)\*\*`)
	matches := re.FindStringSubmatch(body)

	if len(matches) < 2 {
		return "", fmt.Errorf("missing or malformed complexity (expected 'Estimated: **low/medium/high**')")
	}

	complexity := strings.ToLower(strings.TrimSpace(matches[1]))

	// Validate complexity value
	switch complexity {
	case "low", "medium", "high":
		return complexity, nil
	default:
		return "", fmt.Errorf("invalid complexity value: %q (expected low, medium, or high)", complexity)
	}
}

// parseParentReference extracts the parent issue number from "*Part of #<num>*"
func parseParentReference(body string) (int, error) {
	// Match "*Part of #N*" pattern with flexible whitespace
	re := regexp.MustCompile(`\*Part\s+of\s+#(\d+)\*`)
	matches := re.FindStringSubmatch(body)

	if len(matches) < 2 {
		return 0, fmt.Errorf("missing parent issue reference (expected '*Part of #<number>*')")
	}

	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid parent issue number: %w", err)
	}

	return num, nil
}

// =============================================================================
// GitHub Issue to PlannedTask Conversion (task-6-issue-to-task)
// =============================================================================

// ConvertToPlannedTask converts a parsed GitHub sub-issue into an orchestrator.PlannedTask.
// It uses the issue metadata and parsed content to construct a complete PlannedTask
// suitable for ultraplan execution.
//
// Parameters:
//   - issue: The GitHub issue metadata (number, title, URL, etc.)
//   - content: The parsed content from the issue body (task description, files, dependencies, complexity)
//   - issueNumToTaskID: A mapping from issue numbers to task IDs, used to resolve dependency references
//
// The function:
//   - Generates a task ID from the issue number and title (e.g., "task-42-add-auth")
//   - Maps dependency issue numbers to their corresponding task IDs
//   - Converts the complexity string to the appropriate TaskComplexity enum
//   - Sets the IssueURL field to link back to the original GitHub issue
func ConvertToPlannedTask(issue GitHubIssue, content SubIssueContent, issueNumToTaskID map[int]string) (orchestrator.PlannedTask, error) {
	// Validate required fields
	if issue.Number <= 0 {
		return orchestrator.PlannedTask{}, fmt.Errorf("invalid issue number: %d", issue.Number)
	}
	if issue.Title == "" {
		return orchestrator.PlannedTask{}, fmt.Errorf("issue title is required")
	}
	if content.Description == "" {
		return orchestrator.PlannedTask{}, fmt.Errorf("task description is required")
	}

	// Generate the task ID from issue number and title
	taskID := GenerateTaskID(issue.Number, issue.Title)

	// Map dependency issue numbers to task IDs
	dependsOn, err := MapDependenciesToTaskIDs(content.DependencyIssueNums, issueNumToTaskID)
	if err != nil {
		return orchestrator.PlannedTask{}, fmt.Errorf("failed to map dependencies: %w", err)
	}

	// Convert complexity string to TaskComplexity
	complexity, err := ParseTaskComplexity(content.Complexity)
	if err != nil {
		return orchestrator.PlannedTask{}, fmt.Errorf("failed to parse complexity: %w", err)
	}

	// Construct the PlannedTask
	task := orchestrator.PlannedTask{
		ID:            taskID,
		Title:         issue.Title,
		Description:   content.Description,
		Files:         content.Files,
		DependsOn:     dependsOn,
		Priority:      0, // Priority will be determined by execution order during plan construction
		EstComplexity: complexity,
		IssueURL:      issue.URL,
	}

	return task, nil
}

// GenerateTaskID creates a task ID from an issue number and title.
// The format is: task-<number>-<slug>
// where slug is a lowercase, hyphenated version of the title with:
//   - All non-alphanumeric characters converted to hyphens
//   - Multiple consecutive hyphens collapsed to single hyphens
//   - Leading/trailing hyphens removed
//   - Maximum length of 50 characters for the slug portion
//
// Examples:
//   - (42, "Add user authentication") -> "task-42-add-user-authentication"
//   - (123, "Fix bug #456") -> "task-123-fix-bug-456"
//   - (1, "   Multiple   Spaces   ") -> "task-1-multiple-spaces"
func GenerateTaskID(issueNum int, title string) string {
	slug := slugify(title, 50)
	return fmt.Sprintf("task-%d-%s", issueNum, slug)
}

// slugify converts a string to a URL-friendly slug.
// It lowercases the input, replaces non-alphanumeric characters with hyphens,
// collapses multiple hyphens, and trims leading/trailing hyphens.
// The maxLen parameter limits the slug length (0 for no limit).
func slugify(s string, maxLen int) string {
	if s == "" {
		return ""
	}

	// Convert to lowercase
	s = strings.ToLower(s)

	// Build result with only alphanumeric characters and hyphens
	var result strings.Builder
	lastWasHyphen := true // Start as true to avoid leading hyphen

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
			lastWasHyphen = false
		} else if !lastWasHyphen {
			// Non-alphanumeric: add hyphen if we haven't just added one
			result.WriteRune('-')
			lastWasHyphen = true
		}
	}

	slug := result.String()

	// Trim trailing hyphen
	slug = strings.TrimRight(slug, "-")

	// Apply max length limit if specified
	if maxLen > 0 && len(slug) > maxLen {
		slug = slug[:maxLen]
		// Remove any trailing hyphen after truncation
		slug = strings.TrimRight(slug, "-")
	}

	return slug
}

// MapDependenciesToTaskIDs converts a list of issue numbers to their corresponding task IDs.
// It uses the provided mapping to look up each dependency.
// Returns an error if any dependency issue number is not found in the mapping.
func MapDependenciesToTaskIDs(issueNums []int, issueNumToTaskID map[int]string) ([]string, error) {
	if len(issueNums) == 0 {
		return nil, nil
	}

	taskIDs := make([]string, 0, len(issueNums))
	var missingDeps []int

	for _, issueNum := range issueNums {
		taskID, ok := issueNumToTaskID[issueNum]
		if !ok {
			missingDeps = append(missingDeps, issueNum)
			continue
		}
		taskIDs = append(taskIDs, taskID)
	}

	if len(missingDeps) > 0 {
		return nil, fmt.Errorf("dependency issue(s) not found in mapping: %v", missingDeps)
	}

	return taskIDs, nil
}

// ParseTaskComplexity converts a complexity string to orchestrator.TaskComplexity.
// Valid values are "low", "medium", and "high" (case-insensitive).
func ParseTaskComplexity(complexity string) (orchestrator.TaskComplexity, error) {
	switch strings.ToLower(strings.TrimSpace(complexity)) {
	case "low":
		return orchestrator.ComplexityLow, nil
	case "medium":
		return orchestrator.ComplexityMedium, nil
	case "high":
		return orchestrator.ComplexityHigh, nil
	default:
		return "", fmt.Errorf("invalid complexity value: %q (expected low, medium, or high)", complexity)
	}
}

// =============================================================================
// URL-to-PlanSpec Ingestion Pipeline
// =============================================================================

// SourceProvider identifies the source of an issue URL
type SourceProvider string

const (
	ProviderGitHub  SourceProvider = "github"
	ProviderLinear  SourceProvider = "linear"
	ProviderNotion  SourceProvider = "notion"
	ProviderUnknown SourceProvider = ""
)

// DetectSourceProvider determines the issue provider from a URL string
func DetectSourceProvider(url string) SourceProvider {
	if IsGitHubIssueURL(url) {
		return ProviderGitHub
	}
	// Future: Add Linear and Notion detection
	// if IsLinearIssueURL(url) { return ProviderLinear }
	// if IsNotionPageURL(url) { return ProviderNotion }
	return ProviderUnknown
}

// ErrUnsupportedProvider indicates that the URL provider is not supported for ingestion
var ErrUnsupportedProvider = errors.New("unsupported issue provider")

// ErrNoSubIssues indicates that the parent issue has no sub-issues defined
var ErrNoSubIssues = errors.New("parent issue has no sub-issues")

// ErrParsingFailed indicates that parsing the issue content failed
var ErrParsingFailed = errors.New("failed to parse issue content")

// BuildPlanFromURL fetches a parent issue from a URL and converts it (and all
// its sub-issues) into a PlanSpec suitable for ultraplan execution.
//
// The function:
//  1. Detects the source provider (GitHub, Linear, Notion) from the URL
//  2. Fetches the parent issue
//  3. Parses the parent issue body to extract sub-issue references
//  4. Fetches all referenced sub-issues
//  5. Converts each sub-issue to a PlannedTask
//  6. Assembles everything into a complete PlanSpec
//
// Currently only GitHub is supported. Linear and Notion support is planned.
func BuildPlanFromURL(url string) (*orchestrator.PlanSpec, error) {
	return buildPlanFromURLWithExecutor(url, defaultExecutor)
}

// buildPlanFromURLWithExecutor is the internal implementation that accepts a command
// executor for testability.
func buildPlanFromURLWithExecutor(url string, executor CommandExecutor) (*orchestrator.PlanSpec, error) {
	provider := DetectSourceProvider(url)
	if provider == ProviderUnknown {
		return nil, fmt.Errorf("%w: unable to detect provider from URL %q", ErrUnsupportedProvider, url)
	}

	switch provider {
	case ProviderGitHub:
		return buildPlanFromGitHubIssue(url, executor)
	case ProviderLinear:
		return nil, fmt.Errorf("%w: Linear is not yet supported", ErrUnsupportedProvider)
	case ProviderNotion:
		return nil, fmt.Errorf("%w: Notion is not yet supported", ErrUnsupportedProvider)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, provider)
	}
}

// buildPlanFromGitHubIssue implements the GitHub-specific ingestion logic
func buildPlanFromGitHubIssue(url string, executor CommandExecutor) (*orchestrator.PlanSpec, error) {
	// Step 1: Parse the URL to extract owner, repo, and issue number
	owner, repo, issueNum, err := ParseGitHubIssueURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GitHub issue URL: %w", err)
	}

	// Step 2: Fetch the parent issue
	parentIssue, err := fetchIssueWithExecutor(owner, repo, issueNum, executor)
	if err != nil {
		// Enhance error with repo context if not already present
		if ingestErr, ok := err.(*IngestError); ok {
			return nil, ingestErr.WithRepo(owner, repo)
		}
		return nil, NewIngestError(ErrKindParsingFailed, "failed to fetch parent issue").
			WithIssue(issueNum).
			WithRepo(owner, repo).
			WithCause(err)
	}

	// Step 3: Detect format and parse the parent issue body to extract structure
	// This supports both templated (Claudio-generated) and freeform (human-authored) issues
	parentContent, err := ParseParentIssueBodyAuto(parentIssue.Body)
	if err != nil {
		return nil, NewIngestError(ErrKindParsingFailed, "failed to parse parent issue body").
			WithIssue(issueNum).
			WithRepo(owner, repo).
			WithCause(err).
			WithSuggestion("Ensure the issue follows a supported format with sub-issues listed as #N references")
	}

	// Step 4: Collect all sub-issue numbers from execution groups
	subIssueNums := collectSubIssueNumbers(parentContent.ExecutionGroups)
	if len(subIssueNums) == 0 {
		return nil, NewIngestError(ErrKindNoSubIssues, "no sub-issues found in parent issue").
			WithIssue(issueNum).
			WithRepo(owner, repo).
			WithSuggestion("The parent issue must reference sub-issues using #N syntax (e.g., '- [ ] #123 - Task title')")
	}

	// Step 5: Fetch all sub-issues and build the issue number to task ID mapping
	subIssues, issueNumToTaskID, err := fetchSubIssuesWithMappingEnhanced(owner, repo, subIssueNums, executor)
	if err != nil {
		// Error already has context from fetchSubIssuesWithMappingEnhanced
		return nil, err
	}

	// Step 6: Convert each sub-issue to a PlannedTask
	// Pass the parent issue number for freeform sub-issues to exclude from dependencies
	tasks, err := convertSubIssuesToTasksEnhanced(subIssues, issueNumToTaskID, issueNum, owner, repo)
	if err != nil {
		// Error already has context from convertSubIssuesToTasksEnhanced
		return nil, err
	}

	// Step 6.5: Check for circular dependencies
	if cycle := detectCircularDependencies(tasks); cycle != nil {
		return nil, NewIngestError(ErrKindCircularDependency, "circular dependency detected in sub-issues").
			WithRepo(owner, repo).
			WithSuggestion(fmt.Sprintf("Review the dependency chain: %s", formatDependencyCycle(cycle)))
	}

	// Step 7: Build execution order from parent's execution groups
	executionOrder := buildExecutionOrder(parentContent.ExecutionGroups, issueNumToTaskID)

	// Step 8: Build dependency graph from tasks
	dependencyGraph := make(map[string][]string)
	for _, task := range tasks {
		dependencyGraph[task.ID] = task.DependsOn
	}

	// Step 9: Assign priorities based on execution order
	assignPriorities(tasks, executionOrder)

	// Step 10: Assemble the PlanSpec
	plan := &orchestrator.PlanSpec{
		ID:              orchestrator.GenerateID(),
		Objective:       parentIssue.Title,
		Summary:         parentContent.Summary,
		Tasks:           tasks,
		DependencyGraph: dependencyGraph,
		ExecutionOrder:  executionOrder,
		Insights:        parentContent.Insights,
		Constraints:     parentContent.Constraints,
		CreatedAt:       time.Now(),
	}

	return plan, nil
}

// collectSubIssueNumbers extracts all unique issue numbers from execution groups
func collectSubIssueNumbers(executionGroups [][]int) []int {
	seen := make(map[int]bool)
	var nums []int
	for _, group := range executionGroups {
		for _, num := range group {
			if !seen[num] {
				seen[num] = true
				nums = append(nums, num)
			}
		}
	}
	return nums
}

// fetchSubIssuesWithMappingEnhanced fetches all sub-issues and builds a mapping from
// issue numbers to task IDs (which are generated from issue number and title).
// Returns *IngestError with detailed context for each failure.
func fetchSubIssuesWithMappingEnhanced(owner, repo string, issueNums []int, executor CommandExecutor) (map[int]*GitHubIssue, map[int]string, error) {
	issues := make(map[int]*GitHubIssue)
	issueNumToTaskID := make(map[int]string)

	for _, num := range issueNums {
		issue, err := fetchIssueWithExecutor(owner, repo, num, executor)
		if err != nil {
			// Enhance error with repo context if not already present
			if ingestErr, ok := err.(*IngestError); ok {
				return nil, nil, ingestErr.WithRepo(owner, repo)
			}
			return nil, nil, NewIngestError(ErrKindParsingFailed, "failed to fetch sub-issue").
				WithIssue(num).
				WithRepo(owner, repo).
				WithCause(err)
		}
		issues[num] = issue
		// Generate task ID immediately so we can use it for dependency resolution
		issueNumToTaskID[num] = GenerateTaskID(num, issue.Title)
	}

	return issues, issueNumToTaskID, nil
}

// convertSubIssuesToTasksEnhanced converts all fetched sub-issues to PlannedTasks.
// The parentIssueNum is used for freeform issues to exclude the parent from dependencies.
// Returns *IngestError with detailed context for each failure.
func convertSubIssuesToTasksEnhanced(subIssues map[int]*GitHubIssue, issueNumToTaskID map[int]string, parentIssueNum int, owner, repo string) ([]orchestrator.PlannedTask, error) {
	var tasks []orchestrator.PlannedTask

	for num, issue := range subIssues {
		// Parse the sub-issue body using auto-detection
		// This supports both templated (Claudio-generated) and freeform (human-authored) sub-issues
		content, err := ParseSubIssueBodyAuto(issue.Body, parentIssueNum)
		if err != nil {
			return nil, NewIngestError(ErrKindParsingFailed, "failed to parse sub-issue body").
				WithIssue(num).
				WithRepo(owner, repo).
				WithCause(err).
				WithSuggestion("Check that the sub-issue body contains a valid description")
		}

		// Convert to PlannedTask
		task, err := ConvertToPlannedTask(*issue, *content, issueNumToTaskID)
		if err != nil {
			return nil, NewIngestError(ErrKindParsingFailed, "failed to convert sub-issue to task").
				WithIssue(num).
				WithRepo(owner, repo).
				WithCause(err).
				WithSuggestion("Ensure the sub-issue has a title and valid content")
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// detectCircularDependencies checks for circular dependencies in a list of tasks.
// Returns the cycle as a slice of task IDs if found, or nil if no cycle exists.
func detectCircularDependencies(tasks []orchestrator.PlannedTask) []string {
	// Build adjacency list from task dependencies
	deps := make(map[string][]string)
	taskExists := make(map[string]bool)
	for _, task := range tasks {
		deps[task.ID] = task.DependsOn
		taskExists[task.ID] = true
	}

	// Track visit state: 0=unvisited, 1=visiting (in current path), 2=visited (complete)
	state := make(map[string]int)
	var cyclePath []string

	// DFS to detect cycles
	var visit func(taskID string, path []string) bool
	visit = func(taskID string, path []string) bool {
		if state[taskID] == 1 {
			// Found a cycle - extract just the cycle portion
			for i, id := range path {
				if id == taskID {
					cyclePath = append(path[i:], taskID)
					return true
				}
			}
			cyclePath = append(path, taskID)
			return true
		}
		if state[taskID] == 2 {
			return false
		}

		state[taskID] = 1
		path = append(path, taskID)

		for _, depID := range deps[taskID] {
			// Only check dependencies that exist in our task set
			if taskExists[depID] && visit(depID, path) {
				return true
			}
		}

		state[taskID] = 2
		return false
	}

	// Check all tasks
	for _, task := range tasks {
		if state[task.ID] == 0 {
			if visit(task.ID, nil) {
				return cyclePath
			}
		}
	}

	return nil
}

// formatDependencyCycle formats a cycle path for display.
// Input: ["task-1", "task-2", "task-3", "task-1"]
// Output: "task-1 -> task-2 -> task-3 -> task-1"
func formatDependencyCycle(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	return strings.Join(cycle, " -> ")
}

// buildExecutionOrder converts issue number groups to task ID groups
func buildExecutionOrder(executionGroups [][]int, issueNumToTaskID map[int]string) [][]string {
	result := make([][]string, len(executionGroups))
	for i, group := range executionGroups {
		result[i] = make([]string, 0, len(group))
		for _, num := range group {
			if taskID, ok := issueNumToTaskID[num]; ok {
				result[i] = append(result[i], taskID)
			}
		}
	}
	return result
}

// assignPriorities sets task priorities based on their position in execution order
// Tasks in earlier groups have lower (higher priority) values
func assignPriorities(tasks []orchestrator.PlannedTask, executionOrder [][]string) {
	// Build a map of task ID to its group index
	taskPriority := make(map[string]int)
	for groupIdx, group := range executionOrder {
		for taskIdx, taskID := range group {
			// Priority = group index * 100 + position within group
			// This ensures tasks in earlier groups always have lower priority values
			taskPriority[taskID] = groupIdx*100 + taskIdx
		}
	}

	// Apply priorities to tasks
	for i := range tasks {
		if priority, ok := taskPriority[tasks[i].ID]; ok {
			tasks[i].Priority = priority
		}
	}
}

// =============================================================================
// Freeform Parent Issue Parsing
// =============================================================================

// Regex patterns for parsing freeform parent issues
var (
	// freeformGroupHeaderRe matches group headers like "### Group 1", "### Phase 1", "### Step 1"
	// or numbered headers like "### 1. First Group", "### 1) First Task"
	freeformGroupHeaderRe = regexp.MustCompile(`(?i)^###\s*(?:Group|Phase|Step|Stage)?\s*(\d+)`)

	// numberedGroupRe matches numbered list headers like "1. ", "1) ", "1: "
	numberedGroupRe = regexp.MustCompile(`^(\d+)[.):]\s+`)

	// freeformIssueRefRe matches issue references in various formats:
	// - "- [ ] #123 - Title" or "- [x] #123 - Title" (checkbox format)
	// - "- #123 - Title" or "- #123" (simple bullet with issue)
	// - "#123" (just the issue number)
	freeformIssueRefRe = regexp.MustCompile(`#(\d+)`)

	// h2HeaderRe matches any H2 header (## Something)
	h2HeaderRe = regexp.MustCompile(`^##\s+(.+)$`)

	// h3HeaderRe matches any H3 header (### Something)
	h3HeaderRe = regexp.MustCompile(`^###\s+(.+)$`)
)

// ParseFreeformParentIssueBody parses a human-authored (freeform) parent issue body
// and extracts structured content. Unlike templated issues, freeform issues don't
// follow the strict Claudio template but typically have:
//   - A title/summary section at the top (first paragraph(s) before any headers)
//   - Grouped tasks with headers like "### Group 1" or just bullet lists
//   - Issue references like "- [ ] #123 - Title" or "- #123"
//
// The function attempts to extract:
//   - Summary: Text from the beginning until the first H2/H3 header or issue list
//   - ExecutionGroups: Issues grouped by "### Group N" headers, or all issues in a single group
//   - Insights and Constraints are left empty (not typically in freeform issues)
func ParseFreeformParentIssueBody(body string) (*ParentIssueContent, error) {
	content := &ParentIssueContent{
		Insights:        []string{},
		Constraints:     []string{},
		ExecutionGroups: [][]int{},
	}

	if strings.TrimSpace(body) == "" {
		return content, nil
	}

	lines := strings.Split(body, "\n")

	// Phase 1: Extract summary from the beginning of the body
	// Summary is everything before the first H2/H3 header or before issue references start
	var summaryLines []string
	summaryEnded := false
	issueListStarted := false

	// Phase 2: Track groups and their issues
	type groupInfo struct {
		groupNum int
		issues   []int
	}
	var groups []groupInfo
	var currentGroup *groupInfo
	var ungroupedIssues []int

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for H2 header - ends summary section
		if h2HeaderRe.MatchString(trimmed) {
			summaryEnded = true
			continue
		}

		// Check for H3 group header (### Group N or similar)
		if matches := freeformGroupHeaderRe.FindStringSubmatch(trimmed); len(matches) > 1 {
			summaryEnded = true

			// Save any previous group
			if currentGroup != nil && len(currentGroup.issues) > 0 {
				groups = append(groups, *currentGroup)
			}

			// Start new group
			groupNum, _ := strconv.Atoi(matches[1])
			currentGroup = &groupInfo{groupNum: groupNum, issues: []int{}}
			continue
		}

		// Check for any H3 header (might indicate a new section/group)
		if h3HeaderRe.MatchString(trimmed) {
			summaryEnded = true

			// Save any previous group
			if currentGroup != nil && len(currentGroup.issues) > 0 {
				groups = append(groups, *currentGroup)
			}

			// Start a new implicit group
			currentGroup = &groupInfo{groupNum: len(groups) + 1, issues: []int{}}
			continue
		}

		// Check for issue references in bullet points or checklists
		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
			if matches := freeformIssueRefRe.FindAllStringSubmatch(line, -1); len(matches) > 0 {
				summaryEnded = true
				issueListStarted = true

				for _, match := range matches {
					if len(match) > 1 {
						issueNum, err := strconv.Atoi(match[1])
						if err == nil {
							if currentGroup != nil {
								currentGroup.issues = append(currentGroup.issues, issueNum)
							} else {
								ungroupedIssues = append(ungroupedIssues, issueNum)
							}
						}
					}
				}
				continue
			}
		}

		// Check for numbered list items with issues (e.g., "1. #123 - Title")
		if numberedGroupRe.MatchString(trimmed) {
			if matches := freeformIssueRefRe.FindAllStringSubmatch(line, -1); len(matches) > 0 {
				summaryEnded = true
				issueListStarted = true

				for _, match := range matches {
					if len(match) > 1 {
						issueNum, err := strconv.Atoi(match[1])
						if err == nil {
							if currentGroup != nil {
								currentGroup.issues = append(currentGroup.issues, issueNum)
							} else {
								ungroupedIssues = append(ungroupedIssues, issueNum)
							}
						}
					}
				}
				continue
			}
		}

		// Collect summary lines before any headers or issue lists
		if !summaryEnded && !issueListStarted {
			// Skip empty lines at the very beginning
			if len(summaryLines) == 0 && trimmed == "" {
				continue
			}
			summaryLines = append(summaryLines, line)
		}
	}

	// Save any remaining group
	if currentGroup != nil && len(currentGroup.issues) > 0 {
		groups = append(groups, *currentGroup)
	}

	// Build execution groups
	if len(groups) > 0 {
		// Sort groups by group number and extract issues
		// Note: groups should already be in order based on parsing order
		for _, g := range groups {
			if len(g.issues) > 0 {
				content.ExecutionGroups = append(content.ExecutionGroups, deduplicateIssues(g.issues))
			}
		}
	}

	// Add any ungrouped issues as a single group
	if len(ungroupedIssues) > 0 {
		// If we have grouped issues, append ungrouped as a new group
		// Otherwise, ungrouped issues form the first (and only) group
		content.ExecutionGroups = append(content.ExecutionGroups, deduplicateIssues(ungroupedIssues))
	}

	// Process summary: trim trailing empty lines and join
	for len(summaryLines) > 0 && strings.TrimSpace(summaryLines[len(summaryLines)-1]) == "" {
		summaryLines = summaryLines[:len(summaryLines)-1]
	}
	content.Summary = strings.TrimSpace(strings.Join(summaryLines, "\n"))

	return content, nil
}

// deduplicateIssues removes duplicate issue numbers while preserving order
func deduplicateIssues(issues []int) []int {
	seen := make(map[int]bool)
	result := make([]int, 0, len(issues))
	for _, num := range issues {
		if !seen[num] {
			seen[num] = true
			result = append(result, num)
		}
	}
	return result
}

// =============================================================================
// Freeform Sub-Issue Parser
// =============================================================================

// ParseFreeformSubIssueBody parses a freeform sub-issue body that doesn't follow
// the Claudio template structure. This parser is more lenient and extracts what
// it can from human-authored issues.
//
// The function:
//   - Uses the entire body as description if no "## Task" section is found
//   - Extracts file paths from backticks anywhere in the body
//   - Looks for issue references (#N) anywhere for dependencies
//   - Defaults complexity to "medium" if not specified
//   - parentIssueNum is optional (pass 0 if unknown)
//
// This is designed to handle issues that might have:
//   - Description anywhere in the body (not necessarily under '## Task')
//   - File references in various formats (code blocks, bullet lists, inline)
//   - Dependencies mentioned as issue references anywhere
//   - No explicit complexity section
func ParseFreeformSubIssueBody(body string, parentIssueNum int) (*SubIssueContent, error) {
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("sub-issue body is empty")
	}

	content := &SubIssueContent{
		ParentIssueNum: parentIssueNum,
	}

	// Try to extract description from "## Task" section first
	description, err := parseTaskSection(body)
	if err != nil {
		// No "## Task" section found - use the entire body as description
		description = extractFreeformDescription(body)
	}

	if description == "" {
		return nil, fmt.Errorf("could not extract description from issue body")
	}
	content.Description = description

	// Extract file paths from backticks anywhere in the body
	content.Files = extractFilesFromBody(body)

	// Extract issue references (#N) from anywhere in the body, excluding the parent
	content.DependencyIssueNums = extractIssueReferences(body, parentIssueNum)

	// Try to extract complexity, default to "medium" if not found
	content.Complexity = extractComplexityOrDefault(body)

	return content, nil
}

// extractFreeformDescription extracts a description from a freeform issue body.
// It tries several strategies:
//  1. If there are markdown headers (## or ###), use content before the first header
//     or after a "Description" header if one exists
//  2. Otherwise, use the entire body (with some cleanup)
func extractFreeformDescription(body string) string {
	lines := strings.Split(body, "\n")

	// Check if there's a "Description" or similar header
	descriptionHeaderRe := regexp.MustCompile(`(?i)^##?\s*(Description|Overview|Summary|About)\s*$`)

	var descriptionLines []string
	inDescriptionSection := false
	hasHeaders := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for any markdown header
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			hasHeaders = true

			// Check if this is a description header
			if descriptionHeaderRe.MatchString(trimmed) {
				inDescriptionSection = true
				continue
			}

			// If we were in a description section, stop
			if inDescriptionSection {
				break
			}
			continue
		}

		// If we found a description header, collect lines
		if inDescriptionSection {
			descriptionLines = append(descriptionLines, line)
			continue
		}
	}

	// If we found a description section, use it
	if len(descriptionLines) > 0 {
		return strings.TrimSpace(strings.Join(descriptionLines, "\n"))
	}

	// If there are no headers, use the entire body (cleaned up)
	if !hasHeaders {
		return cleanDescription(body)
	}

	// If there are headers but no description section, use content before the first header
	var beforeHeaders []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			break
		}
		beforeHeaders = append(beforeHeaders, line)
	}

	result := strings.TrimSpace(strings.Join(beforeHeaders, "\n"))
	if result != "" {
		return result
	}

	// Fallback: use the entire body cleaned up
	return cleanDescription(body)
}

// cleanDescription cleans up a description by removing certain patterns
// that are clearly not part of the main description
func cleanDescription(body string) string {
	// Remove "*Part of #N*" patterns
	cleanPartOfRe := regexp.MustCompile(`\*Part\s+of\s+#\d+\*`)
	cleaned := cleanPartOfRe.ReplaceAllString(body, "")

	// Remove horizontal rules
	hrRe := regexp.MustCompile(`(?m)^---+\s*$`)
	cleaned = hrRe.ReplaceAllString(cleaned, "")

	// Collapse multiple newlines
	multiNewlineRe := regexp.MustCompile(`\n{3,}`)
	cleaned = multiNewlineRe.ReplaceAllString(cleaned, "\n\n")

	return strings.TrimSpace(cleaned)
}

// filePathRe matches potential file paths in backticks
// Matches patterns like `path/to/file.ext` or `file.ext`
var filePathRe = regexp.MustCompile("`([^`]+\\.[a-zA-Z0-9]+)`")

// extractFilesFromBody extracts file paths from backticks anywhere in the body.
// It looks for patterns that look like file paths (contain a dot and extension).
func extractFilesFromBody(body string) []string {
	matches := filePathRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	var files []string
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		path := strings.TrimSpace(match[1])
		if path == "" || seen[path] {
			continue
		}
		// Additional validation: should look like a file path
		// (contains path separator or is a simple filename with extension)
		if isLikelyFilePath(path) {
			files = append(files, path)
			seen[path] = true
		}
	}

	return files
}

// isLikelyFilePath checks if a string looks like a file path rather than
// other code that might be in backticks (like function names or commands)
func isLikelyFilePath(s string) bool {
	// Must contain a dot (for extension)
	if !strings.Contains(s, ".") {
		return false
	}

	// Should not look like a function call
	if strings.Contains(s, "(") {
		return false
	}

	// Should not be a URL
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return false
	}

	// Common file extensions we expect
	commonExts := []string{
		".go", ".js", ".ts", ".tsx", ".jsx", ".py", ".rb", ".java",
		".md", ".json", ".yaml", ".yml", ".toml", ".xml", ".html", ".css",
		".sh", ".bash", ".zsh", ".sql", ".proto", ".graphql",
		".txt", ".env", ".mod", ".sum", ".lock",
	}

	lowerS := strings.ToLower(s)
	for _, ext := range commonExts {
		if strings.HasSuffix(lowerS, ext) {
			return true
		}
	}

	// Also accept paths with common directory patterns
	if strings.Contains(s, "/") {
		parts := strings.Split(s, "/")
		lastPart := parts[len(parts)-1]
		// If the last part has an extension-like suffix, it's likely a file
		if strings.Contains(lastPart, ".") {
			return true
		}
	}

	return false
}

// issueRefRe matches issue references like #123
var issueRefRe = regexp.MustCompile(`#(\d+)`)

// extractIssueReferences extracts issue numbers from #N patterns anywhere in the body.
// It excludes the parent issue number to avoid circular dependencies.
func extractIssueReferences(body string, parentIssueNum int) []int {
	matches := issueRefRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	var issueNums []int
	seen := make(map[int]bool)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		num, err := strconv.Atoi(match[1])
		if err != nil || num <= 0 {
			continue
		}
		// Skip the parent issue number
		if num == parentIssueNum {
			continue
		}
		if !seen[num] {
			issueNums = append(issueNums, num)
			seen[num] = true
		}
	}

	return issueNums
}

// extractComplexityOrDefault tries to extract complexity from the body.
// Returns "medium" if no complexity is found.
func extractComplexityOrDefault(body string) string {
	// Try the templated format first: Estimated: **low/medium/high**
	complexity, err := parseComplexitySection(body)
	if err == nil {
		return complexity
	}

	// Try alternate patterns
	// Pattern: "complexity: low/medium/high" (case-insensitive)
	complexityAltRe := regexp.MustCompile(`(?i)complexity[:\s]+\*?\*?(low|medium|high)\*?\*?`)
	if matches := complexityAltRe.FindStringSubmatch(body); len(matches) >= 2 {
		return strings.ToLower(matches[1])
	}

	// Pattern: just "low/medium/high complexity" in text
	complexityWordRe := regexp.MustCompile(`(?i)\b(low|medium|high)\s+complexity\b`)
	if matches := complexityWordRe.FindStringSubmatch(body); len(matches) >= 2 {
		return strings.ToLower(matches[1])
	}

	// Default to medium
	return "medium"
}

// =============================================================================
// Issue Format Detection
// =============================================================================

// IssueFormat represents the detected format of a GitHub issue body.
type IssueFormat string

const (
	// IssueFormatTemplated indicates an issue created by Claudio's template system.
	// These issues have structured sections like "## Task", "## Complexity", and "*Part of #N*".
	IssueFormatTemplated IssueFormat = "templated"

	// IssueFormatFreeform indicates a human-authored issue without Claudio's template structure.
	// These issues may have markdown sections but lack the templated markers.
	IssueFormatFreeform IssueFormat = "freeform"
)

// Regex patterns for detecting issue format
var (
	// partOfRe matches the "*Part of #N*" pattern used in templated sub-issues
	partOfRe = regexp.MustCompile(`\*Part\s+of\s+#\d+\*`)

	// taskSectionRe matches the "## Task" header used in templated sub-issues
	taskSectionRe = regexp.MustCompile(`(?m)^##\s*Task\s*$`)

	// complexitySectionRe matches the "Estimated: **low/medium/high**" pattern
	complexitySectionRe = regexp.MustCompile(`(?i)Estimated:\s*\*\*(?:low|medium|high)\*\*`)
)

// DetectIssueFormat analyzes a GitHub issue body and determines whether it follows
// the Claudio template format or is a freeform human-authored issue.
//
// The detection logic looks for signature patterns:
//   - Templated format: Has "*Part of #N*" marker AND "## Task" section
//   - Freeform format: Everything else (including issues with "## Summary" but no templated markers)
//
// This detection is used to route parsing to the appropriate parser in the ingestion pipeline.
func DetectIssueFormat(body string) IssueFormat {
	// Empty bodies are treated as freeform
	if strings.TrimSpace(body) == "" {
		return IssueFormatFreeform
	}

	// Check for templated format markers
	hasPartOf := partOfRe.MatchString(body)
	hasTaskSection := taskSectionRe.MatchString(body)
	hasComplexity := complexitySectionRe.MatchString(body)

	// An issue is considered templated if it has the "*Part of #N*" marker
	// AND at least one of the other templated section indicators.
	// This ensures we correctly identify Claudio-generated sub-issues while
	// avoiding false positives from human-written issues that might accidentally
	// contain similar text.
	if hasPartOf && (hasTaskSection || hasComplexity) {
		return IssueFormatTemplated
	}

	// Everything else is considered freeform
	return IssueFormatFreeform
}

// ParseSubIssueBodyAuto is a unified entry point that auto-detects the issue format
// and delegates to the appropriate parser. This simplifies the ingestion pipeline
// by providing a single function that handles both templated and freeform sub-issues.
//
// Parameters:
//   - body: The markdown body of the sub-issue to parse
//   - parentIssueNum: The parent issue number (used for freeform parsing to exclude
//     the parent from dependencies; pass 0 if unknown)
//
// The function:
//   - Detects the format using DetectIssueFormat
//   - Delegates to ParseSubIssueBody for templated issues
//   - Delegates to ParseFreeformSubIssueBody for freeform issues
//
// Returns a SubIssueContent struct with consistent fields regardless of input format.
// For templated issues, the ParentIssueNum is extracted from the body.
// For freeform issues, the parentIssueNum parameter is used.
func ParseSubIssueBodyAuto(body string, parentIssueNum int) (*SubIssueContent, error) {
	format := DetectIssueFormat(body)

	switch format {
	case IssueFormatTemplated:
		return ParseSubIssueBody(body)
	case IssueFormatFreeform:
		return ParseFreeformSubIssueBody(body, parentIssueNum)
	default:
		// This should never happen given the current implementation,
		// but we handle it gracefully by defaulting to freeform parsing
		return ParseFreeformSubIssueBody(body, parentIssueNum)
	}
}

// DetectParentIssueFormat determines whether a parent issue body is templated or freeform.
// Parent issues are templated if they have:
//   - "## Summary" section AND "## Sub-Issues" section with "### Group N" headers
//
// Parent issues are considered freeform if they:
//   - Use human-authored structure (e.g., "## Overview", "## Tasks", generic headers)
//   - Don't follow the strict Claudio parent template format
func DetectParentIssueFormat(body string) IssueFormat {
	if strings.TrimSpace(body) == "" {
		return IssueFormatFreeform
	}

	// Check for templated parent issue markers by scanning each line
	// (the regex patterns are line-anchored with ^)
	var hasSummarySection, hasSubIssuesSection, hasGroupHeaders bool
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if summaryHeaderRe.MatchString(trimmed) {
			hasSummarySection = true
		}
		if subIssuesHeaderRe.MatchString(trimmed) {
			hasSubIssuesSection = true
		}
		if groupHeaderRe.MatchString(trimmed) {
			hasGroupHeaders = true
		}
	}

	// A parent issue is templated if it has Summary + Sub-Issues sections with Group headers
	if hasSummarySection && hasSubIssuesSection && hasGroupHeaders {
		return IssueFormatTemplated
	}

	return IssueFormatFreeform
}

// ParseParentIssueBodyAuto is a unified entry point that auto-detects the parent issue
// format and delegates to the appropriate parser. This simplifies the ingestion pipeline
// by providing a single function that handles both templated and freeform parent issues.
//
// The function:
//   - Detects the format using DetectParentIssueFormat
//   - Delegates to ParseParentIssueBody for templated issues
//   - Delegates to ParseFreeformParentIssueBody for freeform issues
//
// Returns a ParentIssueContent struct with consistent fields regardless of input format.
func ParseParentIssueBodyAuto(body string) (*ParentIssueContent, error) {
	format := DetectParentIssueFormat(body)

	switch format {
	case IssueFormatTemplated:
		return ParseParentIssueBody(body)
	case IssueFormatFreeform:
		return ParseFreeformParentIssueBody(body)
	default:
		// Default to freeform parsing for robustness
		return ParseFreeformParentIssueBody(body)
	}
}
