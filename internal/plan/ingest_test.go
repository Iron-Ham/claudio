package plan

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

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
