package worktree

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/logging"
)

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "string shorter than max",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "string equal to max",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "string longer than max",
			input:  "hello world",
			maxLen: 5,
			want:   "hello...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "max length zero",
			input:  "hello",
			maxLen: 0,
			want:   "...",
		},
		{
			name:   "unicode string truncation - full string fits",
			input:  "hello 世界",
			maxLen: 20, // "hello " (6 bytes) + 世界 (6 bytes for 2 chars) = 12 bytes total
			want:   "hello 世界",
		},
		{
			name:   "unicode string truncation - byte based",
			input:  "hello 世界",
			maxLen: 6, // Only "hello " fits
			want:   "hello ...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateOutput(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateOutput(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestSetLogger(t *testing.T) {
	m := &Manager{repoDir: "/tmp/test"}

	// Initially logger is nil
	if m.logger != nil {
		t.Error("expected logger to be nil initially")
	}

	// Create a test logger
	logger := logging.NopLogger()

	// Set the logger
	m.SetLogger(logger)

	// Verify logger is set
	if m.logger != logger {
		t.Error("SetLogger did not set the logger correctly")
	}
}

func TestCherryPickConflictError(t *testing.T) {
	err := &CherryPickConflictError{
		Commit:       "abc123",
		SourceBranch: "feature-branch",
		Output:       "CONFLICT in file.txt",
	}

	expected := "cherry-pick conflict on commit abc123 from feature-branch"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestBranchInfo(t *testing.T) {
	// Test that BranchInfo struct works as expected
	info := BranchInfo{
		Name:      "feature-branch",
		IsCurrent: true,
		IsMain:    false,
	}

	if info.Name != "feature-branch" {
		t.Errorf("Name = %q, want %q", info.Name, "feature-branch")
	}
	if !info.IsCurrent {
		t.Error("IsCurrent = false, want true")
	}
	if info.IsMain {
		t.Error("IsMain = true, want false")
	}
}
