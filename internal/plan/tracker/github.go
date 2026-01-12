package tracker

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// CommandExecutor is a function type that executes a command and returns its output.
// This allows for dependency injection in tests.
type CommandExecutor func(name string, args ...string) ([]byte, error)

// defaultExecutor runs commands using os/exec.
var defaultExecutor CommandExecutor = func(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// GitHubTracker implements IssueTracker for GitHub using the gh CLI.
type GitHubTracker struct {
	executor CommandExecutor
}

// NewGitHubTracker creates a new GitHubTracker using the default command executor.
func NewGitHubTracker() *GitHubTracker {
	return &GitHubTracker{
		executor: defaultExecutor,
	}
}

// NewGitHubTrackerWithExecutor creates a new GitHubTracker with a custom command
// executor for testing.
func NewGitHubTrackerWithExecutor(executor CommandExecutor) *GitHubTracker {
	return &GitHubTracker{
		executor: executor,
	}
}

// CreateIssue creates a GitHub issue using the gh CLI.
func (g *GitHubTracker) CreateIssue(opts IssueOptions) (IssueRef, error) {
	if opts.Title == "" {
		return IssueRef{}, fmt.Errorf("issue title is required")
	}

	args := []string{"issue", "create",
		"--title", opts.Title,
		"--body", opts.Body,
	}

	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}

	output, err := g.executor("gh", args...)
	if err != nil {
		return IssueRef{}, g.classifyError(err, output)
	}

	url := strings.TrimSpace(string(output))
	num, err := parseIssueNumber(url)
	if err != nil {
		return IssueRef{URL: url}, err
	}

	// Get the GraphQL node ID for the newly created issue
	nodeID, err := g.getIssueNodeID(num)
	if err != nil {
		// Return partial result - issue was created but node ID retrieval failed.
		// Operations requiring node ID (like AddSubIssue) will fetch it on demand.
		fmt.Fprintf(os.Stderr, "Warning: issue #%d created but failed to get node ID: %v\n", num, err)
		return IssueRef{Number: num, URL: url}, nil
	}

	return IssueRef{
		ID:     nodeID,
		Number: num,
		URL:    url,
	}, nil
}

// UpdateIssue updates a GitHub issue's body using the gh CLI.
func (g *GitHubTracker) UpdateIssue(ref IssueRef, opts IssueOptions) error {
	if ref.Number <= 0 {
		return fmt.Errorf("issue number is required for update")
	}

	args := []string{"issue", "edit", strconv.Itoa(ref.Number)}

	if opts.Body != "" {
		args = append(args, "--body", opts.Body)
	}
	if opts.Title != "" {
		args = append(args, "--title", opts.Title)
	}

	output, err := g.executor("gh", args...)
	if err != nil {
		return g.classifyError(err, output)
	}

	return nil
}

// AddSubIssue links a sub-issue to a parent issue using GitHub's GraphQL API.
func (g *GitHubTracker) AddSubIssue(parentRef, subIssueRef IssueRef) error {
	// If we don't have node IDs, we need to fetch them
	parentNodeID := parentRef.ID
	subNodeID := subIssueRef.ID

	var err error
	if parentNodeID == "" {
		parentNodeID, err = g.getIssueNodeID(parentRef.Number)
		if err != nil {
			return fmt.Errorf("failed to get parent issue node ID: %w", err)
		}
	}
	if subNodeID == "" {
		subNodeID, err = g.getIssueNodeID(subIssueRef.Number)
		if err != nil {
			return fmt.Errorf("failed to get sub-issue node ID: %w", err)
		}
	}

	query := fmt.Sprintf(`mutation {
		addSubIssue(input: {issueId: "%s", subIssueId: "%s"}) {
			issue { number }
			subIssue { number }
		}
	}`, parentNodeID, subNodeID)

	output, err := g.executor("gh", "api", "graphql", "-f", "query="+query)
	if err != nil {
		return g.classifyError(err, output)
	}

	// Verify the mutation succeeded by checking for errors in response
	var response struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return fmt.Errorf("failed to parse GraphQL response: %w", err)
	}
	if len(response.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", response.Errors[0].Message)
	}

	return nil
}

