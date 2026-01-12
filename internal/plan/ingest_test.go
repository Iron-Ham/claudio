package plan

import (
	"errors"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// =============================================================================
// GitHub Issue Fetcher Tests (task-2-fetch-issue)
// =============================================================================

// mockExecutor creates a CommandExecutor that returns predefined output or error.
func mockExecutor(output []byte, err error) CommandExecutor {
	return func(name string, args ...string) ([]byte, error) {
		return output, err
	}
}

// mockExecutorWithValidation creates a CommandExecutor that validates the arguments
// and returns predefined output.
func mockExecutorWithValidation(
	t *testing.T,
	expectedArgs []string,
	output []byte,
	err error,
) CommandExecutor {
	return func(name string, args ...string) ([]byte, error) {
		if name != "gh" {
			t.Errorf("expected command 'gh', got %q", name)
		}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("args mismatch:\ngot:  %v\nwant: %v", args, expectedArgs)
		}
		return output, err
	}
}

func TestFetchIssue_Success(t *testing.T) {
	// Simulate successful gh CLI response
	jsonResponse := `{
		"number": 42,
		"title": "Test Issue Title",
		"body": "This is the issue body with **markdown**.",
		"labels": [
			{"name": "bug"},
			{"name": "priority-high"}
		]
	}`

	executor := mockExecutorWithValidation(t,
		[]string{"issue", "view", "42", "--repo", "owner/repo", "--json", "number,title,body,labels,url,state"},
		[]byte(jsonResponse),
		nil,
	)

	issue, err := fetchIssueWithExecutor("owner", "repo", 42, executor)
	if err != nil {
		t.Fatalf("FetchIssue() unexpected error: %v", err)
	}

	if issue.Number != 42 {
		t.Errorf("Number = %d, want 42", issue.Number)
	}
	if issue.Title != "Test Issue Title" {
		t.Errorf("Title = %q, want %q", issue.Title, "Test Issue Title")
	}
	if issue.Body != "This is the issue body with **markdown**." {
		t.Errorf("Body = %q, want %q", issue.Body, "This is the issue body with **markdown**.")
	}
	expectedLabels := []string{"bug", "priority-high"}
	if !reflect.DeepEqual(issue.Labels, expectedLabels) {
		t.Errorf("Labels = %v, want %v", issue.Labels, expectedLabels)
	}
}

func TestFetchIssue_NoLabels(t *testing.T) {
	jsonResponse := `{
		"number": 1,
		"title": "Simple Issue",
		"body": "Body text",
		"labels": []
	}`

	executor := mockExecutor([]byte(jsonResponse), nil)
	issue, err := fetchIssueWithExecutor("owner", "repo", 1, executor)
	if err != nil {
		t.Fatalf("FetchIssue() unexpected error: %v", err)
	}

	if len(issue.Labels) != 0 {
		t.Errorf("Labels should be empty, got %v", issue.Labels)
	}
}

func TestFetchIssue_EmptyBody(t *testing.T) {
	jsonResponse := `{
		"number": 5,
		"title": "Issue with empty body",
		"body": "",
		"labels": []
	}`

	executor := mockExecutor([]byte(jsonResponse), nil)
	issue, err := fetchIssueWithExecutor("owner", "repo", 5, executor)
	if err != nil {
		t.Fatalf("FetchIssue() unexpected error: %v", err)
	}

	if issue.Body != "" {
		t.Errorf("Body should be empty, got %q", issue.Body)
	}
}

func TestFetchIssue_ValidationErrors(t *testing.T) {
	executor := mockExecutor(nil, nil)

	tests := []struct {
		name      string
		owner     string
		repo      string
		issueNum  int
		wantError string
	}{
		{
			name:      "empty owner",
			owner:     "",
			repo:      "repo",
			issueNum:  1,
			wantError: "owner cannot be empty",
		},
		{
			name:      "empty repo",
			owner:     "owner",
			repo:      "",
			issueNum:  1,
			wantError: "repo cannot be empty",
		},
		{
			name:      "zero issue number",
			owner:     "owner",
			repo:      "repo",
			issueNum:  0,
			wantError: "issue number must be positive",
		},
		{
			name:      "negative issue number",
			owner:     "owner",
			repo:      "repo",
			issueNum:  -1,
			wantError: "issue number must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fetchIssueWithExecutor(tt.owner, tt.repo, tt.issueNum, executor)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantError)
			}
		})
	}
}

func TestFetchIssue_GHNotInstalled(t *testing.T) {
	// Simulate exec.Error which occurs when the executable is not found
	execErr := &exec.Error{
		Name: "gh",
		Err:  errors.New("executable file not found in $PATH"),
	}
	executor := mockExecutor(nil, execErr)

	_, err := fetchIssueWithExecutor("owner", "repo", 1, executor)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrGHNotInstalled) {
		t.Errorf("error = %v, want ErrGHNotInstalled", err)
	}
}

func TestFetchIssue_AuthRequired(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "not logged in message",
			output: "error: not logged in to any GitHub hosts",
		},
		{
			name:   "gh auth login prompt",
			output: "To authenticate, please run `gh auth login`",
		},
		{
			name:   "authentication required",
			output: "error: authentication required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := mockExecutor([]byte(tt.output), errors.New("exit status 1"))

			_, err := fetchIssueWithExecutor("owner", "repo", 1, executor)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, ErrGHAuthRequired) {
				t.Errorf("error = %v, want ErrGHAuthRequired", err)
			}
		})
	}
}

func TestFetchIssue_IssueNotFound(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "could not find issue",
			output: "GraphQL: Could not find issue #9999",
		},
		{
			name:   "issue not found",
			output: "issue not found",
		},
		{
			name:   "generic not found",
			output: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := mockExecutor([]byte(tt.output), errors.New("exit status 1"))

			_, err := fetchIssueWithExecutor("owner", "repo", 9999, executor)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, ErrIssueNotFound) {
				t.Errorf("error = %v, want ErrIssueNotFound", err)
			}
		})
	}
}

func TestFetchIssue_RepoNotFound(t *testing.T) {
	output := "GraphQL: Could not resolve to a Repository"
	executor := mockExecutor([]byte(output), errors.New("exit status 1"))

	_, err := fetchIssueWithExecutor("nonexistent", "repo", 1, executor)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "repository not found") {
		t.Errorf("error = %q, want to contain 'repository not found'", err.Error())
	}
}

func TestFetchIssue_InvalidJSON(t *testing.T) {
	executor := mockExecutor([]byte("not valid json"), nil)

	_, err := fetchIssueWithExecutor("owner", "repo", 1, executor)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse gh output") {
		t.Errorf("error = %q, want to contain 'failed to parse gh output'", err.Error())
	}
}

func TestFetchIssue_GenericError(t *testing.T) {
	// Test that unrecognized errors are wrapped with output
	output := "some unexpected error message"
	executor := mockExecutor([]byte(output), errors.New("exit status 1"))

	_, err := fetchIssueWithExecutor("owner", "repo", 1, executor)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "gh command failed") {
		t.Errorf("error = %q, want to contain 'gh command failed'", err.Error())
	}
	if !strings.Contains(err.Error(), output) {
		t.Errorf("error = %q, want to contain output %q", err.Error(), output)
	}
}

func TestFetchIssue_LargeIssueNumber(t *testing.T) {
	jsonResponse := `{
		"number": 99999,
		"title": "Large number issue",
		"body": "Body",
		"labels": []
	}`

	executor := mockExecutorWithValidation(t,
		[]string{"issue", "view", "99999", "--repo", "org/project", "--json", "number,title,body,labels,url,state"},
		[]byte(jsonResponse),
		nil,
	)

	issue, err := fetchIssueWithExecutor("org", "project", 99999, executor)
	if err != nil {
		t.Fatalf("FetchIssue() unexpected error: %v", err)
	}
	if issue.Number != 99999 {
		t.Errorf("Number = %d, want 99999", issue.Number)
	}
}

func TestFetchIssue_SpecialCharactersInOwnerRepo(t *testing.T) {
	jsonResponse := `{
		"number": 1,
		"title": "Test",
		"body": "Body",
		"labels": []
	}`

	// Test with hyphens, underscores, and dots which are valid in GitHub owner/repo names
	executor := mockExecutorWithValidation(t,
		[]string{"issue", "view", "1", "--repo", "Iron-Ham/my_repo.js", "--json", "number,title,body,labels,url,state"},
		[]byte(jsonResponse),
		nil,
	)

	issue, err := fetchIssueWithExecutor("Iron-Ham", "my_repo.js", 1, executor)
	if err != nil {
		t.Fatalf("FetchIssue() unexpected error: %v", err)
	}
	if issue.Number != 1 {
		t.Errorf("Number = %d, want 1", issue.Number)
	}
}

func TestFetchIssue_ManyLabels(t *testing.T) {
	jsonResponse := `{
		"number": 10,
		"title": "Multi-label issue",
		"body": "Body",
		"labels": [
			{"name": "bug"},
			{"name": "enhancement"},
			{"name": "documentation"},
			{"name": "good first issue"},
			{"name": "help wanted"}
		]
	}`

	executor := mockExecutor([]byte(jsonResponse), nil)
	issue, err := fetchIssueWithExecutor("owner", "repo", 10, executor)
	if err != nil {
		t.Fatalf("FetchIssue() unexpected error: %v", err)
	}

	expectedLabels := []string{"bug", "enhancement", "documentation", "good first issue", "help wanted"}
	if !reflect.DeepEqual(issue.Labels, expectedLabels) {
		t.Errorf("Labels = %v, want %v", issue.Labels, expectedLabels)
	}
}

