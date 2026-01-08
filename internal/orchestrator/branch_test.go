package orchestrator

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/config"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "implement user authentication",
			expected: "implement-user-authentication",
		},
		{
			name:     "uppercase converted",
			input:    "Fix Bug In Parser",
			expected: "fix-bug-in-parser",
		},
		{
			name:     "special characters removed",
			input:    "add: user-auth (v2)",
			expected: "add-user-auth-v2",
		},
		{
			name:     "long text truncated",
			input:    "this is a very long task description that exceeds the limit",
			expected: "this-is-a-very-long-task-descr",
		},
		{
			name:     "trailing dash removed",
			input:    "fix bug: ",
			expected: "fix-bug",
		},
		{
			name:     "numbers preserved",
			input:    "update api v2",
			expected: "update-api-v2",
		},
		{
			name:     "multiple spaces collapsed",
			input:    "fix  multiple   spaces",
			expected: "fix--multiple---spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := slugify(tt.input)
			if result != tt.expected {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateBranchName(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		includeID  bool
		instanceID string
		slug       string
		expected   string
	}{
		{
			name:       "default prefix with ID",
			prefix:     "claudio",
			includeID:  true,
			instanceID: "abc12345",
			slug:       "fix-bug",
			expected:   "claudio/abc12345-fix-bug",
		},
		{
			name:       "custom prefix with ID",
			prefix:     "Iron-Ham",
			includeID:  true,
			instanceID: "def67890",
			slug:       "add-feature",
			expected:   "Iron-Ham/def67890-add-feature",
		},
		{
			name:       "custom prefix without ID",
			prefix:     "Iron-Ham",
			includeID:  false,
			instanceID: "xyz99999",
			slug:       "refactor-auth",
			expected:   "Iron-Ham/refactor-auth",
		},
		{
			name:       "default prefix without ID",
			prefix:     "claudio",
			includeID:  false,
			instanceID: "ignored",
			slug:       "update-deps",
			expected:   "claudio/update-deps",
		},
		{
			name:       "empty prefix uses fallback",
			prefix:     "",
			includeID:  true,
			instanceID: "abc123",
			slug:       "test-task",
			expected:   "claudio/abc123-test-task",
		},
		{
			name:       "feature prefix",
			prefix:     "feature",
			includeID:  false,
			instanceID: "ignored",
			slug:       "user-dashboard",
			expected:   "feature/user-dashboard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Branch: config.BranchConfig{
					Prefix:    tt.prefix,
					IncludeID: tt.includeID,
				},
			}

			// Create a minimal orchestrator with the config
			o := &Orchestrator{
				config: cfg,
			}

			result := o.generateBranchName(tt.instanceID, tt.slug)
			if result != tt.expected {
				t.Errorf("generateBranchName(%q, %q) with prefix=%q, includeID=%v = %q, want %q",
					tt.instanceID, tt.slug, tt.prefix, tt.includeID, result, tt.expected)
			}
		})
	}
}

func TestBranchPrefix(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		expected string
	}{
		{
			name:     "default prefix",
			prefix:   "claudio",
			expected: "claudio",
		},
		{
			name:     "custom prefix",
			prefix:   "Iron-Ham",
			expected: "Iron-Ham",
		},
		{
			name:     "empty prefix returns default",
			prefix:   "",
			expected: "claudio",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Branch: config.BranchConfig{
					Prefix: tt.prefix,
				},
			}

			o := &Orchestrator{
				config: cfg,
			}

			result := o.BranchPrefix()
			if result != tt.expected {
				t.Errorf("BranchPrefix() with config prefix=%q = %q, want %q",
					tt.prefix, result, tt.expected)
			}
		})
	}
}
