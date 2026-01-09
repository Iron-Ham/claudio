package orchestrator

import (
	"errors"
	"testing"
)

func TestParseReviewIssuesFromOutput(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantCount  int
		wantErr    error
		wantTitles []string
	}{
		{
			name: "single issue in valid JSON array",
			output: `Some output text
<review_issues>
[{"type": "security", "severity": "critical", "file": "main.go", "line_start": 10, "title": "SQL Injection", "description": "User input is not sanitized"}]
</review_issues>
More output`,
			wantCount:  1,
			wantTitles: []string{"SQL Injection"},
		},
		{
			name: "multiple issues in array",
			output: `<review_issues>
[
  {"type": "security", "severity": "critical", "file": "main.go", "line_start": 10, "title": "SQL Injection", "description": "Bad"},
  {"type": "performance", "severity": "major", "file": "utils.go", "line_start": 25, "title": "N+1 Query", "description": "Loop queries"}
]
</review_issues>`,
			wantCount:  2,
			wantTitles: []string{"SQL Injection", "N+1 Query"},
		},
		{
			name: "single issue object (not array)",
			output: `<review_issues>
{"type": "style", "severity": "minor", "file": "format.go", "line_start": 5, "title": "Naming Convention", "description": "Use camelCase"}
</review_issues>`,
			wantCount:  1,
			wantTitles: []string{"Naming Convention"},
		},
		{
			name: "multiple review_issues tags",
			output: `<review_issues>
[{"type": "security", "severity": "critical", "file": "a.go", "title": "Issue A", "description": "Desc A"}]
</review_issues>
Some text in between
<review_issues>
[{"type": "performance", "severity": "major", "file": "b.go", "title": "Issue B", "description": "Desc B"}]
</review_issues>`,
			wantCount:  2,
			wantTitles: []string{"Issue A", "Issue B"},
		},
		{
			name:      "no review_issues tag",
			output:    "Just some regular output without any issues",
			wantErr:   ErrNoReviewIssuesTag,
			wantCount: 0,
		},
		{
			name:      "empty review_issues tag",
			output:    "<review_issues></review_issues>",
			wantErr:   ErrNoReviewIssuesTag,
			wantCount: 0,
		},
		{
			name:      "whitespace only in tag",
			output:    "<review_issues>   \n   </review_issues>",
			wantErr:   ErrNoReviewIssuesTag,
			wantCount: 0,
		},
		{
			name: "empty array",
			output: `<review_issues>
[]
</review_issues>`,
			wantCount: 0,
		},
		{
			name: "partial extraction from malformed JSON",
			output: `<review_issues>
[
  {"type": "security", "severity": "critical", "file": "main.go", "title": "Valid Issue", "description": "This is valid"},
  {"type": "broken", "severity": "major", "file": "broken.go", "title": "Broken JSON
</review_issues>`,
			wantCount:  1,
			wantTitles: []string{"Valid Issue"},
		},
		{
			name: "issue with all fields",
			output: `<review_issues>
[{
  "id": "custom-id",
  "type": "security",
  "severity": "critical",
  "file": "auth.go",
  "line_start": 100,
  "line_end": 110,
  "title": "Hardcoded Secret",
  "description": "API key is hardcoded in source",
  "suggestion": "Use environment variables",
  "code_snippet": "apiKey := \"sk-1234\""
}]
</review_issues>`,
			wantCount:  1,
			wantTitles: []string{"Hardcoded Secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := ParseReviewIssuesFromOutput(tt.output)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(issues) != tt.wantCount {
				t.Errorf("got %d issues, want %d", len(issues), tt.wantCount)
			}

			for i, wantTitle := range tt.wantTitles {
				if i >= len(issues) {
					t.Errorf("missing issue at index %d with title %q", i, wantTitle)
					continue
				}
				if issues[i].Title != wantTitle {
					t.Errorf("issue[%d].Title = %q, want %q", i, issues[i].Title, wantTitle)
				}
			}

			// Verify IDs are assigned
			for i, issue := range issues {
				if issue.ID == "" {
					t.Errorf("issue[%d] should have an ID assigned", i)
				}
				if issue.CreatedAt.IsZero() {
					t.Errorf("issue[%d] should have CreatedAt assigned", i)
				}
			}
		})
	}
}

