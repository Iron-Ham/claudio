package issue

import (
	"testing"
)

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected Provider
		wantErr  bool
	}{
		{
			name:     "GitHub issue URL",
			url:      "https://github.com/Iron-Ham/claudio/issues/163",
			expected: ProviderGitHub,
		},
		{
			name:     "GitHub PR URL",
			url:      "https://github.com/owner/repo/pull/42",
			expected: ProviderGitHub,
		},
		{
			name:     "Linear issue URL",
			url:      "https://linear.app/myteam/issue/ENG-123/some-title",
			expected: ProviderLinear,
		},
		{
			name:     "Linear issue URL without title",
			url:      "https://linear.app/myteam/issue/ENG-456",
			expected: ProviderLinear,
		},
		{
			name:     "Notion page URL",
			url:      "https://notion.so/workspace/Page-Title-abc123",
			expected: ProviderNotion,
		},
		{
			name:     "Notion site URL",
			url:      "https://myteam.notion.site/Task-abc123",
			expected: ProviderNotion,
		},
		{
			name:     "Unknown provider",
			url:      "https://jira.atlassian.com/browse/PROJ-123",
			expected: ProviderUnknown,
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: ProviderUnknown,
			wantErr:  false, // Empty URL parses but returns unknown
		},
		{
			name:     "Invalid URL",
			url:      "not a url at all",
			expected: ProviderUnknown,
			wantErr:  false, // url.Parse is lenient
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectProvider(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("DetectProvider() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantNum   string
		wantErr   bool
	}{
		{
			name:      "valid issue URL",
			url:       "https://github.com/Iron-Ham/claudio/issues/163",
			wantOwner: "Iron-Ham",
			wantRepo:  "claudio",
			wantNum:   "163",
		},
		{
			name:      "issue URL with trailing slash",
			url:       "https://github.com/owner/repo/issues/42/",
			wantOwner: "",
			wantRepo:  "",
			wantNum:   "",
			wantErr:   true,
		},
		{
			name:    "PR URL (not an issue)",
			url:     "https://github.com/owner/repo/pull/42",
			wantErr: true,
		},
		{
			name:    "invalid format",
			url:     "https://github.com/owner",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the regex used in closeGitHub
			re := gitHubIssueRegex
			matches := re.FindStringSubmatch(tt.url)

			if tt.wantErr {
				if len(matches) == 4 {
					t.Errorf("expected no match for %q, got %v", tt.url, matches)
				}
				return
			}

			if len(matches) != 4 {
				t.Errorf("expected match for %q, got %v", tt.url, matches)
				return
			}

			if matches[1] != tt.wantOwner {
				t.Errorf("owner = %q, want %q", matches[1], tt.wantOwner)
			}
			if matches[2] != tt.wantRepo {
				t.Errorf("repo = %q, want %q", matches[2], tt.wantRepo)
			}
			if matches[3] != tt.wantNum {
				t.Errorf("number = %q, want %q", matches[3], tt.wantNum)
			}
		})
	}
}

func TestParseLinearURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantID    string
		wantMatch bool
	}{
		{
			name:      "valid Linear URL with title",
			url:       "https://linear.app/myteam/issue/ENG-123/some-task-title",
			wantID:    "ENG-123",
			wantMatch: true,
		},
		{
			name:      "valid Linear URL without title",
			url:       "https://linear.app/myteam/issue/PROJ-456",
			wantID:    "PROJ-456",
			wantMatch: true,
		},
		{
			name:      "Linear URL with long ID",
			url:       "https://linear.app/workspace/issue/ABC-99999/title",
			wantID:    "ABC-99999",
			wantMatch: true,
		},
		{
			name:      "not a Linear issue URL",
			url:       "https://linear.app/myteam/project/123",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := linearIssueRegex
			matches := re.FindStringSubmatch(tt.url)

			if !tt.wantMatch {
				if len(matches) >= 2 {
					t.Errorf("expected no match for %q, got %v", tt.url, matches)
				}
				return
			}

			if len(matches) < 2 {
				t.Errorf("expected match for %q, got none", tt.url)
				return
			}

			if matches[1] != tt.wantID {
				t.Errorf("issue ID = %q, want %q", matches[1], tt.wantID)
			}
		})
	}
}

// Exported regexes for testing (defined in service.go)
var (
	gitHubIssueRegex = gitHubRegex
	linearIssueRegex = linearRegex
)
