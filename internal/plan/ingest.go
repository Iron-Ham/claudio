package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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
