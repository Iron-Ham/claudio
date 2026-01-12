package plan

import (
	"github.com/Iron-Ham/claudio/internal/plan/tracker"
)

// IssueOptions contains options for issue creation.
// This type is maintained for backward compatibility with existing code.
type IssueOptions struct {
	Title  string
	Body   string
	Labels []string
}

// IssueCreationResult holds the results of creating all issues
type IssueCreationResult struct {
	ParentIssueNumber int
	ParentIssueURL    string
	SubIssueNumbers   map[string]int    // task_id -> issue_number
	SubIssueURLs      map[string]string // task_id -> issue_url
	GroupIssueNumbers []int             // group issue numbers (for groups with >1 task)
	GroupIssueURLs    []string          // group issue URLs
}

// defaultTracker is the default IssueTracker used by the legacy functions.
var defaultTracker = tracker.NewGitHubTracker()

// CreateIssue creates a GitHub issue using the gh CLI.
// Returns the issue number and URL on success.
//
// Deprecated: Use tracker.GitHubTracker.CreateIssue for new code.
// This function is maintained for backward compatibility.
func CreateIssue(opts IssueOptions) (int, string, error) {
	ref, err := defaultTracker.CreateIssue(tracker.IssueOptions{
		Title:  opts.Title,
		Body:   opts.Body,
		Labels: opts.Labels,
	})
	if err != nil {
		return 0, "", err
	}
	return ref.Number, ref.URL, nil
}

// UpdateIssueBody updates an existing issue's body.
// This is used to update the parent issue after all sub-issues are created.
//
// Deprecated: Use tracker.GitHubTracker.UpdateIssue for new code.
// This function is maintained for backward compatibility.
func UpdateIssueBody(issueNumber int, newBody string) error {
	return defaultTracker.UpdateIssue(
		tracker.IssueRef{Number: issueNumber},
		tracker.IssueOptions{Body: newBody},
	)
}

// GetIssueNodeID retrieves the GraphQL node ID for a GitHub issue.
// This is required for the addSubIssue GraphQL mutation.
//
// Deprecated: Use tracker.GitHubTracker.GetIssueNodeID for new code.
// This function is maintained for backward compatibility.
func GetIssueNodeID(issueNumber int) (string, error) {
	return defaultTracker.GetIssueNodeID(issueNumber)
}

// AddSubIssue links a sub-issue to a parent issue using GitHub's GraphQL API.
// This creates a proper parent-child relationship visible in the GitHub UI.
//
// Deprecated: Use tracker.GitHubTracker.AddSubIssue for new code.
// This function is maintained for backward compatibility.
func AddSubIssue(parentNodeID, subIssueNodeID string) error {
	return defaultTracker.AddSubIssue(
		tracker.IssueRef{ID: parentNodeID},
		tracker.IssueRef{ID: subIssueNodeID},
	)
}
