// Package tracker provides an abstraction for issue tracking systems.
// It defines the IssueTracker interface that enables support for multiple
// project management backends (GitHub, Linear, Notion, Jira, GitLab).
package tracker

// IssueRef is an opaque reference to an issue in an issue tracking system.
// Different providers may use different identifier schemes (numbers, UUIDs, etc.).
type IssueRef struct {
	// ID is the provider-specific unique identifier.
	// For GitHub, this is the GraphQL node ID (e.g., "I_kwDOA...").
	// For Linear, this would be the issue UUID.
	ID string

	// Number is a human-readable issue number when applicable.
	// For GitHub, this is the issue number (e.g., 42).
	// Some providers may not use numbered issues.
	Number int

	// URL is the web URL to view the issue.
	URL string
}

// IssueOptions contains the parameters for creating or updating an issue.
type IssueOptions struct {
	// Title is the issue title (required for creation).
	Title string

	// Body is the issue description/body in markdown.
	Body string

	// Labels are tags/labels to apply to the issue.
	// Not all providers support labels.
	Labels []string
}

// IssueTracker defines the interface for issue tracking system operations.
// Implementations handle the provider-specific API calls (gh CLI, GraphQL, REST, etc.).
type IssueTracker interface {
	// CreateIssue creates a new issue and returns its reference.
	CreateIssue(opts IssueOptions) (IssueRef, error)

	// UpdateIssue updates an existing issue's content.
	// The ref parameter identifies the issue to update.
	UpdateIssue(ref IssueRef, opts IssueOptions) error

	// AddSubIssue links a sub-issue to a parent issue, creating a hierarchy.
	// Not all providers support hierarchical issues.
	// Returns ErrHierarchyNotSupported if the provider doesn't support this.
	AddSubIssue(parentRef, subIssueRef IssueRef) error

	// RemoveSubIssue removes a sub-issue link from a parent issue.
	// Not all providers support hierarchical issues.
	// Returns ErrHierarchyNotSupported if the provider doesn't support this.
	RemoveSubIssue(parentRef, subIssueRef IssueRef) error

	// SupportsHierarchy returns true if the provider supports parent-child
	// issue relationships (e.g., GitHub sub-issues, Linear parent issues).
	SupportsHierarchy() bool

	// SupportsLabels returns true if the provider supports issue labels/tags.
	SupportsLabels() bool
}