func TestParseReviewIssuesFromOutput_PreservesCustomID(t *testing.T) {
	output := `<review_issues>
[{"id": "my-custom-id", "type": "security", "severity": "critical", "file": "main.go", "title": "Test", "description": "Desc"}]
</review_issues>`

	issues, err := ParseReviewIssuesFromOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}

	if issues[0].ID != "my-custom-id" {
		t.Errorf("ID = %q, want %q", issues[0].ID, "my-custom-id")
	}
}

func TestValidateReviewIssue(t *testing.T) {
	validIssue := ReviewIssue{
		Type:        SecurityReview,
		Severity:    string(SeverityCritical),
		File:        "main.go",
		Title:       "SQL Injection",
		Description: "User input not sanitized",
	}

	tests := []struct {
		name    string
		issue   ReviewIssue
		wantErr error
	}{
		{
			name:    "valid issue",
			issue:   validIssue,
			wantErr: nil,
		},
		{
			name: "empty title",
			issue: ReviewIssue{
				Severity:    string(SeverityCritical),
				File:        "main.go",
				Title:       "",
				Description: "Some description",
			},
			wantErr: ErrEmptyTitle,
		},
		{
			name: "whitespace only title",
			issue: ReviewIssue{
				Severity:    string(SeverityCritical),
				File:        "main.go",
				Title:       "   ",
				Description: "Some description",
			},
			wantErr: ErrEmptyTitle,
		},
		{
			name: "empty description",
			issue: ReviewIssue{
				Severity:    string(SeverityCritical),
				File:        "main.go",
				Title:       "Some title",
				Description: "",
			},
			wantErr: ErrEmptyDescription,
		},
		{
			name: "empty file",
			issue: ReviewIssue{
				Severity:    string(SeverityCritical),
				File:        "",
				Title:       "Some title",
				Description: "Some description",
			},
			wantErr: ErrEmptyFile,
		},
		{
			name: "invalid severity",
			issue: ReviewIssue{
				Severity:    "unknown",
				File:        "main.go",
				Title:       "Some title",
				Description: "Some description",
			},
			wantErr: ErrInvalidSeverity,
		},
		{
			name: "empty severity",
			issue: ReviewIssue{
				Severity:    "",
				File:        "main.go",
				Title:       "Some title",
				Description: "Some description",
			},
			wantErr: ErrInvalidSeverity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReviewIssue(tt.issue)

			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected error %v, got nil", tt.wantErr)
				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestDeduplicateIssues(t *testing.T) {
	tests := []struct {
		name      string
		issues    []ReviewIssue
		wantCount int
	}{
		{
			name:      "empty slice",
			issues:    []ReviewIssue{},
			wantCount: 0,
		},
		{
			name:      "nil slice",
			issues:    nil,
			wantCount: 0,
		},
		{
			name: "no duplicates",
			issues: []ReviewIssue{
				{File: "a.go", LineStart: 1, Title: "Issue A"},
				{File: "b.go", LineStart: 2, Title: "Issue B"},
			},
			wantCount: 2,
		},
		{
			name: "exact duplicates",
			issues: []ReviewIssue{
				{File: "a.go", LineStart: 10, Title: "SQL Injection"},
				{File: "a.go", LineStart: 10, Title: "SQL Injection"},
				{File: "a.go", LineStart: 10, Title: "SQL Injection"},
			},
			wantCount: 1,
		},
		{
			name: "same file different lines",
			issues: []ReviewIssue{
				{File: "a.go", LineStart: 10, Title: "Issue"},
				{File: "a.go", LineStart: 20, Title: "Issue"},
			},
			wantCount: 2,
		},
		{
			name: "same file and line different titles",
			issues: []ReviewIssue{
				{File: "a.go", LineStart: 10, Title: "Issue A"},
				{File: "a.go", LineStart: 10, Title: "Issue B"},
			},
			wantCount: 2,
		},
		{
			name: "mixed duplicates and unique",
			issues: []ReviewIssue{
				{File: "a.go", LineStart: 10, Title: "Duplicate"},
				{File: "b.go", LineStart: 20, Title: "Unique"},
				{File: "a.go", LineStart: 10, Title: "Duplicate"},
				{File: "c.go", LineStart: 30, Title: "Another"},
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeduplicateIssues(tt.issues)

			if len(result) != tt.wantCount {
				t.Errorf("got %d issues, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestDeduplicateIssues_KeepsFirstOccurrence(t *testing.T) {
	issues := []ReviewIssue{
		{ID: "first", File: "a.go", LineStart: 10, Title: "Issue", Description: "First description"},
		{ID: "second", File: "a.go", LineStart: 10, Title: "Issue", Description: "Second description"},
	}

	result := DeduplicateIssues(issues)

	if len(result) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result))
	}

	if result[0].ID != "first" {
		t.Errorf("expected first occurrence to be kept, got ID %q", result[0].ID)
	}

	if result[0].Description != "First description" {
		t.Errorf("expected first description, got %q", result[0].Description)
	}
}

func TestMergeIssues(t *testing.T) {
	tests := []struct {
		name      string
		existing  []ReviewIssue
		new       []ReviewIssue
		wantCount int
	}{
		{
			name:      "both empty",
			existing:  []ReviewIssue{},
			new:       []ReviewIssue{},
			wantCount: 0,
		},
		{
			name:     "empty existing",
			existing: []ReviewIssue{},
			new: []ReviewIssue{
				{File: "a.go", LineStart: 10, Title: "New Issue"},
			},
			wantCount: 1,
		},
		{
			name: "empty new",
			existing: []ReviewIssue{
				{File: "a.go", LineStart: 10, Title: "Existing Issue"},
			},
			new:       []ReviewIssue{},
			wantCount: 1,
		},
		{
			name: "no overlap",
			existing: []ReviewIssue{
				{File: "a.go", LineStart: 10, Title: "Issue A"},
			},
			new: []ReviewIssue{
				{File: "b.go", LineStart: 20, Title: "Issue B"},
			},
			wantCount: 2,
		},
		{
			name: "complete overlap",
			existing: []ReviewIssue{
				{File: "a.go", LineStart: 10, Title: "Issue A"},
			},
			new: []ReviewIssue{
				{File: "a.go", LineStart: 10, Title: "Issue A"},
			},
			wantCount: 1,
		},
		{
			name: "partial overlap",
			existing: []ReviewIssue{
				{File: "a.go", LineStart: 10, Title: "Issue A"},
				{File: "b.go", LineStart: 20, Title: "Issue B"},
			},
			new: []ReviewIssue{
				{File: "a.go", LineStart: 10, Title: "Issue A"}, // duplicate
				{File: "c.go", LineStart: 30, Title: "Issue C"}, // new
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeIssues(tt.existing, tt.new)

			if len(result) != tt.wantCount {
				t.Errorf("got %d issues, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestMergeIssues_PreservesExisting(t *testing.T) {
	existing := []ReviewIssue{
		{ID: "existing-id", File: "a.go", LineStart: 10, Title: "Issue", Description: "Existing"},
	}
	new := []ReviewIssue{
		{ID: "new-id", File: "a.go", LineStart: 10, Title: "Issue", Description: "New"},
	}

	result := MergeIssues(existing, new)

	if len(result) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result))
	}

	if result[0].ID != "existing-id" {
		t.Errorf("expected existing issue to be preserved, got ID %q", result[0].ID)
	}
}

func TestFilterIssuesBySeverity(t *testing.T) {
	issues := []ReviewIssue{
		{Title: "Critical", Severity: string(SeverityCritical)},
		{Title: "Major", Severity: string(SeverityMajor)},
		{Title: "Minor", Severity: string(SeverityMinor)},
		{Title: "Info", Severity: string(SeverityInfo)},
	}

	tests := []struct {
		name        string
		minSeverity string
		wantTitles  []string
	}{
		{
			name:        "filter to critical only",
			minSeverity: string(SeverityCritical),
			wantTitles:  []string{"Critical"},
		},
		{
			name:        "filter to major and above",
			minSeverity: string(SeverityMajor),
			wantTitles:  []string{"Critical", "Major"},
		},
		{
			name:        "filter to minor and above",
			minSeverity: string(SeverityMinor),
			wantTitles:  []string{"Critical", "Major", "Minor"},
		},
		{
			name:        "filter to info (all)",
			minSeverity: string(SeverityInfo),
			wantTitles:  []string{"Critical", "Major", "Minor", "Info"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterIssuesBySeverity(issues, tt.minSeverity)

			if len(result) != len(tt.wantTitles) {
				t.Errorf("got %d issues, want %d", len(result), len(tt.wantTitles))
				return
			}

			for i, wantTitle := range tt.wantTitles {
				if result[i].Title != wantTitle {
					t.Errorf("result[%d].Title = %q, want %q", i, result[i].Title, wantTitle)
				}
			}
		})
	}
}

func TestFilterIssuesBySeverity_EmptySlice(t *testing.T) {
	result := FilterIssuesBySeverity([]ReviewIssue{}, string(SeverityCritical))
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}

func TestGroupIssuesByFile(t *testing.T) {
	issues := []ReviewIssue{
		{File: "main.go", Title: "Issue 1"},
		{File: "utils.go", Title: "Issue 2"},
		{File: "main.go", Title: "Issue 3"},
		{File: "main.go", Title: "Issue 4"},
		{File: "handler.go", Title: "Issue 5"},
	}

	result := GroupIssuesByFile(issues)

	if len(result) != 3 {
		t.Errorf("expected 3 file groups, got %d", len(result))
	}

	if len(result["main.go"]) != 3 {
		t.Errorf("expected 3 issues in main.go, got %d", len(result["main.go"]))
	}

	if len(result["utils.go"]) != 1 {
		t.Errorf("expected 1 issue in utils.go, got %d", len(result["utils.go"]))
	}

	if len(result["handler.go"]) != 1 {
		t.Errorf("expected 1 issue in handler.go, got %d", len(result["handler.go"]))
	}
}

func TestGroupIssuesByFile_Empty(t *testing.T) {
	result := GroupIssuesByFile([]ReviewIssue{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestGroupIssuesByAgent(t *testing.T) {
	issues := []ReviewIssue{
		{Type: SecurityReview, Title: "Security 1"},
		{Type: PerformanceReview, Title: "Perf 1"},
		{Type: SecurityReview, Title: "Security 2"},
		{Type: StyleReview, Title: "Style 1"},
		{Type: SecurityReview, Title: "Security 3"},
	}

	result := GroupIssuesByAgent(issues)

	if len(result) != 3 {
		t.Errorf("expected 3 agent groups, got %d", len(result))
	}

	if len(result[SecurityReview]) != 3 {
		t.Errorf("expected 3 security issues, got %d", len(result[SecurityReview]))
	}

	if len(result[PerformanceReview]) != 1 {
		t.Errorf("expected 1 performance issue, got %d", len(result[PerformanceReview]))
	}

	if len(result[StyleReview]) != 1 {
		t.Errorf("expected 1 style issue, got %d", len(result[StyleReview]))
	}
}

func TestGroupIssuesByAgent_Empty(t *testing.T) {
	result := GroupIssuesByAgent([]ReviewIssue{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestCompareSeverity(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{"critical vs critical", string(SeverityCritical), string(SeverityCritical), 0},
		{"critical vs major", string(SeverityCritical), string(SeverityMajor), -1},
		{"major vs critical", string(SeverityMajor), string(SeverityCritical), 1},
		{"major vs major", string(SeverityMajor), string(SeverityMajor), 0},
		{"minor vs info", string(SeverityMinor), string(SeverityInfo), -1},
		{"info vs minor", string(SeverityInfo), string(SeverityMinor), 1},
		{"info vs info", string(SeverityInfo), string(SeverityInfo), 0},
		{"critical vs info", string(SeverityCritical), string(SeverityInfo), -1},
		{"info vs critical", string(SeverityInfo), string(SeverityCritical), 1},
		{"unknown vs critical", "unknown", string(SeverityCritical), 1},
		{"unknown vs unknown", "unknown", "unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareSeverity(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareSeverity(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsValidSeverity(t *testing.T) {
	validSeverities := []string{
		string(SeverityCritical),
		string(SeverityMajor),
		string(SeverityMinor),
		string(SeverityInfo),
	}

	for _, s := range validSeverities {
		if !isValidSeverity(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}

	invalidSeverities := []string{"", "unknown", "high", "low", "CRITICAL", "Critical"}

	for _, s := range invalidSeverities {
		if isValidSeverity(s) {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestExtractPartialIssues(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantCount int
	}{
		{
			name:      "empty content",
			content:   "",
			wantCount: 0,
		},
		{
			name:      "no JSON objects",
			content:   "just some text",
			wantCount: 0,
		},
		{
			name:      "single valid object",
			content:   `{"title": "Test", "description": "Desc"}`,
			wantCount: 1,
		},
		{
			name:      "multiple valid objects",
			content:   `{"title": "A"} some garbage {"title": "B", "description": "D"}`,
			wantCount: 2,
		},
		{
			name:      "nested braces in strings",
			content:   `{"title": "Test {}", "description": "Has braces {}"}`,
			wantCount: 1,
		},
		{
			name:      "broken JSON object",
			content:   `{"title": "Broken`,
			wantCount: 0,
		},
		{
			name:      "valid followed by broken",
			content:   `{"title": "Valid", "description": "D"} {"title": "Broken`,
			wantCount: 1,
		},
		{
			name:      "object without meaningful content",
			content:   `{"foo": "bar"}`,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPartialIssues(tt.content)
			if len(result) != tt.wantCount {
				t.Errorf("got %d issues, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestIssueDedupeKey(t *testing.T) {
	issue := ReviewIssue{
		File:      "main.go",
		LineStart: 42,
		Title:     "SQL Injection",
	}

	key := issueDedupeKey(issue)
	expected := "main.go:42:SQL Injection"

	if key != expected {
		t.Errorf("issueDedupeKey() = %q, want %q", key, expected)
	}
}

func TestParseReviewIssuesFromOutput_InvalidJSON(t *testing.T) {
	output := `<review_issues>
this is not valid json at all
</review_issues>`

	issues, err := ParseReviewIssuesFromOutput(output)

	// Should return error since no valid issues could be extracted
	if err == nil {
		t.Error("expected error for invalid JSON")
	}

	if !errors.Is(err, ErrInvalidJSON) {
		t.Errorf("expected ErrInvalidJSON, got %v", err)
	}

	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

func TestParseReviewIssuesFromOutput_TypeConversion(t *testing.T) {
	output := `<review_issues>
[{
  "type": "security",
  "severity": "critical",
  "file": "auth.go",
  "line_start": 100,
  "line_end": 110,
  "title": "Test Issue",
  "description": "Test Description"
}]
</review_issues>`

	issues, err := ParseReviewIssuesFromOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}

	issue := issues[0]

	if issue.Type != SecurityReview {
		t.Errorf("Type = %q, want %q", issue.Type, SecurityReview)
	}

	if issue.Severity != string(SeverityCritical) {
		t.Errorf("Severity = %q, want %q", issue.Severity, SeverityCritical)
	}

	if issue.LineStart != 100 {
		t.Errorf("LineStart = %d, want 100", issue.LineStart)
	}

	if issue.LineEnd != 110 {
		t.Errorf("LineEnd = %d, want 110", issue.LineEnd)
	}
}
