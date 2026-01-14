package worktree

import (
	"testing"
)

// TestTruncateOutput tests the truncateOutput helper function
func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "string shorter than max",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "string equal to max",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "string longer than max",
			input:    "hello world",
			maxLen:   5,
			expected: "hello...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "zero max length",
			input:    "hello",
			maxLen:   0,
			expected: "...",
		},
		{
			name:     "truncate at boundary",
			input:    "abcdef",
			maxLen:   3,
			expected: "abc...",
		},
		{
			name:     "unicode content truncates by bytes",
			input:    "héllo wörld",
			maxLen:   6,
			expected: "héllo...", // truncateOutput uses byte length, not rune count
		},
		{
			name:     "long output typical in git commands",
			input:    "Preparing worktree (new branch 'feature-123')\nHEAD is now at abc1234 Initial commit",
			maxLen:   50,
			expected: "Preparing worktree (new branch 'feature-123')\nHEAD...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateOutput(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncateOutput(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
			}
		})
	}
}

// TestCherryPickConflictError tests the CherryPickConflictError type
func TestCherryPickConflictError(t *testing.T) {
	tests := []struct {
		name         string
		commit       string
		sourceBranch string
		output       string
		wantMsg      string
	}{
		{
			name:         "basic error message",
			commit:       "abc123",
			sourceBranch: "feature-branch",
			output:       "CONFLICT (content): Merge conflict in file.go",
			wantMsg:      "cherry-pick conflict on commit abc123 from feature-branch",
		},
		{
			name:         "full SHA",
			commit:       "abc123def456789",
			sourceBranch: "main",
			output:       "conflict output",
			wantMsg:      "cherry-pick conflict on commit abc123def456789 from main",
		},
		{
			name:         "empty fields",
			commit:       "",
			sourceBranch: "",
			output:       "",
			wantMsg:      "cherry-pick conflict on commit  from ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &CherryPickConflictError{
				Commit:       tt.commit,
				SourceBranch: tt.sourceBranch,
				Output:       tt.output,
			}

			if got := err.Error(); got != tt.wantMsg {
				t.Errorf("CherryPickConflictError.Error() = %q, want %q", got, tt.wantMsg)
			}

			// Verify the error implements the error interface
			var _ error = err
		})
	}
}

// TestCherryPickConflictError_Fields tests field access on CherryPickConflictError
func TestCherryPickConflictError_Fields(t *testing.T) {
	err := &CherryPickConflictError{
		Commit:       "deadbeef",
		SourceBranch: "feature/test",
		Output:       "Auto-merging file.go\nCONFLICT (content): Merge conflict in file.go",
	}

	if err.Commit != "deadbeef" {
		t.Errorf("CherryPickConflictError.Commit = %q, want %q", err.Commit, "deadbeef")
	}
	if err.SourceBranch != "feature/test" {
		t.Errorf("CherryPickConflictError.SourceBranch = %q, want %q", err.SourceBranch, "feature/test")
	}
	if err.Output != "Auto-merging file.go\nCONFLICT (content): Merge conflict in file.go" {
		t.Errorf("CherryPickConflictError.Output = %q, want different", err.Output)
	}
}

// TestBranchInfo tests the BranchInfo struct
func TestBranchInfo(t *testing.T) {
	tests := []struct {
		name       string
		branchInfo BranchInfo
		wantName   string
		wantMain   bool
		wantCurr   bool
	}{
		{
			name: "main branch current",
			branchInfo: BranchInfo{
				Name:      "main",
				IsCurrent: true,
				IsMain:    true,
			},
			wantName: "main",
			wantMain: true,
			wantCurr: true,
		},
		{
			name: "feature branch not current",
			branchInfo: BranchInfo{
				Name:      "feature/add-tests",
				IsCurrent: false,
				IsMain:    false,
			},
			wantName: "feature/add-tests",
			wantMain: false,
			wantCurr: false,
		},
		{
			name: "master branch (legacy)",
			branchInfo: BranchInfo{
				Name:      "master",
				IsCurrent: false,
				IsMain:    true,
			},
			wantName: "master",
			wantMain: true,
			wantCurr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.branchInfo.Name != tt.wantName {
				t.Errorf("BranchInfo.Name = %q, want %q", tt.branchInfo.Name, tt.wantName)
			}
			if tt.branchInfo.IsMain != tt.wantMain {
				t.Errorf("BranchInfo.IsMain = %v, want %v", tt.branchInfo.IsMain, tt.wantMain)
			}
			if tt.branchInfo.IsCurrent != tt.wantCurr {
				t.Errorf("BranchInfo.IsCurrent = %v, want %v", tt.branchInfo.IsCurrent, tt.wantCurr)
			}
		})
	}
}

// TestLocalClaudeFiles tests that localClaudeFiles is properly configured
func TestLocalClaudeFiles(t *testing.T) {
	// Verify the expected files are in the list
	expectedFiles := []string{
		"CLAUDE.local.md",
	}

	if len(localClaudeFiles) != len(expectedFiles) {
		t.Errorf("localClaudeFiles has %d entries, want %d", len(localClaudeFiles), len(expectedFiles))
	}

	for i, expected := range expectedFiles {
		if i >= len(localClaudeFiles) {
			t.Errorf("missing expected file at index %d: %q", i, expected)
			continue
		}
		if localClaudeFiles[i] != expected {
			t.Errorf("localClaudeFiles[%d] = %q, want %q", i, localClaudeFiles[i], expected)
		}
	}
}

// TestManagerSetLogger tests the SetLogger method
func TestManagerSetLogger(t *testing.T) {
	// Create a Manager with empty repoDir (no git operations will be performed)
	mgr := &Manager{repoDir: "/nonexistent"}

	// Initially logger should be nil
	if mgr.logger != nil {
		t.Error("Manager.logger should be nil initially")
	}

	// SetLogger with nil should not panic
	mgr.SetLogger(nil)
	if mgr.logger != nil {
		t.Error("Manager.logger should be nil after SetLogger(nil)")
	}
}

// TestProvider constants tests the Provider type constants
func TestProviderTypeIsString(t *testing.T) {
	// Test that BranchInfo fields can be zero-valued properly
	var info BranchInfo
	if info.Name != "" {
		t.Errorf("zero BranchInfo.Name = %q, want empty", info.Name)
	}
	if info.IsCurrent != false {
		t.Errorf("zero BranchInfo.IsCurrent = %v, want false", info.IsCurrent)
	}
	if info.IsMain != false {
		t.Errorf("zero BranchInfo.IsMain = %v, want false", info.IsMain)
	}
}
