package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// GitHubIssue represents a GitHub issue with the fields needed for plan ingestion.
type GitHubIssue struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
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
}

// CommandExecutor is a function type that executes a command and returns its output.
// This allows for dependency injection in tests.
type CommandExecutor func(name string, args ...string) ([]byte, error)

// defaultExecutor runs commands using os/exec.
var defaultExecutor CommandExecutor = func(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// ErrGHNotInstalled indicates that the gh CLI tool is not installed or not in PATH.
var ErrGHNotInstalled = errors.New("gh CLI is not installed or not in PATH")

// ErrGHAuthRequired indicates that gh CLI requires authentication.
var ErrGHAuthRequired = errors.New("gh CLI requires authentication (run 'gh auth login')")

// ErrIssueNotFound indicates that the requested issue does not exist.
var ErrIssueNotFound = errors.New("issue not found")

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
		"--json", "number,title,body,labels",
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
	}, nil
}

// classifyGHError analyzes the error and output from a gh command
// and returns a more specific error type when possible.
func classifyGHError(err error, output []byte, issueNum int) error {
	outStr := strings.ToLower(string(output))

	// Check for "executable file not found" which indicates gh is not installed
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return ErrGHNotInstalled
	}

	// Check for common error patterns in output
	switch {
	case strings.Contains(outStr, "not logged in") ||
		strings.Contains(outStr, "authentication required") ||
		strings.Contains(outStr, "gh auth login"):
		return ErrGHAuthRequired

	case strings.Contains(outStr, "could not find issue") ||
		strings.Contains(outStr, "issue not found") ||
		strings.Contains(outStr, "not found"):
		return fmt.Errorf("%w: #%d", ErrIssueNotFound, issueNum)

	case strings.Contains(outStr, "could not resolve to a repository"):
		return fmt.Errorf("repository not found or not accessible")
	}

	// Return the original error with output for debugging
	return fmt.Errorf("gh command failed: %w\n%s", err, string(output))
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