// RemoveSubIssue removes a sub-issue link from a parent issue.
func (g *GitHubTracker) RemoveSubIssue(parentRef, subIssueRef IssueRef) error {
	// If we don't have node IDs, we need to fetch them
	parentNodeID := parentRef.ID
	subNodeID := subIssueRef.ID

	var err error
	if parentNodeID == "" {
		parentNodeID, err = g.getIssueNodeID(parentRef.Number)
		if err != nil {
			return fmt.Errorf("failed to get parent issue node ID: %w", err)
		}
	}
	if subNodeID == "" {
		subNodeID, err = g.getIssueNodeID(subIssueRef.Number)
		if err != nil {
			return fmt.Errorf("failed to get sub-issue node ID: %w", err)
		}
	}

	query := fmt.Sprintf(`mutation {
		removeSubIssue(input: {issueId: "%s", subIssueId: "%s"}) {
			issue { number }
			subIssue { number }
		}
	}`, parentNodeID, subNodeID)

	output, err := g.executor("gh", "api", "graphql", "-f", "query="+query)
	if err != nil {
		return g.classifyError(err, output)
	}

	// Verify the mutation succeeded by checking for errors in response
	var response struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return fmt.Errorf("failed to parse GraphQL response: %w", err)
	}
	if len(response.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", response.Errors[0].Message)
	}

	return nil
}

// SupportsHierarchy returns true as GitHub supports sub-issues.
func (g *GitHubTracker) SupportsHierarchy() bool {
	return true
}

// SupportsLabels returns true as GitHub supports issue labels.
func (g *GitHubTracker) SupportsLabels() bool {
	return true
}

// getIssueNodeID retrieves the GraphQL node ID for a GitHub issue.
func (g *GitHubTracker) getIssueNodeID(issueNumber int) (string, error) {
	output, err := g.executor("gh", "issue", "view", strconv.Itoa(issueNumber), "--json", "id")
	if err != nil {
		return "", g.classifyError(err, output)
	}

	var response struct {
		ID string `json:"id"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if response.ID == "" {
		return "", fmt.Errorf("no node ID found for issue #%d", issueNumber)
	}

	return response.ID, nil
}

// GetIssueNodeID is a public wrapper for getting issue node IDs.
// This is useful for callers that need to populate IssueRef.ID values.
func (g *GitHubTracker) GetIssueNodeID(issueNumber int) (string, error) {
	return g.getIssueNodeID(issueNumber)
}

// classifyError analyzes the error and output from a gh command
// and returns a more specific error type when possible.
// Errors are wrapped to preserve context while enabling errors.Is() checks.
func (g *GitHubTracker) classifyError(err error, output []byte) error {
	outStr := strings.ToLower(string(output))

	// Check for "executable file not found" which indicates gh is not installed
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return fmt.Errorf("%w: %v", ErrProviderUnavailable, execErr)
	}

	// Check for common error patterns in output
	switch {
	case strings.Contains(outStr, "not logged in") ||
		strings.Contains(outStr, "authentication required") ||
		strings.Contains(outStr, "gh auth login"):
		return fmt.Errorf("%w: %s", ErrAuthRequired, strings.TrimSpace(string(output)))

	case strings.Contains(outStr, "could not find issue") ||
		strings.Contains(outStr, "issue not found"):
		// Only match issue-specific "not found" patterns to avoid false positives
		return fmt.Errorf("%w: %s", ErrIssueNotFound, strings.TrimSpace(string(output)))

	case strings.Contains(outStr, "could not resolve to a repository"):
		return fmt.Errorf("repository not found or not accessible: %s", strings.TrimSpace(string(output)))
	}

	// Return the original error with output for debugging
	return fmt.Errorf("gh command failed: %w\n%s", err, string(output))
}

// parseIssueNumber extracts issue number from gh output URL
// e.g., https://github.com/owner/repo/issues/123
func parseIssueNumber(output string) (int, error) {
	re := regexp.MustCompile(`/issues/(\d+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not parse issue number from: %s", output)
	}

	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid issue number: %w", err)
	}

	return num, nil
}

// Ensure GitHubTracker implements IssueTracker
var _ IssueTracker = (*GitHubTracker)(nil)
