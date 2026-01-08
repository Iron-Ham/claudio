package pr

import (
	"testing"
)

func TestExtractIssueReference(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "simple hash reference",
			text:     "Fix the login bug #42",
			expected: "#42",
		},
		{
			name:     "fixes keyword",
			text:     "Fix the login bug (fixes #123)",
			expected: "#123",
		},
		{
			name:     "closes keyword",
			text:     "Add new feature closes #456",
			expected: "#456",
		},
		{
			name:     "resolves keyword",
			text:     "resolves #789 - memory leak",
			expected: "#789",
		},
		{
			name:     "case insensitive",
			text:     "FIXES #100",
			expected: "#100",
		},
		{
			name:     "no issue reference",
			text:     "Just a regular task description",
			expected: "",
		},
		{
			name:     "multiple references returns first match",
			text:     "Fix #1 and closes #2",
			expected: "#1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractIssueReference(tt.text)
			if result != tt.expected {
				t.Errorf("ExtractIssueReference(%q) = %q, want %q", tt.text, result, tt.expected)
			}
		})
	}
}

func TestResolveReviewers(t *testing.T) {
	tests := []struct {
		name             string
		changedFiles     []string
		defaultReviewers []string
		byPath           map[string][]string
		expectedCount    int
		mustContain      []string
	}{
		{
			name:             "default reviewers only",
			changedFiles:     []string{"main.go"},
			defaultReviewers: []string{"alice", "bob"},
			byPath:           map[string][]string{},
			expectedCount:    2,
			mustContain:      []string{"alice", "bob"},
		},
		{
			name:             "path-based reviewer matching",
			changedFiles:     []string{"internal/tui/view.go"},
			defaultReviewers: []string{},
			byPath: map[string][]string{
				"internal/tui/**": {"ui-team"},
			},
			expectedCount: 1,
			mustContain:   []string{"ui-team"},
		},
		{
			name:             "combined default and path-based",
			changedFiles:     []string{"internal/tui/view.go", "README.md"},
			defaultReviewers: []string{"alice"},
			byPath: map[string][]string{
				"internal/tui/**": {"ui-team"},
			},
			expectedCount: 2,
			mustContain:   []string{"alice", "ui-team"},
		},
		{
			name:             "removes @ prefix",
			changedFiles:     []string{"main.go"},
			defaultReviewers: []string{"@alice", "@bob"},
			byPath:           map[string][]string{},
			expectedCount:    2,
			mustContain:      []string{"alice", "bob"},
		},
		{
			name:             "no duplicates",
			changedFiles:     []string{"internal/tui/view.go"},
			defaultReviewers: []string{"alice"},
			byPath: map[string][]string{
				"internal/tui/**": {"alice", "bob"},
			},
			expectedCount: 2,
			mustContain:   []string{"alice", "bob"},
		},
		{
			name:             "empty inputs",
			changedFiles:     []string{},
			defaultReviewers: []string{},
			byPath:           map[string][]string{},
			expectedCount:    0,
			mustContain:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveReviewers(tt.changedFiles, tt.defaultReviewers, tt.byPath)

			if len(result) != tt.expectedCount {
				t.Errorf("ResolveReviewers() returned %d reviewers, want %d", len(result), tt.expectedCount)
			}

			for _, expected := range tt.mustContain {
				found := false
				for _, r := range result {
					if r == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ResolveReviewers() result missing expected reviewer %q", expected)
				}
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     TemplateData
		contains []string
		wantErr  bool
	}{
		{
			name:     "basic template",
			template: "## Summary\n{{ .AISummary }}\n\n## Task\n{{ .Task }}",
			data: TemplateData{
				AISummary: "This is the AI summary",
				Task:      "Fix the bug",
			},
			contains: []string{"## Summary", "This is the AI summary", "## Task", "Fix the bug"},
			wantErr:  false,
		},
		{
			name:     "template with changed files",
			template: "## Changed Files\n{{range .ChangedFiles}}- {{.}}\n{{end}}",
			data: TemplateData{
				ChangedFiles: []string{"file1.go", "file2.go"},
			},
			contains: []string{"- file1.go", "- file2.go"},
			wantErr:  false,
		},
		{
			name:     "template with linked issue",
			template: "Closes {{ .LinkedIssue }}",
			data: TemplateData{
				LinkedIssue: "#42",
			},
			contains: []string{"Closes #42"},
			wantErr:  false,
		},
		{
			name:     "invalid template",
			template: "{{ .Invalid }}",
			data:     TemplateData{},
			contains: []string{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderTemplate(tt.template, tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("RenderTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				for _, expected := range tt.contains {
					if !containsString(result, expected) {
						t.Errorf("RenderTemplate() result missing expected string %q", expected)
					}
				}
			}
		})
	}
}

func TestFormatClosesClause(t *testing.T) {
	tests := []struct {
		name     string
		issues   []string
		expected string
	}{
		{
			name:     "single issue",
			issues:   []string{"42"},
			expected: "Closes #42",
		},
		{
			name:     "single issue with hash",
			issues:   []string{"#42"},
			expected: "Closes #42",
		},
		{
			name:     "multiple issues",
			issues:   []string{"42", "43"},
			expected: "Closes #42\nCloses #43",
		},
		{
			name:     "empty issues",
			issues:   []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatClosesClause(tt.issues)
			if result != tt.expected {
				t.Errorf("FormatClosesClause(%v) = %q, want %q", tt.issues, result, tt.expected)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