func TestFetchIssue_BodyWithSpecialCharacters(t *testing.T) {
	// Test that markdown and special characters in body are preserved
	jsonResponse := `{
		"number": 7,
		"title": "Special chars",
		"body": "## Summary\n\n- Item 1\n- Item 2\n\n**Bold** and *italic*\n\n` + "`code`" + `\n\n> quote",
		"labels": []
	}`

	executor := mockExecutor([]byte(jsonResponse), nil)
	issue, err := fetchIssueWithExecutor("owner", "repo", 7, executor)
	if err != nil {
		t.Fatalf("FetchIssue() unexpected error: %v", err)
	}

	if !strings.Contains(issue.Body, "## Summary") {
		t.Error("Body should contain markdown headers")
	}
	if !strings.Contains(issue.Body, "**Bold**") {
		t.Error("Body should contain bold markdown")
	}
}

// =============================================================================
// GitHub Issue URL Parser Tests (task-1-url-parser)
// =============================================================================

func TestParseGitHubIssueURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantNum   int
		wantErr   bool
	}{
		// Valid full URLs
		{
			name:      "basic https URL",
			url:       "https://github.com/owner/repo/issues/123",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantNum:   123,
			wantErr:   false,
		},
		{
			name:      "http URL",
			url:       "http://github.com/owner/repo/issues/456",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantNum:   456,
			wantErr:   false,
		},
		{
			name:      "URL with query params",
			url:       "https://github.com/owner/repo/issues/789?query=value",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantNum:   789,
			wantErr:   false,
		},
		{
			name:      "URL with anchor",
			url:       "https://github.com/owner/repo/issues/101#issuecomment-123456",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantNum:   101,
			wantErr:   false,
		},
		{
			name:      "URL with query and anchor",
			url:       "https://github.com/owner/repo/issues/202?q=test#anchor",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantNum:   202,
			wantErr:   false,
		},
		{
			name:      "real-world URL with hyphenated names",
			url:       "https://github.com/Iron-Ham/claudio/issues/42",
			wantOwner: "Iron-Ham",
			wantRepo:  "claudio",
			wantNum:   42,
			wantErr:   false,
		},
		{
			name:      "URL with underscore in names",
			url:       "https://github.com/my_org/my_repo/issues/1",
			wantOwner: "my_org",
			wantRepo:  "my_repo",
			wantNum:   1,
			wantErr:   false,
		},
		{
			name:      "URL with dots in repo name",
			url:       "https://github.com/owner/repo.js/issues/99",
			wantOwner: "owner",
			wantRepo:  "repo.js",
			wantNum:   99,
			wantErr:   false,
		},
		{
			name:      "large issue number",
			url:       "https://github.com/kubernetes/kubernetes/issues/99999",
			wantOwner: "kubernetes",
			wantRepo:  "kubernetes",
			wantNum:   99999,
			wantErr:   false,
		},

		// Valid shorthand format
		{
			name:      "basic shorthand",
			url:       "owner/repo#123",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantNum:   123,
			wantErr:   false,
		},
		{
			name:      "shorthand with hyphenated names",
			url:       "Iron-Ham/claudio#42",
			wantOwner: "Iron-Ham",
			wantRepo:  "claudio",
			wantNum:   42,
			wantErr:   false,
		},
		{
			name:      "shorthand with underscore",
			url:       "my_org/my_repo#1",
			wantOwner: "my_org",
			wantRepo:  "my_repo",
			wantNum:   1,
			wantErr:   false,
		},
		{
			name:      "shorthand with dots",
			url:       "owner/repo.js#99",
			wantOwner: "owner",
			wantRepo:  "repo.js",
			wantNum:   99,
			wantErr:   false,
		},

		// Invalid URLs
		{
			name:    "empty string",
			url:     "",
			wantErr: true,
		},
		{
			name:    "random string",
			url:     "not a url at all",
			wantErr: true,
		},
		{
			name:    "github URL without issue number",
			url:     "https://github.com/owner/repo/issues",
			wantErr: true,
		},
		{
			name:    "github repo URL (not issue)",
			url:     "https://github.com/owner/repo",
			wantErr: true,
		},
		{
			name:    "github pull request URL",
			url:     "https://github.com/owner/repo/pull/123",
			wantErr: true,
		},
		{
			name:    "gitlab URL",
			url:     "https://gitlab.com/owner/repo/issues/123",
			wantErr: true,
		},
		{
			name:    "URL with trailing slash only",
			url:     "https://github.com/owner/repo/issues/123/",
			wantErr: true,
		},
		{
			name:    "URL with extra path segment",
			url:     "https://github.com/owner/repo/issues/123/extra",
			wantErr: true,
		},
		{
			name:    "shorthand without issue number",
			url:     "owner/repo#",
			wantErr: true,
		},
		{
			name:    "shorthand with non-numeric issue",
			url:     "owner/repo#abc",
			wantErr: true,
		},
		{
			name:    "just a hash number",
			url:     "#123",
			wantErr: true,
		},
		{
			name:    "URL with spaces",
			url:     "https://github.com/owner/repo/issues/123 ",
			wantErr: true,
		},
		{
			name:    "URL with leading spaces",
			url:     " https://github.com/owner/repo/issues/123",
			wantErr: true,
		},
		{
			name:    "shorthand with spaces",
			url:     "owner / repo#123",
			wantErr: true,
		},
		{
			name:    "missing owner",
			url:     "/repo#123",
			wantErr: true,
		},
		{
			name:    "missing repo",
			url:     "owner/#123",
			wantErr: true,
		},
		{
			name:    "www subdomain",
			url:     "https://www.github.com/owner/repo/issues/123",
			wantErr: true,
		},
		{
			name:    "enterprise github URL",
			url:     "https://github.company.com/owner/repo/issues/123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, num, err := ParseGitHubIssueURL(tt.url)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseGitHubIssueURL(%q) expected error, got none", tt.url)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseGitHubIssueURL(%q) unexpected error: %v", tt.url, err)
				return
			}

			if owner != tt.wantOwner {
				t.Errorf("ParseGitHubIssueURL(%q) owner = %q, want %q", tt.url, owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("ParseGitHubIssueURL(%q) repo = %q, want %q", tt.url, repo, tt.wantRepo)
			}
			if num != tt.wantNum {
				t.Errorf("ParseGitHubIssueURL(%q) num = %d, want %d", tt.url, num, tt.wantNum)
			}
		})
	}
}

func TestIsGitHubIssueURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		// Valid URLs
		{"basic https URL", "https://github.com/owner/repo/issues/123", true},
		{"http URL", "http://github.com/owner/repo/issues/456", true},
		{"URL with query params", "https://github.com/owner/repo/issues/789?foo=bar", true},
		{"URL with anchor", "https://github.com/owner/repo/issues/101#comment", true},
		{"basic shorthand", "owner/repo#123", true},
		{"shorthand with hyphens", "Iron-Ham/claudio#42", true},

		// Invalid URLs
		{"empty string", "", false},
		{"random string", "not a url", false},
		{"github repo URL", "https://github.com/owner/repo", false},
		{"github PR URL", "https://github.com/owner/repo/pull/123", false},
		{"gitlab URL", "https://gitlab.com/owner/repo/issues/123", false},
		{"missing issue number", "owner/repo#", false},
		{"non-numeric issue", "owner/repo#abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGitHubIssueURL(tt.url)
			if got != tt.want {
				t.Errorf("IsGitHubIssueURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestParseGitHubIssueURL_ConsistencyWithIsGitHubIssueURL(t *testing.T) {
	// Ensure that IsGitHubIssueURL returns true iff ParseGitHubIssueURL succeeds
	urls := []string{
		"https://github.com/owner/repo/issues/123",
		"http://github.com/owner/repo/issues/456",
		"https://github.com/owner/repo/issues/789?query=value",
		"owner/repo#123",
		"Iron-Ham/claudio#42",
		"",
		"not a url",
		"https://github.com/owner/repo",
		"https://gitlab.com/owner/repo/issues/123",
	}

	for _, url := range urls {
		isValid := IsGitHubIssueURL(url)
		_, _, _, err := ParseGitHubIssueURL(url)
		parseSucceeded := err == nil

		if isValid != parseSucceeded {
			t.Errorf("Inconsistency for %q: IsGitHubIssueURL=%v but ParseGitHubIssueURL error=%v",
				url, isValid, err)
		}
	}
}

// =============================================================================
// Parent Issue Body Parser Tests (task-4-parse-parent-body)
// =============================================================================

func TestParseParentIssueBody_BasicTemplate(t *testing.T) {
	// This tests a body that matches the parentIssueBodyTemplate format
	body := `## Summary

This is a multi-line summary
that describes the plan.

## Analysis

- First insight about the codebase
- Second insight with more details

## Constraints

- Must maintain backward compatibility
- Limited time for implementation

## Sub-Issues

### Group 1 (can start immediately)
- [ ] #42 - **Setup infrastructure**
- [ ] #43 - **Create base types**

### Group 2 (depends on previous groups)
- [ ] #44 - **Implement main feature**

## Execution Order

Tasks are grouped by dependencies. All tasks within a group can be worked on in parallel.
Complete each group before starting the next.

## Acceptance Criteria

- [ ] All sub-issues completed
- [ ] Integration verified
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	// Verify summary
	expectedSummary := "This is a multi-line summary\nthat describes the plan."
	if content.Summary != expectedSummary {
		t.Errorf("Summary = %q, want %q", content.Summary, expectedSummary)
	}

	// Verify insights
	expectedInsights := []string{
		"First insight about the codebase",
		"Second insight with more details",
	}
	if !reflect.DeepEqual(content.Insights, expectedInsights) {
		t.Errorf("Insights = %v, want %v", content.Insights, expectedInsights)
	}

	// Verify constraints
	expectedConstraints := []string{
		"Must maintain backward compatibility",
		"Limited time for implementation",
	}
	if !reflect.DeepEqual(content.Constraints, expectedConstraints) {
		t.Errorf("Constraints = %v, want %v", content.Constraints, expectedConstraints)
	}

	// Verify execution groups
	expectedGroups := [][]int{{42, 43}, {44}}
	if !reflect.DeepEqual(content.ExecutionGroups, expectedGroups) {
		t.Errorf("ExecutionGroups = %v, want %v", content.ExecutionGroups, expectedGroups)
	}
}

func TestParseParentIssueBody_RoundTrip(t *testing.T) {
	// Create a plan and render it, then parse it back
	plan := &orchestrator.PlanSpec{
		Objective: "Implement new authentication system",
		Summary:   "Add OAuth2 support with multiple providers",
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "Create auth models"},
			{ID: "task-2", Title: "Implement token service"},
			{ID: "task-3", Title: "Add OAuth endpoints", DependsOn: []string{"task-1", "task-2"}},
		},
		ExecutionOrder: [][]string{{"task-1", "task-2"}, {"task-3"}},
		Insights: []string{
			"Existing user table can be extended",
			"JWT library already in dependencies",
		},
		Constraints: []string{
			"Must support existing session cookies",
		},
	}

	subIssueNumbers := map[string]int{
		"task-1": 100,
		"task-2": 101,
		"task-3": 102,
	}

	// Render the body
	body, err := RenderParentIssueBody(plan, subIssueNumbers)
	if err != nil {
		t.Fatalf("RenderParentIssueBody() error = %v", err)
	}

	// Parse it back
	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	// Verify round-trip values
	if content.Summary != plan.Summary {
		t.Errorf("Summary = %q, want %q", content.Summary, plan.Summary)
	}

	if !reflect.DeepEqual(content.Insights, plan.Insights) {
		t.Errorf("Insights = %v, want %v", content.Insights, plan.Insights)
	}

	if !reflect.DeepEqual(content.Constraints, plan.Constraints) {
		t.Errorf("Constraints = %v, want %v", content.Constraints, plan.Constraints)
	}

	// Verify execution groups match issue numbers
	expectedGroups := [][]int{{100, 101}, {102}}
	if !reflect.DeepEqual(content.ExecutionGroups, expectedGroups) {
		t.Errorf("ExecutionGroups = %v, want %v", content.ExecutionGroups, expectedGroups)
	}
}

func TestParseParentIssueBody_EmptySections(t *testing.T) {
	body := `## Summary

Just a summary.

## Sub-Issues

### Group 1 (can start immediately)
- [ ] #10 - **Only task**

## Execution Order

Tasks are grouped by dependencies.
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	if content.Summary != "Just a summary." {
		t.Errorf("Summary = %q, want %q", content.Summary, "Just a summary.")
	}

	// Should have empty insights and constraints
	if len(content.Insights) != 0 {
		t.Errorf("Insights should be empty, got %v", content.Insights)
	}
	if len(content.Constraints) != 0 {
		t.Errorf("Constraints should be empty, got %v", content.Constraints)
	}

	// Should have one group with one issue
	expectedGroups := [][]int{{10}}
	if !reflect.DeepEqual(content.ExecutionGroups, expectedGroups) {
		t.Errorf("ExecutionGroups = %v, want %v", content.ExecutionGroups, expectedGroups)
	}
}

func TestParseParentIssueBody_CheckedItems(t *testing.T) {
	// Sub-issues might be checked off as completed
	body := `## Summary

Testing checked items.

## Sub-Issues

### Group 1 (can start immediately)
- [x] #1 - **Completed task**
- [ ] #2 - **Pending task**

### Group 2 (depends on previous groups)
- [x] #3 - **Another completed**
- [ ] #4 - **Still pending**
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	// Should extract all issue numbers regardless of check state
	expectedGroups := [][]int{{1, 2}, {3, 4}}
	if !reflect.DeepEqual(content.ExecutionGroups, expectedGroups) {
		t.Errorf("ExecutionGroups = %v, want %v", content.ExecutionGroups, expectedGroups)
	}
}

func TestParseParentIssueBody_ManyGroups(t *testing.T) {
	body := `## Summary

Plan with many execution groups.

## Sub-Issues

### Group 1 (can start immediately)
- [ ] #1 - **Task A**

### Group 2 (depends on previous groups)
- [ ] #2 - **Task B**
- [ ] #3 - **Task C**

### Group 3 (depends on previous groups)
- [ ] #4 - **Task D**

### Group 4 (depends on previous groups)
- [ ] #5 - **Task E**
- [ ] #6 - **Task F**
- [ ] #7 - **Task G**
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	expectedGroups := [][]int{{1}, {2, 3}, {4}, {5, 6, 7}}
	if !reflect.DeepEqual(content.ExecutionGroups, expectedGroups) {
		t.Errorf("ExecutionGroups = %v, want %v", content.ExecutionGroups, expectedGroups)
	}
}

func TestParseParentIssueBody_EmptyBody(t *testing.T) {
	content, err := ParseParentIssueBody("")
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	// Should return empty content without error
	if content.Summary != "" {
		t.Errorf("Summary should be empty, got %q", content.Summary)
	}
	if len(content.Insights) != 0 {
		t.Errorf("Insights should be empty, got %v", content.Insights)
	}
	if len(content.Constraints) != 0 {
		t.Errorf("Constraints should be empty, got %v", content.Constraints)
	}
	if len(content.ExecutionGroups) != 0 {
		t.Errorf("ExecutionGroups should be empty, got %v", content.ExecutionGroups)
	}
}

func TestParseParentIssueBody_WhitespaceVariations(t *testing.T) {
	// Test with various whitespace patterns that are valid markdown
	body := `##  Summary

  Some summary with leading spaces.

##   Analysis

  - Insight with leading spaces
- Insight without leading space
  -  Insight with extra space after dash

## Sub-Issues

###  Group 1  (can start immediately)
  - [ ] #100 - **Spaced task**
- [ ]  #101 - **Another task**
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	// Summary should be trimmed but preserve internal structure
	if !strings.Contains(content.Summary, "Some summary") {
		t.Errorf("Summary not parsed correctly: %q", content.Summary)
	}

	// Should parse insights with various spacing
	if len(content.Insights) != 3 {
		t.Errorf("Expected 3 insights, got %d: %v", len(content.Insights), content.Insights)
	}

	// Should extract issue numbers
	expectedGroups := [][]int{{100, 101}}
	if !reflect.DeepEqual(content.ExecutionGroups, expectedGroups) {
		t.Errorf("ExecutionGroups = %v, want %v", content.ExecutionGroups, expectedGroups)
	}
}

func TestParseParentIssueBody_CaseInsensitiveHeaders(t *testing.T) {
	body := `## SUMMARY

Uppercase header test.

## analysis

Lowercase analysis section.

- Test insight

## CONSTRAINTS

- Test constraint

## sub-issues

### group 1 (can start immediately)
- [ ] #50 - **Task**
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	if content.Summary != "Uppercase header test." {
		t.Errorf("Summary = %q, want %q", content.Summary, "Uppercase header test.")
	}

	if len(content.Insights) != 1 || content.Insights[0] != "Test insight" {
		t.Errorf("Insights = %v, want [Test insight]", content.Insights)
	}

	if len(content.Constraints) != 1 || content.Constraints[0] != "Test constraint" {
		t.Errorf("Constraints = %v, want [Test constraint]", content.Constraints)
	}

	expectedGroups := [][]int{{50}}
	if !reflect.DeepEqual(content.ExecutionGroups, expectedGroups) {
		t.Errorf("ExecutionGroups = %v, want %v", content.ExecutionGroups, expectedGroups)
	}
}

func TestParseParentIssueBody_NoSubIssues(t *testing.T) {
	body := `## Summary

A plan summary without any sub-issues section.

## Analysis

- Some analysis

## Constraints

- Some constraint
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	if content.Summary != "A plan summary without any sub-issues section." {
		t.Errorf("Summary = %q", content.Summary)
	}

	if len(content.ExecutionGroups) != 0 {
		t.Errorf("ExecutionGroups should be empty, got %v", content.ExecutionGroups)
	}
}

func TestParseParentIssueBody_MultiLineSummary(t *testing.T) {
	body := `## Summary

This is a complex summary that spans multiple paragraphs.

It includes technical details about the implementation approach
and considerations for the architecture.

Key points:
- Point one
- Point two

## Analysis

- Analysis item
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	// Summary should preserve paragraph structure
	if !strings.Contains(content.Summary, "multiple paragraphs") {
		t.Errorf("Summary missing first paragraph content")
	}
	if !strings.Contains(content.Summary, "technical details") {
		t.Errorf("Summary missing second paragraph content")
	}
	if !strings.Contains(content.Summary, "Key points:") {
		t.Errorf("Summary missing key points section")
	}
}

func TestParseParentIssueBody_UnknownSections(t *testing.T) {
	// Test that unknown ## sections don't break parsing
	body := `## Summary

Test summary.

## Custom Section

This is a custom section that should be ignored.

## Analysis

- Real insight

## Another Custom

More custom content.

## Sub-Issues

### Group 1 (can start immediately)
- [ ] #99 - **Task**
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	if content.Summary != "Test summary." {
		t.Errorf("Summary = %q, want %q", content.Summary, "Test summary.")
	}

	if len(content.Insights) != 1 || content.Insights[0] != "Real insight" {
		t.Errorf("Insights = %v, want [Real insight]", content.Insights)
	}

	expectedGroups := [][]int{{99}}
	if !reflect.DeepEqual(content.ExecutionGroups, expectedGroups) {
		t.Errorf("ExecutionGroups = %v, want %v", content.ExecutionGroups, expectedGroups)
	}
}

func TestParseParentIssueBody_HighIssueNumbers(t *testing.T) {
	body := `## Summary

Testing with high issue numbers.

## Sub-Issues

### Group 1 (can start immediately)
- [ ] #12345 - **High number task**
- [ ] #99999 - **Even higher number**
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	expectedGroups := [][]int{{12345, 99999}}
	if !reflect.DeepEqual(content.ExecutionGroups, expectedGroups) {
		t.Errorf("ExecutionGroups = %v, want %v", content.ExecutionGroups, expectedGroups)
	}
}

func TestParseParentIssueBody_LongTitles(t *testing.T) {
	// Task titles with special characters shouldn't affect parsing
	body := `## Summary

Test.

## Sub-Issues

### Group 1 (can start immediately)
- [ ] #1 - **Task with "quotes" and 'apostrophes'**
- [ ] #2 - **Task with special chars: @#$%^&*()**
- [ ] #3 - **Task-with-dashes_and_underscores**
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	expectedGroups := [][]int{{1, 2, 3}}
	if !reflect.DeepEqual(content.ExecutionGroups, expectedGroups) {
		t.Errorf("ExecutionGroups = %v, want %v", content.ExecutionGroups, expectedGroups)
	}
}

func TestParseParentIssueBody_InsightsWithSpecialCharacters(t *testing.T) {
	body := `## Summary

Test.

## Analysis

- Insight with **bold** and *italic*
- Insight with ` + "`code`" + ` blocks
- Insight with [links](http://example.com)
- Insight: with colon and "quotes"

## Constraints

- Must use ` + "`interface{}`" + ` for compatibility
- Keep files under 500 LOC (lines of code)
`

	content, err := ParseParentIssueBody(body)
	if err != nil {
		t.Fatalf("ParseParentIssueBody() error = %v", err)
	}

	if len(content.Insights) != 4 {
		t.Errorf("Expected 4 insights, got %d: %v", len(content.Insights), content.Insights)
	}

	if len(content.Constraints) != 2 {
		t.Errorf("Expected 2 constraints, got %d: %v", len(content.Constraints), content.Constraints)
	}
}

// =============================================================================
// Sub-Issue Body Parser Tests (task-5-parse-subissue-body)
// =============================================================================

func TestParseSubIssueBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    *SubIssueContent
		wantErr bool
	}{
		{
			name: "complete body with all sections",
			body: `## Task

Implement the user authentication module with JWT tokens.

## Files to Modify

- ` + "`auth/jwt.go`" + `
- ` + "`auth/middleware.go`" + `

## Dependencies

Complete these issues first:
- #42 - Setup project structure
- #43 - Database models

## Complexity

Estimated: **medium**

---
*Part of #100*
`,
			want: &SubIssueContent{
				Description:         "Implement the user authentication module with JWT tokens.",
				Files:               []string{"auth/jwt.go", "auth/middleware.go"},
				DependencyIssueNums: []int{42, 43},
				Complexity:          "medium",
				ParentIssueNum:      100,
			},
			wantErr: false,
		},
		{
			name: "minimal body without optional sections",
			body: `## Task

Simple task description.

## Complexity

Estimated: **low**

---
*Part of #50*
`,
			want: &SubIssueContent{
				Description:         "Simple task description.",
				Files:               nil,
				DependencyIssueNums: nil,
				Complexity:          "low",
				ParentIssueNum:      50,
			},
			wantErr: false,
		},
		{
			name: "high complexity task",
			body: `## Task

Complex refactoring of the entire API layer.

## Files to Modify

- ` + "`api/handlers.go`" + `

## Complexity

Estimated: **high**

---
*Part of #75*
`,
			want: &SubIssueContent{
				Description:         "Complex refactoring of the entire API layer.",
				Files:               []string{"api/handlers.go"},
				DependencyIssueNums: nil,
				Complexity:          "high",
				ParentIssueNum:      75,
			},
			wantErr: false,
		},
		{
			name: "multiline task description",
			body: `## Task

This is a task that spans multiple lines.

It includes detailed instructions about what needs to be done.

- Point one
- Point two

## Complexity

Estimated: **medium**

---
*Part of #200*
`,
			want: &SubIssueContent{
				Description: `This is a task that spans multiple lines.

It includes detailed instructions about what needs to be done.

- Point one
- Point two`,
				Files:               nil,
				DependencyIssueNums: nil,
				Complexity:          "medium",
				ParentIssueNum:      200,
			},
			wantErr: false,
		},
		{
			name: "dependencies with issue titles",
			body: `## Task

Add feature X.

## Dependencies

Complete these issues first:
- #101 - First dependency task
- #102 - Second dependency task
- #103 - Third dependency task

## Complexity

Estimated: **low**

---
*Part of #99*
`,
			want: &SubIssueContent{
				Description:         "Add feature X.",
				Files:               nil,
				DependencyIssueNums: []int{101, 102, 103},
				Complexity:          "low",
				ParentIssueNum:      99,
			},
			wantErr: false,
		},
		{
			name: "files with various paths",
			body: `## Task

Update multiple files.

## Files to Modify

- ` + "`internal/pkg/service.go`" + `
- ` + "`cmd/main.go`" + `
- ` + "`tests/service_test.go`" + `
- ` + "`README.md`" + `

## Complexity

Estimated: **medium**

---
*Part of #300*
`,
			want: &SubIssueContent{
				Description: "Update multiple files.",
				Files: []string{
					"internal/pkg/service.go",
					"cmd/main.go",
					"tests/service_test.go",
					"README.md",
				},
				DependencyIssueNums: nil,
				Complexity:          "medium",
				ParentIssueNum:      300,
			},
			wantErr: false,
		},
		{
			name:    "empty body",
			body:    "",
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing task section",
			body: `## Complexity

Estimated: **low**

---
*Part of #50*
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing complexity section",
			body: `## Task

Some task.

---
*Part of #50*
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing parent reference",
			body: `## Task

Some task.

## Complexity

Estimated: **low**
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid complexity value",
			body: `## Task

Some task.

## Complexity

Estimated: **extreme**

---
*Part of #50*
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty task section",
			body: `## Task

## Complexity

Estimated: **low**

---
*Part of #50*
`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSubIssueBody(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSubIssueBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSubIssueBody() got = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseTaskSection(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "simple task",
			body: `## Task

Simple description.

## Next Section`,
			want:    "Simple description.",
			wantErr: false,
		},
		{
			name: "task with horizontal rule separator",
			body: `## Task

Description here.

---
Footer`,
			want:    "Description here.",
			wantErr: false,
		},
		{
			name: "multiline task",
			body: `## Task

Line one.

Line two.

Line three.

## Complexity`,
			want: `Line one.

Line two.

Line three.`,
			wantErr: false,
		},
		{
			name:    "missing task section",
			body:    `## Other\n\nContent`,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTaskSection(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTaskSection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseTaskSection() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseFilesSection(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "multiple files",
			body: `## Files to Modify

- ` + "`file1.go`" + `
- ` + "`file2.go`" + `
- ` + "`file3.go`" + `

## Next`,
			want: []string{"file1.go", "file2.go", "file3.go"},
		},
		{
			name: "no files section",
			body: `## Task

Description.

## Complexity`,
			want: nil,
		},
		{
			name: "empty files section",
			body: `## Files to Modify

## Complexity`,
			want: nil,
		},
		{
			name: "files with asterisk bullets",
			body: `## Files to Modify

* ` + "`first.go`" + `
* ` + "`second.go`" + `

## Next`,
			want: []string{"first.go", "second.go"},
		},
		{
			name: "files with paths containing directories",
			body: `## Files to Modify

- ` + "`internal/service/handler.go`" + `
- ` + "`cmd/server/main.go`" + `

## Next`,
			want: []string{"internal/service/handler.go", "cmd/server/main.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFilesSection(tt.body)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseFilesSection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDependenciesSection(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []int
	}{
		{
			name: "multiple dependencies",
			body: `## Dependencies

Complete these issues first:
- #10 - First task
- #20 - Second task
- #30 - Third task

## Complexity`,
			want: []int{10, 20, 30},
		},
		{
			name: "no dependencies section",
			body: `## Task

Description.

## Complexity`,
			want: nil,
		},
		{
			name: "empty dependencies section",
			body: `## Dependencies

## Complexity`,
			want: nil,
		},
		{
			name: "single dependency",
			body: `## Dependencies

Complete these issues first:
- #42 - Only dependency

## Complexity`,
			want: []int{42},
		},
		{
			name: "dependencies without titles",
			body: `## Dependencies

- #100
- #200
- #300

## Complexity`,
			want: []int{100, 200, 300},
		},
		{
			name: "duplicate dependencies are deduplicated",
			body: `## Dependencies

- #50 - First mention
- #50 - Duplicate mention

## Complexity`,
			want: []int{50},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDependenciesSection(tt.body)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDependenciesSection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseComplexitySection(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name:    "low complexity",
			body:    `## Complexity\n\nEstimated: **low**`,
			want:    "low",
			wantErr: false,
		},
		{
			name:    "medium complexity",
			body:    `## Complexity\n\nEstimated: **medium**`,
			want:    "medium",
			wantErr: false,
		},
		{
			name:    "high complexity",
			body:    `## Complexity\n\nEstimated: **high**`,
			want:    "high",
			wantErr: false,
		},
		{
			name:    "uppercase complexity is normalized",
			body:    `## Complexity\n\nEstimated: **LOW**`,
			want:    "low",
			wantErr: false,
		},
		{
			name:    "mixed case complexity is normalized",
			body:    `## Complexity\n\nEstimated: **Medium**`,
			want:    "medium",
			wantErr: false,
		},
		{
			name:    "invalid complexity value",
			body:    `## Complexity\n\nEstimated: **unknown**`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "missing complexity",
			body:    `## Other section\n\nNo complexity here.`,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseComplexitySection(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseComplexitySection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseComplexitySection() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseParentReference(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    int
		wantErr bool
	}{
		{
			name:    "simple parent reference",
			body:    `*Part of #100*`,
			want:    100,
			wantErr: false,
		},
		{
			name:    "parent reference in context",
			body:    "## Task\n\nDescription.\n\n---\n*Part of #42*\n",
			want:    42,
			wantErr: false,
		},
		{
			name:    "large issue number",
			body:    `*Part of #99999*`,
			want:    99999,
			wantErr: false,
		},
		{
			name:    "missing parent reference",
			body:    `No parent reference here.`,
			want:    0,
			wantErr: true,
		},
		{
			name:    "malformed parent reference",
			body:    `Part of #100`,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseParentReference(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseParentReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseParentReference() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseSubIssueBody_RoundTrip(t *testing.T) {
	// Test that we can parse what the template renders
	// This ensures the parser is compatible with the template format

	// Simulate a rendered sub-issue body (similar to what RenderSubIssueBody produces)
	renderedBody := `## Task

Implement the new feature for user management.

## Files to Modify

- ` + "`internal/user/service.go`" + `
- ` + "`internal/user/handler.go`" + `

## Dependencies

Complete these issues first:
- #10 - Setup user model
- #11 - Add database migrations

## Complexity

Estimated: **medium**

---
*Part of #5*
`

	content, err := ParseSubIssueBody(renderedBody)
	if err != nil {
		t.Fatalf("ParseSubIssueBody() failed on round-trip test: %v", err)
	}

	// Verify all fields
	if content.Description != "Implement the new feature for user management." {
		t.Errorf("Description mismatch: got %q", content.Description)
	}

	expectedFiles := []string{"internal/user/service.go", "internal/user/handler.go"}
	if !reflect.DeepEqual(content.Files, expectedFiles) {
		t.Errorf("Files mismatch: got %v, want %v", content.Files, expectedFiles)
	}

	expectedDeps := []int{10, 11}
	if !reflect.DeepEqual(content.DependencyIssueNums, expectedDeps) {
		t.Errorf("DependencyIssueNums mismatch: got %v, want %v", content.DependencyIssueNums, expectedDeps)
	}

	if content.Complexity != "medium" {
		t.Errorf("Complexity mismatch: got %q, want %q", content.Complexity, "medium")
	}

	if content.ParentIssueNum != 5 {
		t.Errorf("ParentIssueNum mismatch: got %d, want %d", content.ParentIssueNum, 5)
	}
}

func TestParseSubIssueBody_WithoutDependenciesSection(t *testing.T) {
	// Test the body format when there are no dependencies
	// (template conditionally omits the section)
	body := `## Task

Simple standalone task.

## Complexity

Estimated: **low**

---
*Part of #1*
`

	content, err := ParseSubIssueBody(body)
	if err != nil {
		t.Fatalf("ParseSubIssueBody() failed: %v", err)
	}

	if content.DependencyIssueNums != nil {
		t.Errorf("Expected nil dependencies, got %v", content.DependencyIssueNums)
	}
}

func TestParseSubIssueBody_WithoutFilesSection(t *testing.T) {
	// Test the body format when there are no files
	// (template conditionally omits the section)
	body := `## Task

Task without specific file targets.

## Complexity

Estimated: **low**

---
*Part of #1*
`

	content, err := ParseSubIssueBody(body)
	if err != nil {
		t.Fatalf("ParseSubIssueBody() failed: %v", err)
	}

	if content.Files != nil {
		t.Errorf("Expected nil files, got %v", content.Files)
	}
}

// =============================================================================
// Issue to PlannedTask Conversion Tests (task-6-issue-to-task)
// =============================================================================

func TestConvertToPlannedTask(t *testing.T) {
	tests := []struct {
		name             string
		issue            GitHubIssue
		content          SubIssueContent
		issueNumToTaskID map[int]string
		wantTask         orchestrator.PlannedTask
		wantErr          bool
		errContains      string
	}{
		{
			name: "basic conversion with all fields",
			issue: GitHubIssue{
				Number: 42,
				Title:  "Add user authentication",
				Body:   "## Task\n\nImplement auth...",
				URL:    "https://github.com/owner/repo/issues/42",
				State:  "open",
			},
			content: SubIssueContent{
				Description:         "Implement user authentication with JWT tokens.",
				Files:               []string{"auth/jwt.go", "auth/middleware.go"},
				DependencyIssueNums: []int{10, 11},
				Complexity:          "medium",
				ParentIssueNum:      100,
			},
			issueNumToTaskID: map[int]string{
				10: "task-10-setup-database",
				11: "task-11-create-models",
			},
			wantTask: orchestrator.PlannedTask{
				ID:            "task-42-add-user-authentication",
				Title:         "Add user authentication",
				Description:   "Implement user authentication with JWT tokens.",
				Files:         []string{"auth/jwt.go", "auth/middleware.go"},
				DependsOn:     []string{"task-10-setup-database", "task-11-create-models"},
				Priority:      0,
				EstComplexity: orchestrator.ComplexityMedium,
				IssueURL:      "https://github.com/owner/repo/issues/42",
			},
			wantErr: false,
		},
		{
			name: "minimal conversion without dependencies or files",
			issue: GitHubIssue{
				Number: 1,
				Title:  "Setup project",
				URL:    "https://github.com/owner/repo/issues/1",
			},
			content: SubIssueContent{
				Description:    "Initialize the project structure.",
				Complexity:     "low",
				ParentIssueNum: 100,
			},
			issueNumToTaskID: map[int]string{},
			wantTask: orchestrator.PlannedTask{
				ID:            "task-1-setup-project",
				Title:         "Setup project",
				Description:   "Initialize the project structure.",
				Files:         nil,
				DependsOn:     nil,
				Priority:      0,
				EstComplexity: orchestrator.ComplexityLow,
				IssueURL:      "https://github.com/owner/repo/issues/1",
			},
			wantErr: false,
		},
		{
			name: "high complexity task",
			issue: GitHubIssue{
				Number: 99,
				Title:  "Refactor entire API layer",
				URL:    "https://github.com/owner/repo/issues/99",
			},
			content: SubIssueContent{
				Description:    "Complete API refactoring including breaking changes.",
				Complexity:     "high",
				ParentIssueNum: 50,
			},
			issueNumToTaskID: map[int]string{},
			wantTask: orchestrator.PlannedTask{
				ID:            "task-99-refactor-entire-api-layer",
				Title:         "Refactor entire API layer",
				Description:   "Complete API refactoring including breaking changes.",
				DependsOn:     nil,
				Priority:      0,
				EstComplexity: orchestrator.ComplexityHigh,
				IssueURL:      "https://github.com/owner/repo/issues/99",
			},
			wantErr: false,
		},
		{
			name: "invalid issue number (zero)",
			issue: GitHubIssue{
				Number: 0,
				Title:  "Invalid issue",
				URL:    "https://github.com/owner/repo/issues/0",
			},
			content: SubIssueContent{
				Description: "Some description.",
				Complexity:  "low",
			},
			wantErr:     true,
			errContains: "invalid issue number",
		},
		{
			name: "invalid issue number (negative)",
			issue: GitHubIssue{
				Number: -1,
				Title:  "Invalid issue",
			},
			content: SubIssueContent{
				Description: "Some description.",
				Complexity:  "low",
			},
			wantErr:     true,
			errContains: "invalid issue number",
		},
		{
			name: "missing title",
			issue: GitHubIssue{
				Number: 42,
				Title:  "",
				URL:    "https://github.com/owner/repo/issues/42",
			},
			content: SubIssueContent{
				Description: "Some description.",
				Complexity:  "low",
			},
			wantErr:     true,
			errContains: "title is required",
		},
		{
			name: "missing description",
			issue: GitHubIssue{
				Number: 42,
				Title:  "Valid title",
				URL:    "https://github.com/owner/repo/issues/42",
			},
			content: SubIssueContent{
				Description: "",
				Complexity:  "low",
			},
			wantErr:     true,
			errContains: "description is required",
		},
		{
			name: "missing dependency in mapping",
			issue: GitHubIssue{
				Number: 42,
				Title:  "Task with missing dep",
				URL:    "https://github.com/owner/repo/issues/42",
			},
			content: SubIssueContent{
				Description:         "Description.",
				DependencyIssueNums: []int{10, 999}, // 999 doesn't exist in mapping
				Complexity:          "low",
			},
			issueNumToTaskID: map[int]string{
				10: "task-10-exists",
			},
			wantErr:     true,
			errContains: "not found in mapping",
		},
		{
			name: "invalid complexity",
			issue: GitHubIssue{
				Number: 42,
				Title:  "Task",
				URL:    "https://github.com/owner/repo/issues/42",
			},
			content: SubIssueContent{
				Description: "Description.",
				Complexity:  "extreme",
			},
			issueNumToTaskID: map[int]string{},
			wantErr:          true,
			errContains:      "invalid complexity",
		},
		{
			name: "title with special characters",
			issue: GitHubIssue{
				Number: 123,
				Title:  "Fix bug #456 in `user.go`",
				URL:    "https://github.com/owner/repo/issues/123",
			},
			content: SubIssueContent{
				Description: "Fix the reported bug.",
				Complexity:  "low",
			},
			issueNumToTaskID: map[int]string{},
			wantTask: orchestrator.PlannedTask{
				ID:            "task-123-fix-bug-456-in-user-go",
				Title:         "Fix bug #456 in `user.go`",
				Description:   "Fix the reported bug.",
				DependsOn:     nil,
				Priority:      0,
				EstComplexity: orchestrator.ComplexityLow,
				IssueURL:      "https://github.com/owner/repo/issues/123",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertToPlannedTask(tt.issue, tt.content, tt.issueNumToTaskID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ConvertToPlannedTask() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ConvertToPlannedTask() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ConvertToPlannedTask() unexpected error: %v", err)
				return
			}

			// Compare relevant fields
			if got.ID != tt.wantTask.ID {
				t.Errorf("ID = %q, want %q", got.ID, tt.wantTask.ID)
			}
			if got.Title != tt.wantTask.Title {
				t.Errorf("Title = %q, want %q", got.Title, tt.wantTask.Title)
			}
			if got.Description != tt.wantTask.Description {
				t.Errorf("Description = %q, want %q", got.Description, tt.wantTask.Description)
			}
			if !reflect.DeepEqual(got.Files, tt.wantTask.Files) {
				t.Errorf("Files = %v, want %v", got.Files, tt.wantTask.Files)
			}
			if !reflect.DeepEqual(got.DependsOn, tt.wantTask.DependsOn) {
				t.Errorf("DependsOn = %v, want %v", got.DependsOn, tt.wantTask.DependsOn)
			}
			if got.EstComplexity != tt.wantTask.EstComplexity {
				t.Errorf("EstComplexity = %q, want %q", got.EstComplexity, tt.wantTask.EstComplexity)
			}
			if got.IssueURL != tt.wantTask.IssueURL {
				t.Errorf("IssueURL = %q, want %q", got.IssueURL, tt.wantTask.IssueURL)
			}
		})
	}
}

func TestGenerateTaskID(t *testing.T) {
	tests := []struct {
		name     string
		issueNum int
		title    string
		want     string
	}{
		{
			name:     "simple title",
			issueNum: 42,
			title:    "Add authentication",
			want:     "task-42-add-authentication",
		},
		{
			name:     "title with special characters",
			issueNum: 123,
			title:    "Fix bug #456 in user.go",
			want:     "task-123-fix-bug-456-in-user-go",
		},
		{
			name:     "title with multiple spaces",
			issueNum: 1,
			title:    "   Multiple   Spaces   Here   ",
			want:     "task-1-multiple-spaces-here",
		},
		{
			name:     "title with underscores and hyphens",
			issueNum: 99,
			title:    "update_user-service",
			want:     "task-99-update-user-service",
		},
		{
			name:     "title with backticks",
			issueNum: 10,
			title:    "Fix `config.yaml` parsing",
			want:     "task-10-fix-config-yaml-parsing",
		},
		{
			name:     "empty title",
			issueNum: 5,
			title:    "",
			want:     "task-5-",
		},
		{
			name:     "title with only special characters",
			issueNum: 7,
			title:    "!@#$%^&*()",
			want:     "task-7-",
		},
		{
			name:     "unicode characters",
			issueNum: 88,
			title:    "Добавить функцию",
			want:     "task-88-добавить-функцию",
		},
		{
			name:     "mixed case preserved as lowercase",
			issueNum: 3,
			title:    "Add USER Authentication",
			want:     "task-3-add-user-authentication",
		},
		{
			name:     "numbers in title",
			issueNum: 500,
			title:    "Migrate to v2 API",
			want:     "task-500-migrate-to-v2-api",
		},
		{
			name:     "very long title gets truncated",
			issueNum: 1,
			title:    "This is a very long title that exceeds the maximum slug length and should be truncated appropriately",
			want:     "task-1-this-is-a-very-long-title-that-exceeds-the-maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateTaskID(tt.issueNum, tt.title)
			if got != tt.want {
				t.Errorf("GenerateTaskID(%d, %q) = %q, want %q", tt.issueNum, tt.title, got, tt.want)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "simple string",
			input:  "Hello World",
			maxLen: 0,
			want:   "hello-world",
		},
		{
			name:   "with special characters",
			input:  "Hello! @World# 123",
			maxLen: 0,
			want:   "hello-world-123",
		},
		{
			name:   "multiple spaces and symbols",
			input:  "  multiple   spaces   ",
			maxLen: 0,
			want:   "multiple-spaces",
		},
		{
			name:   "with max length",
			input:  "This is a long string",
			maxLen: 10,
			want:   "this-is-a",
		},
		{
			name:   "max length cuts mid-hyphen",
			input:  "one two three",
			maxLen: 8,
			want:   "one-two",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "only special characters",
			input:  "!@#$%",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "leading special characters",
			input:  "!!!hello",
			maxLen: 0,
			want:   "hello",
		},
		{
			name:   "trailing special characters",
			input:  "hello!!!",
			maxLen: 0,
			want:   "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugify(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("slugify(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestMapDependenciesToTaskIDs(t *testing.T) {
	tests := []struct {
		name             string
		issueNums        []int
		issueNumToTaskID map[int]string
		want             []string
		wantErr          bool
		errContains      string
	}{
		{
			name:      "empty dependencies",
			issueNums: nil,
			issueNumToTaskID: map[int]string{
				10: "task-10-something",
			},
			want:    nil,
			wantErr: false,
		},
		{
			name:             "empty dependencies with empty map",
			issueNums:        []int{},
			issueNumToTaskID: map[int]string{},
			want:             nil,
			wantErr:          false,
		},
		{
			name:      "single dependency",
			issueNums: []int{10},
			issueNumToTaskID: map[int]string{
				10: "task-10-setup",
			},
			want:    []string{"task-10-setup"},
			wantErr: false,
		},
		{
			name:      "multiple dependencies",
			issueNums: []int{10, 20, 30},
			issueNumToTaskID: map[int]string{
				10: "task-10-first",
				20: "task-20-second",
				30: "task-30-third",
			},
			want:    []string{"task-10-first", "task-20-second", "task-30-third"},
			wantErr: false,
		},
		{
			name:      "missing dependency",
			issueNums: []int{10, 999},
			issueNumToTaskID: map[int]string{
				10: "task-10-exists",
			},
			want:        nil,
			wantErr:     true,
			errContains: "999",
		},
		{
			name:             "all dependencies missing",
			issueNums:        []int{100, 200},
			issueNumToTaskID: map[int]string{},
			want:             nil,
			wantErr:          true,
			errContains:      "100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MapDependenciesToTaskIDs(tt.issueNums, tt.issueNumToTaskID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("MapDependenciesToTaskIDs() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("MapDependenciesToTaskIDs() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("MapDependenciesToTaskIDs() unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MapDependenciesToTaskIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseTaskComplexity(t *testing.T) {
	tests := []struct {
		name       string
		complexity string
		want       orchestrator.TaskComplexity
		wantErr    bool
	}{
		{
			name:       "low complexity",
			complexity: "low",
			want:       orchestrator.ComplexityLow,
			wantErr:    false,
		},
		{
			name:       "medium complexity",
			complexity: "medium",
			want:       orchestrator.ComplexityMedium,
			wantErr:    false,
		},
		{
			name:       "high complexity",
			complexity: "high",
			want:       orchestrator.ComplexityHigh,
			wantErr:    false,
		},
		{
			name:       "uppercase LOW",
			complexity: "LOW",
			want:       orchestrator.ComplexityLow,
			wantErr:    false,
		},
		{
			name:       "mixed case Medium",
			complexity: "Medium",
			want:       orchestrator.ComplexityMedium,
			wantErr:    false,
		},
		{
			name:       "with whitespace",
			complexity: "  high  ",
			want:       orchestrator.ComplexityHigh,
			wantErr:    false,
		},
		{
			name:       "invalid complexity",
			complexity: "extreme",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "empty string",
			complexity: "",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "unknown value",
			complexity: "very-high",
			want:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTaskComplexity(tt.complexity)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseTaskComplexity(%q) expected error, got nil", tt.complexity)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseTaskComplexity(%q) unexpected error: %v", tt.complexity, err)
				return
			}

			if got != tt.want {
				t.Errorf("ParseTaskComplexity(%q) = %q, want %q", tt.complexity, got, tt.want)
			}
		})
	}
}

func TestConvertToPlannedTask_Integration(t *testing.T) {
	// This test simulates a real-world scenario where we have a parent issue
	// with multiple sub-issues that have interdependencies

	// Simulate issue numbers to task IDs (as would be built during ingestion)
	issueNumToTaskID := map[int]string{
		101: "task-101-setup-infrastructure",
		102: "task-102-create-base-types",
		103: "task-103-implement-main-feature",
	}

	// Test converting issue 103 which depends on 101 and 102
	issue := GitHubIssue{
		Number: 103,
		Title:  "Implement main feature",
		Body: `## Task

Implement the core feature using the base types.

## Files to Modify

- ` + "`internal/feature/handler.go`" + `
- ` + "`internal/feature/service.go`" + `

## Dependencies

Complete these issues first:
- #101 - Setup infrastructure
- #102 - Create base types

## Complexity

Estimated: **medium**

---
*Part of #100*
`,
		URL:   "https://github.com/example/repo/issues/103",
		State: "open",
	}

	// Parse the body using existing parser
	content, err := ParseSubIssueBody(issue.Body)
	if err != nil {
		t.Fatalf("ParseSubIssueBody() failed: %v", err)
	}

	// Convert to PlannedTask
	task, err := ConvertToPlannedTask(issue, *content, issueNumToTaskID)
	if err != nil {
		t.Fatalf("ConvertToPlannedTask() failed: %v", err)
	}

	// Verify the task
	if task.ID != "task-103-implement-main-feature" {
		t.Errorf("ID = %q, want %q", task.ID, "task-103-implement-main-feature")
	}
	if task.Title != "Implement main feature" {
		t.Errorf("Title = %q", task.Title)
	}
	if !strings.Contains(task.Description, "core feature") {
		t.Errorf("Description doesn't contain expected content: %q", task.Description)
	}

	expectedFiles := []string{"internal/feature/handler.go", "internal/feature/service.go"}
	if !reflect.DeepEqual(task.Files, expectedFiles) {
		t.Errorf("Files = %v, want %v", task.Files, expectedFiles)
	}

	expectedDeps := []string{"task-101-setup-infrastructure", "task-102-create-base-types"}
	if !reflect.DeepEqual(task.DependsOn, expectedDeps) {
		t.Errorf("DependsOn = %v, want %v", task.DependsOn, expectedDeps)
	}

	if task.EstComplexity != orchestrator.ComplexityMedium {
		t.Errorf("EstComplexity = %q, want %q", task.EstComplexity, orchestrator.ComplexityMedium)
	}

	if task.IssueURL != "https://github.com/example/repo/issues/103" {
		t.Errorf("IssueURL = %q", task.IssueURL)
	}
}

// =============================================================================
// URL-to-PlanSpec Ingestion Pipeline Tests
// =============================================================================

func TestDetectSourceProvider(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want SourceProvider
	}{
		{
			name: "GitHub full URL",
			url:  "https://github.com/owner/repo/issues/123",
			want: ProviderGitHub,
		},
		{
			name: "GitHub shorthand",
			url:  "owner/repo#123",
			want: ProviderGitHub,
		},
		{
			name: "GitHub URL with query",
			url:  "https://github.com/owner/repo/issues/456?foo=bar",
			want: ProviderGitHub,
		},
		{
			name: "empty URL",
			url:  "",
			want: ProviderUnknown,
		},
		{
			name: "random string",
			url:  "not a url",
			want: ProviderUnknown,
		},
		{
			name: "GitLab URL (unsupported)",
			url:  "https://gitlab.com/owner/repo/issues/123",
			want: ProviderUnknown,
		},
		{
			name: "GitHub PR URL (not an issue)",
			url:  "https://github.com/owner/repo/pull/123",
			want: ProviderUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectSourceProvider(tt.url)
			if got != tt.want {
				t.Errorf("DetectSourceProvider(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestBuildPlanFromURL_UnsupportedProvider(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "empty URL",
			url:  "",
		},
		{
			name: "random string",
			url:  "not a valid url",
		},
		{
			name: "GitLab URL",
			url:  "https://gitlab.com/owner/repo/issues/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildPlanFromURLWithExecutor(tt.url, mockExecutor(nil, nil))
			if err == nil {
				t.Error("expected error for unsupported URL, got nil")
			}
			if !errors.Is(err, ErrUnsupportedProvider) {
				t.Errorf("expected ErrUnsupportedProvider, got: %v", err)
			}
		})
	}
}

// multiIssueMockExecutor creates a mock that returns different responses
// based on the issue number being requested
func multiIssueMockExecutor(issueResponses map[int]string) CommandExecutor {
	return func(name string, args ...string) ([]byte, error) {
		// Parse the issue number from the args
		// Format: gh issue view <num> --repo owner/repo --json ...
		if len(args) < 2 {
			return nil, errors.New("unexpected args")
		}
		issueNum, err := strconv.Atoi(args[2])
		if err != nil {
			return nil, err
		}
		response, ok := issueResponses[issueNum]
		if !ok {
			return []byte("issue not found"), errors.New("exit status 1")
		}
		return []byte(response), nil
	}
}

func TestBuildPlanFromURL_FullPipeline(t *testing.T) {
	// This test simulates the full pipeline of:
	// 1. Parsing a parent issue URL
	// 2. Fetching the parent issue
	// 3. Parsing the parent body to get sub-issue refs
	// 4. Fetching all sub-issues
	// 5. Converting to a PlanSpec

	parentIssueJSON := `{
		"number": 100,
		"title": "Plan: Implement user authentication",
		"body": "## Summary\n\nAdd OAuth2 authentication support.\n\n## Analysis\n\n- Existing user table can be extended\n- JWT library available\n\n## Constraints\n\n- Must maintain backward compatibility\n\n## Sub-Issues\n\n### Group 1 (can start immediately)\n- [ ] #101 - **Setup auth models**\n- [ ] #102 - **Add JWT utilities**\n\n### Group 2 (depends on previous groups)\n- [ ] #103 - **Implement OAuth endpoints**\n\n## Execution Order\n\nTasks grouped by dependencies.",
		"labels": [{"name": "plan"}],
		"url": "https://github.com/owner/repo/issues/100",
		"state": "open"
	}`

	subIssue101JSON := `{
		"number": 101,
		"title": "Setup auth models",
		"body": "## Task\n\nCreate the User and Token models.\n\n## Files to Modify\n\n- ` + "`models/user.go`" + `\n- ` + "`models/token.go`" + `\n\n## Complexity\n\nEstimated: **low**\n\n---\n*Part of #100*",
		"labels": [],
		"url": "https://github.com/owner/repo/issues/101",
		"state": "open"
	}`

	subIssue102JSON := `{
		"number": 102,
		"title": "Add JWT utilities",
		"body": "## Task\n\nImplement JWT token generation and validation.\n\n## Files to Modify\n\n- ` + "`auth/jwt.go`" + `\n\n## Complexity\n\nEstimated: **medium**\n\n---\n*Part of #100*",
		"labels": [],
		"url": "https://github.com/owner/repo/issues/102",
		"state": "open"
	}`

	subIssue103JSON := `{
		"number": 103,
		"title": "Implement OAuth endpoints",
		"body": "## Task\n\nAdd OAuth login and callback endpoints.\n\n## Files to Modify\n\n- ` + "`api/oauth.go`" + `\n\n## Dependencies\n\nComplete these issues first:\n- #101 - Setup auth models\n- #102 - Add JWT utilities\n\n## Complexity\n\nEstimated: **high**\n\n---\n*Part of #100*",
		"labels": [],
		"url": "https://github.com/owner/repo/issues/103",
		"state": "open"
	}`

	issueResponses := map[int]string{
		100: parentIssueJSON,
		101: subIssue101JSON,
		102: subIssue102JSON,
		103: subIssue103JSON,
	}

	executor := multiIssueMockExecutor(issueResponses)

	plan, err := buildPlanFromURLWithExecutor("https://github.com/owner/repo/issues/100", executor)
	if err != nil {
		t.Fatalf("buildPlanFromURLWithExecutor() error: %v", err)
	}

	// Verify plan metadata
	if plan.Objective != "Plan: Implement user authentication" {
		t.Errorf("Objective = %q", plan.Objective)
	}
	if plan.Summary != "Add OAuth2 authentication support." {
		t.Errorf("Summary = %q", plan.Summary)
	}

	// Verify insights
	expectedInsights := []string{
		"Existing user table can be extended",
		"JWT library available",
	}
	if !reflect.DeepEqual(plan.Insights, expectedInsights) {
		t.Errorf("Insights = %v, want %v", plan.Insights, expectedInsights)
	}

	// Verify constraints
	expectedConstraints := []string{"Must maintain backward compatibility"}
	if !reflect.DeepEqual(plan.Constraints, expectedConstraints) {
		t.Errorf("Constraints = %v, want %v", plan.Constraints, expectedConstraints)
	}

	// Verify we have 3 tasks
	if len(plan.Tasks) != 3 {
		t.Fatalf("Expected 3 tasks, got %d", len(plan.Tasks))
	}

	// Verify execution order structure (2 groups)
	if len(plan.ExecutionOrder) != 2 {
		t.Fatalf("Expected 2 execution groups, got %d", len(plan.ExecutionOrder))
	}

	// Group 1 should have 2 tasks (101, 102)
	if len(plan.ExecutionOrder[0]) != 2 {
		t.Errorf("Group 1 should have 2 tasks, got %d", len(plan.ExecutionOrder[0]))
	}

	// Group 2 should have 1 task (103)
	if len(plan.ExecutionOrder[1]) != 1 {
		t.Errorf("Group 2 should have 1 task, got %d", len(plan.ExecutionOrder[1]))
	}

	// Find task 103 and verify its dependencies
	var task103 *orchestrator.PlannedTask
	for i := range plan.Tasks {
		if strings.Contains(plan.Tasks[i].ID, "103") {
			task103 = &plan.Tasks[i]
			break
		}
	}
	if task103 == nil {
		t.Fatal("Task 103 not found")
	}

	// Task 103 should depend on tasks 101 and 102
	if len(task103.DependsOn) != 2 {
		t.Errorf("Task 103 should have 2 dependencies, got %d: %v", len(task103.DependsOn), task103.DependsOn)
	}

	// Verify complexity was parsed correctly
	if task103.EstComplexity != orchestrator.ComplexityHigh {
		t.Errorf("Task 103 complexity = %q, want %q", task103.EstComplexity, orchestrator.ComplexityHigh)
	}

	// Verify IssueURL was set
	if task103.IssueURL != "https://github.com/owner/repo/issues/103" {
		t.Errorf("Task 103 IssueURL = %q", task103.IssueURL)
	}
}

func TestBuildPlanFromURL_NoSubIssues(t *testing.T) {
	// Parent issue with no sub-issues should return error
	parentIssueJSON := `{
		"number": 100,
		"title": "Empty Plan",
		"body": "## Summary\n\nNo sub-issues here.\n\n## Analysis\n\n- Some insight\n",
		"labels": [],
		"url": "https://github.com/owner/repo/issues/100",
		"state": "open"
	}`

	executor := multiIssueMockExecutor(map[int]string{100: parentIssueJSON})

	_, err := buildPlanFromURLWithExecutor("https://github.com/owner/repo/issues/100", executor)
	if err == nil {
		t.Error("expected error for parent with no sub-issues")
	}
	if !errors.Is(err, ErrNoSubIssues) {
		t.Errorf("expected ErrNoSubIssues, got: %v", err)
	}
}

func TestBuildPlanFromURL_ParentFetchFails(t *testing.T) {
	// Mock executor that fails to find the parent issue
	executor := mockExecutor([]byte("issue not found"), errors.New("exit status 1"))

	_, err := buildPlanFromURLWithExecutor("https://github.com/owner/repo/issues/999", executor)
	if err == nil {
		t.Error("expected error when parent issue fetch fails")
	}
	if !strings.Contains(err.Error(), "failed to fetch parent issue") {
		t.Errorf("error should mention parent issue fetch failure: %v", err)
	}
}

func TestBuildPlanFromURL_SubIssueFetchFails(t *testing.T) {
	// Parent issue exists but sub-issue #101 doesn't
	parentIssueJSON := `{
		"number": 100,
		"title": "Plan with missing sub-issue",
		"body": "## Summary\n\nTest plan.\n\n## Sub-Issues\n\n### Group 1\n- [ ] #101 - **Missing issue**\n",
		"labels": [],
		"url": "https://github.com/owner/repo/issues/100",
		"state": "open"
	}`

	executor := multiIssueMockExecutor(map[int]string{100: parentIssueJSON})

	_, err := buildPlanFromURLWithExecutor("https://github.com/owner/repo/issues/100", executor)
	if err == nil {
		t.Error("expected error when sub-issue fetch fails")
	}
	if !strings.Contains(err.Error(), "failed to fetch sub-issue") {
		t.Errorf("error should mention sub-issue fetch failure: %v", err)
	}
}

func TestCollectSubIssueNumbers(t *testing.T) {
	tests := []struct {
		name   string
		groups [][]int
		want   []int
	}{
		{
			name:   "empty groups",
			groups: [][]int{},
			want:   nil,
		},
		{
			name:   "single group single issue",
			groups: [][]int{{42}},
			want:   []int{42},
		},
		{
			name:   "multiple groups",
			groups: [][]int{{1, 2}, {3}, {4, 5, 6}},
			want:   []int{1, 2, 3, 4, 5, 6},
		},
		{
			name:   "deduplicates repeated numbers",
			groups: [][]int{{1, 2}, {2, 3}, {3, 4}},
			want:   []int{1, 2, 3, 4},
		},
		{
			name:   "empty groups in middle",
			groups: [][]int{{1}, {}, {2}},
			want:   []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectSubIssueNumbers(tt.groups)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectSubIssueNumbers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildExecutionOrder(t *testing.T) {
	issueNumToTaskID := map[int]string{
		10: "task-10-first",
		20: "task-20-second",
		30: "task-30-third",
	}

	tests := []struct {
		name   string
		groups [][]int
		want   [][]string
	}{
		{
			name:   "empty groups",
			groups: [][]int{},
			want:   [][]string{},
		},
		{
			name:   "single group",
			groups: [][]int{{10, 20}},
			want:   [][]string{{"task-10-first", "task-20-second"}},
		},
		{
			name:   "multiple groups",
			groups: [][]int{{10, 20}, {30}},
			want:   [][]string{{"task-10-first", "task-20-second"}, {"task-30-third"}},
		},
		{
			name:   "missing issue number skipped",
			groups: [][]int{{10, 999, 20}},
			want:   [][]string{{"task-10-first", "task-20-second"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildExecutionOrder(tt.groups, issueNumToTaskID)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildExecutionOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAssignPriorities(t *testing.T) {
	tasks := []orchestrator.PlannedTask{
		{ID: "task-1"},
		{ID: "task-2"},
		{ID: "task-3"},
		{ID: "task-4"},
	}

	executionOrder := [][]string{
		{"task-1", "task-2"}, // Group 0: priorities 0, 1
		{"task-3"},           // Group 1: priority 100
		{"task-4"},           // Group 2: priority 200
	}

	assignPriorities(tasks, executionOrder)

	expectedPriorities := map[string]int{
		"task-1": 0,   // Group 0, position 0
		"task-2": 1,   // Group 0, position 1
		"task-3": 100, // Group 1, position 0
		"task-4": 200, // Group 2, position 0
	}

	for _, task := range tasks {
		expected := expectedPriorities[task.ID]
		if task.Priority != expected {
			t.Errorf("Task %s priority = %d, want %d", task.ID, task.Priority, expected)
		}
	}
}

// =============================================================================
// Issue Format Detection Tests (task-1-detect-issue-format)
// =============================================================================

func TestDetectIssueFormat_TemplatedFormat(t *testing.T) {
	tests := []struct {
		name string
		body string
		want IssueFormat
	}{
		{
			name: "complete templated sub-issue",
			body: `## Task

Implement the new authentication system.

## Files to Modify

- ` + "`internal/auth/handler.go`" + `
- ` + "`internal/auth/service.go`" + `

## Dependencies

Complete these issues first:
- #41 - Setup database models

## Complexity

Estimated: **medium**

---
*Part of #42*
`,
			want: IssueFormatTemplated,
		},
		{
			name: "minimal templated sub-issue",
			body: `## Task

Simple task.

## Complexity

Estimated: **low**

---
*Part of #100*
`,
			want: IssueFormatTemplated,
		},
		{
			name: "templated with high complexity",
			body: `## Task

Complex refactoring work.

## Complexity

Estimated: **high**

---
*Part of #50*
`,
			want: IssueFormatTemplated,
		},
		{
			name: "templated with extra whitespace in Part of",
			body: `## Task

Task description.

## Complexity

Estimated: **medium**

---
*Part  of  #123*
`,
			want: IssueFormatTemplated,
		},
		{
			name: "templated without files or dependencies",
			body: `## Task

Just a simple task.

## Complexity

Estimated: **low**

---
*Part of #1*
`,
			want: IssueFormatTemplated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectIssueFormat(tt.body)
			if got != tt.want {
				t.Errorf("DetectIssueFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectIssueFormat_FreeformFormat(t *testing.T) {
	tests := []struct {
		name string
		body string
		want IssueFormat
	}{
		{
			name: "human-authored with Summary",
			body: `## Summary

I'd like to add a dark mode toggle to the settings page.

## Acceptance Criteria

- [ ] Toggle switch in settings
- [ ] Persists preference to localStorage
- [ ] Theme applies immediately
`,
			want: IssueFormatFreeform,
		},
		{
			name: "simple feature request",
			body: `Add support for CSV export in the reports section.

Users should be able to click "Export" and download a CSV file.
`,
			want: IssueFormatFreeform,
		},
		{
			name: "bug report format",
			body: `## Bug Description

The application crashes when clicking the submit button twice.

## Steps to Reproduce

1. Open the form
2. Fill in the fields
3. Click submit twice quickly

## Expected Behavior

Form should only submit once.
`,
			want: IssueFormatFreeform,
		},
		{
			name: "empty body",
			body: "",
			want: IssueFormatFreeform,
		},
		{
			name: "whitespace only body",
			body: "   \n\t\n   ",
			want: IssueFormatFreeform,
		},
		{
			name: "has Task section but no Part of",
			body: `## Task

This looks like a task but is missing the Part of marker.

## Complexity

Estimated: **low**
`,
			want: IssueFormatFreeform,
		},
		{
			name: "has Part of but no Task section",
			body: `## Summary

Some summary.

*Part of #42*
`,
			want: IssueFormatFreeform,
		},
		{
			name: "casual mention of Part of in prose",
			body: `## Summary

This issue is part of #42 epic. Note that the text "Part of #42" without
asterisks doesn't match the template pattern.

## Details

More details here.
`,
			want: IssueFormatFreeform,
		},
		{
			name: "has complexity but wrong Part of format",
			body: `## Task

Some task.

## Complexity

Estimated: **medium**

---
Part of #42
`,
			want: IssueFormatFreeform,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectIssueFormat(tt.body)
			if got != tt.want {
				t.Errorf("DetectIssueFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectIssueFormat_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		body string
		want IssueFormat
	}{
		{
			name: "Part of with large issue number",
			body: `## Task

Large issue number.

## Complexity

Estimated: **low**

---
*Part of #99999*
`,
			want: IssueFormatTemplated,
		},
		{
			name: "Part of with issue number 1",
			body: `## Task

Small issue.

## Complexity

Estimated: **high**

---
*Part of #1*
`,
			want: IssueFormatTemplated,
		},
		{
			name: "case insensitive complexity",
			body: `## Task

Testing case.

## Complexity

Estimated: **LOW**

---
*Part of #42*
`,
			want: IssueFormatTemplated,
		},
		{
			name: "mixed case complexity",
			body: `## Task

Testing mixed case.

## Complexity

Estimated: **Medium**

---
*Part of #42*
`,
			want: IssueFormatTemplated,
		},
		{
			name: "Part of anywhere in body with Task section",
			body: `## Task

Do the thing.

## Some Other Section

Blah blah.

*Part of #99*
`,
			want: IssueFormatTemplated,
		},
		{
			name: "Part of with Complexity but no Task section",
			body: `## Summary

Some summary with complexity marker.

Estimated: **low**

*Part of #42*
`,
			want: IssueFormatTemplated,
		},
		{
			name: "only Part of marker",
			body: `*Part of #42*`,
			want: IssueFormatFreeform,
		},
		{
			name: "only Task section",
			body: `## Task

Just a task section.
`,
			want: IssueFormatFreeform,
		},
		{
			name: "only Complexity marker",
			body: `Estimated: **low**`,
			want: IssueFormatFreeform,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectIssueFormat(tt.body)
			if got != tt.want {
				t.Errorf("DetectIssueFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectIssueFormat_RoundTrip(t *testing.T) {
	// Test that issues generated by RenderSubIssueBody are detected as templated
	task := orchestrator.PlannedTask{
		ID:            "task-1",
		Title:         "Test Task",
		Description:   "This is a test task description.",
		Files:         []string{"internal/foo/bar.go"},
		DependsOn:     []string{},
		EstComplexity: orchestrator.ComplexityMedium,
	}

	dependencyInfo := map[string]DependencyInfo{}
	parentIssueNumber := 42

	body, err := RenderSubIssueBody(task, parentIssueNumber, dependencyInfo)
	if err != nil {
		t.Fatalf("RenderSubIssueBody() error = %v", err)
	}

	format := DetectIssueFormat(body)
	if format != IssueFormatTemplated {
		t.Errorf("DetectIssueFormat(rendered body) = %q, want %q", format, IssueFormatTemplated)
	}
}

func TestDetectIssueFormat_RoundTripWithDependencies(t *testing.T) {
	// Test with a more complex task that has dependencies
	task := orchestrator.PlannedTask{
		ID:            "task-2",
		Title:         "Complex Task",
		Description:   "Multi-line\ndescription\nwith details.",
		Files:         []string{"file1.go", "file2.go", "file3.go"},
		DependsOn:     []string{"task-1"},
		EstComplexity: orchestrator.ComplexityHigh,
	}

	dependencyInfo := map[string]DependencyInfo{
		"task-1": {IssueNumber: 100, Title: "Prerequisite Task"},
	}
	parentIssueNumber := 99

	body, err := RenderSubIssueBody(task, parentIssueNumber, dependencyInfo)
	if err != nil {
		t.Fatalf("RenderSubIssueBody() error = %v", err)
	}

	format := DetectIssueFormat(body)
	if format != IssueFormatTemplated {
		t.Errorf("DetectIssueFormat(rendered body with deps) = %q, want %q", format, IssueFormatTemplated)
	}
}

func TestIssueFormat_StringValues(t *testing.T) {
	// Verify the string constants have the expected values
	if IssueFormatTemplated != "templated" {
		t.Errorf("IssueFormatTemplated = %q, want %q", IssueFormatTemplated, "templated")
	}
	if IssueFormatFreeform != "freeform" {
		t.Errorf("IssueFormatFreeform = %q, want %q", IssueFormatFreeform, "freeform")
	}
}
